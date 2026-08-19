package conversations

import (
	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"
	db "actionphase/pkg/db/services"
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
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function for creating int32 pointers
func int32Ptr(i int32) *int32 {
	return &i
}

// setupConversationAPITestRouter creates a test router with conversation routes
func setupConversationAPITestRouter(app *core.App, testDB *core.TestDatabase) *chi.Mux {
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
			conversationHandler := Handler{
				App:                 app,
				GameService:         &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger},
				CharacterService:    &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger},
				ConversationService: db.NewConversationService(testDB.Pool, nil),
				PhaseService:        &phasesvc.PhaseService{DB: testDB.Pool},
			}

			r.Group(func(r chi.Router) {
				r.Use(jwtauth.Verifier(tokenAuth))
				r.Use(jwtauth.Authenticator(tokenAuth))
				r.Use(core.RequireAuthenticationMiddleware(userService))
				r.Use(gameHandler.GameMiddleware())

				// Conversation routes
				conversationHandler.RegisterRoutes(r)
			})
		})
	})

	return r
}

// TestConversationAPI_CreateConversation tests POST /games/{gameId}/conversations
func TestConversationAPI_CreateConversation(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "conversations", "characters", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupConversationAPITestRouter(app, testDB)

	// Create test users
	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")

	// Generate JWT tokens for test users
	player1Token, err := core.CreateTestJWTTokenForUser(app, player1)
	core.AssertNoError(t, err, "Should create player1 token")
	_ = player2 // player2 not needed for auth in this test

	// Create test game
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	// Setup services
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	phaseService := &phasesvc.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Add players as participants
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player1.ID), "player")
	core.AssertNoError(t, err, "Should add player1 as participant")
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player2.ID), "player")
	core.AssertNoError(t, err, "Should add player2 as participant")

	// Create common room phase for messaging
	phase, err := phaseService.CreatePhase(context.Background(), core.CreatePhaseRequest{
		GameID:      game.ID,
		PhaseType:   "common_room",
		PhaseNumber: 1,
		Title:       "Common Room 1",
		Description: "Test common room phase",
	})
	core.AssertNoError(t, err, "Should create common room phase")

	// Activate common room phase
	err = phaseService.ActivatePhase(context.Background(), phase.ID, int32(gm.ID))
	core.AssertNoError(t, err, "Should activate common room phase")

	// Create characters for players
	playerChar1, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player1.ID)),
		Name:          "Player 1 Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Should create player1 character")

	playerChar2, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player2.ID)),
		Name:          "Player 2 Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Should create player2 character")

	t.Run("successfully creates conversation with valid participants", func(t *testing.T) {
		reqBody := CreateConversationRequest{
			Title:        "Test Conversation",
			CharacterIDs: []int32{playerChar1.ID, playerChar2.ID},
		}
		reqJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/conversations", game.ID), bytes.NewBuffer(reqJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+player1Token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)

		var response map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.NotNil(t, response["id"])
		assert.Equal(t, float64(game.ID), response["game_id"])
	})

	t.Run("rejects conversation with less than 2 participants", func(t *testing.T) {
		reqBody := CreateConversationRequest{
			Title:        "Invalid Conversation",
			CharacterIDs: []int32{playerChar1.ID}, // Only 1 character
		}
		reqJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/conversations", game.ID), bytes.NewBuffer(reqJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+player1Token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "at least 2 characters")
	})

	t.Run("rejects conversation without title", func(t *testing.T) {
		reqBody := CreateConversationRequest{
			Title:        "", // Empty title
			CharacterIDs: []int32{playerChar1.ID, playerChar2.ID},
		}
		reqJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/conversations", game.ID), bytes.NewBuffer(reqJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+player1Token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "title is required")
	})

	t.Run("rejects unauthenticated requests", func(t *testing.T) {
		reqBody := CreateConversationRequest{
			Title:        "Test Conversation",
			CharacterIDs: []int32{playerChar1.ID, playerChar2.ID},
		}
		reqJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/conversations", game.ID), bytes.NewBuffer(reqJSON))
		req.Header.Set("Content-Type", "application/json")
		// No authenticated user in context

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

// TestConversationAPI_CreateConversation_GroupRestriction tests that the allow_group_conversations
// setting is enforced when creating conversations.
func TestConversationAPI_CreateConversation_GroupRestriction(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "conversations", "characters", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupConversationAPITestRouter(app, testDB)

	player1 := testDB.CreateTestUser(t, "player1r", "player1r@example.com")
	player2 := testDB.CreateTestUser(t, "player2r", "player2r@example.com")
	player3 := testDB.CreateTestUser(t, "player3r", "player3r@example.com")
	gm := testDB.CreateTestUser(t, "gmr", "gmr@example.com")

	player1Token, err := core.CreateTestJWTTokenForUser(app, player1)
	core.AssertNoError(t, err, "Should create player1 token")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	phaseService := &phasesvc.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create game and immediately disable group conversations
	game := testDB.CreateTestGame(t, int32(gm.ID), "No Group Convos Game")
	_, err = gameService.UpdateGame(context.Background(), core.UpdateGameRequest{
		ID:                      game.ID,
		Title:                   game.Title,
		Description:             game.Description.String,
		IsPublic:                true,
		AllowGroupConversations: false,
	})
	core.AssertNoError(t, err, "Should update game to disable group conversations")

	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player1.ID), "player")
	core.AssertNoError(t, err, "Should add player1")
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player2.ID), "player")
	core.AssertNoError(t, err, "Should add player2")
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player3.ID), "player")
	core.AssertNoError(t, err, "Should add player3")

	phase, err := phaseService.CreatePhase(context.Background(), core.CreatePhaseRequest{
		GameID: game.ID, PhaseType: "common_room", PhaseNumber: 1,
		Title: "Common Room", Description: "Test",
	})
	core.AssertNoError(t, err, "Should create phase")
	err = phaseService.ActivatePhase(context.Background(), phase.ID, int32(gm.ID))
	core.AssertNoError(t, err, "Should activate phase")

	char1, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID: game.ID, UserID: int32Ptr(int32(player1.ID)),
		Name: "Char 1", CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Should create char1")
	char2, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID: game.ID, UserID: int32Ptr(int32(player2.ID)),
		Name: "Char 2", CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Should create char2")
	char3, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID: game.ID, UserID: int32Ptr(int32(player3.ID)),
		Name: "Char 3", CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Should create char3")

	t.Run("rejects 3-participant conversation when group conversations disabled", func(t *testing.T) {
		reqBody := CreateConversationRequest{
			Title:        "Group Chat",
			CharacterIDs: []int32{char1.ID, char2.ID, char3.ID},
		}
		reqJSON, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/conversations", game.ID), bytes.NewBuffer(reqJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+player1Token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "group conversations are not allowed")
	})

	t.Run("allows 2-participant conversation when group conversations disabled", func(t *testing.T) {
		reqBody := CreateConversationRequest{
			Title:        "Direct Message",
			CharacterIDs: []int32{char1.ID, char2.ID},
		}
		reqJSON, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/conversations", game.ID), bytes.NewBuffer(reqJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+player1Token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
	})
}

// TestConversationAPI_SendMessage tests POST /games/{gameId}/conversations/{conversationId}/messages
func TestConversationAPI_SendMessage(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "conversations", "characters", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupConversationAPITestRouter(app, testDB)

	// Create test users
	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")
	player3 := testDB.CreateTestUser(t, "player3", "player3@example.com") // Non-participant
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")

	// Generate JWT tokens for test users
	player1Token, err := core.CreateTestJWTTokenForUser(app, player1)
	core.AssertNoError(t, err, "Should create player1 token")
	player2Token, err := core.CreateTestJWTTokenForUser(app, player2)
	core.AssertNoError(t, err, "Should create player2 token")
	player3Token, err := core.CreateTestJWTTokenForUser(app, player3)
	core.AssertNoError(t, err, "Should create player3 token")
	_ = player2Token // player2Token not needed in this test

	// Create test game
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	// Setup services
	conversationService := db.NewConversationService(testDB.Pool, nil)
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	phaseService := &phasesvc.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Add players as participants
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player1.ID), "player")
	core.AssertNoError(t, err, "Should add player1")
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player2.ID), "player")
	core.AssertNoError(t, err, "Should add player2")
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player3.ID), "player")
	core.AssertNoError(t, err, "Should add player3")

	// Create common room phase
	phase2, err := phaseService.CreatePhase(context.Background(), core.CreatePhaseRequest{
		GameID:      game.ID,
		PhaseType:   "common_room",
		PhaseNumber: 1,
		Title:       "Common Room 1",
		Description: "Test common room phase",
	})
	core.AssertNoError(t, err, "Should create common room phase")

	// Activate common room phase
	err = phaseService.ActivatePhase(context.Background(), phase2.ID, int32(gm.ID))
	core.AssertNoError(t, err, "Should activate common room phase")

	// Create characters
	playerChar1, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player1.ID)),
		Name:          "Player 1 Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Should create player1 character")

	playerChar2, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player2.ID)),
		Name:          "Player 2 Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Should create player2 character")

	playerChar3, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player3.ID)),
		Name:          "Player 3 Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Should create player3 character")

	// Create conversation between player1 and player2
	conversation, err := conversationService.CreateConversation(context.Background(), db.CreateConversationRequest{
		GameID:          game.ID,
		Title:           "Test Conversation",
		CreatedByUserID: int32(player1.ID),
		ParticipantIDs:  []int32{playerChar1.ID, playerChar2.ID},
	})
	core.AssertNoError(t, err, "Should create conversation")

	t.Run("successfully sends message as participant", func(t *testing.T) {
		reqBody := SendMessageRequest{
			CharacterID: playerChar1.ID,
			Content:     "Hello from player 1",
		}
		reqJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/conversations/%d/messages", game.ID, conversation.ID), bytes.NewBuffer(reqJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+player1Token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)

		var response map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "Hello from player 1", response["content"])
	})

	t.Run("rejects message from non-participant character", func(t *testing.T) {
		reqBody := SendMessageRequest{
			CharacterID: playerChar3.ID, // Player 3's character not in conversation
			Content:     "Unauthorized message",
		}
		reqJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/conversations/%d/messages", game.ID, conversation.ID), bytes.NewBuffer(reqJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+player3Token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Contains(t, rec.Body.String(), "not a participant")
	})

	t.Run("rejects message using another player's character", func(t *testing.T) {
		reqBody := SendMessageRequest{
			CharacterID: playerChar2.ID, // Player 2's character
			Content:     "Trying to impersonate",
		}
		reqJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/conversations/%d/messages", game.ID, conversation.ID), bytes.NewBuffer(reqJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+player1Token) // Player 1 trying to use Player 2's character

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Contains(t, rec.Body.String(), "cannot send messages as this character")
	})

	t.Run("rejects empty message content", func(t *testing.T) {
		reqBody := SendMessageRequest{
			CharacterID: playerChar1.ID,
			Content:     "", // Empty content
		}
		reqJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/conversations/%d/messages", game.ID, conversation.ID), bytes.NewBuffer(reqJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+player1Token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "content is required")
	})

	t.Run("rejects message exceeding max length", func(t *testing.T) {
		reqBody := SendMessageRequest{
			CharacterID: playerChar1.ID,
			Content:     strings.Repeat("a", 50_001),
		}
		reqJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/conversations/%d/messages", game.ID, conversation.ID), bytes.NewBuffer(reqJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+player1Token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "exceeds maximum length")
	})

	t.Run("rejects message outside common room phase", func(t *testing.T) {
		// Create and activate action phase
		actionPhase, err := phaseService.CreatePhase(context.Background(), core.CreatePhaseRequest{
			GameID:      game.ID,
			PhaseType:   "action",
			PhaseNumber: 2,
			Title:       "Action Phase 1",
			Description: "Test action phase",
		})
		core.AssertNoError(t, err, "Should create action phase")

		err = phaseService.ActivatePhase(context.Background(), actionPhase.ID, int32(gm.ID))
		core.AssertNoError(t, err, "Should activate action phase")

		reqBody := SendMessageRequest{
			CharacterID: playerChar1.ID,
			Content:     "Trying to message during action phase",
		}
		reqJSON, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/conversations/%d/messages", game.ID, conversation.ID), bytes.NewBuffer(reqJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+player1Token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Contains(t, rec.Body.String(), "common room")

		// Restore common room phase for other tests (reactivate the phase2 created earlier)
		err = phaseService.ActivatePhase(context.Background(), phase2.ID, int32(gm.ID))
		core.AssertNoError(t, err, "Should reactivate common room phase")
	})
}

// TestConversationAPI_GetConversationMessages tests GET /games/{gameId}/conversations/{conversationId}/messages
func TestConversationAPI_GetConversationMessages(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "conversations", "characters", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupConversationAPITestRouter(app, testDB)

	// Create test users
	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")
	player3 := testDB.CreateTestUser(t, "player3", "player3@example.com") // Non-participant
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")

	// Generate JWT tokens for test users
	player1Token, err := core.CreateTestJWTTokenForUser(app, player1)
	core.AssertNoError(t, err, "Should create player1 token")
	player3Token, err := core.CreateTestJWTTokenForUser(app, player3)
	core.AssertNoError(t, err, "Should create player3 token")
	_ = player2 // player2 not needed for auth in this test

	// Create test game
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	// Setup services
	conversationService := db.NewConversationService(testDB.Pool, nil)
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Add players as participants
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player1.ID), "player")
	core.AssertNoError(t, err, "Should add player1")
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player2.ID), "player")
	core.AssertNoError(t, err, "Should add player2")
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player3.ID), "player")
	core.AssertNoError(t, err, "Should add player3")

	// Create characters
	playerChar1, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player1.ID)),
		Name:          "Player 1 Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Should create player1 character")

	playerChar2, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player2.ID)),
		Name:          "Player 2 Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Should create player2 character")

	// Create conversation
	conversation, err := conversationService.CreateConversation(context.Background(), db.CreateConversationRequest{
		GameID:          game.ID,
		Title:           "Test Conversation",
		CreatedByUserID: int32(player1.ID),
		ParticipantIDs:  []int32{playerChar1.ID, playerChar2.ID},
	})
	core.AssertNoError(t, err, "Should create conversation")

	// Send some messages
	_, err = conversationService.SendMessage(context.Background(), db.SendMessageRequest{
		ConversationID:    conversation.ID,
		SenderUserID:      int32(player1.ID),
		SenderCharacterID: playerChar1.ID,
		Content:           "Message 1",
	})
	core.AssertNoError(t, err, "Should send message 1")

	_, err = conversationService.SendMessage(context.Background(), db.SendMessageRequest{
		ConversationID:    conversation.ID,
		SenderUserID:      int32(player2.ID),
		SenderCharacterID: playerChar2.ID,
		Content:           "Message 2",
	})
	core.AssertNoError(t, err, "Should send message 2")

	t.Run("successfully retrieves messages as participant", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/conversations/%d/messages", game.ID, conversation.ID), nil)
		req.Header.Set("Authorization", "Bearer "+player1Token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		messages := response["messages"].([]interface{})
		assert.Len(t, messages, 2)
		// Verify the correct messages were returned (not just any 2)
		contents := make([]string, 0, len(messages))
		for _, m := range messages {
			if msg, ok := m.(map[string]interface{}); ok {
				if c, ok := msg["content"].(string); ok {
					contents = append(contents, c)
				}
			}
		}
		assert.Contains(t, contents, "Message 1")
		assert.Contains(t, contents, "Message 2")
	})

	t.Run("rejects non-participant from viewing messages", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/conversations/%d/messages", game.ID, conversation.ID), nil)
		req.Header.Set("Authorization", "Bearer "+player3Token) // Player 3 not in conversation

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Contains(t, rec.Body.String(), "don't have access")
	})

	// The GM passes the handler's conversation-access check (GMs can access any
	// conversation in their game) but is not a participant in this one, so the
	// service's participant check is what rejects them. That rejection must
	// surface as a 403, not a 500 — it is a permission decision, not a failure.
	t.Run("rejects a GM who is not a participant with 403, not 500", func(t *testing.T) {
		gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
		core.AssertNoError(t, err, "Should create gm token")

		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/conversations/%d/messages", game.ID, conversation.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Contains(t, rec.Body.String(), "don't have access")
	})

	t.Run("rejects a non-participant GM requesting scoped message context with 403", func(t *testing.T) {
		gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
		core.AssertNoError(t, err, "Should create gm token")

		messages, err := conversationService.GetConversationMessages(context.Background(), conversation.ID, int32(player1.ID))
		core.AssertNoError(t, err, "Should read messages as participant")
		require.NotEmpty(t, messages)

		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/conversations/%d/messages?context_for=%d", game.ID, conversation.ID, messages[len(messages)-1].ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Contains(t, rec.Body.String(), "don't have access")
	})

	t.Run("rejects a non-participant GM marking the conversation as read with 403", func(t *testing.T) {
		gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
		core.AssertNoError(t, err, "Should create gm token")

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/conversations/%d/read", game.ID, conversation.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Contains(t, rec.Body.String(), "don't have access")
	})

	t.Run("returns empty array when no messages", func(t *testing.T) {
		// Create empty conversation
		emptyConv, err := conversationService.CreateConversation(context.Background(), db.CreateConversationRequest{
			GameID:          game.ID,
			Title:           "Empty Conversation",
			CreatedByUserID: int32(player1.ID),
			ParticipantIDs:  []int32{playerChar1.ID, playerChar2.ID},
		})
		core.AssertNoError(t, err, "Should create empty conversation")

		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/conversations/%d/messages", game.ID, emptyConv.ID), nil)
		req.Header.Set("Authorization", "Bearer "+player1Token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response map[string]interface{}
		err = json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		messages := response["messages"].([]interface{})
		assert.Len(t, messages, 0)
	})

	t.Run("redacts sender username from players in anonymous game, but not from GM", func(t *testing.T) {
		anonGM := testDB.CreateTestUser(t, "anongm", "anongm@example.com")
		anonPlayer1 := testDB.CreateTestUser(t, "anonplayer1", "anonplayer1@example.com")
		anonPlayer2 := testDB.CreateTestUser(t, "anonplayer2", "anonplayer2@example.com")

		anonGMToken, err := core.CreateTestJWTTokenForUser(app, anonGM)
		core.AssertNoError(t, err, "Should create anon GM token")
		anonPlayer1Token, err := core.CreateTestJWTTokenForUser(app, anonPlayer1)
		core.AssertNoError(t, err, "Should create anon player1 token")

		queries := models.New(testDB.Pool)
		anonGame, err := queries.CreateGame(context.Background(), models.CreateGameParams{
			Title:       "Anonymous Test Game",
			Description: pgtype.Text{String: "Test", Valid: true},
			GmUserID:    int32(anonGM.ID),
			IsAnonymous: true,
			IsPublic:    pgtype.Bool{Bool: true, Valid: true},
		})
		core.AssertNoError(t, err, "Should create anonymous game")

		_, err = gameService.AddGameParticipant(context.Background(), anonGame.ID, int32(anonPlayer1.ID), "player")
		core.AssertNoError(t, err, "Should add anonPlayer1")
		_, err = gameService.AddGameParticipant(context.Background(), anonGame.ID, int32(anonPlayer2.ID), "player")
		core.AssertNoError(t, err, "Should add anonPlayer2")

		gmChar, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
			GameID:        anonGame.ID,
			UserID:        int32Ptr(int32(anonGM.ID)),
			Name:          "GM Character",
			CharacterType: "npc",
		})
		core.AssertNoError(t, err, "Should create GM character")

		anonChar1, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
			GameID:        anonGame.ID,
			UserID:        int32Ptr(int32(anonPlayer1.ID)),
			Name:          "Anon Player 1 Character",
			CharacterType: "player_character",
		})
		core.AssertNoError(t, err, "Should create anonPlayer1 character")

		anonConversation, err := conversationService.CreateConversation(context.Background(), db.CreateConversationRequest{
			GameID:          anonGame.ID,
			Title:           "GM DM",
			CreatedByUserID: int32(anonGM.ID),
			ParticipantIDs:  []int32{gmChar.ID, anonChar1.ID},
		})
		core.AssertNoError(t, err, "Should create anonymous conversation")

		_, err = conversationService.SendMessage(context.Background(), db.SendMessageRequest{
			ConversationID:    anonConversation.ID,
			SenderUserID:      int32(anonGM.ID),
			SenderCharacterID: gmChar.ID,
			Content:           "Hello from the GM",
		})
		core.AssertNoError(t, err, "Should send GM message")

		// Player viewing the conversation must not see the GM's username.
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/conversations/%d/messages", anonGame.ID, anonConversation.ID), nil)
		req.Header.Set("Authorization", "Bearer "+anonPlayer1Token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var playerResponse map[string]interface{}
		err = json.Unmarshal(rec.Body.Bytes(), &playerResponse)
		require.NoError(t, err)

		playerMessages := playerResponse["messages"].([]interface{})
		require.Len(t, playerMessages, 1)
		msg := playerMessages[0].(map[string]interface{})
		assert.Equal(t, "Hello from the GM", msg["content"])
		assert.Equal(t, "", msg["sender_username"], "player should not see the GM's username in an anonymous game")

		// The GM viewing the same conversation should still see usernames.
		gmReq := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/conversations/%d/messages", anonGame.ID, anonConversation.ID), nil)
		gmReq.Header.Set("Authorization", "Bearer "+anonGMToken)
		gmRec := httptest.NewRecorder()
		router.ServeHTTP(gmRec, gmReq)

		assert.Equal(t, http.StatusOK, gmRec.Code)

		var gmResponse map[string]interface{}
		err = json.Unmarshal(gmRec.Body.Bytes(), &gmResponse)
		require.NoError(t, err)

		gmMessages := gmResponse["messages"].([]interface{})
		require.Len(t, gmMessages, 1)
		gmMsg := gmMessages[0].(map[string]interface{})
		assert.Equal(t, anonGM.Username, gmMsg["sender_username"], "GM should still see usernames in an anonymous game")
	})
}

// TestConversationAPI_DeleteMessage tests DELETE /games/{gameId}/conversations/{conversationId}/messages/{messageId}
func TestConversationAPI_DeleteMessage(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "conversations", "characters", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupConversationAPITestRouter(app, testDB)

	// Create test users
	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")

	// Generate JWT tokens for test users
	player1Token, err := core.CreateTestJWTTokenForUser(app, player1)
	core.AssertNoError(t, err, "Should create player1 token")
	_ = player2 // player2 not needed for auth in this test

	// Create test game
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	// Setup services
	conversationService := db.NewConversationService(testDB.Pool, nil)
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Add players as participants
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player1.ID), "player")
	core.AssertNoError(t, err, "Should add player1")
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player2.ID), "player")
	core.AssertNoError(t, err, "Should add player2")

	// Create characters
	playerChar1, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player1.ID)),
		Name:          "Player 1 Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Should create player1 character")

	playerChar2, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player2.ID)),
		Name:          "Player 2 Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Should create player2 character")

	// Create conversation
	conversation, err := conversationService.CreateConversation(context.Background(), db.CreateConversationRequest{
		GameID:          game.ID,
		Title:           "Test Conversation",
		CreatedByUserID: int32(player1.ID),
		ParticipantIDs:  []int32{playerChar1.ID, playerChar2.ID},
	})
	core.AssertNoError(t, err, "Should create conversation")

	// Send messages
	msg1, err := conversationService.SendMessage(context.Background(), db.SendMessageRequest{
		ConversationID:    conversation.ID,
		SenderUserID:      int32(player1.ID),
		SenderCharacterID: playerChar1.ID,
		Content:           "Message from Player 1",
	})
	core.AssertNoError(t, err, "Should send message 1")

	msg2, err := conversationService.SendMessage(context.Background(), db.SendMessageRequest{
		ConversationID:    conversation.ID,
		SenderUserID:      int32(player2.ID),
		SenderCharacterID: playerChar2.ID,
		Content:           "Message from Player 2",
	})
	core.AssertNoError(t, err, "Should send message 2")

	t.Run("successfully deletes own message", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/games/%d/conversations/%d/messages/%d", game.ID, conversation.ID, msg1.ID), nil)
		req.Header.Set("Authorization", "Bearer "+player1Token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "deleted successfully")
	})

	t.Run("rejects deleting another player's message", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/games/%d/conversations/%d/messages/%d", game.ID, conversation.ID, msg2.ID), nil)
		req.Header.Set("Authorization", "Bearer "+player1Token) // Player 1 trying to delete Player 2's message

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Contains(t, rec.Body.String(), "your own messages")
	})

	t.Run("returns 404 for non-existent message", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/games/%d/conversations/%d/messages/99999", game.ID, conversation.ID), nil)
		req.Header.Set("Authorization", "Bearer "+player1Token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

// TestConversationAPI_GetUserConversations tests GET /games/{gameId}/conversations
func TestConversationAPI_GetUserConversations(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "conversations", "characters", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupConversationAPITestRouter(app, testDB)

	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")
	player3 := testDB.CreateTestUser(t, "player3", "player3@example.com")
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")

	player1Token, err := core.CreateTestJWTTokenForUser(app, player1)
	core.AssertNoError(t, err, "Should create player1 token")
	player3Token, err := core.CreateTestJWTTokenForUser(app, player3)
	core.AssertNoError(t, err, "Should create player3 token")

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player1.ID), "player")
	core.AssertNoError(t, err, "Should add player1")
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player2.ID), "player")
	core.AssertNoError(t, err, "Should add player2")
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player3.ID), "player")
	core.AssertNoError(t, err, "Should add player3")

	playerChar1, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player1.ID)),
		Name:          "Player 1 Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Should create player1 character")

	playerChar2, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player2.ID)),
		Name:          "Player 2 Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Should create player2 character")

	conversationService := db.NewConversationService(testDB.Pool, nil)
	_, err = conversationService.CreateConversation(context.Background(), db.CreateConversationRequest{
		GameID:          game.ID,
		Title:           "Conversation Between P1 and P2",
		CreatedByUserID: int32(player1.ID),
		ParticipantIDs:  []int32{playerChar1.ID, playerChar2.ID},
	})
	core.AssertNoError(t, err, "Should create conversation")

	t.Run("player sees their own conversations", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/conversations/", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+player1Token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		convs := response["conversations"].([]interface{})
		assert.Len(t, convs, 1)
	})

	t.Run("player without conversations sees empty list", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/conversations/", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+player3Token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		convs := response["conversations"].([]interface{})
		assert.Len(t, convs, 0)
	})
}

// TestConversationAPI_GetConversation tests GET /games/{gameId}/conversations/{conversationId}
func TestConversationAPI_GetConversation(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "conversations", "characters", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupConversationAPITestRouter(app, testDB)

	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")
	player3 := testDB.CreateTestUser(t, "player3", "player3@example.com")
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")

	player1Token, err := core.CreateTestJWTTokenForUser(app, player1)
	core.AssertNoError(t, err, "Should create player1 token")
	player3Token, err := core.CreateTestJWTTokenForUser(app, player3)
	core.AssertNoError(t, err, "Should create player3 token")

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player1.ID), "player")
	core.AssertNoError(t, err, "Should add player1")
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player2.ID), "player")
	core.AssertNoError(t, err, "Should add player2")
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player3.ID), "player")
	core.AssertNoError(t, err, "Should add player3")

	playerChar1, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player1.ID)),
		Name:          "Player 1 Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Should create player1 character")

	playerChar2, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player2.ID)),
		Name:          "Player 2 Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Should create player2 character")

	conversationService := db.NewConversationService(testDB.Pool, nil)
	conversation, err := conversationService.CreateConversation(context.Background(), db.CreateConversationRequest{
		GameID:          game.ID,
		Title:           "Secret Conversation",
		CreatedByUserID: int32(player1.ID),
		ParticipantIDs:  []int32{playerChar1.ID, playerChar2.ID},
	})
	core.AssertNoError(t, err, "Should create conversation")

	t.Run("participant can get conversation details", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/conversations/%d", game.ID, conversation.ID), nil)
		req.Header.Set("Authorization", "Bearer "+player1Token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		conv := response["conversation"].(map[string]interface{})
		assert.Equal(t, "Secret Conversation", conv["title"])
		assert.NotNil(t, response["participants"])
	})

	t.Run("non-participant cannot get conversation details", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/conversations/%d", game.ID, conversation.ID), nil)
		req.Header.Set("Authorization", "Bearer "+player3Token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

// TestConversationAPI_MarkAsRead tests POST /games/{gameId}/conversations/{conversationId}/read
func TestConversationAPI_MarkAsRead(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "conversations", "characters", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupConversationAPITestRouter(app, testDB)

	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")

	player1Token, err := core.CreateTestJWTTokenForUser(app, player1)
	core.AssertNoError(t, err, "Should create player1 token")
	player2Token, err := core.CreateTestJWTTokenForUser(app, player2)
	core.AssertNoError(t, err, "Should create player2 token")

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player1.ID), "player")
	core.AssertNoError(t, err, "Should add player1")
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player2.ID), "player")
	core.AssertNoError(t, err, "Should add player2")

	playerChar1, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player1.ID)),
		Name:          "Player 1 Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Should create player1 character")

	playerChar2, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player2.ID)),
		Name:          "Player 2 Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Should create player2 character")

	conversationService := db.NewConversationService(testDB.Pool, nil)
	conversation, err := conversationService.CreateConversation(context.Background(), db.CreateConversationRequest{
		GameID:          game.ID,
		Title:           "Conversation",
		CreatedByUserID: int32(player1.ID),
		ParticipantIDs:  []int32{playerChar1.ID, playerChar2.ID},
	})
	core.AssertNoError(t, err, "Should create conversation")

	// Player2 sends a message that player1 hasn't read
	_, err = conversationService.SendMessage(context.Background(), db.SendMessageRequest{
		ConversationID:    conversation.ID,
		SenderUserID:      int32(player2.ID),
		SenderCharacterID: playerChar2.ID,
		Content:           "Unread message",
	})
	core.AssertNoError(t, err, "Should send message")

	t.Run("participant marks conversation as read", func(t *testing.T) {
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/conversations/%d/read", game.ID, conversation.ID), nil)
		req.Header.Set("Authorization", "Bearer "+player1Token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Equal(t, true, response["success"])
	})

	t.Run("mark as read is idempotent", func(t *testing.T) {
		// Marking already-read conversation as read should also succeed
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/conversations/%d/read", game.ID, conversation.ID), nil)
		req.Header.Set("Authorization", "Bearer "+player2Token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

// TestConversationAPI_AddParticipant tests POST /api/v1/games/{gameId}/conversations/{conversationId}/participants
func TestConversationAPI_AddParticipant(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "conversations", "characters", "game_participants", "phases", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupConversationAPITestRouter(app, testDB)

	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")
	player3 := testDB.CreateTestUser(t, "player3", "player3@example.com")
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")

	player1Token, err := core.CreateTestJWTTokenForUser(app, player1)
	require.NoError(t, err)
	player3Token, err := core.CreateTestJWTTokenForUser(app, player3)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player1.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player2.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player3.ID), "player")
	require.NoError(t, err)

	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	char1, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID: game.ID, UserID: int32Ptr(int32(player1.ID)), Name: "Char1", CharacterType: "player_character",
	})
	require.NoError(t, err)
	char2, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID: game.ID, UserID: int32Ptr(int32(player2.ID)), Name: "Char2", CharacterType: "player_character",
	})
	require.NoError(t, err)
	char3, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID: game.ID, UserID: int32Ptr(int32(player3.ID)), Name: "Char3", CharacterType: "player_character",
	})
	require.NoError(t, err)

	// Activate a common room phase (required for messaging and participant operations)
	phaseService := &phasesvc.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	phase, err := phaseService.CreatePhase(context.Background(), core.CreatePhaseRequest{
		GameID:    game.ID,
		PhaseType: "common_room",
	})
	require.NoError(t, err)
	err = phaseService.ActivatePhase(context.Background(), phase.ID, int32(gm.ID))
	require.NoError(t, err)

	// Create a conversation between player1 and player2
	conversationService := db.NewConversationService(testDB.Pool, nil)
	conversation, err := conversationService.CreateConversation(context.Background(), db.CreateConversationRequest{
		GameID:          game.ID,
		Title:           "Private Chat",
		CreatedByUserID: int32(player1.ID),
		ParticipantIDs:  []int32{char1.ID, char2.ID},
	})
	require.NoError(t, err)

	t.Run("participant can add a new character to conversation", func(t *testing.T) {
		body := map[string]int32{"character_id": char3.ID}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/conversations/%d/participants", game.ID, conversation.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+player1Token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Equal(t, true, response["success"])
	})

	t.Run("non-participant cannot add to conversation", func(t *testing.T) {
		// player3 is now a participant (added above), use a fresh conversation
		otherConv, err := conversationService.CreateConversation(context.Background(), db.CreateConversationRequest{
			GameID:          game.ID,
			Title:           "Other Chat",
			CreatedByUserID: int32(player1.ID),
			ParticipantIDs:  []int32{char1.ID, char2.ID},
		})
		require.NoError(t, err)

		body := map[string]int32{"character_id": char3.ID}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/conversations/%d/participants", game.ID, otherConv.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+player3Token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

// TestConversationAPI_UpdateMessage tests PATCH /api/v1/games/{gameId}/conversations/{conversationId}/messages/{messageId}
func TestConversationAPI_UpdateMessage(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "conversations", "characters", "game_participants", "phases", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupConversationAPITestRouter(app, testDB)

	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")

	player1Token, err := core.CreateTestJWTTokenForUser(app, player1)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player1.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player2.ID), "player")
	require.NoError(t, err)

	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	char1, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID: game.ID, UserID: int32Ptr(int32(player1.ID)), Name: "Char1", CharacterType: "player_character",
	})
	require.NoError(t, err)
	char2, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID: game.ID, UserID: int32Ptr(int32(player2.ID)), Name: "Char2", CharacterType: "player_character",
	})
	require.NoError(t, err)

	// Activate a common room phase
	phaseService := &phasesvc.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	phase, err := phaseService.CreatePhase(context.Background(), core.CreatePhaseRequest{
		GameID:    game.ID,
		PhaseType: "common_room",
	})
	require.NoError(t, err)
	err = phaseService.ActivatePhase(context.Background(), phase.ID, int32(gm.ID))
	require.NoError(t, err)

	conversationService := db.NewConversationService(testDB.Pool, nil)
	conversation, err := conversationService.CreateConversation(context.Background(), db.CreateConversationRequest{
		GameID:          game.ID,
		Title:           "Chat",
		CreatedByUserID: int32(player1.ID),
		ParticipantIDs:  []int32{char1.ID, char2.ID},
	})
	require.NoError(t, err)

	// Player1 sends a message
	msg, err := conversationService.SendMessage(context.Background(), db.SendMessageRequest{
		ConversationID:    conversation.ID,
		SenderUserID:      int32(player1.ID),
		SenderCharacterID: char1.ID,
		Content:           "Original content",
	})
	require.NoError(t, err)

	player2Token, err := core.CreateTestJWTTokenForUser(app, player2)
	require.NoError(t, err)

	// Player2 sends a message that player1 will try to edit
	msg2, err := conversationService.SendMessage(context.Background(), db.SendMessageRequest{
		ConversationID:    conversation.ID,
		SenderUserID:      int32(player2.ID),
		SenderCharacterID: char2.ID,
		Content:           "Player2 original message",
	})
	require.NoError(t, err)

	editURL := fmt.Sprintf("/api/v1/games/%d/conversations/%d/messages/%d", game.ID, conversation.ID, msg.ID)
	editURL2 := fmt.Sprintf("/api/v1/games/%d/conversations/%d/messages/%d", game.ID, conversation.ID, msg2.ID)

	t.Run("sender can edit their message", func(t *testing.T) {
		body := map[string]string{"content": "Edited content"}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("PATCH", editURL, bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+player1Token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Equal(t, "Edited content", response["content"])
	})

	t.Run("player cannot edit another player's message", func(t *testing.T) {
		body := map[string]string{"content": "Sneaky edit"}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("PATCH", editURL2, bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+player1Token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("returns 400 for empty content", func(t *testing.T) {
		body := map[string]string{"content": ""}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("PATCH", editURL, bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+player1Token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("returns 400 for content exceeding max length", func(t *testing.T) {
		body := map[string]string{"content": strings.Repeat("a", 50_001)}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("PATCH", editURL, bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+player1Token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "exceeds maximum length")
	})

	t.Run("cannot edit message outside common room phase", func(t *testing.T) {
		// Switch to action phase
		actionPhase, err := phaseService.CreatePhase(context.Background(), core.CreatePhaseRequest{
			GameID:    game.ID,
			PhaseType: "action",
		})
		require.NoError(t, err)
		err = phaseService.ActivatePhase(context.Background(), actionPhase.ID, int32(gm.ID))
		require.NoError(t, err)

		body := map[string]string{"content": "Edit during action phase"}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("PATCH", editURL, bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+player1Token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Contains(t, rec.Body.String(), "common room")

		// Restore common room phase
		err = phaseService.ActivatePhase(context.Background(), phase.ID, int32(gm.ID))
		require.NoError(t, err)
	})

	t.Run("non-participant cannot edit a message", func(t *testing.T) {
		player3 := testDB.CreateTestUser(t, "player3", "player3@example.com")
		_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player3.ID), "player")
		require.NoError(t, err)
		player3Token, err := core.CreateTestJWTTokenForUser(app, player3)
		require.NoError(t, err)
		_ = player2Token

		body := map[string]string{"content": "Outsider edit"}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("PATCH", editURL, bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+player3Token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

// TestConversationAPI_InterludePhaseMessaging verifies that private messages are allowed during
// interlude phases (PM-only phases with no public post or action submissions).
func TestConversationAPI_InterludePhaseMessaging(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "conversations", "characters", "game_participants", "phases", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupConversationAPITestRouter(app, testDB)

	player1 := testDB.CreateTestUser(t, "player1i", "player1i@example.com")
	player2 := testDB.CreateTestUser(t, "player2i", "player2i@example.com")
	gm := testDB.CreateTestUser(t, "gmi", "gmi@example.com")

	player1Token, err := core.CreateTestJWTTokenForUser(app, player1)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Interlude Test Game")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player1.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player2.ID), "player")
	require.NoError(t, err)

	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	char1, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID: game.ID, UserID: int32Ptr(int32(player1.ID)), Name: "Char1i", CharacterType: "player_character",
	})
	require.NoError(t, err)
	char2, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID: game.ID, UserID: int32Ptr(int32(player2.ID)), Name: "Char2i", CharacterType: "player_character",
	})
	require.NoError(t, err)

	phaseService := &phasesvc.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}

	conversationService := db.NewConversationService(testDB.Pool, nil)
	conversation, err := conversationService.CreateConversation(context.Background(), db.CreateConversationRequest{
		GameID:          game.ID,
		Title:           "Planning Chat",
		CreatedByUserID: int32(player1.ID),
		ParticipantIDs:  []int32{char1.ID, char2.ID},
	})
	require.NoError(t, err)

	sendURL := fmt.Sprintf("/api/v1/games/%d/conversations/%d/messages", game.ID, conversation.ID)

	t.Run("PMs allowed during active interlude phase", func(t *testing.T) {
		phase, err := phaseService.CreatePhase(context.Background(), core.CreatePhaseRequest{
			GameID:    game.ID,
			PhaseType: core.PhaseTypeInterlude,
			Title:     "Pre-Action Interlude",
		})
		require.NoError(t, err)
		err = phaseService.ActivatePhase(context.Background(), phase.ID, int32(gm.ID))
		require.NoError(t, err)

		body := SendMessageRequest{CharacterID: char1.ID, Content: "Secret planning message"}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", sendURL, bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+player1Token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)

		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Equal(t, "Secret planning message", response["content"])
	})

	t.Run("PM editing allowed during active interlude phase", func(t *testing.T) {
		phase, err := phaseService.CreatePhase(context.Background(), core.CreatePhaseRequest{
			GameID:    game.ID,
			PhaseType: core.PhaseTypeInterlude,
			Title:     "Edit Test Interlude",
		})
		require.NoError(t, err)
		err = phaseService.ActivatePhase(context.Background(), phase.ID, int32(gm.ID))
		require.NoError(t, err)

		msg, err := conversationService.SendMessage(context.Background(), db.SendMessageRequest{
			ConversationID:    conversation.ID,
			SenderUserID:      int32(player1.ID),
			SenderCharacterID: char1.ID,
			Content:           "Original message",
		})
		require.NoError(t, err)

		editURL := fmt.Sprintf("/api/v1/games/%d/conversations/%d/messages/%d", game.ID, conversation.ID, msg.ID)
		body := map[string]string{"content": "Edited message"}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("PATCH", editURL, bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+player1Token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Equal(t, "Edited message", response["content"])
	})

	t.Run("PMs blocked during action phase", func(t *testing.T) {
		phase, err := phaseService.CreatePhase(context.Background(), core.CreatePhaseRequest{
			GameID:    game.ID,
			PhaseType: core.PhaseTypeAction,
			Title:     "Action Phase",
		})
		require.NoError(t, err)
		err = phaseService.ActivatePhase(context.Background(), phase.ID, int32(gm.ID))
		require.NoError(t, err)

		body := SendMessageRequest{CharacterID: char1.ID, Content: "Should be blocked"}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", sendURL, bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+player1Token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Contains(t, rec.Body.String(), "interlude")
	})
}

func TestConversationAPI_GetUserConversations_UnreadOnly(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "conversation_read_receipts", "conversation_messages", "conversation_participants", "conversations", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupConversationAPITestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")

	player1Token, err := core.CreateTestJWTTokenForUser(app, player1)
	core.AssertNoError(t, err, "Should create player1 token")

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	conversationService := db.NewConversationService(testDB.Pool, nil)

	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player1.ID), "player")
	core.AssertNoError(t, err, "Should add player1")
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player2.ID), "player")
	core.AssertNoError(t, err, "Should add player2")

	char1, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID: game.ID, UserID: int32Ptr(int32(player1.ID)),
		Name: "Char1", CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Should create char1")
	char2, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID: game.ID, UserID: int32Ptr(int32(player2.ID)),
		Name: "Char2", CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Should create char2")

	// Create a conversation and send a message from player2 (so player1 has an unread)
	conv, err := conversationService.CreateConversation(context.Background(), db.CreateConversationRequest{
		GameID: game.ID, Title: "Unread Conv",
		CreatedByUserID: int32(player2.ID),
		ParticipantIDs:  []int32{char1.ID, char2.ID},
	})
	core.AssertNoError(t, err, "Should create conversation")
	_, err = conversationService.SendMessage(context.Background(), db.SendMessageRequest{
		ConversationID:    conv.ID,
		SenderUserID:      int32(player2.ID),
		SenderCharacterID: char2.ID,
		Content:           "Hello there",
	})
	core.AssertNoError(t, err, "Should send message")

	unreadURL := fmt.Sprintf("/api/v1/games/%d/conversations/?unread_only=true", game.ID)

	t.Run("returns only conversations with unread messages", func(t *testing.T) {
		req := httptest.NewRequest("GET", unreadURL, nil)
		req.Header.Set("Authorization", "Bearer "+player1Token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		convs := response["conversations"].([]interface{})
		assert.Len(t, convs, 1)
		conv0 := convs[0].(map[string]interface{})
		assert.Equal(t, float64(conv.ID), conv0["id"])
	})

	t.Run("respects limit param", func(t *testing.T) {
		// Create a second conversation with an unread message
		char3, _ := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
			GameID: game.ID, UserID: int32Ptr(int32(gm.ID)),
			Name: "Char3", CharacterType: "npc",
		})
		conv2, err := conversationService.CreateConversation(context.Background(), db.CreateConversationRequest{
			GameID: game.ID, Title: "Second Unread",
			CreatedByUserID: int32(player2.ID),
			ParticipantIDs:  []int32{char1.ID, char3.ID},
		})
		core.AssertNoError(t, err, "Should create second conversation")
		_, err = conversationService.SendMessage(context.Background(), db.SendMessageRequest{
			ConversationID: conv2.ID, SenderUserID: int32(gm.ID), SenderCharacterID: char3.ID, Content: "Another message",
		})
		core.AssertNoError(t, err, "Should send second message")

		req := httptest.NewRequest("GET", unreadURL+"&limit=1", nil)
		req.Header.Set("Authorization", "Bearer "+player1Token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		convs := response["conversations"].([]interface{})
		assert.Len(t, convs, 1, "limit=1 should cap results")
	})

	t.Run("unauthenticated request returns 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", unreadURL, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

// TestConversationAPI_DeleteConversation tests DELETE /games/{gameId}/conversations/{conversationId}
func TestConversationAPI_DeleteConversation(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "conversations", "characters", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupConversationAPITestRouter(app, testDB)

	player1 := testDB.CreateTestUser(t, "convdel_player1", "convdel_player1@example.com")
	player2 := testDB.CreateTestUser(t, "convdel_player2", "convdel_player2@example.com")
	gm := testDB.CreateTestUser(t, "convdel_gm", "convdel_gm@example.com")
	outsider := testDB.CreateTestUser(t, "convdel_outsider", "convdel_outsider@example.com")

	player1Token, err := core.CreateTestJWTTokenForUser(app, player1)
	core.AssertNoError(t, err, "Should create player1 token")
	player2Token, err := core.CreateTestJWTTokenForUser(app, player2)
	core.AssertNoError(t, err, "Should create player2 token")
	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	core.AssertNoError(t, err, "Should create gm token")
	outsiderToken, err := core.CreateTestJWTTokenForUser(app, outsider)
	core.AssertNoError(t, err, "Should create outsider token")

	game := testDB.CreateTestGame(t, int32(gm.ID), "Conversation Delete API Game")

	conversationService := db.NewConversationService(testDB.Pool, nil)
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player1.ID), "player")
	core.AssertNoError(t, err, "Should add player1")
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player2.ID), "player")
	core.AssertNoError(t, err, "Should add player2")

	playerChar1, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player1.ID)),
		Name:          "Convdel Character 1",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Should create player1 character")

	playerChar2, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player2.ID)),
		Name:          "Convdel Character 2",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Should create player2 character")

	newConversation := func(t *testing.T) *models.Conversation {
		t.Helper()
		conv, err := conversationService.CreateConversation(context.Background(), db.CreateConversationRequest{
			GameID:          game.ID,
			Title:           "Oops Conversation",
			CreatedByUserID: int32(player1.ID),
			ParticipantIDs:  []int32{playerChar1.ID, playerChar2.ID},
		})
		core.AssertNoError(t, err, "Should create conversation")
		return conv
	}

	deleteRequest := func(conversationID int32, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/games/%d/conversations/%d", game.ID, conversationID), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	t.Run("creator deletes an empty conversation", func(t *testing.T) {
		conv := newConversation(t)

		rec := deleteRequest(conv.ID, player1Token)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "deleted successfully")

		// A follow-up GET must now fail, proving the row is really gone.
		getReq := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/conversations/%d", game.ID, conv.ID), nil)
		getReq.Header.Set("Authorization", "Bearer "+player1Token)
		getRec := httptest.NewRecorder()
		router.ServeHTTP(getRec, getReq)
		assert.NotEqual(t, http.StatusOK, getRec.Code, "deleted conversation should not be retrievable")
	})

	t.Run("GM deletes an empty conversation created by a player", func(t *testing.T) {
		conv := newConversation(t)

		rec := deleteRequest(conv.ID, gmToken)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("returns 409 when the conversation has messages", func(t *testing.T) {
		conv := newConversation(t)
		_, err := conversationService.SendMessage(context.Background(), db.SendMessageRequest{
			ConversationID:    conv.ID,
			SenderUserID:      int32(player1.ID),
			SenderCharacterID: playerChar1.ID,
			Content:           "Now it has history",
		})
		core.AssertNoError(t, err, "Should send message")

		rec := deleteRequest(conv.ID, player1Token)

		assert.Equal(t, http.StatusConflict, rec.Code)
		assert.Contains(t, rec.Body.String(), "cannot be deleted")

		// The conversation must survive the rejected delete.
		surviving, err := conversationService.GetConversation(context.Background(), conv.ID)
		core.AssertNoError(t, err, "Conversation should still exist")
		assert.Equal(t, conv.ID, surviving.ID)
	})

	t.Run("returns 403 for a participant who is not the creator", func(t *testing.T) {
		conv := newConversation(t)

		rec := deleteRequest(conv.ID, player2Token)

		assert.Equal(t, http.StatusForbidden, rec.Code)

		_, err := conversationService.GetConversation(context.Background(), conv.ID)
		core.AssertNoError(t, err, "Conversation should still exist")
	})

	t.Run("returns 403 for a user with no access to the conversation", func(t *testing.T) {
		conv := newConversation(t)

		rec := deleteRequest(conv.ID, outsiderToken)

		assert.Equal(t, http.StatusForbidden, rec.Code)

		_, err := conversationService.GetConversation(context.Background(), conv.ID)
		core.AssertNoError(t, err, "Conversation should still exist")
	})

	t.Run("requires authentication", func(t *testing.T) {
		conv := newConversation(t)

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/games/%d/conversations/%d", game.ID, conv.ID), nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)

		_, err := conversationService.GetConversation(context.Background(), conv.ID)
		core.AssertNoError(t, err, "Conversation should still exist")
	})

	t.Run("returns 404 for a conversation that does not exist", func(t *testing.T) {
		// A missing row must not read as a server fault: the access check
		// returns an error (not canAccess=false) for a nonexistent ID, which
		// previously surfaced as a 500.
		rec := deleteRequest(999999, player1Token)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("returns 404 when deleting the same conversation twice", func(t *testing.T) {
		conv := newConversation(t)

		first := deleteRequest(conv.ID, player1Token)
		assert.Equal(t, http.StatusOK, first.Code)

		second := deleteRequest(conv.ID, player1Token)
		assert.Equal(t, http.StatusNotFound, second.Code, "second delete should report not-found, not a server error")
	})
}
