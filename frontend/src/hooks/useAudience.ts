import { useQuery, useInfiniteQuery } from '@tanstack/react-query';
import { apiClient } from '../lib/api';

/**
 * Hook to fetch valid participant names for the conversation filter UI.
 * Returns all participants when selectedNames is empty; narrows to co-participants
 * of all selected names when non-empty.
 */
export function useConversationParticipants(gameId: number, selectedCharacterIds: number[]) {
  return useQuery({
    queryKey: ['conversation-participants', gameId, selectedCharacterIds],
    queryFn: async () => {
      const response = await apiClient.games.getConversationParticipants(gameId, selectedCharacterIds);
      return response.data.participants;
    },
    enabled: !!gameId,
  });
}

/**
 * Hook to fetch all private conversations (infinite scroll for GM/audience)
 */
export function useAllPrivateConversations(
  gameId: number,
  options?: { participantCharacterIds?: number[] }
) {
  return useInfiniteQuery({
    queryKey: ['all-private-conversations', gameId, options],
    queryFn: async ({ pageParam = 0 }) => {
      const response = await apiClient.games.listAllPrivateConversations(gameId, {
        ...options,
        offset: pageParam as number,
        limit: 20,
      });
      return response.data;
    },
    getNextPageParam: (lastPage, pages) => {
      const loadedCount = pages.reduce(
        (sum, page) => sum + (page.conversations?.length || 0),
        0
      );
      return loadedCount < lastPage.total ? loadedCount : undefined;
    },
    initialPageParam: 0,
    enabled: !!gameId,
    refetchInterval: 30000, // Refetch every 30 seconds
    // refetchOnWindowFocus: false is the global default - refetchInterval provides sufficient freshness
  });
}

/**
 * Hook to fetch messages for a specific conversation (GM/audience only)
 */
export function useAudienceConversationMessages(
  gameId: number,
  conversationId: string | null
) {
  return useQuery({
    queryKey: ['audience-conversation-messages', gameId, conversationId],
    queryFn: async () => {
      const response = await apiClient.games.getAudienceConversationMessages(gameId, conversationId!);
      return response.data.messages;
    },
    enabled: !!conversationId && !!gameId,
  });
}
