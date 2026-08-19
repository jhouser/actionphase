# Feature Implementation Session

You are working on the ActionPhase platform, a turn-based gaming web application. Before starting any work, read `.claude/context/ARCHITECTURE.md` and `.claude/context/TESTING.md`.

## The Feature

**Feature description**: `[DESCRIBE THE FEATURE]`

**Affected area**: `[frontend / backend / both]`

**Scope**: `[small — single concern, fits in one session]`

---

## Before You Write Any Code

1. **Clarify scope first.** If the feature description is ambiguous, ask before proceeding. A focused feature is better than an over-engineered one.

2. **Read relevant source files.** Find the existing patterns closest to what you're building. Follow them. Do not invent new patterns.

3. **State your plan.** Before writing code, briefly describe:
   - What database changes are needed (if any)
   - What the new service method(s) will do
   - What the API endpoint(s) will look like
   - What the frontend hook/component will do
   - What tests you'll write at each layer

---

## Implementation Order

Follow the test pyramid bottom-up. **Do not skip layers.**

### 1. Database (if schema changes needed)
- Create migration: `just make_migration <name>`
- Write `.up.sql` and `.down.sql`
- Add SQL queries to `backend/pkg/db/queries/`
- Regenerate: `just sqlgen`

### 2. Backend Service
- Add method to interface in `backend/pkg/core/interfaces.go` first
- Implement in `backend/pkg/db/services/`
- **Write service test** using `core.NewTestDatabase(t)` (real DB, no mocks)
- Run: `just test` — must pass before proceeding

### 3. Backend Handler
- Add route in the appropriate `api.go` or `root.go`
- Implement handler using existing patterns
- **Verify with curl** using `backend/scripts/api-test.sh`

### 4. Frontend Hook
- Add API client method to `frontend/src/lib/api.ts`
- Add React Query hook in `frontend/src/hooks/`
- **Write hook test** — verify query key, loading state, returned data shape

### 5. Frontend Component
- Implement UI using components from `@/components/ui` (no raw HTML elements)
- Use semantic CSS tokens (`bg-bg-primary`, `text-text-primary`, etc.) for layout — no hardcoded colors
- **Write component test** using React Testing Library — test what the user sees, not internals
- Run: `just test-frontend` — must pass before proceeding

### 6. Manual Verification
- Start servers: `just dev` + `just run-frontend`
- Walk through the feature manually
- Check browser console for errors
- Verify in both light and dark mode

---

## Test Quality Rules

- Tests must assert real behavior — not just that code runs without crashing
- Backend: assert the actual data returned or state changed, not that a function was called
- Frontend: test what the user sees and can interact with
- Do not write tests that trivially pass regardless of the implementation

**Test placement:**
- Backend: co-locate `_test.go` with the service file
- Frontend: co-locate `.test.tsx` / `.test.ts` with the component or hook

---

## Constraints

- **Do not commit.** The user handles all git operations.
- **Do not over-engineer.** Implement exactly what was described. Note adjacent improvements but don't make them.
- **No hardcoded colors.** Use `@/components/ui` and CSS tokens.
- **Interface first.** Define the service interface before implementing it.
- **E2E tests are last** — only if specifically requested, and only after all other layers pass.

---

## Output Format

After completing the feature:

1. **Summary**: What was built and how it fits into the existing system
2. **API**: Endpoint(s) added (method, path, request/response shape)
3. **Tests**: What each test layer covers
4. **Anything noteworthy**: Decisions made, trade-offs, related issues spotted (but not fixed)
