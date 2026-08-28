# ActionPhase System Architecture

## Overview

ActionPhase is a modern web application for hosting play-by-post RPG games with a cyclical phase-based gameplay system. The architecture follows clean architecture principles with clear separation of concerns and dependency inversion.

## High-Level Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Frontend      │    │   Backend       │    │   Database      │
│   React/TypeScript   │    Go/Chi Router    │    PostgreSQL    │
├─────────────────┤    ├─────────────────┤    ├─────────────────┤
│ • React 19      │◄──►│ • HTTP API      │◄──►│ • User Data     │
│ • TypeScript    │    │ • JWT Auth      │    │ • Game State    │
│ • Vite          │    │ • Business Logic│    │ • Sessions      │
│ • React Query   │    │ • Database Layer│    │ • Characters    │
│ • Tailwind CSS  │    │ • Observability │    │ • Game Phases   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## System Components

### Frontend (React SPA)
- **Framework**: React 19 with TypeScript
- **Build Tool**: Vite for fast development and optimized builds
- **Styling**: Tailwind CSS for utility-first responsive design
- **State Management**: React Query for server state, React hooks for local state
- **Authentication**: JWT tokens with automatic refresh
- **Error Handling**: React Error Boundaries with structured error display

### Backend (Go HTTP API)
- **Framework**: Chi router for HTTP routing and middleware
- **Language**: Go 1.21+ for performance and type safety
- **Authentication**: a single JWT bearer token backed by a server-side session row (revocable on every request). There is **no separate refresh token** — see ADR-003's Implementation Divergence section.
- **Database**: PostgreSQL with connection pooling (pgxpool)
- **Code Generation**: sqlc for type-safe SQL queries
- **Observability**: Structured logging, metrics, and request tracing

### Database (PostgreSQL)
- **Primary Database**: PostgreSQL 15+ with ACID compliance
- **Schema Management**: golang-migrate for version-controlled migrations
- **Connection Management**: pgxpool for efficient connection pooling
- **Data Types**: largely relational typed columns; JSONB in exactly two places (`games.character_sheet`, `user_preferences.preferences`); timestamps with timezone

## Directory Structure

```
actionphase/
├── backend/                    # Go backend service
│   ├── pkg/                   # Go packages
│   │   ├── auth/             # Authentication logic
│   │   ├── characters/       # Character management
│   │   ├── core/             # Core interfaces and utilities
│   │   ├── db/               # Database layer
│   │   │   ├── migrations/   # Database migrations
│   │   │   ├── models/       # Generated sqlc models
│   │   │   ├── queries/      # SQL queries
│   │   │   └── services/     # Repository implementations
│   │   ├── games/            # Game management
│   │   ├── http/             # HTTP handlers and routing
│   │   ├── observability/    # Logging, metrics, tracing
│   │   └── phases/           # Game phase management
│   ├── main.go              # Application entry point
│   └── justfile             # Build and development commands
├── frontend/                 # React frontend application
│   ├── src/
│   │   ├── components/      # React components
│   │   ├── hooks/           # Custom React hooks
│   │   ├── lib/             # Utility libraries
│   │   ├── pages/           # Page components
│   │   └── types/           # TypeScript type definitions
│   └── package.json         # Frontend dependencies
├── docs/                    # Architecture and API documentation
│   ├── architecture/        # System architecture docs
│   ├── adrs/               # Architecture Decision Records
│   └── diagrams/           # System diagrams
└── docker-compose.yml      # Development environment setup
```

## Design Principles

### 1. Clean Architecture
- **Dependency Inversion**: Core business logic depends only on abstractions
- **Interface Segregation**: Small, focused interfaces in `core/interfaces.go`
- **Single Responsibility**: Each package has a single, well-defined purpose
- **Testability**: All components can be unit tested with mocks

### 2. Domain-Driven Design
- **Bounded Contexts**: Clear separation between auth, games, characters, phases
- **Domain Models**: Rich models with behavior in the core package
- **Repository Pattern**: Data access abstracted behind interfaces
- **Service Layer**: Business logic orchestration

### 3. API-First Development
- **OpenAPI Specification**: Complete API documentation with examples
- **Contract-First**: API contracts defined before implementation
- **Versioning**: URL-based versioning (`/api/v1/`)
- **RESTful Design**: Resource-based URLs with standard HTTP methods

### 4. Observability-First
- **Structured Logging**: All logs include correlation IDs and context
- **Metrics Collection**: Performance and business metrics
- **Request Tracing**: End-to-end request tracking
- **Health Monitoring**: Health checks and service status

## Core Concepts

### Game Lifecycle
1. **Setup** - Game created, basic information configured
2. **Recruitment** - Players can apply to join the game
3. **Active** - Game is running with alternating phases
4. **Paused** - Game temporarily suspended
5. **Completed** - Game has ended
6. **Cancelled** - Game terminated before completion

### Phase System
ActionPhase uses a cyclical phase-based system:
- **Planning Phase**: Players submit their intended actions
- **Resolution Phase**: GM processes actions and updates game state
- **Reaction Phase**: Players react to the results
- *Cycle repeats*

### User Roles
- **Player**: Participates in games, creates characters, submits actions
- **Game Master (GM)**: Manages games, controls phases, processes actions
- **Audience**: Can view public games but not participate

## Data Flow

### Authentication Flow
1. User submits credentials → Backend validates → Returns JWT
2. Frontend stores JWT → Includes in all API requests
3. Backend validates JWT → Extracts user context → Processes request
4. JWT refresh handled automatically by axios interceptors

### Game Creation Flow
1. GM creates game → Validation → Database persistence
2. Game enters "recruitment" state → Players can apply
3. GM reviews applications → Approves players
4. GM starts game → Creates first phase
5. Phase management cycle begins

### Request Processing Flow
```
HTTP Request → Middleware Stack → Handler → Service Layer → Repository → Database
     ↓              ↓               ↓           ↓             ↓           ↓
Correlation ID  Authentication  Validation  Business     Data Access  Persistence
Request Tracing   Authorization   Binding    Logic       SQL Queries   ACID Ops
Metrics          Rate Limiting   Error       Orchestration Type Safety  Constraints
Recovery         CORS           Handling    Domain Rules  Connection    Migrations
                                            Service Calls  Pooling
```

## Technology Decisions

### Backend Technology Stack
- **Go**: Chosen for performance, strong typing, and excellent concurrency
- **Chi Router**: Lightweight, composable HTTP router with middleware support
- **PostgreSQL**: ACID compliance, JSONB support, mature ecosystem
- **sqlc**: Type-safe SQL query generation, compile-time validation
- **slog**: Structured logging with context and performance

### Frontend Technology Stack
- **React**: Component-based architecture with hooks and modern patterns
- **TypeScript**: Type safety, better IDE support, reduced runtime errors
- **Vite**: Fast development server and optimized production builds
- **React Query**: Server state management with caching and synchronization
- **Tailwind CSS**: Utility-first CSS with responsive design

### Development Tools
- **justfile**: Modern command runner for development tasks
- **Docker Compose**: Local development environment with database
- **golang-migrate**: Version-controlled database migrations
- **Vitest**: Fast unit testing for frontend components
- **ESLint**: Code quality and style enforcement

## Integration Points

### Frontend ↔ Backend
- **Communication**: HTTP/JSON REST API with JWT authentication
- **Error Handling**: Standardized error responses with structured format
- **State Synchronization**: React Query manages server state caching
- **Real-time Updates**: Polling-based (WebSocket support planned)

### Backend ↔ Database
- **Connection Management**: pgxpool for efficient connection pooling
- **Query Interface**: sqlc-generated type-safe query functions
- **Transaction Management**: Explicit transaction boundaries
- **Migration Management**: Version-controlled schema evolution

### External Integrations
- **Development**: Docker for local PostgreSQL database
- **Observability**: Structured logs ready for aggregation systems
- **Monitoring**: Metrics endpoint for Prometheus/Grafana integration
- **Health Checks**: Standard health endpoint for load balancer probes

## Security Architecture

### Authentication & Authorization
- **JWT Tokens**: Stateless authentication with user claims
- **Refresh Tokens**: Stored securely in database sessions
- **Token Rotation**: Automatic token refresh with sliding expiration
- **Session Management**: Server-side session tracking for security

### Data Protection
- **Password Hashing**: bcrypt with appropriate cost factor
- **SQL Injection Prevention**: Parameterized queries via sqlc
- **CORS Configuration**: Controlled cross-origin resource sharing
- **Input Validation**: Request validation at API boundary

### Infrastructure Security
- **Environment Variables**: Configuration via environment, no hardcoded secrets
- **Database Connections**: TLS encryption for database connections
- **HTTP Security Headers**: Standard security headers in responses
- **Error Information**: Sanitized error responses to prevent information leakage

## Performance Characteristics

### Backend Performance
- **Connection Pooling**: Configurable PostgreSQL connection pool
- **Query Optimization**: Generated queries optimized by sqlc
- **Middleware Efficiency**: Minimal middleware overhead
- **Memory Management**: Go garbage collector with low latency

### Frontend Performance
- **Code Splitting**: Vite automatically splits code by routes
- **Tree Shaking**: Unused code eliminated in production builds
- **React Query**: Efficient caching reduces unnecessary API calls
- **Component Optimization**: React.memo and useMemo where appropriate

### Database Performance
- **Indexing Strategy**: Indexes on foreign keys and query patterns
- **Connection Reuse**: Persistent connections via pgxpool
- **Query Patterns**: Optimized for common access patterns
- **JSONB Usage**: sparing — character sheet *content* is the EAV `character_data` table, not a JSONB document (see ADR-002)

## Scalability Considerations

### Current Architecture
- **Single Server**: Monolithic deployment suitable for small to medium load
- **Database**: Single PostgreSQL instance with connection pooling
- **State Management**: Server-side state in database

### Future Scalability Options
- **Horizontal Scaling**: Multiple backend instances with load balancer
- **Database Scaling**: Read replicas for query performance
- **Caching Layer**: Redis for session and frequently accessed data
- **Microservices**: Service decomposition as system grows

## Deployment Architecture

### Development Environment
- **Local Database**: Docker Compose PostgreSQL instance
- **Hot Reload**: Vite dev server with React Fast Refresh
- **Live Reload**: Go backend with automatic restart
- **Environment Variables**: `.env` file for local configuration

### Production Considerations (Future)
- **Container Deployment**: Docker containers for consistent environments
- **Database Hosting**: Managed PostgreSQL service
- **Static Assets**: CDN for frontend assets
- **Environment Configuration**: Container orchestration secrets management

## Monitoring and Observability

### Logging Strategy
- **Structured Logging**: JSON format with consistent field names
- **Context Propagation**: Correlation IDs throughout request lifecycle
- **Log Levels**: Appropriate use of DEBUG, INFO, WARN, ERROR
- **Centralized Logging**: Ready for log aggregation systems

### Metrics Collection
- **HTTP Metrics**: Request count, latency, error rates by endpoint
- **Business Metrics**: Game creation, user actions, phase transitions
- **System Metrics**: Database connections, memory usage
- **Custom Metrics**: Configurable counters, gauges, histograms

### Health Monitoring
- **Health Endpoint**: `/health` with service status checks
- **Telemetry**: OpenTelemetry traces/metrics/logs pushed to Grafana Cloud via OTLP. There is **no `/metrics` endpoint**.
- **Database Health**: Connection and query performance monitoring
- **Error Rate Monitoring**: Automatic alerting on high error rates

This architecture provides a solid foundation for ActionPhase that can scale from a small community to a large player base while maintaining code quality, performance, and developer experience.
