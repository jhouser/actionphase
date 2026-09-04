package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
)

// TestAuthenticator_RejectsWithProblemJSON is the regression guard for the
// third error emitter.
//
// jwtauth's own Authenticator answers with http.Error, i.e. text/plain and a
// bare message. That made 401 -- the status most likely to reach a user
// mid-session -- the one response no JSON client could parse, so an expired
// session surfaced as a generic failure instead of "your session has expired".
func TestAuthenticator_RejectsWithProblemJSON(t *testing.T) {
	original := render.Respond
	t.Cleanup(func() { render.Respond = original })
	InstallProblemJSONResponder()

	tokenAuth := jwtauth.New("HS256", []byte("test-secret"), nil)

	r := chi.NewRouter()
	r.Use(jwtauth.Verifier(tokenAuth))
	r.Use(Authenticator(tokenAuth))
	r.Get("/protected", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name  string
		token string
	}{
		{name: "no token at all", token: ""},
		{name: "malformed token", token: "not-a-jwt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/protected", nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d (body: %s)", rec.Code, rec.Body.String())
			}

			ct := rec.Header().Get("Content-Type")
			if !strings.HasPrefix(ct, "application/problem+json") {
				t.Errorf("expected application/problem+json, got %q", ct)
			}

			var body map[string]interface{}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("body should be JSON, got %q: %v", rec.Body.String(), err)
			}
			if body["title"] != "Unauthorized" {
				t.Errorf("unexpected title: %v", body["title"])
			}
			if body["status"] != float64(http.StatusUnauthorized) {
				t.Errorf("unexpected status: %v", body["status"])
			}
			// A detail is what the frontend actually displays; an empty one
			// would silently fall back to a generic message.
			if detail, _ := body["detail"].(string); detail == "" {
				t.Errorf("expected a non-empty detail, got %v", body["detail"])
			}
		})
	}
}

// TestAuthenticator_AllowsValidToken confirms the replacement did not become a
// blanket reject: a good token must still reach the handler.
func TestAuthenticator_AllowsValidToken(t *testing.T) {
	tokenAuth := jwtauth.New("HS256", []byte("test-secret"), nil)
	_, tokenString, err := tokenAuth.Encode(map[string]interface{}{"user_id": 1})
	if err != nil {
		t.Fatalf("failed to encode token: %v", err)
	}

	r := chi.NewRouter()
	r.Use(jwtauth.Verifier(tokenAuth))
	r.Use(Authenticator(tokenAuth))
	r.Get("/protected", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for a valid token, got %d (body: %s)", rec.Code, rec.Body.String())
	}
}
