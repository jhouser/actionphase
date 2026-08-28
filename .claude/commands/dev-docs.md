---
description: Create a comprehensive strategic plan with structured task breakdown
argument-hint: Describe what you need planned (e.g., "refactor authentication system", "implement microservices")
---

You are an elite strategic planning specialist. Create a comprehensive, actionable plan for: $ARGUMENTS

## Instructions

1. **Analyze the request** and determine the scope of planning needed
2. **Examine relevant files** in the codebase to understand current state
3. **Create a structured plan** with:
   - Executive Summary
   - Current State Analysis
   - Proposed Future State
   - Implementation Phases (broken into sections)
   - Detailed Tasks (actionable items with clear acceptance criteria)
   - Risk Assessment and Mitigation Strategies
   - Success Metrics
   - Required Resources and Dependencies
   - Timeline Estimates

4. **Task Breakdown Structure**:
   - Each major section represents a phase or component
   - Number and prioritize tasks within sections
   - Include clear acceptance criteria for each task
   - Specify dependencies between tasks
   - Estimate effort levels (S/M/L/XL)

5. **Write the plan to `.claude/planning/`**:
   - **One flat markdown file** per topic: `.claude/planning/[task-name].md`
     (kebab-case). There are no `active/` or `completed/` subdirectories.
   - Open with `# Title` then a `## Background` section explaining *why* the work
     exists — the existing plans are prose-first, not checklist-first.
   - Cite concrete anchors (`backend/pkg/auth/registration.go`,
     `pkg/core/config.go:244`) so the plan stays checkable as code moves.
   - Convert relative dates to absolute (`2026-07-02`, not "last week").
   - `.claude/planning/` is **gitignored** — these are local working notes, so do
     not link to them from tracked docs.

## Quality Standards
- Plans must be self-contained with all necessary context
- Use clear, actionable language
- Include specific technical details where relevant
- Consider both technical and business perspectives
- Account for potential risks and edge cases

## ActionPhase Context References
- Check `CLAUDE.md` for project overview and development patterns
- Consult `.claude/context/ARCHITECTURE.md` for architecture patterns
- Reference `.claude/context/TESTING.md` for testing requirements
- Read an existing plan for tone and depth — `epilogue-game-state.md` (470 lines)
  for a large feature, `cors-origin-hardening.md` (18 lines) for a focused risk note

## Workflow Integration

**Scale the plan to the work.** The existing plans range from ~18 to ~470 lines;
a short, specific note beats a padded template. Long plans earn their length with
design detail (API shapes, schema changes, phased rollout), not boilerplate
headings.

**Use this command for**: bug fixes needing planning, refactors, exploratory
work, and feature additions large enough to outlive one session.

**When work completes**: delete the file, or leave it as a record of what was
decided. There is no `completed/` directory to move it to.

**Note**: This command is ideal to use AFTER exiting plan mode when you have a clear vision of what needs to be done. It will create the persistent task structure that survives context resets.
