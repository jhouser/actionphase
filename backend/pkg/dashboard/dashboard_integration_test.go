package dashboard

import (
	"actionphase/pkg/core"
	services "actionphase/pkg/db/services"
	"actionphase/pkg/humaconfig"
	"actionphase/pkg/observability"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDashboardAPI_GetUserDashboard_Integration(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	// Clean up before and after to ensure isolation
	testDB.CleanupTables(t, "action_submissions", "game_phases", "game_participants", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "action_submissions", "game_phases", "game_participants", "games", "sessions", "users")

	factory := core.NewTestDataFactory(testDB, t)

	// Create test app
	obsLogger := observability.NewLogger("test", "info")
	app := &core.App{
		Pool:      testDB.Pool,
		ObsLogger: obsLogger,
		Config: &core.Config{
			JWT: core.JWTConfig{
				Algorithm: "HS256",
				Secret:    "test-secret-key-for-testing-only",
			},
		},
	}

	// Create authenticated user
	user, _ := factory.CreateAuthenticatedUser()

	// Create JWT token with username (not session_id)
	tokenAuth := jwtauth.New("HS256", []byte(app.Config.JWT.Secret), nil)
	_, tokenString, _ := tokenAuth.Encode(map[string]interface{}{
		"sub":      fmt.Sprintf("%d", user.ID),
		"username": user.Username,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	})

	// Setup router with dashboard handler and authentication middleware
	userService := &services.UserService{DB: testDB.Pool, Logger: app.ObsLogger}
	r := chi.NewRouter()
	r.Use(jwtauth.Verifier(tokenAuth))
	r.Use(jwtauth.Authenticator(tokenAuth))
	r.Use(core.RequireAuthenticationMiddleware(userService))

	handler := &Handler{
		App:              app,
		UserService:      &services.UserService{DB: testDB.Pool, Logger: app.ObsLogger},
		DashboardService: &services.DashboardService{DB: testDB.Pool, Logger: app.ObsLogger},
	}
	RegisterHumaDashboard(humaconfig.New(r, "ActionPhase API", "1.0.0"), handler)

	t.Run("empty_dashboard_for_new_user", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+tokenString)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		core.AssertEqual(t, http.StatusOK, w.Code, "Should return 200 OK")
		body := w.Body.String()
		core.AssertTrue(t, strings.Contains(body, `"has_games":false`), "Should have has_games=false")
		core.AssertTrue(t, strings.Contains(body, `"player_games":[]`), "Should have empty player_games")
	})

	t.Run("dashboard_with_player_game", func(t *testing.T) {
		// Create game and add user as participant
		gm := factory.NewUser().WithUsername("gm").Create()
		game := factory.NewGame().
			WithTitle("Test Game").
			WithGM(gm.ID).
			WithState("in_progress").
			Create()
		factory.NewGameParticipant().
			ForGame(game.ID).
			WithUser(user.ID).
			WithRole("player").
			Create()

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+tokenString)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		core.AssertEqual(t, http.StatusOK, w.Code, "Should return 200 OK")
		body := w.Body.String()
		core.AssertTrue(t, strings.Contains(body, `"has_games":true`), "Should have has_games=true")
		core.AssertTrue(t, strings.Contains(body, `"Test Game"`), "Should contain game title")
		core.AssertTrue(t, strings.Contains(body, `"player_games":[`), "Should have player_games array")
	})

	t.Run("unauthorized_without_token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		core.AssertEqual(t, http.StatusUnauthorized, w.Code, "Should return 401 Unauthorized")
	})

	t.Run("invalid_session_id", func(t *testing.T) {
		// Create token with invalid session
		_, badTokenString, _ := tokenAuth.Encode(map[string]interface{}{
			"session_id": "invalid-session-id",
			"exp":        time.Now().Add(24 * time.Hour).Unix(),
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+badTokenString)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		// Should fail to authenticate with invalid session
		core.AssertTrue(t, w.Code == http.StatusUnauthorized || w.Code == http.StatusInternalServerError,
			"Should return 401 or 500 for invalid session")
	})
}

func TestDashboardAPI_GetUserDashboard_WithUrgentGame(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	// Clean up before and after to ensure isolation
	testDB.CleanupTables(t, "action_submissions", "game_phases", "game_participants", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "action_submissions", "game_phases", "game_participants", "games", "sessions", "users")

	factory := core.NewTestDataFactory(testDB, t)

	// Create test app
	obsLogger := observability.NewLogger("test", "info")
	app := &core.App{
		Pool:      testDB.Pool,
		ObsLogger: obsLogger,
		Config: &core.Config{
			JWT: core.JWTConfig{
				Algorithm: "HS256",
				Secret:    "test-secret-key-for-testing-only",
			},
		},
	}

	// Create authenticated user
	user, _ := factory.CreateAuthenticatedUser()

	// Create JWT token with username
	tokenAuth := jwtauth.New("HS256", []byte(app.Config.JWT.Secret), nil)
	_, tokenString, _ := tokenAuth.Encode(map[string]interface{}{
		"sub":      fmt.Sprintf("%d", user.ID),
		"username": user.Username,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	})

	// Setup router with authentication middleware
	userService := &services.UserService{DB: testDB.Pool, Logger: app.ObsLogger}
	r := chi.NewRouter()
	r.Use(jwtauth.Verifier(tokenAuth))
	r.Use(jwtauth.Authenticator(tokenAuth))
	r.Use(core.RequireAuthenticationMiddleware(userService))

	handler := &Handler{
		App:              app,
		UserService:      &services.UserService{DB: testDB.Pool, Logger: app.ObsLogger},
		DashboardService: &services.DashboardService{DB: testDB.Pool, Logger: app.ObsLogger},
	}
	RegisterHumaDashboard(humaconfig.New(r, "ActionPhase API", "1.0.0"), handler)

	// Create game with urgent deadline
	gm := factory.NewUser().WithUsername("gm").Create()
	game := factory.NewGame().
		WithTitle("Urgent Game").
		WithGM(gm.ID).
		WithState("in_progress").
		Create()
	factory.NewGameParticipant().
		ForGame(game.ID).
		WithUser(user.ID).
		WithRole("player").
		Create()

	// Create active phase with near deadline
	factory.NewPhase().
		InGame(game).
		ActionPhase().
		Active().
		WithDeadline(time.Now().Add(30 * time.Minute)).
		Create()

	// No submission for this phase: submissions have no draft state, so a
	// pending action is simply the absence of a row.

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusOK, w.Code, "Should return 200 OK")
	body := w.Body.String()
	core.AssertTrue(t, strings.Contains(body, `"is_urgent":true`), "Game should be marked as urgent")
	core.AssertTrue(t, strings.Contains(body, `"deadline_status":"critical"`), "Should have critical deadline status")
	core.AssertTrue(t, strings.Contains(body, `"has_pending_action":true`), "Should have pending action")
}

func newDashboardApp(testDB *core.TestDatabase) *core.App {
	return &core.App{
		Pool:      testDB.Pool,
		ObsLogger: observability.NewLogger("test", "info"),
		Config: &core.Config{
			JWT: core.JWTConfig{Algorithm: "HS256", Secret: "test-secret-key-for-testing-only"},
		},
	}
}

func makeDashboardRouter(app *core.App, testDB *core.TestDatabase) (*chi.Mux, *jwtauth.JWTAuth) {
	tokenAuth := jwtauth.New("HS256", []byte(app.Config.JWT.Secret), nil)
	userService := &services.UserService{DB: testDB.Pool, Logger: app.ObsLogger}
	r := chi.NewRouter()
	r.Use(jwtauth.Verifier(tokenAuth))
	r.Use(jwtauth.Authenticator(tokenAuth))
	r.Use(core.RequireAuthenticationMiddleware(userService))
	RegisterHumaDashboard(humaconfig.New(r, "ActionPhase API", "1.0.0"), &Handler{
		App:              app,
		UserService:      &services.UserService{DB: testDB.Pool, Logger: app.ObsLogger},
		DashboardService: &services.DashboardService{DB: testDB.Pool, Logger: app.ObsLogger},
	})
	return r, tokenAuth
}

func makeToken(tokenAuth *jwtauth.JWTAuth, user *core.User) string {
	_, tokenString, _ := tokenAuth.Encode(map[string]interface{}{
		"sub":      fmt.Sprintf("%d", user.ID),
		"username": user.Username,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	})
	return tokenString
}

func TestDashboardAPI_GMGame(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_participants", "games", "sessions", "users")

	app := newDashboardApp(testDB)
	r, tokenAuth := makeDashboardRouter(app, testDB)
	factory := core.NewTestDataFactory(testDB, t)

	gm := testDB.CreateTestUser(t, "gm_dash_test", "gm_dash_test@example.com")
	factory.NewGame().WithTitle("GM's Own Game").WithGM(int32(gm.ID)).WithState("in_progress").Create()

	gmToken := makeToken(tokenAuth, gm)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+gmToken)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp core.DashboardData
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.HasGames)
	require.Len(t, resp.GMGames, 1)
	assert.Equal(t, "GM's Own Game", resp.GMGames[0].Title)
	assert.Equal(t, "gm", resp.GMGames[0].UserRole)
}

func TestDashboardAPI_ResponseShape(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_participants", "games", "sessions", "users")

	app := newDashboardApp(testDB)
	r, tokenAuth := makeDashboardRouter(app, testDB)

	user := testDB.CreateTestUser(t, "shape_test_user", "shape_test_user@example.com")
	token := makeToken(tokenAuth, user)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp, "has_games")
	assert.Contains(t, resp, "player_games")
	assert.Contains(t, resp, "gm_games")
	assert.Contains(t, resp, "mixed_role_games")
	assert.Contains(t, resp, "unread_notifications")
}
