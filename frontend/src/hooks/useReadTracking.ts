import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '../lib/api';
import type { MarkPostReadRequest, ManualCommentReads, PostUnreadComments } from '../types/messages';
import { useMemo } from 'react';

/**
 * Hook to mark a post as read
 */
export function useMarkPostAsRead() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({ gameId, postId, data }: { gameId: number; postId: number; data?: MarkPostReadRequest }) => {
      const response = await apiClient.messages.markPostAsRead(gameId, postId, data || {});
      return response.data;
    },
    onSuccess: async (_, variables) => {
      // Use refetchQueries instead of invalidateQueries to bypass staleTime
      // This ensures fresh data is fetched immediately after marking as read
      await Promise.all([
        queryClient.refetchQueries({ queryKey: ['readMarkers', variables.gameId] }),
        queryClient.refetchQueries({ queryKey: ['postsUnreadInfo', variables.gameId] }),
        queryClient.refetchQueries({ queryKey: ['unreadCommentIDs', variables.gameId] }),
      ]);
    },
  });
}

/**
 * Hook to fetch unread comment IDs for all posts in a game
 * Returns specific comment IDs that are "new since last visit"
 */
export function useUnreadCommentIDs(gameId: number | undefined) {
  return useQuery({
    queryKey: ['unreadCommentIDs', gameId],
    queryFn: async () => {
      if (!gameId) throw new Error('Game ID required');
      const response = await apiClient.messages.getUnreadCommentIDs(gameId);
      return response.data;
    },
    enabled: !!gameId,
    refetchOnWindowFocus: false,
    staleTime: 5 * 60 * 1000, // Consider fresh for 5 minutes
  });
}

/**
 * Helper hook to get unread comment IDs for a specific post
 */
export function usePostUnreadCommentIDs(gameId: number | undefined, postId: number | undefined) {
  const { data: unreadComments = [] } = useUnreadCommentIDs(gameId);

  return useMemo(() => {
    if (!postId) return [];
    const postData = unreadComments.find(pc => pc.post_id === postId);
    return postData?.unread_comment_ids || [];
  }, [unreadComments, postId]);
}

/**
 * Hook to fetch all comment IDs manually marked as read by the user in a game
 */
export function useManualReadCommentIDs(gameId: number | undefined) {
  return useQuery({
    queryKey: ['manualReadCommentIDs', gameId],
    queryFn: async () => {
      if (!gameId) throw new Error('Game ID required');
      const response = await apiClient.messages.getManualReadCommentIDs(gameId);
      return response.data as ManualCommentReads[];
    },
    enabled: !!gameId,
    refetchOnWindowFocus: false,
    staleTime: 5 * 60 * 1000,
  });
}

/**
 * Helper hook to get manually read comment IDs for a specific post
 */
export function usePostManualReadCommentIDs(gameId: number | undefined, postId: number | undefined) {
  const { data: manualReads = [] } = useManualReadCommentIDs(gameId);

  return useMemo(() => {
    if (!postId) return [];
    const postData = manualReads.find(mr => mr.post_id === postId);
    return postData?.read_comment_ids || [];
  }, [manualReads, postId]);
}

/**
 * Mutation to toggle a single comment's manual read state
 */
export function useToggleCommentRead() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({
      gameId,
      postId,
      commentId,
      read,
    }: {
      gameId: number;
      postId: number;
      commentId: number;
      read: boolean;
    }) => {
      await apiClient.messages.toggleCommentRead(gameId, postId, commentId, read);
    },
    onSuccess: (_, variables) => {
      queryClient.refetchQueries({
        queryKey: ['manualReadCommentIDs', variables.gameId],
      });
    },
  });
}

/**
 * Mutation to mark every comment in a phase as read for the current user.
 * Optimistically clears unread comment badges so the UI updates immediately,
 * then reconciles with the server in the background.
 *
 * The optimistic write clears every cached post rather than only the current
 * phase's: scoping it precisely would need a `getGamePosts` round-trip inside
 * `onMutate`, which would make the update non-optimistic and let an unrelated
 * read failure abort the write. The server-side mutation is already scoped to
 * the phase, and `onSettled` refetches, so any over-clearing here is corrected
 * a moment later — and in practice a phase rarely holds more than one post.
 */
export function useMarkAllCommentsRead() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({ gameId, phaseId }: { gameId: number; phaseId: number }) => {
      await apiClient.messages.markAllCommentsRead(gameId, phaseId);
    },
    onMutate: async ({ gameId }) => {
      const queryKey = ['unreadCommentIDs', gameId];
      await queryClient.cancelQueries({ queryKey });

      const previousUnread = queryClient.getQueryData<PostUnreadComments[]>(queryKey);

      queryClient.setQueryData<PostUnreadComments[]>(queryKey, (old) =>
        (old || []).map((entry) => ({ ...entry, unread_comment_ids: [] }))
      );

      return { previousUnread };
    },
    onError: (_err, variables, context) => {
      if (context?.previousUnread) {
        queryClient.setQueryData(['unreadCommentIDs', variables.gameId], context.previousUnread);
      }
    },
    onSettled: (_data, _err, variables) => {
      queryClient.refetchQueries({ queryKey: ['unreadCommentIDs', variables.gameId] });
      queryClient.refetchQueries({ queryKey: ['manualReadCommentIDs', variables.gameId] });
    },
  });
}
