import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import React from 'react';
import { useGlobalHandouts, groupHandoutsByGame } from '../useGlobalHandouts';
import type { HandoutWithGame } from '../../types/handouts';

vi.mock('../../lib/api', () => ({
  apiClient: {
    handouts: {
      listHandoutsAcrossGames: vi.fn(),
    },
  },
}));

import { apiClient } from '../../lib/api';

function makeWrapper() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return ({ children }: { children: React.ReactNode }) =>
    React.createElement(QueryClientProvider, { client: queryClient }, children);
}

function makeHandout(overrides: Partial<HandoutWithGame>): HandoutWithGame {
  return {
    id: 1,
    game_id: 10,
    title: 'Tavern Rules',
    content: '# Tavern Rules',
    status: 'published',
    game_title: 'Alpha Game',
    ...overrides,
  };
}

describe('useGlobalHandouts', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('returns the handouts returned by the API', async () => {
    vi.mocked(apiClient.handouts.listHandoutsAcrossGames).mockResolvedValue({
      data: [
        makeHandout({ id: 1, title: 'Tavern Rules' }),
        makeHandout({ id: 2, game_id: 11, title: 'Faction Primer', game_title: 'Beta Game' }),
      ],
    } as never);

    const { result } = renderHook(() => useGlobalHandouts(), { wrapper: makeWrapper() });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toHaveLength(2);
    expect(result.current.data?.[0].title).toBe('Tavern Rules');
    expect(result.current.data?.[1].game_title).toBe('Beta Game');
  });

  it('normalizes a missing payload to an empty array', async () => {
    vi.mocked(apiClient.handouts.listHandoutsAcrossGames).mockResolvedValue({
      data: undefined,
    } as never);

    const { result } = renderHook(() => useGlobalHandouts(), { wrapper: makeWrapper() });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual([]);
  });

  it('surfaces an API failure as an error state', async () => {
    vi.mocked(apiClient.handouts.listHandoutsAcrossGames).mockRejectedValue(
      new Error('network down')
    );

    const { result } = renderHook(() => useGlobalHandouts(), { wrapper: makeWrapper() });

    await waitFor(() => expect(result.current.isError).toBe(true));
  });
});

describe('groupHandoutsByGame', () => {
  it('groups handouts under their game', () => {
    const groups = groupHandoutsByGame([
      makeHandout({ id: 1, game_id: 10, title: 'Tavern Rules', game_title: 'Alpha Game' }),
      makeHandout({ id: 2, game_id: 10, title: 'World Lore', game_title: 'Alpha Game' }),
      makeHandout({ id: 3, game_id: 11, title: 'Faction Primer', game_title: 'Beta Game' }),
    ]);

    expect(groups).toHaveLength(2);
    expect(groups[0].gameTitle).toBe('Alpha Game');
    expect(groups[0].handouts.map((h) => h.title)).toEqual(['Tavern Rules', 'World Lore']);
    expect(groups[1].handouts.map((h) => h.title)).toEqual(['Faction Primer']);
  });

  /**
   * The display order must not depend on the order the backend happens to send.
   * The query orders handouts by created_at DESC, so feeding rows in any order
   * must still produce games alphabetically and handouts alphabetically within
   * them.
   */
  it('sorts games and handouts regardless of input order', () => {
    const groups = groupHandoutsByGame([
      makeHandout({ id: 3, game_id: 11, title: 'Faction Primer', game_title: 'Beta Game' }),
      makeHandout({ id: 2, game_id: 10, title: 'World Lore', game_title: 'Alpha Game' }),
      makeHandout({ id: 4, game_id: 11, title: 'Ancestral Feuds', game_title: 'Beta Game' }),
      makeHandout({ id: 1, game_id: 10, title: 'Tavern Rules', game_title: 'Alpha Game' }),
    ]);

    expect(groups.map((g) => g.gameTitle)).toEqual(['Alpha Game', 'Beta Game']);
    expect(groups[0].handouts.map((h) => h.title)).toEqual(['Tavern Rules', 'World Lore']);
    expect(groups[1].handouts.map((h) => h.title)).toEqual(['Ancestral Feuds', 'Faction Primer']);
  });

  it('orders same-titled games deterministically by id', () => {
    const groups = groupHandoutsByGame([
      makeHandout({ id: 1, game_id: 20, title: 'Second', game_title: 'Twin Game' }),
      makeHandout({ id: 2, game_id: 12, title: 'First', game_title: 'Twin Game' }),
    ]);

    expect(groups.map((g) => g.gameId)).toEqual([12, 20]);
  });

  it('returns an empty list for no handouts', () => {
    expect(groupHandoutsByGame([])).toEqual([]);
  });
});
