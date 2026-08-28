# Developer Documentation

Welcome to the ActionPhase developer documentation! This guide covers everything you need to know to contribute to the platform.

## Getting Started

- **[Developer Onboarding](/developer/getting-started/onboarding)** - 30-minute quick start guide

## Architecture

Learn about the system design:

- **[System Overview](/developer/architecture/overview)** - High-level architecture
- **[Component Interactions](/developer/architecture/components)** - How components communicate
- **[Architecture Decision Records](/developer/architecture/adrs/)** - Design decisions and rationale

## API

- **[API Reference](/developer/api/reference)** - REST API documentation
- **[Interactive API Docs](/api/v1/docs)** - Swagger UI with live testing

## Testing

- **[Test Coverage Status](/developer/testing/COVERAGE_STATUS)** - Current coverage and gaps
- **[E2E Quick Start](/developer/testing/E2E_QUICK_START)** - Running Playwright tests
- **[ADR-007: Testing Strategy](/developer/architecture/adrs/007-testing-strategy)** - Test pyramid, patterns, and tools

## Tech Stack

**Backend**:
- Go 1.25 with Chi router
- PostgreSQL 17
- sqlc for type-safe SQL
- JWT authentication

**Frontend**:
- React 19 with TypeScript 5.9
- Vite 7 build tool
- React Query for state management
- Tailwind CSS 4

**Testing**:
- Backend: Go's testing package
- Frontend: Vitest + React Testing Library
- E2E: Playwright

## Contributing

1. Read the **[Developer Onboarding](/developer/getting-started/onboarding)** guide
2. Check out the **[Architecture docs](/developer/architecture/overview)**
3. Review **[ADR-007: Testing Strategy](/developer/architecture/adrs/007-testing-strategy)**
4. Start with a good first issue!
