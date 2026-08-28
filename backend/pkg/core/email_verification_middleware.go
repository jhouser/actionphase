package core

import (
	"context"
	"fmt"
	"os"

	db "actionphase/pkg/db/models"

	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
	"github.com/jackc/pgx/v5/pgxpool"
)

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
