import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import React from 'react';
import { useGlobalCharacters, groupCharactersByGame } from '../useGlobalCharacters';
import type { ControllableCharacterWithGame } from '../../types/characters';

vi.mock('../../lib/api', () => ({
  apiClient: {
    characters: {
      getControllableCharactersAcrossGames: vi.fn(),
    },
  },
}));

import { apiClient } from '../../lib/api';

function makeWrapper() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return ({ children }: { children: React.ReactNode }) =>
    React.createElement(QueryClientProvider, { client: queryClient }, children);
}

function makeCharacter(
  overrides: Partial<ControllableCharacterWithGame>
): ControllableCharacterWithGame {
  return {
    id: 1,
    game_id: 10,
    name: 'Kael Vance',
    status: 'approved',
    character_type: 'player_character',
    created_at: '',
    updated_at: '',
    game_title: 'Alpha Game',
    game_state: 'in_progress',
    game_is_anonymous: false,
    game_portrait_avatars: false,
    user_role: 'player',
    ...overrides,
  };
}

describe('useGlobalCharacters', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('returns the characters returned by the API', async () => {
    const characters = [
      makeCharacter({ id: 1, name: 'Kael Vance' }),
      makeCharacter({ id: 2, game_id: 11, name: 'Mira Oduya', game_title: 'Beta Game' }),
    ];
    vi.mocked(apiClient.characters.getControllableCharactersAcrossGames).mockResolvedValue({
      data: characters,
    } as never);

    const { result } = renderHook(() => useGlobalCharacters(), {
      wrapper: makeWrapper(),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toHaveLength(2);
    expect(result.current.data?.[0].name).toBe('Kael Vance');
    expect(result.current.data?.[1].game_title).toBe('Beta Game');
  });

  it('normalizes a missing payload to an empty array', async () => {
    vi.mocked(apiClient.characters.getControllableCharactersAcrossGames).mockResolvedValue({
      data: undefined,
    } as never);

    const { result } = renderHook(() => useGlobalCharacters(), {
      wrapper: makeWrapper(),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual([]);
  });
});

describe('groupCharactersByGame', () => {
  it('groups characters under their game', () => {
    const groups = groupCharactersByGame([
      makeCharacter({ id: 1, game_id: 10, name: 'Kael Vance', game_title: 'Alpha Game' }),
      makeCharacter({ id: 2, game_id: 10, name: 'Tavern Keeper', game_title: 'Alpha Game' }),
      makeCharacter({ id: 3, game_id: 11, name: 'Mira Oduya', game_title: 'Beta Game' }),
    ]);

    expect(groups).toHaveLength(2);
    expect(groups[0].gameTitle).toBe('Alpha Game');
    expect(groups[0].characters.map((c) => c.name)).toEqual(['Kael Vance', 'Tavern Keeper']);
    expect(groups[1].gameTitle).toBe('Beta Game');
    expect(groups[1].characters.map((c) => c.name)).toEqual(['Mira Oduya']);
  });

  /**
   * The display order must not depend on the order the backend happens to send.
   * Feeding the rows in reverse — as a changed ORDER BY would — must still
   * produce games alphabetically and characters alphabetically within them.
   */
  it('sorts games and characters regardless of input order', () => {
    const groups = groupCharactersByGame([
      makeCharacter({ id: 3, game_id: 11, name: 'Mira Oduya', game_title: 'Beta Game' }),
      makeCharacter({ id: 2, game_id: 10, name: 'Tavern Keeper', game_title: 'Alpha Game' }),
      makeCharacter({ id: 4, game_id: 11, name: 'Beta Straggler', game_title: 'Beta Game' }),
      makeCharacter({ id: 1, game_id: 10, name: 'Kael Vance', game_title: 'Alpha Game' }),
    ]);

    expect(groups.map((g) => g.gameTitle)).toEqual(['Alpha Game', 'Beta Game']);
    expect(groups[0].characters.map((c) => c.name)).toEqual(['Kael Vance', 'Tavern Keeper']);
    expect(groups[1].characters.map((c) => c.name)).toEqual(['Beta Straggler', 'Mira Oduya']);
  });

  it('orders same-titled games deterministically by id', () => {
    const groups = groupCharactersByGame([
      makeCharacter({ id: 1, game_id: 20, name: 'Second', game_title: 'Twin Game' }),
      makeCharacter({ id: 2, game_id: 12, name: 'First', game_title: 'Twin Game' }),
    ]);

    expect(groups.map((g) => g.gameId)).toEqual([12, 20]);
  });

  it('returns an empty list for no characters', () => {
    expect(groupCharactersByGame([])).toEqual([]);
  });
});
