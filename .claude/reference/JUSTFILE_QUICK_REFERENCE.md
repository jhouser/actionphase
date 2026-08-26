# Justfile Quick Reference

Quick reference for the actual justfile commands available in ActionPhase.

**Last Verified**: August 2026 — regenerated from `just --list`.

> **Local dev is fully containerized.** The host needs only `just`, `docker`, and
> `docker-compose`. Every app command below execs inside a container. There is no
> `just dev` or `just build` — the stack runs via `just up`, and code hot-reloads
> (Air for Go, Vite HMR for the frontend).

## 🚀 Quick Start

```bash
just dev-setup      # First-time: create .env, build images, start the stack
just up             # Subsequently: start db + backend + frontend
just dev-help       # Containerized-dev cheatsheet
just ps             # Container status
just status         # Full system status check
```

Stack URLs: frontend `http://localhost:5173`, backend `http://localhost:3000`,
Postgres `localhost:5432`, Delve `localhost:2345`.

## 📦 Stack Lifecycle

```bash
just up [build]         # Start the stack ('build' forces a rebuild)
just down               # Stop (containers removed, data preserved)
just restart [service]  # Restart backend|frontend|db|all (default: backend)
just rebuild [service]  # Rebuild a service image
just dev-logs [service] # Tail logs; omit service for all
just sh [service]       # Shell into a container (default: backend)
```

`just sh` is the escape hatch for one-off `go` / `npm` / `psql` commands — use it
instead of running those on the host.

## 🗄️ Database

```bash
just db <action>              # up, down, reset, create, setup
just migrate                  # Apply migrations to the dev database
just migration create <name>  # Create a new up/down migration pair
just migration status         # Show current migration version
just migration rollback       # Roll back the last migration
just migration test           # Apply migrations to the test database
just reset-test-db            # Rebuild test DB + migrated template
just sqlgen                   # Generate Go code from SQL queries
```

> `just make_migration` and `just migrate_status` do **not** exist.

### Test Data

```bash
just test-data reload   # Reset and reload test data
just test-fixtures      # Apply fixtures to the dev database
just load-common        # Common base data (users + config)
just load-demo          # Demo data for staging/showcase
just load-e2e           # E2E fixtures (worker-specific, for parallel runs)
just load-all           # Everything (dev only)
```

## 🧪 Testing

### Backend

```bash
just test               # All backend tests (sets SKIP_DB_TESTS=false itself)
just test-mocks         # Fast mock tests, no database
just test-integration   # Database service integration tests only
just test-run <pattern> # Single test by name (passed to `go test -run`)
just test-coverage      # Coverage report → backend/coverage.out
just test-race          # Race detector
just test-clean         # Clean test cache
```

Each test **package** clones its own DB from a migrated template, so packages run
in parallel. Do not add `-p=1`; set `TEST_P=1` only to debug.

### Frontend

```bash
just test-fe run [file]   # Run frontend tests (optionally one file)
just test-fe watch        # Watch mode
just lint-frontend        # eslint
just type-check           # tsc
just knip                 # Dead-export detection
just relock-frontend      # Regenerate package-lock.json in a Linux container
```

> `just test-frontend` does **not** exist — it is `just test-fe run`.

### E2E

```bash
just e2e                       # Desktop + mobile (sequential)
just e2e-desktop               # Chrome only
just e2e-mobile                # Pixel 5 only
just e2e-test headless         # Headless run
just e2e-test file <path>      # A single spec — ONLY `file` mode takes a path
```

> Passing a path to `headless` silently runs the **whole suite**. Headed/ui/debug
> modes need a display and are host-only.

### Everything

```bash
just test-all   # Backend + frontend
just ci-test    # Full CI suite (lint + test + race)
just verify       # pre-push gate: all checks + backend & frontend builds (parallel)
just verify-quick # fast non-mutating checks, no builds (what the Stop hook runs)
```

## 🛠️ Code Quality

```bash
just fmt         # Format Go code
just vet         # go vet
just lint        # fmt + vet + cross-tree consistency checks
just tidy        # go.mod maintenance
just dead-code   # Unreachable Go code
just clean       # Clean build artifacts and caches
```

## 📚 Documentation

```bash
just docs-dev       # Docs dev server (http://localhost:5174)
just docs-build     # Build the docs site
just docs-preview   # Preview the build (http://localhost:5175)
just docs-embed     # Build + embed docs into the backend
just api-docs-validate  # Validate API documentation completeness
```

## 🚢 Production

```bash
just deploy [no-cache]                    # Deploy on the server (from /opt/actionphase)
just prod-logs [target] [lines] [follow]  # backend|frontend|nginx|postgres|all
just prod-log-grep <pattern> [level] [lines]
```

## 🔌 API Testing

Not a justfile command — use the script directly:

```bash
./backend/scripts/api-test.sh login-player   # token → /tmp/api-token.txt

curl -s -H "Authorization: Bearer $(cat /tmp/api-token.txt)" \
  "http://localhost:3000/api/v1/games" | jq '.'
```

## 📝 Notes

- Database name is `actionphase` (not `database`)
- From the host: `postgres://postgres:example@localhost:5432/actionphase`
- Inside the compose network the host is `db`, not `localhost`
- Backend port 3000, frontend port 5173
- Test fixtures use password: `testpassword123`

## Full Listing (from `just --list`)

```
api-docs-validate                                # Validate API documentation completeness (in backend container)
check-game-states                                # host bash — contributors on Windows get the same check as everyone else.
ci-test                                          # Run CI test suite
claude                                           # Launch Claude Code editor
clean                                            # Clean build artifacts and caches (in containers)
db action="help"                                 # Database operations on the dev stack: up, down, reset, create, setup
dead-code                                        # Find unreachable/dead code in backend (excludes test helpers and mocks)
deploy no_cache=""                               # Deploy latest changes on this server (run from /opt/actionphase); use 'just deploy no-cache' to force full rebuild
dev-help                                         # Print the containerized-dev workflow cheatsheet.
dev-logs service=""                              # Tail logs for a dev service (backend, frontend, db). Omit for all services.
dev-setup                                        # Complete first-time dev setup: create .env, then build images + start the container stack.
docs-build                                       # Build documentation site
docs-dev                                         # Start documentation development server (http://localhost:5174)
docs-embed                                       # Build and embed documentation in backend
docs-preview                                     # Preview built documentation (http://localhost:5175)
down                                             # Stop the dev stack (containers removed, volumes/data preserved).
e2e                                              # Run E2E tests on both desktop and mobile (sequential to avoid fixture conflicts)
e2e-desktop                                      # Run E2E tests on desktop only (Chrome)
e2e-mobile                                       # Run E2E tests on mobile only (Pixel 5)
e2e-test mode="headless" file=""                 # Note: headed/ui/debug need a display and are host-only — see 'just dev-help'.
fmt                                              # Format Go code
help                                             # Show available commands
knip                                             # Dead-export detection (in frontend container)
lint                                             # Run backend linters (fmt + vet) plus cross-tree consistency checks
lint-frontend                                    # Lint frontend code (in frontend container)
load-all                                         # Load all data (dev only) - same as test-fixtures but with new structure
load-common                                      # Load only common base data (users and config)
load-demo                                        # Load demo data for staging/showcase
load-e2e                                         # Load E2E test fixtures (worker-specific for parallel execution)
migrate                                          # Apply migrations to development database (in backend container)
migration action="" name=""                      # Migration operations: create, status, rollback, test (runs in backend container)
prod-log-grep pattern="" level="all" lines="200" # Examples: just prod-log-grep "user_id" | just prod-log-grep "" error | just prod-log-grep "correlation_id" all 500
prod-logs target="backend" lines="50" follow="false" # Targets: backend (default), frontend, nginx, postgres, all
ps                                               # Show status of the dev stack containers.
rebuild service=""                               # volume here so the fresh image re-seeds it on the next `just up`.
relock-frontend                                  # Regenerate frontend/package-lock.json in a Linux container (run after dep changes).
reset-test-db                                    # Use when the test DB gets into a dirty/broken migration state.
restart service="backend"                        # Pass a service name (backend|frontend|db) or 'all' to restart the whole stack.
sh service="backend"                             # Full-purity escape hatch: run one-off go/npm/psql commands in-container.
sqlgen                                           # Generate SQL code using sqlc (in backend container)
status                                           # Complete system status check (containerized dev stack)
test                                             # parallel safely (no -p=1). Set TEST_P (e.g. TEST_P=1) to override concurrency.
test-all                                         # Run full test suite (backend + frontend)
test-clean                                       # Clean test cache
test-coverage                                    # Run tests with coverage report
test-data action="reload"                        # Reset and reload test data
test-fe mode="run" file=""                       # Interactive modes (watch/ui) use a TTY-attached exec.
test-fixtures                                    # Apply test data fixtures to development database
test-integration                                 # Run database service integration tests only
test-mocks                                       # Run fast mock tests only (no database required)
test-race                                        # Run tests with race detector
test-run pattern                                 # Run specific test by name
tidy                                             # Go module maintenance
type-check                                       # TypeScript type-check (in frontend container)
up build=""                                      # Start the full dev stack (db + backend + frontend). Add 'build' to force a rebuild.
verify                                           # Pre-push gate: all code-quality checks + backend/frontend production builds (parallel)
verify-quick                                     # Fast non-mutating checks, no builds (Stop hook)
build                                            # Build backend + frontend
build-backend                                    # Compile the Go binary
build-frontend                                   # Production frontend build (tsc -b && vite build)
tidy-check                                       # go mod tidy -diff (non-mutating)
fmt-check                                        # gofmt -l (non-mutating)
vet                                              # Run Go vet
```
