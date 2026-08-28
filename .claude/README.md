# Claude AI Context Directory

This directory contains all AI-specific context and instructions for working with the ActionPhase codebase.

## Directory Structure

### `/context/` - Read Before Coding
**These files should be read before performing specific tasks:**

- **TESTING.md** - Read before writing any tests (backend or frontend)
- **ARCHITECTURE.md** - Read for architectural context and patterns
- **STATE_MANAGEMENT.md** - Read before working on frontend state
- **FRONTEND_STYLING.md** - Read before creating/modifying ANY frontend component (dark mode required!)
- **TEST_DATA.md** - Read when working with test data and fixtures

### `/reference/` - Detailed Implementation Guides
**Comprehensive guides for specific implementation topics:**

- **BACKEND_ARCHITECTURE.md** - Detailed backend architecture guide
- **FRONTEND_ERROR_HANDLING.md** - Frontend error handling patterns
- **TESTING_GUIDE.md** - Testing implementation guide
- **LOGGING_STANDARDS.md** - Logging best practices
- **API_DOCUMENTATION.md** - API endpoint documentation
- **API_TESTING_WITH_CURL.md** - Complete curl-based API testing guide
- **JUSTFILE_QUICK_REFERENCE.md** - Quick reference for justfile commands
- And more...

### `/commands/` - Custom Slash Commands
**Detailed protocols for common tasks**

- **debug-e2e-test.md** - Mandatory protocol for debugging E2E test failures using Playwright MCP
- **implement-feature.md** - Structured single-feature implementation session
- **implement-features.md** - Structured approach for implementing multiple features with TodoWrite
- **challenge-assumptions.md** - Protocol for clarifying ambiguous requirements before implementation
- **fix-bug.md** - Bug/UI fix session protocol
- **review-changes.md** - Review uncommitted changes
- **audit-test.md** / **audit-test-init.md** - Test audit protocols
- **dev-docs.md** / **dev-docs-update.md** - Dev documentation planning and updates

### `/skills/` - Model-Invoked Skills
**Progressive-disclosure guides loaded on demand. Activation is driven by `skill-rules.json`.**

- **backend-dev-guidelines** - Go/Chi/PostgreSQL Clean Architecture patterns
- **frontend-dev-guidelines** - React/TypeScript patterns (+ `resources/` deep dives)
- **game-domain** - Game lifecycle, phases, characters, messaging (+ `resources/`)
- **testing-patterns** - V&V criteria and test patterns (+ `resources/`)
- **route-tester** - Authenticated API route testing
- **skill-developer** - Meta-skill for authoring skills and trigger rules

Note: several `resources/` files under `game-domain` and `testing-patterns` are
unwritten stubs. They are placeholders, not authoritative — read the source
instead. See `.claude/DOC_AUDIT_INVENTORY.md`.

### `/agents/` - Subagent Definitions

- **plan-reviewer.md**, **refactor-planner.md**, **web-research-specialist.md**
- See `agents/README.md` for usage guidance.

### `/hooks/` - Hook Scripts
**Shell/TypeScript hooks wired up via `settings.json`** (skill activation, build
checks, tsc checks, tool-use tracking). See `hooks/README.md`.

### `/planning/` - Session Planning & Task Tracking
**Persistent planning documents that survive across sessions:**

Use this directory to:
- Track multi-session implementation plans
- Document feature roadmaps and milestones
- Keep TODO lists for ongoing work
- Store design decisions and exploration notes

This allows for continuity between AI sessions and provides historical context for planning decisions.

**Note**: `/planning/` is gitignored — these are local, untracked working notes.

## External Documentation References

### Architecture Decision Records (ADRs)
**Location**: `/docs-site/developer/architecture/adrs/`

Read ADRs for understanding architectural decisions:
- ADR-001: Technology Stack Selection
- ADR-002: Database Design Approach
- ADR-003: Authentication Strategy
- ADR-004: API Design Principles
- ADR-005: Frontend State Management
- ADR-006: Observability Approach
- ADR-007: Testing Strategy

**Note**: ADRs are served via VitePress at http://localhost:3000/docs/developer/architecture/adrs/

### System Architecture
**Location**: `/docs-site/developer/architecture/`

- overview.md - High-level system design
- components.md - How components communicate

**Note**: Architecture docs are served via VitePress at http://localhost:3000/docs/developer/architecture/

### Testing Documentation
**Location**: `/docs-site/developer/testing/`

- COVERAGE_STATUS.md - Current test coverage status
- TEST_DATA.md - Test fixtures and data setup
- E2E_QUICK_START.md - E2E testing quick reference
- E2E_FIXTURES.md - E2E test fixture documentation

**Note**: Testing docs are served via VitePress at http://localhost:3000/docs/developer/testing/

### Remaining docs/ Directory
**Location**: `/docs/`

Active documentation files (not in docs-site yet):
- Development guides (API docs automation, deployment)
- Operations guides (logging, deployment scripts)
- Feature implementation summaries
- State management details

## Workflow: When to Read What

### Before Writing Tests
1. Read `.claude/context/TESTING.md`
2. Review `/docs-site/developer/testing/COVERAGE_STATUS.md`
3. Reference `/docs-site/developer/architecture/adrs/007-testing-strategy.md`
4. Check `.claude/reference/TESTING_GUIDE.md` for implementation details

### Before Implementing Features
1. Read `.claude/context/ARCHITECTURE.md`
2. Review relevant ADRs in `/docs-site/developer/architecture/adrs/`
3. Check `/docs-site/developer/architecture/` for system design context

### Before Frontend State Work
1. Read `.claude/context/STATE_MANAGEMENT.md`
2. Review `/docs-site/developer/architecture/adrs/005-frontend-state-management.md`
3. Reference `/docs/features/STATE_MANAGEMENT.md`

### Before Working with Test Data
1. Read `.claude/context/TEST_DATA.md`
2. Review `/docs-site/developer/testing/TEST_DATA.md` for detailed fixture information
3. Check `/backend/pkg/db/test_fixtures/` for actual fixtures

### Before API Changes
1. Review `/docs-site/developer/architecture/adrs/004-api-design-principles.md`
2. Check `.claude/reference/API_DOCUMENTATION.md`
3. Review `.claude/reference/ERROR_HANDLING.md`

### Before Writing E2E Tests (CRITICAL)
**⚠️ E2E tests are the LAST step, NEVER the first!**

**Mandatory Pre-E2E Checklist** (dev stack is containerized — use `just`, not host tools):
1. ✅ Backend unit test passes: `just test-mocks`
2. ✅ API returns correct data: `curl http://localhost:3000/api/v1/... | jq`
3. ✅ Component test passes: `just test-fe run <file>`
4. ✅ Systems running: `just ps` (or `curl http://localhost:3000/health`)

**E2E Test Rules:**
- Run via `just e2e-desktop` / `just e2e-mobile` / `just e2e` (runs in the playwright container)
- Run synchronously — never background with `&`
- One concern per test
- Use `data-testid` selectors
- Wait for specific conditions, not arbitrary timeouts

**See**: `.claude/context/TESTING.md` E2E section and `frontend/e2e/README.md` for the complete guide

## Quick Start for AI

When starting a coding task:
1. Check CLAUDE.md in project root for general instructions
2. Identify the task type (testing, feature, frontend, etc.)
3. Read the relevant context files from `.claude/context/`
4. Reference detailed guides in `.claude/reference/` as needed
5. Check relevant ADRs for architectural decisions

## Maintenance

- Prefer concise context files; split into `.claude/reference/` when they grow past ~500 lines
  (several currently exceed this — see `.claude/DOC_AUDIT_INVENTORY.md`)
- Update this README when adding new context files, skills, commands, or agents
- Move detailed implementation guides to `.claude/reference/`
- Keep ADRs in `/docs-site/developer/architecture/adrs/` (single source of truth)
- Update CLAUDE.md to reference new context files
