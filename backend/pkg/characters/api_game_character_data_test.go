package characters

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"actionphase/pkg/core"
	services "actionphase/pkg/db/services"
)

// The batch character-data endpoint exists so a view showing many characters'
// sheet references (History, Actions) can resolve them all in one request
// instead of one per character. The permission rule is NOT re-implemented for
// the batch: it applies canViewPrivateCharacterData per character, the same
// helper the single-character endpoint uses, so the two cannot disagree.
//
// The invariant these tests exist to protect: in a live game, a player must
// never receive another character's private sheet fields. Skills, inventory and
// numbers are always stored private (CharacterSheet.saveJsonField writes
// is_public=false), so "private fields" is nearly the whole sheet.

// seedSheet gives a character one public bio row and one private skills row,
// mirroring how the real sheet writes them.
func seedSheet(t *testing.T, testDB *core.TestDatabase, characterID int32, skillName string) {
	t.Helper()
	ctx := context.Background()

	_, err := testDB.Pool.Exec(ctx, `
		INSERT INTO character_data (character_id, module_type, field_name, field_value, field_type, is_public)
		VALUES ($1, 'bio', 'background', 'A public backstory.', 'text', true)
	`, characterID)
	core.AssertNoError(t, err, "Seeding bio row should succeed")

	_, err = testDB.Pool.Exec(ctx, `
		INSERT INTO character_data (character_id, module_type, field_name, field_value, field_type, is_public)
		VALUES ($1, 'skills', 'skills', $2, 'json', false)
	`, characterID, fmt.Sprintf(`[{"id":"s-1","name":%q,"description":"secret"}]`, skillName))
	core.AssertNoError(t, err, "Seeding skills row should succeed")
}

// modulesFor returns the module types returned for one character, so a test can
// assert on what was disclosed without depending on row ordering.
func modulesFor(body map[string][]CharacterDataResponse, characterID int32) []string {
	var mods []string
	for _, row := range body[fmt.Sprintf("%d", characterID)] {
		mods = append(mods, row.ModuleType)
	}
	return mods
}

func contains(haystack []string, needle string) bool {
	return slices.Contains(haystack, needle)
}

func getGameCharacterData(t *testing.T, router http.Handler, gameID int32, token string) map[string][]CharacterDataResponse {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/games/%d/characters/data", gameID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusOK, w.Code, "Should return 200")

	var resp map[string][]CharacterDataResponse
	core.AssertNoError(t, json.Unmarshal(w.Body.Bytes(), &resp), "Response should be valid JSON")
	return resp
}

// TestGetGameCharacterData_LiveGamePlayerCannotSeeOthersPrivateFields is the
// security test. A player in an in-progress game gets their own sheet in full
// and only the public rows of everyone else's.
func TestGetGameCharacterData_LiveGamePlayerCannotSeeOthersPrivateFields(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "character_data", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameStatsTestRouter(app, testDB)

	gmUser := testDB.CreateTestUser(t, "gm_gcd1", "gm_gcd1@example.com")
	player1 := testDB.CreateTestUser(t, "p1_gcd1", "p1_gcd1@example.com")
	player2 := testDB.CreateTestUser(t, "p2_gcd1", "p2_gcd1@example.com")

	game := testDB.CreateTestGame(t, int32(gmUser.ID), "Live Game")

	gameService := &services.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player1.ID), "player")
	core.AssertNoError(t, err, "Adding player1 should succeed")
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player2.ID), "player")
	core.AssertNoError(t, err, "Adding player2 should succeed")

	charService := &services.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	p1ID := int32(player1.ID)
	mine, err := charService.CreateCharacter(context.Background(), services.CreateCharacterRequest{
		GameID: game.ID, UserID: &p1ID, Name: "Mine", CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Creating player1's character should succeed")

	p2ID := int32(player2.ID)
	theirs, err := charService.CreateCharacter(context.Background(), services.CreateCharacterRequest{
		GameID: game.ID, UserID: &p2ID, Name: "Theirs", CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Creating player2's character should succeed")

	seedSheet(t, testDB, mine.ID, "My Secret Skill")
	seedSheet(t, testDB, theirs.ID, "Their Secret Skill")

	token, err := core.CreateTestJWTTokenForUser(app, player1)
	core.AssertNoError(t, err, "Token creation should succeed")

	resp := getGameCharacterData(t, router, game.ID, token)

	// Own character: the full sheet, private rows included.
	ownModules := modulesFor(resp, mine.ID)
	if !contains(ownModules, "skills") {
		t.Errorf("Player should see their OWN private skills row, got modules %v", ownModules)
	}
	if !contains(ownModules, "bio") {
		t.Errorf("Player should see their own public bio row, got modules %v", ownModules)
	}

	// Another player's character: public rows only. This is the leak the
	// endpoint must never allow.
	otherModules := modulesFor(resp, theirs.ID)
	if contains(otherModules, "skills") {
		t.Errorf("SECURITY: player received another character's PRIVATE skills row in a live game, modules %v", otherModules)
	}
	if !contains(otherModules, "bio") {
		t.Errorf("Player should still see another character's public bio row, got modules %v", otherModules)
	}

	// Belt and braces: the secret value must not appear anywhere in the payload.
	raw, err := json.Marshal(resp)
	core.AssertNoError(t, err, "Re-marshalling response should succeed")
	if strings.Contains(string(raw), "Their Secret Skill") {
		t.Error("SECURITY: another character's private skill leaked into the response body")
	}
}

// TestGetGameCharacterData_GMSeesWholeCast: the GM authors these sheets, so the
// batch must return every character's private rows.
func TestGetGameCharacterData_GMSeesWholeCast(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "character_data", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameStatsTestRouter(app, testDB)

	gmUser := testDB.CreateTestUser(t, "gm_gcd2", "gm_gcd2@example.com")
	player := testDB.CreateTestUser(t, "p1_gcd2", "p1_gcd2@example.com")
	game := testDB.CreateTestGame(t, int32(gmUser.ID), "GM View Game")

	gameService := &services.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	core.AssertNoError(t, err, "Adding player should succeed")

	charService := &services.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	pID := int32(player.ID)
	char, err := charService.CreateCharacter(context.Background(), services.CreateCharacterRequest{
		GameID: game.ID, UserID: &pID, Name: "Player Char", CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Creating character should succeed")
	seedSheet(t, testDB, char.ID, "Player Skill")

	token, err := core.CreateTestJWTTokenForUser(app, gmUser)
	core.AssertNoError(t, err, "Token creation should succeed")

	resp := getGameCharacterData(t, router, game.ID, token)

	mods := modulesFor(resp, char.ID)
	if !contains(mods, "skills") {
		t.Errorf("GM should see a player's private skills row, got modules %v", mods)
	}
}

// TestGetGameCharacterData_AudienceSeesWholeCast: audience members are trusted
// with the game's secrets so they can spectate meaningfully.
func TestGetGameCharacterData_AudienceSeesWholeCast(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "character_data", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameStatsTestRouter(app, testDB)

	gmUser := testDB.CreateTestUser(t, "gm_gcd3", "gm_gcd3@example.com")
	player := testDB.CreateTestUser(t, "p1_gcd3", "p1_gcd3@example.com")
	spectator := testDB.CreateTestUser(t, "aud_gcd3", "aud_gcd3@example.com")
	game := testDB.CreateTestGame(t, int32(gmUser.ID), "Audience View Game")

	gameService := &services.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	core.AssertNoError(t, err, "Adding player should succeed")
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(spectator.ID), "audience")
	core.AssertNoError(t, err, "Adding audience member should succeed")

	charService := &services.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	pID := int32(player.ID)
	char, err := charService.CreateCharacter(context.Background(), services.CreateCharacterRequest{
		GameID: game.ID, UserID: &pID, Name: "Watched Char", CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Creating character should succeed")
	seedSheet(t, testDB, char.ID, "Watched Skill")

	token, err := core.CreateTestJWTTokenForUser(app, spectator)
	core.AssertNoError(t, err, "Token creation should succeed")

	resp := getGameCharacterData(t, router, game.ID, token)

	mods := modulesFor(resp, char.ID)
	if !contains(mods, "skills") {
		t.Errorf("Audience member should see private skills rows, got modules %v", mods)
	}
}

// TestGetGameCharacterData_PublicArchiveOpensWholeCast: once a game completes
// there is nothing left to protect, so a player sees everyone's sheets. This is
// the same rule that lets History show the whole cast's submissions in an
// archive.
func TestGetGameCharacterData_PublicArchiveOpensWholeCast(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "character_data", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameStatsTestRouter(app, testDB)

	gmUser := testDB.CreateTestUser(t, "gm_gcd4", "gm_gcd4@example.com")
	player1 := testDB.CreateTestUser(t, "p1_gcd4", "p1_gcd4@example.com")
	player2 := testDB.CreateTestUser(t, "p2_gcd4", "p2_gcd4@example.com")

	// Built live and then completed: an archived game is read-only, so
	// participants and characters cannot be added once it is in that state.
	game := testDB.CreateTestGame(t, int32(gmUser.ID), "Completed Game")

	gameService := &services.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player1.ID), "player")
	core.AssertNoError(t, err, "Adding player1 should succeed")
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player2.ID), "player")
	core.AssertNoError(t, err, "Adding player2 should succeed")

	charService := &services.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	p2ID := int32(player2.ID)
	theirs, err := charService.CreateCharacter(context.Background(), services.CreateCharacterRequest{
		GameID: game.ID, UserID: &p2ID, Name: "Their Archived Char", CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Creating character should succeed")
	seedSheet(t, testDB, theirs.ID, "Archived Skill")

	for _, state := range []string{
		core.GameStateRecruitment,
		core.GameStateCharacterCreation,
		core.GameStateInProgress,
		core.GameStateCompleted,
	} {
		_, err = gameService.UpdateGameState(context.Background(), game.ID, state)
		core.AssertNoError(t, err, "Transitioning game state should succeed")
	}

	token, err := core.CreateTestJWTTokenForUser(app, player1)
	core.AssertNoError(t, err, "Token creation should succeed")

	resp := getGameCharacterData(t, router, game.ID, token)

	mods := modulesFor(resp, theirs.ID)
	if !contains(mods, "skills") {
		t.Errorf("In a completed game a player should see another character's private rows, got modules %v", mods)
	}
}

// TestGetGameCharacterData_NonParticipantForbidden: the game read gate runs
// before any sheet is considered.
func TestGetGameCharacterData_NonParticipantForbidden(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "character_data", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameStatsTestRouter(app, testDB)

	gmUser := testDB.CreateTestUser(t, "gm_gcd5", "gm_gcd5@example.com")
	outsider := testDB.CreateTestUser(t, "out_gcd5", "out_gcd5@example.com")
	game := testDB.CreateTestGame(t, int32(gmUser.ID), "Private Game")

	charService := &services.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	gmID := int32(gmUser.ID)
	char, err := charService.CreateCharacter(context.Background(), services.CreateCharacterRequest{
		GameID: game.ID, UserID: &gmID, Name: "Hidden Char", CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Creating character should succeed")
	seedSheet(t, testDB, char.ID, "Hidden Skill")

	token, err := core.CreateTestJWTTokenForUser(app, outsider)
	core.AssertNoError(t, err, "Token creation should succeed")

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/games/%d/characters/data", game.ID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Errorf("SECURITY: a non-participant read a private game's character data (status %d, body %s)", w.Code, w.Body.String())
	}
}
