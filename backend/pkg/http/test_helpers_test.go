package http

import (
	"actionphase/pkg/auth"
	dbsvc "actionphase/pkg/db/services"
	"actionphase/pkg/humaconfig"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
)

// TestHandlerTestContext_Example demonstrates how to use the handler test infrastructure
// This is an example test that shows the pattern for testing HTTP handlers
func TestHandlerTestContext_Example(t *testing.T) {
	// Skip if database tests are disabled
	if os.Getenv("SKIP_DB_TESTS") == "true" {
		t.Skip("Skipping integration test - SKIP_DB_TESTS=true")
	}

	// Create test context
	ctx := NewHandlerTestContext(t)
	defer ctx.Cleanup()

	// Setup test user
	testUser, plainPassword := ctx.CreateTestUserWithPassword("testuser", "test@example.com", "testpassword123")

	// Setup auth handler with all required services
	pool := ctx.TestDB.Pool
	authHandler := &auth.Handler{
		App:                   ctx.App,
		UserService:           &dbsvc.UserService{DB: pool, Logger: ctx.App.ObsLogger},
		SessionService:        &dbsvc.SessionService{DB: pool, Logger: ctx.App.ObsLogger},
		IPBanService:          &dbsvc.IPBanService{DB: pool, Logger: ctx.App.ObsLogger},
		FingerprintBanService: &dbsvc.FingerprintBanService{DB: pool, Logger: ctx.App.ObsLogger},
	}
	// Auth is huma-registered. /me sits in its own group with Verifier only,
	// matching production, so it answers 200 with a null user when anonymous.
	ctx.Router.Route("/api/v1/auth", func(r chi.Router) {
		auth.RegisterHumaAuthRateLimited(humaconfig.New(r, "ActionPhase API", "1.0.0"), authHandler)
		r.Group(func(r chi.Router) {
			r.Use(jwtauth.Verifier(jwtauth.New("HS256", []byte(ctx.App.Config.JWT.Secret), nil)))
			auth.RegisterHumaAuthProbe(humaconfig.New(r, "ActionPhase API", "1.0.0"), authHandler)
		})
	})

	t.Run("login with valid credentials", func(t *testing.T) {
		resp := ctx.POST("/api/v1/auth/login", map[string]string{
			"username": testUser.Username,
			"password": plainPassword,
		})

		ctx.AssertStatusOK(resp)

		var loginResp struct {
			Token string `json:"Token"`
		}
		ctx.ParseJSONResponse(resp, &loginResp)

		if loginResp.Token == "" {
			t.Error("Expected token in response, got empty string")
		}
	})

	t.Run("login with invalid credentials returns 400", func(t *testing.T) {
		resp := ctx.POST("/api/v1/auth/login", map[string]string{
			"username": testUser.Username,
			"password": "wrongpassword",
		})

		ctx.AssertStatusBadRequest(resp)
	})

	t.Run("accessing /me without auth returns 200 with null user", func(t *testing.T) {
		resp := ctx.GET("/api/v1/auth/me")
		ctx.AssertStatusOK(resp)

		var meResp struct {
			User *struct{} `json:"user"`
		}
		ctx.ParseJSONResponse(resp, &meResp)

		if meResp.User != nil {
			t.Error("Expected null user for unauthenticated request")
		}
	})

	t.Run("accessing /me with auth returns user data", func(t *testing.T) {
		resp := ctx.GETWithAuth("/api/v1/auth/me", testUser)
		ctx.AssertStatusOK(resp)

		var meResp struct {
			ID       int    `json:"id"`
			Username string `json:"username"`
		}
		ctx.ParseJSONResponse(resp, &meResp)

		if meResp.Username != testUser.Username {
			t.Errorf("Expected username %s, got %s", testUser.Username, meResp.Username)
		}
	})
}

// TestHandlerTestContext_ErrorResponses demonstrates error response testing
func TestHandlerTestContext_ErrorResponses(t *testing.T) {
	if os.Getenv("SKIP_DB_TESTS") == "true" {
		t.Skip("Skipping integration test - SKIP_DB_TESTS=true")
	}

	ctx := NewHandlerTestContext(t)
	defer ctx.Cleanup()

	// Setup auth handler with all required services
	pool := ctx.TestDB.Pool
	authHandler := &auth.Handler{
		App:                   ctx.App,
		UserService:           &dbsvc.UserService{DB: pool, Logger: ctx.App.ObsLogger},
		SessionService:        &dbsvc.SessionService{DB: pool, Logger: ctx.App.ObsLogger},
		IPBanService:          &dbsvc.IPBanService{DB: pool, Logger: ctx.App.ObsLogger},
		FingerprintBanService: &dbsvc.FingerprintBanService{DB: pool, Logger: ctx.App.ObsLogger},
	}
	// Auth is huma-registered.
	ctx.Router.Route("/api/v1/auth", func(r chi.Router) {
		auth.RegisterHumaAuthRateLimited(humaconfig.New(r, "ActionPhase API", "1.0.0"), authHandler)
	})

	t.Run("login with missing username", func(t *testing.T) {
		resp := ctx.POST("/api/v1/auth/login", map[string]string{
			"password": "testpassword",
		})

		// Missing credentials return 401 Unauthorized
		ctx.AssertStatusUnauthorized(resp)
	})

	t.Run("login with missing password", func(t *testing.T) {
		resp := ctx.POST("/api/v1/auth/login", map[string]string{
			"username": "testuser",
		})

		// Missing credentials return 401 Unauthorized
		ctx.AssertStatusUnauthorized(resp)
	})
}
