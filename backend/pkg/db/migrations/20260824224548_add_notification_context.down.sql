DROP INDEX IF EXISTS idx_notifications_user_context_unread;

ALTER TABLE notifications
  DROP COLUMN IF EXISTS context_id,
  DROP COLUMN IF EXISTS context_type;
