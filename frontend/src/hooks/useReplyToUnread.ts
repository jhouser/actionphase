import { useMutation, useQueryClient } from '@tanstack/react-query';
import { replyToComment, replyToPm } from '@/utils/unreadInboxApi';
import type { UnreadInboxItem } from '@/types/unreadInbox';
import { postCachingService } from '@/services/PostCachingService';

export interface ReplyToUnreadParams {
  item: UnreadInboxItem;
  characterId: number;
  content: string;
  /** Required for comment replies; ignored for private messages. */
  rootPostId?: number;
}

/**
 * Sends a reply to the source of an Unread inbox item (a comment or a
 * private message) and marks the originating notification as read.
 */
export function useReplyToUnread() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({ item, characterId, content, rootPostId }: ReplyToUnreadParams) => {
      if (item.kind === 'comment') {
        if (rootPostId === undefined) {
          throw new Error('rootPostId is required to reply to a comment');
        }
        await replyToComment({
          gameId: item.gameId,
          notificationId: item.notification.id,
          parentCommentId: item.commentId,
          rootPostId,
          characterId,
          content,
        });
      } else {
        await replyToPm({
          gameId: item.gameId,
          notificationId: item.notification.id,
          conversationId: item.conversationId,
          characterId,
          content,
        });
      }
    },
    onSuccess: (_, v) => {
      queryClient.invalidateQueries({ queryKey: ['unread-inbox'] });
      queryClient.invalidateQueries({ queryKey: ['notifications'] });
      queryClient.invalidateQueries({ queryKey: ['dashboard'] });
      if (v.item.kind === 'comment') {
        postCachingService.remove(postCachingService.createAutosaveId('post-reply', v.item.commentId));
      }
      else {
        postCachingService.remove(postCachingService.createAutosaveId('conversation', v.item.conversationId));
      }
    },
  });
}
