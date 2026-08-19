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
- `POST /auth/refresh` - Refresh expired token
- `POST /auth/logout` - Logout and invalidate session

**Games**:
- `GET /games` - List available games
- `POST /games` - Create a new game (GM only)
- `GET /games/{id}` - Get game details
- `PUT /games/{id}` - Update game (GM only)
- `POST /games/{id}/apply` - Apply to join game

**Characters**:
- `GET /games/{gameId}/characters` - List game characters
- `POST /games/{gameId}/characters` - Create character
- `GET /characters/{id}` - Get character details
- `PUT /characters/{id}` - Update character
- `DELETE /characters/{id}` - Delete character

**Phases**:
- `GET /games/{gameId}/phases` - List game phases
- `POST /games/{gameId}/phases` - Create phase (GM only)
- `PUT /phases/{id}/activate` - Activate phase (GM only)

**Actions & Results**:
- `POST /games/{gameId}/actions` - Submit action
- `GET /games/{gameId}/actions/me` - Get my actions
- `POST /games/{gameId}/results` - Create result (GM only)
- `GET /games/{gameId}/results/me` - Get my results

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

- **Location**: `backend/pkg/docs/openapi.yaml`
- **Lines**: 868
- **Version**: Synchronized with backend code

## Next Steps

- **[Explore Interactive Docs](/api/v1/docs)** - Try the API in your browser
- **[Authentication Guide](/developer/getting-started/onboarding#authentication)** - Learn about JWT authentication
- **[Testing Guide](/developer/testing/overview)** - Test your API integrations
