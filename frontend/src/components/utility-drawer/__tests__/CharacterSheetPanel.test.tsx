import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, fireEvent, waitFor } from '@testing-library/react';
import { renderWithProviders } from '../../../test-utils';
import { CharacterSheetPanel } from '../panels/CharacterSheetPanel';
import type { GameUtilityContext, UtilityContext } from '../types';
import type { Character, ControllableCharacterWithGame } from '../../../types/characters';

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

function makeGameCharacter(overrides: Partial<Character> = {}): Character {
  return {
    id: 1,
    game_id: 10,
    name: 'Kael Vance',
    status: 'approved',
    character_type: 'player_character',
    is_active: true,
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T00:00:00Z',
    ...overrides,
  };
}

/** An in-game drawer context (the common-room case). */
function makeInGameCtx(
  gameOverrides: Partial<GameUtilityContext> = {},
  openCharacterSheet = vi.fn()
): UtilityContext {
  return {
    game: {
      gameId: 10,
      currentPhase: null,
      isGM: false,
      isAudience: false,
      isGameCompleted: false,
      userRole: 'player',
      gameState: 'in_progress',
      isAnonymous: false,
      userCharacters: [makeGameCharacter()],
      allGameCharacters: [makeGameCharacter()],
      commentReadMode: 'manual',
      ...gameOverrides,
    },
    openCharacterSheet,
    closeDrawer: vi.fn(),
  };
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

describe('CharacterSheetPanel — in a game, as GM', () => {
  const cast = [
    makeGameCharacter({ id: 1, name: 'Zara Quill', username: 'dana' }),
    makeGameCharacter({
      id: 2,
      name: 'Tavern Keeper',
      character_type: 'npc',
      assigned_username: 'rob',
    }),
    makeGameCharacter({ id: 3, name: 'Alden Roe', username: 'sam' }),
  ];

  it('offers every character in the game, not just the GM\'s own', () => {
    renderWithProviders(
      <CharacterSheetPanel
        ctx={makeInGameCtx({
          isGM: true,
          userRole: 'gm',
          // The GM controls none of these directly — the point of the feature.
          userCharacters: [],
          allGameCharacters: cast,
        })}
      />
    );

    expect(screen.getByTestId('character-sheet-open-1')).toBeInTheDocument();
    expect(screen.getByTestId('character-sheet-open-2')).toBeInTheDocument();
    expect(screen.getByTestId('character-sheet-open-3')).toBeInTheDocument();
  });

  it('sorts the cast by name so a GM can scan for one', () => {
    renderWithProviders(
      <CharacterSheetPanel
        ctx={makeInGameCtx({ isGM: true, userRole: 'gm', allGameCharacters: cast })}
      />
    );

    // Read the name line specifically — the row also carries an owner line and
    // may carry an NPC badge, so matching on the row's whole text would be
    // brittle as those change.
    const names = screen
      .getAllByTestId(/^character-sheet-open-/)
      .map((el) => el.querySelector('span > span')?.textContent);
    expect(names).toEqual(['Alden Roe', 'Tavern Keeper', 'Zara Quill']);
  });

  it('shows who plays each character', () => {
    renderWithProviders(
      <CharacterSheetPanel
        ctx={makeInGameCtx({ isGM: true, userRole: 'gm', allGameCharacters: cast })}
      />
    );

    // Player characters credit their owner...
    expect(screen.getByTestId('character-sheet-owner-1')).toHaveTextContent('dana');
    expect(screen.getByTestId('character-sheet-owner-3')).toHaveTextContent('sam');
    // ...and an NPC credits whoever it's assigned to, not its creator.
    expect(screen.getByTestId('character-sheet-owner-2')).toHaveTextContent('rob');
  });

  /**
   * An NPC nobody has been given is the GM's own to play. Saying so beats a
   * blank line, which reads as missing data rather than a deliberate state.
   */
  it('marks an unassigned NPC rather than leaving the line blank', () => {
    renderWithProviders(
      <CharacterSheetPanel
        ctx={makeInGameCtx({
          isGM: true,
          userRole: 'gm',
          allGameCharacters: [
            makeGameCharacter({ id: 8, name: 'Wandering Merchant', character_type: 'npc' }),
          ],
        })}
      />
    );

    expect(screen.getByTestId('character-sheet-owner-8')).toHaveTextContent('Unassigned');
  });

  /**
   * The backend withholds usernames in anonymous games from roles that may not
   * see them. A GM is not one of those, but the panel must degrade to just the
   * character name rather than render an empty line if that ever changes.
   */
  it('omits the owner line when no username came back', () => {
    renderWithProviders(
      <CharacterSheetPanel
        ctx={makeInGameCtx({
          isGM: true,
          userRole: 'gm',
          allGameCharacters: [makeGameCharacter({ id: 9, name: 'Nameless', username: undefined })],
        })}
      />
    );

    expect(screen.getByTestId('character-sheet-open-9')).toBeInTheDocument();
    expect(screen.queryByTestId('character-sheet-owner-9')).not.toBeInTheDocument();
  });

  it('marks which entries are NPCs', () => {
    renderWithProviders(
      <CharacterSheetPanel
        ctx={makeInGameCtx({ isGM: true, userRole: 'gm', allGameCharacters: cast })}
      />
    );

    expect(screen.getByTestId('character-sheet-npc-2')).toBeInTheDocument();
    // Player characters carry no badge.
    expect(screen.queryByTestId('character-sheet-npc-1')).not.toBeInTheDocument();
  });

  it('opens the chosen sheet with GM stat-editing rights', () => {
    const openCharacterSheet = vi.fn();
    renderWithProviders(
      <CharacterSheetPanel
        ctx={makeInGameCtx(
          { isGM: true, userRole: 'gm', allGameCharacters: cast },
          openCharacterSheet
        )}
      />
    );

    fireEvent.click(screen.getByTestId('character-sheet-open-2'));
    expect(openCharacterSheet).toHaveBeenCalledWith(2, {
      canEdit: true,
      canEditStats: true,
      isAnonymous: false,
      userRole: 'gm',
      gameState: 'in_progress',
    });
  });

  /**
   * Auto-open is for "the one character you control". A GM's list is a
   * reference of the whole cast, so a game holding a single character must
   * still show the list rather than fling that sheet open.
   */
  it('never auto-opens, even when the game holds one character', () => {
    const openCharacterSheet = vi.fn();
    renderWithProviders(
      <CharacterSheetPanel
        ctx={makeInGameCtx(
          {
            isGM: true,
            userRole: 'gm',
            userCharacters: [],
            allGameCharacters: [makeGameCharacter({ id: 9, name: 'Lone NPC' })],
          },
          openCharacterSheet
        )}
      />
    );

    expect(screen.getByTestId('character-sheet-picker')).toBeInTheDocument();
    expect(openCharacterSheet).not.toHaveBeenCalled();
  });

  it('reports an empty game rather than "you control nothing"', () => {
    renderWithProviders(
      <CharacterSheetPanel
        ctx={makeInGameCtx({
          isGM: true,
          userRole: 'gm',
          userCharacters: [],
          allGameCharacters: [],
        })}
      />
    );

    expect(screen.getByText(/no characters yet/i)).toBeInTheDocument();
  });
});

describe('CharacterSheetPanel — in a game, as a player', () => {
  /** The GM's whole-cast view must not leak to players. */
  it('offers only the characters the player controls', () => {
    renderWithProviders(
      <CharacterSheetPanel
        ctx={makeInGameCtx({
          isGM: false,
          userRole: 'player',
          userCharacters: [
            makeGameCharacter({ id: 1, name: 'Kael Vance', username: 'dana' }),
            makeGameCharacter({ id: 2, name: 'Second Character', username: 'dana' }),
          ],
          allGameCharacters: [
            makeGameCharacter({ id: 1, name: 'Kael Vance' }),
            makeGameCharacter({ id: 2, name: 'Second Character' }),
            makeGameCharacter({ id: 3, name: 'Someone Else' }),
          ],
        })}
      />
    );

    expect(screen.getByTestId('character-sheet-open-1')).toBeInTheDocument();
    expect(screen.getByTestId('character-sheet-open-2')).toBeInTheDocument();
    expect(screen.queryByTestId('character-sheet-open-3')).not.toBeInTheDocument();

    // The owner line is GM context. A player already knows these are theirs, so
    // it would be noise — and it must not become a way to see through anonymity.
    expect(screen.queryByTestId('character-sheet-owner-1')).not.toBeInTheDocument();
  });

  it('still auto-opens the player\'s sole character', () => {
    const openCharacterSheet = vi.fn();
    renderWithProviders(
      <CharacterSheetPanel
        ctx={makeInGameCtx(
          {
            isGM: false,
            userRole: 'player',
            userCharacters: [makeGameCharacter({ id: 5, name: 'Only Mine' })],
            allGameCharacters: [
              makeGameCharacter({ id: 5, name: 'Only Mine' }),
              makeGameCharacter({ id: 6, name: 'Not Mine' }),
            ],
          },
          openCharacterSheet
        )}
      />
    );

    expect(openCharacterSheet).toHaveBeenCalledWith(5, expect.objectContaining({
      canEdit: true,
      canEditStats: false,
    }));
  });
});
