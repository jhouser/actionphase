# Architecture Decision Records (ADRs)

This directory contains Architecture Decision Records for ActionPhase. Each ADR captures an important architectural decision, its context, the alternatives considered, and the rationale for the decision.

## ADR Index

| ADR | Title | Status | Date |
|-----|-------|--------|------|
| [001](001-technology-stack-selection.md) | Technology Stack Selection | Accepted | 2025-08-07 |
| [002](002-database-design-approach.md) | Database Design Approach | Accepted ⚠️ | 2025-08-07 |
| [003](003-authentication-strategy.md) | Authentication Strategy | Accepted ⚠️ | 2025-08-07 |
| [004](004-api-design-principles.md) | API Design Principles | Accepted | 2025-08-07 |
| [005](005-frontend-state-management.md) | Frontend State Management | Accepted | 2025-08-07 |
| [006](006-observability-approach.md) | Observability Approach | **Superseded** | 2025-08-07 (superseded 2026-06-04) |
| [007](007-testing-strategy.md) | Testing Strategy | Accepted | 2025-08-07 |

⚠️ = the decision still holds, but specifics in the ADR diverge from the shipped
code. Each such ADR carries an **Implementation Divergence** section recording
exactly what differs (audited 2026-08-26). Read that section before relying on
any concrete detail — column names, token lifetimes, response shapes.

## ADR Template

When creating new ADRs, use this template:

```markdown
# ADR-XXX: [Title]

## Status
[Proposed | Accepted | Rejected | Deprecated | Superseded]

## Context
[Describe the issue or decision that needs to be made]

## Decision
[State the decision that was made]

## Alternatives Considered
[List the alternative options that were evaluated]

## Consequences
[Describe the positive and negative consequences of this decision]

## References
[Links to additional resources or related ADRs]
```

## ADR Process

1. **Proposal**: Create ADR with "Proposed" status
2. **Discussion**: Review with team, gather feedback
3. **Decision**: Update status to "Accepted" or "Rejected"
4. **Implementation**: Follow through on accepted decisions
5. **Review**: Periodically review for relevance and updates

## Keeping ADRs honest

ADRs record decisions *as made at the time*, so a stale ADR is not automatically
wrong. But readers reasonably treat concrete details — schema, endpoints, token
lifetimes — as current.

When reality diverges:

- **Do not rewrite history.** Keep the original decision text.
- Add an **Implementation Divergence** (or **Recent Architectural Evolution**)
  section stating what actually ships, with file/line citations.
- If the *decision itself* was reversed, set Status to **Superseded** and say
  what replaced it — as ADR-006 does.
- If a divergence turns out to be a deliberate new decision, it deserves its own
  ADR superseding the old one.
