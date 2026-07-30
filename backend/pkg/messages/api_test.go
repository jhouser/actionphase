package messages

import (
	"actionphase/pkg/core"
	dbmodels "actionphase/pkg/db/models"
	db "actionphase/pkg/db/services"
	"actionphase/pkg/db/services/messages"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

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

// setupMessageAPITestRouter creates a test router with message routes
// Note: call t.Setenv("REQUIRE_EMAIL_VERIFICATION", "false") before this if testing CreatePost/CreateComment
func setupMessageAPITestRouter(app *core.App, testDB *core.TestDatabase) *chi.Mux {
	tokenAuth := jwtauth.New("HS256", []byte(app.Config.JWT.Secret), nil)
	userService := &db.UserService{DB: testDB.Pool, Logger: app.ObsLogger}

	r := chi.NewRouter()

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/games", func(r chi.Router) {
			messageHandler := Handler{
				App:            app,
				UserService:    &db.UserService{DB: testDB.Pool, Logger: app.ObsLogger},
				MessageService: &messages.MessageService{DB: testDB.Pool, Logger: app.ObsLogger, Metrics: app.Observability.OTELMetrics},
			}

			r.Group(func(r chi.Router) {
				r.Use(jwtauth.Verifier(tokenAuth))
				r.Use(jwtauth.Authenticator(tokenAuth))
				r.Use(core.RequireAuthenticationMiddleware(userService))

				// Post routes
				r.With(core.RequireEmailVerificationMiddleware(app.Pool)).Post("/{gameId}/posts", messageHandler.CreatePost)
				r.Patch("/{gameId}/posts/{postId}", messageHandler.UpdatePost)
				r.With(core.RequireEmailVerificationMiddleware(app.Pool)).Post("/{gameId}/posts/{postId}/comments", messageHandler.CreateComment)
				r.Patch("/{gameId}/posts/{postId}/comments/{commentId}", messageHandler.UpdateComment)
				r.Delete("/{gameId}/posts/{postId}/comments/{commentId}", messageHandler.DeleteComment)
				r.Get("/{gameId}/posts/{postId}/comments-with-threads", messageHandler.GetPostCommentsWithThreads)
				r.Get("/{gameId}/messages/{messageId}/thread-context", messageHandler.GetMessageThreadContext)
				r.Post("/{gameId}/posts/{postId}/comments/{commentId}/toggle-read", messageHandler.ToggleCommentRead)
				r.Get("/{gameId}/manual-read-comment-ids", messageHandler.GetManualReadCommentIDs)
				r.Post("/{gameId}/phases/{phaseId}/mark-all-comments-read", messageHandler.MarkAllCommentsRead)
				r.Get("/{gameId}/comments/recent", messageHandler.ListRecentCommentsWithParents)
			})
		})

		r.Route("/characters", func(r chi.Router) {
			messageHandler2 := Handler{
				App:            app,
				UserService:    &db.UserService{DB: testDB.Pool, Logger: app.ObsLogger},
				MessageService: &messages.MessageService{DB: testDB.Pool, Logger: app.ObsLogger, Metrics: app.Observability.OTELMetrics},
			}
			r.Group(func(r chi.Router) {
				r.Use(jwtauth.Verifier(tokenAuth))
				r.Use(jwtauth.Authenticator(tokenAuth))
				r.Use(core.RequireAuthenticationMiddleware(userService))

				r.Get("/{id}/comments", messageHandler2.GetCharacterComments)
			})
		})
	})

	return r
}

// TestMessageAPI_UpdatePost tests the PATCH /games/{gameId}/posts/{postId} endpoint
func TestMessageAPI_UpdatePost(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "messages", "characters", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupMessageAPITestRouter(app, testDB)

	// Create test users
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")

	// Create test game
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	// Setup services
	messageService := &messages.MessageService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Add player as participant
	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	core.AssertNoError(t, err, "Should add player as participant")

	// Create character for player
	playerChar, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player.ID)),
		Name:          "Player Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Should create player character")

	// Create character for GM
	gmChar, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(gm.ID)),
		Name:          "GM Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Should create GM character")

	// Create a test post by player
	post, err := messageService.CreatePost(context.Background(), core.CreatePostRequest{
		GameID:      game.ID,
		PhaseID:     nil,
		AuthorID:    int32(player.ID),
		CharacterID: playerChar.ID,
		Content:     "Original post content",
		Visibility:  "game",
	})
	core.AssertNoError(t, err, "Should create test post")

	// Create another post by GM for negative testing
	gmPost, err := messageService.CreatePost(context.Background(), core.CreatePostRequest{
		GameID:      game.ID,
		PhaseID:     nil,
		AuthorID:    int32(gm.ID),
		CharacterID: gmChar.ID,
		Content:     "GM's post",
		Visibility:  "game",
	})
	core.AssertNoError(t, err, "Should create GM's test post")

	// Get tokens for authentication
	userToken, err := core.CreateTestJWTTokenForUser(app, player)
	core.AssertNoError(t, err, "Should generate user JWT")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	core.AssertNoError(t, err, "Should generate GM JWT")

	testCases := []struct {
		name           string
		postID         int32
		token          string
		requestBody    UpdatePostRequest
		expectedStatus int
		description    string
	}{
		{
			name:   "author_can_edit_own_post",
			postID: post.ID,
			token:  userToken,
			requestBody: UpdatePostRequest{
				Content: "Updated content by author",
			},
			expectedStatus: 200,
			description:    "Author should be able to edit their own post",
		},
		{
			name:   "non_author_cannot_edit_post",
			postID: post.ID,
			token:  gmToken,
			requestBody: UpdatePostRequest{
				Content: "Trying to edit someone else's post",
			},
			expectedStatus: 403,
			description:    "Non-author should not be able to edit post",
		},
		{
			name:   "nonexistent_post",
			postID: 99999,
			token:  userToken,
			requestBody: UpdatePostRequest{
				Content: "Trying to edit nonexistent post",
			},
			expectedStatus: 404,
			description:    "Should return 404 for non-existent post",
		},
		{
			name:   "empty_content",
			postID: post.ID,
			token:  userToken,
			requestBody: UpdatePostRequest{
				Content: "",
			},
			expectedStatus: 400,
			description:    "Should reject empty content",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bodyBytes, err := json.Marshal(tc.requestBody)
			core.AssertNoError(t, err, "Should marshal request body")

			url := "/api/v1/games/" + strconv.Itoa(int(game.ID)) + "/posts/" + strconv.Itoa(int(tc.postID))
			req := httptest.NewRequest("PATCH", url, bytes.NewReader(bodyBytes))
			req.Header.Set("Authorization", "Bearer "+tc.token)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			core.AssertEqual(t, tc.expectedStatus, w.Code, tc.description)

			// Verify response structure for successful requests
			if w.Code == 200 {
				var response core.MessageWithDetails
				err := json.Unmarshal(w.Body.Bytes(), &response)
				core.AssertNoError(t, err, "Response should be valid JSON")
				core.AssertEqual(t, tc.requestBody.Content, response.Content, "Content should be updated")
				core.AssertEqual(t, true, response.IsEdited, "IsEdited should be true")
			}
		})
	}

	// Additional test: Verify edit history tracking
	t.Run("edit_history_tracking", func(t *testing.T) {
		// First edit
		bodyBytes, _ := json.Marshal(UpdatePostRequest{Content: "First edit"})
		url := "/api/v1/games/" + strconv.Itoa(int(game.ID)) + "/posts/" + strconv.Itoa(int(gmPost.ID))
		req := httptest.NewRequest("PATCH", url, bytes.NewReader(bodyBytes))
		req.Header.Set("Authorization", "Bearer "+gmToken)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		core.AssertEqual(t, 200, w.Code, "First edit should succeed")

		var firstEdit core.MessageWithDetails
		json.Unmarshal(w.Body.Bytes(), &firstEdit)

		// Second edit
		bodyBytes, _ = json.Marshal(UpdatePostRequest{Content: "Second edit"})
		req = httptest.NewRequest("PATCH", url, bytes.NewReader(bodyBytes))
		req.Header.Set("Authorization", "Bearer "+gmToken)
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()

		router.ServeHTTP(w, req)
		core.AssertEqual(t, 200, w.Code, "Second edit should succeed")

		var secondEdit core.MessageWithDetails
		json.Unmarshal(w.Body.Bytes(), &secondEdit)

		// Note: edit_count is tracked in the database but not exposed in MessageWithDetails
		// The service-layer test verifies edit_count tracking
		core.AssertEqual(t, true, secondEdit.IsEdited, "Should be marked as edited")
		core.AssertEqual(t, "Second edit", secondEdit.Content, "Should have latest content")
	})
}

// TestMessageAPI_CreatePost tests POST /api/v1/games/{gameId}/posts
func TestMessageAPI_CreatePost(t *testing.T) {
	t.Setenv("REQUIRE_EMAIL_VERIFICATION", "false")

	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "messages", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupMessageAPITestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	gmChar, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(gm.ID)),
		Name:          "GM Character",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	playerChar, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player.ID)),
		Name:          "Player Character",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	t.Run("GM creates post successfully", func(t *testing.T) {
		body := CreatePostRequest{
			CharacterID: gmChar.ID,
			Content:     "A post from the GM.",
		}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/posts", game.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Equal(t, "A post from the GM.", response["content"])
		assert.Equal(t, "post", response["message_type"])
	})

	t.Run("non-GM player cannot create post", func(t *testing.T) {
		body := CreatePostRequest{
			CharacterID: playerChar.ID,
			Content:     "Player trying to post.",
		}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/posts", game.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

// TestMessageAPI_CreatePost_NotifiesParticipants verifies that creating a GM post
// fires a common_room_post notification for each participant except the GM.
func TestMessageAPI_CreatePost_NotifiesParticipants(t *testing.T) {
	t.Setenv("REQUIRE_EMAIL_VERIFICATION", "false")

	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "notifications", "messages", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupMessageAPITestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm-notif", "gm-notif@example.com")
	player1 := testDB.CreateTestUser(t, "player1-notif", "player1-notif@example.com")
	player2 := testDB.CreateTestUser(t, "player2-notif", "player2-notif@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Notif Test Game")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player1.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player2.ID), "player")
	require.NoError(t, err)

	gmChar, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(gm.ID)),
		Name:          "GM Char",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	body := CreatePostRequest{CharacterID: gmChar.ID, Content: "Phase one begins!"}
	bodyJSON, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/posts", game.ID), bytes.NewBuffer(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	// Give the fire-and-forget goroutine time to complete
	time.Sleep(100 * time.Millisecond)

	// Each participant except the GM should have a notification
	ctx := context.Background()
	for _, playerID := range []int{player1.ID, player2.ID} {
		var count int
		err := testDB.Pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND type = 'common_room_post' AND game_id = $2`,
			playerID, game.ID,
		).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "player %d should have a common_room_post notification", playerID)
	}

	// GM should NOT receive a notification for their own post
	var gmCount int
	err = testDB.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND type = 'common_room_post'`,
		gm.ID,
	).Scan(&gmCount)
	require.NoError(t, err)
	assert.Equal(t, 0, gmCount, "GM should not receive notification for their own post")
}

// TestMessageAPI_CreateComment tests POST /api/v1/games/{gameId}/posts/{postId}/comments
func TestMessageAPI_CreateComment(t *testing.T) {
	t.Setenv("REQUIRE_EMAIL_VERIFICATION", "false")

	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "messages", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupMessageAPITestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	messageService := &messages.MessageService{DB: testDB.Pool, Logger: app.ObsLogger}

	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	gmChar, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(gm.ID)),
		Name:          "GM Character",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	playerChar, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player.ID)),
		Name:          "Player Character",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	// Create a post to comment on
	post, err := messageService.CreatePost(context.Background(), core.CreatePostRequest{
		GameID:      game.ID,
		AuthorID:    int32(gm.ID),
		CharacterID: gmChar.ID,
		Content:     "A GM announcement.",
		Visibility:  "game",
	})
	require.NoError(t, err)

	t.Run("player creates comment on post", func(t *testing.T) {
		body := CreateCommentRequest{
			CharacterID: playerChar.ID,
			Content:     "Great post!",
		}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/posts/%d/comments", game.ID, post.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Equal(t, "Great post!", response["content"])
		assert.Equal(t, "comment", response["message_type"])
	})

	t.Run("GM creates comment on post", func(t *testing.T) {
		body := CreateCommentRequest{
			CharacterID: gmChar.ID,
			Content:     "GM response.",
		}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/posts/%d/comments", game.ID, post.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
	})

	t.Run("returns 404 for non-existent post", func(t *testing.T) {
		body := CreateCommentRequest{
			CharacterID: playerChar.ID,
			Content:     "Commenting on nothing.",
		}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/posts/99999/comments", game.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.NotEqual(t, http.StatusOK, rec.Code)
	})
}

// TestMessageAPI_DeleteComment tests DELETE /api/v1/games/{gameId}/posts/{postId}/comments/{commentId}
func TestMessageAPI_DeleteComment(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "messages", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	otherPlayer := testDB.CreateTestUser(t, "other", "other@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)
	otherToken, err := core.CreateTestJWTTokenForUser(app, otherPlayer)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	messageService := &messages.MessageService{DB: testDB.Pool, Logger: app.ObsLogger}

	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(otherPlayer.ID), "player")
	require.NoError(t, err)

	gmChar, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(gm.ID)),
		Name:          "GM Character",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	playerChar, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player.ID)),
		Name:          "Player Character",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	post, err := messageService.CreatePost(context.Background(), core.CreatePostRequest{
		GameID:      game.ID,
		AuthorID:    int32(gm.ID),
		CharacterID: gmChar.ID,
		Content:     "Post to comment on.",
		Visibility:  "game",
	})
	require.NoError(t, err)

	comment, err := messageService.CreateComment(context.Background(), core.CreateCommentRequest{
		GameID:      game.ID,
		AuthorID:    int32(player.ID),
		CharacterID: playerChar.ID,
		Content:     "Player's comment.",
		ParentID:    post.ID,
		Visibility:  "game",
	})
	require.NoError(t, err)

	// Create router AFTER t.Setenv since RequireEmailVerificationMiddleware reads env at setup time
	t.Setenv("REQUIRE_EMAIL_VERIFICATION", "false")
	router := setupMessageAPITestRouter(app, testDB)

	t.Run("other player cannot delete someone else's comment", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/games/%d/posts/%d/comments/%d", game.ID, post.ID, comment.ID), nil)
		req.Header.Set("Authorization", "Bearer "+otherToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("GM can delete any comment", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/games/%d/posts/%d/comments/%d", game.ID, post.ID, comment.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Equal(t, "Comment deleted successfully", response["message"])
	})

	t.Run("author can delete own comment", func(t *testing.T) {
		newComment, err := messageService.CreateComment(context.Background(), core.CreateCommentRequest{
			GameID:      game.ID,
			AuthorID:    int32(player.ID),
			CharacterID: playerChar.ID,
			Content:     "Another comment to delete.",
			ParentID:    post.ID,
			Visibility:  "game",
		})
		require.NoError(t, err)

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/games/%d/posts/%d/comments/%d", game.ID, post.ID, newComment.ID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

// TestMessageAPI_GetPostCommentsWithThreads tests GET /api/v1/games/{gameId}/posts/{postId}/comments-with-threads
func TestMessageAPI_GetPostCommentsWithThreads(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "messages", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupMessageAPITestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	messageService := &messages.MessageService{DB: testDB.Pool, Logger: app.ObsLogger}

	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	gmChar, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID: game.ID, UserID: int32Ptr(int32(gm.ID)), Name: "GM Char", CharacterType: "player_character",
	})
	require.NoError(t, err)
	playerChar, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID: game.ID, UserID: int32Ptr(int32(player.ID)), Name: "Player Char", CharacterType: "player_character",
	})
	require.NoError(t, err)

	post, err := messageService.CreatePost(context.Background(), core.CreatePostRequest{
		GameID: game.ID, AuthorID: int32(gm.ID), CharacterID: gmChar.ID, Content: "Post with comments.", Visibility: "game",
	})
	require.NoError(t, err)
	_, err = messageService.CreateComment(context.Background(), core.CreateCommentRequest{
		GameID: game.ID, ParentID: post.ID, AuthorID: int32(player.ID), CharacterID: playerChar.ID, Content: "A reply.", Visibility: "game",
	})
	require.NoError(t, err)

	t.Run("player retrieves comments with threads", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/posts/%d/comments-with-threads", game.ID, post.ID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.NotNil(t, response["comments"])
	})

	t.Run("GM retrieves comments with threads", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/posts/%d/comments-with-threads", game.ID, post.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("returns 400 for invalid limit parameter", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/posts/%d/comments-with-threads?limit=999", game.ID, post.ID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

// TestMessageAPI_GetMessageThreadContext tests GET /games/{gameId}/messages/{messageId}/thread-context,
// the single-request deep-link endpoint that returns a target comment plus a bounded
// slice of its ancestor chain. Builds a nested thread post → c1 → c2 → c3 and deep-links
// to c3, verifying the chain assembly, root_post_id, has_full_thread trimming flag, and
// the max_parents validation the frontend depends on.
func TestMessageAPI_GetMessageThreadContext(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "messages", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupMessageAPITestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	messageService := &messages.MessageService{DB: testDB.Pool, Logger: app.ObsLogger}

	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	gmChar, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID: game.ID, UserID: int32Ptr(int32(gm.ID)), Name: "GM Char", CharacterType: "player_character",
	})
	require.NoError(t, err)
	playerChar, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID: game.ID, UserID: int32Ptr(int32(player.ID)), Name: "Player Char", CharacterType: "player_character",
	})
	require.NoError(t, err)

	// Build a nested thread: post → c1 → c2 → c3
	post, err := messageService.CreatePost(context.Background(), core.CreatePostRequest{
		GameID: game.ID, AuthorID: int32(gm.ID), CharacterID: gmChar.ID, Content: "Root post.", Visibility: "game",
	})
	require.NoError(t, err)
	c1, err := messageService.CreateComment(context.Background(), core.CreateCommentRequest{
		GameID: game.ID, ParentID: post.ID, RootPostID: post.ID, AuthorID: int32(player.ID), CharacterID: playerChar.ID, Content: "Comment one.", Visibility: "game",
	})
	require.NoError(t, err)
	c2, err := messageService.CreateComment(context.Background(), core.CreateCommentRequest{
		GameID: game.ID, ParentID: c1.ID, RootPostID: post.ID, AuthorID: int32(gm.ID), CharacterID: gmChar.ID, Content: "Comment two.", Visibility: "game",
	})
	require.NoError(t, err)
	c3, err := messageService.CreateComment(context.Background(), core.CreateCommentRequest{
		GameID: game.ID, ParentID: c2.ID, RootPostID: post.ID, AuthorID: int32(player.ID), CharacterID: playerChar.ID, Content: "Comment three.", Visibility: "game",
	})
	require.NoError(t, err)

	t.Run("deep-links to nested comment with full ancestor chain", func(t *testing.T) {
		// max_parents large enough to reach the root, so nothing is trimmed.
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/messages/%d/thread-context?max_parents=10", game.ID, c3.ID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response MessageThreadContextResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))

		// root_post_id points at the top-level post regardless of trimming.
		assert.Equal(t, post.ID, response.RootPostID)
		assert.True(t, response.HasFullThread, "chain reaches the root, so has_full_thread should be true")

		// Chain is ordered parent-to-child and ends at the target comment.
		require.NotEmpty(t, response.Chain)
		last := response.Chain[len(response.Chain)-1]
		assert.Equal(t, c3.ID, last.ID, "last element of the chain is the deep-linked target")
		assert.Equal(t, "Comment three.", last.Content)
	})

	t.Run("trims the chain when max_parents is smaller than the depth", func(t *testing.T) {
		// Only the target + 1 nearest ancestor; the chain no longer reaches the root.
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/messages/%d/thread-context?max_parents=1", game.ID, c3.ID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response MessageThreadContextResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))

		// root_post_id is still reported even though the chain is trimmed above.
		assert.Equal(t, post.ID, response.RootPostID)
		assert.False(t, response.HasFullThread, "chain was trimmed, so has_full_thread should be false")
		assert.Len(t, response.Chain, 2, "target + 1 ancestor")
		assert.Equal(t, c3.ID, response.Chain[len(response.Chain)-1].ID)
	})

	t.Run("returns 400 for out-of-range max_parents", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/messages/%d/thread-context?max_parents=999", game.ID, c3.ID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

// TestMessageAPI_ToggleCommentRead tests POST /api/v1/games/{gameId}/posts/{postId}/comments/{commentId}/toggle-read
func TestMessageAPI_ToggleCommentRead(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "messages", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupMessageAPITestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	messageService := &messages.MessageService{DB: testDB.Pool, Logger: app.ObsLogger}

	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	gmChar, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID: game.ID, UserID: int32Ptr(int32(gm.ID)), Name: "GM Char", CharacterType: "player_character",
	})
	require.NoError(t, err)
	playerChar, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID: game.ID, UserID: int32Ptr(int32(player.ID)), Name: "Player Char", CharacterType: "player_character",
	})
	require.NoError(t, err)

	post, err := messageService.CreatePost(context.Background(), core.CreatePostRequest{
		GameID: game.ID, AuthorID: int32(gm.ID), CharacterID: gmChar.ID, Content: "Announcement.", Visibility: "game",
	})
	require.NoError(t, err)
	comment, err := messageService.CreateComment(context.Background(), core.CreateCommentRequest{
		GameID: game.ID, ParentID: post.ID, AuthorID: int32(gm.ID), CharacterID: gmChar.ID, Content: "Response.", Visibility: "game",
	})
	require.NoError(t, err)

	_ = playerChar

	t.Run("player marks comment as read", func(t *testing.T) {
		body := map[string]bool{"read": true}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("POST",
			fmt.Sprintf("/api/v1/games/%d/posts/%d/comments/%d/toggle-read", game.ID, post.ID, comment.ID),
			bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)
	})

	t.Run("player marks comment as unread (idempotent toggle)", func(t *testing.T) {
		body := map[string]bool{"read": false}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("POST",
			fmt.Sprintf("/api/v1/games/%d/posts/%d/comments/%d/toggle-read", game.ID, post.ID, comment.ID),
			bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)
	})
}

// TestMessageAPI_MarkAllCommentsRead tests POST /api/v1/games/{gameId}/phases/{phaseId}/mark-all-comments-read
func TestMessageAPI_MarkAllCommentsRead(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "messages", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupMessageAPITestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	messageService := &messages.MessageService{DB: testDB.Pool, Logger: app.ObsLogger}

	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	gmChar, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID: game.ID, UserID: int32Ptr(int32(gm.ID)), Name: "GM Char", CharacterType: "player_character",
	})
	require.NoError(t, err)

	phase := testDB.CreateTestPhase(t, game.ID, "action", "Phase 1")

	post, err := messageService.CreatePost(context.Background(), core.CreatePostRequest{
		GameID: game.ID, PhaseID: &phase.ID, AuthorID: int32(gm.ID), CharacterID: gmChar.ID, Content: "Announcement.", Visibility: "game",
	})
	require.NoError(t, err)
	comment1, err := messageService.CreateComment(context.Background(), core.CreateCommentRequest{
		GameID: game.ID, PhaseID: &phase.ID, ParentID: post.ID, RootPostID: post.ID,
		AuthorID: int32(gm.ID), CharacterID: gmChar.ID, Content: "Response 1.", Visibility: "game",
	})
	require.NoError(t, err)
	comment2, err := messageService.CreateComment(context.Background(), core.CreateCommentRequest{
		GameID: game.ID, PhaseID: &phase.ID, ParentID: post.ID, RootPostID: post.ID,
		AuthorID: int32(gm.ID), CharacterID: gmChar.ID, Content: "Response 2.", Visibility: "game",
	})
	require.NoError(t, err)

	t.Run("player marks all comments in phase as read", func(t *testing.T) {
		req := httptest.NewRequest("POST",
			fmt.Sprintf("/api/v1/games/%d/phases/%d/mark-all-comments-read", game.ID, phase.ID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)

		reads, err := messageService.GetManualReadCommentIDsForGame(context.Background(), int32(player.ID), game.ID)
		require.NoError(t, err)
		readIDs := []int32{}
		for _, r := range reads {
			readIDs = append(readIDs, r.ReadCommentIDs...)
		}
		assert.Contains(t, readIDs, comment1.ID)
		assert.Contains(t, readIDs, comment2.ID)
	})

	t.Run("rejects unauthenticated request", func(t *testing.T) {
		req := httptest.NewRequest("POST",
			fmt.Sprintf("/api/v1/games/%d/phases/%d/mark-all-comments-read", game.ID, phase.ID), nil)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

// TestMessageAPI_GetManualReadCommentIDs tests GET /api/v1/games/{gameId}/manual-read-comment-ids
func TestMessageAPI_GetManualReadCommentIDs(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "messages", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupMessageAPITestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	messageService := &messages.MessageService{DB: testDB.Pool, Logger: app.ObsLogger}

	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	gmChar, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID: game.ID, UserID: int32Ptr(int32(gm.ID)), Name: "GM Char", CharacterType: "player_character",
	})
	require.NoError(t, err)

	post, err := messageService.CreatePost(context.Background(), core.CreatePostRequest{
		GameID: game.ID, AuthorID: int32(gm.ID), CharacterID: gmChar.ID, Content: "Post.", Visibility: "game",
	})
	require.NoError(t, err)
	comment, err := messageService.CreateComment(context.Background(), core.CreateCommentRequest{
		GameID: game.ID, ParentID: post.ID, AuthorID: int32(gm.ID), CharacterID: gmChar.ID, Content: "Comment.", Visibility: "game",
	})
	require.NoError(t, err)

	// Mark the comment as manually read
	err = messageService.ToggleCommentRead(context.Background(), int32(player.ID), game.ID, post.ID, comment.ID, true)
	require.NoError(t, err)

	t.Run("returns comment IDs marked as read by player", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/manual-read-comment-ids", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		// Response is an array of {post_id, read_comment_ids} objects grouped by post
		var response []map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		// Should have one entry (the post that contains the manually-read comment)
		require.Len(t, response, 1)
		readIDs := response[0]["read_comment_ids"].([]interface{})
		assert.Len(t, readIDs, 1)
		assert.Equal(t, float64(comment.ID), readIDs[0])
	})

	t.Run("returns empty array for game with no manual reads", func(t *testing.T) {
		otherGM := testDB.CreateTestUser(t, "othergm", "othergm@example.com")
		otherGMToken, err := core.CreateTestJWTTokenForUser(app, otherGM)
		require.NoError(t, err)
		emptyGame := testDB.CreateTestGame(t, int32(otherGM.ID), "Empty Game")

		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/manual-read-comment-ids", emptyGame.ID), nil)
		req.Header.Set("Authorization", "Bearer "+otherGMToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response []map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Len(t, response, 0)
	})
}

// TestMessageAPI_GetCharacterComments tests GET /api/v1/characters/{id}/comments
func TestMessageAPI_GetCharacterComments(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "messages", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupMessageAPITestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	messageService := &messages.MessageService{DB: testDB.Pool, Logger: app.ObsLogger}

	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	playerChar, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID: game.ID, UserID: int32Ptr(int32(player.ID)), Name: "Player Char", CharacterType: "player_character",
	})
	require.NoError(t, err)
	gmChar, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID: game.ID, UserID: int32Ptr(int32(gm.ID)), Name: "GM Char", CharacterType: "player_character",
	})
	require.NoError(t, err)

	// Create a post and have playerChar comment on it
	post, err := messageService.CreatePost(context.Background(), core.CreatePostRequest{
		GameID: game.ID, AuthorID: int32(gm.ID), CharacterID: gmChar.ID, Content: "Announcement.", Visibility: "game",
	})
	require.NoError(t, err)
	_, err = messageService.CreateComment(context.Background(), core.CreateCommentRequest{
		GameID: game.ID, ParentID: post.ID, AuthorID: int32(player.ID), CharacterID: playerChar.ID,
		Content: "Character's comment.", Visibility: "game",
	})
	require.NoError(t, err)

	t.Run("retrieves comments by character", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/characters/%d/comments", playerChar.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		items := response["messages"].([]interface{})
		assert.Len(t, items, 1)
		item := items[0].(map[string]interface{})
		assert.Equal(t, "Character's comment.", item["content"])
	})

	t.Run("returns empty list for character with no posts or comments", func(t *testing.T) {
		otherChar, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
			GameID: game.ID, UserID: int32Ptr(int32(gm.ID)), Name: "Silent Char", CharacterType: "player_character",
		})
		require.NoError(t, err)

		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/characters/%d/comments", otherChar.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		items := response["messages"].([]interface{})
		assert.Len(t, items, 0)
	})
}

// TestMessageAPI_GetCharacterComments_AnonymousGame tests that author_username is hidden
// for players in anonymous games when viewing character comment history.
func TestMessageAPI_GetCharacterComments_AnonymousGame(t *testing.T) {
	if os.Getenv("SKIP_DB_TESTS") == "true" {
		t.Skip("Skipping integration test - SKIP_DB_TESTS=true")
	}

	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "messages", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupMessageAPITestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "anon_gm2", "anon_gm2@example.com")
	player := testDB.CreateTestUser(t, "anon_player2", "anon_player2@example.com")
	otherPlayer := testDB.CreateTestUser(t, "anon_other2", "anon_other2@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	otherPlayerToken, err := core.CreateTestJWTTokenForUser(app, otherPlayer)
	require.NoError(t, err)

	queries := dbmodels.New(testDB.Pool)
	ctx := context.Background()

	anonGame, err := queries.CreateGame(ctx, dbmodels.CreateGameParams{
		Title:       "Anonymous Character Test Game",
		Description: pgtype.Text{String: "Test", Valid: true},
		GmUserID:    int32(gm.ID),
		IsAnonymous: true,
	})
	require.NoError(t, err)

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	messageService := &messages.MessageService{DB: testDB.Pool, Logger: app.ObsLogger}

	_, err = gameService.AddGameParticipant(ctx, anonGame.ID, int32(player.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(ctx, anonGame.ID, int32(otherPlayer.ID), "player")
	require.NoError(t, err)

	gmChar, err := characterService.CreateCharacter(ctx, db.CreateCharacterRequest{
		GameID: anonGame.ID, UserID: int32Ptr(int32(gm.ID)), Name: "GM Char", CharacterType: "player_character",
	})
	require.NoError(t, err)
	playerChar, err := characterService.CreateCharacter(ctx, db.CreateCharacterRequest{
		GameID: anonGame.ID, UserID: int32Ptr(int32(player.ID)), Name: "Player Char", CharacterType: "player_character",
	})
	require.NoError(t, err)

	post, err := messageService.CreatePost(ctx, core.CreatePostRequest{
		GameID: anonGame.ID, AuthorID: int32(gm.ID), CharacterID: gmChar.ID, Content: "A post.", Visibility: "game",
	})
	require.NoError(t, err)
	_, err = messageService.CreateComment(ctx, core.CreateCommentRequest{
		GameID: anonGame.ID, ParentID: post.ID, AuthorID: int32(player.ID), CharacterID: playerChar.ID,
		Content: "Player's comment.", Visibility: "game",
	})
	require.NoError(t, err)

	t.Run("GM sees author_username in anonymous game", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/characters/%d/comments", playerChar.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		items := response["messages"].([]interface{})
		require.Len(t, items, 1)
		item := items[0].(map[string]interface{})
		assert.NotEmpty(t, item["author_username"], "GM should see the real username")
	})

	t.Run("other player sees empty author_username in anonymous game", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/characters/%d/comments", playerChar.ID), nil)
		req.Header.Set("Authorization", "Bearer "+otherPlayerToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		items := response["messages"].([]interface{})
		require.Len(t, items, 1)
		item := items[0].(map[string]interface{})
		assert.Equal(t, "", item["author_username"], "Player should not see username in anonymous game")
	})
}

// TestMessageAPI_UpdateComment tests PATCH /api/v1/games/{gameId}/posts/{postId}/comments/{commentId}
func TestMessageAPI_UpdateComment(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "messages", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupMessageAPITestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	other := testDB.CreateTestUser(t, "other", "other@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)
	otherToken, err := core.CreateTestJWTTokenForUser(app, other)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	messageService := &messages.MessageService{DB: testDB.Pool, Logger: app.ObsLogger}

	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(other.ID), "player")
	require.NoError(t, err)

	gmChar, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID: game.ID, UserID: int32Ptr(int32(gm.ID)), Name: "GM Char", CharacterType: "player_character",
	})
	require.NoError(t, err)
	playerChar, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID: game.ID, UserID: int32Ptr(int32(player.ID)), Name: "Player Char", CharacterType: "player_character",
	})
	require.NoError(t, err)

	post, err := messageService.CreatePost(context.Background(), core.CreatePostRequest{
		GameID: game.ID, AuthorID: int32(gm.ID), CharacterID: gmChar.ID, Content: "Announcement.", Visibility: "game",
	})
	require.NoError(t, err)

	comment, err := messageService.CreateComment(context.Background(), core.CreateCommentRequest{
		GameID: game.ID, ParentID: post.ID, AuthorID: int32(player.ID), CharacterID: playerChar.ID,
		Content: "Original comment.", Visibility: "game",
	})
	require.NoError(t, err)

	commentURL := fmt.Sprintf("/api/v1/games/%d/posts/%d/comments/%d", game.ID, post.ID, comment.ID)

	t.Run("author can edit own comment", func(t *testing.T) {
		body := UpdateCommentRequest{Content: "Edited comment."}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("PATCH", commentURL, bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Equal(t, "Edited comment.", response["content"])
	})

	t.Run("non-author cannot edit comment", func(t *testing.T) {
		body := UpdateCommentRequest{Content: "Sneaky edit."}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("PATCH", commentURL, bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+otherToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("GM cannot edit player's comment", func(t *testing.T) {
		body := UpdateCommentRequest{Content: "GM editing player comment."}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("PATCH", commentURL, bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

}

// TestMessageAPI_ListRecentCommentsWithParents tests GET /api/v1/games/{gameId}/comments/recent
func TestMessageAPI_ListRecentCommentsWithParents(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "messages", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupMessageAPITestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	messageService := &messages.MessageService{DB: testDB.Pool, Logger: app.ObsLogger}

	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	gmChar, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID: game.ID, UserID: int32Ptr(int32(gm.ID)), Name: "GM Char", CharacterType: "player_character",
	})
	require.NoError(t, err)
	playerChar, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID: game.ID, UserID: int32Ptr(int32(player.ID)), Name: "Player Char", CharacterType: "player_character",
	})
	require.NoError(t, err)

	post, err := messageService.CreatePost(context.Background(), core.CreatePostRequest{
		GameID: game.ID, AuthorID: int32(gm.ID), CharacterID: gmChar.ID, Content: "Post.", Visibility: "game",
	})
	require.NoError(t, err)
	_, err = messageService.CreateComment(context.Background(), core.CreateCommentRequest{
		GameID: game.ID, ParentID: post.ID, AuthorID: int32(player.ID), CharacterID: playerChar.ID,
		Content: "A reply.", Visibility: "game",
	})
	require.NoError(t, err)

	recentURL := fmt.Sprintf("/api/v1/games/%d/comments/recent", game.ID)

	t.Run("returns comments with pagination metadata", func(t *testing.T) {
		req := httptest.NewRequest("GET", recentURL, nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.NotNil(t, response["comments"])
		pagination := response["pagination"].(map[string]interface{})
		assert.NotNil(t, pagination["limit"])
		assert.NotNil(t, pagination["offset"])
		assert.NotNil(t, pagination["total"])
	})

	t.Run("player can also retrieve recent comments", func(t *testing.T) {
		req := httptest.NewRequest("GET", recentURL, nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("returns 400 for invalid limit", func(t *testing.T) {
		req := httptest.NewRequest("GET", recentURL+"?limit=bad", nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("returns 400 for invalid offset", func(t *testing.T) {
		req := httptest.NewRequest("GET", recentURL+"?offset=-5", nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("returns 400 for zero limit", func(t *testing.T) {
		req := httptest.NewRequest("GET", recentURL+"?limit=0", nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

// TestMessageAPI_GetCharacterComments_InvalidParams tests invalid pagination params for GetCharacterComments
func TestMessageAPI_GetCharacterComments_InvalidParams(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "messages", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupMessageAPITestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	gmChar, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID: game.ID, UserID: int32Ptr(int32(gm.ID)), Name: "GM Char", CharacterType: "player_character",
	})
	require.NoError(t, err)

	baseURL := fmt.Sprintf("/api/v1/characters/%d/comments", gmChar.ID)

	t.Run("returns 400 for non-numeric limit", func(t *testing.T) {
		req := httptest.NewRequest("GET", baseURL+"?limit=abc", nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("returns 400 for negative offset", func(t *testing.T) {
		req := httptest.NewRequest("GET", baseURL+"?offset=-1", nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("returns 400 for zero limit", func(t *testing.T) {
		req := httptest.NewRequest("GET", baseURL+"?limit=0", nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}
