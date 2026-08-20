-- =============================================================================
-- HANDOUT QUERIES
-- =============================================================================
-- Queries for managing GM handouts (informational documents)
-- Only GMs can create/update/delete handouts and comments
-- Players can view published handouts and comments

-- =============================================================================
-- HANDOUTS
-- =============================================================================

-- name: CreateHandout :one
INSERT INTO handouts (
    game_id,
    title,
    content,
    status
) VALUES (
    $1, $2, $3, $4
) RETURNING *;

-- name: GetHandout :one
SELECT * FROM handouts
WHERE id = $1;

-- name: ListHandoutsByGame :many
-- For GM: show all handouts (when $2 is TRUE)
-- For players: only show published handouts (when $2 is FALSE)
SELECT * FROM handouts
WHERE game_id = $1
  AND (status = 'published' OR $2 = TRUE)
ORDER BY created_at DESC;

-- name: UpdateHandout :one
UPDATE handouts
SET title = $1,
    content = $2,
    status = $3,
    updated_at = NOW()
WHERE id = $4
RETURNING *;

-- name: DeleteHandout :exec
DELETE FROM handouts WHERE id = $1;

-- name: PublishHandout :one
UPDATE handouts
SET status = 'published'
WHERE id = $1
RETURNING *;

-- name: UnpublishHandout :one
UPDATE handouts
SET status = 'draft'
WHERE id = $1
RETURNING *;

-- =============================================================================
-- HANDOUT COMMENTS
-- =============================================================================

-- name: CreateHandoutComment :one
INSERT INTO handout_comments (
    handout_id,
    user_id,
    parent_comment_id,
    content
) VALUES (
    $1, $2, $3, $4
) RETURNING *;

-- name: GetHandoutComment :one
SELECT
    hc.*,
    u.username as author_username
FROM handout_comments hc
JOIN users u ON hc.user_id = u.id
WHERE hc.id = $1;

-- name: ListHandoutComments :many
SELECT
    hc.*,
    u.username as author_username
FROM handout_comments hc
JOIN users u ON hc.user_id = u.id
WHERE hc.handout_id = $1
  AND hc.deleted_at IS NULL
ORDER BY hc.created_at ASC;

-- name: UpdateHandoutComment :one
UPDATE handout_comments
SET content = $1,
    edited_at = NOW(),
    edit_count = edit_count + 1,
    updated_at = NOW()
WHERE id = $2 AND deleted_at IS NULL
RETURNING *;

-- name: DeleteHandoutComment :exec
UPDATE handout_comments
SET deleted_at = NOW(),
    deleted_by_user_id = $2,
    updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL;

-- name: ListPublishedHandoutsAcrossGames :many
-- Cross-game handout list for a user, backing surfaces with no game in scope
-- (the global Utility Drawer). The per-game list is ListHandoutsByGame above.
--
-- Published only, for everyone including the GM. The drawer is a reading
-- surface: a GM's drafts are work in progress and stay on the Handouts tab,
-- which is where they can be edited. That also keeps this query free of any
-- per-row role — every handout it returns is one the requester may read, so the
-- handler has nothing further to filter.
--
-- Restricted to in_progress games, matching
-- GetUserControllableCharactersAcrossGames: this backs "what am I currently
-- playing", and a finished game's handouts stay reachable from the game itself.
--
-- Membership is GM, co-GM, or any non-removed participant. Audience members
-- participate through this same table, so they receive the published handouts of
-- games they follow, exactly as the in-game tab already shows them.
SELECT
  h.id, h.game_id, h.title, h.content, h.status, h.created_at, h.updated_at,
  g.title AS game_title
FROM handouts h
JOIN games g ON h.game_id = g.id
LEFT JOIN game_participants gp
  ON gp.game_id = g.id AND gp.user_id = $1 AND gp.removed_at IS NULL
WHERE g.state = 'in_progress'
  AND h.status = 'published'
  AND (g.gm_user_id = $1 OR gp.user_id IS NOT NULL)
ORDER BY game_title, g.id, h.created_at DESC;
