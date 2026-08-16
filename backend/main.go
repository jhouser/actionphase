package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"

	"actionphase/pkg/core"
	dbsvc "actionphase/pkg/db/services"
	dbactions "actionphase/pkg/db/services/actions"
	phasesvc "actionphase/pkg/db/services/phases"
	"actionphase/pkg/discord"
	"actionphase/pkg/exports"
	"actionphase/pkg/http"
	"actionphase/pkg/observability"
	"actionphase/pkg/scheduler"
	"actionphase/pkg/storage"
)

func main() {
	// Load .env file if it exists (for local development)
	// Look for .env file in current directory and parent directories
	loadDotEnvFile()

	// Load configuration from environment with validation
	config, err := core.LoadConfig()
	if err != nil {
		// Log to stderr before observability is available
		fmt.Fprintf(os.Stderr, "FATAL: Configuration error: %v\n", err)
		os.Exit(1)
	}

	// Initialize OpenTelemetry tracing. When OTEL_ENABLED=false a no-op provider
	// is installed so the rest of the code is unaffected.
	tracerShutdown, err := observability.InitTracer(observability.TracerConfig{
		Enabled:     config.Telemetry.OTELEnabled,
		Endpoint:    config.Telemetry.OTELEndpoint,
		Environment: config.App.Environment,
		ServiceName: "actionphase-backend",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: Failed to initialize tracer: %v\n", err)
		os.Exit(1)
	}

	// Initialize OpenTelemetry metrics. When OTEL_ENABLED=false a no-op provider
	// is installed so the Prometheus /metrics endpoint still works locally.
	otelMetrics, meterShutdown, err := observability.InitMeterProvider(observability.MeterConfig{
		Enabled:      config.Telemetry.OTELEnabled,
		OTELEndpoint: config.Telemetry.OTELEndpoint,
		Environment:  config.App.Environment,
		ServiceName:  "actionphase-backend",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: Failed to initialize meter provider: %v\n", err)
		os.Exit(1)
	}

	// Setup observability system with structured logging and metrics
	obs := observability.New(config.App.Environment, config.App.LogLevel)
	obs.OTELMetrics = otelMetrics

	// Initialize OTEL log shipping. When enabled, obs.Logger fans out to both
	// the local console and Grafana Cloud Loki via the OTLP pipeline.
	logShutdown, logErr := observability.InitLogProvider(observability.LogConfig{
		Enabled:      config.Telemetry.OTELEnabled,
		OTELEndpoint: config.Telemetry.OTELEndpoint,
		Environment:  config.App.Environment,
		ServiceName:  "actionphase-backend",
		LogLevel:     config.App.LogLevel,
	}, obs.Logger)
	if logErr != nil {
		fmt.Fprintf(os.Stderr, "FATAL: Failed to initialize log provider: %v\n", logErr)
		os.Exit(1)
	}

	// Shutdown order (defers run LIFO): tracer first, then meter, then log last.
	// Log must shut down last so any records emitted during trace/metric flush are shipped.
	defer logShutdown()
	defer meterShutdown()
	defer tracerShutdown()

	// Keep backward compatibility with existing slog.Logger
	logLevel := slog.LevelInfo
	switch config.App.LogLevel {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))

	logger.Info("Starting ActionPhase backend",
		"environment", config.App.Environment,
		"log_level", config.App.LogLevel,
		"port", config.Server.Port)

	// Setup database connection pool
	ctx := context.Background()
	poolConfig, err := pgxpool.ParseConfig(config.Database.URL)
	if err != nil {
		logger.Error("Invalid database configuration", "error", err)
		os.Exit(1)
	}

	// Configure connection pool settings for optimal performance and reliability
	poolConfig.MaxConns = int32(config.Database.MaxConnections)
	poolConfig.MinConns = int32(config.Database.MinConnections)
	poolConfig.MaxConnLifetime = config.Database.MaxConnLifetime
	poolConfig.MaxConnIdleTime = config.Database.MaxIdleTime
	poolConfig.HealthCheckPeriod = config.Database.HealthCheckPeriod

	// Instrument all database queries with OpenTelemetry spans.
	// When OTEL_ENABLED=false the global tracer is a no-op so this is free.
	poolConfig.ConnConfig.Tracer = otelpgx.NewTracer()

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		logger.Error("Failed to create database pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Test database connection
	if err := pool.Ping(ctx); err != nil {
		logger.Error("Database connection failed", "error", err)
		os.Exit(1)
	}

	// Log connection pool configuration for monitoring and debugging
	logger.Info("Database connection established",
		"max_connections", poolConfig.MaxConns,
		"min_connections", poolConfig.MinConns,
		"max_conn_lifetime", poolConfig.MaxConnLifetime,
		"max_idle_time", poolConfig.MaxConnIdleTime,
		"health_check_period", poolConfig.HealthCheckPeriod)

	// Initialize storage backend based on configuration
	var storageBackend core.StorageBackendInterface
	if config.Storage.Backend == "s3" {
		// S3 storage for production
		// Only use custom PublicURL if explicitly set (for CDN)
		// Otherwise, let S3Storage auto-generate the S3 URL
		publicURL := ""
		if os.Getenv("STORAGE_PUBLIC_URL") != "" {
			publicURL = config.Storage.PublicURL
		}

		s3Storage, err := storage.NewS3Storage(
			config.Storage.S3Bucket,
			config.Storage.S3Region,
			publicURL,
			config.Storage.S3Endpoint,
		)
		if err != nil {
			logger.Error("Failed to initialize S3 storage", "error", err)
			os.Exit(1)
		}
		storageBackend = s3Storage
		logger.Info("Using S3 storage",
			"bucket", config.Storage.S3Bucket,
			"region", config.Storage.S3Region,
			"public_url", publicURL)
	} else {
		// Local filesystem storage for development/staging
		storageBackend = storage.NewLocalStorage(
			config.Storage.LocalPath,
			config.Storage.PublicURL,
		)
		logger.Info("Using local filesystem storage",
			"path", config.Storage.LocalPath,
			"public_url", config.Storage.PublicURL)

		// Ensure upload directory exists
		if err := os.MkdirAll(config.Storage.LocalPath, 0755); err != nil {
			logger.Error("Failed to create upload directory", "error", err)
			os.Exit(1)
		}
	}

	// Initialize Discord notifier
	// When DISCORD_BOT_TOKEN is set, use the real bot client.
	// Otherwise use the mock client (logs to stdout for local development).
	var discordNotifier core.DiscordClientInterface
	if config.Discord.BotToken != "" {
		discordNotifier = &discord.BotClient{
			BotToken: config.Discord.BotToken,
			Logger:   obs.Logger,
		}
		logger.Info("Discord notifier: using bot client")
	} else {
		discordNotifier = &discord.MockClient{Logger: obs.Logger}
		logger.Info("Discord notifier: using mock client (DISCORD_BOT_TOKEN not set)")
	}

	// Register the Discord notifier as the application-wide notifier so that
	// service-internal NotificationService instantiations also dispatch DMs.
	dbsvc.SetAppDiscordNotifier(discordNotifier)

	// Initialize application context with observability
	app := &core.App{
		Logger:          *logger,
		ObsLogger:       obs.Logger,
		Pool:            pool,
		DB:              pool, // Alias for compatibility
		Config:          config,
		Observability:   obs,
		Storage:         storageBackend,
		DiscordNotifier: discordNotifier,
	}

	// Run database migrations if configured
	if config.App.RunMigrations {
		if err := runMigrations(logger, pool); err != nil {
			logger.Error("Migration failed", "error", err)
			// Don't exit - allow manual migration in production
			if config.IsProduction() {
				logger.Warn("Skipping failed migrations in production - please run manually")
			} else {
				os.Exit(1)
			}
		}
	} else {
		logger.Info("Skipping database migrations (RUN_MIGRATIONS=false)")
	}

	// Start phase scheduler (auto-activates phases based on start_time)
	phaseService := &phasesvc.PhaseService{DB: pool, Logger: obs.Logger}
	sched := scheduler.New(phaseService, obs.Logger, time.Minute)
	cancelScheduler := sched.Start(ctx)
	defer cancelScheduler()

	// Start the staged result release worker. It reveals the later parts of a
	// timed result chain once each part's delay has elapsed. Deliberately
	// independent of the phase scheduler above: a chain must keep releasing on
	// its own schedule even if its phase ends or the game advances.
	actionService := &dbactions.ActionSubmissionService{
		DB:                  pool,
		Logger:              obs.Logger,
		NotificationService: dbsvc.NewNotificationService(pool, obs.Logger),
	}
	releaseWorker := dbactions.NewStagedReleaseWorker(actionService, obs.Logger, dbactions.DefaultReleaseInterval)
	cancelReleaseWorker := releaseWorker.Start(ctx)
	defer cancelReleaseWorker()

	// Start the game archive export worker. It requeues jobs abandoned by a
	// crashed process on startup, then drains the queue on each tick.
	exportService := exports.NewService(pool, config.Storage.ArchivePath, obs.Logger)
	exportWorker := exports.NewWorker(exportService, obs.Logger, exports.DefaultPollInterval)
	cancelExportWorker := exportWorker.Start(ctx)
	defer cancelExportWorker()

	// Periodically delete expired sessions to prevent accumulation
	sessionService := &dbsvc.SessionService{DB: pool, Logger: obs.Logger}
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		// Run once on startup to clean up any accumulated expired sessions
		if err := sessionService.CleanupExpiredSessions(ctx); err != nil {
			obs.Logger.LogError(ctx, err, "Startup expired session cleanup failed")
		}
		for {
			select {
			case <-ticker.C:
				if err := sessionService.CleanupExpiredSessions(ctx); err != nil {
					obs.Logger.LogError(ctx, err, "Periodic expired session cleanup failed")
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Start HTTP server
	logger.Info("Starting HTTP server",
		"address", config.GetServerAddress(),
		"environment", config.App.Environment)

	httpHandler := &http.Handler{
		App: app,
	}

	// httpHandler.Start() should be updated to use config for server settings
	// For now, it will use the existing implementation
	httpHandler.Start()
}

// runMigrations applies database schema migrations
func runMigrations(logger *slog.Logger, pool *pgxpool.Pool) error {
	logger.Info("Running database migrations...")

	// Convert pgx pool to database/sql for migrate library
	database := stdlib.OpenDBFromPool(pool)
	defer database.Close()

	driver, err := postgres.WithInstance(database, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://pkg/db/migrations",
		"postgres", // database name for migrate
		driver,
	)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}
	defer m.Close()

	// Check for dirty migrations and auto-fix
	version, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		// If we can't get version info, something is wrong
		return fmt.Errorf("failed to get migration version: %w", err)
	}

	if dirty {
		logger.Warn("Detected dirty migration state, attempting to fix...",
			"version", version)

		// Force the version to clean state
		// This marks the migration as complete in the schema_migrations table
		if err := m.Force(int(version)); err != nil {
			return fmt.Errorf("failed to force clean migration state: %w", err)
		}

		logger.Info("Migration state fixed", "version", version)

		// Close the current migrate instance and create a fresh one
		// This ensures the migrate library properly detects the current clean state
		m.Close()

		// Recreate driver and migrate instance
		driver, err = postgres.WithInstance(database, &postgres.Config{})
		if err != nil {
			return fmt.Errorf("failed to recreate migration driver: %w", err)
		}

		m, err = migrate.NewWithDatabaseInstance(
			"file://pkg/db/migrations",
			"postgres",
			driver,
		)
		if err != nil {
			return fmt.Errorf("failed to recreate migration instance: %w", err)
		}
		defer m.Close()

		logger.Info("Migration instance recreated, continuing with pending migrations...")
	}

	// Apply migrations
	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migration failed: %w", err)
	}

	if err == migrate.ErrNoChange {
		logger.Info("Database is up to date")
	} else {
		logger.Info("Database migrations completed successfully")
	}

	return nil
}

// loadDotEnvFile loads environment variables from .env file if it exists.
// It searches for .env file in current directory and parent directories,
// making it work whether you run from backend/ or project root.
//
// This function is designed to fail gracefully - if no .env file is found
// or if there are errors reading it, the application continues using
// system environment variables.
func loadDotEnvFile() {
	// List of possible .env file locations (in order of preference)
	envFiles := []string{
		".env",       // Current directory
		"../.env",    // Parent directory (for running from backend/)
		"../../.env", // Two levels up (for nested execution)
	}

	for _, envFile := range envFiles {
		if absPath, err := filepath.Abs(envFile); err == nil {
			if _, err := os.Stat(absPath); err == nil {
				// .env file exists, try to load it
				if err := godotenv.Load(absPath); err == nil {
					// Successfully loaded .env file
					return
				}
				// Failed to load .env file, continue searching
			}
		}
	}

	// No .env file found - this is okay for production or when using system env vars
}
