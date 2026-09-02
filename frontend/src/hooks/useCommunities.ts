import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '../lib/api';
import type {
  CreateCommunityBanRequest,
  CreateCommunityDocumentRequest,
  CreateCommunityWebhookRequest,
  UpdateCommunityDocumentRequest,
  UpdateCommunityWebhookRequest,
  CreateCommunityRequest,
  UpdateCommunityProfileRequest,
  UpdateCommunityRequest,
} from '../types/communities';

/** Shared cache key so mutations and reads cannot drift apart. */
const COMMUNITIES_QUERY_KEY = ['admin', 'communities'] as const;

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
const ACTIVE_COMMUNITIES_QUERY_KEY = ['communities', 'active'] as const;

/** Cache key for one community's profile. */
const communityQueryKey = (slug: string) => ['communities', slug] as const;

/** Cache key for one community's moderator roster. */
const moderatorsQueryKey = (slug: string) =>
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

/** Cache key for one community's banlist. */
const bansQueryKey = (slug: string) => ['communities', slug, 'bans'] as const;

/** Cache key for one community's ban audit log. */
const banEventsQueryKey = (slug: string) =>
  ['communities', slug, 'ban-events'] as const;

/**
 * A community's banlist plus the ban and unban mutations.
 *
 * `enabled` holds the request back until the caller knows the viewer may
 * moderate; the endpoint 403s otherwise, and firing it regardless would seed
 * the cache with a guaranteed error for every ordinary visitor.
 *
 * Both mutations invalidate the AUDIT LOG as well as the list. They are written
 * in one transaction server-side, so a refreshed banlist beside a stale log
 * would show the two disagreeing about what just happened.
 */
export function useCommunityBans(slug: string | undefined, enabled = true) {
  const queryClient = useQueryClient();
  const key = bansQueryKey(slug ?? '');

  const { data, isLoading, isError, error } = useQuery({
    queryKey: key,
    queryFn: () => apiClient.communities.listBans(slug!).then((res) => res.data),
    enabled: Boolean(slug) && enabled,
  });

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: key });
    queryClient.invalidateQueries({ queryKey: banEventsQueryKey(slug ?? '') });
    // is_banned rides on the community payloads, so a ban that lands changes
    // what the affected user's picker should offer. Refreshed here because the
    // listing is the picker's source.
    queryClient.invalidateQueries({ queryKey: ACTIVE_COMMUNITIES_QUERY_KEY });
  };

  const banUser = useMutation({
    mutationFn: (payload: CreateCommunityBanRequest) =>
      apiClient.communities.banUser(slug!, payload).then((res) => res.data),
    onSuccess: invalidate,
  });

  const unbanUser = useMutation({
    mutationFn: (userId: number) => apiClient.communities.unbanUser(slug!, userId),
    onSuccess: invalidate,
  });

  return { bans: data ?? [], isLoading, isError, error, banUser, unbanUser };
}

/**
 * A community's ban audit log, newest first.
 *
 * Paged rather than fetched whole: the log is append-only and only grows.
 */
export function useCommunityBanEvents(
  slug: string | undefined,
  { enabled = true, limit = 50, offset = 0 }: {
    enabled?: boolean;
    limit?: number;
    offset?: number;
  } = {}
) {
  const { data, isLoading, isError, error } = useQuery({
    queryKey: [...banEventsQueryKey(slug ?? ''), limit, offset] as const,
    queryFn: () =>
      apiClient.communities.listBanEvents(slug!, { limit, offset }).then((res) => res.data),
    enabled: Boolean(slug) && enabled,
  });

  return { events: data ?? [], isLoading, isError, error };
}

/**
 * The ACTIVE communities the current user may actually create a game in.
 *
 * Distinct from useActiveCommunities, which is the browse listing and
 * deliberately still shows communities the user is banned from -- a ban blocks
 * joining, not looking. This one is for the create/edit game picker, where
 * offering a community the server will refuse is a dead end.
 *
 * Reads `is_banned` off the community records rather than fetching a separate
 * banlist: the flag is computed per request on the same payload, so the picker
 * costs no extra round-trip and the two can never disagree about who is banned.
 *
 * Filtering here is CONVENIENCE. Enforcement is the ban check on game creation;
 * if this list were wrong the server would still refuse.
 */
export function useSelectableCommunities() {
  const { communities, isLoading, isError, error } = useActiveCommunities();

  return {
    // Treats a missing is_banned as "not banned": an older cached payload
    // predating the field should not empty the picker, and the server refuses
    // a banned choice regardless.
    communities: communities.filter((c) => !c.is_banned),
    isLoading,
    isError,
    error,
  };
}

// ---------------------------------------------------------------- documents

/**
 * Cache keys for a community's documents.
 *
 * The public list and the moderator list are SEPARATE keys, not one key with a
 * flag. They return different data for the same community -- the moderator's
 * includes drafts -- so sharing a key would let whichever loaded last serve the
 * other's consumer, leaking a draft into a public view or blanking a manage
 * screen.
 */
const documentsQueryKey = (slug: string) => ['communities', slug, 'documents'] as const;
const allDocumentsQueryKey = (slug: string) =>
  ['communities', slug, 'documents', 'manage'] as const;
const gameDocumentsQueryKey = (gameId: number) =>
  ['games', gameId, 'community-documents'] as const;

/**
 * A community's PUBLISHED documents, for the public community page.
 *
 * Open to any authenticated user -- a community's rules are what someone reads
 * before deciding whether to join.
 */
export function useCommunityDocuments(slug: string | undefined, enabled = true) {
  const { data, isLoading, isError, error } = useQuery({
    queryKey: documentsQueryKey(slug ?? ''),
    queryFn: () => apiClient.communities.listDocuments(slug!).then((res) => res.data),
    enabled: Boolean(slug) && enabled,
  });

  return { documents: data ?? [], isLoading, isError, error };
}

/**
 * Every document including drafts, plus the write mutations. Moderator-only.
 *
 * `enabled` gates the request on the caller's standing so a non-moderator does
 * not fire a request that is certain to 403. The server is the authority
 * regardless -- this only avoids a pointless round trip.
 */
export function useManageCommunityDocuments(slug: string | undefined, enabled = true) {
  const queryClient = useQueryClient();
  const key = allDocumentsQueryKey(slug ?? '');

  const { data, isLoading, isError, error } = useQuery({
    queryKey: key,
    queryFn: () => apiClient.communities.listAllDocuments(slug!).then((res) => res.data),
    enabled: Boolean(slug) && enabled,
  });

  /**
   * Refresh both lists after any write.
   *
   * The public list must be invalidated too: publishing or deleting changes
   * what an ordinary visitor sees, and it is a different cache key. Without
   * this a moderator would publish a document and still see the old public
   * page. The per-game list is invalidated for the same reason -- a published
   * document appears on the Info tab of every game in the community.
   */
  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: key });
    queryClient.invalidateQueries({ queryKey: documentsQueryKey(slug ?? '') });
    // Predicate rather than an exact key: the Info tab lists are keyed by game
    // id, and one community owns many games. This catches all of them without
    // the hook needing to know which games exist.
    queryClient.invalidateQueries({
      predicate: (query) =>
        query.queryKey[0] === 'games' && query.queryKey[2] === 'community-documents',
    });
  };

  const createDocument = useMutation({
    mutationFn: (payload: CreateCommunityDocumentRequest) =>
      apiClient.communities.createDocument(slug!, payload).then((res) => res.data),
    onSuccess: invalidate,
  });

  const updateDocument = useMutation({
    mutationFn: ({ id, data }: { id: number; data: UpdateCommunityDocumentRequest }) =>
      apiClient.communities.updateDocument(slug!, id, data).then((res) => res.data),
    onSuccess: invalidate,
  });

  const deleteDocument = useMutation({
    mutationFn: (id: number) => apiClient.communities.deleteDocument(slug!, id),
    onSuccess: invalidate,
  });

  return {
    documents: data ?? [],
    isLoading,
    isError,
    error,
    createDocument,
    updateDocument,
    deleteDocument,
  };
}

/**
 * The published documents of the community owning a game -- the Info tab.
 *
 * Returns an empty list for a legacy game with no community, so the caller
 * renders no community section rather than special-casing a missing one.
 */
export function useGameCommunityDocuments(gameId: number | undefined, enabled = true) {
  const { data, isLoading, isError, error } = useQuery({
    queryKey: gameDocumentsQueryKey(gameId ?? 0),
    queryFn: () =>
      apiClient.communities.listGameCommunityDocuments(gameId!).then((res) => res.data),
    enabled: Boolean(gameId) && enabled,
  });

  return { documents: data ?? [], isLoading, isError, error };
}

/**
 * Query key for a community's webhooks.
 *
 * Only ONE key, unlike documents which need a public and a moderator variant.
 * There is no public webhook list at all: every endpoint is moderator-gated,
 * because the rows carry a channel credential and its delivery status.
 */
const webhooksQueryKey = (slug: string) => ['communities', slug, 'webhooks'] as const;

/**
 * Moderator: a community's Discord webhooks (req 9).
 *
 * 🔴 Every `url` in this data is MASKED. The real URL is a credential the server
 * never returns, so nothing here may be sent back as an update's `url` -- doing
 * so would overwrite the stored credential with bullet characters. Omit `url`
 * on any save that is not a deliberate rotation.
 */
export function useCommunityWebhooks(slug: string | undefined, enabled = true) {
  const queryClient = useQueryClient();
  const key = webhooksQueryKey(slug ?? '');

  const { data, isLoading, isError, error } = useQuery({
    queryKey: key,
    queryFn: () => apiClient.communities.listWebhooks(slug!).then((res) => res.data),
    enabled: Boolean(slug) && enabled,
  });

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: key });
  };

  const createWebhook = useMutation({
    mutationFn: (payload: CreateCommunityWebhookRequest) =>
      apiClient.communities.createWebhook(slug!, payload).then((res) => res.data),
    onSuccess: invalidate,
  });

  const updateWebhook = useMutation({
    mutationFn: ({ id, data }: { id: number; data: UpdateCommunityWebhookRequest }) =>
      apiClient.communities.updateWebhook(slug!, id, data).then((res) => res.data),
    onSuccess: invalidate,
  });

  const deleteWebhook = useMutation({
    mutationFn: (id: number) => apiClient.communities.deleteWebhook(slug!, id),
    onSuccess: invalidate,
  });

  /**
   * Send a test message and refresh afterwards.
   *
   * Invalidates on SUCCESS AND FAILURE, unlike the mutations above: a failed
   * test stamps `last_error` on the row, and that new diagnosis is exactly what
   * the moderator pressed the button to obtain. Refreshing only on success
   * would leave the stale status on screen in the one case that matters.
   */
  const testWebhook = useMutation({
    mutationFn: (id: number) =>
      apiClient.communities.testWebhook(slug!, id).then((res) => res.data),
    onSettled: invalidate,
  });

  return {
    webhooks: data ?? [],
    isLoading,
    isError,
    error,
    createWebhook,
    updateWebhook,
    deleteWebhook,
    testWebhook,
  };
}
