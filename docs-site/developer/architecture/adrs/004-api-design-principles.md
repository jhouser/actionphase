# ADR-004: API Design Principles

## Status
Accepted

## Context
ActionPhase requires a well-designed HTTP API that supports:
- Frontend-backend communication with clear contracts
- Future mobile app development
- Third-party integrations
- Maintainable and evolvable API surface
- Strong typing and validation
- Comprehensive error handling
- Performance optimization

The API design must balance developer experience, performance, and long-term maintainability.

## Decision
We adopted **RESTful API design principles** with modern enhancements:

**Core REST Principles**:
- Resource-based URLs with clear noun-based endpoints
- Standard HTTP methods (GET, POST, PUT, PATCH, DELETE)
- Stateless design with JWT authentication
- Consistent response formats and status codes

**Modern Enhancements**:
- Structured error responses with correlation IDs
- Request/response validation with detailed error messages
- OpenAPI documentation with examples
- Consistent field naming conventions (snake_case for JSON)

**API Versioning Strategy**:
- URL-based versioning (`/api/v1/`) for major versions
- Backward compatibility within major versions
- Deprecation notices for evolving endpoints

## Alternatives Considered

### 1. GraphQL API
**Approach**: Single endpoint with query-based data fetching.

**Pros**:
- Flexible data fetching reduces over-fetching
- Strong type system with introspection
- Single request for complex data requirements
- Excellent developer tooling

**Cons**:
- Complexity of implementation and learning curve
- Caching challenges compared to REST
- Security concerns with query complexity
- Less familiar to most developers

### 2. RPC-Style API (gRPC)
**Approach**: Procedure-call based API with protocol buffers.

**Pros**:
- High performance with binary protocols
- Strong typing with code generation
- Streaming support for real-time features
- Excellent for service-to-service communication

**Cons**:
- Less browser-friendly, requires proxy for web
- More complex tooling and debugging
- Steeper learning curve
- Limited browser support without additional layers

### 3. HATEOAS (Hypermedia)
**Approach**: Include links to related resources in responses.

**Pros**:
- Self-documenting API with discoverable actions
- Loose coupling between client and server
- Dynamic API navigation
- Standards-compliant REST implementation

**Cons**:
- Increased payload size and complexity
- Frontend complexity for link following
- Less predictable API behavior
- Overkill for current application complexity

## Consequences

### Positive Consequences

**Developer Experience**:
- Familiar REST patterns reduce onboarding time
- Clear URL structure makes API intuitive
- Standard HTTP status codes simplify error handling
- Consistent response formats reduce client complexity

**Frontend Integration**:
- Easy integration with React Query for caching
- Predictable data fetching patterns
- Simple error handling with structured responses
- Type-safe API calls with TypeScript definitions

**Performance**:
- HTTP caching for GET requests
- Efficient JSON serialization
- Minimal payload overhead
- Connection reuse with keep-alive

**Maintainability**:
- Clear separation of concerns between endpoints
- Easy to add new resources without affecting existing ones
- Standard HTTP semantics make behavior predictable
- OpenAPI documentation keeps contracts up-to-date

### Negative Consequences

**API Chattiness**:
- Multiple requests needed for related data
- Potential over-fetching or under-fetching
- Network latency impact for complex operations
- More complex state management in frontend

**Versioning Complexity**:
- URL-based versioning creates endpoint proliferation
- Backward compatibility constraints limit evolution
- Migration complexity for major version changes
- API documentation maintenance across versions

**Standardization Rigidity**:
- REST constraints may not fit all use cases
- Binary data handling limitations
- Real-time features require additional solutions
- Batch operations don't map naturally to REST

### Risk Mitigation Strategies

**Performance Optimization**:
- Implement efficient JSON serialization
- Use HTTP caching headers where appropriate
- Consider batch endpoints for related operations
- Monitor API performance with metrics

**Version Management**:
- Clear deprecation timeline for old versions
- Automated testing across API versions
- Documentation for migration paths
- Feature flags for gradual rollouts

**Error Handling**:
- Structured error responses with actionable messages
- Correlation IDs for request tracing
- Consistent error format across all endpoints
- Client-friendly error categorization

## Implementation Details

### URL Structure
```
Base URL: /api/v1/

Resources:
GET    /api/v1/games                  # List games (filtered)
POST   /api/v1/games                  # Create game
GET    /api/v1/games/{gameID}         # Get specific game
PUT    /api/v1/games/{gameID}         # Update game
DELETE /api/v1/games/{gameID}         # Delete game

Nested Resources:
GET    /api/v1/games/{gameID}/characters    # Game characters
POST   /api/v1/games/{gameID}/characters    # Create character
GET    /api/v1/games/{gameID}/phases        # Game phases
POST   /api/v1/games/{gameID}/phases        # Create phase

Authentication:
POST   /api/v1/auth/register      # User registration
POST   /api/v1/auth/login         # User login
GET    /api/v1/auth/refresh       # Token rotation (GET, not POST)
POST   /api/v1/auth/logout        # User logout
```

> Two corrections from the original text (verified 2026-08-26 against
> `backend/pkg/http/root.go`):
> - The path parameter is **`{gameID}`**, not `{id}`.
> - **`/auth/refresh` is a `GET`** (`root.go:126`). It requires a currently
>   valid token and mints a new one; there is no refresh-token body to POST.
>   See ADR-003's divergence section.

### Request/Response Format

> ❌ **The envelope originally documented here was never implemented.** It
> specified a `data` wrapper, an `error: {code, message, details[]}` object, and
> a `meta` block on every response. None of that ships. Replaced below with
> responses captured live from the running API on 2026-08-26.

```json
// Single resource — returned bare, with no "data" wrapper and no "meta"
{
  "id": 50718,
  "title": "Chronicles of Westmarch",
  "state": "in_progress",
  "created_at": "2026-06-27T20:33:51.798299Z"
}

// Collection — keyed by RESOURCE NAME (not "data"), paginated under "metadata"
{
  "games": [
    {"id": 50718, "title": "Chronicles of Westmarch"},
    {"id": 50717, "title": "..."}
  ],
  "metadata": {
    "total_count": 7,
    "filtered_count": 7,
    "available_states": ["completed", "in_progress", "recruitment"],
    "page": 1,
    "page_size": 2,
    "total_pages": 4,
    "has_next_page": true,
    "has_previous_page": false
  }
}

// Error — a flat {status, error} pair. No error code, no field-level details.
{
  "status": "Invalid request.",
  "error": "title is required; description is required"
}
```

Notable differences from the original design, worth knowing before writing a
client:

- **No `data` envelope.** Single resources are returned bare; collections use
  the resource name as the key (`games`, `characters`, …).
- **Pagination is `metadata`**, not `pagination`, and uses `page_size` /
  `total_count` (not `per_page` / `total`). It also carries `filtered_count`,
  `available_states`, and `has_next_page` / `has_previous_page`.
- **Errors are flat.** Multiple validation failures are joined into one
  `error` string (`"title is required; description is required"`) rather than a
  structured `details[]` array — so clients cannot reliably attribute an error
  to a field.
- **No `meta.correlation_id` in the body.** Correlation IDs travel in the
  `X-Correlation-ID` response header instead.
- Some unauthenticated failures return **plain text**, not JSON (e.g.
  `GET /api/v1/games/1` without a token returns `no token found`).

### HTTP Status Code Usage
```
Success Codes:
200 OK          - Successful GET, PUT, PATCH
201 Created     - Successful POST
204 No Content  - Successful DELETE
304 Not Modified - Cached resource still valid

Client Error Codes:
400 Bad Request      - Malformed request or validation error
401 Unauthorized     - Authentication required
403 Forbidden        - Authorization failed
404 Not Found        - Resource doesn't exist
409 Conflict         - Resource conflict (duplicate, etc.)
422 Unprocessable    - Valid request, business logic error

Server Error Codes:
500 Internal Error   - Unexpected server error
503 Service Unavailable - Temporary server issue
```

### Authentication Headers
```http
Authorization: Bearer <jwt_token>
X-Correlation-ID: corr_abc123 (optional, generated if missing)
Content-Type: application/json
Accept: application/json
```

### Field Naming Conventions
- **JSON Fields**: snake_case (user_id, created_at, max_players)
- **Go Structs**: PascalCase (UserID, CreatedAt, MaxPlayers)
- **Database Columns**: snake_case (user_id, created_at, max_players)
- **HTTP Headers**: Kebab-Case (X-Correlation-ID, Content-Type)

### Validation Strategy
- **Request Validation**: Validate all input at API boundary
- **Business Logic Validation**: Domain-specific rules in service layer
- **Database Constraints**: Final safety net with database constraints
- **Error Messages**: User-friendly messages with field-specific details

### Content Negotiation
```http
# Request JSON (default)
Content-Type: application/json
Accept: application/json

# Future considerations
Accept: application/json; version=1
Accept: application/hal+json  # For HATEOAS if needed
```

## OpenAPI Documentation

All endpoints are documented with OpenAPI 3.0 specification including:
- Complete request/response schemas
- Authentication requirements
- Error response formats
- Example requests and responses
- Field descriptions and constraints

## Future Considerations

### Planned Enhancements
- **Batch Operations**: Endpoints for bulk operations
- **Filtering and Sorting**: Query parameters for collection endpoints
- **Field Selection**: Sparse field sets to reduce payload size
- **Rate Limiting**: API usage limits with proper headers

### Real-time Features
- **WebSocket Endpoints**: For live game updates
- **Server-Sent Events**: For one-way real-time notifications
- **WebHooks**: For external system integration
- **Push Notifications**: Mobile app integration

### Advanced Features
- **API Analytics**: Usage tracking and performance monitoring
- **SDK Generation**: Auto-generated client SDKs from OpenAPI
- **Mock Server**: OpenAPI-based mock server for frontend development
- **Contract Testing**: Consumer-driven contract testing

## References
- [RESTful API Design Best Practices](https://docs.microsoft.com/en-us/azure/architecture/best-practices/api-design)
- [OpenAPI 3.0 Specification](https://swagger.io/specification/)
- [HTTP Status Code Registry](https://www.iana.org/assignments/http-status-codes/)
- [JSON API Specification](https://jsonapi.org/)
