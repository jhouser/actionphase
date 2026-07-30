import { useQuery } from '@tanstack/react-query';
import { apiClient } from '../lib/api';

export function useCharacterStats(characterId: number | undefined) {
  return useQuery({
    queryKey: ['characterStats', characterId],
    queryFn: () =>
      apiClient.characters.getCharacterStats(characterId!).then((r) => r.data),
    enabled: !!characterId,
    staleTime: 60_000,
  });
}

// Stats for an entire game's character roster in one request, instead of one
// useCharacterStats call per rendered CharacterCard. A roster of 20+ characters
// firing that many parallel requests on mount was bursting the backend (503s
// under the resulting DB connection pool pressure).
export function useGameCharacterStats(gameId: number | undefined) {
  return useQuery({
    queryKey: ['gameCharacterStats', gameId],
    queryFn: () =>
      apiClient.characters.getGameCharacterStats(gameId!).then((r) => r.data),
    enabled: !!gameId,
    staleTime: 60_000,
  });
}
