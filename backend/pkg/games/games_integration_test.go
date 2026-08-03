package games

import (
	"actionphase/pkg/auth"
	"actionphase/pkg/core"
	db "actionphase/pkg/db/services"
	dbactions "actionphase/pkg/db/services/actions"
	dbmessages "actionphase/pkg/db/services/messages"
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

// TestGameAPI_CompleteGameLifecycle tests the complete game management workflow
func TestGameAPI_CompleteGameLifecycle(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	// Clean up before and after to ensure isolation
	testDB.CleanupTables(t, "game_applications", "game_participants", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "game_applications", "game_participants", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)

	router := setupGameTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	// Create auth token for test user
	accessToken, err := core.CreateTestJWTTokenForUser(app, fixtures.TestUser)
	core.AssertNoError(t, err, "Test token creation should succeed")

	var createdGameID int32

	// Step 1: Create a game
	t.Run("create_game", func(t *testing.T) {
		gameData := CreateGameRequest{
			Title:       "Test RPG Campaign",
			Description: "A comprehensive test campaign for integration testing purposes",
			Genre:       "Fantasy RPG",
			MaxPlayers:  6,
		}

		payload, _ := json.Marshal(gameData)
		req := httptest.NewRequest("POST", "/api/v1/games", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+accessToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 201, w.Code, "Game creation should succeed")

		var response GameResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")

		// Store game ID for subsequent tests
		createdGameID = response.ID

		core.AssertEqual(t, gameData.Title, response.Title, "Title should match")
		core.AssertEqual(t, gameData.Description, response.Description, "Description should match")
		core.AssertEqual(t, gameData.Genre, response.Genre, "Genre should match")
		core.AssertEqual(t, gameData.MaxPlayers, response.MaxPlayers, "Max players should match")
		core.AssertEqual(t, "setup", response.State, "New game should be in setup state")
	})

	// Step 2: Get the created game
	t.Run("get_game", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/games/"+strconv.Itoa(int(createdGameID)), nil)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "Get game should succeed")

		var response GameResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")

		core.AssertEqual(t, createdGameID, response.ID, "Game ID should match")
		core.AssertEqual(t, "Test RPG Campaign", response.Title, "Title should match")
	})

	// Step 3: Get game with details
	t.Run("get_game_with_details", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/games/"+strconv.Itoa(int(createdGameID))+"/details", nil)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "Get game with details should succeed")

		var response GameWithDetailsResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")

		core.AssertEqual(t, createdGameID, response.ID, "Game ID should match")
		core.AssertEqual(t, int64(0), response.CurrentPlayers, "New game should have 0 players")
		core.AssertNotEqual(t, "", response.GMUsername, "GM username should be populated")
	})

	// Step 4: Update game details
	t.Run("update_game", func(t *testing.T) {
		updateData := UpdateGameRequest{
			Title:       "Updated RPG Campaign",
			Description: "An updated comprehensive test campaign",
			Genre:       "Sci-Fi RPG",
			MaxPlayers:  8,
			IsPublic:    true,
		}

		payload, _ := json.Marshal(updateData)
		req := httptest.NewRequest("PUT", "/api/v1/games/"+strconv.Itoa(int(createdGameID)), bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+accessToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "Game update should succeed")

		var response GameResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")

		core.AssertEqual(t, updateData.Title, response.Title, "Updated title should match")
		core.AssertEqual(t, updateData.Description, response.Description, "Updated description should match")
		core.AssertEqual(t, updateData.Genre, response.Genre, "Updated genre should match")
		core.AssertEqual(t, updateData.MaxPlayers, response.MaxPlayers, "Updated max players should match")
	})

	// Step 5: Advance game through valid state sequence to in_progress
	for _, step := range []string{"recruitment", "character_creation", "in_progress"} {
		step := step
		t.Run("update_game_state_to_"+step, func(t *testing.T) {
			stateData := UpdateGameStateRequest{State: step}
			payload, _ := json.Marshal(stateData)
			req := httptest.NewRequest("PUT", "/api/v1/games/"+strconv.Itoa(int(createdGameID))+"/state", bytes.NewBuffer(payload))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+accessToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			core.AssertEqual(t, 200, w.Code, "Game state update to "+step+" should succeed")

			var response GameResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			core.AssertNoError(t, err, "Response should be valid JSON")
			core.AssertEqual(t, step, response.State, "Response body should reflect new state")
		})
	}

	// Step 6: Cancel game (required before deletion)
	t.Run("cancel_game", func(t *testing.T) {
		stateData := UpdateGameStateRequest{
			State: "cancelled",
		}

		payload, _ := json.Marshal(stateData)
		req := httptest.NewRequest("PUT", "/api/v1/games/"+strconv.Itoa(int(createdGameID))+"/state", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+accessToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "Game state update to cancelled should succeed")
	})

	// Step 7: Delete game
	t.Run("delete_game", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/api/v1/games/"+strconv.Itoa(int(createdGameID)), nil)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 204, w.Code, "Game deletion should succeed")

		// Verify game is deleted by trying to get it
		getReq := httptest.NewRequest("GET", "/api/v1/games/"+strconv.Itoa(int(createdGameID)), nil)
		getReq.Header.Set("Authorization", "Bearer "+accessToken)
		getW := httptest.NewRecorder()

		router.ServeHTTP(getW, getReq)
		core.AssertEqual(t, 404, getW.Code, "Getting deleted game should return 404")
	})
}

// TestGameAPI_PublicEndpoints tests game listing endpoints that require authentication
func TestGameAPI_PublicEndpoints(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_applications", "game_participants", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)

	router := setupGameTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	// Create auth token for test user
	accessToken, err := core.CreateTestJWTTokenForUser(app, fixtures.TestUser)
	core.AssertNoError(t, err, "Test token creation should succeed")

	// Create a test game directly via service
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	createdGame, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:       "Public Test Game",
		Description: "A game for testing public endpoints",
		GMUserID:    int32(fixtures.TestUser.ID),
		Genre:       "Action",
		MaxPlayers:  4,
		IsPublic:    true,
	})
	core.AssertNoError(t, err, "Test game creation should succeed")

	// Update game to recruiting state for recruiting games test
	_, err = gameService.UpdateGameState(context.Background(), createdGame.ID, "recruitment")
	core.AssertNoError(t, err, "Game state update should succeed")

	// Test get all games (authenticated) - using GetFilteredGames endpoint
	t.Run("get_all_games", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/games", nil)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "Get all games should succeed")

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")

		// GetFilteredGames returns {games: [...], metadata: {...}}
		games, ok := response["games"].([]interface{})
		core.AssertTrue(t, ok, "Response should have games array")
		core.AssertTrue(t, len(games) >= 1, "Should return at least one game")

		// Find our test game in the response
		found := false
		for _, g := range games {
			game := g.(map[string]interface{})
			if int32(game["id"].(float64)) == createdGame.ID {
				core.AssertEqual(t, "Public Test Game", game["title"].(string), "Game title should match")
				found = true
				break
			}
		}
		core.AssertTrue(t, found, "Created game should be in the response")
	})

	// Test get recruiting games (authenticated)
	t.Run("get_recruiting_games", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/games/recruiting", nil)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "Get recruiting games should succeed")

		var response []map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")

		// New games should be in recruiting state by default
		found := false
		for _, game := range response {
			if int32(game["id"].(float64)) == createdGame.ID {
				core.AssertEqual(t, "recruitment", game["state"].(string), "Game should be recruiting")
				found = true
				break
			}
		}
		core.AssertTrue(t, found, "Recruiting game should be in the response")
	})

	// Test get single game (authenticated)
	t.Run("get_single_game", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/games/"+strconv.Itoa(int(createdGame.ID)), nil)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "Get single game should succeed")

		var response GameResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")

		core.AssertEqual(t, createdGame.ID, response.ID, "Game ID should match")
		core.AssertEqual(t, "Public Test Game", response.Title, "Game title should match")
	})

	// Test get game participants (authenticated)
	t.Run("get_game_participants", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/games/"+strconv.Itoa(int(createdGame.ID))+"/participants", nil)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "Get game participants should succeed")

		var response []map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")

		core.AssertEqual(t, 0, len(response), "New game should have no participants")
	})
}

// TestGameAPI_ParticipantManagement tests game participation features
func TestGameAPI_ParticipantManagement(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_applications", "game_participants", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)

	router := setupGameTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	// Create auth token for test user
	accessToken, _ := core.CreateTestJWTTokenForUser(app, fixtures.TestUser)

	// Create a game for testing participation
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	testGame, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:       "Participant Test Game",
		Description: "A game for testing participant management",
		GMUserID:    int32(fixtures.TestUser.ID),
		MaxPlayers:  3,
		IsPublic:    true,
	})
	core.AssertNoError(t, err, "Test game creation should succeed")

	// Update game to recruitment state to allow joining
	_, err = gameService.UpdateGameState(context.Background(), testGame.ID, "recruitment")
	core.AssertNoError(t, err, "Game state update should succeed")

	// Create a second test user for joining the game
	userService := &db.UserService{DB: testDB.Pool, Logger: app.ObsLogger}
	secondUser := &core.User{
		Username: "participant_user_" + strconv.Itoa(int(time.Now().UnixNano())),
		Email:    "participant_" + strconv.Itoa(int(time.Now().UnixNano())) + "@test.com",
		Password: "testpassword123",
	}
	_ = secondUser.HashPassword()
	createdSecondUser, err := userService.CreateUser(secondUser)
	core.AssertNoError(t, err, "Second user creation should succeed")

	secondUserToken, _ := core.CreateTestJWTTokenForUser(app, createdSecondUser)

	// NOTE: Direct joining tests removed because direct joining is no longer supported.
	// All game participation now goes through the application system.
	// See game_applications_integration_test.go for tests of the new application-based joining process.

	// For this test, manually add participant to test leave functionality
	_, err = gameService.AddGameParticipant(context.Background(), testGame.ID, int32(createdSecondUser.ID), "player")
	core.AssertNoError(t, err, "Failed to add test participant")

	// Test getting game participants
	t.Run("get_participants", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/games/"+strconv.Itoa(int(testGame.ID))+"/participants", nil)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "Get participants should succeed")

		var response []map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")

		core.AssertEqual(t, 1, len(response), "Should have one participant")
		core.AssertEqual(t, createdSecondUser.Username, response[0]["username"].(string), "Username should match")
		core.AssertEqual(t, "player", response[0]["role"].(string), "Role should match")
	})

	// NOTE: Duplicate join test removed because direct joining is no longer supported.

	// Test leaving a game
	t.Run("leave_game", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/api/v1/games/"+strconv.Itoa(int(testGame.ID))+"/leave", nil)
		req.Header.Set("Authorization", "Bearer "+secondUserToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 204, w.Code, "Leaving game should succeed")

		// Verify participant was removed
		getReq := httptest.NewRequest("GET", "/api/v1/games/"+strconv.Itoa(int(testGame.ID))+"/participants", nil)
		getReq.Header.Set("Authorization", "Bearer "+accessToken)
		getW := httptest.NewRecorder()

		router.ServeHTTP(getW, getReq)

		var response []map[string]interface{}
		err := json.Unmarshal(getW.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")

		core.AssertEqual(t, 0, len(response), "Should have no participants after leaving")
	})
}

// TestGameAPI_Authorization tests authorization rules for game management
func TestGameAPI_Authorization(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_applications", "game_participants", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)

	router := setupGameTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	// Create a game owned by the test user
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	testGame, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:       "Authorization Test Game",
		Description: "A game for testing authorization",
		GMUserID:    int32(fixtures.TestUser.ID),
		IsPublic:    true,
	})
	core.AssertNoError(t, err, "Test game creation should succeed")

	// Create a second user who doesn't own the game
	userService := &db.UserService{DB: testDB.Pool, Logger: app.ObsLogger}
	nonOwner := &core.User{
		Username: "nonowner_" + strconv.Itoa(int(time.Now().UnixNano())),
		Email:    "nonowner_" + strconv.Itoa(int(time.Now().UnixNano())) + "@test.com",
		Password: "testpassword123",
	}
	_ = nonOwner.HashPassword()
	createdNonOwner, err := userService.CreateUser(nonOwner)
	core.AssertNoError(t, err, "Non-owner user creation should succeed")

	ownerToken, _ := core.CreateTestJWTTokenForUser(app, fixtures.TestUser)
	nonOwnerToken, _ := core.CreateTestJWTTokenForUser(app, createdNonOwner)

	// Test that non-owner cannot update game
	t.Run("non_owner_cannot_update_game", func(t *testing.T) {
		updateData := UpdateGameRequest{
			Title:       "Unauthorized Update",
			Description: "This should not work",
			IsPublic:    false,
		}

		payload, _ := json.Marshal(updateData)
		req := httptest.NewRequest("PUT", "/api/v1/games/"+strconv.Itoa(int(testGame.ID)), bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+nonOwnerToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 403, w.Code, "Non-owner should not be able to update game")
	})

	// Test that non-owner cannot update game state
	t.Run("non_owner_cannot_update_state", func(t *testing.T) {
		stateData := UpdateGameStateRequest{
			State: "active",
		}

		payload, _ := json.Marshal(stateData)
		req := httptest.NewRequest("PUT", "/api/v1/games/"+strconv.Itoa(int(testGame.ID))+"/state", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+nonOwnerToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 403, w.Code, "Non-owner should not be able to update game state")
	})

	// Test that non-owner cannot delete game
	t.Run("non_owner_cannot_delete_game", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/api/v1/games/"+strconv.Itoa(int(testGame.ID)), nil)
		req.Header.Set("Authorization", "Bearer "+nonOwnerToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 403, w.Code, "Non-owner should not be able to delete game")
	})

	// Test that owner can perform all operations
	t.Run("owner_can_update_game", func(t *testing.T) {
		updateData := UpdateGameRequest{
			Title:       "Owner Update",
			Description: "This should work",
			IsPublic:    true,
		}

		payload, _ := json.Marshal(updateData)
		req := httptest.NewRequest("PUT", "/api/v1/games/"+strconv.Itoa(int(testGame.ID)), bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+ownerToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "Owner should be able to update game")
	})
}

// TestGameAPI_ErrorHandling tests various error conditions
func TestGameAPI_ErrorHandling(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_applications", "game_participants", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)

	router := setupGameTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)
	token, _ := core.CreateTestJWTTokenForUser(app, fixtures.TestUser)

	testCases := []struct {
		name           string
		method         string
		endpoint       string
		payload        interface{}
		requiresAuth   bool
		expectedStatus int
		description    string
	}{
		{
			name:     "create_game_missing_title",
			method:   "POST",
			endpoint: "/api/v1/games",
			payload: CreateGameRequest{
				Description: "Game without title",
			},
			requiresAuth:   true,
			expectedStatus: 400,
			description:    "Creating game without title should fail",
		},
		{
			name:     "create_game_no_auth",
			method:   "POST",
			endpoint: "/api/v1/games",
			payload: CreateGameRequest{
				Title:       "Unauthorized Game",
				Description: "This should fail",
			},
			requiresAuth:   false,
			expectedStatus: 401,
			description:    "Creating game without auth should fail",
		},
		{
			name:           "get_nonexistent_game",
			method:         "GET",
			endpoint:       "/api/v1/games/99999",
			payload:        nil,
			requiresAuth:   true,
			expectedStatus: 404, // Non-existent resources should return 404
			description:    "Getting non-existent game should return 404",
		},
		{
			name:           "invalid_game_id",
			method:         "GET",
			endpoint:       "/api/v1/games/invalid",
			payload:        nil,
			requiresAuth:   true,
			expectedStatus: 400,
			description:    "Invalid game ID should return 400",
		},
		// NOTE: join_nonexistent_game test removed because direct joining is no longer supported
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var req *http.Request
			if tc.payload != nil {
				payload, _ := json.Marshal(tc.payload)
				req = httptest.NewRequest(tc.method, tc.endpoint, bytes.NewBuffer(payload))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(tc.method, tc.endpoint, nil)
			}

			if tc.requiresAuth {
				req.Header.Set("Authorization", "Bearer "+token)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			core.AssertEqual(t, tc.expectedStatus, w.Code, tc.description)

			// Verify error response has proper structure
			if w.Code >= 400 {
				// Some unauthorized responses might not be JSON (e.g., from middleware)
				if w.Header().Get("Content-Type") == "application/json" ||
					w.Code != 401 { // Allow non-JSON responses for 401 (authentication middleware)
					var response map[string]interface{}
					err := json.Unmarshal(w.Body.Bytes(), &response)
					if err != nil {
						t.Logf("Response body: %s", w.Body.String())
						t.Logf("Content-Type: %s", w.Header().Get("Content-Type"))
					}
					core.AssertNoError(t, err, "Error response should be valid JSON")
					core.AssertNotEqual(t, "", response["status"], "Error response should have status field")
				}
			}
		})
	}
}

// setupGameTestRouter creates a test router with game routes configured
func setupGameTestRouter(app *core.App, testDB *core.TestDatabase) *chi.Mux {
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

			// All routes require authentication
			r.Group(func(r chi.Router) {
				r.Use(jwtauth.Verifier(tokenAuth))
				r.Use(jwtauth.Authenticator(tokenAuth))
				r.Use(core.RequireAuthenticationMiddleware(userService))

				// Game listing and viewing
				r.Get("/", gameHandler.GetFilteredGames) // Main game listing endpoint with filters
				r.Get("/recruiting", gameHandler.GetRecruitingGames)

				// Game management
				r.Post("/", gameHandler.CreateGame)

				//Routes with game ID parameter
				r.Route("/{gameID}", func(r chi.Router) {
					r.Use(gameHandler.GameMiddleware())

					// Game listing and viewing
					r.Get("/", gameHandler.GetGame)
					r.Get("/details", gameHandler.GetGameWithDetails)
					r.Get("/participants", gameHandler.GetGameParticipants)

					// Game management
					r.Put("/", gameHandler.UpdateGame)
					r.Delete("/", gameHandler.DeleteGame)
					r.Put("/state", gameHandler.UpdateGameState)

					// Participant management
					// NOTE: join endpoint removed - use application system instead
					r.Delete("/leave", gameHandler.LeaveGame)
					r.Post("/participants/direct-add", gameHandler.AddParticipantDirectly)
					r.Delete("/participants/{userId}", gameHandler.RemovePlayer)
					r.Post("/participants/{userId}/promote-to-co-gm", gameHandler.PromoteToCoGM)
					r.Post("/participants/{userId}/demote-from-co-gm", gameHandler.DemoteFromCoGM)
					r.Post("/participants/{userId}/to-audience", gameHandler.TransitionPlayerToAudience)

					// Game application management
					r.Post("/apply", gameHandler.ApplyToGame)
					r.Get("/applications", gameHandler.GetGameApplications)
					r.Put("/applications/{applicationId}/review", gameHandler.ReviewGameApplication)
					r.Get("/application", gameHandler.GetMyGameApplication)
					r.Delete("/application", gameHandler.WithdrawGameApplication)

					// Audience management
					r.Get("/audience", gameHandler.ListAudienceMembers)
					r.Put("/settings/auto-accept-audience", gameHandler.UpdateAutoAcceptAudience)
					r.Get("/characters/audience-npcs", gameHandler.ListAudienceNPCs)
					r.Get("/private-messages/all", gameHandler.ListAllPrivateConversations)
					r.Get("/private-messages/participants", gameHandler.GetConversationParticipants)
					r.Get("/private-messages/conversations/{conversationId}", gameHandler.GetAudienceConversationMessages)
					r.Get("/action-submissions/all", gameHandler.ListAllActionSubmissions)

					// Logs
					r.Get("/logs", gameHandler.GetGameLogs)
				})

			})
		})
	})

	return r
}

// setupAuthTestRouter creates a test router with auth routes for token creation
func setupAuthTestRouter(app *core.App, testDB *core.TestDatabase) *chi.Mux {
	tokenAuth := jwtauth.New("HS256", []byte(app.Config.JWT.Secret), nil)
	userService := &db.UserService{DB: testDB.Pool, Logger: app.ObsLogger}

	r := chi.NewRouter()

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			authHandler := auth.Handler{
				App:                    app,
				UserService:            &db.UserService{DB: testDB.Pool, Logger: app.ObsLogger},
				SessionService:         &db.SessionService{DB: testDB.Pool, Logger: app.ObsLogger},
				UserPreferencesService: db.NewUserPreferencesService(testDB.Pool),
				IPBanService:           &db.IPBanService{DB: testDB.Pool, Logger: app.ObsLogger},
				FingerprintBanService:  &db.FingerprintBanService{DB: testDB.Pool, Logger: app.ObsLogger},
				DiscordService:         &db.DiscordAccountService{DB: testDB.Pool, Logger: app.ObsLogger},
			}
			r.Post("/register", authHandler.V1Register)
			r.Post("/login", authHandler.V1Login)
			r.Group(func(r chi.Router) {
				r.Use(jwtauth.Verifier(tokenAuth))
				r.Use(jwtauth.Authenticator(tokenAuth))
				r.Use(core.RequireAuthenticationMiddleware(userService))
				r.Get("/refresh", authHandler.V1Refresh)
			})
		})
	})

	return r
}

// createTestAuthToken creates a JWT token for testing purposes

// Benchmark tests for performance monitoring
func BenchmarkGameAPI_CreateGame(b *testing.B) {
	testDB := core.NewTestDatabase(b)
	defer testDB.Close()
	defer testDB.CleanupTables(b, "game_applications", "game_participants", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)

	router := setupGameTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(b)
	token, _ := core.CreateTestJWTTokenForUser(app, fixtures.TestUser)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		gameData := CreateGameRequest{
			Title:       "Benchmark Game " + strconv.Itoa(i),
			Description: "A game created during benchmark testing",
			Genre:       "Test",
			MaxPlayers:  4,
		}

		payload, _ := json.Marshal(gameData)
		req := httptest.NewRequest("POST", "/api/v1/games", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != 201 {
			b.Fatalf("Game creation failed with status %d", w.Code)
		}
	}
}

func BenchmarkGameAPI_GetAllGames(b *testing.B) {
	testDB := core.NewTestDatabase(b)
	defer testDB.Close()
	defer testDB.CleanupTables(b, "game_applications", "game_participants", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)

	router := setupGameTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(b)

	// Create auth token for test user
	accessToken, err := core.CreateTestJWTTokenForUser(app, fixtures.TestUser)
	if err != nil {
		b.Fatalf("Test token creation should succeed: %v", err)
	}

	// Create some test games
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	for i := 0; i < 10; i++ {
		_, _ = gameService.CreateGame(context.Background(), core.CreateGameRequest{
			Title:       "Benchmark Game " + strconv.Itoa(i),
			Description: "A game for benchmark testing",
			GMUserID:    int32(fixtures.TestUser.ID),
			IsPublic:    true,
		})
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/api/v1/games/public", nil)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != 200 {
			b.Fatalf("Get all games failed with status %d", w.Code)
		}
	}
}

// TestGameAPI_GameApplications tests the game application workflow
func TestGameAPI_GameApplications(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	testDB.CleanupTables(t, "game_applications", "game_participants", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "game_applications", "game_participants", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	// Create GM and player users
	gmToken, err := core.CreateTestJWTTokenForUser(app, fixtures.TestUser)
	core.AssertNoError(t, err, "GM token creation should succeed")

	userService := &db.UserService{DB: testDB.Pool, Logger: app.ObsLogger}
	playerUser, err := userService.CreateUser(&core.User{
		Username: "player1",
		Password: "testpass123",
		Email:    "player1@example.com",
	})
	core.AssertNoError(t, err, "Player user creation should succeed")

	playerToken, err := core.CreateTestJWTTokenForUser(app, playerUser)
	core.AssertNoError(t, err, "Player token creation should succeed")

	// Create a recruiting game
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	game, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:       "Test Game for Applications",
		Description: "A game to test applications",
		GMUserID:    int32(fixtures.TestUser.ID),
		IsPublic:    true,
	})
	core.AssertNoError(t, err, "Game creation should succeed")

	// Update game state to recruitment
	_, err = gameService.UpdateGameState(context.Background(), game.ID, "recruitment")
	core.AssertNoError(t, err, "Game state update should succeed")

	var applicationID int32

	t.Run("apply_to_game_success", func(t *testing.T) {
		payload := map[string]string{
			"role":    "player",
			"message": "I'd love to join your game!",
		}
		payloadBytes, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/api/v1/games/"+strconv.Itoa(int(game.ID))+"/apply", bytes.NewBuffer(payloadBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+playerToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 201, w.Code, "Should return 201 Created")

		var response GameApplicationResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")

		applicationID = response.ID
		core.AssertEqual(t, game.ID, response.GameID, "Game ID should match")
		core.AssertEqual(t, int32(playerUser.ID), response.UserID, "User ID should match")
		core.AssertEqual(t, "player", response.Role, "Role should be player")
		core.AssertEqual(t, "pending", response.Status, "Status should be pending")
		core.AssertEqual(t, "I'd love to join your game!", response.Message, "Message should match")
	})

	t.Run("apply_to_game_duplicate", func(t *testing.T) {
		payload := map[string]string{
			"role": "player",
		}
		payloadBytes, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/api/v1/games/"+strconv.Itoa(int(game.ID))+"/apply", bytes.NewBuffer(payloadBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+playerToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 400, w.Code, "Should return 400 Bad Request for duplicate application")
	})

	t.Run("apply_to_game_invalid_role", func(t *testing.T) {
		payload := map[string]string{
			"role": "invalid_role",
		}
		payloadBytes, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/api/v1/games/"+strconv.Itoa(int(game.ID))+"/apply", bytes.NewBuffer(payloadBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+playerToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 400, w.Code, "Should return 400 Bad Request for invalid role")
	})

	t.Run("apply_to_game_unauthorized", func(t *testing.T) {
		payload := map[string]string{
			"role": "player",
		}
		payloadBytes, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/api/v1/games/"+strconv.Itoa(int(game.ID))+"/apply", bytes.NewBuffer(payloadBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 401, w.Code, "Should return 401 Unauthorized")
	})

	t.Run("get_game_applications_as_gm", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/games/"+strconv.Itoa(int(game.ID))+"/applications", nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "Should return 200 OK")

		var response []map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")

		core.AssertTrue(t, len(response) > 0, "Should have at least one application")
		core.AssertEqual(t, float64(applicationID), response[0]["id"].(float64), "Application ID should match")
		core.AssertEqual(t, "player", response[0]["role"].(string), "Role should be player")
	})

	t.Run("get_game_applications_as_non_gm", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/games/"+strconv.Itoa(int(game.ID))+"/applications", nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 403, w.Code, "Should return 403 Forbidden for non-GM")
	})

	t.Run("get_game_applications_unauthorized", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/games/"+strconv.Itoa(int(game.ID))+"/applications", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 401, w.Code, "Should return 401 Unauthorized")
	})

	t.Run("review_application_approve", func(t *testing.T) {
		payload := map[string]string{
			"action": "approve",
		}
		payloadBytes, _ := json.Marshal(payload)

		req := httptest.NewRequest("PUT", "/api/v1/games/"+strconv.Itoa(int(game.ID))+"/applications/"+strconv.Itoa(int(applicationID))+"/review", bytes.NewBuffer(payloadBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "Should return 200 OK")

		var response GameApplicationResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")

		core.AssertEqual(t, "approved", response.Status, "Status should be approved")
		core.AssertNotEqual(t, nil, response.ReviewedAt, "Should have reviewed_at timestamp")
		core.AssertNotEqual(t, nil, response.ReviewedByUserID, "Should have reviewed_by_user_id")
	})

	t.Run("review_application_as_non_gm", func(t *testing.T) {
		// Create another application to test rejection
		player2, err := userService.CreateUser(&core.User{
			Username: "player2",
			Password: "testpass123",
			Email:    "player2@example.com",
		})
		core.AssertNoError(t, err, "Player 2 creation should succeed")

		player2Token, err := core.CreateTestJWTTokenForUser(app, player2)
		core.AssertNoError(t, err, "Player 2 token creation should succeed")

		// Apply as player2
		applyPayload := map[string]string{
			"role": "player",
		}
		applyBytes, _ := json.Marshal(applyPayload)
		applyReq := httptest.NewRequest("POST", "/api/v1/games/"+strconv.Itoa(int(game.ID))+"/apply", bytes.NewBuffer(applyBytes))
		applyReq.Header.Set("Content-Type", "application/json")
		applyReq.Header.Set("Authorization", "Bearer "+player2Token)
		applyW := httptest.NewRecorder()
		router.ServeHTTP(applyW, applyReq)

		var applyResponse GameApplicationResponse
		json.Unmarshal(applyW.Body.Bytes(), &applyResponse)

		// Try to review as player (not GM)
		payload := map[string]string{
			"action": "approve",
		}
		payloadBytes, _ := json.Marshal(payload)

		req := httptest.NewRequest("PUT", "/api/v1/games/"+strconv.Itoa(int(game.ID))+"/applications/"+strconv.Itoa(int(applyResponse.ID))+"/review", bytes.NewBuffer(payloadBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+playerToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 403, w.Code, "Should return 403 Forbidden for non-GM")
	})

	t.Run("review_application_reject", func(t *testing.T) {
		// Create another user and application for rejection test
		player3, err := userService.CreateUser(&core.User{
			Username: "player3",
			Password: "testpass123",
			Email:    "player3@example.com",
		})
		core.AssertNoError(t, err, "Player 3 creation should succeed")

		player3Token, err := core.CreateTestJWTTokenForUser(app, player3)
		core.AssertNoError(t, err, "Player 3 token creation should succeed")

		// Apply as player3
		applyPayload := map[string]string{
			"role": "player",
		}
		applyBytes, _ := json.Marshal(applyPayload)
		applyReq := httptest.NewRequest("POST", "/api/v1/games/"+strconv.Itoa(int(game.ID))+"/apply", bytes.NewBuffer(applyBytes))
		applyReq.Header.Set("Content-Type", "application/json")
		applyReq.Header.Set("Authorization", "Bearer "+player3Token)
		applyW := httptest.NewRecorder()
		router.ServeHTTP(applyW, applyReq)

		var applyResponse GameApplicationResponse
		json.Unmarshal(applyW.Body.Bytes(), &applyResponse)

		// Reject the application
		payload := map[string]string{
			"action": "reject",
		}
		payloadBytes, _ := json.Marshal(payload)

		req := httptest.NewRequest("PUT", "/api/v1/games/"+strconv.Itoa(int(game.ID))+"/applications/"+strconv.Itoa(int(applyResponse.ID))+"/review", bytes.NewBuffer(payloadBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "Should return 200 OK")

		var response GameApplicationResponse
		err = json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")

		core.AssertEqual(t, "rejected", response.Status, "Status should be rejected")
	})

	t.Run("review_application_invalid_action", func(t *testing.T) {
		payload := map[string]string{
			"action": "invalid_action",
		}
		payloadBytes, _ := json.Marshal(payload)

		req := httptest.NewRequest("PUT", "/api/v1/games/"+strconv.Itoa(int(game.ID))+"/applications/"+strconv.Itoa(int(applicationID))+"/review", bytes.NewBuffer(payloadBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 400, w.Code, "Should return 400 Bad Request for invalid action")
	})
}

// TestGameAPI_AudienceManagement tests the audience membership workflow
func TestGameAPI_AudienceManagement(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	testDB.CleanupTables(t, "game_participants", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "game_participants", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	// Create GM token
	gmToken, err := core.CreateTestJWTTokenForUser(app, fixtures.TestUser)
	core.AssertNoError(t, err, "GM token creation should succeed")

	// Create audience user
	userService := &db.UserService{DB: testDB.Pool, Logger: app.ObsLogger}
	audienceUser, err := userService.CreateUser(&core.User{
		Username: "audience1",
		Password: "testpass123",
		Email:    "audience1@example.com",
	})
	core.AssertNoError(t, err, "Audience user creation should succeed")

	audienceToken, err := core.CreateTestJWTTokenForUser(app, audienceUser)
	core.AssertNoError(t, err, "Audience token creation should succeed")

	// Create a game
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	game, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:       "Test Game for Audience",
		Description: "A game to test audience features",
		GMUserID:    int32(fixtures.TestUser.ID),
		IsPublic:    true,
	})
	core.AssertNoError(t, err, "Game creation should succeed")

	// Enable auto-accept audience
	err = gameService.UpdateGameAutoAcceptAudience(context.Background(), game.ID, true)
	core.AssertNoError(t, err, "Auto-accept audience update should succeed")

	// Set game to in_progress (bypassing transition validation — state is test setup, not subject of test)
	testDB.SetGameStateDirectly(t, game.ID, "in_progress")

	t.Run("apply_as_audience_success", func(t *testing.T) {
		payload := ApplyToGameRequest{
			Role:    "audience",
			Message: "I would love to watch this game!",
		}
		payloadBytes, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/api/v1/games/"+strconv.Itoa(int(game.ID))+"/apply", bytes.NewBuffer(payloadBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+audienceToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 201, w.Code, "Should return 201 Created")

		var response GameApplicationResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")

		core.AssertEqual(t, game.ID, response.GameID, "Game ID should match")
		core.AssertEqual(t, int32(audienceUser.ID), response.UserID, "User ID should match")
		core.AssertEqual(t, "audience", response.Role, "Role should be audience")
	})

	t.Run("apply_as_audience_duplicate", func(t *testing.T) {
		payload := ApplyToGameRequest{
			Role:    "audience",
			Message: "I would love to watch this game!",
		}
		payloadBytes, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/api/v1/games/"+strconv.Itoa(int(game.ID))+"/apply", bytes.NewBuffer(payloadBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+audienceToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Handler now returns 400 for duplicate applications
		core.AssertEqual(t, 400, w.Code, "Should return 400 Bad Request for duplicate application")
	})

	t.Run("apply_as_audience_optional_message", func(t *testing.T) {
		// Create another user for this test
		user2, err := userService.CreateUser(&core.User{
			Username: "audience2",
			Password: "testpass123",
			Email:    "audience2@example.com",
		})
		core.AssertNoError(t, err, "User creation should succeed")

		token2, err := core.CreateTestJWTTokenForUser(app, user2)
		core.AssertNoError(t, err, "Token creation should succeed")

		// Message is optional for audience applications
		payload := ApplyToGameRequest{
			Role:    "audience",
			Message: "", // Empty message is allowed
		}
		payloadBytes, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/api/v1/games/"+strconv.Itoa(int(game.ID))+"/apply", bytes.NewBuffer(payloadBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token2)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 201, w.Code, "Should return 201 Created - message is optional")
	})

	t.Run("apply_as_audience_unauthorized", func(t *testing.T) {
		payload := ApplyToGameRequest{
			Role:    "audience",
			Message: "I would love to watch this game!",
		}
		payloadBytes, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/api/v1/games/"+strconv.Itoa(int(game.ID))+"/apply", bytes.NewBuffer(payloadBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 401, w.Code, "Should return 401 Unauthorized")
	})

	t.Run("list_audience_members_success", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/games/"+strconv.Itoa(int(game.ID))+"/audience", nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "Should return 200 OK")

		var response ListAudienceMembersResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")

		core.AssertTrue(t, len(response.AudienceMembers) > 0, "Should have at least one audience member")
		core.AssertEqual(t, int32(audienceUser.ID), response.AudienceMembers[0].UserID, "User ID should match")
		core.AssertEqual(t, "audience", response.AudienceMembers[0].Role, "Role should be audience")
	})

	t.Run("list_audience_members_as_audience", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/games/"+strconv.Itoa(int(game.ID))+"/audience", nil)
		req.Header.Set("Authorization", "Bearer "+audienceToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "Should return 200 OK - audience can view audience list")
	})

	t.Run("list_audience_members_unauthorized", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/games/"+strconv.Itoa(int(game.ID))+"/audience", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 401, w.Code, "Should return 401 Unauthorized")
	})

	t.Run("update_auto_accept_audience_success", func(t *testing.T) {
		payload := map[string]bool{
			"auto_accept_audience": false,
		}
		payloadBytes, _ := json.Marshal(payload)

		req := httptest.NewRequest("PUT", "/api/v1/games/"+strconv.Itoa(int(game.ID))+"/settings/auto-accept-audience", bytes.NewBuffer(payloadBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "Should return 200 OK")

		var response map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")
		core.AssertNotEqual(t, "", response["message"], "Should have message")
	})

	t.Run("update_auto_accept_audience_as_non_gm", func(t *testing.T) {
		payload := map[string]bool{
			"auto_accept_audience": true,
		}
		payloadBytes, _ := json.Marshal(payload)

		req := httptest.NewRequest("PUT", "/api/v1/games/"+strconv.Itoa(int(game.ID))+"/settings/auto-accept-audience", bytes.NewBuffer(payloadBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+audienceToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 403, w.Code, "Should return 403 Forbidden for non-GM")
	})

	t.Run("update_auto_accept_audience_unauthorized", func(t *testing.T) {
		payload := map[string]bool{
			"auto_accept_audience": true,
		}
		payloadBytes, _ := json.Marshal(payload)

		req := httptest.NewRequest("PUT", "/api/v1/games/"+strconv.Itoa(int(game.ID))+"/settings/auto-accept-audience", bytes.NewBuffer(payloadBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 401, w.Code, "Should return 401 Unauthorized")
	})

	t.Run("apply_as_audience_during_character_creation", func(t *testing.T) {
		// Create a new game in character_creation state
		charCreationGame, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
			Title:       "Test Game Character Creation",
			Description: "A game to test audience joining during character creation",
			GMUserID:    int32(fixtures.TestUser.ID),
			IsPublic:    true,
		})
		core.AssertNoError(t, err, "Game creation should succeed")

		// Enable auto-accept audience
		err = gameService.UpdateGameAutoAcceptAudience(context.Background(), charCreationGame.ID, true)
		core.AssertNoError(t, err, "Auto-accept audience update should succeed")

		// Transition game to character_creation state
		_, err = gameService.UpdateGameState(context.Background(), charCreationGame.ID, "recruitment")
		core.AssertNoError(t, err, "Game state update to recruitment should succeed")

		_, err = gameService.UpdateGameState(context.Background(), charCreationGame.ID, "character_creation")
		core.AssertNoError(t, err, "Game state update to character_creation should succeed")

		// Create a new user to apply as audience
		charCreationAudienceUser, err := userService.CreateUser(&core.User{
			Username: "charaudience",
			Password: "testpass123",
			Email:    "charaudience@example.com",
		})
		core.AssertNoError(t, err, "User creation should succeed")

		charCreationAudienceToken, err := core.CreateTestJWTTokenForUser(app, charCreationAudienceUser)
		core.AssertNoError(t, err, "Token creation should succeed")

		// Apply as audience during character_creation state
		payload := ApplyToGameRequest{
			Role:    "audience",
			Message: "I would love to watch character creation!",
		}
		payloadBytes, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/api/v1/games/"+strconv.Itoa(int(charCreationGame.ID))+"/apply", bytes.NewBuffer(payloadBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+charCreationAudienceToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 201, w.Code, "Should return 201 Created - audience can join during character_creation")

		var response GameApplicationResponse
		err = json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")

		core.AssertEqual(t, charCreationGame.ID, response.GameID, "Game ID should match")
		core.AssertEqual(t, int32(charCreationAudienceUser.ID), response.UserID, "User ID should match")
		core.AssertEqual(t, "audience", response.Role, "Role should be audience")
	})
}

// TestGetGameParticipants_IncludesAvatarUrl verifies that the GetGameParticipants
// endpoint includes the avatar_url field in its response for both users with
// and without avatars. This is a regression test for a bug where avatar_url
// was being fetched from the database but not included in the API response.
func TestGetGameParticipants_IncludesAvatarUrl(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	// Clean up before and after
	testDB.CleanupTables(t, "game_participants", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "game_participants", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)

	// Create GM user
	userService := &db.UserService{DB: testDB.Pool, Logger: app.ObsLogger}
	gmUser, err := userService.CreateUser(&core.User{
		Username: "testgm",
		Password: "testpass123",
		Email:    "testgm@example.com",
	})
	core.AssertNoError(t, err, "GM user creation should succeed")

	// Create user WITH avatar
	userWithAvatar, err := userService.CreateUser(&core.User{
		Username: "userwithavatar",
		Password: "testpass123",
		Email:    "withavatar@example.com",
	})
	core.AssertNoError(t, err, "User with avatar creation should succeed")

	// Update user to have an avatar URL
	_, err = testDB.Pool.Exec(context.Background(),
		"UPDATE users SET avatar_url = $1 WHERE id = $2",
		"https://example.com/avatars/user123.jpg", userWithAvatar.ID)
	core.AssertNoError(t, err, "Avatar URL update should succeed")

	// Create user WITHOUT avatar
	userWithoutAvatar, err := userService.CreateUser(&core.User{
		Username: "usernoavatar",
		Password: "testpass123",
		Email:    "noavatar@example.com",
	})
	core.AssertNoError(t, err, "User without avatar creation should succeed")

	// Create a game
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	game, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:       "Avatar Test Game",
		Description: "Testing avatar URLs in participants",
		GMUserID:    int32(gmUser.ID),
	})
	core.AssertNoError(t, err, "Game creation should succeed")

	// Add both users as participants
	_, err = gameService.AddParticipantWithRole(context.Background(), game.ID, int32(userWithAvatar.ID), "player")
	core.AssertNoError(t, err, "Adding user with avatar should succeed")

	_, err = gameService.AddParticipantWithRole(context.Background(), game.ID, int32(userWithoutAvatar.ID), "player")
	core.AssertNoError(t, err, "Adding user without avatar should succeed")

	// Create auth token for GM
	accessToken, err := core.CreateTestJWTTokenForUser(app, gmUser)
	core.AssertNoError(t, err, "Test token creation should succeed")

	// Make API request to get game participants
	req := httptest.NewRequest("GET", "/api/v1/games/"+strconv.Itoa(int(game.ID))+"/participants", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	core.AssertEqual(t, 200, w.Code, "Get game participants should succeed")

	// Parse response
	var participants []map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &participants)
	core.AssertNoError(t, err, "Response should be valid JSON")

	// Verify we have participants (should be 3: GM + 2 players)
	if len(participants) < 2 {
		t.Fatalf("Should have at least 2 participants, got %d", len(participants))
	}

	// Find and verify user with avatar
	foundWithAvatar := false
	for _, p := range participants {
		userID, ok := p["user_id"].(float64)
		if !ok {
			continue
		}
		if int32(userID) == int32(userWithAvatar.ID) {
			foundWithAvatar = true

			// CRITICAL: Verify avatar_url field exists and has correct value
			avatarURL, exists := p["avatar_url"]
			if !exists {
				t.Errorf("avatar_url field should exist in response")
			}
			core.AssertEqual(t, "https://example.com/avatars/user123.jpg", avatarURL,
				"avatar_url should match the value in database")
			break
		}
	}
	if !foundWithAvatar {
		t.Errorf("User with avatar should be in participants list")
	}

	// Find and verify user without avatar
	foundWithoutAvatar := false
	for _, p := range participants {
		userID, ok := p["user_id"].(float64)
		if !ok {
			continue
		}
		if int32(userID) == int32(userWithoutAvatar.ID) {
			foundWithoutAvatar = true

			// CRITICAL: Verify avatar_url field exists and is nil
			avatarURL, exists := p["avatar_url"]
			if !exists {
				t.Errorf("avatar_url field should exist in response even when nil")
			}
			if avatarURL != nil {
				t.Errorf("avatar_url should be nil for users without avatars, got %v", avatarURL)
			}
			break
		}
	}
	if !foundWithoutAvatar {
		t.Errorf("User without avatar should be in participants list")
	}
}
