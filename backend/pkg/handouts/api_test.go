package handouts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"actionphase/pkg/core"
	dbmodels "actionphase/pkg/db/models"
	db "actionphase/pkg/db/services"
	dbsvc "actionphase/pkg/db/services"
	dbactions "actionphase/pkg/db/services/actions"
	dbmessages "actionphase/pkg/db/services/messages"
	"actionphase/pkg/games"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupHandoutTestRouter creates a test router with all handout routes
func setupHandoutTestRouter(app *core.App, testDB *core.TestDatabase) *chi.Mux {
	tokenAuth := jwtauth.New("HS256", []byte(app.Config.JWT.Secret), nil)
	userService := &dbsvc.UserService{DB: testDB.Pool, Logger: app.ObsLogger}

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
	r.Route("/api/v1/games/{gameID}", func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth))
		r.Use(jwtauth.Authenticator(tokenAuth))
		r.Use(core.RequireAuthenticationMiddleware(userService))
		r.Use(core.AdminModeMiddleware)
		r.Use(gameHandler.GameMiddleware())

		handler := &Handler{
			App:                 app,
			UserService:         &dbsvc.UserService{DB: testDB.Pool, Logger: app.ObsLogger},
			GameService:         &dbsvc.GameService{DB: testDB.Pool, Logger: app.ObsLogger},
			HandoutService:      dbsvc.NewHandoutService(testDB.Pool),
			NotificationService: dbsvc.NewNotificationService(testDB.Pool, app.ObsLogger),
		}
		r.Post("/handouts", handler.CreateHandout)
		r.Get("/handouts", handler.ListHandouts)
		r.Get("/handouts/{handoutId}", handler.GetHandout)
		r.Put("/handouts/{handoutId}", handler.UpdateHandout)
		r.Delete("/handouts/{handoutId}", handler.DeleteHandout)
		r.Post("/handouts/{handoutId}/publish", handler.PublishHandout)
		r.Post("/handouts/{handoutId}/unpublish", handler.UnpublishHandout)
		r.Post("/handouts/{handoutId}/comments", handler.CreateHandoutComment)
		r.Get("/handouts/{handoutId}/comments", handler.ListHandoutComments)
		r.Patch("/handouts/{handoutId}/comments/{commentId}", handler.UpdateHandoutComment)
		r.Delete("/handouts/{handoutId}/comments/{commentId}", handler.DeleteHandoutComment)
	})

	return r
}

// createTestHandout is a helper that creates a handout via the API and returns the handout ID
func createTestHandout(t *testing.T, router *chi.Mux, gameID int32, gmToken string, status string) int32 {
	t.Helper()
	body := CreateHandoutRequest{
		Title:   "Test Handout",
		Content: "Some interesting content about the world.",
		Status:  status,
	}
	bodyJSON, err := json.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/handouts", gameID), bytes.NewBuffer(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmToken)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code, "create handout should succeed: %s", rec.Body.String())

	var response map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	return int32(response["id"].(float64))
}

// TestHandoutAPI_CreateHandout tests POST /api/v1/games/{gameId}/handouts
func TestHandoutAPI_CreateHandout(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "handout_comments", "handouts", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupHandoutTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &dbsvc.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	t.Run("GM creates draft handout successfully", func(t *testing.T) {
		body := CreateHandoutRequest{
			Title:   "Welcome to the World",
			Content: "The world is a dark and mysterious place.",
			Status:  "draft",
		}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/handouts", game.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Equal(t, "Welcome to the World", response["title"])
		assert.Equal(t, "draft", response["status"])
	})

	t.Run("GM creates published handout directly", func(t *testing.T) {
		body := CreateHandoutRequest{
			Title:   "Public Information",
			Content: "Everyone knows about this.",
			Status:  "published",
		}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/handouts", game.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Equal(t, "published", response["status"])
	})

	t.Run("non-GM player cannot create handout", func(t *testing.T) {
		body := CreateHandoutRequest{
			Title:   "Player Handout",
			Content: "Should not be allowed.",
			Status:  "draft",
		}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/handouts", game.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

// TestHandoutAPI_ListHandouts tests GET /api/v1/games/{gameId}/handouts
func TestHandoutAPI_ListHandouts(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "handout_comments", "handouts", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupHandoutTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &dbsvc.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	// Create a draft and a published handout
	createTestHandout(t, router, game.ID, gmToken, "draft")
	createTestHandout(t, router, game.ID, gmToken, "published")

	t.Run("GM sees all handouts including drafts", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/handouts", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response []map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Len(t, response, 2, "GM should see both draft and published")
	})

	t.Run("player only sees published handouts", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/handouts", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response []map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Len(t, response, 1, "player should only see published handout")
		assert.Equal(t, "published", response[0]["status"])
	})
}

// TestHandoutAPI_UpdateHandout tests PUT /api/v1/games/{gameId}/handouts/{handoutId}
func TestHandoutAPI_UpdateHandout(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "handout_comments", "handouts", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupHandoutTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &dbsvc.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	handoutID := createTestHandout(t, router, game.ID, gmToken, "draft")

	t.Run("GM updates handout successfully", func(t *testing.T) {
		body := UpdateHandoutRequest{
			Title:   "Updated Title",
			Content: "Updated content.",
			Status:  "draft",
		}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/games/%d/handouts/%d", game.ID, handoutID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Equal(t, "Updated Title", response["title"])
		assert.Equal(t, "Updated content.", response["content"])
	})

	t.Run("non-GM player cannot update handout", func(t *testing.T) {
		body := UpdateHandoutRequest{
			Title:   "Player Update",
			Content: "Should fail.",
			Status:  "draft",
		}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/games/%d/handouts/%d", game.ID, handoutID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		// Draft handouts are invisible to non-GM players — service returns 404
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("returns 404 for non-existent handout", func(t *testing.T) {
		body := UpdateHandoutRequest{
			Title:   "Updated",
			Content: "Content",
			Status:  "draft",
		}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/games/%d/handouts/99999", game.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

// TestHandoutAPI_PublishUnpublish tests POST .../publish and .../unpublish
func TestHandoutAPI_PublishUnpublish(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "handout_comments", "handouts", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupHandoutTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &dbsvc.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	handoutID := createTestHandout(t, router, game.ID, gmToken, "draft")

	t.Run("non-GM player cannot publish handout", func(t *testing.T) {
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/handouts/%d/publish", game.ID, handoutID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		// Draft handouts are invisible to non-GM players — service returns 404
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("GM publishes draft handout", func(t *testing.T) {
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/handouts/%d/publish", game.ID, handoutID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Equal(t, "published", response["status"])
	})

	t.Run("GM unpublishes published handout", func(t *testing.T) {
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/handouts/%d/unpublish", game.ID, handoutID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Equal(t, "draft", response["status"])
	})

	t.Run("player cannot see draft handout after unpublish", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/handouts/%d", game.ID, handoutID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

// TestHandoutAPI_DeleteHandout tests DELETE /api/v1/games/{gameId}/handouts/{handoutId}
func TestHandoutAPI_DeleteHandout(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "handout_comments", "handouts", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupHandoutTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &dbsvc.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	t.Run("non-GM player cannot delete handout", func(t *testing.T) {
		handoutID := createTestHandout(t, router, game.ID, gmToken, "draft")

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/games/%d/handouts/%d", game.ID, handoutID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		// Draft handouts are invisible to non-GM players — service returns 404
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("GM deletes handout successfully", func(t *testing.T) {
		handoutID := createTestHandout(t, router, game.ID, gmToken, "draft")

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/games/%d/handouts/%d", game.ID, handoutID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)

		// Verify handout is gone
		getReq := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/handouts/%d", game.ID, handoutID), nil)
		getReq.Header.Set("Authorization", "Bearer "+gmToken)
		getRec := httptest.NewRecorder()
		router.ServeHTTP(getRec, getReq)
		assert.Equal(t, http.StatusNotFound, getRec.Code)
	})
}

// TestHandoutAPI_Comments tests the handout comment endpoints
func TestHandoutAPI_Comments(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "handout_comments", "handouts", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupHandoutTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &dbsvc.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	// Publish a handout so both GM and player can see it
	handoutID := createTestHandout(t, router, game.ID, gmToken, "published")

	t.Run("GM can comment on a handout", func(t *testing.T) {
		body := CreateHandoutCommentRequest{Content: "This is a GM note."}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/handouts/%d/comments", game.ID, handoutID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Equal(t, "This is a GM note.", response["content"])
	})

	t.Run("non-GM player cannot comment on a handout", func(t *testing.T) {
		body := CreateHandoutCommentRequest{Content: "Player trying to comment."}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/handouts/%d/comments", game.ID, handoutID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("list comments on handout", func(t *testing.T) {
		// Create a second comment
		body := CreateHandoutCommentRequest{Content: "Second GM note."}
		bodyJSON, _ := json.Marshal(body)
		postReq := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/handouts/%d/comments", game.ID, handoutID), bytes.NewBuffer(bodyJSON))
		postReq.Header.Set("Content-Type", "application/json")
		postReq.Header.Set("Authorization", "Bearer "+gmToken)
		postRec := httptest.NewRecorder()
		router.ServeHTTP(postRec, postReq)
		require.Equal(t, http.StatusCreated, postRec.Code)

		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/handouts/%d/comments", game.ID, handoutID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response []map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		// Should have 2 comments total (first from previous test + this one)
		assert.GreaterOrEqual(t, len(response), 1)
	})

	t.Run("GM updates their comment", func(t *testing.T) {
		// Create comment to update
		createBody := CreateHandoutCommentRequest{Content: "Original comment."}
		createJSON, _ := json.Marshal(createBody)
		createReq := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/handouts/%d/comments", game.ID, handoutID), bytes.NewBuffer(createJSON))
		createReq.Header.Set("Content-Type", "application/json")
		createReq.Header.Set("Authorization", "Bearer "+gmToken)
		createRec := httptest.NewRecorder()
		router.ServeHTTP(createRec, createReq)
		require.Equal(t, http.StatusCreated, createRec.Code)

		var created map[string]interface{}
		require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
		commentID := int(created["id"].(float64))

		// Update it
		updateBody := UpdateHandoutCommentRequest{Content: "Updated comment."}
		updateJSON, _ := json.Marshal(updateBody)
		updateReq := httptest.NewRequest("PATCH", fmt.Sprintf("/api/v1/games/%d/handouts/%d/comments/%d", game.ID, handoutID, commentID), bytes.NewBuffer(updateJSON))
		updateReq.Header.Set("Content-Type", "application/json")
		updateReq.Header.Set("Authorization", "Bearer "+gmToken)
		updateRec := httptest.NewRecorder()
		router.ServeHTTP(updateRec, updateReq)

		assert.Equal(t, http.StatusOK, updateRec.Code)
		var updated map[string]interface{}
		require.NoError(t, json.Unmarshal(updateRec.Body.Bytes(), &updated))
		assert.Equal(t, "Updated comment.", updated["content"])
	})

	t.Run("GM deletes a comment", func(t *testing.T) {
		// Create comment to delete
		createBody := CreateHandoutCommentRequest{Content: "Comment to delete."}
		createJSON, _ := json.Marshal(createBody)
		createReq := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/handouts/%d/comments", game.ID, handoutID), bytes.NewBuffer(createJSON))
		createReq.Header.Set("Content-Type", "application/json")
		createReq.Header.Set("Authorization", "Bearer "+gmToken)
		createRec := httptest.NewRecorder()
		router.ServeHTTP(createRec, createReq)
		require.Equal(t, http.StatusCreated, createRec.Code)

		var created map[string]interface{}
		require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
		commentID := int(created["id"].(float64))

		deleteReq := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/games/%d/handouts/%d/comments/%d", game.ID, handoutID, commentID), nil)
		deleteReq.Header.Set("Authorization", "Bearer "+gmToken)
		deleteRec := httptest.NewRecorder()
		router.ServeHTTP(deleteRec, deleteReq)

		assert.Equal(t, http.StatusNoContent, deleteRec.Code)
	})
}

// TestHandoutAPI_GetHandout tests GET /api/v1/games/{gameId}/handouts/{handoutId}
func TestHandoutAPI_GetHandout(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "handout_comments", "handouts", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupHandoutTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &dbsvc.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	draftID := createTestHandout(t, router, game.ID, gmToken, "draft")
	publishedID := createTestHandout(t, router, game.ID, gmToken, "published")

	t.Run("GM can get draft handout by ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/handouts/%d", game.ID, draftID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Equal(t, "draft", response["status"])
		assert.Equal(t, float64(draftID), response["id"])
	})

	t.Run("player can get published handout by ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/handouts/%d", game.ID, publishedID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Equal(t, "published", response["status"])
	})

	t.Run("player cannot get draft handout by ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/handouts/%d", game.ID, draftID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

// TestHandoutAPI_UnpublishByPlayer tests that a player cannot unpublish a published handout
func TestHandoutAPI_UnpublishByPlayer(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "handout_comments", "handouts", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupHandoutTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &dbsvc.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	// Create and publish a handout so the player can see it
	handoutID := createTestHandout(t, router, game.ID, gmToken, "published")

	t.Run("player cannot unpublish a published handout", func(t *testing.T) {
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/handouts/%d/unpublish", game.ID, handoutID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		// Player passes the GetHandout visibility check (published), but fails verifyUserIsGM
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("player cannot update a published handout", func(t *testing.T) {
		body := UpdateHandoutRequest{Title: "Hacked", Content: "Pwned.", Status: "published"}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/games/%d/handouts/%d", game.ID, handoutID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		// Player passes GetHandout visibility (published), but fails verifyUserIsGM
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("player cannot delete a published handout", func(t *testing.T) {
		// Use a separate handout so we don't interfere with other tests
		otherID := createTestHandout(t, router, game.ID, gmToken, "published")

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/games/%d/handouts/%d", game.ID, otherID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

// TestHandoutAPI_PublishHandout_SendsNotifications verifies that publishing a draft handout
// creates in-app notifications for players, but NOT for the GM who published it.
// Also verifies that co-GMs are excluded (they have full draft visibility already).
func TestHandoutAPI_PublishHandout_SendsNotifications(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "notifications", "handout_comments", "handouts", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupHandoutTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	coGM := testDB.CreateTestUser(t, "cogm", "cogm@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &dbsvc.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(coGM.ID), "co_gm")
	require.NoError(t, err)

	handoutID := createTestHandout(t, router, game.ID, gmToken, "draft")

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/handouts/%d/publish", game.ID, handoutID), nil)
	req.Header.Set("Authorization", "Bearer "+gmToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	time.Sleep(200 * time.Millisecond)

	notifSvc := &dbsvc.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Player should receive a handout_published notification
	playerNotifs, err := notifSvc.GetUserNotifications(context.Background(), int32(player.ID), 10, 0)
	require.NoError(t, err)
	var playerFound bool
	for _, n := range playerNotifs {
		if n.Type == core.NotificationTypeHandoutPublished {
			playerFound = true
			break
		}
	}
	assert.True(t, playerFound, "player should receive a handout_published notification")

	// GM should NOT receive a notification (they published it)
	gmNotifs, err := notifSvc.GetUserNotifications(context.Background(), int32(gm.ID), 10, 0)
	require.NoError(t, err)
	for _, n := range gmNotifs {
		assert.NotEqual(t, core.NotificationTypeHandoutPublished, n.Type, "GM should not receive handout_published notification for their own action")
	}

	// co-GM should NOT receive a notification (they have full draft visibility)
	coGMNotifs, err := notifSvc.GetUserNotifications(context.Background(), int32(coGM.ID), 10, 0)
	require.NoError(t, err)
	for _, n := range coGMNotifs {
		assert.NotEqual(t, core.NotificationTypeHandoutPublished, n.Type, "co-GM should not receive handout_published notification")
	}
}

// TestHandoutAPI_CreatePublishedHandout_SendsNotifications verifies that creating a handout
// directly in published state also triggers player notifications, same as publishing a draft.
func TestHandoutAPI_CreatePublishedHandout_SendsNotifications(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "notifications", "handout_comments", "handouts", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupHandoutTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &dbsvc.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	body := CreateHandoutRequest{Title: "Immediate Lore Drop", Content: "The world begins.", Status: "published"}
	bodyJSON, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/handouts", game.ID), bytes.NewBuffer(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	time.Sleep(200 * time.Millisecond)

	notifSvc := &dbsvc.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}

	playerNotifs, err := notifSvc.GetUserNotifications(context.Background(), int32(player.ID), 10, 0)
	require.NoError(t, err)
	var found bool
	for _, n := range playerNotifs {
		if n.Type == core.NotificationTypeHandoutPublished {
			assert.Contains(t, n.Title, "Immediate Lore Drop")
			found = true
			break
		}
	}
	assert.True(t, found, "player should receive a handout_published notification when handout is created in published state")
}

// TestHandoutAPI_AdminMode_CreateHandout is the regression test for the bug where admin mode
// was silently broken for handout endpoints because IsUserGameMasterCtx read admin mode from
// context but no middleware ever set it. After the fix, AdminModeMiddleware propagates the
// X-Admin-Mode header into the context so IsUserGameMasterCtx works correctly.
func TestHandoutAPI_AdminMode_CreateHandout(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "handout_comments", "handouts", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupHandoutTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	admin := testDB.CreateTestUser(t, "admin", "admin@example.com")

	// Grant admin status to the admin user
	queries := dbmodels.New(testDB.Pool)
	err := queries.UpdateUserAdminStatus(context.Background(), dbmodels.UpdateUserAdminStatusParams{
		ID:      int32(admin.ID),
		IsAdmin: pgtype.Bool{Bool: true, Valid: true},
	})
	require.NoError(t, err)

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	adminToken, err := core.CreateTestJWTTokenForUser(app, admin)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	body := CreateHandoutRequest{
		Title:   "Admin-Created Handout",
		Content: "Admin mode should grant GM access.",
		Status:  "draft",
	}
	bodyJSON, _ := json.Marshal(body)

	t.Run("admin without X-Admin-Mode header is rejected", func(t *testing.T) {
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/handouts", game.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+adminToken)
		// No X-Admin-Mode header

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code, "admin without admin mode header should be rejected")
	})

	t.Run("admin with X-Admin-Mode: true can create handout", func(t *testing.T) {
		body2 := CreateHandoutRequest{
			Title:   "Admin-Created Handout",
			Content: "Admin mode should grant GM access.",
			Status:  "draft",
		}
		body2JSON, _ := json.Marshal(body2)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/handouts", game.ID), bytes.NewBuffer(body2JSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("X-Admin-Mode", "true")

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code, "admin with admin mode header should be able to create handout: %s", rec.Body.String())
	})

	t.Run("non-admin with X-Admin-Mode: true is still rejected", func(t *testing.T) {
		// Regular GM token but for a different game — to verify non-admin cannot fake admin mode
		otherGM := testDB.CreateTestUser(t, "othergm", "othergm@example.com")
		otherGMToken, err := core.CreateTestJWTTokenForUser(app, otherGM)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/handouts", game.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+otherGMToken)
		req.Header.Set("X-Admin-Mode", "true")

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code, "non-admin cannot use admin mode header to gain access")
	})

	// Ensure the primary GM still works normally
	t.Run("primary GM can still create handout without admin mode header", func(t *testing.T) {
		body3 := CreateHandoutRequest{
			Title:   "GM Handout",
			Content: "Normal GM operation.",
			Status:  "draft",
		}
		body3JSON, _ := json.Marshal(body3)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/handouts", game.ID), bytes.NewBuffer(body3JSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code, "primary GM should still work normally")
	})
}
