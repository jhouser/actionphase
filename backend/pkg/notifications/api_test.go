package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"actionphase/pkg/core"
	dbsvc "actionphase/pkg/db/services"
	"actionphase/pkg/humaconfig"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupNotificationTestRouter creates a test router with notification routes
func setupNotificationTestRouter(app *core.App, testDB *core.TestDatabase) *chi.Mux {
	tokenAuth := jwtauth.New("HS256", []byte(app.Config.JWT.Secret), nil)
	userService := &dbsvc.UserService{DB: testDB.Pool, Logger: app.ObsLogger}

	r := chi.NewRouter()
	r.Route("/api/v1/notifications", func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth))
		r.Use(jwtauth.Authenticator(tokenAuth))
		r.Use(core.RequireAuthenticationMiddleware(userService))

		handler := &Handler{
			App:                 app,
			NotificationService: dbsvc.NewNotificationService(testDB.Pool, app.ObsLogger),
		}
		RegisterHumaNotifications(humaconfig.New(r, "ActionPhase API", "1.0.0"), handler)
	})

	return r
}

// createTestNotification is a helper that creates a notification via the service
func createTestNotification(t *testing.T, app *core.App, testDB *core.TestDatabase, userID int32, title string) int32 {
	t.Helper()
	service := &dbsvc.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}
	notifReq := &core.CreateNotificationRequest{
		UserID: userID,
		Type:   "action_result",
		Title:  title,
	}
	_, err := service.CreateNotification(context.Background(), notifReq)
	require.NoError(t, err)

	// Fetch the notification to get its ID
	notifications, err := service.GetUserNotifications(context.Background(), userID, 10, 0)
	require.NoError(t, err)
	require.NotEmpty(t, notifications)
	return notifications[0].ID
}

// TestNotificationAPI_GetNotifications tests GET /api/v1/notifications
func TestNotificationAPI_GetNotifications(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "notifications", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupNotificationTestRouter(app, testDB)

	user := testDB.CreateTestUser(t, "user", "user@example.com")
	token, err := core.CreateTestJWTTokenForUser(app, user)
	require.NoError(t, err)

	// Create 2 notifications for the user
	createTestNotification(t, app, testDB, int32(user.ID), "Notification 1")
	createTestNotification(t, app, testDB, int32(user.ID), "Notification 2")

	t.Run("retrieves user notifications with pagination", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/notifications/", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		data := response["data"].([]interface{})
		assert.Len(t, data, 2)
		// Verify field values on returned notifications, not just count
		titles := make([]string, 0, len(data))
		for _, item := range data {
			n := item.(map[string]interface{})
			titles = append(titles, n["title"].(string))
		}
		assert.Contains(t, titles, "Notification 1", "Notification 1 should appear in results")
		assert.Contains(t, titles, "Notification 2", "Notification 2 should appear in results")
		pagination := response["pagination"].(map[string]interface{})
		assert.NotNil(t, pagination["limit"])
	})

	t.Run("respects limit query parameter", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/notifications/?limit=1", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		data := response["data"].([]interface{})
		assert.Len(t, data, 1)
	})
}

// TestNotificationAPI_GetUnreadCount tests GET /api/v1/notifications/unread-count
func TestNotificationAPI_GetUnreadCount(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "notifications", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupNotificationTestRouter(app, testDB)

	user := testDB.CreateTestUser(t, "user", "user@example.com")
	token, err := core.CreateTestJWTTokenForUser(app, user)
	require.NoError(t, err)

	t.Run("returns 0 when no unread notifications", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/notifications/unread-count", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Equal(t, float64(0), response["unread_count"])
	})

	t.Run("returns correct count of unread notifications", func(t *testing.T) {
		createTestNotification(t, app, testDB, int32(user.ID), "Unread 1")
		createTestNotification(t, app, testDB, int32(user.ID), "Unread 2")

		req := httptest.NewRequest("GET", "/api/v1/notifications/unread-count", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Equal(t, float64(2), response["unread_count"])
	})
}

// TestNotificationAPI_MarkAsRead tests PUT /api/v1/notifications/{id}/mark-read
func TestNotificationAPI_MarkAsRead(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "notifications", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupNotificationTestRouter(app, testDB)

	user := testDB.CreateTestUser(t, "user", "user@example.com")
	otherUser := testDB.CreateTestUser(t, "other", "other@example.com")
	token, err := core.CreateTestJWTTokenForUser(app, user)
	require.NoError(t, err)
	otherToken, err := core.CreateTestJWTTokenForUser(app, otherUser)
	require.NoError(t, err)

	notifID := createTestNotification(t, app, testDB, int32(user.ID), "Test Notification")

	t.Run("marks notification as read", func(t *testing.T) {
		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/notifications/%d/mark-read", notifID), nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		// Verify unread count decreased
		countReq := httptest.NewRequest("GET", "/api/v1/notifications/unread-count", nil)
		countReq.Header.Set("Authorization", "Bearer "+token)
		countRec := httptest.NewRecorder()
		router.ServeHTTP(countRec, countReq)

		var countResponse map[string]interface{}
		require.NoError(t, json.Unmarshal(countRec.Body.Bytes(), &countResponse))
		assert.Equal(t, float64(0), countResponse["unread_count"])
	})

	t.Run("other user cannot mark someone else's notification", func(t *testing.T) {
		notifID2 := createTestNotification(t, app, testDB, int32(user.ID), "Another notification")

		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/notifications/%d/mark-read", notifID2), nil)
		req.Header.Set("Authorization", "Bearer "+otherToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		// Should return 404 (not found in their notifications) or 403
		assert.NotEqual(t, http.StatusOK, rec.Code)
	})
}

// TestNotificationAPI_MarkAllAsRead tests PUT /api/v1/notifications/mark-all-read
func TestNotificationAPI_MarkAllAsRead(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "notifications", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupNotificationTestRouter(app, testDB)

	user := testDB.CreateTestUser(t, "user", "user@example.com")
	token, err := core.CreateTestJWTTokenForUser(app, user)
	require.NoError(t, err)

	createTestNotification(t, app, testDB, int32(user.ID), "N1")
	createTestNotification(t, app, testDB, int32(user.ID), "N2")
	createTestNotification(t, app, testDB, int32(user.ID), "N3")

	t.Run("marks all notifications as read", func(t *testing.T) {
		req := httptest.NewRequest("PUT", "/api/v1/notifications/mark-all-read", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		// Verify unread count is now 0
		countReq := httptest.NewRequest("GET", "/api/v1/notifications/unread-count", nil)
		countReq.Header.Set("Authorization", "Bearer "+token)
		countRec := httptest.NewRecorder()
		router.ServeHTTP(countRec, countReq)

		var countResponse map[string]interface{}
		require.NoError(t, json.Unmarshal(countRec.Body.Bytes(), &countResponse))
		assert.Equal(t, float64(0), countResponse["unread_count"])
	})
}

// TestNotificationAPI_GetNotification_Ownership tests GET /api/v1/notifications/{id} ownership enforcement
func TestNotificationAPI_GetNotification_Ownership(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "notifications", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupNotificationTestRouter(app, testDB)

	user := testDB.CreateTestUser(t, "owner", "owner@example.com")
	otherUser := testDB.CreateTestUser(t, "spy", "spy@example.com")

	token, err := core.CreateTestJWTTokenForUser(app, user)
	require.NoError(t, err)
	otherToken, err := core.CreateTestJWTTokenForUser(app, otherUser)
	require.NoError(t, err)

	notifID := createTestNotification(t, app, testDB, int32(user.ID), "Private notification")

	t.Run("owner can GET their notification", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/notifications/%d", notifID), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("other user cannot GET someone else's notification", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/notifications/%d", notifID), nil)
		req.Header.Set("Authorization", "Bearer "+otherToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		// Should be 404 (not found in their scope) or 403
		assert.NotEqual(t, http.StatusOK, rec.Code,
			"other user should not be able to read someone else's notification")
	})
}

// TestNotificationAPI_DeleteNotification tests DELETE /api/v1/notifications/{id}
func TestNotificationAPI_DeleteNotification(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "notifications", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupNotificationTestRouter(app, testDB)

	user := testDB.CreateTestUser(t, "user", "user@example.com")
	otherUser := testDB.CreateTestUser(t, "other", "other@example.com")
	token, err := core.CreateTestJWTTokenForUser(app, user)
	require.NoError(t, err)
	otherToken, err := core.CreateTestJWTTokenForUser(app, otherUser)
	require.NoError(t, err)

	t.Run("other user deleting someone else's notification is a no-op", func(t *testing.T) {
		notifID := createTestNotification(t, app, testDB, int32(user.ID), "User's notification")

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/notifications/%d", notifID), nil)
		req.Header.Set("Authorization", "Bearer "+otherToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		// Handler returns 204 regardless (delete by owner+ID is a no-op if not found)
		assert.Equal(t, http.StatusNoContent, rec.Code)

		// Verify the notification still exists for the original user
		getReq := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/notifications/%d", notifID), nil)
		getReq.Header.Set("Authorization", "Bearer "+token)
		getRec := httptest.NewRecorder()
		router.ServeHTTP(getRec, getReq)
		assert.Equal(t, http.StatusOK, getRec.Code)
	})

	t.Run("user deletes their own notification", func(t *testing.T) {
		notifID := createTestNotification(t, app, testDB, int32(user.ID), "Deletable notification")

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/notifications/%d", notifID), nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)
	})
}
