package core

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// DiscordConfig contains Discord OAuth2 and bot configuration.
type DiscordConfig struct {
	// BotToken is the Discord bot token for sending DMs.
	// When empty, a MockClient is used (dev/test mode).
	BotToken string

	// OAuthClientID is the Discord application client ID for OAuth2 login.
	OAuthClientID string

	// OAuthClientSecret is the Discord application client secret.
	OAuthClientSecret string

	// OAuthRedirectURL is the callback URL Discord redirects to after authorization.
	OAuthRedirectURL string
}

// TelemetryConfig contains Grafana Cloud / OpenTelemetry configuration.
// All telemetry is disabled by default; set OTEL_ENABLED=true to ship data.
//
// Authentication: the OTEL Go SDK automatically reads OTEL_EXPORTER_OTLP_HEADERS
// from the environment (format: "key=value,key2=value2"). Set it to e.g.
// "Authorization=Basic <base64(instanceID:apikey)>" for Grafana Cloud auth.
// No explicit header config field is needed — the SDK handles it transparently.
type TelemetryConfig struct {
	// OTELEnabled gates all telemetry shipping (traces, metrics, logs).
	// When false, no-op providers are used — zero performance cost.
	OTELEnabled bool

	// OTELEndpoint is the OTLP HTTP endpoint for traces, metrics, and logs.
	// For Grafana Cloud: https://otlp-gateway-prod-XX.grafana.net/otlp
	OTELEndpoint string

	// PrometheusURL is the Grafana Cloud Prometheus remote_write endpoint (unused by OTLP push).
	// Kept for reference; OTLP push auth uses OTEL_EXPORTER_OTLP_METRICS_HEADERS env var.
	PrometheusURL string
}

// Config holds all application configuration values.
// It provides a centralized location for environment-based configuration
// with sensible defaults and validation.
//
// Usage Example:
//
//	config, err := LoadConfig()
//	if err != nil {
//	    log.Fatal("Failed to load config", "error", err)
//	}
//
//	// Use configuration values
//	pool, err := pgxpool.New(ctx, config.Database.URL)
type Config struct {
	Database  DatabaseConfig  `env:"DATABASE"`
	JWT       JWTConfig       `env:"JWT"`
	Server    ServerConfig    `env:"SERVER"`
	App       AppConfig       `env:"APP"`
	Storage   StorageConfig   `env:"STORAGE"`
	Discord   DiscordConfig   `env:"DISCORD"`
	Telemetry TelemetryConfig `env:"TELEMETRY"`
}

// DatabaseConfig contains database connection and behavior settings.
// Supports both development and production database configurations.
//
// Connection Pool Sizing Guidelines:
//   - Formula: connections_per_server = (total_db_connections * 0.8) / num_app_servers
//   - Example: PostgreSQL max 100, 2 app servers = (100 * 0.8) / 2 = 40 connections per server
//   - Keep MinConnections at ~25% of MaxConnections for warm connections
//
// Environment Defaults:
//   - Development: MaxConns=5, MinConns=1 (single developer, minimal load)
//   - Staging: MaxConns=10, MinConns=3 (light testing, 1 server)
//   - Production: MaxConns=35, MinConns=9 (adjust based on server count and DB limits)
type DatabaseConfig struct {
	// URL is the full PostgreSQL connection string
	// Example: "postgres://user:pass@localhost:5432/dbname?sslmode=disable"
	URL string `env:"DATABASE_URL"`

	// TestURL is used for running tests (optional - falls back to URL with _test suffix)
	TestURL string `env:"TEST_DATABASE_URL"`

	// MaxConnections controls maximum connection pool size
	// Environment-aware defaults: dev=5, staging=10, production=35
	MaxConnections int `env:"DATABASE_MAX_CONNECTIONS"`

	// MinConnections controls minimum connection pool size
	// Maintains warm connections to avoid latency on cold start
	// Environment-aware defaults: dev=1, staging=3, production=9
	MinConnections int `env:"DATABASE_MIN_CONNECTIONS"`

	// MaxConnLifetime controls maximum connection lifetime before recycling
	// Prevents stale connections and handles database-side connection limits
	// Default: 1h (connections are recycled after 1 hour)
	MaxConnLifetime time.Duration `env:"DATABASE_MAX_CONN_LIFETIME"`

	// MaxIdleTime controls how long idle connections stay in pool
	// Idle connections are closed after this duration
	// Default: 30m
	MaxIdleTime time.Duration `env:"DATABASE_MAX_IDLE_TIME"`

	// HealthCheckPeriod controls how often to check connection health
	// Detects and removes dead connections from pool
	// Default: 1m (check every minute)
	HealthCheckPeriod time.Duration `env:"DATABASE_HEALTH_CHECK_PERIOD"`
}

// JWTConfig contains JWT token configuration for authentication.
// Supports both access tokens (short-lived) and refresh tokens (long-lived).
type JWTConfig struct {
	// Secret is the signing key for JWT tokens (required)
	Secret string `env:"JWT_SECRET"`

	// AccessTokenExpiry controls access token lifetime (default: 15m)
	AccessTokenExpiry time.Duration `env:"JWT_ACCESS_TOKEN_EXPIRY"`

	// RefreshTokenExpiry controls refresh token lifetime (default: 7d)
	RefreshTokenExpiry time.Duration `env:"JWT_REFRESH_TOKEN_EXPIRY"`

	// Algorithm specifies the signing algorithm (default: HS256)
	Algorithm string `env:"JWT_ALGORITHM"`
}

// ServerConfig contains HTTP server configuration.
type ServerConfig struct {
	// Port is the HTTP server port (default: 3000)
	Port int `env:"PORT"`

	// Host is the bind address (default: "0.0.0.0")
	Host string `env:"HOST"`

	// ReadTimeout controls request read timeout (default: 10s)
	ReadTimeout time.Duration `env:"SERVER_READ_TIMEOUT"`

	// WriteTimeout controls response write timeout (default: 10s)
	WriteTimeout time.Duration `env:"SERVER_WRITE_TIMEOUT"`

	// IdleTimeout controls idle connection timeout (default: 60s)
	IdleTimeout time.Duration `env:"SERVER_IDLE_TIMEOUT"`
}

// AppConfig contains application-specific settings.
type AppConfig struct {
	// Environment specifies the deployment environment (development, staging, production)
	Environment string `env:"ENVIRONMENT"`

	// LogLevel controls logging verbosity (debug, info, warn, error)
	LogLevel string `env:"LOG_LEVEL"`

	// RunMigrations controls whether to run database migrations at startup
	RunMigrations bool `env:"RUN_MIGRATIONS"`

	// CORS settings for cross-origin requests
	CORSEnabled bool     `env:"CORS_ENABLED"`
	CORSOrigins []string `env:"CORS_ORIGINS"`

	// CommentMaxDepth controls the maximum nesting level for threaded comments (1-10)
	// Comments are shown at depths 0 through (CommentMaxDepth - 1) with Reply buttons
	// "Continue thread" button appears on comments at (CommentMaxDepth - 1) that have deeper replies
	// Default: 5 (shows depths 0-4 with Reply buttons, "Continue thread" at depth 4)
	CommentMaxDepth int `env:"COMMENT_MAX_DEPTH"`

	// RequireRegistrationApproval gates new account creation behind admin approval.
	// When true, new accounts are created with pending_approval=true and cannot login
	// until an admin approves them via the admin panel.
	RequireRegistrationApproval bool `env:"REQUIRE_REGISTRATION_APPROVAL"`
}

// StorageConfig contains file storage configuration.
// Supports both local filesystem (dev/staging) and S3-compatible cloud storage (production).
type StorageConfig struct {
	// Backend specifies the storage backend ("local" or "s3")
	Backend string `env:"STORAGE_BACKEND"`

	// LocalPath is the filesystem path for local storage (e.g., "/var/uploads")
	LocalPath string `env:"STORAGE_LOCAL_PATH"`

	// PublicURL is the base URL for serving uploaded files
	// For local: "http://localhost:3000/uploads"
	// For S3 with CDN: "https://cdn.example.com"
	PublicURL string `env:"STORAGE_PUBLIC_URL"`

	// S3 configuration (only used when Backend = "s3")
	S3Bucket   string `env:"STORAGE_S3_BUCKET"`
	S3Region   string `env:"STORAGE_S3_REGION"`
	S3Endpoint string `env:"STORAGE_S3_ENDPOINT"` // Optional, for S3-compatible services (MinIO, DigitalOcean Spaces)

	// ArchivePath is the filesystem directory for game export archives.
	//
	// Always local disk regardless of Backend, and deliberately outside
	// LocalPath: archives hold every private message in a game, whereas the
	// uploads tree is public-by-design imagery served statically (and backed by
	// a public-read bucket in production). Downloads go through an authorizing
	// handler instead. See exports.ArchiveStore.
	ArchivePath string `env:"STORAGE_ARCHIVE_PATH"`
}

// LoadConfig loads configuration from environment variables with sensible defaults.
// It validates required fields and returns an error if critical configuration is missing.
//
// Required Environment Variables:
//   - DATABASE_URL: PostgreSQL connection string
//   - JWT_SECRET: Secret key for JWT signing (must be strong in production)
//
// Example Environment Setup:
//
//	export DATABASE_URL="postgres://postgres:example@localhost:5432/actionphase?sslmode=disable"
//	export JWT_SECRET="your-super-secret-key-here"
//	export ENVIRONMENT="development"
//	export LOG_LEVEL="info"
func LoadConfig() (*Config, error) {
	// Determine environment first to set appropriate defaults
	environment := getEnvString("ENVIRONMENT", "development")

	// Get environment-specific connection pool defaults
	maxConns, minConns := getPoolDefaults(environment)

	config := &Config{
		Database: DatabaseConfig{
			URL:               getEnvString("DATABASE_URL", ""),
			TestURL:           getEnvString("TEST_DATABASE_URL", ""),
			MaxConnections:    getEnvInt("DATABASE_MAX_CONNECTIONS", maxConns),
			MinConnections:    getEnvInt("DATABASE_MIN_CONNECTIONS", minConns),
			MaxConnLifetime:   getEnvDuration("DATABASE_MAX_CONN_LIFETIME", 1*time.Hour),
			MaxIdleTime:       getEnvDuration("DATABASE_MAX_IDLE_TIME", 30*time.Minute),
			HealthCheckPeriod: getEnvDuration("DATABASE_HEALTH_CHECK_PERIOD", 1*time.Minute),
		},
		JWT: JWTConfig{
			Secret:             getEnvString("JWT_SECRET", ""),
			AccessTokenExpiry:  getEnvDuration("JWT_ACCESS_TOKEN_EXPIRY", 15*time.Minute),
			RefreshTokenExpiry: getEnvDuration("JWT_REFRESH_TOKEN_EXPIRY", 7*24*time.Hour),
			Algorithm:          getEnvString("JWT_ALGORITHM", "HS256"),
		},
		Server: ServerConfig{
			Port:         getEnvInt("PORT", 3000),
			Host:         getEnvString("HOST", "0.0.0.0"),
			ReadTimeout:  getEnvDuration("SERVER_READ_TIMEOUT", 10*time.Second),
			WriteTimeout: getEnvDuration("SERVER_WRITE_TIMEOUT", 10*time.Second),
			IdleTimeout:  getEnvDuration("SERVER_IDLE_TIMEOUT", 60*time.Second),
		},
		App: AppConfig{
			Environment:                 environment,
			LogLevel:                    getEnvString("LOG_LEVEL", "info"),
			RunMigrations:               getEnvBool("RUN_MIGRATIONS", true),
			CORSEnabled:                 getEnvBool("CORS_ENABLED", true),
			CORSOrigins:                 getEnvStringSlice("CORS_ORIGINS", []string{"http://localhost:5173"}),
			CommentMaxDepth:             getEnvInt("COMMENT_MAX_DEPTH", 5),
			RequireRegistrationApproval: getEnvBool("REQUIRE_REGISTRATION_APPROVAL", false),
		},
		Storage: StorageConfig{
			Backend:    getEnvString("STORAGE_BACKEND", "local"),
			LocalPath:  getEnvString("STORAGE_LOCAL_PATH", "./uploads"),
			PublicURL:  getEnvString("STORAGE_PUBLIC_URL", "http://localhost:3000/uploads"),
			S3Bucket:   getEnvString("STORAGE_S3_BUCKET", ""),
			S3Region:   getEnvString("STORAGE_S3_REGION", "us-east-1"),
			S3Endpoint: getEnvString("STORAGE_S3_ENDPOINT", ""),
			// Sibling of ./uploads rather than a subdirectory of it, so a static
			// file server mapping the uploads tree never exposes archives.
			ArchivePath: getEnvString("STORAGE_ARCHIVE_PATH", "./archives"),
		},
		Discord: DiscordConfig{
			BotToken:          getEnvString("DISCORD_BOT_TOKEN", ""),
			OAuthClientID:     getEnvString("DISCORD_CLIENT_ID", ""),
			OAuthClientSecret: getEnvString("DISCORD_CLIENT_SECRET", ""),
			OAuthRedirectURL:  getEnvString("DISCORD_REDIRECT_URL", "http://localhost:3000/api/v1/auth/discord/callback"),
		},
		Telemetry: TelemetryConfig{
			OTELEnabled:   getEnvBool("OTEL_ENABLED", false),
			OTELEndpoint:  getEnvString("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
			PrometheusURL: getEnvString("GRAFANA_PROMETHEUS_URL", ""),
		},
	}

	// Validate required configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

// Validate checks that required configuration values are present and valid.
// It returns a descriptive error if any critical configuration is missing or invalid.
func (c *Config) Validate() error {
	if c.Database.URL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}

	if c.JWT.Secret == "" {
		return fmt.Errorf("JWT_SECRET is required")
	}

	// Warn about weak JWT secrets in production
	if c.App.Environment == "production" && len(c.JWT.Secret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters in production")
	}

	// Validate environment values
	validEnvironments := []string{"development", "staging", "production"}
	if !contains(validEnvironments, c.App.Environment) {
		return fmt.Errorf("ENVIRONMENT must be one of: %v", validEnvironments)
	}

	validLogLevels := []string{"debug", "info", "warn", "error"}
	if !contains(validLogLevels, c.App.LogLevel) {
		return fmt.Errorf("LOG_LEVEL must be one of: %v", validLogLevels)
	}

	// Validate storage configuration
	validStorageBackends := []string{"local", "s3"}
	if !contains(validStorageBackends, c.Storage.Backend) {
		return fmt.Errorf("STORAGE_BACKEND must be one of: %v", validStorageBackends)
	}

	// S3-specific validation
	if c.Storage.Backend == "s3" {
		if c.Storage.S3Bucket == "" {
			return fmt.Errorf("STORAGE_S3_BUCKET is required when using S3 storage")
		}
		if c.Storage.S3Region == "" {
			return fmt.Errorf("STORAGE_S3_REGION is required when using S3 storage")
		}
	}

	// Validate comment max depth
	if c.App.CommentMaxDepth < 1 || c.App.CommentMaxDepth > 10 {
		return fmt.Errorf("COMMENT_MAX_DEPTH must be between 1 and 10")
	}

	return nil
}

// GetServerAddress returns the full server bind address (host:port).
func (c *Config) GetServerAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// IsDevelopment returns true if running in development mode.
func (c *Config) IsDevelopment() bool {
	return c.App.Environment == "development"
}

// IsProduction returns true if running in production mode.
func (c *Config) IsProduction() bool {
	return c.App.Environment == "production"
}

// Helper functions for environment variable parsing

func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		switch value {
		case "true", "1", "yes", "on":
			return true
		case "false", "0", "no", "off":
			return false
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvStringSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		// Simple comma-separated parsing - could be enhanced
		result := []string{}
		for _, v := range []string{value} {
			if v != "" {
				result = append(result, v)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return defaultValue
}

func contains(slice []string, item string) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}

// getPoolDefaults returns environment-specific database connection pool defaults.
// These values balance resource usage with performance for each environment.
//
// Returns: (maxConnections, minConnections)
func getPoolDefaults(environment string) (int, int) {
	switch environment {
	case "development":
		// Development: Minimal connections for single developer
		return 5, 1
	case "staging":
		// Staging: Light testing load with single server
		return 10, 3
	case "production":
		// Production: Assumes 2 app servers, PostgreSQL max_connections=100
		// Formula: (100 * 0.8) / 2 = 40 connections per server
		// Conservative default of 35 to leave headroom
		// Adjust DATABASE_MAX_CONNECTIONS based on your deployment
		return 35, 9
	default:
		// Unknown environment - use development defaults
		return 5, 1
	}
}
