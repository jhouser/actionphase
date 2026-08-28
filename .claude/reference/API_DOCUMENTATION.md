# ActionPhase API Documentation

This document provides comprehensive documentation for the ActionPhase REST API, including usage examples, authentication flows, and integration patterns.

**Last Verified**: August 2026 — endpoint paths spot-checked against
`backend/pkg/http/root.go`. For the complete, generated endpoint list use the
Swagger UI at `/api/v1/docs/` rather than this narrative guide.

## Overview

ActionPhase provides a RESTful API for managing play-by-post RPG games with a cyclical phase-based gameplay system. The API supports:

- **User Authentication** with JWT tokens and automatic refresh
- **Game Management** including creation, updates, and state transitions
- **Game Applications** for joining games during recruitment
- **Character Management** within games
- **Phase Management** for alternating gameplay phases

## Base URL

- **Development**: `http://localhost:3000/api/v1`
- **Production**: `https://api.action-phase.com/api/v1`

## Interactive Documentation

- **Swagger UI**: Available at `/api/v1/docs/` when the server is running
- **OpenAPI Spec**: Available at `/api/v1/docs/openapi.yaml`

## Authentication

ActionPhase uses JWT (JSON Web Tokens) for authentication with automatic token refresh.

> **One token, not two.** There is no separate refresh token. A single bearer
> token is issued at login, valid **7 days**, and backed by a server-side session
> row. `GET /auth/refresh` exchanges a still-valid token for a fresh one; it does
> not consume a distinct refresh credential. Revocation happens by deleting the
> session. See `.claude/context/ARCHITECTURE.md` for the claim structure.

### Authentication Flow

#### 1. User Registration
```bash
curl -X POST http://localhost:3000/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "new_player",
    "email": "player@example.com",
    "password": "secure_password123"
  }'
```

**Response (201 Created):**
```json
{
  "user": {
    "id": 123,
    "username": "new_player",
    "email": "player@example.com",
    "created_at": "2025-08-07T18:30:00Z",
    "updated_at": "2025-08-07T18:30:00Z"
  },
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

#### 2. User Login
```bash
curl -X POST http://localhost:3000/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "new_player",
    "password": "secure_password123"
  }'
```

#### 3. Token Refresh
```bash
curl -X GET http://localhost:3000/api/v1/auth/refresh \
  -H "Authorization: Bearer YOUR_CURRENT_TOKEN"
```

**Response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.NEW_TOKEN..."
}
```

### Using Authentication Tokens

Include the JWT token in the Authorization header for all protected endpoints:

```bash
curl -X GET http://localhost:3000/api/v1/games \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

## Core API Usage Examples

### Game Management

#### List Games (main listing endpoint)
```bash
# Authentication is OPTIONAL here — the response is enriched when a token is sent.
curl -X GET "http://localhost:3000/api/v1/games" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

> There is no `/games/public`. The main listing endpoint is `GET /games`
> (`GetFilteredGames`), which accepts filter query parameters.

#### Get Games Currently Recruiting
```bash
curl -X GET http://localhost:3000/api/v1/games/recruiting \
  -H "Authorization: Bearer YOUR_TOKEN"
```

#### Create a New Game
```bash
curl -X POST http://localhost:3000/api/v1/games \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "The Chronicles of Eldoria",
    "description": "A high-fantasy RPG campaign set in the mystical realm of Eldoria where players will embark on epic quests.",
    "genre": "Fantasy RPG",
    "max_players": 6,
    "start_date": "2025-09-01T19:00:00Z",
    "recruitment_deadline": "2025-08-25T23:59:59Z"
  }'
```

**Response (201 Created):**
```json
{
  "id": 456,
  "title": "The Chronicles of Eldoria",
  "description": "A high-fantasy RPG campaign...",
  "gm_user_id": 123,
  "state": "setup",
  "genre": "Fantasy RPG",
  "max_players": 6,
  "start_date": "2025-09-01T19:00:00Z",
  "recruitment_deadline": "2025-08-25T23:59:59Z",
  "created_at": "2025-08-07T18:30:00Z",
  "updated_at": "2025-08-07T18:30:00Z"
}
```

#### Get Game Details
```bash
curl -X GET http://localhost:3000/api/v1/games/456 \
  -H "Authorization: Bearer YOUR_TOKEN"
```

#### Update Game Information (GM Only)
```bash
curl -X PUT http://localhost:3000/api/v1/games/456 \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "The Chronicles of Eldoria - Updated",
    "description": "An enhanced high-fantasy RPG campaign...",
    "max_players": 8,
    "is_public": true
  }'
```

#### Update Game State (GM Only)
```bash
curl -X PUT http://localhost:3000/api/v1/games/456/state \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "state": "recruitment"
  }'
```

### Game Application System

#### Apply to Join a Game
```bash
curl -X POST http://localhost:3000/api/v1/games/456/apply \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "message": "I am an experienced fantasy RPG player with 5+ years of play-by-post experience. I love collaborative storytelling and character development."
  }'
```

#### Get Game Applications (GM Only)
```bash
curl -X GET http://localhost:3000/api/v1/games/456/applications \
  -H "Authorization: Bearer YOUR_TOKEN"
```

#### Review Application (GM Only)
```bash
curl -X PUT http://localhost:3000/api/v1/games/456/applications/789/review \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "status": "approved",
    "response_message": "Welcome to the campaign! Your experience sounds perfect for our group."
  }'
```

#### Withdraw Application
```bash
curl -X DELETE http://localhost:3000/api/v1/games/456/application \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### Game Participation

#### Get Game Participants
```bash
curl -X GET http://localhost:3000/api/v1/games/456/participants \
  -H "Authorization: Bearer YOUR_TOKEN"
```

#### Leave Game
```bash
curl -X DELETE http://localhost:3000/api/v1/games/456/leave \
  -H "Authorization: Bearer YOUR_TOKEN"
```

## Frontend Integration

### JavaScript/TypeScript Usage

#### API Client Setup
```typescript
import axios from 'axios';

const apiClient = axios.create({
  baseURL: 'http://localhost:3000/api/v1',
  headers: {
    'Content-Type': 'application/json',
  },
});

// Add request interceptor for authentication
apiClient.interceptors.request.use((config) => {
  const token = localStorage.getItem('auth_token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// Add response interceptor for token refresh
apiClient.interceptors.response.use(
  (response) => response,
  async (error) => {
    if (error.response?.status === 401) {
      // Handle token refresh or redirect to login
      try {
        const refreshResponse = await apiClient.get('/auth/refresh');
        const newToken = refreshResponse.data.token;
        localStorage.setItem('auth_token', newToken);

        // Retry original request with new token
        error.config.headers.Authorization = `Bearer ${newToken}`;
        return apiClient(error.config);
      } catch (refreshError) {
        // Redirect to login
        window.location.href = '/login';
      }
    }
    return Promise.reject(error);
  }
);
```

#### Authentication Functions
```typescript
export const authAPI = {
  async register(userData: RegisterRequest) {
    const response = await apiClient.post('/auth/register', userData);
    const { token } = response.data;
    localStorage.setItem('auth_token', token);
    return response.data;
  },

  async login(credentials: LoginRequest) {
    const response = await apiClient.post('/auth/login', credentials);
    const { token } = response.data;
    localStorage.setItem('auth_token', token);
    return response.data;
  },

  async refreshToken() {
    const response = await apiClient.get('/auth/refresh');
    const { token } = response.data;
    localStorage.setItem('auth_token', token);
    return token;
  },

  logout() {
    localStorage.removeItem('auth_token');
    window.location.href = '/login';
  }
};
```

#### Game Management Functions
```typescript
export const gamesAPI = {
  async getAllGames() {
    const response = await apiClient.get('/games');
    return response.data;
  },

  async getRecruitingGames() {
    const response = await apiClient.get('/games/recruiting');
    return response.data;
  },

  async createGame(gameData: CreateGameRequest) {
    const response = await apiClient.post('/games', gameData);
    return response.data;
  },

  async getGame(id: number) {
    const response = await apiClient.get(`/games/${id}`);
    return response.data;
  },

  async updateGame(id: number, updates: UpdateGameRequest) {
    const response = await apiClient.put(`/games/${id}`, updates);
    return response.data;
  },

  async applyToGame(id: number, application: ApplyToGameRequest) {
    const response = await apiClient.post(`/games/${id}/apply`, application);
    return response.data;
  }
};
```

#### React Hook Example
```typescript
import { useState, useEffect } from 'react';

export function useGames() {
  const [games, setGames] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    const fetchGames = async () => {
      try {
        setLoading(true);
        const data = await gamesAPI.getAllGames();
        setGames(data);
      } catch (err) {
        setError(err.message);
      } finally {
        setLoading(false);
      }
    };

    fetchGames();
  }, []);

  return { games, loading, error };
}
```

## Error Handling

### Standard Error Response Format
All API errors return a consistent JSON structure:

```json
{
  "status": "Bad request.",
  "code": 1001,
  "error": "Detailed error message for debugging"
}
```

### HTTP Status Codes

- **200 OK**: Request successful
- **201 Created**: Resource created successfully
- **204 No Content**: Request successful, no response body
- **400 Bad Request**: Invalid request data
- **401 Unauthorized**: Authentication required or invalid token
- **403 Forbidden**: Valid authentication but insufficient permissions
- **404 Not Found**: Resource not found
- **409 Conflict**: Resource conflict (e.g., duplicate username)
- **422 Unprocessable Entity**: Validation failed
- **500 Internal Server Error**: Server error

### Error Handling in Frontend
```typescript
try {
  const game = await gamesAPI.createGame(gameData);
  console.log('Game created:', game);
} catch (error) {
  if (error.response) {
    const { status, data } = error.response;

    switch (status) {
      case 400:
        console.error('Validation error:', data.error);
        break;
      case 401:
        console.error('Authentication required');
        authAPI.logout();
        break;
      case 403:
        console.error('Permission denied:', data.error);
        break;
      default:
        console.error('API error:', data.error || 'Unknown error');
    }
  } else {
    console.error('Network error:', error.message);
  }
}
```

## Rate Limiting and Best Practices

### Rate Limits
- **Authentication endpoints**: 5 requests per minute per IP
- **General API endpoints**: 100 requests per minute per authenticated user
- **File upload endpoints**: 10 requests per minute per user

### Best Practices

#### 1. Token Management
```typescript
// Store tokens securely
const storeToken = (token: string) => {
  // In production, consider using secure HTTP-only cookies
  localStorage.setItem('auth_token', token);
};

// Check token expiration
const isTokenExpired = (token: string): boolean => {
  try {
    const payload = JSON.parse(atob(token.split('.')[1]));
    return payload.exp * 1000 < Date.now();
  } catch {
    return true;
  }
};
```

#### 2. Request Optimization
```typescript
// Use AbortController for cancellation
const controller = new AbortController();

const fetchGames = async () => {
  try {
    const response = await apiClient.get('/games', {
      signal: controller.signal
    });
    return response.data;
  } catch (error) {
    if (error.name === 'AbortError') {
      console.log('Request cancelled');
    }
    throw error;
  }
};

// Cancel request when component unmounts
useEffect(() => {
  return () => controller.abort();
}, []);
```

#### 3. Caching Strategy
```typescript
// Simple cache implementation
const cache = new Map();
const CACHE_DURATION = 5 * 60 * 1000; // 5 minutes

const getCachedData = async (key: string, fetchFn: Function) => {
  const cached = cache.get(key);

  if (cached && Date.now() - cached.timestamp < CACHE_DURATION) {
    return cached.data;
  }

  const data = await fetchFn();
  cache.set(key, { data, timestamp: Date.now() });
  return data;
};

// Usage
const games = await getCachedData('public-games', gamesAPI.getAllGames);
```

## Testing the API

### Health Check
```bash
curl http://localhost:3000/ping
# Expected response: "ponger"
```

### Authentication Test
```bash
# Register new user
TOKEN=$(curl -s -X POST http://localhost:3000/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","email":"test@example.com","password":"password123"}' \
  | jq -r '.token')

# Use token to access protected endpoint
curl -X GET http://localhost:3000/api/v1/games \
  -H "Authorization: Bearer $TOKEN"
```

## API Versioning

ActionPhase API uses URL-based versioning:
- **Current version**: `v1` (as in `/api/v1/games`)
- **Backward compatibility**: Maintained for at least one major version
- **Deprecation notice**: Provided 6 months before version retirement

## Support and Resources

- **Interactive Documentation**: Available at `/api/v1/docs/` on your server
- **OpenAPI Specification**: Download from `/api/v1/docs/openapi.yaml`
- **GitHub Repository**: [ActionPhase on GitHub](https://github.com/actionphase/actionphase)
- **Issue Tracking**: Report API issues on GitHub

This documentation covers the core API functionality. For advanced features and detailed schema information, refer to the interactive Swagger UI documentation.
