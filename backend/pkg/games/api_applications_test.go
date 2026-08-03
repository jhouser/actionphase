package games

import (
	"actionphase/pkg/core"
	db "actionphase/pkg/db/services"
	dbactions "actionphase/pkg/db/services/actions"
	dbmessages "actionphase/pkg/db/services/messages"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
)

// setupApplicationsTestRouter creates a minimal router for testing application endpoints
func setupApplicationsTestRouter(app *core.App, testDB *core.TestDatabase) *chi.Mux {
	tokenAuth := jwtauth.New("HS256", []byte(app.Config.JWT.Secret), nil)
	userService := &db.UserService{DB: testDB.Pool, Logger: app.ObsLogger}

	r := chi.NewRouter()

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Games API
		r.Route("/games", func(r chi.Router) {
			gameHandler := Handler{
				App:                     app,
				UserService:             &db.UserService{DB: testDB.Pool, Logger: app.ObsLogger},
				GameService:             &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger},
				GameApplicationService:  &db.GameApplicationService{DB: testDB.Pool, Logger: app.ObsLogger},
				CharacterService:        &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger},
				NotificationService:     db.NewNotificationService(testDB.Pool, app.ObsLogger),
				MessageService:          &dbmessages.MessageService{DB: testDB.Pool, Logger: app.ObsLogger, Metrics: app.Observability.OTELMetrics},
				ActionSubmissionService: &dbactions.ActionSubmissionService{DB: testDB.Pool, Logger: app.ObsLogger, NotificationService: db.NewNotificationService(testDB.Pool, app.ObsLogger)},
			}

			// Public routes (no authentication required)
			r.Group(func(r chi.Router) {
				r.Use(jwtauth.Verifier(tokenAuth))
				r.With(gameHandler.GameMiddleware()).Get("/{gameID}/applicants", gameHandler.GetPublicGameApplicants)
			})

			// Authenticated routes
			r.Group(func(r chi.Router) {
				r.Use(jwtauth.Verifier(tokenAuth))
				r.Use(core.RequireAuthenticationMiddleware(userService))

				r.With(gameHandler.GameMiddleware()).Get("/{gameID}/applications", gameHandler.GetGameApplications)
			})
		})
	})

	return r
}

// PublicApplicantResponse matches the response from GetPublicGameApplicants
type PublicApplicantResponse struct {
	ID        int32  `json:"id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	AppliedAt string `json:"applied_at"`
}

// TestGetPublicGameApplicants_Success tests successful retrieval of public applicants
func TestGetPublicGameApplicants_Success(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	testDB.CleanupTables(t, "game_applications", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "game_applications", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupApplicationsTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	// Create test users
	userService := &db.UserService{DB: testDB.Pool, Logger: app.ObsLogger}
	player1, err := userService.CreateUser(&core.User{
		Username: "testplayer1",
		Password: "testpass123",
		Email:    "player1@example.com",
	})
	core.AssertNoError(t, err, "Player 1 creation should succeed")

	player2, err := userService.CreateUser(&core.User{
		Username: "testplayer2",
		Password: "testpass123",
		Email:    "player2@example.com",
	})
	core.AssertNoError(t, err, "Player 2 creation should succeed")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	applicationService := &db.GameApplicationService{DB: testDB.Pool}

	// Create a game in recruitment state
	game, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:       "Test Recruiting Game",
		Description: "Testing public applicants",
		GMUserID:    int32(fixtures.TestUser.ID),
		IsPublic:    true,
	})
	core.AssertNoError(t, err, "Game creation should succeed")

	// Update game to recruitment state
	_, err = gameService.UpdateGameState(context.Background(), game.ID, core.GameStateRecruitment)
	core.AssertNoError(t, err, "Game state update should succeed")

	// Create test applicants
	applicant1, err := applicationService.CreateGameApplication(context.Background(), core.CreateGameApplicationRequest{
		GameID:  game.ID,
		UserID:  int32(player1.ID),
		Role:    core.RolePlayer,
		Message: "I'd like to join as a player",
	})
	core.AssertNoError(t, err, "Application 1 creation should succeed")

	applicant2, err := applicationService.CreateGameApplication(context.Background(), core.CreateGameApplicationRequest{
		GameID:  game.ID,
		UserID:  int32(player2.ID),
		Role:    core.RoleAudience,
		Message: "I want to watch",
	})
	core.AssertNoError(t, err, "Application 2 creation should succeed")

	// Make request without authentication (public endpoint)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/games/"+strconv.Itoa(int(game.ID))+"/applicants", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusOK, w.Code, "Should return 200 OK")

	var response []PublicApplicantResponse
	err = json.NewDecoder(w.Body).Decode(&response)
	core.AssertNoError(t, err, "Should decode response")

	// Verify response
	core.AssertEqual(t, 2, len(response), "Should return 2 applicants")

	// Verify no sensitive data is exposed
	for _, applicant := range response {
		core.AssertTrue(t, applicant.ID > 0, "Should have applicant ID")
		core.AssertTrue(t, applicant.Username != "", "Should have username")
		core.AssertTrue(t, applicant.Role == core.RolePlayer || applicant.Role == core.RoleAudience, "Should have valid role")
		core.AssertTrue(t, applicant.AppliedAt != "", "Should have applied_at timestamp")
	}

	// Verify applicant 1
	var player1Resp *PublicApplicantResponse
	for _, a := range response {
		if a.ID == applicant1.ID {
			player1Resp = &a
			break
		}
	}
	if player1Resp == nil {
		t.Fatal("Should find applicant 1")
	}
	core.AssertEqual(t, player1.Username, player1Resp.Username, "Should have correct username")
	core.AssertEqual(t, core.RolePlayer, player1Resp.Role, "Should have player role")

	// Verify applicant 2
	var player2Resp *PublicApplicantResponse
	for _, a := range response {
		if a.ID == applicant2.ID {
			player2Resp = &a
			break
		}
	}
	if player2Resp == nil {
		t.Fatal("Should find applicant 2")
	}
	core.AssertEqual(t, player2.Username, player2Resp.Username, "Should have correct username")
	core.AssertEqual(t, core.RoleAudience, player2Resp.Role, "Should have audience role")
}

// TestGetPublicGameApplicants_ForbiddenWhenNotRecruiting tests that the endpoint rejects non-recruiting games
func TestGetPublicGameApplicants_ForbiddenWhenNotRecruiting(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	testDB.CleanupTables(t, "game_applications", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "game_applications", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupApplicationsTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	tests := []struct {
		name      string
		gameState string
	}{
		{
			name:      "setup state",
			gameState: core.GameStateSetup,
		},
		{
			name:      "character_creation state",
			gameState: core.GameStateCharacterCreation,
		},
		{
			name:      "in_progress state",
			gameState: core.GameStateInProgress,
		},
		{
			name:      "completed state",
			gameState: core.GameStateCompleted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a game
			game, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
				Title:       "Test Game - " + tt.gameState,
				Description: "Testing forbidden access",
				GMUserID:    int32(fixtures.TestUser.ID),
				IsPublic:    true,
			})
			core.AssertNoError(t, err, "Game creation should succeed")

			// Set game to test state (bypassing transition validation — state is test setup, not subject of test)
			if tt.gameState != core.GameStateSetup {
				testDB.SetGameStateDirectly(t, game.ID, tt.gameState)
			}

			// Make request without authentication
			req := httptest.NewRequest(http.MethodGet, "/api/v1/games/"+strconv.Itoa(int(game.ID))+"/applicants", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			core.AssertEqual(t, http.StatusForbidden, w.Code, "Should return 403 Forbidden for non-recruiting game")

			var response map[string]interface{}
			err = json.NewDecoder(w.Body).Decode(&response)
			core.AssertNoError(t, err, "Should decode error response")

			core.AssertEqual(t, "Forbidden.", response["status"], "Should have forbidden status")
			// Verify error message contains "recruitment"
			errMsg, ok := response["error"].(string)
			core.AssertTrue(t, ok, "Should have error message")
			core.AssertTrue(t, len(errMsg) > 0 && (errMsg == "applicant list is only visible during recruitment" || errMsg == "Applicant list is only visible during recruitment"), "Error message should mention recruitment")
		})
	}
}

// TestGetPublicGameApplicants_EmptyList tests that empty list is returned for game with no applications
func TestGetPublicGameApplicants_EmptyList(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	testDB.CleanupTables(t, "game_applications", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "game_applications", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupApplicationsTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create a game in recruitment state
	game, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:       "Test Recruiting Game",
		Description: "Testing empty applicant list",
		GMUserID:    int32(fixtures.TestUser.ID),
		IsPublic:    true,
	})
	core.AssertNoError(t, err, "Game creation should succeed")

	// Update game to recruitment state
	_, err = gameService.UpdateGameState(context.Background(), game.ID, core.GameStateRecruitment)
	core.AssertNoError(t, err, "Game state update should succeed")

	// Make request without authentication
	req := httptest.NewRequest(http.MethodGet, "/api/v1/games/"+strconv.Itoa(int(game.ID))+"/applicants", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusOK, w.Code, "Should return 200 OK")

	var response []PublicApplicantResponse
	err = json.NewDecoder(w.Body).Decode(&response)
	core.AssertNoError(t, err, "Should decode response")

	core.AssertEqual(t, 0, len(response), "Should return empty array")
}

// TestGetPublicGameApplicants_NoStatusExposed tests that application status is not exposed
func TestGetPublicGameApplicants_NoStatusExposed(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	testDB.CleanupTables(t, "game_applications", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "game_applications", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupApplicationsTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	applicationService := &db.GameApplicationService{DB: testDB.Pool}

	// Create a game in recruitment state
	game, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:       "Test Recruiting Game",
		Description: "Testing status privacy",
		GMUserID:    int32(fixtures.TestUser.ID),
		IsPublic:    true,
	})
	core.AssertNoError(t, err, "Game creation should succeed")

	// Update game to recruitment state
	_, err = gameService.UpdateGameState(context.Background(), game.ID, core.GameStateRecruitment)
	core.AssertNoError(t, err, "Game state update should succeed")

	// Create test users for applications
	player1 := testDB.CreateTestUser(t, "testplayer1", "player1")
	player2 := testDB.CreateTestUser(t, "testplayer2", "player2")

	// Create applicants with different statuses
	_, err = applicationService.CreateGameApplication(context.Background(), core.CreateGameApplicationRequest{
		GameID:  game.ID,
		UserID:  int32(player1.ID),
		Role:    core.RolePlayer,
		Message: "Pending application",
	})
	core.AssertNoError(t, err, "Pending application creation should succeed")

	approvedApp, err := applicationService.CreateGameApplication(context.Background(), core.CreateGameApplicationRequest{
		GameID:  game.ID,
		UserID:  int32(player2.ID),
		Role:    core.RolePlayer,
		Message: "To be approved",
	})
	core.AssertNoError(t, err, "Application creation should succeed")

	// Approve one application
	err = applicationService.ApproveGameApplication(context.Background(), approvedApp.ID, int32(fixtures.TestUser.ID))
	core.AssertNoError(t, err, "Application approval should succeed")

	// Make request to public endpoint
	req := httptest.NewRequest(http.MethodGet, "/api/v1/games/"+strconv.Itoa(int(game.ID))+"/applicants", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusOK, w.Code, "Should return 200 OK")

	// Decode to raw JSON to verify no status field exists
	var rawResponse []map[string]interface{}
	err = json.NewDecoder(w.Body).Decode(&rawResponse)
	core.AssertNoError(t, err, "Should decode response")

	core.AssertEqual(t, 2, len(rawResponse), "Should return 2 applicants")

	// Verify status field does NOT exist in response
	for _, applicant := range rawResponse {
		_, hasStatus := applicant["status"]
		core.AssertTrue(t, !hasStatus, "Response should NOT contain status field")

		_, hasMessage := applicant["message"]
		core.AssertTrue(t, !hasMessage, "Response should NOT contain message field")

		_, hasEmail := applicant["email"]
		core.AssertTrue(t, !hasEmail, "Response should NOT contain email field")

		_, hasReviewedAt := applicant["reviewed_at"]
		core.AssertTrue(t, !hasReviewedAt, "Response should NOT contain reviewed_at field")

		_, hasReviewedBy := applicant["reviewed_by_user_id"]
		core.AssertTrue(t, !hasReviewedBy, "Response should NOT contain reviewed_by_user_id field")

		// Verify only expected fields exist
		core.AssertTrue(t, applicant["id"] != nil, "Should have id field")
		core.AssertTrue(t, applicant["username"] != nil, "Should have username field")
		core.AssertTrue(t, applicant["role"] != nil, "Should have role field")
		core.AssertTrue(t, applicant["applied_at"] != nil, "Should have applied_at field")
	}
}

// TestGetPublicGameApplicants_OrderedByAppliedAt tests that applicants are ordered by application time
func TestGetPublicGameApplicants_OrderedByAppliedAt(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	testDB.CleanupTables(t, "game_applications", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "game_applications", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupApplicationsTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	applicationService := &db.GameApplicationService{DB: testDB.Pool}

	// Create a game in recruitment state
	game, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:       "Test Recruiting Game",
		Description: "Testing ordering",
		GMUserID:    int32(fixtures.TestUser.ID),
		IsPublic:    true,
	})
	core.AssertNoError(t, err, "Game creation should succeed")

	_, err = gameService.UpdateGameState(context.Background(), game.ID, core.GameStateRecruitment)
	core.AssertNoError(t, err, "Game state update should succeed")

	// Create test users for applications
	player1 := testDB.CreateTestUser(t, "testplayera", "playera")
	player2 := testDB.CreateTestUser(t, "testplayerb", "playerb")
	audience1 := testDB.CreateTestUser(t, "testaudiencemember", "audience")

	// Create multiple applicants (they should be ordered by creation time)
	app1, err := applicationService.CreateGameApplication(context.Background(), core.CreateGameApplicationRequest{
		GameID: game.ID,
		UserID: int32(player1.ID),
		Role:   core.RolePlayer,
	})
	core.AssertNoError(t, err, "Application 1 creation should succeed")

	app2, err := applicationService.CreateGameApplication(context.Background(), core.CreateGameApplicationRequest{
		GameID: game.ID,
		UserID: int32(player2.ID),
		Role:   core.RolePlayer,
	})
	core.AssertNoError(t, err, "Application 2 creation should succeed")

	app3, err := applicationService.CreateGameApplication(context.Background(), core.CreateGameApplicationRequest{
		GameID: game.ID,
		UserID: int32(audience1.ID),
		Role:   core.RoleAudience,
	})
	core.AssertNoError(t, err, "Application 3 creation should succeed")

	// Make request
	req := httptest.NewRequest(http.MethodGet, "/api/v1/games/"+strconv.Itoa(int(game.ID))+"/applicants", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusOK, w.Code, "Should return 200 OK")

	var response []PublicApplicantResponse
	err = json.NewDecoder(w.Body).Decode(&response)
	core.AssertNoError(t, err, "Should decode response")

	core.AssertEqual(t, 3, len(response), "Should return 3 applicants")

	// Verify ordering (should be in order of application)
	core.AssertEqual(t, app1.ID, response[0].ID, "First applicant should be app1")
	core.AssertEqual(t, app2.ID, response[1].ID, "Second applicant should be app2")
	core.AssertEqual(t, app3.ID, response[2].ID, "Third applicant should be app3")
}

// TestGetPublicGameApplicants_InvalidGameID tests error handling for invalid game ID
func TestGetPublicGameApplicants_InvalidGameID(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	testDB.CleanupTables(t, "games", "sessions", "users")
	defer testDB.CleanupTables(t, "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupApplicationsTestRouter(app, testDB)

	// Make request with invalid game ID
	req := httptest.NewRequest(http.MethodGet, "/api/v1/games/invalid/applicants", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusBadRequest, w.Code, "Should return 400 Bad Request for invalid game ID")
}

// TestGetPublicGameApplicants_NonexistentGame tests error handling for nonexistent game
func TestGetPublicGameApplicants_NonexistentGame(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	testDB.CleanupTables(t, "games", "sessions", "users")
	defer testDB.CleanupTables(t, "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupApplicationsTestRouter(app, testDB)

	// Make request with nonexistent game ID
	req := httptest.NewRequest(http.MethodGet, "/api/v1/games/99999/applicants", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusNotFound, w.Code, "Should return 404 for nonexistent game")
}
