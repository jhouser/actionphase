package core

import (
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
