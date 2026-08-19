package phases

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"actionphase/pkg/core"
	models2 "actionphase/pkg/db/models"
	dbsvc2 "actionphase/pkg/db/services"
	actionsvc2 "actionphase/pkg/db/services/actions"
	phasesvc2 "actionphase/pkg/db/services/phases"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupDraftUpdatesTestState creates common test data for draft update tests including a character and action result
func setupDraftUpdatesTestState(t *testing.T, testDB *core.TestDatabase, app *core.App) (
	gm *core.User, player *core.User, gmToken string, playerToken string,
	game *models2.Game, phase *models2.GamePhase, result *models2.ActionResult, character *models2.Character,
) {
	t.Helper()

	gm = testDB.CreateTestUser(t, "gm", "gm@example.com")
	player = testDB.CreateTestUser(t, "player", "player@example.com")

	var err error
	gmToken, err = core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err = core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	game = testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &dbsvc2.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	phaseService := &phasesvc2.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	phase, err = phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(gm.ID), core.TransitionPhaseRequest{
		PhaseType: "action",
		Title:     "Action Phase",
	})
	require.NoError(t, err)

	// Create an approved character for the player in this game
	var charID int32
	err = testDB.Pool.QueryRow(context.Background(),
		`INSERT INTO characters (game_id, user_id, name, character_type, status) VALUES ($1, $2, $3, 'player_character', 'approved') RETURNING id`,
		game.ID, int32(player.ID), "Test Character",
	).Scan(&charID)
	require.NoError(t, err)
	character = &models2.Character{ID: charID, GameID: game.ID, UserID: pgtype.Int4{Int32: int32(player.ID), Valid: true}}

	actionService := &actionsvc2.ActionSubmissionService{
		DB:                  testDB.Pool,
		Logger:              app.ObsLogger,
		NotificationService: &dbsvc2.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger},
	}
	result, err = actionService.CreateActionResult(context.Background(), core.CreateActionResultRequest{
		GameID:      game.ID,
		PhaseID:     phase.ID,
		UserID:      int32(player.ID),
		GMUserID:    int32(gm.ID),
		Content:     "Result content.",
		IsPublished: false,
	})
	require.NoError(t, err)

	return
}

// TestPhaseAPI_CreateDraftCharacterUpdate tests POST /api/v1/games/{gameId}/results/{resultId}/character-updates
func TestPhaseAPI_CreateDraftCharacterUpdate(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "draft_character_updates", "action_results", "action_submissions", "phases", "characters", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupFullPhaseAPITestRouter(app, testDB)

	gm, _, gmToken, playerToken, game, _, result, character := setupDraftUpdatesTestState(t, testDB, app)

	_ = gm

	t.Run("GM creates draft character update successfully", func(t *testing.T) {
		body := map[string]interface{}{
			"character_id": character.ID,
			"module_type":  "skills",
			"field_name":   "strength",
			"field_value":  "18",
			"field_type":   "number",
			"operation":    "upsert",
		}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/results/%d/character-updates", game.ID, result.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Equal(t, "skills", response["module_type"])
		assert.Equal(t, "strength", response["field_name"])
		assert.Equal(t, "18", response["field_value"])
	})

	t.Run("non-GM player cannot create draft update", func(t *testing.T) {
		body := map[string]interface{}{
			"character_id": character.ID,
			"module_type":  "skills",
			"field_name":   "strength",
			"field_value":  "18",
			"field_type":   "number",
			"operation":    "upsert",
		}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/results/%d/character-updates", game.ID, result.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("rejects character that does not belong to this game", func(t *testing.T) {
		body := map[string]interface{}{
			"character_id": int32(99999), // non-existent character
			"module_type":  "skills",
			"field_name":   "strength",
			"field_value":  "18",
			"field_type":   "number",
			"operation":    "upsert",
		}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/results/%d/character-updates", game.ID, result.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

// TestPhaseAPI_GetDraftCharacterUpdates tests GET /api/v1/games/{gameId}/results/{resultId}/character-updates
func TestPhaseAPI_GetDraftCharacterUpdates(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "draft_character_updates", "action_results", "action_submissions", "phases", "characters", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupFullPhaseAPITestRouter(app, testDB)

	gm, _, gmToken, playerToken, game, _, result, character := setupDraftUpdatesTestState(t, testDB, app)
	_ = gm

	// Create a draft update via the service directly
	actionService := &actionsvc2.ActionSubmissionService{
		DB:                  testDB.Pool,
		Logger:              app.ObsLogger,
		NotificationService: &dbsvc2.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger},
	}
	_, err := actionService.CreateDraftCharacterUpdate(context.Background(), core.CreateDraftCharacterUpdateRequest{
		ActionResultID: result.ID,
		CharacterID:    character.ID,
		ModuleType:     "skills",
		FieldName:      "persuasion",
		FieldValue:     "5",
		FieldType:      "number",
		Operation:      "upsert",
	})
	require.NoError(t, err)

	t.Run("GM retrieves draft character updates", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/results/%d/character-updates", game.ID, result.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response []map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Len(t, response, 1)
		assert.Equal(t, "persuasion", response[0]["field_name"])
	})

	t.Run("non-GM player cannot retrieve draft updates", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/results/%d/character-updates", game.ID, result.ID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

// TestPhaseAPI_GetDraftUpdateCount tests GET /api/v1/games/{gameId}/results/{resultId}/character-updates/count
func TestPhaseAPI_GetDraftUpdateCount(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "draft_character_updates", "action_results", "action_submissions", "phases", "characters", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupFullPhaseAPITestRouter(app, testDB)

	gm, _, gmToken, playerToken, game, _, result, character := setupDraftUpdatesTestState(t, testDB, app)
	_ = gm

	t.Run("returns count of 0 when no drafts", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/results/%d/character-updates/count", game.ID, result.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Equal(t, float64(0), response["count"])
	})

	t.Run("returns correct count after creating drafts", func(t *testing.T) {
		actionService := &actionsvc2.ActionSubmissionService{
			DB:                  testDB.Pool,
			Logger:              app.ObsLogger,
			NotificationService: &dbsvc2.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger},
		}
		for i := 0; i < 3; i++ {
			_, err := actionService.CreateDraftCharacterUpdate(context.Background(), core.CreateDraftCharacterUpdateRequest{
				ActionResultID: result.ID,
				CharacterID:    character.ID,
				ModuleType:     "skills",
				FieldName:      fmt.Sprintf("stat_%d", i),
				FieldValue:     "10",
				FieldType:      "number",
				Operation:      "upsert",
			})
			require.NoError(t, err)
		}

		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/results/%d/character-updates/count", game.ID, result.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Equal(t, float64(3), response["count"])
	})

	t.Run("non-GM player cannot get draft count", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/results/%d/character-updates/count", game.ID, result.ID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

// TestPhaseAPI_UpdateDraftCharacterUpdate tests PUT /api/v1/games/{gameId}/results/{resultId}/character-updates/{draftId}
func TestPhaseAPI_UpdateDraftCharacterUpdate(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "draft_character_updates", "action_results", "action_submissions", "phases", "characters", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupFullPhaseAPITestRouter(app, testDB)

	gm, _, gmToken, playerToken, game, _, result, character := setupDraftUpdatesTestState(t, testDB, app)
	_ = gm

	actionService := &actionsvc2.ActionSubmissionService{
		DB:                  testDB.Pool,
		Logger:              app.ObsLogger,
		NotificationService: &dbsvc2.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger},
	}
	draft, err := actionService.CreateDraftCharacterUpdate(context.Background(), core.CreateDraftCharacterUpdateRequest{
		ActionResultID: result.ID,
		CharacterID:    character.ID,
		ModuleType:     "skills",
		FieldName:      "strength",
		FieldValue:     "10",
		FieldType:      "number",
		Operation:      "upsert",
	})
	require.NoError(t, err)

	t.Run("GM updates draft field value", func(t *testing.T) {
		body := map[string]interface{}{
			"field_value": "20",
		}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/games/%d/results/%d/character-updates/%d", game.ID, result.ID, draft.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Equal(t, "20", response["field_value"])
	})

	t.Run("non-GM player cannot update draft", func(t *testing.T) {
		body := map[string]interface{}{
			"field_value": "99",
		}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/games/%d/results/%d/character-updates/%d", game.ID, result.ID, draft.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

// TestPhaseAPI_DeleteDraftCharacterUpdate tests DELETE /api/v1/games/{gameId}/results/{resultId}/character-updates/{draftId}
// TestPhaseAPI_DraftCharacterUpdate_CrossGameMismatch verifies the game-ownership check inside
// validateGMAccessAndResult: a GM cannot access a result from another game through their own
// game ID in the URL. Without this guard, cross-game data leakage is possible.
func TestPhaseAPI_DraftCharacterUpdate_CrossGameMismatch(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "draft_character_updates", "action_results", "action_submissions", "phases", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupFullPhaseAPITestRouter(app, testDB)

	// Game 1 with its own GM and result
	gm1 := testDB.CreateTestUser(t, "gm1", "gm1@example.com")
	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	gm1Token, err := core.CreateTestJWTTokenForUser(app, gm1)
	require.NoError(t, err)
	game1 := testDB.CreateTestGame(t, int32(gm1.ID), "Game One")

	gameService := &dbsvc2.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game1.ID, int32(player1.ID), "player")
	require.NoError(t, err)

	phaseService := &phasesvc2.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	phase1, err := phaseService.TransitionToNextPhase(context.Background(), game1.ID, int32(gm1.ID), core.TransitionPhaseRequest{
		PhaseType: "action",
		Title:     "Action Phase",
	})
	require.NoError(t, err)

	actionService := &actionsvc2.ActionSubmissionService{
		DB:                  testDB.Pool,
		Logger:              app.ObsLogger,
		NotificationService: &dbsvc2.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger},
	}
	result1, err := actionService.CreateActionResult(context.Background(), core.CreateActionResultRequest{
		GameID:      game1.ID,
		PhaseID:     phase1.ID,
		UserID:      int32(player1.ID),
		GMUserID:    int32(gm1.ID),
		Content:     "Game 1 result.",
		IsPublished: false,
	})
	require.NoError(t, err)

	// Game 2 — gm1 is also GM here so they pass the GM check, but result1 belongs to game1
	game2 := testDB.CreateTestGame(t, int32(gm1.ID), "Game Two")

	// GM tries to access game1's result through game2's URL — should get 400 (game mismatch)
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/results/%d/character-updates", game2.ID, result1.ID), nil)
	req.Header.Set("Authorization", "Bearer "+gm1Token)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestPhaseAPI_DeleteDraftCharacterUpdate(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "draft_character_updates", "action_results", "action_submissions", "phases", "characters", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupFullPhaseAPITestRouter(app, testDB)

	gm, _, gmToken, playerToken, game, _, result, character := setupDraftUpdatesTestState(t, testDB, app)
	_ = gm

	actionService := &actionsvc2.ActionSubmissionService{
		DB:                  testDB.Pool,
		Logger:              app.ObsLogger,
		NotificationService: &dbsvc2.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger},
	}

	t.Run("non-GM player cannot delete draft", func(t *testing.T) {
		draft, err := actionService.CreateDraftCharacterUpdate(context.Background(), core.CreateDraftCharacterUpdateRequest{
			ActionResultID: result.ID,
			CharacterID:    character.ID,
			ModuleType:     "skills",
			FieldName:      "dexterity",
			FieldValue:     "12",
			FieldType:      "number",
			Operation:      "upsert",
		})
		require.NoError(t, err)

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/games/%d/results/%d/character-updates/%d", game.ID, result.ID, draft.ID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("GM deletes draft character update successfully", func(t *testing.T) {
		draft, err := actionService.CreateDraftCharacterUpdate(context.Background(), core.CreateDraftCharacterUpdateRequest{
			ActionResultID: result.ID,
			CharacterID:    character.ID,
			ModuleType:     "skills",
			FieldName:      "constitution",
			FieldValue:     "14",
			FieldType:      "number",
			Operation:      "upsert",
		})
		require.NoError(t, err)

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/games/%d/results/%d/character-updates/%d", game.ID, result.ID, draft.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)

		// Verify the deleted draft is gone by checking we can fetch the list and it doesn't contain this draft ID
		getReq := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/results/%d/character-updates", game.ID, result.ID), nil)
		getReq.Header.Set("Authorization", "Bearer "+gmToken)
		getRec := httptest.NewRecorder()
		router.ServeHTTP(getRec, getReq)

		assert.Equal(t, http.StatusOK, getRec.Code)
		var drafts []map[string]interface{}
		require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &drafts))
		for _, d := range drafts {
			assert.NotEqual(t, float64(draft.ID), d["id"], "deleted draft should not appear in list")
		}
	})
}
