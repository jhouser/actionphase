# E2E Testing Learnings - Codified for Future Sessions

> ⚠️ **HISTORICAL ARTIFACT (October 2025).** Its two "see also" links point at
> `.claude/planning/` files that no longer exist (that directory is gitignored).
> Current E2E guidance lives in `frontend/e2e/README.md`, `.claude/context/TESTING.md`,
> and the `testing-patterns` skill. **Merge anything still valuable into those, then delete.**

**Date**: 2025-10-18
**Context**: Phase 7 of FEATURE_COMMENT_EDITOR_IMPROVEMENTS.md revealed significant challenges with E2E testing in an AI-driven development context.

## What Was Learned

During Phase 7 (Character Mentions E2E Testing), I struggled significantly with E2E tests because:

1. **Jumped to E2E too early** - Wrote E2E tests before verifying lower-level tests
2. **Async execution blindness** - Ran tests in background with `&`, couldn't see what was happening
3. **No incremental verification** - Tried to test everything at once
4. **Missing debugging tools** - No console logs, network inspection, or state visibility
5. **Generic selectors** - Used class names (`.mention`) that didn't match actual HTML (`<mark>`)

## What Was Codified

To ensure these lessons persist across sessions, I've updated:

### 1. CLAUDE.md (Primary Reference)
**Location**: `/CLAUDE.md` lines 24-32, 137-164

**Added**:
- ⚠️ Warning section "BEFORE WRITING E2E TESTS - CRITICAL"
- Test Pyramid visualization showing E2E is LAST
- 5-step checklist before E2E
- E2E test rules (synchronous, data-testid, no timeouts)

**Why**: CLAUDE.md is read at the start of every session. Making E2E requirements prominent here ensures they're impossible to miss.

### 2. .claude/context/TESTING.md (Detailed Testing Guidance)
**Location**: `.claude/context/TESTING.md` lines 96-211

**Added**:
- **Entire E2E section** with "⚠️ CRITICAL: READ BEFORE WRITING E2E TESTS"
- "THE GOLDEN RULE: E2E tests are the LAST step, not the first"
- Mandatory Pre-E2E Test Checklist with code examples
- E2E Test Structure Rules (5 rules with ❌ BAD / ✅ GOOD examples)
- E2E Development Workflow (incremental steps)
- Common E2E Failures & Solutions table
- Reference to comprehensive strategy document

**Why**: This is the file I'm instructed to read "before writing tests". The E2E section is now impossible to skip.

### 3. .claude/README.md (Workflow Guide)
**Location**: `.claude/README.md` lines 105-120

**Added**:
- "Before Writing E2E Tests (CRITICAL)" section
- Mandatory Pre-E2E Checklist with specific commands
- E2E Test Rules (bullet points)
- References to detailed docs

**Why**: This file guides workflow decisions. Including E2E requirements here ensures they're part of the standard workflow.

### 4. .claude/planning/AI_E2E_TESTING_STRATEGY.md (Comprehensive Guide)
**Location**: `.claude/planning/AI_E2E_TESTING_STRATEGY.md` (new file, 600+ lines)

**Contains**:
- Current pain points (observed during Phase 7)
- Strategic solutions (Phase 1-7)
- Pre-E2E verification (CRITICAL)
- E2E test structure patterns
- AI-optimized test execution
- Debugging workflows
- Test data management
- Progressive E2E development
- AI workflow integration
- Implementation checklist
- Success metrics
- Example of well-structured E2E test

**Why**: Comprehensive reference for complex E2E scenarios. Planning docs persist across sessions and provide deep context.

## How This Helps Future Sessions

### Before Writing E2E Tests
Future Claude sessions will:
1. **Read CLAUDE.md** → See prominent warning about E2E being LAST
2. **Read TESTING.md** → See mandatory checklist and rules
3. **Follow checklist**:
   - Run backend unit test
   - Verify API with curl
   - Run component test
   - Verify systems running
4. **Only then** write E2E test

### While Writing E2E Tests
Future sessions will:
- Run tests synchronously (no `&`)
- Use `data-testid` selectors
- Wait for specific conditions
- Add console/network logging
- Test one concern per test
- Build incrementally (navigation → interaction → behavior)

### When E2E Tests Fail
Future sessions will:
1. Check screenshot (included in output)
2. Review console logs (captured in test)
3. Verify API response (logged in test)
4. Inspect component state (via browser evaluation)
5. Work down the test pyramid (E2E → component → API → unit)

## Verification

To verify this codification works, a future session should:

1. **Start new feature requiring E2E test**
2. **Check**: Does session read CLAUDE.md and see E2E warning?
3. **Check**: Does session read TESTING.md E2E section?
4. **Check**: Does session run pre-E2E checklist?
5. **Check**: Does session write tests synchronously?
6. **Check**: Does session use data-testid selectors?

**Success criteria**: E2E test passes on first run >80% of the time.

## Key Takeaways for Future AI Sessions

```
E2E Test Pyramid:

4. E2E (Playwright)     ← 20-30 sec, limited visibility, LAST
   ↑
3. Component (React)    ← 1-2 sec, full visibility
   ↑
2. API (curl)           ← <1 sec, full visibility
   ↑
1. Unit (Go/TS)         ← <1 sec, full visibility, FIRST
```

**The Problem**: AI development needs tight feedback loops. E2E tests have long feedback loops.

**The Solution**: Only write E2E tests after lower levels pass. This gives AI tight feedback loops where it matters most.

**The Rule**: Test pyramid is bottom-up, ALWAYS.

## Files Modified

| File | Lines | Purpose |
|------|-------|---------|
| `/CLAUDE.md` | 24-32, 137-164 | Primary project instructions |
| `.claude/context/TESTING.md` | 96-211 | Testing context and patterns |
| `.claude/README.md` | 105-120 | AI workflow guide |
| `.claude/planning/AI_E2E_TESTING_STRATEGY.md` | ALL (new) | Comprehensive E2E strategy |
| `.claude/planning/E2E_TESTING_LEARNINGS_CODIFIED.md` | ALL (new) | This summary document |

## Next Steps

1. ✅ Learnings codified in persistent documentation
2. ⏭️ Return to Phase 7 E2E failures
3. ⏭️ Apply new strategy:
   - Verify API returns `mentioned_character_ids`
   - Verify component receives and processes data
   - Add console logging to test
   - Fix rendering issue
   - Run E2E test synchronously

---

**Meta-Learning**: When AI struggles repeatedly with a task type, the solution isn't just to fix the immediate problem - it's to codify the learnings in documentation that persists across sessions. This ensures the same mistakes aren't repeated.
