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

The OpenAPI spec is **generated from Go types**, not maintained by hand. Every
route is a huma operation whose input and output structs describe the request
and response; `backend/pkg/docs/spec_metadata.go` supplies the document
metadata (info, servers, security, tag descriptions).

```bash
just gen-openapi      # regenerate backend/pkg/docs/openapi.gen.yaml
just check-api-docs   # fail if the committed copy is stale
```

`check-api-docs` regenerates the document to a temp file and diffs it against
the committed one, the same shape as `go mod tidy -diff` or `sqlc diff`. A
failure means the API changed without `just gen-openapi` being run; the fix is
to run it and commit the result. It runs as part of `just lint`, `just verify`
and `just verify-quick`.

Because the spec is derived from the code, the three drift states the previous
checker looked for are no longer reachable: a route that exists is documented by
construction, and a documented path cannot exist without a handler behind it.
The `scripts/api-docs-baseline.txt` debt ledger was retired with it.

Two chi routes are served but undocumented, and nothing will flag them — the
generator only sees huma operations. Both are deliberate and are recorded in
`spec_metadata.go`: the Discord OAuth callback (a 302 redirect with no JSON
contract) and `/uploads/*` (a local-only static file server).

## Workflow

### When Adding New Routes

1. **Implement the handler** as a huma operation — input and output structs plus
   a `huma.Register` call in the package's `huma_api.go`. The struct tags are
   the documentation: `doc:`, `minLength:`, `enum:` and the `Responses` map all
   appear in the spec.

2. **Register it** in `pkg/http/root.go` via the package's `RegisterHumaX`.

3. **Regenerate and commit the spec**:
   ```bash
   just gen-openapi
   ```

4. **Check it renders** at http://localhost:3000/api/v1/docs/ (the backend
   hot-reloads via Air)

If the operation introduces a new tag, add it to `specTags()` in
`pkg/docs/spec_metadata.go` — `TestSpecTagsCoverEveryOperation` fails otherwise.

### Pre-Commit Checklist

- [ ] `just check-api-docs` is green (the committed spec is current)
- [ ] The operation has a `Summary` — `TestSpecCoversEveryOperationWithASummary`
      enforces this
- [ ] Real error codes declared in the `Responses` map, not just the success case
- [ ] Swagger UI displays the endpoint correctly

---


**Last Updated**: 2025-11-24
