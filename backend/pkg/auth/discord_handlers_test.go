package auth

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

// setupDiscordTestRouter creates a test router with Discord auth routes.
func setupDiscordTestRouter(app *core.App) *chi.Mux {
	tokenAuth := jwtauth.New("HS256", []byte(app.Config.JWT.Secret), nil)
	userService := &dbsvc.UserService{DB: app.Pool, Logger: app.ObsLogger}

	r := chi.NewRouter()
	h := newTestHandler(app.Pool)
	h.App = app
	handler := &h

	// Public callback. Still a chi handler: it answers a 302 redirect and
	// writes plain-text errors, so it was left unconverted.
	r.Get("/api/v1/auth/discord/callback", handler.V1DiscordCallback)

	// Protected routes. Mounted at the prefix production uses, with the huma
	// operations carrying the rest of the path, so the test exercises the URL
	// that ships rather than one the test's own mount shape invented.
	r.Route("/api/v1/auth", func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth))
		r.Use(jwtauth.Authenticator(tokenAuth))
		r.Use(core.RequireAuthenticationMiddleware(userService))

		RegisterHumaAuthProtected(humaconfig.New(r, "ActionPhase API", "1.0.0"), handler)
	})

	return r
}

// TestDiscordConnect_RequiresAuth verifies the connect endpoint rejects unauthenticated requests.
func TestDiscordConnect_RequiresAuth(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupDiscordTestRouter(app)

	req := httptest.NewRequest("GET", "/api/v1/auth/discord/connect", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestDiscordConnect_ReturnsURL verifies authenticated users get a valid OAuth2 URL.
func TestDiscordConnect_ReturnsURL(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "users")

	app := core.NewTestApp(testDB.Pool)
	// Set test OAuth config
	app.Config.Discord.OAuthClientID = "test-client-id"
	app.Config.Discord.OAuthRedirectURL = "http://localhost:3000/api/v1/auth/discord/callback"

	router := setupDiscordTestRouter(app)

	user := testDB.CreateTestUser(t, "player1", "player1@example.com")
	token, err := core.CreateTestJWTTokenForUser(app, user)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/v1/auth/discord/connect", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp DiscordConnectResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Contains(t, resp.URL, "discord.com/api/oauth2/authorize")
	assert.Contains(t, resp.URL, "client_id=test-client-id")
	assert.Contains(t, resp.URL, "identify") // scope
	assert.Contains(t, resp.URL, "state=")
}

// TestDiscordCallback_InvalidState verifies the callback rejects tampered state.
func TestDiscordCallback_InvalidState(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupDiscordTestRouter(app)

	req := httptest.NewRequest("GET", "/api/v1/auth/discord/callback?code=somecode&state=invalidddddd", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestDiscordCallback_MissingState verifies missing state returns 400.
func TestDiscordCallback_MissingState(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupDiscordTestRouter(app)

	req := httptest.NewRequest("GET", "/api/v1/auth/discord/callback?code=somecode", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestDiscordStatus_NotLinked verifies status returns {linked: false} when no account linked.
func TestDiscordStatus_NotLinked(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "users", "user_discord_accounts")

	app := core.NewTestApp(testDB.Pool)
	router := setupDiscordTestRouter(app)

	user := testDB.CreateTestUser(t, "player1", "player1@example.com")
	token, err := core.CreateTestJWTTokenForUser(app, user)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/v1/auth/discord/status", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp DiscordStatusResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp.Linked)
	assert.Nil(t, resp.DiscordUsername)
}

// TestDiscordStatus_Linked verifies status returns {linked: true, discord_username} when linked.
func TestDiscordStatus_Linked(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "users", "user_discord_accounts")

	app := core.NewTestApp(testDB.Pool)
	router := setupDiscordTestRouter(app)

	user := testDB.CreateTestUser(t, "player1", "player1@example.com")
	token, err := core.CreateTestJWTTokenForUser(app, user)
	require.NoError(t, err)

	// Pre-link a Discord account
	discordSvc := &dbsvc.DiscordAccountService{DB: testDB.Pool}
	_, err = discordSvc.UpsertDiscordAccount(context.Background(), &core.UpsertDiscordAccountRequest{
		UserID:          int32(user.ID),
		DiscordUserID:   "discord-123",
		DiscordUsername: "testuser#1234",
		AccessToken:     "tok",
	})
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/v1/auth/discord/status", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp DiscordStatusResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.Linked)
	require.NotNil(t, resp.DiscordUsername)
	assert.Equal(t, "testuser#1234", *resp.DiscordUsername)
}

// TestDiscordDisconnect_Success verifies the disconnect endpoint removes the account.
func TestDiscordDisconnect_Success(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "users", "user_discord_accounts")

	app := core.NewTestApp(testDB.Pool)
	router := setupDiscordTestRouter(app)

	user := testDB.CreateTestUser(t, "player1", "player1@example.com")
	token, err := core.CreateTestJWTTokenForUser(app, user)
	require.NoError(t, err)

	// Pre-link a Discord account
	discordSvc := &dbsvc.DiscordAccountService{DB: testDB.Pool}
	_, err = discordSvc.UpsertDiscordAccount(context.Background(), &core.UpsertDiscordAccountRequest{
		UserID:          int32(user.ID),
		DiscordUserID:   "discord-456",
		DiscordUsername: "linked#5678",
		AccessToken:     "tok",
	})
	require.NoError(t, err)

	req := httptest.NewRequest("DELETE", "/api/v1/auth/discord/disconnect", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	// Verify account is gone
	acct, err := discordSvc.GetDiscordAccount(context.Background(), int32(user.ID))
	require.NoError(t, err)
	assert.Nil(t, acct)
}

// TestDiscordState_RoundTrip verifies the HMAC state can be built and verified.
func TestDiscordState_RoundTrip(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	h := newTestHandler(testDB.Pool)
	handler := &h

	userID := int32(42)
	state := handler.buildDiscordState(userID)

	decoded, err := handler.verifyDiscordState(state)
	require.NoError(t, err)
	assert.Equal(t, userID, decoded)
}

// TestDiscordState_TamperedSignatureFails verifies tampered state is rejected.
func TestDiscordState_TamperedSignatureFails(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	h := newTestHandler(testDB.Pool)
	handler := &h

	// Build state then tamper with it
	state := handler.buildDiscordState(99)
	tampered := state[:len(state)-4] + "XXXX"

	_, err := handler.verifyDiscordState(tampered)
	assert.Error(t, err)
}

// TestDiscordConnect_RequiresAuth_Delete verifies the disconnect endpoint rejects unauthenticated requests.
func TestDiscordDisconnect_RequiresAuth(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)
	router := setupDiscordTestRouter(app)

	req := httptest.NewRequest("DELETE", "/api/v1/auth/discord/disconnect", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestDiscordCallback_TokenExchangeFails verifies the callback returns 500 when
// Discord's token exchange endpoint returns an error.
func TestDiscordCallback_TokenExchangeFails(t *testing.T) {
	// Stub the Discord API to return an error on token exchange
	discordStub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth2/token" {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"error":"invalid_client","error_description":"Invalid OAuth2 credentials"}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer discordStub.Close()
	t.Setenv("DISCORD_API_BASE_URL", discordStub.URL)

	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupDiscordTestRouter(app)

	user := testDB.CreateTestUser(t, "cbfail", "cbfail@example.com")
	h := newTestHandler(testDB.Pool)
	state := h.buildDiscordState(int32(user.ID))

	req := httptest.NewRequest("GET",
		fmt.Sprintf("/api/v1/auth/discord/callback?code=badcode&state=%s", state), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// TestDiscordCallback_FetchUserFails verifies the callback returns 500 when
// Discord's user info endpoint returns an error (e.g. invalid access token).
func TestDiscordCallback_FetchUserFails(t *testing.T) {
	discordStub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth2/token" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"access_token":"tok","token_type":"Bearer","expires_in":604800}`)
			return
		}
		if r.URL.Path == "/users/@me" {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"message":"401: Unauthorized","code":0}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer discordStub.Close()
	t.Setenv("DISCORD_API_BASE_URL", discordStub.URL)

	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupDiscordTestRouter(app)

	user := testDB.CreateTestUser(t, "userfail", "userfail@example.com")
	h := newTestHandler(testDB.Pool)
	state := h.buildDiscordState(int32(user.ID))

	req := httptest.NewRequest("GET",
		fmt.Sprintf("/api/v1/auth/discord/callback?code=validcode&state=%s", state), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// TestDiscordCallback_Success verifies the full happy path: valid state + code,
// Discord stubs return success, account is upserted, and user is redirected.
func TestDiscordCallback_Success(t *testing.T) {
	discordStub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/oauth2/token" {
			fmt.Fprint(w, `{"access_token":"test-access-tok","token_type":"Bearer","expires_in":604800}`)
			return
		}
		if r.URL.Path == "/users/@me" {
			fmt.Fprint(w, `{"id":"999888777666555444","username":"DiscordUser#1234"}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer discordStub.Close()
	t.Setenv("DISCORD_API_BASE_URL", discordStub.URL)

	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "users", "user_discord_accounts")

	app := core.NewTestApp(testDB.Pool)
	router := setupDiscordTestRouter(app)

	user := testDB.CreateTestUser(t, "cbsuccess", "cbsuccess@example.com")
	h := newTestHandler(testDB.Pool)
	state := h.buildDiscordState(int32(user.ID))

	req := httptest.NewRequest("GET",
		fmt.Sprintf("/api/v1/auth/discord/callback?code=goodcode&state=%s", state), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Should redirect (302) to the frontend settings page
	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Contains(t, rec.Header().Get("Location"), "/settings?tab=notifications&discord=linked")

	// Discord account must be persisted in the DB
	discordSvc := &dbsvc.DiscordAccountService{DB: testDB.Pool}
	acct, err := discordSvc.GetDiscordAccount(context.Background(), int32(user.ID))
	require.NoError(t, err)
	require.NotNil(t, acct)
	assert.Equal(t, "999888777666555444", acct.DiscordUserID)
	assert.Equal(t, "DiscordUser#1234", acct.DiscordUsername)
}

// Ensure DiscordAccountService satisfies the interface (compile-time check).
var _ core.DiscordAccountServiceInterface = (*dbsvc.DiscordAccountService)(nil)

// Helper to avoid unused import.
var _ = fmt.Sprintf
