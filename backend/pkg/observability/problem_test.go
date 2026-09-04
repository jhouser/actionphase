package observability

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestErrorRecoveryMiddleware_EmitsProblemJSON covers the one error response a
// client can never anticipate. A panic used to answer with
// {"error":"...","code":500}, a shape no other endpoint emitted, so the
// frontend's error handling found nothing it recognised precisely when the
// server had already failed.
func TestErrorRecoveryMiddleware_EmitsProblemJSON(t *testing.T) {
	logger := NewLogger("test", "error")

	handler := ErrorRecoveryMiddleware(logger)(http.HandlerFunc(
		func(http.ResponseWriter, *http.Request) {
			panic("boom")
		}))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/explode", nil))

	require.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Equal(t, ProblemContentType, rr.Header().Get("Content-Type"))

	var body map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body),
		"panic response must be valid JSON")

	assert.Equal(t, "Internal Server Error", body["title"])
	assert.Equal(t, float64(http.StatusInternalServerError), body["status"])
	assert.Equal(t, "Internal server error", body["detail"])

	// The legacy shape must not survive anywhere.
	assert.NotContains(t, body, "error")
	assert.NotContains(t, body, "code")
}

// TestErrorRecoveryMiddleware_CarriesCorrelationID verifies a panic still
// yields a traceable `instance`. This is the response where a correlation ID
// matters most: the user has nothing else to report, and the server-side stack
// trace is only findable by ID.
func TestErrorRecoveryMiddleware_CarriesCorrelationID(t *testing.T) {
	logger := NewLogger("test", "error")

	panicking := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})

	// Composed the way MiddlewareStack does it: recovery OUTSIDE tracing, so
	// that a panic in tracing is still caught. That ordering means the context
	// recovery unwinds to predates the correlation ID, which is exactly the
	// case correlationIDForPanic exists to handle -- composing these the other
	// way round would pass while the real stack emitted no `instance` at all.
	handler := ErrorRecoveryMiddleware(logger)(
		RequestTracingMiddleware(logger)(panicking))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/explode", nil))

	require.Equal(t, http.StatusInternalServerError, rr.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))

	instance, _ := body["instance"].(string)
	require.NotEmpty(t, instance, "panic response should carry a correlation ID")
	assert.Contains(t, instance, correlationURNPrefix)

	// The ID in the body must be the one in the header, or a user quoting the
	// body sends support chasing a request that does not exist.
	assert.Equal(t, CorrelationInstance(rr.Header().Get("X-Correlation-ID")), instance)
}

func TestCorrelationInstance(t *testing.T) {
	assert.Equal(t, "urn:actionphase:correlation:corr_abc", CorrelationInstance("corr_abc"))
	assert.Empty(t, CorrelationInstance(""), "empty ID must omit the field, not emit a bare prefix")
}
