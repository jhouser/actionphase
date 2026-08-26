---
description: Update dev documentation before context compaction
argument-hint: Optional - specific context or tasks to focus on (leave empty for comprehensive update)
---

We're approaching context limits. Please update the development documentation to ensure seamless continuation after context reset.

## Required Updates

### 1. Update the Relevant Planning Doc
Plans live as **flat markdown files** in `.claude/planning/` (one per topic,
kebab-case — there is no `active/` subdirectory or per-task file split).

For each plan touched this session, update it in place with:
- Current implementation state
- Key decisions made this session, and *why*
- Files modified, with concrete anchors (`pkg/core/config.go:244`)
- Any blockers or issues discovered
- Next immediate steps
- Absolute dates (`2026-08-26`), never "today" or "last week"

If the session's work has no plan file yet and is worth resuming, create one.

### 2. Capture Session Context
Include any relevant information about:
- Complex problems solved
- Architectural decisions made
- Tricky bugs found and fixed
- Integration points discovered
- Testing approaches used
- Performance optimizations made

### 3. Update ActionPhase Documentation
- Update `.claude/context/` files if patterns changed
- If a plan's work is fully finished, delete the file or mark it done in place —
  there is no `completed/` directory
- Update `CLAUDE.md` if workflow improvements discovered
- Document new patterns in appropriate context files

### 4. Document Unfinished Work
- What was being worked on when context limit approached
- Exact state of any partially completed features
- Commands that need to be run on restart (`just` commands)
- Any temporary workarounds that need permanent fixes

### 5. Create Handoff Notes
If switching to a new conversation:
- Exact file and line being edited
- The goal of current changes
- Any uncommitted changes that need attention
- Test commands to verify work:
  - Backend: `just test`, `just test-mocks`
  - Frontend: `just test-fe run`
  - E2E: `just e2e`

## Additional Context: $ARGUMENTS

**Priority**: Focus on capturing information that would be hard to rediscover or reconstruct from code alone.

## ActionPhase-Specific Considerations

**Check for updates to**:
- Game domain logic (states, phases, character workflows)
- Test fixtures (especially E2E worker-specific data)
- Backend service interfaces
- Frontend UI component patterns
- Authentication/authorization logic
