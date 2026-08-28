# Component Interaction Patterns

## Overview

This document describes the interaction patterns between different components in ActionPhase, illustrating how data flows through the system and how components communicate with each other.

## System Component Map

```
┌─────────────────────────────────────────────────────────────────┐
│                          Frontend (React)                       │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────┐ │
│  │   Pages     │  │ Components  │  │    Hooks    │  │  Utils  │ │
│  │ GamesList   │  │  GameForm   │  │ useGames()  │  │   API   │ │
│  │ GameDetail  │  │ CharForm    │  │ useAuth()   │  │ Client  │ │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────┘ │
│                          │                                       │
│                          ▼ HTTP/JSON                            │
├─────────────────────────────────────────────────────────────────┤
│                     HTTP API Layer                              │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────┐ │
│  │ Middleware  │  │  Handlers   │  │  Services   │  │  Core   │ │
│  │   Tracing   │  │    Games    │  │   Business  │  │ Domain  │ │
│  │    Auth     │  │    Auth     │  │    Logic    │  │ Models  │ │
│  │   Metrics   │  │  Characters │  │             │  │         │ │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────┘ │
│                          │                                       │
│                          ▼ SQL Queries                          │
├─────────────────────────────────────────────────────────────────┤
│                     Database Layer                              │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐              │
│  │  Services   │  │    sqlc     │  │ PostgreSQL  │              │
│  │ Interfaces  │  │  Generated  │  │  Database   │              │
│  │             │  │   Queries   │  │             │              │
│  └─────────────┘  └─────────────┘  └─────────────┘              │
└─────────────────────────────────────────────────────────────────┘
```

## Interaction Patterns

### 1. Request Flow Pattern

**Typical HTTP Request Processing:**

```
User Action → React Component → Custom Hook → API Client → HTTP Request
     ↓
HTTP Middleware Stack → Route Handler → Service Layer → sqlc Queries
     ↓
Database Query → Results → Response → React Query Cache → UI Update
```

**Detailed Flow:**
1. **User Interaction**: User clicks "Create Game" button
2. **Component Event**: GameForm handles form submission
3. **Hook Invocation**: `useCreateGame()` mutation is triggered
4. **API Call**: Axios sends POST request to `/api/v1/games`
5. **Middleware Processing**:
   - Request tracing adds correlation ID
   - Authentication validates JWT token
   - Metrics records request start time
6. **Handler Processing**: Game handler validates and processes request
7. **Service Layer**: Business logic validation and orchestration
8. **Data Access**: Services call sqlc-generated queries directly (no repository layer)
9. **Response Chain**: Results bubble back up through layers
10. **UI Update**: React Query updates cache and re-renders components

### 2. Authentication Flow Pattern

**JWT Authentication and Refresh Pattern:**

```
Login Request → Auth Handler → Credential Validation → JWT Generation
     ↓
Store Access + Refresh Tokens → Client Storage → API Request Headers
     ↓
JWT Validation Middleware → Extract User Context → Pass to Handlers
     ↓
Token Expiry → Axios Interceptor → Auto-Refresh → Retry Original Request
```

**Components Involved:**
- `AuthContext` - Client-side auth state management
- `axios.interceptors` - Automatic token refresh
- `JWTAuthenticator` middleware - Server-side token validation
- `SessionService` - Server-side session management
- `RefreshTokenHandler` - Token refresh endpoint

### 3. Game Management Pattern

**Complex Business Logic Flow:**

```
Game Creation → Validation → Database Transaction → Event Logging
     ↓
Phase Management → State Transitions → Player Notifications → History
     ↓
Character Actions → Game State Updates → Conflict Resolution → Results
```

**Service Interactions:**
```go
// Game creation involves multiple services
func (h *GameHandler) CreateGame(w http.ResponseWriter, r *http.Request) {
    // 1. Extract user from JWT context
    userID := GetUserIDFromContext(r.Context())

    // 2. Validate request data
    game, err := h.validateGameRequest(r)

    // 3. Check business rules
    if err := h.GameService.ValidateGameCreation(userID, game); err != nil {
        // Handle validation error
    }

    // 4. Create game in database
    createdGame, err := h.GameService.CreateGame(r.Context(), game)

    // 5. Log business event
    h.MetricsService.IncrementCounter("games_created")
    h.Logger.LogOperation(r.Context(), "game_created",
        "game_id", createdGame.ID, "user_id", userID)

    // 6. Return response
    respondWithJSON(w, http.StatusCreated, createdGame)
}
```

### 4. Data Consistency Pattern

**Cross-Component State Management:**

```
Server State (React Query) ←→ API ←→ Database
     ↓
Local UI State (useState) → Form Controls → Validation
     ↓
Global State (Context) → Auth, Settings → Cross-Component Communication
```

**React Query Integration:**
```typescript
// Cache invalidation pattern
export function useCreateCharacter(gameId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: api.characters.create,
    onSuccess: (newCharacter) => {
      // Update multiple related caches
      queryClient.setQueryData(['characters', newCharacter.id], newCharacter);
      queryClient.invalidateQueries(['games', gameId, 'characters']);
      queryClient.invalidateQueries(['games', gameId]); // Update player count

      // Optimistic updates for better UX
      queryClient.setQueryData(['games'], (oldGames) =>
        oldGames.map(game =>
          game.id === gameId
            ? { ...game, playerCount: game.playerCount + 1 }
            : game
        )
      );
    }
  });
}
```

## Communication Patterns

### 1. HTTP API Communication

**Standard REST Patterns:**
```
GET    /api/v1/games              → GameHandler.ListGames()
POST   /api/v1/games              → GameHandler.CreateGame()
GET    /api/v1/games/{id}         → GameHandler.GetGame()
PUT    /api/v1/games/{id}         → GameHandler.UpdateGame()
DELETE /api/v1/games/{id}         → GameHandler.DeleteGame()

# Nested resources
GET    /api/v1/games/{id}/characters → CharacterHandler.ListGameCharacters()
POST   /api/v1/games/{id}/join       → GameHandler.JoinGame()
```

**Request/Response Format:**
```json
// Standard request format
{
  "data": {
    "title": "My New Game",
    "description": "Epic fantasy campaign",
    "max_players": 4
  }
}

// Standard response format
{
  "data": {
    "id": 123,
    "title": "My New Game",
    "status": "setup",
    "created_at": "2025-08-07T10:30:00Z"
  },
  "meta": {
    "correlation_id": "corr_abc123",
    "timestamp": "2025-08-07T10:30:00Z"
  }
}
```

### 2. Database Access Pattern

**Service Layer Implementation:**

> ❌ **There is no repository layer.** The sample originally here described a
> `GameRepositoryInterface` / `GameRepository` pair that does not exist — as does
> "Repository" in the diagram and request-flow list above. Services depend on the
> pgx pool directly and call sqlc-generated queries. Corrected below from
> `backend/pkg/db/services/games.go` (verified 2026-08-26).

```go
// Interface in core package (pkg/core/interfaces.go)
type GameServiceInterface interface {
    CreateGame(ctx context.Context, req CreateGameRequest) (*models.Game, error)
    GetGame(ctx context.Context, gameID int32) (*models.Game, error)
    GetGamesByUser(ctx context.Context, userID int32) ([]*models.Game, error)
}

// Implementation in pkg/db/services — no repository, just the pool
type GameService struct {
    DB     *pgxpool.Pool
    Logger *observability.Logger
}

// Compile-time check that the implementation satisfies the interface
var _ core.GameServiceInterface = (*GameService)(nil)

func (s *GameService) CreateGame(ctx context.Context, req core.CreateGameRequest) (*models.Game, error) {
    queries := models.New(s.DB)

    game, err := queries.CreateGame(ctx, models.CreateGameParams{
        Title:      req.Title,
        GmUserID:   req.GMUserID,
        MaxPlayers: req.MaxPlayers,
        // ... 20 columns total; character_sheet is COALESCE'd to '{}'
    })
    if err != nil {
        return nil, fmt.Errorf("creating game: %w", err)
    }

    return &game, nil
}
```

Note that sqlc's generated types **are** the domain models here (`models.Game`),
so there is no separate DB-model-to-domain-model conversion step.

### 3. Error Handling Pattern

**Layered Error Handling:**
```go
// Domain errors in core package
type ValidationError struct {
    Field   string
    Message string
}

func (e ValidationError) Error() string {
    return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Service layer error handling.
// NOTE: there is no repository layer — services hold the pgx pool and call
// sqlc-generated queries directly.
func (s *GameService) CreateGame(ctx context.Context, req core.CreateGameRequest) (*models.Game, error) {
    // Validate business rules
    if err := s.validateGame(req); err != nil {
        return nil, ValidationError{Field: "game", Message: err.Error()}
    }

    queries := models.New(s.DB)
    createdGame, err := queries.CreateGame(ctx, models.CreateGameParams{ /* ... */ })
    if err != nil {
        // Wrap database errors
        return nil, fmt.Errorf("failed to create game: %w", err)
    }

    return &createdGame, nil
}

// Handler layer error response
func (h *GameHandler) CreateGame(w http.ResponseWriter, r *http.Request) {
    game, err := h.GameService.CreateGame(r.Context(), gameData)
    if err != nil {
        // Convert errors to HTTP responses
        var validationErr ValidationError
        if errors.As(err, &validationErr) {
            respondWithError(w, http.StatusBadRequest, validationErr.Error())
            return
        }

        // Log unexpected errors
        h.Logger.LogError(r.Context(), err, "create_game_failed")
        respondWithError(w, http.StatusInternalServerError, "Internal server error")
        return
    }

    respondWithJSON(w, http.StatusCreated, game)
}
```

### 4. Frontend State Coordination

**Multi-Level State Management:**
```typescript
// Global auth state via Context
const AuthProvider = ({ children }) => {
  const [user, setUser] = useState(null);
  const [tokens, setTokens] = useState(null);

  // Auth state affects all components
  return (
    <AuthContext.Provider value={{ user, tokens, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
};

// Server state via React Query
const useGames = () => {
  return useQuery({
    queryKey: ['games'],
    queryFn: api.games.list,
    staleTime: 5 * 60 * 1000, // 5 minutes
  });
};

// Local component state
const GameForm = () => {
  const [formData, setFormData] = useState({});
  const [errors, setErrors] = useState({});
  const createGame = useCreateGame();

  const handleSubmit = async (data) => {
    try {
      await createGame.mutateAsync(data);
      navigate('/games');
    } catch (error) {
      setErrors(parseAPIError(error));
    }
  };

  return (
    // Form JSX
  );
};
```

## Event Flow Diagrams

### Game Creation Flow

```
Frontend                Backend                 Database
   │                      │                       │
   │──── POST /games ────►│                       │
   │                      │──── Validate ────────│
   │                      │                       │
   │                      │──── CreateGame ─────►│
   │                      │                       │
   │                      │◄─── Game Record ─────│
   │                      │                       │
   │                      │──── Log Event ───────│
   │                      │                       │
   │◄─── 201 Created ─────│                       │
   │                      │                       │
   │── Cache Update ──────│                       │
   │                      │                       │
   │── UI Re-render ──────│                       │
```

### Authentication Flow

```
Frontend                Backend                 Database
   │                      │                       │
   │──── POST /login ────►│                       │
   │                      │──── Validate ────────│
   │                      │                       │
   │                      │──── Check Password ──►│
   │                      │                       │
   │                      │◄─── User Record ─────│
   │                      │                       │
   │                      │──── Create Session ──►│
   │                      │                       │
   │                      │◄─── Session Record ──│
   │                      │                       │
   │                      │──── Generate JWT ────│
   │                      │                       │
   │◄─── JWT Tokens ──────│                       │
   │                      │                       │
   │── Store Tokens ──────│                       │
   │                      │                       │
   │── Update Auth State ─│                       │
```

### Error Handling Flow

```
Frontend                Backend                 Database
   │                      │                       │
   │──── API Request ────►│                       │
   │                      │──── Process ─────────►│
   │                      │                       │
   │                      │◄─── Error ───────────│
   │                      │                       │
   │                      │──── Log Error ───────│
   │                      │                       │
   │                      │──── Create Response ─│
   │                      │                       │
   │◄─── Error Response ──│                       │
   │                      │                       │
   │── Handle Error ──────│                       │
   │                      │                       │
   │── Show User Message ─│                       │
   │                      │                       │
   │── Retry/Recovery ────│                       │
```

## Performance Optimization Patterns

### 1. Caching Strategy

**Multi-Level Caching:**
```
Browser Cache → React Query Cache → Application Cache → Database Query Cache
     ↓               ↓                    ↓                     ↓
304 Not Modified   Stale While        In-Memory Cache        Query Plans
ETag Headers       Revalidate         Business Objects      Connection Pool
```

**React Query Configuration:**
```typescript
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 5 * 60 * 1000,        // 5 minutes
      cacheTime: 10 * 60 * 1000,       // 10 minutes
      retry: (failureCount, error) => {
        if (error.status === 404) return false;
        return failureCount < 3;
      },
    },
  },
});
```

### 2. Batch Operations

**Database Batch Queries:**
```go
// Batch character creation.
// NOTE: `characters` has no `character_data` column — sheet content lives in
// the separate EAV `character_data` TABLE (see ADR-002).
func (s *CharacterService) CreateCharactersBatch(ctx context.Context, characters []*Character) error {
    batch := &pgx.Batch{}

    for _, char := range characters {
        batch.Queue("INSERT INTO characters (game_id, user_id, name, character_type) VALUES ($1, $2, $3, $4)",
            char.GameID, char.UserID, char.Name, char.CharacterType)
    }

    results := r.DB.SendBatch(ctx, batch)
    defer results.Close()

    for range characters {
        _, err := results.Exec()
        if err != nil {
            return fmt.Errorf("batch character creation failed: %w", err)
        }
    }

    return nil
}
```

**Frontend Batch Updates:**
```typescript
// Batch state updates
const useBulkCharacterUpdate = (gameId: string) => {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: api.characters.bulkUpdate,
    onSuccess: (updatedCharacters) => {
      // Update cache for all characters at once
      updatedCharacters.forEach(character => {
        queryClient.setQueryData(['characters', character.id], character);
      });

      // Single cache invalidation for the list
      queryClient.invalidateQueries(['games', gameId, 'characters']);
    }
  });
};
```

### 3. Connection Management

**Database Connection Pooling:**
```go
config, _ := pgxpool.ParseConfig(databaseURL)
config.MaxConns = 20                    // Maximum connections
config.MinConns = 5                     // Minimum connections
config.MaxConnLifetime = time.Hour      // Connection lifetime
config.MaxConnIdleTime = 30 * time.Minute // Idle timeout

pool, err := pgxpool.ConnectConfig(context.Background(), config)
```

**HTTP Connection Reuse:**
```typescript
// Axios instance with connection reuse
const apiClient = axios.create({
  baseURL: '/api/v1',
  timeout: 30000,
  headers: {
    'Connection': 'keep-alive',
  },
});

// Request/response interceptors for token management
apiClient.interceptors.request.use(addAuthToken);
apiClient.interceptors.response.use(handleResponse, handleError);
```

## Security Interaction Patterns

### 1. Request Authentication

**Multi-Layer Security:**
```
Request → CORS Check → JWT Validation → Authorization → Rate Limiting
   ↓          ↓             ↓              ↓              ↓
Allow      Validate      Extract User   Check Perms    Throttle
Origin     Token         Claims         Resources      Requests
```

**Middleware Stack:**
```go
r.Use(middleware.CORS(&middleware.CORSConfig{
    AllowedOrigins:   []string{"http://localhost:3000"},
    AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE"},
    AllowedHeaders:   []string{"Authorization", "Content-Type"},
    AllowCredentials: true,
}))

r.Use(JWTAuthenticator(tokenAuth))
r.Use(RequestTracingMiddleware(logger))
r.Use(MetricsMiddleware(metrics))
r.Use(RateLimitMiddleware())
```

### 2. Data Validation

**Input Sanitization Pipeline:**
```
Raw Request → JSON Parsing → Schema Validation → Business Rules → Database Constraints
     ↓             ↓              ↓                   ↓                 ↓
HTTP Body      Struct Tags    Custom Validators   Service Logic    SQL Constraints
```

**Validation Implementation:**
```go
// Abridged from backend/pkg/games/requests.go. There is no `game_config` field.
type CreateGameRequest struct {
    Title       string  `json:"title" validate:"required,min=3,max=255"`
    Description string  `json:"description" validate:"required,min=10"`
    Genre       string  `json:"genre,omitempty"`
    MaxPlayers  int32   `json:"max_players,omitempty"`
    IsAnonymous bool    `json:"is_anonymous"`
    // ... plus scheduling fields and a sparse character_sheet override
}

// Validation runs in Bind, NOT in the handler or service.
//
// A `validate` tag alone enforces nothing — something must call ValidateStruct.
// And putting the check in the service is worse than useless for the caller:
// a service-layer rejection renders as a 500 "unexpected error", so a user
// typing a 2-character title would be told the server broke. Bind failures
// render as 400 with the message, which is what bad input actually is.
func (r *CreateGameRequest) Bind(req *http.Request) error {
    if err := core.ValidateStruct(r); err != nil {
        return err
    }

    // Business rule validation
    if req.MaxPlayers > 6 && req.GameConfig["complexity"] == "high" {
        return errors.New("high complexity games limited to 6 players")
    }

    return nil
}
```

## Future Interaction Patterns

### 1. Real-time Communication

**Planned WebSocket Integration:**
```
Game Event → WebSocket Server → Connected Clients → UI Updates
     ↓              ↓                  ↓              ↓
Phase Change    Broadcast Event    React Context   Component
Character Act   Filter by Game     State Update    Re-render
```

### 2. Event-Driven Architecture

**Future Event System:**
```go
type GameEvent struct {
    Type      string                 `json:"type"`
    GameID    int                    `json:"game_id"`
    UserID    int                    `json:"user_id"`
    Data      map[string]interface{} `json:"data"`
    Timestamp time.Time              `json:"timestamp"`
}

type EventBus interface {
    Publish(event GameEvent) error
    Subscribe(eventType string, handler EventHandler) error
}
```

### 3. Microservice Decomposition

**Potential Service Boundaries:**
```
API Gateway → Auth Service → Game Service → Character Service
     ↓            ↓              ↓               ↓
Route         JWT/Sessions   Game Logic    Character Logic
Transform     User Mgmt      Phase Mgmt    Sheet Validation
Rate Limit    Permissions    State Mgmt    Data Processing
```

This component interaction documentation provides a clear understanding of how different parts of the ActionPhase system communicate and work together, making it easier for developers to understand the codebase and make informed architectural decisions.
