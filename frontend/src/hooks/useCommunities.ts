import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '../lib/api';
import type {
  CreateCommunityRequest,
  UpdateCommunityProfileRequest,
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

/**
 * Cache key for the PUBLIC listing.
 *
 * Deliberately distinct from COMMUNITIES_QUERY_KEY: the admin list carries
 * every community including inactive ones, while this one carries only active
 * ones. Sharing a key would let one surface serve the other's data.
 */
export const ACTIVE_COMMUNITIES_QUERY_KEY = ['communities', 'active'] as const;

/** Cache key for one community's profile. */
export const communityQueryKey = (slug: string) => ['communities', slug] as const;

/** Cache key for one community's moderator roster. */
export const moderatorsQueryKey = (slug: string) =>
  ['communities', slug, 'moderators'] as const;

/** The active communities, for browsing surfaces. */
export function useActiveCommunities() {
  const { data, isLoading, isError, error } = useQuery({
    queryKey: ACTIVE_COMMUNITIES_QUERY_KEY,
    queryFn: () => apiClient.communities.listActiveCommunities().then((res) => res.data),
  });

  return { communities: data ?? [], isLoading, isError, error };
}

/** One community's profile, by slug. */
export function useCommunity(slug: string | undefined) {
  const { data, isLoading, isError, error } = useQuery({
    queryKey: communityQueryKey(slug ?? ''),
    queryFn: () => apiClient.communities.getCommunity(slug!).then((res) => res.data),
    enabled: Boolean(slug),
  });

  return { community: data, isLoading, isError, error };
}

/**
 * Moderator-level profile editing for one community.
 *
 * Separate from useCommunities()'s admin `updateCommunity`: that one is keyed
 * by id and can reassign ownership, and lives on the admin surface.
 *
 * Invalidates BOTH cache keys on success. The profile page reads
 * communityQueryKey(slug) while every game form's picker reads
 * ACTIVE_COMMUNITIES_QUERY_KEY -- refreshing only the first would leave a
 * renamed community showing its old name in every dropdown until the cache
 * expired.
 */
export function useUpdateCommunityProfile(slug: string | undefined) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (payload: UpdateCommunityProfileRequest) =>
      apiClient.communities.updateCommunityProfile(slug!, payload).then((res) => res.data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: communityQueryKey(slug ?? '') });
      queryClient.invalidateQueries({ queryKey: ACTIVE_COMMUNITIES_QUERY_KEY });
      // The admin listing shows the same name; leaving it stale would make the
      // admin table disagree with the community page.
      queryClient.invalidateQueries({ queryKey: COMMUNITIES_QUERY_KEY });
    },
  });
}

/**
 * A community's moderator roster plus the owner-only mutations.
 *
 * `enabled` lets a caller hold the request back until it knows the viewer may
 * moderate -- the endpoint 403s otherwise, and firing it regardless would put a
 * guaranteed error in the cache for every ordinary visitor.
 */
export function useCommunityModerators(slug: string | undefined, enabled = true) {
  const queryClient = useQueryClient();
  const key = moderatorsQueryKey(slug ?? '');

  const { data, isLoading, isError, error } = useQuery({
    queryKey: key,
    queryFn: () => apiClient.communities.listModerators(slug!).then((res) => res.data),
    enabled: Boolean(slug) && enabled,
  });

  const addModerator = useMutation({
    mutationFn: (userId: number) =>
      apiClient.communities.addModerator(slug!, { user_id: userId }).then((res) => res.data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: key });
    },
  });

  const removeModerator = useMutation({
    mutationFn: (userId: number) => apiClient.communities.removeModerator(slug!, userId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: key });
    },
  });

  return {
    moderators: data ?? [],
    isLoading,
    isError,
    error,
    addModerator,
    removeModerator,
  };
}
