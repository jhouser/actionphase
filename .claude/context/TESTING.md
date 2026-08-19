# Testing Context — Read Before Writing Tests

## The Core Principle: Verification & Validation

Every test must serve one or both of these purposes:

**Verification** — "Did we build it correctly?" Guards against regressions when code changes. A future refactor, a renamed struct field, a removed middleware — these break things silently. Tests catch them.

**Validation** — "Did we build the right thing?" Confirms the feature does what users actually need. A handler that returns 200 with the wrong data, an authorization check that passes when it shouldn't, a state mutation that succeeds but leaves the DB wrong — these only surface in production without validation tests.

**A test earns its place if a future developer can look at the failure and understand *what behavior broke*, not just that *something changed*.**

If a test only asserts "returns 200 and some JSON" on a simple read endpoint with no auth logic or conditional behavior, it adds noise without guarding anything real. Don't write it.

---

## What Deserves Tests (Decision Criteria)

Ask these questions, roughly in order:

1. **If this were silently wrong, would users be harmed or deceived?**
   - Authorization returning the wrong result → security issue. Always test.
   - State transition producing wrong DB state → data corruption. Always test.

2. **If someone refactors this, what fails silently?**
   - Response field names/types the frontend depends on → test the shape.
   - Business rules embedded in a handler (not just in a service) → test the handler.
   - Middleware applied in the wrong order → test the auth boundary explicitly.

3. **Is the behavior non-obvious from reading the code?**
   - "Only the author can edit, but the GM can also delete" → test both paths.
   - "Outsiders get 403, banned users get 401" → test the distinction.

4. **Would this path be caught immediately by manual testing if broken?**
   - A 500 on game creation → obvious. Low priority.
   - A published result being readable by a player who shouldn't see it → subtle. High priority.

**Categories that reliably clear the bar:**
- Access control (who can/can't — assert the 403, not just the 200)
- State mutations verified against DB (re-query after the HTTP call)
- Response shape for fields the frontend uses (specific field names, not just status codes)
- Behavioral distinctions (draft vs published, active vs inactive, GM vs player)
- Error paths with business meaning (not-found, forbidden, invalid state transition)

**Categories that rarely clear the bar:**
- Simple read endpoints: parse ID → call service → return JSON (no auth, no conditional logic)
- Infrastructure code: config loading, logger setup, CORS middleware
- External service wrappers: email sending, S3 uploads (test the integration, not the wrapper)
- Mock implementations and test helpers (these show up in coverage but are not production code)

---

## Previous Mistake to Avoid

An earlier version of this document listed `pkg/conversations`, `pkg/handouts`, `pkg/notifications`, and `pkg/phases` as packages where HTTP handler tests should be **skipped**, on the grounds that "business logic lives in the service layer." This was wrong. Those packages contain:

- Authorization checks that gate user access
- Response shapes the frontend depends on
- State transitions with DB consequences

We now have tests for all of them. The lesson: **the right question is never "is this a thin handler?" but "does this handler make decisions whose failure would be silent or harmful?"**

---

## HTTP Handler Test Pattern

This is the standard pattern. Every integration test follows it.

```go
func TestHandler_Endpoint(t *testing.T) {
    testDB := core.NewTestDatabase(t)
    defer testDB.Close()
    defer testDB.CleanupTables(t, "affected_table", "users")

    app := core.NewTestApp(testDB.Pool)
    router := setupTestRouter(app, testDB)

    gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
    player := testDB.CreateTestUser(t, "player", "player@example.com")
    outsider := testDB.CreateTestUser(t, "outsider", "outsider@example.com")

    game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")
    // ... set up additional state

    gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
    require.NoError(t, err)
    playerToken, err := core.CreateTestJWTTokenForUser(app, player)
    require.NoError(t, err)

    t.Run("GM succeeds and DB state is correct", func(t *testing.T) {
        req := httptest.NewRequest("POST", "/api/v1/...", body)
        req.Header.Set("Authorization", "Bearer "+gmToken)
        rec := httptest.NewRecorder()
        router.ServeHTTP(rec, req)

        assert.Equal(t, http.StatusOK, rec.Code)

        // Validate response shape — specific fields, not just status
        var response map[string]interface{}
        require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
        assert.Equal(t, expectedValue, response["field_name"])

        // Validate DB state actually changed (for mutations)
        var dbValue string
        err := testDB.Pool.QueryRow(ctx, "SELECT col FROM table WHERE id = $1", id).Scan(&dbValue)
        require.NoError(t, err)
        assert.Equal(t, expectedValue, dbValue)
    })

    t.Run("non-GM gets 403", func(t *testing.T) {
        req := httptest.NewRequest("POST", "/api/v1/...", body)
        req.Header.Set("Authorization", "Bearer "+playerToken)
        rec := httptest.NewRecorder()
        router.ServeHTTP(rec, req)

        assert.Equal(t, http.StatusForbidden, rec.Code)
    })

    t.Run("outsider gets 403", func(t *testing.T) {
        // Separate from player test — outsider is not a game participant at all
        req := httptest.NewRequest("POST", "/api/v1/...", body)
        req.Header.Set("Authorization", "Bearer "+outsiderToken)
        rec := httptest.NewRecorder()
        router.ServeHTTP(rec, req)

        assert.Equal(t, http.StatusForbidden, rec.Code)
    })
}
```

Key things every handler test should include:
- At least one assertion on response body content (not just status code)
- For mutations: a re-query of the DB to verify state changed
- Distinct test cases for different roles (GM, player, outsider) where the handler behaves differently
- The unauthenticated case (no token → 401) for any authenticated endpoint

---

## Service Layer Test Pattern

For business logic in `pkg/db/services/`:

```go
func TestService_Operation(t *testing.T) {
    testDB := core.NewTestDatabase(t)
    defer testDB.Close()

    service := &SomeService{DB: testDB.Pool}

    user := testDB.CreateTestUser(t, "user", "user@example.com")
    game := testDB.CreateTestGame(t, int32(user.ID), "Game")

    t.Run("succeeds with valid input", func(t *testing.T) {
        result, err := service.Operation(context.Background(), ValidRequest{...})
        require.NoError(t, err)
        // Assert specific fields — not just "no error"
        assert.Equal(t, expectedField, result.Field)
    })

    t.Run("returns error for invalid state", func(t *testing.T) {
        _, err := service.Operation(context.Background(), InvalidRequest{...})
        require.Error(t, err)
        // Where meaningful, assert the error type or message
        assert.Contains(t, err.Error(), "expected fragment")
    })
}
```

---

## Bug Fix Process (Mandatory)

1. Write a test that reproduces the bug — it must **fail** before the fix
2. Fix the bug
3. Verify the test **passes** after the fix
4. Commit test and fix together

The test should live at the layer where the bug lives:
- Logic bug in a service → service test
- Wrong HTTP status code or authorization → handler test
- Wrong response shape or field → handler test with body assertion
- Frontend state mutation → component test

Do not write a unit test for a bug that only manifests end-to-end. Write the test at the right level.

---

## Coverage as a Signal, Not a Target

Coverage numbers are useful for finding **where tests are absent**, not for measuring test quality. A package at 30% might have excellent tests for its 3 critical paths and no tests for 10 trivial ones. A package at 80% might have tests that assert nothing meaningful.

When reviewing coverage output:
- Zero-coverage functions are worth examining — ask "would this fail silently?"
- Low-coverage functions with complex conditional logic are worth examining
- Low-coverage functions that are simple pass-through wrappers can usually be left alone

Run coverage to **find gaps to evaluate**, not to hit a number:
```bash
TEST_DATABASE_URL="postgres://postgres:example@localhost:5432/actionphase_test?sslmode=disable" \
  SKIP_DB_TESTS=false go test -p=1 ./... -coverprofile=/tmp/coverage.out -covermode=atomic
go tool cover -func=/tmp/coverage.out | grep "0.0%"
```

---

## Test Commands

```bash
# Backend — all tests
just test

# Backend — fast unit tests (no DB)
just test-mocks

# Backend — integration tests only
just test-integration

# Backend — specific package
TEST_DATABASE_URL="postgres://..." SKIP_DB_TESTS=false go test -p=1 ./pkg/games/... -v

# Backend — specific test
TEST_DATABASE_URL="postgres://..." SKIP_DB_TESTS=false go test -p=1 ./pkg/games/... -run TestGameAPI_ListAll -v

# Frontend — all
just test-frontend

# Frontend — watch mode
just test-fe watch

# E2E — all tests (desktop + mobile)
just e2e

# E2E — desktop only
just e2e-desktop

# E2E — mobile only
just e2e-mobile

# E2E — specific file or headed mode
just e2e-test headed
just e2e-test headless messaging/common-room.spec.ts
```

---

## E2E Tests: Last, Not First

E2E tests (Playwright) are slow (~20-30s each), hard to debug, and provide poor failure messages. They are the **last layer**, written only after all lower-level tests pass.

**Required before writing any E2E test:**
1. Backend unit/integration test passes
2. API endpoint verified working (curl)
3. Frontend component test passes
4. Both servers running

**E2E tests are valuable for:**
- Full user journeys that cross multiple layers (login → submit action → see result)
- Flows where the integration between frontend state and backend API has caused bugs
- Regression tests for bugs that were first caught manually in the UI

**E2E tests are not valuable for:**
- Anything already covered by a component test
- Simple API correctness (use curl or a handler test)
- Authorization rules (handler tests are faster and more precise)

See `frontend/e2e/STATUS.md` for current E2E test coverage and `frontend/e2e/README.md` for the complete testing guide.

**Current E2E coverage** (41 spec files in `frontend/e2e/`):
- Auth, Games (lifecycle/applications/settings/co-GM), Gameplay (actions/phases/characters/handouts/polls/deadlines), Messaging (common room/private messages/mentions), Notifications, Settings, Admin, Security, Smoke, Edge cases

**Note on mobile testing**: `just e2e` runs both Chromium (desktop) and Pixel 5 (mobile) projects. When adding new tests, verify they pass on both projects — mobile can fail due to layout differences.

**Note on `just e2e-test`**: only the `file` mode takes a path —
`just e2e-test file e2e/games/foo.spec.ts`. Passing a path to `headless` silently
runs the **whole suite** instead.

**Some specs are flaky under parallel load**, notably
`edge-cases/unsaved-changes-warning` and `gameplay/action-submission-flow` (tight
3s waits plus `networkidle`). Before concluding a full-suite failure is yours:
re-run that spec alone with `just e2e-test file …`, and check whether the failing
*set* shifts between runs. A shifting set is flake, not regression. Playwright
also emits `Internal error: step id not found: fixture@NN` on a clean tree.

---

## Hidden Elements: In the DOM, Not in the A11y Tree

*Added 2026-08-19, from the game form tab split.*

Tabbed forms keep inactive panels mounted and hide them with `hidden` /
`display: none` (see ARCHITECTURE.md — unmounting breaks native validation).
This splits query behaviour, and the distinction is the point of the test:

Verified against `hidden` (i.e. `display: none`) in jsdom:

| Query | Sees hidden panel? |
|---|---|
| `getByTestId` | ✅ yes |
| **`getByLabelText`** | ✅ **yes** — does *not* consult the a11y tree |
| `getByRole` (any role, incl. `tooltip`) | ❌ no — a11y tree only |
| `getByRole(..., { hidden: true })` | ✅ yes (opt-in) |
| `toBeVisible()` | ❌ no (correctly) |
| Playwright `.click()` / `.fill()` | ❌ no — must open the tab first |
| Playwright `.inputValue()` / `.isChecked()` | ✅ yes (read-only) |

The trap is that `getByLabelText` and `getByRole` disagree, so a test can fill a
field on a hidden tab and then fail to assert on it — or pass while testing
nothing. A test asserting fields **stay mounted** must use `getByTestId` +
`toBeInTheDocument()`. That test earns its place: without it, a later
"optimisation" to unmount inactive tabs reintroduces the silent-submit bug with
nothing failing.

Counting elements across tabs (e.g. "all four tooltips render") must be done
**per tab**, since role queries only ever see the open one.

E2E page objects need an `openTab()` before interacting with a field on another
tab — but *not* before reading one. See `e2e/pages/GameSettingsPage.ts`.

---

## Quick Reference: Test File Locations

| What | Where |
|------|-------|
| HTTP handler tests | `pkg/<feature>/api_*_test.go` (same package) |
| Service tests | `pkg/db/services/*_test.go` |
| Middleware tests | `pkg/http/middleware/*_test.go` |
| Frontend component tests | `frontend/src/components/**/*.test.tsx` |
| E2E tests | `frontend/e2e/**/*.spec.ts` |

Reference implementations (good tests to copy patterns from):
- Handler tests: `pkg/admin/api_test.go`, `pkg/phases/api_lifecycle_test.go`, `pkg/games/api_audience_test.go`
- Service tests: `pkg/db/services/actions/submissions_test.go`, `pkg/db/services/games_test.go`
- E2E: `frontend/e2e/gameplay/action-submission-flow.spec.ts`, `frontend/e2e/messaging/common-room.spec.ts`
