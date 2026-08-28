# Agents

Specialized subagent definitions for complex, multi-step tasks.

## What Are Agents?

Agents are autonomous Claude instances launched via the Task tool. Unlike skills
(which load guidance inline into the current conversation), an agent runs in its
own context window and reports back a result.

Each agent is a single markdown file with YAML frontmatter (`name`,
`description`, optional `model` / `color`) followed by its system prompt.

## Available Agents (3)

| Agent | Purpose | Model |
|---|---|---|
| `plan-reviewer` | Review a development plan before implementation — finds missing considerations, risks, and better alternatives | opus |
| `refactor-planner` | Analyze structure and produce a step-by-step refactoring plan with risk assessment | inherits |
| `web-research-specialist` | Research technical problems across GitHub issues, Stack Overflow, forums | sonnet |

All three are project-agnostic: they contain no ActionPhase-specific paths or
assumptions, so they need no customization.

> Claude Code also provides built-in agents (`Explore`, `Plan`,
> `general-purpose`, and others). Those are not defined in this directory.

## Agents vs Skills

| Use an agent when… | Use a skill when… |
|---|---|
| The task is open-ended and multi-step | You need patterns/conventions while writing code |
| It would consume a lot of context (broad search, deep research) | The guidance is short and inline |
| You want an independent second opinion | You are following an established workflow |

Skills live in `.claude/skills/` and activate via `skill-rules.json`.

## Creating an Agent

Add a `.md` file to this directory:

```markdown
---
name: my-agent
description: When this agent should be used. Include concrete examples — this text is what Claude matches against.
model: opus        # optional; omit to inherit
---

You are a [role]. Your responsibilities are...
```

The `description` is the routing signal, so make it specific about *when* to
use the agent, not just what it does.

## Troubleshooting

**Agent not found** — confirm the file exists and its frontmatter `name` matches
what you are asking for:

```bash
ls -la .claude/agents/
grep -h "^name:" .claude/agents/*.md
```

**Agent assumes the wrong environment** — agents here should stay
project-agnostic. Project specifics belong in `.claude/context/` or a skill, so
that they are maintained in one place. Note in particular that this project
authenticates with a **`Authorization: Bearer <token>`** header (the HTTP-only
cookie carries only the refresh token) — see `frontend/src/lib/api/client.ts`.
