-- name: CreateGame :one
INSERT INTO games (
    title, description, gm_user_id, genre, start_date, end_date,
    recruitment_deadline, max_players, is_public, is_anonymous, auto_accept_audience, allow_group_conversations, portrait_avatars, banner_url,
    common_room_open_day, common_room_open_time, common_room_close_day, common_room_close_time, schedule_timezone,
    -- Required by the application on every new game (req 5), though the column
    -- stays nullable so pre-community games remain valid. See the migration.
    community_id,
    character_sheet
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14,
    $15, $16, $17, $18, $19, $20,
    -- COALESCE so a caller that builds CreateGameParams directly and leaves
    -- CharacterSheet nil gets '{}' rather than a NOT NULL violation. Naming the
    -- column in the INSERT disables the column DEFAULT, so the default has to be
    -- restated here.
    COALESCE(sqlc.narg('character_sheet')::jsonb, '{}'::jsonb)
) RETURNING *;

-- name: GetGame :one
SELECT * FROM games WHERE id = $1;

-- name: GetGamesByGM :many
SELECT * FROM games WHERE gm_user_id = $1 ORDER BY created_at DESC;

-- name: GetGamesByUser :many
SELECT g.*, gp.role as user_role, u.username as gm_username
FROM games g
JOIN game_participants gp ON g.id = gp.game_id
JOIN users u ON g.gm_user_id = u.id
WHERE gp.user_id = $1 AND gp.status = 'active'
ORDER BY g.updated_at DESC;

-- name: UpdateGameState :one
UPDATE games
SET state = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateGame :one
UPDATE games
SET title = $2, description = $3, genre = $4, start_date = $5,
    end_date = $6, recruitment_deadline = $7, max_players = $8,
    is_public = $9, is_anonymous = $10, auto_accept_audience = $11, allow_group_conversations = $12, portrait_avatars = $13,
    banner_url = COALESCE($14, banner_url),
    -- Preserve-on-absent, like banner_url above and for the same reason: the
    -- caller passes a *int32 it can leave nil. It is NOT part of the full
    -- replace, because an edit that omitted it would otherwise NULL the
    -- community -- and a game with no community is one no ban can reach.
    -- The setup-only rule (decision 4) is enforced in the service, not here.
    community_id = COALESCE(sqlc.narg('community_id')::integer, games.community_id),
    common_room_open_day = $15, common_room_open_time = $16,
    common_room_close_day = $17, common_room_close_time = $18,
    schedule_timezone = $19,
    -- The COALESCE is belt-and-braces only: it fires just for a caller that builds
    -- UpdateGameParams by hand and leaves CharacterSheet nil. It is NOT a
    -- preserve-on-absent contract, and must not be read as one.
    --
    -- UpdateGame is a full replace, not a patch. core.UpdateGameRequest.CharacterSheet
    -- is a value, not a pointer, and the service marshals an empty config to '{}'
    -- rather than nil, so a request that omits `character_sheet` RESETS the labels to
    -- defaults. That is deliberate and is how the GM unsets them: the edit form clears
    -- all three boxes and sends no key at all.
    --
    -- It also matches every other field here -- omitting `portrait_avatars` writes
    -- false, omitting `title` fails validation. `banner_url` above is the one genuine
    -- preserve-on-absent field, and it earns that by being a *string the caller can
    -- leave nil. If character_sheet ever needs the same, it has to become a pointer
    -- too; the COALESCE alone cannot express it.
    character_sheet = COALESCE(sqlc.narg('character_sheet')::jsonb, games.character_sheet),
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateGameBannerURL :one
UPDATE games
SET banner_url = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteGame :exec
-- Only allow deletion of cancelled games
-- Foreign key constraints will cascade delete related data
DELETE FROM games WHERE id = $1 AND state = 'cancelled';

-- name: GetGameParticipants :many
SELECT gp.*, u.username, u.avatar_url
FROM game_participants gp
JOIN users u ON gp.user_id = u.id
WHERE gp.game_id = $1 AND gp.status = 'active'
ORDER BY gp.joined_at;

-- name: AddGameParticipant :one
INSERT INTO game_participants (game_id, user_id, role)
VALUES ($1, $2, $3)
ON CONFLICT (game_id, user_id) DO UPDATE
SET role = EXCLUDED.role,
    status = 'active',
    removed_at = NULL,
    removed_by_user_id = NULL,
    is_former_player = game_participants.is_former_player,
    joined_at = NOW()
WHERE game_participants.status != 'active'
RETURNING *;

-- name: UpdateParticipantStatus :one
UPDATE game_participants
SET status = $3
WHERE game_id = $1 AND user_id = $2
RETURNING *;

-- name: RemoveGameParticipant :exec
DELETE FROM game_participants
WHERE game_id = $1 AND user_id = $2;

-- name: GetParticipantRole :one
SELECT role FROM game_participants
WHERE game_id = $1 AND user_id = $2 AND status = 'active';

-- name: IsUserInGame :one
SELECT EXISTS(
    SELECT 1 FROM game_participants
    WHERE game_id = $1 AND user_id = $2 AND status = 'active'
);

-- name: GetGameParticipantCount :one
SELECT COUNT(*) FROM game_participants
WHERE game_id = $1 AND role = 'player' AND status = 'active';

-- name: GetRecruitingGames :many
SELECT games.*, COALESCE(users.username, 'Unknown') as gm_username,
       COALESCE(participant_count.count, 0) as current_players
FROM games
LEFT JOIN users ON games.gm_user_id = users.id
LEFT JOIN (
    SELECT game_id, COUNT(*) as count
    FROM game_participants
    WHERE role = 'player' AND status = 'active'
    GROUP BY game_id
) participant_count ON games.id = participant_count.game_id
WHERE games.is_public = true
AND games.state = 'recruitment'
AND (games.recruitment_deadline IS NULL OR games.recruitment_deadline > NOW())
ORDER BY games.created_at DESC;

-- name: GetGameWithDetails :one
-- Joins the owning community's name and slug so a game surface can label and
-- link its community without a second request.
--
-- LEFT JOIN, not inner: a game predating communities has community_id NULL
-- (req 5) and must still resolve here. Both columns come back NULL for those,
-- which the caller renders as no community section rather than a placeholder.
SELECT
    g.*,
    u.username as gm_username,
    c.name as community_name,
    c.slug as community_slug,
    COALESCE(pc.player_count, 0) as current_players
FROM games g
LEFT JOIN users u ON g.gm_user_id = u.id
LEFT JOIN communities c ON c.id = g.community_id
LEFT JOIN (
    SELECT game_id, COUNT(*) as player_count
    FROM game_participants
    WHERE role = 'player' AND status = 'active'
    GROUP BY game_id
) pc ON g.id = pc.game_id
WHERE g.id = $1;

-- name: CanUserJoinGame :one
SELECT
    CASE
        WHEN g.state != 'recruitment' THEN 'game_not_recruiting'
        WHEN g.recruitment_deadline IS NOT NULL AND g.recruitment_deadline < NOW() THEN 'deadline_passed'
        WHEN COALESCE(pc.player_count, 0) >= g.max_players THEN 'game_full'
        WHEN EXISTS(SELECT 1 FROM game_participants gp WHERE gp.game_id = $1 AND gp.user_id = $2 AND gp.status = 'active') THEN 'already_joined'
        ELSE 'can_join'
    END as join_status
FROM games g
LEFT JOIN (
    SELECT game_id, COUNT(*) as player_count
    FROM game_participants
    WHERE role = 'player' AND status = 'active'
    GROUP BY game_id
) pc ON g.id = pc.game_id
WHERE g.id = $1;

-- name: GetGamesNeedingStateUpdate :many
SELECT * FROM games
WHERE (state = 'recruitment' AND recruitment_deadline IS NOT NULL AND recruitment_deadline < NOW())
   OR (state = 'in_progress' AND end_date IS NOT NULL AND end_date < NOW());

-- Player Management Queries

-- name: RemoveParticipant :exec
UPDATE game_participants
SET removed_at = NOW(),
    removed_by_user_id = $3,
    status = 'removed'
WHERE game_id = $1 AND user_id = $2 AND removed_at IS NULL;

-- name: AddParticipantWithRole :one
INSERT INTO game_participants (game_id, user_id, role, status)
VALUES ($1, $2, $3, 'active')
ON CONFLICT (game_id, user_id) DO UPDATE
SET removed_at = NULL,
    removed_by_user_id = NULL,
    status = 'active',
    role = EXCLUDED.role,
    joined_at = NOW()
WHERE game_participants.removed_at IS NOT NULL
RETURNING *;

-- name: GetActiveParticipants :many
SELECT gp.*, u.username, u.email
FROM game_participants gp
JOIN users u ON gp.user_id = u.id
WHERE gp.game_id = $1 AND gp.removed_at IS NULL AND gp.status = 'active'
ORDER BY gp.joined_at;

-- name: CheckParticipantExists :one
SELECT EXISTS(
    SELECT 1 FROM game_participants
    WHERE game_id = $1 AND user_id = $2 AND removed_at IS NULL AND status = 'active'
);

-- name: RestoreParticipant :one
UPDATE game_participants
SET removed_at = NULL,
    removed_by_user_id = NULL,
    status = 'active'
WHERE game_id = $1 AND user_id = $2 AND removed_at IS NOT NULL
RETURNING *;

-- Audience Participation Queries

-- name: GetGameAutoAcceptAudience :one
SELECT auto_accept_audience FROM games WHERE id = $1;

-- name: UpdateGameAutoAcceptAudience :exec
UPDATE games
SET auto_accept_audience = $2, updated_at = NOW()
WHERE id = $1;

-- name: DisableAnonymousMode :exec
UPDATE games
SET is_anonymous = false, updated_at = NOW()
WHERE id = $1;

-- name: CreateAudienceApplication :one
INSERT INTO game_participants (game_id, user_id, role, status)
VALUES ($1, $2, 'audience', $3)
ON CONFLICT (game_id, user_id) DO UPDATE
SET removed_at = NULL,
    removed_by_user_id = NULL,
    status = EXCLUDED.status,
    role = 'audience',
    joined_at = NOW()
WHERE game_participants.removed_at IS NOT NULL OR game_participants.status = 'removed'
RETURNING *;

-- name: ListAudienceMembers :many
SELECT gp.*, u.username, u.email
FROM game_participants gp
JOIN users u ON gp.user_id = u.id
WHERE gp.game_id = $1 AND gp.role = 'audience' AND gp.status = 'active'
ORDER BY gp.joined_at;

-- name: CheckAudienceAccess :one
SELECT EXISTS(
    SELECT 1 FROM game_participants
    WHERE game_id = $1 AND user_id = $2 AND role IN ('audience', 'co_gm') AND status = 'active'
) OR EXISTS(
    SELECT 1 FROM games
    WHERE id = $1 AND gm_user_id = $2
);

-- Co-GM Management Queries

-- name: UpdateParticipantRole :one
UPDATE game_participants
SET role = $3
WHERE game_id = $1 AND user_id = $2 AND status = 'active'
RETURNING *;

-- name: TransitionParticipantToAudience :one
UPDATE game_participants
SET role = 'audience', is_former_player = TRUE
WHERE game_id = $1 AND user_id = $2 AND status = 'active'
RETURNING *;

-- name: GetParticipantByGameAndUser :one
SELECT gp.*, u.username, u.email
FROM game_participants gp
JOIN users u ON gp.user_id = u.id
WHERE gp.game_id = $1 AND gp.user_id = $2 AND gp.status = 'active';

-- name: CountCoGMsInGame :one
SELECT COUNT(*) FROM game_participants
WHERE game_id = $1 AND role = 'co_gm' AND status = 'active';

-- name: GetGameCoGMs :many
SELECT user_id FROM game_participants
WHERE game_id = $1 AND role = 'co_gm' AND status = 'active';
