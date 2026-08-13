import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '../lib/api';
import type { StagedResultPart } from '../types/phases';

export function useUserActionResults(gameId: number) {
  return useQuery({
    queryKey: ['actionResults', 'user', gameId],
    queryFn: async () => {
      const response = await apiClient.phases.getUserResults(gameId);
      return response.data;
    },
    enabled: !!gameId,
  });
}

export function useGameActionResults(gameId: number) {
  return useQuery({
    queryKey: ['actionResults', 'game', gameId],
    queryFn: async () => {
      const response = await apiClient.phases.getGameResults(gameId);
      return response.data;
    },
    enabled: !!gameId,
  });
}

export function useCreateActionResult(gameId: number) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (data: { user_id: number; character_id?: number; action_submission_id?: number; content: string; is_published?: boolean }) => {
      const response = await apiClient.phases.createActionResult(gameId, data);
      return response.data;
    },
    onSuccess: () => {
      // Invalidate action results queries to refetch
      queryClient.invalidateQueries({ queryKey: ['actionResults', 'game', gameId] });
      queryClient.invalidateQueries({ queryKey: ['actionResults', 'user', gameId] });
      queryClient.invalidateQueries({ queryKey: ['unpublishedResultsCount'] });
    },
  });
}

/**
 * Creates a staged result chain: several linked parts revealed on a timer.
 *
 * Invalidates the same query keys as useCreateActionResult — a chain lands in
 * the same result lists as any other result, and its head is immediately
 * visible when the chain is published.
 */
export function useCreateStagedResultChain(gameId: number) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (data: {
      user_id: number;
      character_id?: number;
      action_submission_id?: number;
      parts: StagedResultPart[];
      is_published?: boolean;
    }) => {
      const response = await apiClient.phases.createStagedResultChain(gameId, data);
      return response.data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['actionResults', 'game', gameId] });
      queryClient.invalidateQueries({ queryKey: ['actionResults', 'user', gameId] });
      queryClient.invalidateQueries({ queryKey: ['unpublishedResultsCount'] });
    },
  });
}

/**
 * Appends one part to the end of a draft chain.
 *
 * This is the "write it over several sittings" path: a GM saves an opening
 * beat and comes back later to add the payoff. resultId may be any member of
 * the chain, and a plain unstaged draft is a chain of one, so the same call
 * also converts a draft into a staged chain.
 *
 * Invalidates unpublishedResultsCount alongside the result lists: appending
 * creates a new unpublished row, so the GM's "N results waiting" badge is now
 * stale. This is the difference from useCancelPendingStagedPart, which removes
 * an already-published row and therefore leaves that count alone.
 */
export function useAppendStagedPart(gameId: number) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({ resultId, part }: { resultId: number; part: StagedResultPart }) => {
      const response = await apiClient.phases.appendStagedPart(gameId, resultId, part);
      return response.data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['actionResults', 'game', gameId] });
      queryClient.invalidateQueries({ queryKey: ['actionResults', 'user', gameId] });
      queryClient.invalidateQueries({ queryKey: ['unpublishedResultsCount'] });
    },
  });
}

/**
 * Retimes a staged part that has not yet been revealed.
 *
 * Also invalidates the player-facing list: retiming a *published* pending part
 * changes the unlocks_at the recipient's countdown is rendering, so leaving
 * that cache alone would leave the player counting down to the old time.
 *
 * Does not touch unpublishedResultsCount — a retime never changes how many
 * results are waiting to be published.
 */
export function useUpdateStagedPartDelay(gameId: number) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({ resultId, delayMinutes }: { resultId: number; delayMinutes: number }) => {
      const response = await apiClient.phases.updateStagedPartDelay(gameId, resultId, delayMinutes);
      return response.data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['actionResults', 'game', gameId] });
      queryClient.invalidateQueries({ queryKey: ['actionResults', 'user', gameId] });
    },
  });
}

/**
 * Cancels a staged part that has not yet been revealed.
 *
 * Deliberately does not invalidate unpublishedResultsCount: a pending part is
 * published already — it is waiting on its timer, not on the GM — so removing
 * it cannot change the unpublished count.
 */
export function useCancelPendingStagedPart(gameId: number) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (resultId: number) => {
      await apiClient.phases.cancelPendingStagedPart(gameId, resultId);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['actionResults', 'game', gameId] });
      queryClient.invalidateQueries({ queryKey: ['actionResults', 'user', gameId] });
    },
  });
}

export function useUpdateActionResult(gameId: number) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({ resultId, content }: { resultId: number; content: string }) => {
      const response = await apiClient.phases.updateActionResult(gameId, resultId, { content });
      return response.data;
    },
    onSuccess: () => {
      // Invalidate action results queries to refetch
      queryClient.invalidateQueries({ queryKey: ['actionResults', 'game', gameId] });
      queryClient.invalidateQueries({ queryKey: ['actionResults', 'user', gameId] });
    },
  });
}

export function useDeleteActionResult(gameId: number) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (resultId: number) => {
      await apiClient.phases.deleteActionResult(gameId, resultId);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['actionResults', 'game', gameId] });
      queryClient.invalidateQueries({ queryKey: ['actionResults', 'user', gameId] });
      queryClient.invalidateQueries({ queryKey: ['unpublishedResultsCount'] });
    },
  });
}

export function usePublishActionResult(gameId: number) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (resultId: number) => {
      const response = await apiClient.phases.publishActionResult(gameId, resultId);
      return response.data;
    },
    onSuccess: () => {
      // Invalidate queries to refetch data
      queryClient.invalidateQueries({ queryKey: ['actionResults', 'game', gameId] });
      queryClient.invalidateQueries({ queryKey: ['actionResults', 'user', gameId] });
      queryClient.invalidateQueries({ queryKey: ['draftCharacterUpdates'] });
      queryClient.invalidateQueries({ queryKey: ['draftUpdateCount'] });
      // Invalidate character data to show published character updates
      queryClient.invalidateQueries({ queryKey: ['characterData'] });
    },
  });
}
