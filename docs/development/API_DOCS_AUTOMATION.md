# API Documentation Automation

> ⚠️ **`just api-docs-validate` is currently broken and its numbers are
> meaningless.** It reports `Total registered routes: 9` and
> `Coverage: 700.0%`.
>
> Cause: `listRoutes` in `backend/pkg/http/debug.go` walks `ctx.Routes` — the
> *matched* subrouter, not the root — and its walk function skips any route
> containing `/*`. `chi.Walk` therefore never descends into the mounted API
> subrouters. `/api/v1/games` returns 200 but does not appear in
> `/api/v1/debug/routes`; the real count is ~184 routes registered in
> `backend/pkg/http/root.go`.
>
> Until `debug.go` is fixed, treat the "documented but not registered" list as
> noise and the coverage percentage as invalid.



**Status**: ✅ Implemented
**Last Updated**: 2025-11-15

## Overview

Automated tools to keep OpenAPI documentation (`pkg/docs/openapi.yaml`) in sync with actual implemented routes.

## Components

### 1. Debug Endpoint (`/api/v1/debug/routes`)

**Development-only HTTP endpoint** that exposes all registered routes from the Chi router.

**Implementation**:
- File: `backend/pkg/http/debug.go`
- Registration: `backend/pkg/http/root.go` (lines 468-474)
- Only enabled when `ENVIRONMENT=development` in `.env`

**Usage**:
```bash
curl http://localhost:3000/api/v1/debug/routes | jq
```

**Output**:
```json
[
  {
    "method": "GET",
    "path": "/api/v1/ping"
  },
  {
    "method": "POST",
    "path": "/api/v1/auth/login"
  },
  ...
]
```

### 2. Route Validator (`backend/scripts/validate-api-docs.go`)

Compares registered routes against documented routes in `openapi.yaml`.

**Usage**:
```bash
just api-docs-validate
```

**Example Output**:
```
=== API Documentation Validation ===

Total registered routes: 107
Total documented routes: 42
Coverage: 39.3%

❌ Missing from documentation (65 routes):
   GET /api/v1/games/{gameId}/deadlines
   POST /api/v1/games/{gameId}/deadlines
   ...

💡 Tip: Run 'go run scripts/generate-doc-skeleton.go' to generate skeletons for missing routes
```

**Exit Codes**:
- `0` - All routes documented
- `1` - Missing or extra routes found

### 3. Skeleton Generator (`backend/scripts/generate-doc-skeleton.go`)

Generates basic OpenAPI YAML for undocumented routes.

**Usage**:
```bash
docker compose -f docker-compose.dev.yml exec -T backend go run scripts/generate-doc-skeleton.go > /tmp/skeleton.yaml
```

**Example Output**:
```yaml
  /games/{gameId}/deadlines:
    get:
      summary: TODO: GET /games/{gameId}/deadlines
      description: TODO: Add description
      tags:
        - Games
      responses:
        '200':
          description: Success
          content:
            application/json:
              schema:
                type: object
        '400':
          description: Bad request
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '401':
          description: Unauthorized
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
    post:
      summary: TODO: POST /games/{gameId}/deadlines
      ...
```

## Workflow

### Adding New Routes

1. **Implement** the route in your handler:
   ```go
   func (h *GameHandler) GetDeadlines(w http.ResponseWriter, r *http.Request) {
       // implementation
   }
   ```

2. **Register** in `pkg/http/root.go`:
   ```go
   r.Get("/games/{gameId}/deadlines", gameHandler.GetDeadlines)
   ```

3. **Validate** documentation coverage:
   ```bash
   just api-docs-validate
   ```

4. **Generate** skeleton for new route:
   ```bash
   docker compose -f docker-compose.dev.yml exec -T backend go run scripts/generate-doc-skeleton.go > /tmp/skeleton.yaml
   ```

5. **Enhance** the skeleton:
   - Copy relevant sections from `/tmp/skeleton.yaml` to `pkg/docs/openapi.yaml`
   - Replace all `TODO:` placeholders with real descriptions
   - Add proper request/response schemas
   - Add path parameters
   - Add examples

6. **Rebuild** backend to embed updated docs:
   ```bash
   go build -o actionphase main.go
   ```

7. **Verify** in Swagger UI:
   ```
   http://localhost:3000/api/v1/docs/
   ```

### Pre-Commit Checklist

Before committing new routes:

- [ ] `just api-docs-validate` shows no new missing routes
- [ ] All `TODO:` placeholders replaced with real descriptions
- [ ] Request/response schemas defined in `components/schemas`
- [ ] Examples provided for complex endpoints
- [ ] Swagger UI displays endpoint correctly
- [ ] Test the endpoint with curl or Postman

## CI Integration

Add to `.github/workflows/ci.yml`:

```yaml
- name: Validate API Documentation
  run: |
    cd backend
    go build -o actionphase main.go
    ./actionphase > /tmp/server.log 2>&1 &
    sleep 5  # Wait for server startup
    just api-docs-validate
```

This will fail CI if routes are added without documentation.

## Implementation Details

### Route Discovery

1. **Chi Router Walking**: Uses `chi.Walk()` to traverse router tree
2. **Debug Endpoint**: Provides HTTP access to route list
3. **OpenAPI Parsing**: Uses `gopkg.in/yaml.v3` to parse `openapi.yaml`
4. **Set Comparison**: Compares registered vs documented using maps

### Security

- Debug endpoint **only enabled in development** (`ENVIRONMENT=development`)
- Not included in production builds
- No authentication required (development only)

### Limitations

- **Route parameters**: Chi `{id}` syntax matches OpenAPI (compatible)
- **Wildcard routes**: Routes with `/*` are skipped (internal Chi routes)
- **Middleware-only routes**: Won't appear in route list
- **Generated descriptions**: Use simple heuristics (tag guessing)

## Future Enhancements

1. **Schema inference**: Analyze Go struct tags → component schemas
2. **Example extraction**: Parse test fixtures → realistic examples
3. **Description inference**: Use Go doc comments → OpenAPI descriptions
4. **Automated PR comments**: Bot comments on PRs with coverage changes
5. **IDE integration**: VS Code extension for inline documentation hints

## Files Modified/Created

### Created:
- `backend/pkg/http/debug.go` - Debug endpoint handler
- `backend/scripts/validate-api-docs.go` - Route validator
- `backend/scripts/generate-doc-skeleton.go` - Skeleton generator
- `backend/scripts/README.md` - Detailed automation guide
- `docs/development/API_DOCS_AUTOMATION.md` - This file

### Modified:
- `backend/pkg/http/root.go` - Register debug endpoint
- `justfile` - Add `api-docs-validate` and `api-docs-generate` commands

## Troubleshooting

### "Error fetching routes: connection refused"

**Problem**: Backend server isn't running

**Solution**:
```bash
just up
```

### "server returned status 404"

**Problem**: Debug endpoint not registered or not in development mode

**Solution**:
1. Check `.env` has `ENVIRONMENT=development`
2. Verify `pkg/http/root.go` registers debug handler (line 469)
3. Restart server

### "No routes found" or "Empty route list"

**Problem**: Chi.Walk() not finding routes

**Solution**:
- Ensure server has fully started (wait 5 seconds after launch)
- Check that routes are properly registered in `pkg/http/root.go`
- Verify `chi.Walk()` receives correct router instance

## Related Documentation

- **OpenAPI Specification**: `backend/pkg/docs/openapi.yaml`
- **Script README**: `backend/scripts/README.md`
- **API Documentation Guide**: `/docs/development/API_DOCUMENTATION.md`
- **Developer Onboarding**: `/docs/getting-started/DEVELOPER_ONBOARDING.md`

---

**Maintainer**: Development Team
**Feedback**: Open an issue or PR for improvements
