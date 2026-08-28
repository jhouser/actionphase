// Package humaconfig holds the huma setup shared by production routing and
// package tests.
//
// It exists as a leaf package (importing only huma and chi) so that handler
// tests can build an API configured exactly like the served one without
// importing pkg/http, which would be an import cycle: pkg/http imports every
// handler package.
//
// Before this existed, each package's tests kept their own copy of the config
// and the error shim. A copy that drifts from production silently stops
// testing what ships, which is the failure mode this package prevents.
package humaconfig

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
)

// LegacyError reproduces the error body the API has always sent:
//
//	{"status": "Forbidden.", "error": "admin privileges required"}
//
// Huma's default is RFC 7807 (application/problem+json), which the frontend
// cannot parse. See .claude/planning/rfc7807-error-format.md.
type LegacyError struct {
	StatusText string `json:"status"`
	ErrorMsg   string `json:"error,omitempty"`

	status int
}

func (e *LegacyError) Error() string  { return e.ErrorMsg }
func (e *LegacyError) GetStatus() int { return e.status }

// LegacyStatusText mirrors the StatusText values core's error constructors use,
// so a converted endpoint is byte-identical to the chi one it replaced.
func LegacyStatusText(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "Invalid request."
	case http.StatusUnauthorized:
		return "Unauthorized."
	case http.StatusForbidden:
		return "Forbidden."
	case http.StatusNotFound:
		return "Resource not found."
	case http.StatusConflict:
		return "Conflict."
	case http.StatusUnprocessableEntity:
		return "Invalid request."
	default:
		return "Internal server error."
	}
}

// InstallLegacyErrorFormat points huma's error constructor at the legacy shape.
// It mutates a package-level variable in huma, so it must run once, before any
// huma API is served. Idempotent.
func InstallLegacyErrorFormat() {
	huma.NewError = func(status int, msg string, errs ...error) huma.StatusError {
		// Validation failures arrive as errs; their message is more specific
		// than the generic msg ("validation failed"), so prefer them.
		detail := msg
		if len(errs) > 0 && errs[0] != nil {
			detail = errs[0].Error()
		}

		// Huma hardcodes 422 for request binding/validation failures (a bad
		// path param, a missing required field). The chi handlers these
		// replace parsed by hand and returned 400 via core.ErrInvalidRequest,
		// and existing tests assert 400. Remap so the migration does not
		// change status codes. Whether to adopt huma's split is tracked in
		// .claude/planning/http-status-codes.md.
		if status == http.StatusUnprocessableEntity {
			status = http.StatusBadRequest
		}

		return &LegacyError{
			StatusText: LegacyStatusText(status),
			ErrorMsg:   detail,
			status:     status,
		}
	}
}

// New builds a huma API bound to an existing chi router.
//
// DocsPath and SchemaLinkTransformer are disabled deliberately:
//   - docs are served by pkg/docs (Swagger UI at /api/v1/docs/)
//   - the $schema link huma injects into response bodies would change the
//     response shape that 236 frontend call sites depend on
func New(r chi.Router, title, version string) huma.API {
	InstallLegacyErrorFormat()
	cfg := huma.DefaultConfig(title, version)
	cfg.DocsPath = ""
	cfg.SchemasPath = ""
	cfg.CreateHooks = nil
	api := humachi.New(r, cfg)
	// Lets handlers reach the underlying request for client IP, user agent and
	// cookie writes; see RequestMiddleware.
	api.UseMiddleware(RequestMiddleware)
	return api
}

// Trimming request strings
//
// core.ValidateStruct trims every string field in place *before* validating, so
// for chi handlers a title of "   " failed `min=1`. Huma's minLength counts raw
// characters, so the same input passes and a blank-titled row reaches the
// database. Body structs restore the old behaviour by implementing Resolve:
//
//	func (b *createBody) Resolve(huma.Context) []error {
//	    return humaconfig.TrimStrings(b)
//	}
//
// TrimStrings trims all exported string fields of v in place, reporting the
// JSON names of those which a non-zero minLength requires but which are empty
// after trimming.
//
// Call it from a body struct's Resolve. It is exported separately from
// TrimmedBody because Go embeds methods, not field access: a promoted Resolve
// cannot see the outer struct's fields, so each body type calls this on itself.
func TrimStrings(v any) []error {
	val := reflect.ValueOf(v)
	if val.Kind() != reflect.Ptr || val.IsNil() {
		return nil
	}
	val = val.Elem()
	if val.Kind() != reflect.Struct {
		return nil
	}

	var errs []error
	t := val.Type()
	for i := 0; i < val.NumField(); i++ {
		f := val.Field(i)
		if f.Kind() != reflect.String || !f.CanSet() {
			continue
		}
		trimmed := strings.TrimSpace(f.String())
		f.SetString(trimmed)

		sf := t.Field(i)
		if trimmed != "" {
			continue
		}
		// Only fields declaring a minimum length are required to be non-empty.
		if min := sf.Tag.Get("minLength"); min != "" && min != "0" {
			name := sf.Tag.Get("json")
			if idx := strings.Index(name, ","); idx >= 0 {
				name = name[:idx]
			}
			if name == "" {
				name = sf.Name
			}
			errs = append(errs, &huma.ErrorDetail{
				Message:  fmt.Sprintf("%s must not be blank", name),
				Location: "body." + name,
			})
		}
	}
	return errs
}

// Reaching the underlying *http.Request from a huma handler
//
// Huma handlers receive a context.Context, not a *http.Request, which is
// normally the point: inputs arrive as typed struct fields. A few operations
// genuinely need the raw request anyway --
//
//   - core.GetClientIP reads X-Real-IP, X-Forwarded-For *and* RemoteAddr, and
//     RemoteAddr has no struct-tag equivalent;
//   - setting a cookie needs the http.ResponseWriter.
//
// pkg/auth needs both (login/register record the client IP and user agent on
// the session, and issue the jwt cookie). Rather than reimplement IP extraction
// against huma.Context -- which would fork the precedence rules that decide
// which header wins behind a proxy -- RequestMiddleware stashes the request and
// writer that humachi already holds, so the existing helpers keep working
// unchanged.

type requestCtxKey struct{}

type requestPair struct {
	r *http.Request
	w http.ResponseWriter
}

// RequestMiddleware makes the underlying request and response writer available
// to handlers via RequestFrom. Register it on any API whose operations need
// client IP, user agent, or cookie access.
func RequestMiddleware(ctx huma.Context, next func(huma.Context)) {
	// humachi's context can hand back the pair it wrapped. Any adapter that
	// cannot is simply skipped: RequestFrom then reports false and the caller
	// falls back, rather than panicking.
	type unwrapper interface {
		Unwrap() (*http.Request, http.ResponseWriter)
	}
	if u, ok := ctx.(unwrapper); ok {
		r, w := u.Unwrap()
		ctx = huma.WithValue(ctx, requestCtxKey{}, &requestPair{r: r, w: w})
	}
	next(ctx)
}

// RequestFrom returns the *http.Request and http.ResponseWriter for the current
// operation. It reports false when RequestMiddleware is not installed on the
// API, so callers must handle absence rather than assume.
func RequestFrom(ctx context.Context) (*http.Request, http.ResponseWriter, bool) {
	pair, ok := ctx.Value(requestCtxKey{}).(*requestPair)
	if !ok || pair == nil {
		return nil, nil, false
	}
	return pair.r, pair.w, true
}
