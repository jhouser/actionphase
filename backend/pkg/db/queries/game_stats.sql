-- =============================================================================
-- GAME STATISTICS QUERIES
-- =============================================================================
-- Post-game statistics for the Stats tab on completed games. See
-- pkg/db/services/gamestats.go and pkg/games/api_stats.go.
--
-- Terminology: what players call a "post" is a `comment` row. The `post`
-- message_type is the GM-authored topic container that a phase's discussion
-- hangs off (typically 1 post per phase with hundreds of comments beneath it),
-- so it is deliberately excluded from every aggregate here — averaging GM topic
-- text into player writing stats would measure the wrong thing.
--
-- Exclusions match the archive rules used elsewhere: drafts (is_draft) and
-- soft-deleted rows (is_deleted) were never visible to anyone but their author
-- and never count toward stats.

-- name: GetGameCommentStats :one
-- Game-wide common room aggregates over player comments.
--
-- avg_length is rounded to a whole number of characters: sub-character
-- precision is noise in the UI. COALESCE keeps the zero-comment case returning
-- a row of zeros rather than NULLs, so the handler needs no null handling.
--
-- Thread depth is denormalized on messages, so reply-tree shape costs nothing
-- extra here. Depth is stored 0-based (a direct reply to the post is depth 1
-- in practice); it is reported raw and labeled in the UI.
SELECT
    COUNT(*) AS total_comments,
    COALESCE(ROUND(AVG(LENGTH(content))), 0)::bigint AS avg_comment_length,
    COALESCE(MAX(LENGTH(content)), 0)::bigint AS longest_comment_length,
    COALESCE(MAX(thread_depth), 0)::bigint AS max_thread_depth,
    -- Cast to float8 rather than leaving it numeric: sqlc maps numeric to
    -- pgtype.Numeric, which is a clumsy type to carry through the service and
    -- serialize for a value that is only ever displayed to one decimal place.
    COALESCE(ROUND(AVG(thread_depth), 1), 0)::float8 AS avg_thread_depth
FROM messages
WHERE game_id = $1
  AND message_type = 'comment'
  AND is_deleted = false
  AND is_draft = false;

-- name: GetGameLongestComment :one
-- The single longest comment, for a deep link from the stats page.
--
-- Kept separate from GetGameCommentStats rather than folded into it: that query
-- is a pure aggregate, and making it also carry the winning row's id would mean
-- a window function or self-join over every comment in the game. This is a
-- plain ORDER BY ... LIMIT 1 against the same filtered set.
--
-- Returns no row for a game with no comments; callers must treat pgx.ErrNoRows
-- as "not applicable". Ties break on the earliest comment (id ASC) so the
-- result is stable across requests.
SELECT
    m.id AS message_id,
    LENGTH(m.content)::bigint AS content_length,
    m.parent_id,
    m.phase_id,
    ch.name AS character_name
FROM messages m
LEFT JOIN characters ch ON ch.id = m.character_id
WHERE m.game_id = $1
  AND m.message_type = 'comment'
  AND m.is_deleted = false
  AND m.is_draft = false
ORDER BY LENGTH(m.content) DESC, m.id ASC
LIMIT 1;

-- name: GetGamePrivateMessageStats :one
-- Game-wide private message aggregates.
--
-- Scoped through conversations because private_messages has no game_id of its
-- own. private_messages has no is_draft column (PMs are never drafted), so
-- is_deleted is the only exclusion.
SELECT
    COUNT(pm.id) AS total_private_messages,
    COALESCE(ROUND(AVG(LENGTH(pm.content))), 0)::bigint AS avg_private_message_length,
    COALESCE(MAX(LENGTH(pm.content)), 0)::bigint AS longest_private_message_length,
    COUNT(DISTINCT pm.conversation_id) AS active_conversations
FROM conversations c
LEFT JOIN private_messages pm
       ON pm.conversation_id = c.id
      AND pm.is_deleted = false
WHERE c.game_id = $1;

-- name: GetGameLongestConversation :one
-- The conversation with the most (non-deleted) messages, "longest" by message
-- count rather than character volume or duration.
--
-- Returns no row when the game has no conversations with messages; callers must
-- treat pgx.ErrNoRows as "not applicable" rather than an error. The HAVING
-- clause is what guarantees that: an empty conversation is not a longest one.
--
-- Participant names are aggregated here so the caller renders "Alice + Bruno"
-- without a second query. DISTINCT guards against a character appearing twice
-- via multiple participant rows.
SELECT
    c.id AS conversation_id,
    c.title,
    c.conversation_type,
    COUNT(pm.id) AS message_count,
    -- Explicit casts: sqlc cannot infer the type of MIN/MAX over the
    -- nullable-by-outer-context timestamptz and falls back to interface{}.
    MIN(pm.created_at)::timestamptz AS first_message_at,
    MAX(pm.created_at)::timestamptz AS last_message_at,
    COALESCE(
        (SELECT ARRAY_AGG(DISTINCT ch.name ORDER BY ch.name)
         FROM conversation_participants cp
         JOIN characters ch ON ch.id = cp.character_id
         WHERE cp.conversation_id = c.id),
        ARRAY[]::varchar[]
    )::varchar[] AS participant_names
FROM conversations c
JOIN private_messages pm
  ON pm.conversation_id = c.id
 AND pm.is_deleted = false
WHERE c.game_id = $1
GROUP BY c.id, c.title, c.conversation_type
HAVING COUNT(pm.id) > 0
ORDER BY message_count DESC, c.id ASC
LIMIT 1;

-- name: GetCharacterStatsByGame :many
-- Per-character writing stats for every character in a game, in one query.
--
-- Comments and private messages are aggregated in separate subqueries rather
-- than joined directly onto characters: joining both at once multiplies the two
-- row sets together and inflates every AVG and COUNT. (The existing
-- GetCharacterActivityStatsByGame avoids this with COUNT(DISTINCT ...), which
-- works for counts but cannot be made to work for averages.)
--
-- LEFT JOINs keep characters with no activity in the result with zero counts,
-- so the roster is complete and the frontend needs no separate character list.
SELECT
    ch.id AS character_id,
    ch.name AS character_name,
    ch.user_id,
    ch.is_active,
    COALESCE(cm.comment_count, 0)::bigint AS comment_count,
    COALESCE(cm.avg_comment_length, 0)::bigint AS avg_comment_length,
    COALESCE(pm.private_message_count, 0)::bigint AS private_message_count,
    COALESCE(pm.avg_private_message_length, 0)::bigint AS avg_private_message_length
FROM characters ch
LEFT JOIN (
    SELECT
        msg.character_id,
        COUNT(*) AS comment_count,
        ROUND(AVG(LENGTH(msg.content))) AS avg_comment_length
    FROM messages msg
    WHERE msg.game_id = $1
      AND msg.message_type = 'comment'
      AND msg.is_deleted = false
      AND msg.is_draft = false
    GROUP BY msg.character_id
) cm ON cm.character_id = ch.id
LEFT JOIN (
    SELECT
        pmsg.sender_character_id,
        COUNT(*) AS private_message_count,
        ROUND(AVG(LENGTH(pmsg.content))) AS avg_private_message_length
    FROM private_messages pmsg
    JOIN conversations conv ON conv.id = pmsg.conversation_id
    WHERE conv.game_id = $1
      AND pmsg.is_deleted = false
    GROUP BY pmsg.sender_character_id
) pm ON pm.sender_character_id = ch.id
WHERE ch.game_id = $1
ORDER BY comment_count DESC, ch.name ASC;
