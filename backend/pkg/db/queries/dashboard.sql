-- Dashboard Queries
-- Comprehensive queries for user dashboard with game status, deadlines, and activity

-- name: GetUserDashboardGames :many
-- Get all games user participates in OR is GM of, with enriched metadata for dashboard
SELECT
  g.id,
  g.title,
  g.description,
  g.state,
  g.genre,
  g.gm_user_id,
  COALESCE(gp.role, 'gm') as user_role,
  COALESCE(gp.status, 'active') as participant_status,
  -- Current phase information
  current_phase.id as current_phase_id,
  current_phase.phase_type as current_phase_type,
  current_phase.title as current_phase_title,
  current_phase.deadline as current_phase_deadline,
  -- Game Master information
  gm_user.username as gm_username,
  -- Counts and metrics
  (SELECT COUNT(*)
   FROM game_applications
   WHERE game_id = g.id AND status = 'pending') as pending_applications_count,
  -- Active polls with no vote from this user
  (SELECT COUNT(*)
   FROM common_room_polls crp
   WHERE crp.game_id = g.id
     AND crp.deadline > NOW()
     AND crp.is_deleted = false
     AND NOT EXISTS (
       SELECT 1 FROM poll_votes pv
       WHERE pv.poll_id = crp.id AND pv.user_id = $1
     )) as unvoted_polls_count,
  -- Action submission status for current phase
  CASE
    WHEN current_phase.id IS NOT NULL AND current_phase.phase_type = 'action' THEN
      (SELECT CASE WHEN COUNT(*) = 0 THEN true
                   WHEN bool_or(is_draft = true) THEN true
                   ELSE false END
       FROM action_submissions
       WHERE game_id = g.id
         AND user_id = $1
         AND phase_id = current_phase.id)
    ELSE false
  END as has_pending_action,
  g.updated_at,
  g.created_at
FROM games g
LEFT JOIN game_participants gp ON g.id = gp.game_id AND gp.user_id = $1 AND gp.status = 'active'
LEFT JOIN game_phases current_phase ON g.id = current_phase.game_id AND current_phase.is_active = true
LEFT JOIN users gm_user ON g.gm_user_id = gm_user.id
WHERE ((gp.user_id = $1 AND gp.status = 'active') OR g.gm_user_id = $1)
  AND g.state != 'completed'
ORDER BY
  -- Urgent games first: action phases with pending submissions and near deadlines
  CASE
    WHEN current_phase.phase_type = 'action'
         AND current_phase.deadline IS NOT NULL
         AND current_phase.deadline < NOW() + INTERVAL '24 hours'
         AND (SELECT CASE WHEN COUNT(*) = 0 THEN true
                          WHEN bool_or(is_draft = true) THEN true
                          ELSE false END
              FROM action_submissions
              WHERE game_id = g.id
                AND user_id = $1
                AND phase_id = current_phase.id) THEN 0
    ELSE 1
  END,
  -- Then by approaching deadlines
  current_phase.deadline ASC NULLS LAST,
  -- Then by unread activity
  (SELECT COUNT(*)
   FROM notifications n
   WHERE n.game_id = g.id AND n.user_id = $1 AND n.is_read = false) DESC,
  -- Finally by recently updated
  g.updated_at DESC
LIMIT 15;

-- name: GetUserRecentMessages :many
-- Get recent messages from games user participates in OR is GM of (excluding their own messages)
SELECT
  m.id as message_id,
  m.game_id,
  m.content,
  m.created_at,
  g.title as game_title,
  g.is_anonymous,
  COALESCE(gp.role, 'gm') as viewer_role,
  author.username as author_name,
  character.name as character_name,
  m.message_type,
  m.phase_id
FROM messages m
INNER JOIN games g ON m.game_id = g.id
LEFT JOIN game_participants gp ON g.id = gp.game_id AND gp.user_id = $1 AND gp.status = 'active'
INNER JOIN users author ON m.author_id = author.id
LEFT JOIN characters character ON m.character_id = character.id
WHERE ((gp.user_id = $1 AND gp.status = 'active' AND gp.role != 'audience') OR g.gm_user_id = $1)
  AND m.created_at > NOW() - INTERVAL '7 days'
  AND m.author_id != $1
  AND m.is_deleted = false
  AND m.is_draft = false
ORDER BY m.created_at DESC
LIMIT $2;

-- name: GetUserUpcomingDeadlines :many
-- Get upcoming deadlines across all user's games: phase, arbitrary, and poll deadlines.
-- Excludes audience-only participants (they don't have actionable deadlines).

-- Phase deadlines
SELECT
  'phase' as deadline_type,
  gp.id as source_id,
  gp.id as phase_id,
  g.id as game_id,
  g.title as game_title,
  gp.title as title,
  gp.phase_type,
  gp.title as phase_title,
  gp.phase_number,
  gp.deadline as end_time,
  CASE
    WHEN gp.phase_type = 'action' AND part.user_id IS NOT NULL THEN
      (SELECT CASE WHEN COUNT(*) = 0 THEN true
                   WHEN bool_or(is_draft = true) THEN true
                   ELSE false END
       FROM action_submissions acts
       WHERE acts.game_id = g.id
         AND acts.user_id = $1
         AND acts.phase_id = gp.id)
    ELSE false
  END as has_pending_submission
FROM game_phases gp
INNER JOIN games g ON gp.game_id = g.id
LEFT JOIN game_participants part ON g.id = part.game_id AND part.user_id = $1 AND part.status = 'active'
WHERE ((part.user_id = $1 AND part.status = 'active' AND part.role != 'audience') OR g.gm_user_id = $1)
  AND gp.is_active = true
  AND gp.deadline IS NOT NULL
  AND gp.deadline > NOW()

UNION ALL

-- Arbitrary deadlines (GM-created)
SELECT
  'deadline' as deadline_type,
  gd.id as source_id,
  0 as phase_id,
  g.id as game_id,
  g.title as game_title,
  gd.title as title,
  '' as phase_type,
  '' as phase_title,
  0 as phase_number,
  gd.deadline as end_time,
  false as has_pending_submission
FROM game_deadlines gd
INNER JOIN games g ON gd.game_id = g.id
LEFT JOIN game_participants part ON g.id = part.game_id AND part.user_id = $1 AND part.status = 'active'
WHERE ((part.user_id = $1 AND part.status = 'active' AND part.role != 'audience') OR g.gm_user_id = $1)
  AND gd.deleted_at IS NULL
  AND gd.deadline > NOW()

UNION ALL

-- Poll deadlines
SELECT
  'poll' as deadline_type,
  crp.id as source_id,
  0 as phase_id,
  g.id as game_id,
  g.title as game_title,
  crp.question as title,
  '' as phase_type,
  '' as phase_title,
  0 as phase_number,
  crp.deadline as end_time,
  false as has_pending_submission
FROM common_room_polls crp
INNER JOIN games g ON crp.game_id = g.id
LEFT JOIN game_participants part ON g.id = part.game_id AND part.user_id = $1 AND part.status = 'active'
WHERE ((part.user_id = $1 AND part.status = 'active' AND part.role != 'audience') OR g.gm_user_id = $1)
  AND crp.is_deleted = false
  AND crp.deadline IS NOT NULL
  AND crp.deadline > NOW()

ORDER BY end_time ASC
LIMIT $2;

-- name: CountUserGames :one
-- Count total games user participates in OR is GM of
SELECT COUNT(DISTINCT g.id) as game_count
FROM games g
LEFT JOIN game_participants gp ON g.id = gp.game_id AND gp.user_id = $1 AND gp.status = 'active'
WHERE (gp.user_id = $1 AND gp.status = 'active') OR g.gm_user_id = $1;

-- GetUnreadCommentCountsForDashboard is implemented as a raw query in dashboard.go
-- due to sqlc limitations with recursive CTEs (same pattern as GetPostCommentsWithThreads).
-- See getUnreadCommentCountsForDashboard() in backend/pkg/db/services/dashboard.go.

-- name: GetDashboardUnreadCount :one
-- Get count of all unread notifications for user (dashboard-specific)
SELECT COUNT(*) as count
FROM notifications
WHERE user_id = $1 AND is_read = false;

-- name: GetUserUnreadNotificationsByType :many
-- Get unread notification counts grouped by type for the dashboard digest
SELECT type, COUNT(*) as count
FROM notifications
WHERE user_id = $1 AND is_read = false
GROUP BY type
ORDER BY count DESC;
