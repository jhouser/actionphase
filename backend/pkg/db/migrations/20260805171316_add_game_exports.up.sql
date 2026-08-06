-- Game archive exports: async jobs that assemble a completed game into a
-- downloadable ZIP of Markdown files for long-term preservation.
--
-- Exports are per-game rather than per-user: completed games are a public
-- archive readable by any authenticated user (CanUserViewGame), so every
-- requester receives byte-identical content and one artifact can be cached
-- and reused. content_fingerprint detects post-completion GM edits.

BEGIN;

CREATE TABLE game_exports (
    id SERIAL PRIMARY KEY,
    game_id INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,

    -- NULL when the requesting user is later deleted; the export stays valid.
    requested_by_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,

    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'running', 'complete', 'failed')),

    -- Hash of max(updated_at) + row counts across exported tables. A cached
    -- export is reusable only while this matches a freshly computed value.
    content_fingerprint VARCHAR(64),

    -- Path within the storage backend (pkg/storage), not a URL.
    storage_path TEXT,
    size_bytes BIGINT,
    file_count INTEGER,

    error_message TEXT,
    progress_note TEXT,

    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- A terminal row must record how it terminated.
    CONSTRAINT game_exports_complete_has_artifact
        CHECK (status <> 'complete' OR storage_path IS NOT NULL),
    CONSTRAINT game_exports_failed_has_reason
        CHECK (status <> 'failed' OR error_message IS NOT NULL)
);

-- Cache lookup: newest reusable export for a game.
CREATE INDEX idx_game_exports_cache
    ON game_exports(game_id, completed_at DESC)
    WHERE status = 'complete';

-- Worker claim scan: pending/running rows, oldest first.
CREATE INDEX idx_game_exports_claim
    ON game_exports(created_at)
    WHERE status IN ('pending', 'running');

-- At most one in-flight export per game, so concurrent requests coalesce onto
-- a single job instead of assembling the same archive several times.
CREATE UNIQUE INDEX idx_game_exports_one_active
    ON game_exports(game_id)
    WHERE status IN ('pending', 'running');

COMMIT;
