# Codebase Value Plan — May 2026

High-value work prioritized for remaining Claude subscription time.
No active features/bugs; goal is lasting quality improvement.

> **Status re-verified 2026-08-27.** Items 1–5 are complete. Only item 6
> remains open. Statuses below are updated in place.

---

## Tier 1: High Impact, Low Risk

### 1. Dashboard handler tests ✅ ALREADY DONE
`backend/pkg/dashboard/dashboard_integration_test.go` exists with 5 test functions covering:
- Empty dashboard for new user
- Dashboard with player game
- Unauthorized without token
- Invalid session ID
- GM game response shape
- Urgent game with deadline and pending action

No work needed here.

### 2. Resolve deprecated `User()` method callers → `GetUserByID()`
**Status**: ✅ DONE — the deprecated `User()` method is gone from `users.go`
and there are zero non-test callers remaining.

`UserService.User()` is deprecated in favor of `GetUserByID()`.
Active callers (non-test):
- `backend/pkg/core/handler_utils.go:70`
- `backend/pkg/messages/api.go:1193`
- `backend/pkg/handouts/api_handouts.go:21`
- `backend/pkg/auth/api.go:66,129,180`
- `backend/pkg/polls/api_polls.go:148`
- `backend/pkg/deadlines/api_deadlines.go:357`
- `backend/pkg/games/api_participants.go:241`

Plan: Update all callers, then remove the deprecated `User()` method from `users.go`.

### 3. Remove deprecated `useAuthLegacy` barrel export
**Status**: ✅ DONE — no longer exported from `frontend/src/hooks/index.ts`.

`frontend/src/hooks/index.ts` exports `useAuth as useAuthLegacy`.
No consumer imports `useAuthLegacy` — the two test files that use `useAuth`
import directly from `hooks/useAuth`, not the barrel.

Plan: Remove the export line from `hooks/index.ts`.

---

## Tier 2: Meaningful but Heavier

### 4. Resolve 4 service migration TODOs in phases package
**Status**: ✅ DONE — the migration TODOs are no longer present in
`phases/api_results.go` or `phases/api_actions.go`.

TODOs in `phases/api_results.go` and `phases/api_actions.go` indicate
methods should be migrated to the `actions` package but weren't completed.
- `GetUserResults` / `GetGameResults` → actions package
- `GetUserActions` → actions package

### 5. Decompose `messages/api.go` (1,524 lines)
**Status**: ✅ DONE — `api.go` no longer exists. The package is now split into
`api_posts.go`, `api_read_tracking.go`, `api_recent.go`, `api_draft_posts.go`,
`api_types.go`, and `render_error.go`.

The handler was not decomposed when the service was. Natural splits:
- `api_posts.go` — post CRUD
- `api_reactions.go` — reactions
- `api_read_tracking.go` — read tracking
- `api_audience.go` — audience management
- `api_character_messages.go` — character-specific messages

Pattern already exists in `db/services/messages/`.

### 6. Replace `fmt.Errorf` with typed errors in the messages package
**Status**: 🔄 OPEN — the only item still outstanding.

**26 instances** remain (re-counted 2026-08-27), now spread across the
decomposed files rather than a single `api.go`:

| File | `fmt.Errorf` |
|---|---|
| `api_posts.go` | 13 |
| `api_read_tracking.go` | 6 |
| `api_recent.go` | 5 |
| `api_draft_posts.go` | 1 |
| `api_types.go` | 1 |

Replace with `core.ErrXxx()` constants.

---

## Tier 3: Skip

- **Frontend UI component tests** (Button, Card, Badge): No business logic — testing that a button renders is tautological.
- **TypeScript `any` elimination**: Risky without context on each site's intent.
- **`root.go` decomposition**: Works fine, no clear architectural gain.
