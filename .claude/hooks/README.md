# ActionPhase Claude Code Hooks

This directory contains hooks that enhance Claude Code's capabilities for the ActionPhase project.

## Installed Hooks

### 1. Skill Activation Prompt (`skill-activation-prompt`)
**Trigger:** UserPromptSubmit
**Purpose:** Auto-suggests relevant skills based on your prompts

When you type a prompt, this hook analyzes it and suggests relevant skills.
The six skills that exist (one directory each under `.claude/skills/`):
- `backend-dev-guidelines` - Go/Chi/PostgreSQL Clean Architecture patterns
- `frontend-dev-guidelines` - React/TypeScript patterns
- `game-domain` - Game lifecycle, phases, characters, messaging
- `testing-patterns` - V&V criteria and test patterns
- `route-tester` - Authenticated API route testing
- `skill-developer` - Meta-skill for authoring skills and trigger rules

The hook works from any directory within the project by automatically finding the project root.

### 2. Post Tool Use Tracker (`post-tool-use-tracker`)
**Trigger:** PostToolUse (Edit|MultiEdit|Write)
**Purpose:** Tracks file changes for context management and build command caching

When you edit files, this hook:
- Tracks which parts of the codebase were modified (frontend/backend)
- Stores appropriate build commands for later use
- Creates a cache in `.claude/tsc-cache/{session_id}/`

### 3. ActionPhase Build Check (`actionphase-build-check`)
**Trigger:** Stop
**Purpose:** Surfaces code-quality problems before ending the session

The hook is a thin wrapper: it runs **`just verify-quick`** and reports the
output. All check logic lives in the justfile so it cannot drift from what you
run by hand.

`just verify-quick` runs these in **parallel** (~8-11s), and none of them
mutate the tree:

| Check | Catches |
|---|---|
| `tidy-check` | `go.mod` / `go.sum` out of sync (`go mod tidy -diff`) |
| `fmt-check`  | Unformatted Go (`gofmt -l`) |
| `vet`        | Go static-analysis problems |
| `check-game-states` | Game-state list disagreeing across the trees |
| `type-check` | TypeScript errors (`tsc -b`) |
| `lint-frontend` | ESLint errors |

It deliberately does **not** compile. For the full pre-push gate — every check
above plus `tidy`/`fmt` and both production builds — run **`just verify`**.

**When the dev stack is down**, every check would fail on infrastructure rather
than code, so the hook exits 0 silently. Start the stack with `just up` to get
checks back.

> The hook runs everything through the containers, so it keeps working if you
> delete host `node_modules`.

## Directory Structure

```
.claude/hooks/
├── README.md                           # This file
├── package.json                        # Node dependencies
├── skill-activation-prompt.sh          # Shell wrapper for skill activation
├── skill-activation-prompt.ts          # TypeScript skill activation logic
├── post-tool-use-tracker.sh           # File tracking hook (customized for ActionPhase)
├── actionphase-build-check.sh         # Stop hook — wraps `just verify-quick`
├── tsc-check.sh                       # Original TypeScript check (not used)
└── stop-build-check-enhanced.sh       # Original build check (not used)
```

## Skills Configuration

Skills are configured in `.claude/skills/skill-rules.json` with:
- Prompt triggers (keywords and intent patterns)
- File triggers (path patterns and content patterns)
- ActionPhase-specific domains (phases, characters, messaging, etc.)

## Cache Management

The tracker creates a cache directory at `.claude/tsc-cache/{session_id}/` containing:
- `edited-files.log` - Timestamped log of edited files
- `affected-repos.txt` - List of affected components (frontend/backend)
- `commands.txt` - Build commands for affected components

**Note:** since the Stop hook moved to `just verify-quick` (which checks the
whole tree), nothing active reads this cache any more — only the two unused
scripts below do. It is kept because `edited-files.log` is a useful record of
what a session touched, and because scoping `verify-quick` to just the affected
tree is the obvious next optimization.

## Testing Hooks

To test hooks manually:

```bash
# Test skill activation
echo '{"prompt": "I need to add a new API endpoint"}' | \
  npx tsx .claude/hooks/skill-activation-prompt.ts

# Test file tracking (requires CLAUDE_PROJECT_DIR)
export CLAUDE_PROJECT_DIR=/path/to/actionphase
echo '{"tool_name": "Edit", "tool_input": {"file_path": "/path/to/file.tsx"}, "session_id": "test"}' | \
  ./.claude/hooks/post-tool-use-tracker.sh

# Test build check (equivalent to just running `just verify-quick`)
echo '{"session_id": "test"}' | \
  ./.claude/hooks/actionphase-build-check.sh
```

## Troubleshooting

### Hooks not triggering
- Check that hooks are configured in `.claude/settings.local.json`
- Verify scripts have execute permissions: `chmod +x .claude/hooks/*.sh`

### Build check errors
- Ensure the dev stack is running (`just ps`); if it is down the hook exits 0
  silently and checks nothing
- Reproduce by hand with `just verify-quick`
- Formatting/module failures are fixed by their mutating counterparts:
  `just fmt` and `just tidy`

### Skill suggestions not appearing
- Check that `.claude/skills/skill-rules.json` exists
- Verify the skill-activation-prompt hook is properly configured

## Disabling Hooks

To temporarily disable hooks, remove or comment out the `"hooks"` section in `.claude/settings.local.json`.

To permanently remove hooks:
```bash
# Remove hooks configuration from settings
# Then delete the hooks directory
rm -rf .claude/hooks
rm -rf .claude/tsc-cache
```
