-- Loot tables have a rename path (UpdateLootTable) but no updated_at, so there was
-- no way to tell when one last changed. Backfill existing rows from created_at.
ALTER TABLE game_loot_tables
    ADD COLUMN updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW();

UPDATE game_loot_tables SET updated_at = created_at WHERE updated_at IS NULL;
