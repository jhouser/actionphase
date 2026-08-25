import type { Notification } from '@/types/notifications';
import type { UnreadInboxItem, UnreadPrivateMessageItem } from '@/types/unreadInbox';

const COMMENT_NOTIFICATION_TYPES = new Set(['comment_reply', 'character_mention']);

/**
 * Extracts the `conversation` query param from a notification's link_url,
 * e.g. "/games/12?tab=messages&conversation=34" -> 34.
 * The conversation ID isn't stored on the notification itself, only in this URL.
 */
export function parseConversationIdFromLinkUrl(linkUrl?: string): number | null {
  if (!linkUrl) return null;

  try {
    const url = new URL(linkUrl, 'http://placeholder');
    const raw = url.searchParams.get('conversation');
    if (!raw) return null;

    const conversationId = parseInt(raw, 10);
    return Number.isNaN(conversationId) ? null : conversationId;
  } catch {
    return null;
  }
}

/**
 * Resolves the conversation a private message notification belongs to.
 *
 * Prefers context_id, which the backend sets on new notifications and uses to
 * clear a whole conversation at once. Falls back to parsing link_url for
 * notifications created before context tracking existed, which were never
 * backfilled.
 */
function resolveConversationId(notification: Notification): number | null {
  if (notification.context_type === 'conversation' && notification.context_id) {
    return notification.context_id;
  }
  return parseConversationIdFromLinkUrl(notification.link_url);
}

/**
 * Classifies a notification into a reply-capable inbox item, or null if it's
 * not one of the types the Unread inbox can show a reply box for.
 */
export function classifyNotification(notification: Notification): UnreadInboxItem | null {
  if (!notification.game_id) return null;

  if (COMMENT_NOTIFICATION_TYPES.has(notification.type)) {
    // related_id is always the comment/reply message id for these types.
    // `type` alone is sufficient to classify these notifications as
    // comment-shaped; related_type isn't needed.
    if (!notification.related_id) return null;
    return {
      kind: 'comment',
      notification,
      gameId: notification.game_id,
      commentId: notification.related_id,
    };
  }

  if (notification.type === 'private_message') {
    const conversationId = resolveConversationId(notification);
    if (!conversationId) return null;
    // related_id is the specific message this notification was for — needed to
    // preview the right message when a conversation has multiple unread notifications.
    if (!notification.related_id) return null;
    return {
      kind: 'private_message',
      notification,
      gameId: notification.game_id,
      conversationId,
      messageId: notification.related_id,
      // Overwritten by collapseInboxItems when several notifications share a
      // conversation; a lone notification stands for exactly itself.
      unreadCount: 1,
    };
  }

  return null;
}

/**
 * Collapses private message items so each conversation occupies one inbox row.
 *
 * A group conversation left running overnight produces one notification per
 * message; listing them individually buries everything else in the inbox. The
 * surviving row is the newest notification, since acting on it clears the whole
 * conversation anyway, and it carries the count of what it stands for.
 *
 * Comment items pass through untouched — each one is a distinct thing to reply
 * to, not repetition of the same one.
 *
 * Input order is preserved: notifications arrive newest-first, so each
 * conversation keeps the position of its most recent message.
 */
export function collapseInboxItems(items: UnreadInboxItem[]): UnreadInboxItem[] {
  const collapsed: UnreadInboxItem[] = [];
  const byConversation = new Map<number, UnreadPrivateMessageItem>();

  for (const item of items) {
    if (item.kind !== 'private_message') {
      collapsed.push(item);
      continue;
    }

    const existing = byConversation.get(item.conversationId);
    if (existing) {
      existing.unreadCount += 1;
      continue;
    }

    // Copy so repeated calls never mutate the caller's objects.
    const row = { ...item };
    byConversation.set(item.conversationId, row);
    collapsed.push(row);
  }

  return collapsed;
}
