-- Index the foreign keys on the loot table tables.
--
-- Postgres does not index FK columns automatically, and these two columns are the
-- sole filter in every loot query: GetLootTables and IsLootTableInGame scan by
-- game_id; GetLootTableContents and DeleteLootTableContents scan by loot_table_id.
-- They also back the ON DELETE CASCADE checks, which otherwise scan the child
-- table on every parent delete (deleting a game, or a loot table with contents).

-- Covers GetLootTables (WHERE game_id = $1 ORDER BY created_at) and the game_id
-- half of IsLootTableInGame. created_at is included so the sort is satisfied by
-- the index rather than a separate sort step.
CREATE INDEX idx_game_loot_tables_game_id
    ON game_loot_tables (game_id, created_at);

-- Covers GetLootTableContents (WHERE loot_table_id = $1 ORDER BY id),
-- DeleteLootTableContents, and the cascade from game_loot_tables.
CREATE INDEX idx_game_loot_table_contents_loot_table_id
    ON game_loot_table_contents (loot_table_id, id);
