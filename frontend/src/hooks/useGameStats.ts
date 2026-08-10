import { useQuery } from '@tanstack/react-query';
import { apiClient } from '../lib/api';

/**
 * Post-game statistics for a completed game.
 *
 * `enabled` guards on more than a defined id: the endpoint returns 409 for any
 * game that has not completed, so callers pass the game state and the query
 * stays idle until the game is actually finished.
 *
 * staleTime is Infinity because the underlying data genuinely cannot change: a
 * completed game is a frozen archive, so these numbers are fixed the moment the
 * game ends. There is nothing to refetch for, and the query is only ever enabled
 * once that is already true.
 */
export function useGameStats(gameId: number | undefined, gameState?: string) {
  return useQuery({
    queryKey: ['gameStats', gameId],
    queryFn: () => apiClient.games.getGameStats(gameId!).then((r) => r.data),
    enabled: !!gameId && gameState === 'completed',
    staleTime: Infinity,
  });
}
