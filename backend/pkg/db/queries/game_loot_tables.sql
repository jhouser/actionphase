-- name: CreateLootTable :one
INSERT INTO game_loot_tables (
    game_id, name
) VALUES (
    $1, $2
) RETURNING id, game_id, name, created_at;

-- name: GetLootTables :many
SELECT
    id, game_id, name, created_at
FROM game_loot_tables
WHERE game_id = $1
ORDER BY created_at ASC;

-- name: IsLootTableInGame :one
SELECT
    EXISTS(SELECT 1 FROM game_loot_tables WHERE game_id = $1 AND id = $2) 
    AS is_game_loot_table;

-- name: UpdateLootTable :one
UPDATE game_loot_tables
SET name = $2
WHERE id = $1
RETURNING id, game_id, name, created_at;

-- name: DeleteLootTable :exec
DELETE FROM game_loot_tables WHERE id = $1;

