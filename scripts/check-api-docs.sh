#!/bin/bash

# API Documentation Consistency Check
#
# The OpenAPI spec is generated from the Go types of every registered huma
# operation (see .claude/planning/huma-migration.md). The committed copy at
# backend/pkg/docs/openapi.gen.yaml is what reviewers read and what any client
# generator consumes, so it must not fall behind the code.
#
# This regenerates the document and diffs it against the committed file. A
# failure means the API changed without `just gen-openapi` being run — the fix
# is to run it and commit the result.
#
# It replaces an older check that parsed root.go for route literals and compared
# them against a hand-written openapi.yaml, tracking known gaps in a baseline
# file. That was necessary while the two descriptions were maintained
# independently and could drift in three different directions (undocumented
# routes, phantom paths, unreachable URLs). With generation none of those states
# is reachable: a route that exists is documented by construction, and a
# documented path cannot exist without a handler behind it.
#
# Usage: just check-api-docs
#    or: ./scripts/check-api-docs.sh   (directly, on a host with bash and Go)

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# In the container the repo is bind-mounted read-only at /repo while the build
# tree lives at /app; on a host both are the same directory.
BACKEND="$ROOT/backend"
if [ -d /app/cmd/genopenapi ] && [ ! -w "$BACKEND" ]; then
    BACKEND=/app
fi

COMMITTED="$BACKEND/pkg/docs/openapi.gen.yaml"

if [ ! -f "$COMMITTED" ]; then
    echo -e "${RED}✗ missing $COMMITTED — run 'just gen-openapi'${NC}"
    exit 1
fi

FRESH="$(mktemp)"
trap 'rm -f "$FRESH"' EXIT

if ! (cd "$BACKEND" && go run ./cmd/genopenapi -o "$FRESH"); then
    echo -e "${RED}✗ failed to generate the OpenAPI spec${NC}"
    exit 1
fi

if diff -u "$COMMITTED" "$FRESH" > /tmp/openapi-drift.diff 2>&1; then
    # Counted by scanning the paths block. Indentation is the marshaller's
    # (4 spaces), so these patterns move if that ever changes — hence the
    # sanity check below rather than trusting the numbers silently.
    paths=$(awk '/^paths:/{p=1;next} p&&/^[^ ]/{exit} p&&/^    \//{n++} END{print n+0}' "$COMMITTED")
    ops=$(awk '/^paths:/{p=1;next} p&&/^[^ ]/{exit} p&&/^        (get|post|put|delete|patch|head|options):/{n++} END{print n+0}' "$COMMITTED")

    if [ "$paths" -eq 0 ] || [ "$ops" -eq 0 ]; then
        echo -e "${RED}✗ the spec parsed as empty — the counting patterns are stale${NC}"
        exit 1
    fi

    echo -e "${GREEN}✓ OpenAPI spec is up to date${NC}"
    echo "  $paths paths, $ops operations — all generated from Go types"
    exit 0
fi

echo -e "${RED}✗ backend/pkg/docs/openapi.gen.yaml is stale${NC}"
echo
echo "  The registered operations no longer match the committed spec."
echo "  Run 'just gen-openapi' and commit the result."
echo
echo "  Difference (committed → generated), first 60 lines:"
head -60 /tmp/openapi-drift.diff | sed 's/^/    /'

total=$(wc -l < /tmp/openapi-drift.diff | tr -d ' ')
if [ "$total" -gt 60 ]; then
    echo "    ... $((total - 60)) more lines (full diff: /tmp/openapi-drift.diff)"
fi

exit 1
