import { useQuery } from '@tanstack/react-query';
import { apiClient } from '../lib/api';
import type { HandoutWithGame } from '../types/handouts';

/** A game's worth of the user's handouts, for grouped display. */
export interface GameHandoutGroup {
  gameId: number;
  gameTitle: string;
  handouts: HandoutWithGame[];
}

/**
 * The published handouts of every in_progress game the current user takes part
 * in. Used by the global Utility Drawer, where there is no GameContext to
 * source handouts from.
 *
 * No `enabled` gate is needed: the only caller is the handouts panel, which
 * mounts only once the user selects that utility. Pages that never open the
 * drawer never render it, so they never pay for the fetch.
 */
export function useGlobalHandouts() {
  return useQuery({
    queryKey: ['handoutsAcrossGames'],
    queryFn: () => apiClient.handouts.listHandoutsAcrossGames().then((r) => r.data ?? []),
    staleTime: 60_000,
  });
}

/**
 * Group a flat handout list by game, sorted by game title and then by handout
 * title within each game.
 *
 * The backend already returns rows grouped by game title, but sorting here
 * rather than relying on that keeps the display order correct on its own terms
 * — a change to the query's ORDER BY would otherwise silently scramble the
 * picker with nothing to catch it. Ties on title are broken by game id so the
 * order is total (two games may share a name).
 *
 * Handouts sort by title rather than the backend's created_at DESC: the drawer
 * is a lookup surface where you know the name of what you want, and scanning
 * for it alphabetically beats recency.
 */
export function groupHandoutsByGame(handouts: HandoutWithGame[]): GameHandoutGroup[] {
  const groups = new Map<number, GameHandoutGroup>();

  for (const handout of handouts) {
    const existing = groups.get(handout.game_id);
    if (existing) {
      existing.handouts.push(handout);
    } else {
      groups.set(handout.game_id, {
        gameId: handout.game_id,
        gameTitle: handout.game_title,
        handouts: [handout],
      });
    }
  }

  const ordered = Array.from(groups.values());
  ordered.sort((a, b) => a.gameTitle.localeCompare(b.gameTitle) || a.gameId - b.gameId);
  for (const group of ordered) {
    group.handouts.sort((a, b) => a.title.localeCompare(b.title));
  }

  return ordered;
}
