package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	core "actionphase/pkg/core"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockHandler is a simple handler that returns 200 OK
func mockHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	})
}

func TestRateLimitMiddleware(t *testing.T) {
	tests := []struct {
		name          string
		config        RateLimitConfig
		numRequests   int
		requestDelay  time.Duration
		expectBlocked int
		expectAllowed int
		description   string
	}{
		{
			name: "allows requests within limit",
			config: RateLimitConfig{
				RequestsPerSecond: 10.0,
				Burst:             5,
				TTL:               time.Minute,
				IPLookups:         []string{"RemoteAddr"},
			},
			numRequests:   3,
			requestDelay:  0,
			expectBlocked: 0,
			expectAllowed: 3,
			description:   "Should allow all 3 requests when limit is 10/sec with burst of 5",
		},
		{
			name: "blocks requests exceeding burst",
			config: RateLimitConfig{
				RequestsPerSecond: 1.0,
				Burst:             2,
				TTL:               time.Minute,
				IPLookups:         []string{"RemoteAddr"},
			},
			numRequests:   5,
			requestDelay:  0,
			expectBlocked: 3,
			expectAllowed: 2,
			description:   "Should allow burst of 2, then block remaining 3 requests",
		},
		{
			name: "allows requests after rate limit window",
			config: RateLimitConfig{
				RequestsPerSecond: 2.0, // 1 request every 500ms
				Burst:             1,
				TTL:               time.Minute,
				IPLookups:         []string{"RemoteAddr"},
			},
			numRequests:   3,
			requestDelay:  600 * time.Millisecond, // Wait 600ms between requests
			expectBlocked: 0,
			expectAllowed: 3,
			description:   "Should allow all requests when spaced out beyond rate window",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create middleware with test config
			middleware := RateLimitMiddleware(tt.config)
			handler := middleware(mockHandler())

			// Track results
			allowed := 0
			blocked := 0

			// Make multiple requests from the same IP
			for i := 0; i < tt.numRequests; i++ {
				if i > 0 && tt.requestDelay > 0 {
					time.Sleep(tt.requestDelay)
				}

				req := httptest.NewRequest("GET", "/test", nil)
				req.RemoteAddr = "192.168.1.1:12345" // Same IP for all requests
				w := httptest.NewRecorder()

				handler.ServeHTTP(w, req)

				if w.Code == http.StatusOK {
					allowed++
				} else if w.Code == http.StatusTooManyRequests {
					blocked++
				}
			}

			assert.Equal(t, tt.expectAllowed, allowed, "Expected %d allowed requests, got %d. %s", tt.expectAllowed, allowed, tt.description)
			assert.Equal(t, tt.expectBlocked, blocked, "Expected %d blocked requests, got %d. %s", tt.expectBlocked, blocked, tt.description)
		})
	}
}

func TestRateLimitMiddleware_DifferentIPs(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerSecond: 1.0,
		Burst:             2,
		TTL:               time.Minute,
		IPLookups:         []string{"RemoteAddr"},
	}

	middleware := RateLimitMiddleware(config)
	handler := middleware(mockHandler())

	// First IP should be allowed burst of 2 requests
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "Request %d from IP1 should be allowed", i+1)
	}

	// Third request from first IP should be blocked
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusTooManyRequests, w1.Code, "Third request from IP1 should be blocked")

	// Second IP should still be allowed (independent limit)
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "192.168.1.2:12345"
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code, "First request from IP2 should be allowed")
}

func TestRateLimitMiddleware_IPLookupMethods(t *testing.T) {
	tests := []struct {
		name          string
		ipLookups     []string
		setHeaders    map[string]string
		setRemoteAddr string
		expectedIP    string
		description   string
	}{
		{
			name:          "uses X-Real-IP header when configured",
			ipLookups:     []string{"X-Real-IP", "RemoteAddr"},
			setHeaders:    map[string]string{"X-Real-IP": "10.0.0.1"},
			setRemoteAddr: "192.168.1.1:12345",
			expectedIP:    "10.0.0.1",
			description:   "Should prefer X-Real-IP over RemoteAddr",
		},
		{
			name:          "uses X-Forwarded-For header when configured",
			ipLookups:     []string{"X-Forwarded-For", "RemoteAddr"},
			setHeaders:    map[string]string{"X-Forwarded-For": "10.0.0.2, 10.0.0.3"},
			setRemoteAddr: "192.168.1.1:12345",
			expectedIP:    "10.0.0.2", // First IP in X-Forwarded-For
			description:   "Should use first IP from X-Forwarded-For",
		},
		{
			name:          "falls back to RemoteAddr when no headers",
			ipLookups:     []string{"X-Real-IP", "RemoteAddr"},
			setHeaders:    map[string]string{},
			setRemoteAddr: "192.168.1.1:12345",
			expectedIP:    "192.168.1.1",
			description:   "Should fall back to RemoteAddr when X-Real-IP is not present",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := RateLimitConfig{
				RequestsPerSecond: 1.0,
				Burst:             1,
				TTL:               time.Minute,
				IPLookups:         tt.ipLookups,
			}

			middleware := RateLimitMiddleware(config)
			handler := middleware(mockHandler())

			// First request should succeed
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = tt.setRemoteAddr
			for header, value := range tt.setHeaders {
				req.Header.Set(header, value)
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code, "First request should be allowed")

			// Second request with same IP should be blocked (burst = 1)
			req2 := httptest.NewRequest("GET", "/test", nil)
			req2.RemoteAddr = tt.setRemoteAddr
			for header, value := range tt.setHeaders {
				req2.Header.Set(header, value)
			}
			w2 := httptest.NewRecorder()
			handler.ServeHTTP(w2, req2)
			assert.Equal(t, http.StatusTooManyRequests, w2.Code, "Second request from same IP should be blocked")

			// Request with different IP should succeed
			req3 := httptest.NewRequest("GET", "/test", nil)
			req3.RemoteAddr = "10.10.10.10:12345"
			w3 := httptest.NewRecorder()
			handler.ServeHTTP(w3, req3)
			assert.Equal(t, http.StatusOK, w3.Code, "Request from different IP should be allowed")
		})
	}
}

func TestRateLimitMiddleware_ErrorResponse(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerSecond: 1.0,
		Burst:             1,
		TTL:               time.Minute,
		IPLookups:         []string{"RemoteAddr"},
	}

	middleware := RateLimitMiddleware(config)
	handler := middleware(mockHandler())

	// First request allowed
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)
	require.Equal(t, http.StatusOK, w1.Code)

	// Second request blocked
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "192.168.1.1:12345"
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	// Verify error response
	assert.Equal(t, http.StatusTooManyRequests, w2.Code, "Should return 429 status")
	assert.Equal(t, core.ProblemContentType, w2.Header().Get("Content-Type"), "Should return RFC 7807 content type")
	assert.Contains(t, w2.Body.String(), "Rate limit exceeded", "Should contain rate limit error message")
}

func TestStrictRateLimit(t *testing.T) {
	middleware := StrictRateLimit(false) // Not dev mode
	handler := middleware(mockHandler())

	// Should allow burst of 3 requests
	allowed := 0
	blocked := 0

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			allowed++
		} else if w.Code == http.StatusTooManyRequests {
			blocked++
		}
	}

	assert.Equal(t, 3, allowed, "StrictRateLimit should allow burst of 3 requests")
	assert.Equal(t, 2, blocked, "StrictRateLimit should block remaining 2 requests")
}

// Benchmark tests to verify performance
func BenchmarkRateLimitMiddleware(b *testing.B) {
	config := RateLimitConfig{
		RequestsPerSecond: 100.0,
		Burst:             50,
		TTL:               time.Minute,
		IPLookups:         []string{"RemoteAddr"},
	}

	middleware := RateLimitMiddleware(config)
	handler := middleware(mockHandler())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = fmt.Sprintf("192.168.1.%d:12345", i%256)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}

// TestRateLimitBodyMatchesErrResponse pins the hand-written 429 body to the
// shape core.ErrResponse actually serializes.
//
// The rate limiter is the one error emitter that cannot render through
// core.ErrResponse -- tollbooth takes a fixed string, not a renderer -- so its
// body is a literal that nothing else would catch drifting. Marshalling the
// equivalent ErrResponse and comparing field-for-field means a change to the
// struct's json tags fails here rather than silently leaving 429 on an older
// shape, which is exactly how this response came to still be emitting the
// pre-RFC-7807 {"error": ...} format.
func TestRateLimitBodyMatchesErrResponse(t *testing.T) {
	const detail = "Rate limit exceeded. Please try again later."

	rendered, err := json.Marshal(&core.ErrResponse{
		HTTPStatusCode: http.StatusTooManyRequests,
		Title:          http.StatusText(http.StatusTooManyRequests),
		Status:         http.StatusTooManyRequests,
		Detail:         detail,
	})
	require.NoError(t, err)

	var want, got map[string]any
	require.NoError(t, json.Unmarshal(rendered, &want))
	require.NoError(t, json.Unmarshal([]byte(rateLimitProblemJSON), &got),
		"rate limit body must be valid JSON")

	assert.Equal(t, want, got,
		"429 body has drifted from core.ErrResponse; update rateLimitProblemJSON")
}

// TestRateLimitResponseIsProblemJSON verifies the 429 a caller actually
// receives: RFC 7807 body under the problem+json media type, not the
// application/json the legacy shape used.
func TestRateLimitResponseIsProblemJSON(t *testing.T) {
	handler := RateLimitMiddleware(RateLimitConfig{
		RequestsPerSecond: 1.0,
		Burst:             1,
		TTL:               time.Minute,
		IPLookups:         []string{"RemoteAddr"},
	})(mockHandler())

	var blocked *httptest.ResponseRecorder
	for i := 0; i < 10; i++ {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.0.2.10:1234"
		handler.ServeHTTP(rr, req)
		if rr.Code == http.StatusTooManyRequests {
			blocked = rr
			break
		}
	}
	require.NotNil(t, blocked, "expected a request to be rate limited")

	assert.Equal(t, core.ProblemContentType, blocked.Header().Get("Content-Type"))

	var body map[string]any
	require.NoError(t, json.Unmarshal(blocked.Body.Bytes(), &body))

	assert.Equal(t, "Too Many Requests", body["title"])
	assert.Equal(t, float64(http.StatusTooManyRequests), body["status"])
	assert.NotEmpty(t, body["detail"])
	assert.NotContains(t, body, "error", "legacy error field must not reappear")
}
