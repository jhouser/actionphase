# API Reference

ActionPhase provides a comprehensive REST API for all platform functionality.

## Interactive API Documentation

The best way to explore the ActionPhase API is through our interactive Swagger UI documentation:

**[Open Interactive API Docs](/api/v1/docs)** (opens in new tab)

## Features

Our API documentation includes:

- **Live Testing**: Test API endpoints directly from your browser
- **Authentication**: Built-in JWT Bearer token support
- **Request/Response Examples**: See real examples for every endpoint
- **Schema Definitions**: Complete data models and types
- **Deep Linking**: Share links to specific endpoints

## API Overview

### Base URL
```
Production: https://action-phase.com/api/v1
Development: http://localhost:3000/api/v1
```

### Authentication

All authenticated endpoints require a JWT Bearer token:

```bash
Authorization: Bearer <your-jwt-token>
```

**Getting a Token**:
1. POST `/api/v1/auth/login` with credentials
2. Receive JWT token in response
3. Include token in Authorization header for subsequent requests

### Core Endpoints

**Authentication**:
- `POST /auth/register` - Create new account
- `POST /auth/login` - Login and get JWT token
- `GET /auth/refresh` - Rotate token (**GET**, and it requires a *valid*
  token — it cannot recover an expired session)
- `POST /auth/logout` - Logout (clears the JWT cookie; the session row is not
  deleted)

**Games**:
- `GET /games` - List available games
- `POST /games` - Create a new game (GM only)
- `GET /games/{gameID}` - Get game details
- `PUT /games/{gameID}` - Update game (GM only)
- `POST /games/{gameID}/apply` - Apply to join game

**Characters**:
- `GET /games/{gameID}/characters` - List game characters
- `POST /games/{gameID}/characters` - Create character
- `GET /characters/{id}` - Get character details
- `GET /characters/{id}/data` - Get character sheet fields
- `POST /characters/{id}/data` - Set character sheet fields
- `PUT /characters/{id}/rename` - Rename (GM or owner)
- `PUT /characters/{id}/reassign` - Reassign an inactive character (GM)
- `DELETE /characters/{id}` - Delete character with no activity (GM)

> There is no bare `PUT /characters/{id}`. Updates are split into the specific
> operations above.

**Phases**:
- `GET /games/{gameID}/phases` - List game phases
- `POST /games/{gameID}/phases` - Create phase (GM only)
- `POST /phases/{id}/activate` - Activate phase (GM only) — **POST**, not PUT

**Actions & Results**:
- `POST /games/{gameID}/actions` - Submit action
- `GET /games/{gameID}/actions` - List game actions (GM)
- `GET /games/{gameID}/actions/mine` - Get my actions (**`/mine`**, not `/me`)
- `POST /games/{gameID}/results` - Create result (GM only)
- `POST /games/{gameID}/results/staged` - Create a staged result chain (GM)
- `GET /games/{gameID}/results` - List game results (GM)
- `GET /games/{gameID}/results/mine` - Get my results (**`/mine`**, not `/me`)

**Messages**:
- `GET /games/{gameId}/posts` - Get common room posts
- `POST /games/{gameId}/posts` - Create post
- `GET /conversations` - List private conversations
- `POST /conversations` - Start new conversation

## Rate Limiting

API requests are rate-limited to prevent abuse:

- **Authenticated**: 100 requests per minute
- **Unauthenticated**: 20 requests per minute

Rate limit headers are included in responses:
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1640000000
```

## Error Handling

The API uses standard HTTP status codes:

- `200 OK` - Success
- `201 Created` - Resource created
- `400 Bad Request` - Invalid input
- `401 Unauthorized` - Authentication required
- `403 Forbidden` - Permission denied
- `404 Not Found` - Resource not found
- `429 Too Many Requests` - Rate limit exceeded
- `500 Internal Server Error` - Server error

Error responses include details:
```json
{
  "error": "Error message",
  "details": "Additional context"
}
```

## Testing the API

### Using curl

```bash
# Login
curl -X POST http://localhost:3000/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com", "password": "password"}'

# Save token
TOKEN="your-jwt-token"

# Authenticated request
curl http://localhost:3000/api/v1/games \
  -H "Authorization: Bearer $TOKEN"
```

### Using the test script

ActionPhase includes API test scripts:

```bash
# From the repo root
./backend/scripts/api-test.sh login-player
./backend/scripts/api-test.sh games
```

## OpenAPI Specification

The complete API specification is available in OpenAPI 3.0.3 format:

- **Location**: `backend/pkg/docs/openapi.gen.yaml` (generated — run `just gen-openapi`, never edit by hand)
- **Lines**: 868
- **Version**: Synchronized with backend code

## Next Steps

- **[Explore Interactive Docs](/api/v1/docs)** - Try the API in your browser
- **[Authentication Guide](/developer/getting-started/onboarding#authentication)** - Learn about JWT authentication
- **[Testing Guide](/developer/testing/overview)** - Test your API integrations
