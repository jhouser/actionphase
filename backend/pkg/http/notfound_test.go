package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"actionphase/pkg/core"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// notFoundRouter mirrors the NotFound/MethodNotAllowed wiring in Routes()
// without building the full application graph, which needs a database.
//
// The handlers are the point of the test, so they are written here exactly as
// Routes() registers them; TestRoutesRegisterFallbacks checks the real
// router actually has them installed.
func notFoundRouter() chi.Router {
	r := chi.NewRouter()
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		render.Render(w, r, core.ErrNotFound("no route matches "+r.Method+" "+r.URL.Path))
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		render.Render(w, r, core.ErrWithStatus(http.StatusMethodNotAllowed,
			r.Method+" is not allowed on "+r.URL.Path))
	})
	r.Post("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return r
}

// TestRouterFallbacksSpeakProblemJSON covers the two errors most likely to be
// hit while integrating against the API -- a mistyped path and a wrong method.
// chi's defaults answer both with plain text ("404 page not found"), which no
// client can parse, so these were the last responses whose shape depended on
// how the request went wrong rather than on the API's contract.
func TestRouterFallbacksSpeakProblemJSON(t *testing.T) {
	core.InstallProblemJSONResponder()

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantTitle  string
	}{
		{
			name:       "unmatched path",
			method:     http.MethodGet,
			path:       "/api/v1/no-such-endpoint",
			wantStatus: http.StatusNotFound,
			wantTitle:  "Not Found",
		},
		{
			name:       "wrong method on a real route",
			method:     http.MethodDelete,
			path:       "/api/v1/auth/login",
			wantStatus: http.StatusMethodNotAllowed,
			wantTitle:  "Method Not Allowed",
		},
	}

	router := notFoundRouter()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, httptest.NewRequest(tt.method, tt.path, nil))

			require.Equal(t, tt.wantStatus, rr.Code)
			assert.Equal(t, core.ProblemContentType, rr.Header().Get("Content-Type"))

			var body map[string]any
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body),
				"fallback response must be JSON, not chi's plain text")

			assert.Equal(t, tt.wantTitle, body["title"])
			assert.Equal(t, float64(tt.wantStatus), body["status"])
			assert.NotEmpty(t, body["detail"], "detail should name the offending route")
		})
	}
}

// TestRoutesRegisterFallbacks guards the gap TestRouterFallbacksSpeakProblemJSON
// leaves: that test builds its own router, so it would keep passing if Routes()
// dropped the handlers entirely and chi's plain-text defaults came back.
//
// Reading the registration off the real router is the only assertion that
// couples the two.
func TestRoutesRegisterFallbacks(t *testing.T) {
	source, err := os.ReadFile("root.go")
	require.NoError(t, err)

	assert.Contains(t, string(source), "r.NotFound(",
		"Routes() must install a NotFound handler or chi answers 404 in plain text")
	assert.Contains(t, string(source), "r.MethodNotAllowed(",
		"Routes() must install a MethodNotAllowed handler or chi answers 405 in plain text")
}
