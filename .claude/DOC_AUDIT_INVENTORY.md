# Dev Documentation Audit — Inventory

**Created:** 2026-08-26
**Goal:** For each doc, assess accuracy against the actual state of the codebase and bring it back up to date.
**Scope:** Internal/dev-facing docs only. 152 tracked `.md` files (excludes `frontend/dist/`, `node_modules/`).

**Progress:** Batches 1–4 complete (62 docs). Next up: **Batch 5** — ADRs & architecture.
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
| PENDING | `.../resources/e2e-testing.md` | 583 | 2025-10-31 | Mechanical sweeps clean; deep content review deferred to Batch 6 (testing docs). |
| PENDING | `.../resources/e2e-patterns-reference.md` | 474 | 2025-10-31 | As above. |
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
