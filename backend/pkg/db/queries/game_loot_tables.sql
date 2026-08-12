-- name: CreateLootTable :one
INSERT INTO game_loot_tables (
    game_id, name
) VALUES (
    $1, $2
) RETURNING id, game_id, name, created_at, updated_at;

-- name: GetLootTables :many
SELECT
    id, game_id, name, created_at, updated_at
FROM game_loot_tables
WHERE game_id = $1
ORDER BY created_at ASC;

-- Handlers MUST call this before mutating a loot table by ID: the update and
-- delete below are keyed on id alone, so this is what enforces game ownership.
-- Covered by TestLootTableEndpointsRejectCrossGameAccess.
-- name: IsLootTableInGame :one
SELECT
    EXISTS(SELECT 1 FROM game_loot_tables WHERE game_id = $1 AND id = $2)
    AS is_game_loot_table;

-- name: UpdateLootTable :one
UPDATE game_loot_tables
SET name = $2, updated_at = NOW()
WHERE id = $1
RETURNING id, game_id, name, created_at, updated_at;

-- Contents live in a child table, so rewriting them leaves the parent row
-- untouched and updated_at stale. Handlers that change contents MUST call this
-- so "Updated" reflects item edits, which are the common operation — renaming is
-- comparatively rare.
-- name: TouchLootTable :exec
UPDATE game_loot_tables
SET updated_at = NOW()
WHERE id = $1;

-- name: DeleteLootTable :exec
DELETE FROM game_loot_tables WHERE id = $1;
