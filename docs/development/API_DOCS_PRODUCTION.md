# API Documentation - Production Configuration

**Last Updated**: 2025-11-15

## Production URLs

### ✅ API Documentation (OpenAPI/Swagger)
- **Swagger UI**: `https://api.action-phase.com/api/v1/docs/`
- **OpenAPI Spec (YAML)**: `https://api.action-phase.com/api/v1/docs/openapi.yaml`
- **OpenAPI Spec (alternate)**: `https://api.action-phase.com/api/v1/docs/openapi`

### ✅ VitePress Documentation
- **User Docs**: `https://action-phase.com/docs/`
- **Served from**: Backend embedded static files

## Production Status: ✅ READY

### What's Working

1. **Backend serves OpenAPI docs** ✅
   - Route: `/api/v1/docs/` (Swagger UI)
   - Route: `/api/v1/docs/openapi` (OpenAPI YAML)
   - Embedded in Go binary via `//go:embed`

2. **Nginx proxies API requests** ✅
   - Lines 114-137 in `nginx/nginx.prod.conf`
   - `/api/` proxied to backend:3000
   - Includes rate limiting and security headers

3. **Separate VitePress docs** ✅
   - Lines 176-192 in `nginx/nginx.prod.conf`
   - `/docs/` proxied to backend
   - Static files cached for 1 year

## Nginx Configuration

**File**: `nginx/nginx.prod.conf`

### API Endpoints (Lines 114-137)
```nginx
# API endpoints (backend)
location /api/ {
    # Rate limiting for API
    limit_req zone=api_limit burst=20 nodelay;
    limit_conn conn_limit 10;

    # Proxy to backend
    proxy_pass http://backend;
    proxy_http_version 1.1;

    # Headers
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header Connection "";

    # Timeouts
    proxy_connect_timeout 10s;
    proxy_send_timeout 30s;
    proxy_read_timeout 30s;

    # Disable buffering for SSE/WebSocket
    proxy_buffering off;
}
```

**This covers**:
- ✅ `/api/v1/docs/` (Swagger UI)
- ✅ `/api/v1/docs/openapi.yaml` (OpenAPI spec)
- ✅ All other `/api/*` endpoints

### VitePress Documentation (Lines 176-192)
```nginx
# Documentation (VitePress static files from backend)
location /docs/ {
    proxy_pass http://backend;
    proxy_http_version 1.1;

    # Headers
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;

    # Cache static documentation assets
    location ~* ^/docs/.*\.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|eot)$ {
        proxy_pass http://backend;
        expires 1y;
        add_header Cache-Control "public, immutable";
    }
}
```

**This covers**:
- ✅ `/docs/` (VitePress user documentation)
- ✅ Static assets cached for 1 year

## Verification Steps

### Local Development

```bash
# Start backend
cd backend
go build -o actionphase main.go
./actionphase &

# Test API docs
curl http://localhost:3000/api/v1/docs/ | grep "Swagger"
# Should return: HTML with "ActionPhase API Documentation"

curl http://localhost:3000/api/v1/docs/openapi.yaml | head -5
# Should return: openapi: 3.0.3

# Test in browser
open http://localhost:3000/api/v1/docs/
```

### Production Deployment

```bash
# After deployment, verify:

# 1. API Documentation
curl https://api.action-phase.com/api/v1/docs/ | grep "Swagger"

# 2. OpenAPI Spec
curl https://api.action-phase.com/api/v1/docs/openapi.yaml | head -5

# 3. Check response headers
curl -I https://api.action-phase.com/api/v1/docs/openapi.yaml
# Should return: HTTP/2 200
#                Content-Type: application/yaml
```

## Security Considerations

### ✅ Rate Limiting
API endpoints have rate limiting configured:
- **API calls**: 10 req/s (burst 20)
- **Auth endpoints**: 5 req/min (burst 2)

### ✅ HTTPS Only
Production config enforces HTTPS:
- Lines 68-90: HTTP → HTTPS redirect
- Lines 93-223: HTTPS server with TLS 1.2+

### ✅ Security Headers
Production includes security headers:
- `X-Frame-Options: SAMEORIGIN`
- `X-Content-Type-Options: nosniff`
- `X-XSS-Protection: 1; mode=block`
- Content Security Policy (CSP)

### ✅ No Debug Endpoint in Production
The `/api/v1/debug/routes` endpoint is **only enabled in development**:

**File**: `backend/pkg/http/root.go` (lines 468-474)
```go
// Debug routes (development only) - exposed via /api/v1/debug/*
if h.App.Config.App.Environment == "development" {
    debugHandler := &DebugHandler{}
    apiV1Router.Route("/debug", func(r chi.Router) {
        debugHandler.RegisterRoutes(r)
    })
}
```

When `ENVIRONMENT=production`, this endpoint is not registered.

## Two Documentation Systems

ActionPhase has **two separate documentation systems**:

### 1. API Documentation (for developers)
- **URL**: `https://api.action-phase.com/api/v1/docs/`
- **Purpose**: Interactive API reference (Swagger UI)
- **Audience**: Frontend developers, mobile developers, API consumers
- **Source**: generated from the huma operations' Go types, plus the document
  metadata in `backend/pkg/docs/spec_metadata.go`
- **Technology**: OpenAPI 3.0.3 + Swagger UI

### 2. User Documentation (for players/GMs)
- **URL**: `https://action-phase.com/docs/`
- **Purpose**: User guides, tutorials, help pages
- **Audience**: Players, Game Masters, end users
- **Source**: `docs-site/` (VitePress)
- **Technology**: VitePress (Vue-based static site generator)

**They are independent and serve different purposes!**

## Updating Documentation

### API Documentation (OpenAPI)

The spec is generated from the code, so there is nothing to edit by hand:
changing a handler's input/output structs changes the documentation.

1. Change the huma operation (struct tags carry `doc:`, `enum:`, `minLength:`
   and the `Responses` map)
2. Regenerate the committed copy so reviewers see the API diff:
   ```bash
   just gen-openapi
   ```
3. Rebuild and deploy the backend
4. Verify: `https://api.action-phase.com/api/v1/docs/`

The served document is assembled in memory at request time from the registered
operations, so it cannot go stale relative to the deployed binary. The committed
`openapi.gen.yaml` exists for review and for client generation;
`just check-api-docs` fails when it falls behind.

### User Documentation (VitePress)

1. Edit markdown files in `docs-site/`
2. Build documentation:
   ```bash
   just docs-embed
   ```
3. Rebuild backend (to embed new static files)
4. Deploy updated binary
5. Verify: `https://action-phase.com/docs/`

## Troubleshooting

### API docs return 404

**Check**:
```bash
# 1. Is backend running?
curl http://localhost:3000/ping

# 2. Are routes registered?
curl http://localhost:3000/api/v1/debug/routes | jq  # Dev only

# 3. Does the spec render?
curl -s http://localhost:3000/api/v1/docs/openapi.yaml | head -5
```

**Fix**:
The spec is built in memory from the registered operations, so a 404 here means
the docs routes are not registered rather than a missing file. Check that
`docsHandler.RegisterRoutes` still runs in `pkg/http/root.go`.

### Nginx returns 502 Bad Gateway

**Check**:
```bash
# Is backend container running?
docker-compose ps backend

# Check backend logs
docker-compose logs backend
```

**Fix**:
```bash
docker-compose restart backend
```

### OpenAPI spec shows old data

**Cause**: Spec is embedded in binary at compile time

**Fix**:
```bash
# Rebuild backend
cd backend
go build -o actionphase main.go

# Restart
docker-compose restart backend
```

## Production Checklist

Before deploying API documentation updates:

- [ ] Run `just gen-openapi` and commit `backend/pkg/docs/openapi.gen.yaml`
- [ ] Run `just check-api-docs` to verify the spec matches registered routes
- [ ] Replace all `TODO:` placeholders
- [ ] Test locally: `http://localhost:3000/api/v1/docs/`
- [ ] Rebuild backend: `go build -o actionphase main.go`
- [ ] Test Swagger UI displays correctly
- [ ] Try "Execute" on sample endpoints
- [ ] Commit changes to git
- [ ] Deploy to production
- [ ] Verify: `https://api.action-phase.com/api/v1/docs/`
- [ ] Test production Swagger UI
- [ ] Check OpenAPI spec loads: `/api/v1/docs/openapi.yaml`

## Links

**Correct Production URLs**:
- ✅ API Docs (Swagger): `https://api.action-phase.com/api/v1/docs/`
- ✅ OpenAPI Spec: `https://api.action-phase.com/api/v1/docs/openapi.yaml`
- ✅ User Docs (VitePress): `https://action-phase.com/docs/`

**Incorrect URLs** (these will 404):
- ❌ `https://api.action-phase.com/api/docs/` (missing /v1/)
- ❌ `https://api.action-phase.com/swagger/` (not configured)
- ❌ `https://api.action-phase.com/openapi.yaml` (missing /api/v1/docs/)

---

**Summary**: Production is **ready to serve API documentation**. No nginx changes needed. Just rebuild the backend after changing the API.
