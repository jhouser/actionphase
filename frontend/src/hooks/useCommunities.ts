import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '../lib/api';
import type {
  CreateCommunityRequest,
  UpdateCommunityRequest,
} from '../types/communities';

/** Shared cache key so mutations and reads cannot drift apart. */
export const COMMUNITIES_QUERY_KEY = ['admin', 'communities'] as const;

/**
 * Admin community management: the list plus create and update mutations.
 *
 * Scoped to the admin surface -- creating a community and assigning its owner
 * are site-admin operations. Moderator-facing hooks arrive in later phases.
 */
export function useCommunities() {
  const queryClient = useQueryClient();

  const { data, isLoading, isError, error } = useQuery({
    queryKey: COMMUNITIES_QUERY_KEY,
    queryFn: () => apiClient.communities.listCommunities().then((res) => res.data),
  });

  const createCommunity = useMutation({
    mutationFn: (payload: CreateCommunityRequest) =>
      apiClient.communities.createCommunity(payload).then((res) => res.data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: COMMUNITIES_QUERY_KEY });
    },
  });

  const updateCommunity = useMutation({
    mutationFn: ({ id, data }: { id: number; data: UpdateCommunityRequest }) =>
      apiClient.communities.updateCommunity(id, data).then((res) => res.data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: COMMUNITIES_QUERY_KEY });
    },
  });

  return {
    communities: data ?? [],
    isLoading,
    isError,
    error,
    createCommunity,
    updateCommunity,
  };
}
