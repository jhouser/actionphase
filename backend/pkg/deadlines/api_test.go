package deadlines

import (
	"actionphase/pkg/core"
	db "actionphase/pkg/db/services"
	dbactions "actionphase/pkg/db/services/actions"
	dbmessages "actionphase/pkg/db/services/messages"
	"actionphase/pkg/games"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
)

// setupDeadlineTestRouter creates a test router with deadline endpoints
func setupDeadlineTestRouter(app *core.App, testDB *core.TestDatabase) *chi.Mux {
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
		// Games API - deadlines nested under games
		r.Route("/games/{gameID}", func(r chi.Router) {
			deadlineHandler := Handler{
				App:             app,
				UserService:     &db.UserService{DB: testDB.Pool, Logger: app.ObsLogger},
				GameService:     &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger},
				DeadlineService: &db.DeadlineService{DB: testDB.Pool, Logger: app.ObsLogger},
			}

			r.Group(func(r chi.Router) {
				r.Use(jwtauth.Verifier(tokenAuth))
				r.Use(jwtauth.Authenticator(tokenAuth))
				r.Use(core.RequireAuthenticationMiddleware(userService))
				r.Use(gameHandler.GameMiddleware())

				// Deadline endpoints
				r.Post("/deadlines", deadlineHandler.CreateDeadline)
				r.Get("/deadlines", deadlineHandler.GetGameDeadlines)
			})
		})

		// Dedicated deadlines router
		r.Route("/deadlines", func(r chi.Router) {
			deadlineHandler := Handler{
				App:             app,
				UserService:     &db.UserService{DB: testDB.Pool, Logger: app.ObsLogger},
				GameService:     &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger},
				DeadlineService: &db.DeadlineService{DB: testDB.Pool, Logger: app.ObsLogger},
			}

			r.Group(func(r chi.Router) {
				r.Use(jwtauth.Verifier(tokenAuth))
				r.Use(jwtauth.Authenticator(tokenAuth))
				r.Use(core.RequireAuthenticationMiddleware(userService))

				r.Get("/upcoming", deadlineHandler.GetUpcomingDeadlines)
				r.Patch("/{deadlineId}", deadlineHandler.UpdateDeadline)
				r.Delete("/{deadlineId}", deadlineHandler.DeleteDeadline)
			})
		})
	})

	return r
}

// TestCreateDeadline_Success tests successful deadline creation
func TestCreateDeadline_Success(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	testDB.CleanupTables(t, "game_deadlines", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "game_deadlines", "games", "sessions", "users")

	router := setupDeadlineTestRouter(app, testDB)

	// Create GM user and regular user
	gmUser := testDB.CreateTestUser(t, "testgm", "testgm@example.com")
	// Note: only gmUser needed for this test

	// Create a game owned by test GM
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	game, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:       "Test Game for Deadlines",
		Description: "Testing deadline creation",
		GMUserID:    int32(gmUser.ID),
		IsPublic:    true,
	})
	core.AssertNoError(t, err, "Game creation should succeed")

	// Create auth token for GM
	accessToken, err := core.CreateTestJWTTokenForUser(app, gmUser)
	core.AssertNoError(t, err, "Test token creation should succeed")

	// Create deadline request
	deadlineTime := time.Now().Add(7 * 24 * time.Hour) // 7 days from now
	reqBody := CreateDeadlineRequest{
		Title:       "Action Submission Deadline",
		Description: "Submit all character actions by this date",
		Deadline:    deadlineTime,
	}
	reqJSON, err := json.Marshal(reqBody)
	core.AssertNoError(t, err, "JSON marshaling should succeed")

	// Make request
	req := httptest.NewRequest(http.MethodPost, "/api/v1/games/"+strconv.Itoa(int(game.ID))+"/deadlines", bytes.NewReader(reqJSON))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusCreated, w.Code, "Should return 201 Created")

	var response DeadlineResponse
	err = json.NewDecoder(w.Body).Decode(&response)
	core.AssertNoError(t, err, "Should decode response")

	core.AssertEqual(t, game.ID, response.GameID, "Game ID should match")
	core.AssertEqual(t, "Action Submission Deadline", response.Title, "Title should match")
	core.AssertEqual(t, "Submit all character actions by this date", *response.Description, "Description should match")

	// Verify non-nil fields
	if response.Deadline == nil {
		t.Fatal("Deadline should not be nil")
	}
	if response.CreatedAt == nil {
		t.Fatal("CreatedAt should not be nil")
	}
	if response.UpdatedAt == nil {
		t.Fatal("UpdatedAt should not be nil")
	}
}

// TestCreateDeadline_Unauthorized tests that non-GM cannot create deadline
func TestCreateDeadline_Unauthorized(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	testDB.CleanupTables(t, "game_deadlines", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "game_deadlines", "games", "sessions", "users")

	router := setupDeadlineTestRouter(app, testDB)

	// Create GM user and regular user
	gmUser := testDB.CreateTestUser(t, "testgm", "testgm@example.com")
	regularUser := testDB.CreateTestUser(t, "regularuser", "regular@example.com")

	// Create a game owned by test GM
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	game, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:       "Test Game for Deadlines",
		Description: "Testing unauthorized access",
		GMUserID:    int32(gmUser.ID),
		IsPublic:    true,
	})
	core.AssertNoError(t, err, "Game creation should succeed")

	// Create auth token for regular user (not GM)
	accessToken, err := core.CreateTestJWTTokenForUser(app, regularUser)
	core.AssertNoError(t, err, "Test token creation should succeed")

	// Try to create deadline
	deadlineTime := time.Now().Add(7 * 24 * time.Hour)
	reqBody := CreateDeadlineRequest{
		Title:       "Unauthorized Deadline",
		Description: "This should fail",
		Deadline:    deadlineTime,
	}
	reqJSON, err := json.Marshal(reqBody)
	core.AssertNoError(t, err, "JSON marshaling should succeed")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/games/"+strconv.Itoa(int(game.ID))+"/deadlines", bytes.NewReader(reqJSON))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusUnauthorized, w.Code, "Should return 401 Unauthorized")
}

// TestCreateDeadline_InvalidGameID tests handling of invalid game ID
func TestCreateDeadline_InvalidGameID(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	testDB.CleanupTables(t, "game_deadlines", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "game_deadlines", "games", "sessions", "users")

	router := setupDeadlineTestRouter(app, testDB)

	// Create GM user
	gmUser := testDB.CreateTestUser(t, "testgm", "testgm@example.com")

	accessToken, err := core.CreateTestJWTTokenForUser(app, gmUser)
	core.AssertNoError(t, err, "Test token creation should succeed")

	deadlineTime := time.Now().Add(7 * 24 * time.Hour)
	reqBody := CreateDeadlineRequest{
		Title:       "Test Deadline",
		Description: "Testing invalid game ID",
		Deadline:    deadlineTime,
	}
	reqJSON, err := json.Marshal(reqBody)
	core.AssertNoError(t, err, "JSON marshaling should succeed")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/games/invalid/deadlines", bytes.NewReader(reqJSON))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusBadRequest, w.Code, "Should return 400 Bad Request for invalid game ID")
}

// TestGetGameDeadlines_Success tests retrieving deadlines for a game
func TestGetGameDeadlines_Success(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)
	deadlineService := &db.DeadlineService{DB: testDB.Pool, Logger: app.ObsLogger}

	testDB.CleanupTables(t, "game_deadlines", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "game_deadlines", "games", "sessions", "users")

	router := setupDeadlineTestRouter(app, testDB)

	// Create GM user
	gmUser := testDB.CreateTestUser(t, "testgm", "testgm@example.com")

	// Create a game
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	game, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:       "Test Game for Deadlines",
		Description: "Testing deadline retrieval",
		GMUserID:    int32(gmUser.ID),
		IsPublic:    true,
	})
	core.AssertNoError(t, err, "Game creation should succeed")

	// Create multiple deadlines
	deadline1, err := deadlineService.CreateDeadline(context.Background(), core.CreateDeadlineRequest{
		GameID:      game.ID,
		Title:       "First Deadline",
		Description: "First test deadline",
		Deadline:    time.Now().Add(7 * 24 * time.Hour),
		CreatedBy:   int32(gmUser.ID),
	})
	core.AssertNoError(t, err, "Deadline 1 creation should succeed")

	deadline2, err := deadlineService.CreateDeadline(context.Background(), core.CreateDeadlineRequest{
		GameID:      game.ID,
		Title:       "Second Deadline",
		Description: "Second test deadline",
		Deadline:    time.Now().Add(14 * 24 * time.Hour),
		CreatedBy:   int32(gmUser.ID),
	})
	core.AssertNoError(t, err, "Deadline 2 creation should succeed")

	// Create auth token for GM
	accessToken, err := core.CreateTestJWTTokenForUser(app, gmUser)
	core.AssertNoError(t, err, "Test token creation should succeed")

	// Get deadlines
	req := httptest.NewRequest(http.MethodGet, "/api/v1/games/"+strconv.Itoa(int(game.ID))+"/deadlines", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusOK, w.Code, "Should return 200 OK")

	var response []*UnifiedDeadlineResponse
	err = json.NewDecoder(w.Body).Decode(&response)
	core.AssertNoError(t, err, "Should decode response")

	core.AssertEqual(t, 2, len(response), "Should return 2 deadlines")

	// Both deadlines should be type "deadline" from game_deadlines table
	core.AssertEqual(t, "deadline", response[0].DeadlineType, "First deadline type should be 'deadline'")
	core.AssertEqual(t, deadline1.ID, response[0].SourceID, "First deadline source_id should match")
	core.AssertEqual(t, "First Deadline", response[0].Title, "First deadline title should match")

	core.AssertEqual(t, "deadline", response[1].DeadlineType, "Second deadline type should be 'deadline'")
	core.AssertEqual(t, deadline2.ID, response[1].SourceID, "Second deadline source_id should match")
	core.AssertEqual(t, "Second Deadline", response[1].Title, "Second deadline title should match")
}

// TestGetGameDeadlines_NonParticipant tests that any authenticated user can view deadlines,
// even if they are not a GM or participant of the game.
func TestGetGameDeadlines_NonParticipant(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	testDB.CleanupTables(t, "game_deadlines", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "game_deadlines", "games", "sessions", "users")

	router := setupDeadlineTestRouter(app, testDB)

	// Create GM user and a user who is not a participant
	gmUser := testDB.CreateTestUser(t, "testgm", "testgm@example.com")
	outsideUser := testDB.CreateTestUser(t, "outsideuser", "outside@example.com")

	// Create a game owned by test GM
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	game, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:       "Some Game",
		Description: "Testing non-participant access",
		GMUserID:    int32(gmUser.ID),
		IsPublic:    false,
	})
	core.AssertNoError(t, err, "Game creation should succeed")

	// Non-participant should be able to view deadlines
	accessToken, err := core.CreateTestJWTTokenForUser(app, outsideUser)
	core.AssertNoError(t, err, "Test token creation should succeed")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/games/"+strconv.Itoa(int(game.ID))+"/deadlines", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusOK, w.Code, "Non-participant should be able to view deadlines")
}

// TestUpdateDeadline_Success tests successful deadline update
func TestUpdateDeadline_Success(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)
	deadlineService := &db.DeadlineService{DB: testDB.Pool, Logger: app.ObsLogger}

	testDB.CleanupTables(t, "game_deadlines", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "game_deadlines", "games", "sessions", "users")

	router := setupDeadlineTestRouter(app, testDB)

	// Create GM user
	gmUser := testDB.CreateTestUser(t, "testgm", "testgm@example.com")

	// Create a game
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	game, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:       "Test Game",
		Description: "Testing deadline update",
		GMUserID:    int32(gmUser.ID),
		IsPublic:    true,
	})
	core.AssertNoError(t, err, "Game creation should succeed")

	// Create a deadline
	deadline, err := deadlineService.CreateDeadline(context.Background(), core.CreateDeadlineRequest{
		GameID:      game.ID,
		Title:       "Original Title",
		Description: "Original description",
		Deadline:    time.Now().Add(7 * 24 * time.Hour),
		CreatedBy:   int32(gmUser.ID),
	})
	core.AssertNoError(t, err, "Deadline creation should succeed")

	// Create auth token for GM
	accessToken, err := core.CreateTestJWTTokenForUser(app, gmUser)
	core.AssertNoError(t, err, "Test token creation should succeed")

	// Update deadline
	newDeadlineTime := time.Now().Add(14 * 24 * time.Hour)
	reqBody := UpdateDeadlineRequest{
		Title:       "Updated Title",
		Description: "Updated description",
		Deadline:    newDeadlineTime,
	}
	reqJSON, err := json.Marshal(reqBody)
	core.AssertNoError(t, err, "JSON marshaling should succeed")

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/deadlines/"+strconv.Itoa(int(deadline.ID)), bytes.NewReader(reqJSON))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusOK, w.Code, "Should return 200 OK")

	var response DeadlineResponse
	err = json.NewDecoder(w.Body).Decode(&response)
	core.AssertNoError(t, err, "Should decode response")

	core.AssertEqual(t, "Updated Title", response.Title, "Title should be updated")
	core.AssertEqual(t, "Updated description", *response.Description, "Description should be updated")
}

// TestUpdateDeadline_Unauthorized tests that non-GM cannot update deadline
func TestUpdateDeadline_Unauthorized(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)
	deadlineService := &db.DeadlineService{DB: testDB.Pool, Logger: app.ObsLogger}

	testDB.CleanupTables(t, "game_deadlines", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "game_deadlines", "games", "sessions", "users")

	router := setupDeadlineTestRouter(app, testDB)

	// Create GM user and regular user
	gmUser := testDB.CreateTestUser(t, "testgm", "testgm@example.com")
	regularUser := testDB.CreateTestUser(t, "regularuser", "regular@example.com")

	// Create a game owned by test GM
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	game, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:       "Test Game",
		Description: "Testing unauthorized update",
		GMUserID:    int32(gmUser.ID),
		IsPublic:    true,
	})
	core.AssertNoError(t, err, "Game creation should succeed")

	// Create a deadline
	deadline, err := deadlineService.CreateDeadline(context.Background(), core.CreateDeadlineRequest{
		GameID:      game.ID,
		Title:       "Test Deadline",
		Description: "Testing",
		Deadline:    time.Now().Add(7 * 24 * time.Hour),
		CreatedBy:   int32(gmUser.ID),
	})
	core.AssertNoError(t, err, "Deadline creation should succeed")

	// Create auth token for regular user (not GM)
	accessToken, err := core.CreateTestJWTTokenForUser(app, regularUser)
	core.AssertNoError(t, err, "Test token creation should succeed")

	// Try to update deadline
	reqBody := UpdateDeadlineRequest{
		Title:       "Unauthorized Update",
		Description: "This should fail",
		Deadline:    time.Now().Add(14 * 24 * time.Hour),
	}
	reqJSON, err := json.Marshal(reqBody)
	core.AssertNoError(t, err, "JSON marshaling should succeed")

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/deadlines/"+strconv.Itoa(int(deadline.ID)), bytes.NewReader(reqJSON))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusUnauthorized, w.Code, "Should return 401 Unauthorized")
}

// TestUpdateDeadline_NotFound tests handling of non-existent deadline
func TestUpdateDeadline_NotFound(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	testDB.CleanupTables(t, "game_deadlines", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "game_deadlines", "games", "sessions", "users")

	router := setupDeadlineTestRouter(app, testDB)

	// Create GM user
	gmUser := testDB.CreateTestUser(t, "testgm", "testgm@example.com")

	accessToken, err := core.CreateTestJWTTokenForUser(app, gmUser)
	core.AssertNoError(t, err, "Test token creation should succeed")

	reqBody := UpdateDeadlineRequest{
		Title:       "Non-existent Deadline",
		Description: "This should fail",
		Deadline:    time.Now().Add(14 * 24 * time.Hour),
	}
	reqJSON, err := json.Marshal(reqBody)
	core.AssertNoError(t, err, "JSON marshaling should succeed")

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/deadlines/99999", bytes.NewReader(reqJSON))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusNotFound, w.Code, "Should return 404 Not Found")
}

// TestDeleteDeadline_Success tests successful deadline deletion
func TestDeleteDeadline_Success(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)
	deadlineService := &db.DeadlineService{DB: testDB.Pool, Logger: app.ObsLogger}

	testDB.CleanupTables(t, "game_deadlines", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "game_deadlines", "games", "sessions", "users")

	router := setupDeadlineTestRouter(app, testDB)

	// Create GM user
	gmUser := testDB.CreateTestUser(t, "testgm", "testgm@example.com")

	// Create a game
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	game, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:       "Test Game",
		Description: "Testing deadline deletion",
		GMUserID:    int32(gmUser.ID),
		IsPublic:    true,
	})
	core.AssertNoError(t, err, "Game creation should succeed")

	// Create a deadline
	deadline, err := deadlineService.CreateDeadline(context.Background(), core.CreateDeadlineRequest{
		GameID:      game.ID,
		Title:       "Test Deadline",
		Description: "To be deleted",
		Deadline:    time.Now().Add(7 * 24 * time.Hour),
		CreatedBy:   int32(gmUser.ID),
	})
	core.AssertNoError(t, err, "Deadline creation should succeed")

	// Create auth token for GM
	accessToken, err := core.CreateTestJWTTokenForUser(app, gmUser)
	core.AssertNoError(t, err, "Test token creation should succeed")

	// Delete deadline
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/deadlines/"+strconv.Itoa(int(deadline.ID)), nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusNoContent, w.Code, "Should return 204 No Content")

	// Verify deletion
	_, err = deadlineService.GetDeadline(context.Background(), deadline.ID)
	core.AssertError(t, err, "Getting deleted deadline should fail")
}

// TestDeleteDeadline_Unauthorized tests that non-GM cannot delete deadline
func TestDeleteDeadline_Unauthorized(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)
	deadlineService := &db.DeadlineService{DB: testDB.Pool, Logger: app.ObsLogger}

	testDB.CleanupTables(t, "game_deadlines", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "game_deadlines", "games", "sessions", "users")

	router := setupDeadlineTestRouter(app, testDB)

	// Create GM user and regular user
	gmUser := testDB.CreateTestUser(t, "testgm", "testgm@example.com")
	regularUser := testDB.CreateTestUser(t, "regularuser", "regular@example.com")

	// Create a game owned by test GM
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	game, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:       "Test Game",
		Description: "Testing unauthorized deletion",
		GMUserID:    int32(gmUser.ID),
		IsPublic:    true,
	})
	core.AssertNoError(t, err, "Game creation should succeed")

	// Create a deadline
	deadline, err := deadlineService.CreateDeadline(context.Background(), core.CreateDeadlineRequest{
		GameID:      game.ID,
		Title:       "Test Deadline",
		Description: "Should not be deleted",
		Deadline:    time.Now().Add(7 * 24 * time.Hour),
		CreatedBy:   int32(gmUser.ID),
	})
	core.AssertNoError(t, err, "Deadline creation should succeed")

	// Create auth token for regular user (not GM)
	accessToken, err := core.CreateTestJWTTokenForUser(app, regularUser)
	core.AssertNoError(t, err, "Test token creation should succeed")

	// Try to delete deadline
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/deadlines/"+strconv.Itoa(int(deadline.ID)), nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusUnauthorized, w.Code, "Should return 401 Unauthorized")
}

// TestDeleteDeadline_NotFound tests handling of non-existent deadline
func TestDeleteDeadline_NotFound(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	testDB.CleanupTables(t, "game_deadlines", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "game_deadlines", "games", "sessions", "users")

	router := setupDeadlineTestRouter(app, testDB)

	// Create GM user
	gmUser := testDB.CreateTestUser(t, "testgm", "testgm@example.com")

	accessToken, err := core.CreateTestJWTTokenForUser(app, gmUser)
	core.AssertNoError(t, err, "Test token creation should succeed")

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/deadlines/99999", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusNotFound, w.Code, "Should return 404 Not Found")
}

// TestGetUpcomingDeadlines_Success tests retrieving upcoming deadlines across all user's games
func TestGetUpcomingDeadlines_Success(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)
	deadlineService := &db.DeadlineService{DB: testDB.Pool, Logger: app.ObsLogger}

	testDB.CleanupTables(t, "game_deadlines", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "game_deadlines", "games", "sessions", "users")

	router := setupDeadlineTestRouter(app, testDB)

	// Create GM user
	gmUser := testDB.CreateTestUser(t, "testgm", "testgm@example.com")

	// Create two games
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	game1, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:       "Game 1",
		Description: "First game",
		GMUserID:    int32(gmUser.ID),
		IsPublic:    true,
	})
	core.AssertNoError(t, err, "Game 1 creation should succeed")

	game2, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:       "Game 2",
		Description: "Second game",
		GMUserID:    int32(gmUser.ID),
		IsPublic:    true,
	})
	core.AssertNoError(t, err, "Game 2 creation should succeed")

	// Create deadlines for both games
	_, err = deadlineService.CreateDeadline(context.Background(), core.CreateDeadlineRequest{
		GameID:      game1.ID,
		Title:       "Game 1 Deadline",
		Description: "First game deadline",
		Deadline:    time.Now().Add(7 * 24 * time.Hour),
		CreatedBy:   int32(gmUser.ID),
	})
	core.AssertNoError(t, err, "Deadline 1 creation should succeed")

	_, err = deadlineService.CreateDeadline(context.Background(), core.CreateDeadlineRequest{
		GameID:      game2.ID,
		Title:       "Game 2 Deadline",
		Description: "Second game deadline",
		Deadline:    time.Now().Add(3 * 24 * time.Hour),
		CreatedBy:   int32(gmUser.ID),
	})
	core.AssertNoError(t, err, "Deadline 2 creation should succeed")

	// Create auth token for GM
	accessToken, err := core.CreateTestJWTTokenForUser(app, gmUser)
	core.AssertNoError(t, err, "Test token creation should succeed")

	// Get upcoming deadlines
	req := httptest.NewRequest(http.MethodGet, "/api/v1/deadlines/upcoming", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusOK, w.Code, "Should return 200 OK")

	var response []*DeadlineWithGameResponse
	err = json.NewDecoder(w.Body).Decode(&response)
	core.AssertNoError(t, err, "Should decode response")

	core.AssertEqual(t, 2, len(response), "Should return 2 deadlines")

	// Verify game titles are included
	foundGame1 := false
	foundGame2 := false
	for _, deadline := range response {
		if deadline.GameTitle == "Game 1" {
			foundGame1 = true
		}
		if deadline.GameTitle == "Game 2" {
			foundGame2 = true
		}
	}
	core.AssertEqual(t, true, foundGame1, "Should include Game 1 deadline")
	core.AssertEqual(t, true, foundGame2, "Should include Game 2 deadline")
}

// TestGetUpcomingDeadlines_WithLimit tests the limit parameter
func TestGetUpcomingDeadlines_WithLimit(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)
	deadlineService := &db.DeadlineService{DB: testDB.Pool, Logger: app.ObsLogger}

	testDB.CleanupTables(t, "game_deadlines", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "game_deadlines", "games", "sessions", "users")

	router := setupDeadlineTestRouter(app, testDB)

	// Create GM user
	gmUser := testDB.CreateTestUser(t, "testgm", "testgm@example.com")

	// Create a game
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	game, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:       "Test Game",
		Description: "Testing limit parameter",
		GMUserID:    int32(gmUser.ID),
		IsPublic:    true,
	})
	core.AssertNoError(t, err, "Game creation should succeed")

	// Create 5 deadlines
	for i := 1; i <= 5; i++ {
		_, err := deadlineService.CreateDeadline(context.Background(), core.CreateDeadlineRequest{
			GameID:      game.ID,
			Title:       "Deadline " + strconv.Itoa(i),
			Description: "Test deadline",
			Deadline:    time.Now().Add(time.Duration(i*7) * 24 * time.Hour),
			CreatedBy:   int32(gmUser.ID),
		})
		core.AssertNoError(t, err, "Deadline creation should succeed")
	}

	// Create auth token for GM
	accessToken, err := core.CreateTestJWTTokenForUser(app, gmUser)
	core.AssertNoError(t, err, "Test token creation should succeed")

	// Get upcoming deadlines with limit=3
	req := httptest.NewRequest(http.MethodGet, "/api/v1/deadlines/upcoming?limit=3", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusOK, w.Code, "Should return 200 OK")

	var response []*DeadlineWithGameResponse
	err = json.NewDecoder(w.Body).Decode(&response)
	core.AssertNoError(t, err, "Should decode response")

	core.AssertEqual(t, 3, len(response), "Should return only 3 deadlines due to limit")
}
