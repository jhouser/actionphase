# Review Uncommitted Changes

**Usage**: `/review-changes [plan-name]`

**Purpose**: Perform a comprehensive pre-commit code review of uncommitted changes, ensuring they meet project standards and optionally validating alignment with a specific implementation plan.

---

## Instructions

You are performing a pre-commit code review. Follow this systematic process:

### 1. Get Uncommitted Changes

```bash
# Get all uncommitted changes (staged and unstaged)
git diff HEAD

# Also check for untracked files that should be reviewed
git status --short
```

### 2. Load Relevant Documentation

Based on what changed, load the appropriate documentation:

**Always Read**:
- **`.claude/context/TESTING.md`** - Testing requirements (MANDATORY for all code changes)

**Backend Changes**:
- Use the `backend-dev-guidelines` skill for Go/Chi/PostgreSQL patterns
- Check **`/docs/adrs/`** for relevant architectural decisions (auth, database, API design)
- Reference **`.claude/reference/ERROR_HANDLING.md`** for error patterns
- Reference **`.claude/reference/LOGGING_STANDARDS.md`** for observability

**Frontend Changes**:
- Use the `frontend-dev-guidelines` skill for React/TypeScript patterns
- Read **`/frontend/src/components/ui/README.md`** for UI component usage (CRITICAL for styling)
- Check **`.claude/context/STATE_MANAGEMENT.md`** for data fetching patterns
- Reference **`/docs/adrs/005-frontend-state-management.md`** for state architecture

**Game Domain Changes**:
- Use the `game-domain` skill for game state, phases, characters, and messaging patterns

**Test Changes**:
- Use the `testing-patterns` skill for test structure and patterns
- Check **`/docs/testing/COVERAGE_STATUS.md`** for current coverage expectations

### 3. Load Plan (if specified)

If the user provided a plan name:

**3a. Flexible Plan Loading** - Search for plan in multiple locations:
1. **Directory-based plans** (primary):
   - Try `.claude/planning/active/[plan-name]/plan.md`
   - Try `.claude/planning/active/[plan-name]/index.md`
   - Try `.claude/planning/active/[plan-name]/*.md` (any markdown file in directory)
2. **Single-file plans** (fallback):
   - Try `.claude/planning/active/[plan-name].md`
   - Try `.claude/planning/FEATURE_[plan-name].md`
3. **Glob search** (if above fail):
   - Use Glob pattern: `.claude/planning/**/*[plan-name]*.md`
   - If multiple matches, use the most recently modified file

**3b. Compare changes against plan**:
   - Plan phases and implementation order
   - Stated requirements and acceptance criteria
   - API contracts defined in the plan
   - Database schema changes planned
   - Test coverage requirements from plan

### 4. Pre-Commit Checklist Review

Systematically check each category:

#### 🧪 Testing Requirements (CRITICAL)
- [ ] **New features have tests** (unit/integration/component/E2E as appropriate)
- [ ] **Bug fixes include regression tests** that reproduce the bug
- [ ] **Tests follow the test pyramid** (unit → API → component → E2E)
- [ ] **Backend tests use table-driven patterns** where appropriate
- [ ] **Frontend tests use React Testing Library** with `screen` queries
- [ ] **E2E tests are LAST** (only after unit/API/component tests pass)
- [ ] **Mock external dependencies** appropriately

#### 🏗️ Architecture & Code Quality
- [ ] **Interface-first development** (backend changes define interfaces in `core/interfaces.go`)
- [ ] **Clean Architecture layers** respected (routes → handlers → services → database)
- [ ] **DRY principle** followed (no duplicated logic)
- [ ] **Error handling** is comprehensive with proper logging
- [ ] **Type safety** maintained (TypeScript strict mode, Go type checking)
- [ ] **No hardcoded values** (configuration, secrets, magic numbers)

#### 🎨 Frontend Standards (if applicable)
- [ ] **UI Component Library used** (`@/components/ui`) instead of native HTML elements
  - ❌ No `<button>`, use `<Button>`
  - ❌ No `<input>`, use `<Input>`
  - ❌ No manual divs with styling, use `<Card>`, `<Modal>`, etc.
- [ ] **Semantic tokens** used for layout (`surface-*`, `text-content-*`, `border-theme-*`) — never retired `bg-bg-*`/`border-border-*`
- [ ] **Dark mode support** via CSS variables (no hardcoded colors)
- [ ] **React Query** used for data fetching (not useEffect with fetch)
- [ ] **AuthContext** used for auth state (not local state)

#### 🔒 Security & Best Practices
- [ ] **No secrets in code** (.env usage, no hardcoded passwords/keys/tokens)
- [ ] **SQL injection prevention** (parameterized queries only)
- [ ] **XSS prevention** (proper escaping, no dangerouslySetInnerHTML without sanitization)
- [ ] **CSRF protection** maintained
- [ ] **Input validation** on both frontend and backend

#### 📝 Observability
- [ ] **Structured logging** with correlation IDs (backend)
- [ ] **Error logging** includes context
- [ ] **Console logging** removed or converted to proper logging

#### 📚 Documentation
- [ ] **Public functions documented** (Go comments, JSDoc)
- [ ] **Complex logic explained** with comments
- [ ] **API changes reflected** in documentation
- [ ] **README updated** if needed for new features

#### 🗃️ Database Changes
- [ ] **Migrations created** for schema changes (`just make_migration`)
- [ ] **Both up and down migrations** written
- [ ] **SQL queries added** to `backend/pkg/db/queries/`
- [ ] **Code regenerated** with `just sqlgen`
- [ ] **Indexes considered** for new queries

### 5. Generate Review Report

Provide a structured review report:

```markdown
## Code Review Summary

**Changes Reviewed**: [Brief description of what changed]
**Plan Alignment**: [If plan specified: alignment status, or "No plan specified"]

### ✅ Passes
- [List items that meet standards]

### ⚠️ Issues Found
- [List violations with file:line references and explanations]

### 🔧 Required Before Commit
1. [Blocking issues that MUST be fixed]
2. [In priority order]

### 💡 Suggestions (Optional)
- [Non-blocking improvements]

### 📋 Checklist Status
- Testing: [X/Y checks passed]
- Architecture: [X/Y checks passed]
- Frontend: [X/Y checks passed]
- Security: [X/Y checks passed]
- Documentation: [X/Y checks passed]

### Recommendation
**[APPROVED / NEEDS CHANGES]**
[Brief explanation]
```

### 6. Plan Alignment Check (if applicable)

If a plan was provided, add this section:

```markdown
### 📋 Plan Alignment Analysis

**Plan**: [path to loaded plan file]

#### Implementation Phase Match
- Current plan phase: [phase from plan]
- Changes align with phase: [YES/NO with explanation]

#### Requirements Coverage
- [List requirements from plan]
- [Mark which are addressed by changes]
- [Flag any out-of-order implementation]

#### API Contract Compliance
- [Check if API changes match plan specifications]
- [Verify request/response types match plan]

#### Test Coverage vs Plan
- [Compare test requirements in plan vs actual tests added]

#### Gaps or Deviations
- [List any changes that deviate from plan]
- [List planned items not yet implemented]
```

---

## Important Reminders

1. **Be thorough but practical** - Focus on critical issues that affect functionality, security, or maintainability
2. **Provide specific feedback** - Include file names, line numbers, and concrete examples
3. **Distinguish blocking vs. suggestions** - Make it clear what MUST be fixed vs. nice-to-have
4. **Check for common anti-patterns**:
   - Early returns with loading states (should use LoadingOverlay)
   - Native HTML elements instead of UI components
   - Hardcoded Tailwind colors instead of CSS variables
   - Missing tests for new features
   - Database queries without proper error handling
5. **Reference documentation** - Point to relevant ADRs, context files, or skills used

---

## Examples

### Good Usage
```
/review-changes user-profiles
```
Reviews uncommitted changes and compares against `.claude/planning/active/user-profiles/` plan (directory-based).

```
/review-changes feature-polls
```
Reviews uncommitted changes and compares against `.claude/planning/FEATURE_polls.md` plan (single-file).

```
/review-changes
```
Reviews uncommitted changes against general project standards without plan comparison.

### What Gets Caught

**Missing Tests**:
```
⚠️ New feature added without tests
File: backend/pkg/polls/service.go:45
Issue: CreatePoll method added but no corresponding test file found.
Required: Add backend/pkg/polls/service_test.go with table-driven tests.
Reference: .claude/context/TESTING.md - "Tests are MANDATORY for all features"
```

**UI Component Violation**:
```
⚠️ Native HTML element used instead of UI component
File: frontend/src/components/PollForm.tsx:23
Issue: <button> used instead of <Button> component
Fix: import { Button } from './ui'; and replace line 23
Reference: frontend/src/components/ui/README.md - UI Component Library usage
```

**Plan Deviation**:
```
⚠️ Implementation order doesn't match plan
Plan: Phase 1 requires backend API first, then frontend
Current: Frontend component added but backend API not yet implemented
Recommendation: Implement backend/pkg/polls/api.go before proceeding
Reference: .claude/planning/active/user-profiles/plan.md - Phase 1 requirements
```
