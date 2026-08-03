import { useInfiniteQuery, useQuery } from '@tanstack/react-query';
import { apiClient } from '../lib/api';

const COMMENTS_PER_PAGE = 20;

/**
 * Hook to fetch recent comments with their parent context
 * Supports infinite scrolling via cursor-based pagination
 *
 * When `unreadOnly` is true the server omits comments the user has manually
 * marked as read. Filtering server-side (rather than on the fetched pages)
 * keeps pagination and the total count accurate. `unreadOnly` is part of the
 * query key so filtered and unfiltered lists cache independently.
 */
export function useRecentComments(gameId: number | undefined, unreadOnly: boolean = false) {
  return useInfiniteQuery({
    queryKey: ['games', gameId, 'recentComments', { unreadOnly }],
    queryFn: async ({ pageParam }: { pageParam?: number }) => {
      if (!gameId) {
        throw new Error('Game ID is required');
      }

      const response = await apiClient.messages.getRecentComments(
        gameId,
        COMMENTS_PER_PAGE,
        pageParam ?? 0,
        unreadOnly
      );
      return response.data;
    },
    initialPageParam: 0,
    getNextPageParam: (lastPage, allPages) => {
      // If the last page had fewer items than the page size, we're at the end
      if (lastPage.comments.length < COMMENTS_PER_PAGE) {
        return undefined;
      }
      // Calculate next offset based on pages loaded (defensive approach)
      // This doesn't rely on the API response including an offset field
      return allPages.length * COMMENTS_PER_PAGE;
    },
    enabled: !!gameId,
    // refetchOnWindowFocus: false is the global default - preserves scroll position
  });
}

/**
 * Hook to fetch the total comment count for a game
 * Used for pagination info and empty states
 */
export function useTotalCommentCount(gameId: number | undefined) {
  return useQuery({
    queryKey: ['games', gameId, 'totalCommentCount'],
    queryFn: async () => {
      if (!gameId) {
        throw new Error('Game ID is required');
      }

      const count = await apiClient.messages.getTotalCommentCount(gameId);
      return count;
    },
    enabled: !!gameId,
    // refetchOnWindowFocus: false is the global default - preserves scroll position
  });
}
