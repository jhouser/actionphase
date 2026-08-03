-- ================================================
-- COMMON ROOM POLLS
-- ================================================

-- name: CreatePoll :one
INSERT INTO common_room_polls (
    game_id,
    phase_id,
    created_by_user_id,
    created_by_character_id,
    question,
    description,
    deadline,
    show_individual_votes,
    allow_other_option,
    hide_results_from_players,
    allow_audience_voting,
    show_running_totals_to_players
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: GetPoll :one
SELECT * FROM common_room_polls
WHERE id = $1 AND is_deleted = FALSE;

-- name: ListPollsByPhase :many
SELECT * FROM common_room_polls
WHERE game_id = $1
  AND phase_id = $2
  AND is_deleted = FALSE
ORDER BY created_at DESC;

-- name: ListPollsByGame :many
SELECT * FROM common_room_polls
WHERE game_id = $1
  AND is_deleted = FALSE
  AND (
    -- Include expired polls if requested
    $2 = true OR deadline > NOW()
  )
ORDER BY deadline ASC;

-- name: SoftDeletePoll :exec
UPDATE common_room_polls
SET is_deleted = TRUE, updated_at = NOW()
WHERE id = $1;

-- name: UpdatePoll :one
UPDATE common_room_polls
SET question = $2,
    description = $3,
    deadline = $4,
    show_individual_votes = $5,
    allow_other_option = $6,
    hide_results_from_players = $7,
    allow_audience_voting = $8,
    show_running_totals_to_players = $9,
    updated_at = NOW()
WHERE id = $1 AND is_deleted = FALSE
RETURNING *;

-- ================================================
-- POLL OPTIONS
-- ================================================

-- name: CreatePollOption :one
INSERT INTO poll_options (poll_id, option_text, display_order)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetPollOptions :many
SELECT * FROM poll_options
WHERE poll_id = $1
ORDER BY display_order ASC;

-- name: DeletePollOption :exec
DELETE FROM poll_options
WHERE id = $1;

-- ================================================
-- POLL VOTES
-- ================================================

-- name: SubmitVote :one
INSERT INTO poll_votes (
    poll_id,
    user_id,
    selected_option_id,
    other_response
)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateVote :one
UPDATE poll_votes
SET selected_option_id = $2,
    other_response = $3,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: GetVoteByPollAndUser :one
SELECT * FROM poll_votes
WHERE poll_id = $1
  AND user_id = $2;

-- name: DeleteVote :exec
DELETE FROM poll_votes
WHERE id = $1;

-- name: GetPollVoteCount :one
SELECT COUNT(*) as vote_count
FROM poll_votes
WHERE poll_id = $1;

-- name: GetOptionVoteCount :one
SELECT COUNT(*) as vote_count
FROM poll_votes
WHERE poll_id = $1
  AND selected_option_id = $2;

-- name: GetOtherVoteCount :one
SELECT COUNT(*) as vote_count
FROM poll_votes
WHERE poll_id = $1
  AND other_response IS NOT NULL;

-- ================================================
-- POLL RESULTS (with visibility logic)
-- ================================================

-- name: GetPollResultsSummary :many
-- Get vote counts per option for results display
SELECT
    po.id as option_id,
    po.option_text,
    po.display_order,
    COUNT(pv.id) as vote_count
FROM poll_options po
LEFT JOIN poll_votes pv ON po.id = pv.selected_option_id
WHERE po.poll_id = $1
GROUP BY po.id, po.option_text, po.display_order
ORDER BY po.display_order ASC;

-- name: GetPollVotesWithDetails :many
-- Get detailed vote information (for when show_individual_votes = true)
SELECT
    pv.id,
    pv.user_id,
    pv.selected_option_id,
    pv.other_response,
    pv.created_at,
    u.username,
    c.name as character_name,
    gp.role as voter_role
FROM poll_votes pv
JOIN users u ON pv.user_id = u.id
LEFT JOIN characters c ON c.id = (
    SELECT id FROM characters
    WHERE game_id = (SELECT game_id FROM common_room_polls WHERE id = pv.poll_id)
      AND user_id = pv.user_id
    LIMIT 1
)
LEFT JOIN game_participants gp
    ON gp.game_id = (SELECT game_id FROM common_room_polls WHERE id = pv.poll_id)
   AND gp.user_id = pv.user_id
   AND gp.status = 'active'
WHERE pv.poll_id = $1
ORDER BY pv.created_at ASC;

-- name: GetOtherResponses :many
-- Get all "other" text responses
SELECT
    pv.id,
    pv.user_id,
    pv.other_response,
    pv.created_at,
    u.username,
    c.name as character_name,
    gp.role as voter_role
FROM poll_votes pv
JOIN users u ON pv.user_id = u.id
LEFT JOIN characters c ON c.id = (
    SELECT id FROM characters
    WHERE game_id = (SELECT game_id FROM common_room_polls WHERE id = pv.poll_id)
      AND user_id = pv.user_id
    LIMIT 1
)
LEFT JOIN game_participants gp
    ON gp.game_id = (SELECT game_id FROM common_room_polls WHERE id = pv.poll_id)
   AND gp.user_id = pv.user_id
   AND gp.status = 'active'
WHERE pv.poll_id = $1
  AND pv.other_response IS NOT NULL
ORDER BY pv.created_at ASC;

-- ================================================
-- UTILITY QUERIES
-- ================================================

-- name: GetActivePollsForGame :many
-- Get all active (not expired, not deleted) polls for a game
SELECT * FROM common_room_polls
WHERE game_id = $1
  AND is_deleted = FALSE
  AND deadline > NOW()
ORDER BY deadline ASC;

-- name: GetExpiredPolls :many
-- For cleanup or notification jobs
SELECT * FROM common_room_polls
WHERE is_deleted = FALSE
  AND deadline <= NOW()
ORDER BY deadline DESC;

-- name: HasUserVoted :one
-- Check if user has already voted in a poll
SELECT EXISTS(
    SELECT 1 FROM poll_votes
    WHERE poll_id = @poll_id
      AND user_id = @user_id
) as has_voted;
