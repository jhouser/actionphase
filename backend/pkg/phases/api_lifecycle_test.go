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
	db "actionphase/pkg/db/services"
	actionsvc "actionphase/pkg/db/services/actions"
	dbactions "actionphase/pkg/db/services/actions"
	dbmessages "actionphase/pkg/db/services/messages"
	phasesvc "actionphase/pkg/db/services/phases"
	"actionphase/pkg/games"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPhaseAPI_PublishAllPhaseResults tests POST /api/v1/games/{gameId}/phases/{phaseId}/results/publish
func TestPhaseAPI_PublishAllPhaseResults(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "action_results", "action_submissions", "phases", "characters", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupFullPhaseAPITestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	phaseService := &phasesvc.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	phase, err := phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(gm.ID), core.TransitionPhaseRequest{
		PhaseType: "action",
		Title:     "Action Phase",
	})
	require.NoError(t, err)

	// Create an unpublished action result
	actionService := &actionsvc.ActionSubmissionService{
		DB:                  testDB.Pool,
		Logger:              app.ObsLogger,
		NotificationService: &db.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger},
	}
	_, err = actionService.CreateActionResult(context.Background(), core.CreateActionResultRequest{
		GameID:      game.ID,
		PhaseID:     phase.ID,
		UserID:      int32(player.ID),
		GMUserID:    int32(gm.ID),
		Content:     "You find a clue.",
		IsPublished: false,
	})
	require.NoError(t, err)

	t.Run("GM publishes all phase results successfully", func(t *testing.T) {
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/phases/%d/results/publish", game.ID, phase.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Contains(t, response["message"], "published")
	})

	t.Run("non-GM player cannot publish all phase results", func(t *testing.T) {
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/phases/%d/results/publish", game.ID, phase.ID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("returns 400 for invalid game ID", func(t *testing.T) {
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/abc/phases/%d/results/publish", phase.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

// TestPhaseAPI_GetUnpublishedResultsCount tests GET /api/v1/games/{gameId}/phases/{phaseId}/results/unpublished-count
func TestPhaseAPI_GetUnpublishedResultsCount(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "action_results", "action_submissions", "phases", "characters", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupFullPhaseAPITestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	phaseService := &phasesvc.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	phase, err := phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(gm.ID), core.TransitionPhaseRequest{
		PhaseType: "action",
		Title:     "Action Phase",
	})
	require.NoError(t, err)

	actionService := &actionsvc.ActionSubmissionService{
		DB:                  testDB.Pool,
		Logger:              app.ObsLogger,
		NotificationService: &db.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger},
	}

	t.Run("returns count of 0 when no results", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/phases/%d/results/unpublished-count", game.ID, phase.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Equal(t, float64(0), response["count"])
	})

	t.Run("returns correct count of unpublished results", func(t *testing.T) {
		// Create 2 unpublished results
		for i := 0; i < 2; i++ {
			_, err = actionService.CreateActionResult(context.Background(), core.CreateActionResultRequest{
				GameID:      game.ID,
				PhaseID:     phase.ID,
				UserID:      int32(player.ID),
				GMUserID:    int32(gm.ID),
				Content:     fmt.Sprintf("Result %d", i),
				IsPublished: false,
			})
			require.NoError(t, err)
		}

		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/phases/%d/results/unpublished-count", game.ID, phase.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Equal(t, float64(2), response["count"])
	})

	t.Run("non-GM player cannot get unpublished count", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/phases/%d/results/unpublished-count", game.ID, phase.ID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

// setupFullPhaseAPITestRouter creates a router with all phase handler routes including lifecycle and result routes
func setupFullPhaseAPITestRouter(app *core.App, testDB *core.TestDatabase) *chi.Mux {
	tokenAuth := jwtauth.New("HS256", []byte(app.Config.JWT.Secret), nil)
	userService := &db.UserService{DB: testDB.Pool, Logger: app.ObsLogger}

	gameHandler := games.Handler{
		App:                     app,
		UserService:             &db.UserService{DB: app.Pool, Logger: app.ObsLogger},
		GameService:             &db.GameService{DB: app.Pool, Logger: app.ObsLogger},
		GameApplicationService:  &db.GameApplicationService{DB: app.Pool, Logger: app.ObsLogger},
		CharacterService:        &db.CharacterService{DB: app.Pool, Logger: app.ObsLogger},
		NotificationService:     db.NewNotificationService(app.Pool, app.ObsLogger),
		MessageService:          &dbmessages.MessageService{DB: app.Pool, Logger: app.ObsLogger, Metrics: app.Observability.OTELMetrics},
		ActionSubmissionService: &dbactions.ActionSubmissionService{DB: app.Pool, Logger: app.ObsLogger, NotificationService: db.NewNotificationService(app.Pool, app.ObsLogger)},
	}
	r := chi.NewRouter()

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/games/{gameID}", func(r chi.Router) {
			phaseHandler := Handler{
				App:                     app,
				PhaseService:            &phasesvc.PhaseService{DB: testDB.Pool},
				ActionSubmissionService: &actionsvc.ActionSubmissionService{DB: testDB.Pool, Logger: app.ObsLogger, NotificationService: db.NewNotificationService(testDB.Pool, app.ObsLogger)},
				GameService:             &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger},
				NotificationService:     db.NewNotificationService(testDB.Pool, app.ObsLogger),
			}

			r.Group(func(r chi.Router) {
				r.Use(jwtauth.Verifier(tokenAuth))
				r.Use(jwtauth.Authenticator(tokenAuth))
				r.Use(core.RequireAuthenticationMiddleware(userService))
				r.Use(gameHandler.GameMiddleware())

				r.Post("/phases", phaseHandler.CreatePhase)
				r.Get("/current-phase", phaseHandler.GetCurrentPhase)
				r.Get("/phases", phaseHandler.GetGamePhases)
				r.Post("/actions", phaseHandler.SubmitAction)
				r.Get("/actions", phaseHandler.GetGameActions)
				r.Get("/actions/mine", phaseHandler.GetUserActions)

				// Action results
				r.Post("/results", phaseHandler.CreateActionResult)
				r.Get("/results", phaseHandler.GetGameActionResults)
				r.Get("/results/mine", phaseHandler.GetUserActionResults)
				r.Put("/results/{resultId}", phaseHandler.UpdateActionResult)
				r.Post("/results/{resultId}/publish", phaseHandler.PublishActionResult)
				r.Post("/phases/{phaseId}/results/publish", phaseHandler.PublishAllPhaseResults)
				r.Get("/phases/{phaseId}/results/unpublished-count", phaseHandler.GetUnpublishedResultsCount)

				// Draft character updates
				r.Post("/results/{resultId}/character-updates", phaseHandler.CreateDraftCharacterUpdate)
				r.Get("/results/{resultId}/character-updates", phaseHandler.GetDraftCharacterUpdates)
				r.Get("/results/{resultId}/character-updates/count", phaseHandler.GetDraftUpdateCount)
				r.Put("/results/{resultId}/character-updates/{draftId}", phaseHandler.UpdateDraftCharacterUpdate)
				r.Delete("/results/{resultId}/character-updates/{draftId}", phaseHandler.DeleteDraftCharacterUpdate)
			})
		})

		r.Route("/phases", func(r chi.Router) {
			phaseHandler := Handler{
				App:                     app,
				PhaseService:            &phasesvc.PhaseService{DB: testDB.Pool},
				ActionSubmissionService: &actionsvc.ActionSubmissionService{DB: testDB.Pool, Logger: app.ObsLogger, NotificationService: db.NewNotificationService(testDB.Pool, app.ObsLogger)},
				GameService:             &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger},
				NotificationService:     db.NewNotificationService(testDB.Pool, app.ObsLogger),
			}

			r.Group(func(r chi.Router) {
				r.Use(jwtauth.Verifier(tokenAuth))
				r.Use(jwtauth.Authenticator(tokenAuth))
				r.Use(core.RequireAuthenticationMiddleware(userService))

				r.Put("/{id}", phaseHandler.UpdatePhase)
				r.Put("/{id}/deadline", phaseHandler.UpdatePhaseDeadline)
				r.Delete("/{id}", phaseHandler.DeletePhase)
				r.Post("/{id}/activate", phaseHandler.ActivatePhase)
			})
		})
	})

	return r
}

// TestPhaseAPI_UpdatePhaseDeadline tests PUT /api/v1/phases/{id}/deadline
func TestPhaseAPI_UpdatePhaseDeadline(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "phases", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupFullPhaseAPITestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	phaseService := &phasesvc.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	phase, err := phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(gm.ID), core.TransitionPhaseRequest{
		PhaseType: "action",
		Title:     "Action Phase",
	})
	require.NoError(t, err)

	t.Run("GM sets a deadline on a phase", func(t *testing.T) {
		body := map[string]string{"deadline": "2030-12-31T23:59:00Z"}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/phases/%d/deadline", phase.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		// Deadline should be set in the response
		assert.NotNil(t, response["deadline"])
	})

	t.Run("non-GM player cannot update deadline", func(t *testing.T) {
		body := map[string]string{"deadline": "2030-12-31T23:59:00Z"}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/phases/%d/deadline", phase.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}
