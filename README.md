# ActionPhase

[![CI](https://github.com/RallinaTricolor/actionphase/actions/workflows/ci.yml/badge.svg)](https://github.com/RallinaTricolor/actionphase/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/RallinaTricolor/actionphase/branch/master/graph/badge.svg)](https://codecov.io/gh/RallinaTricolor/actionphase)

A modern turn-based gaming platform built with Go and React.

## Features

- 🎮 Turn-based game management
- 👥 Character creation and management
- 💬 Real-time messaging and discussions
- 📊 Game phase management
- 🔐 Secure JWT-based authentication
- 📱 Responsive design with dark mode
- 🎯 Action submission and results system

## Tech Stack

**Backend:**
- Go 1.25+ with Chi router
- PostgreSQL 17 with sqlc
- JWT authentication
- Clean Architecture patterns

**Frontend:**
- React 19 with TypeScript
- Vite for build tooling
- TanStack Query (React Query)
- Tailwind CSS
- Vitest + Playwright for testing

## Quick Start

### Prerequisites

Local development is **fully containerized** — you do not need Go, Node, or
PostgreSQL on the host:

- Docker & Docker Compose
- [just](https://github.com/casey/just) command runner
- git

(For reference, the containers run Go 1.25, Node 24, and PostgreSQL 17.)

### Installation

```bash
git clone https://github.com/RallinaTricolor/actionphase.git
cd actionphase

# First-time setup: create .env, build images, start the stack
just dev-setup

# Load demo data (optional)
just test-data reload
```

Migrations run automatically when the backend boots.

### Development

```bash
just up                  # Start db + backend + frontend
just down                # Stop (data preserved)
just ps                  # Container status
just dev-logs backend    # Tail a service's logs

# Tests
just test                # Backend tests
just test-fe run         # Frontend tests
just e2e                 # E2E (desktop + mobile)

# Linting
just lint                # Backend
just lint-frontend       # Frontend
```

Code hot-reloads: edit a `.go` file (Air rebuilds in ~1-2s) or a frontend file
(Vite HMR) on the host and the container picks it up.

Visit:
- **Frontend**: http://localhost:5173
- **Backend API**: http://localhost:3000
- **API Docs**: http://localhost:3000/api/v1/docs

### Test Accounts

After loading test data (`just load-demo`):

- **GM**: test_gm@example.com / testpassword123
- **Players**: test_player1@example.com through test_player5@example.com / testpassword123
- **Audience**: test_audience@example.com / testpassword123

## Documentation

- **[Developer Onboarding](docs-site/developer/getting-started/onboarding.md)** - 30-minute setup guide
- **[Architecture Overview](docs-site/developer/architecture/overview.md)** - System design
- **[ADRs](docs-site/developer/architecture/adrs/)** - Architecture Decision Records
- **[Testing Guide](.claude/context/TESTING.md)** - Testing patterns and requirements
- **[Operations & Deployment](docs/README.md)** - Deployment, logging, env checklist
- **[API Documentation](http://localhost:3000/api/v1/docs)** - OpenAPI/Swagger docs (when server running)

Run `just docs-dev` to preview the documentation site locally.

## Project Structure

```
actionphase/
├── backend/          # Go backend
│   ├── pkg/
│   │   ├── core/     # Domain models and interfaces
│   │   ├── db/       # Database queries and services
│   │   ├── http/     # API handlers and middleware
│   │   └── ...
│   └── main.go
├── frontend/         # React frontend
│   ├── src/
│   │   ├── components/
│   │   ├── contexts/
│   │   ├── hooks/
│   │   ├── lib/
│   │   └── pages/
│   └── e2e/         # Playwright E2E tests
├── docs/            # Documentation
└── .claude/         # AI development context
```

## Development Commands

See the [justfile](justfile) for all available commands:

```bash
just --list            # Show all commands
just dev-help          # Development cheatsheet

# Stack lifecycle
just up                # Start the stack
just down              # Stop the stack
just ps                # Container status

# Database
just migrate           # Run migrations (also automatic on backend boot)
just test-data reload  # Reset + load demo/E2E fixtures
just migration create <name>   # New migration

# Testing
just test              # Backend tests
just test-mocks        # Fast unit tests (no DB)
just test-fe run       # Frontend tests
just e2e               # E2E tests
just test-coverage     # Backend coverage report

# Linting & verification
just lint              # Backend lint
just lint-frontend     # Frontend lint
just verify            # Full pre-push gate (lint + type-check + builds)

# Build
just build             # Backend + frontend production builds
```

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Write tests for your changes
4. Ensure all tests pass (`just test && just test-fe run`)
5. Ensure linting passes (`just lint && just lint-frontend`)
6. Commit your changes (follow [Conventional Commits](https://www.conventionalcommits.org/))
7. Push to the branch (`git push origin feature/amazing-feature`)
8. Open a Pull Request

### Code Quality Standards

- **Tests Required**: All new features and bug fixes must include tests
- **Coverage**: PRs should maintain or improve code coverage
- **Linting**: All code must pass linting checks
- **Documentation**: Update docs for API or architectural changes


## Testing

```bash
# Backend
just test              # All tests
just test-mocks        # Fast unit tests only
just test-integration  # Database integration tests
just test-coverage     # With coverage report

# Frontend
just test-fe run            # Run once
just test-fe watch          # Watch mode
just test-fe coverage       # With coverage

# E2E (runs in the playwright container)
just e2e                    # Desktop + mobile
just e2e-desktop            # chromium only
just e2e-mobile             # Pixel 5 only
just e2e-test file <spec>   # A single spec
just e2e-test report        # HTML report
```

## License

[MIT License](MIT-LICENSE)

## Acknowledgments

Built with Clean Architecture principles, TDD practices, and modern development workflows.

---

**Status**: Active Development

For questions or issues, please [open an issue](https://github.com/RallinaTricolor/actionphase/issues).
