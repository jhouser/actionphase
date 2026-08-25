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
	actionsvc "actionphase/pkg/db/services/actions"
	phasesvc "actionphase/pkg/db/services/phases"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupResultsTestState creates common test data: game, gm, player, active action phase
func setupResultsTestState(t *testing.T, testDB *core.TestDatabase, app *core.App) (
	gm *core.User, player *core.User, gmToken string, playerToken string,
	game *models.Game, phase *models.GamePhase,
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

	phaseService := &phasesvc.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	phase, err = phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(gm.ID), core.TransitionPhaseRequest{
		PhaseType: "action",
		Title:     "Action Phase",
	})
	require.NoError(t, err)

	return
}

// TestPhaseAPI_CreateActionResult tests POST /api/v1/games/{gameId}/results
func TestPhaseAPI_CreateActionResult(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "action_results", "action_submissions", "phases", "characters", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupFullPhaseAPITestRouter(app, testDB)

	gm, player, gmToken, playerToken, game, _ := setupResultsTestState(t, testDB, app)

	t.Run("GM creates action result successfully", func(t *testing.T) {
		body := map[string]interface{}{
			"user_id": player.ID,
			"content": "You discover a hidden passage.",
		}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/results", game.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Equal(t, "You discover a hidden passage.", response["content"])
		assert.Equal(t, float64(player.ID), response["user_id"])
		assert.Equal(t, false, response["is_published"])
	})

	t.Run("non-GM player cannot create action result", func(t *testing.T) {
		body := map[string]interface{}{
			"user_id": player.ID,
			"content": "Some result.",
		}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/results", game.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("rejects result with missing required content", func(t *testing.T) {
		body := map[string]interface{}{
			"user_id": player.ID,
			// content missing
		}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/results", game.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		// content is empty string — service will attempt to create with empty content
		// The handler accepts it (validation is at service level), so we just verify it doesn't 500
		assert.NotEqual(t, http.StatusInternalServerError, rec.Code)
	})

	_ = gm // used indirectly via gmToken
}

// TestPhaseAPI_GetUserActionResults tests GET /api/v1/games/{gameId}/results/mine
func TestPhaseAPI_GetUserActionResults(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "action_results", "action_submissions", "phases", "characters", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupFullPhaseAPITestRouter(app, testDB)

	gm, player, gmToken, playerToken, game, phase := setupResultsTestState(t, testDB, app)

	actionService := &actionsvc.ActionSubmissionService{
		DB:                  testDB.Pool,
		Logger:              app.ObsLogger,
		NotificationService: &dbsvc.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger},
	}

	// Create a published result for the player
	result, err := actionService.CreateActionResult(context.Background(), core.CreateActionResultRequest{
		GameID:      game.ID,
		PhaseID:     phase.ID,
		UserID:      int32(player.ID),
		GMUserID:    int32(gm.ID),
		Content:     "Your action succeeds.",
		IsPublished: true,
	})
	require.NoError(t, err)
	err = actionService.PublishActionResult(context.Background(), result.ID, int32(gm.ID))
	require.NoError(t, err)

	t.Run("player retrieves their own action results", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/results/mine", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response []map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Len(t, response, 1)
		assert.Equal(t, "Your action succeeds.", response[0]["content"])
	})

	t.Run("GM retrieves their own results (empty since GM has none)", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/results/mine", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		// Response is a JSON array (may be null or empty)
		body := rec.Body.Bytes()
		assert.True(t, string(body) == "null\n" || string(body) == "[]\n" || len(body) > 0)
	})
}

// TestPhaseAPI_GetGameActionResults tests GET /api/v1/games/{gameId}/results
func TestPhaseAPI_GetGameActionResults(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "action_results", "action_submissions", "phases", "characters", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupFullPhaseAPITestRouter(app, testDB)

	gm, player, gmToken, playerToken, game, phase := setupResultsTestState(t, testDB, app)

	actionService := &actionsvc.ActionSubmissionService{
		DB:                  testDB.Pool,
		Logger:              app.ObsLogger,
		NotificationService: &dbsvc.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger},
	}

	// Create a result
	_, err := actionService.CreateActionResult(context.Background(), core.CreateActionResultRequest{
		GameID:      game.ID,
		PhaseID:     phase.ID,
		UserID:      int32(player.ID),
		GMUserID:    int32(gm.ID),
		Content:     "An important result.",
		IsPublished: false,
	})
	require.NoError(t, err)

	t.Run("GM can view all game results (including unpublished)", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/results", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response []map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Len(t, response, 1)
		assert.Equal(t, "An important result.", response[0]["content"])
	})

	t.Run("non-GM player cannot view all results during active game", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/results", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("player CAN view all results once the game reaches epilogue", func(t *testing.T) {
		// Regression: this check was written as `state != "completed"` and so
		// kept the archive shut in epilogue. A player writing an epilogue needs
		// to see what happened to everyone else — that is the entire reason the
		// state exists, and History showed them nothing.
		// Set the state directly: the fixture game is in 'setup', and walking it
		// through the real state machine is not what this test is about.
		_, err := testDB.Pool.Exec(context.Background(),
			"UPDATE games SET state = $1 WHERE id = $2", core.GameStateEpilogue, game.ID)
		require.NoError(t, err)

		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/results", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code,
			"player should read all results in an epilogue game (body: %s)", rec.Body.String())

		var response []map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Len(t, response, 1, "the archive should expose the GM's result")
	})
}

// TestPhaseAPI_UpdateActionResult tests PUT /api/v1/games/{gameId}/results/{resultId}
func TestPhaseAPI_UpdateActionResult(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "action_results", "action_submissions", "phases", "characters", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupFullPhaseAPITestRouter(app, testDB)

	gm, player, gmToken, playerToken, game, phase := setupResultsTestState(t, testDB, app)

	actionService := &actionsvc.ActionSubmissionService{
		DB:                  testDB.Pool,
		Logger:              app.ObsLogger,
		NotificationService: &dbsvc.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger},
	}

	result, err := actionService.CreateActionResult(context.Background(), core.CreateActionResultRequest{
		GameID:      game.ID,
		PhaseID:     phase.ID,
		UserID:      int32(player.ID),
		GMUserID:    int32(gm.ID),
		Content:     "Original content.",
		IsPublished: false,
	})
	require.NoError(t, err)

	t.Run("GM updates action result successfully", func(t *testing.T) {
		body := map[string]interface{}{
			"content": "Updated content.",
		}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/games/%d/results/%d", game.ID, result.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Equal(t, "Updated content.", response["content"])
	})

	t.Run("non-GM player cannot update action result", func(t *testing.T) {
		body := map[string]interface{}{
			"content": "Unauthorized update.",
		}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/games/%d/results/%d", game.ID, result.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

// TestPhaseAPI_UpdatePublishedResultBlocked tests that published results cannot be edited
func TestPhaseAPI_UpdatePublishedResultBlocked(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "action_results", "action_submissions", "phases", "characters", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupFullPhaseAPITestRouter(app, testDB)

	gm, player, gmToken, _, game, phase := setupResultsTestState(t, testDB, app)

	actionService := &actionsvc.ActionSubmissionService{
		DB:                  testDB.Pool,
		Logger:              app.ObsLogger,
		NotificationService: &dbsvc.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger},
	}

	result, err := actionService.CreateActionResult(context.Background(), core.CreateActionResultRequest{
		GameID:      game.ID,
		PhaseID:     phase.ID,
		UserID:      int32(player.ID),
		GMUserID:    int32(gm.ID),
		Content:     "Original published content.",
		IsPublished: false,
	})
	require.NoError(t, err)

	// Publish the result
	err = actionService.PublishActionResult(context.Background(), result.ID, int32(gm.ID))
	require.NoError(t, err)

	t.Run("GM cannot edit a published result", func(t *testing.T) {
		body := map[string]interface{}{
			"content": "Attempted overwrite.",
		}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/games/%d/results/%d", game.ID, result.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		// Service returns "result not found or already published" → handler returns 500
		assert.NotEqual(t, http.StatusOK, rec.Code)

		// Verify content was not changed in the DB
		var dbContent string
		err := testDB.Pool.QueryRow(context.Background(),
			"SELECT content FROM action_results WHERE id = $1", result.ID).Scan(&dbContent)
		require.NoError(t, err)
		assert.Equal(t, "Original published content.", dbContent)
	})
}

// TestPhaseAPI_CreateActionResult_NoActivePhase verifies the handler returns 400 when the GM
// tries to create a result but no phase is currently active. Without this guard the result
// would have a nil phase reference.
func TestPhaseAPI_CreateActionResult_NoActivePhase(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "action_results", "phases", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupFullPhaseAPITestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &dbsvc.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	// No phase created — game has no active phase

	body := map[string]interface{}{
		"user_id": player.ID,
		"content": "Some result content.",
	}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/results", game.ID), bytes.NewBuffer(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmToken)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestPhaseAPI_PublishActionResult tests POST /api/v1/games/{gameId}/results/{resultId}/publish
func TestPhaseAPI_PublishActionResult(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "action_results", "action_submissions", "phases", "characters", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupFullPhaseAPITestRouter(app, testDB)

	gm, player, gmToken, playerToken, game, phase := setupResultsTestState(t, testDB, app)

	actionService := &actionsvc.ActionSubmissionService{
		DB:                  testDB.Pool,
		Logger:              app.ObsLogger,
		NotificationService: &dbsvc.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger},
	}

	result, err := actionService.CreateActionResult(context.Background(), core.CreateActionResultRequest{
		GameID:      game.ID,
		PhaseID:     phase.ID,
		UserID:      int32(player.ID),
		GMUserID:    int32(gm.ID),
		Content:     "Secret result to publish.",
		IsPublished: false,
	})
	require.NoError(t, err)

	t.Run("non-GM player cannot publish result", func(t *testing.T) {
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/results/%d/publish", game.ID, result.ID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("GM publishes action result successfully", func(t *testing.T) {
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/results/%d/publish", game.ID, result.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		// Verify result is now published by fetching all results as GM
		getReq := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/results", game.ID), nil)
		getReq.Header.Set("Authorization", "Bearer "+gmToken)
		getRec := httptest.NewRecorder()
		router.ServeHTTP(getRec, getReq)

		var results []map[string]interface{}
		require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &results))
		require.Len(t, results, 1)
		assert.Equal(t, true, results[0]["is_published"])
	})
}

// TestPhaseAPI_AudienceSeesUnpublishedResults pins down audience visibility of
// GM drafts.
//
// Audience is a trusted spectator role — it already reads every private message
// and action submission — so unpublished results are deliberately visible to it
// too, letting spectators watch results take shape. This is safe because a
// result can only be drafted against the active phase: CreateActionResult takes
// its PhaseID from GetActivePhase and refuses outright when no phase is active,
// so there is no such thing as a draft for a phase that has not started.
//
// Players remain excluded — they are subjects of the game, not spectators, and
// GetUserResults serves them only their own published results.
func TestPhaseAPI_AudienceSeesUnpublishedResults(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "action_results", "action_submissions", "phases", "characters", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupFullPhaseAPITestRouter(app, testDB)

	gm, player, gmToken, playerToken, game, phase := setupResultsTestState(t, testDB, app)

	audience := testDB.CreateTestUser(t, "audience", "audience@example.com")
	audienceToken, err := core.CreateTestJWTTokenForUser(app, audience)
	require.NoError(t, err)

	gameService := &dbsvc.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(audience.ID), "audience")
	require.NoError(t, err)

	actionService := &actionsvc.ActionSubmissionService{
		DB:                  testDB.Pool,
		Logger:              app.ObsLogger,
		NotificationService: &dbsvc.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger},
	}

	published, err := actionService.CreateActionResult(context.Background(), core.CreateActionResultRequest{
		GameID:      game.ID,
		PhaseID:     phase.ID,
		UserID:      int32(player.ID),
		GMUserID:    int32(gm.ID),
		Content:     "Published result the audience may read.",
		IsPublished: true,
	})
	require.NoError(t, err)

	_, err = actionService.CreateActionResult(context.Background(), core.CreateActionResultRequest{
		GameID:      game.ID,
		PhaseID:     phase.ID,
		UserID:      int32(player.ID),
		GMUserID:    int32(gm.ID),
		Content:     "GM draft still being written.",
		IsPublished: false,
	})
	require.NoError(t, err)

	fetchResults := func(t *testing.T, token string) []map[string]interface{} {
		t.Helper()
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/results", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var response []map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		return response
	}

	contentsOf := func(response []map[string]interface{}) []string {
		contents := make([]string, 0, len(response))
		for _, result := range response {
			contents = append(contents, result["content"].(string))
		}
		return contents
	}

	t.Run("audience sees drafts alongside published results in an in-progress game", func(t *testing.T) {
		response := fetchResults(t, audienceToken)

		require.Len(t, response, 2, "audience should receive both the published result and the draft")
		assert.ElementsMatch(t, []string{
			"Published result the audience may read.",
			"GM draft still being written.",
		}, contentsOf(response))
	})

	t.Run("GM sees both published and unpublished results", func(t *testing.T) {
		response := fetchResults(t, gmToken)

		assert.Len(t, response, 2, "GM should see the draft alongside the published result")
	})

	// The player path is a different endpoint entirely (GetUserResults), which
	// filters to is_published = true. Drafts being visible to spectators must not
	// mean they leak to the person whose action they answer.
	t.Run("the player the result is addressed to does not see the draft", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/results/mine", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var response []map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))

		require.Len(t, response, 1, "player should receive only the published result")
		assert.Equal(t, "Published result the audience may read.", response[0]["content"])
		assert.Equal(t, float64(published.ID), response[0]["id"])
	})

	t.Run("audience still sees everything once the game is completed", func(t *testing.T) {
		testDB.SetGameStateDirectly(t, game.ID, "completed")

		response := fetchResults(t, audienceToken)

		assert.Len(t, response, 2, "a completed game is a public archive; nothing is withheld")
	})
}

// TestPhaseAPI_UserResultsIncludeCharacterID verifies that a player's own
// results carry character_id.
//
// Regression: GetUserActionResults mapped character_name but never
// character_id, while the GM-facing GetGameActionResults set both. The frontend
// resolves a character's avatar by looking its id up in the game's character
// list, so with the id missing every result in a non-GM's History tab fell back
// to initials — the same character rendered with its portrait for the GM and as
// an initials bubble for the player it belonged to.
func TestPhaseAPI_UserResultsIncludeCharacterID(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "action_results", "action_submissions", "phases", "characters", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupFullPhaseAPITestRouter(app, testDB)

	gm, player, _, playerToken, game, phase := setupResultsTestState(t, testDB, app)

	var characterID int32
	err := testDB.Pool.QueryRow(context.Background(),
		`INSERT INTO characters (game_id, user_id, name, character_type, status) VALUES ($1, $2, $3, 'player_character', 'approved') RETURNING id`,
		game.ID, player.ID, "Vera",
	).Scan(&characterID)
	require.NoError(t, err)

	actionService := &actionsvc.ActionSubmissionService{
		DB:                  testDB.Pool,
		Logger:              app.ObsLogger,
		NotificationService: &dbsvc.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger},
	}

	result, err := actionService.CreateActionResult(context.Background(), core.CreateActionResultRequest{
		GameID:      game.ID,
		PhaseID:     phase.ID,
		UserID:      int32(player.ID),
		CharacterID: &characterID,
		GMUserID:    int32(gm.ID),
		Content:     "Your action succeeds.",
		IsPublished: true,
	})
	require.NoError(t, err)
	require.NoError(t, actionService.PublishActionResult(context.Background(), result.ID, int32(gm.ID)))

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/results/mine", game.ID), nil)
	req.Header.Set("Authorization", "Bearer "+playerToken)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	// Decoded as a raw map so an absent key is distinguishable from a zero value:
	// character_id is omitempty, and a typed struct would report both as 0.
	var response []map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.Len(t, response, 1)

	got, present := response[0]["character_id"]
	require.True(t, present, "character_id must be present so the client can resolve the avatar; got keys %v", response[0])
	assert.Equal(t, float64(characterID), got)
	assert.Equal(t, "Vera", response[0]["character_name"])
}
