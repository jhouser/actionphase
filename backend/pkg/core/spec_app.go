package core

import (
	"log/slog"
	"os"

	"actionphase/pkg/observability"
)

// NewSpecApp builds the minimal App needed to construct the route tree without
// serving it.
//
// Used by cmd/genopenapi, which builds the router purely to render the OpenAPI
// document. Nothing here is reachable by a request: Pool is nil because
// registering handlers never dials the database, and no handler runs.
//
// This exists rather than reusing NewTestApp because that lives in
// test_utils.go, which imports "testing" — linking it into a real command would
// drag the testing package and its flags into the binary.
func NewSpecApp() *App {
	return &App{
		Pool:          nil,
		Logger:        *slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
		Config:        specConfig(),
		ObsLogger:     observability.NewLogger("genopenapi", "error"),
		Observability: observability.New("genopenapi", "error"),
	}
}

// specConfig supplies the few values the route tree reads while being built.
//
// The JWT secret is a placeholder: jwtauth.New needs one to construct the token
// authenticator, but no token is ever signed or verified here.
func specConfig() *Config {
	return &Config{
		JWT: JWTConfig{
			Secret:    "openapi-generation-only",
			Algorithm: "HS256",
		},
		App: AppConfig{
			// Not "development": that would register the debug routes, which
			// are not part of the documented API surface.
			Environment: "production",
		},
		Storage: StorageConfig{
			Backend: "local",
		},
	}
}
