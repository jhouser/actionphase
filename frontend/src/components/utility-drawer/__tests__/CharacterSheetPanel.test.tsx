import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, fireEvent, waitFor } from '@testing-library/react';
import { renderWithProviders } from '../../../test-utils';
import { CharacterSheetPanel } from '../panels/CharacterSheetPanel';
import type { UtilityContext } from '../types';
import type { ControllableCharacterWithGame } from '../../../types/characters';

vi.mock('../../../lib/api', () => ({
  apiClient: {
    characters: {
      getControllableCharactersAcrossGames: vi.fn(),
    },
  },
}));

import { apiClient } from '../../../lib/api';

function makeCharacter(
  overrides: Partial<ControllableCharacterWithGame> = {}
): ControllableCharacterWithGame {
  return {
    id: 1,
    game_id: 10,
    name: 'Kael Vance',
    status: 'approved',
    character_type: 'player_character',
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T00:00:00Z',
    game_title: 'Alpha Game',
    game_state: 'in_progress',
    game_is_anonymous: false,
    game_portrait_avatars: false,
    user_role: 'player',
    ...overrides,
  };
}

/** Global (no game in scope) drawer context. */
function makeGlobalCtx(openCharacterSheet = vi.fn()): UtilityContext {
  return { game: null, openCharacterSheet, closeDrawer: vi.fn() };
}

function mockCharacters(characters: ControllableCharacterWithGame[]) {
  vi.mocked(apiClient.characters.getControllableCharactersAcrossGames).mockResolvedValue({
    data: characters,
  } as never);
}

describe('CharacterSheetPanel — outside a game', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('groups the user\'s characters under their game titles', async () => {
    mockCharacters([
      makeCharacter({ id: 1, name: 'Kael Vance', game_id: 10, game_title: 'Alpha Game' }),
      makeCharacter({ id: 2, name: 'Tavern Keeper', game_id: 10, game_title: 'Alpha Game' }),
      makeCharacter({ id: 3, name: 'Mira Oduya', game_id: 11, game_title: 'Beta Game' }),
    ]);

    renderWithProviders(<CharacterSheetPanel ctx={makeGlobalCtx()} />);

    expect(await screen.findByTestId('global-character-sheet-picker')).toBeInTheDocument();

    // Both games are named, so the user can tell which character is which.
    expect(screen.getByText('Alpha Game')).toBeInTheDocument();
    expect(screen.getByText('Beta Game')).toBeInTheDocument();

    // Every character is offered.
    expect(screen.getByText('Kael Vance')).toBeInTheDocument();
    expect(screen.getByText('Tavern Keeper')).toBeInTheDocument();
    expect(screen.getByText('Mira Oduya')).toBeInTheDocument();
  });

  it('never auto-opens a sheet, even with a single character', async () => {
    const openCharacterSheet = vi.fn();
    mockCharacters([makeCharacter({ id: 7 })]);

    renderWithProviders(<CharacterSheetPanel ctx={makeGlobalCtx(openCharacterSheet)} />);

    // The picker is shown and nothing opened on its own — a global button
    // shouldn't fling a modal at the user.
    expect(await screen.findByTestId('global-character-sheet-picker')).toBeInTheDocument();
    expect(openCharacterSheet).not.toHaveBeenCalled();
  });

  it('opens the chosen character with permissions from that game', async () => {
    const openCharacterSheet = vi.fn();
    mockCharacters([
      makeCharacter({ id: 1, name: 'Kael Vance', user_role: 'player' }),
      makeCharacter({
        id: 2,
        name: 'Beta NPC',
        game_id: 11,
        game_title: 'Beta Game',
        user_role: 'gm',
        game_is_anonymous: true,
      }),
    ]);

    renderWithProviders(<CharacterSheetPanel ctx={makeGlobalCtx(openCharacterSheet)} />);

    fireEvent.click(await screen.findByTestId('character-sheet-open-1'));

    // A player in an active game can edit their sheet but not its stats.
    expect(openCharacterSheet).toHaveBeenCalledWith(1, {
      canEdit: true,
      canEditStats: false,
      isAnonymous: false,
      userRole: 'player',
      gameState: 'in_progress',
    });

    // The GM of a different game gets stat editing, and that game's anonymity.
    fireEvent.click(screen.getByTestId('character-sheet-open-2'));
    expect(openCharacterSheet).toHaveBeenCalledWith(2, {
      canEdit: true,
      canEditStats: true,
      isAnonymous: true,
      userRole: 'gm',
      gameState: 'in_progress',
    });
  });

  it('opens a completed game\'s sheet read-only', async () => {
    const openCharacterSheet = vi.fn();
    mockCharacters([
      makeCharacter({ id: 4, game_state: 'completed', user_role: 'gm' }),
    ]);

    renderWithProviders(<CharacterSheetPanel ctx={makeGlobalCtx(openCharacterSheet)} />);

    fireEvent.click(await screen.findByTestId('character-sheet-open-4'));

    // Finished games are locked — even for the GM.
    expect(openCharacterSheet).toHaveBeenCalledWith(
      4,
      expect.objectContaining({ canEdit: false, canEditStats: false })
    );
  });

  it('reports having no characters rather than showing an empty picker', async () => {
    mockCharacters([]);

    renderWithProviders(<CharacterSheetPanel ctx={makeGlobalCtx()} />);

    expect(
      await screen.findByText(/don't control a character in any active game/i)
    ).toBeInTheDocument();
    expect(screen.queryByTestId('global-character-sheet-picker')).not.toBeInTheDocument();
  });

  it('surfaces a load failure instead of an empty state', async () => {
    vi.mocked(apiClient.characters.getControllableCharactersAcrossGames).mockRejectedValue(
      new Error('boom')
    );

    renderWithProviders(<CharacterSheetPanel ctx={makeGlobalCtx()} />);

    await waitFor(() =>
      expect(screen.getByTestId('global-characters-error')).toBeInTheDocument()
    );
    // Must not be mistaken for "you have no characters".
    expect(
      screen.queryByText(/don't control a character in any active game/i)
    ).not.toBeInTheDocument();
  });
});
