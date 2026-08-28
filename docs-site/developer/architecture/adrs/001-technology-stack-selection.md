# ADR-001: Technology Stack Selection

## Status
Accepted

## Context
ActionPhase requires a technology stack that can support:
- Real-time collaboration for play-by-post RPG games
- Complex game state management with phases and characters
- User authentication and authorization
- Scalability for growing user base
- Developer productivity and maintainability
- Strong type safety to prevent runtime errors

The decision needed to balance performance, development speed, ecosystem maturity, and team expertise.

> **Version note (checked 2026-08-26):** the versions below are those chosen
> when this decision was made and are kept as the historical record. Current
> versions in use: **Go 1.25**, **React 19**, **PostgreSQL 17**, **Vite 7**,
> **Tailwind CSS 4** (CSS-first `@theme` config — no `tailwind.config.js`),
> **TanStack Query 5**, **React Router 7**. The technology *choices* below all
> still hold.

## Decision
We selected the following technology stack:

**Backend**: Go with Chi router
- Go 1.21+ for the backend HTTP API
- Chi router for HTTP routing and middleware
- PostgreSQL for primary database storage
- sqlc for type-safe SQL query generation

**Frontend**: React with TypeScript
- React 18 with TypeScript for the user interface
- Vite for build tooling and development server
- React Query for server state management
- Tailwind CSS for styling

**Database**: PostgreSQL
- PostgreSQL 15+ as primary database
- golang-migrate for schema migrations
- JSONB columns for flexible game data storage

## Alternatives Considered

### Backend Alternatives
1. **Node.js with Express**
   - Pros: JavaScript ecosystem, rapid development, large community
   - Cons: Runtime type safety issues, memory usage, single-threaded

2. **Python with Django/FastAPI**
   - Pros: Rapid development, excellent ecosystem, strong typing with mypy
   - Cons: Performance limitations, GIL constraints, deployment complexity

3. **Rust with Axum**
   - Pros: Extreme performance, memory safety, growing ecosystem
   - Cons: Steeper learning curve, slower development, smaller community

### Frontend Alternatives
1. **Vue.js with TypeScript**
   - Pros: Gentler learning curve, excellent TypeScript support
   - Cons: Smaller ecosystem, less job market demand

2. **Svelte/SvelteKit**
   - Pros: Smaller bundle sizes, compile-time optimizations
   - Cons: Smaller ecosystem, less mature tooling

3. **Angular**
   - Pros: Full framework with everything included, excellent TypeScript
   - Cons: Heavy bundle size, complex for simple applications

### Database Alternatives
1. **MongoDB**
   - Pros: Schema flexibility, JSON-native, horizontal scaling
   - Cons: Eventual consistency, less mature transaction support

2. **SQLite**
   - Pros: Zero configuration, embedded, excellent for development
   - Cons: Limited concurrency, no network access, scaling limitations

## Consequences

### Positive Consequences
- **Type Safety**: Go's static typing and TypeScript prevent many runtime errors
- **Performance**: Go provides excellent performance with low resource usage
- **Ecosystem Maturity**: Both Go and React have mature, stable ecosystems
- **Developer Experience**: Excellent tooling and IDE support for both languages
- **Database Reliability**: PostgreSQL provides ACID compliance and mature features
- **Code Generation**: sqlc generates type-safe Go code from SQL queries
- **Build Performance**: Vite provides fast development and optimized production builds

### Negative Consequences
- **Learning Curve**: Team needs to learn Go if not already familiar
- **Verbosity**: Go can be more verbose than dynamic languages
- **Ecosystem Size**: Go's ecosystem is smaller than Node.js or Python
- **Complex State**: React Query adds complexity but provides powerful caching

### Technical Debt Considerations
- **Go Dependencies**: Limited to Go ecosystem for backend utilities
- **Frontend Bundle Size**: React adds baseline bundle size overhead
- **Database Migrations**: Need careful planning for schema changes
- **Type Definitions**: Must maintain TypeScript definitions for API contracts

### Risk Mitigation
- **Go Learning**: Invest in Go training and establish coding standards
- **Performance Monitoring**: Implement observability early to catch issues
- **API Versioning**: Design API with versioning to allow evolution
- **Testing Strategy**: Comprehensive testing to leverage type safety benefits

## Implementation Notes
- Use Go modules for dependency management
- Follow Go naming conventions and idiomatic patterns
- Implement comprehensive error handling with structured logging
- Use React hooks and functional components exclusively
- Implement strict TypeScript configuration
- Use PostgreSQL features like JSONB for game data flexibility

## References
- [Go Official Documentation](https://golang.org/doc/)
- [React Official Documentation](https://react.dev/)
- [Chi Router Documentation](https://github.com/go-chi/chi)
- [sqlc Documentation](https://sqlc.dev/)
- [PostgreSQL Documentation](https://www.postgresql.org/docs/)
- [Vite Documentation](https://vitejs.dev/)
