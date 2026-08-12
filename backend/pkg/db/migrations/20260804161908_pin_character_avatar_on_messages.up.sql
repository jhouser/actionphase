-- Pin the character's avatar URL at the time a message is authored.
--
-- Message read paths previously joined characters.avatar_url live, so every
-- historical post rendered whatever the character's avatar is *now*. For games
-- where appearance is narrative (progressive injury, shapeshifting, costume
-- changes), that silently misrepresents the archive.
--
-- Nullable by design: existing rows stay NULL and read paths fall back to the
-- live join via COALESCE, so this migration needs no backfill and does not
-- change how any pre-existing message renders.
ALTER TABLE messages
    ADD COLUMN character_avatar_url_at_post VARCHAR(500);

COMMENT ON COLUMN messages.character_avatar_url_at_post IS
    'Character avatar URL captured when this message was created. Written once at insert and never updated (editing a message does not repaint its avatar). NULL for messages authored before this column existed, or when the character had no avatar; readers COALESCE to characters.avatar_url.';
