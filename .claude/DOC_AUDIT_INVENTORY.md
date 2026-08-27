# Dev Documentation Audit — Inventory

**Created:** 2026-08-26
**Goal:** For each doc, assess accuracy against the actual state of the codebase and bring it back up to date.
**Scope:** Internal/dev-facing docs only. 152 tracked `.md` files (excludes `frontend/dist/`, `node_modules/`).

**Progress:** ✅ **All 8 batches complete** (114 docs audited; 4 deleted). **Zero PENDING remaining.**

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
| UPDATED | `.claude/planning-doc.md` | 81 | 2026-05-07 | Re-verified 2026-08-27: **items 1–5 are complete** (deprecated `User()` removed with 0 callers left; `useAuthLegacy` gone; phases migration TODOs resolved; `messages/api.go` decomposed into 6 files). Only item 6 remains — 26 `fmt.Errorf` instances, re-counted and re-mapped onto the decomposed files. Statuses updated in place rather than deleted; this is an active work plan. |

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
| UPDATED | `.claude/skills/backend-dev-guidelines/SKILL.md` | 541 | 2026-08-19 | Structurally sound (already carried the Batch-2 "Validate in Bind" fix). Fixed `just dev` + `just make_migration` (neither exists) → container lifecycle + `just migration create`; added the in-network `db:5432` DSN alongside the host one; `core/models.go` → split `core/*.go` (×2); ADR path `/docs/adrs/` → `/docs-site/developer/architecture/adrs/`; removed 2 links into gitignored `.claude/planning/`. |
| REWRITTEN | `.claude/skills/frontend-dev-guidelines/SKILL.md` | 405→~300 | 2025-10-30 | **Described a different project.** See findings #1. |
| DELETED | `.../resources/file-organization.md` | 479 | 2025-10-30 | Entirely `features/`-based; that directory does not exist. |
| DELETED | `.../resources/routing-guide.md` | 554 | 2025-10-30 | Teaches React Router **v6** `<Routes>` JSX; app uses **v7** `createBrowserRouter`. |
| DELETED | `.../resources/common-patterns.md` | 463 | 2025-10-30 | Built on react-hook-form, Zod, Zustand, DataGrid — **none installed**. Its `useAuth` example also invented a `user.roles` field. |
| DELETED | `.../resources/complete-examples.md` | 726 | 2025-10-30 | `features/` + Suspense + MUI throughout. |
| OK | `.../resources/styling-guide.md` | 436 | 2025-10-30 | Accurate. Defers to `context/FRONTEND_STYLING.md` as the fuller reference. |
| OK | `.../resources/typescript-standards.md` | 418 | 2025-10-30 | Accurate. |
| KEPT (flagged) | `.../resources/performance.md` | 406 | 2025-10-30 | Patterns sound; examples use `React.FC` / occasional Suspense. Flagged in SKILL.md accuracy table. |
| KEPT (flagged) | `.../resources/component-patterns.md` | 495 | 2025-10-30 | `SuspenseLoader` (nonexistent) + default-export rule. Flagged. |
| KEPT (flagged) | `.../resources/data-fetching.md` | 752 | 2025-10-30 | `useSuspenseQuery`-first. Flagged. |
| KEPT (flagged) | `.../resources/loading-and-error-states.md` | 604 | 2025-10-30 | "No early returns" rule contradicts real code. Flagged. |
| OK | `.claude/skills/game-domain/SKILL.md` | 527 | 2026-08-25 | **Verified accurate.** All 8 `GameState` constants match `core/constants.go` incl. the recent `epilogue`; `is_published` semantics correct. |
| OK | `.claude/skills/game-domain/resources/game-states.md` | 151 | 2026-08-25 | Matches the DB CHECK constraint in migration `20260825193725`. |
| OK | `.claude/skills/game-domain/resources/messaging-system.md` | 92 | 2026-08-25 | No defects found. |
| OK | `.claude/skills/game-domain/resources/phase-system.md` | 91 | 2026-08-06 | `is_published` correctly documented as "GM published results", NOT visibility. |
| UPDATED | `.claude/skills/testing-patterns/SKILL.md` | 202 | 2026-04-02 | Removed obsolete `go test -p=1` host invocation (packages clone their own DB and run parallel — `-p=1` is counterproductive) → `just test-run`; `just test-frontend` → `just test-fe run`; repointed the dead `frontend-testing.md` nav row. |
| UPDATED | `.../resources/test-fixtures.md` | 550 | 2025-10-30 | `just db-restart` (nonexistent) → `just restart db`; added `just reset-test-db` for a dirty test template. |
| UPDATED | `.../resources/e2e-testing.md` | 583 | 2025-10-31 | **Completed in Batch 6** — see that section. |
| UPDATED | `.../resources/e2e-patterns-reference.md` | 474 | 2025-10-31 | **Completed in Batch 6** — see that section. |
| UPDATED | `.claude/skills/route-tester/SKILL.md` | 662 | 2025-10-30 | **Third copy of the 15-min/refresh-token fiction** — rewrote the auth overview (one 7-day session-backed token; `sub`/`session_id`/`exp`) and the 401 troubleshooting cause. Deleted a fabricated "justfile integrates api-test.sh" block (`just api-login`, `api-games`, `api-game` — none exist) and replaced it with the real 14 script subcommands. `just psql` → `just sh backend` + psql; `just dev` → `just up`; `core/models.go` → `core/*.go`. |
| UPDATED | `.claude/skills/skill-rules.json` | 398 | — | **Two activation bugs.** `backend/cmd/**/*.go` → `backend/main.go` (no `cmd/` dir). `game-domain`'s three frontend globs (`components/{games,phases,characters}/**`) matched **zero** files — components are flat, not subdirectoried; replaced with name globs + `character-updates/` + `pages/Game*`, now matching **61** files. **Superseded by Batch 4 finding #3:** `fileTriggers` is read by no hook, so these were real config bugs but fixing them changed no runtime behavior. |
| DELETED | `testing-patterns/resources/` ×8 stubs | 7 each | — | `anti-patterns`, `backend-testing`, `bug-fix-workflow`, `coverage-targets`, `frontend-testing`, `real-examples`, `test-commands`, `testing-pyramid`. Content-free filler ("*(Detailed documentation to be added)*"); 7 of 8 had zero inbound links. |
| KEPT | `game-domain/resources/` ×7 stubs | 19 each | — | **Deliberately kept.** Unlike the above, these warn loudly ("⚠️ STUB — not yet written… do not infer rules from it") and redirect to specific source files. That is useful behavior, not noise. |

### `skill-developer` skill — audited for `skill-rules.json` drift only

`SKILL.md` (426), `TROUBLESHOOTING.md` (514), `SKILL_RULES_REFERENCE.md` (315),
`TRIGGER_TYPES.md` (305), `HOOK_MECHANISMS.md` (306), `ADVANCED.md` (197),
`PATTERNS_LIBRARY.md` (152) — all 2025-10-30. **No changes made.** Generic
tooling docs; their `frontend/src/features/…`-style paths are hypothetical
worked examples, not claims about this repo. They do reference three hook files
that do not exist (`error-handling-reminder.ts`, `skill-verification-guard.ts`,
`.claude/hooks/state/`) and `.claude/settings.json` (actual file is
`settings.local.json`) — noted for Batch 4, which covers hooks.

### Batch 3 findings (completed 2026-08-26)

**21 docs + `skill-rules.json` reviewed: 1 rewritten, 5 updated, 8 verified OK,
12 deleted (4 resources + 8 stubs).**

1. **`frontend-dev-guidelines` was written for a different codebase.** This is the
   most severe finding of the audit so far — worse than stale, it was *foreign*.
   Its copy-paste component template imported **MUI** (`@mui/material`, `Box`,
   `Paper`), which is **not a dependency**. It also prescribed, none of which exist
   here: a `features/` directory tree, a `routes/` directory, a `SuspenseLoader`
   component, and a `~types/` alias. Its cross-referenced patterns relied on
   react-hook-form, Zod, Zustand, and DataGrid — **zero of the four are installed**.

   Where it did name real tools, it inverted the conventions:

   | Skill prescribed | Codebase reality |
   |---|---|
   | `useSuspenseQuery` as PRIMARY | `useQuery` in **54** files; `useSuspenseQuery` in **1** |
   | default exports | **180** named exports vs **6** default |
   | `React.FC<Props>` required | 29 files use it; both styles are live |
   | `<Routes>`/`<Route>` JSX (v6) | React Router **7** `createBrowserRouter` |
   | `import { api }` | `import { apiClient }` |
   | "NEVER early-return on loading" | `useQuery` code branches on `isLoading` throughout |

   A developer following this skill would have written code that does not compile.
   SKILL.md rewritten from verified source; every symbol in the new template
   (`Card`, `CardBody`, `Button`, `Spinner`, `Alert`, their variant values, and
   `apiClient.games.getGame`) checked to exist. The four unsalvageable resources
   were **deleted** rather than bannered, per the Batch-2 precedent: a banner does
   not protect a reader landing on a grep hit mid-file. The four survivors carry a
   per-file accuracy table in SKILL.md.

2. **The auth fiction had a third copy.** `route-tester/SKILL.md` independently
   asserted "Token lifetime: 15 minutes (access token) / Refresh tokens: 7 days"
   — the same claim corrected in `ARCHITECTURE.md` (Batch 1) and
   `BACKEND_ARCHITECTURE.md` (Batch 2). It appeared **twice**, once in the
   overview and once mid-document in a 401 troubleshooting list — the exact
   grep-hit failure mode that motivated deleting rather than bannering.
   Now purged tree-wide (verified by grep). Removing the dead config in the
   Batch-2 follow-up removed this error's source of truth.

3. **`skill-rules.json` had two silent activation bugs.** These fail *closed* —
   no error, the skill simply never loads. `game-domain` declared three frontend
   globs (`components/{games,phases,characters}/**/*.tsx`) that matched **zero
   files**, because game/phase/character components are flat files
   (`CharacterSheet.tsx`, `PhaseCard.tsx`), not subdirectories. Replaced with
   patterns matching **61** real files. `backend-dev-guidelines` pointed at
   `backend/cmd/**/*.go`; the entrypoint is `backend/main.go`.
   **New check for future batches: validate trigger globs actually match files.**

4. **Fabricated tooling.** `route-tester` documented a block of `just api-login` /
   `api-login-gm` / `api-games` / `api-game` recipes under the heading "justfile
   integrates api-test.sh". No such recipes exist and never did. Replaced with the
   real 14 `api-test.sh` subcommands, most of which the doc had never mentioned.

5. **Two token families coexist, and the docs teach the losing one.** Both
   `text-text-*`/`bg-bg-*` and `text-content-*`/`surface-*` are declared in
   `@theme` and render correctly, so nothing is broken — but the UI component
   library itself uses `content-`/`surface-`, which leads ~20:1 in app code
   (`text-content-primary` 483 vs `text-text-primary` 25). **`CLAUDE.md` and
   `context/FRONTEND_STYLING.md` both teach the legacy names.** The new frontend
   skill documents the split and the preference. *Worth deciding whether to
   migrate the stragglers or update those two docs.*

6. **The `-p=1` regression reappeared**, as in Batch 1 — `testing-patterns` still
   carried a raw host `go test -p=1` invocation. Per-package DB cloning exists
   precisely to allow parallelism.

7. **`game-domain` is the healthiest skill in the tree** and needed no changes —
   all 8 `GameState` constants match, including `epilogue` (added 2026-08-25),
   and its `is_published` documentation correctly warns it means "GM published
   results", not phase visibility.

**Mechanical sweeps now clean tree-wide:** every `just <recipe>` validates
against `just --list`; every relative `.md` link resolves; every repo path
resolves (remaining hits are `skill-developer`'s hypothetical examples).

**Carried forward:**
- `skill-rules.json` declares an `implement-feature` skill with **no directory** —
  it is a `.claude/commands/` entry, not a skill. Resolve in **Batch 4**.
- Three hook files referenced by `skill-developer` do not exist, and it names
  `.claude/settings.json` when the real file is `settings.local.json` — **Batch 4**.
- `e2e-testing.md` + `e2e-patterns-reference.md` deep review → **Batch 6**.
- Decide on the `content-`/`surface-` vs `text-`/`bg-` token split (finding #5).
- The 4 flagged-but-kept frontend resources should eventually be rewritten or
  deleted; they are currently accurate only about generic React, not this app.

---

### Follow-up: design-token standardization (2026-08-26, after Batch 3)

Batch 3 finding #5 said two token families coexisted and "both work". **That was
wrong**, and the correction is the most consequential result of the audit so far.

**`bg-bg-*` and `border-border-*` emit no CSS whatsoever.** They are registered
in the `@theme` block of `src/index.css` but were **never assigned values** in
`src/lib/theme/themes.ts`. Tailwind generates no rule, so an element using one
renders with *no background or border* — silently, in both light and dark mode.
Verified by building the bundle and grepping the emitted CSS (`surface-base`
appears 11×; `bg-bg-page`, `bg-primary`, `bg-danger-light` appear 0×).

Independent corroboration: `ui/Toggle.test.tsx` already carried a regression test
titled *"Off-state visibility (regression)"* whose comment reads "the off-state
used unassigned tokens … that render invisible in light mode." Someone hit this
bug once, fixed one component, and documented it only in that file's local note.

**Shipped bugs this was causing:**
- `GamesList` skeleton loaders — every placeholder bar invisible
- `PollResults` — progress bar track, winning fill, and losing fill all invisible
- `CommonRoom` — active-tab underline missing (no selected-tab indicator)
- `EditHandoutModal` / `CreateHandoutModal` — required-field `*` invisible
- `AdminsTab` / `BannedUsersTab` — hand-rolled badges, light-mode-only colors

**Code changes (~185 replacements across ~55 files):**

| Retired | Replacement | Basis |
|---|---|---|
| `bg-bg-primary` | `surface-base` | both white / gray-800 |
| `bg-bg-secondary` | `surface-raised` | both gray-50 / gray-900 |
| `bg-bg-tertiary`, `bg-bg-input` | `surface-sunken` | gray-100 / gray-900 |
| `border-border-primary`, `-default` | `border-theme-default` | both gray-200 / gray-700 |
| `border-border-secondary`, `-input` | `border-theme-strong` | gray-300–400 / gray-500–600 |
| `border-border-warning` | `border-semantic-warning` | intent-preserving |
| `bg-accent-primary`, `bg-primary` | `bg-interactive-primary` | intent-preserving |
| `bg-accent-primary/10` | `bg-interactive-primary-subtle` | opacity modifiers don't work here |
| `text-danger`, `text-danger-text` | `text-semantic-danger` | 57 existing uses |

Mappings were derived from the literal RGB values in `themes.ts`, not guessed —
a blind rename would have shifted colors. Note `text-text-primary` resolves to
`--color-content-secondary`, **not** `-primary`, so it is *not* a drop-in rename;
the `text-text-*` classes are hand-written utilities that do work and were left
alone.

Also replaced 3 hand-rolled badge `<span>`s with `<Badge>` in the admin tabs.

**Guard added:** `frontend/src/__tests__/retired-tokens.test.ts` scans all of
`src/` for 29 retired class names and fails with the offending file + token.
Verified it actually fails by planting a canary, not just that it passes.

**Docs standardized on the verified set** (all confirmed to emit CSS by building
the bundle and grepping):
- `frontend/src/components/ui/README.md` — now the **authoritative** token
  reference; rewrote its "CSS Variables Reference" and added a retired-token
  mapping table
- `.claude/context/FRONTEND_STYLING.md` — replaced the token list I *expanded* in
  Batch 1 (I had documented the dead tokens more thoroughly)
- `CLAUDE.md`, `.claude/commands/implement-feature.md`,
  `.claude/commands/review-changes.md`, and 5 frontend skill docs

**Deleted** `frontend/src/styles/MIGRATION_PATTERNS.md` and
`CSS_VARIABLES_USAGE.md` (both 2025-10-20). These were the *origin* of the
error: mid-migration docs that taught `bg-bg-*`/`text-text-*` as the target
state, then proposed building the UI library that superseded them. Neither had
inbound references after the `ui/README.md` link was removed.

**Verification:** `tsc -b` clean; **3,857 frontend tests pass** (241 files).
Four test assertions that referenced retired class names were updated. One
(`HandoutView`) needed a real selector fix, not a rename: `.bg-bg-secondary` had
been acting as a de-facto "comment container" selector, and `surface-raised` also
matches `Button variant="secondary"` — so the filter caught the top-level Edit
button too. Re-anchored to the comment action row.

**Not changed:** decorative mid-gray drag-handle dots in `CommentEditor` (legible
in both themes, a visual judgment call), and `MarkdownPreview`'s `text-gray-300/400`
(intentional, on a dark tooltip alongside `text-white`).

---

### Follow-up: retiring the `text-text-*` family (2026-08-26)

The token standardization above left `text-text-*` in place because those classes
*did* emit CSS. On closer inspection they were the most dangerous names in the
system, and were removed.

**`text-text-primary` resolved to `--color-content-secondary` — not `-primary`.**
The class name misrepresented the color it produced. `text-text-muted` and
`text-text-disabled` were worse still: they referenced variables that **no theme
assigns**, so `color: rgb(var(--unset))` is invalid and the element silently
inherited its parent's color. A "muted" error detail in `NewCommentsView` was
rendering at full Alert color.

**Root cause, traced to commit `cbb8533f` ("CSS refactor", 2025-11-12):** that
commit deleted `--color-text-primary`, `-secondary`, `-muted`, and `-disabled`
from `themes.ts`, then repointed `.text-text-primary` at `--color-content-secondary`
to stop it rendering as nothing. It was damage control, not a design decision —
and it left a class whose name and behavior disagreed.

**The renames are provable no-ops.** The deleted `--color-text-primary` values
(`75 85 99` / `209 213 219` / `30 30 30` / `230 230 230` / `55 65 81`) are
byte-identical to the current `--color-content-secondary` in all five themes
(light, dark, highContrast, highContrastDark, colorblind). Same for
`--color-text-heading` vs `--color-content-primary`.

| Retired | Replacement | Visual change |
|---|---|---|
| `text-text-heading` | `text-content-primary` | none (identical values) |
| `text-text-primary` | `text-content-secondary` | none (identical values) |
| `text-text-secondary` | `text-content-secondary` | none (was already this) |
| `text-text-muted` | `text-content-tertiary` | **yes** — now actually muted |
| `text-text-tertiary` | `text-content-tertiary` | **yes** — was never declared |

**Changes:** 76 replacements across 23 files; the five `.text-text-*` utility
classes deleted from `index.css` (with a comment explaining why); their orphaned
`@theme` registrations removed; the now-unread `--color-text-heading` value
dropped from all five themes. Guard extended to 35 retired names and
canary-verified against the new family. Docs updated in `ui/README.md`,
`context/FRONTEND_STYLING.md`, the frontend SKILL.md, and 4 skill resources
(which also referenced a `text-text-danger` that never existed).

**Net result:** one text-color family (`text-content-*`), no aliases, no name
that disagrees with its value.

---

## Batch 4 — Commands & agents

| Status | Doc | Lines | Last Commit | Notes |
|---|---|---|---|---|
| UPDATED | `.claude/commands/review-changes.md` | 255 | 2025-11-06 | 6-step plan lookup where 5 paths could never match → real flat layout. `/docs/adrs/`, `/docs/testing/` → `docs-site/developer/…`. `just make_migration` → `just migration create`. Examples used fictional plans → real ones. Added `text-text-*` to retired tokens. |
| OK | `.claude/commands/audit-test.md` | 197 | 2026-04-22 | Self-consistent; V&V criteria are sound. |
| OK | `.claude/commands/challenge-assumptions.md` | 128 | 2025-10-27 | Generic protocol, no project claims. |
| UPDATED | `.claude/commands/implement-feature.md` | 100 | 2026-08-18 | `just make_migration`→`migration create`; `test-frontend`→`test-fe run`; `just dev`+`run-frontend`→`just up`; `lib/api.ts` (split up)→`lib/api/`. Verified `core.NewTestDatabase`, `api-test.sh`, `interfaces.go` all real. |
| OK | `.claude/commands/audit-test-init.md` | 86 | 2026-04-22 | All 3 globs resolve (244 FE / 144 BE / 49 e2e). |
| UPDATED | `.claude/commands/dev-docs.md` | 68 | 2025-10-30 | Prescribed `planning/active/<name>/` dirs, a 3-file split, and `FEATURE_PLAN_TEMPLATE.md` — **none exist**. Real practice is one flat file. Rewritten to match. |
| UPDATED | `.claude/commands/dev-docs-update.md` | 68 | 2025-10-30 | Same fictional structure; `just test-frontend`→`test-fe run`. |
| OK | `.claude/commands/fix-bug.md` | 62 | 2026-03-09 | Accurate; correctly states the no-commit rule. |
| UPDATED | `.claude/commands/implement-features.md` | 58 | 2025-10-27 | **Told Claude to "Commit with descriptive message"**, contradicting CLAUDE.md's critical git rule. |
| OK | `.claude/commands/debug-e2e-test.md` | 57 | 2025-10-27 | Playwright MCP tool names all real. |
| REWRITTEN | `.claude/agents/README.md` | 300→69 | 2025-10-30 | Documented **10 agents; 7 do not exist**. Referenced a `showcase/` dir that doesn't exist. Claimed "JWT cookie auth" (4th instance of that fiction — it's `Authorization: Bearer`). |
| OK | `.claude/agents/web-research-specialist.md` | 78 | 2025-10-30 | Project-agnostic, no false claims. |
| OK | `.claude/agents/refactor-planner.md` | 62 | 2025-10-30 | Project-agnostic. |
| OK | `.claude/agents/plan-reviewer.md` | 52 | 2025-10-30 | Project-agnostic. |
| UPDATED | `.claude/hooks/README.md` | 116 | 2026-04-22 | Listed 8 skills, **6 nonexistent**. Build-check section rewritten for the new `just verify-quick` delegation. |

### Batch 4 findings (completed 2026-08-26)

**15 docs reviewed: 1 rewritten, 8 updated, 6 verified OK. Plus 7
`skill-developer` files corrected and 2 code fixes.**

1. **The Stop hook was a no-op that reported success.** All three of its branches
   were dead: `go build ./backend/cmd/server` (path doesn't exist, and `go.mod`
   lives in `backend/` so the root-level build fails regardless); `go mod verify`
   (guarded on a root `go.mod` that doesn't exist); and host `npx tsc`/`npm run
   build` (breaks the moment host `node_modules` is deleted, which is the stated
   direction of the repo). Worse, it piped `go build` into `tee` and tested the
   **pipeline** status, so failures were swallowed and it printed
   "✅ All build checks passed! • Backend (Go build + vet)".

   Replaced with a thin wrapper over a new **`just verify-quick`**.

2. **`just type-check` was passing vacuously.** It ran `tsc --noEmit` against the
   root `tsconfig.json`, which is solution-style (`"files": []` + project
   references) — so it type-checked **zero files**. Caught by canary: a real
   `const x: number = "bad"` passed. Now `tsc -b --force`. This is a live
   correctness fix, not a doc fix.

3. **`fileTriggers` is inert.** The only skill hook is `UserPromptSubmit`, which
   reads `promptTriggers` only — its TypeScript interface doesn't even declare
   `fileTriggers`. Six skills carry file-trigger config that activates nothing.
   **This corrects Batch 3 finding #7**: the zero-matching globs I fixed there
   were real bugs, but fixing them changed no behavior.

4. **`skill-developer` documented two hooks that don't exist** — a PreToolUse
   verification guard and a Stop error-handling reminder — across 3 files,
   including a 306-line `HOOK_MECHANISMS.md` where half the content described a
   blocking-enforcement architecture this project has never had. No `PreToolUse`
   hook is registered at all. Also fictional: `skipConditions`,
   `SKIP_SKILL_GUARDRAILS`, `SKIP_DB_VERIFICATION`, `.claude/hooks/state/`.

5. **`skill-developer` was vendored from another codebase.** Same class as Batch
   3's `frontend-dev-guidelines`: examples throughout referenced **Prisma**,
   `PrismaService`, `@project/database`, `schema.prisma`, and a `form/src/`
   tree with a workflow engine. Replaced with verified ActionPhase equivalents
   (`backend/pkg/db/services/**/*.go`, `import.*pkg/db/models`,
   `\.GetDraftCharacterUpdates\(`) — each confirmed to match real files
   (23 and 7 respectively).

6. **`.claude/settings.json` does not exist**; the real file is
   `settings.local.json`. Corrected in 3 files (carried over from Batch 3).

7. **`implement-feature` in `skill-rules.json` is a command, not a skill** — no
   directory under `.claude/skills/`. It *does* resolve, because Claude Code
   surfaces `.claude/commands/*.md` as invocable skills, so the trigger works.
   Annotated rather than deleted.

**New justfile recipes** (`verify` rewritten, `verify-quick`, `build`,
`build-backend`, `build-frontend`, `tidy-check`, `fmt-check`): `verify` is now
the pre-push gate (all checks + both production builds, parallel, ~27s);
`verify-quick` is the Stop-hook bundle (6 non-mutating checks, parallel,
~8-11s, exits 0 silently when the stack is down). Both canary-verified to fail
on real errors and to report every failing parallel job independently.

## Batch 5 — ADRs & architecture (`docs-site/developer/`)

Most are 2025-11-15 and predate major architectural work.

| Status | Doc | Lines | Last Commit | Notes |
|---|---|---|---|---|
| UPDATED | `adrs/007-testing-strategy.md` | 843 | 2026-05-07 | Strategy sound. Fixed: `just test-bench` / `test-frontend` / `test-e2e` (all nonexistent, incl. inside the CI YAML block); directory structure listed `pkg/testutil/`, `backend/tests/`, `__tests__/integration/`, `__tests__/e2e/` — **none exist**. Rewrote from the real co-located layout; added `verify`/`verify-quick`. |
| UPDATED | `adrs/005-frontend-state-management.md` | 719 | 2025-11-15 | Decision holds. Samples used **TanStack Query v4 syntax** (`invalidateQueries(['games'])`) — v5 requires the object form, so copy-paste type-errors. Real QueryClient config differs (`retry: 1`, plus a deliberate `refetchOnWindowFocus: false` the ADR omitted). `AuthContextType.user` → real shape is `currentUser` + `isCheckingAuth`. |
| UPDATED | `adrs/006-observability-approach.md` | 483→540 | 2025-11-15 | **Decision superseded.** ADR rejected APM vendors (cost/lock-in) and OTel ("overkill"); project adopted **both** on 2026-06-04 (`ab76cdd1`) — full OTel stack → **Grafana Cloud**, plus Faro on the frontend. Logging design survived intact. `ObservabilityHandler` + `/metrics` endpoint **deleted**; `PrometheusHandler` is built but never mounted (dead code). Status → Superseded; evolution section added. |
| UPDATED | `adrs/004-api-design-principles.md` | 316 | 2025-11-15 | Principles hold; **the documented response envelope was never implemented**. No `data` wrapper (collections key on the resource name), pagination is `metadata` with different field names, errors are a flat `{status, error}` with no code/details[], and some 401s return plain text. All replaced with responses captured live. Also `{id}`→`{gameID}`, and `/auth/refresh` is **GET**. |
| UPDATED | `adrs/003-authentication-strategy.md` | 258→~310 | 2026-05-07 | **Origin of the "15-minute token" fiction** corrected 3× in Batch 3. Reality: **7 days** (`SessionLifetime`), `sub` **is** the user ID (ADR claimed it was excluded "for security"), no `iat`/`jti`, and **no separate refresh token** — `sessions.data` holds the JWT. Fabricated `sessions` schema replaced. Logout **only clears the cookie**; it does not delete the session row. Flagged `Secure: true` commented out unconditionally. |
| UPDATED | `adrs/002-database-design-approach.md` | 203→~290 | 2025-11-15 | **3 of 4 named JSONB columns do not exist.** Schema has exactly **two** JSONB columns. `character_data` is an **EAV table**, not a column; `game_config` and `action_data` never existed. All sample DDL and every JSONB query rewritten from `schema.sql` and verified against the live DB. Core decision still holds. |
| UPDATED | `adrs/001-technology-stack-selection.md` | 117 | 2025-11-15 | Choices all still hold; only versions drifted. Added a current-versions note (Go 1.25, React 19, PG 17, Vite 7, Tailwind 4, TanStack Query 5, RR7) while keeping the historical record. |
| UPDATED | `adrs/README.md` | 49 | 2025-11-15 | Index said ADR-006 "Accepted" (now Superseded); flagged 002/003 as diverged. Added a "Keeping ADRs honest" policy: never rewrite history, add a divergence section instead. |
| UPDATED | `architecture/components.md` | 632 | 2025-11-15 | **Documented a repository layer that does not exist** — `GameRepositoryInterface`/`GameRepository`, plus "Repository" in the diagram and request-flow list. Services hold the pgx pool and call sqlc directly. Also `game_config` field, `characters.character_data` INSERT, and a handler-level `validator.New()` — real validation runs in `Bind` via `core.ValidateStruct` (a service-layer reject renders as 500). |
| UPDATED | `architecture/overview.md` | 285 | 2025-11-15 | React 18→19; "JWT with refresh tokens"→single-token reality; JSONB overstated (2 columns, not a general strategy); `/metrics` endpoint → OTLP/Grafana Cloud. |
| UPDATED | `getting-started/onboarding.md` | 449 | 2025-11-15 | **Worst doc in the batch — quick start failed at step 3.** 10 of 19 `just` recipes did not exist (`just dev`, `run-frontend`, `make_migration`, `db_up`…). Also: host Go/Node prerequisites (stack is containerized), Go 1.21 (→1.25), React 18 (→19), `/ping` returns `ponger` not JSON, `cd backend` before a root-level justfile, fabricated `GameServiceInterface`/repository-layer example, `api.games.list()`, `game_config` SQL, `routes.go`, `core/models.go`, `lib/api.ts`, `/metrics`, wrong container name, and React Query DevTools recommended twice while not installed. All recipes now valid; SQL verified against the live DB. |
| UPDATED | `api/reference.md` | 156 | 2026-08-18 | `POST /auth/refresh`→**GET**; `PUT /phases/{id}/activate`→**POST**; `/actions/me`→**`/actions/mine`** (same for results); no bare `PUT /characters/{id}` (it's `/rename`, `/reassign`, `/data`); `{id}`→`{gameID}` on game routes. All verified live against the running API. |
| UPDATED | `developer/index.md` | 50 | 2026-04-22 | Linked `/developer/testing/overview` twice — **does not exist**. Repointed to real testing docs + ADR-007. |

**ADR note:** ADRs record decisions *as made at the time*. Where reality diverged, prefer adding a "Superseded / Evolution" section over rewriting history.


### Batch 5 findings (completed 2026-08-26)

**13 docs reviewed: all 13 updated.** Mechanically this batch looked the
cleanest so far — every `just` recipe at line-start resolved and every file path
existed. The damage was **semantic**: docs describing systems that were designed
but never built.

1. **ADR-006's decision was reversed.** It rejected APM vendors on cost and
   OpenTelemetry as "overkill"; on 2026-06-04 the project adopted **both** — full
   OTel → **Grafana Cloud**, plus Faro on the frontend. Status set to Superseded.
   The logging design survived intact, but `ObservabilityHandler` and the
   `/metrics` endpoint were deleted, and `PrometheusHandler` is still constructed
   with a comment claiming it "serves /metrics" while being **mounted on no
   route** — dead code.

2. **ADR-003 is the source of the "15-minute token" fiction** I corrected three
   separate times in Batch 3 without knowing where it came from. Reality: **7
   days**. Also wrong: `sub` is the **user ID** (the ADR claims it is excluded
   "for security"), there is no `iat`/`jti`, the documented `sessions` schema is
   fabricated, and **no separate refresh token exists**. The single-token design
   is deliberate and well-reasoned (`pkg/core/config.go:115-127`) — revocation
   via per-request session revalidation, not short expiry — it was simply never
   written down here.

   Two things worth acting on: `Secure: true` is **commented out
   unconditionally** on the JWT cookie, and **logout does not delete the session
   row** (it only clears the cookie), so a captured token stays valid up to 7 days.

3. **ADR-002 named four JSONB columns; three do not exist.** The schema has
   exactly **two**. `character_data` is an **EAV table**, not a JSONB column —
   a materially different design whose real trade-offs (per-field `is_public`
   visibility, application-level typing, multi-row sheet reads) the ADR never
   analyses.

4. **ADR-004's response envelope was never implemented.** No `data` wrapper,
   pagination is `metadata` with different field names, and errors are a flat
   `{status, error}` string rather than structured `details[]` — so clients
   cannot attribute a validation failure to a field. Replaced with responses
   captured live from the running API.

5. **A repository layer is documented across three docs and does not exist.**
   `components.md` had a full `GameRepositoryInterface`/`GameRepository`
   implementation section; services actually hold the pgx pool and call sqlc
   directly. sqlc's generated types *are* the domain models, so there is no
   conversion step either.

6. **`onboarding.md` was the worst doc in the batch** — the quick start failed at
   step 3. **10 of 19 `just` recipes did not exist**, prerequisites listed host
   Go/Node for a fully containerized stack, and it recommended React Query
   DevTools twice while the package is not installed.

7. **Live-verified rather than assumed.** API shapes, status codes, SQL queries,
   and route methods in this batch were checked against the running stack and
   database, not inferred from source reading. That is how the `/actions/me` →
   `/actions/mine` and `PUT` → `POST /activate` errors surfaced.

**Method note:** grepping `just <recipe>` only at line-start missed invalid
recipes embedded in prose and inside CI YAML blocks. Batch 6 should grep
unanchored.

## Batch 6 — Testing docs (`docs-site/developer/testing/`) — ✅ COMPLETE

| Status | Doc | Lines | Last Commit | Notes |
|---|---|---|---|---|
| UPDATED | `TEST_DATA.md` | 1256 | 2026-08-18 | **Fixture paths are no longer flat.** Files moved into `common/`, `demo/`, `e2e/`, `perf/`; every documented path (`test_fixtures/00_reset.sql` etc.) was wrong. Replaced a stale inline copy of `apply_all.sh` and a "add this to your justfile" block (inventing `reset-test-data`) with the real recipes. **Demo game IDs are sequence-assigned (50700+), not 1–10** — replaced the fictional 10-game table with the verified 8, keyed by title, and relabelled "Game #1–9" as scenario slots A–I. Users 7→10. Action counts corrected against the live DB (Heist 4 submissions; Starfall 8 published + 1 draft). Noted `action_submissions` has **no draft column**. |
| UPDATED | `COVERAGE_STATUS.md` | 732 | 2025-11-15 | Every metric ~10 months stale, and **self-contradictory** (claimed both 75.0% and 69.5% backend coverage). Measured: **58.4%** overall, **78–83%** service layer. Test files 16→144, test funcs 467→1131, frontend files 38→244, frontend tests 1,022→3,857. Dropped the "users.go 0.0% / No Tests" row (`users_test.go` exists). Fixed `just test-frontend`/`-watch`. |
| UPDATED | `E2E_QUICK_START.md` | 462 | 2025-11-15 | Told readers to `npm install -D @playwright/test` + `npx playwright install` — wrong for the containerized stack. Rewrote for `just e2e-*`. **Fixture claim was doubly wrong**: global-setup no longer runs `apply_all.sh` (it applies per-worker fixtures) and in the container path skips entirely via `E2E_SKIP_FIXTURE_SETUP`. Dead ref to `docs/E2E_TEST_CATALOG.md`. |
| UPDATED | `E2E_FIXTURES.md` | 203 | 2025-11-15 | Listed **10** fixture files; there are **66** (31 families). **Worker offset documented as +1000; actual is +10000.** All hardcoded game IDs (#608, #600–604, #334–338) return zero rows. 6 of 8 referenced spec files don't exist. Replaced ID tables with title-keyed `FIXTURE_GAMES` constants; corrected the `getFixtureGameId` example (resolves by **title**, not arithmetic). Fixed maintenance step claiming the loader needs editing — it globs `e2e/*.sql`. |
| UPDATED | `TEST_COVERAGE_REFERENCE.md` | 50 | 2025-11-15 | Self-declared "authoritative source" carrying stale numbers, host `npm`/`go` commands, and 2 wrong doc paths. Rewritten against measured data with container-based regeneration commands. |
| UPDATED | `frontend/e2e/README.md` | 659 | 2026-07-16 | **Deleted the 80-line Visual Regression section** — `e2e/visual/` was removed in `a439acc0` (2025-11-01), yet the doc carried a "✅ Current Coverage" checklist for it. Replaced host `npm run test:e2e*` / `npx playwright` throughout. Fixed `just dev` (doesn't exist). Fixed a `psql` example using a nonexistent `games.name` column (it's `title`) — caught by running it. |
| UPDATED | `frontend/e2e/pages/README.md` | 444 | 2025-10-26 | Documented 7 of **26** page objects with no index for the rest. **`CharacterSheetPage`'s entire documented API was obsolete** — 8 of its methods don't exist; the real API uses Notes/Skills/Inventory/Numbers tabs taking a **label**, not Abilities/Currency modules taking a count. Added a full POM index; fixed 2 dead doc links. |
| REWRITTEN | `frontend/e2e/STATUS.md` | 348 | 2025-10-25 | Point-in-time "Day 1 / Day 2 / Phase 3" progress log, structurally stale. Claimed **43** tests; actual is **345** across 49 files. Every "target for next quarter" already exceeded. Rewrote as a live status doc with generated counts + a Known Gaps section. |
| UPDATED | `.claude/skills/testing-patterns/resources/e2e-testing.md` | 583 | 2025-10-31 | *(deferred here from Batch 3)* Documented `just e2e-test headed|ui|debug` — the recipe accepts only **`headless|report|file`**. Dead spec path `e2e/journeys/critical/`. CI block replaced with the real `.github/workflows/e2e.yml` flow. |
| UPDATED | `.claude/skills/testing-patterns/resources/e2e-patterns-reference.md` | 474 | 2025-10-31 | *(deferred here from Batch 3)* Stale hardcoded `// Game #164` comment. |
| UPDATED | `.claude/skills/testing-patterns/SKILL.md` | — | — | Same invalid `just e2e-test headed`. |
| UPDATED | `.claude/context/TESTING.md` | — | — | *(Batch 1 file, invalidated by this batch's ground truth)* `just e2e-test headed`, plus `just e2e-test headless <file>` — `headless` mode takes no file argument; `file` mode does. |

### Batch 6 findings (completed 2026-08-26)

1. **Coverage numbers were ~10 months stale and internally inconsistent.** `COVERAGE_STATUS.md` asserted 75.0% and 69.5% backend coverage in the same document. Measured reality: 58.4% overall, 78–83% in the service layer. Test counts were off by 3–8×.
2. **Fixtures were reorganised into subdirectories and no doc followed.** Every `test_fixtures/NN_*.sql` path in `TEST_DATA.md` was broken; files now live under `common/`, `demo/`, `e2e/`, `perf/`.
3. **Hardcoded fixture game IDs are systematically wrong** and were the batch's most-repeated error. Demo IDs are sequence-assigned (50700+ after a reload) and E2E IDs shift by `worker * 10000`. Fixed by pointing everything at title-based resolution (`getFixtureGameId`). The convention also appears in `CLAUDE.md`, `.claude/context/TEST_DATA.md`, and `.claude/reference/API_TESTING_WITH_CURL.md` — **not yet fixed, carry to Batch 7/8**.
4. **Documented-but-deleted subsystem.** `e2e/visual/` was removed in `a439acc0` (2025-11-01); the README kept an 80-line guide with a "✅ Current Coverage" checklist. Deleted per the standing rule that annotation doesn't protect a grep-landing reader. **The `test:e2e:visual` / `:visual:update` npm scripts in `frontend/package.json` still point at the deleted directory and will fail — a code cleanup, not a doc fix.**
5. **`just e2e-test` modes were wrong in 4 files.** The recipe supports `headless|report|file`; docs advertised `headed`, `ui`, and `debug` (which need a display and can't run in the container at all).
6. **`CharacterSheetPage`'s documented API was entirely obsolete** — 8 nonexistent methods, and a module vocabulary (Abilities/Currency) the component no longer uses.
7. **Live bug found: `tags.REGRESSION` is undefined.** `auth/registration.spec.ts:53` references it but `test-tags.ts` never defines it. `tagTest()` filters the `undefined`, so the name silently renders `@auth  registration errors…` with a double space instead of a second tag. Recorded in `STATUS.md` Known Gaps.
8. **Correction to my own first pass:** I initially reported tags as entirely unused, having grepped for literal `@smoke` strings. They are applied through the `tagTest()` helper, so no literal appears in source. Real figure: **2 of 49** spec files tagged; `--grep "@smoke"` matches 6 tests. Caught by running `--list` against the container.

**Method note for Batch 7:** verifying commands by *running* them caught three errors that reading could not — the `games.name` column, the `e2e-test headless <file>` signature, and the tag-usage claim above. Prefer execution over inspection wherever a command or query is cheap to run.

## Batch 7 — `docs/` tree (deployment, operations, legacy) — ✅ COMPLETE

**This tree is not published.** VitePress builds only `docs-site/`; `docs/` is
reachable only via `CLAUDE.md` and the root `README.md`.

| Status | Doc | Lines | Notes |
|---|---|---|---|
| DELETED | `docs/features/IMPLEMENTATION_SUMMARY.md` | 416 | Completion report for state-management work integrated Oct 2025 ("Implementation completed by: Claude Code", "Next Step: Review with team"). Superseded by `STATE_MANAGEMENT.md` + ADR-005. **0 inbound links.** |
| DELETED | `docs/operations/DEPLOYMENT_SCRIPT_UPDATES.md` | 382 | Pure changelog of a single 2025-11-08 edit, down to "**Line 20**: Added `LOG_DIR`". Content is the script itself. **0 inbound links.** |
| DELETED | `docs/development/API_DOCS_PROGRESS.md` | 281 | "Achievements Today" progress log with a stale metrics table. Its one inbound link removed. |
| REWRITTEN | `docs/README.md` | 147 | **41 of 46 relative links broken.** Indexed `docs/architecture/` and `docs/adrs/` — moved to `docs-site/` long ago. Also linked `../backend/README.md`, `../frontend/docs/STATE_MANAGEMENT.md`, and `.claude/planning/completed/MVP_STATUS.md`, none of which exist. Rewritten as an index of what this tree actually holds, with a pointer to `docs-site/`. Now **0 broken of 15**. |
| REPLACED | `docs/getting-started/DEVELOPER_ONBOARDING.md` | 449→24 | Near-duplicate of the `docs-site` onboarding guide rewritten in Batch 5, but still carrying **11 invalid recipes** (`just dev`, `db_up`, `db_down`, `db_reset`, `db_exec`, `make_migration`, `migrate_test`, `run-frontend`, `test-frontend`, `test-e2e`, `docs`) and host Go/Node prerequisites. Replaced with a pointer to the maintained copy. |
| UPDATED | `docs/deployment/PRODUCTION_ENV_CHECKLIST.md` | 339 | **Would have broken a production deploy.** Claimed `CORS_ORIGINS` was "Auto-configured by user-data.sh" — it is not; `.env.docker` ships `CORS_ORIGINS=https://localhost` and nothing overrides it. Also conflated two mechanisms: `user-data.sh` generates only `JWT_SECRET`/`DOMAIN`/`ADMIN_EMAIL`, everything else comes from committed `.env.docker` defaults (incl. the `example` DB password). Added the missing OTel/Faro section and a note that `VITE_*` are build-time. Added compose-file guidance. |
| UPDATED | `docs/operations/LOGGING_STRATEGY.md` | 465 | Reads as an open proposal for something **already shipped** (Option 2, 2025-11-08). Added a status banner + divergence table: proposed `rotate 30` vs deployed **`rotate 14`**; per-directory logrotate blocks rather than a single glob; `db` container (20m×5) not covered by the proposal. |
| UPDATED | `docs/operations/DEPLOYMENT_IMPROVEMENTS.md` | 347 | Retention documented as **30 days**; actual is **14**. Docker log sizes given as a vague "50-100MB / 5-10 files" range; replaced with the real per-service table from `docker-compose.logging.yml`. Kept — unlike the other point-in-time docs, it carries real operational reference content. |
| UPDATED | `docs/operations/LOGGING_QUICK_REFERENCE.md` | 355 | Framed log persistence as opt-in "Option 2"; it has been the **default** in `deploy-production.sh` since 2025-11-08. All 9 `docker-compose` commands lacked compose-file flags, so they silently targeted `docker-compose.yml` alone. |
| UPDATED | `docs/development/API_DOCS_AUTOMATION.md` | 263 | Documented `just api-docs-generate`, which **does not exist** (only `api-docs-validate` does); the script is real but unwired. Added a warning that the validator itself is broken (see finding #2). `just dev` → `just up`. |
| UPDATED | `docs/development/API_DOCS_QUICK_START.md` | 284 | Same invalid recipe ×2, `just dev` ×2, plus the dangling `API_DOCS_PROGRESS.md` reference. |
| UPDATED | `docs/features/STATE_MANAGEMENT.md` | 1606 | Largest doc in the tree and the **most accurate** — all 7 file references and all hook exports verified. Fixed 4 instances of **TanStack Query v4 positional syntax** (`invalidateQueries(['games'])`); v5 requires the object form, which is what the real hooks use. Same defect Batch 5 fixed in ADR-005. |
| OK | `docs/deployment/SSL_BOOTSTRAP_GUIDE.md` | 281 | One broken link (`./DEPLOYMENT.md`) fixed; added a note that `scripts/renew-ssl.sh` is **generated by `setup-ssl.sh`**, not committed. Cron schedule and renewal logic verified correct. |
| OK | `docs/deployment/ROUTE53_SSL_SETUP.md` | 292 | All script references resolve. No changes. |
| OK | `docs/development/API_DOCS_PRODUCTION.md` | 300 | References resolve. No changes. |
| FIXED | `docs-site/.../onboarding.md` | — | *(Batch 5 file)* Removed a duplicated `git` bullet I left in the prerequisites list. |

### Batch 7 findings (completed 2026-08-26)

1. **A production-breaking doc error.** `PRODUCTION_ENV_CHECKLIST.md` told operators `CORS_ORIGINS` was auto-configured. It is not — the committed default is `https://localhost`, which would block every browser request from the real domain. The same section conflated Terraform-generated vars with committed `.env.docker` defaults, obscuring that the DB password ships as `example`.
2. **Live bug: `just api-docs-validate` is structurally broken.** It reports `Total registered routes: 9` and `Coverage: 700.0%`. `listRoutes` (`backend/pkg/http/debug.go:37`) walks `ctx.Routes` — the *matched* subrouter, not the root — and its walk function skips any route containing `/*`, so `chi.Walk` never descends into mounted subrouters. Verified: `/api/v1/games` returns 200 but is absent from `/api/v1/debug/routes`; ~184 routes are registered in `root.go`. **Code fix, not a doc fix.**
3. **`docs/README.md` was a near-total loss as an index** — 41 of 46 links broken, pointing at a `docs/architecture/` + `docs/adrs/` layout that moved to `docs-site/` long ago.
4. **Point-in-time changelogs, deleted.** Three docs (1,079 lines) recorded completed work as "Changes Made" / "Achievements Today" with line-number references. Two had zero inbound links.
5. **Documented config drifted from deployed config** in both logging docs: `rotate 30` vs actual `rotate 14`, and a vague size range instead of the real per-service values.
6. **A recipe documented but never created.** `just api-docs-generate` appears in 3 docs; the underlying script works, but no justfile recipe was ever added — the validator's own tip points at the bare `go run` instead.
7. **`docs/` overlaps `docs-site/` and loses.** The onboarding duplicate had 11 invalid recipes while the `docs-site` copy (fixed in Batch 5) was correct. Consolidation, not parallel maintenance, is the fix.
8. **Correction to my own work:** I reported `terraform/` as nonexistent early on — a zsh `nomatch` failure on a glob, not a real absence. It exists and is where the cron and logrotate config live.

## Batch 8 — Component-level READMEs & misc — ✅ COMPLETE

| Status | Doc | Lines | Notes |
|---|---|---|---|
| UPDATED | `frontend/src/components/ui/README.md` | 583 | **7 of 22 exported components were entirely undocumented** — `Textarea`, `Select`, `Modal`, `Drawer`, `DateTimeInput`, `HelpTooltip`, `MetadataItem`/`Group`/`Separator` — several of which CLAUDE.md instructs developers to use. Added sections for all, including two real footguns: `Textarea`/`Select` take **`textareaSize`/`selectSize`**, not `size`; and `Drawer` uses **`open`** while `Modal` uses `isOpen`. Fixed 3 wrong variant unions: Button 4→7 (`outline`, `warning`, `success` missing), Card 3→6, and Spinner's `'white'` → **`'inverse'`** (a prop value that does not exist). Replaced the "Future Components (Planned)" list — every Tier 2 entry and most of Tier 3 had already shipped and were documented *in the same file*. Design-token section verified accurate, including the retired-token table. |
| UPDATED | `CLAUDE.md` | — | Repeated the same wrong Button variant list, and an incomplete Badge list omitting its **default** (`neutral`). Added the 7 missing components. |
| UPDATED | `backend/pkg/core/UTILITIES_GUIDE.md` | 260 | **4 documented functions do not exist.** `core.HandleDBError` (only `HandleDBErrorWithID` is exported), `core.GetUsernameFromJWT` (real one is `GetUserIDFromJWT(ctx, userService) (int32, render.Renderer)` — different name, args, *and* return type), `core.ValidateStringLength` (validation is tag-based via `ValidateStruct` in `Bind`), and `Observability.Metrics.IncrementCounter` (the metrics registry was removed with the OTel migration, per ADR-006). |
| DELETED | `backend/pkg/core/MIGRATION_EDGE_CASES.md` | 236 | Point-in-time retrospective ("Week 1, Days 2-3", "Files Migrated Successfully"). Cites `characters/api.go:592-597`; that file is now **34 lines** after decomposition. Referenced the same fictional `GetUsernameFromJWT`. **0 inbound links.** |
| UPDATED | `backend/scripts/README.md` | 290 | Documented only the S3 script and the API-docs tooling; **6 files undocumented** — `api-test.sh` (14 subcommands, all verified), `dev-entrypoint.sh`, two endpoint smoke-test scripts, and the `fix_character_data_ids.sql` diagnostic. Added a section for each. API-docs tooling left untouched per your note. |
| UPDATED | `README.md` (root) | 207 | Public entry point with **8 invalid recipes** (`just dev`, `start frontend`, `start all`, `build-all all`, `db setup`, `test-frontend` ×4, `e2e-test headed`, `e2e-test ui`). Claimed React **18** (actual 19) and listed host Go/Node/PostgreSQL prerequisites for a containerized stack. Fixed 4 broken links (`docs/architecture/`, `docs/adrs/`, `docs/CONTRIBUTING.md`). **Claimed "MIT License" with no LICENSE file in the repo and no `license` field in any package.json** — replaced with a factual statement rather than inventing one. |
| REWRITTEN | `frontend/README.md` | 66 | Not Vite boilerplate, but the oldest doc in the repo (2025-08-06) and materially wrong: `just db_up` / `just run` (neither exists), host `npm install`, and "JWT authentication with **automatic token refresh**" implying a refresh-token pair. Reality is a single JWT in an HTTP-only cookie that `/auth/refresh` re-issues. Rewritten against verified versions (React 19, Router 7, Query 5, Tailwind 4, Vite 7). |
| UPDATED | `.github/workflows/README.md` | 114 | Documented **1 of 3** workflows — `e2e.yml` and `nightly.yml` absent entirely — and 5 of 6 CI jobs (missing `upload-sourcemaps`). Wrong versions: PostgreSQL 16→**17**, Node 20→**24**. Claimed "E2E tests run separately on main branch push"; they are **manual-only** (`workflow_dispatch`). Claimed artifact uploads are not in CI; `e2e.yml` uploads the Playwright report. Documented the CI type-check gap (finding #2). |
| UPDATED | `docs-site/README.md` | 212 | Documented a `/user/` tree with `getting-started`/`game-guide`/`gm-guide` subdirectories — the actual directory is a flat `/guide/`. Wrong dev-server port (5173 → **5174**). `just docs-install` does not exist. The `npm run docs:*` scripts live in `docs-site/package.json`, **not** the root `package.json`, which has no scripts at all — so the documented root-level invocations all fail. |
| OK | `docs-site/STATUS.md` | 53 | Every one of the 19 listed guide files exists; no unlisted feature docs. No changes. |
| OK | `docs-site/index.md` | 27 | VitePress home page; links are site routes and resolve. No changes. |
| N/A | `frontend/src/styles/MIGRATION_PATTERNS.md` | 475 | **Already deleted** in `82108137` ("Tons of styling fixes"). No dangling references. |
| N/A | `frontend/src/styles/CSS_VARIABLES_USAGE.md` | 298 | Same — already deleted, no dangling references. |

### Batch 8 findings (completed 2026-08-27)

1. **The UI library README — the doc CLAUDE.md leans on hardest — omitted a third of the library.** 7 of 22 exported components had zero coverage, including `Modal`, `Drawer`, `Textarea`, and `Select`. Three variant unions were wrong, and `Spinner variant="white"` was a value that has never existed.
2. **Live bug: the CI TypeScript check verified nothing — ✅ FIXED 2026-08-27.** `.github/workflows/ci.yml` ran `npx tsc --noEmit` against `frontend/tsconfig.json`, a solution-style file (`"files": []` + project references). It type-checked **zero files** and passed vacuously. Same defect as `just type-check` (fixed earlier); the justfile was fixed, CI was not. Changed to `npx tsc -b --force`. Canary-verified: with a deliberate type error present, `--noEmit` exits **0** while `tsc -b` exits **2**.
3. **~~The root README claimed an MIT license that does not exist.~~ Retracted — my error.** The file is `MIT-LICENSE`, which my `ls LICENSE*` glob missed. It is tracked, and the GitHub API confirms detection (`spdx_id: MIT`), since `MIT-LICENSE` is on GitHub's recognized-filenames list. The README link was restored to `[MIT License](MIT-LICENSE)`.
4. **Four fictional functions in `UTILITIES_GUIDE.md`**, presented as real API with `core.` prefixes and dedicated sections. `GetUsernameFromJWT` differed from the real helper in name, arguments, and return type.
5. **Version drift in CI docs**: PostgreSQL 16→17, Node 20→24. Both would mislead anyone debugging a CI-only failure.
6. **"Planned" sections describing shipped work**, again — the UI README listed 10 components as future work, all of which exist, most documented in the same file.
7. **Two docs in this batch were already deleted**, and the inventory was stale on them. Worth a pre-pass in future batches: confirm the file exists before auditing it.
8. **Correction to my own work:** I invented an `api-test.sh create-game` subcommand while writing its docs. Caught by verifying each of the 14 subcommands against the script's `case` block before finalizing.

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
