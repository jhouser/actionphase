package characters

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
	services "actionphase/pkg/db/services"
	dbactions "actionphase/pkg/db/services/actions"
	dbmessages "actionphase/pkg/db/services/messages"
	"actionphase/pkg/games"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
)

func TestCharacterAPI_CompleteCharacterLifecycle(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "character_data", "npc_assignments", "characters", "game_participants", "games", "sessions", "users")

	// Setup application
	app := core.NewTestApp(testDB.Pool)

	// Create test users
	gmUser := testDB.CreateTestUser(t, "gm", "gm@example.com")
	playerUser := testDB.CreateTestUser(t, "player", "player@example.com")

	// Create test game
	gameService := &services.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	game, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:       "Character Test Game",
		Description: "Testing character functionality",
		GMUserID:    int32(gmUser.ID),
		IsPublic:    true,
	})
	core.AssertNoError(t, err, "Failed to create test game")

	// Add player as participant
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(playerUser.ID), "player")
	core.AssertNoError(t, err, "Failed to add player participant")

	// Create tokens for authentication using app config
	gmToken, err := core.CreateTestJWTTokenForUser(app, gmUser)
	core.AssertNoError(t, err, "Failed to create GM token")
	playerToken, err := core.CreateTestJWTTokenForUser(app, playerUser)
	core.AssertNoError(t, err, "Failed to create player token")

	// Setup router with character routes and JWT middleware
	tokenAuth := jwtauth.New("HS256", []byte(app.Config.JWT.Secret), nil)
	userService := &services.UserService{DB: testDB.Pool, Logger: app.ObsLogger}
	r := chi.NewRouter()
	handler := Handler{
		App:                 app,
		UserService:         &services.UserService{DB: testDB.Pool, Logger: app.ObsLogger},
		CharacterService:    &services.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger},
		GameService:         &services.GameService{DB: testDB.Pool, Logger: app.ObsLogger},
		NotificationService: services.NewNotificationService(testDB.Pool, app.ObsLogger),
	}
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

	// Character routes
	r.Route("/api/v1/games/{gameID}/characters", func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth))
		r.Use(core.RequireAuthenticationMiddleware(userService))
		r.Use(gameHandler.GameMiddleware())
		r.Post("/", handler.CreateCharacter)
		r.Get("/", handler.GetGameCharacters)
	})
	r.Route("/api/v1/characters/{id}", func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth))
		r.Use(core.RequireAuthenticationMiddleware(userService))
		r.Get("/", handler.GetCharacter)
		r.Post("/approve", handler.ApproveCharacter)
		r.Post("/assign", handler.AssignNPC)
		r.Post("/data", handler.SetCharacterData)
		r.Get("/data", handler.GetCharacterData)
	})

	var createdCharacterID int32

	t.Run("create player character", func(t *testing.T) {
		requestBody := CreateCharacterRequest{
			Name:          "Aragorn",
			CharacterType: "player_character",
		}

		body, _ := json.Marshal(requestBody)
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/characters", game.ID), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+playerToken)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		core.AssertEqual(t, http.StatusCreated, w.Code, "Expected 201 Created")

		var response CharacterResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Failed to unmarshal response")

		core.AssertEqual(t, "Aragorn", response.Name, "Character name mismatch")
		if response.CharacterType == nil {
			t.Errorf("CharacterType should not be nil")
		} else {
			core.AssertEqual(t, "player_character", *response.CharacterType, "Character type mismatch")
		}
		core.AssertEqual(t, "pending", response.Status, "Character should start as pending")

		createdCharacterID = response.ID
		t.Logf("Created character with ID: %d", createdCharacterID)
	})

	t.Run("get character", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/characters/%d", createdCharacterID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		core.AssertEqual(t, http.StatusOK, w.Code, "Expected 200 OK")

		var response CharacterResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Failed to unmarshal response")

		core.AssertEqual(t, createdCharacterID, response.ID, "Character ID mismatch")
		core.AssertEqual(t, "Aragorn", response.Name, "Character name mismatch")
	})

	t.Run("get game characters", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/characters", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		core.AssertEqual(t, http.StatusOK, w.Code, "Expected 200 OK")

		var response []map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Failed to unmarshal response")

		core.AssertEqual(t, 1, len(response), "Expected 1 character")

		// Type assert the ID field
		idField, ok := response[0]["id"].(float64)
		if !ok {
			t.Fatalf("Expected ID to be float64, got %T", response[0]["id"])
		}
		core.AssertEqual(t, float64(createdCharacterID), idField, "Character ID mismatch")
	})

	t.Run("approve character as GM", func(t *testing.T) {
		requestBody := ApproveCharacterRequest{
			Status: "approved",
		}

		body, _ := json.Marshal(requestBody)
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/characters/%d/approve", createdCharacterID), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		core.AssertEqual(t, http.StatusOK, w.Code, "Expected 200 OK")

		var response CharacterResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Failed to unmarshal response")

		core.AssertEqual(t, "approved", response.Status, "Character should be approved")
	})

	t.Run("set character data", func(t *testing.T) {
		requestBody := CharacterDataRequest{
			ModuleType: "bio",
			FieldName:  "background",
			FieldValue: "A ranger from the north, heir to the throne of Gondor",
			FieldType:  "text",
			IsPublic:   true,
		}

		body, _ := json.Marshal(requestBody)
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/characters/%d/data", createdCharacterID), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+playerToken)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		core.AssertEqual(t, http.StatusNoContent, w.Code, "Expected 204 No Content")
	})

	t.Run("get character data", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/characters/%d/data", createdCharacterID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		core.AssertEqual(t, http.StatusOK, w.Code, "Expected 200 OK")

		var response []map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Failed to unmarshal response")

		core.AssertEqual(t, 1, len(response), "Expected 1 data entry")
		core.AssertEqual(t, "bio", response[0]["module_type"], "Module type mismatch")
		core.AssertEqual(t, "background", response[0]["field_name"], "Field name mismatch")
	})
}

// TestCharacterAPI_CompletedGamePlayersCanViewPrivateData is a regression test for the bug where
// players who participated in a completed game could not view full (private) character sheet data
// for OTHER players' characters. After game completion, all participants should have audience-level
// visibility, meaning they can see private data on any character in the game.
func TestCharacterAPI_CompletedGamePlayersCanViewPrivateData(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "character_data", "npc_assignments", "characters", "game_participants", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)

	gmUser := testDB.CreateTestUser(t, "gm", "gm@example.com")
	// player1 owns the character; player2 is a fellow participant viewing it
	player1User := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2User := testDB.CreateTestUser(t, "player2", "player2@example.com")
	outsiderUser := testDB.CreateTestUser(t, "outsider", "outsider@example.com")

	gameService := &services.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	game, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:       "Completed Game",
		Description: "A finished game",
		GMUserID:    int32(gmUser.ID),
		IsPublic:    true,
	})
	core.AssertNoError(t, err, "Failed to create test game")

	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player1User.ID), "player")
	core.AssertNoError(t, err, "Failed to add player1 participant")
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player2User.ID), "player")
	core.AssertNoError(t, err, "Failed to add player2 participant")

	// Create player1's character with private data
	characterService := &services.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	player1CharID := int32(player1User.ID)
	char, err := characterService.CreateCharacter(context.Background(), services.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        &player1CharID,
		Name:          "Test Hero",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Failed to create character")
	_, err = characterService.ApproveCharacter(context.Background(), char.ID)
	core.AssertNoError(t, err, "Failed to approve character")

	// Add private (non-public) character data to player1's character
	err = characterService.SetCharacterData(context.Background(), services.CharacterDataRequest{
		CharacterID: char.ID,
		ModuleType:  "bio",
		FieldName:   "secret_notes",
		FieldValue:  "Hidden backstory",
		FieldType:   "text",
		IsPublic:    false,
	})
	core.AssertNoError(t, err, "Failed to set private character data")

	// Transition game through valid states to completed
	_, err = gameService.UpdateGameState(context.Background(), game.ID, core.GameStateRecruitment)
	core.AssertNoError(t, err, "Failed to transition to recruitment")
	_, err = gameService.UpdateGameState(context.Background(), game.ID, core.GameStateCharacterCreation)
	core.AssertNoError(t, err, "Failed to transition to character_creation")
	_, err = gameService.UpdateGameState(context.Background(), game.ID, core.GameStateInProgress)
	core.AssertNoError(t, err, "Failed to transition to in_progress")
	_, err = gameService.UpdateGameState(context.Background(), game.ID, core.GameStateCompleted)
	core.AssertNoError(t, err, "Failed to transition to completed")

	// player2 is a fellow participant (not the character owner) — this is the bug scenario
	player2Token, err := core.CreateTestJWTTokenForUser(app, player2User)
	core.AssertNoError(t, err, "Failed to create player2 token")
	outsiderToken, err := core.CreateTestJWTTokenForUser(app, outsiderUser)
	core.AssertNoError(t, err, "Failed to create outsider token")

	tokenAuth := jwtauth.New("HS256", []byte(app.Config.JWT.Secret), nil)
	userService := &services.UserService{DB: testDB.Pool, Logger: app.ObsLogger}
	r := chi.NewRouter()
	handler := Handler{
		App:                 app,
		UserService:         &services.UserService{DB: testDB.Pool, Logger: app.ObsLogger},
		CharacterService:    &services.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger},
		GameService:         &services.GameService{DB: testDB.Pool, Logger: app.ObsLogger},
		NotificationService: services.NewNotificationService(testDB.Pool, app.ObsLogger),
	}

	r.Route("/api/v1/characters/{id}", func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth))
		r.Use(core.RequireAuthenticationMiddleware(userService))
		r.Get("/data", handler.GetCharacterData)
	})

	t.Run("fellow player in completed game can view private data on another player's character", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/characters/%d/data", char.ID), nil)
		req.Header.Set("Authorization", "Bearer "+player2Token)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		core.AssertEqual(t, http.StatusOK, w.Code, "Expected 200 OK")

		var response []map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Failed to unmarshal response")

		foundPrivate := false
		for _, item := range response {
			if item["field_name"] == "secret_notes" {
				foundPrivate = true
			}
		}
		core.AssertTrue(t, foundPrivate, "Fellow player in completed game should see private character data")
	})

	t.Run("outsider not in the game only sees public data even after completion", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/characters/%d/data", char.ID), nil)
		req.Header.Set("Authorization", "Bearer "+outsiderToken)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		core.AssertEqual(t, http.StatusOK, w.Code, "Expected 200 OK")

		var response []map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Failed to unmarshal response")

		for _, item := range response {
			if item["field_name"] == "secret_notes" {
				t.Error("Outsider should not see private character data even in a completed game")
			}
		}
	})
}

func TestCharacterAPI_NPCManagement(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "character_data", "npc_assignments", "characters", "game_participants", "games", "sessions", "users")

	// Setup application
	app := core.NewTestApp(testDB.Pool)

	// Create test users
	gmUser := testDB.CreateTestUser(t, "gm", "gm@example.com")
	audienceUser := testDB.CreateTestUser(t, "audience", "audience@example.com")

	// Create test game
	gameService := &services.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	game, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:       "NPC Test Game",
		Description: "Testing NPC functionality",
		GMUserID:    int32(gmUser.ID),
		IsPublic:    true,
	})
	core.AssertNoError(t, err, "Failed to create test game")

	// Add audience member as participant
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(audienceUser.ID), "audience")
	core.AssertNoError(t, err, "Failed to add audience participant")

	// Create tokens
	gmToken, err := core.CreateTestJWTTokenForUser(app, gmUser)
	core.AssertNoError(t, err, "Failed to create GM token")

	// Setup router with JWT middleware
	tokenAuth := jwtauth.New("HS256", []byte(app.Config.JWT.Secret), nil)
	userService := &services.UserService{DB: testDB.Pool, Logger: app.ObsLogger}
	r := chi.NewRouter()
	handler := Handler{
		App:                 app,
		UserService:         &services.UserService{DB: testDB.Pool, Logger: app.ObsLogger},
		CharacterService:    &services.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger},
		GameService:         &services.GameService{DB: testDB.Pool, Logger: app.ObsLogger},
		NotificationService: services.NewNotificationService(testDB.Pool, app.ObsLogger),
	}
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

	r.Route("/api/v1/games/{gameID}/characters", func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth))
		r.Use(core.RequireAuthenticationMiddleware(userService))
		r.Use(gameHandler.GameMiddleware())
		r.Post("/", handler.CreateCharacter)
		r.Get("/", handler.GetGameCharacters)
	})
	r.Route("/api/v1/characters/{id}", func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth))
		r.Use(core.RequireAuthenticationMiddleware(userService))
		r.Get("/", handler.GetCharacter)
		r.Post("/assign", handler.AssignNPC)
	})

	var npcCharacterID int32

	t.Run("create NPC as GM", func(t *testing.T) {
		requestBody := CreateCharacterRequest{
			Name:          "Gandalf",
			CharacterType: "npc",
		}

		body, _ := json.Marshal(requestBody)
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/characters", game.ID), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		core.AssertEqual(t, http.StatusCreated, w.Code, "Expected 201 Created")

		var response CharacterResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Failed to unmarshal response")

		core.AssertEqual(t, "Gandalf", response.Name, "NPC name mismatch")
		if response.CharacterType == nil {
			t.Errorf("CharacterType should not be nil")
		} else {
			core.AssertEqual(t, "npc", *response.CharacterType, "NPC type mismatch")
		}

		npcCharacterID = response.ID
	})

	t.Run("assign NPC to audience member", func(t *testing.T) {
		requestBody := AssignNPCRequest{
			AssignedUserID: int32(audienceUser.ID),
		}

		body, _ := json.Marshal(requestBody)
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/characters/%d/assign", npcCharacterID), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		core.AssertEqual(t, http.StatusNoContent, w.Code, "Expected 204 No Content")
	})

	t.Run("verify game characters shows assigned NPC", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/characters", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		core.AssertEqual(t, http.StatusOK, w.Code, "Expected 200 OK")

		var response []map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Failed to unmarshal response")

		core.AssertEqual(t, 1, len(response), "Expected 1 character")
		core.AssertEqual(t, "Gandalf", response[0]["name"], "NPC name mismatch")
	})
}

func TestCharacterAPI_Authorization(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "character_data", "npc_assignments", "characters", "game_participants", "games", "sessions", "users")

	// Setup application
	app := core.NewTestApp(testDB.Pool)

	// Create test users
	gmUser := testDB.CreateTestUser(t, "gm", "gm@example.com")
	playerUser := testDB.CreateTestUser(t, "player", "player@example.com")
	otherUser := testDB.CreateTestUser(t, "other", "other@example.com")

	// Create test game
	gameService := &services.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	game, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:       "Authorization Test Game",
		Description: "Testing character authorization",
		GMUserID:    int32(gmUser.ID),
		IsPublic:    true,
	})
	core.AssertNoError(t, err, "Failed to create test game")

	// Add player as participant
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(playerUser.ID), "player")
	core.AssertNoError(t, err, "Failed to add player participant")

	// Create character
	characterService := &services.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	character, err := characterService.CreateCharacter(context.Background(), services.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        core.Int32Ptr(int32(playerUser.ID)),
		Name:          "Test Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Failed to create test character")

	// Create tokens
	gmToken, err := core.CreateTestJWTTokenForUser(app, gmUser)
	core.AssertNoError(t, err, "Failed to create GM token")
	playerToken, err := core.CreateTestJWTTokenForUser(app, playerUser)
	core.AssertNoError(t, err, "Failed to create player token")
	otherToken, err := core.CreateTestJWTTokenForUser(app, otherUser)
	core.AssertNoError(t, err, "Failed to create other token")

	// Setup router with JWT middleware
	tokenAuth := jwtauth.New("HS256", []byte(app.Config.JWT.Secret), nil)
	userService := &services.UserService{DB: testDB.Pool, Logger: app.ObsLogger}
	r := chi.NewRouter()
	handler := Handler{
		App:                 app,
		UserService:         &services.UserService{DB: testDB.Pool, Logger: app.ObsLogger},
		CharacterService:    &services.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger},
		GameService:         &services.GameService{DB: testDB.Pool, Logger: app.ObsLogger},
		NotificationService: services.NewNotificationService(testDB.Pool, app.ObsLogger),
	}
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

	r.Route("/api/v1/games/{gameID}/characters", func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth))
		r.Use(core.RequireAuthenticationMiddleware(userService))
		r.Use(gameHandler.GameMiddleware())
		r.Post("/", handler.CreateCharacter)
	})
	r.Route("/api/v1/characters/{id}", func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth))
		r.Use(core.RequireAuthenticationMiddleware(userService))
		r.Post("/approve", handler.ApproveCharacter)
		r.Post("/data", handler.SetCharacterData)
	})

	testCases := []struct {
		name           string
		endpoint       string
		method         string
		token          string
		expectedStatus int
		body           interface{}
		reason         string
	}{
		{
			name:           "non-participant cannot create character",
			endpoint:       fmt.Sprintf("/api/v1/games/%d/characters", game.ID),
			method:         "POST",
			token:          otherToken,
			expectedStatus: http.StatusForbidden,
			body: CreateCharacterRequest{
				Name:          "Unauthorized Character",
				CharacterType: "player_character",
			},
			reason: "non-participants should not create characters",
		},
		{
			name:           "only GM can approve characters",
			endpoint:       fmt.Sprintf("/api/v1/characters/%d/approve", character.ID),
			method:         "POST",
			token:          playerToken,
			expectedStatus: http.StatusForbidden,
			body: ApproveCharacterRequest{
				Status: "approved",
			},
			reason: "only GM should approve characters",
		},
		{
			name:           "GM can approve characters",
			endpoint:       fmt.Sprintf("/api/v1/characters/%d/approve", character.ID),
			method:         "POST",
			token:          gmToken,
			expectedStatus: http.StatusOK,
			body: ApproveCharacterRequest{
				Status: "approved",
			},
			reason: "GM should be able to approve characters",
		},
		{
			name:           "character owner can edit data",
			endpoint:       fmt.Sprintf("/api/v1/characters/%d/data", character.ID),
			method:         "POST",
			token:          playerToken,
			expectedStatus: http.StatusNoContent,
			body: CharacterDataRequest{
				ModuleType: "bio",
				FieldName:  "background",
				FieldValue: "A noble hero",
				FieldType:  "text",
				IsPublic:   true,
			},
			reason: "character owner should edit their character data",
		},
		{
			name:           "other users cannot edit character data",
			endpoint:       fmt.Sprintf("/api/v1/characters/%d/data", character.ID),
			method:         "POST",
			token:          otherToken,
			expectedStatus: http.StatusForbidden,
			body: CharacterDataRequest{
				ModuleType: "bio",
				FieldName:  "background",
				FieldValue: "Unauthorized edit",
				FieldType:  "text",
				IsPublic:   true,
			},
			reason: "other users should not edit character data",
		},
		{
			name:           "character owner cannot edit abilities (stats)",
			endpoint:       fmt.Sprintf("/api/v1/characters/%d/data", character.ID),
			method:         "POST",
			token:          playerToken,
			expectedStatus: http.StatusForbidden,
			body: CharacterDataRequest{
				ModuleType: "abilities",
				FieldName:  "abilities",
				FieldValue: `[{"id":"ability-1","name":"Fireball","description":"Cast fireball","type":"spell"}]`,
				FieldType:  "json",
				IsPublic:   true,
			},
			reason: "only GMs can edit character abilities",
		},
		{
			name:           "character owner cannot edit skills (stats)",
			endpoint:       fmt.Sprintf("/api/v1/characters/%d/data", character.ID),
			method:         "POST",
			token:          playerToken,
			expectedStatus: http.StatusForbidden,
			body: CharacterDataRequest{
				ModuleType: "skills",
				FieldName:  "skills",
				FieldValue: `[{"id":"skill-1","name":"Archery","proficiency":"expert"}]`,
				FieldType:  "json",
				IsPublic:   true,
			},
			reason: "only GMs can edit character skills",
		},
		{
			name:           "character owner cannot edit items (stats)",
			endpoint:       fmt.Sprintf("/api/v1/characters/%d/data", character.ID),
			method:         "POST",
			token:          playerToken,
			expectedStatus: http.StatusForbidden,
			body: CharacterDataRequest{
				ModuleType: "inventory",
				FieldName:  "items",
				FieldValue: `[{"id":"item-1","name":"Sword","quantity":1}]`,
				FieldType:  "json",
				IsPublic:   true,
			},
			reason: "only GMs can edit character items",
		},
		{
			name:           "character owner cannot edit currency (stats)",
			endpoint:       fmt.Sprintf("/api/v1/characters/%d/data", character.ID),
			method:         "POST",
			token:          playerToken,
			expectedStatus: http.StatusForbidden,
			body: CharacterDataRequest{
				ModuleType: "currency",
				FieldName:  "currency",
				FieldValue: `[{"name":"Gold","amount":100}]`,
				FieldType:  "json",
				IsPublic:   false,
			},
			reason: "only GMs can edit character currency",
		},
		{
			name:           "GM can edit character abilities (stats)",
			endpoint:       fmt.Sprintf("/api/v1/characters/%d/data", character.ID),
			method:         "POST",
			token:          gmToken,
			expectedStatus: http.StatusNoContent,
			body: CharacterDataRequest{
				ModuleType: "abilities",
				FieldName:  "abilities",
				FieldValue: `[{"id":"ability-1","name":"Fireball","description":"Cast fireball","type":"spell"}]`,
				FieldType:  "json",
				IsPublic:   true,
			},
			reason: "GMs should be able to edit character abilities",
		},
		{
			name:           "GM can edit character items (stats)",
			endpoint:       fmt.Sprintf("/api/v1/characters/%d/data", character.ID),
			method:         "POST",
			token:          gmToken,
			expectedStatus: http.StatusNoContent,
			body: CharacterDataRequest{
				ModuleType: "inventory",
				FieldName:  "items",
				FieldValue: `[{"id":"item-1","name":"Sword","quantity":1}]`,
				FieldType:  "json",
				IsPublic:   true,
			},
			reason: "GMs should be able to edit character items",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(tc.method, tc.endpoint, bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+tc.token)

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Errorf("%s: expected %d, got %d. Response: %s", tc.reason, tc.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestCharacterAPI_ErrorHandling(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "character_data", "npc_assignments", "characters", "game_participants", "games", "sessions", "users")

	// Setup application
	app := core.NewTestApp(testDB.Pool)

	// Create test user and game
	gmUser := testDB.CreateTestUser(t, "gm", "gm@example.com")
	playerUser := testDB.CreateTestUser(t, "player", "player@example.com")

	gameService := &services.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	game, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:       "Error Test Game",
		Description: "Testing error handling",
		GMUserID:    int32(gmUser.ID),
		IsPublic:    true,
	})
	core.AssertNoError(t, err, "Failed to create test game")

	// Add player as participant for character creation tests
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(playerUser.ID), "player")
	core.AssertNoError(t, err, "Failed to add player participant")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gmUser)
	core.AssertNoError(t, err, "Failed to create GM token")

	playerToken, err := core.CreateTestJWTTokenForUser(app, playerUser)
	core.AssertNoError(t, err, "Failed to create player token")

	// Setup router with JWT middleware
	tokenAuth := jwtauth.New("HS256", []byte(app.Config.JWT.Secret), nil)
	userService := &services.UserService{DB: testDB.Pool, Logger: app.ObsLogger}
	r := chi.NewRouter()
	handler := Handler{
		App:                 app,
		UserService:         &services.UserService{DB: testDB.Pool, Logger: app.ObsLogger},
		CharacterService:    &services.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger},
		GameService:         &services.GameService{DB: testDB.Pool, Logger: app.ObsLogger},
		NotificationService: services.NewNotificationService(testDB.Pool, app.ObsLogger),
	}
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

	r.Route("/api/v1/games/{gameID}/characters", func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth))
		r.Use(core.RequireAuthenticationMiddleware(userService))
		r.Use(gameHandler.GameMiddleware())
		r.Post("/", handler.CreateCharacter)
	})
	r.Route("/api/v1/characters/{id}", func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth))
		r.Use(core.RequireAuthenticationMiddleware(userService))
		r.Get("/", handler.GetCharacter)
		r.Post("/approve", handler.ApproveCharacter)
	})

	testCases := []struct {
		name           string
		endpoint       string
		method         string
		body           interface{}
		token          string
		expectedStatus int
		reason         string
	}{
		{
			name:     "create character with missing name",
			endpoint: fmt.Sprintf("/api/v1/games/%d/characters", game.ID),
			method:   "POST",
			token:    playerToken,
			body: CreateCharacterRequest{
				Name:          "",
				CharacterType: "player_character",
			},
			expectedStatus: http.StatusBadRequest,
			reason:         "should reject empty character name",
		},
		{
			name:     "create character with invalid type",
			endpoint: fmt.Sprintf("/api/v1/games/%d/characters", game.ID),
			method:   "POST",
			token:    playerToken,
			body: CreateCharacterRequest{
				Name:          "Invalid Character",
				CharacterType: "invalid_type",
			},
			expectedStatus: http.StatusBadRequest,
			reason:         "should reject invalid character type",
		},
		{
			name:           "get nonexistent character",
			endpoint:       "/api/v1/characters/99999",
			method:         "GET",
			token:          gmToken,
			body:           nil,
			expectedStatus: http.StatusInternalServerError,
			reason:         "should handle nonexistent character",
		},
		{
			name:     "approve nonexistent character",
			endpoint: "/api/v1/characters/99999/approve",
			method:   "POST",
			token:    gmToken,
			body: ApproveCharacterRequest{
				Status: "approved",
			},
			expectedStatus: http.StatusInternalServerError,
			reason:         "should handle nonexistent character for approval",
		},
		{
			name:     "invalid character ID format",
			endpoint: "/api/v1/characters/invalid/approve",
			method:   "POST",
			token:    gmToken,
			body: ApproveCharacterRequest{
				Status: "approved",
			},
			expectedStatus: http.StatusBadRequest,
			reason:         "should reject invalid character ID format",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var req *http.Request
			if tc.body != nil {
				body, _ := json.Marshal(tc.body)
				req = httptest.NewRequest(tc.method, tc.endpoint, bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(tc.method, tc.endpoint, nil)
			}
			req.Header.Set("Authorization", "Bearer "+tc.token)

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Errorf("%s: expected %d, got %d. Response: %s", tc.reason, tc.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestCharacterAPI_UnauthenticatedAccess(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "character_data", "npc_assignments", "characters", "games", "sessions", "users")

	// Setup application
	app := core.NewTestApp(testDB.Pool)

	// Create test user (game not needed for this test)
	gmUser := testDB.CreateTestUser(t, "gm", "gm@example.com")
	_ = gmUser // Suppress unused variable warning

	// Setup router with JWT middleware
	r := chi.NewRouter()
	tokenAuth := jwtauth.New("HS256", []byte(app.Config.JWT.Secret), nil)
	userService := &services.UserService{DB: testDB.Pool, Logger: app.ObsLogger}

	handler := Handler{
		App:                 app,
		UserService:         &services.UserService{DB: testDB.Pool, Logger: app.ObsLogger},
		CharacterService:    &services.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger},
		GameService:         &services.GameService{DB: testDB.Pool, Logger: app.ObsLogger},
		NotificationService: services.NewNotificationService(testDB.Pool, app.ObsLogger),
	}
	r.Route("/api/v1/characters/{id}", func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth))
		r.Use(core.RequireAuthenticationMiddleware(userService))
		r.Get("/", handler.GetCharacter)
		r.Post("/data", handler.SetCharacterData)
	})

	testCases := []struct {
		name     string
		endpoint string
		method   string
	}{
		{
			name:     "get character without auth",
			endpoint: "/api/v1/characters/1",
			method:   "GET",
		},
		{
			name:     "set character data without auth",
			endpoint: "/api/v1/characters/1/data",
			method:   "POST",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.endpoint, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			core.AssertEqual(t, http.StatusUnauthorized, w.Code, "Should require authentication")
		})
	}
}

func TestCharacterAPI_ControllableAndInactive(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "npc_assignments", "character_data", "characters", "game_participants", "games", "sessions", "users")

	// Setup application
	app := core.NewTestApp(testDB.Pool)

	// Create test users
	gmUser := testDB.CreateTestUser(t, "gm", "gm@example.com")
	playerUser := testDB.CreateTestUser(t, "player", "player@example.com")
	audienceUser := testDB.CreateTestUser(t, "audience", "audience@example.com")
	inactivePlayerUser := testDB.CreateTestUser(t, "inactive_player", "inactive@example.com")

	// Create test game
	gameService := &services.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	game, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:       "Character Management Test",
		Description: "Testing controllable and inactive characters",
		GMUserID:    int32(gmUser.ID),
		IsPublic:    true,
	})
	core.AssertNoError(t, err, "Failed to create test game")

	// Add participants
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(playerUser.ID), "player")
	core.AssertNoError(t, err, "Failed to add player participant")
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(audienceUser.ID), "audience")
	core.AssertNoError(t, err, "Failed to add audience participant")
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(inactivePlayerUser.ID), "player")
	core.AssertNoError(t, err, "Failed to add inactive player participant")

	// Create tokens
	gmToken, err := core.CreateTestJWTTokenForUser(app, gmUser)
	core.AssertNoError(t, err, "Failed to create GM token")
	playerToken, err := core.CreateTestJWTTokenForUser(app, playerUser)
	core.AssertNoError(t, err, "Failed to create player token")
	audienceToken, err := core.CreateTestJWTTokenForUser(app, audienceUser)
	core.AssertNoError(t, err, "Failed to create audience token")

	// Setup router
	tokenAuth := jwtauth.New("HS256", []byte(app.Config.JWT.Secret), nil)
	userService := &services.UserService{DB: testDB.Pool, Logger: app.ObsLogger}
	r := chi.NewRouter()
	handler := Handler{
		App:                 app,
		UserService:         &services.UserService{DB: testDB.Pool, Logger: app.ObsLogger},
		CharacterService:    &services.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger},
		GameService:         &services.GameService{DB: testDB.Pool, Logger: app.ObsLogger},
		NotificationService: services.NewNotificationService(testDB.Pool, app.ObsLogger),
	}
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

	r.Route("/api/v1/games/{gameID}/characters", func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth))
		r.Use(core.RequireAuthenticationMiddleware(userService))
		r.Use(gameHandler.GameMiddleware())
		r.Post("/", handler.CreateCharacter)
		r.Get("/controllable", handler.GetUserControllableCharacters)
		r.Get("/inactive", handler.ListInactiveCharacters)
	})
	r.Route("/api/v1/characters/{id}", func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth))
		r.Use(core.RequireAuthenticationMiddleware(userService))
		r.Post("/approve", handler.ApproveCharacter)
		r.Post("/assign", handler.AssignNPC)
		r.Put("/reassign", handler.ReassignCharacter)
	})

	// Create characters for testing
	characterService := &services.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Player's own character (approved)
	playerCharID := int32(playerUser.ID)
	playerChar, err := characterService.CreateCharacter(context.Background(), services.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        &playerCharID,
		Name:          "Player Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Failed to create player character")
	_, err = characterService.ApproveCharacter(context.Background(), playerChar.ID)
	core.AssertNoError(t, err, "Failed to approve player character")

	// GM's NPC
	gmCharID := int32(gmUser.ID)
	gmNPC, err := characterService.CreateCharacter(context.Background(), services.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        &gmCharID,
		Name:          "GM NPC",
		CharacterType: "npc",
	})
	core.AssertNoError(t, err, "Failed to create GM NPC")
	_, err = characterService.ApproveCharacter(context.Background(), gmNPC.ID)
	core.AssertNoError(t, err, "Failed to approve GM NPC")

	// Audience NPC (assigned to audience user)
	audienceNPC, err := characterService.CreateCharacter(context.Background(), services.CreateCharacterRequest{
		GameID:        game.ID,
		Name:          "Audience NPC",
		CharacterType: "npc",
	})
	core.AssertNoError(t, err, "Failed to create audience NPC")
	_, err = characterService.ApproveCharacter(context.Background(), audienceNPC.ID)
	core.AssertNoError(t, err, "Failed to approve audience NPC")
	err = characterService.AssignNPCToUser(context.Background(), audienceNPC.ID, int32(audienceUser.ID), int32(gmUser.ID))
	core.AssertNoError(t, err, "Failed to assign NPC to audience")

	// Inactive character (for reassignment testing) - owned by separate user
	inactivePlayerCharID := int32(inactivePlayerUser.ID)
	inactiveChar, err := characterService.CreateCharacter(context.Background(), services.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        &inactivePlayerCharID,
		Name:          "Inactive Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Failed to create inactive character")
	_, err = characterService.ApproveCharacter(context.Background(), inactiveChar.ID)
	core.AssertNoError(t, err, "Failed to approve inactive character")
	// Deactivate this user's characters (will only affect the Inactive Character)
	err = characterService.DeactivatePlayerCharacters(context.Background(), game.ID, int32(inactivePlayerUser.ID))
	core.AssertNoError(t, err, "Failed to mark character as inactive")

	t.Run("get_controllable_characters_as_player", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/characters/controllable", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		core.AssertEqual(t, http.StatusOK, w.Code, "Should return 200 OK")

		var response []map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")

		// Player should see their own active character (not the inactive one)
		core.AssertTrue(t, len(response) >= 1, "Player should have at least 1 controllable character")

		foundActiveChar := false
		for _, char := range response {
			if char["name"] == "Player Character" {
				foundActiveChar = true
			}
			// Should not include inactive character
			core.AssertTrue(t, char["name"] != "Inactive Character", "Should not include inactive characters")
		}
		core.AssertTrue(t, foundActiveChar, "Should include player's active character")
	})

	t.Run("get_controllable_characters_as_audience", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/characters/controllable", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+audienceToken)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		core.AssertEqual(t, http.StatusOK, w.Code, "Should return 200 OK")

		var response []map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")

		// Audience should see their assigned NPC
		foundAssignedNPC := false
		for _, char := range response {
			if char["name"] == "Audience NPC" {
				foundAssignedNPC = true
			}
		}
		core.AssertTrue(t, foundAssignedNPC, "Audience should see their assigned NPC")
	})

	t.Run("get_controllable_characters_unauthorized", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/characters/controllable", game.ID), nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		core.AssertEqual(t, http.StatusUnauthorized, w.Code, "Should return 401 Unauthorized")
	})

	t.Run("list_inactive_characters_as_gm", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/characters/inactive", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		core.AssertEqual(t, http.StatusOK, w.Code, "Should return 200 OK")

		var response []map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")

		// Should include the inactive character
		foundInactive := false
		for _, char := range response {
			if char["name"] == "Inactive Character" {
				foundInactive = true
				core.AssertTrue(t, char["is_active"] == false, "Character should be marked as inactive")
			}
		}
		core.AssertTrue(t, foundInactive, "Should include inactive character")
	})

	t.Run("list_inactive_characters_as_non_gm", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/characters/inactive", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		core.AssertEqual(t, http.StatusForbidden, w.Code, "Should return 403 Forbidden for non-GM")
	})

	t.Run("list_inactive_characters_unauthorized", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/characters/inactive", game.ID), nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		core.AssertEqual(t, http.StatusUnauthorized, w.Code, "Should return 401 Unauthorized")
	})

	t.Run("reassign_character_as_gm", func(t *testing.T) {
		requestBody := ReassignCharacterRequest{
			NewOwnerUserID: int32(audienceUser.ID),
		}

		body, _ := json.Marshal(requestBody)
		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/characters/%d/reassign", inactiveChar.ID), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		core.AssertEqual(t, http.StatusOK, w.Code, "Should return 200 OK")

		var response CharacterResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")

		core.AssertEqual(t, inactiveChar.ID, response.ID, "Character ID should match")
		core.AssertNotEqual(t, nil, response.UserID, "Character should have new owner")
		if response.UserID != nil {
			core.AssertEqual(t, int32(audienceUser.ID), *response.UserID, "Character should be reassigned to audience user")
		}
	})

	t.Run("reassign_active_character_fails", func(t *testing.T) {
		requestBody := ReassignCharacterRequest{
			NewOwnerUserID: int32(audienceUser.ID),
		}

		body, _ := json.Marshal(requestBody)
		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/characters/%d/reassign", playerChar.ID), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		core.AssertEqual(t, http.StatusConflict, w.Code, "Should return 409 Conflict for active character")
	})

	t.Run("reassign_character_as_non_gm", func(t *testing.T) {
		requestBody := ReassignCharacterRequest{
			NewOwnerUserID: int32(gmUser.ID),
		}

		body, _ := json.Marshal(requestBody)
		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/characters/%d/reassign", inactiveChar.ID), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+playerToken)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		core.AssertEqual(t, http.StatusForbidden, w.Code, "Should return 403 Forbidden for non-GM")
	})

	t.Run("reassign_character_unauthorized", func(t *testing.T) {
		requestBody := ReassignCharacterRequest{
			NewOwnerUserID: int32(gmUser.ID),
		}

		body, _ := json.Marshal(requestBody)
		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/characters/%d/reassign", inactiveChar.ID), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		core.AssertEqual(t, http.StatusUnauthorized, w.Code, "Should return 401 Unauthorized")
	})
}
