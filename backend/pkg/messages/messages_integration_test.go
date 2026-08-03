package messages

import (
	"actionphase/pkg/core"
	db "actionphase/pkg/db/services"
	dbactions "actionphase/pkg/db/services/actions"
	dbmessages "actionphase/pkg/db/services/messages"
	messagesvc "actionphase/pkg/db/services/messages"
	"actionphase/pkg/games"
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
)

// TestMessageAPI_PostCreationFlow tests the complete post creation workflow
func TestMessageAPI_PostCreationFlow(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "messages", "character_mentions", "characters", "game_participants", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupMessageTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	// Create auth token for GM user
	gmToken, err := createTestAuthToken(app, fixtures.TestUser)
	core.AssertNoError(t, err, "GM token creation should succeed")

	gameID := fixtures.TestGame.ID

	// Create a character for the GM to post as
	charService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	userID := int32(fixtures.TestUser.ID)
	gmCharacter, err := charService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        gameID,
		UserID:        &userID,
		Name:          "GM Test Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "GM character creation should succeed")

	testCases := []struct {
		name           string
		payload        CreatePostRequest
		expectedStatus int
		description    string
		validateFn     func(t *testing.T, response *MessageResponse)
	}{
		{
			name: "successful_post_creation",
			payload: CreatePostRequest{
				CharacterID: gmCharacter.ID,
				Content:     "Welcome to the mission briefing!",
			},
			expectedStatus: 201,
			description:    "Valid post should be created successfully",
			validateFn: func(t *testing.T, response *MessageResponse) {
				core.AssertEqual(t, "Welcome to the mission briefing!", response.Content, "Content should match")
				core.AssertEqual(t, "post", response.MessageType, "Message type should be post")
				core.AssertEqual(t, gameID, response.GameID, "Game ID should match")
				core.AssertEqual(t, gmCharacter.ID, response.CharacterID, "Character ID should match")
				core.AssertEqual(t, int32(0), response.ThreadDepth, "Post thread depth should be 0")
			},
		},
		{
			name: "post_with_character_mention",
			payload: CreatePostRequest{
				CharacterID: gmCharacter.ID,
				Content:     "Attention @Test Player 1 Character and @Test Player 2 Character, report in!",
			},
			expectedStatus: 201,
			description:    "Post with character mentions should be created",
			validateFn: func(t *testing.T, response *MessageResponse) {
				core.AssertEqual(t, "post", response.MessageType, "Message type should be post")
				// Note: Mention extraction happens in the service layer
				// This test verifies the endpoint accepts the content
			},
		},
		{
			name: "post_missing_character_id",
			payload: CreatePostRequest{
				Content: "This should fail",
			},
			expectedStatus: 500, // TODO: Backend should validate and return 400
			description:    "Post without character ID currently returns 500 (needs validation)",
		},
		{
			name: "post_missing_content",
			payload: CreatePostRequest{
				CharacterID: gmCharacter.ID,
				Content:     "",
			},
			expectedStatus: 201, // TODO: Backend should validate content and return 400
			description:    "Post with empty content currently succeeds (needs validation)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			payload, _ := json.Marshal(tc.payload)
			req := httptest.NewRequest("POST", "/api/v1/games/"+strconv.Itoa(int(gameID))+"/posts", bytes.NewBuffer(payload))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+gmToken)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			core.AssertEqual(t, tc.expectedStatus, w.Code, tc.description)

			if tc.expectedStatus == 201 && tc.validateFn != nil {
				var response MessageResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				core.AssertNoError(t, err, "Response should be valid JSON")
				tc.validateFn(t, &response)
			}
		})
	}
}

// TestMessageAPI_CommentCreationFlow tests comment creation with mentions
func TestMessageAPI_CommentCreationFlow(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "messages", "character_mentions", "characters", "game_participants", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupMessageTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	// Create player user
	playerUser := testDB.CreateTestUser(t, "player", "player@example.com")

	playerToken, err := createTestAuthToken(app, playerUser)
	core.AssertNoError(t, err, "Player token creation should succeed")

	gameID := fixtures.TestGame.ID

	// Create characters
	charService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	gmUserID := int32(fixtures.TestUser.ID)
	gmCharacter, err := charService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        gameID,
		UserID:        &gmUserID,
		Name:          "GM Test Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "GM character creation should succeed")

	playerUserID := int32(playerUser.ID)
	playerCharacter, err := charService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        gameID,
		UserID:        &playerUserID,
		Name:          "Player Test Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Player character creation should succeed")

	// Create a post first
	messageService := &messagesvc.MessageService{DB: testDB.Pool, Logger: app.ObsLogger}
	post, err := messageService.CreatePost(context.Background(), core.CreatePostRequest{
		GameID:      gameID,
		AuthorID:    int32(fixtures.TestUser.ID),
		CharacterID: gmCharacter.ID,
		Content:     "This is a test post for commenting",
		Visibility:  "game",
	})
	core.AssertNoError(t, err, "Post creation should succeed")

	testCases := []struct {
		name           string
		token          string
		payload        CreateCommentRequest
		expectedStatus int
		description    string
		validateFn     func(t *testing.T, response *MessageResponse)
	}{
		{
			name:  "successful_comment_creation",
			token: playerToken,
			payload: CreateCommentRequest{
				CharacterID: playerCharacter.ID,
				Content:     "I acknowledge the briefing!",
			},
			expectedStatus: 201,
			description:    "Valid comment should be created successfully",
			validateFn: func(t *testing.T, response *MessageResponse) {
				core.AssertEqual(t, "comment", response.MessageType, "Message type should be comment")
				core.AssertEqual(t, "I acknowledge the briefing!", response.Content, "Content should match")
				core.AssertEqual(t, post.ID, *response.ParentID, "Parent ID should match the post")
				core.AssertEqual(t, int32(1), response.ThreadDepth, "Comment thread depth should be 1")
			},
		},
		{
			name:  "comment_with_mention",
			token: playerToken,
			payload: CreateCommentRequest{
				CharacterID: playerCharacter.ID,
				Content:     "Hey @GM Test Character, what are the mission parameters?",
			},
			expectedStatus: 201,
			description:    "Comment with character mention should be created",
			validateFn: func(t *testing.T, response *MessageResponse) {
				core.AssertEqual(t, "comment", response.MessageType, "Message type should be comment")
				// Verify content preserved (mention extraction is service-level concern)
				core.AssertEqual(t, "Hey @GM Test Character, what are the mission parameters?", response.Content, "Content should preserve mentions")
			},
		},
		{
			name:  "comment_missing_character_id",
			token: playerToken,
			payload: CreateCommentRequest{
				Content: "This should fail",
			},
			expectedStatus: 500, // TODO: Backend should validate and return 400
			description:    "Comment without character ID currently returns 500 (needs validation)",
		},
		{
			name:  "comment_missing_content",
			token: playerToken,
			payload: CreateCommentRequest{
				CharacterID: playerCharacter.ID,
				Content:     "",
			},
			expectedStatus: 201, // TODO: Backend should validate content and return 400
			description:    "Comment with empty content currently succeeds (needs validation)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			payload, _ := json.Marshal(tc.payload)
			url := "/api/v1/games/" + strconv.Itoa(int(gameID)) + "/posts/" + strconv.Itoa(int(post.ID)) + "/comments"
			req := httptest.NewRequest("POST", url, bytes.NewBuffer(payload))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+tc.token)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			core.AssertEqual(t, tc.expectedStatus, w.Code, tc.description)

			if tc.expectedStatus == 201 && tc.validateFn != nil {
				var response MessageResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				core.AssertNoError(t, err, "Response should be valid JSON")
				tc.validateFn(t, &response)
			}
		})
	}
}

// TestMessageAPI_GetGamePosts tests fetching posts for a game
func TestMessageAPI_GetGamePosts(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "messages", "character_mentions", "characters", "game_participants", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupMessageTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	gmToken, err := createTestAuthToken(app, fixtures.TestUser)
	core.AssertNoError(t, err, "GM token creation should succeed")

	gameID := fixtures.TestGame.ID

	// Create a character
	charService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	gmUserID := int32(fixtures.TestUser.ID)
	gmCharacter, err := charService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        gameID,
		UserID:        &gmUserID,
		Name:          "GM Test Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Character creation should succeed")

	// Create multiple posts
	messageService := &messagesvc.MessageService{DB: testDB.Pool, Logger: app.ObsLogger}
	for i := 1; i <= 3; i++ {
		_, err := messageService.CreatePost(context.Background(), core.CreatePostRequest{
			GameID:      gameID,
			AuthorID:    int32(fixtures.TestUser.ID),
			CharacterID: gmCharacter.ID,
			Content:     "Test post " + strconv.Itoa(i),
			Visibility:  "game",
		})
		core.AssertNoError(t, err, "Post creation should succeed")
	}

	t.Run("get_all_posts", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/games/"+strconv.Itoa(int(gameID))+"/posts", nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "Get posts should succeed")

		var response []map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")
		core.AssertEqual(t, 3, len(response), "Should return 3 posts")
	})

	t.Run("get_posts_with_pagination", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/games/"+strconv.Itoa(int(gameID))+"/posts?limit=2&offset=0", nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "Get posts with pagination should succeed")

		var response []map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")
		core.AssertEqual(t, 2, len(response), "Should return 2 posts (limit applied)")
	})
}

// TestMessageAPI_GetPostComments tests fetching comments for a post
func TestMessageAPI_GetPostComments(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "messages", "character_mentions", "characters", "game_participants", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupMessageTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	// Create player user
	playerUser := testDB.CreateTestUser(t, "player", "player@example.com")

	playerToken, err := createTestAuthToken(app, playerUser)
	core.AssertNoError(t, err, "Player token creation should succeed")

	gameID := fixtures.TestGame.ID

	// Create characters
	charService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	gmUserID := int32(fixtures.TestUser.ID)
	gmCharacter, err := charService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        gameID,
		UserID:        &gmUserID,
		Name:          "GM Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "GM character creation should succeed")

	playerUserID := int32(playerUser.ID)
	playerCharacter, err := charService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        gameID,
		UserID:        &playerUserID,
		Name:          "Player Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Player character creation should succeed")

	// Create a post
	messageService := &messagesvc.MessageService{DB: testDB.Pool, Logger: app.ObsLogger}
	post, err := messageService.CreatePost(context.Background(), core.CreatePostRequest{
		GameID:      gameID,
		AuthorID:    int32(fixtures.TestUser.ID),
		CharacterID: gmCharacter.ID,
		Content:     "Test post for comments",
		Visibility:  "game",
	})
	core.AssertNoError(t, err, "Post creation should succeed")

	// Create multiple comments
	for i := 1; i <= 2; i++ {
		_, err := messageService.CreateComment(context.Background(), core.CreateCommentRequest{
			GameID:      gameID,
			AuthorID:    int32(playerUser.ID),
			CharacterID: playerCharacter.ID,
			Content:     "Test comment " + strconv.Itoa(i),
			ParentID:    post.ID,
			Visibility:  "game",
		})
		core.AssertNoError(t, err, "Comment creation should succeed")
	}

	t.Run("get_post_comments", func(t *testing.T) {
		url := "/api/v1/games/" + strconv.Itoa(int(gameID)) + "/posts/" + strconv.Itoa(int(post.ID)) + "/comments"
		req := httptest.NewRequest("GET", url, nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "Get comments should succeed")

		var response []map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")
		core.AssertEqual(t, 2, len(response), "Should return 2 comments")

		// Verify comment structure
		firstComment := response[0]
		core.AssertNotEqual(t, nil, firstComment["id"], "Comment should have ID")
		core.AssertNotEqual(t, nil, firstComment["content"], "Comment should have content")
		core.AssertEqual(t, "comment", firstComment["message_type"], "Should be comment type")
	})
}

// TestMessageAPI_AuthorizationChecks tests that only GMs can create posts
func TestMessageAPI_AuthorizationChecks(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "messages", "character_mentions", "characters", "game_participants", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupMessageTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	// Create player user
	playerUser := testDB.CreateTestUser(t, "player", "player@example.com")

	playerToken, err := createTestAuthToken(app, playerUser)
	core.AssertNoError(t, err, "Player token creation should succeed")

	gameID := fixtures.TestGame.ID

	// Add player as a participant so they can access the game
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), gameID, int32(playerUser.ID), "player")
	core.AssertNoError(t, err, "Failed to add player as participant")

	// Create a character for the player
	charService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	playerUserID := int32(playerUser.ID)
	playerCharacter, err := charService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        gameID,
		UserID:        &playerUserID,
		Name:          "Player Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Player character creation should succeed")

	t.Run("player_cannot_create_post", func(t *testing.T) {
		payload := CreatePostRequest{
			CharacterID: playerCharacter.ID,
			Content:     "Players should not be able to create posts",
		}

		payloadBytes, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/v1/games/"+strconv.Itoa(int(gameID))+"/posts", bytes.NewBuffer(payloadBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+playerToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 403, w.Code, "Non-GM should not be able to create posts")
	})
}

// setupMessageTestRouter creates a test router with message routes
func setupMessageTestRouter(app *core.App, testDB *core.TestDatabase) *chi.Mux {
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
			r.Use(jwtauth.Verifier(tokenAuth))
			r.Use(jwtauth.Authenticator(tokenAuth))
			r.Use(core.RequireAuthenticationMiddleware(userService))
			r.Use(gameHandler.GameMiddleware())

			messageHandler := &Handler{
				App:            app,
				UserService:    &db.UserService{DB: testDB.Pool, Logger: app.ObsLogger},
				MessageService: &messagesvc.MessageService{DB: testDB.Pool, Logger: app.ObsLogger, Metrics: app.Observability.OTELMetrics},
			}

			// Post routes
			r.Post("/posts", messageHandler.CreatePost)
			r.Get("/posts", messageHandler.GetGamePosts)
			r.Post("/posts/{postId}/mark-read", messageHandler.MarkPostRead)
			r.Get("/posts-unread-info", messageHandler.GetPostsUnreadInfo)
			r.Get("/unread-comment-ids", messageHandler.GetUnreadCommentIDs)

			// Comment routes
			r.Post("/posts/{postId}/comments", messageHandler.CreateComment)
			r.Get("/posts/{postId}/comments", messageHandler.GetPostComments)
			r.Get("/comments/recent", messageHandler.ListRecentCommentsWithParents)
			r.Patch("/posts/{postId}/comments/{commentId}", messageHandler.UpdateComment)
			r.Delete("/posts/{postId}/comments/{commentId}", messageHandler.DeleteComment)

			// Other routes
			r.Get("/messages/{messageId}", messageHandler.GetMessage)
			r.Get("/read-markers", messageHandler.GetGameReadMarkers)
		})
	})

	return r
}

// createTestAuthToken creates a JWT token for testing
func createTestAuthToken(app *core.App, user *core.User) (string, error) {
	return core.CreateTestJWTTokenForUser(app, user)
}

// TestMessageAPI_GetMessage tests the GetMessage endpoint
func TestMessageAPI_GetMessage(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	testDB.CleanupTables(t, "messages", "characters", "game_participants", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "messages", "characters", "game_participants", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupMessageTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	gmToken, err := core.CreateTestJWTTokenForUser(app, fixtures.TestUser)
	core.AssertNoError(t, err, "GM token creation should succeed")

	gameID := fixtures.TestGame.ID

	// Create a character and post
	charService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	userID := int32(fixtures.TestUser.ID)
	character, err := charService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        gameID,
		UserID:        &userID,
		Name:          "Test Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Character creation should succeed")

	// Create a post via the service
	messageService := &messagesvc.MessageService{DB: testDB.Pool, Logger: app.ObsLogger}
	post, err := messageService.CreatePost(context.Background(), core.CreatePostRequest{
		GameID:      gameID,
		AuthorID:    int32(fixtures.TestUser.ID),
		CharacterID: character.ID,
		Content:     "Test post content",
		Visibility:  "game",
	})
	core.AssertNoError(t, err, "Post creation should succeed")

	t.Run("get_message_success", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/games/"+strconv.Itoa(int(gameID))+"/messages/"+strconv.Itoa(int(post.ID)), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "Should return 200 OK")

		var response MessageResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")
		core.AssertEqual(t, post.ID, response.ID, "Message ID should match")
		core.AssertEqual(t, "Test post content", response.Content, "Content should match")
	})

	t.Run("get_message_not_found", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/games/"+strconv.Itoa(int(gameID))+"/messages/99999", nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Current behavior returns 500 - TODO: improve handler to return 404
		core.AssertEqual(t, 500, w.Code, "Should return error for not found")
	})

	t.Run("get_message_unauthorized", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/games/"+strconv.Itoa(int(gameID))+"/messages/"+strconv.Itoa(int(post.ID)), nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 401, w.Code, "Should return 401 Unauthorized")
	})
}

// TestMessageAPI_MarkPostRead tests the MarkPostRead endpoint
func TestMessageAPI_MarkPostRead(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	testDB.CleanupTables(t, "message_read_markers", "messages", "characters", "game_participants", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "message_read_markers", "messages", "characters", "game_participants", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupMessageTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	gmToken, err := core.CreateTestJWTTokenForUser(app, fixtures.TestUser)
	core.AssertNoError(t, err, "GM token creation should succeed")

	gameID := fixtures.TestGame.ID

	// Create a character and post
	charService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	userID := int32(fixtures.TestUser.ID)
	character, err := charService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        gameID,
		UserID:        &userID,
		Name:          "Test Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Character creation should succeed")

	messageService := &messagesvc.MessageService{DB: testDB.Pool, Logger: app.ObsLogger}
	post, err := messageService.CreatePost(context.Background(), core.CreatePostRequest{
		GameID:      gameID,
		AuthorID:    int32(fixtures.TestUser.ID),
		CharacterID: character.ID,
		Content:     "Test post for read marking",
		Visibility:  "game",
	})
	core.AssertNoError(t, err, "Post creation should succeed")

	t.Run("mark_post_read_success", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/games/"+strconv.Itoa(int(gameID))+"/posts/"+strconv.Itoa(int(post.ID))+"/mark-read", nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "Should return 200 OK")
	})

	t.Run("mark_post_read_not_found", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/games/"+strconv.Itoa(int(gameID))+"/posts/99999/mark-read", nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Current behavior returns 500 - TODO: improve handler to return 404
		core.AssertEqual(t, 500, w.Code, "Should return error for not found")
	})

	t.Run("mark_post_read_unauthorized", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/games/"+strconv.Itoa(int(gameID))+"/posts/"+strconv.Itoa(int(post.ID))+"/mark-read", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 401, w.Code, "Should return 401 Unauthorized")
	})
}

// TestMessageAPI_UpdateDeleteComment tests UpdateComment and DeleteComment endpoints
func TestMessageAPI_UpdateDeleteComment(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	testDB.CleanupTables(t, "messages", "characters", "game_participants", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "messages", "characters", "game_participants", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupMessageTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	gmToken, err := core.CreateTestJWTTokenForUser(app, fixtures.TestUser)
	core.AssertNoError(t, err, "GM token creation should succeed")

	gameID := fixtures.TestGame.ID

	// Create a character and post
	charService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	userID := int32(fixtures.TestUser.ID)
	character, err := charService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        gameID,
		UserID:        &userID,
		Name:          "Test Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Character creation should succeed")

	messageService := &messagesvc.MessageService{DB: testDB.Pool, Logger: app.ObsLogger}
	post, err := messageService.CreatePost(context.Background(), core.CreatePostRequest{
		GameID:      gameID,
		AuthorID:    int32(fixtures.TestUser.ID),
		CharacterID: character.ID,
		Content:     "Test post for comments",
		Visibility:  "game",
	})
	core.AssertNoError(t, err, "Post creation should succeed")

	// Create a comment
	comment, err := messageService.CreateComment(context.Background(), core.CreateCommentRequest{
		GameID:      gameID,
		ParentID:    post.ID,
		AuthorID:    int32(fixtures.TestUser.ID),
		CharacterID: character.ID,
		Content:     "Original comment content",
		Visibility:  "game",
	})
	core.AssertNoError(t, err, "Comment creation should succeed")

	t.Run("update_comment_success", func(t *testing.T) {
		payload := map[string]string{
			"content": "Updated comment content",
		}
		payloadBytes, _ := json.Marshal(payload)

		req := httptest.NewRequest("PATCH", "/api/v1/games/"+strconv.Itoa(int(gameID))+"/posts/"+strconv.Itoa(int(post.ID))+"/comments/"+strconv.Itoa(int(comment.ID)), bytes.NewBuffer(payloadBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "Should return 200 OK")

		var response MessageResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")
		core.AssertEqual(t, "Updated comment content", response.Content, "Content should be updated")
	})

	t.Run("update_comment_not_found", func(t *testing.T) {
		payload := map[string]string{
			"content": "Updated content",
		}
		payloadBytes, _ := json.Marshal(payload)

		req := httptest.NewRequest("PATCH", "/api/v1/games/"+strconv.Itoa(int(gameID))+"/posts/"+strconv.Itoa(int(post.ID))+"/comments/99999", bytes.NewBuffer(payloadBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Current behavior returns 500 - TODO: improve handler to return 404
		core.AssertEqual(t, 500, w.Code, "Should return error for not found")
	})

	t.Run("update_comment_unauthorized", func(t *testing.T) {
		payload := map[string]string{
			"content": "Updated content",
		}
		payloadBytes, _ := json.Marshal(payload)

		req := httptest.NewRequest("PATCH", "/api/v1/games/"+strconv.Itoa(int(gameID))+"/posts/"+strconv.Itoa(int(post.ID))+"/comments/"+strconv.Itoa(int(comment.ID)), bytes.NewBuffer(payloadBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 401, w.Code, "Should return 401 Unauthorized")
	})

	t.Run("delete_comment_success", func(t *testing.T) {
		// Create a new comment to delete
		commentToDelete, err := messageService.CreateComment(context.Background(), core.CreateCommentRequest{
			GameID:      gameID,
			ParentID:    post.ID,
			AuthorID:    int32(fixtures.TestUser.ID),
			CharacterID: character.ID,
			Content:     "Comment to be deleted",
			Visibility:  "game",
		})
		core.AssertNoError(t, err, "Comment creation should succeed")

		req := httptest.NewRequest("DELETE", "/api/v1/games/"+strconv.Itoa(int(gameID))+"/posts/"+strconv.Itoa(int(post.ID))+"/comments/"+strconv.Itoa(int(commentToDelete.ID)), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "Should return 200 OK")
	})

	t.Run("delete_comment_not_found", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/api/v1/games/"+strconv.Itoa(int(gameID))+"/posts/"+strconv.Itoa(int(post.ID))+"/comments/99999", nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Current behavior returns 500 - TODO: improve handler to return 404
		core.AssertEqual(t, 500, w.Code, "Should return error for not found")
	})

	t.Run("delete_comment_unauthorized", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/api/v1/games/"+strconv.Itoa(int(gameID))+"/posts/"+strconv.Itoa(int(post.ID))+"/comments/"+strconv.Itoa(int(comment.ID)), nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 401, w.Code, "Should return 401 Unauthorized")
	})
}

func TestMessageAPI_ReadTrackingAndRecent(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "comment_read_markers", "comments", "posts", "characters", "game_participants", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupMessageTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	gmToken, err := core.CreateTestJWTTokenForUser(app, fixtures.TestUser)
	core.AssertNoError(t, err, "GM token creation should succeed")

	playerUser := testDB.CreateTestUser(t, "player", "player@example.com")
	playerToken, err := core.CreateTestJWTTokenForUser(app, playerUser)
	core.AssertNoError(t, err, "Player token creation should succeed")

	gameID := fixtures.TestGame.ID

	// Add player as participant
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), gameID, int32(playerUser.ID), "player")
	core.AssertNoError(t, err, "Adding player participant should succeed")

	// Create characters
	charService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	gmCharID := int32(fixtures.TestUser.ID)
	gmChar, err := charService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        gameID,
		UserID:        &gmCharID,
		Name:          "GM Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "GM character creation should succeed")
	_, err = charService.ApproveCharacter(context.Background(), gmChar.ID)
	core.AssertNoError(t, err, "GM character approval should succeed")

	playerCharID := int32(playerUser.ID)
	playerChar, err := charService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        gameID,
		UserID:        &playerCharID,
		Name:          "Player Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Player character creation should succeed")
	_, err = charService.ApproveCharacter(context.Background(), playerChar.ID)
	core.AssertNoError(t, err, "Player character approval should succeed")

	// Create post with comments via service
	messageService := &messagesvc.MessageService{DB: testDB.Pool, Logger: app.ObsLogger}
	post, err := messageService.CreatePost(context.Background(), core.CreatePostRequest{
		GameID:      gameID,
		AuthorID:    int32(fixtures.TestUser.ID),
		CharacterID: gmChar.ID,
		Content:     "Test post for read tracking",
		Visibility:  "game",
	})
	core.AssertNoError(t, err, "Post creation should succeed")

	// Create comment via service (GM comment)
	comment1, err := messageService.CreateComment(context.Background(), core.CreateCommentRequest{
		GameID:      gameID,
		ParentID:    post.ID,
		AuthorID:    int32(fixtures.TestUser.ID),
		CharacterID: gmChar.ID,
		Content:     "First comment",
		Visibility:  "game",
	})
	core.AssertNoError(t, err, "Comment 1 creation should succeed")

	// Create reply via service (Player reply)
	comment2, err := messageService.CreateComment(context.Background(), core.CreateCommentRequest{
		GameID:      gameID,
		ParentID:    comment1.ID,
		AuthorID:    int32(playerUser.ID),
		CharacterID: playerChar.ID,
		Content:     "Reply to first comment",
		Visibility:  "game",
	})
	core.AssertNoError(t, err, "Comment 2 creation should succeed")

	t.Run("get_game_read_markers", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/games/"+strconv.Itoa(int(gameID))+"/read-markers", nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "Should return 200 OK")

		// Response is an array of read markers
		var response []interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON array")
	})

	t.Run("get_game_read_markers_unauthorized", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/games/"+strconv.Itoa(int(gameID))+"/read-markers", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 401, w.Code, "Should return 401 Unauthorized")
	})

	t.Run("get_posts_unread_info", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/games/"+strconv.Itoa(int(gameID))+"/posts-unread-info", nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "Should return 200 OK")

		// Response is an array of unread info
		var response []interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON array")
	})

	t.Run("get_posts_unread_info_unauthorized", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/games/"+strconv.Itoa(int(gameID))+"/posts-unread-info", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 401, w.Code, "Should return 401 Unauthorized")
	})

	t.Run("get_unread_comment_ids", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/games/"+strconv.Itoa(int(gameID))+"/unread-comment-ids", nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "Should return 200 OK")

		// Response is an array of IDs
		var response []interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON array")
		// Player should have unread comments from GM
		core.AssertTrue(t, len(response) > 0, "Player should have unread comments")
	})

	t.Run("get_unread_comment_ids_unauthorized", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/games/"+strconv.Itoa(int(gameID))+"/unread-comment-ids", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 401, w.Code, "Should return 401 Unauthorized")
	})

	t.Run("list_recent_comments_with_parents", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/games/"+strconv.Itoa(int(gameID))+"/comments/recent", nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "Should return 200 OK")

		// Response is an object with comments array and metadata
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON object")

		comments, ok := response["comments"].([]interface{})
		core.AssertTrue(t, ok, "Response should have comments array")
		core.AssertTrue(t, len(comments) > 0, "Should have recent comments")
	})

	t.Run("list_recent_comments_with_pagination", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/games/"+strconv.Itoa(int(gameID))+"/comments/recent?limit=1", nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "Should return 200 OK")

		// Response is an object with pagination
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON object")

		comments, ok := response["comments"].([]interface{})
		core.AssertTrue(t, ok, "Response should have comments array")
		core.AssertTrue(t, len(comments) <= 1, "Should respect limit parameter")
	})

	t.Run("list_recent_comments_unauthorized", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/games/"+strconv.Itoa(int(gameID))+"/comments/recent", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 401, w.Code, "Should return 401 Unauthorized")
	})

	t.Run("mark_post_read_then_check_unread", func(t *testing.T) {
		// Mark post as read
		markReadReq := httptest.NewRequest("POST", "/api/v1/games/"+strconv.Itoa(int(gameID))+"/posts/"+strconv.Itoa(int(post.ID))+"/mark-read", nil)
		markReadReq.Header.Set("Authorization", "Bearer "+playerToken)
		markReadW := httptest.NewRecorder()
		router.ServeHTTP(markReadW, markReadReq)
		core.AssertEqual(t, 200, markReadW.Code, "Mark read should succeed")

		// Now check unread comment IDs - after marking as read
		req := httptest.NewRequest("GET", "/api/v1/games/"+strconv.Itoa(int(gameID))+"/unread-comment-ids", nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "Should return 200 OK")

		// Response is an array
		var unreadIDs []interface{}
		err := json.Unmarshal(w.Body.Bytes(), &unreadIDs)
		core.AssertNoError(t, err, "Response should be valid JSON array")

		// Player's own comment (comment2) should not be in unread list
		for _, id := range unreadIDs {
			idFloat, ok := id.(float64)
			if ok {
				core.AssertTrue(t, int32(idFloat) != comment2.ID, "Player's own comment should not be unread")
			}
		}
	})
}
