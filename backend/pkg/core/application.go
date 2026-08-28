package core

import (
	"context"

	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"actionphase/pkg/observability"
)

// App holds the core application dependencies and configuration.
// It serves as the central context for the entire application,
// providing access to database, logging, and configuration.
//
// Usage Example:
//
//	config, err := LoadConfig()
//	if err != nil {
//	    log.Fatal("Config error", err)
//	}
//
//	app := &App{
//	    Logger: *slog.Default(),
//	    Pool:   dbPool,
//	    Config: config,
//	}
//
//	// Pass app to handlers
//	handler := &games.Handler{App: app}
type App struct {
	// Logger provides structured logging throughout the application
	Logger slog.Logger

	// ObsLogger provides context-aware structured logging with correlation IDs
	ObsLogger *observability.Logger

	// Pool provides database connection pooling for PostgreSQL
	Pool *pgxpool.Pool

	// DB is an alias for Pool for compatibility with services expecting DB
	DB *pgxpool.Pool

	// Config holds all application configuration loaded from environment
	Config *Config

	// Observability provides unified logging, metrics, and tracing
	Observability *observability.Observability

	// Storage provides file storage backend (local filesystem or S3)
	Storage StorageBackendInterface

	// DiscordNotifier dispatches Discord DMs for notifications.
	// When nil, no Discord notifications are sent.
	// When DISCORD_BOT_TOKEN is not set, a MockClient is injected for local testing.
	DiscordNotifier DiscordClientInterface
}

// Logger interface for dependency injection in middleware and services.
// This allows components to be testable with mock loggers.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// ContextLogger interface for context-aware logging with structured fields.
// This is the preferred interface for new code that supports observability.
type ContextLogger interface {
	Debug(ctx context.Context, msg string, args ...any)
	Info(ctx context.Context, msg string, args ...any)
	Warn(ctx context.Context, msg string, args ...any)
	Error(ctx context.Context, msg string, args ...any)
	LogError(ctx context.Context, err error, msg string, args ...any)
	LogOperation(ctx context.Context, operation string, args ...any) func()
}
