package auth

// Serving a request through huma in tests that used to call a chi handler.
//
// The converted handlers have signature func(ctx, *Input) (*Output, error), so
// they cannot be invoked with (w, req) the way the chi ones were. serveHuma
// builds a one-route huma API and serves the request through it, which keeps
// every existing assertion -- status code and error body alike -- meaningful,
// while also exercising huma's own binding and validation the way production
// does.
//
// The request's context is preserved, so tests that inject an authenticated
// user with withAuthContext keep working unchanged.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"actionphase/pkg/core"
	"actionphase/pkg/humaconfig"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
)

// serveHuma runs req through a router carrying the operations that register
// returns, writing the response into w.
//
// mount is the chi path the operations hang off, matching production so the
// test exercises the URL that ships (see gotcha 16 in the migration plan).
func serveHuma(w http.ResponseWriter, req *http.Request, mount string, register func(huma.API)) {
	r := chi.NewRouter()
	r.Route(mount, func(r chi.Router) {
		register(humaconfig.New(r, "ActionPhase API", "1.0.0"))
	})
	r.ServeHTTP(w, req)
}

// serveAuthHuma serves req through the full protected auth surface, mounted at
// /api/v1/auth. Most handler tests want this.
func serveAuthHuma(w *httptest.ResponseRecorder, req *http.Request, h *Handler) {
	serveHuma(w, req, "/api/v1/auth", func(api huma.API) {
		RegisterHumaAuthPublic(api, h)
		RegisterHumaAuthRateLimited(api, h)
		RegisterHumaAuthProtected(api, h)
		RegisterHumaAuthRateLimitedProtected(api, h)
	})
}

// registrationPayload builds the body a real registration client sends.
//
// The tests used to marshal a whole core.User, which carries server-owned
// fields (id, is_admin, is_banned, pending_approval, createdAt...) that no
// client sends and the API never read. The chi handler ignored them; huma
// rejects unknown properties with a 400. Posting only the fields the endpoint
// actually accepts makes these tests exercise the real contract -- matching
// the frontend's RegisterRequest exactly.
func registrationPayload(u core.User) []byte {
	body := map[string]any{
		"username": u.Username,
		"email":    u.Email,
		"password": u.Password,
	}
	if u.Bio != nil {
		body["bio"] = *u.Bio
	}
	b, _ := json.Marshal(body)
	return b
}
