# ActionPhase Development Setup Guide

Local development is **fully containerized**. All application toolchains — Go,
Node, `migrate`, `sqlc`, `psql` — run *inside* containers. You do **not** install
them on your host.

## Quick Start (5 minutes)

```bash
# 1. Clone and navigate to project
git clone <repository-url>
cd actionphase

# 2. First-time setup: creates .env, builds images, starts the stack
just dev-setup

# 3. That's it. Migrations auto-run on backend startup.
#    Load test data if you want it:
just test-data reload
```

- Frontend (Vite + HMR): `http://localhost:5173`
- Backend (API, Air hot-reload): `http://localhost:3000`
- Postgres: `localhost:5432`
- Delve debugger: `localhost:2345`

Run `just dev-help` any time for the workflow cheatsheet.

## Prerequisites

The **only** host requirements are:

- **Docker & Docker Compose** — runs the entire stack
  ```bash
  # macOS: install Docker Desktop (includes compose)
  brew install --cask docker

  # Ubuntu/Debian
  sudo apt install docker.io docker-compose-plugin
  ```

- **Just** — task runner
  ```bash
  brew install just   # macOS
  # Other platforms: https://github.com/casey/just#installation
  ```

There is **nothing else to install** — no Go, Node, `migrate`, `sqlc`, or
`psql` on the host. Those live in the container images (`backend/Dockerfile.dev`,
`frontend/Dockerfile.dev`). `.tool-versions` is kept only as documentation of
the versions the images pin.

## Containerized Workflow

The dev stack is defined in `docker-compose.dev.yml` (separate from the
production `docker-compose.yml`):

| Service | What it runs | Notes |
|---------|-------------|-------|
| `db` | Postgres 17 | data in the `pgdata` volume |
| `backend` | Air live-reload under Delve | rebuilds Go on save (~1–2s) |
| `frontend` | Vite dev server | HMR over the bind mount |
| `playwright` | E2E runner (profile `e2e`) | only starts for `just e2e*` |

### Everyday commands

```bash
just up              # start the stack (add `build` to rebuild images)
just down            # stop it (data preserved in volumes)
just ps              # container status
just status          # health + migrations + git overview
just dev-logs [svc]  # tail logs (backend|frontend|db)
just dev-restart svc # restart one service (config changes / wedged container)
just sh [svc]        # shell into a container for one-off commands
just rebuild [svc]   # rebuild image(s) after Dockerfile/dependency changes
```

Code changes hot-reload automatically — edit a `.go` or frontend file on the
host and the running container picks it up. No manual restart for code changes.

### Running tests, migrations, codegen

These all exec inside the containers (the stack must be up — `just up`):

```bash
just test            # full backend suite (integration + mocks)
just test-mocks      # fast, DB-less unit tests
just test-frontend   # frontend component tests
just migrate         # apply migrations (also auto-runs on backend boot)
just sqlgen          # regenerate Go from SQL (sqlc)
just lint            # go fmt + vet
just lint-frontend   # eslint
```

## Debugging the Backend (Delve)

The backend runs under a headless [Delve](https://github.com/go-delve/delve)
server on port **2345** (`--continue`, so the app runs without waiting for an
attach; `--accept-multiclient`, so you can detach/reattach across Air rebuilds).

### VS Code

A ready config ships in `.vscode/launch.json`:

1. `just up`
2. Run the **"Attach to backend (Docker/Delve)"** launch configuration.
3. Set breakpoints. If an Air rebuild detaches the session, re-run the config.

### GoLand / IntelliJ

Create a **Go Remote** run configuration:

- Host: `localhost`
- Port: `2345`

Then **Run → Debug** it after `just up`. Under *Path Mappings*, map your local
`backend/` directory to `/app` in the container so breakpoints resolve.

> Note: builds use `-gcflags "all=-N -l"` (optimizations/inlining disabled) so
> breakpoints and variable inspection map cleanly to source.

## Environment Setup

### 1. Environment Variables (.env file)

ActionPhase uses environment variables for configuration. A working `.env` file is included in the repository with secure defaults for local development.

#### Default .env Configuration

The provided `.env` file includes:
```bash
# Database (works with docker-compose.yml)
DATABASE_URL="postgres://postgres:example@localhost:5432/actionphase?sslmode=disable"
TEST_DATABASE_URL="postgres://postgres:example@localhost:5432/actionphase_test?sslmode=disable"

# JWT Authentication
JWT_SECRET="dev-jwt-secret-key-not-for-production-use-only-12345"

# Application Settings
ENVIRONMENT=development
LOG_LEVEL=info
PORT=3000
```

#### Customizing Environment Variables

1. **For different database credentials:**
   ```bash
   # Edit .env file
   DATABASE_URL="postgres://your-user:your-password@localhost:5432/your-database"
   ```

2. **For production deployment:**
   ```bash
   cp .env.example .env.production
   # Edit .env.production with production values
   # Use strong JWT_SECRET, set ENVIRONMENT=production, etc.
   ```

3. **For different ports/hosts:**
   ```bash
   PORT=8080
   HOST=127.0.0.1
   ```

#### Environment Variable Reference

| Variable | Default | Purpose |
|----------|---------|---------|
| `DATABASE_URL` | *See .env* | Main database connection string |
| `TEST_DATABASE_URL` | *See .env* | Test database connection string |
| `JWT_SECRET` | *Dev key* | JWT token signing key (change for production!) |
| `ENVIRONMENT` | `development` | Deployment environment (development/staging/production) |
| `LOG_LEVEL` | `info` | Logging verbosity (debug/info/warn/error) |
| `PORT` | `3000` | HTTP server port |
| `HOST` | `0.0.0.0` | HTTP server bind address |
| `RUN_MIGRATIONS` | `true` | Auto-run migrations on startup |
| `SKIP_DB_TESTS` | `false` | Skip database-dependent tests |

## Database Setup

### Docker-Based Database (Recommended)

ActionPhase uses PostgreSQL in Docker for local development, making setup consistent across all platforms.

The `db` service starts with `just up`; the `actionphase` database is created
automatically by the container. The test database and migrations are handled by
the commands below (`psql`/`migrate` run inside the backend container).

```bash
just db create      # create actionphase_test (actionphase is auto-created)
just migrate        # apply migrations (also auto-runs on backend startup)
just migration status   # show current migration version
```

### Database Commands Reference

| Command | Purpose |
|---------|---------|
| `just db up` | Start the Postgres container |
| `just db down` | Stop the Postgres container |
| `just db reset` | Wipe the data volume and recreate the DB |
| `just db create` | Create the `actionphase_test` database |
| `just db setup` | Start + create databases |
| `just migrate` | Apply migrations to the dev database |
| `just migrate_test` | Apply migrations to the test database |
| `just migration status` | Show migration version |
| `just migration rollback` | Roll back the last migration |
| `just reset_test_db` | Drop + recreate + re-migrate the test DB (when it gets dirty) |

## Development Workflow

### Starting Development

```bash
# First time: create .env, build images, start the stack
just dev-setup

# Subsequently: just bring the stack up
just up

# Load test data (optional)
just test-data reload
```

Everything runs in containers from here — see the **Containerized Workflow** and
**Everyday commands** sections above.

### Development Commands

| Command | Purpose |
|---------|---------|
| `just dev-setup` | First-time setup (.env + build + start) |
| `just up` | Start the containerized stack |
| `just down` | Stop the stack |
| `just status` | Health + migrations + git overview |
| `just build` | Compile-check the backend (in container) |
| `just tidy` | `go mod tidy` (in container) |
| `just sh backend` | Shell into the backend container |

## Testing

ActionPhase has a comprehensive testing strategy with both mock-based unit tests and database integration tests.

### Test Types

#### 1. Mock Tests (Fast - ~0.3 seconds)
```bash
# Run only mock-based unit tests (no database required)
just test-mocks

# Perfect for TDD and rapid development
# Uses in-memory mocks, works without any dependencies
```

#### 2. Integration Tests (Slower - several seconds)
```bash
# Run tests with real database
just test-integration

# Requires PostgreSQL and test database setup
# Tests full request/response flows
```

#### 3. All Tests
```bash
# Run complete test suite
just test

# Automatically skips database tests if DB unavailable
# Run in parallel for speed: just test-parallel
```

### Test Setup

#### Database Tests Setup
```bash
# One-time setup for database tests
just test-db-setup

# This creates test database and applies migrations
# Run this before your first integration test
```

#### Environment Variables for Testing

```bash
# Skip database tests entirely
SKIP_DB_TESTS=true just test

# Use different test database
TEST_DATABASE_URL="postgres://localhost/my_test_db" just test

# Run with debug logging
TEST_LOG_LEVEL=debug just test
```

### Test Commands Reference

| Command | Speed | Database Required | Purpose |
|---------|-------|-------------------|---------|
| `just test-mocks` | ⚡ Fastest | ❌ No | Unit tests with mocks |
| `just test-integration` | 🐢 Slow | ✅ Yes | Integration tests |
| `just test` | 🐢 Slow | ⚠️ Optional | All tests |
| `just test-parallel` | ⚡ Fast | ⚠️ Optional | All tests in parallel |
| `just test-coverage` | 🐢 Slow | ⚠️ Optional | Tests with coverage report |
| `just test-db-setup` | - | ✅ Yes | Setup test database |

## Code Quality

### Linting & Formatting

```bash
# Format Go code
just fmt

# Run Go vet
just vet

# Run both formatting and vetting
just lint

# Build project (includes compile-time checks)
just build
```

### Database Code Generation

ActionPhase uses SQLC for type-safe database queries:

```bash
# Generate Go code from SQL queries
just sqlgen

# Run after modifying files in backend/pkg/db/queries/
# Generated files go to backend/pkg/db/models/
```

### Migration Management

```bash
# Create new migration
just make_migration add_user_preferences

# Apply migrations
just migrate

# Check status
just migrate_status

# Rollback (careful!)
just rollback
```

## Troubleshooting

### Common Issues

#### Database Connection Errors

**Problem:** `connection refused` or `database does not exist`

**Solution:**
```bash
# 1. Ensure Docker is running
docker ps

# 2. Start database
just db_up

# 3. Wait a moment, then create databases
just db_create

# 4. Apply migrations
just migrate
```

#### Tests Failing

**Problem:** Database test failures

**Solution:**
```bash
# Use mock tests during development
just test-mocks

# Or skip database tests
SKIP_DB_TESTS=true just test

# For integration tests, ensure test DB is set up
just test-db-setup
```

#### Environment Variables Not Loading

**Problem:** Application not finding configuration

**Solution:**
```bash
# 1. Ensure .env file exists
ls -la .env

# 2. If missing, copy from example
cp .env.example .env

# 3. Check environment loading
go run main.go
# Should see: "✓ Loaded environment from: /path/to/.env"
```

#### Port Already in Use

**Problem:** `address already in use` error

**Solution:**
```bash
# Find process using port 3000
lsof -i :3000

# Kill the process (replace PID)
kill <PID>

# Or use different port in .env
PORT=3001
```

### Getting Help

#### Check Application Status
```bash
just status
# Shows git status, database status, and Go modules
```

#### View All Available Commands
```bash
just --list
# Or just: just help
```

#### Log Levels
```bash
# Debug mode (verbose logging)
LOG_LEVEL=debug just dev

# Quiet mode (errors only)
LOG_LEVEL=error just dev
```

### Database Utilities

#### Connect to Database Directly
```bash
# Connect to main database
psql -h localhost -U postgres actionphase
# Password: example

# Connect to test database
psql -h localhost -U postgres actionphase_test
```

#### Reset Database (Nuclear Option)
```bash
# Stop database, remove data, start fresh
just db_down
docker volume rm actionphase_pgdata  # WARNING: Destroys all data
just db_setup
just migrate
```

## Production Considerations

### Environment Variables for Production

1. **Create production .env**:
   ```bash
   cp .env.example .env.production
   ```

2. **Update critical settings**:
   ```bash
   # Strong JWT secret (32+ characters)
   JWT_SECRET="$(openssl rand -base64 32)"

   # Production database URL
   DATABASE_URL="postgres://user:pass@db-host:5432/prod_db?sslmode=require"

   # Production environment
   ENVIRONMENT=production

   # Disable auto-migrations in production
   RUN_MIGRATIONS=false
   ```

3. **Security considerations**:
   - Use strong, unique JWT secret
   - Enable SSL for database connections (`sslmode=require`)
   - Set appropriate CORS origins
   - Use appropriate log levels (`warn` or `error`)

### Deployment Notes

- **Migrations**: Run manually in production (`RUN_MIGRATIONS=false`)
- **Environment**: Never commit real `.env` files to version control
- **Secrets**: Use proper secret management systems
- **Database**: Use managed PostgreSQL services (AWS RDS, etc.)

## Next Steps

After successful setup:

1. **Explore the API**: Backend runs at `http://localhost:3000`
2. **Check health**: Visit `http://localhost:3000/ping`
3. **Review code**: Start with `backend/pkg/core/` for domain models
4. **Run tests**: `just test-mocks` for fast feedback loop
5. **Database management**: Use `just migrate` for schema changes
6. **Frontend**: Set up frontend development (see frontend documentation)

## Architecture Overview

- **Backend**: Go with Chi router, PostgreSQL, JWT auth
- **Database**: PostgreSQL with SQLC for type-safe queries
- **Configuration**: Environment-based with .env support
- **Testing**: Mock-based unit tests + database integration tests
- **Development**: Docker-based database, auto-reload server

For more details, see [BACKEND_ARCHITECTURE.md](BACKEND_ARCHITECTURE.md).

---

## Quick Reference Card

```bash
# Setup (one-time)
just dev-setup && just migrate

# Daily development
just dev                    # Start backend
just test-mocks            # Run fast tests
just db_up                 # Start database
just migrate              # Apply new migrations

# Troubleshooting
just status               # Check everything
just --list              # See all commands
SKIP_DB_TESTS=true just test  # Skip DB tests
```

**Happy coding! 🚀**
