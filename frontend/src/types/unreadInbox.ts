import type { Notification } from '@/types/notifications';

export interface UnreadCommentItem {
  kind: 'comment';
  notification: Notification;
  gameId: number;
  commentId: number;
}

export interface UnreadPrivateMessageItem {
  kind: 'private_message';
  /** The newest notification for this conversation. Acting on it clears every
   * notification the conversation produced, not just this one. */
  notification: Notification;
  gameId: number;
  conversationId: number;
  /** The specific message this notification was for — used to preview the
   * right message when a conversation has multiple unread notifications. */
  messageId: number;
  /** How many unread message notifications this row stands for. A busy group
   * conversation collapses into one row rather than one row per message. */
  unreadCount: number;
}

export type UnreadInboxItem = UnreadCommentItem | UnreadPrivateMessageItem;
