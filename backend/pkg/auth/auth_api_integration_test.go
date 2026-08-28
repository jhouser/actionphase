package auth

import (
	"actionphase/pkg/core"
	db "actionphase/pkg/db/services"
	"actionphase/pkg/humaconfig"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	ratelimitmw "actionphase/pkg/middleware"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
)

// TestAuthAPI_RegistrationEndpoint tests the user registration API endpoint
func TestAuthAPI_RegistrationEndpoint(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "registration_attempts", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)

	router := setupAuthAPITestRouter(app, testDB)

	testCases := []struct {
		name           string
		payload        map[string]interface{}
		expectedStatus int
		description    string
		checkFields    []string // Fields to verify in successful responses
	}{
		{
			name: "successful_registration",
			payload: map[string]interface{}{
				"username": "newuser",
				"email":    "newuser@example.com",
				"password": "securepassword123",
			},
			expectedStatus: 201,
			description:    "Valid registration should succeed",
			checkFields:    []string{"Token"},
		},
		{
			name: "registration_missing_username",
			payload: map[string]interface{}{
				"email":    "test@example.com",
				"password": "securepassword123",
			},
			expectedStatus: 400,
			description:    "Registration without username should fail",
		},
		{
			name: "registration_missing_email",
			payload: map[string]interface{}{
				"username": "testuser",
				"password": "securepassword123",
			},
			expectedStatus: 400,
			description:    "Registration without email should fail",
		},
		{
			name: "registration_missing_password",
			payload: map[string]interface{}{
				"username": "testuser",
				"email":    "test@example.com",
			},
			expectedStatus: 400,
			description:    "Registration without password should fail",
		},
		{
			name: "registration_weak_password",
			payload: map[string]interface{}{
				"username": "testuser",
				"email":    "test@example.com",
				"password": "123", // too short
			},
			expectedStatus: 400,
			description:    "Registration with weak password should fail",
		},
		{
			name: "registration_invalid_email",
			payload: map[string]interface{}{
				"username": "testuser",
				"email":    "invalid-email",
				"password": "securepassword123",
			},
			expectedStatus: 400,
			description:    "Registration with invalid email should fail",
		},
		{
			name: "registration_duplicate_username",
			payload: map[string]interface{}{
				"username": "newuser", // same as first test
				"email":    "different@example.com",
				"password": "securepassword123",
			},
			expectedStatus: 400,
			description:    "Registration with duplicate username should fail",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			payload, _ := json.Marshal(tc.payload)
			req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(payload))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			core.AssertEqual(t, tc.expectedStatus, w.Code, tc.description)

			// Verify response structure for successful registrations
			if w.Code == 201 && len(tc.checkFields) > 0 {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				core.AssertNoError(t, err, "Registration response should be valid JSON")

				for _, field := range tc.checkFields {
					core.AssertNotEqual(t, "", response[field], field+" should be present in response")
				}
			}

			// Verify error responses have proper structure
			if w.Code >= 400 {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				core.AssertNoError(t, err, "Error response should be valid JSON")
				core.AssertNotEqual(t, "", response["status"], "Error response should have status field")
			}
		})
	}
}

// TestAuthAPI_LoginEndpoint tests the user login API endpoint
func TestAuthAPI_LoginEndpoint(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "registration_attempts", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)

	router := setupAuthAPITestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	// Use the test fixture user (password is "test_password")
	plainPassword := "test_password"

	testCases := []struct {
		name           string
		payload        map[string]interface{}
		expectedStatus int
		description    string
		checkFields    []string
	}{
		{
			name: "successful_login",
			payload: map[string]interface{}{
				"username": fixtures.TestUser.Username,
				"password": plainPassword,
			},
			expectedStatus: 200,
			description:    "Valid login should succeed",
			checkFields:    []string{"Token"},
		},
		{
			name: "login_wrong_password",
			payload: map[string]interface{}{
				"username": fixtures.TestUser.Username,
				"password": "wrongpassword",
			},
			expectedStatus: 400,
			description:    "Login with wrong password should fail",
		},
		{
			name: "login_nonexistent_user",
			payload: map[string]interface{}{
				"username": "nonexistent",
				"password": "anypassword",
			},
			expectedStatus: 401, // Fixed to return unauthorized instead of internal error
			description:    "Login with non-existent user should fail",
		},
		{
			name: "login_missing_username",
			payload: map[string]interface{}{
				"password": plainPassword,
			},
			expectedStatus: 401,
			description:    "Login without username should fail",
		},
		{
			name: "login_missing_password",
			payload: map[string]interface{}{
				"username": fixtures.TestUser.Username,
			},
			expectedStatus: 400,
			description:    "Login without password should fail",
		},
		{
			name: "login_empty_credentials",
			payload: map[string]interface{}{
				"username": "",
				"password": "",
			},
			expectedStatus: 401,
			description:    "Login with empty credentials should fail",
		},
		{
			name: "login_username_uppercase",
			payload: map[string]interface{}{
				"username": strings.ToUpper(fixtures.TestUser.Username),
				"password": plainPassword,
			},
			expectedStatus: 200,
			description:    "Login with uppercase username should succeed (case-insensitive)",
			checkFields:    []string{"Token"},
		},
		{
			name: "login_email_uppercase",
			payload: map[string]interface{}{
				"username": strings.ToUpper(fixtures.TestUser.Email),
				"password": plainPassword,
			},
			expectedStatus: 200,
			description:    "Login with uppercase email should succeed (case-insensitive)",
			checkFields:    []string{"Token"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			payload, _ := json.Marshal(tc.payload)
			req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(payload))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Logf("Login test %s failed. Expected %d, got %d. Response: %s",
					tc.name, tc.expectedStatus, w.Code, w.Body.String())
			}
			core.AssertEqual(t, tc.expectedStatus, w.Code, tc.description)

			// Verify response structure for successful logins
			if w.Code == 200 && len(tc.checkFields) > 0 {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				core.AssertNoError(t, err, "Login response should be valid JSON")

				for _, field := range tc.checkFields {
					core.AssertNotEqual(t, "", response[field], field+" should be present in response")
				}
			}

			// Verify error responses
			if w.Code >= 400 {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				core.AssertNoError(t, err, "Error response should be valid JSON")
				core.AssertNotEqual(t, "", response["status"], "Error response should have status field")
			}
		})
	}
}

// TestAuthAPI_RefreshEndpoint tests the token refresh API endpoint
func TestAuthAPI_RefreshEndpoint(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "registration_attempts", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)

	router := setupAuthAPITestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	// Create a valid JWT token for the test user
	validToken, err := createTestAuthToken(app, fixtures.TestUser)
	core.AssertNoError(t, err, "Test token creation should succeed")

	// Create an expired token
	expiredToken, err := createExpiredTestAuthToken(app, fixtures.TestUser)
	core.AssertNoError(t, err, "Expired token creation should succeed")

	testCases := []struct {
		name           string
		token          string
		expectedStatus int
		description    string
		checkFields    []string
	}{
		{
			name:           "successful_refresh",
			token:          validToken,
			expectedStatus: 200,
			description:    "Valid token refresh should succeed",
			checkFields:    []string{"token"},
		},
		{
			name:           "refresh_no_token",
			token:          "",
			expectedStatus: 401,
			description:    "Refresh without token should fail",
		},
		{
			name:           "refresh_invalid_token",
			token:          "invalid.jwt.token",
			expectedStatus: 401,
			description:    "Refresh with invalid token should fail",
		},
		{
			name:           "refresh_expired_token",
			token:          expiredToken,
			expectedStatus: 401,
			description:    "Refresh with expired token should fail",
		},
		{
			name:           "refresh_malformed_token",
			token:          "not-a-jwt-token",
			expectedStatus: 401,
			description:    "Refresh with malformed token should fail",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/auth/refresh", nil)
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			core.AssertEqual(t, tc.expectedStatus, w.Code, tc.description)

			// Verify response structure for successful refresh
			if w.Code == 200 && len(tc.checkFields) > 0 {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				core.AssertNoError(t, err, "Refresh response should be valid JSON")

				for _, field := range tc.checkFields {
					core.AssertNotEqual(t, "", response[field], field+" should be present in response")
				}
			}
		})
	}
}

// TestAuthAPI_ContentTypeHandling tests proper content-type handling
func TestAuthAPI_ContentTypeHandling(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "registration_attempts", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)

	router := setupAuthAPITestRouter(app, testDB)

	testCases := []struct {
		name           string
		endpoint       string
		method         string
		contentType    string
		payload        string
		expectedStatus int
		description    string
	}{
		{
			name:           "register_valid_json",
			endpoint:       "/api/v1/auth/register",
			method:         "POST",
			contentType:    "application/json",
			payload:        `{"username":"testuser","email":"test@example.com","password":"testpassword123"}`,
			expectedStatus: 201,
			description:    "Registration with valid JSON should succeed",
		},
		{
			name:           "register_invalid_json",
			endpoint:       "/api/v1/auth/register",
			method:         "POST",
			contentType:    "application/json",
			payload:        `{"username":"testuser","email":"test@example.com"`, // missing closing brace
			expectedStatus: 400,
			description:    "Registration with invalid JSON should fail",
		},
		{
			name:        "register_wrong_content_type",
			endpoint:    "/api/v1/auth/register",
			method:      "POST",
			contentType: "text/plain",
			payload:     `{"username":"testuser","email":"test@example.com","password":"testpassword123"}`,
			// 415, not 400: huma rejects an unsupported media type before
			// parsing, which is the more accurate code. The chi handler read
			// the body regardless of Content-Type and failed later as a 400.
			// Either way the request is refused.
			expectedStatus: 415,
			description:    "Registration with wrong content type should fail",
		},
		{
			name:           "register_no_content_type",
			endpoint:       "/api/v1/auth/register",
			method:         "POST",
			contentType:    "",
			payload:        `{"username":"testuser","email":"test@example.com","password":"testpassword123"}`,
			expectedStatus: 400,
			description:    "Registration without content type should fail",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.endpoint, bytes.NewBufferString(tc.payload))
			if tc.contentType != "" {
				req.Header.Set("Content-Type", tc.contentType)
			}
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			core.AssertEqual(t, tc.expectedStatus, w.Code, tc.description)
		})
	}
}

// TestAuthAPI_RateLimiting tests that rate limiting middleware is wired to login/register.
// In development mode (isDevelopment=true), StrictRateLimit allows burst of 20,
// so 10 rapid requests should all pass through to the handler (returning 401/400, not 429).
func TestAuthAPI_RateLimiting(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "registration_attempts", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)

	router := setupAuthAPITestRouterWithRateLimit(app, testDB)

	// Test rapid successive login attempts — dev-mode rate limit allows burst of 20
	t.Run("rapid_login_attempts_pass_through_in_dev_mode", func(t *testing.T) {
		loginPayload := `{"username":"nonexistent","password":"wrongpassword"}`

		processedCount := 0
		for i := 0; i < 10; i++ {
			req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(loginPayload))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			// In dev mode (burst=20), all 10 requests reach the handler (401 = bad credentials)
			if w.Code == 401 || w.Code == 400 {
				processedCount++
			}
		}

		core.AssertEqual(t, 10, processedCount, "All requests should reach the handler in dev-mode rate limiting (burst=20 > 10 requests)")
	})

	// Production mode: burst=3, so 5 rapid requests should exhaust the burst and return 429
	t.Run("production_mode_rate_limit_triggers_429", func(t *testing.T) {
		prodRouter := setupAuthAPITestRouterWithRateLimitProduction(app, testDB)
		loginPayload := `{"username":"nonexistent","password":"wrongpassword"}`

		codes := make([]int, 5)
		for i := 0; i < 5; i++ {
			req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(loginPayload))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			prodRouter.ServeHTTP(w, req)
			codes[i] = w.Code
		}

		// First 3 requests hit the handler (burst=3), subsequent ones should be rate-limited
		got429 := false
		for _, code := range codes {
			if code == http.StatusTooManyRequests {
				got429 = true
				break
			}
		}
		core.AssertTrue(t, got429, "Production rate limit should return 429 after burst of 3 is exhausted")
	})
}

// setupAuthAPITestRouter creates a test router with auth routes configured for API testing
func setupAuthAPITestRouter(app *core.App, testDB *core.TestDatabase) *chi.Mux {
	// Use the same secret from app config for consistency
	tokenAuth := jwtauth.New("HS256", []byte(app.Config.JWT.Secret), nil)
	userService := &db.UserService{DB: testDB.Pool, Logger: app.ObsLogger}

	r := chi.NewRouter()

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Auth routes
		r.Route("/auth", func(r chi.Router) {
			authHandler := newTestHandler(app.Pool)
			authHandler.App = app

			// Mirrors production's middleware groups (see pkg/http/root.go):
			// public, then the Verifier-only probe, then fully authenticated.
			r.Group(func(r chi.Router) {
				RegisterHumaAuthRateLimited(humaconfig.New(r, "ActionPhase API", "1.0.0"), &authHandler)
			})
			// /me carries Verifier but NOT Authenticator, which is what lets it
			// answer 200 with a null user instead of 401.
			r.Group(func(r chi.Router) {
				r.Use(jwtauth.Verifier(tokenAuth))
				RegisterHumaAuthProbe(humaconfig.New(r, "ActionPhase API", "1.0.0"), &authHandler)
			})
			r.Group(func(r chi.Router) {
				r.Use(jwtauth.Verifier(tokenAuth))
				r.Use(jwtauth.Authenticator(tokenAuth))
				r.Use(core.RequireAuthenticationMiddleware(userService))
				RegisterHumaAuthProtected(humaconfig.New(r, "ActionPhase API", "1.0.0"), &authHandler)
			})
		})
	})

	return r
}

// setupAuthAPITestRouterWithRateLimitProduction creates a test router with StrictRateLimit
// in production mode (0.1 rps, burst 3) to verify 429 behavior under load.
func setupAuthAPITestRouterWithRateLimitProduction(app *core.App, testDB *core.TestDatabase) *chi.Mux {
	tokenAuth := jwtauth.New("HS256", []byte(app.Config.JWT.Secret), nil)
	userService := &db.UserService{DB: testDB.Pool, Logger: app.ObsLogger}

	r := chi.NewRouter()

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			authHandler := newTestHandler(app.Pool)
			authHandler.App = app
			r.Group(func(r chi.Router) {
				r.Use(ratelimitmw.StrictRateLimit(false))
				RegisterHumaAuthRateLimited(humaconfig.New(r, "ActionPhase API", "1.0.0"), &authHandler)
			})
			r.Group(func(r chi.Router) {
				r.Use(jwtauth.Verifier(tokenAuth))
				r.Use(jwtauth.Authenticator(tokenAuth))
				r.Use(core.RequireAuthenticationMiddleware(userService))
				RegisterHumaAuthProtected(humaconfig.New(r, "ActionPhase API", "1.0.0"), &authHandler)
			})
		})
	})

	return r
}

// setupAuthAPITestRouterWithRateLimit creates a test router with StrictRateLimit applied
// in development mode (relaxed limits: 10 rps, burst 20), matching production wiring.
func setupAuthAPITestRouterWithRateLimit(app *core.App, testDB *core.TestDatabase) *chi.Mux {
	tokenAuth := jwtauth.New("HS256", []byte(app.Config.JWT.Secret), nil)
	userService := &db.UserService{DB: testDB.Pool, Logger: app.ObsLogger}

	r := chi.NewRouter()

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			authHandler := newTestHandler(app.Pool)
			authHandler.App = app
			r.Group(func(r chi.Router) {
				r.Use(ratelimitmw.StrictRateLimit(true))
				RegisterHumaAuthRateLimited(humaconfig.New(r, "ActionPhase API", "1.0.0"), &authHandler)
			})
			r.Group(func(r chi.Router) {
				r.Use(jwtauth.Verifier(tokenAuth))
				RegisterHumaAuthProbe(humaconfig.New(r, "ActionPhase API", "1.0.0"), &authHandler)
			})
			r.Group(func(r chi.Router) {
				r.Use(jwtauth.Verifier(tokenAuth))
				r.Use(jwtauth.Authenticator(tokenAuth))
				r.Use(core.RequireAuthenticationMiddleware(userService))
				RegisterHumaAuthProtected(humaconfig.New(r, "ActionPhase API", "1.0.0"), &authHandler)
			})
		})
	})

	return r
}

// TestAuthAPI_BannedUserLogin tests that banned users cannot log in
func TestAuthAPI_BannedUserLogin(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "registration_attempts", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupAuthAPITestRouter(app, testDB)

	// Create a user
	userService := &db.UserService{DB: testDB.Pool, Logger: app.ObsLogger}
	user := &core.User{
		Username: "testuser",
		Password: "testpassword123",
		Email:    "testuser@example.com",
	}
	createdUser, err := userService.CreateUser(user)
	core.AssertNoError(t, err, "Should create user successfully")

	// Verify user can log in before being banned
	loginPayload := map[string]interface{}{
		"username": "testuser",
		"password": "testpassword123",
	}
	payload, _ := json.Marshal(loginPayload)
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	core.AssertEqual(t, 200, w.Code, "User should be able to log in before ban")

	// Ban the user via direct SQL update (simulating admin action)
	ctx := context.Background()
	_, err = testDB.Pool.Exec(ctx, "UPDATE users SET is_banned = TRUE WHERE id = $1", createdUser.ID)
	core.AssertNoError(t, err, "Should ban user successfully")

	// Attempt to log in again
	req = httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should be forbidden
	core.AssertEqual(t, 403, w.Code, "Banned user should not be able to log in")

	// Verify error message
	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	core.AssertNoError(t, err, "Response should be valid JSON")
	core.AssertNotEqual(t, "", response["error"], "Response should contain error message")
}

// TestAuthAPI_V1Me tests the /me endpoint
func TestAuthAPI_V1Me(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	testDB.CleanupTables(t, "registration_attempts", "sessions", "users")
	defer testDB.CleanupTables(t, "registration_attempts", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupAuthAPITestRouter(app, testDB)

	// Create test user
	userService := &db.UserService{DB: testDB.Pool, Logger: app.ObsLogger}
	user := &core.User{
		Username: "testuser",
		Password: "testpassword123",
		Email:    "testuser@example.com",
	}
	createdUser, err := userService.CreateUser(user)
	core.AssertNoError(t, err, "Should create user successfully")

	// Create valid JWT token
	token, err := core.CreateTestJWTTokenForUser(app, createdUser)
	core.AssertNoError(t, err, "Should create token successfully")

	t.Run("get_current_user_info", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "Should return 200 OK")

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")

		// User fields are at top level
		core.AssertEqual(t, "testuser", response["username"], "Username should match")
		core.AssertEqual(t, "testuser@example.com", response["email"], "Email should match")
		core.AssertNotEqual(t, nil, response["id"], "Should have user ID")
		core.AssertEqual(t, "", response["Token"], "Token should be empty")
	})

	// /me is a probe: it is mounted with jwtauth.Verifier but deliberately
	// without Authenticator, so it answers 200 with a null user rather than
	// 401. The frontend polls it to decide auth state without console errors.
	// These cases previously asserted 401 because the test router mounted /me
	// behind Authenticator, which production never did.
	t.Run("null_user_without_token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "Probe should return 200, not 401")

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")
		core.AssertEqual(t, nil, response["user"], "User should be null")
	})

	t.Run("null_user_for_invalid_token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
		req.Header.Set("Authorization", "Bearer invalid.token.here")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "Probe should return 200 for an invalid token")

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")
		core.AssertEqual(t, nil, response["user"], "User should be null")
	})
}

// TestAuthAPI_Preferences tests the user preferences endpoints
func TestAuthAPI_Preferences(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	testDB.CleanupTables(t, "registration_attempts", "sessions", "users")
	defer testDB.CleanupTables(t, "registration_attempts", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupAuthAPITestRouter(app, testDB)

	// Create test user
	userService := &db.UserService{DB: testDB.Pool, Logger: app.ObsLogger}
	user := &core.User{
		Username: "prefstest",
		Password: "testpassword123",
		Email:    "prefstest@example.com",
	}
	createdUser, err := userService.CreateUser(user)
	core.AssertNoError(t, err, "Should create user successfully")

	token, err := core.CreateTestJWTTokenForUser(app, createdUser)
	core.AssertNoError(t, err, "Should create token successfully")

	t.Run("get_default_preferences", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/auth/preferences", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "Should return 200 OK")

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")
		core.AssertNotEqual(t, nil, response["preferences"], "Should have preferences field")
	})

	t.Run("update_preferences", func(t *testing.T) {
		payload := map[string]interface{}{
			"preferences": map[string]interface{}{
				"theme":             "dark",
				"comment_read_mode": "auto",
				"font_size":         "medium",
			},
		}
		payloadBytes, _ := json.Marshal(payload)

		req := httptest.NewRequest("PUT", "/api/v1/auth/preferences", bytes.NewBuffer(payloadBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "Should return 200 OK")

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")

		prefs, ok := response["preferences"].(map[string]interface{})
		core.AssertTrue(t, ok, "Should have preferences object")
		core.AssertEqual(t, "dark", prefs["theme"], "Theme should be dark")
	})

	t.Run("update_preferences_verify_persisted", func(t *testing.T) {
		// Get preferences again to verify they were persisted
		req := httptest.NewRequest("GET", "/api/v1/auth/preferences", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "Should return 200 OK")

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")

		prefs, ok := response["preferences"].(map[string]interface{})
		core.AssertTrue(t, ok, "Should have preferences object")
		core.AssertEqual(t, "dark", prefs["theme"], "Theme should still be dark")
	})

	t.Run("update_preferences_missing_field", func(t *testing.T) {
		payload := map[string]interface{}{
			"notPreferences": map[string]interface{}{
				"theme": "light",
			},
		}
		payloadBytes, _ := json.Marshal(payload)

		req := httptest.NewRequest("PUT", "/api/v1/auth/preferences", bytes.NewBuffer(payloadBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 400, w.Code, "Should return 400 Bad Request for missing preferences field")
	})

	t.Run("unauthorized_access", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/auth/preferences", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 401, w.Code, "Should return 401 Unauthorized without token")
	})
}

// TestAuthAPI_SearchUsers tests the user search endpoint
func TestAuthAPI_SearchUsers(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	testDB.CleanupTables(t, "registration_attempts", "sessions", "users")
	defer testDB.CleanupTables(t, "registration_attempts", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupAuthAPITestRouter(app, testDB)

	// Create test users
	userService := &db.UserService{DB: testDB.Pool, Logger: app.ObsLogger}
	users := []*core.User{
		{Username: "alice", Email: "alice@example.com", Password: "password123"},
		{Username: "bob", Email: "bob@example.com", Password: "password123"},
		{Username: "charlie", Email: "charlie@example.com", Password: "password123"},
		{Username: "alicia", Email: "alicia@example.com", Password: "password123"},
	}

	var testUser *core.User
	for i, user := range users {
		createdUser, err := userService.CreateUser(user)
		core.AssertNoError(t, err, "Should create user successfully")
		if i == 0 {
			testUser = createdUser
		}
	}

	token, err := core.CreateTestJWTTokenForUser(app, testUser)
	core.AssertNoError(t, err, "Should create token successfully")

	t.Run("search_by_username", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/auth/users/search?q=ali", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "Should return 200 OK")

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")

		usersArray, ok := response["users"].([]interface{})
		core.AssertTrue(t, ok, "Should have users array")
		core.AssertTrue(t, len(usersArray) >= 2, "Should find at least alice and alicia")
	})

	t.Run("search_missing_query", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/auth/users/search", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 400, w.Code, "Should return 400 Bad Request for missing query")
	})

	t.Run("search_empty_query", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/auth/users/search?q=", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 400, w.Code, "Should return 400 Bad Request for empty query")
	})

	t.Run("search_no_results", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/auth/users/search?q=nonexistent", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "Should return 200 OK even with no results")

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		core.AssertNoError(t, err, "Response should be valid JSON")

		usersArray, ok := response["users"].([]interface{})
		core.AssertTrue(t, ok, "Should have users array")
		core.AssertEqual(t, 0, len(usersArray), "Should find no users")
	})

	t.Run("unauthorized_search", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/auth/users/search?q=test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		core.AssertEqual(t, 401, w.Code, "Should return 401 Unauthorized without token")
	})
}

// TestAuthAPI_V1Me_RevokedSession verifies the session-validity gate in V1Me.
// A valid JWT whose session_id points to a deleted session must return {"user": null}.
// Without this check, banned/force-logged-out users stay "logged in" on the frontend.
func TestAuthAPI_V1Me_RevokedSession(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupAuthAPITestRouter(app, testDB)

	userSvc := &db.UserService{DB: testDB.Pool, Logger: app.ObsLogger}
	user := &core.User{Username: "sessiontest", Password: "password123", Email: "sessiontest@example.com"}
	createdUser, err := userSvc.CreateUser(user)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Create a real session so we have a valid session ID.
	sessionSvc := &db.SessionService{DB: testDB.Pool, Logger: app.ObsLogger}
	session, err := sessionSvc.CreateSession(&core.Session{
		User:  createdUser,
		Token: "test-token",
	})
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Build a token that carries the session_id claim (matching production login flow).
	tokenAuth := jwtauth.New("HS256", []byte(app.Config.JWT.Secret), nil)
	_, tokenWithSession, err := tokenAuth.Encode(map[string]interface{}{
		"sub":        fmt.Sprintf("%d", createdUser.ID),
		"username":   createdUser.Username,
		"session_id": float64(session.ID),
		"exp":        time.Now().Add(time.Hour).Unix(),
	})
	if err != nil {
		t.Fatalf("failed to encode token: %v", err)
	}

	t.Run("valid session returns user", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
		req.Header.Set("Authorization", "Bearer "+tokenWithSession)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "active session should return 200 with user")
		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("response not valid JSON: %v", err)
		}
		core.AssertEqual(t, "sessiontest", resp["username"], "should return user data")
	})

	// Delete the session to simulate ban / forced logout.
	_, err = testDB.Pool.Exec(context.Background(), "DELETE FROM sessions WHERE id = $1", session.ID)
	if err != nil {
		t.Fatalf("failed to delete session: %v", err)
	}

	t.Run("revoked session returns null user", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
		req.Header.Set("Authorization", "Bearer "+tokenWithSession)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		core.AssertEqual(t, 200, w.Code, "revoked session must still return 200 (probe endpoint never 401)")
		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("response not valid JSON: %v", err)
		}
		core.AssertEqual(t, nil, resp["user"], "revoked session must yield null user, not the user object")
	})
}

// createTestAuthToken creates a JWT token for testing purposes
func createTestAuthToken(app *core.App, user *core.User) (string, error) {
	return core.CreateTestJWTTokenForUser(app, user)
}

// createExpiredTestAuthToken creates an expired JWT token for testing purposes
func createExpiredTestAuthToken(app *core.App, user *core.User) (string, error) {
	tokenAuth := jwtauth.New("HS256", []byte(app.Config.JWT.Secret), nil)

	claims := map[string]interface{}{
		"sub":      fmt.Sprintf("%d", user.ID), // User ID required
		"username": user.Username,
		"exp":      time.Now().Add(-time.Hour).Unix(), // expired 1 hour ago
	}

	_, tokenString, err := tokenAuth.Encode(claims)
	return tokenString, err
}

// Benchmark tests for performance monitoring
func BenchmarkAuthAPI_Registration(b *testing.B) {
	testDB := core.NewTestDatabase(b)
	defer testDB.Close()
	defer testDB.CleanupTables(b, "registration_attempts", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)

	router := setupAuthAPITestRouter(app, testDB)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		payload := `{"username":"benchuser` + string(rune(i)) + `","email":"bench` + string(rune(i)) + `@example.com","password":"benchpassword123"}`
		req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != 201 && w.Code != 400 { // 400 might be duplicate key error
			b.Fatalf("Registration failed with unexpected status %d", w.Code)
		}
	}
}

func BenchmarkAuthAPI_Login(b *testing.B) {
	testDB := core.NewTestDatabase(b)
	defer testDB.Close()
	defer testDB.CleanupTables(b, "registration_attempts", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)

	router := setupAuthAPITestRouter(app, testDB)

	// Create a test user for login benchmarks
	userService := &db.UserService{DB: testDB.Pool, Logger: app.ObsLogger}
	plainPassword := "benchpassword123"
	testUser := &core.User{
		Username: "benchuser",
		Email:    "bench@example.com",
		Password: plainPassword,
	}
	_ = testUser.HashPassword()
	_, _ = userService.CreateUser(testUser)

	loginPayload := `{"username":"benchuser","password":"` + plainPassword + `"}`

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(loginPayload))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != 200 && w.Code != 400 && w.Code != 500 {
			b.Fatalf("Login failed with unexpected status %d", w.Code)
		}
	}
}

func BenchmarkAuthAPI_TokenRefresh(b *testing.B) {
	testDB := core.NewTestDatabase(b)
	defer testDB.Close()
	defer testDB.CleanupTables(b, "registration_attempts", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)

	router := setupAuthAPITestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(b)

	// Create a valid token
	validToken, _ := createTestAuthToken(app, fixtures.TestUser)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/api/v1/auth/refresh", nil)
		req.Header.Set("Authorization", "Bearer "+validToken)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != 200 && w.Code != 401 {
			b.Fatalf("Token refresh failed with unexpected status %d", w.Code)
		}
	}
}
