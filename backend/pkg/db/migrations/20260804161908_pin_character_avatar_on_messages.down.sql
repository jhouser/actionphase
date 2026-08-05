-- Dropping this column permanently discards pinned historical avatar URLs.
-- The data is not recoverable by re-running the up migration: the pinned values
-- record what each character's avatar was at authoring time, which cannot be
-- reconstructed from the current characters.avatar_url.
ALTER TABLE messages
    DROP COLUMN character_avatar_url_at_post;
