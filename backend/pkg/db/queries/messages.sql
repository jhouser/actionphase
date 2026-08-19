-- Messages Queries (Common Room posts and future private messages)

-- ============================================================================
-- POST MANAGEMENT (Top-level messages)
-- ============================================================================

-- name: CreatePost :one
-- character_avatar_url_at_post pins the character's avatar as of this insert so
-- the post keeps rendering the avatar it was written with. Resolved inline to
-- keep the capture atomic with the insert.
INSERT INTO messages (
    game_id,
    phase_id,
    author_id,
    character_id,
    content,
    message_type,
    visibility,
    mentioned_character_ids,
    character_avatar_url_at_post
) VALUES (
    $1, $2, $3, $4, $5, 'post', $6, $7,
    (SELECT avatar_url FROM characters WHERE id = $4)
)
RETURNING *;

-- name: GetPost :one
SELECT m.*,
       u.username as author_username,
       c.name as character_name,
       COALESCE(m.character_avatar_url_at_post, c.avatar_url) as character_avatar_url,
       (SELECT COUNT(*) FROM messages WHERE parent_id = m.id) as comment_count
FROM messages m
JOIN users u ON m.author_id = u.id
LEFT JOIN characters c ON m.character_id = c.id
WHERE m.id = $1;

-- name: GetMessage :one
-- Get any message by ID (post or comment) - used for deep linking
SELECT m.*,
       u.username as author_username,
       c.name as character_name,
       COALESCE(m.character_avatar_url_at_post, c.avatar_url) as character_avatar_url,
       (SELECT COUNT(*) FROM messages WHERE parent_id = m.id) as reply_count
FROM messages m
JOIN users u ON m.author_id = u.id
LEFT JOIN characters c ON m.character_id = c.id
WHERE m.id = $1
  AND m.is_deleted = false;

-- name: GetMessagePhaseID :one
-- Get just the phase_id of a message, used when a new comment inherits its
-- phase from the message it replies to. Deliberately does NOT filter on
-- is_deleted: a reply to a soft-deleted parent still belongs to that phase.
SELECT phase_id
FROM messages
WHERE id = $1;

-- name: GetMessageWithParentContext :many
-- Get a message plus a bounded slice of its parent chain for deep linking with context.
-- Walks up from the target, returning the target + up to $2 nearest parents, in a single
-- query (avoids a per-level request waterfall from the client).
-- The trim happens in SQL via thread_depth (a real, monotonic column) so a very deep
-- thread does not join/count rows we will discard. Every returned row also carries
-- root_post_id (the true top-level post) so callers get the real root post ID for read
-- tracking / replies without a second round trip, even when the slice is trimmed.
-- Returns rows in parent-to-child order (nearest-included-parent → target).
-- $1 = message_id (target comment)
-- $2 = max_parents (how many ancestor levels above the target to include; 0 = target only)
WITH RECURSIVE parent_chain AS (
    -- Base case: Start with the target message.
    SELECT
        m.id,
        m.game_id,
        m.phase_id,
        m.author_id,
        m.character_id,
        m.content,
        m.message_type,
        m.parent_id,
        m.thread_depth,
        m.visibility,
        m.mentioned_character_ids,
        m.is_edited,
        m.is_deleted,
        m.deleted_at,
        m.deleted_by_user_id,
        m.edited_at,
        m.edit_count,
        m.created_at,
        m.character_avatar_url_at_post
    FROM messages m
    WHERE m.id = sqlc.arg(message_id)

    UNION ALL

    -- Recursive case: Walk up the parent chain to root. Always continues to root (the
    -- walk itself is cheap: one indexed lookup per level) so root_post_id is resolvable
    -- even when the returned slice is trimmed by thread_depth below.
    SELECT
        m.id,
        m.game_id,
        m.phase_id,
        m.author_id,
        m.character_id,
        m.content,
        m.message_type,
        m.parent_id,
        m.thread_depth,
        m.visibility,
        m.mentioned_character_ids,
        m.is_edited,
        m.is_deleted,
        m.deleted_at,
        m.deleted_by_user_id,
        m.edited_at,
        m.edit_count,
        m.created_at,
        m.character_avatar_url_at_post
    FROM messages m
    INNER JOIN parent_chain pch ON m.id = pch.parent_id
),
root AS (
    -- The top-level post is the single row in the chain with no parent.
    SELECT id AS root_post_id
    FROM parent_chain
    WHERE parent_id IS NULL
)
SELECT
    pc.id,
    pc.game_id,
    pc.phase_id,
    pc.author_id,
    pc.character_id,
    pc.content,
    pc.message_type,
    pc.parent_id,
    pc.thread_depth,
    pc.visibility,
    pc.mentioned_character_ids,
    pc.is_edited,
    pc.is_deleted,
    pc.deleted_at,
    pc.deleted_by_user_id,
    pc.edited_at,
    pc.edit_count,
    pc.created_at,
    u.username as author_username,
    c.name as character_name,
    COALESCE(pc.character_avatar_url_at_post, c.avatar_url) as character_avatar_url,
    (SELECT COUNT(*) FROM messages WHERE parent_id = pc.id) as reply_count,
    root.root_post_id as root_post_id
FROM parent_chain pc
JOIN users u ON pc.author_id = u.id
LEFT JOIN characters c ON pc.character_id = c.id
LEFT JOIN root ON true
-- Keep the target and its $2 nearest ancestors. thread_depth is a real messages column,
-- so sqlc can analyze this (unlike a CTE-only hop counter).
WHERE pc.thread_depth > (SELECT m.thread_depth FROM messages m WHERE m.id = sqlc.arg(message_id)) - (sqlc.arg(max_parents)::integer + 1)
ORDER BY pc.thread_depth ASC;  -- Return in parent-to-child order

-- name: GetGamePosts :many
SELECT m.*,
       u.username as author_username,
       c.name as character_name,
       COALESCE(m.character_avatar_url_at_post, c.avatar_url) as character_avatar_url,
       (SELECT COUNT(*) FROM messages WHERE parent_id = m.id) as comment_count
FROM messages m
JOIN users u ON m.author_id = u.id
LEFT JOIN characters c ON m.character_id = c.id
WHERE m.game_id = $1
  AND m.message_type = 'post'
  AND m.is_deleted = false
  AND m.is_draft = false
  AND (CASE WHEN $2 = 0 THEN TRUE ELSE m.phase_id = $2 END)
ORDER BY m.created_at DESC
LIMIT $3 OFFSET $4;

-- name: GetPhasePosts :many
SELECT m.*,
       u.username as author_username,
       c.name as character_name,
       COALESCE(m.character_avatar_url_at_post, c.avatar_url) as character_avatar_url,
       (SELECT COUNT(*) FROM messages WHERE parent_id = m.id) as comment_count
FROM messages m
JOIN users u ON m.author_id = u.id
LEFT JOIN characters c ON m.character_id = c.id
WHERE m.phase_id = $1
  AND m.message_type = 'post'
  AND m.is_deleted = false
  AND m.is_draft = false
ORDER BY m.created_at DESC;

-- name: UpdatePost :one
WITH updated AS (
  UPDATE messages
  SET content = $2,
      is_edited = true,
      edited_at = NOW(),
      edit_count = edit_count + 1
  WHERE messages.id = $1
    AND messages.is_deleted = false
    AND messages.message_type = 'post'
  RETURNING *
)
SELECT
  m.*,
  u.username as author_username,
  c.name as character_name,
  -- Editing a post never repaints its avatar, so return the pinned value.
  COALESCE(m.character_avatar_url_at_post, c.avatar_url) as character_avatar_url,
  (SELECT COUNT(*) FROM messages WHERE parent_id = m.id) as comment_count
FROM updated m
JOIN users u ON m.author_id = u.id
LEFT JOIN characters c ON m.character_id = c.id;

-- name: DeletePost :one
UPDATE messages
SET is_deleted = true
WHERE id = $1
  AND message_type = 'post'
RETURNING *;

-- ============================================================================
-- DRAFT POST MANAGEMENT (GM-authored posts stored before phase activation)
-- ============================================================================

-- name: CreateDraftPost :one
-- Pins the avatar at draft-authoring time, matching CreatePost. A draft
-- published later keeps the avatar it was written with, not the one current at
-- publication.
INSERT INTO messages (
    game_id,
    phase_id,
    author_id,
    character_id,
    content,
    message_type,
    visibility,
    mentioned_character_ids,
    is_draft,
    character_avatar_url_at_post
) VALUES (
    $1, $2, $3, $4, $5, 'post', $6, $7, true,
    (SELECT avatar_url FROM characters WHERE id = $4)
)
RETURNING *;

-- name: GetDraftPostForPhase :one
-- Returns the single draft post for a phase (GM only). Returns no rows if none exists.
SELECT m.*,
       u.username as author_username,
       c.name as character_name,
       COALESCE(m.character_avatar_url_at_post, c.avatar_url) as character_avatar_url,
       0::bigint as comment_count
FROM messages m
JOIN users u ON m.author_id = u.id
LEFT JOIN characters c ON m.character_id = c.id
WHERE m.phase_id = $1
  AND m.message_type = 'post'
  AND m.is_draft = true
  AND m.is_deleted = false
LIMIT 1;

-- name: UpdateDraftPost :one
UPDATE messages
SET content = $2,
    mentioned_character_ids = $3
WHERE id = $1
  AND message_type = 'post'
  AND is_draft = true
  AND is_deleted = false
RETURNING *;

-- name: PublishDraftPostsForPhase :exec
-- Called atomically during phase activation. Clears is_draft flag.
UPDATE messages
SET is_draft = false
WHERE phase_id = $1
  AND message_type = 'post'
  AND is_draft = true
  AND is_deleted = false;

-- name: DeleteDraftPost :exec
-- Hard delete a single draft post by ID.
DELETE FROM messages
WHERE id = $1
  AND is_draft = true;

-- name: DeleteDraftPostsForPhase :exec
-- Hard delete draft posts when a phase is deleted.
DELETE FROM messages
WHERE phase_id = $1
  AND message_type = 'post'
  AND is_draft = true;

-- name: CountDraftPostsByPhase :one
-- Count draft posts for a phase (used to enforce one-draft-per-phase constraint).
SELECT COUNT(*)
FROM messages
WHERE phase_id = $1
  AND message_type = 'post'
  AND is_draft = true
  AND is_deleted = false;

-- ============================================================================
-- COMMENT MANAGEMENT (Threaded replies)
-- ============================================================================

-- name: CreateComment :one
-- Pins the avatar at comment-authoring time. See CreatePost.
INSERT INTO messages (
    game_id,
    phase_id,
    author_id,
    character_id,
    content,
    message_type,
    parent_id,
    visibility,
    mentioned_character_ids,
    character_avatar_url_at_post
) VALUES (
    $1, $2, $3, $4, $5, 'comment', $6, $7, $8,
    (SELECT avatar_url FROM characters WHERE id = $4)
)
RETURNING *;

-- name: GetComment :one
SELECT m.*,
       u.username as author_username,
       c.name as character_name,
       COALESCE(m.character_avatar_url_at_post, c.avatar_url) as character_avatar_url,
       (SELECT COUNT(*) FROM messages WHERE parent_id = m.id) as reply_count
FROM messages m
JOIN users u ON m.author_id = u.id
LEFT JOIN characters c ON m.character_id = c.id
WHERE m.id = $1;

-- name: GetPostComments :many
-- Get direct comments for a specific post
-- For threaded display, call this recursively on the frontend
-- Sorted newest first (DESC) for better UX - users see latest comments at top
-- INCLUDES deleted comments to preserve thread structure - UI will show "[Comment deleted]" placeholder
--
-- reply_count is the count of ALL descendants, not just direct children. It has
-- to match GetPostCommentsWithThreads (see descendant_counts in
-- backend/pkg/db/services/messages/comments.go), because the frontend renders
-- this field as the "Continue this thread (N replies)" preview of how much
-- thread is hidden behind the link. The main view is fed by the threaded query
-- and the thread modal re-fetches each level through this query; when the two
-- disagreed, a linear thread (one reply per comment) showed "(1 reply)" in the
-- modal no matter how many comments were actually below.
WITH RECURSIVE descendants AS (
    -- Seed with the direct children we're about to return...
    SELECT id AS root_id, id AS descendant_id
    FROM messages
    WHERE parent_id = $1
      AND message_type = 'comment'

    UNION ALL

    -- ...then walk down the whole subtree under each of them.
    SELECT d.root_id, m.id
    FROM descendants d
    JOIN messages m ON m.parent_id = d.descendant_id
    WHERE m.message_type = 'comment'
),
descendant_counts AS (
    -- COUNT(*) - 1 drops the seed row (the comment counting itself).
    SELECT root_id, COUNT(*) - 1 AS reply_count
    FROM descendants
    GROUP BY root_id
)
SELECT m.*,
       u.username as author_username,
       c.name as character_name,
       COALESCE(m.character_avatar_url_at_post, c.avatar_url) as character_avatar_url,
       COALESCE(dc.reply_count, 0)::bigint as reply_count
FROM messages m
JOIN users u ON m.author_id = u.id
LEFT JOIN characters c ON m.character_id = c.id
LEFT JOIN descendant_counts dc ON dc.root_id = m.id
WHERE m.parent_id = $1
  AND m.message_type = 'comment'
ORDER BY m.created_at DESC;

-- NOTE: GetCommentThread with recursive CTE is not supported by sqlc
-- Use GetPostComments recursively on the frontend to build the tree
-- This is actually more efficient for large thread trees anyway
--
-- UPDATE: GetPostCommentsWithThreads uses raw SQL in service layer
-- sqlc does not support recursive CTEs, so we implement this query directly in Go
-- See: backend/pkg/db/services/messages/comments.go

-- name: CountTopLevelComments :one
-- Count total top-level comments for a post (for pagination)
-- INCLUDES deleted comments to preserve thread structure
SELECT COUNT(*) as total
FROM messages
WHERE parent_id = $1
  AND message_type = 'comment';

-- name: UpdateComment :one
UPDATE messages
SET content = $2,
    character_id = COALESCE($3, character_id),
    mentioned_character_ids = $4,
    is_edited = true,
    edited_at = NOW(),
    edit_count = edit_count + 1
WHERE id = $1
  AND deleted_at IS NULL
  AND message_type = 'comment'
RETURNING *;

-- name: DeleteComment :exec
UPDATE messages
SET deleted_at = NOW(),
    deleted_by_user_id = $2,
    is_deleted = true
WHERE id = $1
  AND deleted_at IS NULL
  AND message_type = 'comment';

-- name: CheckCommentOwnership :one
SELECT author_id, deleted_at
FROM messages
WHERE id = $1 AND message_type = 'comment';

-- name: CheckPostOwnership :one
SELECT author_id, deleted_at
FROM messages
WHERE id = $1 AND message_type = 'post';

-- ============================================================================
-- STATISTICS & COUNTS
-- ============================================================================

-- name: GetGamePostCount :one
SELECT COUNT(*)
FROM messages
WHERE game_id = $1
  AND message_type = 'post'
  AND is_deleted = false
  AND is_draft = false
  AND (CASE WHEN $2 = 0 THEN TRUE ELSE phase_id = $2 END);

-- name: GetPostCommentCount :one
SELECT COUNT(*)
FROM messages
WHERE parent_id = $1
  AND is_deleted = false;

-- name: GetAllDescendantComments :many
-- Get all descendant comments recursively for counting
-- Includes deleted comments to preserve thread structure counts
WITH RECURSIVE comment_tree AS (
  SELECT messages.id as comment_id
  FROM messages
  WHERE messages.parent_id = $1

  UNION ALL

  SELECT m.id as comment_id
  FROM messages m
  INNER JOIN comment_tree ct ON m.parent_id = ct.comment_id
)
SELECT comment_id as id FROM comment_tree;

-- name: GetUserPostsInGame :many
SELECT m.*,
       u.username as author_username,
       c.name as character_name,
       COALESCE(m.character_avatar_url_at_post, c.avatar_url) as character_avatar_url,
       (SELECT COUNT(*) FROM messages WHERE parent_id = m.id) as comment_count
FROM messages m
JOIN users u ON m.author_id = u.id
LEFT JOIN characters c ON m.character_id = c.id
WHERE m.game_id = $1
  AND m.author_id = $2
  AND m.message_type = 'post'
  AND m.is_deleted = false
  AND m.is_draft = false
ORDER BY m.created_at DESC;

-- ============================================================================
-- REACTIONS (Optional - for future use)
-- ============================================================================

-- name: AddReaction :one
INSERT INTO message_reactions (message_id, user_id, reaction_type)
VALUES ($1, $2, $3)
ON CONFLICT (message_id, user_id, reaction_type) DO NOTHING
RETURNING *;

-- name: RemoveReaction :exec
DELETE FROM message_reactions
WHERE message_id = $1 AND user_id = $2 AND reaction_type = $3;

-- name: GetMessageReactions :many
SELECT mr.*, u.username
FROM message_reactions mr
JOIN users u ON mr.user_id = u.id
WHERE mr.message_id = $1
ORDER BY mr.created_at;

-- name: GetReactionCounts :many
SELECT reaction_type, COUNT(*) as count
FROM message_reactions
WHERE message_id = $1
GROUP BY reaction_type;

-- ============================================================================
-- READ TRACKING (Common Room)
-- ============================================================================

-- name: MarkPostRead :one
-- Mark a post (and optionally a specific comment) as read by a user
-- This is an upsert - creates new record or updates existing one
INSERT INTO user_common_room_reads (
    user_id,
    game_id,
    post_id,
    last_read_comment_id,
    last_read_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, NOW(), NOW()
)
ON CONFLICT (user_id, post_id)
DO UPDATE SET
    last_read_comment_id = EXCLUDED.last_read_comment_id,
    last_read_at = NOW(),
    updated_at = NOW()
RETURNING *;

-- name: GetUserReadMarker :one
-- Get the read tracking info for a specific user and post
SELECT * FROM user_common_room_reads
WHERE user_id = $1 AND post_id = $2;

-- name: GetUserReadMarkersForGame :many
-- Get all read markers for a user in a specific game
-- Used to batch-check which posts have unread content
SELECT * FROM user_common_room_reads
WHERE user_id = $1 AND game_id = $2
ORDER BY last_read_at DESC;

-- name: GetPostsWithUnreadCount :many
-- Get posts with their total comment count and last comment timestamp
-- Frontend will compare these with read markers to determine unread status
SELECT
    m.id as post_id,
    m.created_at as post_created_at,
    COUNT(c.id) as total_comments,
    MAX(c.created_at) as latest_comment_at
FROM messages m
LEFT JOIN messages c ON c.parent_id = m.id AND c.is_deleted = false
WHERE m.game_id = $1
  AND m.message_type = 'post'
  AND m.is_deleted = false
  AND m.is_draft = false
GROUP BY m.id, m.created_at
ORDER BY m.created_at DESC;

-- name: DeleteReadMarkersForPost :exec
-- Delete all read markers for a post (e.g., when post is deleted)
DELETE FROM user_common_room_reads
WHERE post_id = $1;

-- name: DeleteReadMarkersForGame :exec
-- Delete all read markers for a game (e.g., when game is deleted or completed)
DELETE FROM user_common_room_reads
WHERE game_id = $1;

-- name: DeleteReadMarkersForUser :exec
-- Delete all read markers for a user (e.g., when user account is deleted)
DELETE FROM user_common_room_reads
WHERE user_id = $1;

-- name: GetUnreadCommentIDsForPosts :many
-- Get unread comment IDs for each post in a game for a specific user
-- Returns post_id and array of comment IDs that are "new since last visit"
-- A comment is new if:
--   1. It was created AFTER the user's last_read_at timestamp
--   2. It was NOT authored by the current user (users don't see their own comments as NEW)
-- NOTE: If user has never visited (ucr.last_read_at IS NULL), returns empty array
-- This prevents overwhelming users with "NEW" badges on their first visit
WITH RECURSIVE comment_threads AS (
    -- Base case: all top-level comments (direct children of posts)
    SELECT
        c.id as comment_id,
        c.parent_id as post_id,
        c.created_at,
        c.author_id
    FROM messages c
    WHERE c.parent_id IN (
        SELECT id FROM messages
        WHERE game_id = $2 AND message_type = 'post' AND is_deleted = false AND is_draft = false
    )
    AND c.is_deleted = false

    UNION ALL

    -- Recursive case: all nested replies
    SELECT
        m.id as comment_id,
        ct.post_id,
        m.created_at,
        m.author_id
    FROM messages m
    INNER JOIN comment_threads ct ON m.parent_id = ct.comment_id
    WHERE m.is_deleted = false
)
SELECT
    posts.id as post_id,
    COALESCE(
        array_agg(ct.comment_id ORDER BY ct.created_at DESC) FILTER (
            WHERE ucr.last_read_at IS NOT NULL
            AND ct.created_at > ucr.last_read_at
        ),
        '{}'::integer[]
    ) as unread_comment_ids
FROM messages posts
LEFT JOIN user_common_room_reads ucr ON ucr.post_id = posts.id AND ucr.user_id = $1
LEFT JOIN comment_threads ct ON ct.post_id = posts.id AND ct.author_id != $1
WHERE posts.game_id = $2
  AND posts.message_type = 'post'
  AND posts.is_deleted = false
GROUP BY posts.id
ORDER BY posts.created_at DESC;

-- ============================================================================
-- MANUAL READ TRACKING (per-comment, user-controlled)
-- ============================================================================

-- name: MarkCommentRead :exec
-- Insert a manual read record; ignore if already exists (idempotent)
INSERT INTO user_comment_reads (user_id, comment_id, post_id, game_id)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id, comment_id) DO NOTHING;

-- name: UnmarkCommentRead :exec
-- Remove a manual read record
DELETE FROM user_comment_reads
WHERE user_id = $1 AND comment_id = $2;

-- name: GetManualReadCommentIDsForPost :many
-- Returns comment IDs explicitly marked as read by the user under a specific post
SELECT comment_id
FROM user_comment_reads
WHERE user_id = $1 AND post_id = $2;

-- name: GetManualReadCommentIDsForGame :many
-- Returns (post_id, comment_id) pairs for all manually read comments in a game
-- Used to batch-load read state for the entire common room view
SELECT post_id, comment_id
FROM user_comment_reads
WHERE user_id = $1 AND game_id = $2
ORDER BY post_id, comment_id;

-- name: DeleteManualCommentReadsForGame :exec
-- Delete all manual read records for a game (called when game is completed/archived)
DELETE FROM user_comment_reads
WHERE game_id = $1;

-- name: MarkAllCommentsReadForPhase :exec
-- Bulk-insert manual read records for every comment in a phase, for one user.
-- Comments can be nested arbitrarily deep (replies to replies), so each
-- comment's root post_id is resolved by walking up parent_id, mirroring the
-- comment_threads CTE used by GetUnreadCommentIDsForPosts.
-- Idempotent: existing records are left untouched.
WITH RECURSIVE comment_threads AS (
    SELECT
        c.id as comment_id,
        c.parent_id as post_id
    FROM messages c
    WHERE c.parent_id IN (
        SELECT p.id FROM messages p
        WHERE p.game_id = $2 AND p.phase_id = $3 AND p.message_type = 'post' AND p.is_deleted = false
    )
    AND c.is_deleted = false

    UNION ALL

    SELECT
        m.id as comment_id,
        ct.post_id
    FROM messages m
    INNER JOIN comment_threads ct ON m.parent_id = ct.comment_id
    WHERE m.is_deleted = false
)
INSERT INTO user_comment_reads (user_id, comment_id, post_id, game_id)
SELECT $1, ct.comment_id, ct.post_id, $2
FROM comment_threads ct
ON CONFLICT (user_id, comment_id) DO NOTHING;

-- ============================================================================
-- AUDIENCE PARTICIPATION (Private Message Access)
-- ============================================================================

-- name: ListAllPrivateConversations :many
-- List all private message conversations in a game (for audience/GM)
-- Returns all conversations with metadata, participant information, and last message preview
-- Supports pagination and filtering by participant names
WITH conversation_messages AS (
  SELECT
    c.id as conversation_id,
    c.title,
    c.conversation_type,
    c.created_at,
    COUNT(pm.id) as message_count,
    MAX(pm.created_at) as latest_message_at
  FROM conversations c
  LEFT JOIN private_messages pm ON c.id = pm.conversation_id
  WHERE c.game_id = $1
  GROUP BY c.id, c.title, c.conversation_type, c.created_at
),
participants_agg AS (
  -- Deduplicate by character_id first: an NPC shared by GM + co-GMs produces
  -- multiple conversation_participants rows for the same character, so we pick
  -- one row per (conversation_id, character_id) before aggregating names.
  SELECT
    cp.conversation_id,
    array_agg(COALESCE(ch.name, u.username) ORDER BY cp.id) as participant_names,
    array_agg(u.username ORDER BY cp.id) as participant_usernames,
    array_agg(cp.character_id ORDER BY cp.id) as participant_character_ids
  FROM (
    SELECT DISTINCT ON (conversation_id, character_id)
      id, conversation_id, user_id, character_id
    FROM conversation_participants
    ORDER BY conversation_id, character_id, id
  ) cp
  JOIN users u ON cp.user_id = u.id
  LEFT JOIN characters ch ON cp.character_id = ch.id
  GROUP BY cp.conversation_id
),
last_messages AS (
  -- Get the most recent message for each conversation with sender info
  SELECT DISTINCT ON (pm.conversation_id)
    pm.conversation_id,
    LEFT(pm.content, 150) as last_message_content,
    COALESCE(c.name, u.username) as last_sender_name,
    u.username as last_sender_username,
    pm.sender_character_id as last_sender_character_id
  FROM private_messages pm
  JOIN users u ON pm.sender_user_id = u.id
  LEFT JOIN characters c ON pm.sender_character_id = c.id
  WHERE pm.conversation_id IN (
    SELECT id FROM conversations WHERE game_id = $1
  )
  ORDER BY pm.conversation_id, pm.created_at DESC
)
SELECT
  cm.conversation_id,
  cm.title as subject,
  cm.conversation_type,
  cm.created_at,
  cm.message_count,
  cm.latest_message_at as last_message_at,
  pa.participant_names,
  pa.participant_usernames,
  pa.participant_character_ids,
  lm.last_message_content,
  lm.last_sender_name,
  lm.last_sender_username,
  lm.last_sender_character_id
FROM conversation_messages cm
LEFT JOIN participants_agg pa ON cm.conversation_id = pa.conversation_id
LEFT JOIN last_messages lm ON cm.conversation_id = lm.conversation_id
WHERE (
  -- If participant_names filter is provided (not empty array), filter by it
  -- Otherwise, show all conversations
  CASE
    WHEN sqlc.arg(participant_names)::text[] IS NULL OR array_length(sqlc.arg(participant_names)::text[], 1) IS NULL THEN true
    ELSE pa.participant_names::text[] @> sqlc.arg(participant_names)::text[]
  END
)
ORDER BY cm.latest_message_at DESC NULLS LAST
LIMIT sqlc.arg(result_limit)
OFFSET sqlc.arg(result_offset);

-- name: CountAllPrivateConversations :one
-- Count total private conversations in a game, with optional participant filter
-- Uses the same filter logic as ListAllPrivateConversations
WITH participants_agg AS (
  SELECT
    cp.conversation_id,
    array_agg(COALESCE(ch.name, u.username) ORDER BY cp.id) as participant_names
  FROM (
    SELECT DISTINCT ON (conversation_id, character_id)
      id, conversation_id, user_id, character_id
    FROM conversation_participants
    ORDER BY conversation_id, character_id, id
  ) cp
  JOIN users u ON cp.user_id = u.id
  LEFT JOIN characters ch ON cp.character_id = ch.id
  GROUP BY cp.conversation_id
)
SELECT COUNT(*)
FROM conversations c
LEFT JOIN participants_agg pa ON c.id = pa.conversation_id
WHERE c.game_id = sqlc.arg(game_id)
  AND (
    CASE
      WHEN sqlc.arg(participant_names)::text[] IS NULL OR array_length(sqlc.arg(participant_names)::text[], 1) IS NULL THEN true
      ELSE pa.participant_names::text[] @> sqlc.arg(participant_names)::text[]
    END
  );

-- name: GetAudienceConversationMessages :many
-- Get all messages in a specific conversation (for audience/GM)
SELECT pm.*,
       u.username as sender_username,
       c.name as sender_character_name
FROM private_messages pm
JOIN users u ON pm.sender_user_id = u.id
LEFT JOIN characters c ON pm.sender_character_id = c.id
WHERE pm.conversation_id = $1
ORDER BY pm.created_at ASC;

-- name: CountMessagesByCharacter :one
-- Count messages (posts and comments) by a specific character
-- Used to check if character can be deleted
SELECT COUNT(*)
FROM messages
WHERE character_id = $1
  AND is_deleted = false;

-- name: GetConversationParticipantNames :many
-- Get all character/user names that appear in at least one conversation in the game,
-- optionally narrowed to only those who share a conversation with ALL of the given names.
-- When selected_names is empty, returns every participant across all conversations.
-- When selected_names is non-empty, returns names that co-appear with all selected names.
WITH participants_per_conv AS (
  SELECT
    cp.conversation_id,
    array_agg(COALESCE(ch.name, u.username) ORDER BY cp.id) AS names
  FROM conversation_participants cp
  JOIN users u ON cp.user_id = u.id
  LEFT JOIN characters ch ON cp.character_id = ch.id
  JOIN conversations c ON cp.conversation_id = c.id
  WHERE c.game_id = $1
  GROUP BY cp.conversation_id
),
matching_convs AS (
  -- Conversations that contain ALL of the selected names (or all if none selected)
  SELECT conversation_id
  FROM participants_per_conv
  WHERE
    CASE
      WHEN sqlc.arg(selected_names)::text[] IS NULL
        OR array_length(sqlc.arg(selected_names)::text[], 1) IS NULL
      THEN true
      ELSE names::text[] @> sqlc.arg(selected_names)::text[]
    END
)
SELECT DISTINCT unnest(ppc.names) AS participant_name
FROM participants_per_conv ppc
JOIN matching_convs mc ON ppc.conversation_id = mc.conversation_id
ORDER BY participant_name;
