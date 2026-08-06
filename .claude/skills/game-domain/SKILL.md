---
name: game-domain
description: Complete game lifecycle and domain patterns for ActionPhase turn-based gaming platform. Covers game states, phase transitions (common room and action phases), character workflows (creation, approval, death), messaging systems (common room posts, private messages, action results), action submissions, public archive mode for completed games, and the complete flow from recruitment through game completion. Use when working with game state management, phase advancement, character status, audience access, or understanding game mechanics.
---

# ActionPhase Game Domain

## Purpose

Comprehensive guide to the game lifecycle, state management, phase transitions, and player workflows in ActionPhase's turn-based gaming system.

## When to Use This Skill

- Understanding game flow and state transitions
- Implementing phase management (common room and action phases)
- Working with character workflows and approvals
- Building messaging systems (posts, comments, private messages)
- Managing action submissions and action results
- Reasoning about visibility, audience access, and public archive mode
- Debugging game state issues
- Designing features that interact with game mechanics

---

## Game Lifecycle Overview

### Complete Game Flow

```
1. SETUP → 2. RECRUITMENT → 3. CHARACTER_CREATION → 4. IN_PROGRESS → 5. COMPLETED/CANCELLED
```

**IN_PROGRESS** contains the repeating phase cycle:
```
COMMON ROOM (discussion) → ACTION (submissions + GM results) → [repeat]
INTERLUDE (private messaging only) may occur between cycles
```

⚠️ **There are THREE phase types: `common_room`, `action`, and `interlude`.**
There is no `results` phase — GM results are `action_results` rows attached to
an *action* phase. See [phase-system.md](resources/phase-system.md).

**See**: [game-states.md](resources/game-states.md) for complete state definitions and transition rules

---

## Quick Reference

### Game States

```sql
state CHECK (state IN (
    'setup',              -- Initial creation
    'recruitment',        -- Accepting players
    'character_creation', -- Players creating characters
    'in_progress',        -- Active gameplay
    'paused',            -- Temporarily suspended
    'completed',         -- Finished normally
    'cancelled'          -- Terminated early
))
```

**See**: [game-states.md](resources/game-states.md)

### Phase Types

```sql
-- Live constraint (migration 20260605174513); see warning below
CHECK (phase_type IN (
    'common_room',  -- Discussion/planning phase
    'action',       -- Players submit actions; GM publishes results here
    'interlude'     -- Private messaging only: no public posts, no submissions
))
```

Canonical list: `core.ValidPhaseTypes` (`backend/pkg/core/constants.go:43`).

Results live in the `action_results` table (FK to `game_phases` + `action_submissions`),
not in a phase of their own.

⚠️ **`backend/pkg/db/schema.sql` is STALE** — it still shows the two-type
constraint and omits `interlude`. sqlc generates from it, but the live database
is defined by `backend/pkg/db/migrations/`. **Trust the migrations and
`core/constants.go`, not `schema.sql`.**

**See**: [phase-system.md](resources/phase-system.md)

### Character Status

```sql
-- backend/pkg/db/schema.sql — NOTE: no CHECK constraint; values are enforced in code
status VARCHAR(50) DEFAULT 'pending'
```

Values actually used in code: `pending` (awaiting GM review), `approved` (GM
approved), `rejected` (GM rejected; player revises and resubmits).

⚠️ **There is no `dead` status.** Character removal from play is modeled by the
separate `characters.is_active` boolean, and the player-side transition is
`POST /api/v1/games/{gameID}/participants/{userId}/to-audience` (permadeath —
moves the *player* to the audience role). Inactive characters are listed via
`GET /api/v1/games/{gameID}/characters/inactive` and can be reassigned with
`PUT /api/v1/characters/{id}/reassign`.

**See**: [character-workflows.md](resources/character-workflows.md)

---

## Core Concepts

### 1. Game State Machine

Games transition through defined states with specific rules for advancement.

- **SETUP**: GM configures game
- **RECRUITMENT**: Players apply to join
- **CHARACTER_CREATION**: Players create characters, GM approves
- **IN_PROGRESS**: Active gameplay with phase cycles
- **COMPLETED**: Game ends normally, archive becomes public
- **CANCELLED**: Game terminates early

**Details**: [game-states.md](resources/game-states.md)

### 2. Phase Cycle (The Game Loop)

The repeating cycle during IN_PROGRESS state:

1. **COMMON ROOM**: Public discussion forum, planning, coordination
2. **ACTION**: Private action submissions, one per player; the GM writes and
   publishes `action_results` against this same phase

Phases are **created and activated**, not "advanced" through a dedicated
endpoint. A phase becomes live either when the GM activates it or when the
background scheduler reaches its `start_time`
(`RunScheduledActivations`, `backend/pkg/db/services/phases/scheduler.go`).

**Details**: [phase-system.md](resources/phase-system.md)

### 3. Character Lifecycle

Players create characters that go through approval workflow:

**PENDING** → **APPROVED** (GM approves; character plays)

Or: **PENDING** → **REJECTED** (player revises and resubmits)

Removal from play is `is_active = false` (not a status value); the player may be
moved to the audience role via the permadeath transition.

**Details**: [character-workflows.md](resources/character-workflows.md)

### 4. Messaging System

Message content lives in two places:

- **`messages` table** — one table for common room content, discriminated by
  `message_type` (`post` / `comment` / `private_message`) and `visibility`
  (`game` / `private`). Comments nest via self-referential `parent_id` with a
  denormalized `thread_depth`; threads can be **very** deep.
- **`conversations` + `private_messages`** — character-to-character direct and
  group conversations.

Plus **`action_submissions`** (one per player per phase; hidden from other
players) and **`action_results`** (GM → player, gated by `is_published`).

**Details**: [messaging-system.md](resources/messaging-system.md)

### 5. Visibility & Public Archive Mode

⚠️ **The single most misunderstood part of this domain.** Once a game reaches
`completed`, **`CanUserViewGame` returns true for ANY authenticated user**
(`backend/pkg/db/services/games.go:1029`). Completed games are a public archive.

Audience members and GMs can read **all** private conversations and **all**
action submissions in a game via dedicated queries that apply no participant
filter — `ListAllPrivateConversations`, `GetAudienceConversationMessages`
(`pkg/db/queries/messages.sql`), and `ListAllActionSubmissions`
(`pkg/db/queries/phases.sql`).

So private messages and results are **not** permanently private. Completion
also lifts two play-time restrictions:

- **Poll vote attribution** — every authenticated user gets
  `canSeeIndividualVotes` regardless of the poll's `show_individual_votes`
  (`checkPollViewAccess`, `backend/pkg/polls/api_polls.go`). But
  `hide_results_from_players` is **not** lifted: only GM/co-GM/audience
  (`isPrivileged`) ever see those polls.
- **Anonymous games** — usernames are disclosed to everyone once completed
  (`CanSeeUsernamesInAnonymousGame`, `backend/pkg/core/permissions.go`).

What *does* stay hidden: drafts (`is_draft`) and soft-deleted content
(`is_deleted` / `deleted_at`). Cancelled games are **not** public and follow
normal permission rules, keeping both restrictions above.

**Always reuse `CanUserViewGame` for read authorization rather than
hand-rolling a participant check.**

---

## Data Models

### Core Tables

**games**
- Game metadata, state, GM
- Recruitment settings
- Date tracking

**game_participants**
- Players, co-GMs, audience
- Role and status tracking

**characters**
- Player characters and NPCs
- Type and status workflow

**game_phases**
- Phase history and current phase
- Type, number, timing

**messages**
- Common room posts AND comments AND private messages in ONE table
- Discriminated by `message_type` + `visibility`
- Self-referential `parent_id` for nested comment trees (+ `thread_depth`)
- ⚠️ There are no separate `posts` / `comments` tables

**conversations** + **conversation_participants** + **private_messages**
- Character-to-character direct and group conversations
- Note: `conversations` has NO `phase_id` — they span phases

**action_submissions**
- Player actions during an action phase (`UNIQUE(game_id, user_id, phase_id)`)

**action_results**
- GM results attached to an action phase and submission; gated by `is_published`

**See**: [data-models.md](resources/data-models.md) for complete schema

---

## API Endpoints

### Game Management
```
GET/POST/PUT/DELETE  /api/v1/games
PUT                  /api/v1/games/{id}/state
```

⚠️ Routes below are transcribed from `backend/pkg/http/root.go`. Most game
content is nested under `/api/v1/games/{gameID}/...` — there are **no**
top-level `/api/v1/phases/...` routes.

### Characters
```
GET/POST  /api/v1/games/{gameID}/characters
GET       /api/v1/games/{gameID}/characters/inactive
POST      /api/v1/characters/{id}/approve     -- body carries "approved"|"rejected"
PUT       /api/v1/characters/{id}/reassign
PUT       /api/v1/characters/{id}/rename
```
(There is no `POST /characters/{id}/reject`; rejection goes through `/approve`.)

### Phases
```
GET   /api/v1/games/{gameID}/phases
POST  /api/v1/games/{gameID}/phases
GET   /api/v1/games/{gameID}/current-phase
```
(There is no `POST /api/v1/phases/advance`.)

### Messaging (Common Room)
```
GET   /api/v1/games/{gameID}/posts
GET   /api/v1/games/{gameID}/posts/{postId}/comments
GET   /api/v1/games/{gameID}/posts/{postId}/comments-with-threads  -- paginated + nested
GET   /api/v1/games/{gameID}/messages/{messageId}/thread-context   -- ancestor chain
GET   /api/v1/games/{gameID}/comments/recent
```

### Actions & Results
```
POST/GET  /api/v1/games/{gameID}/actions
GET       /api/v1/games/{gameID}/actions/mine
POST/GET  /api/v1/games/{gameID}/results
GET       /api/v1/games/{gameID}/results/mine
POST      /api/v1/games/{gameID}/results/{resultId}/publish
POST      /api/v1/games/{gameID}/phases/{phaseId}/results/publish   -- bulk
```

### Audience / Archive Access
```
GET  /api/v1/games/{gameID}/private-messages/all
GET  /api/v1/games/{gameID}/private-messages/conversations/{conversationId}
GET  /api/v1/games/{gameID}/action-submissions/all
```

**See**: [api-reference.md](resources/api-reference.md) for complete API documentation

---

## Business Rules

### Key Constraints

- ✅ Only ONE active phase per game
- ✅ One action submission per player per phase (`UNIQUE(game_id, user_id, phase_id)`)
- ✅ Only the GM/co-GM creates and activates phases (or the scheduler, by `start_time`)
- ✅ Must have an active character to post/act
- ✅ Action results are hidden from the player until `is_published`
- ✅ Drafts (`is_draft`) are visible only to their author
- ✅ Soft-deleted content (`is_deleted` / `deleted_at`) is hidden from normal reads
- ✅ Completed games are a PUBLIC ARCHIVE readable by any authenticated user
- ✅ Audience and GM can read ALL private conversations and ALL action submissions
- ❌ NOT TRUE: "private messages are never visible to the GM"
- ❌ NOT TRUE: "results remain private forever"

**See**: [business-rules.md](resources/business-rules.md) for complete rules

---

## Common Workflows

### Workflow 1: Complete Game Setup to First Phase

```bash
# 1. Create game → 2. Open recruitment → 3. Move to character creation
# 4. Approve characters → 5. Start game → 6. Create first phase

# Full example in resources
```

### Workflow 2: Phase Cycle

```bash
# Common Room → ACTION (submissions + published results) → (new Common Room)
# GM creates the next phase and activates it, or sets start_time and lets
# the scheduler activate it.

# Full example in resources
```

### Workflow 3: Character Removal and Replacement

```bash
# Set characters.is_active = false → optionally move player to audience
# (permadeath) → player creates new character → GM approves

# Full example in resources
```

**See**: [workflows.md](resources/workflows.md) for complete workflow examples with curl commands

---

## Testing Patterns

### Test Complete Phase Cycle

```bash
# Load test fixtures
./backend/pkg/db/test_fixtures/apply_e2e.sh

# Login as GM and create the next phase
./backend/scripts/api-test.sh login-gm
curl -X POST -H "Authorization: Bearer $(cat /tmp/api-token.txt)" \
  -H "Content-Type: application/json" \
  http://localhost:3000/api/v1/games/164/phases \
  -d '{"phase_type": "action", "title": "Descent"}'

# Login as player and submit action
./backend/scripts/api-test.sh login-player
curl -X POST -H "Authorization: Bearer $(cat /tmp/api-token.txt)" \
  -H "Content-Type: application/json" \
  http://localhost:3000/api/v1/games/164/actions \
  -d '{"content": "I investigate the library"}'
```

**See**: [testing-guide.md](resources/testing-guide.md) for complete testing patterns

---

## Database Queries

### Understanding Game State

```sql
-- Get game with current phase
SELECT g.id, g.title, g.state, gp.phase_type, gp.is_active
FROM games g
LEFT JOIN game_phases gp ON gp.game_id = g.id AND gp.is_active = true
WHERE g.id = 164;

-- Get phase history
SELECT phase_number, phase_type, start_time, end_time
FROM game_phases
WHERE game_id = 164
ORDER BY phase_number;

-- Get action submissions for phase
SELECT u.username, c.name, a.submitted_at
FROM action_submissions a
JOIN users u ON a.user_id = u.id
LEFT JOIN characters c ON a.character_id = c.id
WHERE a.phase_id = 456;
```

**See**: [database-queries.md](resources/database-queries.md) for complete query reference

---

## Navigation Guide

| Need to... | Read this |
|------------|-----------|
| Understand game states and transitions | [game-states.md](resources/game-states.md) |
| Learn phase cycle (common room → action) | [phase-system.md](resources/phase-system.md) |
| Understand character approval workflow | [character-workflows.md](resources/character-workflows.md) |
| Learn messaging types and visibility | [messaging-system.md](resources/messaging-system.md) |
| See complete data model | [data-models.md](resources/data-models.md) |
| Find API endpoints | [api-reference.md](resources/api-reference.md) |
| Understand business rules and constraints | [business-rules.md](resources/business-rules.md) |
| See workflow examples with code | [workflows.md](resources/workflows.md) |
| Learn testing patterns | [testing-guide.md](resources/testing-guide.md) |
| Find useful database queries | [database-queries.md](resources/database-queries.md) |

---

## Design Principles

### Privacy Is Scoped to the ACTIVE Game, Not Permanent
- Drafts are visible only to their author (`is_draft`)
- Action results are hidden from the player until `is_published`
- Soft-deleted content is hidden from normal reads
- **But**: audience members and GMs can read all private conversations and all
  action submissions, and a `completed` game is readable by any authenticated
  user. Privacy here means "not visible to other *players* mid-game" — it does
  not mean "never disclosed."

### Asynchronous by Default
- No real-time requirements
- Players work at their own pace
- Deadlines provide structure but no hard enforcement
- Email notifications for phase changes

### GM Control
- GM controls all phase advancement
- GM decides character approval
- GM manages game state
- GM sets deadlines and pacing

### Public Archive
- Completed games become readable by ANY authenticated user
  (`CanUserViewGame`, `backend/pkg/db/services/games.go:1029`)
- Action submissions, private conversations, and published results are all
  part of that archive
- Cancelled games are NOT public — they follow normal permission rules
- Creates valuable reference material

---

## Related Skills & Context

### Skills
- **backend-dev-guidelines** - Implementing game domain services
- **route-tester** - Testing game API endpoints
- **frontend-dev-guidelines** - Building game UI components
- **game-phase-management** - Phase service implementation details
- **character-management** - Character-specific patterns
- **messaging-system** - Messaging implementation

### Context Files
- `.claude/context/ARCHITECTURE.md` - Service patterns
- `.claude/context/TEST_DATA.md` - Test fixtures with game data
- `.claude/context/TESTING.md` - Testing game workflows
- `.claude/reference/API_DOCUMENTATION.md` - Complete API reference

### Key Backend Files
- `backend/pkg/db/services/phases/` - Phase management service
- `backend/pkg/db/services/games.go` - Game state management
- `backend/pkg/db/services/characters.go` - Character workflow
- `backend/pkg/db/services/messages/` - Messaging services
- `backend/pkg/db/services/actions/` - Action submission service

---

## Quick Start

**New to ActionPhase game domain?** Read in this order:

1. [game-states.md](resources/game-states.md) - Understand the lifecycle
2. [phase-system.md](resources/phase-system.md) - Learn the phase cycle
3. [character-workflows.md](resources/character-workflows.md) - Character approval flow
4. [messaging-system.md](resources/messaging-system.md) - Message types and visibility
5. [workflows.md](resources/workflows.md) - See complete examples

**Implementing a feature?** Check:
- [api-reference.md](resources/api-reference.md) - Find the right endpoints
- [business-rules.md](resources/business-rules.md) - Understand constraints
- [data-models.md](resources/data-models.md) - Database schema

**Debugging?** Use:
- [database-queries.md](resources/database-queries.md) - Query game state
- [testing-guide.md](resources/testing-guide.md) - Test workflows

---

**Skill Status**: COMPLETE ✅
**Line Count**: < 500 ✅
**Progressive Disclosure**: 10 resource files ✅
**Coverage**: Full game lifecycle, state management, phase system, character workflow, messaging ✅
