# Developer Onboarding Guide

## Welcome to ActionPhase! 🎲

This guide will get you up and running with the ActionPhase codebase in under 30 minutes. ActionPhase is a modern web application for hosting play-by-post RPG games with a cyclical phase-based gameplay system.

## Quick Start (5 minutes)

### Prerequisites
Local development is **fully containerized**. You need only three things on the
host — no host Go, Node, psql, or migrate:

- **`just`** - [Install just](https://github.com/casey/just#installation)
- **Docker & Docker Compose** - [Install Docker](https://docs.docker.com/get-docker/)
- **git**

(For reference, the containers run Go 1.25 and Node 24.)

### Get Running Immediately

```bash
# 1. Clone the repository
git clone https://github.com/yourorg/actionphase
cd actionphase

# 2. First-time setup: create .env, build images, start the stack
#    (run from the repo root — the justfile lives there)
just dev-setup

# 3. Subsequently, just bring the stack up
#    (db + backend + frontend in one command; migrations auto-run on boot)
just up
```

**That's it!** 🎉 You should now have:
- Backend API running on http://localhost:3000
- Frontend app running on http://localhost:5173
- PostgreSQL database running in Docker
- All dependencies installed

### Verify Everything Works

```bash
# Check container status
just ps

# Test the API
curl http://localhost:3000/ping
# Should return: ponger

curl http://localhost:3000/health
# Should return: {"status":"healthy","timestamp":"..."}

# Test frontend
open http://localhost:5173
# Should show the ActionPhase login/registration page
```

## Architecture Overview (10 minutes)

### High-Level System Design

ActionPhase follows Clean Architecture principles with clear separation of concerns:

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Frontend      │    │   Backend       │    │   Database      │
│   React 19/TS   │◄──►│   Go/Chi        │◄──►│   PostgreSQL    │
│   + TanStack Q  │    │   + JWT Auth    │    │   + Migrations  │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

### Tech Stack at a Glance

**Backend (Go)**:
- **Chi Router**: HTTP routing and middleware
- **PostgreSQL**: Primary database with JSONB for flexible game data
- **sqlc**: Type-safe SQL query generation
- **JWT**: `Authorization: Bearer` access token, with the refresh token in an
  HTTP-only cookie
- **Structured Logging**: Context-aware logging with correlation IDs
- **OpenTelemetry**: Traces, metrics, and logs exported to Grafana Cloud

**Frontend (React)**:
- **React 19**: Modern React with hooks
- **TypeScript**: Type safety throughout
- **TanStack Query 5**: Server state management and caching (`useQuery`)
- **React Router 7**: `createBrowserRouter` (not the v6 `<Routes>` JSX form)
- **Tailwind CSS 4**: CSS-first config via `@theme` in `src/index.css` —
  there is **no `tailwind.config.js`**
- **Vite 7**: Fast development and building
- **Grafana Faro**: RUM and browser tracing

**Key Concepts**:
- **Games**: RPG campaigns managed by Game Masters (GMs)
- **Characters**: Player characters within games
- **Phases**: Cyclical gameplay phases (Planning → Action → Resolution)
- **Users**: Players and GMs with JWT-based authentication

### Project Structure

```
actionphase/
├── backend/                    # Go backend service
│   ├── pkg/                   # Go packages (main code)
│   │   ├── core/             # Domain models and interfaces
│   │   ├── auth/             # Authentication logic
│   │   ├── games/            # Game management
│   │   ├── db/               # Database layer (sqlc models, queries, services)
│   │   ├── http/             # HTTP handlers and routing
│   │   └── observability/    # Logging, metrics, tracing
│   └── main.go              # Application entry point
├── frontend/                 # React frontend
│   ├── src/
│   │   ├── components/      # Reusable React components
│   │   ├── pages/           # Page components
│   │   ├── hooks/           # Custom React hooks
│   │   └── lib/             # Utilities and API client
│   └── package.json
├── docs-site/               # VitePress documentation site
│   └── developer/
│       ├── architecture/    # System design docs + adrs/
│       └── testing/         # Testing guides
├── docs/                    # Deployment, operations, feature notes
└── justfile                 # All development commands (repo root)
```

## Core Development Concepts (10 minutes)

### 1. Backend Patterns

**Interface-First Development**: All services are defined as interfaces in `pkg/core/interfaces.go`:

```go
// Define the contract first (pkg/core/interfaces.go)
type GameServiceInterface interface {
    CreateGame(ctx context.Context, req CreateGameRequest) (*models.Game, error)
    GetGame(ctx context.Context, gameID int32) (*models.Game, error)
}

// Implement in the service layer (pkg/db/services/games.go).
// Services hold the pgx pool directly — there is no repository layer.
type GameService struct {
    DB     *pgxpool.Pool
    Logger *observability.Logger
}

// Verify the implementation satisfies the interface at compile time
var _ core.GameServiceInterface = (*GameService)(nil)

func (s *GameService) CreateGame(ctx context.Context, req CreateGameRequest) (*models.Game, error) {
    // Business logic, then sqlc-generated queries against s.DB
}
```

**Request Processing Flow**:
```
HTTP Request → Middleware → Handler → Service → sqlc Queries → Database
     ↓              ↓          ↓         ↓            ↓            ↓
Correlation ID  Auth/CORS   Bind +    Business   Type-safe    PostgreSQL
Telemetry       Rate Limit  Validate  Logic      generated Go  ACID Ops
```

**Database Integration**: We use `sqlc` for type-safe SQL:
```sql
-- In backend/pkg/db/queries/games.sql
-- name: CreateGame :one
-- (abridged — the real query inserts 20 columns)
INSERT INTO games (
    title, description, gm_user_id, genre, max_players,
    is_public, character_sheet
) VALUES (
    $1, $2, $3, $4, $5, $6, COALESCE($7, '{}'::jsonb)
)
RETURNING *;
```

This generates type-safe Go code:
```go
// Auto-generated by sqlc
func (q *Queries) CreateGame(ctx context.Context, arg CreateGameParams) (Game, error) {
    // Implementation generated automatically
}
```

### 2. Frontend Patterns

**Server State with React Query**: All API interactions use React Query for caching and synchronization:

```typescript
// Custom hook for games (see src/hooks/useGameListing.ts).
// Note it takes no arguments — it derives filters from URL search params,
// so the filter state is shareable and survives a reload.
import { useQuery } from '@tanstack/react-query';
import { apiClient } from '../lib/api';

export function useGameListing() {
  const [searchParams] = useSearchParams();
  const filters = useMemo(() => parseFiltersFrom(searchParams), [searchParams]);

  return useQuery({
    queryKey: ['games', 'filtered', filters],
    queryFn: async () => {
      const response = await apiClient.games.getFilteredGames(filters);
      return response.data;
    },
  });
}

// In components
function GamesList() {
  const { data: games, isLoading, error } = useGameListing();

  if (isLoading) return <LoadingSpinner />;
  if (error) return <ErrorMessage error={error} />;

  return (
    <div className="grid gap-4">
      {games?.map(game => <GameCard key={game.id} game={game} />)}
    </div>
  );
}
```

**Authentication Context**: Global auth state managed via React Context:
```typescript
// Note the field is `currentUser`, not `user`.
const { currentUser, login, logout, isAuthenticated, isCheckingAuth } = useAuth();

// Automatic token refresh via axios interceptors (src/lib/api/client.ts).
// Two details the naive version gets wrong:
//   1. refreshes go through a SEPARATE axios client, or the interceptor
//      recurses into itself on a failing refresh
//   2. a shared refreshPromise de-duplicates concurrent 401s, so N parallel
//      requests trigger one refresh rather than N
response => response,
async error => {
  if (error.response?.status === 401) {
    await this.refreshOnce();      // shared promise, separate client
    return this.client(error.config); // retry once
  }
  return Promise.reject(error);
}
```

### 3. Database Patterns

**Hybrid Relational-Document Design**: Core entities are relational, flexible data uses JSONB:

```sql
-- Structured data (abridged from backend/pkg/db/schema.sql)
CREATE TABLE games (
    id SERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    gm_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    state VARCHAR(50) DEFAULT 'setup',
    max_players INTEGER DEFAULT 6,
    -- Per-game character sheet config. Sparse: '{}' means all defaults.
    character_sheet JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Query JSONB data
SELECT character_sheet->>'tabLabel' AS tab_label
FROM games
WHERE character_sheet != '{}'::jsonb;
```

Other JSONB columns: `user_preferences.preferences` (GIN-indexed).

**Migration Management**: Database schema changes are versioned:
```bash
just migration create add_character_approval_system
just migrate           # Apply to development
just reset-test-db     # Rebuild the test DB if it gets into a dirty state
```

## Development Workflow (5 minutes)

### Common Development Tasks

```bash
# Stack lifecycle (code hot-reloads; no restart needed after edits)
just up                 # Start db + backend + frontend
just down               # Stop the stack (data preserved)
just ps                 # Container status
just dev-logs backend   # Tail a service's logs
just sh backend         # Shell into a container for one-off commands

# Backend Development (runs in the backend container)
just test-mocks         # Run fast unit tests
just test-integration   # Run database integration tests
just test               # Run all tests
just lint               # Format and lint Go code
just sqlgen             # Regenerate sqlc queries after SQL changes

# Frontend Development (runs in the frontend container)
just test-fe run        # Run React component tests
just test-fe watch      # Watch mode
just lint-frontend      # Run ESLint
just build-frontend     # Build for production

# Checks
just verify-quick       # Fast non-mutating checks (no builds)
just verify             # Pre-push gate: all checks + production builds

# Database Management
just db up              # Start just the database container
just db down            # Stop it
just migration create add_new_feature  # Create new migration
just migrate            # Apply pending migrations

# E2E (only after unit/component tests pass)
just e2e-desktop        # Chrome
just e2e-mobile         # Pixel 5
```

### Typical Feature Development Flow

1. **Plan**: Create/update Architecture Decision Records if needed
2. **Backend**:
   - Add interface to `core/interfaces.go`
   - Implement service logic
   - Add database queries in `db/queries/`
   - Create HTTP handlers
   - Write tests (unit + integration)
3. **Frontend**:
   - Create API client methods
   - Build React components
   - Add custom hooks for state management
   - Write component tests
4. **Integration**: Test end-to-end functionality
5. **Documentation**: Update relevant docs

### Code Review Checklist

- [ ] **Tests**: Unit tests for business logic, integration tests for database operations
- [ ] **Error Handling**: Proper error messages and HTTP status codes
- [ ] **Logging**: Structured logs with correlation IDs for debugging
- [ ] **Types**: Strong typing in both Go and TypeScript
- [ ] **Security**: No hardcoded secrets, proper input validation
- [ ] **Performance**: Efficient database queries, proper React Query usage

## Key Files to Understand

### Backend Entry Points
- `main.go` - Application startup and dependency injection
- `pkg/http/root.go` - HTTP routing and middleware setup
- `pkg/core/interfaces.go` - All service interfaces (your API contracts)
- `pkg/core/*.go` - Domain models, split per bounded context
  (`games.go`, `characters.go`, `phases.go`, `notifications.go`, ...)

### Frontend Entry Points
- `src/main.tsx` - React app initialization with providers
- `src/lib/api/` - API client, split per domain (`client.ts` holds the axios
  instance + JWT/refresh interceptors; `games.ts`, `characters.ts`, ...)
- `src/contexts/AuthContext.tsx` - Authentication state management
- `src/hooks/` - Custom hooks for server state management

### Configuration & Setup
- `backend/.env` - Environment variables (copy from `.env.example`)
- `frontend/package.json` - Frontend dependencies and scripts
- `backend/justfile` - All development commands in one place
- `docker-compose.yml` - Development database setup

## Testing Philosophy

ActionPhase uses a multi-layered testing approach:

**Fast Unit Tests** (`just test-mocks`):
- Test business logic with mocked dependencies
- Interface-based mocking for clean isolation
- Run in ~300ms for rapid feedback

**Integration Tests** (`just test-integration`):
- Test database interactions with real PostgreSQL
- Use transactions for test isolation
- Comprehensive API endpoint testing

**Frontend Tests** (`just test-fe run`):
- Component testing with React Testing Library
- Custom hook testing for complex state logic
- Mock API responses for predictable testing

```bash
# Test pyramid in practice
just test-mocks       # 🟢 Fast feedback
just test-fe run      # 🟢 Frontend component tests
just test-integration # 🟡 Comprehensive coverage (real DB)
just e2e-desktop      # 🔴 Full user journeys (slow — run last)
```

## Common Gotchas & Solutions

### 1. Database Connection Issues
```bash
# If database tests fail
just db reset        # Wipe + recreate the database volume
just migrate         # Ensure migrations are applied
just reset-test-db   # Rebuild the test DB if it is in a dirty state
```

### 2. JWT Token Expiry in Development
```bash
# If API returns 401 errors unexpectedly
# Frontend will auto-refresh tokens, but you might need to clear storage
localStorage.clear() # In browser console
```

### 3. React Query Cache Confusion
```typescript
// If UI doesn't update after mutations
const queryClient = useQueryClient();
queryClient.invalidateQueries(['games']); // Force refetch

// Note: React Query DevTools is not installed in this project
```

### 4. Go Module Issues
```bash
# If Go dependencies are problematic (runs in the backend container)
just tidy            # go mod tidy
docker compose -f docker-compose.dev.yml exec -T backend go mod download
```

## Debugging Tools

### Backend Debugging
- **Structured Logs**: Every request has a correlation ID for tracing
- **Health Endpoint**: http://localhost:3000/health - service status
- **Telemetry**: Traces/metrics/logs export to Grafana Cloud via OTLP. There is
  **no `/metrics` endpoint** on the backend.
- **Delve debugger**: attach on `localhost:2345`
- **Logs**: `just dev-logs backend` (or `db` / `frontend`)

### Frontend Debugging
- **React DevTools**: Component state and props inspection
- **Network Tab**: API request/response inspection
- **Grafana Faro**: frontend RUM/tracing ships to Grafana Cloud (`src/lib/faro.ts`)

> React Query DevTools is **not currently installed** (`@tanstack/react-query-devtools`
> is not a dependency). Add it if you want cache inspection.

### Example Debugging Session
```bash
# Backend issue: Find requests by correlation ID
just dev-logs backend | grep "corr_abc123"

# Frontend issue: Check React Query cache
# Open React Query DevTools in browser (bottom left toggle)

# Database issue: Connect directly to investigate
docker compose -f docker-compose.dev.yml exec -T db \
  psql -U postgres -d actionphase \
  -c "SELECT * FROM games WHERE created_at > NOW() - INTERVAL '1 hour';"
```

## Getting Help

### Documentation
- **Architecture**: `/docs/architecture/` - System design and patterns
- **ADRs**: `/docs/adrs/` - Architectural decisions and rationale
- **API Documentation**: Generate with `just docs` (future)
- **Database Schema**: Check migrations in `pkg/db/migrations/`

### Code Navigation Tips
- **Find Interface**: All contracts in `pkg/core/interfaces.go`
- **Find Implementation**: Look in corresponding service/repository packages
- **Find API Endpoint**: Check `pkg/http/root.go` for URL mapping
- **Find React Component**: Components are in `src/components/` or `src/pages/`

### Common Questions

**Q: How do I add a new API endpoint?**
1. Add interface method to `core/interfaces.go`
2. Implement in service layer
3. Add handler in the relevant `api.go`
4. Add route in `pkg/http/root.go`
5. Add frontend API client method in `src/lib/api/<domain>.ts`

**Q: How do I modify the database schema?**
```bash
just migration create add_new_column
# Edit the generated .up.sql / .down.sql files
just migrate
```

**Q: How do I add a new React component?**
1. Create component in `src/components/`
2. Add to appropriate page in `src/pages/`
3. Create custom hooks if complex state needed
4. Add tests in `.test.tsx` file

**Q: How do I debug a failing test?**
```bash
# Run specific test with verbose output
go test -v ./pkg/games -run TestCreateGame
npm test -- --verbose GameForm.test.tsx
```

---

## Next Steps

Now that you're set up:

1. **Explore the Codebase**: Browse `pkg/games/` and `src/pages/GamesPage.tsx` to see a complete feature
2. **Run Tests**: `just test-mocks && just test-fe run` to verify everything works
3. **Make a Small Change**: Try adding a field to the game creation form
4. **Read Architecture Docs**: Check `/docs/architecture/` for deeper system understanding
5. **Check Out Issues**: Look for "good first issue" labels in the repository

Welcome to the team! 🎉 The codebase is designed to be approachable, and this onboarding guide should have you productive quickly. Don't hesitate to ask questions or suggest improvements to this guide.

**Happy coding!** 🚀
