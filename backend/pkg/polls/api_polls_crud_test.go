package polls

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
	dbservices "actionphase/pkg/db/services"
	dbactions "actionphase/pkg/db/services/actions"
	dbmessages "actionphase/pkg/db/services/messages"
	"actionphase/pkg/games"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupPollCRUDTestRouter creates a router with all poll routes
func setupPollCRUDTestRouter(app *core.App, testDB *core.TestDatabase) *chi.Mux {
	tokenAuth := jwtauth.New("HS256", []byte(app.Config.JWT.Secret), nil)
	userService := &dbservices.UserService{DB: testDB.Pool, Logger: app.ObsLogger}

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
	router := chi.NewRouter()
	router.Route("/api/v1", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(jwtauth.Verifier(tokenAuth))
			r.Use(jwtauth.Authenticator(tokenAuth))
			r.Use(core.RequireAuthenticationMiddleware(userService))
			r.Use(core.AdminModeMiddleware)

			handler := &Handler{
				App:                 app,
				UserService:         &dbservices.UserService{DB: testDB.Pool, Logger: app.ObsLogger},
				GameService:         &dbservices.GameService{DB: testDB.Pool, Logger: app.ObsLogger},
				PollService:         &dbservices.PollService{DB: testDB.Pool, Logger: app.ObsLogger},
				CharacterService:    &dbservices.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger},
				NotificationService: dbservices.NewNotificationService(testDB.Pool, app.ObsLogger),
			}

			// Game-scoped poll routes
			r.With(gameHandler.GameMiddleware()).Post("/games/{gameID}/polls", handler.CreatePoll)
			r.With(gameHandler.GameMiddleware()).Get("/games/{gameID}/polls", handler.ListGamePolls)

			// Poll-specific routes
			r.Get("/polls/{pollId}", handler.GetPoll)
			r.Put("/polls/{pollId}", handler.UpdatePoll)
			r.Delete("/polls/{pollId}", handler.DeletePoll)
		})
	})

	return router
}

// createTestPoll is a helper that creates a poll via the API and returns the poll ID
func createTestPoll(t *testing.T, router *chi.Mux, gameID int32, gmToken string) int32 {
	t.Helper()
	body := CreatePollRequest{
		Question: "Which route should we take?",
		Deadline: time.Now().Add(24 * time.Hour),
		Options: []PollOptionRequest{
			{Text: "Forest path", DisplayOrder: 1},
			{Text: "Mountain pass", DisplayOrder: 2},
		},
	}
	bodyJSON, err := json.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/polls", gameID), bytes.NewBuffer(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmToken)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, "create poll should succeed: %s", rec.Body.String())

	var response map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	return int32(response["id"].(float64))
}

// TestPollCRUD_CreatePoll tests POST /api/v1/games/{gameId}/polls
func TestPollCRUD_CreatePoll(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "poll_votes", "poll_options", "common_room_polls", "game_participants", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupPollCRUDTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &dbservices.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	t.Run("GM creates poll successfully", func(t *testing.T) {
		body := CreatePollRequest{
			Question: "Which path to take?",
			Deadline: time.Now().Add(24 * time.Hour),
			Options: []PollOptionRequest{
				{Text: "Option A", DisplayOrder: 1},
				{Text: "Option B", DisplayOrder: 2},
			},
		}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/polls", game.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Equal(t, "Which path to take?", response["question"])
		options, ok := response["options"].([]interface{})
		require.True(t, ok, "options should be a list")
		assert.Len(t, options, 2)
	})

	t.Run("non-GM player cannot create poll", func(t *testing.T) {
		body := CreatePollRequest{
			Question: "Should we rest?",
			Deadline: time.Now().Add(24 * time.Hour),
			Options: []PollOptionRequest{
				{Text: "Yes", DisplayOrder: 1},
				{Text: "No", DisplayOrder: 2},
			},
		}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/polls", game.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("rejects poll with only one option", func(t *testing.T) {
		body := CreatePollRequest{
			Question: "Which path?",
			Deadline: time.Now().Add(24 * time.Hour),
			Options: []PollOptionRequest{
				{Text: "Only option", DisplayOrder: 1},
			},
		}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/polls", game.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

// TestPollCRUD_UpdatePoll tests PUT /api/v1/polls/{pollId}
func TestPollCRUD_UpdatePoll(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "poll_votes", "poll_options", "common_room_polls", "game_participants", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupPollCRUDTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &dbservices.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	pollID := createTestPoll(t, router, game.ID, gmToken)

	t.Run("GM updates poll question and deadline successfully", func(t *testing.T) {
		body := UpdatePollRequest{
			Question: "Updated question?",
			Deadline: time.Now().Add(48 * time.Hour),
		}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/polls/%d", pollID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Equal(t, "Updated question?", response["question"])
	})

	t.Run("non-GM player cannot update poll", func(t *testing.T) {
		body := UpdatePollRequest{
			Question: "Unauthorized update",
			Deadline: time.Now().Add(24 * time.Hour),
		}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/polls/%d", pollID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("returns 404 for non-existent poll", func(t *testing.T) {
		body := UpdatePollRequest{
			Question: "Updated?",
			Deadline: time.Now().Add(24 * time.Hour),
		}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("PUT", "/api/v1/polls/99999", bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

// TestPollCRUD_DeletePoll tests DELETE /api/v1/polls/{pollId}
func TestPollCRUD_DeletePoll(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "poll_votes", "poll_options", "common_room_polls", "game_participants", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupPollCRUDTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &dbservices.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	t.Run("non-GM player cannot delete poll", func(t *testing.T) {
		pollID := createTestPoll(t, router, game.ID, gmToken)

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/polls/%d", pollID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("GM deletes poll successfully", func(t *testing.T) {
		pollID := createTestPoll(t, router, game.ID, gmToken)

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/polls/%d", pollID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		// The handler calls render.Status(r, 204) but doesn't write a body,
		// so the actual response code is 200 (default empty response)
		assert.Equal(t, http.StatusOK, rec.Code)

		// Verify poll is gone
		getReq := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/polls/%d", pollID), nil)
		getReq.Header.Set("Authorization", "Bearer "+gmToken)
		getRec := httptest.NewRecorder()
		router.ServeHTTP(getRec, getReq)
		assert.Equal(t, http.StatusNotFound, getRec.Code)
	})
}

// TestPollCRUD_GetPoll_NonParticipant tests that a user not in the game cannot view a poll
func TestPollCRUD_GetPoll_NonParticipant(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "poll_votes", "poll_options", "common_room_polls", "game_participants", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupPollCRUDTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	outsider := testDB.CreateTestUser(t, "outsider", "outsider@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	outsiderToken, err := core.CreateTestJWTTokenForUser(app, outsider)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")
	pollID := createTestPoll(t, router, game.ID, gmToken)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/polls/%d", pollID), nil)
	req.Header.Set("Authorization", "Bearer "+outsiderToken)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Non-participants can now read polls (visibility rules enforced on results, not listing)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestPollCRUD_ListGamePolls tests GET /api/v1/games/{gameId}/polls
func TestPollCRUD_ListGamePolls(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "poll_votes", "poll_options", "common_room_polls", "game_participants", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupPollCRUDTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	outsider := testDB.CreateTestUser(t, "outsider", "outsider@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)
	outsiderToken, err := core.CreateTestJWTTokenForUser(app, outsider)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &dbservices.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	// Create 2 polls
	createTestPoll(t, router, game.ID, gmToken)
	createTestPoll(t, router, game.ID, gmToken)

	t.Run("GM lists all game polls", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/polls", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response []interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Len(t, response, 2)
		// Verify field value on a returned item, not just count
		firstPoll := response[0].(map[string]interface{})
		assert.Equal(t, "Which route should we take?", firstPoll["question"])
	})

	t.Run("player lists game polls", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/polls", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response []interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Len(t, response, 2)
	})

	t.Run("non-participant can list game polls", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/polls", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+outsiderToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		// Non-participants can see polls exist; individual vote visibility is controlled separately
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("response includes user_has_voted field", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/polls", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response []map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		require.NotEmpty(t, response)
		_, hasField := response[0]["user_has_voted"]
		assert.True(t, hasField, "each poll should include user_has_voted field")
	})
}

// TestPollCRUD_ListGamePolls_IncludeExpired tests the include_expired query parameter
func TestPollCRUD_ListGamePolls_IncludeExpired(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "poll_votes", "poll_options", "common_room_polls", "game_participants", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupPollCRUDTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	// Create an active poll via API
	createTestPoll(t, router, game.ID, gmToken)

	// Create an expired poll directly via service
	pollService := &dbservices.PollService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = pollService.CreatePollWithOptions(context.Background(), core.CreatePollRequest{
		GameID:          game.ID,
		CreatedByUserID: int32(gm.ID),
		Question:        "Expired poll",
		Deadline:        time.Now().Add(-1 * time.Hour),
		Options: []core.PollOptionInput{
			{Text: "A", DisplayOrder: 1},
			{Text: "B", DisplayOrder: 2},
		},
	})
	require.NoError(t, err)

	t.Run("without include_expired returns only active polls", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/polls", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response []interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Len(t, response, 1, "should only return active polls by default")
	})

	t.Run("with include_expired=true returns all polls", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/polls?include_expired=true", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response []interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Len(t, response, 2, "should return both active and expired polls")
	})
}

// TestPollAPI_AdminMode_CreatePoll is the regression test for the bug where admin mode was
// silently broken for poll endpoints. IsUserGameMasterCtx read admin mode from context but
// no middleware ever set it. After the fix, AdminModeMiddleware propagates the X-Admin-Mode
// header into the context so IsUserGameMasterCtx works correctly.
func TestPollAPI_AdminMode_CreatePoll(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "poll_options", "common_room_polls", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupPollCRUDTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	admin := testDB.CreateTestUser(t, "admin", "admin@example.com")

	// Grant admin status
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

	validBody := CreatePollRequest{
		Question: "Admin mode poll test?",
		Deadline: time.Now().Add(24 * time.Hour),
		Options: []PollOptionRequest{
			{Text: "Option A", DisplayOrder: 1},
			{Text: "Option B", DisplayOrder: 2},
		},
	}

	makeCreateRequest := func(token string, adminModeHeader string) *httptest.ResponseRecorder {
		bodyJSON, _ := json.Marshal(validBody)
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/polls", game.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		if adminModeHeader != "" {
			req.Header.Set("X-Admin-Mode", adminModeHeader)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	t.Run("admin without X-Admin-Mode header is rejected", func(t *testing.T) {
		rec := makeCreateRequest(adminToken, "")
		assert.Equal(t, http.StatusForbidden, rec.Code, "admin without admin mode header should be rejected")
	})

	t.Run("admin with X-Admin-Mode: true can create poll", func(t *testing.T) {
		rec := makeCreateRequest(adminToken, "true")
		assert.Equal(t, http.StatusOK, rec.Code, "admin with admin mode should be able to create poll: %s", rec.Body.String())
	})

	t.Run("non-admin with X-Admin-Mode: true is still rejected", func(t *testing.T) {
		outsider := testDB.CreateTestUser(t, "outsider", "outsider@example.com")
		outsiderToken, err := core.CreateTestJWTTokenForUser(app, outsider)
		require.NoError(t, err)
		rec := makeCreateRequest(outsiderToken, "true")
		assert.Equal(t, http.StatusForbidden, rec.Code, "non-admin cannot use admin mode header to gain access")
	})

	t.Run("primary GM can still create poll without admin mode header", func(t *testing.T) {
		rec := makeCreateRequest(gmToken, "")
		assert.Equal(t, http.StatusOK, rec.Code, "primary GM should still work normally")
	})
}
