# API Documentation Quick Start Guide

**Quick commands to maintain OpenAPI documentation**

## Prerequisites

```bash
# Ensure backend server is running
just up

# Or manually:
cd backend
go build -o actionphase main.go
./actionphase &
```

## Common Tasks

### 1. Check Documentation Coverage

```bash
cd backend
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
   ...
```

### 2. Generate Skeleton for Missing Routes

```bash
cd backend
docker compose -f docker-compose.dev.yml exec -T backend go run scripts/generate-doc-skeleton.go > /tmp/skeleton.yaml
```

Then review `/tmp/skeleton.yaml` and copy relevant sections to `pkg/docs/openapi.yaml`.

### 3. Add New Endpoint (Full Workflow)

#### Step 1: Implement the Route

**File**: `pkg/games/api.go`
```go
func (h *GameHandler) GetDeadlines(w http.ResponseWriter, r *http.Request) {
    // implementation
}
```

#### Step 2: Register the Route

**File**: `pkg/http/root.go`
```go
r.Get("/games/{gameId}/deadlines", gameHandler.GetDeadlines)
```

#### Step 3: Generate Documentation Skeleton

```bash
cd backend
go run scripts/generate-doc-skeleton.go > /tmp/new-routes.yaml
```

#### Step 4: Enhance the Skeleton

Copy from `/tmp/new-routes.yaml` to `pkg/docs/openapi.yaml`:

**Before** (generated skeleton):
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
```

**After** (enhanced):
```yaml
  /games/{gameId}/deadlines:
    get:
      summary: List game deadlines
      description: Retrieve all deadlines for a specific game, including upcoming and past deadlines
      tags:
        - Deadlines
      parameters:
        - name: gameId
          in: path
          required: true
          schema:
            type: integer
          description: Game ID
      responses:
        '200':
          description: List of deadlines
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: '#/components/schemas/Deadline'
        '404':
          description: Game not found
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
```

#### Step 5: Add Component Schemas

**In `components/schemas` section**:
```yaml
    Deadline:
      type: object
      required:
        - id
        - game_id
        - title
        - due_at
      properties:
        id:
          type: integer
          format: int64
          example: 123
        game_id:
          type: integer
          format: int64
          example: 42
        title:
          type: string
          example: "Submit Character Actions"
        description:
          type: string
          example: "Submit your character's actions for Phase 3"
        due_at:
          type: string
          format: date-time
          example: "2025-11-20T23:59:59Z"
        created_at:
          type: string
          format: date-time
        updated_at:
          type: string
          format: date-time
```

#### Step 6: Validate

```bash
just api-docs-validate
```

Should show the new route as documented.

#### Step 7: Rebuild & Verify

```bash
# Rebuild backend to embed updated docs
go build -o actionphase main.go

# Restart server
lsof -ti:3000 | xargs kill
./actionphase &

# Verify in Swagger UI
open http://localhost:3000/api/v1/docs/
```

## Justfile Commands

```bash
just api-docs-validate    # Check documentation coverage
just sh backend            # then: go run scripts/generate-doc-skeleton.go
```

## Direct Script Usage

```bash
# From backend directory:
go run scripts/validate-api-docs.go
go run scripts/generate-doc-skeleton.go > /tmp/skeleton.yaml
```

## Debug Endpoint

**Development only** - View all registered routes:

```bash
curl http://localhost:3000/api/v1/debug/routes | jq
```

Example output:
```json
[
  {
    "method": "GET",
    "path": "/api/v1/ping"
  },
  {
    "method": "POST",
    "path": "/api/v1/auth/login"
  }
]
```

## Tips

### Replace TODO Markers

Always search for and replace `TODO:` placeholders:
```bash
grep -n "TODO:" pkg/docs/openapi.yaml
```

### Test Endpoints in Swagger UI

1. Navigate to `http://localhost:3000/api/v1/docs/`
2. Find your endpoint
3. Click "Try it out"
4. Fill in parameters
5. Click "Execute"
6. Verify response matches schema

### Validate YAML Syntax

```bash
# Check for YAML syntax errors
python3 -c "import yaml; yaml.safe_load(open('pkg/docs/openapi.yaml'))"
```

## Common Issues

### "Error fetching routes: connection refused"

Server isn't running. Start it:
```bash
just up
```

### "Coverage: 480%"

You're documenting routes that don't exist yet. Remove unused documentation or implement the missing routes.

### "YAML syntax error"

Check indentation (2 spaces, no tabs):
```bash
# View with visible whitespace
cat -A pkg/docs/openapi.yaml | grep "TODO" | head -5
```

## CI Integration

Add to `.github/workflows/ci.yml`:

```yaml
- name: Validate API Documentation
  run: |
    cd backend
    go build -o actionphase main.go
    ./actionphase > /tmp/server.log 2>&1 &
    sleep 5
    go run scripts/validate-api-docs.go
```

---

**See Also**:
- Full automation guide: `/docs/development/API_DOCS_AUTOMATION.md`
- Technical details: `/backend/scripts/README.md`
