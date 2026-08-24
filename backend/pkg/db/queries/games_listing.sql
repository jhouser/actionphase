-- name: GetFilteredGames :many
-- Get games with filters, sorting, and user participation enrichment
SELECT
  g.id,
  g.title,
  g.description,
  g.gm_user_id,
  u.username as gm_username,
  g.state,
  g.genre,
  g.start_date,
  g.end_date,
  g.recruitment_deadline,
  g.max_players,
  g.is_public,
  g.is_anonymous,
  g.auto_accept_audience,
  g.allow_group_conversations,
  g.portrait_avatars,
  g.banner_url,
  g.created_at,
  g.updated_at,

  -- Computed fields
  (SELECT COUNT(*) FROM game_participants WHERE game_id = g.id AND status = 'active' AND role = 'player') as current_players,

  -- User participation status (empty string if not logged in)
  COALESCE(
    CASE
      WHEN $1::int IS NULL OR $1 = 0 THEN NULL
      WHEN g.gm_user_id = $1 THEN 'gm'
      -- Split by role: the badge in the games list distinguishes Player from
      -- Audience from Co-GM, so collapsing them into 'participant' would show
      -- an audience member "Player".
      WHEN EXISTS(SELECT 1 FROM game_participants WHERE game_id = g.id AND user_id = $1 AND status = 'active' AND role = 'co_gm') THEN 'co_gm'
      WHEN EXISTS(SELECT 1 FROM game_participants WHERE game_id = g.id AND user_id = $1 AND status = 'active' AND role = 'audience') THEN 'audience'
      WHEN EXISTS(SELECT 1 FROM game_participants WHERE game_id = g.id AND user_id = $1 AND status = 'active') THEN 'participant'
      WHEN EXISTS(SELECT 1 FROM game_applications WHERE game_id = g.id AND user_id = $1) THEN 'applied'
      ELSE 'none'
    END,
    ''
  ) as user_relationship,

  -- Current phase info for in-progress games (empty string if none)
  COALESCE((SELECT phase_type FROM game_phases WHERE game_id = g.id AND is_active = true), '') as current_phase_type,
  (SELECT end_time FROM game_phases WHERE game_id = g.id AND is_active = true) as current_phase_deadline,

  -- Deadline urgency calculation
  CASE
    WHEN g.state = 'recruitment' AND g.recruitment_deadline IS NOT NULL THEN
      CASE
        WHEN g.recruitment_deadline < NOW() + INTERVAL '24 hours' THEN 'critical'
        WHEN g.recruitment_deadline < NOW() + INTERVAL '3 days' THEN 'warning'
        ELSE 'normal'
      END
    WHEN g.state = 'in_progress' THEN
      CASE
        WHEN (SELECT end_time FROM game_phases WHERE game_id = g.id AND is_active = true) < NOW() + INTERVAL '24 hours' THEN 'critical'
        WHEN (SELECT end_time FROM game_phases WHERE game_id = g.id AND is_active = true) < NOW() + INTERVAL '3 days' THEN 'warning'
        ELSE 'normal'
      END
    ELSE 'normal'
  END as deadline_urgency,

  -- Recent activity flag (updated in last 24h)
  (g.updated_at > NOW() - INTERVAL '24 hours') as has_recent_activity

FROM games g
INNER JOIN users u ON g.gm_user_id = u.id
WHERE
  -- Show public games, OR games user is in, OR all games if admin mode enabled and user is admin
  (g.is_public = true OR
   ($6::boolean IS TRUE AND $7::int IS NOT NULL AND EXISTS(SELECT 1 FROM users WHERE id = $7 AND is_admin = true)) OR
   ($1::int IS NOT NULL AND (g.gm_user_id = $1 OR EXISTS(SELECT 1 FROM game_participants WHERE game_id = g.id AND user_id = $1 AND status = 'active'))))

  -- Filter by states (optional array of states)
  AND ($2::text[] IS NULL OR g.state = ANY($2::text[]))

  -- Filter by user participation (optional)
  AND (
    $3::text IS NULL OR $3 = '' OR
    ($3 = 'my_games' AND ($1::int IS NOT NULL AND (g.gm_user_id = $1 OR EXISTS(SELECT 1 FROM game_participants WHERE game_id = g.id AND user_id = $1 AND status = 'active')))) OR
    ($3 = 'applied' AND ($1::int IS NOT NULL AND EXISTS(SELECT 1 FROM game_applications WHERE game_id = g.id AND user_id = $1 AND status = 'pending'))) OR
    ($3 = 'not_joined' AND ($1::int IS NOT NULL AND g.gm_user_id != $1 AND NOT EXISTS(SELECT 1 FROM game_participants WHERE game_id = g.id AND user_id = $1)))
  )

  -- Filter by open spots (only recruiting games with available spots)
  AND (
    $4::boolean IS NOT true OR
    (
      g.state = 'recruitment' AND
      (g.max_players IS NULL OR (SELECT COUNT(*) FROM game_participants WHERE game_id = g.id AND status = 'active' AND role = 'player') < g.max_players)
    )
  )

  -- Filter by search text (case-insensitive search in title and description)
  AND (
    $8::text IS NULL OR $8 = '' OR
    g.title ILIKE '%' || $8 || '%' OR
    g.description::text ILIKE '%' || $8 || '%'
  )

ORDER BY
  -- Dynamic sorting based on $5 parameter
  CASE
    WHEN $5 = 'recent_activity' THEN g.updated_at
    WHEN $5 = 'created' THEN g.created_at
    WHEN $5 = 'start_date' THEN g.start_date
    WHEN $5 = 'alphabetical' THEN NULL
    ELSE g.updated_at
  END DESC NULLS LAST,

  CASE
    WHEN $5 = 'alphabetical' THEN g.title
    ELSE NULL
  END ASC NULLS LAST,

  -- Secondary sort by ID for consistency
  g.id DESC

-- Pagination
LIMIT $9::int
OFFSET $10::int;

-- Parameters:
-- $1: user_id (int, nullable) - for participation enrichment
-- $2: states (text[], nullable) - array of game states to filter
-- $3: participation_filter (text, nullable) - 'my_games', 'applied', 'not_joined'
-- $4: has_open_spots (boolean, nullable) - only games with available spots
-- $5: sort_by (text) - 'recent_activity', 'created', 'start_date', 'alphabetical'
-- $6: admin_mode (boolean) - bypass is_public filter if user is admin
-- $7: admin_user_id (int, nullable) - user ID to validate admin status
-- $8: search (text, nullable) - case-insensitive search in title and description
-- $9: limit (int) - number of records per page
-- $10: offset (int) - number of records to skip

-- name: CountPublicGames :one
SELECT COUNT(*) FROM games WHERE is_public = true;

-- name: CountFilteredGames :one
-- Count games matching the same filters as GetFilteredGames (for pagination metadata)
SELECT COUNT(*)
FROM games g
WHERE
  -- Show public games, OR games user is in, OR all games if admin mode enabled and user is admin
  (g.is_public = true OR
   ($5::boolean IS TRUE AND $6::int IS NOT NULL AND EXISTS(SELECT 1 FROM users WHERE id = $6 AND is_admin = true)) OR
   ($1::int IS NOT NULL AND (g.gm_user_id = $1 OR EXISTS(SELECT 1 FROM game_participants WHERE game_id = g.id AND user_id = $1 AND status = 'active'))))

  -- Filter by states (optional array of states)
  AND ($2::text[] IS NULL OR g.state = ANY($2::text[]))

  -- Filter by user participation (optional)
  AND (
    $3::text IS NULL OR $3 = '' OR
    ($3 = 'my_games' AND ($1::int IS NOT NULL AND (g.gm_user_id = $1 OR EXISTS(SELECT 1 FROM game_participants WHERE game_id = g.id AND user_id = $1 AND status = 'active')))) OR
    ($3 = 'applied' AND ($1::int IS NOT NULL AND EXISTS(SELECT 1 FROM game_applications WHERE game_id = g.id AND user_id = $1 AND status = 'pending'))) OR
    ($3 = 'not_joined' AND ($1::int IS NOT NULL AND g.gm_user_id != $1 AND NOT EXISTS(SELECT 1 FROM game_participants WHERE game_id = g.id AND user_id = $1)))
  )

  -- Filter by open spots (only recruiting games with available spots)
  AND (
    $4::boolean IS NOT true OR
    (
      g.state = 'recruitment' AND
      (g.max_players IS NULL OR (SELECT COUNT(*) FROM game_participants WHERE game_id = g.id AND status = 'active' AND role = 'player') < g.max_players)
    )
  )

  -- Filter by search text (case-insensitive search in title and description)
  AND (
    $7::text IS NULL OR $7 = '' OR
    g.title ILIKE '%' || $7 || '%' OR
    g.description::text ILIKE '%' || $7 || '%'
  );

-- Parameters:
-- $1: user_id (int, nullable) - for participation enrichment
-- $2: states (text[], nullable) - array of game states to filter
-- $3: participation_filter (text, nullable) - 'my_games', 'applied', 'not_joined'
-- $4: has_open_spots (boolean, nullable) - only games with available spots
-- $5: admin_mode (boolean) - bypass is_public filter if user is admin
-- $6: admin_user_id (int, nullable) - user ID to validate admin status
-- $7: search (text, nullable) - case-insensitive search in title and description

-- name: GetAvailableStates :many
SELECT DISTINCT state
FROM games
WHERE is_public = true
ORDER BY state ASC;
