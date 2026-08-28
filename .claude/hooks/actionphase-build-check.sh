#!/bin/bash
# Stop hook: fast, non-mutating code-quality check at end of session.
#
# Delegates to `just verify-quick` so the checks live in ONE place (the
# justfile) instead of drifting in a parallel shell implementation.
#
# `verify-quick` intentionally does NOT compile — use `just verify` before
# pushing for the full checks + production builds.
#
# It exits 0 quietly when the dev stack is down, since every check runs via
# `docker compose exec`; a stopped stack is not a code problem.

set -uo pipefail

cd "${CLAUDE_PROJECT_DIR:-$(dirname "$0")/../..}" || exit 0

# No justfile (or no just) => nothing to check.
command -v just >/dev/null 2>&1 || exit 0
[[ -f justfile ]] || exit 0

output=$(just verify-quick 2>&1)
code=$?

if [[ $code -ne 0 ]]; then
    {
        echo ""
        echo "⚠️  CODE QUALITY CHECKS FAILED (just verify-quick)"
        echo "=================================================="
        echo "$output"
        echo ""
        echo "💡 Fix the issues above, or run 'just verify' for the full"
        echo "   pre-push gate (adds backend + frontend production builds)."
    } >&2
    exit 2
fi

exit 0
