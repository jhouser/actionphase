import { useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '../lib/api';

/**
 * Discards every staged character sheet update on an action result.
 *
 * Needed because the sheet editor only ever writes whole-sheet snapshots: emptying a
 * module stages an EMPTY snapshot rather than removing the staged update, and
 * publishing that snapshot would wipe the character's sheet. Without this there is no
 * way to un-stage updates short of deleting the whole result.
 *
 * The API deletes one draft row at a time, so this fans out over the result's current
 * rows. Deletes run sequentially: they are few (at most one per module) and a partial
 * failure should leave a coherent state rather than a half-applied burst.
 */
export function useDiscardSheetDrafts(gameId: number, resultId: number) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async () => {
      // Re-read rather than trusting cache: the count drives a destructive action,
      // and a stale list would silently leave rows behind.
      const response = await apiClient.phases.getDraftCharacterUpdates(gameId, resultId);
      const drafts = response.data ?? [];

      for (const draft of drafts) {
        await apiClient.phases.deleteDraftCharacterUpdate(gameId, resultId, draft.id);
      }

      return drafts.length;
    },
    // onSettled, not onSuccess: the deletes run one at a time, so a failure partway
    // through has still removed rows server-side. Invalidating only on success would
    // leave the editor and the update badge showing drafts that no longer exist, and
    // the next edit would write a snapshot built on that phantom state.
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ['draftCharacterUpdates', gameId, resultId] });
      // Unkeyed: the sibling-conflict warning reads counts for OTHER results, and
      // clearing this result's drafts should clear their warnings too.
      queryClient.invalidateQueries({ queryKey: ['draftUpdateCount'] });
    },
  });
}
