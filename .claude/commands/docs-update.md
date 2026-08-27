---
description: Find documentation the current branch has made wrong, by diffing against develop
argument-hint: Optional - area to focus on (e.g. "phases", "e2e"). Leave empty to check the whole diff.
---

# Docs Update

Finds documentation that the current branch's changes have made wrong, before
the branch merges.

## Why this exists

A 2026-08 audit found ~110 dev docs with real drift. Nearly all of it was
introduced one branch at a time by changes whose author had no signal that a
doc had gone stale. This command is that signal. It runs against a branch diff,
not the whole tree, so it stays cheap enough to run every time.

## Establishing the baseline

```bash
git fetch origin develop --quiet
BASE=$(git merge-base HEAD origin/develop)
git diff --stat "$BASE" HEAD
```

Use **`origin/develop`**, not local `develop` — the local branch is routinely
stale, and it silently produces a different merge-base that hides or invents
changes. If `origin/develop` is unreachable, say so and fall back to local
`develop`, stating which baseline you used.

If the diff is empty, say so and stop.

## Pass 1 — Broken references (mechanical)

Extract identifiers that were **removed or renamed** in the diff, then grep the
docs for them. Every hit is a concrete defect.

```bash
# Removed/renamed justfile recipes
git diff "$BASE" HEAD -- justfile | grep -E '^-[a-z][a-z0-9_-]*:'

# Deleted or renamed files
git diff --diff-filter=DR --name-status "$BASE" HEAD

# Removed exported Go symbols
git diff "$BASE" HEAD -- 'backend/**/*.go' | grep -E '^-func [A-Z]|^-type [A-Z]'

# Removed frontend exports
git diff "$BASE" HEAD -- 'frontend/src/**/*.ts*' | grep -E '^-export (function|const|type|interface)'
```

For each removed name, search the docs — including `.claude/`, `docs/`,
`docs-site/`, and every `README.md`:

```bash
git grep -n "<name>" -- '*.md' ':!docs-site/.vitepress/dist'
```

Then verify each hit still resolves, and check relative links in changed docs
still point at real files. **Report the ones that don't; a name appearing in a
doc is not automatically wrong** — it may be prose about history, or an
unrelated identifier that shares the name. Open the hit and read it.

## Pass 2 — Semantic drift (requires reading)

Map changed code areas to the docs that describe them. Read both; ask whether
the prose is still true.

**The mapping table lives in [`docs-update-map.md`](./docs-update-map.md).**
Read it now and apply the rows matching this diff's changed paths.

```bash
# Changed non-doc areas, most-touched first
git diff --name-only "$BASE" HEAD -- ':!*.md' | sed 's|/[^/]*$||' | sort | uniq -c | sort -rn
```

The map is a starting point, not a whitelist. If the diff touches something it
does not cover, find the affected docs yourself, check them, and **append a row
to the map** — that is how this command gets better over time. Adding a row is
part of a normal run, not a separate task.

## Pass 3 — Derived-content guard

Reject new documentation that is stale by construction. Flag any **added** line
that hardcodes:

- test counts, coverage percentages, file counts
- "Last updated" / "Status: Complete" tables
- "Recent changes" or campaign/progress narrative
- roadmap items phrased as done

```bash
git diff "$BASE" HEAD -- '*.md' | grep -E '^\+' | \
  grep -inE '[0-9]+ (tests|files)|[0-9]+(\.[0-9]+)?% cover|last (updated|measured)|status: (complete|done)'
```

These decay the moment anyone commits. Prefer the command that regenerates the
number over the number. If a doc's *whole purpose* is derived content, it should
not exist — `git log` and the code are the source of truth.

**A measurement date does not rescue a number.** "Last measured: 2026-08-26"
still reads as current to someone who greps it in 2027, and dating it does not
make it true — it just makes the lie auditable after the fact. There is no
tier of acceptable hardcoded metrics.

**Coverage and test counts are tracked live by Codecov.** Never restate them in
markdown. Link to Codecov, or name the command (`just test-coverage`,
`just test-fe run`) and let the reader run it. A number in a doc competes with
the live source and always loses.

## Verification — run it, don't read it

**Every command a doc mentions must be executed, not eyeballed.** The audit's
most common defect was commands that read fine and don't exist. Reading cannot
catch a recipe that was renamed, a flag that takes no argument, or a column that
no longer exists.

```bash
just --list | grep <recipe>          # recipe exists
just <recipe> --help                 # signature is right
```

Same for SQL columns, API paths, and file paths in examples.

## Output

Report as a table — file, what's wrong, evidence:

| Doc | Issue | Evidence |
|---|---|---|
| `CLAUDE.md:412` | `just test-frontend` doesn't exist | `just --list` → `test-fe` |

Then **fix them**, unless the fix is a judgment call about intent — surface
those and ask.

If a doc is wrong because it is a point-in-time record, **propose deleting it
rather than annotating it**. A banner at the top does not protect a reader who
lands on a grep match mid-document.

State plainly what you verified by execution vs. what you only read. If you
could not verify something, say so — do not present it as checked.

## Scope

Only what the diff touches. This is not a full audit; a clean run means *this
branch* introduced no drift, not that the docs are correct.

Focus area, if given: $ARGUMENTS
