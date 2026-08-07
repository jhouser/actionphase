-- Reverses 20260805171316_add_game_exports.up.sql
--
-- Note: this drops export job records only. Generated ZIP artifacts live in
-- the storage backend (local disk or S3) and are not removed here.

BEGIN;

DROP INDEX IF EXISTS idx_game_exports_one_active;
DROP INDEX IF EXISTS idx_game_exports_claim;
DROP INDEX IF EXISTS idx_game_exports_cache;

DROP TABLE IF EXISTS game_exports;

COMMIT;
