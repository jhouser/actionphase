package db

import (
	"context"
	"testing"

	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"

	"github.com/jackc/pgx/v5/pgtype"
)

// TestCreateGameNilCharacterSheet pins the NOT NULL trap that broke every
// DB-backed test when games.character_sheet was first added.
//
// The column is NOT NULL DEFAULT '{}', but naming it in the INSERT column list
// disables that default — so a caller who builds CreateGameParams directly and
// leaves CharacterSheet nil sends a SQL NULL and trips the constraint. The
// service always supplies a validated value, but the test factories and several
// tests build the params struct themselves, and they are the ones that broke.
//
// The fix is COALESCE in the query rather than defaulting in Go, so nil is safe
// no matter which caller sends it. This test drives the raw query on purpose.
func TestCreateGameNilCharacterSheet(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "games", "sessions", "users")

	fixtures := testDB.SetupFixtures(t)
	queries := models.New(testDB.Pool)

	game, err := queries.CreateGame(context.Background(), models.CreateGameParams{
		Title:       "Game Without A Sheet Config",
		Description: pgtype.Text{String: "Built without setting CharacterSheet.", Valid: true},
		GmUserID:    int32(fixtures.TestUser.ID),
		IsPublic:    pgtype.Bool{Bool: true, Valid: true},
		// CharacterSheet deliberately left nil.
	})
	if err != nil {
		t.Fatalf("creating a game without a character sheet config must not fail: %v", err)
	}

	if string(game.CharacterSheet) != "{}" {
		t.Errorf("character_sheet = %q, want %q", game.CharacterSheet, "{}")
	}

	// And the stored value must parse as an empty config, not merely be non-null.
	config, err := core.UnmarshalCharacterSheetConfig(game.CharacterSheet)
	if err != nil {
		t.Fatalf("stored default does not parse: %v", err)
	}
	if config.Labels != nil {
		t.Errorf("expected no labels, got %+v", config.Labels)
	}
}

// TestUpdateGameNilCharacterSheetKeepsRawQuerySafe covers the raw query's nil
// guard, which exists only for a caller that builds UpdateGameParams by hand.
//
// This is NOT the API's contract. The service can never send nil (see
// TestGameService_UpdateGameUnsetsCharacterSheetLabels below) -- it marshals an
// empty config to '{}'. The COALESCE is here so a hand-built params struct does
// not trip the NOT NULL constraint, nothing more.
func TestUpdateGameNilCharacterSheetKeepsRawQuerySafe(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "games", "sessions", "users")

	fixtures := testDB.SetupFixtures(t)
	queries := models.New(testDB.Pool)
	ctx := context.Background()

	game, err := queries.CreateGame(ctx, models.CreateGameParams{
		Title:          "Game With Renamed Tabs",
		Description:    pgtype.Text{String: "Has GM label overrides.", Valid: true},
		GmUserID:       int32(fixtures.TestUser.ID),
		IsPublic:       pgtype.Bool{Bool: true, Valid: true},
		CharacterSheet: []byte(`{"labels":{"skills":"Approaches"}}`),
	})
	if err != nil {
		t.Fatalf("unexpected error creating game: %v", err)
	}

	updated, err := queries.UpdateGame(ctx, models.UpdateGameParams{
		ID:          game.ID,
		Title:       "Game With Renamed Tabs",
		Description: pgtype.Text{String: "Updated, but not for the sheet.", Valid: true},
		IsPublic:    pgtype.Bool{Bool: true, Valid: true},
		// CharacterSheet deliberately left nil.
	})
	if err != nil {
		t.Fatalf("unexpected error updating game: %v", err)
	}

	config, err := core.UnmarshalCharacterSheetConfig(updated.CharacterSheet)
	if err != nil {
		t.Fatalf("stored config does not parse: %v", err)
	}
	if config.Labels == nil || config.Labels.Skills != "Approaches" {
		t.Errorf("raw query with nil params must leave the column alone, got: %s", updated.CharacterSheet)
	}
}

// TestGameService_UpdateGameUnsetsCharacterSheetLabels pins how a GM clears tab
// labels back to the defaults.
//
// UpdateGame is a full replace, not a patch: an omitted `character_sheet` RESETS
// the labels. That is the unset path, and it is the only one -- the edit form
// clears all three boxes, buildCharacterSheetConfig returns undefined, and the
// key never reaches the wire. There is no separate "reset" verb, so if this
// starts preserving instead, the GM can rename a tab but never undo it.
//
// Driven through the service rather than the raw query on purpose. The query
// takes a nullable param and the service cannot produce nil for it, so a
// raw-query test proves nothing about what the API actually does.
func TestGameService_UpdateGameUnsetsCharacterSheetLabels(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "games", "sessions", "users")

	fixtures := testDB.SetupFixtures(t)
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	ctx := context.Background()

	game, err := gameService.CreateGame(ctx, core.CreateGameRequest{
		Title:       "Game With Renamed Tabs",
		Description: "Starts with GM label overrides.",
		GMUserID:    int32(fixtures.TestUser.ID),
		CommunityID: int32(fixtures.TestCommunity.ID),
		IsPublic:    true,
		CharacterSheet: core.CharacterSheetConfig{
			Labels: &core.CharacterSheetLabels{Skills: "Approaches", Numbers: "Stress"},
		},
	})
	core.AssertNoError(t, err, "Failed to create game")

	stored := core.CharacterSheetConfigForResponse(game.CharacterSheet)
	if stored == nil || stored.Labels == nil || stored.Labels.Skills != "Approaches" {
		t.Fatalf("setup did not persist the labels, got: %s", game.CharacterSheet)
	}

	// An update carrying no config -- exactly what the edit form sends once the
	// GM has emptied every label box.
	updated, err := gameService.UpdateGame(ctx, core.UpdateGameRequest{
		ID:          game.ID,
		Title:       "Game With Renamed Tabs",
		Description: "The GM cleared every label box.",
		IsPublic:    true,
		// CharacterSheet left as its zero value: no overrides.
	})
	core.AssertNoError(t, err, "Failed to update game")

	if string(updated.CharacterSheet) != "{}" {
		t.Errorf("clearing every label must store %q, got %q -- the GM cannot unset a renamed tab", "{}", updated.CharacterSheet)
	}
	if got := core.CharacterSheetConfigForResponse(updated.CharacterSheet); got != nil {
		t.Errorf("a cleared config must render as an absent key, got %+v", got)
	}
}

// TestGameService_UpdateGameReplacesCharacterSheetLabels pins the partial-replace
// half: supplying a config overwrites the stored one wholesale rather than
// merging into it, so dropping one label of two removes it.
func TestGameService_UpdateGameReplacesCharacterSheetLabels(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "games", "sessions", "users")

	fixtures := testDB.SetupFixtures(t)
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	ctx := context.Background()

	game, err := gameService.CreateGame(ctx, core.CreateGameRequest{
		Title:       "Game With Two Renamed Tabs",
		Description: "Starts with two GM label overrides.",
		GMUserID:    int32(fixtures.TestUser.ID),
		CommunityID: int32(fixtures.TestCommunity.ID),
		IsPublic:    true,
		CharacterSheet: core.CharacterSheetConfig{
			Labels: &core.CharacterSheetLabels{Skills: "Approaches", Numbers: "Stress"},
		},
	})
	core.AssertNoError(t, err, "Failed to create game")

	// Only Skills survives: Numbers was cleared in the form.
	updated, err := gameService.UpdateGame(ctx, core.UpdateGameRequest{
		ID:          game.ID,
		Title:       "Game With Two Renamed Tabs",
		Description: "The GM cleared only the Numbers label.",
		IsPublic:    true,
		CharacterSheet: core.CharacterSheetConfig{
			Labels: &core.CharacterSheetLabels{Skills: "Approaches"},
		},
	})
	core.AssertNoError(t, err, "Failed to update game")

	got := core.CharacterSheetConfigForResponse(updated.CharacterSheet)
	if got == nil || got.Labels == nil {
		t.Fatalf("expected the surviving label to persist, got: %s", updated.CharacterSheet)
	}
	if got.Labels.Skills != "Approaches" {
		t.Errorf("Skills = %q, want %q", got.Labels.Skills, "Approaches")
	}
	if got.Labels.Numbers != "" {
		t.Errorf("Numbers = %q, want it cleared -- the update replaces rather than merges", got.Labels.Numbers)
	}
}
