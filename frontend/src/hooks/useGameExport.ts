import { useCallback, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '../lib/api';
import { isExportInProgress } from '../types/exports';
import type { GameExport } from '../types/exports';

/** How often to re-check a job that is still assembling. */
const POLL_INTERVAL_MS = 3000;

const gameExportQueryKey = (gameId: number) => ['game-export', gameId] as const;

interface UseGameExportOptions {
  /**
   * Whether to look up an existing export on mount. Exports only exist for
   * completed games, so callers pass false elsewhere to avoid a guaranteed 404.
   */
  enabled?: boolean;
}

interface UseGameExportResult {
  /** The current export job, if one exists. */
  export: GameExport | undefined;
  /** True while a job is queued or assembling. */
  isWorking: boolean;
  /** True while the initial status lookup is in flight. */
  isLoading: boolean;
  /** True while a new export request is being submitted. */
  isRequesting: boolean;
  /** Error from requesting an export, if any. */
  requestError: Error | null;
  /** Queue an export (or adopt an existing one). */
  requestExport: () => void;
  /** Clear a surfaced request error. */
  dismissError: () => void;
}

/**
 * Manages the archive export lifecycle for one game.
 *
 * Polls only while a job is pending or running: once it completes or fails
 * there is nothing further to learn, so polling stops rather than hitting the
 * API forever on an idle page.
 */
export function useGameExport(
  gameId: number,
  { enabled = true }: UseGameExportOptions = {}
): UseGameExportResult {
  const queryClient = useQueryClient();
  const [requestError, setRequestError] = useState<Error | null>(null);

  const statusQuery = useQuery({
    queryKey: gameExportQueryKey(gameId),
    queryFn: async () => {
      try {
        const response = await apiClient.exports.getLatestExport(gameId);
        return response.data;
      } catch (error) {
        // 404 means "no export yet", which is a normal state rather than a
        // failure; anything else is a real error worth surfacing.
        if (isNotFound(error)) return null;
        throw error;
      }
    },
    enabled: enabled && Number.isFinite(gameId) && gameId > 0,
    refetchInterval: (query) => {
      const data = query.state.data as GameExport | null | undefined;
      if (!data) return false;
      return isExportInProgress(data.status) ? POLL_INTERVAL_MS : false;
    },
  });

  const requestMutation = useMutation({
    mutationFn: async () => {
      const response = await apiClient.exports.requestExport(gameId);
      return response.data;
    },
    onSuccess: (data) => {
      setRequestError(null);
      // Seed the cache so polling picks up immediately without a round trip.
      queryClient.setQueryData(gameExportQueryKey(gameId), data);
    },
    onError: (error: Error) => {
      setRequestError(error);
    },
  });

  const requestExport = useCallback(() => {
    setRequestError(null);
    requestMutation.mutate();
  }, [requestMutation]);

  const dismissError = useCallback(() => setRequestError(null), []);

  const current = statusQuery.data ?? undefined;

  return {
    export: current,
    isWorking: current ? isExportInProgress(current.status) : false,
    isLoading: statusQuery.isLoading,
    isRequesting: requestMutation.isPending,
    requestError,
    requestExport,
    dismissError,
  };
}

/** Detects an axios-style 404 without depending on axios types. */
function isNotFound(error: unknown): boolean {
  return (
    typeof error === 'object' &&
    error !== null &&
    'response' in error &&
    typeof (error as { response?: { status?: number } }).response?.status === 'number' &&
    (error as { response: { status: number } }).response.status === 404
  );
}
