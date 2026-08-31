-- name: CreateCharacter :one
INSERT INTO characters (game_id, user_id, name, character_type, status)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetCharacter :one
SELECT * FROM characters WHERE id = $1;

-- name: GetCharactersByGame :many
SELECT c.*, u.username as owner_username, na.assigned_user_id, au.username as assigned_username
FROM characters c
LEFT JOIN users u ON c.user_id = u.id
LEFT JOIN npc_assignments na ON c.id = na.character_id
LEFT JOIN users au ON na.assigned_user_id = au.id
WHERE c.game_id = $1
ORDER BY c.character_type, c.name;

-- name: GetCharactersByUser :many
SELECT c.*, g.title as game_title
FROM characters c
JOIN games g ON c.game_id = g.id
WHERE c.user_id = $1
ORDER BY c.created_at DESC;

-- name: GetPlayerCharactersByGame :many
SELECT c.*, u.username as owner_username
FROM characters c
JOIN users u ON c.user_id = u.id
WHERE c.game_id = $1 AND c.character_type = 'player_character'
ORDER BY c.name;

-- name: HasApprovedCharacterInGame :one
-- Returns true if the user has at least one approved player_character in the game
SELECT EXISTS (
  SELECT 1 FROM characters
  WHERE game_id = $1 AND user_id = $2 AND character_type = 'player_character' AND status = 'approved'
) AS has_approved_character;

-- name: GetNPCsByGame :many
SELECT c.*, u.username as owner_username, na.assigned_user_id, au.username as assigned_username
FROM characters c
LEFT JOIN users u ON c.user_id = u.id
LEFT JOIN npc_assignments na ON c.id = na.character_id
LEFT JOIN users au ON na.assigned_user_id = au.id
WHERE c.game_id = $1 AND c.character_type = 'npc'
ORDER BY c.name;

-- name: UpdateCharacter :one
UPDATE characters
SET name = $2, status = $3, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateCharacterStatus :one
UPDATE characters
SET status = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteCharacter :exec
DELETE FROM characters WHERE id = $1;

-- name: AssignNPCToUser :one
INSERT INTO npc_assignments (character_id, assigned_user_id, assigned_by_user_id)
VALUES ($1, $2, $3)
ON CONFLICT (character_id)
DO UPDATE SET assigned_user_id = $2, assigned_by_user_id = $3, assigned_at = NOW()
RETURNING *;

-- name: UnassignNPC :exec
DELETE FROM npc_assignments WHERE character_id = $1;

-- name: GetNPCAssignment :one
SELECT * FROM npc_assignments WHERE character_id = $1;

-- name: GetUserNPCs :many
SELECT c.*, g.title as game_title
FROM characters c
JOIN games g ON c.game_id = g.id
JOIN npc_assignments na ON c.id = na.character_id
WHERE na.assigned_user_id = $1
ORDER BY g.title, c.name;

-- name: CreateCharacterData :one
INSERT INTO character_data (character_id, module_type, field_name, field_value, field_type, is_public)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (character_id, module_type, field_name)
DO UPDATE SET field_value = $4, field_type = $5, is_public = $6, updated_at = NOW()
RETURNING *;

-- name: GetCharacterData :many
SELECT * FROM character_data
WHERE character_id = $1
ORDER BY module_type, field_name;

-- name: GetCharacterDataByModule :many
SELECT * FROM character_data
WHERE character_id = $1 AND module_type = $2
ORDER BY field_name;

-- name: GetPublicCharacterData :many
SELECT * FROM character_data
WHERE character_id = $1 AND is_public = true
ORDER BY module_type, field_name;

-- Every sheet row for a game, tagged with the owning user so the caller can
-- apply per-character visibility without a query per character. Rows come back
-- for the whole cast; filtering is the handler's job via core.SheetAccess,
-- which is the single definition of that rule.
-- name: GetCharacterDataByGame :many
SELECT cd.*, c.user_id AS owner_id
FROM character_data cd
JOIN characters c ON c.id = cd.character_id
WHERE c.game_id = $1
ORDER BY cd.character_id, cd.module_type, cd.field_name;

-- name: DeleteCharacterData :exec
DELETE FROM character_data
WHERE character_id = $1 AND module_type = $2 AND field_name = $3;

-- name: DeleteCharacterModule :exec
DELETE FROM character_data
WHERE character_id = $1 AND module_type = $2;

-- name: GetUserControllableCharacters :many
-- Get all characters a user can control in a game:
-- 1. Their own player characters (where user_id matches)
-- 2. NPCs assigned to them via npc_assignments
-- 3. If they're the GM or co-GM, all NPCs (for emergency situations, GMs/co-GMs can control any NPC)
-- 4. When game is in_progress, pending/rejected characters are only visible to GMs/co-GMs
SELECT DISTINCT c.id, c.game_id, c.user_id, c.name, c.character_type, c.status, c.avatar_url, c.created_at, c.updated_at
FROM characters c
LEFT JOIN npc_assignments na ON c.id = na.character_id
LEFT JOIN games g ON c.game_id = g.id
LEFT JOIN game_participants gp ON c.game_id = gp.game_id AND gp.user_id = $2 AND gp.role = 'co_gm'
WHERE c.game_id = $1
  AND (
    -- User's own player characters
    (c.user_id = $2 AND c.character_type = 'player_character')
    OR
    -- NPCs assigned to user
    (na.assigned_user_id = $2)
    OR
    -- If user is GM or co-GM, all NPCs (GMs/co-GMs can control any NPC in their game)
    ((g.gm_user_id = $2 OR gp.user_id IS NOT NULL) AND c.character_type = 'npc')
  )
  AND (
    -- User's own player characters are always visible regardless of status
    (c.user_id = $2 AND c.character_type = 'player_character')
    OR
    -- If user is GM or co-GM, include all characters regardless of status
    (g.gm_user_id = $2 OR gp.user_id IS NOT NULL)
    OR
    -- If game is not in_progress, include all characters regardless of status
    (g.state != 'in_progress')
    OR
    -- For NPCs during in_progress: exclude pending/rejected unless assigned to this user
    (g.state = 'in_progress' AND c.character_type = 'npc' AND c.status NOT IN ('pending', 'rejected'))
    OR
    -- NPCs assigned to this user are always visible to the assignee regardless of status
    (na.assigned_user_id = $2 AND c.character_type = 'npc')
  )
ORDER BY c.character_type, c.name;

-- name: GetUserControllableCharactersAcrossGames :many
-- Cross-game variant of GetUserControllableCharacters, for surfaces that need a
-- user's characters without a game in scope (e.g. the global Utility Drawer).
--
-- Control rules follow the per-game query: own player characters, NPCs assigned
-- via npc_assignments, and — for GMs/co-GMs — every character in their game.
-- Restricted to in_progress games: this backs "what am I currently playing",
-- and sheets for finished games stay reachable from the game itself.
--
-- The GM's entry is the whole cast, not just NPCs, matching what the in-game
-- drawer offers them: they run the game, every sheet in it is theirs to
-- reference, and they can already open any of them from the Characters tab.
-- Restricting them to NPCs left a GM with no game in scope seeing a fraction of
-- their own game.
--
-- Also returns the game context each character's sheet permissions depend on
-- (game title/state/flags and the user's role in that game), plus who plays each
-- character. Callers outside a game route have no other way to resolve role or
-- ownership per character without a fetch per game, and without the role the
-- sheet would fall back to read-only.
SELECT DISTINCT
  c.id, c.game_id, c.user_id, c.name, c.character_type, c.status, c.avatar_url,
  c.created_at, c.updated_at,
  g.title AS game_title,
  g.state AS game_state,
  g.is_anonymous AS game_is_anonymous,
  g.portrait_avatars AS game_portrait_avatars,
  -- The drawer renders sheets with no game in scope, so it cannot read the
  -- config from game context the way in-game surfaces do; it has to travel with
  -- each character. Safe in this SELECT DISTINCT: jsonb has a default btree
  -- opclass, so it dedups directly. (Plain `json` does not, and would fail with
  -- "could not identify an equality operator" — the column is jsonb.)
  g.character_sheet AS game_character_sheet,
  u.username AS owner_username,
  au.username AS assigned_username,
  CASE
    WHEN g.gm_user_id = $1 THEN 'gm'
    WHEN gp_role.role IS NOT NULL THEN gp_role.role
    ELSE 'player'
  END AS user_role
FROM characters c
LEFT JOIN npc_assignments na ON c.id = na.character_id
JOIN games g ON c.game_id = g.id
LEFT JOIN users u ON c.user_id = u.id
LEFT JOIN users au ON na.assigned_user_id = au.id
LEFT JOIN game_participants gp ON c.game_id = gp.game_id AND gp.user_id = $1 AND gp.role = 'co_gm'
LEFT JOIN game_participants gp_role ON c.game_id = gp_role.game_id AND gp_role.user_id = $1
WHERE g.state = 'in_progress'
  -- Characters deactivated when a player leaves a game are not "currently played".
  AND c.is_active = true
  AND (
    -- User's own player characters
    (c.user_id = $1 AND c.character_type = 'player_character')
    OR
    -- NPCs assigned to user
    (na.assigned_user_id = $1)
    OR
    -- If user is GM or co-GM, the entire cast of their game
    (g.gm_user_id = $1 OR gp.user_id IS NOT NULL)
  )
  AND (
    -- User's own player characters are always visible regardless of status
    (c.user_id = $1 AND c.character_type = 'player_character')
    OR
    -- If user is GM or co-GM, include all characters regardless of status
    (g.gm_user_id = $1 OR gp.user_id IS NOT NULL)
    OR
    -- For NPCs: exclude pending/rejected unless assigned to this user
    (c.character_type = 'npc' AND c.status NOT IN ('pending', 'rejected'))
    OR
    -- NPCs assigned to this user are always visible to the assignee regardless of status
    (na.assigned_user_id = $1 AND c.character_type = 'npc')
  )
ORDER BY game_title, c.character_type, c.name;

-- name: GetCharacterByNameAndGame :one
-- Look up a character by name within a specific game (for mention parsing)
SELECT * FROM characters
WHERE name = $1 AND game_id = $2
LIMIT 1;

-- name: UpdateCharacterAvatar :one
UPDATE characters
SET avatar_url = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteCharacterAvatar :exec
UPDATE characters
SET avatar_url = NULL, updated_at = NOW()
WHERE id = $1;

-- name: GetCharactersWithAvatars :many
-- For cleanup/maintenance: find all characters with avatars
SELECT id, avatar_url FROM characters
WHERE avatar_url IS NOT NULL;

-- Player Management Queries

-- name: DeactivatePlayerCharacters :exec
UPDATE characters
SET is_active = false,
    updated_at = NOW()
WHERE game_id = $1 AND user_id = $2 AND character_type = 'player_character';

-- name: ReassignCharacter :one
UPDATE characters
SET user_id = $2,
    original_owner_user_id = COALESCE(original_owner_user_id, user_id),
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: ListInactiveCharacters :many
SELECT c.*, u.username as current_owner_username, ou.username as original_owner_username
FROM characters c
LEFT JOIN users u ON c.user_id = u.id
LEFT JOIN users ou ON c.original_owner_user_id = ou.id
WHERE c.game_id = $1 AND c.is_active = false
ORDER BY c.updated_at DESC;

-- Audience Participation Queries

-- name: ListAudienceNPCs :many
SELECT c.*, u.username as owner_username, na.assigned_user_id, au.username as assigned_username
FROM characters c
LEFT JOIN users u ON c.user_id = u.id
LEFT JOIN npc_assignments na ON c.id = na.character_id
LEFT JOIN users au ON na.assigned_user_id = au.id
WHERE c.game_id = $1 AND c.character_type = 'npc'
ORDER BY c.name;

-- name: AssignNPCToAudience :one
INSERT INTO npc_assignments (character_id, assigned_user_id, assigned_by_user_id)
VALUES ($1, $2, $3)
ON CONFLICT (character_id)
DO UPDATE SET assigned_user_id = $2, assigned_by_user_id = $3, assigned_at = NOW()
RETURNING *;

-- name: GetCharacterActivityStats :one
-- Returns public message count and private message count for a character.
-- public_messages: all non-deleted messages (posts + comments) in the common room
-- private_messages: all non-deleted private messages sent as this character
SELECT
    COUNT(DISTINCT m.id) FILTER (WHERE m.is_deleted = false) AS public_messages,
    COUNT(DISTINCT pm.id) FILTER (WHERE pm.is_deleted = false) AS private_messages
FROM characters c
LEFT JOIN messages m ON m.character_id = c.id
LEFT JOIN private_messages pm ON pm.sender_character_id = c.id
WHERE c.id = $1;

-- name: GetCharacterActivityStatsByGame :many
-- Same as GetCharacterActivityStats, but for every character in a game in one
-- query. Used to avoid firing one /stats request per roster member from the
-- frontend, which was bursting the DB connection pool on rosters of 20+
-- characters.
SELECT
    c.id AS character_id,
    COUNT(DISTINCT m.id) FILTER (WHERE m.is_deleted = false) AS public_messages,
    COUNT(DISTINCT pm.id) FILTER (WHERE pm.is_deleted = false) AS private_messages
FROM characters c
LEFT JOIN messages m ON m.character_id = c.id
LEFT JOIN private_messages pm ON pm.sender_character_id = c.id
WHERE c.game_id = $1
GROUP BY c.id;
