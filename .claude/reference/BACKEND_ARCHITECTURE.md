# ActionPhase Backend Architecture

This document provides a comprehensive overview of the ActionPhase Go backend architecture, designed to be highly AI-friendly for development and maintenance.

**Last Verified**: August 2026

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Project Structure](#project-structure)
3. [Design Patterns](#design-patterns)
4. [Data Layer](#data-layer)
5. [Service Layer](#service-layer)
6. [Transport Layer](#transport-layer)
7. [Authentication & Authorization](#authentication--authorization)
8. [Error Handling](#error-handling)
9. [Configuration Management](#configuration-management)
10. [Testing Strategy](#testing-strategy)
11. [AI-Friendly Features](#ai-friendly-features)
12. [Development Guidelines](#development-guidelines)

## Architecture Overview

ActionPhase follows **Clean Architecture** principles with clear separation of concerns across layers:

```
┌─────────────────────────────────────────────────────────┐
│                   Transport Layer                        │
│                    (pkg/http, pkg/games, pkg/auth)     │
├─────────────────────────────────────────────────────────┤
│                  Application Layer                       │
│            (Business Logic & Use Cases)                 │
├─────────────────────────────────────────────────────────┤
│                    Domain Layer                         │
│                    (pkg/core)                          │
├─────────────────────────────────────────────────────────┤
│                Infrastructure Layer                      │
│                (pkg/db, External Services)             │
└─────────────────────────────────────────────────────────┘
```

### Key Principles

- **Dependency Inversion**: Higher layers depend on interfaces, not concrete implementations
- **Single Responsibility**: Each component has one clear purpose
- **Interface Segregation**: Small, focused interfaces
- **Testability**: Every component can be unit tested with mocks
- **AI-Friendly**: Clear naming, comprehensive documentation, consistent patterns

## Project Structure

```
backend/
├── main.go                          # Application entry point
├── pkg/
│   ├── core/                       # Domain layer — entities, interfaces, middleware
│   │   ├── interfaces.go           # Service interfaces & contracts
│   │   ├── constants.go            # Application constants & enums
│   │   ├── permissions.go          # Permission helpers (IsUserGameMaster, ...)
│   │   ├── middleware.go           # Auth middleware + AuthenticatedUser
│   │   ├── config.go               # Configuration management
│   │   ├── api_errors.go           # Structured error handling
│   │   ├── games.go, characters.go, phases.go, notifications.go, ...
│   │   │                           # Domain models, one file per bounded context
│   │   │                           # (there is no single models.go)
│   │   ├── repositories.go         # Repository interfaces
│   │   └── test_utils.go, test_factories.go, repository_mocks.go
│   ├── db/                         # Data access layer
│   │   ├── migrations/             # Versioned schema migrations
│   │   ├── models/                 # Generated sqlc models
│   │   ├── queries/                # SQL query definitions
│   │   ├── services/               # Service implementations
│   │   │   ├── phases/             # Decomposed: service, crud, transitions,
│   │   │   │                       #   validation, history, scheduler
│   │   │   ├── actions/            # Decomposed: service, submissions, results,
│   │   │   │                       #   validation, queries, draft_updates,
│   │   │   │                       #   staged, staged_worker
│   │   │   ├── messages/           # Decomposed: service, posts, draft_posts,
│   │   │   │                       #   comments, reactions, validation,
│   │   │   │                       #   read_tracking, audience, character_messages
│   │   │   └── *.go                # games, characters, users, sessions,
│   │   │                           #   notifications, conversations, handouts,
│   │   │                           #   dashboard, deadlines, polls, ...
│   │   ├── test_fixtures/          # Seed SQL + apply scripts
│   │   ├── schema.sql              # Full generated schema
│   │   └── sqlc.yaml               # sqlc configuration
│   ├── auth/                       # Authentication
│   │   ├── api.go, jwt.go, login.go, registration.go, refresh_token.go
│   │   ├── password*.go, account_*.go, session_handlers.go
│   │   ├── discord_handlers.go     # Discord OAuth
│   │   └── bot_prevention_service.go
│   ├── http/                       # HTTP routing & middleware
│   │   └── root.go                 # Router setup — the routing source of truth
│   │
│   │   # Handler packages (each exposes api.go):
│   ├── admin/  auth/  avatars/  characters/  conversations/  dashboard/
│   ├── deadlines/  exports/  games/  handouts/  notifications/  phases/
│   ├── polls/  users/
│   │
│   │   # Supporting packages:
│   ├── observability/              # Logging, tracing, metrics middleware
│   ├── middleware/                 # Shared HTTP middleware
│   ├── validation/                 # Request validation helpers
│   ├── scheduler/                  # Background scheduled jobs
│   ├── cleanup/                    # Retention / sweep workers
│   ├── email/                      # Transactional email
│   ├── discord/                    # Discord integration
│   ├── storage/                    # File/blob storage
│   ├── messages/                   # Message domain (service-backed, no api.go)
│   └── docs/                       # Embedded VitePress docs handler
└── go.mod
```

## Design Patterns

### 1. Repository Pattern

**Purpose**: Abstracts data access logic and enables testability.

```go
// Interface in domain layer (pkg/core/interfaces.go)
type UserServiceInterface interface {
    CreateUser(user *User) (*User, error)
    UserByUsername(username string) (*User, error)
    // ... other methods
}

// Implementation in infrastructure layer (pkg/db/services/users.go)
type UserService struct {
    DB *pgxpool.Pool
}

func (us *UserService) CreateUser(user *User) (*User, error) {
    // Database implementation
}

// Mock in domain layer for testing (pkg/core/repository_mocks.go)
type MockUserRepository struct {
    CreateUserFn func(*User) (*User, error)
    // ... other function fields
}
```

### 2. Dependency Injection

**Purpose**: Enables loose coupling and testability.

```go
// Handler depends on interface, not concrete implementation
type Handler struct {
    App         *core.App
    UserService core.UserServiceInterface
    GameService core.GameServiceInterface
}

// Easy to inject mocks for testing
func TestHandler() {
    mockUserService := &core.MockUserRepository{...}
    handler := &Handler{UserService: mockUserService}
    // Test with mocks
}
```

### 3. Builder Pattern (Test Data)

**Purpose**: Flexible test data creation with fluent interface.

```go
// Create test data with custom properties
user := factory.NewUser().
    WithUsername("testuser").
    WithEmail("test@example.com").
    WithAdmin(true).
    Create()

game := factory.NewGame().
    WithTitle("Epic Campaign").
    WithGM(user).
    WithMaxPlayers(6).
    Create()
```

### 4. Middleware Chain

**Purpose**: Reusable request processing components.

```go
r.Group(func(r chi.Router) {
    r.Use(jwtauth.Verifier(tokenAuth))
    r.Use(core.RequireAuthenticationMiddleware(userService))
    r.Use(core.LoggingMiddleware(logger))

    // Protected routes
    r.Post("/games", handler.CreateGame)
    r.Put("/games/{id}/state", handler.UpdateGameState)
})
```

## Data Layer

### SQLC Integration

ActionPhase uses [SQLC](https://sqlc.dev/) for type-safe database access:

```sql
-- queries/games.sql
-- name: CreateGame :one
INSERT INTO games (title, description, gm_user_id, genre, max_players)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetGame :one
SELECT * FROM games WHERE id = $1;
```

```go
// Generated by SQLC in pkg/db/models/
type CreateGameParams struct {
    Title       string
    Description string
    GmUserID    int32
    Genre       pgtype.Text
    MaxPlayers  pgtype.Int4
}

func (q *Queries) CreateGame(ctx context.Context, arg CreateGameParams) (Game, error) {
    // Generated implementation
}
```

### Migration Management

Database schema changes are managed through versioned migrations:

All migration operations go through the single `migration` recipe (runs in the
backend container):

```bash
just migration create add_user_preferences   # create up/down pair
just migration status                        # show current version
just migration rollback                      # roll back the last migration
just migration test                          # apply migrations to the test database

just migrate                                 # apply to the dev database
just reset-test-db                           # rebuild test DB + migrated template
```

> Note: `just make_migration` and `just migrate_status` do **not** exist.

### Repository Implementation

```go
type GameService struct {
    DB *pgxpool.Pool
}

func (gs *GameService) CreateGame(ctx context.Context, req core.CreateGameRequest) (*models.Game, error) {
    queries := models.New(gs.DB)

    game, err := queries.CreateGame(ctx, models.CreateGameParams{
        Title:       req.Title,
        Description: req.Description,
        GmUserID:    req.GMUserID,
        // ... other fields
    })

    return &game, err
}
```

## Service Layer

### Interface Definition

Service interfaces are defined in the domain layer (`pkg/core/interfaces.go`) with comprehensive documentation:

```go
// GameServiceInterface defines the contract for game management operations.
// Handles complete game lifecycle from creation through completion, including
// participant management and state transitions.
//
// Usage Example:
//
//	gameService := &services.GameService{DB: pool}
//	game, err := gameService.CreateGame(ctx, CreateGameRequest{...})
type GameServiceInterface interface {
    // CreateGame creates a new game with the given parameters
    CreateGame(ctx context.Context, req CreateGameRequest) (*models.Game, error)

    // GetGame retrieves a game by its ID
    GetGame(ctx context.Context, gameID int32) (*models.Game, error)

    // ... other methods with full documentation
}
```

### Business Logic Implementation

```go
func (gs *GameService) JoinGame(ctx context.Context, gameID, userID int32, role string) error {
    // 1. Validate input parameters
    if !core.IsValidParticipantRole(role) {
        return fmt.Errorf("invalid role: %s", role)
    }

    // 2. Check business rules
    joinStatus, err := gs.CanUserJoinGame(ctx, gameID, userID)
    if err != nil {
        return fmt.Errorf("failed to check join eligibility: %w", err)
    }

    if joinStatus != core.CanJoin {
        return fmt.Errorf("cannot join game: %s", joinStatus)
    }

    // 3. Perform the operation
    _, err = gs.AddGameParticipant(ctx, gameID, userID, role)
    return err
}
```

## Transport Layer

### HTTP Handler Structure

Handlers follow a consistent pattern for processing requests:

```go
func (h *Handler) CreateGame(w http.ResponseWriter, r *http.Request) {
    // 1. Parse and validate request
    data := &CreateGameRequest{}
    if err := render.Bind(r, data); err != nil {
        render.Render(w, r, core.ErrInvalidRequest(err))
        return
    }

    // 2. Extract authentication context
    user := core.GetAuthenticatedUser(r.Context())
    if user == nil {
        render.Render(w, r, core.ErrUnauthorized("authentication required"))
        return
    }

    // 3. Call service layer
    game, err := h.GameService.CreateGame(r.Context(), core.CreateGameRequest{
        Title:       data.Title,
        Description: data.Description,
        GMUserID:    user.ID,
        // ... other fields
    })

    // 4. Handle errors with appropriate responses
    if err != nil {
        h.App.Logger.Error("Failed to create game", "error", err)
        render.Render(w, r, core.ErrInternalError(err))
        return
    }

    // 5. Transform to response format
    response := &GameResponse{
        ID:          game.ID,
        Title:       game.Title,
        Description: game.Description,
        // ... other fields
    }

    // 6. Send response
    render.Status(r, http.StatusCreated)
    render.Render(w, r, response)
}
```

### Request/Response Types

```go
type CreateGameRequest struct {
    Title               string     `json:"title" validate:"required,min=3,max=255"`
    Description         string     `json:"description" validate:"required,min=10"`
    Genre               string     `json:"genre,omitempty"`
    StartDate           *time.Time `json:"start_date,omitempty"`
    EndDate             *time.Time `json:"end_date,omitempty"`
    RecruitmentDeadline *time.Time `json:"recruitment_deadline,omitempty"`
    MaxPlayers          int32      `json:"max_players,omitempty"`
}

type GameResponse struct {
    ID                  int32      `json:"id"`
    Title               string     `json:"title"`
    Description         string     `json:"description"`
    GMUserID            int32      `json:"gm_user_id"`
    State               string     `json:"state"`
    Genre               string     `json:"genre,omitempty"`
    StartDate           *time.Time `json:"start_date,omitempty"`
    EndDate             *time.Time `json:"end_date,omitempty"`
    RecruitmentDeadline *time.Time `json:"recruitment_deadline,omitempty"`
    MaxPlayers          int32      `json:"max_players,omitempty"`
    CreatedAt           time.Time  `json:"created_at"`
    UpdatedAt           time.Time  `json:"updated_at"`
}
```

## Authentication & Authorization

### JWT Token Management

A **single bearer token, valid 7 days, backed by a server-side session row**.
There is no separate refresh token. Issued by `JWTHandler.CreateToken`
(`backend/pkg/auth/jwt.go`) in two phases: sign a temporary token to create the
session, then re-sign with `session_id` attached and write it back.

```go
// Final token — see backend/pkg/auth/jwt.go
finalToken := jwt.NewWithClaims(jwt.SigningMethodHS256,
    jwt.MapClaims{
        "sub":        strconv.Itoa(user.ID), // user ID, NOT username
        "session_id": session.ID,
        "exp":        time.Now().Add(time.Hour * 24 * 7).Unix(),
    })
```

Because the session lives in the database, **revocation is by deleting the
session** — that, not a short expiry, is the containment mechanism.

> ⚠️ `JWTConfig.AccessTokenExpiry` (15m) and `JWTConfig.RefreshTokenExpiry` (7d)
> exist in `core/config.go` with those defaults, but **nothing reads them** —
> `jwt.go` hardcodes the 7-day lifetime. Treat them as dead config; do not
> document or tune them as if they were live. This dead config is most likely
> what earlier revisions of this doc were describing.

### Middleware-Based Authorization

Authentication is middleware; **GM authorization is not**. There is no
`RequireGameMasterMiddleware`. Instead `games.Handler.GameMiddleware()` loads the
game and precomputes permissions into the request context, and handlers branch on
the result.

```go
// backend/pkg/http/root.go
r.Group(func(r chi.Router) {
    r.Use(core.RequireAuthenticationMiddleware(userService))
    r.Use(core.AdminModeMiddleware)

    r.Route("/games/{gameID}", func(r chi.Router) {
        r.Use(gameHandler.GameMiddleware())  // loads game + is_gm into ctx
        r.Put("/state", gameHandler.UpdateGameState)

        // Some routes additionally require a verified email:
        r.With(core.RequireEmailVerificationMiddleware(h.App.Pool)).
            Post("/characters", characterHandler.CreateCharacter)
    })
})
```

`GameMiddleware` puts `game`, `gameID`, and `is_gm` on the context, deriving
`is_gm` from `core.IsUserGameMaster(...)` (which honours admin mode). Note the
URL param is **`{gameID}`**, not `{id}`. A missing game renders 404 while a
database failure renders 500 — deliberately distinguished so alerting is not
misled by a blanket 404.

### Context-Based User Access

`core.GetAuthenticatedUser` is the only accessor — there is no
`GetAuthenticatedUserID` or `GetAuthenticatedUsername`. Read the fields off the
returned struct, and **nil-check it**.

```go
func (h *Handler) MyProtectedHandler(w http.ResponseWriter, r *http.Request) {
    user := core.GetAuthenticatedUser(r.Context())  // *core.AuthenticatedUser
    if user == nil {
        render.Render(w, r, core.ErrUnauthorized())
        return
    }
    _ = user.ID       // int32
    _ = user.Username // string
    _ = user.Email    // string
    _ = user.IsAdmin  // bool
}
```

## Error Handling

### Structured Error Responses

```go
type ErrResponse struct {
    Err            error `json:"-"`              // Internal error (never exposed)
    HTTPStatusCode int   `json:"-"`              // HTTP status code
    StatusText     string `json:"status"`         // User-friendly status
    AppCode        int64  `json:"code,omitempty"` // Application error code
    ErrorText      string `json:"error,omitempty"` // Safe error message
}
```

### Application Error Codes

```go
const (
    ErrCodeValidation         = 1001
    ErrCodeGameNotRecruiting  = 1302
    ErrCodeGameFull          = 1303
    ErrCodeAlreadyParticipant = 1305
    // ... other codes
)
```

### Error Helper Functions

```go
// Generic errors
render.Render(w, r, core.ErrInvalidRequest(err))
render.Render(w, r, core.ErrUnauthorized("Invalid token"))
render.Render(w, r, core.ErrInternalError(err))

// Specific business errors
render.Render(w, r, core.ErrGameNotRecruiting())
render.Render(w, r, core.ErrGameFull())
render.Render(w, r, core.ErrWithCode(400, ErrCodeCustom, "Custom message"))
```

## Configuration Management

### Environment-Based Configuration

```go
type Config struct {
    Database DatabaseConfig `env:"DATABASE"`
    JWT      JWTConfig      `env:"JWT"`
    Server   ServerConfig   `env:"SERVER"`
    App      AppConfig      `env:"APP"`
}

// Load configuration with validation
config, err := core.LoadConfig()
if err != nil {
    log.Fatal("Configuration error", "error", err)
}
```

### Required Environment Variables

**`.env.example` is the authoritative list** (~75 variables). It is kept current;
this section only orients you to the groups. Do not treat the sample below as
complete.

```bash
# Required
DATABASE_URL="postgres://postgres:example@localhost:5432/actionphase?sslmode=disable"
JWT_SECRET="your-super-secret-jwt-signing-key"

# Common optional (with defaults)
ENVIRONMENT="development"  # development, staging, production
LOG_LEVEL="info"           # debug, info, warn, error
PORT="3000"                # HTTP server port
RUN_MIGRATIONS="true"      # auto-apply migrations on backend boot
```

Other groups you will encounter in `.env.example`:

| Group | Prefix / examples |
|---|---|
| Server tuning | `HOST`, `SERVER_READ_TIMEOUT`, `SERVER_WRITE_TIMEOUT`, `SERVER_IDLE_TIMEOUT` |
| CORS | `CORS_ENABLED`, `CORS_ORIGINS`, `FRONTEND_URL` |
| Email | `EMAIL_PROVIDER`, `RESEND_API_KEY`, `SMTP_*`, `MAILHOG_*` |
| Registration / anti-bot | `HCAPTCHA_*`, `REQUIRE_EMAIL_VERIFICATION`, `MAX_REGISTRATIONS_PER_IP_PER_DAY`, `BLOCK_DISPOSABLE_EMAILS` |
| Discord | `DISCORD_BOT_TOKEN`, `DISCORD_CLIENT_ID`, `DISCORD_CLIENT_SECRET`, `DISCORD_REDIRECT_URL` |
| Storage | `STORAGE_BACKEND`, `STORAGE_LOCAL_PATH`, `STORAGE_S3_*`, `STORAGE_ARCHIVE_PATH` |
| Observability | `OTEL_*`, `GRAFANA_FARO_*`, `VITE_FARO_*` |
| Testing | `SKIP_DB_TESTS`, `TEST_DATABASE_URL`, `TEST_PARALLEL`, `TEST_CLEANUP` |

Note the `VITE_`-prefixed entries are consumed by the **frontend** build, not the
Go backend, though both read the same `.env`.

> ⚠️ `JWT_ACCESS_TOKEN_EXPIRY` / `JWT_REFRESH_TOKEN_EXPIRY` appear here and parse
> into `JWTConfig`, but nothing reads them — see the JWT section above.

### Configuration Validation

```go
func (c *Config) Validate() error {
    if c.Database.URL == "" {
        return fmt.Errorf("DATABASE_URL is required")
    }

    if c.JWT.Secret == "" {
        return fmt.Errorf("JWT_SECRET is required")
    }

    if c.App.Environment == "production" && len(c.JWT.Secret) < 32 {
        return fmt.Errorf("JWT_SECRET must be at least 32 characters in production")
    }

    return nil
}
```

## Testing Strategy

### Test Categories

1. **Unit Tests**: Test individual functions/methods with mocks
2. **Integration Tests**: Test with real database
3. **API Tests**: Test HTTP endpoints end-to-end

### Test Database Isolation

Each test **package** clones its own database from a migrated template
(`actionphase_test_template`), so packages run in parallel safely. Consequences:

- Do **not** add `-p=1`; the isolation exists precisely to avoid serializing.
  Set `TEST_P=1` only when debugging.
- Cross-package fixture collisions (duplicate keys, FK surprises) should not
  happen. If they do, the template is stale — run `just reset-test-db`.
- Run with `just test` / `just test-integration`, which set `TEST_ENV`
  (including `SKIP_DB_TESTS=false`) for you.

### Mock-Based Unit Testing

```go
func TestGameService_CreateGame(t *testing.T) {
    t.Parallel()

    mockRepo := core.CreateMockDatabaseRepo()

    req := core.CreateGameRequest{
        Title:       "Test Game",
        Description: "Test Description",
        GMUserID:    1,
    }

    game, err := mockRepo.Game.CreateGame(context.Background(), req)

    core.AssertNoError(t, err, "Should create game successfully")
    core.AssertEqual(t, req.Title, game.Title, "Title should match")
}
```

### Database Integration Testing

```go
func TestGameService_CreateGame_Integration(t *testing.T) {
    t.Parallel()

    testDB := core.NewTestDatabase(t)
    defer testDB.Close()
    defer testDB.CleanupTables(t)

    gameService := &db.GameService{DB: testDB.Pool}

    // Create test user first
    factory := core.NewTestDataFactory(testDB, t)
    user := factory.NewUser().WithUsername("testgm").Create()

    req := core.CreateGameRequest{
        Title:       "Integration Test Game",
        Description: "Test with real database",
        GMUserID:    int32(user.ID),
    }

    game, err := gameService.CreateGame(context.Background(), req)

    core.AssertNoError(t, err, "Should create game in database")
    core.AssertNotEqual(t, int32(0), game.ID, "Game should have valid ID")
}
```

### Test Data Factories

```go
// Create complex test scenarios easily
user := factory.NewUser().
    WithUsername("player1").
    WithEmail("player1@example.com").
    Create()

game := factory.NewGame().
    WithTitle("Epic Campaign").
    WithGM(user).
    WithState(core.GameStateRecruitment).
    WithMaxPlayers(6).
    Create()

// Add participants
factory.NewParticipant().
    ForGame(game).
    WithUser(player2).
    WithRole(core.RolePlayer).
    Create()
```

## AI-Friendly Features

### 1. Comprehensive Documentation

Every interface, function, and package includes:
- Purpose and usage examples
- Parameter descriptions
- Return value explanations
- Error conditions
- Business rule documentation

### 2. Consistent Naming Conventions

- **Interfaces**: `UserServiceInterface`, `GameServiceInterface`
- **Implementations**: `UserService`, `GameService`
- **Requests/Responses**: `CreateGameRequest`, `GameResponse`
- **Constants**: `GameStateSetup`, `RolePlayer`
- **Errors**: `ErrGameNotFound`, `ErrInvalidRequest`

### 3. Type Safety

- SQLC generates type-safe database code
- Strong typing throughout the application
- Interface-based design enables compile-time checking
- Generic helper functions where appropriate

### 4. Self-Documenting Code

```go
// Clear, descriptive function names
func (gs *GameService) CanUserJoinGame(ctx context.Context, gameID, userID int32) (string, error)

// Descriptive variable names
joinStatus, err := gameService.CanUserJoinGame(ctx, gameID, userID)
if joinStatus != core.CanJoin {
    return fmt.Errorf("user cannot join game: %s", joinStatus)
}

// Constants instead of magic strings
if game.State != core.GameStateRecruitment {
    return core.ErrGameNotRecruiting()
}
```

### 5. Structured Error Handling

- Consistent error response format
- Application-specific error codes
- Clear separation between internal errors and user messages
- Helper functions for common error scenarios

### 6. Testing Infrastructure

- Mock implementations for all services
- Test data factories with fluent interfaces
- Database test utilities with automatic cleanup
- Parallel test execution support
- Environment-based test configuration
- Fast mock tests (~0.3s) for rapid development
- Docker-integrated database tests for CI/CD

### 7. Development Environment Excellence

- **Environment Variable Management**: `.env` file support with validation
- **Docker Integration**: PostgreSQL database runs in containers
- **One-Command Setup**: `just dev-setup` for new developers
- **Multiple Test Strategies**: Fast mocks vs. comprehensive integration tests
- **Smart Configuration**: Auto-detection of environment capabilities
- **Production Ready**: Security validation and deployment considerations

## Development Guidelines

### Adding New Features

1. **Setup Development Environment** (first time):
   ```bash
   just dev-setup      # .env + build images + start the stack
   # migrations auto-run on backend boot; code hot-reloads via Air/Vite
   ```

2. **Define Domain Model**: Add types to `pkg/core/`
   - Use `pkg/core/constants.go` for enums and magic values
   - Add validation functions where appropriate

3. **Create Interface**: Define service contract in `pkg/core/interfaces.go`
   - Include comprehensive documentation with usage examples
   - Define request/response types in core package

4. **Implement Repository**: Add data access in `pkg/db/services/`
   - Use SQLC for type-safe database queries
   - Add both interface implementation and compile-time verification

5. **Create Handlers**: Add HTTP handlers in appropriate package
   - Use structured error responses from `pkg/core/api_errors.go`
   - Follow consistent request/response patterns

6. **Add Tests**: Multiple testing strategies
   ```bash
   just test-mocks        # Fast development feedback
   just test-integration  # Comprehensive validation
   ```

7. **Update Documentation**: Add examples and usage notes
   - Update API documentation
   - Add environment variables to `.env.example` if needed

### Code Review Checklist

- [ ] All public functions have documentation with examples
- [ ] Error handling follows established patterns
- [ ] Constants used instead of magic strings/numbers
- [ ] Interfaces defined before implementations
- [ ] Mock implementations created for testing
- [ ] Integration tests cover happy path and error cases
- [ ] Configuration externalized to environment variables
- [ ] Proper separation of concerns maintained
- [ ] Environment variables documented in `.env.example`
- [ ] Fast mock tests written for rapid feedback
- [ ] Integration tests work with Docker database

### Performance Considerations

- Database connection pooling configured appropriately
- Proper indexing on frequently queried columns
- Context timeout handling for long operations
- Graceful degradation for external service failures
- Efficient pagination for list endpoints

### Security Best Practices

- Password hashing with bcrypt
- JWT tokens with appropriate expiration
- Input validation and sanitization
- SQL injection prevention through parameterized queries
- Proper CORS configuration
- Rate limiting for authentication endpoints
- Secure headers in HTTP responses

## Conclusion

The ActionPhase backend architecture prioritizes:

1. **Maintainability**: Clear separation of concerns and consistent patterns
2. **Testability**: Every component can be unit tested with mocks
3. **AI-Friendliness**: Comprehensive documentation and self-documenting code
4. **Scalability**: Modular design that can grow with requirements
5. **Security**: Industry-standard security practices throughout

This architecture enables rapid development while maintaining high code quality and making it easy for AI assistants to understand, navigate, and modify the codebase safely.
