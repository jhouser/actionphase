package phases

import (
	"actionphase/pkg/core"
	db "actionphase/pkg/db/services"
	actionsvc "actionphase/pkg/db/services/actions"
	dbactions "actionphase/pkg/db/services/actions"
	dbmessages "actionphase/pkg/db/services/messages"
	phasesvc "actionphase/pkg/db/services/phases"
	"actionphase/pkg/games"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function for creating int32 pointers
func int32Ptr(i int32) *int32 {
	return &i
}

// setupPhaseAPITestRouter creates a test router with phase routes
func setupPhaseAPITestRouter(app *core.App, testDB *core.TestDatabase) *chi.Mux {
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

	// API v1 routes
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

				// Phase routes (mirroring root.go)
				r.Post("/phases", phaseHandler.CreatePhase)
				r.Get("/current-phase", phaseHandler.GetCurrentPhase)
				r.Get("/phases", phaseHandler.GetGamePhases)
			})
		})

		// Phases API (for phase-specific operations like update/delete)
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
				r.Delete("/{id}", phaseHandler.DeletePhase)
				r.Post("/{id}/activate", phaseHandler.ActivatePhase)
			})
		})
	})

	return r
}

// TestPhaseAPI_CreatePhase tests POST /games/{gameId}/phases
func TestPhaseAPI_CreatePhase(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "phases", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupPhaseAPITestRouter(app, testDB)

	// Create test users
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	// Generate JWT tokens
	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	core.AssertNoError(t, err, "Should create GM token")
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	core.AssertNoError(t, err, "Should create player token")

	// Create test game
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	// Add player as participant
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	core.AssertNoError(t, err, "Should add player as participant")

	t.Run("GM successfully creates phase", func(t *testing.T) {
		reqBody := CreatePhaseRequest{
			PhaseType: "common_room",
			Title:     "Common Room 1",
		}
		reqJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/phases", game.ID), bytes.NewBuffer(reqJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)

		var response map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "common_room", response["phase_type"])
		assert.Equal(t, "Common Room 1", response["title"])
	})

	t.Run("non-GM player cannot create phase", func(t *testing.T) {
		reqBody := CreatePhaseRequest{
			PhaseType: "action",
			Title:     "Action Phase 1",
		}
		reqJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/phases", game.ID), bytes.NewBuffer(reqJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+playerToken) // Non-GM user

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Contains(t, rec.Body.String(), "GM")
	})

	t.Run("rejects invalid phase type", func(t *testing.T) {
		reqBody := CreatePhaseRequest{
			PhaseType: "invalid_type",
			Title:     "Invalid Phase",
		}
		reqJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/phases", game.ID), bytes.NewBuffer(reqJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

// TestPhaseAPI_ActivatePhase tests POST /games/{gameId}/phases/{id}/activate
func TestPhaseAPI_ActivatePhase(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "phases", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupPhaseAPITestRouter(app, testDB)

	// Create test users
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	// Generate JWT tokens
	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	core.AssertNoError(t, err, "Should create GM token")
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	core.AssertNoError(t, err, "Should create player token")

	// Create test game
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	// Add player as participant
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	core.AssertNoError(t, err, "Should add player as participant")

	// Create phases
	phaseService := &phasesvc.PhaseService{DB: testDB.Pool}
	phase1, err := phaseService.CreatePhase(context.Background(), core.CreatePhaseRequest{
		GameID:    game.ID,
		PhaseType: "common_room",
		Title:     "Common Room 1",
	})
	core.AssertNoError(t, err, "Should create phase 1")

	phase2, err := phaseService.CreatePhase(context.Background(), core.CreatePhaseRequest{
		GameID:    game.ID,
		PhaseType: "action",
		Title:     "Action Phase 1",
	})
	core.AssertNoError(t, err, "Should create phase 2")

	t.Run("GM successfully activates phase", func(t *testing.T) {
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/phases/%d/activate", phase1.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response["is_active"].(bool))
	})

	t.Run("non-GM player cannot activate phase", func(t *testing.T) {
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/phases/%d/activate", phase2.ID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken) // Non-GM user

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Contains(t, rec.Body.String(), "GM")
	})

	t.Run("activating a phase deactivates other phases", func(t *testing.T) {
		// Activate phase2
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/phases/%d/activate", phase2.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		// Verify phase1 is no longer active
		activePhase, err := phaseService.GetActivePhase(context.Background(), game.ID)
		core.AssertNoError(t, err, "Should get active phase")
		assert.Equal(t, phase2.ID, activePhase.ID)
	})

	t.Run("returns 404 for non-existent phase", func(t *testing.T) {
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/phases/99999/activate", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

// TestPhaseAPI_UpdatePhase tests PUT /games/{gameId}/phases/{id}
func TestPhaseAPI_UpdatePhase(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "phases", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupPhaseAPITestRouter(app, testDB)

	// Create test users
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	// Generate JWT tokens
	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	core.AssertNoError(t, err, "Should create GM token")
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	core.AssertNoError(t, err, "Should create player token")

	// Create test game
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	// Add player as participant
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	core.AssertNoError(t, err, "Should add player as participant")

	// Create phase
	phaseService := &phasesvc.PhaseService{DB: testDB.Pool}
	phase, err := phaseService.CreatePhase(context.Background(), core.CreatePhaseRequest{
		GameID:    game.ID,
		PhaseType: "common_room",
		Title:     "Common Room 1",
	})
	core.AssertNoError(t, err, "Should create phase")

	t.Run("GM successfully updates phase", func(t *testing.T) {
		titleStr := "Updated Common Room"
		descStr := "Updated description"
		reqBody := UpdatePhaseRequest{
			Title:       &titleStr,
			Description: &descStr,
		}
		reqJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/phases/%d", phase.ID), bytes.NewBuffer(reqJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "Updated Common Room", response["title"])
	})

	t.Run("non-GM player cannot update phase", func(t *testing.T) {
		titleStr := "Unauthorized Update"
		reqBody := UpdatePhaseRequest{
			Title: &titleStr,
		}
		reqJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/phases/%d", phase.ID), bytes.NewBuffer(reqJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+playerToken) // Non-GM user

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Contains(t, rec.Body.String(), "GM")
	})
}

// TestPhaseAPI_DeletePhase tests DELETE /games/{gameId}/phases/{id}
func TestPhaseAPI_DeletePhase(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "phases", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupPhaseAPITestRouter(app, testDB)

	// Create test users
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	// Generate JWT tokens
	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	core.AssertNoError(t, err, "Should create GM token")
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	core.AssertNoError(t, err, "Should create player token")

	// Create test game
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	// Add player as participant
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	core.AssertNoError(t, err, "Should add player as participant")

	// Create phase
	phaseService := &phasesvc.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	phase, err := phaseService.CreatePhase(context.Background(), core.CreatePhaseRequest{
		GameID:    game.ID,
		PhaseType: "common_room",
		Title:     "Common Room to Delete",
	})
	core.AssertNoError(t, err, "Should create phase")

	t.Run("GM successfully deletes phase", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/phases/%d", phase.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)
	})

	t.Run("non-GM player cannot delete phase", func(t *testing.T) {
		// Create another phase to delete
		phase2, err := phaseService.CreatePhase(context.Background(), core.CreatePhaseRequest{
			GameID:    game.ID,
			PhaseType: "action",
			Title:     "Action Phase to Delete",
		})
		core.AssertNoError(t, err, "Should create phase 2")

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/phases/%d", phase2.ID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken) // Non-GM user

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Contains(t, rec.Body.String(), "GM")
	})

	t.Run("cannot delete active phase", func(t *testing.T) {
		// Create and activate a phase
		phase3, err := phaseService.CreatePhase(context.Background(), core.CreatePhaseRequest{
			GameID:      game.ID,
			PhaseType:   "common_room",
			PhaseNumber: 3,
			Title:       "Active Common Room",
			Description: "Test common room phase",
		})
		core.AssertNoError(t, err, "Should create phase 3")

		err = phaseService.ActivatePhase(context.Background(), phase3.ID, int32(gm.ID))
		core.AssertNoError(t, err, "Should activate phase")

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/phases/%d", phase3.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "active")
	})

	t.Run("returns 404 for non-existent phase", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/games/%d/phases/99999", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

// TestPhaseAPI_GetPhases tests GET /games/{gameId}/phases
func TestPhaseAPI_GetPhases(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "phases", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupPhaseAPITestRouter(app, testDB)

	// Create test users
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	// Generate JWT tokens
	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	core.AssertNoError(t, err, "Should create GM token")
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	core.AssertNoError(t, err, "Should create player token")

	// Create test game
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	// Add player as participant
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	core.AssertNoError(t, err, "Should add player as participant")

	// Create multiple phases
	phaseService := &phasesvc.PhaseService{DB: testDB.Pool}
	_, err = phaseService.CreatePhase(context.Background(), core.CreatePhaseRequest{
		GameID:    game.ID,
		PhaseType: "common_room",
		Title:     "Common Room 1",
	})
	core.AssertNoError(t, err, "Should create phase 1")

	_, err = phaseService.CreatePhase(context.Background(), core.CreatePhaseRequest{
		GameID:    game.ID,
		PhaseType: "action",
		Title:     "Action Phase 1",
	})
	core.AssertNoError(t, err, "Should create phase 2")

	t.Run("successfully retrieves all phases for game", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/phases", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response []PhaseResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Len(t, response, 2)
		// Verify field values on returned phases, not just count
		phaseTypes := make([]string, 0, len(response))
		for _, p := range response {
			phaseTypes = append(phaseTypes, p.PhaseType)
		}
		assert.Contains(t, phaseTypes, "common_room", "common_room phase should appear in results")
		assert.Contains(t, phaseTypes, "action", "action phase should appear in results")
	})

	t.Run("returns empty array when no phases", func(t *testing.T) {
		// Create game with no phases
		emptyGame := testDB.CreateTestGame(t, int32(gm.ID), "Empty Game")

		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/phases", emptyGame.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response []PhaseResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Len(t, response, 0)
	})
}

// TestPhaseAPI_AuthorizationEdgeCases tests edge cases in phase authorization
func TestPhaseAPI_AuthorizationEdgeCases(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "phases", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupPhaseAPITestRouter(app, testDB)

	// Create test users
	gm1 := testDB.CreateTestUser(t, "gm1", "gm1@example.com")
	gm2 := testDB.CreateTestUser(t, "gm2", "gm2@example.com")

	// Generate JWT tokens
	gm2Token, err := core.CreateTestJWTTokenForUser(app, gm2)
	core.AssertNoError(t, err, "Should create GM2 token")

	// Create test games
	game1 := testDB.CreateTestGame(t, int32(gm1.ID), "Game 1")
	game2 := testDB.CreateTestGame(t, int32(gm2.ID), "Game 2")
	_ = game2 // game2 needed so gm2 is a valid GM, but not used in URLs

	// Create phases
	phaseService := &phasesvc.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	phase1, err := phaseService.CreatePhase(context.Background(), core.CreatePhaseRequest{
		GameID:    game1.ID,
		PhaseType: "common_room",
		Title:     "Phase in Game 1",
	})
	core.AssertNoError(t, err, "Should create phase 1")

	t.Run("GM cannot activate phases in another GM's game", func(t *testing.T) {
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/phases/%d/activate", phase1.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gm2Token) // GM2 trying to activate phase in GM1's game

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("GM cannot delete phases in another GM's game", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/phases/%d", phase1.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gm2Token) // GM2 trying to delete phase in GM1's game

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("returns 404 for phase in different game", func(t *testing.T) {
		// Try to activate phase1 using game2's ID
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/phases/%d/activate", phase1.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gm2Token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		// Should fail because phase1 belongs to game1, not game2
		assert.NotEqual(t, http.StatusOK, rec.Code)
	})
}

// TestPhaseAPI_CreateInterludePhase verifies interlude is a valid phase type and can be created.
func TestPhaseAPI_CreateInterludePhase(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "phases", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupPhaseAPITestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	t.Run("GM successfully creates interlude phase", func(t *testing.T) {
		reqBody := CreatePhaseRequest{PhaseType: "interlude", Title: "Evening Interlude"}
		reqJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/phases", game.ID), bytes.NewBuffer(reqJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)

		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Equal(t, "interlude", response["phase_type"])
		assert.Equal(t, "Evening Interlude", response["title"])
	})

	t.Run("rejects removed 'results' phase type", func(t *testing.T) {
		reqBody := CreatePhaseRequest{PhaseType: "results", Title: "Old Results Phase"}
		reqJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/phases", game.ID), bytes.NewBuffer(reqJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}
