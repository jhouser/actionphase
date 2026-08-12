import { useQueries } from '@tanstack/react-query';
import { apiClient } from '../lib/api';
import { useGameActionResults } from './useActionResults';
import type { ActionResult } from '../types/phases';

/**
 * Identifies the recipient a result's sheet updates would apply to.
 *
 * Mirrors the characterId passed to UpdateCharacterSheetModal: results without a
 * character fall back to the user, so results for the same player still collide.
 */
function recipientKey(result: ActionResult): string {
  return result.character_id !== null && result.character_id !== undefined
    ? `c:${result.character_id}`
    : `u:${result.user_id}`;
}

/**
 * Detects whether other unpublished results in the same phase have staged character
 * sheet updates for the same recipient as `result`.
 *
 * Why this matters: a draft sheet update stores a whole-array snapshot under a fixed
 * field name (e.g. inventory/items), and each result's editor seeds from character_data,
 * which does not yet include sibling unpublished drafts. Two results staging updates for
 * one character therefore hold divergent snapshots, and publishing both applies the
 * later one over the earlier — silently discarding the first result's changes.
 *
 * All sheet updates for a character in a phase belong in exactly ONE result.
 *
 * Reuses the ['draftUpdateCount', gameId, resultId] keys the result cards already
 * populate, so detection adds no requests once those are cached.
 */
export function useConflictingSheetDrafts(
  gameId: number,
  result: ActionResult,
  phaseResults: ActionResult[]
): { conflictCount: number; hasConflict: boolean } {
  const target = recipientKey(result);

  // A published result can neither clobber nor be clobbered — its drafts are already
  // applied and deleted — and its callers discard this value anyway. Skip the fan-out
  // rather than issuing a draft-count request per sibling for every card on the page.
  const siblings = result.is_published
    ? []
    : // Only unpublished siblings can still clobber: published results have already
      // written their snapshot and had their drafts deleted.
      phaseResults.filter(
        (candidate) =>
          candidate.id !== result.id &&
          !candidate.is_published &&
          recipientKey(candidate) === target
      );

  const counts = useQueries({
    queries: siblings.map((sibling) => ({
      queryKey: ['draftUpdateCount', gameId, sibling.id],
      queryFn: async () => {
        const response = await apiClient.phases.getDraftUpdateCount(gameId, sibling.id);
        return response.data.count;
      },
      enabled: !!gameId && !!sibling.id,
    })),
  });

  // A sibling whose count is still in flight counts as a possible conflict. The caller
  // gates the warning on `hasConflict && hasStagedOrUnknown`, and hardens only the local
  // half of that expression — so treating pending as "no conflict" here would let a GM
  // who clicks straight through to Publish skip the warning entirely and clobber the
  // sibling's snapshot. Erring toward a spurious warning is the recoverable direction.
  const conflictCount = counts.filter(
    (query) => query.isPending || (query.data ?? 0) > 0
  ).length;

  return { conflictCount, hasConflict: conflictCount > 0 };
}

/**
 * Phase-wide version: reports whether ANY character in the phase has staged sheet
 * updates spread across more than one unpublished result.
 *
 * Publish All is the worst case for this bug — it publishes every conflicting result in
 * a single transaction, so the clobber is guaranteed rather than merely possible. The
 * bulk confirmation dialogs need the same warning the per-result ones show.
 *
 * Reads the same cached results and draft counts the results list already loads.
 *
 * `enabled` defers the work until the warning can actually be shown: callers render one
 * instance per phase card but pass the same currentPhaseId to all of them, so an
 * ungated hook does N identical computations for a dialog only one card has open.
 */
export function usePhaseSheetDraftConflicts(
  gameId: number,
  phaseId: number | undefined,
  enabled = true
): { affectedCharacterCount: number; hasConflict: boolean } {
  const { data: results } = useGameActionResults(gameId);

  // Without a phase there is nothing to compare within. Falling back to a whole-game
  // scan would flag results in DIFFERENT phases as conflicting, which they are not:
  // publishing phase N applies before phase N+1's editor is ever seeded. phaseId is
  // undefined while the current-phase query is in flight and between phases.
  const active = enabled && phaseId !== undefined;

  const unpublished = active
    ? (results ?? []).filter(
        (result) => !result.is_published && result.phase_id === phaseId
      )
    : [];

  const counts = useQueries({
    queries: unpublished.map((result) => ({
      queryKey: ['draftUpdateCount', gameId, result.id],
      queryFn: async () => {
        const response = await apiClient.phases.getDraftUpdateCount(gameId, result.id);
        return response.data.count;
      },
      enabled: !!gameId && !!result.id,
    })),
  });

  // Count staged results per recipient; a recipient with two or more will be clobbered.
  //
  // A count still in flight counts as staged, matching useConflictingSheetDrafts above.
  // Bulk publishing is the worst case for this bug — it applies every conflicting result
  // in one transaction — so a GM who opens the dialog before the counts settle is exactly
  // who the warning is for. Failing open here would hide it from them.
  const stagedPerRecipient = new Map<string, number>();
  unpublished.forEach((result, index) => {
    const query = counts[index];
    const staged = query?.isPending || (query?.data ?? 0) > 0;
    if (!staged) return;
    const key = recipientKey(result);
    stagedPerRecipient.set(key, (stagedPerRecipient.get(key) ?? 0) + 1);
  });

  const affectedCharacterCount = [...stagedPerRecipient.values()].filter((n) => n > 1).length;

  return { affectedCharacterCount, hasConflict: affectedCharacterCount > 0 };
}
