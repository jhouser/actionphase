-- Community bans and their audit log.
--
-- 🔴 EVERY read that answers "is this user banned right now" MUST carry the
-- expiry test: (expires_at IS NULL OR expires_at > NOW()). An expired row stays
-- in the table -- it is history, and lifting it is a separate deliberate act --
-- so a query that omits the test silently enforces bans that have lapsed.

-- name: CreateCommunityBan :one
-- Bans a user, or updates the existing ban if one is already recorded.
--
-- UPSERT rather than INSERT because UNIQUE(community_id, user_id) means a
-- moderator re-banning an already-banned user would otherwise get a constraint
-- violation. Re-banning is the natural way to change a reason or extend an
-- expiry, so it updates in place. banned_at is refreshed too: the new decision
-- is the one that counts.
INSERT INTO community_bans (
    community_id, user_id, reason, banned_by_user_id, expires_at
) VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (community_id, user_id) DO UPDATE SET
    reason            = EXCLUDED.reason,
    banned_by_user_id = EXCLUDED.banned_by_user_id,
    expires_at        = EXCLUDED.expires_at,
    banned_at         = NOW()
RETURNING *;

-- name: DeleteCommunityBan :one
-- Lifts a ban. Returns the deleted row so the caller can snapshot its values
-- into an 'unbanned' audit event -- after this the row is gone, and the log is
-- the only remaining record of what the ban said.
DELETE FROM community_bans
WHERE community_id = $1 AND user_id = $2
RETURNING *;

-- name: GetCommunityBan :one
-- Fetches one ban row REGARDLESS of expiry, for management surfaces that need
-- to show an expired ban still sitting on the list. Not for enforcement.
SELECT * FROM community_bans
WHERE community_id = $1 AND user_id = $2;

-- name: IsUserBannedFromCommunity :one
-- The enforcement primitive. Carries the expiry test.
SELECT EXISTS(
    SELECT 1 FROM community_bans
    WHERE community_id = sqlc.arg('community_id')
      AND user_id = sqlc.arg('user_id')
      AND (expires_at IS NULL OR expires_at > NOW())
) AS banned;

-- name: IsUserBannedFromGameCommunity :one
-- Enforcement for a path that knows a game but not a community.
--
-- The join is INNER on communities via games.community_id, so a game with
-- community_id IS NULL produces no row and EXISTS is false. That is the
-- grandfathering guarantee expressed in SQL: a legacy game is never blocked,
-- and it holds here rather than depending on every caller remembering to check.
SELECT EXISTS(
    SELECT 1
    FROM games g
    JOIN community_bans cb ON cb.community_id = g.community_id
    WHERE g.id = sqlc.arg('game_id')
      AND cb.user_id = sqlc.arg('user_id')
      AND (cb.expires_at IS NULL OR cb.expires_at > NOW())
) AS banned;

-- name: ListCommunityBans :many
-- The management view. Returns EXPIRED bans too, ordered newest first, so a
-- moderator can see that a temporary ban has lapsed rather than watching it
-- vanish from the list with no explanation. Callers render expiry state from
-- expires_at.
SELECT
    cb.*,
    u.username     AS username,
    u.display_name AS display_name,
    u.avatar_url   AS avatar_url,
    b.username     AS banned_by_username
FROM community_bans cb
JOIN users u ON u.id = cb.user_id
LEFT JOIN users b ON b.id = cb.banned_by_user_id
WHERE cb.community_id = $1
ORDER BY cb.banned_at DESC;

-- name: ListBannedCommunityIDsForUser :many
-- Every community the user is currently banned from, for filtering the
-- community picker on game creation. Carries the expiry test.
SELECT community_id FROM community_bans
WHERE user_id = $1
  AND (expires_at IS NULL OR expires_at > NOW());

-- name: CreateCommunityBanEvent :one
-- Appends to the audit log. Values are SNAPSHOTS, not references: by the time
-- an 'unbanned' event is read its ban row no longer exists.
INSERT INTO community_ban_events (
    community_id, target_user_id, actor_user_id, action, reason, expires_at
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListCommunityBanEvents :many
-- The audit log for one community, newest first.
--
-- Usernames come from a LEFT JOIN on both sides: actor_user_id is nullable
-- (ON DELETE SET NULL, so a deleted moderator's events survive with no actor),
-- and the target user may likewise be gone. The event must still render.
SELECT
    e.*,
    t.username AS target_username,
    a.username AS actor_username
FROM community_ban_events e
LEFT JOIN users t ON t.id = e.target_user_id
LEFT JOIN users a ON a.id = e.actor_user_id
WHERE e.community_id = $1
ORDER BY e.created_at DESC, e.id DESC
LIMIT $2 OFFSET $3;
