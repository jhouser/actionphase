-- name: CreateNotification :one
INSERT INTO notifications (user_id, game_id, type, title, content, related_type, related_id, link_url, context_type, context_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetUserNotifications :many
SELECT n.*, g.title as game_title
FROM notifications n
LEFT JOIN games g ON n.game_id = g.id
WHERE n.user_id = $1
ORDER BY n.created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetUnreadNotifications :many
SELECT n.*, g.title as game_title
FROM notifications n
LEFT JOIN games g ON n.game_id = g.id
WHERE n.user_id = $1 AND n.is_read = false
ORDER BY n.created_at DESC;

-- name: GetUnreadNotificationCount :one
SELECT COUNT(*) FROM notifications
WHERE user_id = $1 AND is_read = false;

-- name: MarkNotificationUnread :one
UPDATE notifications
SET is_read = false, read_at = NULL
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: MarkAllNotificationsRead :exec
UPDATE notifications
SET is_read = true, read_at = NOW()
WHERE user_id = $1 AND is_read = FALSE;

-- name: MarkGameNotificationsRead :exec
UPDATE notifications
SET is_read = true, read_at = NOW()
WHERE user_id = $1 AND game_id = $2 AND is_read = FALSE;

-- name: NotificationExistsForUser :one
-- Ownership probe used to distinguish "already read" from "not yours", which
-- rows-affected alone cannot tell apart.
SELECT EXISTS (
  SELECT 1 FROM notifications WHERE id = $1 AND user_id = $2
);

-- name: MarkNotificationAndContextRead :execrows
-- Marks a notification read along with every other unread notification sharing
-- its context (e.g. clicking one message notification clears the whole
-- conversation). Notifications with a NULL context affect only themselves.
-- Scoped to the owning user, so a foreign notification ID matches nothing.
WITH target AS (
  SELECT t0.id AS id, t0.context_type AS context_type, t0.context_id AS context_id
  FROM notifications t0
  WHERE t0.id = $1 AND t0.user_id = $2
)
UPDATE notifications n
SET is_read = true, read_at = NOW()
FROM target t
WHERE n.user_id = $2
  AND n.is_read = FALSE
  AND (
    n.id = t.id
    OR (
      t.context_type IS NOT NULL
      AND t.context_id IS NOT NULL
      AND n.context_type = t.context_type
      AND n.context_id = t.context_id
    )
  );

-- name: MarkNotificationsReadByContext :execrows
-- Marks every unread notification a user has for one container (e.g. all
-- messages in a conversation) as read. Returns the number of rows affected so
-- callers can reconcile client-side unread counts.
UPDATE notifications
SET is_read = true, read_at = NOW()
WHERE user_id = $1
  AND context_type = $2
  AND context_id = $3
  AND is_read = FALSE;

-- name: DeleteNotification :exec
DELETE FROM notifications
WHERE id = $1 AND user_id = $2;

-- name: DeleteOldNotifications :exec
DELETE FROM notifications
WHERE created_at < NOW() - INTERVAL '30 days';

-- name: GetGameNotifications :many
SELECT n.*, u.username
FROM notifications n
JOIN users u ON n.user_id = u.id
WHERE n.game_id = $1
ORDER BY n.created_at DESC
LIMIT $2 OFFSET $3;

-- Helper queries for creating notifications
-- name: NotifyGameParticipants :exec
INSERT INTO notifications (user_id, game_id, type, title, content, related_type, related_id, link_url)
SELECT gp.user_id, $1, $2, $3, $4, $5, $6, $7
FROM game_participants gp
WHERE gp.game_id = $1 AND gp.status = 'active' AND gp.user_id != $8;

-- name: NotifyGM :exec
INSERT INTO notifications (user_id, game_id, type, title, content, related_type, related_id, link_url)
SELECT g.gm_user_id, $1, $2, $3, $4, $5, $6, $7
FROM games g
WHERE g.id = $1 AND g.gm_user_id != $8;

-- name: NotifyAudienceMembers :exec
INSERT INTO notifications (user_id, game_id, type, title, content, related_type, related_id, link_url)
SELECT gp.user_id, $1, $2, $3, $4, $5, $6, $7
FROM game_participants gp
WHERE gp.game_id = $1 AND gp.role = 'audience' AND gp.status = 'active' AND gp.user_id != $8;
