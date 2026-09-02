# ADR-008: Community Scoping, Grandfathered Games, and Best-Effort Webhooks

## Status
Accepted

## Context

ActionPhase began as a single undifferentiated space: every game sat in one
global pool, and the only moderation lever was site administration. That stops
working once distinct groups run games on the same instance. A group needs to
publish its own rules, keep its own moderator roster, and — the actual driver —
exclude a disruptive player from *its* games without that person being banned
from the platform.

Three decisions had to be made together, because each constrains the others:

1. **How games associate with a group**, given thousands of existing games that
   predate the concept entirely.
2. **Who may moderate**, given that site admins already have a global override
   and communities introduce a second, narrower authority.
3. **How a community learns about game events** in an external system (Discord),
   given that delivery is a network call in the middle of a state transition.

## Decision

### 1. Communities own games through a nullable `games.community_id`

New games require a community, enforced in the application create path. The
column itself stays **nullable**, and legacy rows keep `NULL` forever.

```sql
-- backend/pkg/db/migrations/20260831172428_create_communities.up.sql
ALTER TABLE games ADD COLUMN community_id INTEGER
    REFERENCES communities(id) ON DELETE RESTRICT;
```

The requirement lives in the write path rather than in a `NOT NULL` constraint.
This is the crux of the grandfathering decision: a `NOT NULL` column would have
required backfilling every pre-existing game into some invented "default"
community, inventing an ownership claim that was never made and handing that
community's moderators authority over games they had nothing to do with.

`ON DELETE RESTRICT`, not `CASCADE`: deleting a community must never delete the
games inside it.

Two consequences follow and are load-bearing everywhere downstream:

- **`community_id IS NULL` means "no community", which means "no ban can reach
  it."** Ban enforcement treats a legacy game as never blocked, because there is
  no community whose ban could apply.
- **Every read path joins with `LEFT JOIN`, never `INNER JOIN`.** An inner join
  would silently drop every legacy game from listings — a data-loss bug that
  looks like an empty page rather than an error.

The fixtures keep a permanent legacy game with no community
(`backend/pkg/db/test_fixtures/e2e/30_legacy_no_community.sql`). It is the
regression guard: it exists to fail loudly the day someone adds `NOT NULL` or
tightens a join.

### 2. Two permission tiers, with site admin gated behind admin mode

Community authority splits in two (`backend/pkg/core/permissions.go:274,289`):

| Capability | Owner | Moderator | Site admin (admin mode on) |
|---|---|---|---|
| Bans, documents, webhooks, profile, banner | ✅ | ✅ | ✅ |
| Add/remove moderators | ✅ | ❌ | ✅ |

`CanModerateCommunity` covers ordinary upkeep. `CanAdministerCommunity` covers
only the moderator roster.

Moderators cannot appoint further moderators. Without that split, one moderator
could expand the roster indefinitely, and the owner's authority would be
advisory rather than real — a moderator could appoint allies faster than the
owner could remove them.

Site admins qualify for both tiers **only with admin mode enabled**, matching
the existing GM-override convention. Admin mode is an explicit, deliberate act,
which keeps an admin from moderating a community by accident while browsing
normally.

`GetCommunityRole` returns `CommunityRoleNone` when the lookup errors. A failed
permission check must never read as elevated access.

### 3. Discord webhooks are best-effort, with no durable queue

Webhook delivery is fire-and-forget, detached from the request
(`backend/pkg/db/services/webhook_dispatch.go:100`):

```go
dispatchCtx, cancel := context.WithTimeout(
    observability.WithCorrelationID(context.Background(), observability.GetCorrelationID(ctx)),
    core.WebhookDispatchTimeout,
)
```

Three properties of that line are deliberate:

- **`context.Background()`, not the request context.** A goroutine closing over
  the request context is cancelled the instant the HTTP response is written, so
  delivery would fail exactly in production and pass in every synchronous test.
  This is the single most important detail in the dispatch path.
- **The correlation ID is carried forward** so the detached work stays traceable
  back to the request that caused it.
- **`SafeGo`, not a bare `go func()`.** An unrecovered panic in any goroutine
  takes down the whole process.

Delivery failure never fails the state transition. The game state change is the
user's actual intent; a Discord post is a side effect, and a Discord outage must
not prevent a GM from advancing a phase. Persistent breakage surfaces through
the `last_error` column rather than through a failed request.

**We accept that a webhook can be lost** — on restart, or after retries are
exhausted. A durable queue was rejected: it introduces a broker, worker
lifecycle, and dead-letter handling to make an *announcement* reliable. The
failure mode we actually care about is a webhook that is permanently misconfigured,
and `last_error` catches that. A single missed notification during a deploy does
not warrant that infrastructure.

The webhook URL is treated as a **credential**: never returned unmasked by the
API, never logged, and validated against host, scheme, and path shape at both
save and dispatch time to close off SSRF.

## Alternatives Considered

### Backfill every game into a default community (rejected)
Would have permitted `NOT NULL` and removed the nullable-column special cases.
Rejected because it fabricates ownership: a real community would gain moderation
authority over historical games whose participants never joined it. The
nullable column keeps "this game predates communities" representable, which is
the truth.

### Single community permission tier (rejected)
Simpler, but makes owner and moderator indistinguishable and lets any moderator
grow the roster without limit. See §2.

### Site admins moderate communities unconditionally (rejected)
Rejected in favor of requiring admin mode, matching the GM override. An admin
browsing normally should not be one misclick from a moderation action.

### Durable webhook queue (rejected)
See §3. Correct for payments; disproportionate for announcements.

### Synchronous webhook delivery (rejected)
Would make a slow or hanging Discord endpoint delay — or fail — a game state
transition. The transition is the user's intent and must not depend on a third
party's availability.

## Consequences

**Positive**
- Groups self-moderate without site-admin involvement, the stated driver.
- Legacy games keep working untouched; no backfill, no invented ownership.
- Bans are scoped: a ban in one community has no effect in another.
- A Discord outage cannot block gameplay.

**Negative**
- `community_id IS NULL` is a permanent special case every new query must
  handle. Mitigated by the migration comment, the permanent fixture, and
  regression tests.
- Webhook delivery is not guaranteed. Accepted; `last_error` covers the case
  that matters.
- Two permission helpers rather than one, and choosing the wrong one is a
  security bug rather than a visible error. Mitigated by the defining test,
  `TestCanAdministerCommunity_ModeratorCannotManageRoster`
  (`backend/pkg/core/community_permissions_test.go:117`): a moderator must not
  be able to add a moderator.

## Related

- `.claude/context/ARCHITECTURE.md` — community permission tier, detached
  dispatch pattern
- `backend/pkg/core/permissions.go` — `GetCommunityRole`,
  `CanModerateCommunity`, `CanAdministerCommunity`
- `backend/pkg/db/services/webhook_dispatch.go` — detached dispatch
- `backend/pkg/db/migrations/20260831172428_create_communities.up.sql` — the
  nullable-column rationale, stated at the point of definition
- ADR-003 (Authentication Strategy) — the admin-mode convention this reuses
