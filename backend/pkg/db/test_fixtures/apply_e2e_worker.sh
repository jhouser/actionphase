#!/bin/bash
set -euo pipefail

# Apply E2E fixtures for a specific worker
# Usage: ./apply_e2e_worker.sh <worker_index>
#
# This script takes existing E2E fixture files and adapts them for a specific worker by:
# 1. Replacing user references (TestGM -> TestGM_N for worker N)
# 2. Offsetting game IDs (game 300 -> 10300 for worker 1, 20300 for worker 2, etc.)

WORKER_INDEX="${1:-0}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DB_NAME="${DB_NAME:-actionphase}"

# Database connection
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:-postgres}"
DB_PASSWORD="${DB_PASSWORD:-example}"

# Calculate game ID offset (worker 0: +0, worker 1: +10000, worker 2: +20000, etc.)
GAME_ID_OFFSET=$((WORKER_INDEX * 10000))

echo "🔧 Applying E2E fixtures for Worker #$WORKER_INDEX (game ID offset: +$GAME_ID_OFFSET)"

# Helper function to apply SQL with worker-specific replacements
apply_worker_sql() {
    local sql_file="$1"
    local filename=$(basename "$sql_file")

    echo "  📄 Processing $filename for worker $WORKER_INDEX..."

    # Create temporary file with worker-specific replacements
    local temp_file=$(mktemp)

    if [ "$WORKER_INDEX" -eq 0 ]; then
        # Worker 0: Keep original users and keep DELETE with IDs (Python will offset them)
        # Only remove DELETE statements that use title matching (can't be offset)
        sed -E -e "/^DELETE FROM games WHERE title IN/,/;$/d" "$sql_file" > "$temp_file"
    else
        # Other workers: Need user suffix and game ID offset
        # Step 1: Replace user references with worker-specific versions
        # Only remove title-based DELETE statements (ID-based ones will be offset by Python)
        sed -E \
            -e "s/'Test(GM|Player[0-9]|Audience)@/'Test\1_${WORKER_INDEX}@/g" \
            -e "s/test_(gm|player[0-9]|audience[0-9]|audience)@/test_\1_${WORKER_INDEX}@/g" \
            -e "s/'Test(GM|Player[0-9]|Audience[0-9]|Audience)'/'Test\1_${WORKER_INDEX}'/g" \
            -e "/^DELETE FROM games WHERE title IN/,/;$/d" \
            "$sql_file" > "$temp_file.step1"

        # Step 2: Offset game IDs using Python for reliable arithmetic
        python3 -c "
import sys
import re

offset = $GAME_ID_OFFSET

with open('$temp_file.step1', 'r') as f:
    content = f.read()

# Helper to offset a game ID value
def offset_id(id_str):
    return str(int(id_str) + offset)

# 0. Replace worker_game_id_offset variable assignments in DO blocks
# Handles: worker_game_id_offset INTEGER := 0;
content = re.sub(
    r'(worker_game_id_offset\s+INTEGER\s*:=\s*)0(\s*;)',
    lambda m: m.group(1) + str(offset) + m.group(2),
    content, flags=re.IGNORECASE)

# 1. Offset game IDs in INSERT INTO games statements (handles both inline and multi-line)
# Only match 1-3 digit game IDs (original range) to avoid double-offsetting
content = re.sub(
    r'(INSERT INTO games \([^)]*\bid\b[^)]*\)\s+VALUES\s*\(\s*)(\d{1,3})(\s*,)',
    lambda m: m.group(1) + offset_id(m.group(2)) + m.group(3),
    content, flags=re.IGNORECASE)

# 1b. Offset game IDs in INSERT INTO games ... SELECT statements
# Handles pattern: INSERT INTO games (...) SELECT 700, ...
content = re.sub(
    r'(INSERT INTO games \([^)]*\)\s+SELECT\s+)(\d{1,3})(\s*,)',
    lambda m: m.group(1) + offset_id(m.group(2)) + m.group(3),
    content, flags=re.IGNORECASE)

# 2. Offset game_id in related table INSERTs (game_participants, game_phases, characters, etc.)
# This pattern matches: INSERT INTO table (...game_id...) VALUES (164, ...)
# Only match 1-3 digit game IDs to avoid double-offsetting
content = re.sub(
    r'(INSERT INTO (?!games\b)\w+\s*\([^)]*\bgame_id\b[^)]*\)\s+VALUES\s*\(\s*)(\d{1,3})(\s*,)',
    lambda m: m.group(1) + offset_id(m.group(2)) + m.group(3),
    content, flags=re.IGNORECASE)

# 2a. Offset game_id in INSERT INTO non_games_table (...) SELECT game_id, ... statements
# Handles pattern: INSERT INTO conversations (id, game_id, ...) SELECT 9991, 354, ...
# This is critical for fixtures like private message deletion that use SELECT instead of VALUES
content = re.sub(
    r'(INSERT INTO (?!games\b)\w+\s*\([^)]*\)\s+SELECT\s+\d+\s*,\s*)(\d{1,3})(\s*,)',
    lambda m: m.group(1) + offset_id(m.group(2)) + m.group(3),
    content, flags=re.IGNORECASE)

# 2a-ii. Offset conversation/message IDs in INSERT INTO ... SELECT statements
# Handles patterns:
#   - INSERT INTO conversations (...) SELECT 9991, 354, ...
#   - INSERT INTO private_messages (...) SELECT 99911, 9991, ...
#   - INSERT INTO conversation_participants (...) SELECT 9991, ...
# Offsets 4-5 digit IDs (9000-99999 range) used for test-specific conversations/messages
content = re.sub(
    r'(INSERT INTO (?:conversations|private_messages|conversation_participants)\s*\([^)]*\)\s+SELECT\s+)(\d{4,5})(\s*,)',
    lambda m: m.group(1) + offset_id(m.group(2)) + m.group(3),
    content, flags=re.IGNORECASE)

# 2a-iii. Offset conversation_id references in SELECT statements (for participants and messages)
# Handles: SELECT 9991, c.user_id, ... (conversation_id in conversation_participants)
# Handles: SELECT 99911, 9991, c.user_id, ... (message_id, conversation_id in private_messages)
content = re.sub(
    r'(SELECT\s+\d{4,5}\s*,\s*)(\d{4,5})(\s*,)',
    lambda m: m.group(1) + offset_id(m.group(2)) + m.group(3),
    content, flags=re.IGNORECASE)

# 2a-iv. Offset conversation/message IDs in ON CONFLICT clauses
# Handles: ON CONFLICT (id) DO UPDATE SET title = EXCLUDED.title, game_id = 354;
# WHERE id appears in conversations/messages INSERT statements
content = re.sub(
    r'(ON CONFLICT \(id\) DO UPDATE SET [^;]+ game_id = )(\d{1,3})(;)',
    lambda m: m.group(1) + offset_id(m.group(2)) + m.group(3),
    content, flags=re.IGNORECASE)

# 2a-i. Offset game_id = NNN in WHERE/JOIN clauses
# Handles patterns: c.game_id = 354, AND c.game_id = 354, ON c.game_id = 354
# Matches table aliases (c., u., etc.) or direct column names
content = re.sub(
    r'\b(\w+\.)?game_id\s*=\s*(\d{1,3})\b',
    lambda m: (m.group(1) or '') + 'game_id = ' + offset_id(m.group(2)),
    content, flags=re.IGNORECASE)

# 2b. Offset subsequent rows in multi-row INSERTs
# Matches BOTH newline-separated AND inline comma-separated rows
# Pattern: , (164, user_id, ...) OR newline (164, user_id, ...)
content = re.sub(
    r'([,\n]\s*\()(\d{1,3})(\s*,\s*\w+_id)',
    lambda m: m.group(1) + offset_id(m.group(2)) + m.group(3),
    content)

# 3. Offset game_id variable assignments: game_xxx_id := NNN; OR game_xxx_id INT := NNN;
# Handles both with and without type declaration (INT or INTEGER)
# Matches: game_id, game1_id, game_complete_id, etc.
content = re.sub(
    r'\b(game_?\w*_id(?:\s+(?:INT|INTEGER))?)\s*:=\s*(\d{1,3});',
    lambda m: f'{m.group(1)} := {offset_id(m.group(2))};',
    content)

# 4. Offset DELETE FROM games WHERE id IN (...) statements
# Only offset 1-3 digit game IDs in the list
content = re.sub(
    r'(DELETE FROM games WHERE id IN \()([\d,\s]+)(\))',
    lambda m: m.group(1) + ','.join(offset_id(id.strip()) if len(id.strip()) <= 3 else id.strip() for id in m.group(2).split(',') if id.strip()) + m.group(3),
    content)

# 4b. Offset DELETE FROM games WHERE id = NNN statements
# Handles single-ID DELETE statements like: DELETE FROM games WHERE id = 700;
content = re.sub(
    r'(DELETE FROM games WHERE id\s*=\s*)(\d{1,3})(;)',
    lambda m: m.group(1) + offset_id(m.group(2)) + m.group(3),
    content)

# 5. Offset comments with game numbers: -- GAME #164:
# Only match 1-3 digit game IDs
content = re.sub(
    r'(-- GAME #)(\d{1,3})(:)',
    lambda m: m.group(1) + offset_id(m.group(2)) + m.group(3),
    content)

# 6. Offset RAISE NOTICE messages with game IDs
# Only match 1-3 digit game IDs
content = re.sub(
    r\"(RAISE NOTICE 'Created Game #)(\d{1,3})(:)\",
    lambda m: m.group(1) + offset_id(m.group(2)) + m.group(3),
    content)

with open('$temp_file', 'w') as f:
    f.write(content)
" || {
    echo "ERROR: Python script failed"
    rm -f "$temp_file" "$temp_file.step1"
    return 1
}

        # Clean up intermediate file
        rm -f "$temp_file.step1"
    fi

    # Apply the modified SQL (with error checking)
    if ! PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -f "$temp_file"; then
        echo "❌ ERROR: Failed to apply $filename for worker $WORKER_INDEX"
        # Keep temp file for debugging
        echo "   Temp file saved at: $temp_file"
        return 1
    fi

    # Clean up
    rm "$temp_file"
}

# Pre-flight: fail fast if two fixtures claim the same game ID. Cheap to run and
# catches the "Fixture game not found" class of failure at its source instead of
# in whichever unrelated spec loses the race.
echo "🔎 Checking fixture game ID uniqueness..."
if ! python3 "$SCRIPT_DIR/check_fixture_ids.py"; then
    echo "❌ Aborting: fix the duplicate game IDs above before applying fixtures."
    exit 1
fi

# First, apply worker setup to create helper functions
echo "  📄 Applying worker setup (creates helper functions)..."
if ! PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -f "$SCRIPT_DIR/e2e/00_worker_setup.sql"; then
    echo "❌ ERROR: Failed to apply worker setup"
    exit 1
fi

# Apply all E2E fixture files in order
for file in "$SCRIPT_DIR"/e2e/*.sql; do
    filename=$(basename "$file")

    # Skip worker setup (already applied); worker-specific files are filtered below.
    if [ -f "$file" ] && [ "$filename" != "00_worker_setup.sql" ]; then
        # For private message deletion, co-GM management, and co-GM action results, use worker-specific file (no transformation needed)
        if [[ "$filename" == 17_private_message_deletion_w*.sql ]] || [[ "$filename" == 18_co_gm_management_w*.sql ]] || [[ "$filename" == 18_co_gm_action_results_w*.sql ]] || [[ "$filename" == 19_player_multiple_characters_w*.sql ]] || [[ "$filename" == 21_audience_private_messages_w*.sql ]] || [[ "$filename" == 23_private_message_editing_w*.sql ]] || [[ "$filename" == 26_player_to_audience_w*.sql ]]; then
            # Only process if it matches our worker index
            if [[ "$filename" == "17_private_message_deletion_w${WORKER_INDEX}.sql" ]] || [[ "$filename" == "18_co_gm_management_w${WORKER_INDEX}.sql" ]] || [[ "$filename" == "18_co_gm_action_results_w${WORKER_INDEX}.sql" ]] || [[ "$filename" == "19_player_multiple_characters_w${WORKER_INDEX}.sql" ]] || [[ "$filename" == "21_audience_private_messages_w${WORKER_INDEX}.sql" ]] || [[ "$filename" == "23_private_message_editing_w${WORKER_INDEX}.sql" ]] || [[ "$filename" == "26_player_to_audience_w${WORKER_INDEX}.sql" ]]; then
                echo "  📄 Processing $filename for worker $WORKER_INDEX (no transformation)..."
                # Apply directly without transformation
                if ! PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -f "$file"; then
                    echo "❌ ERROR: Failed to apply $filename for worker $WORKER_INDEX"
                fi
            fi
        else
            # Process all other files with transformation
            apply_worker_sql "$file"
        fi
    fi
done

# Advance ID sequences past every fixture row, for all workers.
#
# Individual fixture files each run setval(seq, MAX(id)+1) after their own
# inserts. That is only correct for the file that happens to run last: worker N
# offsets IDs by N*10000, so a low-offset file applied after a high-offset one
# rewinds the sequence below IDs that already exist. The next game a test
# creates then collides — "duplicate key value violates unique constraint
# games_pkey" — as an intermittent 500 that looks like a flaky frontend timeout.
#
# Re-syncing here, after every file for this worker, keeps the sequence ahead of
# whatever has been loaded so far regardless of file or worker ordering.
echo "  🔢 Re-syncing ID sequences past fixture rows..."
if ! PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -q -c "
SELECT setval('games_id_seq',    (SELECT COALESCE(MAX(id), 0) + 1 FROM games));
SELECT setval('messages_id_seq', (SELECT COALESCE(MAX(id), 0) + 1 FROM messages));
"; then
    echo "❌ ERROR: Failed to re-sync ID sequences for worker $WORKER_INDEX"
    exit 1
fi

echo "✅ Worker #$WORKER_INDEX E2E fixtures applied successfully!"
