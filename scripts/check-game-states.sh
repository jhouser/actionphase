#!/bin/bash

# Game State Consistency Check
#
# Four places independently define the set of valid game states, in three
# languages, with nothing connecting them at compile time:
#
#   1. core.ValidGameStates            backend/pkg/core/constants.go
#   2. allowedTransitions              backend/pkg/db/services/games.go
#   3. the games.state CHECK constraint backend/pkg/db/migrations/*.sql
#   4. the GameState union             frontend/src/types/games.ts
#
# A state added to some but not all of them fails in ways that only surface in
# a running app: the UI offers a transition the API rejects, or the API accepts
# a state the UI cannot render, or Postgres rejects the write outright.
#
# This runs on the host (via `just lint`) rather than as a unit test because
# neither the backend nor the frontend container can see the other's tree —
# each is built from its own directory as its build context.
#
# Usage: ./scripts/check-game-states.sh

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

CONSTANTS="$ROOT/backend/pkg/core/constants.go"
TRANSITIONS="$ROOT/backend/pkg/db/services/games.go"
TYPES="$ROOT/frontend/src/types/games.ts"

for f in "$CONSTANTS" "$TRANSITIONS" "$TYPES"; do
    if [ ! -f "$f" ]; then
        echo -e "${RED}✗ missing file: $f${NC}"
        exit 1
    fi
done

# --- 1. Backend: the ValidGameStates slice body -----------------------------
# Take the lines between the var declaration and its closing brace, then pull
# the string literal off each GameState* constant's own declaration.
backend_states=$(
    awk '/^var ValidGameStates = \[\]string\{/{flag=1; next} /^\}/{flag=0} flag' "$CONSTANTS" |
    grep -o 'GameState[A-Za-z]*' |
    while read -r ident; do
        sed -n "s/^	$ident = \"\([a-z_]*\)\".*/\1/p" "$CONSTANTS" || true
    done | sort -u
)

# --- 2. Frontend: the GameState union members -------------------------------
frontend_states=$(
    awk '/^export type GameState =/{flag=1} flag{print; if (/;/) exit}' "$TYPES" |
    sed -n "s/.*'\([a-z_]*\)'.*/\1/p" | sort -u
)

# --- 3. Backend: the allowedTransitions map keys ----------------------------
transition_states=$(
    awk '/^var allowedTransitions = map\[string\]\[\]string\{/{flag=1; next} /^\}/{flag=0} flag' "$TRANSITIONS" |
    sed -n 's/^[[:space:]]*"\([a-z_]*\)".*/\1/p' | sort -u
)

# --- 4. Migrations: the newest games.state CHECK constraint -----------------
# Later migrations may drop and recreate the constraint, so take the last
# definition in filename (chronological) order rather than the first.
#
# The state list routinely wraps across lines, so match from "CHECK (state IN"
# through the closing paren rather than grepping single lines — a line-at-a-time
# read silently sees only the first few states and reports a false mismatch.
constraint_block=""
for f in $(ls "$ROOT/backend/pkg/db/migrations/"*.up.sql 2>/dev/null | sort); do
    block=$(
        tr '\n' ' ' < "$f" |
        sed -n 's/.*CHECK (state IN \(([^)]*)\).*/\1/p'
    )
    if [ -n "$block" ]; then
        constraint_block="$block"
    fi
done
constraint_states=$(echo "$constraint_block" | tr ',' '\n' | sed -n "s/.*'\([a-z_]*\)'.*/\1/p" | sort -u)

fail=0
compare() {
    local label="$1" actual="$2"
    if [ "$actual" != "$backend_states" ]; then
        echo -e "${RED}✗ $label does not match core.ValidGameStates${NC}"
        echo "  core.ValidGameStates: $(echo "$backend_states" | tr '\n' ' ')"
        echo "  $label: $(echo "$actual" | tr '\n' ' ')"
        diff <(echo "$backend_states") <(echo "$actual") | sed 's/^/    /' || true
        fail=1
    fi
}

compare "frontend GameState union (frontend/src/types/games.ts)" "$frontend_states"
compare "allowedTransitions (backend/pkg/db/services/games.go)" "$transition_states"

if [ -z "$constraint_states" ]; then
    echo -e "${RED}✗ could not find a 'CHECK (state IN ...)' constraint in migrations${NC}"
    fail=1
else
    compare "games.state CHECK constraint (latest migration)" "$constraint_states"
fi

if [ "$fail" -ne 0 ]; then
    echo ""
    echo "Game state definitions are out of sync. Update all four:"
    echo "  - backend/pkg/core/constants.go       (const + ValidGameStates)"
    echo "  - backend/pkg/db/services/games.go    (allowedTransitions)"
    echo "  - backend/pkg/db/migrations/          (new migration for the CHECK)"
    echo "  - frontend/src/types/games.ts         (GameState union)"
    exit 1
fi

echo -e "${GREEN}✓ game states consistent across constants, transitions, migration, and frontend${NC}"
echo "  $(echo "$backend_states" | tr '\n' ' ')"
