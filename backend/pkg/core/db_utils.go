package core

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/render"
	"github.com/jackc/pgx/v5"
)

// HandleDBErrorWithID is like HandleDBError but includes the resource ID in the message.
//
// Example Usage:
//
//	user, err := userService.GetUser(ctx, userID)
//	if err != nil {
//	    render.Render(w, r, HandleDBErrorWithID(err, "user", userID))
//	    return
//	}
func HandleDBErrorWithID(err error, resourceName string, id interface{}) render.Renderer {
	if err == nil {
		return nil
	}

	// Check for "not found" errors
	if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound(fmt.Sprintf("%s with ID %v not found", resourceName, id))
	}

	// For other database errors, return internal error
	return ErrInternalError(err)
}

// NotFoundOr500 converts a failed row lookup into the huma error the caller
// should return: 404 when the row simply is not there, 500 for anything else.
//
// This is the huma-era counterpart to HandleDBErrorWithID above, which returns
// a chi render.Renderer and so cannot be used from a huma handler. Without it,
// handlers pass the raw lookup error to huma.Error500InternalServerError and a
// missing id answers 500 "no rows in result set" -- which the frontend then
// reports to the user as a generic "something went wrong" (see the 500 branch
// of frontend/src/lib/errors.ts, which discards the server's message), rather
// than the 404 that lets it say what was actually missing.
//
// resourceName names the thing in the 404 message, e.g. "character".
func NotFoundOr500(err error, resourceName string) error {
	if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
		return huma.Error404NotFound(fmt.Sprintf("%s not found", resourceName))
	}
	return huma.Error500InternalServerError(err.Error())
}
