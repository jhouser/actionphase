# ADR-003: Authentication Strategy

## Status
Accepted

## Context
ActionPhase requires secure user authentication that supports:
- Stateless API design for scalability
- Secure session management to prevent unauthorized access
- Token refresh without requiring re-login
- Cross-device session management
- Logout functionality that truly invalidates sessions
- Protection against common authentication attacks

The solution must balance security, user experience, and implementation complexity.

> ⚠️ **Accuracy notice (verified 2026-08-26):** several specifics in this
> section were never implemented as written — the token lifetime, the claim
> contents, and the separate refresh token all differ in the shipped code. See
> [Implementation Divergence](#implementation-divergence-verified-2026-08-26) at
> the end before relying on any detail here.

## Decision
We implemented a **JWT + Refresh Token Strategy** with server-side session management:

**Primary Authentication**: JWT Access Tokens
- Short-lived JWT tokens (15 minutes) for API access
- Contains only `sub` (username), `exp`, `iat`, `jti` — user ID intentionally excluded
- Stateless verification for API performance
- Automatic refresh via axios interceptors

**Session Management**: Database-stored refresh tokens
- Long-lived refresh tokens (7 days) stored in database
- Unique session tracking with device identification
- Secure logout with token invalidation
- Session management for multiple devices

**Security Features**:
- bcrypt password hashing with appropriate cost
- Secure HTTP-only cookie option for refresh tokens (future)
- CSRF protection through token validation
- Rate limiting on authentication endpoints

## Alternatives Considered

### 1. Session-Based Authentication
**Approach**: Traditional server-side sessions with cookies.

**Pros**:
- Simple to implement and understand
- Easy session invalidation
- Lower client-side complexity
- Automatic CSRF protection with SameSite cookies

**Cons**:
- Stateful server design complicates scaling
- Session storage requirements grow with users
- CORS complexity with cookies
- Requires sticky sessions in load-balanced environments

### 2. JWT-Only Strategy
**Approach**: Long-lived JWT tokens without refresh mechanism.

**Pros**:
- Completely stateless server design
- Simple implementation
- Good performance
- Easy to scale horizontally

**Cons**:
- Cannot revoke tokens before expiration
- Security risk with long-lived tokens
- No session management capabilities
- Difficult to handle compromised tokens

### 3. OAuth2 with External Provider
**Approach**: Delegate authentication to Google, GitHub, Discord, etc.

**Pros**:
- No password storage or management
- Leverages secure, tested implementations
- User convenience with existing accounts
- Reduced authentication attack surface

**Cons**:
- Dependency on external services
- Limited customization of user experience
- Privacy concerns with data sharing
- Potential vendor lock-in

## Consequences

### Positive Consequences

**Security Benefits**:
- Short-lived access tokens limit exposure window
- Server-side refresh token storage allows revocation
- Session tracking enables multi-device management
- bcrypt password hashing protects against breaches

**User Experience**:
- Seamless token refresh without user interaction
- Cross-device session management
- Proper logout functionality
- No frequent re-authentication required

**Scalability**:
- Stateless API calls with JWT validation
- Database sessions scale with connection pooling
- Horizontal scaling without session affinity
- Efficient caching of user claims

**Development Benefits**:
- Clear separation between access and refresh flows
- Testable authentication components
- Integration with existing HTTP middleware
- Standard JWT libraries and tooling

### Negative Consequences

**Implementation Complexity**:
- Two-token system requires careful coordination
- Frontend must handle token refresh logic
- Database session management adds state
- Error handling for various token scenarios

**Security Considerations**:
- JWT payload visible to clients (no sensitive data)
- Refresh token compromise requires detection
- Token timing windows require careful design
- Rate limiting needed to prevent brute force

**Performance Impact**:
- Database queries for refresh token validation
- Additional storage for session data
- Network overhead for token refresh requests
- JWT parsing and validation on each request

### Risk Mitigation Strategies

**Token Security**:
- Use strong, random secrets for JWT signing
- Implement secure token storage in frontend
- Add correlation IDs for request tracking
- Monitor for unusual authentication patterns

**Session Management**:
- Implement session cleanup for expired tokens
- Add device fingerprinting for security
- Provide user interface for session management
- Log authentication events for audit trails

**Frontend Security**:
- Store tokens in memory where possible
- Implement automatic logout on token expiration
- Handle network errors gracefully during refresh
- Validate JWT structure and claims

## Implementation Details

### JWT Token Structure

> ❌ **The structure originally documented here was never implemented.** It
> claimed `sub` held a username with `iat`/`jti` present, and that the user ID
> was deliberately excluded. The shipped token does the opposite.

**As designed (not built):**
```json
{ "sub": "username", "exp": 1625097600, "iat": 1625096700, "jti": "token-uuid" }
```

**As actually issued** (`pkg/auth/jwt.go:78-83`):
```json
{
  "sub": "42",          // the user ID, as a string
  "session_id": 1337,   // FK into the sessions table
  "exp": 1625097600
}
```

The "user ID intentionally excluded" rationale below does **not** describe this
system. Tamper-resistance comes from the HMAC signature (HS256), not from
omitting the ID; and `session_id` means every request is checked against a
server-side session row, so a forged or revoked token fails regardless of claims.

### Database Session Schema

> ❌ **The schema originally documented here does not exist.** It had a
> `refresh_token` column, `device_id`, and `expires_at`. The real table has no
> refresh token at all — it stores the JWT itself in `data`.

**Actual schema** (`backend/pkg/db/schema.sql`):
```sql
CREATE TABLE sessions (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id),
    data TEXT NOT NULL,          -- the JWT itself
    expires TIMESTAMP WITH TIME ZONE,
    ip_address VARCHAR(45),
    user_agent TEXT,
    fingerprint VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

Note `expires` / `last_seen_at` (not `expires_at` / `last_used_at`), and
`fingerprint` in place of `device_id`.

### Authentication Flow

**As actually implemented** (there is only one token):

1. **Login**: Validate credentials → create `sessions` row → return the JWT in
   the body **and** as an HTTP-only `jwt` cookie
2. **API Access**: `Authorization: Bearer <jwt>` → signature + session check →
   process request
3. **Token Rotation**: `GET /api/v1/auth/refresh` with a **currently valid**
   token → mint a new token and a **new session row**. There is no separate
   refresh credential, so this cannot recover an already-expired session.
4. **Logout**: `V1Logout` **only clears the `jwt` cookie**
   (`pkg/auth/login.go:153`). It does **not** delete the session row, so a token
   already captured by a client remains valid server-side until `exp` (up to
   7 days). Session rows are deleted elsewhere — e.g. password change revokes
   all *other* sessions (`pkg/auth/account_service.go:475`).

### Frontend Integration
```typescript
// Axios interceptors for automatic token management
axios.interceptors.request.use((config) => {
  const token = getAccessToken();
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

axios.interceptors.response.use(
  (response) => response,
  async (error) => {
    if (error.response?.status === 401) {
      const newToken = await refreshAccessToken();
      if (newToken) {
        error.config.headers.Authorization = `Bearer ${newToken}`;
        return axios(error.config);
      } else {
        redirectToLogin();
      }
    }
    return Promise.reject(error);
  }
);
```

### Security Headers and Middleware
```go
// JWT validation middleware
func JWTAuthenticator(tokenAuth *jwtauth.JWTAuth) func(http.Handler) http.Handler {
    return jwtauth.Authenticator(tokenAuth)
}

// Rate limiting for auth endpoints
func AuthRateLimit() func(http.Handler) http.Handler {
    return middleware.RateLimit(5, time.Minute) // 5 requests per minute
}
```

## Implementation Divergence (verified 2026-08-26)

The design above describes a two-token scheme. **The shipped system uses a
single token.** Each row below was checked against source.

| ADR claims | Reality | Source |
|---|---|---|
| Access token lives **15 minutes** | **7 days** (`core.SessionLifetime`) | `pkg/core/config.go:128`, `pkg/auth/jwt.go:61` |
| `sub` is the **username**, "user ID intentionally excluded" | `sub` **is the user ID** (`strconv.Itoa(user.ID)`) | `pkg/auth/jwt.go:60,80` |
| Claims include `iat` and `jti` | Neither is set. Claims are `sub`, `session_id`, `exp` | `pkg/auth/jwt.go:78-83` |
| Separate long-lived **refresh token** in the DB | **No separate refresh token exists.** `sessions.data` stores the JWT itself | `pkg/db/schema.sql` (`sessions`) |
| HTTP-only cookie for refresh tokens is a "future" item | **Already shipped** — `SetJWTCookie` sets an HTTP-only `jwt` cookie | `pkg/auth/jwt.go:17` |

### How authentication actually works

1. `CreateToken` mints a JWT, creates a `sessions` row, then re-mints the token
   with `session_id` embedded and stores it in `sessions.data`.
2. The token is returned in the body **and** set as an HTTP-only `jwt` cookie.
   The frontend sends it as `Authorization: Bearer <token>`
   (`src/lib/api/client.ts:41`); `withCredentials: true` carries the cookie.
3. `GET /api/v1/auth/refresh` requires a **currently valid** token, reads `sub`,
   and issues a brand-new token plus a **new session row**. It is a token
   *rotation* endpoint, not a refresh-credential exchange.

**This is a deliberate design, not an oversight.** `pkg/core/config.go:115-127`
documents the reasoning: `ValidateSessionMiddleware` (mounted in
`pkg/http/root.go:52`) revalidates the `session_id` against the database on
**every authenticated request**, so a token dies the moment its session row is
deleted. Revocation, not a short expiry, is the containment mechanism — which is
why a 7-day lifetime is acceptable here and why the two-token scheme above was
unnecessary.

That comment also records an invariant worth preserving: the token `exp` claim
and `sessions.expires` **must** both derive from `SessionLifetime`, or a token
could outlive the row it authenticates against.

The practical caveat is that revocation is only as good as the code paths that
use it — and logout currently does not (see the flow above).

### Security notes for deployment

- `SetJWTCookie` has **`Secure: true` commented out unconditionally**
  (`pkg/auth/jwt.go:23`), so the cookie is transmissible over plain HTTP. This
  is not environment-gated. Set it before any production HTTPS deployment.
- `SameSite` is `Lax`.
- Each `/auth/refresh` call creates an *additional* session row, and logout does
  not remove rows, so the ADR's "Implement session cleanup for expired tokens"
  item is load-bearing, not optional.
- Logout is client-side only. If server-side invalidation on logout is wanted,
  `V1Logout` needs to delete the session identified by the token's `session_id`.

> **This section documents divergence, not a decision.** If the single-token
> design is intentional (it has real merits — server-side revocation, no refresh
> plumbing), it deserves its own ADR superseding this one.

## Future Considerations

### Planned Enhancements
- **HTTP-Only Cookies**: Move refresh tokens to secure cookies
- **Multi-Factor Authentication**: Add TOTP/SMS verification
- **Social Login**: Integration with OAuth2 providers
- **Passwordless Authentication**: Magic link or WebAuthn support

### Security Hardening
- **Device Fingerprinting**: Enhanced session tracking
- **Anomaly Detection**: Unusual login pattern detection
- **Token Binding**: Bind tokens to specific network characteristics
- **Audit Logging**: Comprehensive authentication event logging

## References
- [JWT Best Practices](https://tools.ietf.org/html/rfc8725)
- [OWASP Authentication Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html)
- [Go JWT Auth Library](https://github.com/go-chi/jwtauth)
- [bcrypt Documentation](https://pkg.go.dev/golang.org/x/crypto/bcrypt)
