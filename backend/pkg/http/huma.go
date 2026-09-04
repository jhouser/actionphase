package http

// Huma wiring for the type-first handler migration.
//
// Handlers are moving from func(w http.ResponseWriter, r *http.Request) to
// func(ctx, *Input) (*Output, error) so that the OpenAPI spec is derived from
// Go types instead of maintained by hand. See .claude/planning/huma-migration.md.
//
// Chi is NOT going away: huma mounts onto the existing chi router via the
// humachi adapter, so routing, mounting, and every r.Use middleware continue to
// work unchanged. Only the handler signature and response encoding differ.
//
// The API config and legacy error shim live in pkg/humaconfig, a leaf package,
// so handler tests can use the same setup without importing pkg/http (which
// imports every handler package and would be an import cycle).

import (
	"actionphase/pkg/humaconfig"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
	"gopkg.in/yaml.v3"
)

// InstallProblemErrorFormat stamps the correlation ID into RFC 7807 errors.
func InstallProblemErrorFormat() { humaconfig.InstallProblemErrorFormat() }

// newHumaAPI builds a huma API bound to an existing chi router.
func newHumaAPI(r chi.Router, title, version string) huma.API {
	return humaconfig.New(r, title, version)
}

// generatedSpecFor renders the OpenAPI documents of the migrated packages as a
// single YAML spec, re-prefixing each one's paths with the mount point it is
// served under.
//
// Huma operations are registered relative to their chi mount (e.g. "/users" on
// a router mounted at /api/v1/admin), because that is what makes routing work.
// The spec, however, is served against a base URL of /api/v1, so the mount
// prefix has to be added back for the documented URL to be correct.
//
// A prefix maps to a *slice* of APIs because one mount may need several, when
// its routes divide into groups with different middleware. /auth is the case
// that forced this: rate-limited, public, probe (Verifier only) and fully
// protected routes are four chi groups under one mount, and huma binds to a
// router, so each group needs its own API. Their paths are disjoint, so merging
// the documents is safe; keying by a bare huma.API silently dropped all but one.
func generatedSpecFor(apis map[string][]huma.API) ([]byte, error) {
	merged := map[string]any{"paths": map[string]any{}, "components": map[string]any{"schemas": map[string]any{}}}
	paths := merged["paths"].(map[string]any)
	schemas := merged["components"].(map[string]any)["schemas"].(map[string]any)

	for prefix, group := range apis {
		for _, api := range group {
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
					// A package's index operation registers as "/" relative to
					// its mount, which would document "/dashboard/" — a second,
					// trailing-slash entry sitting beside the hand-written
					// "/dashboard" rather than superseding it.
					full := prefix + path
					if path == "/" {
						full = prefix
					}
					paths[full] = item
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
	}

	return yaml.Marshal(merged)
}
