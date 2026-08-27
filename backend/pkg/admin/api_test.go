package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"actionphase/pkg/core"
	dbsvc "actionphase/pkg/db/services"
	messagesvc "actionphase/pkg/db/services/messages"
	httpmiddleware "actionphase/pkg/http/middleware"
	"actionphase/pkg/humaconfig"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func jsonBody(b []byte) *bytes.Reader {
	return bytes.NewReader(b)
}

// newTestHumaAPI uses the same setup as production (pkg/http.newHumaAPI), so
// tests see exactly the config and error shape that ships.
func newTestHumaAPI(r chi.Router) huma.API {
	return humaconfig.New(r, "ActionPhase API", "1.0.0")
}

// setupAdminTestRouter creates a test router with admin routes (with RequireAdmin middleware)
func setupAdminTestRouter(app *core.App, testDB *core.TestDatabase) *chi.Mux {
	tokenAuth := jwtauth.New("HS256", []byte(app.Config.JWT.Secret), nil)
	userService := &dbsvc.UserService{DB: testDB.Pool, Logger: app.ObsLogger}

	r := chi.NewRouter()
	r.Route("/api/v1/admin", func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth))
		r.Use(jwtauth.Authenticator(tokenAuth))
		r.Use(core.RequireAuthenticationMiddleware(userService))
		r.Use(httpmiddleware.RequireAdmin(app))

		handler := &Handler{
			App:                   app,
			UserService:           &dbsvc.UserService{DB: testDB.Pool, Logger: app.ObsLogger},
			SessionService:        &dbsvc.SessionService{DB: testDB.Pool, Logger: app.ObsLogger},
			IPBanService:          &dbsvc.IPBanService{DB: testDB.Pool, Logger: app.ObsLogger},
			FingerprintBanService: &dbsvc.FingerprintBanService{DB: testDB.Pool, Logger: app.ObsLogger},
			MessageService:        &messagesvc.MessageService{DB: testDB.Pool, Logger: app.ObsLogger, Metrics: app.Observability.OTELMetrics},
		}
		// Routes are registered through huma (see huma_api.go). These tests
		// assert on HTTP behaviour via router.ServeHTTP, so they are unchanged
		// from when chi handlers served them — that is what makes them a
		// migration harness rather than implementation tests.
		RegisterHumaAdmin(newTestHumaAPI(r), handler)
	})

	return r
}

// TestAdminAPI_RequiresAdmin verifies that all admin endpoints reject non-admin users
func TestAdminAPI_RequiresAdmin(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupAdminTestRouter(app, testDB)

	regularUser := testDB.CreateTestUser(t, "regular", "regular@example.com")
	token, err := core.CreateTestJWTTokenForUser(app, regularUser)
	require.NoError(t, err)

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/admin/admins"},
		{"PUT", fmt.Sprintf("/api/v1/admin/users/%d/admin", regularUser.ID)},
		{"DELETE", fmt.Sprintf("/api/v1/admin/users/%d/admin", regularUser.ID)},
		{"POST", fmt.Sprintf("/api/v1/admin/users/%d/ban", regularUser.ID)},
		{"DELETE", fmt.Sprintf("/api/v1/admin/users/%d/ban", regularUser.ID)},
		{"GET", "/api/v1/admin/users/banned"},
	}

	for _, e := range endpoints {
		t.Run(fmt.Sprintf("%s %s forbidden for non-admin", e.method, e.path), func(t *testing.T) {
			req := httptest.NewRequest(e.method, e.path, nil)
			req.Header.Set("Authorization", "Bearer "+token)

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusForbidden, rec.Code, "non-admin should be rejected for %s %s", e.method, e.path)
		})
	}
}

// TestAdminAPI_ListAdmins tests GET /api/v1/admin/admins
func TestAdminAPI_ListAdmins(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupAdminTestRouter(app, testDB)

	adminUser := testDB.CreateTestUser(t, "admin", "admin@example.com")
	// Make user admin directly in DB
	_, err := testDB.Pool.Exec(context.Background(), "UPDATE users SET is_admin = true WHERE id = $1", adminUser.ID)
	require.NoError(t, err)

	adminToken, err := core.CreateTestJWTTokenForUser(app, adminUser)
	require.NoError(t, err)

	t.Run("admin can list admins", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/admin/admins", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response []interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		require.GreaterOrEqual(t, len(response), 1, "should include the admin user")
		usernames := make([]string, 0, len(response))
		for _, item := range response {
			if u, ok := item.(map[string]interface{}); ok {
				if name, ok := u["username"].(string); ok {
					usernames = append(usernames, name)
				}
			}
		}
		assert.Contains(t, usernames, adminUser.Username, "admin user should appear in the list")
	})
}

// TestAdminAPI_GrantRevokeAdmin tests PUT/DELETE /api/v1/admin/users/{id}/admin
func TestAdminAPI_GrantRevokeAdmin(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupAdminTestRouter(app, testDB)

	adminUser := testDB.CreateTestUser(t, "admin", "admin@example.com")
	targetUser := testDB.CreateTestUser(t, "target", "target@example.com")

	_, err := testDB.Pool.Exec(context.Background(), "UPDATE users SET is_admin = true WHERE id = $1", adminUser.ID)
	require.NoError(t, err)

	adminToken, err := core.CreateTestJWTTokenForUser(app, adminUser)
	require.NoError(t, err)

	t.Run("admin grants admin status to user", func(t *testing.T) {
		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/admin/users/%d/admin", targetUser.ID), nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)

		// Verify the user is now an admin
		var isAdmin bool
		err := testDB.Pool.QueryRow(context.Background(), "SELECT is_admin FROM users WHERE id = $1", targetUser.ID).Scan(&isAdmin)
		require.NoError(t, err)
		assert.True(t, isAdmin)
	})

	t.Run("admin revokes admin status from user", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/admin/users/%d/admin", targetUser.ID), nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)

		// Verify the user is no longer an admin
		var isAdmin bool
		err := testDB.Pool.QueryRow(context.Background(), "SELECT is_admin FROM users WHERE id = $1", targetUser.ID).Scan(&isAdmin)
		require.NoError(t, err)
		assert.False(t, isAdmin)
	})
}

// TestAdminAPI_BanUnbanUser tests POST/DELETE /api/v1/admin/users/{id}/ban
func TestAdminAPI_BanUnbanUser(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "banned_users", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupAdminTestRouter(app, testDB)

	adminUser := testDB.CreateTestUser(t, "admin", "admin@example.com")
	targetUser := testDB.CreateTestUser(t, "target", "target@example.com")

	_, err := testDB.Pool.Exec(context.Background(), "UPDATE users SET is_admin = true WHERE id = $1", adminUser.ID)
	require.NoError(t, err)

	adminToken, err := core.CreateTestJWTTokenForUser(app, adminUser)
	require.NoError(t, err)

	t.Run("admin bans user successfully", func(t *testing.T) {
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/admin/users/%d/ban", targetUser.ID), nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)

		// Verify ban is recorded
		userService := &dbsvc.UserService{DB: testDB.Pool, Logger: app.ObsLogger}
		isBanned, err := userService.CheckUserBanned(context.Background(), int32(targetUser.ID))
		require.NoError(t, err)
		assert.True(t, isBanned)
	})

	t.Run("admin unbans user successfully", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/admin/users/%d/ban", targetUser.ID), nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)

		// Verify ban is removed
		userService := &dbsvc.UserService{DB: testDB.Pool, Logger: app.ObsLogger}
		isBanned, err := userService.CheckUserBanned(context.Background(), int32(targetUser.ID))
		require.NoError(t, err)
		assert.False(t, isBanned)
	})
}

// TestAdminAPI_ListBannedUsers tests GET /api/v1/admin/users/banned
func TestAdminAPI_ListBannedUsers(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "banned_users", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupAdminTestRouter(app, testDB)

	adminUser := testDB.CreateTestUser(t, "admin", "admin@example.com")
	bannedUser := testDB.CreateTestUser(t, "banned", "banned@example.com")

	_, err := testDB.Pool.Exec(context.Background(), "UPDATE users SET is_admin = true WHERE id = $1", adminUser.ID)
	require.NoError(t, err)

	adminToken, err := core.CreateTestJWTTokenForUser(app, adminUser)
	require.NoError(t, err)

	// Ban the user directly via service
	userService := &dbsvc.UserService{DB: testDB.Pool, Logger: app.ObsLogger}
	err = userService.BanUser(context.Background(), int32(bannedUser.ID), int32(adminUser.ID))
	require.NoError(t, err)

	t.Run("admin lists banned users", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/admin/users/banned", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response []interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		require.GreaterOrEqual(t, len(response), 1)
		usernames := make([]string, 0, len(response))
		for _, item := range response {
			if u, ok := item.(map[string]interface{}); ok {
				if name, ok := u["username"].(string); ok {
					usernames = append(usernames, name)
				}
			}
		}
		assert.Contains(t, usernames, bannedUser.Username, "banned user should appear in the list")
	})
}

// TestAdminAPI_DeleteMessage tests DELETE /api/v1/admin/messages/{messageId}
func TestAdminAPI_DeleteMessage(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "messages", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupAdminTestRouter(app, testDB)

	adminUser := testDB.CreateTestUser(t, "admin", "admin@example.com")
	author := testDB.CreateTestUser(t, "author", "author@example.com")

	_, err := testDB.Pool.Exec(context.Background(), "UPDATE users SET is_admin = true WHERE id = $1", adminUser.ID)
	require.NoError(t, err)

	adminToken, err := core.CreateTestJWTTokenForUser(app, adminUser)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(adminUser.ID), "Test Game")

	gameService := &dbsvc.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(author.ID), "player")
	require.NoError(t, err)

	// Create a character for the author
	characterService := &dbsvc.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	authorChar, err := characterService.CreateCharacter(context.Background(), dbsvc.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        func(i int32) *int32 { return &i }(int32(author.ID)),
		Name:          "Author Character",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	// Create a post first, then a comment on it (the admin delete handler works on comments)
	messageService := &messagesvc.MessageService{DB: testDB.Pool}
	post, err := messageService.CreatePost(context.Background(), core.CreatePostRequest{
		GameID:      game.ID,
		AuthorID:    int32(author.ID),
		CharacterID: authorChar.ID,
		Content:     "Parent post",
		Visibility:  "game",
	})
	require.NoError(t, err)

	comment, err := messageService.CreateComment(context.Background(), core.CreateCommentRequest{
		GameID:      game.ID,
		ParentID:    post.ID,
		AuthorID:    int32(author.ID),
		CharacterID: authorChar.ID,
		Content:     "This comment will be deleted by admin",
		Visibility:  "game",
	})
	require.NoError(t, err)

	t.Run("admin can delete a comment", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/admin/messages/%d", comment.ID), nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)
	})

	t.Run("deleting already-deleted comment returns 403", func(t *testing.T) {
		// Already deleted above — trying again should return 403
		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/admin/messages/%d", comment.ID), nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

// TestAdminAPI_ListUsers tests GET /api/v1/admin/users
func TestAdminAPI_ListUsers(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupAdminTestRouter(app, testDB)

	adminUser := testDB.CreateTestUser(t, "admin", "admin@example.com")
	user1 := testDB.CreateTestUser(t, "user1", "user1@example.com")
	user2 := testDB.CreateTestUser(t, "user2", "user2@example.com")

	_, err := testDB.Pool.Exec(context.Background(), "UPDATE users SET is_admin = true WHERE id = $1", adminUser.ID)
	require.NoError(t, err)

	adminToken, err := core.CreateTestJWTTokenForUser(app, adminUser)
	require.NoError(t, err)

	t.Run("returns all users with correct total and pagination fields", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/admin/users?page=1&limit=25", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.Equal(t, float64(3), resp["total"], "total should reflect all 3 created users")
		assert.Equal(t, float64(1), resp["page"])
		assert.Equal(t, float64(25), resp["page_size"])
		users := resp["users"].([]interface{})
		usernames := make([]string, 0, len(users))
		for _, u := range users {
			usernames = append(usernames, u.(map[string]interface{})["username"].(string))
		}
		assert.Contains(t, usernames, adminUser.Username)
		assert.Contains(t, usernames, user1.Username)
		assert.Contains(t, usernames, user2.Username)
	})

	t.Run("search returns only matching user", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/admin/users?search="+user1.Username, nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		users := resp["users"].([]interface{})
		require.Equal(t, 1, len(users), "search by exact username should return exactly one user")
		assert.Equal(t, user1.Username, users[0].(map[string]interface{})["username"])
	})
}

// TestAdminAPI_ListPendingApprovalUsers tests GET /api/v1/admin/users/pending
func TestAdminAPI_ListPendingApprovalUsers(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupAdminTestRouter(app, testDB)

	adminUser := testDB.CreateTestUser(t, "admin", "admin@example.com")
	pendingUser := testDB.CreateTestUser(t, "pending", "pending@example.com")
	testDB.CreateTestUser(t, "active", "active@example.com")

	_, err := testDB.Pool.Exec(context.Background(), "UPDATE users SET is_admin = true WHERE id = $1", adminUser.ID)
	require.NoError(t, err)

	userService := &dbsvc.UserService{DB: testDB.Pool, Logger: app.ObsLogger}
	require.NoError(t, userService.SetPendingApproval(context.Background(), int32(pendingUser.ID)))

	adminToken, err := core.CreateTestJWTTokenForUser(app, adminUser)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/v1/admin/users/pending", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var users []map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &users))
	hasPending := false
	hasActive := false
	for _, u := range users {
		name := u["username"].(string)
		if strings.HasPrefix(name, "pending") {
			hasPending = true
		}
		if strings.HasPrefix(name, "active") {
			hasActive = true
		}
	}
	assert.True(t, hasPending, "pending user should appear in the pending list")
	assert.False(t, hasActive, "active user should not appear in the pending list")
}

// TestAdminAPI_ApproveUser tests POST /api/v1/admin/users/{id}/approve
func TestAdminAPI_ApproveUser(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupAdminTestRouter(app, testDB)

	adminUser := testDB.CreateTestUser(t, "admin", "admin@example.com")
	pendingUser := testDB.CreateTestUser(t, "pending", "pending@example.com")
	activeUser := testDB.CreateTestUser(t, "active", "active@example.com")

	_, err := testDB.Pool.Exec(context.Background(), "UPDATE users SET is_admin = true WHERE id = $1", adminUser.ID)
	require.NoError(t, err)

	userService := &dbsvc.UserService{DB: testDB.Pool, Logger: app.ObsLogger}
	require.NoError(t, userService.SetPendingApproval(context.Background(), int32(pendingUser.ID)))

	adminToken, err := core.CreateTestJWTTokenForUser(app, adminUser)
	require.NoError(t, err)

	t.Run("approves pending user successfully", func(t *testing.T) {
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/admin/users/%d/approve", pendingUser.ID), nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)

		var isPending bool
		err := testDB.Pool.QueryRow(context.Background(),
			"SELECT pending_approval FROM users WHERE id = $1", pendingUser.ID).Scan(&isPending)
		require.NoError(t, err)
		assert.False(t, isPending, "user should no longer be pending after approval")
	})

	t.Run("returns 400 when user is not pending", func(t *testing.T) {
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/admin/users/%d/approve", activeUser.ID), nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("returns 404 for non-existent user", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/admin/users/999999/approve", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

// TestAdminAPI_RejectUser tests POST /api/v1/admin/users/{id}/reject
func TestAdminAPI_RejectUser(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupAdminTestRouter(app, testDB)

	adminUser := testDB.CreateTestUser(t, "admin", "admin@example.com")
	pendingUser := testDB.CreateTestUser(t, "pending", "pending@example.com")
	activeUser := testDB.CreateTestUser(t, "active", "active@example.com")

	_, err := testDB.Pool.Exec(context.Background(), "UPDATE users SET is_admin = true WHERE id = $1", adminUser.ID)
	require.NoError(t, err)

	userService := &dbsvc.UserService{DB: testDB.Pool, Logger: app.ObsLogger}
	require.NoError(t, userService.SetPendingApproval(context.Background(), int32(pendingUser.ID)))

	adminToken, err := core.CreateTestJWTTokenForUser(app, adminUser)
	require.NoError(t, err)

	t.Run("rejects pending user: deletes the account", func(t *testing.T) {
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/admin/users/%d/reject", pendingUser.ID), nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)

		var count int
		err := testDB.Pool.QueryRow(context.Background(),
			"SELECT COUNT(*) FROM users WHERE id = $1", pendingUser.ID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count, "user should be deleted after rejection")
	})

	t.Run("returns 400 when user is not pending", func(t *testing.T) {
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/admin/users/%d/reject", activeUser.ID), nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("returns 404 for non-existent user", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/admin/users/999999/reject", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

// TestAdminAPI_GetUserSessions tests GET /api/v1/admin/users/{id}/sessions
func TestAdminAPI_GetUserSessions(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupAdminTestRouter(app, testDB)

	adminUser := testDB.CreateTestUser(t, "admin", "admin@example.com")
	targetUser := testDB.CreateTestUser(t, "target", "target@example.com")

	_, err := testDB.Pool.Exec(context.Background(), "UPDATE users SET is_admin = true WHERE id = $1", adminUser.ID)
	require.NoError(t, err)

	adminToken, err := core.CreateTestJWTTokenForUser(app, adminUser)
	require.NoError(t, err)

	t.Run("returns empty session list for user with no sessions", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/admin/users/%d/sessions", targetUser.ID), nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var sessions []interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &sessions))
		assert.Empty(t, sessions)
	})

	t.Run("returns session with metadata for user with an active session", func(t *testing.T) {
		sessionSvc := &dbsvc.SessionService{DB: testDB.Pool, Logger: app.ObsLogger}
		ip := "1.2.3.4"
		ua := "TestBrowser/1.0"
		fp := "testfingerprint42"
		_, err := sessionSvc.CreateSessionWithMetadata(context.Background(), &core.Session{
			User:        targetUser,
			Token:       "test-refresh-token",
			IPAddress:   &ip,
			UserAgent:   &ua,
			Fingerprint: &fp,
		})
		require.NoError(t, err)

		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/admin/users/%d/sessions", targetUser.ID), nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var sessions []map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &sessions))
		require.Equal(t, 1, len(sessions), "should return the one created session")
		assert.Equal(t, ip, sessions[0]["ip_address"])
		assert.Equal(t, ua, sessions[0]["user_agent"])
		assert.Equal(t, fp, sessions[0]["fingerprint"])
	})

	t.Run("returns 400 for invalid user ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/admin/users/notanid/sessions", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

// TestAdminAPI_IPBans tests GET/POST/DELETE /api/v1/admin/ip-bans
func TestAdminAPI_IPBans(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "ip_bans", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupAdminTestRouter(app, testDB)

	adminUser := testDB.CreateTestUser(t, "admin", "admin@example.com")
	_, err := testDB.Pool.Exec(context.Background(), "UPDATE users SET is_admin = true WHERE id = $1", adminUser.ID)
	require.NoError(t, err)

	adminToken, err := core.CreateTestJWTTokenForUser(app, adminUser)
	require.NoError(t, err)

	var createdBanID float64

	t.Run("creates an IP ban", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"ip_address": "203.0.113.42",
			"reason":     "test ban",
		})
		req := httptest.NewRequest("POST", "/api/v1/admin/ip-bans", jsonBody(body))
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
		var ban map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &ban))
		assert.Equal(t, "203.0.113.42", ban["ip_address"])
		createdBanID = ban["id"].(float64)
	})

	t.Run("lists IP bans including the created one", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/admin/ip-bans", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var bans []map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &bans))
		ips := make([]string, 0, len(bans))
		for _, b := range bans {
			ips = append(ips, b["ip_address"].(string))
		}
		assert.Contains(t, ips, "203.0.113.42")
	})

	t.Run("returns 400 for duplicate IP ban", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"ip_address": "203.0.113.42",
			"reason":     "duplicate",
		})
		req := httptest.NewRequest("POST", "/api/v1/admin/ip-bans", jsonBody(body))
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("returns 400 for malformed IP address", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"ip_address": "not-an-ip",
			"reason":     "test",
		})
		req := httptest.NewRequest("POST", "/api/v1/admin/ip-bans", jsonBody(body))
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("returns 400 when ip_address is empty", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"ip_address": "",
			"reason":     "test",
		})
		req := httptest.NewRequest("POST", "/api/v1/admin/ip-bans", jsonBody(body))
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("deletes an IP ban and it no longer appears in list", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/admin/ip-bans/%d", int(createdBanID)), nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)

		listReq := httptest.NewRequest("GET", "/api/v1/admin/ip-bans", nil)
		listReq.Header.Set("Authorization", "Bearer "+adminToken)
		listRec := httptest.NewRecorder()
		router.ServeHTTP(listRec, listReq)

		var bans []map[string]interface{}
		require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &bans))
		for _, b := range bans {
			assert.NotEqual(t, "203.0.113.42", b["ip_address"], "deleted IP ban should not appear in list")
		}
	})

	t.Run("returns 400 for non-numeric ban ID", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/api/v1/admin/ip-bans/notanid", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

// TestAdminAPI_FingerprintBans tests GET/POST/DELETE /api/v1/admin/fingerprint-bans
func TestAdminAPI_FingerprintBans(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "fingerprint_bans", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupAdminTestRouter(app, testDB)

	adminUser := testDB.CreateTestUser(t, "admin", "admin@example.com")
	_, err := testDB.Pool.Exec(context.Background(), "UPDATE users SET is_admin = true WHERE id = $1", adminUser.ID)
	require.NoError(t, err)

	adminToken, err := core.CreateTestJWTTokenForUser(app, adminUser)
	require.NoError(t, err)

	var createdBanID float64

	t.Run("creates a fingerprint ban", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"fingerprint": "abc123testfingerprint",
			"reason":      "test ban",
		})
		req := httptest.NewRequest("POST", "/api/v1/admin/fingerprint-bans", jsonBody(body))
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
		var ban map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &ban))
		assert.Equal(t, "abc123testfingerprint", ban["fingerprint"])
		createdBanID = ban["id"].(float64)
	})

	t.Run("lists fingerprint bans including the created one", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/admin/fingerprint-bans", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var bans []map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &bans))
		fps := make([]string, 0, len(bans))
		for _, b := range bans {
			fps = append(fps, b["fingerprint"].(string))
		}
		assert.Contains(t, fps, "abc123testfingerprint")
	})

	t.Run("returns 400 for duplicate fingerprint ban", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"fingerprint": "abc123testfingerprint",
			"reason":      "duplicate",
		})
		req := httptest.NewRequest("POST", "/api/v1/admin/fingerprint-bans", jsonBody(body))
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("returns 400 when fingerprint is empty", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"fingerprint": "",
			"reason":      "test",
		})
		req := httptest.NewRequest("POST", "/api/v1/admin/fingerprint-bans", jsonBody(body))
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("returns 400 when fingerprint exceeds 512 characters", func(t *testing.T) {
		longFP := make([]byte, 513)
		for i := range longFP {
			longFP[i] = 'a'
		}
		body, _ := json.Marshal(map[string]interface{}{
			"fingerprint": string(longFP),
			"reason":      "test",
		})
		req := httptest.NewRequest("POST", "/api/v1/admin/fingerprint-bans", jsonBody(body))
		req.Header.Set("Authorization", "Bearer "+adminToken)
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("deletes a fingerprint ban and it no longer appears in list", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/admin/fingerprint-bans/%d", int(createdBanID)), nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)

		listReq := httptest.NewRequest("GET", "/api/v1/admin/fingerprint-bans", nil)
		listReq.Header.Set("Authorization", "Bearer "+adminToken)
		listRec := httptest.NewRecorder()
		router.ServeHTTP(listRec, listReq)

		var bans []map[string]interface{}
		require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &bans))
		for _, b := range bans {
			assert.NotEqual(t, "abc123testfingerprint", b["fingerprint"], "deleted fingerprint ban should not appear in list")
		}
	})

	t.Run("returns 400 for non-numeric ban ID", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/api/v1/admin/fingerprint-bans/notanid", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}
