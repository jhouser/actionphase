-- name: CreateGameApplication :one
INSERT INTO game_applications (
    game_id, user_id, role, message
) VALUES (
    $1, $2, $3, $4
) RETURNING id, game_id, user_id, role, message, status, reviewed_by_user_id, reviewed_at, applied_at, is_published;

-- name: GetGameApplication :one
SELECT id, game_id, user_id, role, message, status, reviewed_by_user_id, reviewed_at, applied_at, is_published
FROM game_applications WHERE id = $1;

-- name: GetGameApplicationByUserAndGame :one
SELECT id, game_id, user_id, role, message, status, reviewed_by_user_id, reviewed_at, applied_at, is_published
FROM game_applications
WHERE game_id = $1 AND user_id = $2;

-- name: GetGameApplications :many
SELECT
    ga.id, ga.game_id, ga.user_id, ga.role, ga.message, ga.status, ga.reviewed_by_user_id, ga.reviewed_at, ga.applied_at, ga.is_published,
    u.username,
    u.avatar_url
FROM game_applications ga
JOIN users u ON ga.user_id = u.id
WHERE ga.game_id = $1
ORDER BY ga.applied_at ASC;

-- name: GetGameApplicationsByStatus :many
SELECT
    ga.id, ga.game_id, ga.user_id, ga.role, ga.message, ga.status, ga.reviewed_by_user_id, ga.reviewed_at, ga.applied_at, ga.is_published,
    u.username,
    u.avatar_url
FROM game_applications ga
JOIN users u ON ga.user_id = u.id
WHERE ga.game_id = $1 AND ga.status = $2
ORDER BY ga.applied_at ASC;

-- name: GetUserGameApplications :many
SELECT
    ga.id, ga.game_id, ga.user_id, ga.role, ga.message, ga.status, ga.reviewed_by_user_id, ga.reviewed_at, ga.applied_at, ga.is_published,
    g.title AS game_title,
    g.state AS game_state
FROM game_applications ga
JOIN games g ON ga.game_id = g.id
WHERE ga.user_id = $1
ORDER BY ga.applied_at DESC;

-- name: UpdateGameApplicationStatus :one
UPDATE game_applications
SET
    status = $2,
    reviewed_at = NOW(),
    reviewed_by_user_id = $3
WHERE id = $1
RETURNING id, game_id, user_id, role, message, status, reviewed_by_user_id, reviewed_at, applied_at, is_published;

-- name: DeleteGameApplication :exec
DELETE FROM game_applications WHERE id = $1 AND user_id = $2;

-- name: DeleteRejectedApplicationForUser :exec
DELETE FROM game_applications WHERE game_id = $1 AND user_id = $2 AND status = 'rejected';

-- name: DeleteStaleApprovedApplicationForUser :exec
-- Deletes a leftover 'approved' audience application for a user who is no longer an
-- active participant in the game (e.g. they left or were removed after approval).
-- Scoped to role = 'audience' because audience approval creates a game_participants row
-- immediately (see ApproveGameApplication), unlike player applications, whose approved
-- application row is intentionally kept until recruitment closes. Without this, the
-- UNIQUE(game_id, user_id) constraint permanently blocks an audience member from
-- re-applying after leaving.
DELETE FROM game_applications ga
WHERE ga.game_id = $1 AND ga.user_id = $2 AND ga.status = 'approved' AND ga.role = 'audience'
  AND NOT EXISTS (
    SELECT 1 FROM game_participants gp
    WHERE gp.game_id = ga.game_id AND gp.user_id = ga.user_id AND gp.status = 'active'
  );

-- name: CountPendingApplicationsForGame :one
SELECT COUNT(*) FROM game_applications
WHERE game_id = $1 AND status = 'pending';

-- name: GetApprovedApplicationsForGame :many
SELECT
    ga.id, ga.game_id, ga.user_id, ga.role, ga.message, ga.status, ga.reviewed_by_user_id, ga.reviewed_at, ga.applied_at, ga.is_published,
    u.username,
    u.avatar_url
FROM game_applications ga
JOIN users u ON ga.user_id = u.id
WHERE ga.game_id = $1 AND ga.status = 'approved'
ORDER BY ga.reviewed_at ASC;

-- name: BulkApproveApplications :exec
UPDATE game_applications
SET
    status = 'approved',
    reviewed_at = NOW(),
    reviewed_by_user_id = $2
WHERE game_id = $1 AND status = 'pending';

-- name: BulkRejectApplications :exec
-- Reject all pending applications for a game
-- This is called when GM closes recruitment
UPDATE game_applications
SET
    status = 'rejected',
    reviewed_at = NOW(),
    reviewed_by_user_id = $2
WHERE game_id = $1 AND status = 'pending';

-- name: HasUserAppliedToGame :one
SELECT EXISTS(
    SELECT 1 FROM game_applications
    WHERE game_id = $1 AND user_id = $2
);

-- name: CanUserApplyToGame :one
SELECT CASE
    WHEN EXISTS(SELECT 1 FROM games g WHERE g.id = sqlc.arg('game_id') AND g.gm_user_id = sqlc.arg('user_id')) THEN 'is_game_master'
    WHEN EXISTS(SELECT 1 FROM game_participants gp WHERE gp.game_id = sqlc.arg('game_id') AND gp.user_id = sqlc.arg('user_id') AND gp.status = 'active') THEN 'already_participant'
    WHEN EXISTS(SELECT 1 FROM game_applications ga WHERE ga.game_id = sqlc.arg('game_id') AND ga.user_id = sqlc.arg('user_id') AND ga.status = 'pending') THEN 'application_pending'
    WHEN EXISTS(SELECT 1 FROM game_applications ga2 WHERE ga2.game_id = sqlc.arg('game_id') AND ga2.user_id = sqlc.arg('user_id') AND ga2.status = 'rejected')
         AND NOT EXISTS(SELECT 1 FROM game_participants gp2 WHERE gp2.game_id = sqlc.arg('game_id') AND gp2.user_id = sqlc.arg('user_id')) THEN 'application_rejected'
    WHEN EXISTS(SELECT 1 FROM games g2 WHERE g2.id = sqlc.arg('game_id') AND g2.state != 'recruitment') THEN 'not_recruiting'
    ELSE 'can_apply'
END AS status;

-- name: PublishApplicationStatuses :exec
-- Mark all application statuses as published for a game
-- This is called when GM closes recruitment
UPDATE game_applications
SET is_published = TRUE
WHERE game_id = $1;

-- name: DeleteRejectedApplications :exec
-- Delete all rejected applications for a game
-- This is called when transitioning out of recruitment to clean up rejected applications
DELETE FROM game_applications
WHERE game_id = $1 AND status = 'rejected';

-- name: GetPublicGameApplicants :many
-- Public endpoint: Get list of applicants for a game (no approval/rejection status)
-- Available to anyone when game is in recruiting state
-- Returns only username and role, NOT status or review information
SELECT
    ga.id, ga.game_id, ga.user_id, ga.role, ga.applied_at,
    u.username,
    u.avatar_url
FROM game_applications ga
JOIN users u ON ga.user_id = u.id
WHERE ga.game_id = $1
ORDER BY ga.applied_at ASC;
