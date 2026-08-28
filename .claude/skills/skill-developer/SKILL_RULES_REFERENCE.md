# skill-rules.json - Complete Reference

Complete schema and configuration reference for `.claude/skills/skill-rules.json`.

## Table of Contents

- [File Location](#file-location)
- [Complete TypeScript Schema](#complete-typescript-schema)
- [Field Guide](#field-guide)
- [Example: Domain Skill](#example-domain-skill)
- [Validation](#validation)

---

## File Location

**Path:** `.claude/skills/skill-rules.json`

This JSON file defines all skills and their trigger conditions for the auto-activation system.

---

## Complete TypeScript Schema

```typescript
interface SkillRules {
    version: string;
    skills: Record<string, SkillRule>;
}

interface SkillRule {
    type: 'guardrail' | 'domain';
    enforcement: 'block' | 'suggest' | 'warn';
    priority: 'critical' | 'high' | 'medium' | 'low';

    promptTriggers?: {
        keywords?: string[];
        intentPatterns?: string[];  // Regex strings
    };

    fileTriggers?: {
        pathPatterns: string[];     // Glob patterns
        pathExclusions?: string[];  // Glob patterns
        contentPatterns?: string[]; // Regex strings
        createOnly?: boolean;       // Only trigger on file creation
    };

    blockMessage?: string;  // Unused — nothing blocks in this project
}
```

---

## Field Guide

### Top Level

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `version` | string | Yes | Schema version (currently "1.0") |
| `skills` | object | Yes | Map of skill name → SkillRule |

### SkillRule Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Yes | "domain" (advisory). All 7 skills here use "domain"; "guardrail" exists in the schema but nothing enforces it, since there is no PreToolUse hook. |
| `enforcement` | string | Yes | "suggest", "warn", or "block". **Advisory only** — the activation hook prints suggestions and never blocks, so all three behave identically today. |
| `priority` | string | Yes | "critical", "high", "medium", or "low" |
| `promptTriggers` | object | Optional | Triggers for UserPromptSubmit hook |
| `fileTriggers` | object | Optional | **Inert** — no hook reads this. There is no PreToolUse hook; only `promptTriggers` affects activation. |
| `blockMessage` | string | Optional* | Required if enforcement="block". Use `{file_path}` placeholder |

*Required for guardrails

### promptTriggers Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `keywords` | string[] | Optional | Exact substring matches (case-insensitive) |
| `intentPatterns` | string[] | Optional | Regex patterns for intent detection |

### fileTriggers Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `pathPatterns` | string[] | Yes* | Glob patterns for file paths |
| `pathExclusions` | string[] | Optional | Glob patterns to exclude (e.g., test files) |
| `contentPatterns` | string[] | Optional | Regex patterns to match file content |
| `createOnly` | boolean | Optional | Only trigger when creating new files |

*Required if fileTriggers is present

## Example: Domain Skill

Complete example of a suggestion-based domain skill:

```json
{
  "project-catalog-developer": {
    "type": "domain",
    "enforcement": "suggest",
    "priority": "high",

    "promptTriggers": {
      "keywords": [
        "layout",
        "layout system",
        "grid",
        "grid layout",
        "toolbar",
        "column",
        "cell editor",
        "cell renderer",
        "submission",
        "submissions",
        "blog dashboard",
        "datagrid",
        "data grid",
        "CustomToolbar",
        "GridLayoutDialog",
        "useGridLayout",
        "auto-save",
        "column order",
        "column width",
        "filter",
        "sort"
      ],
      "intentPatterns": [
        "(how does|how do|explain|what is|describe).*?(layout|grid|toolbar|column|submission|catalog)",
        "(add|create|modify|change).*?(toolbar|column|cell|editor|renderer)",
        "blog dashboard.*?"
      ]
    },

    "fileTriggers": {
      "pathPatterns": [
        "frontend/src/features/submissions/**/*.tsx",
        "frontend/src/features/submissions/**/*.ts"
      ],
      "pathExclusions": [
        "**/*.test.tsx",
        "**/*.test.ts"
      ]
    }
  }
}
```

### Key Points for Domain Skills

1. **type**: Must be "domain"
2. **enforcement**: Usually "suggest"
3. **priority**: "high" or "medium"
4. **blockMessage**: Not needed — nothing blocks
5. **promptTriggers**: The only triggers that work; invest your effort here
6. **fileTriggers**: Inert (no hook reads it), so it cannot activate a skill

---

## Validation

### Check JSON Syntax

```bash
cat .claude/skills/skill-rules.json | jq .
```

If valid, jq will pretty-print the JSON. If invalid, it will show the error.

### Common JSON Errors

**Trailing comma:**
```json
{
  "keywords": ["one", "two",]  // ❌ Trailing comma
}
```

**Missing quotes:**
```json
{
  type: "guardrail"  // ❌ Missing quotes on key
}
```

**Single quotes (invalid JSON):**
```json
{
  'type': 'guardrail'  // ❌ Must use double quotes
}
```

### Validation Checklist

- [ ] JSON syntax valid (use `jq`)
- [ ] All skill names match SKILL.md filenames
- [ ] Guardrails have `blockMessage`
- [ ] Block messages use `{file_path}` placeholder
- [ ] Intent patterns are valid regex (test on regex101.com)
- [ ] File path patterns use correct glob syntax
- [ ] Content patterns escape special characters
- [ ] Priority matches enforcement level
- [ ] No duplicate skill names

---

**Related Files:**
- [SKILL.md](SKILL.md) - Main skill guide
- [TRIGGER_TYPES.md](TRIGGER_TYPES.md) - Complete trigger documentation
- [TROUBLESHOOTING.md](TROUBLESHOOTING.md) - Debugging configuration issues
