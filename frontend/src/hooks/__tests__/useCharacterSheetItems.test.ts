import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import React from 'react';
import { useCharacterSheetItems } from '../useCharacterSheetItems';
import type { CharacterData } from '../../types/characters';

vi.mock('../../lib/api', () => ({
  apiClient: {
    characters: {
      getCharacterData: vi.fn(),
    },
  },
}));

import { apiClient } from '../../lib/api';

function makeWrapper() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return ({ children }: { children: React.ReactNode }) =>
    React.createElement(QueryClientProvider, { client: queryClient }, children);
}

function makeDataRow(overrides: Partial<CharacterData>): CharacterData {
  return {
    id: 1,
    character_id: 42,
    module_type: 'skills',
    field_name: 'skills',
    field_type: 'json',
    is_public: true,
    created_at: '',
    updated_at: '',
    ...overrides,
  };
}

describe('useCharacterSheetItems', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('returns empty array when characterId is null', () => {
    const { result } = renderHook(() => useCharacterSheetItems(null), {
      wrapper: makeWrapper(),
    });
    expect(result.current).toEqual([]);
    expect(apiClient.characters.getCharacterData).not.toHaveBeenCalled();
  });

  it('parses skills into SheetItem[]', async () => {
    vi.mocked(apiClient.characters.getCharacterData).mockResolvedValue({
      data: [
        makeDataRow({
          module_type: 'skills',
          field_name: 'skills',
          field_value: JSON.stringify([
            { id: 'sk-1', name: 'Stealth', level: 3, category: 'Combat' },
          ]),
        }),
      ],
    } as never);

    const { result } = renderHook(() => useCharacterSheetItems(42), {
      wrapper: makeWrapper(),
    });

    await waitFor(() => expect(result.current).toHaveLength(1));

    expect(result.current[0]).toMatchObject({
      id: 'sk-1',
      name: 'Stealth',
      type: 'skill',
      metadata: 'Combat · Level 3',
    });
  });

  it('parses inventory items into SheetItem[]', async () => {
    vi.mocked(apiClient.characters.getCharacterData).mockResolvedValue({
      data: [
        makeDataRow({
          module_type: 'inventory',
          field_name: 'items',
          field_value: JSON.stringify([
            { id: 'it-1', name: 'Elvish Longbow', description: 'A fine bow', quantity: 1, category: 'Weapon' },
          ]),
        }),
      ],
    } as never);

    const { result } = renderHook(() => useCharacterSheetItems(42), {
      wrapper: makeWrapper(),
    });

    await waitFor(() => expect(result.current).toHaveLength(1));

    expect(result.current[0]).toMatchObject({
      id: 'it-1',
      name: 'Elvish Longbow',
      type: 'item',
      metadata: 'Weapon',
    });
  });

  it('filters out skills missing id or name', async () => {
    vi.mocked(apiClient.characters.getCharacterData).mockResolvedValue({
      data: [
        makeDataRow({
          field_name: 'skills',
          field_value: JSON.stringify([
            { id: 'abc-1', name: 'Good Skill', level: 2, category: 'Combat' },
            { name: 'No ID', level: 2, category: 'Combat' },
            { id: 'abc-3', level: 2, category: 'Combat' },
          ]),
        }),
      ],
    } as never);

    const { result } = renderHook(() => useCharacterSheetItems(42), {
      wrapper: makeWrapper(),
    });

    // Wait for query to settle — only 1 valid item should appear. Asserting the
    // result inside waitFor is a positive condition, so it polls until the state
    // lands; a bare sleep would let that update escape act() and warn.
    await waitFor(() => {
      expect(apiClient.characters.getCharacterData).toHaveBeenCalledWith(42);
      expect(result.current).toHaveLength(1);
    });

    expect(result.current[0].name).toBe('Good Skill');
  });
});
