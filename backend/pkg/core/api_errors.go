package core

import (
	"net/http"
	"strings"

	"actionphase/pkg/observability"

	"github.com/go-chi/render"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// ErrResponse is the chi/render half of the API's RFC 7807 error responses.
//
// Two emitters, one shape. Handler errors are serialized by huma (see
// pkg/humaconfig); middleware errors -- notably every 401 and 403 raised before
// a handler runs -- take this chi path instead. The two must agree, because a
// client cannot tell which produced a given response.
//
// Error Response Format (RFC 7807, application/problem+json):
//
//	{
//	  "title": "Forbidden",
//	  "status": 403,
//	  "detail": "admin privileges required",
//	  "instance": "urn:actionphase:correlation:corr-abc123"
//	}
//
// Design Principles:
//   - Internal errors (Err field) are never exposed to clients
//   - HTTP status codes follow REST conventions
//   - Title is the short, static summary of the problem type
//   - Detail explains this specific occurrence (client-safe)
//   - Instance carries the correlation ID, so an error body pasted into a
//     support ticket is enough to find the request in the logs
type ErrResponse struct {
	Err            error `json:"-"` // Internal runtime error (never serialized)
	HTTPStatusCode int   `json:"-"` // HTTP response status code (never serialized)

	Type     string `json:"type,omitempty"`     // URI identifying the error class
	Title    string `json:"title"`              // Short, static summary of the problem type
	Status   int    `json:"status"`             // HTTP status code, mirrored for client convenience
	Detail   string `json:"detail,omitempty"`   // Explanation specific to this occurrence
	Instance string `json:"instance,omitempty"` // URI identifying this occurrence (correlation ID)
}

// Render implements the chi/render.Renderer interface for HTTP response rendering.
// It sets the HTTP status code and allows the JSON marshaling to handle the response body.
func (e *ErrResponse) Render(w http.ResponseWriter, r *http.Request) error {
	render.Status(r, e.HTTPStatusCode)

	// Status mirrors the HTTP status code inside the body, per RFC 7807. It is
	// set here rather than in each constructor so the two cannot drift.
	e.Status = e.HTTPStatusCode

	// Bodies get pasted into support tickets; the X-Correlation-ID header does
	// not. Carrying the ID in `instance` is what makes a user-reported error
	// traceable.
	if id := observability.GetCorrelationID(r.Context()); id != "" {
		e.Instance = "urn:actionphase:correlation:" + id
	}

	// Mark the active span as an error for 5xx responses so Tempo shows them
	// as failed spans. 4xx errors are client mistakes, not service errors.
	if e.HTTPStatusCode >= 500 {
		span := trace.SpanFromContext(r.Context())
		span.SetStatus(codes.Error, e.Detail)
		if e.Err != nil {
			span.RecordError(e.Err)
		}
	}

	return nil
}

// InstallProblemJSONResponder makes chi/render send error bodies as
// application/problem+json.
//
// The Content-Type cannot simply be set inside Render: render.Render calls the
// Renderer first and *then* render.Respond, whose JSON responder
// unconditionally overwrites Content-Type with "application/json"
// (go-chi/render@v1.0.3 responder.go:102). Anything the Renderer sets is
// therefore clobbered before the client sees it.
//
// render.Respond is a package-level variable that chi/render documents as the
// extension point for precisely this ("maybe you want to test if v is an error
// and respond differently"). Wrapping it lets non-error responses keep their
// existing behaviour untouched while error responses get the RFC 7807 type.
//
// It mutates package state, so it must run once during startup, before serving.
// Idempotent.
func InstallProblemJSONResponder() {
	inner := render.Respond
	render.Respond = func(w http.ResponseWriter, r *http.Request, v interface{}) {
		if _, isProblem := v.(*ErrResponse); isProblem {
			// Set after the responder writes, since the responder overwrites
			// the header; the body has been buffered, not flushed, so the
			// header is still mutable here only if WriteHeader has not run.
			// Wrapping the writer is what keeps that true.
			w = &problemJSONWriter{ResponseWriter: w}
		}
		inner(w, r, v)
	}
}

// problemJSONWriter rewrites the Content-Type at the moment the status line is
// written, which is the last point before headers become immutable.
type problemJSONWriter struct {
	http.ResponseWriter
	written bool
}

func (w *problemJSONWriter) WriteHeader(status int) {
	if !w.written {
		w.written = true
		w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *problemJSONWriter) Write(b []byte) (int, error) {
	if !w.written {
		w.written = true
		w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	}
	return w.ResponseWriter.Write(b)
}

// ErrInvalidRequest creates a 400 Bad Request error for invalid request data.
// Use this for validation failures, malformed JSON, missing required fields, etc.
//
// Example Usage:
//
//	if validationErr := validateUser(user); validationErr != nil {
//	    render.Render(w, r, ErrInvalidRequest(validationErr))
//	    return
//	}
func ErrInvalidRequest(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: 400,
		Title:          "Bad Request",
		Detail:         err.Error(),
	}
}

// ErrInternalError creates a 500 Internal Server Error for unexpected system errors.
// Use this for database connection failures, external service errors, etc.
// The internal error details are logged but not exposed to clients.
//
// Example Usage:
//
//	if dbErr := db.SaveUser(user); dbErr != nil {
//	    log.Error("Database save failed", "error", dbErr)
//	    render.Render(w, r, ErrInternalError(dbErr))
//	    return
//	}
func ErrInternalError(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: 500,
		Title:          "Internal Server Error",
		Detail:         "An unexpected error occurred. Please try again later.",
	}
}

// ErrUnauthorized creates a 401 Unauthorized error for authentication failures.
// Use this when user credentials are invalid or missing.
//
// Example Usage:
//
//	if !isValidToken(token) {
//	    render.Render(w, r, ErrUnauthorized("Invalid or expired token"))
//	    return
//	}
func ErrUnauthorized(message string) render.Renderer {
	return &ErrResponse{
		HTTPStatusCode: 401,
		Title:          "Unauthorized",
		Detail:         message,
	}
}

// ErrForbidden creates a 403 Forbidden error for authorization failures.
// Use this when user is authenticated but lacks permission for the action.
//
// Example Usage:
//
//	if userRole != "admin" {
//	    render.Render(w, r, ErrForbidden("Admin access required"))
//	    return
//	}
func ErrForbidden(message string) render.Renderer {
	return &ErrResponse{
		HTTPStatusCode: 403,
		Title:          "Forbidden",
		Detail:         message,
	}
}

// ErrBadRequest creates a 400 Bad Request error for client request errors.
// Similar to ErrInvalidRequest but for more general request processing failures.
//
// Example Usage:
//
//	if gameState == "completed" {
//	    render.Render(w, r, ErrBadRequest(errors.New("Cannot join completed game")))
//	    return
//	}
func ErrBadRequest(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: 400,
		Title:          "Bad Request",
		Detail:         err.Error(),
	}
}

// ErrNotFound creates a 404 Not Found error for missing resources.
// Use this when a specific resource (user, game, etc.) cannot be found.
//
// Example Usage:
//
//	game, err := gameService.GetGame(ctx, gameID)
//	if err != nil {
//	    render.Render(w, r, ErrNotFound("Game not found"))
//	    return
//	}
func ErrNotFound(message string) render.Renderer {
	return &ErrResponse{
		HTTPStatusCode: 404,
		Title:          "Not Found",
		Detail:         message,
	}
}

// ErrConflict creates a 409 Conflict error for resource conflicts.
// Use this when the request conflicts with the current state of the system.
//
// Example Usage:
//
//	if user.IsAlreadyRegistered {
//	    render.Render(w, r, ErrConflict("Username already exists"))
//	    return
//	}
func ErrConflict(message string) render.Renderer {
	return &ErrResponse{
		HTTPStatusCode: 409,
		Title:          "Conflict",
		Detail:         message,
	}
}

// ErrWithStatus creates an error response for a status code that has no
// dedicated constructor above.
//
// It replaced ErrWithCode when the API adopted RFC 7807: the application-
// specific error codes that function carried had no consumer -- the frontend
// never read the `code` field -- and RFC 7807's `type` URI is the idiomatic
// replacement should a machine-readable identifier be wanted again.
//
// Example Usage:
//
//	if game.State != "recruitment" {
//	    render.Render(w, r, ErrWithStatus(400, "Game is not accepting new players"))
//	    return
//	}
func ErrWithStatus(httpStatus int, message string) render.Renderer {
	return &ErrResponse{
		HTTPStatusCode: httpStatus,
		Title:          getTitle(httpStatus),
		Detail:         message,
	}
}

// getTitle returns the RFC 7807 `title` for an HTTP status code.
//
// net/http's own status text is used rather than a hand-maintained map, since
// RFC 7807 titles are exactly the registered reason phrases ("Not Found",
// "Unprocessable Entity") and a local copy would only drift from them.
func getTitle(httpStatus int) string {
	if text := http.StatusText(httpStatus); text != "" {
		return text
	}
	return "Unknown Error"
}

// ErrGameArchived creates a specific error for write operations on completed/cancelled games.
// Completed games are read-only archives and no new content can be created.
func ErrGameArchived() render.Renderer {
	return ErrWithStatus(403,
		"This game is archived and read-only. No new content can be created.")
}

// IsArchivedGameError checks if an error is from an archived game validation failure.
// Returns true if the error message contains "archived", indicating a write operation
// was attempted on a completed or cancelled game.
//
// Example Usage:
//
//	phase, err := phaseService.CreatePhase(ctx, req)
//	if err != nil {
//	    if core.IsArchivedGameError(err) {
//	        render.Render(w, r, core.ErrGameArchived())
//	        return
//	    }
//	    render.Render(w, r, core.ErrInternalError(err))
//	    return
//	}
func IsArchivedGameError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "archived")
}
