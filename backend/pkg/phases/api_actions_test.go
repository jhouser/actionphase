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
	models "actionphase/pkg/db/models"
	dbsvc "actionphase/pkg/db/services"
	phasesvc "actionphase/pkg/db/services/phases"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupActionsTestState creates gm, player, game, and an active action phase.
func setupActionsTestState(t *testing.T, testDB *core.TestDatabase, app *core.App) (
	gm *core.User, player *core.User, gmToken string, playerToken string,
	game *models.Game, phase *models.GamePhase, playerCharID int32,
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

	gameService := &dbsvc.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	// Create a character for the player
	err = testDB.Pool.QueryRow(context.Background(),
		`INSERT INTO characters (game_id, user_id, name, character_type, status) VALUES ($1, $2, $3, 'player_character', 'approved') RETURNING id`,
		game.ID, int32(player.ID), "Test Character",
	).Scan(&playerCharID)
	require.NoError(t, err)

	// Activate an action phase
	phaseService := &phasesvc.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	phase, err = phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(gm.ID), core.TransitionPhaseRequest{
		PhaseType: "action",
		Title:     "Action Phase",
	})
	require.NoError(t, err)

	return
}

// TestPhaseAPI_SubmitAction tests POST /api/v1/games/{gameId}/actions
func TestPhaseAPI_SubmitAction(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "action_results", "action_submissions", "game_phases", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupFullPhaseAPITestRouter(app, testDB)

	_, player, gmToken, playerToken, game, _, playerCharID := setupActionsTestState(t, testDB, app)

	t.Run("player submits a final action", func(t *testing.T) {
		body := SubmitActionRequest{
			CharacterID: int32Ptr(playerCharID),
			Content:     "I carefully investigate the ancient ruins.",
		}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/actions", game.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Equal(t, "I carefully investigate the ancient ruins.", response["content"])
		assert.Equal(t, float64(player.ID), response["user_id"])
	})

	t.Run("GM cannot submit player actions", func(t *testing.T) {
		body := SubmitActionRequest{
			Content: "GM submitting action.",
		}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/actions", game.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

// TestPhaseAPI_SubmitAction_NoActivePhase verifies that submitting an action when no phase
// is active returns 400 rather than silently creating an orphaned action.
func TestPhaseAPI_SubmitAction_NoActivePhase(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "action_submissions", "game_phases", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupFullPhaseAPITestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &dbsvc.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	// No phase created — game has no active phase

	body := SubmitActionRequest{Content: "I look around"}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/actions", game.ID), bytes.NewBuffer(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+playerToken)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Should be 400, not 201 — no active phase means the action cannot be associated with anything
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestPhaseAPI_GetUserActions tests GET /api/v1/games/{gameId}/actions/mine
func TestPhaseAPI_GetUserActions(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "action_results", "action_submissions", "game_phases", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupFullPhaseAPITestRouter(app, testDB)

	_, _, gmToken, playerToken, game, _, playerCharID := setupActionsTestState(t, testDB, app)

	t.Run("returns empty list when no actions submitted", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/actions/mine", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("player sees their own submitted action", func(t *testing.T) {
		// Submit an action first
		body := SubmitActionRequest{
			CharacterID: int32Ptr(playerCharID),
			Content:     "My action for this phase.",
		}
		bodyJSON, _ := json.Marshal(body)
		submitReq := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/actions", game.ID), bytes.NewBuffer(bodyJSON))
		submitReq.Header.Set("Content-Type", "application/json")
		submitReq.Header.Set("Authorization", "Bearer "+playerToken)
		submitRec := httptest.NewRecorder()
		router.ServeHTTP(submitRec, submitReq)
		require.Equal(t, http.StatusCreated, submitRec.Code)

		// Now retrieve it
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/actions/mine", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response []map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		require.Len(t, response, 1)
		assert.Equal(t, "My action for this phase.", response[0]["content"])
	})

	t.Run("GM sees empty list from their own mine endpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/actions/mine", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

// TestPhaseAPI_GetGameActions tests GET /api/v1/games/{gameId}/actions
func TestPhaseAPI_GetGameActions(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "action_results", "action_submissions", "game_phases", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupFullPhaseAPITestRouter(app, testDB)

	_, _, gmToken, playerToken, game, _, playerCharID := setupActionsTestState(t, testDB, app)

	// Submit an action as player
	body := SubmitActionRequest{
		CharacterID: int32Ptr(playerCharID),
		Content:     "Player action for GM to see.",
	}
	bodyJSON, _ := json.Marshal(body)
	submitReq := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/actions", game.ID), bytes.NewBuffer(bodyJSON))
	submitReq.Header.Set("Content-Type", "application/json")
	submitReq.Header.Set("Authorization", "Bearer "+playerToken)
	submitRec := httptest.NewRecorder()
	router.ServeHTTP(submitRec, submitReq)
	require.Equal(t, http.StatusCreated, submitRec.Code)

	t.Run("GM can view all game actions", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/actions", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response []map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Len(t, response, 1)
		assert.Equal(t, "Player action for GM to see.", response[0]["content"])
	})

	t.Run("non-GM player cannot view all game actions", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/actions", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

// TestPhaseAPI_GetCurrentPhase tests GET /api/v1/games/{gameId}/current-phase
func TestPhaseAPI_GetCurrentPhase(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "action_results", "action_submissions", "game_phases", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupFullPhaseAPITestRouter(app, testDB)

	_, _, gmToken, playerToken, game, activePhase, _ := setupActionsTestState(t, testDB, app)

	t.Run("player can get the current active phase", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/current-phase", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		phase := response["phase"].(map[string]interface{})
		assert.Equal(t, float64(activePhase.ID), phase["id"])
		assert.Equal(t, "action", phase["phase_type"])
		assert.Equal(t, true, phase["is_active"])
	})

	t.Run("GM can get the current active phase", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/current-phase", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.NotNil(t, response["phase"])
	})

	t.Run("returns null phase when no active phase", func(t *testing.T) {
		otherGM := testDB.CreateTestUser(t, "othergm", "othergm@example.com")
		otherGMToken, err := core.CreateTestJWTTokenForUser(app, otherGM)
		require.NoError(t, err)
		emptyGame := testDB.CreateTestGame(t, int32(otherGM.ID), "No Phase Game")

		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/current-phase", emptyGame.ID), nil)
		req.Header.Set("Authorization", "Bearer "+otherGMToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Nil(t, response["phase"])
	})
}

// TestPhaseAPI_InterludeBlocksActionSubmission verifies that action submissions are rejected
// during interlude phases (interlude allows PMs only, not actions).
func TestPhaseAPI_InterludeBlocksActionSubmission(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "action_results", "action_submissions", "game_phases", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupFullPhaseAPITestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &dbsvc.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	var playerCharID int32
	err = testDB.Pool.QueryRow(context.Background(),
		`INSERT INTO characters (game_id, user_id, name, character_type, status) VALUES ($1, $2, $3, 'player_character', 'approved') RETURNING id`,
		game.ID, int32(player.ID), "Test Character",
	).Scan(&playerCharID)
	require.NoError(t, err)

	phaseService := &phasesvc.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(gm.ID), core.TransitionPhaseRequest{
		PhaseType: core.PhaseTypeInterlude,
		Title:     "Evening Interlude",
	})
	require.NoError(t, err)

	t.Run("player cannot submit action during interlude phase", func(t *testing.T) {
		body := SubmitActionRequest{
			CharacterID: int32Ptr(playerCharID),
			Content:     "I attempt to investigate the ruins.",
		}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/actions", game.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}
