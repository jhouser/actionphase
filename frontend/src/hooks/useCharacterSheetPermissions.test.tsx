import { renderHook } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { useCharacterSheetPermissions } from './useCharacterSheetPermissions';
import type { Character } from '../types/characters';

// useCharacterSheetPermissions -> useCharacterOwnership -> useUserCharacters
vi.mock('./useUserCharacters');
import { useUserCharacters } from './useUserCharacters';
const mockUseUserCharacters = vi.mocked(useUserCharacters);

function character(overrides: Partial<Character> = {}): Character {
  return {
    id: 1,
    game_id: 1,
    name: 'Char',
    status: 'approved',
    is_active: true,
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T00:00:00Z',
    ...overrides,
  };
}

function setOwnedCharacters(chars: Character[]) {
  mockUseUserCharacters.mockReturnValue({
    characters: chars,
    isLoading: false,
    error: null,
    refetch: async () => {},
  });
}

describe('useCharacterSheetPermissions', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setOwnedCharacters([]);
  });

  describe('isUserCharacter', () => {
    it('recognizes owned characters', () => {
      const mine = character({ id: 5 });
      setOwnedCharacters([mine]);
      const { result } = renderHook(() =>
        useCharacterSheetPermissions(1, 'player', 'in_progress')
      );
      expect(result.current.isUserCharacter(mine)).toBe(true);
      expect(result.current.isUserCharacter(character({ id: 6 }))).toBe(false);
    });

    it('treats unassigned NPCs as owned by the GM', () => {
      const npc = character({ id: 9, character_type: 'npc', assigned_user_id: undefined });
      const { result } = renderHook(() =>
        useCharacterSheetPermissions(1, 'gm', 'in_progress')
      );
      expect(result.current.isUserCharacter(npc)).toBe(true);
    });

    it('does not treat unassigned NPCs as owned by a player', () => {
      const npc = character({ id: 9, character_type: 'npc' });
      const { result } = renderHook(() =>
        useCharacterSheetPermissions(1, 'player', 'in_progress')
      );
      expect(result.current.isUserCharacter(npc)).toBe(false);
    });
  });

  describe('canView', () => {
    it('lets GM and audience view any character', () => {
      const other = character({ id: 2, status: 'pending' });
      const gm = renderHook(() => useCharacterSheetPermissions(1, 'gm', 'in_progress'));
      const audience = renderHook(() => useCharacterSheetPermissions(1, 'audience', 'in_progress'));
      expect(gm.result.current.canView(other)).toBe(true);
      expect(audience.result.current.canView(other)).toBe(true);
    });

    it('lets anyone view approved characters but not pending ones they do not own', () => {
      const { result } = renderHook(() =>
        useCharacterSheetPermissions(1, 'player', 'in_progress')
      );
      expect(result.current.canView(character({ id: 2, status: 'approved' }))).toBe(true);
      expect(result.current.canView(character({ id: 2, status: 'pending' }))).toBe(false);
    });
  });

  describe('canEdit', () => {
    it('lets a player edit their own character', () => {
      const mine = character({ id: 5, status: 'pending' });
      setOwnedCharacters([mine]);
      const { result } = renderHook(() =>
        useCharacterSheetPermissions(1, 'player', 'in_progress')
      );
      expect(result.current.canEdit(mine)).toBe(true);
      expect(result.current.canEdit(character({ id: 6 }))).toBe(false);
    });

    it('blocks all editing in completed or cancelled games', () => {
      const mine = character({ id: 5 });
      setOwnedCharacters([mine]);
      const completed = renderHook(() =>
        useCharacterSheetPermissions(1, 'gm', 'completed')
      );
      const cancelled = renderHook(() =>
        useCharacterSheetPermissions(1, 'player', 'cancelled')
      );
      expect(completed.result.current.canEdit(mine)).toBe(false);
      expect(cancelled.result.current.canEdit(mine)).toBe(false);
    });
  });

  describe('canEditStats', () => {
    it('is GM-only and disabled once the game is over', () => {
      expect(
        renderHook(() => useCharacterSheetPermissions(1, 'gm', 'in_progress'))
          .result.current.canEditStats()
      ).toBe(true);
      expect(
        renderHook(() => useCharacterSheetPermissions(1, 'player', 'in_progress'))
          .result.current.canEditStats()
      ).toBe(false);
      expect(
        renderHook(() => useCharacterSheetPermissions(1, 'gm', 'completed'))
          .result.current.canEditStats()
      ).toBe(false);
    });
  });
});
