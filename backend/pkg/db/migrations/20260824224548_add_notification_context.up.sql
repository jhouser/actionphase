-- Adds a generic "container" reference to notifications, distinct from the
-- existing related_type/related_id pair which points at the specific item that
-- triggered the notification.
--
--   context_type/context_id -> the thing a user opens (e.g. a conversation)
--   related_type/related_id -> the item inside it (e.g. one message)
--
-- This lets "clear every notification for this conversation" be a single
-- indexed UPDATE, while related_id still identifies the exact message so the
-- dashboard inbox can preview it.
--
-- Existing rows are intentionally left NULL: a NULL context keeps the old
-- one-row-at-a-time mark-read behaviour, which is what those rows do today.
ALTER TABLE notifications
  ADD COLUMN IF NOT EXISTS context_type TEXT,
  ADD COLUMN IF NOT EXISTS context_id INTEGER;

-- Supports the bulk clear: unread notifications for one user scoped to one
-- container. Partial, because read rows are never the target of that UPDATE.
CREATE INDEX IF NOT EXISTS idx_notifications_user_context_unread
  ON notifications (user_id, context_type, context_id)
  WHERE is_read = false;
