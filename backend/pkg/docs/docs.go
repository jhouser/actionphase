package docs

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	"gopkg.in/yaml.v3"
)

//go:embed static/*
var staticFiles embed.FS

//go:embed *.yaml
var specFiles embed.FS

// Handler provides HTTP endpoints for serving API documentation.
//
// The served spec is a merge of two sources:
//
//   - openapi.yaml — hand-maintained, covers packages not yet migrated
//   - GeneratedSpec — derived from Go types by huma, for packages that have
//     been converted (see .claude/planning/huma-migration.md)
//
// Generated paths win on conflict, so each converted package's entries in the
// hand-written file are superseded automatically and the docs improve as the
// migration proceeds. When every package is converted, openapi.yaml can be
// deleted and this merge removed.
type Handler struct {
	// GeneratedSpec is the huma-produced OpenAPI document, or nil before any
	// package has been migrated.
	GeneratedSpec func() ([]byte, error)
}

// RegisterRoutes adds documentation routes to the chi router
// Routes are registered with relative paths and will be mounted at /api/v1
func (h *Handler) RegisterRoutes(r chi.Router) {
	// Redirect /docs to /docs/
	r.Get("/docs", h.redirectToSwaggerUI)

	// Serve Swagger UI at /docs/
	r.Get("/docs/", h.serveSwaggerUI)

	// Serve OpenAPI spec
	// NOTE: Chi router strips file extensions, so route "/docs/openapi" matches requests to "/docs/openapi.yaml"
	// This is why we register without the .yaml extension
	r.Get("/docs/openapi", h.serveOpenAPISpec)

	// Serve the Swagger UI assets (CSS/JS) from the embedded filesystem.
	// These are self-hosted rather than loaded from a CDN so that the page
	// satisfies the production CSP, which only allows script/style from 'self'.
	r.Handle("/docs/static/*", h.swaggerAssetServer())
}

// swaggerAssetServer returns a file server for the embedded Swagger UI assets
func (h *Handler) swaggerAssetServer() http.Handler {
	assetFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "documentation assets unavailable", http.StatusInternalServerError)
		})
	}

	return http.StripPrefix("/api/v1/docs/static/", http.FileServer(http.FS(assetFS)))
}

// serveOpenAPISpec serves the OpenAPI specification as YAML, merging the
// generated portion over the hand-written one.
func (h *Handler) serveOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	spec, err := specFiles.ReadFile("openapi.yaml")
	if err != nil {
		http.Error(w, "OpenAPI spec not found", http.StatusNotFound)
		return
	}

	if merged, err := h.mergeGenerated(spec); err == nil {
		spec = merged
	}
	// On merge failure fall through to the hand-written spec: stale docs beat
	// no docs, and the generated half is additive.

	w.Header().Set("Content-Type", "application/yaml")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(spec)
}

// mergeGenerated overlays huma's generated paths and schemas onto the
// hand-written spec. Generated entries win, since they are derived from the
// code rather than maintained by hand.
func (h *Handler) mergeGenerated(handWritten []byte) ([]byte, error) {
	if h.GeneratedSpec == nil {
		return handWritten, nil
	}

	genRaw, err := h.GeneratedSpec()
	if err != nil {
		return nil, err
	}

	var base, gen map[string]any
	if err := yaml.Unmarshal(handWritten, &base); err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(genRaw, &gen); err != nil {
		return nil, err
	}

	mergeSection(base, gen, "paths")

	// Schemas live one level deeper, under components.
	if baseComp, ok := base["components"].(map[string]any); ok {
		if genComp, ok := gen["components"].(map[string]any); ok {
			mergeSection(baseComp, genComp, "schemas")
		}
	}

	return yaml.Marshal(base)
}

// mergeSection copies every key of gen[name] over base[name].
func mergeSection(base, gen map[string]any, name string) {
	genSection, ok := gen[name].(map[string]any)
	if !ok {
		return
	}
	baseSection, ok := base[name].(map[string]any)
	if !ok {
		baseSection = map[string]any{}
		base[name] = baseSection
	}
	for k, v := range genSection {
		baseSection[k] = v
	}
}

// redirectToSwaggerUI redirects /docs to /docs/
func (h *Handler) redirectToSwaggerUI(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/api/v1/docs/", http.StatusMovedPermanently)
}

// serveSwaggerUI serves the main Swagger UI page
func (h *Handler) serveSwaggerUI(w http.ResponseWriter, r *http.Request) {
	// Serve a simple Swagger UI HTML page
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>ActionPhase API Documentation</title>
    <link rel="stylesheet" type="text/css" href="/api/v1/docs/static/swagger-ui.css" />
    <style>
        html {
            box-sizing: border-box;
            overflow: -moz-scrollbars-vertical;
            overflow-y: scroll;
        }
        *, *:before, *:after {
            box-sizing: inherit;
        }
        body {
            margin:0;
            background: #fafafa;
        }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="/api/v1/docs/static/swagger-ui-bundle.js"></script>
    <script src="/api/v1/docs/static/swagger-ui-standalone-preset.js"></script>
    <script>
        window.onload = function() {
            SwaggerUIBundle({
                url: '/api/v1/docs/openapi.yaml',
                dom_id: '#swagger-ui',
                deepLinking: true,
                presets: [
                    SwaggerUIBundle.presets.apis,
                    SwaggerUIStandalonePreset
                ],
                plugins: [
                    SwaggerUIBundle.plugins.DownloadUrl
                ],
                layout: "StandaloneLayout",
                validatorUrl: null,
                tryItOutEnabled: false,
                requestInterceptor: function(request) {
                    // Add any request modifications here
                    return request;
                },
                responseInterceptor: function(response) {
                    // Add any response modifications here
                    return response;
                }
            });
        };
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write([]byte(html))
}
