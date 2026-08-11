import { useQuery } from '@tanstack/react-query';
import {
  fetchCommentContext,
  fetchPmContext,
  fetchConversationParticipantCharacterIds,
  fetchAllGameCharacters,
  resolveReplyCharacters,
} from '@/utils/unreadInboxApi';
import type { UnreadInboxItem } from '@/types/unreadInbox';
import type { Character } from '@/types/characters';

/** Maps directly onto ParentCommentPreview's props for the quoted-content block. */
interface PreviewMessage {
  content: string;
  createdAt: string | null;
  authorUsername: string | null;
  characterId: number | null;
  characterName: string | null;
  characterAvatarUrl: string | null;
}

/** Same shape as PreviewMessage plus deletion state, which the parent block renders as "[deleted]". */
export interface ParentPreview extends PreviewMessage {
  isDeleted: boolean;
}

interface CommentItemContext {
  kind: 'comment';
  previewMessage: PreviewMessage;
  /**
   * The message being replied to, shown collapsed above the unread item to
   * refresh the reader's memory. Null when the unread comment is itself a
   * top-level reply to a post that couldn't be fetched. Costs no extra network
   * call — fetchCommentContext already walks one level up.
   */
  parentMessage: ParentPreview | null;
  rootPostId: number;
  controllableCharacters: Character[];
  /** Every character in the game, for the @-mention list (matches Common Room scope). */
  mentionableCharacters: Character[];
  defaultCharacterId: number | null;
  /** True if the comment has been deleted or is still a draft; Common Room hides replying in both cases. */
  isReplyDisabled: boolean;
}

interface PmItemContext {
  kind: 'private_message';
  previewMessage: PreviewMessage;
  /**
   * The message immediately preceding the unread one in the conversation.
   * Null when the unread message opened the conversation. Costs no extra
   * network call — fetchPmContext returns the notified message and its
   * predecessor in a single scoped request.
   */
  parentMessage: ParentPreview | null;
  controllableCharacters: Character[];
  /** Characters participating in this conversation, for the @-mention list (matches MessageThread scope). */
  mentionableCharacters: Character[];
  defaultCharacterId: number | null;
}

export type UnreadItemContext = CommentItemContext | PmItemContext;

function pickDefaultCharacterId(
  controllable: Character[],
  preferredCharacterId: number | undefined | null
): number | null {
  if (controllable.length === 0) return null;
  if (preferredCharacterId) {
    const preferred = controllable.find((c) => c.id === preferredCharacterId);
    if (preferred) return preferred.id;
  }
  return controllable[0].id;
}

/**
 * Loads the reply context for a single Unread inbox item: the content to
 * display, the user's controllable characters (for the reply-as picker), the
 * full mentionable character list (for @-mention autocomplete), and which
 * character to default the reply-as picker to.
 *
 * Comment default: the parent-of-the-replied-comment's character (if
 * controlled), matching ThreadedComment.tsx's nested-reply behavior so
 * conversations continue as the same NPC/character. Mention scope is every
 * character in the game, matching the Common Room.
 * PM default: a character already participating in the conversation. Mention
 * scope is limited to conversation participants, matching MessageThread.
 */
export function useUnreadItemContext(item: UnreadInboxItem, enabled: boolean) {
  return useQuery<UnreadItemContext>({
    queryKey: [
      'unread-inbox',
      'item-context',
      item.kind,
      item.gameId,
      item.kind === 'comment' ? item.commentId : item.conversationId,
      item.kind === 'private_message' ? item.messageId : undefined,
    ],
    enabled,
    queryFn: async () => {
      const controllableCharacters = await resolveReplyCharacters(item.gameId);

      if (item.kind === 'comment') {
        const [{ comment, parent, rootPostId }, mentionableCharacters] = await Promise.all([
          fetchCommentContext(item.gameId, item.commentId),
          fetchAllGameCharacters(item.gameId),
        ]);
        const preferredCharacterId = parent?.character_id ?? comment.character_id;
        return {
          kind: 'comment',
          previewMessage: {
            content: comment.content,
            createdAt: comment.created_at,
            authorUsername: comment.author_username,
            characterId: comment.character_id,
            characterName: comment.character_name,
            characterAvatarUrl: comment.character_avatar_url ?? null,
          },
          parentMessage: parent
            ? {
                content: parent.content,
                createdAt: parent.created_at,
                authorUsername: parent.author_username,
                characterId: parent.character_id,
                characterName: parent.character_name,
                characterAvatarUrl: parent.character_avatar_url ?? null,
                isDeleted: parent.is_deleted,
              }
            : null,
          rootPostId,
          controllableCharacters,
          mentionableCharacters,
          defaultCharacterId: pickDefaultCharacterId(controllableCharacters, preferredCharacterId),
          isReplyDisabled: comment.is_deleted || comment.is_draft,
        };
      }

      const [messages, mentionableCharacters, participantCharacterIds] = await Promise.all([
        fetchPmContext(item.gameId, item.conversationId, item.messageId),
        fetchAllGameCharacters(item.gameId),
        fetchConversationParticipantCharacterIds(item.gameId, item.conversationId),
      ]);
      // The server returns the notified message last, preceded by at most one
      // message. If the target is missing the response is empty rather than a
      // full thread (the query is scoped to it), so there is no meaningful
      // fallback row — the card renders blank and messages[-1] stays undefined.
      const messageIndex = messages.findIndex((m) => m.id === item.messageId);
      const lastMessageIndex = messageIndex === -1 ? messages.length - 1 : messageIndex;
      const lastMessage = messages[lastMessageIndex];
      // The message immediately before it in the thread is the conversational
      // context the reader most likely needs; absent when this opened the thread.
      const precedingMessage = lastMessageIndex > 0 ? messages[lastMessageIndex - 1] : null;
      const participantSet = new Set(participantCharacterIds);
      // Matches MessageThread's participantCharacters scoping: the reply-as
      // picker only offers characters already in this conversation, not every
      // character the user controls in the game.
      const participantControllableCharacters = controllableCharacters.filter((c) => participantSet.has(c.id));
      const preferredCharacterId = participantCharacterIds.find((id) =>
        controllableCharacters.some((c) => c.id === id)
      );

      return {
        kind: 'private_message',
        previewMessage: {
          content: lastMessage?.content ?? '',
          createdAt: lastMessage?.created_at ?? null,
          authorUsername: lastMessage?.sender_username ?? null,
          characterId: lastMessage?.sender_character_id ?? null,
          characterName: lastMessage?.sender_character_name ?? null,
          characterAvatarUrl: lastMessage?.sender_avatar_url ?? null,
        },
        parentMessage: precedingMessage
          ? {
              content: precedingMessage.content,
              createdAt: precedingMessage.created_at,
              authorUsername: precedingMessage.sender_username ?? null,
              characterId: precedingMessage.sender_character_id ?? null,
              characterName: precedingMessage.sender_character_name ?? null,
              characterAvatarUrl: precedingMessage.sender_avatar_url ?? null,
              isDeleted: precedingMessage.is_deleted ?? false,
            }
          : null,
        controllableCharacters: participantControllableCharacters,
        mentionableCharacters: mentionableCharacters.filter((c) => participantSet.has(c.id)),
        defaultCharacterId: pickDefaultCharacterId(participantControllableCharacters, preferredCharacterId),
      };
    },
  });
}
