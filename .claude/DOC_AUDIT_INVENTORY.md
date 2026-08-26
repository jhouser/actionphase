# Dev Documentation Audit — Inventory

**Created:** 2026-08-26
**Goal:** For each doc, assess accuracy against the actual state of the codebase and bring it back up to date.
**Scope:** Internal/dev-facing docs only. 152 tracked `.md` files (excludes `frontend/dist/`, `node_modules/`).

**Progress:** Batches 1 and 2 complete (26 docs). Next up: **Batch 3** — skills.
One doc remains PENDING in Batch 2: `.claude/planning-doc.md`.

## How to use this file

Work batch by batch. For each doc, set **Status** to one of:

- `PENDING` — not yet reviewed
- `OK` — verified accurate, no changes needed
- `UPDATED` — corrected against the codebase
- `DELETE` — recommend removal (stale, superseded, or point-in-time artifact)
- `MERGE→<path>` — content should fold into another doc

Record findings in the Notes column. Update this file at the end of each session so the next session can resume cleanly.

## Priority rationale

`Last Commit` is the strongest staleness signal available. The codebase has moved
substantially since late 2025 (containerized dev stack, test DB isolation, phase
semantics, Faro observability, epilogue game state). Docs last touched in
**Oct–Nov 2025** are the highest risk and are batched first.

---

## Batch 1 — Core AI context (highest traffic, read before most tasks)

These are loaded most often and cause the most damage when wrong.

| Status | Doc | Lines | Last Commit | Notes |
|---|---|---|---|---|
| UPDATED | `CLAUDE.md` | 699 | 2026-08-18 | Fixed: `just test-frontend`→`test-fe run` (3x); `just migrate_test` (nonexistent)→`reset-test-db`; `npx playwright` on host→`just e2e-*`; redundant `SKIP_DB_TESTS=false just test`; stale paths `core/models.go` and `lib/api.ts` (both split up). Added per-package test-DB cloning note. All path refs now resolve. |
| UPDATED | `.claude/README.md` | 150 | 2026-05-07 | Added missing `/skills/`, `/agents/`, `/hooks/` sections (were entirely absent); completed `/commands/` list (3→10); noted `/planning/` is gitignored; containerized the E2E pre-flight checklist (was host `go test`/`npm test`/`npx playwright`); softened the widely-violated <500-line rule. |
| UPDATED | `.claude/context/ARCHITECTURE.md` | 547 | 2026-08-25 | **Major auth correction** — doc claimed 15-min access + 7-day refresh tokens and `sub`=username; actual is ONE 7-day session-backed token with `sub`=user ID + `session_id`. Also: Go 1.21→1.25, React 18→19, Tailwind→v4 CSS-first (no `tailwind.config.js`), Vite 7. Fixed handler pkg list (+`exports`, −`messages`), service subpackage files, contexts list, `db/` tree, `models.go`→split. |
| UPDATED | `.claude/context/TESTING.md` | 334 | 2026-08-19 | Replaced host `go test -p=1` invocations (obsolete flag — packages now clone their own DB and run parallel) with `just test-coverage` / `just test-run` / `just sh backend`; `test-frontend`→`test-fe run`. E2E spec count 41→49 and noted they live in subdirectories. Coverage *philosophy* section was already sound — no hardcoded percentages to purge. |
| UPDATED | `.claude/context/TEST_DATA.md` | 522 | 2026-08-18 | Only defect: `just migrate_test` in the schema-drift runbook. Replaced with `just reset-test-db` + explanation that a stale `actionphase_test_template` propagates to every package. `docker ps`→`just ps`. Fixture inventory otherwise matched `test_fixtures/`. |
| OK | `.claude/context/STATE_MANAGEMENT.md` | 331 | 2026-08-19 | No defects found. All path refs resolve, no stale commands, no obsolete API-client references. |
| UPDATED | `.claude/context/FRONTEND_STYLING.md` | 870 | 2026-05-07 | Documented 4 exported-but-undocumented UI components (`Modal`, `Drawer`, `HelpTooltip`, `MetadataItem`) as §13–16. Expanded semantic token reference from 9 → all 19 tokens actually declared in `@theme`. Confirmed no Tailwind v3 config assumptions. |

### Batch 1 findings (completed 2026-08-26)

**6 of 7 docs had defects; 1 was clean.** Highest-impact discoveries:

1. **`ARCHITECTURE.md` described an authentication system that does not exist.**
   It documented 15-minute access tokens plus separate 7-day refresh tokens, with
   `sub` holding the *username*. The code (`backend/pkg/auth/jwt.go`) issues a
   **single 7-day token** carrying `sub` (user **ID**, stringified) and
   `session_id`, backed by a server-side session row — revocation is by deleting
   the session, not by short expiry. Anyone reasoning about auth from this doc
   would have been wrong about the threat model. Corrected, with a note recording
   the prior claim so the discrepancy isn't silently re-introduced.

2. **Dependency versions were a full major release behind** across the board:
   Go 1.21→**1.25**, React 18→**19**, Vite→**7**, and Tailwind→**v4**. The
   Tailwind jump matters most: v4 uses CSS-first config (`@import "tailwindcss"`
   + `@theme` in `src/index.css`) and the repo has **no `tailwind.config.js`** —
   a doc-guided attempt to edit one would have failed outright.

3. **`.claude/README.md` — the index — omitted three whole directories.**
   `/skills/`, `/agents/`, and `/hooks/` were entirely absent despite `/skills/`
   being the primary progressive-disclosure mechanism. `/commands/` listed 3 of 10.

4. **Two "Key Files" paths in `CLAUDE.md` pointed at deleted files.**
   `backend/pkg/core/models.go` and `frontend/src/lib/api.ts` were both split up
   (per-domain `core/*.go`; `lib/api/` modules). All Batch 1 path references now
   resolve — verified mechanically.

5. **Containerization drift was pervasive**, as predicted: host `go test`,
   `npm test`, `npx playwright`, `docker ps`, and the nonexistent
   `just migrate_test` / `just test-frontend`. The `-p=1` flag in `TESTING.md`
   was actively counterproductive — per-package DB cloning exists precisely to
   allow parallelism.

**Carried forward:**
- `frontend/src/lib/api.ts.backup` is dead weight in the repo — not a docs issue,
  but worth deleting.
- Several context files exceed the README's own <500-line guidance
  (`FRONTEND_STYLING.md` ~940, `ARCHITECTURE.md` ~560). Guidance softened rather
  than files split; revisit if they keep growing.
- `frontend/src/features/` does **not** exist, yet the `frontend-dev-guidelines`
  skill description advertises a "features directory". Flagged for **Batch 3**.

---

## Batch 2 — Stale reference docs (Oct–Nov 2025, highest risk)

Nearly a year untouched while the backend was decomposed and the dev stack containerized.

| Status | Doc | Lines | Last Commit | Notes |
|---|---|---|---|---|
| UPDATED | `.claude/reference/BACKEND_ARCHITECTURE.md` | 736 | 2025-10-16 | **Same wrong auth model as ARCHITECTURE.md** (`username` claim + `AccessTokenExpiry`) — rewrote with real `sub`/`session_id`/7d. Discovered **`JWTConfig.AccessTokenExpiry`/`RefreshTokenExpiry` are dead config**: defined + defaulted (15m/7d) but never read. Removed fabricated `RequireGameMasterMiddleware` and `GetAuthenticatedUserID`/`GetAuthenticatedUsername` (none exist) — documented real `GameMiddleware()` + `IsUserGameMaster` + `GetAuthenticatedUser`. Fixed `just make_migration`/`just migrate_status` (neither exists). Rewrote project tree for service decomposition + 27 pkg dirs. Env section listed 5 of ~75 vars → pointed at `.env.example` + group table. Added test-DB isolation section. |
| UPDATED | `.claude/reference/API_DOCUMENTATION.md` | 522 | 2025-11-07 | Narrative curl guide, not an endpoint reference (router has 195 routes; doc covers ~14). **`/games/public` does not exist** — fixed 4 occurrences to `GET /games` (`GetFilteredGames`, auth-optional). Added "one token, not two" note so the `/auth/refresh` examples aren't read as a two-token model. All other documented paths + methods verified against `root.go` (incl. `GET /auth/refresh`, `PUT .../applications/{id}/review`). Pointed readers at Swagger UI `/api/v1/docs/` for the full list. |
| OK | `.claude/reference/BUILDER_USAGE_GUIDE.md` | 593 | 2025-10-21 | Verified against `backend/pkg/db/services/test_suite.go` — every claimed builder method (`NewTestSuite`, `WithCleanup`, `WithTables`, `WithFixtures`, `Setup`, `Cleanup`, service accessors, `AddParticipant`, `TransitionGameTo`) exists. Only defect: one link to a nonexistent `.claude/planning/TEST_UTILITIES_ANALYSIS.md`. |
| OK | `.claude/reference/LOGGING_STANDARDS.md` | 430 | 2025-10-16 | No command drift, no broken paths, logger API claims check out. Not deeply re-derived against the observability package — flag if logging is refactored. |
| OK | `.claude/reference/FRONTEND_ERROR_HANDLING.md` | 434 | 2025-10-16 | No command drift, no broken paths. Content-level review deferred to the frontend batch (Batch 3) where it overlaps the skill resources. |
| UPDATED | `.claude/reference/API_TESTING_WITH_CURL.md` | 340 | 2025-10-27 | All endpoints and both `api-test.sh` subcommands verified against `root.go` / the script. Added a "Last Verified" header and documented the other 12 script subcommands (`health`, `status`, `games`, `posts`, `create-post`, `test-mentions`, ...) the doc never mentioned. Overlap with `route-tester` skill still to reconcile in Batch 3. |
| OK | `.claude/reference/ERROR_HANDLING.md` | 263 | 2025-10-16 | Every `core.*` error helper referenced exists in `backend/pkg/core/`. |
| DELETED | `.claude/reference/TESTING_PARALLEL_EXECUTION.md` | 230 | 2025-10-16 | Confirmed a **historical changelog**, not current guidance — the named tests still exist, but isolation is now per-package DB cloning. Added a banner pointing to `.claude/context/TESTING.md`. Does not contradict current practice, so not deleted. |
| UPDATED | `.claude/reference/TESTING_GUIDE.md` | 200 | 2025-10-27 | Setup premise was pre-container: `brew install postgresql`, host `createdb`, `just db_up`, and a nonexistent `just test-db-setup` as the central one-time step (×5). Replaced with the real model — test recipes prepare the migrated template themselves and each package clones its own DB. Go test-code sections verified accurate and left as-is. |
| UPDATED | `.claude/reference/JUSTFILE_QUICK_REFERENCE.md` | 193 | 2025-10-27 | **Regenerated wholesale.** Described the pre-container workflow: 5 nonexistent recipes (`just dev`, `just build`, `install-frontend`, `preview-frontend`, `test-frontend`) and a trailing "from `just --list`" snapshot listing 31 recipes when 57 exist. Rewrote around the container lifecycle, added `db`/`migration`/`test-fe`/`e2e-test` subcommand forms and the `e2e-test file` path gotcha. Every recipe named now validates against `just --list`. |
| OK | `.claude/reference/GAME_APPLICATIONS_IMPLEMENTATION.md` | 157 | 2025-10-16 | No command drift or broken paths. Application endpoints verified live in `root.go` (`/apply`, `/application`, `/applications`, `PUT .../review`). |
| OK | `.claude/reference/GAME_APPLICATIONS_DESIGN.md` | 67 | 2025-10-16 | Design rationale; no stale commands or paths. |
| UPDATED | `.claude/reference/DEVELOPMENT_SETUP.md` | 549 | 2026-07-16 | Newer and mostly container-aware, but carried **16 invalid recipe names** — old underscore forms (`db_up`, `db_create`, `db_setup`, `reset_test_db`, `migrate_test`, `make_migration`, `migrate_status`) plus `test-db-setup`, `test-parallel`, `dev-restart`, `just dev`, `just build`. All mapped to real equivalents. |
| OK | `.claude/reference/PROJECT_SPEC.md` | 275 | 2026-08-18 | Recent; no drift found on scan. |

### Likely-obsolete point-in-time artifacts (recommend DELETE unless still useful)

| Status | Doc | Lines | Last Commit | Notes |
|---|---|---|---|---|
| DELETED | `.claude/reference/VERIFICATION_REPORT.md` | 89 | 2025-11-07 | An unfinished Oct 2025 audit of these same reference docs, still marked "In Progress". **Zero inbound references**, and it is the *only* referrer to 3 other artifact docs. It marked ✅ several docs this audit found materially wrong, so leaving it discoverable is actively misleading. |
| DELETED | `.claude/reference/TESTING_IMPROVEMENTS_SUMMARY.md` | 309 | 2025-10-16 | Aug 2025 changelog. Only referrer is `VERIFICATION_REPORT.md`. Delete alongside it. |
| DELETED | `.claude/reference/AI_FRIENDLY_IMPROVEMENTS.md` | 168 | 2025-10-16 | Oct 2025 progress tracker (✅/🚧/⏳ never re-verified). Referred to by `VERIFICATION_REPORT.md` and `docs/README.md`. |
| DELETED | `.claude/reference/E2E_TESTING_LEARNINGS_CODIFIED.md` | 161 | 2025-10-18 | Both its "see also" links point at gitignored `.claude/planning/` files that no longer exist. Merge anything still valuable into `frontend/e2e/README.md` / `testing-patterns`, then delete. |
| PENDING | `.claude/planning-doc.md` | 81 | 2026-05-07 | Not yet reviewed — carried into the next session. |

### Batch 2 findings (completed 2026-08-26)

**19 docs reviewed: 7 updated, 7 verified OK, 5 bannered as historical.**

1. **The wrong auth model was duplicated, not isolated.**
   `BACKEND_ARCHITECTURE.md` carried the same fiction as `ARCHITECTURE.md` — a
   `username` claim and `config.JWT.AccessTokenExpiry`. Root cause found:
   **`JWTConfig.AccessTokenExpiry` (15m) and `RefreshTokenExpiry` (7d) genuinely
   exist in `core/config.go` with those defaults, but nothing ever reads them.**
   `jwt.go` hardcodes 7 days. The docs were describing dead config. Both docs now
   say so explicitly. *Worth deciding separately whether to delete the dead
   config — it will keep re-seeding this error.*

2. **`BACKEND_ARCHITECTURE.md` documented three APIs that do not exist:**
   `RequireGameMasterMiddleware`, `GetAuthenticatedUserID`, and
   `GetAuthenticatedUsername`. Real pattern: `games.Handler.GameMiddleware()`
   loads game + `is_gm` into context; authorization is the `core.IsUserGameMaster`
   *helper*, not middleware; and `GetAuthenticatedUser` returns a struct you read
   fields off (and must nil-check). Also fixed `{id}` → `{gameID}`.

3. **`JUSTFILE_QUICK_REFERENCE.md` — the doc most likely to be trusted verbatim —
   was the most wrong.** It described the pre-container workflow: 5 nonexistent
   recipes and a trailing "from `just --list`" block listing 31 recipes when 57
   exist. Regenerated from the real `just --list`.

4. **Invalid `just` recipes were the dominant defect class.** A mechanical sweep
   of every `just <recipe>` mention against `just --list` found **16 invalid names
   in `DEVELOPMENT_SETUP.md` alone** (old underscore forms like `db_up`,
   `reset_test_db`, `migrate_test`), 5 in `JUSTFILE_QUICK_REFERENCE.md`, 2 in
   `TESTING_GUIDE.md`, plus stragglers in `BACKEND_ARCHITECTURE.md` and — missed
   in Batch 1 — `just reset-test-data` in `context/TEST_DATA.md`.
   **This sweep is now the cheapest high-yield check available; run it every batch.**

5. **`TESTING_GUIDE.md`'s entire setup premise was pre-container** — `brew install
   postgresql`, host `createdb`, and a nonexistent `just test-db-setup` presented
   as the mandatory one-time step (×5). No such step exists now.

6. **`API_DOCUMENTATION.md` is a narrative curl guide, not a reference.** It
   covers ~14 of 195 routes. `/games/public` (used 4×) does not exist — the real
   listing endpoint is `GET /games`. Readers now get pointed at Swagger UI
   (`/api/v1/docs/`, verified live) for the complete list.

7. **A cluster of dead historical artifacts, self-referentially linked.**
   `VERIFICATION_REPORT.md` is an unfinished Oct 2025 audit of these same docs
   with **zero inbound references** — and it is the *only* referrer to three other
   artifact docs. It marked ✅ several docs this audit found materially wrong.
   All five bannered rather than deleted; **deletion is your call.**

**Verified accurate (no changes):** `BUILDER_USAGE_GUIDE.md` (full `TestSuite`
builder surface checked method-by-method), `ERROR_HANDLING.md` (all `core.*`
helpers exist), `LOGGING_STANDARDS.md`, `GAME_APPLICATIONS_*`, `PROJECT_SPEC.md`.

**Carried forward:**
- Dead `JWTConfig` expiry fields — delete or wire up.
- `.claude/planning/` links persist in 2 docs though the dir is gitignored.
- `API_TESTING_WITH_CURL.md` vs the `route-tester` skill overlap — reconcile in Batch 3.

---

### Follow-up actions taken (2026-08-26, after Batch 2)

**All 5 historical artifacts deleted**, not just bannered. Rationale: a banner
only protects a reader who starts at the top of the file — someone landing on a
grep match mid-document never sees it. Each was independently confirmed
inaccurate first:

- `E2E_TESTING_LEARNINGS_CODIFIED.md` — every line-number citation stale
  (claims CLAUDE.md "lines 24-32"; the E2E section is now ~line 250).
- `TESTING_PARALLEL_EXECUTION.md` — bare host `go test -parallel N` commands that
  don't work in the containerized stack, and "each test creates its own database
  connection" when the isolation unit is now the *package*.
- `AI_FRIENDLY_IMPROVEMENTS.md` — ✅/🚧/⏳ status list frozen at Oct 2025.
- `VERIFICATION_REPORT.md` / `TESTING_IMPROVEMENTS_SUMMARY.md` — as described above.

Dangling link removed from `docs/README.md`. Zero references remain to any of the five.

**Dead JWT expiry config removed (code change).** `JWTConfig.AccessTokenExpiry`
and `RefreshTokenExpiry` deleted from `core/config.go`, its defaults, and
`test_utils.go`; `JWT_ACCESS_TOKEN_EXPIRY` / `JWT_REFRESH_TOKEN_EXPIRY` dropped
from `.env.example`. `JWT_SECRET` and `JWT_ALGORITHM` retained (both live).

Introduced **`core.SessionLifetime`** (7 days) and pointed the three previously
duplicated literals at it — `jwt.go` ×2 (temp + final token `exp`) and
`sessions.go` ×1 (session row `expires`). These were separate hardcoded values
that had to agree; a token outliving its own session row would authenticate
against a row the cleanup worker had already swept.

Verified: `go build` clean, `go vet` clean, `just test-mocks` and the full
`just test` suite pass. Docs updated to describe the constant rather than the
removed config.

**Deliberately not changed:** the 7-day lifetime itself (kept long — hobby site,
infrequent re-login is the goal) and the refresh session leak (`V1Refresh`
creates a new session row without deleting the old; orphans are swept within
7 days by the existing cleanup worker).

---

## Batch 3 — Skills

`.claude/planning/` is gitignored (untracked WIP) — **out of scope**, skip.

| Status | Doc | Lines | Last Commit | Notes |
|---|---|---|---|---|
| PENDING | `.claude/skills/backend-dev-guidelines/SKILL.md` | 541 | 2026-08-19 | |
| PENDING | `.claude/skills/game-domain/SKILL.md` | 527 | 2026-08-25 | |
| PENDING | `.claude/skills/game-domain/resources/game-states.md` | 151 | 2026-08-25 | |
| PENDING | `.claude/skills/game-domain/resources/messaging-system.md` | 92 | 2026-08-25 | |
| PENDING | `.claude/skills/game-domain/resources/phase-system.md` | 91 | 2026-08-06 | Verify vs. `is_published` semantics. |
| PENDING | `.claude/skills/testing-patterns/SKILL.md` | 202 | 2026-04-02 | |
| PENDING | `.claude/skills/testing-patterns/resources/e2e-testing.md` | 583 | 2025-10-31 | |
| PENDING | `.claude/skills/testing-patterns/resources/test-fixtures.md` | 550 | 2025-10-30 | |
| PENDING | `.claude/skills/testing-patterns/resources/e2e-patterns-reference.md` | 474 | 2025-10-31 | |
| PENDING | `.claude/skills/route-tester/SKILL.md` | 662 | 2025-10-30 | |
| PENDING | `.claude/skills/frontend-dev-guidelines/SKILL.md` | 405 | 2025-10-30 | Whole tree is Oct 2025 — high risk. |
| PENDING | `.claude/skills/frontend-dev-guidelines/resources/data-fetching.md` | 752 | 2025-10-30 | |
| PENDING | `.claude/skills/frontend-dev-guidelines/resources/complete-examples.md` | 726 | 2025-10-30 | |
| PENDING | `.claude/skills/frontend-dev-guidelines/resources/loading-and-error-states.md` | 604 | 2025-10-30 | |
| PENDING | `.claude/skills/frontend-dev-guidelines/resources/routing-guide.md` | 554 | 2025-10-30 | |
| PENDING | `.claude/skills/frontend-dev-guidelines/resources/component-patterns.md` | 495 | 2025-10-30 | |
| PENDING | `.claude/skills/frontend-dev-guidelines/resources/file-organization.md` | 479 | 2025-10-30 | |
| PENDING | `.claude/skills/frontend-dev-guidelines/resources/common-patterns.md` | 463 | 2025-10-30 | |
| PENDING | `.claude/skills/frontend-dev-guidelines/resources/styling-guide.md` | 436 | 2025-10-30 | Overlaps `context/FRONTEND_STYLING.md`. |
| PENDING | `.claude/skills/frontend-dev-guidelines/resources/typescript-standards.md` | 418 | 2025-10-30 | |
| PENDING | `.claude/skills/frontend-dev-guidelines/resources/performance.md` | 406 | 2025-10-30 | |

### `skill-developer` skill (generic tooling, not codebase-specific — lowest priority)

`SKILL.md` (426), `TROUBLESHOOTING.md` (514), `SKILL_RULES_REFERENCE.md` (315),
`TRIGGER_TYPES.md` (305), `HOOK_MECHANISMS.md` (306), `ADVANCED.md` (197),
`PATTERNS_LIBRARY.md` (152) — all 2025-10-30. Audit only for `skill-rules.json` drift.

### Stub files — decide: write or delete

Empty placeholders that add noise. The `game-domain` ones at least warn they're stubs; the `testing-patterns` ones do not.

- `testing-patterns/resources/`: `anti-patterns`, `backend-testing`, `bug-fix-workflow`, `coverage-targets`, `frontend-testing`, `real-examples`, `test-commands`, `testing-pyramid` (7 lines each)
- `game-domain/resources/`: `api-reference`, `business-rules`, `character-workflows`, `data-models`, `database-queries`, `testing-guide`, `workflows` (19 lines each)

## Batch 4 — Commands & agents

| Status | Doc | Lines | Last Commit | Notes |
|---|---|---|---|---|
| PENDING | `.claude/commands/review-changes.md` | 255 | 2025-11-06 | |
| PENDING | `.claude/commands/audit-test.md` | 197 | 2026-04-22 | |
| PENDING | `.claude/commands/challenge-assumptions.md` | 128 | 2025-10-27 | |
| PENDING | `.claude/commands/implement-feature.md` | 100 | 2026-08-18 | |
| PENDING | `.claude/commands/audit-test-init.md` | 86 | 2026-04-22 | |
| PENDING | `.claude/commands/dev-docs.md` | 68 | 2025-10-30 | |
| PENDING | `.claude/commands/dev-docs-update.md` | 68 | 2025-10-30 | |
| PENDING | `.claude/commands/fix-bug.md` | 62 | 2026-03-09 | |
| PENDING | `.claude/commands/implement-features.md` | 58 | 2025-10-27 | Overlaps `implement-feature.md` — consolidate? |
| PENDING | `.claude/commands/debug-e2e-test.md` | 57 | 2025-10-27 | |
| PENDING | `.claude/agents/README.md` | 300 | 2025-10-30 | |
| PENDING | `.claude/agents/web-research-specialist.md` | 78 | 2025-10-30 | |
| PENDING | `.claude/agents/refactor-planner.md` | 62 | 2025-10-30 | |
| PENDING | `.claude/agents/plan-reviewer.md` | 52 | 2025-10-30 | |
| PENDING | `.claude/hooks/README.md` | 116 | 2026-04-22 | Verify vs. actual hook scripts + settings.json. |

## Batch 5 — ADRs & architecture (`docs-site/developer/`)

Most are 2025-11-15 and predate major architectural work.

| Status | Doc | Lines | Last Commit | Notes |
|---|---|---|---|---|
| PENDING | `adrs/007-testing-strategy.md` | 843 | 2026-05-07 | |
| PENDING | `adrs/005-frontend-state-management.md` | 719 | 2025-11-15 | |
| PENDING | `adrs/006-observability-approach.md` | 483 | 2025-11-15 | Predates all Faro work — likely very stale. |
| PENDING | `adrs/004-api-design-principles.md` | 316 | 2025-11-15 | |
| PENDING | `adrs/003-authentication-strategy.md` | 258 | 2026-05-07 | |
| PENDING | `adrs/002-database-design-approach.md` | 203 | 2025-11-15 | |
| PENDING | `adrs/001-technology-stack-selection.md` | 117 | 2025-11-15 | |
| PENDING | `adrs/README.md` | 49 | 2025-11-15 | |
| PENDING | `architecture/components.md` | 632 | 2025-11-15 | |
| PENDING | `architecture/overview.md` | 285 | 2025-11-15 | |
| PENDING | `getting-started/onboarding.md` | 449 | 2025-11-15 | Predates containerized dev — likely wrong setup steps. |
| PENDING | `api/reference.md` | 156 | 2026-08-18 | |
| PENDING | `developer/index.md` | 50 | 2026-04-22 | |

**ADR note:** ADRs record decisions *as made at the time*. Where reality diverged, prefer adding a "Superseded / Evolution" section over rewriting history.

## Batch 6 — Testing docs (`docs-site/developer/testing/`)

| Status | Doc | Lines | Last Commit | Notes |
|---|---|---|---|---|
| PENDING | `TEST_DATA.md` | 1256 | 2026-08-18 | Largest doc in repo. Verify vs. fixture SQL. |
| PENDING | `COVERAGE_STATUS.md` | 732 | 2025-11-15 | Coverage numbers are certainly stale — regenerate. |
| PENDING | `E2E_QUICK_START.md` | 462 | 2025-11-15 | |
| PENDING | `E2E_FIXTURES.md` | 203 | 2025-11-15 | |
| PENDING | `TEST_COVERAGE_REFERENCE.md` | 50 | 2025-11-15 | |
| PENDING | `frontend/e2e/README.md` | 659 | 2026-07-16 | |
| PENDING | `frontend/e2e/pages/README.md` | 444 | 2025-10-26 | Page objects — verify vs. actual files. |
| PENDING | `frontend/e2e/STATUS.md` | 348 | 2025-10-25 | Coverage status; note mobile E2E gap. |

## Batch 7 — `docs/` tree (deployment, operations, legacy)

Entirely Oct–Nov 2025. Overlaps `docs-site/` and `.claude/reference/` — a consolidation candidate as much as an accuracy audit.

| Status | Doc | Lines | Last Commit | Notes |
|---|---|---|---|---|
| PENDING | `docs/features/STATE_MANAGEMENT.md` | 1606 | 2025-10-17 | Huge + stale. Overlaps context + ADR-005. Strong merge/delete candidate. |
| PENDING | `docs/operations/LOGGING_STRATEGY.md` | 465 | 2025-11-08 | |
| PENDING | `docs/getting-started/DEVELOPER_ONBOARDING.md` | 449 | 2025-10-27 | Duplicates `docs-site/.../onboarding.md`. |
| PENDING | `docs/features/IMPLEMENTATION_SUMMARY.md` | 416 | 2025-10-17 | Point-in-time; likely DELETE. |
| PENDING | `docs/operations/DEPLOYMENT_SCRIPT_UPDATES.md` | 382 | 2025-11-08 | Point-in-time; likely DELETE. |
| PENDING | `docs/operations/LOGGING_QUICK_REFERENCE.md` | 355 | 2025-11-08 | |
| PENDING | `docs/operations/DEPLOYMENT_IMPROVEMENTS.md` | 347 | 2025-11-08 | Point-in-time; likely DELETE. |
| PENDING | `docs/deployment/PRODUCTION_ENV_CHECKLIST.md` | 339 | 2025-11-16 | Verify vs. `.env.example`. |
| PENDING | `docs/deployment/ROUTE53_SSL_SETUP.md` | 292 | 2025-11-07 | |
| PENDING | `docs/deployment/SSL_BOOTSTRAP_GUIDE.md` | 281 | 2025-11-07 | |
| PENDING | `docs/development/API_DOCS_PRODUCTION.md` | 300 | 2025-11-16 | |
| PENDING | `docs/development/API_DOCS_QUICK_START.md` | 284 | 2025-11-16 | |
| PENDING | `docs/development/API_DOCS_PROGRESS.md` | 281 | 2025-11-16 | Progress tracker; likely DELETE. |
| PENDING | `docs/development/API_DOCS_AUTOMATION.md` | 263 | 2025-11-16 | |
| PENDING | `docs/README.md` | 148 | 2025-10-27 | Index for this tree. |

## Batch 8 — Component-level READMEs & misc

| Status | Doc | Lines | Last Commit | Notes |
|---|---|---|---|---|
| PENDING | `frontend/src/components/ui/README.md` | 583 | 2026-07-27 | UI library API ref — CLAUDE.md leans on this heavily. |
| PENDING | `frontend/src/styles/MIGRATION_PATTERNS.md` | 475 | 2025-10-21 | Migration likely complete → DELETE? |
| PENDING | `frontend/src/styles/CSS_VARIABLES_USAGE.md` | 298 | 2025-10-21 | |
| PENDING | `backend/scripts/README.md` | 290 | 2025-11-24 | |
| PENDING | `backend/pkg/core/UTILITIES_GUIDE.md` | 260 | 2025-10-19 | |
| PENDING | `backend/pkg/core/MIGRATION_EDGE_CASES.md` | 236 | 2025-10-19 | |
| PENDING | `README.md` (root) | 207 | 2026-04-22 | Public-facing entry point. |
| PENDING | `docs-site/README.md` | 212 | 2025-11-15 | |
| PENDING | `frontend/README.md` | 66 | 2025-08-06 | Oldest doc in repo — probably stock Vite boilerplate. |
| PENDING | `.github/workflows/README.md` | 114 | 2026-04-22 | Verify vs. actual workflow YAML. |
| PENDING | `docs-site/STATUS.md` | 53 | 2026-04-22 | |
| PENDING | `docs-site/index.md` | 27 | 2026-04-22 | |

## Out of scope

- **`docs-site/guide/**`** (20 files) — end-user documentation, not dev-facing. Worth a separate audit against actual UI behavior.
- **`.claude/planning/**`** (4 files) — gitignored WIP scratch.
- **`frontend/dist/community-guidelines.md`** — build artifact.
- **`frontend/public/community-guidelines.md`** — user-facing site content.

---

## Cross-cutting issues to watch for

Recurring drift to check in every doc:

1. **`just test-frontend`** — doesn't exist; correct command is `just test-fe run`. *(CONFIRMED — found in CLAUDE.md ×3, TESTING.md, ARCHITECTURE.md)*
2. **`just make_migration` / `just migrate_test`** — neither exists. Use `just migration create <name>` and `just reset-test-db`. *(CONFIRMED — `migrate_test` found in CLAUDE.md, TEST_DATA.md)*
3. **Host-run commands** — dev stack is containerized; host `npm`/`npx`/`go` invocations are wrong or misleading.
4. **Database name** — must be `actionphase`; host `localhost:5432`, in-network host is `db`.
5. **Test DB isolation** — backend tests clone a per-package DB from a migrated template. Older docs describe a single shared test DB.
6. **Service decomposition** — `phases/`, `actions/`, `messages/` are now multi-file packages, not single files.
7. **`phase.is_published`** — means "GM published action results", NOT phase visibility.
8. **Observability** — Grafana Faro frontend instrumentation postdates ADR-006 entirely.
9. **Epilogue game state** — recently merged; check state-machine docs include it.
10. **Coverage numbers** — any hardcoded percentage/test count is stale; regenerate or drop.
