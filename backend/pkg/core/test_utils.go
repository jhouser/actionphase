package core

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/jwtauth/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	models "actionphase/pkg/db/models"
	"actionphase/pkg/observability"
)

// Per-package (per test binary) isolated database provisioning.
//
// Test packages run as separate processes and `go test` runs them concurrently.
// They all point at the same PostgreSQL server, so sharing one database causes
// cross-package interference: pkey collisions (cleanup resets ID sequences),
// deadlocks (overlapping multi-table DELETEs), and FK violations (one package's
// cleanup deleting another's rows). To make packages independent, the first
// NewTestDatabase call in a process clones a private database from a migrated
// template and every subsequent call in that process reuses it.
//
// The clone is exactly one per process (sync.Once). Leftover clones are named
// actionphase_test_p<pid> and swept by `just _prepare-test-template` before a run.
var (
	isolatedDBOnce sync.Once
	isolatedDBURL  string
	isolatedDBErr  error
)

// provisionIsolatedDB creates a per-process database cloned from the migrated
// template and returns a connection URL pointing at it. baseURL supplies the
// server/credentials; the database name is swapped for the private clone.
func provisionIsolatedDB(baseURL string) (string, error) {
	isolatedDBOnce.Do(func() {
		u, err := url.Parse(baseURL)
		if err != nil {
			isolatedDBErr = fmt.Errorf("invalid test database URL %q: %w", baseURL, err)
			return
		}

		// Template to clone from: the migrated template DB when the harness set it,
		// otherwise the base test DB name (so a bare `go test` is still isolated).
		template := os.Getenv("TEST_TEMPLATE_DB")
		if template == "" {
			template = strings.TrimPrefix(u.Path, "/")
		}

		cloneName := fmt.Sprintf("actionphase_test_p%d", os.Getpid())

		// Connect to the maintenance `postgres` database to run CREATE/DROP DATABASE.
		adminURL := *u
		adminURL.Path = "/postgres"
		ctx := context.Background()
		conn, err := pgxpool.New(ctx, adminURL.String())
		if err != nil {
			isolatedDBErr = fmt.Errorf("connect to admin db for test isolation: %w", err)
			return
		}
		defer conn.Close()

		// Drop a stale clone from a previous run with the same pid, then clone fresh.
		// WITH (FORCE) terminates any lingering connections to the old clone.
		if _, err := conn.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", cloneName)); err != nil {
			isolatedDBErr = fmt.Errorf("drop stale test clone %s: %w", cloneName, err)
			return
		}
		if _, err := conn.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s TEMPLATE %s", cloneName, template)); err != nil {
			isolatedDBErr = fmt.Errorf("clone test db %s from template %s: %w", cloneName, template, err)
			return
		}

		cloneURL := *u
		cloneURL.Path = "/" + cloneName
		isolatedDBURL = cloneURL.String()
	})

	return isolatedDBURL, isolatedDBErr
}

// TestDatabase provides utilities for database testing
type TestDatabase struct {
	Pool *pgxpool.Pool
}

// TestingInterface defines the interface that both *testing.T and *testing.B satisfy
type TestingInterface interface {
	Fatalf(format string, args ...interface{})
	Logf(format string, args ...interface{})
	Helper()
}

// NewTestDatabase creates a new test database connection
// Note: This requires a running PostgreSQL instance for integration tests
// Set TEST_DATABASE_URL environment variable to override default connection string
// Set SKIP_DB_TESTS=true to skip tests that require a database
func NewTestDatabase(t TestingInterface) *TestDatabase {
	// Check if database tests should be skipped
	if os.Getenv("SKIP_DB_TESTS") == "true" {
		t.Logf("Skipping database test - SKIP_DB_TESTS=true")
		if tb, ok := t.(*testing.T); ok {
			tb.Skip("Database tests skipped - set SKIP_DB_TESTS=false to enable")
		}
		return nil // This won't be reached due to Skip(), but keeps the compiler happy
	}

	connectionString := os.Getenv("TEST_DATABASE_URL")
	if connectionString == "" {
		connectionString = "postgres://postgres:example@localhost:5432/actionphase_test?sslmode=disable"
	}

	// Validate connection string format
	if _, err := url.Parse(connectionString); err != nil {
		t.Fatalf("Invalid test database URL '%s': %v", connectionString, err)
	}

	// Provision (or reuse) a database private to this test binary so packages
	// running concurrently never share rows, sequences, or locks. Falls back to
	// the shared DB if provisioning fails (e.g. server unreachable) so the usual
	// skip-on-unavailable path below still applies.
	if isolatedURL, err := provisionIsolatedDB(connectionString); err != nil {
		t.Logf("Warning: could not provision isolated test database, using shared DB: %v", err)
	} else if isolatedURL != "" {
		connectionString = isolatedURL
	}

	pool, err := pgxpool.New(context.Background(), connectionString)
	if err != nil {
		// Instead of failing immediately, try to provide helpful guidance
		t.Logf("Failed to connect to test database: %v", err)
		t.Logf("To skip database tests, set SKIP_DB_TESTS=true")
		t.Logf("To run with database tests, ensure PostgreSQL is running and database 'actionphase' exists")
		t.Logf("  or set TEST_DATABASE_URL to point to your test database")

		if tb, ok := t.(*testing.T); ok {
			tb.Skip("Database unavailable - skipping test")
		}
		return nil
	}

	// Test the connection
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Logf("Database connection test failed: %v", err)
		if tb, ok := t.(*testing.T); ok {
			tb.Skip("Database connection failed - skipping test")
		}
		return nil
	}

	return &TestDatabase{Pool: pool}
}

// Close closes the database connection
// NOTE: Does NOT automatically clean up tables to allow parallel test execution
// If you need to clean tables between test runs, call CleanupAllTables() explicitly
// at the START of your test suite (e.g., in TestMain), not at the end
func (td *TestDatabase) Close() {
	if td.Pool != nil {
		td.Pool.Close()
	}
}

// CleanupAllTables removes all test data from all tables
// Call this at the START of your test suite (in TestMain) if needed,
// not at the end of individual tests (to allow parallel execution)
func (td *TestDatabase) CleanupAllTables() {
	simpleT := &simpleTestInterface{}
	td.CleanupTables(simpleT)
}

// simpleTestInterface implements TestingInterface for cleanup logging
type simpleTestInterface struct{}

func (s *simpleTestInterface) Helper() {
	// No-op for cleanup interface
}

func (s *simpleTestInterface) Logf(format string, args ...interface{}) {
	// Silent cleanup - only log errors to stderr
	// Most cleanup warnings are not critical
}

func (s *simpleTestInterface) Fatalf(format string, args ...interface{}) {
	// Convert fatal to log for cleanup errors
	// We don't want cleanup failures to crash the test harness
}

// CleanupTables removes test data from tables (useful for test cleanup)
// Tables should be provided in dependency order (child tables first)
// Uses CASCADE to handle foreign key dependencies automatically
func (td *TestDatabase) CleanupTables(t TestingInterface, tables ...string) {
	ctx := context.Background()

	// If no tables specified, clean up all test tables in proper dependency order
	// (child tables first to avoid foreign key constraint violations)
	if len(tables) == 0 {
		tables = []string{
			// Leaf tables (most dependent)
			"user_preferences",
			"user_common_room_reads",
			"phase_transitions",
			"message_reactions",
			"message_recipients",
			"conversation_reads",
			"handout_comments",
			"thread_posts",
			// Mid-level dependent tables
			"private_messages",
			"messages",
			"handouts",
			"threads",
			"notifications",
			"action_results",
			"action_submissions",
			"npc_assignments",
			"conversation_participants",
			"conversations",
			// Game-level tables
			"game_phases",
			"game_participants",
			"game_applications",
			"character_data",
			"characters",
			// Top-level tables (least dependent)
			"games",
			"registration_attempts",
			"sessions",
			"users",
		}
	}

	// First check which tables exist to avoid transaction aborts
	existingTables := []string{}
	for _, table := range tables {
		var exists bool
		err := td.Pool.QueryRow(ctx,
			"SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = $1)",
			table).Scan(&exists)
		if err != nil {
			t.Logf("Warning: Failed to check if table %s exists: %v", table, err)
			continue
		}
		if exists {
			existingTables = append(existingTables, table)
		} else {
			// A name that matches no table is a typo, not a no-op. Skipping it
			// silently leaves that table's rows and id sequence untouched while
			// the rest of the list is reset, which later surfaces as a
			// duplicate-key failure in an unrelated test. Fail at the typo.
			t.Fatalf("CleanupTables: no table named %q exists; "+
				"check the name against the schema (e.g. phases -> game_phases)", table)
		}
	}

	// If no tables exist, nothing to clean up
	if len(existingTables) == 0 {
		return
	}

	// Use a single transaction for atomic cleanup of existing tables
	tx, err := td.Pool.Begin(ctx)
	if err != nil {
		t.Logf("Warning: Failed to begin cleanup transaction: %v", err)
		return
	}

	committed := false
	defer func() {
		if !committed {
			if err := tx.Rollback(ctx); err != nil {
				t.Logf("Warning: Failed to rollback cleanup transaction: %v", err)
			}
		}
	}()

	for _, table := range existingTables {
		// Use DELETE instead of TRUNCATE to avoid deadlock issues with foreign keys
		_, err := tx.Exec(ctx, "DELETE FROM "+table)
		if err != nil {
			t.Logf("Warning: Failed to cleanup table %s: %v", table, err)
			return
		}
		// Reset sequences for primary key columns
		_, err = tx.Exec(ctx, "SELECT setval(pg_get_serial_sequence('"+table+"', 'id'), 1, false)")
		if err != nil {
			// Not all tables have id columns, so just log this as info
			t.Logf("Note: No sequence to reset for table %s", table)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		t.Logf("Warning: Failed to commit cleanup transaction: %v", err)
	} else {
		committed = true
	}
}

// generateUniqueUsername creates a unique username for testing
func generateUniqueUsername(base string) string {
	return fmt.Sprintf("%s_%d_%d", base, time.Now().UnixNano(), rand.Intn(10000))
}

// generateUniqueEmail creates a unique email for testing
func generateUniqueEmail(base string) string {
	return fmt.Sprintf("%s_%d_%d@example.com", base, time.Now().UnixNano(), rand.Intn(10000))
}

// CreateTestUser creates a test user for use in tests
func (td *TestDatabase) CreateTestUser(t TestingInterface, username, email string) *User {
	ctx := context.Background()
	queries := models.New(td.Pool)

	// Make username and email unique to avoid conflicts
	uniqueUsername := generateUniqueUsername(username)
	uniqueEmail := generateUniqueEmail(email)

	user := &User{
		Username: uniqueUsername,
		Email:    uniqueEmail,
		Password: "test_password",
	}
	user.HashPassword() // Hash the password before storing

	dbUser, err := queries.CreateUser(ctx, models.CreateUserParams{
		Username: user.Username,
		Password: user.Password,
		Email:    user.Email,
	})
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	return &User{
		ID:        int(dbUser.ID),
		Username:  dbUser.Username,
		Email:     dbUser.Email,
		CreatedAt: &dbUser.CreatedAt.Time,
	}
}

// CreateTestGame creates a test game for use in tests
func (td *TestDatabase) CreateTestGame(t TestingInterface, gmUserID int32, title string) *models.Game {
	ctx := context.Background()
	queries := models.New(td.Pool)

	game, err := queries.CreateGame(ctx, models.CreateGameParams{
		Title:       title,
		Description: pgtype.Text{String: "Test game created for testing purposes", Valid: true},
		GmUserID:    gmUserID,
		Genre:       pgtype.Text{String: "Test", Valid: true},
		MaxPlayers:  pgtype.Int4{Int32: 6, Valid: true},
		IsPublic:    pgtype.Bool{Bool: true, Valid: true},
	})
	if err != nil {
		t.Fatalf("Failed to create test game: %v", err)
	}

	return &game
}

// CreateTestGameWithState creates a test game with a specific state
func (td *TestDatabase) CreateTestGameWithState(t TestingInterface, gmUserID int32, title string, state string) *models.Game {
	ctx := context.Background()
	queries := models.New(td.Pool)

	// First create the game (state defaults to 'setup')
	game, err := queries.CreateGame(ctx, models.CreateGameParams{
		Title:       title,
		Description: pgtype.Text{String: "Test game created for testing purposes", Valid: true},
		GmUserID:    gmUserID,
		Genre:       pgtype.Text{String: "Test", Valid: true},
		MaxPlayers:  pgtype.Int4{Int32: 6, Valid: true},
		IsPublic:    pgtype.Bool{Bool: true, Valid: true},
	})
	if err != nil {
		t.Fatalf("Failed to create test game: %v", err)
	}

	// Then update the state if different from default
	if state != "setup" {
		updatedGame, err := queries.UpdateGameState(ctx, models.UpdateGameStateParams{
			ID:    game.ID,
			State: pgtype.Text{String: state, Valid: true},
		})
		if err != nil {
			t.Fatalf("Failed to update game state to %s: %v", state, err)
		}
		return &updatedGame
	}

	return &game
}

// SetGameStateDirectly updates a game's state by writing directly to the database,
// bypassing service-layer transition validation. Use this only in test setup when
// you need a game in a specific state and the path to get there is not the subject
// of the test.
func (td *TestDatabase) SetGameStateDirectly(t TestingInterface, gameID int32, state string) *models.Game {
	ctx := context.Background()
	queries := models.New(td.Pool)

	updatedGame, err := queries.UpdateGameState(ctx, models.UpdateGameStateParams{
		ID:    gameID,
		State: pgtype.Text{String: state, Valid: true},
	})
	if err != nil {
		t.Fatalf("SetGameStateDirectly: failed to set game %d to state %s: %v", gameID, state, err)
	}
	return &updatedGame
}

// CreateTestPhase creates a test phase for a game
func (td *TestDatabase) CreateTestPhase(t TestingInterface, gameID int32, phaseType string, title string) *models.GamePhase {
	ctx := context.Background()
	queries := models.New(td.Pool)

	phase, err := queries.CreateGamePhase(ctx, models.CreateGamePhaseParams{
		GameID:      gameID,
		PhaseType:   phaseType,
		PhaseNumber: 1,
		Title:       title,
		Description: pgtype.Text{String: "Test phase", Valid: true},
		StartTime:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	if err != nil {
		t.Fatalf("Failed to create test phase: %v", err)
	}

	return &phase
}

// CreateTestPost creates a test post for a game
func (td *TestDatabase) CreateTestPost(t TestingInterface, gameID int32, authorID int32, characterID int32, content string) *models.Message {
	ctx := context.Background()
	queries := models.New(td.Pool)

	post, err := queries.CreatePost(ctx, models.CreatePostParams{
		GameID:                gameID,
		AuthorID:              authorID,
		CharacterID:           characterID,
		Content:               content,
		Visibility:            models.MessageVisibilityGame,
		MentionedCharacterIds: []int32{}, // Empty array for mentioned characters
	})
	if err != nil {
		t.Fatalf("Failed to create test post: %v", err)
	}

	return &post
}

// AddTestGameParticipant directly inserts a game participant without validation
// Use this for test setup when you need to add participants to archived games
func (td *TestDatabase) AddTestGameParticipant(t TestingInterface, gameID int32, userID int32, role string) *models.GameParticipant {
	ctx := context.Background()
	queries := models.New(td.Pool)

	participant, err := queries.AddGameParticipant(ctx, models.AddGameParticipantParams{
		GameID: gameID,
		UserID: userID,
		Role:   role,
	})
	if err != nil {
		t.Fatalf("Failed to add test game participant: %v", err)
	}

	return &participant
}

// TestFixtures provides common test data
type TestFixtures struct {
	TestUser *User
	TestGame *models.Game
}

// SetupFixtures creates common test data for use across tests
func (td *TestDatabase) SetupFixtures(t TestingInterface) *TestFixtures {
	// Create test user
	testUser := td.CreateTestUser(t, "testuser", "test@example.com")

	// Create test game
	testGame := td.CreateTestGame(t, int32(testUser.ID), "Test Game")

	return &TestFixtures{
		TestUser: testUser,
		TestGame: testGame,
	}
}

// AssertNoError is a test helper that fails the test if err is not nil
func AssertNoError(t *testing.T, err error, message string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", message, err)
	}
}

// AssertError is a test helper that fails the test if err is nil
func AssertError(t *testing.T, err error, message string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: expected error but got nil", message)
	}
}

// AssertEqual is a generic test helper for equality assertions
func AssertEqual[T comparable](t *testing.T, expected, actual T, message string) {
	t.Helper()
	if expected != actual {
		t.Errorf("%s: expected %v, got %v", message, expected, actual)
	}
}

// AssertNotEqual is a generic test helper for inequality assertions
func AssertNotEqual[T comparable](t *testing.T, notExpected, actual T, message string) {
	t.Helper()
	if notExpected == actual {
		t.Errorf("%s: expected not %v, but got %v", message, notExpected, actual)
	}
}

// TimePtr is a helper function to get a pointer to a time.Time
func TimePtr(t time.Time) *time.Time {
	return &t
}

// StringPtr is a helper function to get a pointer to a string
func StringPtr(s string) *string {
	return &s
}

// Int32Ptr is a helper function to get a pointer to an int32
func Int32Ptr(i int32) *int32 {
	return &i
}

// BoolPtr is a helper function to get a pointer to a bool
func BoolPtr(b bool) *bool {
	return &b
}

// NewTestLogger creates a no-op logger for testing purposes
func NewTestLogger() slog.Logger {
	return *slog.New(slog.NewTextHandler(&discardWriter{}, nil))
}

// discardWriter is a writer that discards all input (for test logging)
type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

// AssertTrue is a test helper that fails if the condition is false
func AssertTrue(t *testing.T, condition bool, message string) {
	t.Helper()
	if !condition {
		t.Errorf("%s: expected true, got false", message)
	}
}

// AssertErrorContains checks that an error contains a specific substring
func AssertErrorContains(t *testing.T, err error, substring string, message string) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: expected error containing '%s', got nil", message, substring)
		return
	}
	if !strings.Contains(err.Error(), substring) {
		t.Errorf("%s: expected error to contain '%s', got '%s'", message, substring, err.Error())
	}
}

// AssertHttpStatus checks HTTP status code with detailed error message
func AssertHttpStatus(t *testing.T, expected, actual int, testName, responseBody string) {
	t.Helper()
	if expected != actual {
		t.Errorf("Test '%s' failed: expected status %d, got %d. Response: %s",
			testName, expected, actual, responseBody)
	}
}

// TestConfig holds configuration for test execution
type TestConfig struct {
	DatabaseURL    string
	EnableParallel bool
	CleanupTables  bool
	LogLevel       string
	JWTSecret      string
}

// LoadTestConfig loads test configuration from environment variables
func LoadTestConfig() *TestConfig {
	return &TestConfig{
		DatabaseURL:    getEnvOrDefault("TEST_DATABASE_URL", "postgres://postgres:example@localhost:5432/actionphase?sslmode=disable"),
		EnableParallel: getEnvBoolOrDefault("TEST_PARALLEL", false),
		CleanupTables:  getEnvBoolOrDefault("TEST_CLEANUP", true),
		LogLevel:       getEnvOrDefault("TEST_LOG_LEVEL", "warn"),
		JWTSecret:      getEnvOrDefault("TEST_JWT_SECRET", "test_jwt_secret_for_unit_tests_only"),
	}
}

// getEnvOrDefault returns environment variable value or default if not set
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvBoolOrDefault returns environment variable as bool or default if not set
func getEnvBoolOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		switch value {
		case "true", "1", "yes":
			return true
		case "false", "0", "no":
			return false
		}
	}
	return defaultValue
}

// CreateTestJWTToken creates a JWT token for testing purposes using configurable secret
// Deprecated: Use CreateTestJWTTokenForUser instead
func CreateTestJWTToken(username string) (string, error) {
	config := LoadTestConfig()
	tokenAuth := jwtauth.New("HS256", []byte(config.JWTSecret), nil)

	claims := map[string]interface{}{
		"username": username,
		"exp":      time.Now().Add(time.Hour).Unix(),
	}

	_, tokenString, err := tokenAuth.Encode(claims)
	return tokenString, err
}

// CreateTestJWTTokenForUser creates a JWT token for a given user using the app config
// This matches the pattern used in production and ensures compatibility with JWT middleware
func CreateTestJWTTokenForUser(app *App, user *User) (string, error) {
	tokenAuth := jwtauth.New("HS256", []byte(app.Config.JWT.Secret), nil)
	_, tokenString, err := tokenAuth.Encode(map[string]interface{}{
		"sub":      fmt.Sprintf("%d", user.ID), // User ID in "sub" claim (required by RequireAuthenticationMiddleware)
		"username": user.Username,
		"exp":      time.Now().Add(time.Hour * 24).Unix(), // 24 hour expiry
	})
	return tokenString, err
}

// CreateTestTokenAuth creates a JWT auth instance for testing with configurable secret
func CreateTestTokenAuth() *jwtauth.JWTAuth {
	config := LoadTestConfig()
	return jwtauth.New("HS256", []byte(config.JWTSecret), nil)
}

// CreateTestUserWithCredentials creates a test user and returns both the user and plain password
// This is useful for tests that need to know the plain password for login
func (td *TestDatabase) CreateTestUserWithCredentials(t TestingInterface, username, email, plainPassword string) (*User, string) {
	ctx := context.Background()
	queries := models.New(td.Pool)

	// Make username and email unique to avoid conflicts
	uniqueUsername := generateUniqueUsername(username)
	uniqueEmail := generateUniqueEmail(email)

	user := &User{
		Username: uniqueUsername,
		Email:    uniqueEmail,
		Password: plainPassword,
	}
	user.HashPassword() // Hash the password before storing

	dbUser, err := queries.CreateUser(ctx, models.CreateUserParams{
		Username: user.Username,
		Password: user.Password,
		Email:    user.Email,
	})
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	return &User{
		ID:        int(dbUser.ID),
		Username:  dbUser.Username,
		Email:     dbUser.Email,
		CreatedAt: &dbUser.CreatedAt.Time,
	}, plainPassword
}

// NewTestConfig creates a Config instance suitable for testing
// This provides all required configuration with test-appropriate defaults
func NewTestConfig() *Config {
	return &Config{
		Database: DatabaseConfig{
			URL:            "postgres://postgres:example@localhost:5432/actionphase?sslmode=disable",
			TestURL:        "postgres://postgres:example@localhost:5432/actionphase?sslmode=disable",
			MaxConnections: 10,
			MaxIdleTime:    30 * time.Minute,
		},
		JWT: JWTConfig{
			Secret:    "test_jwt_secret_for_unit_tests_only",
			Algorithm: "HS256",
		},
		Server: ServerConfig{
			Port:         3000,
			Host:         "0.0.0.0",
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		App: AppConfig{
			Environment:   "development",
			LogLevel:      "info",
			RunMigrations: false,
			CORSEnabled:   true,
			CORSOrigins:   []string{"http://localhost:5173"},
		},
	}
}

// NewTestApp creates a fully initialized App instance for testing
// This includes all required fields: Pool, Logger, Config, ObsLogger, and Observability
func NewTestApp(pool *pgxpool.Pool) *App {
	logger := NewTestLogger()
	config := NewTestConfig()

	// Create observability logger for context-aware logging
	obsLogger := observability.NewLogger("test", "error")

	// Create observability instance with logger and metrics
	obs := observability.New("test", "error")

	return &App{
		Pool:          pool,
		Logger:        logger,
		Config:        config,
		ObsLogger:     obsLogger,
		Observability: obs,
	}
}

// WithAuthenticatedUser adds an authenticated user to the request context for testing
// This mimics what the RequireAuthenticationMiddleware does in production
func WithAuthenticatedUser(ctx context.Context, user *User) context.Context {
	authUser := &AuthenticatedUser{
		ID:       int32(user.ID),
		Username: user.Username,
		Email:    user.Email,
		IsAdmin:  false, // Default to non-admin; set explicitly if needed
	}
	return context.WithValue(ctx, UserContextKey, authUser)
}

// ========================================
// Phase 4: Cleanup Presets
// ========================================

// Common table cleanup combinations used across tests
var CleanupPresets = map[string][]string{
	// Basic game cleanup
	"games": {"games", "sessions", "users"},

	// Character-related cleanup (includes all character dependencies)
	"characters": {"character_data", "npc_assignments", "characters", "games", "sessions", "users"},

	// Phase-related cleanup
	"phases": {"game_phases", "games", "sessions", "users"},

	// Message-related cleanup (posts and comments)
	"messages": {"messages", "characters", "games", "sessions", "users"},

	// Action submission cleanup
	"actions": {"action_results", "action_submissions", "game_phases", "characters", "games", "sessions", "users"},

	// Game participants and applications
	"participants": {"game_participants", "game_applications", "games", "sessions", "users"},

	// Conversation cleanup (private messages)
	"conversations": {"conversation_reads", "private_messages", "conversation_participants", "conversations", "characters", "games", "sessions", "users"},

	// Deadline cleanup
	"deadlines": {"game_deadlines", "game_participants", "games", "sessions", "users"},

	// Poll cleanup
	"polls": {"poll_votes", "poll_options", "common_room_polls", "games", "sessions", "users"},

	// Complete cleanup (all tables)
	"all": {
		"conversation_reads",
		"private_messages",
		"conversation_participants",
		"conversations",
		"message_reactions",
		"messages",
		"action_results",
		"action_submissions",
		"character_data",
		"npc_assignments",
		"characters",
		"game_phases",
		"game_deadlines",
		"game_participants",
		"game_applications",
		"games",
		"sessions",
		"users",
	},
}
