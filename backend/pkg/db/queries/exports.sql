-- =============================================================================
-- GAME EXPORT QUERIES
-- =============================================================================
-- Async archive exports for completed games. See pkg/db/services/exports/.
--
-- Content queries here intentionally return EVERYTHING a completed game holds:
-- completed games are a public archive readable by any authenticated user
-- (CanUserViewGame), so there is no per-viewer filtering. The only exclusions
-- are draft posts (messages.is_draft) and soft-deleted rows (is_deleted /
-- deleted_at), which were never visible to anyone but their author. Action
-- submissions have no draft state — they are exported in full.

-- =============================================================================
-- JOB LIFECYCLE
-- =============================================================================

-- name: CreateGameExport :one
-- Enqueue an export. The partial unique index idx_game_exports_one_active
-- makes this fail if an export for this game is already pending/running,
-- which is how concurrent requests coalesce onto a single job.
INSERT INTO game_exports (game_id, requested_by_user_id, status)
VALUES ($1, $2, 'pending')
RETURNING *;

-- name: GetGameExport :one
-- Joins the game title so the download handler can name the file after the
-- game rather than its numeric export id.
SELECT ge.*, g.title AS game_title
FROM game_exports ge
JOIN games g ON g.id = ge.game_id
WHERE ge.id = $1;

-- name: GetLatestGameExport :one
-- Newest export for a game in any state, for status polling.
SELECT * FROM game_exports
WHERE game_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: GetReusableGameExport :one
-- Newest completed export that is still usable: artifact present, not expired,
-- and fingerprint still matching current content.
--
-- Completed games are effectively immutable, so the fingerprint should always
-- match; it is kept as a safety check rather than a cache key. If it ever
-- mismatches (a future GM tool, an admin action), the archive is rebuilt
-- instead of serving stale content.
SELECT * FROM game_exports
WHERE game_id = $1
  AND status = 'complete'
  AND storage_path IS NOT NULL
  AND (expires_at IS NULL OR expires_at > NOW())
  AND content_fingerprint = $2
ORDER BY completed_at DESC
LIMIT 1;

-- name: ListExpiredGameExports :many
-- Complete exports whose retention window has passed but whose artifact is
-- still in storage. The worker deletes each artifact, then calls
-- MarkGameExportExpired to clear the path.
SELECT id, game_id, storage_path
FROM game_exports
WHERE status = 'complete'
  AND storage_path IS NOT NULL
  AND expires_at IS NOT NULL
  AND expires_at <= NOW()
ORDER BY expires_at
LIMIT sqlc.arg(result_limit);

-- name: MarkGameExportExpired :exec
-- Clears the artifact pointer after the file has been deleted from storage.
-- The row is kept so export history survives; size/file counts stay for
-- reference. Regeneration happens on the next request.
UPDATE game_exports
SET storage_path = NULL
WHERE id = $1;

-- name: GetActiveGameExport :one
-- The in-flight export for a game, if any.
SELECT * FROM game_exports
WHERE game_id = $1 AND status IN ('pending', 'running')
LIMIT 1;

-- name: ClaimNextGameExport :one
-- Atomically claim the oldest pending job. SKIP LOCKED keeps this correct
-- when several backend replicas poll concurrently.
UPDATE game_exports
SET status = 'running', started_at = NOW()
WHERE id = (
    SELECT id FROM game_exports
    WHERE status = 'pending'
    ORDER BY created_at
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
RETURNING *;

-- name: MarkGameExportProgress :exec
UPDATE game_exports SET progress_note = $2 WHERE id = $1;

-- name: MarkGameExportComplete :one
-- retention_period sets how long the artifact is kept before the sweep
-- reclaims it; the row itself is retained as history.
UPDATE game_exports
SET status = 'complete',
    storage_path = $2,
    size_bytes = $3,
    file_count = $4,
    content_fingerprint = $5,
    progress_note = NULL,
    completed_at = NOW(),
    expires_at = NOW() + sqlc.arg(retention_period)::interval
WHERE id = $1
RETURNING *;

-- name: MarkGameExportFailed :one
UPDATE game_exports
SET status = 'failed',
    error_message = $2,
    progress_note = NULL,
    completed_at = NOW()
WHERE id = $1
RETURNING *;

-- name: RequeueStalledGameExports :execrows
-- Startup crash recovery: return jobs left 'running' by a dead process to
-- 'pending'. Bounded by age so a job actively running elsewhere is untouched.
UPDATE game_exports
SET status = 'pending', started_at = NULL, progress_note = NULL
WHERE status = 'running'
  AND started_at < NOW() - sqlc.arg(stale_after)::interval;

-- =============================================================================
-- CONTENT FINGERPRINT
-- =============================================================================

-- name: GetGameContentFingerprint :one
-- Cheap change-detection across every table the archive draws from. Completed
-- games are mostly immutable, but a GM can still edit after completion, so a
-- cached artifact is only reusable while this value is unchanged.
SELECT
    (SELECT COUNT(*) FROM messages m WHERE m.game_id = $1)::bigint AS message_count,
    (SELECT MAX(GREATEST(m.created_at, COALESCE(m.edited_at::timestamp, m.created_at)))
       FROM messages m WHERE m.game_id = $1)::timestamptz AS message_max_ts,
    (SELECT COUNT(*) FROM action_submissions s WHERE s.game_id = $1)::bigint AS submission_count,
    (SELECT MAX(s.updated_at) FROM action_submissions s WHERE s.game_id = $1)::timestamptz AS submission_max_ts,
    (SELECT COUNT(*) FROM action_results r WHERE r.game_id = $1)::bigint AS result_count,
    (SELECT MAX(r.sent_at) FROM action_results r WHERE r.game_id = $1)::timestamptz AS result_max_ts,
    (SELECT COUNT(*) FROM private_messages pm
       JOIN conversations pc ON pm.conversation_id = pc.id
      WHERE pc.game_id = $1)::bigint AS private_message_count,
    (SELECT MAX(pm.updated_at) FROM private_messages pm
       JOIN conversations pc ON pm.conversation_id = pc.id
      WHERE pc.game_id = $1)::timestamptz AS private_message_max_ts,
    (SELECT COUNT(*) FROM game_phases ph WHERE ph.game_id = $1)::bigint AS phase_count,
    (SELECT COUNT(*) FROM characters ch WHERE ch.game_id = $1)::bigint AS character_count,
    (SELECT MAX(ch.updated_at) FROM characters ch WHERE ch.game_id = $1)::timestamptz AS character_max_ts,
    (SELECT COUNT(*) FROM handouts h WHERE h.game_id = $1)::bigint AS handout_count,
    (SELECT MAX(h.updated_at) FROM handouts h WHERE h.game_id = $1)::timestamptz AS handout_max_ts,
    (SELECT COUNT(*) FROM common_room_polls p WHERE p.game_id = $1 AND p.is_deleted = FALSE)::bigint AS poll_count,
    (SELECT MAX(p.updated_at) FROM common_room_polls p WHERE p.game_id = $1)::timestamptz AS poll_max_ts,
    (SELECT g.updated_at FROM games g WHERE g.id = $1)::timestamptz AS game_updated_at;

-- =============================================================================
-- ARCHIVE CONTENT
-- =============================================================================

-- name: ListExportPhases :many
-- Every phase that actually ran, in order.
--
-- Do NOT filter on is_published: that column means "GM has published this
-- ACTION phase's results" and is documented as always FALSE for common_room
-- and interlude phases (migration 20251015001708). Filtering on it drops every
-- discussion phase and misfiles their content as unfiled.
--
-- All phases are exported, including any never activated: a phase row is part
-- of the game's structure, an empty phase directory is self-describing, and
-- filtering risks orphaning content into "unfiled" the way is_published did.
SELECT id, phase_type, phase_number, title, description,
       start_time, end_time, deadline, activated_at, created_at
FROM game_phases
WHERE game_id = $1
ORDER BY phase_number;

-- name: ListExportCharacters :many
-- Roster. avatar_url is deliberately NOT selected: archives are text-only.
SELECT c.id, c.name, c.character_type, c.status, c.is_active,
       c.created_at, u.username AS player_username
FROM characters c
LEFT JOIN users u ON c.user_id = u.id
WHERE c.game_id = $1
ORDER BY c.character_type, c.name;

-- name: ListExportCharacterData :many
-- Character sheet fields for the whole game, grouped by character in Go.
SELECT cd.character_id, cd.module_type, cd.field_name, cd.field_value,
       cd.field_type, cd.is_public
FROM character_data cd
JOIN characters c ON cd.character_id = c.id
WHERE c.game_id = $1
ORDER BY cd.character_id, cd.module_type, cd.field_name;

-- name: ListExportPosts :many
-- Top-level common room posts. Comments are fetched per-post as a tree.
SELECT m.id, m.phase_id, m.content, m.created_at, m.is_edited, m.edit_count,
       m.mentioned_character_ids,
       c.name AS character_name, u.username AS author_username
FROM messages m
JOIN characters c ON m.character_id = c.id
JOIN users u ON m.author_id = u.id
WHERE m.game_id = $1
  AND m.message_type = 'post'
  AND m.visibility = 'game'
  AND m.is_draft = FALSE
  AND m.is_deleted = FALSE
ORDER BY m.created_at;

-- name: ListExportCommentTree :many
-- Entire descendant tree of one post in a single recursive pass, ordered by
-- materialized path so parents always precede their children. Avoids the
-- N+1-per-level walk that deep threads (100+ replies) would otherwise cause.
WITH RECURSIVE tree AS (
    SELECT m.id, m.parent_id, m.character_id, m.author_id, m.content,
           m.created_at, m.is_edited, m.edit_count, m.thread_depth,
           ARRAY[m.created_at]::timestamp[] AS path
    FROM messages m
    WHERE m.parent_id = $1
      AND m.is_draft = FALSE
      AND m.is_deleted = FALSE
    UNION ALL
    SELECT child.id, child.parent_id, child.character_id, child.author_id,
           child.content, child.created_at, child.is_edited, child.edit_count,
           child.thread_depth, t.path || child.created_at
    FROM messages child
    JOIN tree t ON child.parent_id = t.id
    WHERE child.is_draft = FALSE
      AND child.is_deleted = FALSE
)
SELECT t.id, t.parent_id, t.content, t.created_at, t.is_edited, t.edit_count,
       t.thread_depth, t.path,
       c.name AS character_name, u.username AS author_username
FROM tree t
JOIN characters c ON t.character_id = c.id
JOIN users u ON t.author_id = u.id
ORDER BY t.path;

-- name: ListExportConversations :many
-- All private conversations in the game. No participant filter: the audience
-- view already exposes these, and a completed game is a public archive.
SELECT c.id, c.title, c.conversation_type, c.created_at
FROM conversations c
WHERE c.game_id = $1
ORDER BY c.created_at;

-- name: ListExportConversationParticipants :many
SELECT cp.conversation_id, u.username, ch.name AS character_name
FROM conversation_participants cp
JOIN conversations c ON cp.conversation_id = c.id
JOIN users u ON cp.user_id = u.id
LEFT JOIN characters ch ON cp.character_id = ch.id
WHERE c.game_id = $1
ORDER BY cp.conversation_id, cp.id;

-- name: ListExportPrivateMessages :many
SELECT pm.id, pm.conversation_id, pm.content, pm.created_at,
       pm.is_edited, pm.edit_count,
       u.username AS sender_username, ch.name AS sender_character_name
FROM private_messages pm
JOIN conversations c ON pm.conversation_id = c.id
JOIN users u ON pm.sender_user_id = u.id
LEFT JOIN characters ch ON pm.sender_character_id = ch.id
WHERE c.game_id = $1 AND pm.is_deleted = FALSE
ORDER BY pm.conversation_id, pm.created_at;

-- name: ListExportActionSubmissions :many
-- Submitted actions only; drafts were never visible to anyone but their author.
-- user_id is selected so results can be paired by (phase_id, user_id): see
-- ListExportActionResults for why the FK alone is not reliable.
SELECT a.id, a.phase_id, a.user_id, a.content, a.submitted_at,
       u.username, ch.name AS character_name
FROM action_submissions a
JOIN users u ON a.user_id = u.id
LEFT JOIN characters ch ON a.character_id = ch.id
WHERE a.game_id = $1
ORDER BY a.phase_id, a.submitted_at;

-- name: ListExportActionResults :many
-- Published results only: unpublished ones never reached the player.
--
-- action_submission_id is frequently NULL in practice even when the matching
-- submission exists, so the archive pairs results to submissions by
-- (phase_id, user_id) — the tuple action_submissions enforces as unique via
-- UNIQUE(game_id, user_id, phase_id). user_id is selected for that join;
-- action_submission_id is kept as a preferred hint when it is set.
SELECT r.id, r.phase_id, r.user_id, r.action_submission_id, r.content, r.sent_at,
       u.username AS recipient_username, ch.name AS character_name,
       gm.username AS gm_username
FROM action_results r
JOIN users u ON r.user_id = u.id
JOIN users gm ON r.gm_user_id = gm.id
LEFT JOIN characters ch ON r.character_id = ch.id
WHERE r.game_id = $1 AND r.is_published = TRUE
ORDER BY r.phase_id, r.sent_at;

-- name: ListExportHandouts :many
SELECT id, title, content, status, created_at, updated_at
FROM handouts
WHERE game_id = $1 AND status = 'published'
ORDER BY created_at;

-- name: ListExportPolls :many
-- Polls flagged hide_results_from_players are EXCLUDED entirely. Completing a
-- game does not lift that flag: checkPollViewAccess grants completed-game
-- viewers canSeeIndividualVotes but NOT isPrivileged, and the results handler
-- refuses hidden-results polls to anyone unprivileged
-- (backend/pkg/polls/api_polls.go:553). Only GM/co-GM/audience may ever see
-- them, so they must not appear in an archive any authenticated user can read.
SELECT p.id, p.phase_id, p.question, p.description, p.deadline,
       p.show_individual_votes, p.hide_results_from_players, p.created_at,
       u.username AS creator_username, ch.name AS creator_character_name
FROM common_room_polls p
JOIN users u ON p.created_by_user_id = u.id
LEFT JOIN characters ch ON p.created_by_character_id = ch.id
WHERE p.game_id = $1
  AND p.is_deleted = FALSE
  AND p.hide_results_from_players = FALSE
ORDER BY p.created_at;

-- name: ListExportPollOptions :many
-- Mirrors the hide_results_from_players exclusion in ListExportPolls.
SELECT o.id, o.poll_id, o.option_text, o.display_order
FROM poll_options o
JOIN common_room_polls p ON o.poll_id = p.id
WHERE p.game_id = $1
  AND p.is_deleted = FALSE
  AND p.hide_results_from_players = FALSE
ORDER BY o.poll_id, o.display_order;

-- name: ListExportPollVotes :many
-- Vote tallies WITH voter identity: in a completed game every authenticated
-- user sees individual votes regardless of the poll's show_individual_votes
-- setting (checkPollViewAccess, backend/pkg/polls/api_polls.go:241). The
-- archive therefore matches the completed-game web view.
--
-- Hidden-results polls are excluded, matching ListExportPolls.
SELECT v.poll_id, v.selected_option_id, v.other_response, v.created_at,
       u.username AS voter_username
FROM poll_votes v
JOIN common_room_polls p ON v.poll_id = p.id
JOIN users u ON v.user_id = u.id
WHERE p.game_id = $1
  AND p.is_deleted = FALSE
  AND p.hide_results_from_players = FALSE
ORDER BY v.poll_id, v.created_at;

-- name: ListExportParticipants :many
-- Roster for the archive README.
--
-- The primary GM is games.gm_user_id and is NOT guaranteed to have a
-- game_participants row, so a plain join silently omits them. Union the GM in
-- explicitly, and de-duplicate in case they also hold a participant row.
SELECT user_id, role, status, joined_at, is_former_player, username
FROM (
    SELECT gp.user_id, gp.role, gp.status, gp.joined_at, gp.is_former_player,
           u.username,
           2 AS sort_rank
    FROM game_participants gp
    JOIN users u ON gp.user_id = u.id
    WHERE gp.game_id = $1
      AND gp.user_id <> (SELECT g.gm_user_id FROM games g WHERE g.id = $1)

    UNION ALL

    SELECT g.gm_user_id AS user_id,
           'gm' AS role,
           'active'::varchar AS status,
           g.created_at AS joined_at,
           FALSE AS is_former_player,
           u.username,
           1 AS sort_rank
    FROM games g
    JOIN users u ON g.gm_user_id = u.id
    WHERE g.id = $1
) roster
ORDER BY sort_rank, role, username;
