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
	"net/http"

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
	return humachi.New(r, cfg)
}
