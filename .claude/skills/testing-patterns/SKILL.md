---
name: testing-patterns
description: Testing guide for ActionPhase. Covers when to write tests (V&V criteria), backend handler and service test patterns, frontend component tests, and E2E rules. Use when deciding whether to write a test, writing any type of test, or debugging failures. CRITICAL: tests must verify behavior, not just execution — read the V&V decision framework before writing anything.
---

# Testing Patterns

## The Core Principle: Verification & Validation (V&V)

Every test must serve one or both purposes:

**Verification** — "Did we build it correctly?" Guards against regressions. A refactor that renames a response field, a middleware accidentally removed, a service call swapped out — these fail silently without tests.

**Validation** — "Did we build the right thing?" Confirms behavior users depend on. A handler that returns 200 with the wrong data, an auth check that passes when it shouldn't, a state mutation that succeeds but corrupts the DB.

**A test earns its place if a future developer can look at the failure and understand *what behavior broke*, not just that *something changed*.**

---

## The V&V Decision Framework

Before writing any test, ask:

**1. If this were silently wrong, would users be harmed or deceived?**
- Authorization returning wrong result → security issue. Always test.
- State transition producing wrong DB state → data corruption. Always test.

**2. If someone refactors this, what fails silently?**
- Response field names the frontend uses → test the shape.
- Business rules in a handler (not just in a service) → test the handler.
- Middleware applied in wrong order → test the auth boundary explicitly.

**3. Would this path be caught immediately by manual testing if broken?**
- A 500 on game creation → obvious. Lower priority.
- A player silently reading data they should not → subtle. High priority.

**Always test:**
- Access control: assert 403/401 explicitly — do not only test the happy path
- State mutations: re-query the DB after the HTTP call to verify the change
- Response shape: assert specific field names and values the frontend uses
- Behavioral distinctions: draft vs published, GM vs player, active vs inactive

**Usually skip:**
- Simple read endpoints with no auth logic and no conditional behavior
- Infrastructure wrappers: email sending, S3, config loading
- Mock implementations and test utilities (inflate coverage, test nothing)

### The Mistake to Avoid

A previous version of this guidance said handler tests should be skipped for "thin wrapper" packages like `pkg/phases`, `pkg/conversations`, `pkg/handouts`, `pkg/notifications`. **This was wrong.** Those handlers make authorization decisions and shape responses that the frontend depends on. We now have tests for all of them.

The right question is never "is this a thin handler?" but "does this handler make decisions whose failure would be silent or harmful?"

---

## Testing Pyramid

```
4. E2E (Playwright)        <- Slow (~20-30s each). LAST step only.
3. Frontend Components     <- Test conditional rendering and interactions
2. HTTP Handler Tests      <- Test auth, response shape, state mutations
1. Service / Unit Tests    <- Test business logic and DB operations
```

Work bottom-up. E2E only after lower layers pass for the same feature.

---

## Backend Handler Test Pattern

Standard pattern for all HTTP handler integration tests:

```go
func TestHandler_Operation(t *testing.T) {
    testDB := core.NewTestDatabase(t)
    defer testDB.Close()
    defer testDB.CleanupTables(t, "affected_table", "users")

    app := core.NewTestApp(testDB.Pool)
    router := setupTestRouter(app, testDB)

    gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
    player := testDB.CreateTestUser(t, "player", "player@example.com")
    game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")
    gmToken, _ := core.CreateTestJWTTokenForUser(app, gm)
    playerToken, _ := core.CreateTestJWTTokenForUser(app, player)

    t.Run("GM succeeds — response shape and DB state correct", func(t *testing.T) {
        req := httptest.NewRequest("POST", "/api/v1/path", body)
        req.Header.Set("Authorization", "Bearer "+gmToken)
        rec := httptest.NewRecorder()
        router.ServeHTTP(rec, req)

        assert.Equal(t, http.StatusOK, rec.Code)

        // Assert specific fields — not just status code
        var resp map[string]interface{}
        json.Unmarshal(rec.Body.Bytes(), &resp)
        assert.Equal(t, "expected_value", resp["field_name"])

        // For mutations: verify DB state changed
        var val string
        testDB.Pool.QueryRow(ctx, "SELECT col FROM table WHERE id=$1", id).Scan(&val)
        assert.Equal(t, "expected_value", val)
    })

    t.Run("non-GM player gets 403", func(t *testing.T) {
        req := httptest.NewRequest("POST", "/api/v1/path", body)
        req.Header.Set("Authorization", "Bearer "+playerToken)
        rec := httptest.NewRecorder()
        router.ServeHTTP(rec, req)
        assert.Equal(t, http.StatusForbidden, rec.Code)
    })
}
```

Good reference implementations to copy patterns from:
- `pkg/admin/api_test.go` — admin access control + DB state verification
- `pkg/phases/api_lifecycle_test.go` — GM-only operations with role assertions
- `pkg/games/api_audience_test.go` — access control with response shape validation

---

## Coverage: Signal, Not Target

Coverage tells you **where tests are absent**. It does not measure test quality.

Use coverage to find gaps to *evaluate*:
```bash
TEST_DATABASE_URL="postgres://postgres:example@localhost:5432/actionphase_test?sslmode=disable" \
  SKIP_DB_TESTS=false go test -p=1 ./... -coverprofile=/tmp/coverage.out -covermode=atomic
go tool cover -func=/tmp/coverage.out | grep "0.0%"
```

When you find a zero-coverage function: apply the V&V framework above. Do not write a test just because coverage flagged it. A simple read endpoint at 0% may be fine. An authorization function at 0% is not.

---

## Bug Fix Process (Mandatory)

1. Write a test that reproduces the bug — must **fail** before the fix
2. Fix the bug
3. Verify the test **passes** after the fix
4. Commit test and fix together

Write the test at the layer where the bug lives:
- Logic bug in a service → service test
- Wrong HTTP status or authorization → handler test
- Wrong response shape or field → handler test with body assertion
- Frontend rendering → component test

---

## Key Commands

```bash
# Backend
just test                  # All tests
just test-mocks            # Fast unit tests (no DB, ~300ms)
just test-integration      # DB integration tests only
just ci-test               # Full CI suite (lint + test + race)

# Run specific package or test (runs in the backend container)
just test-run TestGameAPI_ListAll

# Note: do NOT pass `go test -p=1`. Each test package clones its own database
# from a migrated template, so packages are safe to run in parallel. Set
# TEST_P=1 only when serializing to debug.

# Frontend
just test-fe run           # All frontend tests
just test-fe watch         # Watch mode

# E2E
just e2e                   # Desktop + mobile
just e2e-desktop           # chromium only
just e2e-mobile            # Pixel 5 only
just e2e-test file <spec>  # One spec file
just e2e-test report       # HTML report
just load-e2e              # Load E2E fixtures
```

---

## E2E Tests: Last Step

**Required before writing any E2E test:**
1. Backend unit/integration test passes for the feature
2. API endpoint verified working (curl or handler test)
3. Frontend component test passes
4. Both servers running

**E2E tests are valuable for:** full user journeys across multiple layers, flows where frontend/backend integration has caused real bugs.

**E2E tests are not valuable for:** authorization rules (use handler tests), API correctness (use handler tests or curl), anything already covered by a component test.

**See**: [e2e-testing.md](resources/e2e-testing.md) for complete rules, debugging protocol, and fixture guide.

---

## Navigation

| Task | Resource |
|------|----------|
| Full pattern reference with examples | `.claude/context/TESTING.md` |
| E2E rules, debugging, fixtures | [e2e-testing.md](resources/e2e-testing.md) |
| Test fixture data and setup | [test-fixtures.md](resources/test-fixtures.md) |
| Frontend component testing | `.claude/context/TESTING.md` (Frontend section) |
