#!/usr/bin/env python3
"""Pre-flight check: assert no two E2E fixtures claim the same game ID.

Fixtures hardcode game IDs and each opens with DELETE FROM games WHERE id = N.
Files are applied in filename order, so when two files claim the same ID the
later one silently destroys the earlier one's game. The failure then surfaces in
an unrelated spec as "Fixture game not found", far from the file that caused it.

This check turns that into an error before any fixture is applied.

Two duplicate shapes are legitimate and are NOT reported:

  * Worker variants (foo_w0.sql ... foo_w5.sql) are one logical fixture. They
    are collapsed to a single owner by stripping the _wN suffix.
  * Worker variants are PRE-OFFSET: _w5 hardcodes 50360 rather than 360. IDs at
    or above WORKER_OFFSET are derived, not independently claimed, so only the
    base ID (id % WORKER_OFFSET) is attributed to the fixture.
"""
import re
import sys
from collections import defaultdict
from pathlib import Path

WORKER_OFFSET = 10000

# Shapes that claim a game ID. Each must capture the ID (or an ID list) in
# group 1. Kept deliberately narrow: a false positive here blocks the suite.
CLAIM_PATTERNS = [
    # game_id := 349;  /  game_complete_id INTEGER := 349;
    re.compile(r'\bgame\w*_id(?:\s+(?:INT|INTEGER))?\s*:=\s*(\d{1,5})\b', re.I),
    # INSERT INTO games (id, ...) VALUES (349, ...)
    re.compile(r'INSERT INTO games\s*\([^)]*\bid\b[^)]*\)\s*VALUES\s*\(\s*(\d{1,5})\s*,', re.I | re.S),
    # INSERT INTO games (...) SELECT 349, ...
    re.compile(r'INSERT INTO games\s*\([^)]*\)\s*SELECT\s+(\d{1,5})\s*,', re.I | re.S),
    # DELETE FROM games WHERE id = 349
    re.compile(r'DELETE FROM games WHERE id\s*=\s*(\d{1,5})\b', re.I),
    # DELETE FROM games WHERE id IN (348, 349)
    re.compile(r'DELETE FROM games WHERE id IN \(([\d,\s]+)\)', re.I),
    # VALUES tuple lists driving a loop: (349, 'epilogue', 'Title', 9949)
    re.compile(r'\(\s*(\d{3,5})\s*,\s*\'', re.S),
]


def logical_owner(filename: str) -> str:
    """Collapse foo_w3.sql -> foo.sql so worker variants share one owner."""
    return re.sub(r'_w\d+(?=\.sql$)', '', filename)


def base_id(game_id: int) -> int:
    """Strip the worker offset: 50360 -> 360, 360 -> 360."""
    return game_id % WORKER_OFFSET


def claims_in(text: str) -> set[int]:
    found = set()
    for pattern in CLAIM_PATTERNS:
        for match in pattern.finditer(text):
            for token in match.group(1).split(','):
                token = token.strip()
                if token.isdigit():
                    found.add(base_id(int(token)))
    return found


def main() -> int:
    e2e_dir = Path(__file__).parent / 'e2e'
    owners: dict[int, set[str]] = defaultdict(set)

    for sql_file in sorted(e2e_dir.glob('*.sql')):
        for game_id in claims_in(sql_file.read_text()):
            owners[game_id].add(logical_owner(sql_file.name))

    conflicts = {gid: sorted(files) for gid, files in owners.items() if len(files) > 1}

    if not conflicts:
        print(f"✅ Fixture game IDs unique across {len(owners)} games.")
        return 0

    print("❌ Duplicate fixture game IDs detected.\n", file=sys.stderr)
    for game_id, files in sorted(conflicts.items()):
        print(f"  Game {game_id} claimed by: {', '.join(files)}", file=sys.stderr)
    print(
        "\nFixtures are applied in filename order and each DELETEs its game ID "
        "first,\nso the later file will destroy the earlier one's game. "
        "Assign a free ID.",
        file=sys.stderr,
    )
    return 1


if __name__ == '__main__':
    sys.exit(main())
