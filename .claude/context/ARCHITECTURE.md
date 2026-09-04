# Architecture Context - Read Before Implementing Features

**IMPORTANT: Read this file before implementing new features or making architectural changes.**

**Last Verified**: August 2026

## Core Architectural Principles

ActionPhase follows **Clean Architecture** with clear separation of concerns:

1. **Interface-First Development** - Define interfaces before implementation
2. **Domain-Driven Design** - Clear bounded contexts (auth, games, characters, phases)
3. **Dependency Inversion** - Business logic isolated from infrastructure
4. **Observability-First** - Structured logging, correlation IDs, metrics
5. **API-First** - RESTful design with comprehensive validation

## Technology Stack

### Backend
- **Language**: Go 1.25
- **Router**: Chi (HTTP routing and middleware)
- **Database**: PostgreSQL with JSONB for flexible game data
- **Query Builder**: sqlc (type-safe SQL → Go code generation)
- **Authentication**: JWT bearer tokens backed by server-side sessions
- **Migrations**: golang-migrate

### Frontend
- **Framework**: React 19 + TypeScript 5.9
- **Build Tool**: Vite 7
- **Styling**: Tailwind CSS v4 (CSS-first config: `@import "tailwindcss"` + `@theme` in `src/index.css`; there is no `tailwind.config.js`)
- **State Management**: React Query + Context API (see STATE_MANAGEMENT.md)
- **HTTP Client**: Axios with JWT interceptors (`src/lib/api/client.ts`)

**See**: `/docs-site/developer/architecture/adrs/001-technology-stack-selection.md` for rationale

## Request Processing Flow

```
HTTP Request → Middleware Stack → Handler → Service → Repository → Database
     ↓              ↓               ↓         ↓          ↓           ↓
Correlation ID  Auth/CORS       Validate  Business   SQL Queries  PostgreSQL
Request Trace   Rate Limit      Bind      Logic      Type-Safe    ACID Ops
Metrics         Recovery        Error     Domain     Connection   Constraints
                                Handling   Rules      Pooling
```

## Backend Architecture Patterns

### 1. Interface-First Service Definition

**All services MUST be defined as interfaces in**: `backend/pkg/core/interfaces.go`

```go
// Define interface first
type GameServiceInterface interface {
    CreateGame(ctx context.Context, req *CreateGameRequest) (*Game, error)
    GetGame(ctx context.Context, id int) (*Game, error)
}

// Implement with compile-time verification
var _ GameServiceInterface = (*GameService)(nil)

type GameService struct {
    DB *pgxpool.Pool
}

func (s *GameService) CreateGame(ctx context.Context, req *CreateGameRequest) (*Game, error) {
    // Implementation
}
```

**Benefits**:
- Enables easy mocking for tests
- Clear contracts between layers
- Compile-time interface verification
- Supports dependency injection

### 2. Domain Models in Core

**Location**: `backend/pkg/core/*.go` — split per bounded context
(`games.go`, `characters.go`, `phases.go`, `notifications.go`, `conversations.go`,
`handouts.go`, `application.go`, ...). There is no single `models.go`.

- Define business entities here
- Keep separate from database models
- Use for API requests/responses
- Shared across all layers

### 3. Database Layer with sqlc

**Location**: `backend/pkg/db/`

```
db/
├── queries/        # SQL query files (*.sql)
├── models/         # Generated Go types (from sqlc)
├── migrations/     # Database schema migrations
├── services/       # Service implementations using queries
│                   #   (phases/, actions/, messages/ are multi-file subpackages)
├── test_fixtures/  # Seed SQL + apply scripts (common/, demo/, e2e/, perf/)
├── schema.sql      # Full generated schema
└── sqlc.yaml       # sqlc configuration
```

**Pattern**:
1. Write SQL in `queries/*.sql` with sqlc annotations
2. Run `just sqlgen` to generate type-safe Go code
3. Use generated queries in service implementations

**Example SQL** (`queries/games.sql`):
```sql
-- name: GetGame :one
SELECT id, title, description, gm_user_id, state, created_at
FROM games WHERE id = $1;

-- name: CreateGame :one
INSERT INTO games (title, description, gm_user_id)
VALUES ($1, $2, $3)
RETURNING id, title, description, gm_user_id, state, created_at;
```

### 4. HTTP Handler Pattern

**Location**: `backend/pkg/*/api.go` (one handler package per domain)

Current handler packages (each has an `api.go`): `admin`, `auth`, `avatars`, `characters`, `conversations`, `dashboard`, `deadlines`, `exports`, `games`, `handouts`, `notifications`, `phases`, `polls`, `users`

```go
func (h *Handler) CreateGame(w http.ResponseWriter, r *http.Request) {
    // 1. Get context values (user, correlation ID)
    ctx := r.Context()
    user := middleware.GetUserFromContext(ctx)
    correlationID := middleware.GetCorrelationID(ctx)

    // 2. Parse and validate request.
    //    render.Bind decodes the body, then calls the request type's Bind
    //    method — the only hook that runs after decoding, and so the place
    //    validation belongs. Failures render as 400.
    data := &CreateGameRequest{}
    if err := render.Bind(r, data); err != nil {
        core.WriteError(w, core.ErrInvalidRequest(err, correlationID))
        return
    }

    // 3. Call service layer
    game, err := h.service.CreateGame(ctx, data)
    if err != nil {
        core.WriteError(w, err)
        return
    }

    // 4. Return success response
    core.WriteJSON(w, http.StatusCreated, game)
}
```

**Request validation lives in `Bind`.** Tag the request struct and execute the
tags with `core.ValidateStruct`:

```go
type RenameCharacterRequest struct {
    Name string `json:"name" validate:"required,min=1,max=255"`
}

func (r *RenameCharacterRequest) Bind(req *http.Request) error {
    return core.ValidateStruct(r)
}
```

It trims string fields in place before validating (so `"   "` fails `required`)
and names fields by their JSON key in the error. Rules the tags cannot express —
cross-field constraints, `json.Valid` — stay as explicit checks in `Bind`
alongside it.

A `validate` tag on a struct whose `Bind` returns a bare `nil` enforces nothing,
so wire both up in the same change. And do not rely on the service to reject bad
input: service errors render as a 500 "unexpected error", not the 400 a bad
payload deserves.

### 5. Authentication Pattern

**A single JWT bearer token, valid 7 days, backed by a server-side session.**

Issued by `JWTHandler.CreateToken` (`backend/pkg/auth/jwt.go`). Claims are:

| Claim | Contents |
|---|---|
| `sub` | User ID, stringified via `strconv.Itoa` |
| `session_id` | FK to the `sessions` row |
| `exp` | Issued time + `core.SessionLifetime` (7 days) |

Token creation is two-phase: a temporary token (`sub` + `exp`) is signed to
create the session row, then the final token is re-signed with `session_id`
added and written back to the session.

Because the session is stored server-side, tokens can be revoked by deleting the
session — that, not a short expiry, is the containment mechanism.

**See**: `/docs-site/developer/architecture/adrs/003-authentication-strategy.md`

> **History**: earlier revisions described 15-minute access tokens plus separate
> 7-day refresh tokens and a `sub` holding the *username* — none of which matched
> the code. That fiction traced to unused `JWTConfig.AccessTokenExpiry` /
> `RefreshTokenExpiry` fields, removed in August 2026. Lifetime now lives in one
> place: `core.SessionLifetime`, shared by the token `exp` and the session row.

### 5b. Community Permission Tiers (added 2026-09-02)

Community authority is **two** functions, not one
(`backend/pkg/core/permissions.go:274,289`). Picking the wrong one is a
security bug, not a visible error:

| Function | Grants | Owner | Moderator | Site admin |
|---|---|---|---|---|
| `CanModerateCommunity` | bans, documents, webhooks, profile, banner | ✅ | ✅ | admin mode only |
| `CanAdministerCommunity` | moderator roster **only** | ✅ | ❌ | admin mode only |

**A moderator may do everything except appoint more moderators.** Without that
split, one moderator could grow the roster faster than the owner could prune it,
making ownership advisory. The defining test is
`TestCanAdministerCommunity_ModeratorCannotManageRoster`.

Site admins qualify only with **admin mode enabled**, matching the GM-override
convention (`IsUserGameMasterCtx`) — an admin browsing normally is not one
misclick from a moderation action.

`GetCommunityRole` returns `CommunityRoleNone` on a lookup error: a failed
permission check must never read as elevated access.

**`games.community_id` is nullable on purpose.** Games predating communities
carry `NULL` and must keep working:
- Always `LEFT JOIN communities`, never `INNER JOIN` — an inner join silently
  drops every legacy game from listings, which looks like an empty page rather
  than an error.
- `community_id IS NULL` means no ban can reach the game; there is no community
  whose ban would apply.
- New games require a community, but that is enforced in the **application
  create path**, never as a `NOT NULL` constraint.

**See**: `/docs-site/developer/architecture/adrs/008-community-scoping.md`

### 5c. Detached Background Dispatch (added 2026-09-02)

Work that outlives the request — webhook delivery, notification fan-out — must
**not** close over the request context. It is cancelled the moment the response
is written, so delivery fails in production and passes in every synchronous
test.

```go
// backend/pkg/db/services/webhook_dispatch.go:100
dispatchCtx, cancel := context.WithTimeout(
    observability.WithCorrelationID(context.Background(), observability.GetCorrelationID(ctx)),
    core.WebhookDispatchTimeout,
)
observability.SafeGo(dispatchCtx, gs.Logger, "dispatch-community-webhooks", func() {
    defer cancel()
    gs.deliverStateChangeWebhooks(dispatchCtx, game, newState)
})
```

Three non-negotiable parts:
1. **`context.Background()`**, with the correlation ID carried across so the
   detached work stays traceable.
2. **A timeout**, so a hanging endpoint cannot leak the goroutine.
3. **`SafeGo`, never a bare `go func()`** — an unrecovered panic in any
   goroutine kills the process.

Failure never fails the triggering operation: the state change is the user's
intent, and a third party's outage must not block it. Persistent breakage
surfaces via `last_error`.

`context.WithoutCancel(ctx)` is the equivalent idiom used by the notification
paths (`services/messages/posts.go:68`, `services/phases/transitions.go:320`).

### 6. Error Handling Pattern

**Use typed errors with context**:

```go
// In core/errors.go
type APIError struct {
    Code          string `json:"code"`
    Message       string `json:"message"`
    CorrelationID string `json:"correlation_id,omitempty"`
    Details       any    `json:"details,omitempty"`
}

// Usage in services
if err != nil {
    return nil, core.ErrNotFound("game", gameID, correlationID)
}
```

**Consistent error responses across API**

## Frontend Architecture Patterns

### 1. State Management Strategy

**See**: `.claude/context/STATE_MANAGEMENT.md` for details

- **Server State**: React Query (TanStack Query)
- **Auth State**: Custom AuthContext + React Query
- **UI State**: Component-local useState/useReducer
- **Global Settings**: React Context (sparingly)

**Key Pattern**: Centralized AuthContext eliminates duplicate user fetching

### 2. Component Organization

```
components/
├── ComponentName.tsx          # Component implementation
├── ComponentName.test.tsx     # Component tests (co-located)
├── __tests__/                 # Shared test utilities for components
└── ui/                        # UI component library (Button, Card, Input, etc.)

hooks/
├── useCustomHook.ts          # Custom hooks
├── useCustomHook.test.ts     # Hook tests (co-located)
└── __tests__/                # Shared hook test utilities

pages/
├── PageName.tsx              # Page components
└── __tests__/                # Page tests

contexts/
├── AuthContext.tsx           # Authentication state
├── GameContext.tsx           # Game-specific state and permissions
├── ThemeContext.tsx          # Dark/light mode
├── ToastContext.tsx          # Toast notifications
├── ConversationContext.tsx   # Conversation state
└── AdminModeContext.tsx      # Admin mode state

lib/api/
├── client.ts                 # Axios instance + interceptors
├── auth.ts                   # Auth endpoints
├── games.ts                  # Games endpoints
├── characters.ts             # Characters endpoints
├── messages.ts               # Messages endpoints
├── conversations.ts          # Conversations endpoints
├── phases.ts                 # Phases endpoints
├── polls.ts                  # Polls endpoints
└── ...                       # Other endpoint modules

types/
├── domain.ts                 # Type definitions
└── ...
```

### 3. Tabbed Forms and Native Validation

*Added 2026-08-19, from the game create/edit form tab split.*

When a long form is split into tabs, **render every panel and hide the inactive
ones** with `hidden` / `display: none`. Do not unmount them.

```tsx
<div hidden={!isActive} role="tabpanel" data-testid={`my-form-panel-${id}`}>
```

- **Unmounting breaks validation silently.** A `required` control that is not in
  the document is not validated at all, so submitting from another tab posts an
  empty value with no error shown.
- **Use `hidden`, not `visibility`/opacity.** A laid-out panel still contributes
  its height to the modal's scroll — which is usually the reason for tabs.

**But rendering all panels is not sufficient.** Chromium will not focus a control
inside a `display: none` panel. Measured: a hidden control behaves exactly like a
detached one — `willValidate` is `true` and the `invalid` event fires, but the
console logs *"An invalid form control … is not focusable"* and **the submit does
nothing at all**, with no message.

The fix is to switch to the offending tab *before* the browser reports validity.
`invalid` fires on each failing control before the submit is cancelled, and does
not bubble — so listen in the capture phase via React's `onInvalid` on the form:

```tsx
<form onSubmit={handleSubmit} onInvalid={handleInvalid}>
```

See `useRevealInvalidTab.ts` + `gameFormTabs.ts` (`findFirstInvalidTab`).

**Buttons inside forms need an explicit `type="button"`.** Neither the shared
`ui/Button` nor `TabNavigation`'s tabs set a default `type`, and `<button>`
defaults to `type="submit"` — so a tab click or a confirmation action submits the
enclosing form. Symptom: *"Form submission canceled because the form is not
connected"* in the console, plus a save the user never asked for.

**Tab ids become testids.** `TabNavigation` derives `data-testid="tab-${id}"`, so
a form tab named `info` collides with the game page's own `info` tab and makes
`getByTestId('tab-info')` ambiguous. Namespace nested tab ids (`game-form-info`).

### 4. API Client Pattern

**Location**: `frontend/src/lib/api/` (split into domain modules)

- `client.ts` — Axios instance with JWT interceptors, automatic token refresh on 401
- Domain modules per feature: `auth.ts`, `games.ts`, `characters.ts`, `messages.ts`, etc.
- `index.ts` — re-exports all API functions
- Type-safe API methods with consistent error handling

## Database Design Pattern

**Hybrid Relational-Document Design**

- **Structured data**: Traditional relational tables with foreign keys
- **Flexible data**: JSONB columns for game-specific data (character sheets, game config)
- **Type safety**: sqlc generates Go structs from schema
- **Migrations**: Version-controlled schema evolution

**Example**: Games table
```sql
CREATE TABLE games (
    id SERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT,
    gm_user_id INTEGER REFERENCES users(id),
    state TEXT NOT NULL DEFAULT 'recruitment',
    game_config JSONB DEFAULT '{}'::jsonb,  -- Flexible game settings
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```

### Notification Context vs Related (added 2026-08-24)

Notifications carry **two** reference pairs, and they are not interchangeable:

| Pair | Meaning | Used for |
|------|---------|----------|
| `context_type` / `context_id` | The container a user opens (e.g. a conversation) | Bulk clear: opening the container dismisses every notification pointing at it |
| `related_type` / `related_id` | The specific item that triggered it (e.g. one message) | Previewing the exact item in the dashboard inbox |

**Why both**: a group conversation produces one notification per message. Keying
only by the message means clicking one notification strands the rest; keying only
by the conversation loses the ability to preview which message arrived.

```go
// Private message notification
RelatedType: stringPtr("message"),      RelatedID: &messageID,
ContextType: stringPtr(core.NotificationContextConversation), ContextID: &conversationID,
```

**Clearing rules**:
- `MarkConversationAsRead` clears the conversation's notifications *in the same
  transaction* as the `conversation_reads` upsert — both are facts about one act
  of reading.
- `MarkAsRead` on a single notification also clears its context siblings. A
  notification with a NULL context clears only itself.
- Nothing flows the other way: clearing a notification never writes
  `conversation_reads`, which preserves the jump-to-first-unread anchor.
  Notifications and read-position tracking stay separate systems.

Both columns are nullable and were **not** backfilled, so pre-existing
notifications keep one-at-a-time behaviour. New notification types opt in by
setting the context pair in their `Notify*` helper.

### Character Sheet Storage

The sheet is **five flat tabs**, each one `module_type` in `character_data`.
There is no second level — an earlier design nested sub-tabs under two parent
modules, and code or docs still describing that is stale.

`character_data.module_type` is **not** constrained in the database; the
allowlist lives in application code (`api_data.go`). The `check_module_type`
constraint is on `action_result_character_updates` and covers only the three
stat modules, since a draft update never targets bio or notes.

| `module_type` | Holds | Renameable |
|---|---|---|
| `bio` | Public description (one text row) | No — platform concept |
| `notes` | Private notes (one text row) | No — platform concept |
| `skills` | JSON array of skills | Yes |
| `inventory` | JSON array of items | Yes |
| `numbers` | JSON array of named quantities/tracks | Yes |

Two invariants worth knowing before touching this:

1. **Each stat tab is ONE row holding a JSON array**, not a row per entry. The
   `field_name` equals the `module_type` for skills and numbers (`items` for
   inventory).
2. **`module_type` == React symbol == default label.** That equality is what
   keeps the renaming feature a straight substitution with no mapping table.
   Preserve it when adding a tab.

GM-supplied labels live in `games.character_sheet` (JSONB), stored **sparse** —
only genuine overrides, never defaults. An absent key means "use the frontend's
default", so defaults have exactly one home (`useSheetLabels.ts`) and changing
one later does not silently skip games that already stored it.

Three renames shipped with the refactor and behave differently on purpose:

- `currency` → `numbers` (module_type): a **real migration**, because reads are
  keyed by `module_type` and a missed row renders an empty tab.
- skill `level` → `rank`, number `type` → `name` (keys *inside* the JSON):
  **no migration**, resolved on read via `skillRank()` / `numberEntryName()`. A
  fallback covers every old row, archived payload, and rolled-back deploy at no
  coordination cost, where a migration would need all three to line up.

**See**: `/docs-site/developer/architecture/adrs/002-database-design-approach.md`

## Observability Pattern

**Structured JSON logging with correlation IDs**

```go
// Generate correlation ID in middleware
correlationID := uuid.New().String()
ctx = context.WithValue(ctx, middleware.CorrelationIDKey, correlationID)

// Use in logging
log.Info().
    Str("correlation_id", correlationID).
    Str("user_id", userID).
    Str("action", "create_game").
    Msg("Game created successfully")
```

**See**: `/docs-site/developer/architecture/adrs/006-observability-approach.md`

## API Design Principles

**RESTful design with `/api/v1/` versioning**

- **Standard HTTP status codes** (200, 201, 400, 401, 404, 500)
- **Structured error responses** with correlation IDs
- **Input validation** at handler layer
- **Rate limiting** on sensitive endpoints

**See**: `/docs-site/developer/architecture/adrs/004-api-design-principles.md`

## Key Implementation Files

**Backend Core**:
- `backend/pkg/core/interfaces.go` - All service contracts
- `backend/pkg/core/*.go` - Business entities, split per domain (games, characters, phases, ...)
- `backend/pkg/core/errors.go` - Error types
- `backend/pkg/http/root.go` - API routing and middleware

**Backend Services**:
- `backend/pkg/db/services/` - Service implementations
  - `phases/` - Phase service (service, crud, transitions, validation, history, scheduler)
  - `actions/` - Action submission service (service, submissions, results, validation, queries,
    draft_updates, staged, staged_worker)
  - `messages/` - Message service (service, posts, draft_posts, comments, reactions, validation,
    read_tracking, audience, character_messages)
  - `*.go` - Other services (games, characters, users, sessions, notifications, conversations, handouts, dashboard, deadlines, polls, user_preferences)
- `backend/pkg/db/queries/*.sql` - SQL queries (generates models/)
- `backend/pkg/db/migrations/*.sql` - Database migrations

**Frontend Core**:
- `frontend/src/lib/api/` - API client (split into domain modules; `index.ts` re-exports all)
- `frontend/src/contexts/` - React contexts:
  - `AuthContext.tsx` - Authentication state
  - `GameContext.tsx` - Game state + permissions
  - `ThemeContext.tsx` - Light/dark/system theme
  - `ToastContext.tsx`, `UtilityDrawerContext.tsx`, `ConversationContext.tsx`,
    `AdminModeContext.tsx`, `ScreenshotModeContext.tsx`
- `frontend/src/App.tsx` - Application setup

## Development Workflow

### Integrated Feature Development

**Implement BOTH backend and frontend together before moving to next feature**

1. **Backend**:
   - Database migration (if needed)
   - SQL queries (sqlc)
   - Service interface definition
   - Write unit tests first (TDD)
   - Service implementation
   - Handler implementation
   - Write API endpoint tests
   - Run tests: `just test`

2. **Frontend**:
   - API client method
   - Custom hooks
   - Write hook tests
   - Components
   - Write component tests
   - Run tests: `just test-fe run`

3. **Manual Testing**: Test complete feature in UI before moving on

4. **Documentation**: Update API docs and relevant guides

### Bug Fix Workflow

**MANDATORY**: Add regression test before fixing

1. Write test that reproduces bug (should fail)
2. Fix the bug
3. Verify test passes
4. Commit test and fix together

## Configuration Management

**Environment variables in `.env`**:
- `DATABASE_URL` - PostgreSQL connection
- `JWT_SECRET` - JWT signing secret
- `ENVIRONMENT` - development/staging/production
- `LOG_LEVEL` - debug/info/warn/error
- `SKIP_DB_TESTS` - Skip database tests if "true"

**Validation**: All env vars validated at startup

## References

### Architecture Decision Records
**Location**: `/docs-site/developer/architecture/adrs/`
- ADR-001: Technology Stack Selection
- ADR-002: Database Design Approach
- ADR-003: Authentication Strategy
- ADR-004: API Design Principles
- ADR-005: Frontend State Management
- ADR-006: Observability Approach
- ADR-007: Testing Strategy
- ADR-008: Community Scoping, Grandfathered Games, and Best-Effort Webhooks

### System Design
**Location**: `/docs-site/developer/architecture/`
- `overview.md` - High-level system design
- `components.md` - How components communicate

### Detailed Guides
- `.claude/reference/BACKEND_ARCHITECTURE.md`
- `.claude/reference/API_DOCUMENTATION.md`
- `.claude/reference/ERROR_HANDLING.md`
- `.claude/reference/LOGGING_STANDARDS.md`

## Quick Checklist Before Implementation

- [ ] Read relevant ADRs for architectural context
- [ ] Define service interface in `core/interfaces.go`
- [ ] Write tests first (TDD approach)
- [ ] Follow established patterns (see examples above)
- [ ] Add correlation IDs for observability
- [ ] Validate inputs at handler layer
- [ ] Handle errors with typed error responses
- [ ] Update documentation
