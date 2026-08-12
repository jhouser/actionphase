# ActionPhase Development Commands
#
# Quick Reference:
#   just help          - Show this help
#   just up            - Start the containerized dev stack
#   just test          - Run backend tests
#   just test-fe       - Run frontend tests
#   just e2e           - Run E2E tests
#
# For detailed help on any command with subcommands, run:
#   just <command> help

set windows-shell := ["pwsh.exe", "-c"]

# Show available commands
help:
  @just --list

# Deploy latest changes on this server (run from /opt/actionphase); use 'just deploy no-cache' to force full rebuild
deploy no_cache="":
  #!/usr/bin/env bash
  if [ "{{no_cache}}" = "no-cache" ]; then
    NO_CACHE=true ./scripts/deploy-production.sh
  else
    ./scripts/deploy-production.sh
  fi

# Launch Claude Code editor
claude:
    CLAUDE_CONFIG_DIR="$HOME/.claude-personal" ~/.local/bin/claude

# ═══════════════════════════════════════════════════════════════════════════
# CONTAINERIZED DEV (docker-compose.dev.yml)
# ═══════════════════════════════════════════════════════════════════════════
# Fully containerized local dev: db + backend (Air hot-reload + Delve) + frontend
# (Vite HMR). No host Go/Node needed. See "just dev-help" for the full workflow.

# Compose invocation for the dev stack — reused by every dev recipe below.
DEV_COMPOSE := if os_family() == "windows" { "docker-compose -f docker-compose.dev.yml" } else { "docker compose -f docker-compose.dev.yml" }

# Start the full dev stack (db + backend + frontend). Add 'build' to force a rebuild.
up build="":
  @just _up-{{os_family()}} {{build}}

_up-windows build="":
  #!pwsh.exe
  if ("{{build}}" -eq "build") {
    Write-Host "Building"
    {{DEV_COMPOSE}} up -d --build
  }
  else {
    Write-Host "Running"
    {{DEV_COMPOSE}} up -d
  }
  Write-Host "
    🚀 Dev stack up:
      Frontend (Vite):  http://localhost:5173
      Backend  (API):   http://localhost:3000
      Delve debugger:   localhost:2345  (attach from IDE)
      Postgres:         localhost:5432

      Logs:    just dev-logs [backend|frontend|db]
      Shell:   just sh backend   (run go/npm commands in-container)"

_up-unix build="":
  #!/usr/bin/env bash
  if [ "{{build}}" = "build" ]; then
    {{DEV_COMPOSE}} up -d --build
  else
    {{DEV_COMPOSE}} up -d
  fi
  echo ""
  echo "🚀 Dev stack up:"
  echo "   Frontend (Vite):  http://localhost:5173"
  echo "   Backend  (API):   http://localhost:3000"
  echo "   Delve debugger:   localhost:2345  (attach from IDE)"
  echo "   Postgres:         localhost:5432"
  echo ""
  echo "   Logs:    just dev-logs [backend|frontend|db]"
  echo "   Shell:   just sh backend   (run go/npm commands in-container)"

# Stop the dev stack (containers removed, volumes/data preserved).
down:
  {{DEV_COMPOSE}} down

# Rebuild images from scratch (after Dockerfile.dev / dependency changes).
rebuild service="":
  {{DEV_COMPOSE}} build --no-cache {{service}}
  @echo "✅ Rebuilt. Run 'just up' to (re)start."

# Tail logs for a dev service (backend, frontend, db). Omit for all services.
dev-logs service="":
  {{DEV_COMPOSE}} logs -f {{service}}

# Restart dev service(s) for config changes or a wedged container (code hot-reloads).
# Pass a service name (backend|frontend|db) or 'all' to restart the whole stack.
restart service="backend":
  @just _restart-{{os_family()}} {{service}}
  
_restart-windows service="backend":
  #!pwsh.exe
  if ( "{{service}}" -eq "all" ) {
    {{DEV_COMPOSE}} restart
    Write-Host "✅ Restarted all services"
  }
  else {
    {{DEV_COMPOSE}} restart {{service}}
    Write-Host "✅ Restarted {{service}}"
  }
  
_restart-unix service="backend":
  #!/usr/bin/env bash
  if [ "{{service}}" = "all" ]; then
    {{DEV_COMPOSE}} restart
    echo "✅ Restarted all services"
  else
    {{DEV_COMPOSE}} restart {{service}}
    echo "✅ Restarted {{service}}"
  fi

# Open a shell in a running dev container (backend, frontend, db).
# Full-purity escape hatch: run one-off go/npm/psql commands in-container.
sh service="backend":
  {{DEV_COMPOSE}} exec {{service}} sh

# Show status of the dev stack containers.
ps:
  {{DEV_COMPOSE}} ps

# Print the containerized-dev workflow cheatsheet.
dev-help:
  @just _dev-help-{{os_family()}}

_dev-help-windows:
  #!pwsh.exe
  Write-Host "Containerized dev workflow (no host Go/Node required)
  ─────────────────────────────────────────────────────
    just up [build]     Start db + backend + frontend (build to rebuild images)
    just down           Stop the stack (data preserved in volumes)
    just ps             Container status
    just dev-logs [svc] Tail logs (backend|frontend|db; blank = all)
    just restart [svc]  Restart a service (backend|frontend|db|all)
    just rebuild [svc]  Rebuild image(s) from scratch after Dockerfile changes
    just sh [svc]       Shell into a container (backend|frontend|db)

  URLs
    Frontend  http://localhost:5173   (Vite HMR; proxies /api -> backend)
    Backend   http://localhost:3000   (Air rebuilds on save)
    Delve     localhost:2345          (attach IDE debugger — see docs)
    Postgres  localhost:5432

  Hot reload is automatic: edit a .go or frontend file on the host and the
  running container picks it up. No manual restart needed for code changes."

_dev-help-unix:
  #!/usr/bin/env bash
  cat <<'EOF'
  Containerized dev workflow (no host Go/Node required)
  ─────────────────────────────────────────────────────
    just up [build]     Start db + backend + frontend (build to rebuild images)
    just down           Stop the stack (data preserved in volumes)
    just ps             Container status
    just dev-logs [svc] Tail logs (backend|frontend|db; blank = all)
    just restart [svc]  Restart a service (backend|frontend|db|all)
    just rebuild [svc]  Rebuild image(s) from scratch after Dockerfile changes
    just sh [svc]       Shell into a container (backend|frontend|db)

  URLs
    Frontend  http://localhost:5173   (Vite HMR; proxies /api -> backend)
    Backend   http://localhost:3000   (Air rebuilds on save)
    Delve     localhost:2345          (attach IDE debugger — see docs)
    Postgres  localhost:5432

  Hot reload is automatic: edit a .go or frontend file on the host and the
  running container picks it up. No manual restart needed for code changes.
  EOF

# ═══════════════════════════════════════════════════════════════════════════
# DATABASE COMMANDS
# ═══════════════════════════════════════════════════════════════════════════

# Database operations on the dev stack: up, down, reset, create, setup
db action="help":
  @just _db-{{os_family()}} {{action}}

_db-windows action="help":
  #!pwsh.exe
  DC='docker compose -f docker-compose.dev.yml'
  switch ("{{action}}") {
    "up" { {{DEV_COMPOSE}} up -d db }
    "down" { {{DEV_COMPOSE}} stop db }
    "reset" { 
      {{DEV_COMPOSE}} rm -sf db
      docker volume rm actionphase-dev_pgdata 2>/dev/null || true
      {{DEV_COMPOSE}} up -d db
      Write-Host "✅ Database reset (fresh volume). Run 'just migrate' to apply schema."
    }
    "create" {
      # The 'actionphase' db is created by the container (POSTGRES_DB). This
      # just adds the test database. Runs psql inside the backend container.
      Write-Host "Creating actionphase_test database (actionphase is auto-created)..."
      {{DEV_COMPOSE}} sh -c 'PGPASSWORD=example psql -h db -U postgres -d postgres -c "CREATE DATABASE actionphase_test;"' 2>/dev/null || echo "  (actionphase_test already exists)"
      Write-Host "✅ Databases ready"
    }
    "setup" {
      just db up
      just db create
      Write-Host "Database setup complete! Migrations auto-run on backend startup."
    }
    "help" {
      Write-Host "Usage: just db [action]
      
      Actions:
        up        Start database container
        down      Stop database container
        reset     Reset database (wipe volume + recreate)
        create    Create the test database (dev db is auto-created)
        setup     Full database setup (up + create)"
    }
  }

_db-unix action="help":
  #!/usr/bin/env bash
  DC='docker compose -f docker-compose.dev.yml'
  case "{{action}}" in
    up)
      $DC up -d db
      ;;
    down)
      $DC stop db
      ;;
    reset)
      # Wipe the dev db volume and recreate from scratch.
      $DC rm -sf db
      docker volume rm actionphase-dev_pgdata 2>/dev/null || true
      $DC up -d db
      echo "✅ Database reset (fresh volume). Run 'just migrate' to apply schema."
      ;;
    create)
      # The 'actionphase' db is created by the container (POSTGRES_DB). This
      # just adds the test database. Runs psql inside the backend container.
      echo "Creating actionphase_test database (actionphase is auto-created)..."
      {{BE}} sh -c 'PGPASSWORD=example psql -h db -U postgres -d postgres -c "CREATE DATABASE actionphase_test;"' 2>/dev/null \
        || echo "  (actionphase_test already exists)"
      echo "✅ Databases ready"
      ;;
    setup)
      just db up
      just db create
      echo "Database setup complete! Migrations auto-run on backend startup."
      ;;
    help|*)
      echo "Usage: just db [action]"
      echo ""
      echo "Actions:"
      echo "  up        Start database container"
      echo "  down      Stop database container"
      echo "  reset     Reset database (wipe volume + recreate)"
      echo "  create    Create the test database (dev db is auto-created)"
      echo "  setup     Full database setup (up + create)"
      ;;
  esac

# ═══════════════════════════════════════════════════════════════════════════
# MIGRATION COMMANDS
# ═══════════════════════════════════════════════════════════════════════════

# All migration/db tooling runs inside the backend container (has migrate, psql,
# sqlc, and the source mounted). Inside the compose network the db is at db:5432.
BE := if os_family() == "windows" { "docker-compose -f docker-compose.dev.yml exec -T backend" } else { "docker compose -f docker-compose.dev.yml exec -T backend" }
FE := if os_family() == "windows" { "docker-compose -f docker-compose.dev.yml exec -T frontend" } else { "docker compose -f docker-compose.dev.yml exec -T frontend" }
DEV_DB_URL := "postgres://postgres:example@db:5432/actionphase?sslmode=disable"
TEST_DB_URL := "postgres://postgres:example@db:5432/actionphase_test?sslmode=disable"
# Env overrides for the test process. The backend container sets
# ENVIRONMENT=development (via .env), which flips on dev-mode bypasses in the
# app (e.g. registration rate limiting / uniqueness checks are skipped). Tests
# assert the production code paths, so force ENVIRONMENT=test when running them.
# TEST_DATABASE_URL points at the base test DB; TEST_TEMPLATE_DB tells the Go
# harness which migrated template to clone a private per-package database from.
# When TEST_TEMPLATE_DB is set, NewTestDatabase provisions an isolated DB per test
# binary, so packages can run concurrently (no -p=1 needed).
TEST_ENV := "ENVIRONMENT=test TEST_DATABASE_URL=\"" + TEST_DB_URL + "\" TEST_TEMPLATE_DB=" + TEST_TEMPLATE_DB + " SKIP_DB_TESTS=false"

# Migration operations: create, status, rollback, test (runs in backend container)
migration action="" name="":
  #!/usr/bin/env bash
  case "{{action}}" in
    create)
      if [ -z "{{name}}" ]; then
        echo "❌ Migration name required"
        echo "Usage: just migration create <name>"
        exit 1
      fi
      {{BE}} migrate create -ext sql -dir pkg/db/migrations {{name}}
      ;;
    status)
      {{BE}} migrate -source file://pkg/db/migrations -database "{{DEV_DB_URL}}" version
      ;;
    rollback)
      {{BE}} migrate -source file://pkg/db/migrations -database "{{DEV_DB_URL}}" down
      ;;
    test)
      {{BE}} migrate -source file://pkg/db/migrations -database "{{TEST_DB_URL}}" up
      ;;
    help|*)
      echo "Usage: just migration [action]"
      echo ""
      echo "Actions:"
      echo "  create <name>    Create new migration"
      echo "  status           Show migration status"
      echo "  rollback         Rollback last migration"
      echo "  test             Apply migrations to test database"
      ;;
  esac

# Apply migrations to development database (in backend container)
migrate:
  {{BE}} migrate -source file://pkg/db/migrations -database "{{DEV_DB_URL}}" up

# Drop and recreate the test database from scratch, then apply all migrations.
# Use when the test DB gets into a dirty/broken migration state.
reset-test-db:
  #!/usr/bin/env bash
  echo "Dropping test database..."
  {{BE}} sh -c 'PGPASSWORD=example psql -h db -U postgres -d postgres -c "DROP DATABASE IF EXISTS actionphase_test;"'
  echo "Creating test database..."
  {{BE}} sh -c 'PGPASSWORD=example psql -h db -U postgres -d postgres -c "CREATE DATABASE actionphase_test;"'
  echo "Applying migrations..."
  just migration test
  echo "✅ Test database reset complete"

# Name of the migrated template DB that per-package test databases are cloned from.
TEST_TEMPLATE_DB := "actionphase_test_template"
TEST_TEMPLATE_URL := "postgres://postgres:example@db:5432/" + TEST_TEMPLATE_DB + "?sslmode=disable"

# Rebuild the migrated test template DB and sweep leftover per-package clones.
# Each test binary (package) clones this template into its own database at runtime
# (see NewTestDatabase), so packages never share rows/sequences and can run in
# parallel. Run before a test suite; cheap because migrations are fast.
_prepare-test-template:
  #!/usr/bin/env bash
  set -euo pipefail
  # Drop any leftover per-package clones from previous (possibly interrupted) runs,
  # then rebuild the template so it always matches current migrations. The sweep uses
  # psql's \gexec to run one DROP per clone server-side (DROP DATABASE can't run inside
  # a DO/function body), so it is robust to how many clones exist with no shell loop.
  {{BE}} sh -c 'PGPASSWORD=example psql -h db -U postgres -d postgres -q -v ON_ERROR_STOP=1 <<'"'"'SQL'"'"'
  SELECT format('"'"'DROP DATABASE IF EXISTS %I WITH (FORCE)'"'"', datname)
  FROM pg_database WHERE datname LIKE '"'"'actionphase_test_p%'"'"'
  \gexec
  DROP DATABASE IF EXISTS {{TEST_TEMPLATE_DB}} WITH (FORCE);
  CREATE DATABASE {{TEST_TEMPLATE_DB}};
  SQL'
  {{BE}} migrate -source file://pkg/db/migrations -database "{{TEST_TEMPLATE_URL}}" up
  echo "✅ Test template ready ({{TEST_TEMPLATE_DB}})"

# ═══════════════════════════════════════════════════════════════════════════
# TEST DATA COMMANDS
# ═══════════════════════════════════════════════════════════════════════════

# Fixture scripts run inside the backend container (source is mounted there,
# and psql reaches the db service at host 'db'). DB_HOST=db is passed to each.

# Apply test data fixtures to development database
test-fixtures:
  @echo "Applying test data fixtures..."
  {{BE}} env DB_HOST=db bash pkg/db/test_fixtures/apply_all.sh
  @echo "✅ Test data loaded successfully!"

# Reset and reload test data
test-data action="reload":
  #!/usr/bin/env bash
  case "{{action}}" in
    reset)
      echo "Resetting test data..."
      {{BE}} sh -c 'PGPASSWORD=example psql -h db -U postgres -d actionphase -f pkg/db/test_fixtures/00_reset.sql'
      echo "✅ Test data reset complete"
      ;;
    reload)
      echo "Resetting test data..."
      {{BE}} sh -c 'PGPASSWORD=example psql -h db -U postgres -d actionphase -f pkg/db/test_fixtures/00_reset.sql'
      echo "Applying test data fixtures..."
      {{BE}} env DB_HOST=db bash pkg/db/test_fixtures/apply_all.sh
      echo "🎉 Test data reloaded!"
      echo ""
      echo "Test Accounts Available:"
      echo "  GM: test_gm@example.com / testpassword123"
      echo "  Players: test_player1@example.com through test_player5@example.com / testpassword123"
      echo "  Audience: test_audience@example.com / testpassword123"
      ;;
    *)
      echo "Usage: just test-data [action]"
      echo ""
      echo "Actions:"
      echo "  reset     Reset test data only"
      echo "  reload    Full reset and reload (default)"
      ;;
  esac

# Load only common base data (users and config)
load-common:
  @echo "🧹 Loading common base data..."
  {{BE}} env DB_HOST=db DB_NAME=actionphase bash pkg/db/test_fixtures/apply_common.sh
  @echo "✅ Common data loaded (users only, no games)"

# Load demo data for staging/showcase
load-demo:
  @echo "🎭 Loading demo showcase data..."
  {{BE}} env DB_HOST=db DB_NAME=actionphase bash pkg/db/test_fixtures/apply_demo.sh
  @echo "✅ Demo data loaded (rich, human-friendly content)"

# Load E2E test fixtures (worker-specific for parallel execution)
load-e2e:
  #!/usr/bin/env bash
  set -euo pipefail
  echo "🤖 Loading E2E test fixtures for parallel execution (6 workers)..."
  echo "📦 Applying common fixtures..."
  {{BE}} env DB_HOST=db DB_NAME=actionphase bash pkg/db/test_fixtures/apply_common.sh
  echo "👥 Creating E2E parallel worker users..."
  {{BE}} env DB_HOST=db DB_NAME=actionphase bash pkg/db/test_fixtures/apply_e2e_users.sh
  echo "🔧 Applying worker-specific fixtures..."
  for i in 0 1 2 3 4 5; do
    echo "  Worker $i..."
    {{BE}} env DB_HOST=db DB_NAME=actionphase bash pkg/db/test_fixtures/apply_e2e_worker.sh $i > /dev/null 2>&1
  done
  echo "✅ E2E fixtures loaded for 6 parallel workers (isolated test games)"

# Load all data (dev only) - same as test-fixtures but with new structure
load-all:
  #!/usr/bin/env bash
  set -euo pipefail
  echo "⚠️  Loading ALL data (demo + E2E)..."
  just load-demo
  just load-e2e
  echo "✅ All data loaded (not recommended for staging)"

# ═══════════════════════════════════════════════════════════════════════════
# CODE GENERATION
# ═══════════════════════════════════════════════════════════════════════════

# Generate SQL code using sqlc
# Generate SQL code using sqlc (in backend container)
sqlgen:
  {{BE}} sh -c 'cd pkg/db && sqlc generate'

# ═══════════════════════════════════════════════════════════════════════════
# GO BACKEND COMMANDS  (all run inside the backend container)
# ═══════════════════════════════════════════════════════════════════════════

# Go module maintenance
tidy:
  {{BE}} go mod tidy

# Format Go code
fmt:
  {{BE}} go fmt ./...

# Run Go vet
vet:
  #!/usr/bin/env bash
  # Exclude pkg/docs/dist (embedded frontend assets)
  packages=$({{BE}} go list ./... | grep -v '/pkg/docs/dist' | tr '\n' ' ')
  if [ -n "$packages" ]; then
    {{BE}} go vet $packages
  else
    echo "No packages to vet"
  fi

# Run backend linters (fmt + vet)
lint: fmt vet
  @echo "Go linting complete"

# Find unreachable/dead code in backend (excludes test helpers and mocks)
dead-code:
  #!/usr/bin/env bash
  output=$({{BE}} deadcode ./... 2>&1 | grep -v \
    "pkg/core/test_\|pkg/core/mocks\|pkg/core/repository_mocks\|pkg/db/services/test_suite\|pkg/http/test_helpers\|pkg/core/test_best_practices" || true)
  if [ -n "$output" ]; then echo "$output"; exit 1; fi

# TypeScript type-check (in frontend container)
type-check:
  {{FE}} npx tsc --noEmit

# Dead-export detection (in frontend container)
knip:
  {{FE}} npx knip

# Run all code-quality checks (tidy + lint + dead-code + frontend lint + type-check + knip)
verify:
  @echo "Verifying code quality..."
  @just tidy
  @just lint
  @just dead-code
  @just lint-frontend
  @just type-check
  @just knip
  @echo "✅ Code quality verified"

# ═══════════════════════════════════════════════════════════════════════════
# DEVELOPMENT WORKFLOW
# ═══════════════════════════════════════════════════════════════════════════

# Complete first-time dev setup: create .env, then build images + start the container stack.
dev-setup:
  #!/usr/bin/env bash
  echo "Setting up containerized development environment..."
  if [ ! -f .env ]; then cp .env.example .env; echo "✓ Created .env from .env.example"; fi
  echo "Building images and starting the stack..."
  just up build
  echo ""
  echo "🎉 Dev environment ready. Migrations auto-run on backend startup."
  echo "   Load test data with: just test-data reload"

# ═══════════════════════════════════════════════════════════════════════════
# BACKEND TESTING
# ═══════════════════════════════════════════════════════════════════════════

# Helper function to clean test database
_clean_test_db:
  #!/usr/bin/env bash
  echo "🧹 Cleaning actionphase_test database for integration tests..."
  {{BE}} sh -c 'PGPASSWORD=example psql -h db -U postgres -d actionphase_test -q -c "
  DO \$\$
  DECLARE
      r RECORD;
  BEGIN
      FOR r IN (SELECT tablename FROM pg_tables WHERE schemaname = '"'"'public'"'"') LOOP
          EXECUTE '"'"'TRUNCATE TABLE '"'"' || quote_ident(r.tablename) || '"'"' RESTART IDENTITY CASCADE'"'"';
      END LOOP;
  END \$\$;
  "' 2>&1 | grep -v "NOTICE" || true
  echo "✅ Test database cleaned"

# Run all backend tests (default: everything with database)
# Each package clones its own DB from the migrated template, so packages run in
# parallel safely (no -p=1). Set TEST_P (e.g. TEST_P=1) to override concurrency.
test:
  @echo "🧪 Running all backend tests (integration + mocks)..."
  @just _prepare-test-template
  {{BE}} env {{TEST_ENV}} go test ./...

# Run fast mock tests only (no database required)
test-mocks:
  @echo "⚡ Running mock tests only (fast, parallel)..."
  {{BE}} env ENVIRONMENT=test SKIP_DB_TESTS=true go test ./...

# Run database service integration tests only
test-integration:
  @echo "🗄️  Running database integration tests..."
  @just _prepare-test-template
  {{BE}} env {{TEST_ENV}} go test ./pkg/db/services/...

# Run tests with coverage report
test-coverage:
  @echo "📊 Running all tests with coverage..."
  @just _prepare-test-template
  {{BE}} env {{TEST_ENV}} go test -coverprofile=coverage.out ./...
  @echo ""
  @echo "Coverage report generated: backend/coverage.out"
  @{{BE}} go tool cover -func=coverage.out | tail -1

# Run tests with race detector
test-race:
  @echo "🔍 Running tests with race detector..."
  @just _prepare-test-template
  {{BE}} env {{TEST_ENV}} CGO_ENABLED=1 go test -race ./...

# Clean test cache
test-clean:
  {{BE}} go clean -testcache
  @echo "✅ Test cache cleaned"

# Run specific test by name
test-run pattern:
  @echo "🎯 Running tests matching: {{pattern}}"
  @just _prepare-test-template
  {{BE}} env {{TEST_ENV}} go test -v -run {{pattern}} ./...

# ═══════════════════════════════════════════════════════════════════════════
# FRONTEND TESTING
# ═══════════════════════════════════════════════════════════════════════════

# Regenerate frontend/package-lock.json inside a Linux container (matches CI).
# ALWAYS use this instead of a bare `npm install` on macOS: some deps
# (@oxc-parser, etc.) carry platform-specific optional-dependency subtrees that
# npm only resolves into the lockfile on Linux, so a local install produces a
# lockfile that fails `npm ci` in CI. Run this whenever you change frontend deps.
#
# The container runs in an isolated temp copy of ONLY package.json +
# package-lock.json, so it can never touch your local node_modules (a Linux
# `npm ci`/`install` against a mounted tree would clobber your macOS native
# bindings, e.g. @oxc-parser/binding-darwin-arm64, and break `just knip`).
#
# Regenerate frontend/package-lock.json in a Linux container (run after dep changes).
relock-frontend:
  #!/usr/bin/env bash
  set -euo pipefail
  echo "🔒 Regenerating frontend/package-lock.json in node:24 (linux)..."
  workdir=$(mktemp -d)
  trap 'rm -rf "$workdir"' EXIT
  cp frontend/package.json frontend/package-lock.json "$workdir/"
  docker run --rm -v "$workdir":/app -w /app node:24 \
    npm install --package-lock-only
  cp "$workdir/package-lock.json" frontend/package-lock.json
  echo "✅ Lockfile regenerated. Review the diff and commit frontend/package-lock.json"

# Frontend testing with options (runs in frontend container)
# Interactive modes (watch/ui) use a TTY-attached exec.
test-fe mode="run" file="":
  #!/usr/bin/env bash
  FE='docker compose -f docker-compose.dev.yml exec -T frontend'
  FE_IT='docker compose -f docker-compose.dev.yml exec frontend'
  # `npm test` is bare `vitest`, which watches unless told otherwise. The
  # non-interactive modes pass `--run` explicitly rather than relying on the
  # absence of a TTY to imply it.
  case "{{mode}}" in
    run|file)
      # A path narrows the run to matching files; without one the whole suite
      # runs. `file` is kept as an alias so `just test-fe file <path>` still
      # works, but it's the same thing as passing a path to `run`.
      if [ "{{mode}}" = "file" ] && [ -z "{{file}}" ]; then
        echo "❌ File path required for file mode"
        echo "Usage: just test-fe file path/to/test.tsx"
        exit 1
      fi
      $FE npx vitest run {{file}}
      ;;
    watch)
      $FE_IT npm run test:watch -- {{file}}
      ;;
    coverage)
      $FE npx vitest run --coverage {{file}}
      ;;
    ui)
      $FE_IT npm run test:ui -- {{file}}
      ;;
    *)
      echo "Usage: just test-fe [mode] [file]"
      echo ""
      echo "Modes:"
      echo "  run [path]      Run tests once (default), optionally filtered to path"
      echo "  watch [path]    Run tests in watch mode"
      echo "  coverage [path] Run tests with coverage report"
      echo "  ui [path]       Run tests with interactive UI"
      echo "  file <path>     Run specific test file (alias for: run <path>)"
      echo ""
      echo "Examples:"
      echo "  just test-fe"
      echo "  just test-fe run src/components/Foo.test.tsx"
      echo "  just test-fe watch src/hooks"
      ;;
  esac

# ═══════════════════════════════════════════════════════════════════════════
# E2E TESTING
# ═══════════════════════════════════════════════════════════════════════════

# E2E runs in the Playwright container (profile "e2e"), driving the running
# frontend/backend services. Fixtures are loaded first via `just load-e2e`
# (backend container), then Playwright runs with in-process setup skipped.
# --rm cleans up the one-shot container; the stack must be up (just up).
PW := "docker compose -f docker-compose.dev.yml --profile e2e run --rm playwright"

# Run E2E tests on both desktop and mobile (sequential to avoid fixture conflicts)
e2e:
  @just e2e-desktop
  @just e2e-mobile

# Run E2E tests on mobile only (Pixel 5)
e2e-mobile:
  @echo "🔄 Applying E2E test fixtures..."
  @just load-e2e
  @echo ""
  {{PW}} npx playwright test --project=mobile-chrome

# Run E2E tests on desktop only (Chrome)
e2e-desktop:
  @echo "🔄 Applying E2E test fixtures..."
  @just load-e2e
  @echo ""
  {{PW}} npx playwright test --project=chromium

# E2E testing with options (runs in Playwright container)
# Note: headed/ui/debug need a display and are host-only — see 'just dev-help'.
e2e-test mode="headless" file="":
  #!/usr/bin/env bash
  PW='docker compose -f docker-compose.dev.yml --profile e2e run --rm playwright'
  echo "🔄 Applying E2E test fixtures..."
  just load-e2e
  echo ""
  case "{{mode}}" in
    headless)
      $PW npx playwright test
      ;;
    report)
      $PW npx playwright show-report --host 0.0.0.0
      ;;
    file)
      if [ -z "{{file}}" ]; then
        echo "❌ File path required for file mode"
        echo "Usage: just e2e-test file path/to/test.spec.ts"
        exit 1
      fi
      $PW npx playwright test {{file}}
      ;;
    headed|ui|debug)
      echo "❌ '{{mode}}' needs a display and can't run headless in the container."
      echo "   For visual/interactive E2E, run Playwright on the host against the"
      echo "   containerized app: cd frontend && E2E_NO_WEBSERVER=true npx playwright test --{{mode}}"
      exit 1
      ;;
    *)
      echo "Usage: just e2e-test [mode] [file]"
      echo ""
      echo "Modes:"
      echo "  headless    Run headless (default)"
      echo "  report      Show HTML test report"
      echo "  file <path> Run specific test file"
      echo "  headed/ui/debug  (host-only — needs a display)"
      ;;
  esac

# ═══════════════════════════════════════════════════════════════════════════
# PRODUCTION LOGGING  (run on the prod server; reads /opt/actionphase/logs)
# ═══════════════════════════════════════════════════════════════════════════
# For containerized DEV logs use `just dev-logs [svc]` instead.

# View production logs: just prod-logs [target] [lines] [follow]
# Targets: backend (default), frontend, nginx, postgres, all
prod-logs target="backend" lines="50" follow="false":
  #!/usr/bin/env bash
  LOG_DIR=/opt/actionphase/logs
  case "{{target}}" in
    backend)
      LOG_FILE="$LOG_DIR/backend/app.log"
      if [ "{{follow}}" = "true" ]; then
        tail -f "$LOG_FILE" 2>/dev/null | jq . 2>/dev/null || tail -f "$LOG_FILE"
      else
        tail -n {{lines}} "$LOG_FILE" 2>/dev/null | jq . 2>/dev/null || tail -n {{lines}} "$LOG_FILE"
      fi
      ;;
    frontend)
      LOG_FILE="$LOG_DIR/frontend/access.log"
      if [ "{{follow}}" = "true" ]; then
        tail -f "$LOG_FILE"
      else
        tail -n {{lines}} "$LOG_FILE"
      fi
      ;;
    nginx)
      LOG_FILE="$LOG_DIR/nginx/access.log"
      if [ "{{follow}}" = "true" ]; then
        tail -f "$LOG_FILE"
      else
        tail -n {{lines}} "$LOG_FILE"
      fi
      ;;
    postgres)
      LOG_FILE=$(ls -t "$LOG_DIR"/postgres/postgresql-*.log 2>/dev/null | head -1)
      if [ -z "$LOG_FILE" ]; then echo "No postgres log file found"; exit 1; fi
      if [ "{{follow}}" = "true" ]; then
        tail -f "$LOG_FILE"
      else
        tail -n {{lines}} "$LOG_FILE"
      fi
      ;;
    all)
      echo "=== Backend (last {{lines}} lines) ==="
      tail -n {{lines}} "$LOG_DIR/backend/app.log" 2>/dev/null | jq -c . 2>/dev/null || tail -n {{lines}} "$LOG_DIR/backend/app.log" 2>/dev/null
      echo ""
      echo "=== Nginx (last {{lines}} lines) ==="
      tail -n {{lines}} "$LOG_DIR/nginx/access.log" 2>/dev/null
      echo ""
      echo "=== Frontend (last {{lines}} lines) ==="
      tail -n {{lines}} "$LOG_DIR/frontend/access.log" 2>/dev/null
      ;;
    *)
      echo "Usage: just prod-logs [target] [lines] [follow]"
      echo "Targets: backend (default), frontend, nginx, postgres, all"
      ;;
  esac

# Search production backend logs: just prod-log-grep [pattern] [level] [lines]
# level: all (default), error, warn, info, debug
# Examples: just prod-log-grep "user_id" | just prod-log-grep "" error | just prod-log-grep "correlation_id" all 500
prod-log-grep pattern="" level="all" lines="200":
  #!/usr/bin/env bash
  LOG_FILE=/opt/actionphase/logs/backend/app.log
  if [ ! -f "$LOG_FILE" ]; then echo "Backend log not found: $LOG_FILE"; exit 1; fi
  CMD="tail -n {{lines}} \"$LOG_FILE\""
  if [ -n "{{pattern}}" ]; then
    CMD="$CMD | grep -i '{{pattern}}'"
  fi
  if [ "{{level}}" != "all" ]; then
    LEVEL=$(echo "{{level}}" | tr '[:lower:]' '[:upper:]')
    CMD="$CMD | grep '\"level\":\"$LEVEL\"'"
  fi
  eval "$CMD" | jq . 2>/dev/null || eval "$CMD"

# ═══════════════════════════════════════════════════════════════════════════
# STATUS & HEALTH
# ═══════════════════════════════════════════════════════════════════════════

# Complete system status check (containerized dev stack)
status:
  @echo "═══════════════════════════════════════════════════════════"
  @echo "            ActionPhase System Status"
  @echo "═══════════════════════════════════════════════════════════"
  @echo ""
  @echo "=== Containers ==="
  @docker compose -f docker-compose.dev.yml ps
  @echo ""
  @echo "=== Migrations ==="
  @just migration status 2>/dev/null || echo "❌ Database connection failed (is the stack up? just up)"
  @echo ""
  @echo "=== API Health ==="
  @printf "Health endpoint: "
  @curl -sf http://localhost:3000/health > /dev/null 2>&1 && echo "✅ Healthy" || echo "❌ Down"
  @printf "Frontend (Vite): "
  @curl -sf http://localhost:5173/ > /dev/null 2>&1 && echo "✅ Serving" || echo "❌ Down"
  @echo ""
  @echo "=== Git Status ==="
  @git status --short | head -10
  @echo ""
  @echo "═══════════════════════════════════════════════════════════"

# ═══════════════════════════════════════════════════════════════════════════
# CLEANUP
# ═══════════════════════════════════════════════════════════════════════════

# Clean build artifacts and caches (in containers)
clean:
  @echo "Cleaning build artifacts..."
  @{{BE}} go clean -testcache
  @{{BE}} go clean
  @{{FE}} sh -c 'rm -rf node_modules/.cache dist 2>/dev/null || true'
  @echo "✅ Cleanup complete"

# ═══════════════════════════════════════════════════════════════════════════
# CI/CD
# ═══════════════════════════════════════════════════════════════════════════

# Run CI test suite
ci-test: lint
  @just test-race
  @just test-fe
  @echo "✅ CI test suite complete"

# Run full test suite (backend + frontend)
test-all:
  @just test
  @just test-fe
  @echo "✅ All tests complete"

# ═══════════════════════════════════════════════════════════════════════════
# FRONTEND PACKAGE MANAGEMENT
# ═══════════════════════════════════════════════════════════════════════════

# Frontend deps live in the image (installed at build time). To change deps:
# edit package.json, then `just relock-frontend` and `just rebuild frontend`.

# Lint frontend code (in frontend container)
lint-frontend:
  {{FE}} npm run lint

# ═══════════════════════════════════════════════════════════════════════════
# DOCUMENTATION  (VitePress; runs in an ephemeral node container)
# ═══════════════════════════════════════════════════════════════════════════

# Ephemeral node container for the separate docs-site npm project.
DOCS_RUN := "docker run --rm -v " + justfile_directory() + "/docs-site:/docs -w /docs node:24"

# Start documentation development server (http://localhost:5174)
docs-dev:
  docker run --rm -it -p 5174:5174 -v "{{justfile_directory()}}/docs-site":/docs -w /docs node:24 \
    sh -c 'npm install && npm run docs:dev -- --host --port 5174'

# Build documentation site
docs-build:
  {{DOCS_RUN}} sh -c 'npm install && npm run docs:build'
  @echo "✅ Documentation built to docs-site/.vitepress/dist"

# Preview built documentation (http://localhost:5175)
docs-preview:
  docker run --rm -it -p 5175:5175 -v "{{justfile_directory()}}/docs-site":/docs -w /docs node:24 \
    sh -c 'npm run docs:preview -- --host --port 5175'

# Build and embed documentation in backend
docs-embed: docs-build
  @echo "📦 Embedding documentation in backend..."
  rm -rf backend/pkg/docs/dist
  cp -r docs-site/.vitepress/dist backend/pkg/docs/dist
  @echo "✅ Documentation embedded at backend/pkg/docs/dist"
  @echo "🔧 Restart the backend to include updated docs: just restart backend"

# Validate API documentation completeness (in backend container)
api-docs-validate:
  {{BE}} go run scripts/validate-api-docs.go
