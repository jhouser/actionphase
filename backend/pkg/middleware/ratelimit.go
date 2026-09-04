package middleware

import (
	"net/http"
	"time"

	core "actionphase/pkg/core"

	"github.com/didip/tollbooth/v7"
	"github.com/didip/tollbooth/v7/limiter"
)

// rateLimitProblemJSON is the 429 body, written as a literal because tollbooth
// wants a string rather than a renderer. It is kept in sync with
// core.ErrResponse by TestRateLimitBodyMatchesErrResponse.
const rateLimitProblemJSON = `{"title":"Too Many Requests","status":429,` +
	`"detail":"Rate limit exceeded. Please try again later."}`

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	RequestsPerSecond float64
	Burst             int
	TTL               time.Duration
	IPLookups         []string
}

// RateLimitMiddleware creates a rate limiting middleware with custom config
func RateLimitMiddleware(config RateLimitConfig) func(http.Handler) http.Handler {
	lmt := tollbooth.NewLimiter(config.RequestsPerSecond, &limiter.ExpirableOptions{
		DefaultExpirationTTL: config.TTL,
	})

	// Set burst size
	lmt.SetBurst(config.Burst)

	// Configure IP lookup methods
	lmt.SetIPLookups(config.IPLookups)

	// Rate limiting is a fourth error emitter, alongside huma, chi/render and
	// core.Authenticator, and it must speak RFC 7807 like the other three.
	//
	// tollbooth takes a fixed string rather than a handler, so this body cannot
	// carry a per-request `instance` correlation ID the way the others do. That
	// is acceptable here -- a 429 is caused by the caller's own request rate,
	// not by server state a correlation ID would help trace.
	lmt.SetMessage(rateLimitProblemJSON)
	lmt.SetMessageContentType(core.ProblemContentType)

	return func(next http.Handler) http.Handler {
		return tollbooth.LimitHandler(lmt, next)
	}
}

// StrictRateLimit creates a strict rate limiter for sensitive endpoints
// (e.g., registration, password reset, login)
// In development mode, uses relaxed limits to allow E2E testing
func StrictRateLimit(isDevelopment bool) func(http.Handler) http.Handler {
	// In development mode, use relaxed limits for E2E testing
	if isDevelopment {
		return RateLimitMiddleware(RateLimitConfig{
			RequestsPerSecond: 10.0, // 10 requests per second (for E2E tests)
			Burst:             20,   // Allow burst of 20
			TTL:               1 * time.Minute,
			IPLookups:         []string{"X-Real-IP", "X-Forwarded-For", "RemoteAddr"},
		})
	}

	// Production: strict rate limiting
	return RateLimitMiddleware(RateLimitConfig{
		RequestsPerSecond: 0.1, // 1 request per 10 seconds
		Burst:             3,   // Allow small burst of 3
		TTL:               60 * time.Minute,
		IPLookups:         []string{"X-Real-IP", "X-Forwarded-For", "RemoteAddr"},
	})
}
