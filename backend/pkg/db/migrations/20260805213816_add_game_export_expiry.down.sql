-- Reverses 20260805213816_add_game_export_expiry.up.sql

BEGIN;

ALTER TABLE game_exports
    DROP CONSTRAINT IF EXISTS game_exports_complete_has_expiry;

DROP INDEX IF EXISTS idx_game_exports_expiry;

-- Expired rows are 'complete' with storage_path cleared, which the restored
-- constraint forbids. Drop them rather than fail the migration: they describe
-- artifacts that no longer exist, so nothing recoverable is lost.
DELETE FROM game_exports
WHERE status = 'complete' AND storage_path IS NULL;

ALTER TABLE game_exports
    ADD CONSTRAINT game_exports_complete_has_artifact
    CHECK (status <> 'complete' OR storage_path IS NOT NULL);

ALTER TABLE game_exports
    DROP COLUMN expires_at;

COMMIT;
