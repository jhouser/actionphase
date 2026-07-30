import { useQuery } from '@tanstack/react-query';
import { apiClient } from '../lib/api';
import type { ControllableCharacterWithGame } from '../types/characters';

/** A game's worth of the user's characters, for grouped display. */
export interface GameCharacterGroup {
  gameId: number;
  gameTitle: string;
  characters: ControllableCharacterWithGame[];
}

/**
 * The current user's controllable characters across all their in_progress
 * games. Used by the global Utility Drawer, where there is no GameContext to
 * source characters from.
 *
 * No `enabled` gate is needed: the only caller is the character-sheet panel,
 * which mounts only once the user selects that utility. Pages that never open
 * the drawer never render it, so they never pay for the fetch.
 */
export function useGlobalCharacters() {
  return useQuery({
    queryKey: ['controllableCharactersAcrossGames'],
    queryFn: () =>
      apiClient.characters.getControllableCharactersAcrossGames().then((r) => r.data ?? []),
    staleTime: 60_000,
  });
}

/**
 * Group a flat character list by game, sorted by game title and then by
 * character name within each game.
 *
 * The backend already returns rows in this order, but sorting here rather than
 * relying on that keeps the display order correct on its own terms — a change
 * to the query's ORDER BY would otherwise silently scramble the picker with
 * nothing to catch it. Ties on title are broken by game id so the order is
 * total (two games may share a name).
 */
export function groupCharactersByGame(
  characters: ControllableCharacterWithGame[]
): GameCharacterGroup[] {
  const groups = new Map<number, GameCharacterGroup>();

  for (const character of characters) {
    const existing = groups.get(character.game_id);
    if (existing) {
      existing.characters.push(character);
    } else {
      groups.set(character.game_id, {
        gameId: character.game_id,
        gameTitle: character.game_title,
        characters: [character],
      });
    }
  }

  const ordered = Array.from(groups.values());
  ordered.sort(
    (a, b) => a.gameTitle.localeCompare(b.gameTitle) || a.gameId - b.gameId
  );
  for (const group of ordered) {
    group.characters.sort((a, b) => a.name.localeCompare(b.name));
  }

  return ordered;
}
