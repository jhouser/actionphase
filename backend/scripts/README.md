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

The OpenAPI spec (`backend/pkg/docs/openapi.yaml`) is maintained by hand and
verified by `just check-api-docs`, which compares it against the routes
registered in `backend/pkg/http/root.go`.

```bash
just check-api-docs
```

It reports three kinds of drift:

- **undocumented** — a route exists but the spec omits it
- **phantom** — the spec describes a path with no handler (auth middleware
  returns 401 rather than 404, which disguises the cause)
- **unreachable** — documented under `/api/v1` but registered at the root, so
  the documented URL 404s

Pre-existing gaps are listed in `scripts/api-docs-baseline.txt` and ignored, so
the check fails only on *new* drift. It runs as part of `just lint`,
`just verify`, and `just verify-quick`. Backfill progress for the baselined
routes is tracked in `.claude/planning/openapi-backfill.md`.

**Never add a new route to the baseline to make the check pass** — the baseline
only shrinks.

> Two earlier tools, `validate-api-docs.go` and `generate-doc-skeleton.go`
> (`just api-docs-validate`), were removed in August 2026. Both read the
> `/api/v1/debug/routes` endpoint, whose `listRoutes` in `pkg/http/debug.go`
> walks the *matched* subrouter and skips any path containing `/*`. They saw 9
> routes instead of ~195 and reported coverage of 688%. `just check-api-docs`
> parses `root.go` directly and has no such dependency.

## Workflow

### When Adding New Routes

1. **Implement the route** in your handler (e.g., `pkg/games/api.go`)

2. **Register the route** in `pkg/http/root.go`

3. **Document it** in `pkg/docs/openapi.yaml` — path, parameters, request body,
   response schema, and the real error codes (`401` on authenticated routes,
   `404` on `{id}` paths, `403` where permissions apply)

4. **Verify**:
   ```bash
   just check-api-docs
   ```

5. **Check it renders** at http://localhost:3000/api/v1/docs/ (the backend
   hot-reloads the embedded spec via Air)

### Pre-Commit Checklist

- [ ] `just check-api-docs` is green
- [ ] Request/response schemas reference `components/schemas` where one fits
- [ ] Error responses documented, not just the success case
- [ ] Swagger UI displays the endpoint correctly

---


**Last Updated**: 2025-11-24
