# Docs Update — Code-to-Doc Mapping

Lookup table used by [`/docs-update`](./docs-update.md) Pass 2. Maps changed
code areas to the documentation that describes them.

**This file is append-only by design.** When `/docs-update` runs against a diff
that touches something not covered here, the correct response is to find the
affected docs and **add a row** — not to skip the area. Growth is the point;
each row is drift that will be caught automatically next time.

## Conventions

- **Changed path** — glob or directory, relative to repo root.
- **Docs to check** — every doc describing that area. Paths must resolve.
- **Watch for** — the specific drift this pairing historically produces.

Rows are grouped by area. Within a group, order does not matter.

---

## Backend — domain & architecture

| Changed path | Docs to check | Watch for |
|---|---|---|
| `backend/pkg/core/interfaces.go` | `.claude/reference/BACKEND_ARCHITECTURE.md`, `.claude/context/ARCHITECTURE.md` | Interface signatures quoted inline; the interface-first workflow |
| `backend/pkg/core/*.go` (domain models) | `.claude/context/ARCHITECTURE.md`, `.claude/skills/backend-dev-guidelines/SKILL.md` | Struct fields listed in prose; stale file paths after a split |
| `backend/pkg/core/UTILITIES_GUIDE.md` helpers | `backend/pkg/core/UTILITIES_GUIDE.md` | Functions documented that no longer exist |
| `backend/pkg/db/services/**` | `.claude/reference/BACKEND_ARCHITECTURE.md` | Service decomposition — `phases/`, `actions/`, `messages/` are multi-file packages |
| `backend/pkg/http/root.go` | `.claude/reference/API_DOCUMENTATION.md`, `docs-site/developer/architecture/adrs/004-api-design-principles.md` | Added/removed/renamed routes; changed auth middleware on a route |
| `backend/pkg/auth/**` | `docs-site/developer/architecture/adrs/003-authentication-strategy.md` | Single JWT in an HTTP-only cookie — there is no separate refresh token |
| Error handling patterns | `.claude/reference/ERROR_HANDLING.md` | Sentinel vs. wrapped error guidance |
| Logging calls | `.claude/reference/LOGGING_STANDARDS.md` | Correlation-ID conventions; field naming |

## Backend — data

| Changed path | Docs to check | Watch for |
|---|---|---|
| `backend/pkg/db/migrations/**` | `docs-site/developer/architecture/adrs/002-database-design-approach.md`, `.claude/context/TEST_DATA.md` | Schema quoted inline; column renames; new nullable columns absent from fixtures |
| `backend/pkg/db/queries/**` | `.claude/reference/BACKEND_ARCHITECTURE.md` | The `just sqlgen` step in the documented workflow |
| `backend/pkg/db/test_fixtures/**` | `docs-site/developer/testing/TEST_DATA.md`, `docs-site/developer/testing/E2E_FIXTURES.md`, `.claude/context/TEST_DATA.md` | Fixture game IDs/titles; worker offsets; the `common/`+`demo/`+`e2e/`+`perf/` layout |
| Test DB setup / template cloning | `.claude/context/TESTING.md`, `docs-site/developer/architecture/adrs/007-testing-strategy.md` | Per-package DB cloning — older docs describe one shared test DB |

## Game domain

| Changed path | Docs to check | Watch for |
|---|---|---|
| Game state machine / transitions | `.claude/skills/game-domain/resources/game-states.md`, `docs-site/guide/game-states.md` | A new state missing from the list (`just check-game-states` catches this) |
| Phase logic | `.claude/skills/game-domain/resources/phase-system.md`, `.claude/skills/game-domain/SKILL.md` | `is_published` means "GM published action results", **not** phase visibility |
| Messaging / conversations | `.claude/skills/game-domain/resources/messaging-system.md` | Conversation types; audience visibility rules |
| Notifications | `.claude/skills/game-domain/SKILL.md` | `context_*` = container (bulk clear); `related_*` = the item (preview) |

## Frontend

| Changed path | Docs to check | Watch for |
|---|---|---|
| `frontend/src/components/ui/**` | `frontend/src/components/ui/README.md`, `CLAUDE.md` (component list + variant unions) | Components missing from the list; variant unions drifting from the source type |
| `frontend/src/hooks/**`, `frontend/src/contexts/**` | `.claude/context/STATE_MANAGEMENT.md`, `docs-site/developer/architecture/adrs/005-frontend-state-management.md` | TanStack Query **v5** object-form filters, not v4 positional args |
| `frontend/src/lib/api/**` | `.claude/reference/API_DOCUMENTATION.md`, `.claude/context/STATE_MANAGEMENT.md` | Stale `lib/api.ts` path — it is split per domain |
| Styling, Tailwind tokens | `.claude/context/FRONTEND_STYLING.md`, `frontend/src/components/ui/README.md` | Retired `bg-bg-*` / `border-border-*` tokens — they emit no CSS |
| `frontend/src/lib/faro.ts`, instrumentation | `docs-site/developer/architecture/adrs/006-observability-approach.md` | Faro postdates the original ADR text |
| `package.json` deps | `frontend/README.md`, `README.md` | Pinned version numbers quoted in prose |

## Testing

| Changed path | Docs to check | Watch for |
|---|---|---|
| `frontend/e2e/**` | `frontend/e2e/README.md`, `docs-site/developer/testing/E2E_QUICK_START.md` | Container-based `just e2e-*` invocation, never host `npx playwright` |
| `frontend/e2e/pages/**` | `frontend/e2e/pages/README.md` | POM method signatures documented but never implemented |
| `frontend/e2e/fixtures/**` | `frontend/e2e/README.md`, `docs-site/developer/testing/E2E_FIXTURES.md` | `FIXTURE_GAMES` keys; games resolved by **title**, not hardcoded ID |
| `frontend/e2e/fixtures/test-tags.ts` | `.github/workflows/README.md` | Tags applied via `tagTest()`, so no literal `@smoke` appears in source |
| Test infrastructure generally | `.claude/context/TESTING.md`, `.claude/skills/testing-patterns/SKILL.md`, `docs-site/developer/architecture/adrs/007-testing-strategy.md` | The test-pyramid ordering; E2E last |

## Tooling & infrastructure

| Changed path | Docs to check | Watch for |
|---|---|---|
| `justfile` | `CLAUDE.md`, `.claude/reference/JUSTFILE_QUICK_REFERENCE.md`, `.claude/hooks/README.md`, `README.md` | **Highest-drift file in the repo.** Any renamed recipe breaks several docs at once |
| `docker-compose*.yml` | `.claude/reference/DEVELOPMENT_SETUP.md`, `docs/deployment/PRODUCTION_ENV_CHECKLIST.md` | Production stacks three compose files; bare `docker-compose` uses only the first |
| `terraform/**` | `docs/operations/LOGGING_STRATEGY.md`, `docs/operations/LOGGING_QUICK_REFERENCE.md` | Logrotate retention and cron schedules are authoritative **here**, not in prose |
| `.github/workflows/**` | `.github/workflows/README.md` | Job counts; action versions; `tsc -b` vs the vacuous `tsc --noEmit` |
| `.claude/hooks/**` | `.claude/hooks/README.md` | Which hooks are active vs. unused scripts |
| `backend/scripts/**` | `backend/scripts/README.md`, `.claude/reference/API_TESTING_WITH_CURL.md` | Subcommands documented that the script's `case` block does not implement |
| `.env.example` | `docs/deployment/PRODUCTION_ENV_CHECKLIST.md`, `.claude/reference/DEVELOPMENT_SETUP.md` | New required vars missing from the checklist |

---

## Adding a row

1. Find every doc describing the changed area:
   `git grep -ln '<concept>' -- '*.md' ':!docs-site/.vitepress/dist'`
2. Confirm each path resolves — a broken path here makes Pass 2 silently skip.
3. Put "Watch for" in terms of the *specific* mistake, not a generic reminder.
   "Fixture game IDs" is useful; "check it's correct" is not.
