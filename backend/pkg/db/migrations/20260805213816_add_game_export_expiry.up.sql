-- Retention for game archive exports.
--
-- A completed game is effectively immutable: creates are blocked server-side
-- (ValidateGameNotCompleted) and the History tab is read-only, so an archive
-- built once stays correct. Artifacts therefore never need rebuilding — only
-- expiring, so storage does not grow without bound.
--
-- After expiry the export row is retained (as history) with storage_path
-- cleared, and the next request regenerates the archive on demand.

BEGIN;

ALTER TABLE game_exports
    ADD COLUMN expires_at TIMESTAMPTZ;

-- Backfill existing completed exports so they age out like any other.
UPDATE game_exports
SET expires_at = completed_at + INTERVAL '7 days'
WHERE status = 'complete' AND completed_at IS NOT NULL AND expires_at IS NULL;

-- Sweep lookup: complete exports whose artifact is still present and due.
CREATE INDEX idx_game_exports_expiry
    ON game_exports(expires_at)
    WHERE status = 'complete' AND storage_path IS NOT NULL;

-- The original constraint required every 'complete' row to carry a
-- storage_path. Expiry deliberately clears it, so the invariant is relaxed to
-- "a complete export must record when it expires" instead.
ALTER TABLE game_exports
    DROP CONSTRAINT IF EXISTS game_exports_complete_has_artifact;

ALTER TABLE game_exports
    ADD CONSTRAINT game_exports_complete_has_expiry
    CHECK (status <> 'complete' OR expires_at IS NOT NULL);

COMMIT;
