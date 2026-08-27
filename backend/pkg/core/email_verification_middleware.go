package core

import (
	"context"
	"fmt"
	"net/http"
	"os"

	db "actionphase/pkg/db/models"

	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RequireEmailVerificationMiddleware is middleware that requires the user to have a verified email
// This should be used on routes that require email verification (e.g., creating games, posting content)
//
// The middleware respects the REQUIRE_EMAIL_VERIFICATION environment variable:
// - "true" (default for production): Email verification is enforced
// - "false" (default for development): Email verification is not enforced
//
// Recommended routes to protect (apply this middleware to):
// - POST /api/v1/games - Create game
// - POST /api/v1/games/{gameId}/posts - Create common room post
// - POST /api/v1/games/{gameId}/posts/{postId}/comments - Create comment
// - POST /api/v1/games/{gameId}/characters - Create character
// - POST /api/v1/games/{gameId}/apply - Apply to game
//
// Example usage in router:
//
//	r.Group(func(r chi.Router) {
//	    r.Use(jwtauth.Verifier(tokenAuth))
//	    r.Use(jwtauth.Authenticator(tokenAuth))
//	    r.Use(core.RequireAuthenticationMiddleware(userService))
//	    r.Use(core.RequireEmailVerificationMiddleware(pool))  // Add email verification requirement
//	    r.Post("/", gameHandler.CreateGame)
//	})
func RequireEmailVerificationMiddleware(pool *pgxpool.Pool) func(http.Handler) http.Handler {
	// Check if email verification is required (default to true for production safety)
	requireVerification := os.Getenv("REQUIRE_EMAIL_VERIFICATION")
	if requireVerification == "" {
		requireVerification = "true" // Default to requiring verification
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// If email verification is not required, skip the check
			if requireVerification == "false" {
				next.ServeHTTP(w, r)
				return
			}

			// Get user ID from JWT token (stored in "sub" claim)
			token, _, err := jwtauth.FromContext(r.Context())
			if err != nil {
				render.Render(w, r, ErrUnauthorized("invalid token"))
				return
			}

			userIDStr, ok := token.Get("sub")
			if !ok {
				render.Render(w, r, ErrUnauthorized("user id not found in token"))
				return
			}

			// Parse user ID string to int32
			var userID int32
			_, err = fmt.Sscanf(userIDStr.(string), "%d", &userID)
			if err != nil || userID == 0 {
				render.Render(w, r, ErrUnauthorized("invalid user id in token"))
				return
			}

			// Get user from database to check email verification status
			queries := db.New(pool)
			user, err := queries.GetUser(r.Context(), userID)
			if err != nil {
				render.Render(w, r, ErrInternalError(err))
				return
			}

			// Check if email is verified
			if !user.EmailVerified {
				render.Render(w, r, ErrForbidden("Please verify your email address to perform this action. Check your email for a verification link or request a new one."))
				return
			}

			// Email is verified, continue to next handler
			next.ServeHTTP(w, r)
		})
	}
}

// RequireVerifiedEmailCtx is the context-based form of the middleware above,
// for huma handlers, which receive a context rather than a *http.Request and so
// cannot be wrapped by an http.Handler middleware.
//
// It performs the same two checks in the same order and returns the same
// render.Renderer errors, so a converted endpoint answers identically. Callers
// translate the renderer into their framework's error type.
//
// Returns nil when the caller may proceed — including when verification is
// switched off by REQUIRE_EMAIL_VERIFICATION=false, which is the development
// default.
func RequireVerifiedEmailCtx(ctx context.Context, pool *pgxpool.Pool) render.Renderer {
	// Read per call rather than once at construction: unlike the middleware,
	// which is built at route-registration time, there is no construction step
	// here to capture it in.
	if os.Getenv("REQUIRE_EMAIL_VERIFICATION") == "false" {
		return nil
	}

	token, _, err := jwtauth.FromContext(ctx)
	if err != nil {
		return ErrUnauthorized("invalid token")
	}

	userIDStr, ok := token.Get("sub")
	if !ok {
		return ErrUnauthorized("user id not found in token")
	}

	var userID int32
	if _, err := fmt.Sscanf(userIDStr.(string), "%d", &userID); err != nil || userID == 0 {
		return ErrUnauthorized("invalid user id in token")
	}

	user, err := db.New(pool).GetUser(ctx, userID)
	if err != nil {
		return ErrInternalError(err)
	}

	if !user.EmailVerified {
		return ErrForbidden("Please verify your email address to perform this action. Check your email for a verification link or request a new one.")
	}

	return nil
}
