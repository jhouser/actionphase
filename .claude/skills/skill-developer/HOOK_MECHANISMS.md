# Hook Mechanisms - Deep Dive

Technical deep dive into how this project's skill-activation hook works.

> **Scope:** ActionPhase registers three hooks in `.claude/settings.local.json` —
> `UserPromptSubmit` (skill activation), `PostToolUse` (file tracking), and
> `Stop` (`just verify-quick`). Only `UserPromptSubmit` is part of the skill
> system and is documented here; see `.claude/hooks/README.md` for the other two.
>
> There is **no `PreToolUse` hook and no session-state tracking** in this
> project. Skill activation is advisory (`enforcement: "suggest"`), not
> enforced — nothing blocks a tool call.

## Table of Contents

- [UserPromptSubmit Hook Flow](#userpromptsubmit-hook-flow)
- [Exit Code Behavior](#exit-code-behavior)
- [Performance Considerations](#performance-considerations)

---

## UserPromptSubmit Hook Flow

### Execution Sequence

```
User submits prompt
    ↓
.claude/settings.local.json registers hook
    ↓
skill-activation-prompt.sh executes
    ↓
npx tsx skill-activation-prompt.ts
    ↓
Hook reads stdin (JSON with prompt)
    ↓
Loads skill-rules.json
    ↓
Matches keywords + intent patterns
    ↓
Groups matches by priority (critical → high → medium → low)
    ↓
Outputs formatted message to stdout
    ↓
stdout becomes context for Claude (injected before prompt)
    ↓
Claude sees: [skill suggestion] + user's prompt
```

### Key Points

- **Exit code**: Always 0 (allow)
- **stdout**: → Claude's context (injected as system message)
- **Timing**: Runs BEFORE Claude processes prompt
- **Behavior**: Non-blocking, advisory only
- **Purpose**: Make Claude aware of relevant skills

### Input Format

```json
{
  "session_id": "abc-123",
  "transcript_path": "/path/to/transcript.json",
  "cwd": "/root/git/your-project",
  "permission_mode": "normal",
  "hook_event_name": "UserPromptSubmit",
  "prompt": "how does the layout system work?"
}
```

### Output Format (to stdout)

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🎯 SKILL ACTIVATION CHECK
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📚 RECOMMENDED SKILLS:
  → project-catalog-developer

ACTION: Use Skill tool BEFORE responding
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

Claude sees this output as additional context before processing the user's prompt.

---

## Exit Code Behavior

### Exit Code Reference Table

| Hook | Exit 0 | Exit 2 |
|---|---|---|
| `UserPromptSubmit` | stdout is injected as context ahead of the prompt | stderr goes to Claude; the prompt is blocked |
| `Stop` | Session ends normally | stderr goes to Claude, which continues the turn |
| `PostToolUse` | Silent | stderr goes to Claude |

The skill-activation hook always exits 0 and writes its suggestion to **stdout**
— that is what makes the "🎯 SKILL ACTIVATION CHECK" block appear ahead of the
prompt.

The Stop hook (`actionphase-build-check.sh`) is the one place this project uses
**exit 2** deliberately: when `just verify-quick` fails, the failure text on
stderr is fed back so the problems can be fixed before the session ends.

### Example Flow

```
User: "add a poll results component"
    ↓
UserPromptSubmit hook [exit 0]
    stdout: "🎯 RECOMMENDED SKILLS: → frontend-dev-guidelines"
    ↓
Claude sees the suggestion prepended to the prompt, loads the skill,
and writes the component.
```

---

## Performance Considerations

### Target Metric

- **UserPromptSubmit**: < 100ms. It runs on *every* prompt, so cost is felt
  constantly. Note the wrapper shells out to `npx tsx`, so process startup
  dominates the actual matching work.

### Performance Bottlenecks

1. **`npx tsx` startup** — the largest fixed cost, paid per prompt
2. **Loading `skill-rules.json`** — re-read on every execution
3. **Regex matching** — every `intentPatterns` entry is compiled per run

### Optimization Strategies

**Reduce patterns:**
- Use specific keywords; every extra pattern is compiled on every prompt
- Combine similar intent patterns where possible

**Keep triggers honest:**
- A trigger that never matches is dead weight, and one that matches everything
  trains you to ignore the suggestion. Verify new file globs actually match:

```bash
ls frontend/src/components/**/*Poll*.tsx
```

---

**Related Files:**
- [SKILL.md](SKILL.md) - Main skill guide
- [TROUBLESHOOTING.md](TROUBLESHOOTING.md) - Debug hook issues
- [SKILL_RULES_REFERENCE.md](SKILL_RULES_REFERENCE.md) - Configuration reference
