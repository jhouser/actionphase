# Scripts

This directory contains utility scripts for API documentation, testing, and maintenance operations.

## Overview

The hybrid automation approach ensures API documentation stays in sync with the actual implemented routes without requiring extensive code annotations.

## Tools

### Infrastructure & Maintenance

#### S3 Cache-Control Migration (`migrate-s3-cache-control.sh`)

Migrates existing S3 objects to have proper Cache-Control headers for optimal browser and CDN caching.

**When to use:**
- After upgrading to the new caching implementation
- To apply cache headers to existing avatar uploads

**Requirements:**
- AWS CLI (`aws` command)
- `jq` for JSON parsing
- S3 bucket credentials configured

**Usage:**
```bash
./scripts/migrate-s3-cache-control.sh
```

**Environment variables:**
- `S3_BUCKET`: S3 bucket name (required)
- `S3_REGION`: AWS region (optional, defaults to us-east-1)
- `S3_ENDPOINT`: S3-compatible endpoint URL (optional, for DigitalOcean Spaces/MinIO)

**What it does:**
1. Lists all objects with `avatars/` prefix
2. For each object, checks current Cache-Control header
3. Skips objects that already have correct headers
4. Copies objects in-place with new metadata (no re-upload)
5. Prints progress and summary

**Output example:**
```
================================
S3 Cache-Control Migration
================================

Bucket:        my-bucket
Region:        us-east-1
Prefix:        avatars/
Cache-Control: public, max-age=31536000, immutable

✓ Updated: avatars/users/1/1234567890.jpg (Content-Type: image/jpeg)
✓ Skipped (already correct): avatars/users/2/1234567891.jpg
Progress: 100 objects processed...

================================
✅ Migration complete!
================================

Total objects processed: 150
Updated:                 120
Skipped (already set):   30
Duration:                45s
```

**Notes:**
- Safe to run multiple times (idempotent)
- No data is lost (copy operation)
- Updates metadata only, doesn't re-upload files
- Can be interrupted and resumed safely

### Development & Testing

#### API Test Utility (`api-test.sh`)

Authenticated API testing from the host. Logs in, caches the JWT at
`/tmp/api-token.txt`, and reuses it for subsequent calls.

```bash
./backend/scripts/api-test.sh login-player   # or login-gm
./backend/scripts/api-test.sh games
curl -s -H "Authorization: Bearer $(cat /tmp/api-token.txt)" \
  http://localhost:3000/api/v1/games | jq
```

**Subcommands:** `health`, `status`, `login`, `login-gm`, `login-player`,
`test-token`, `games`, `game`, `characters`, `posts`,
`create-post`, `comments`, `create-comment`, `test-mentions`.

#### Dev Container Entrypoint (`dev-entrypoint.sh`)

Entrypoint for the backend dev container. Waits for Postgres to be resolvable
and accepting connections, then `exec`s Air so it becomes PID 1's child and
receives signals cleanly. Not run directly — `docker-compose.dev.yml` invokes it.

#### Endpoint Smoke Tests

- **`test_character_rename.sh`** — exercises `PUT /api/v1/characters/{id}/rename`
- **`test_game_settings_api.sh`** — game create/update with `is_anonymous` and
  `auto_accept_audience`

Both are standalone bash scripts hitting a running backend; they are not part of
`just test`.

#### Character Data Diagnostics (`fix_character_data_ids.sql`)

⚠️ **Diagnostic only — do not run automatically.** Finds character data rows with
missing `id` fields (corruption from an old draft-merge bug) across currency,
items, abilities, and skills. Read it before running anything it suggests.

```bash
docker compose -f docker-compose.dev.yml exec -T db \
  psql -U postgres -d actionphase -f /dev/stdin < backend/scripts/fix_character_data_ids.sql
```

### API Documentation

#### 1. Route Validator (`validate-api-docs.go`)

Compares registered routes in the Chi router against documented routes in `openapi.yaml`.

**Usage:**
```bash
just api-docs-validate
```

**Output:**
- Current coverage percentage
- List of missing routes (in code but not documented)
- List of extra routes (documented but not registered)

**Example:**
```
=== API Documentation Validation ===

Total registered routes: 107
Total documented routes: 42
Coverage: 39.3%

❌ Missing from documentation (65 routes):
   GET /api/v1/games/{gameId}/deadlines
   POST /api/v1/games/{gameId}/deadlines
   ...

✅ All routes are documented!
```

### 2. Skeleton Generator (`generate-doc-skeleton.go`)

Generates basic OpenAPI YAML skeletons for undocumented routes.

**Usage:**
```bash
just api-docs-generate > /tmp/skeleton.yaml
```

**Output:**
Generates OpenAPI-compliant YAML with:
- Appropriate HTTP methods
- Basic response codes (200/201/204/400/401)
- Placeholder descriptions marked with `TODO:`
- Auto-guessed tags based on path segments

**Example Output:**
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
```

### 3. Debug Endpoint (`/api/v1/debug/routes`)

**Development only** - Exposes all registered routes via HTTP endpoint.

**Usage:**
```bash
curl http://localhost:3000/api/v1/debug/routes | jq
```

**Output:**
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

**Security:** Only enabled when `ENVIRONMENT=development` in `.env`.

## Workflow

### When Adding New Routes

1. **Implement the route** in your handler (e.g., `pkg/games/api.go`)

2. **Register the route** in `pkg/http/root.go`:
   ```go
   r.Get("/games/{gameId}/deadlines", gameHandler.GetDeadlines)
   ```

3. **Validate documentation coverage**:
   ```bash
   just api-docs-validate
   ```

4. **Generate skeleton** for new route:
   ```bash
   just api-docs-generate > /tmp/skeleton.yaml
   ```

5. **Copy & enhance** the relevant section from `/tmp/skeleton.yaml` into `pkg/docs/openapi.yaml`:
   - Replace `TODO:` placeholders with real descriptions
   - Add proper request/response schemas
   - Add path parameters
   - Add examples

6. **Rebuild backend** to embed updated docs:
   ```bash
   go build -o actionphase main.go
   ```

7. **Verify** in Swagger UI:
   ```
   http://localhost:3000/api/v1/docs/
   ```

### Pre-Commit Checklist

Before committing new routes, ensure:

- [ ] `just api-docs-validate` shows no new missing routes
- [ ] All `TODO:` placeholders have been replaced
- [ ] Request/response schemas are defined in `components/schemas`
- [ ] Examples are provided for complex endpoints
- [ ] Swagger UI displays the endpoint correctly

## CI Integration (Future)

Add to `.github/workflows/ci.yml`:

```yaml
- name: Validate API Documentation
  run: |
    just dev & # Start server in background
    sleep 5     # Wait for startup
    just api-docs-validate
```

This will fail CI if routes are added without documentation.

## Implementation Details

### How Route Discovery Works

1. **Chi Router Walking**: Uses `chi.Walk()` to traverse the router tree and collect all registered routes
2. **Debug Endpoint**: Provides HTTP access to the route list for the validation script
3. **OpenAPI Parsing**: Uses `gopkg.in/yaml.v3` to parse `openapi.yaml` and extract documented paths
4. **Set Comparison**: Compares registered vs documented routes using Go maps

### Limitations

- **Route parameters**: Chi uses `{id}` syntax, OpenAPI also uses `{id}` (compatible)
- **Wildcard routes**: Routes with `/*` are skipped (internal Chi routes)
- **Middleware-only routes**: Routes that only apply middleware won't appear
- **Generated descriptions**: Auto-generated skeletons use simple heuristics for tags/descriptions

### Dependencies

- `gopkg.in/yaml.v3` - YAML parsing
- `github.com/go-chi/chi/v5` - Router introspection

Both already in `go.mod`.

## Future Enhancements

1. **Schema inference**: Analyze Go struct tags to generate component schemas
2. **Example extraction**: Parse test fixtures for realistic examples
3. **Description inference**: Use Go doc comments as OpenAPI descriptions
4. **Automated PR comments**: Bot that comments on PRs with coverage changes

## Troubleshooting

### "Error fetching routes: connection refused"

**Problem**: Backend server isn't running

**Solution**:
```bash
just dev  # Start the backend
```

### "server returned status 404"

**Problem**: Debug endpoint not registered or environment not set to development

**Solution**:
1. Check `.env` has `ENVIRONMENT=development`
2. Verify `pkg/http/root.go` registers the debug handler
3. Restart the server

### "No routes found"

**Problem**: Chi.Walk() not finding routes (likely router not properly passed)

**Solution**: Check that `WalkRoutes()` receives the correct router instance from the context.

---

**Last Updated**: 2025-11-24
