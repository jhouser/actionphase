import { renderHook, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { useCanMessageCharacter } from './useCanMessageCharacter';
import type { Character } from '../types/characters';

vi.mock('../lib/api', () => ({
  apiClient: {
    phases: { getCurrentPhase: vi.fn() },
    characters: { getUserControllableCharacters: vi.fn() },
  },
}));

// The hook reads GameContext when one is mounted for the character's game.
// Tests here exercise the no-context path (standalone character page), which is
// the one that has to fetch for itself.
vi.mock('../contexts/GameContext', () => ({
  useOptionalGameContext: () => null,
}));

import { apiClient } from '../lib/api';
const mockGetCurrentPhase = vi.mocked(apiClient.phases.getCurrentPhase);
const mockGetControllable = vi.mocked(apiClient.characters.getUserControllableCharacters);

function character(overrides: Partial<Character> = {}): Character {
  return {
    id: 1,
    game_id: 7,
    name: 'Char',
    status: 'approved',
    is_active: true,
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T00:00:00Z',
    ...overrides,
  };
}

function setPhase(phaseType: string | null) {
  mockGetCurrentPhase.mockResolvedValue({
    data: { phase: phaseType ? { phase_type: phaseType } : null },
  } as never);
}

function setMyCharacters(chars: Character[]) {
  mockGetControllable.mockResolvedValue({ data: chars } as never);
}

function wrapper({ children }: { children: ReactNode }) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

/** Renders the hook and waits for both underlying queries to settle. */
async function renderCanMessage(target: Character | undefined) {
  const { result } = renderHook(() => useCanMessageCharacter(target), { wrapper });
  await waitFor(() => expect(mockGetCurrentPhase).toHaveBeenCalled());
  return result;
}

describe('useCanMessageCharacter', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setPhase('common_room');
    setMyCharacters([character({ id: 10 })]);
  });

  it('allows messaging another approved character during a common room phase', async () => {
    const result = await renderCanMessage(character({ id: 1 }));
    await waitFor(() => expect(result.current.canMessage).toBe(true));
    expect(result.current.gameId).toBe(7);
  });

  it('allows messaging during an interlude phase', async () => {
    setPhase('interlude');
    const result = await renderCanMessage(character({ id: 1 }));
    await waitFor(() => expect(result.current.canMessage).toBe(true));
  });

  it('disallows messaging during an action phase', async () => {
    setPhase('action');
    const result = await renderCanMessage(character({ id: 1 }));
    await waitFor(() => expect(mockGetControllable).toHaveBeenCalled());
    expect(result.current.canMessage).toBe(false);
  });

  it('disallows messaging when the game has no active phase', async () => {
    setPhase(null);
    const result = await renderCanMessage(character({ id: 1 }));
    await waitFor(() => expect(mockGetControllable).toHaveBeenCalled());
    expect(result.current.canMessage).toBe(false);
  });

  it('disallows messaging when the user controls no character in the game', async () => {
    setMyCharacters([]);
    const result = await renderCanMessage(character({ id: 1 }));
    await waitFor(() => expect(mockGetControllable).toHaveBeenCalled());
    expect(result.current.canMessage).toBe(false);
  });

  it('disallows messaging when the user only has a pending character', async () => {
    setMyCharacters([character({ id: 10, status: 'pending' })]);
    const result = await renderCanMessage(character({ id: 1 }));
    await waitFor(() => expect(mockGetControllable).toHaveBeenCalled());
    expect(result.current.canMessage).toBe(false);
  });

  it('disallows messaging a pending character', async () => {
    const result = await renderCanMessage(character({ id: 1, status: 'pending' }));
    await waitFor(() => expect(mockGetControllable).toHaveBeenCalled());
    expect(result.current.canMessage).toBe(false);
  });

  it('disallows messaging one of your own characters', async () => {
    setMyCharacters([character({ id: 10 }), character({ id: 1 })]);
    const result = await renderCanMessage(character({ id: 1 }));
    await waitFor(() => expect(mockGetControllable).toHaveBeenCalled());
    expect(result.current.canMessage).toBe(false);
  });

  it('reports no game and no permission without a character', async () => {
    const { result } = renderHook(() => useCanMessageCharacter(undefined), { wrapper });
    expect(result.current.canMessage).toBe(false);
    expect(result.current.gameId).toBeUndefined();
    expect(mockGetCurrentPhase).not.toHaveBeenCalled();
  });
});
