-- name: CreateThread :one
INSERT INTO threads (game_id, phase_id, created_by_user_id, title, content)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetThread :one
SELECT t.*, u.username as creator_username
FROM threads t
JOIN users u ON t.created_by_user_id = u.id
WHERE t.id = $1;

-- name: GetGameThreads :many
SELECT t.*, u.username as creator_username,
       (SELECT COUNT(*) FROM thread_posts WHERE thread_id = t.id) as post_count
FROM threads t
JOIN users u ON t.created_by_user_id = u.id
WHERE t.game_id = $1
ORDER BY t.is_pinned DESC, t.updated_at DESC;

-- name: GetPhaseThreads :many
SELECT t.*, u.username as creator_username,
       (SELECT COUNT(*) FROM thread_posts WHERE thread_id = t.id) as post_count
FROM threads t
JOIN users u ON t.created_by_user_id = u.id
WHERE t.phase_id = $1
ORDER BY t.is_pinned DESC, t.updated_at DESC;

-- name: UpdateThread :one
UPDATE threads
SET title = $2, content = $3, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: ToggleThreadPin :one
UPDATE threads
SET is_pinned = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteThread :exec
DELETE FROM threads WHERE id = $1;

-- name: CreateThreadPost :one
INSERT INTO thread_posts (thread_id, parent_post_id, user_id, character_id, content)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetThreadPosts :many
SELECT tp.*, u.username, c.name as character_name
FROM thread_posts tp
JOIN users u ON tp.user_id = u.id
LEFT JOIN characters c ON tp.character_id = c.id
WHERE tp.thread_id = $1
ORDER BY tp.created_at;

-- name: GetThreadPost :one
SELECT tp.*, u.username, c.name as character_name
FROM thread_posts tp
JOIN users u ON tp.user_id = u.id
LEFT JOIN characters c ON tp.character_id = c.id
WHERE tp.id = $1;

-- name: UpdateThreadPost :one
UPDATE thread_posts
SET content = $2, is_edited = true, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteThreadPost :exec
DELETE FROM thread_posts WHERE id = $1;

-- name: UpdateThreadActivity :exec
UPDATE threads
SET updated_at = NOW()
WHERE id = $1;

-- name: CreateConversation :one
INSERT INTO conversations (game_id, conversation_type, title, created_by_user_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetConversation :one
SELECT * FROM conversations WHERE id = $1;

-- name: GetUserUnreadConversations :many
-- Get conversations with unread messages for a user in a game, capped at a limit.
-- Used by the dashboard PM preview to avoid fetching the full conversation list.
SELECT c.*,
       (SELECT COUNT(*) FROM conversation_participants WHERE conversation_id = c.id) as participant_count,
       COALESCE(lm.last_message, '') as last_message,
       lm.last_message_at,
       COALESCE(
           (SELECT STRING_AGG(name, ', ')
            FROM (
                SELECT DISTINCT chars.name
                FROM conversation_participants cps
                LEFT JOIN characters chars ON cps.character_id = chars.id
                LEFT JOIN games g ON c.game_id = g.id
                WHERE cps.conversation_id = c.id
                  AND chars.id IS NOT NULL
                  AND (
                      g.gm_user_id = sqlc.arg(user_id)
                      OR (chars.user_id IS NOT NULL AND chars.user_id != sqlc.arg(user_id))
                      OR chars.user_id IS NULL
                  )
                ORDER BY chars.name
            ) unique_participants),
           ''
       )::text as participant_names,
       unread.unread_count::bigint,
       cr.last_read_message_id,
       cr.last_read_at
FROM conversations c
JOIN conversation_participants cp ON c.id = cp.conversation_id
LEFT JOIN conversation_reads cr ON c.id = cr.conversation_id AND cr.user_id = sqlc.arg(user_id)
LEFT JOIN LATERAL (
    SELECT content as last_message, created_at as last_message_at
    FROM private_messages
    WHERE conversation_id = c.id
      AND is_deleted = false
      AND deleted_at IS NULL
    ORDER BY created_at DESC
    LIMIT 1
) lm ON true
LEFT JOIN LATERAL (
    SELECT COUNT(*) as unread_count
    FROM private_messages pm
    WHERE pm.conversation_id = c.id
      AND pm.created_at > COALESCE(cr.last_read_at, '1970-01-01'::timestamptz)
      AND pm.sender_user_id != sqlc.arg(user_id)
) unread ON true
WHERE cp.user_id = sqlc.arg(user_id)
  AND c.game_id = sqlc.arg(game_id)
  AND unread.unread_count > 0
ORDER BY unread.unread_count DESC, c.updated_at DESC
LIMIT sqlc.arg(max_results);

-- name: GetUserConversations :many
SELECT c.*,
       (SELECT COUNT(*) FROM conversation_participants WHERE conversation_id = c.id) as participant_count,
       COALESCE(lm.last_message, '') as last_message,
       lm.last_message_at,
       COALESCE(
           (SELECT STRING_AGG(name, ', ')
            FROM (
                SELECT DISTINCT chars.name
                FROM conversation_participants cps
                LEFT JOIN characters chars ON cps.character_id = chars.id
                LEFT JOIN games g ON c.game_id = g.id
                WHERE cps.conversation_id = c.id
                  AND chars.id IS NOT NULL
                  AND (
                      -- GM sees all participants
                      g.gm_user_id = $1
                      -- Non-GM sees only other people's characters
                      OR (chars.user_id IS NOT NULL AND chars.user_id != $1)
                      -- Non-GM sees NPCs (characters without user_id)
                      OR chars.user_id IS NULL
                  )
                ORDER BY chars.name
            ) unique_participants),
           ''
       )::text as participant_names,
       COALESCE(
           (SELECT COUNT(*)
            FROM private_messages pm
            WHERE pm.conversation_id = c.id
              AND pm.created_at > COALESCE(cr.last_read_at, '1970-01-01'::timestamptz)),
           0
       )::bigint as unread_count,
       cr.last_read_message_id,
       cr.last_read_at
FROM conversations c
JOIN conversation_participants cp ON c.id = cp.conversation_id
LEFT JOIN conversation_reads cr ON c.id = cr.conversation_id AND cr.user_id = $1
LEFT JOIN LATERAL (
    SELECT content as last_message, created_at as last_message_at
    FROM private_messages
    WHERE conversation_id = c.id
      AND is_deleted = false
      AND deleted_at IS NULL
    ORDER BY created_at DESC
    LIMIT 1
) lm ON true
WHERE cp.user_id = $1 AND c.game_id = $2
ORDER BY COALESCE(
    (SELECT COUNT(*)
     FROM private_messages pm
     WHERE pm.conversation_id = c.id
       AND pm.created_at > COALESCE(cr.last_read_at, '1970-01-01'::timestamptz)),
    0
)::bigint DESC NULLS LAST, c.updated_at DESC;

-- name: AddConversationParticipant :one
INSERT INTO conversation_participants (conversation_id, user_id, character_id)
VALUES ($1, $2, $3)
ON CONFLICT (conversation_id, user_id, character_id) DO NOTHING
RETURNING *;

-- name: GetConversationParticipants :many
SELECT cp.*, u.username, c.name as character_name
FROM conversation_participants cp
JOIN users u ON cp.user_id = u.id
LEFT JOIN characters c ON cp.character_id = c.id
WHERE cp.conversation_id = $1
ORDER BY cp.joined_at;

-- name: RemoveConversationParticipant :exec
DELETE FROM conversation_participants
WHERE conversation_id = $1 AND user_id = $2 AND
      (character_id = $3 OR ($3 IS NULL AND character_id IS NULL));

-- name: IsUserInConversation :one
SELECT EXISTS(
    SELECT 1 FROM conversation_participants
    WHERE conversation_id = $1 AND user_id = $2
);

-- name: SendPrivateMessage :one
INSERT INTO private_messages (conversation_id, sender_user_id, sender_character_id, content)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetConversationMessages :many
SELECT pm.id,
       pm.conversation_id,
       pm.sender_user_id,
       pm.sender_character_id,
       pm.content,
       pm.created_at,
       pm.updated_at,
       pm.deleted_at,
       pm.is_deleted,
       pm.is_edited,
       pm.edited_at,
       pm.edit_count,
       u.username as sender_username,
       c.name as sender_character_name,
       c.avatar_url as sender_avatar_url
FROM private_messages pm
JOIN users u ON pm.sender_user_id = u.id
LEFT JOIN characters c ON pm.sender_character_id = c.id
WHERE pm.conversation_id = $1
ORDER BY pm.created_at;

-- name: UpdateLastReadTime :exec
UPDATE conversation_participants
SET last_read_at = NOW()
WHERE conversation_id = $1 AND user_id = $2;

-- name: GetUnreadMessageCount :one
SELECT COUNT(*)
FROM private_messages pm
WHERE pm.conversation_id = $2
  AND pm.created_at > COALESCE(
    (SELECT last_read_at FROM conversation_reads WHERE user_id = $1 AND conversation_id = $2),
    '1970-01-01'::timestamptz
  )
  AND pm.sender_user_id != $1;

-- name: UpdateConversationActivity :exec
UPDATE conversations
SET updated_at = NOW()
WHERE id = $1;

-- name: GetPrivateMessage :one
SELECT * FROM private_messages WHERE id = $1;

-- name: SoftDeletePrivateMessage :exec
UPDATE private_messages
SET deleted_at = NOW(), is_deleted = true
WHERE id = $1 AND sender_user_id = $2;

-- name: UpdatePrivateMessage :one
UPDATE private_messages
SET content = $2,
    is_edited = true,
    edited_at = NOW(),
    edit_count = edit_count + 1
WHERE id = $1
  AND sender_user_id = $3
  AND is_deleted = false
RETURNING *;

-- name: SoftDeleteMessage :exec
UPDATE messages
SET deleted_at = NOW(), is_deleted = true
WHERE id = $1;

-- name: ListRecentCommentsWithParents :many
-- Get recent comments with their parent comments/posts for New Comments view
WITH RECURSIVE recent_comments AS (
    SELECT
        m.id,
        m.game_id,
        m.parent_id,
        m.author_id,
        m.character_id,
        m.content,
        m.created_at,
        m.edited_at,
        m.edit_count,
        m.deleted_at,
        m.is_deleted,
        u.username as author_username,
        c.name as character_name,
        c.avatar_url as character_avatar_url
    FROM messages m
    JOIN users u ON m.author_id = u.id
    LEFT JOIN characters c ON m.character_id = c.id
    WHERE m.game_id = $1
      AND m.message_type = 'comment'
      AND m.is_deleted = false
      AND m.deleted_at IS NULL
    ORDER BY m.created_at DESC
    LIMIT $2 OFFSET $3
),
-- Walk up the message tree recursively to find the root post for each comment
root_posts AS (
    -- Base: walk up from each recent comment, tracking the original comment's id
    SELECT rc.id AS comment_id, rc.parent_id AS current_id
    FROM recent_comments rc
    WHERE rc.parent_id IS NOT NULL
    UNION ALL
    -- Recursive step: keep walking up until we hit a post
    SELECT rp.comment_id, m.parent_id AS current_id
    FROM root_posts rp
    JOIN messages m ON m.id = rp.current_id AND m.message_type = 'comment'
    WHERE m.parent_id IS NOT NULL
),
-- Pick the post at the top of each comment's chain
root_post_ids AS (
    SELECT rp.comment_id, rp.current_id AS post_id
    FROM root_posts rp
    JOIN messages m ON m.id = rp.current_id AND m.message_type = 'post'
),
parent_messages AS (
    SELECT
        m.id,
        m.content,
        m.created_at,
        m.deleted_at,
        m.is_deleted,
        m.message_type,
        u.username as author_username,
        c.name as character_name,
        c.avatar_url as character_avatar_url
    FROM messages m
    JOIN users u ON m.author_id = u.id
    LEFT JOIN characters c ON m.character_id = c.id
    WHERE m.id IN (
        SELECT parent_id FROM recent_comments
        WHERE parent_id IS NOT NULL
    )
)
SELECT
    rc.id,
    rc.game_id,
    rc.parent_id,
    rp.post_id,
    rc.author_id,
    rc.character_id,
    rc.content,
    rc.created_at,
    rc.edited_at,
    rc.edit_count,
    rc.deleted_at,
    rc.is_deleted,
    rc.author_username,
    rc.character_name,
    rc.character_avatar_url,
    pm.content as parent_content,
    pm.created_at as parent_created_at,
    pm.deleted_at as parent_deleted_at,
    pm.is_deleted as parent_is_deleted,
    pm.message_type as parent_message_type,
    pm.author_username as parent_author_username,
    pm.character_name as parent_character_name,
    pm.character_avatar_url as parent_character_avatar_url
FROM recent_comments rc
LEFT JOIN root_post_ids rp ON rp.comment_id = rc.id
LEFT JOIN parent_messages pm ON rc.parent_id = pm.id
ORDER BY rc.created_at DESC;

-- name: GetTotalCommentCount :one
-- Get total count of comments in a game
SELECT COUNT(*) as total
FROM messages
WHERE game_id = $1
  AND message_type = 'comment'
  AND is_deleted = false
  AND deleted_at IS NULL;

-- name: ListRecentUnreadCommentsWithParents :many
-- Same as ListRecentCommentsWithParents, but excludes comments the user has
-- manually marked as read. Used by the "New Comments" view's unread-only filter
-- in manual read mode. Filtering happens before LIMIT/OFFSET so pagination
-- counts stay accurate.
WITH RECURSIVE recent_comments AS (
    SELECT
        m.id,
        m.game_id,
        m.parent_id,
        m.author_id,
        m.character_id,
        m.content,
        m.created_at,
        m.edited_at,
        m.edit_count,
        m.deleted_at,
        m.is_deleted,
        u.username as author_username,
        c.name as character_name,
        c.avatar_url as character_avatar_url
    FROM messages m
    JOIN users u ON m.author_id = u.id
    LEFT JOIN characters c ON m.character_id = c.id
    WHERE m.game_id = $1
      AND m.message_type = 'comment'
      AND m.is_deleted = false
      AND m.deleted_at IS NULL
      AND NOT EXISTS (
          SELECT 1 FROM user_comment_reads ucr
          WHERE ucr.comment_id = m.id AND ucr.user_id = $4
      )
    ORDER BY m.created_at DESC
    LIMIT $2 OFFSET $3
),
-- Walk up the message tree recursively to find the root post for each comment
root_posts AS (
    -- Base: walk up from each recent comment, tracking the original comment's id
    SELECT rc.id AS comment_id, rc.parent_id AS current_id
    FROM recent_comments rc
    WHERE rc.parent_id IS NOT NULL
    UNION ALL
    -- Recursive step: keep walking up until we hit a post
    SELECT rp.comment_id, m.parent_id AS current_id
    FROM root_posts rp
    JOIN messages m ON m.id = rp.current_id AND m.message_type = 'comment'
    WHERE m.parent_id IS NOT NULL
),
-- Pick the post at the top of each comment's chain
root_post_ids AS (
    SELECT rp.comment_id, rp.current_id AS post_id
    FROM root_posts rp
    JOIN messages m ON m.id = rp.current_id AND m.message_type = 'post'
),
parent_messages AS (
    SELECT
        m.id,
        m.content,
        m.created_at,
        m.deleted_at,
        m.is_deleted,
        m.message_type,
        u.username as author_username,
        c.name as character_name,
        c.avatar_url as character_avatar_url
    FROM messages m
    JOIN users u ON m.author_id = u.id
    LEFT JOIN characters c ON m.character_id = c.id
    WHERE m.id IN (
        SELECT parent_id FROM recent_comments
        WHERE parent_id IS NOT NULL
    )
)
SELECT
    rc.id,
    rc.game_id,
    rc.parent_id,
    rp.post_id,
    rc.author_id,
    rc.character_id,
    rc.content,
    rc.created_at,
    rc.edited_at,
    rc.edit_count,
    rc.deleted_at,
    rc.is_deleted,
    rc.author_username,
    rc.character_name,
    rc.character_avatar_url,
    pm.content as parent_content,
    pm.created_at as parent_created_at,
    pm.deleted_at as parent_deleted_at,
    pm.is_deleted as parent_is_deleted,
    pm.message_type as parent_message_type,
    pm.author_username as parent_author_username,
    pm.character_name as parent_character_name,
    pm.character_avatar_url as parent_character_avatar_url
FROM recent_comments rc
LEFT JOIN root_post_ids rp ON rp.comment_id = rc.id
LEFT JOIN parent_messages pm ON rc.parent_id = pm.id
ORDER BY rc.created_at DESC;

-- name: GetTotalUnreadCommentCount :one
-- Count of comments in a game the user has not manually marked as read
SELECT COUNT(*) as total
FROM messages m
WHERE m.game_id = $1
  AND m.message_type = 'comment'
  AND m.is_deleted = false
  AND m.deleted_at IS NULL
  AND NOT EXISTS (
      SELECT 1 FROM user_comment_reads ucr
      WHERE ucr.comment_id = m.id AND ucr.user_id = $2
  );

-- name: ListCharacterPostsAndComments :many
-- Get all posts and comments by a specific character (for Character Page)
-- Returns both posts and comments with parent context for comments
-- Only returns public (game-visibility) messages, not deleted ones
-- NPCs only show comments (not top-level posts)
WITH character_messages AS (
    SELECT
        m.id,
        m.game_id,
        m.parent_id,
        m.author_id,
        m.character_id,
        m.content,
        m.message_type,
        m.created_at,
        m.edited_at,
        m.edit_count,
        m.deleted_at,
        m.is_deleted,
        u.username as author_username,
        c.name as character_name,
        c.avatar_url as character_avatar_url
    FROM messages m
    JOIN users u ON m.author_id = u.id
    JOIN characters c ON m.character_id = c.id
    WHERE m.character_id = $1
      AND m.visibility = 'game'
      AND m.is_deleted = false
      AND m.deleted_at IS NULL
      AND NOT (c.character_type = 'npc' AND m.message_type = 'post')
    ORDER BY m.created_at DESC
    LIMIT $2 OFFSET $3
),
parent_messages AS (
    SELECT
        m.id,
        m.content,
        m.created_at,
        m.deleted_at,
        m.is_deleted,
        m.message_type,
        u.username as author_username,
        c.name as character_name,
        c.avatar_url as character_avatar_url
    FROM messages m
    JOIN users u ON m.author_id = u.id
    LEFT JOIN characters c ON m.character_id = c.id
    WHERE m.id IN (
        SELECT parent_id FROM character_messages
        WHERE parent_id IS NOT NULL
    )
)
SELECT
    cm.id,
    cm.game_id,
    cm.parent_id,
    cm.author_id,
    cm.character_id,
    cm.content,
    cm.message_type,
    cm.created_at,
    cm.edited_at,
    cm.edit_count,
    cm.deleted_at,
    cm.is_deleted,
    cm.author_username,
    cm.character_name,
    cm.character_avatar_url,
    pm.content as parent_content,
    pm.created_at as parent_created_at,
    pm.deleted_at as parent_deleted_at,
    pm.is_deleted as parent_is_deleted,
    pm.message_type as parent_message_type,
    pm.author_username as parent_author_username,
    pm.character_name as parent_character_name,
    pm.character_avatar_url as parent_character_avatar_url
FROM character_messages cm
LEFT JOIN parent_messages pm ON cm.parent_id = pm.id
ORDER BY cm.created_at DESC;

-- name: CountCharacterPostsAndComments :one
-- Count all public non-deleted posts and comments by a character
-- NPCs only count comments (not top-level posts)
SELECT COUNT(*) as total
FROM messages m
JOIN characters c ON m.character_id = c.id
WHERE m.character_id = $1
  AND m.visibility = 'game'
  AND m.is_deleted = false
  AND m.deleted_at IS NULL
  AND NOT (c.character_type = 'npc' AND m.message_type = 'post');
