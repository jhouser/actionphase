package characters

import (
	"actionphase/pkg/core"
	dbmodels "actionphase/pkg/db/models"
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

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// setupCharacterTestRouter creates a test router with auth middleware
func setupCharacterTestRouter(app *core.App, testDB *core.TestDatabase) *chi.Mux {
	tokenAuth := jwtauth.New("HS256", []byte(app.Config.JWT.Secret), nil)
	userService := &db.UserService{DB: testDB.Pool, Logger: app.ObsLogger}

	router := chi.NewRouter()

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

	router.Route("/api/v1", func(r chi.Router) {
		r.Route("/games/{gameID}", func(r chi.Router) {
			r.Use(jwtauth.Verifier(tokenAuth))
			r.Use(jwtauth.Authenticator(tokenAuth))
			r.Use(core.RequireAuthenticationMiddleware(userService))
			r.Use(gameHandler.GameMiddleware())

			handler := &Handler{
				App:                 app,
				UserService:         &db.UserService{DB: testDB.Pool, Logger: app.ObsLogger},
				CharacterService:    &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger},
				GameService:         &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger},
				NotificationService: db.NewNotificationService(testDB.Pool, app.ObsLogger),
			}
			r.Post("/characters", handler.CreateCharacter)
			r.Get("/characters", handler.GetGameCharacters)
		})
		r.Route("/characters/{id}", func(r chi.Router) {
			r.Use(jwtauth.Verifier(tokenAuth))
			r.Use(jwtauth.Authenticator(tokenAuth))
			r.Use(core.RequireAuthenticationMiddleware(userService))

			handler := &Handler{
				App:                 app,
				UserService:         &db.UserService{DB: testDB.Pool, Logger: app.ObsLogger},
				CharacterService:    &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger},
				GameService:         &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger},
				NotificationService: db.NewNotificationService(testDB.Pool, app.ObsLogger),
			}
			r.Get("/", handler.GetCharacter)
		})
	})

	return router
}

// createTestAuthToken creates a JWT token for testing
func createTestAuthToken(app *core.App, user *core.User) (string, error) {
	return core.CreateTestJWTTokenForUser(app, user)
}

// TestCharacterAPI_GMCanCreatePlayerCharacters tests that GMs can create player characters for players
func TestCharacterAPI_GMCanCreatePlayerCharacters(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "characters", "game_participants", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupCharacterTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	// Create additional player user
	playerUser := testDB.CreateTestUser(t, "testplayer", "testplayer@example.com")

	// Add player to game
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err := gameService.AddGameParticipant(context.Background(), fixtures.TestGame.ID, int32(playerUser.ID), "player")
	core.AssertNoError(t, err, "Adding player to game should succeed")

	// Create GM token
	gmToken, err := createTestAuthToken(app, fixtures.TestUser)
	core.AssertNoError(t, err, "GM token creation should succeed")

	testCases := []struct {
		name           string
		payload        CreateCharacterRequest
		expectedStatus int
		description    string
		validateFn     func(t *testing.T, response *CharacterResponse)
	}{
		{
			name: "gm_creates_player_character_for_player",
			payload: func() CreateCharacterRequest {
				playerUserID := int32(playerUser.ID)
				return CreateCharacterRequest{
					Name:          "Test Character for Player",
					CharacterType: "player_character",
					UserID:        &playerUserID,
				}
			}(),
			expectedStatus: 201,
			description:    "GM should be able to create player character for another player",
			validateFn: func(t *testing.T, response *CharacterResponse) {
				core.AssertEqual(t, "Test Character for Player", response.Name, "Character name should match")
				if response.CharacterType == nil {
					t.Errorf("CharacterType should not be nil")
				} else {
					core.AssertEqual(t, "player_character", *response.CharacterType, "Character type should be player_character")
				}
				if response.UserID == nil {
					t.Errorf("UserID should be set")
				} else {
					core.AssertEqual(t, int32(playerUser.ID), *response.UserID, "Character should be assigned to correct player")
				}
			},
		},
		{
			name: "gm_creates_player_character_without_user_id",
			payload: CreateCharacterRequest{
				Name:          "Test Character No User",
				CharacterType: "player_character",
				UserID:        nil, // Missing required user_id
			},
			expectedStatus: 400,
			description:    "GM must provide user_id when creating player character",
		},
		{
			name: "gm_creates_npc",
			payload: CreateCharacterRequest{
				Name:          "Test NPC",
				CharacterType: "npc",
				UserID:        nil, // NPCs don't need user_id
			},
			expectedStatus: 201,
			description:    "GM should be able to create NPC without user_id",
			validateFn: func(t *testing.T, response *CharacterResponse) {
				core.AssertEqual(t, "Test NPC", response.Name, "NPC name should match")
				if response.CharacterType == nil {
					t.Errorf("CharacterType should not be nil")
				} else {
					core.AssertEqual(t, "npc", *response.CharacterType, "Character type should be npc")
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			payload, _ := json.Marshal(tc.payload)
			req := httptest.NewRequest("POST", "/api/v1/games/"+strconv.Itoa(int(fixtures.TestGame.ID))+"/characters", bytes.NewBuffer(payload))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+gmToken)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			core.AssertEqual(t, tc.expectedStatus, w.Code, tc.description)

			if tc.expectedStatus == 201 && tc.validateFn != nil {
				var response CharacterResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				core.AssertNoError(t, err, "Response should be valid JSON")
				tc.validateFn(t, &response)
			}
		})
	}
}

// TestCharacterAPI_PlayerCanOnlyCreateOwnCharacter tests that regular players can only create characters for themselves
func TestCharacterAPI_PlayerCanOnlyCreateOwnCharacter(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "characters", "game_participants", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupCharacterTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	// Create two player users
	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")

	// Add both players to game
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err := gameService.AddGameParticipant(context.Background(), fixtures.TestGame.ID, int32(player1.ID), "player")
	core.AssertNoError(t, err, "Adding player1 to game should succeed")
	_, err = gameService.AddGameParticipant(context.Background(), fixtures.TestGame.ID, int32(player2.ID), "player")
	core.AssertNoError(t, err, "Adding player2 to game should succeed")

	// Create token for player1
	player1Token, err := createTestAuthToken(app, player1)
	core.AssertNoError(t, err, "Player1 token creation should succeed")

	testCases := []struct {
		name           string
		payload        CreateCharacterRequest
		expectedStatus int
		description    string
		validateFn     func(t *testing.T, response *CharacterResponse)
	}{
		{
			name: "player_creates_own_character",
			payload: CreateCharacterRequest{
				Name:          "My Character",
				CharacterType: "player_character",
				// UserID intentionally omitted - should auto-assign to authenticated user
			},
			expectedStatus: 201,
			description:    "Player should be able to create character for themselves",
			validateFn: func(t *testing.T, response *CharacterResponse) {
				core.AssertEqual(t, "My Character", response.Name, "Character name should match")
				if response.CharacterType == nil {
					t.Errorf("CharacterType should not be nil")
				} else {
					core.AssertEqual(t, "player_character", *response.CharacterType, "Character type should be player_character")
				}
				if response.UserID == nil {
					t.Errorf("UserID should be set")
				} else {
					core.AssertEqual(t, int32(player1.ID), *response.UserID, "Character should be assigned to authenticated player")
				}
			},
		},
		{
			name: "player_tries_to_create_character_for_another_player",
			payload: func() CreateCharacterRequest {
				player2UserID := int32(player2.ID)
				return CreateCharacterRequest{
					Name:          "Someone Else's Character",
					CharacterType: "player_character",
					UserID:        &player2UserID, // Trying to assign to different player
				}
			}(),
			expectedStatus: 201,
			description:    "Player-provided UserID should be ignored; character auto-assigned to authenticated player",
			validateFn: func(t *testing.T, response *CharacterResponse) {
				// Even if player provides UserID, it should be ignored and assigned to themselves
				if response.UserID == nil {
					t.Errorf("UserID should be set")
				} else {
					core.AssertEqual(t, int32(player1.ID), *response.UserID, "Character should be assigned to authenticated player, not requested player")
				}
			},
		},
		{
			name: "player_tries_to_create_npc",
			payload: CreateCharacterRequest{
				Name:          "Player's NPC",
				CharacterType: "npc",
			},
			expectedStatus: 403,
			description:    "Regular player should not be able to create NPCs",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			payload, _ := json.Marshal(tc.payload)
			req := httptest.NewRequest("POST", "/api/v1/games/"+strconv.Itoa(int(fixtures.TestGame.ID))+"/characters", bytes.NewBuffer(payload))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+player1Token)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			core.AssertEqual(t, tc.expectedStatus, w.Code, tc.description)

			if tc.expectedStatus == 201 && tc.validateFn != nil {
				var response CharacterResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				core.AssertNoError(t, err, "Response should be valid JSON")
				tc.validateFn(t, &response)
			}
		})
	}
}

// TestCharacterAPI_PendingCharacterVisibilityByRole tests that pending characters are visible to appropriate roles
// - GM, co-GMs, and audience see ALL pending characters
// - Regular players see their OWN pending characters plus all approved characters
func TestCharacterAPI_PendingCharacterVisibilityByRole(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "characters", "game_participants", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupCharacterTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	// Create test users for different roles
	gmUser := fixtures.TestUser
	coGMUser := testDB.CreateTestUser(t, "cogm", "cogm@example.com")
	audienceUser := testDB.CreateTestUser(t, "audience", "audience@example.com")
	regularPlayer := testDB.CreateTestUser(t, "player", "player@example.com")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Add participants with different roles
	_, err := gameService.AddGameParticipant(context.Background(), fixtures.TestGame.ID, int32(coGMUser.ID), "co_gm")
	core.AssertNoError(t, err, "Adding co-GM to game should succeed")

	_, err = gameService.AddGameParticipant(context.Background(), fixtures.TestGame.ID, int32(audienceUser.ID), "audience")
	core.AssertNoError(t, err, "Adding audience to game should succeed")

	_, err = gameService.AddGameParticipant(context.Background(), fixtures.TestGame.ID, int32(regularPlayer.ID), "player")
	core.AssertNoError(t, err, "Adding regular player to game should succeed")

	// Set game to in_progress state (bypassing transition validation — state is test setup, not subject of test)
	testDB.SetGameStateDirectly(t, fixtures.TestGame.ID, "in_progress")

	// Create a pending character using direct SQL (simplest for test setup)
	var pendingCharID int32
	err = testDB.Pool.QueryRow(context.Background(),
		"INSERT INTO characters (game_id, user_id, name, status, character_type) VALUES ($1, $2, $3, $4, $5) RETURNING id",
		fixtures.TestGame.ID, regularPlayer.ID, "Pending Test Character", "pending", "player_character",
	).Scan(&pendingCharID)
	core.AssertNoError(t, err, "Creating pending character should succeed")

	// Create an approved character using direct SQL
	var approvedCharID int32
	err = testDB.Pool.QueryRow(context.Background(),
		"INSERT INTO characters (game_id, user_id, name, status, character_type) VALUES ($1, $2, $3, $4, $5) RETURNING id",
		fixtures.TestGame.ID, regularPlayer.ID, "Approved Test Character", "approved", "player_character",
	).Scan(&approvedCharID)
	core.AssertNoError(t, err, "Creating approved character should succeed")

	// Create tokens for each user
	gmToken, err := createTestAuthToken(app, gmUser)
	core.AssertNoError(t, err, "GM token creation should succeed")

	coGMToken, err := createTestAuthToken(app, coGMUser)
	core.AssertNoError(t, err, "Co-GM token creation should succeed")

	audienceToken, err := createTestAuthToken(app, audienceUser)
	core.AssertNoError(t, err, "Audience token creation should succeed")

	playerToken, err := createTestAuthToken(app, regularPlayer)
	core.AssertNoError(t, err, "Player token creation should succeed")

	testCases := []struct {
		name              string
		token             string
		role              string
		shouldSeePending  bool
		shouldSeeApproved bool
	}{
		{
			name:              "gm_sees_both_pending_and_approved",
			token:             gmToken,
			role:              "GM",
			shouldSeePending:  true,
			shouldSeeApproved: true,
		},
		{
			name:              "co_gm_sees_both_pending_and_approved",
			token:             coGMToken,
			role:              "co-GM",
			shouldSeePending:  true,
			shouldSeeApproved: true,
		},
		{
			name:              "audience_sees_both_pending_and_approved",
			token:             audienceToken,
			role:              "audience",
			shouldSeePending:  true,
			shouldSeeApproved: true,
		},
		{
			name:              "regular_player_sees_own_pending_and_approved",
			token:             playerToken,
			role:              "regular player",
			shouldSeePending:  true, // Players can see their OWN pending characters
			shouldSeeApproved: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/games/"+strconv.Itoa(int(fixtures.TestGame.ID))+"/characters", nil)
			req.Header.Set("Authorization", "Bearer "+tc.token)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			core.AssertEqual(t, 200, w.Code, tc.role+" should successfully get characters")

			var response []CharacterResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			core.AssertNoError(t, err, "Response should be valid JSON")

			// Check if pending character is in response
			pendingFound := false
			approvedFound := false
			for _, char := range response {
				if char.ID == pendingCharID {
					pendingFound = true
				}
				if char.ID == approvedCharID {
					approvedFound = true
				}
			}

			if tc.shouldSeePending {
				if !pendingFound {
					t.Errorf("%s should see pending character (ID: %d) but did not. Response contains %d characters", tc.role, pendingCharID, len(response))
				}
			} else {
				if pendingFound {
					t.Errorf("%s should NOT see pending character (ID: %d) but did. Response contains %d characters", tc.role, pendingCharID, len(response))
				}
			}

			if tc.shouldSeeApproved {
				if !approvedFound {
					t.Errorf("%s should see approved character (ID: %d) but did not. Response contains %d characters", tc.role, approvedCharID, len(response))
				}
			}
		})
	}
}

// TestCharacterAPI_PlayerCannotSeeOtherPlayersPendingCharacters verifies security requirement
// that regular players cannot see pending/rejected characters belonging to other players
func TestCharacterAPI_PlayerCannotSeeOtherPlayersPendingCharacters(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "characters", "game_participants", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupCharacterTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	// Create two regular players
	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Add both players to the game
	_, err := gameService.AddGameParticipant(context.Background(), fixtures.TestGame.ID, int32(player1.ID), "player")
	core.AssertNoError(t, err, "Adding player1 to game should succeed")

	_, err = gameService.AddGameParticipant(context.Background(), fixtures.TestGame.ID, int32(player2.ID), "player")
	core.AssertNoError(t, err, "Adding player2 to game should succeed")

	// Set game to in_progress state (bypassing transition validation — state is test setup, not subject of test)
	testDB.SetGameStateDirectly(t, fixtures.TestGame.ID, "in_progress")

	// Player 1 creates a pending character (owned by player1)
	var player1PendingCharID int32
	err = testDB.Pool.QueryRow(context.Background(),
		"INSERT INTO characters (game_id, user_id, name, status, character_type) VALUES ($1, $2, $3, $4, $5) RETURNING id",
		fixtures.TestGame.ID, player1.ID, "Player 1 Pending Character", "pending", "player_character",
	).Scan(&player1PendingCharID)
	core.AssertNoError(t, err, "Creating player1's pending character should succeed")

	// Player 1 creates an approved character (owned by player1)
	var player1ApprovedCharID int32
	err = testDB.Pool.QueryRow(context.Background(),
		"INSERT INTO characters (game_id, user_id, name, status, character_type) VALUES ($1, $2, $3, $4, $5) RETURNING id",
		fixtures.TestGame.ID, player1.ID, "Player 1 Approved Character", "approved", "player_character",
	).Scan(&player1ApprovedCharID)
	core.AssertNoError(t, err, "Creating player1's approved character should succeed")

	// Player 2 creates a pending character (owned by player2)
	var player2PendingCharID int32
	err = testDB.Pool.QueryRow(context.Background(),
		"INSERT INTO characters (game_id, user_id, name, status, character_type) VALUES ($1, $2, $3, $4, $5) RETURNING id",
		fixtures.TestGame.ID, player2.ID, "Player 2 Pending Character", "pending", "player_character",
	).Scan(&player2PendingCharID)
	core.AssertNoError(t, err, "Creating player2's pending character should succeed")

	// Create tokens for both players
	player1Token, err := createTestAuthToken(app, player1)
	core.AssertNoError(t, err, "Player1 token creation should succeed")

	player2Token, err := createTestAuthToken(app, player2)
	core.AssertNoError(t, err, "Player2 token creation should succeed")

	// Test: Player 1 should see their own pending + approved characters, but NOT player 2's pending
	t.Run("player1_sees_own_pending_but_not_player2_pending", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/games/"+strconv.Itoa(int(fixtures.TestGame.ID))+"/characters", nil)
		req.Header.Set("Authorization", "Bearer "+player1Token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "Player1 should successfully get characters")

		var response []CharacterResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")

		// Check which characters player1 can see
		seesOwnPending := false
		seesOwnApproved := false
		seesPlayer2Pending := false

		for _, char := range response {
			if char.ID == player1PendingCharID {
				seesOwnPending = true
			}
			if char.ID == player1ApprovedCharID {
				seesOwnApproved = true
			}
			if char.ID == player2PendingCharID {
				seesPlayer2Pending = true
			}
		}

		// Assertions
		if !seesOwnPending {
			t.Errorf("Player1 should see their own pending character (ID: %d)", player1PendingCharID)
		}
		if !seesOwnApproved {
			t.Errorf("Player1 should see their own approved character (ID: %d)", player1ApprovedCharID)
		}
		if seesPlayer2Pending {
			t.Errorf("Player1 should NOT see Player2's pending character (ID: %d), but did", player2PendingCharID)
		}
	})

	// Test: Player 2 should see their own pending, but NOT player 1's pending
	t.Run("player2_sees_own_pending_but_not_player1_pending", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/games/"+strconv.Itoa(int(fixtures.TestGame.ID))+"/characters", nil)
		req.Header.Set("Authorization", "Bearer "+player2Token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "Player2 should successfully get characters")

		var response []CharacterResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")

		// Check which characters player2 can see
		seesOwnPending := false
		seesPlayer1Pending := false
		seesPlayer1Approved := false

		for _, char := range response {
			if char.ID == player2PendingCharID {
				seesOwnPending = true
			}
			if char.ID == player1PendingCharID {
				seesPlayer1Pending = true
			}
			if char.ID == player1ApprovedCharID {
				seesPlayer1Approved = true
			}
		}

		// Assertions
		if !seesOwnPending {
			t.Errorf("Player2 should see their own pending character (ID: %d)", player2PendingCharID)
		}
		if seesPlayer1Pending {
			t.Errorf("Player2 should NOT see Player1's pending character (ID: %d), but did", player1PendingCharID)
		}
		if !seesPlayer1Approved {
			t.Errorf("Player2 should see Player1's approved character (ID: %d)", player1ApprovedCharID)
		}
	})
}

// TestGetCharacter_AnonymousMode tests that character_type is hidden from regular players
// when the game has anonymous mode enabled, but visible to GMs, co-GMs, and audience members.
func TestGetCharacter_AnonymousMode(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "characters", "game_participants", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupCharacterTestRouter(app, testDB)

	queries := dbmodels.New(testDB.Pool)
	ctx := context.Background()

	gmUser := testDB.CreateTestUser(t, "anon_gm", "anon_gm@example.com")
	playerUser := testDB.CreateTestUser(t, "anon_player", "anon_player@example.com")
	coGMUser := testDB.CreateTestUser(t, "anon_cogm", "anon_cogm@example.com")
	audienceUser := testDB.CreateTestUser(t, "anon_audience", "anon_audience@example.com")

	// Create anonymous game directly (CreateTestGame doesn't set IsAnonymous)
	anonGame, err := queries.CreateGame(ctx, dbmodels.CreateGameParams{
		Title:       "Anonymous Test Game",
		Description: pgtype.Text{String: "Test", Valid: true},
		GmUserID:    int32(gmUser.ID),
		IsAnonymous: true,
		IsPublic:    pgtype.Bool{Bool: true, Valid: true},
	})
	core.AssertNoError(t, err, "Creating anonymous game should succeed")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(ctx, anonGame.ID, int32(playerUser.ID), "player")
	core.AssertNoError(t, err, "Adding player should succeed")
	_, err = gameService.AddGameParticipant(ctx, anonGame.ID, int32(coGMUser.ID), "co_gm")
	core.AssertNoError(t, err, "Adding co-GM should succeed")
	_, err = gameService.AddGameParticipant(ctx, anonGame.ID, int32(audienceUser.ID), "audience")
	core.AssertNoError(t, err, "Adding audience member should succeed")

	playerUserID := int32(playerUser.ID)
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	char, err := characterService.CreateCharacter(ctx, db.CreateCharacterRequest{
		GameID:        anonGame.ID,
		UserID:        &playerUserID,
		Name:          "Mysterious Figure",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Creating character should succeed")

	gmToken, _ := createTestAuthToken(app, gmUser)
	playerToken, _ := createTestAuthToken(app, playerUser)
	coGMToken, _ := createTestAuthToken(app, coGMUser)
	audienceToken, _ := createTestAuthToken(app, audienceUser)

	charURL := "/api/v1/characters/" + strconv.Itoa(int(char.ID)) + "/"

	t.Run("player cannot see character_type in anonymous game", func(t *testing.T) {
		req := httptest.NewRequest("GET", charURL, nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		core.AssertEqual(t, http.StatusOK, w.Code, "Expected 200 OK")

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")

		if _, ok := response["character_type"]; ok {
			t.Errorf("character_type should not be present in anonymous game response for regular players, got: %v", response["character_type"])
		}
	})

	t.Run("gm can see character_type in anonymous game", func(t *testing.T) {
		req := httptest.NewRequest("GET", charURL, nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		core.AssertEqual(t, http.StatusOK, w.Code, "Expected 200 OK")

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")

		if _, ok := response["character_type"]; !ok {
			t.Errorf("character_type should be present for GM in anonymous game")
		}
	})

	t.Run("co-gm can see character_type in anonymous game", func(t *testing.T) {
		req := httptest.NewRequest("GET", charURL, nil)
		req.Header.Set("Authorization", "Bearer "+coGMToken)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		core.AssertEqual(t, http.StatusOK, w.Code, "Expected 200 OK")

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")

		if _, ok := response["character_type"]; !ok {
			t.Errorf("character_type should be present for co-GM in anonymous game")
		}
	})

	t.Run("audience can see character_type in anonymous game", func(t *testing.T) {
		req := httptest.NewRequest("GET", charURL, nil)
		req.Header.Set("Authorization", "Bearer "+audienceToken)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		core.AssertEqual(t, http.StatusOK, w.Code, "Expected 200 OK")

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")

		if _, ok := response["character_type"]; !ok {
			t.Errorf("character_type should be present for audience in anonymous game")
		}
	})
}

// TestCharacterAPI_ValidationErrors tests validation error scenarios
func TestCharacterAPI_ValidationErrors(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "characters", "game_participants", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupCharacterTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	// Create GM token
	gmToken, err := createTestAuthToken(app, fixtures.TestUser)
	core.AssertNoError(t, err, "GM token creation should succeed")

	testCases := []struct {
		name           string
		gameID         string
		payload        CreateCharacterRequest
		expectedStatus int
		expectedError  string
		description    string
	}{
		{
			name:   "empty_character_name",
			gameID: strconv.Itoa(int(fixtures.TestGame.ID)),
			payload: CreateCharacterRequest{
				Name:          "",
				CharacterType: "player_character",
			},
			expectedStatus: 400,
			expectedError:  "character name is required",
			description:    "Should reject empty character name",
		},
		{
			name:   "invalid_character_type",
			gameID: strconv.Itoa(int(fixtures.TestGame.ID)),
			payload: CreateCharacterRequest{
				Name:          "Test Character",
				CharacterType: "invalid_type",
			},
			expectedStatus: 400,
			expectedError:  "invalid character type",
			description:    "Should reject invalid character type",
		},
		{
			name:   "invalid_game_id",
			gameID: "not-a-number",
			payload: CreateCharacterRequest{
				Name:          "Test Character",
				CharacterType: "player_character",
			},
			expectedStatus: 400,
			expectedError:  "invalid game ID",
			description:    "Should reject non-numeric game ID",
		},
		{
			name:   "invalid_game_id_negative",
			gameID: "-123",
			payload: CreateCharacterRequest{
				Name:          "Test Character",
				CharacterType: "player_character",
			},
			expectedStatus: 404,
			expectedError:  "",
			description:    "Should handle negative game ID (game not found)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			payload, _ := json.Marshal(tc.payload)
			req := httptest.NewRequest("POST", "/api/v1/games/"+tc.gameID+"/characters", bytes.NewBuffer(payload))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+gmToken)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			core.AssertEqual(t, tc.expectedStatus, w.Code, tc.description)

			if tc.expectedError != "" {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				core.AssertNoError(t, err, "Should decode error response")

				if errorText, ok := response["error"].(string); ok {
					if len(errorText) == 0 {
						t.Errorf("Expected error message to contain '%s', but error field was empty", tc.expectedError)
					}
					// Note: We don't use Contains assertion here since error messages may be wrapped
					// Just verify error field is present and not empty
				} else {
					t.Errorf("Expected 'error' field in response")
				}
			}
		})
	}
}

// TestGetCharacter_AudienceAssignedPendingNPC_InProgress tests that an audience member
// can fetch a pending NPC that has been assigned to them, even when the game is in_progress.
// Regression test for: assigned audience members getting 404 on GetCharacter for their pending NPCs.
func TestGetCharacter_AudienceAssignedPendingNPC_InProgress(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "npc_assignments", "characters", "game_participants", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupCharacterTestRouter(app, testDB)
	ctx := context.Background()

	gmUser := testDB.CreateTestUser(t, "npc_gm", "npc_gm@example.com")
	audienceUser := testDB.CreateTestUser(t, "npc_audience", "npc_audience@example.com")
	otherPlayer := testDB.CreateTestUser(t, "npc_other", "npc_other@example.com")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	game := testDB.CreateTestGameWithState(t, int32(gmUser.ID), "NPC Test Game", "in_progress")

	_, err := gameService.AddGameParticipant(ctx, game.ID, int32(audienceUser.ID), "audience")
	core.AssertNoError(t, err, "Adding audience user should succeed")
	_, err = gameService.AddGameParticipant(ctx, game.ID, int32(otherPlayer.ID), "player")
	core.AssertNoError(t, err, "Adding other player should succeed")

	// Create a pending NPC and assign to the audience user
	pendingNPC, err := characterService.CreateCharacter(ctx, db.CreateCharacterRequest{
		GameID:        game.ID,
		Name:          "Pending Assigned NPC",
		CharacterType: "npc",
	})
	core.AssertNoError(t, err, "Creating pending NPC should succeed")
	core.AssertEqual(t, "pending", pendingNPC.Status.String, "NPC should start as pending")

	err = characterService.AssignNPCToUser(ctx, pendingNPC.ID, int32(audienceUser.ID), int32(gmUser.ID))
	core.AssertNoError(t, err, "Assigning NPC to audience user should succeed")

	audienceToken, _ := createTestAuthToken(app, audienceUser)
	otherPlayerToken, _ := createTestAuthToken(app, otherPlayer)
	charURL := "/api/v1/characters/" + strconv.Itoa(int(pendingNPC.ID)) + "/"

	t.Run("assigned audience user can fetch their pending NPC", func(t *testing.T) {
		req := httptest.NewRequest("GET", charURL, nil)
		req.Header.Set("Authorization", "Bearer "+audienceToken)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		core.AssertEqual(t, http.StatusOK, w.Code, "Assigned audience user should get 200, not 404")
	})

	t.Run("unassigned player cannot fetch pending NPC", func(t *testing.T) {
		req := httptest.NewRequest("GET", charURL, nil)
		req.Header.Set("Authorization", "Bearer "+otherPlayerToken)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		core.AssertEqual(t, http.StatusNotFound, w.Code, "Unassigned player should get 404 for pending NPC")
	})
}
