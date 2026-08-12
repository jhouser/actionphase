import { apiClient } from '@/lib/api';
import { findRootPostId, fetchCommentWithParents } from '@/utils/threadUtils';
import { logger } from '@/services/LoggingService';
import type { Message } from '@/types/messages';
import type { PrivateMessage } from '@/types/conversations';
import type { Character } from '@/types/characters';
import { classifyNotification } from './parseUnreadNotification';
import type { UnreadInboxItem } from '@/types/unreadInbox';

/**
 * Fetches unread notifications and classifies them into reply-capable inbox
 * items (comment replies/mentions and private messages). Non-repliable
 * notification types are dropped.
 */
export async function fetchUnreadInboxItems(): Promise<UnreadInboxItem[]> {
  const response = await apiClient.notifications.getNotifications({ unread: true, limit: 100 });
  return response.data.data
    .map(classifyNotification)
    .filter((item): item is UnreadInboxItem => item !== null);
}

export interface CommentContext {
  comment: Message;
  parent: Message | null;
  rootPostId: number;
}

/**
 * Fetches the comment being replied to, its immediate parent (for character
 * defaulting), and the root post id (required by createComment).
 */
export async function fetchCommentContext(gameId: number, commentId: number): Promise<CommentContext> {
  const { messages } = await fetchCommentWithParents(gameId, commentId, 1);
  const comment = messages[messages.length - 1];
  if (!comment) {
    throw new Error(`Comment ${commentId} could not be loaded (it may have been deleted)`);
  }
  const parent = messages.length > 1 ? messages[messages.length - 2] : null;
  // parent (if fetched) is already the message findRootPostId would fetch first;
  // starting from it instead of `comment` avoids re-fetching it over the network.
  const rootPostId = await findRootPostId(gameId, parent ?? comment);

  return { comment, parent, rootPostId };
}

/**
 * Fetches the message a notification was for plus the one preceding it, for
 * showing PM context in the Unread inbox. Deliberately does not fetch the whole
 * conversation — a long-running thread would otherwise send its entire history
 * over the wire just to preview two messages.
 *
 * Returns messages oldest-first, so the preceding message comes before the
 * notified one.
 */
export async function fetchPmContext(
  gameId: number,
  conversationId: number,
  messageId: number
): Promise<PrivateMessage[]> {
  const response = await apiClient.conversations.getMessageWithContext(gameId, conversationId, messageId);
  return response.data.messages;
}

export async function resolveReplyCharacters(gameId: number): Promise<Character[]> {
  const response = await apiClient.characters.getUserControllableCharacters(gameId);
  return response.data;
}

/**
 * All characters in the game (permission-filtered by the backend), used as
 * the @-mention list when replying to a comment — matches the Common Room's
 * mention scope, which is every character in the game, not just the ones the
 * replier controls.
 */
export async function fetchAllGameCharacters(gameId: number): Promise<Character[]> {
  const response = await apiClient.characters.getGameCharacters(gameId);
  return response.data;
}

/**
 * Returns the character IDs already participating in a conversation, used to
 * default the reply-as character picker to a character the user already
 * spoke as in this thread, and to scope the @-mention list to conversation
 * participants (matching MessageThread's mention scope for PMs).
 */
export async function fetchConversationParticipantCharacterIds(
  gameId: number,
  conversationId: number
): Promise<number[]> {
  const response = await apiClient.conversations.getConversation(gameId, conversationId);
  return response.data.participants
    .map((p) => p.character_id)
    .filter((id): id is number => id !== undefined);
}

export interface ReplyToCommentParams {
  gameId: number;
  notificationId: number;
  parentCommentId: number;
  rootPostId: number;
  characterId: number;
  content: string;
}

export async function replyToComment({
  gameId,
  notificationId,
  parentCommentId,
  rootPostId,
  characterId,
  content,
}: ReplyToCommentParams): Promise<void> {
  await apiClient.messages.createComment(gameId, parentCommentId, {
    character_id: characterId,
    content,
    root_post_id: rootPostId,
  });
  await markNotificationAsReadBestEffort(notificationId);
}

export interface ReplyToPmParams {
  gameId: number;
  notificationId: number;
  conversationId: number;
  characterId: number;
  content: string;
}

export async function replyToPm({
  gameId,
  notificationId,
  conversationId,
  characterId,
  content,
}: ReplyToPmParams): Promise<void> {
  await apiClient.conversations.sendMessage(gameId, conversationId, {
    character_id: characterId,
    content,
  });
  await markNotificationAsReadBestEffort(notificationId);
}

/**
 * Marks a notification as read without failing the calling mutation if it
 * errors (e.g. the notification was already read/removed elsewhere) — the
 * reply/message itself has already been sent successfully by this point, so
 * a mark-read failure shouldn't be reported to the user as a failed reply.
 */
async function markNotificationAsReadBestEffort(notificationId: number): Promise<void> {
  try {
    await apiClient.notifications.markNotificationAsRead(notificationId);
  } catch (error) {
    logger.error(`Failed to mark notification ${notificationId} as read after a successful reply`, { error });
  }
}
