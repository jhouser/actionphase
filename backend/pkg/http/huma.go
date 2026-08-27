package http

// Shared huma setup for the type-first handler migration.
//
// Handlers are moving from func(w http.ResponseWriter, r *http.Request) to
// func(ctx, *Input) (*Output, error) so that the OpenAPI spec is derived from
// Go types instead of maintained by hand. See .claude/planning/huma-migration.md.
//
// Chi is NOT going away: huma mounts onto the existing chi router via the
// humachi adapter, so routing, mounting, and every r.Use middleware continue to
// work unchanged. Only the handler signature and response encoding differ.

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"gopkg.in/yaml.v3"
)

// legacyError reproduces the error body the API has always sent:
//
//	{"status": "Forbidden.", "error": "admin privileges required"}
//
// Huma's default is RFC 7807 (application/problem+json), which the frontend
// cannot parse: frontend/src/lib/errors.ts reads `.error`, falls through to
// `.status`, and would render the bare number 422 as the user's error message.
// Adopting RFC 7807 properly is tracked in
// .claude/planning/rfc7807-error-format.md; until then this shim keeps the
// migration invisible to clients.
type legacyError struct {
	StatusText string `json:"status"`
	ErrorMsg   string `json:"error,omitempty"`

	status int
}

func (e *legacyError) Error() string  { return e.ErrorMsg }
func (e *legacyError) GetStatus() int { return e.status }

// legacyStatusText mirrors the StatusText values core's error constructors use,
// so a converted endpoint is byte-identical to the chi one it replaced.
func legacyStatusText(status int) string {
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
		// change status codes.
		//
		// This is safe for clients: frontend/src/types/errors.ts maps both 400
		// and 422 to ErrorType.VALIDATION_ERROR, and errors.ts handles them in
		// the same switch branch — so the two are already interchangeable
		// downstream. Revisit if the API ever wants to distinguish
		// "malformed" from "semantically invalid".
		if status == http.StatusUnprocessableEntity {
			status = http.StatusBadRequest
		}

		return &legacyError{
			StatusText: legacyStatusText(status),
			ErrorMsg:   detail,
			status:     status,
		}
	}
}

// newHumaAPI builds a huma API bound to an existing chi router.
//
// DocsPath and SchemaLinkTransformer are disabled deliberately:
//   - docs are served by pkg/docs (Swagger UI at /api/v1/docs/)
//   - the $schema link huma injects into response bodies would change the
//     response shape that 236 frontend call sites depend on
func newHumaAPI(r chi.Router, title, version string) huma.API {
	cfg := huma.DefaultConfig(title, version)
	cfg.DocsPath = ""
	cfg.SchemasPath = ""
	cfg.CreateHooks = nil
	return humachi.New(r, cfg)
}

// generatedSpecFor renders the OpenAPI documents of the migrated packages as a
// single YAML spec, re-prefixing each one's paths with the mount point it is
// served under.
//
// Huma operations are registered relative to their chi mount (e.g. "/users" on
// a router mounted at /api/v1/admin), because that is what makes routing work.
// The spec, however, is served against a base URL of /api/v1, so the mount
// prefix has to be added back for the documented URL to be correct.
func generatedSpecFor(apis map[string]huma.API) ([]byte, error) {
	merged := map[string]any{"paths": map[string]any{}, "components": map[string]any{"schemas": map[string]any{}}}
	paths := merged["paths"].(map[string]any)
	schemas := merged["components"].(map[string]any)["schemas"].(map[string]any)

	for prefix, api := range apis {
		if api == nil {
			continue
		}
		raw, err := api.OpenAPI().YAML()
		if err != nil {
			return nil, err
		}
		var doc map[string]any
		if err := yaml.Unmarshal(raw, &doc); err != nil {
			return nil, err
		}
		if p, ok := doc["paths"].(map[string]any); ok {
			for path, item := range p {
				paths[prefix+path] = item
			}
		}
		if comp, ok := doc["components"].(map[string]any); ok {
			if s, ok := comp["schemas"].(map[string]any); ok {
				for name, schema := range s {
					schemas[name] = schema
				}
			}
		}
	}

	return yaml.Marshal(merged)
}
