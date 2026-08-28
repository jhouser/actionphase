package characters

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"
	services "actionphase/pkg/db/services"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createCharacterFor inserts a character, optionally owned by a user.
func createCharacterFor(t *testing.T, testDB *core.TestDatabase, gameID int32, name, charType string, ownerID *int32) models.Character {
	t.Helper()
	owner := pgtype.Int4{Valid: false}
	if ownerID != nil {
		owner = pgtype.Int4{Int32: *ownerID, Valid: true}
	}
	char, err := models.New(testDB.Pool).CreateCharacter(context.Background(), models.CreateCharacterParams{
		GameID:        gameID,
		Name:          name,
		CharacterType: charType,
		UserID:        owner,
		Status:        pgtype.Text{String: "approved", Valid: true},
	})
	require.NoError(t, err)
	return char
}

// assignNPC gives control of an NPC to a user, the second way (besides owning a
// player character) that a character becomes controllable.
func assignNPC(t *testing.T, testDB *core.TestDatabase, characterID, assignedUserID, assignedByUserID int32) {
	t.Helper()
	_, err := testDB.Pool.Exec(context.Background(),
		`INSERT INTO npc_assignments (character_id, assigned_user_id, assigned_by_user_id) VALUES ($1, $2, $3)`,
		characterID, assignedUserID, assignedByUserID)
	require.NoError(t, err)
}

// namesIn collects the character names from the response body, which is what
// the Utility Drawer renders.
func namesIn(t *testing.T, body []byte) []string {
	t.Helper()
	var resp []*ControllableCharacterWithGameResponse
	require.NoError(t, json.Unmarshal(body, &resp))
	names := make([]string, 0, len(resp))
	for _, c := range resp {
		names = append(names, c.Name)
	}
	return names
}

func getControllable(t *testing.T, router http.Handler, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/v1/characters/controllable", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// The cross-game endpoint takes no game in the path, so the query itself is the
// only thing scoping what a caller sees. If it over-returned, a player would
// read characters out of games they have nothing to do with — a leak no status
// code reveals. These tests pin the scoping rules the drawer depends on.
func TestGetControllableAcrossGames_ScopesToTheCallersOwnCharacters(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "npc_assignments", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupStatsTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "xg_gm", "xg_gm@example.com")
	player := testDB.CreateTestUser(t, "xg_player", "xg_player@example.com")
	stranger := testDB.CreateTestUser(t, "xg_stranger", "xg_stranger@example.com")

	gameSvc := &services.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	game := testDB.CreateTestGameWithState(t, int32(gm.ID), "In Progress Game", core.GameStateInProgress)
	_, err := gameSvc.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	playerID := int32(player.ID)
	createCharacterFor(t, testDB, game.ID, "Player Own PC", "player_character", &playerID)

	strangerID := int32(stranger.ID)
	createCharacterFor(t, testDB, game.ID, "Someone Elses PC", "player_character", &strangerID)

	npc := createCharacterFor(t, testDB, game.ID, "Assigned NPC", "npc", nil)
	assignNPC(t, testDB, npc.ID, int32(player.ID), int32(gm.ID))

	createCharacterFor(t, testDB, game.ID, "Unassigned NPC", "npc", nil)

	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	rec := getControllable(t, router, playerToken)
	require.Equal(t, http.StatusOK, rec.Code)

	names := namesIn(t, rec.Body.Bytes())
	assert.ElementsMatch(t,
		[]string{"Player Own PC", "Assigned NPC"}, names,
		"a player controls their own PC and NPCs assigned to them — nothing else")
}

// The GM's entry is deliberately the whole cast, not just NPCs: with no game in
// scope the drawer would otherwise show them a fraction of their own game.
func TestGetControllableAcrossGames_GMSeesWholeCast(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "npc_assignments", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupStatsTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "cast_gm", "cast_gm@example.com")
	player := testDB.CreateTestUser(t, "cast_player", "cast_player@example.com")

	gameSvc := &services.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	game := testDB.CreateTestGameWithState(t, int32(gm.ID), "GM Game", core.GameStateInProgress)
	_, err := gameSvc.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	playerID := int32(player.ID)
	createCharacterFor(t, testDB, game.ID, "A Players PC", "player_character", &playerID)
	createCharacterFor(t, testDB, game.ID, "An Unassigned NPC", "npc", nil)

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)

	rec := getControllable(t, router, gmToken)
	require.Equal(t, http.StatusOK, rec.Code)

	names := namesIn(t, rec.Body.Bytes())
	assert.ElementsMatch(t,
		[]string{"A Players PC", "An Unassigned NPC"}, names,
		"the GM controls every character in their game")
}

// The endpoint answers "what am I currently playing". Characters in games that
// have not started, or that have finished, must not appear — sheets for those
// stay reachable from the game itself.
func TestGetControllableAcrossGames_OnlyInProgressGames(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "npc_assignments", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupStatsTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "state_gm", "state_gm@example.com")
	player := testDB.CreateTestUser(t, "state_player", "state_player@example.com")
	playerID := int32(player.ID)

	gameSvc := &services.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	for _, tc := range []struct{ state, charName string }{
		{core.GameStateInProgress, "Active Character"},
		{core.GameStateRecruitment, "Recruitment Character"},
		{core.GameStateCompleted, "Completed Character"},
	} {
		// Build each game as in_progress first: joining and casting happen
		// while a game is live, and a completed game is archived and
		// read-only. Then move it to the state under test, the way a real
		// game reaches that state.
		g := testDB.CreateTestGameWithState(t, int32(gm.ID), "Game "+tc.state, core.GameStateInProgress)
		_, err := gameSvc.AddGameParticipant(context.Background(), g.ID, int32(player.ID), "player")
		require.NoError(t, err)
		createCharacterFor(t, testDB, g.ID, tc.charName, "player_character", &playerID)

		if tc.state != core.GameStateInProgress {
			_, err := testDB.Pool.Exec(context.Background(),
				`UPDATE games SET state = $1 WHERE id = $2`, tc.state, g.ID)
			require.NoError(t, err)
		}
	}

	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	rec := getControllable(t, router, playerToken)
	require.Equal(t, http.StatusOK, rec.Code)

	names := namesIn(t, rec.Body.Bytes())
	assert.Equal(t, []string{"Active Character"}, names,
		"only characters in in_progress games are returned")
}

// Each entry has to carry its own game context: the drawer has no game in scope
// and would otherwise render defaults that read as a bug.
func TestGetControllableAcrossGames_CarriesGameContext(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "npc_assignments", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupStatsTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "ctx_gm", "ctx_gm@example.com")
	player := testDB.CreateTestUser(t, "ctx_player", "ctx_player@example.com")
	playerID := int32(player.ID)

	gameSvc := &services.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	game := testDB.CreateTestGameWithState(t, int32(gm.ID), "Context Game", core.GameStateInProgress)
	_, err := gameSvc.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)
	createCharacterFor(t, testDB, game.ID, "Contextual PC", "player_character", &playerID)

	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	rec := getControllable(t, router, playerToken)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp []*ControllableCharacterWithGameResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp, 1)

	got := resp[0]
	assert.Equal(t, game.ID, got.GameID)
	assert.Equal(t, "Context Game", got.GameTitle, "the drawer labels the row with this")
	if assert.NotNil(t, got.GameState) {
		assert.Equal(t, "in_progress", *got.GameState)
	}
	// The role decides whether the sheet renders editable; without it the
	// frontend falls back to read-only.
	assert.Equal(t, "player", got.UserRole)
	if assert.NotNil(t, got.Username) {
		assert.Equal(t, player.Username, *got.Username, "who plays the character, for the GM's cast list")
	}
}

// An unauthenticated caller must never reach the query — this endpoint has no
// game in the path to scope it, so auth is the only boundary.
func TestGetControllableAcrossGames_RequiresAuth(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupStatsTestRouter(app, testDB)

	rec := getControllable(t, router, "")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// A user with no in_progress games gets an empty JSON array, not null. The
// drawer maps over this directly, and null would break the render.
func TestGetControllableAcrossGames_EmptyIsArrayNotNull(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupStatsTestRouter(app, testDB)

	loner := testDB.CreateTestUser(t, "xg_loner", "xg_loner@example.com")
	token, err := core.CreateTestJWTTokenForUser(app, loner)
	require.NoError(t, err)

	rec := getControllable(t, router, token)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `[]`, rec.Body.String())
}
