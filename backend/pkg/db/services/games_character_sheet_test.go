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

// TestUpdateGameNilCharacterSheetPreservesLabels covers the other half of the
// COALESCE decision. On create, nil means "no config" and becomes '{}'. On
// update, nil means "the caller did not supply one" — and must keep whatever the
// GM already set rather than silently wiping it, which is what coalescing to
// '{}' here would have done.
func TestUpdateGameNilCharacterSheetPreservesLabels(t *testing.T) {
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
		t.Errorf("an update that did not mention the sheet wiped the GM's labels: %s", updated.CharacterSheet)
	}
}
