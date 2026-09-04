import { useQuery } from '@tanstack/react-query';
import { useSearchParams } from 'react-router-dom';
import { useCallback, useMemo } from 'react';
import { apiClient } from '../lib/api';
import { useAdminMode } from './useAdminMode';
import type {
  GameListingFilters,
  GameState,
  ParticipationFilter,
  SortBy,
} from '../types/games';

/**
 * Custom hook for game listing with URL-synced filters
 *
 * Features:
 * - Fetches filtered games from API
 * - Synchronizes filter state with URL query parameters
 * - Provides methods to update individual filters
 * - Browser back/forward navigation support
 *
 * @example
 * ```tsx
 * const { games, metadata, filters, setStates, setSortBy, isLoading } = useGameListing();
 * ```
 */
export function useGameListing() {
  const [searchParams, setSearchParams] = useSearchParams();
  const { adminModeEnabled } = useAdminMode();

  // Parse filters from URL
  const filters = useMemo<GameListingFilters>(() => {
    const searchParam = searchParams.get('search');
    const statesParam = searchParams.get('states');
    const participationParam = searchParams.get('participation');
    const openSpotsParam = searchParams.get('has_open_spots');
    const communityParam = searchParams.get('community_id');
    const sortByParam = searchParams.get('sort_by');
    const pageParam = searchParams.get('page');
    const pageSizeParam = searchParams.get('page_size');

    return {
      search: searchParam || undefined,
      states: statesParam ? (statesParam.split(',') as GameState[]) : undefined,
      participation: participationParam as ParticipationFilter | undefined,
      has_open_spots: openSpotsParam === 'true' ? true : openSpotsParam === 'false' ? false : undefined,
      // Parsed defensively: a junk community_id in a shared URL should show
      // everything rather than nothing.
      community_id: communityParam && Number(communityParam) > 0 ? Number(communityParam) : undefined,
      sort_by: (sortByParam as SortBy) || 'recent_activity',
      admin_mode: adminModeEnabled, // Add admin mode from context
      page: pageParam ? parseInt(pageParam, 10) : 1,
      page_size: pageSizeParam ? parseInt(pageSizeParam, 10) : 20,
    };
  }, [searchParams, adminModeEnabled]);

  // Fetch games with current filters
  const query = useQuery({
    queryKey: ['games', 'filtered', filters],
    queryFn: async () => {
      const response = await apiClient.games.getFilteredGames(filters);
      return response.data;
    },
    // Keep data fresh when returning to the page
    staleTime: 60000, // 1 minute
    // refetchOnWindowFocus: false is the global default - preserves pagination and filters
  });

  // Helper to update search params
  const updateFilters = useCallback(
    (updates: Partial<GameListingFilters>) => {
      const newParams = new URLSearchParams(searchParams);

      // Update or remove search
      if ('search' in updates) {
        if (updates.search && updates.search.trim()) {
          newParams.set('search', updates.search.trim());
        } else {
          newParams.delete('search');
        }
      }

      // Update or remove states
      if ('states' in updates) {
        if (updates.states && updates.states.length > 0) {
          newParams.set('states', updates.states.join(','));
        } else {
          newParams.delete('states');
        }
      }

      // Update or remove participation filter
      if ('participation' in updates) {
        if (updates.participation) {
          newParams.set('participation', updates.participation);
        } else {
          newParams.delete('participation');
        }
      }

      // Update or remove community_id
      if ('community_id' in updates) {
        if (updates.community_id) {
          newParams.set('community_id', updates.community_id.toString());
        } else {
          newParams.delete('community_id');
        }
      }

      // Update or remove has_open_spots
      if ('has_open_spots' in updates) {
        if (updates.has_open_spots !== null && updates.has_open_spots !== undefined) {
          newParams.set('has_open_spots', updates.has_open_spots.toString());
        } else {
          newParams.delete('has_open_spots');
        }
      }

      // Update or remove sort_by
      if ('sort_by' in updates) {
        if (updates.sort_by && updates.sort_by !== 'recent_activity') {
          newParams.set('sort_by', updates.sort_by);
        } else {
          newParams.delete('sort_by'); // Default to recent_activity
        }
      }

      // Update or remove page
      if ('page' in updates) {
        if (updates.page && updates.page !== 1) {
          newParams.set('page', updates.page.toString());
        } else {
          newParams.delete('page'); // Default to page 1
        }
      }

      // Update or remove page_size
      if ('page_size' in updates) {
        if (updates.page_size && updates.page_size !== 20) {
          newParams.set('page_size', updates.page_size.toString());
        } else {
          newParams.delete('page_size'); // Default to 20
        }
      }

      setSearchParams(newParams);
    },
    [searchParams, setSearchParams]
  );

  // Convenience methods for updating individual filters
  const setSearch = useCallback(
    (search: string) => {
      updateFilters({ search });
    },
    [updateFilters]
  );

  const setStates = useCallback(
    (states: GameState[]) => {
      updateFilters({ states });
    },
    [updateFilters]
  );

  const setParticipation = useCallback(
    (participation?: ParticipationFilter) => {
      updateFilters({ participation });
    },
    [updateFilters]
  );

  const setHasOpenSpots = useCallback(
    (hasOpenSpots?: boolean | null) => {
      updateFilters({ has_open_spots: hasOpenSpots === null ? undefined : hasOpenSpots });
    },
    [updateFilters]
  );

  const setCommunity = useCallback(
    (communityId?: number) => {
      // Back to page 1: the current page may not exist in the narrowed set.
      updateFilters({ community_id: communityId, page: 1 });
    },
    [updateFilters]
  );

  const setSortBy = useCallback(
    (sortBy: SortBy) => {
      updateFilters({ sort_by: sortBy });
    },
    [updateFilters]
  );

  const setPage = useCallback(
    (page: number) => {
      updateFilters({ page });
    },
    [updateFilters]
  );

  const setPageSize = useCallback(
    (pageSize: number) => {
      updateFilters({ page_size: pageSize, page: 1 }); // Reset to page 1 when changing page size
    },
    [updateFilters]
  );

  // Clear all filters
  const clearFilters = useCallback(() => {
    setSearchParams(new URLSearchParams());
  }, [setSearchParams]);

  return {
    // Data
    games: query.data?.games ?? [],
    metadata: query.data?.metadata ?? {
      total_count: 0,
      filtered_count: 0,
      available_states: [],
      page: 1,
      page_size: 20,
      total_pages: 1,
      has_next_page: false,
      has_previous_page: false,
    },

    // Current filter state
    filters,

    // Filter setters
    setSearch,
    setStates,
    setParticipation,
    setHasOpenSpots,
    setCommunity,
    setSortBy,
    setPage,
    setPageSize,
    updateFilters,
    clearFilters,

    // Query state
    isLoading: query.isLoading,
    isError: query.isError,
    error: query.error,
    refetch: query.refetch,
  };
}
