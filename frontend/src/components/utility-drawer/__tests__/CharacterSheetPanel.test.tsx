import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, fireEvent, waitFor } from '@testing-library/react';
import { renderWithProviders } from '../../../test-utils';
import { CharacterSheetPanel } from '../panels/CharacterSheetPanel';
import type { GameUtilityContext, UtilityContext } from '../types';
import type { Character, ControllableCharacterWithGame } from '../../../types/characters';

/** The signed-in user for these tests; the GM cases treat this id as the GM. */
const CURRENT_USER_ID = 42;

vi.mock('../../../lib/api', () => ({
  apiClient: {
    characters: {
      getControllableCharactersAcrossGames: vi.fn(),
    },
    // AuthProvider resolves the current user on mount. The panel needs its id to
    // tell a GM's own player character from one they merely oversee.
    auth: {
      getCurrentUser: vi.fn().mockResolvedValue({
        data: { id: 42, username: 'gm-under-test', email: 'gm@example.com' },
      }),
    },
    setAuthToken: vi.fn(),
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
  return { game: null, openCharacterSheet, openHandout: vi.fn(), closeDrawer: vi.fn() };
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
      isGameWritable: true,
      userRole: 'player',
      gameState: 'in_progress',
      isAnonymous: false,
      portraitAvatars: false,
      userCharacters: [makeGameCharacter()],
      allGameCharacters: [makeGameCharacter()],
      commentReadMode: 'manual',
      ...gameOverrides,
    },
    openCharacterSheet,
    openHandout: vi.fn(),
    closeDrawer: vi.fn(),
  };
}

function mockCharacters(characters: ControllableCharacterWithGame[]) {
  vi.mocked(apiClient.characters.getControllableCharactersAcrossGames).mockResolvedValue({
    data: characters,
  } as never);
}

/**
 * Re-arms the current-user stub. `vi.clearAllMocks()` in each beforeEach strips
 * mock implementations, so without this AuthProvider resolves to no user and the
 * panel can't tell a GM's own character from one they oversee.
 */
function mockCurrentUser() {
  vi.mocked(apiClient.auth.getCurrentUser).mockResolvedValue({
    data: { id: CURRENT_USER_ID, username: 'gm-under-test', email: 'gm@example.com' },
  } as never);
}

describe('CharacterSheetPanel — outside a game', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockCurrentUser();
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

  it('auto-opens the sheet when the user controls exactly one character', async () => {
    const openCharacterSheet = vi.fn();
    mockCharacters([
      makeCharacter({ id: 7, game_state: 'in_progress', user_role: 'player' }),
    ]);

    renderWithProviders(<CharacterSheetPanel ctx={makeGlobalCtx(openCharacterSheet)} />);

    // Nothing to choose between, so the picker is skipped entirely and the one
    // sheet opens with that game's permissions.
    await waitFor(() =>
      expect(openCharacterSheet).toHaveBeenCalledWith(7, {
        canEdit: true,
        canEditStats: false,
        isAnonymous: false,
        userRole: 'player',
        gameState: 'in_progress',
        portraitAvatars: false,
      })
    );
    expect(screen.queryByTestId('global-character-sheet-picker')).not.toBeInTheDocument();
  });

  it('carries the game\'s portrait-avatar setting through to the sheet', async () => {
    const openCharacterSheet = vi.fn();
    mockCharacters([makeCharacter({ id: 7, game_portrait_avatars: true })]);

    renderWithProviders(<CharacterSheetPanel ctx={makeGlobalCtx(openCharacterSheet)} />);

    // Out here there is no GameContext for the sheet to read the setting from,
    // so it has to ride along with the character or the sheet falls back to
    // circular avatars and misrepresents a portrait game.
    await waitFor(() =>
      expect(openCharacterSheet).toHaveBeenCalledWith(
        7,
        expect.objectContaining({ portraitAvatars: true })
      )
    );
  });

  it('opens a picked character with its own game\'s avatar shape', async () => {
    const openCharacterSheet = vi.fn();
    mockCharacters([
      makeCharacter({ id: 1, name: 'Kael Vance', game_portrait_avatars: false }),
      makeCharacter({
        id: 2,
        name: 'Mira Oduya',
        game_id: 11,
        game_title: 'Beta Game',
        game_portrait_avatars: true,
      }),
    ]);

    renderWithProviders(<CharacterSheetPanel ctx={makeGlobalCtx(openCharacterSheet)} />);

    fireEvent.click(await screen.findByTestId('character-sheet-open-2'));

    // The setting is per-game, so picking a character from a portrait game must
    // use that game's shape and not the first game's.
    expect(openCharacterSheet).toHaveBeenCalledWith(
      2,
      expect.objectContaining({ portraitAvatars: true })
    );
  });

  it('auto-opens only once, not on every re-render', async () => {
    const openCharacterSheet = vi.fn();
    mockCharacters([makeCharacter({ id: 7 })]);

    const { rerender } = renderWithProviders(
      <CharacterSheetPanel ctx={makeGlobalCtx(openCharacterSheet)} />
    );

    await waitFor(() => expect(openCharacterSheet).toHaveBeenCalledTimes(1));

    // Opening the sheet updates state above this panel, which re-renders it.
    // That must not re-fire and fling the modal open again.
    rerender(<CharacterSheetPanel ctx={makeGlobalCtx(openCharacterSheet)} />);
    await waitFor(() => expect(openCharacterSheet).toHaveBeenCalledTimes(1));
  });

  it('shows the picker rather than auto-opening when there is a choice', async () => {
    const openCharacterSheet = vi.fn();
    mockCharacters([
      makeCharacter({ id: 1, name: 'Kael Vance' }),
      makeCharacter({ id: 2, name: 'Mira Oduya', game_id: 11, game_title: 'Beta Game' }),
    ]);

    renderWithProviders(<CharacterSheetPanel ctx={makeGlobalCtx(openCharacterSheet)} />);

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
      portraitAvatars: false,
    });

    // The GM of a different game gets stat editing, and that game's anonymity.
    fireEvent.click(screen.getByTestId('character-sheet-open-2'));
    expect(openCharacterSheet).toHaveBeenCalledWith(2, {
      canEdit: true,
      canEditStats: true,
      isAnonymous: true,
      userRole: 'gm',
      gameState: 'in_progress',
      portraitAvatars: false,
    });
  });

  it('opens a completed game\'s sheet read-only', async () => {
    const openCharacterSheet = vi.fn();
    // A second character keeps the picker up, so the click is what opens the
    // sheet — with one character the panel auto-opens and there's nothing to
    // click. The read-only rule under test is the same either way.
    mockCharacters([
      makeCharacter({ id: 4, game_state: 'completed', user_role: 'gm' }),
      makeCharacter({ id: 5, name: 'Mira Oduya', game_id: 11, game_title: 'Beta Game' }),
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

/**
 * A GM off a game page used to see only their NPCs, because the cross-game
 * endpoint withheld the rest of the cast. The list is a cast reference for the
 * game they run, so it must hold every character in it — with the same owner
 * lines, NPC badges and no-auto-open behavior as the in-game list.
 */
describe('CharacterSheetPanel — outside a game, as GM', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockCurrentUser();
    localStorage.clear();
  });

  /** A GM's game: their own NPC plus two players' characters. */
  const gmCast = [
    makeCharacter({
      id: 1,
      name: 'Tavern Keeper',
      character_type: 'npc',
      user_role: 'gm',
      assigned_username: 'rob',
    }),
    makeCharacter({
      id: 2,
      name: 'Zara Quill',
      user_id: 99,
      username: 'dana',
      user_role: 'gm',
    }),
    makeCharacter({
      id: 3,
      name: 'Alden Roe',
      user_id: 98,
      username: 'sam',
      user_role: 'gm',
    }),
  ];

  it('offers every character in the game they run, not just their NPCs', async () => {
    mockCharacters(gmCast);

    renderWithProviders(<CharacterSheetPanel ctx={makeGlobalCtx()} />);

    expect(await screen.findByTestId('global-character-sheet-picker')).toBeInTheDocument();
    // The NPC was always here; the players' characters are the regression.
    expect(screen.getByTestId('character-sheet-open-1')).toBeInTheDocument();
    expect(screen.getByTestId('character-sheet-open-2')).toBeInTheDocument();
    expect(screen.getByTestId('character-sheet-open-3')).toBeInTheDocument();
  });

  it('shows who plays each character', async () => {
    mockCharacters(gmCast);

    renderWithProviders(<CharacterSheetPanel ctx={makeGlobalCtx()} />);

    await screen.findByTestId('global-character-sheet-picker');
    expect(screen.getByTestId('character-sheet-owner-2')).toHaveTextContent('dana');
    expect(screen.getByTestId('character-sheet-owner-3')).toHaveTextContent('sam');
    // An NPC credits its assignee, not its creator.
    expect(screen.getByTestId('character-sheet-owner-1')).toHaveTextContent('rob');
  });

  it('marks an unassigned NPC rather than leaving the line blank', async () => {
    mockCharacters([
      makeCharacter({ id: 8, name: 'Wandering Merchant', character_type: 'npc', user_role: 'gm' }),
      makeCharacter({ id: 9, name: 'Zara Quill', user_id: 99, username: 'dana', user_role: 'gm' }),
    ]);

    renderWithProviders(<CharacterSheetPanel ctx={makeGlobalCtx()} />);

    await screen.findByTestId('global-character-sheet-picker');
    expect(screen.getByTestId('character-sheet-owner-8')).toHaveTextContent('Unassigned');
  });

  it('marks which entries are NPCs', async () => {
    mockCharacters(gmCast);

    renderWithProviders(<CharacterSheetPanel ctx={makeGlobalCtx()} />);

    await screen.findByTestId('global-character-sheet-picker');
    expect(screen.getByTestId('character-sheet-npc-1')).toBeInTheDocument();
    expect(screen.queryByTestId('character-sheet-npc-2')).not.toBeInTheDocument();
  });

  /**
   * Auto-open is for "the one character you control". A GM's list is a cast
   * reference, so a game holding a single character they merely oversee must
   * still show the list rather than fling that sheet open.
   */
  it('never auto-opens a character it only oversees', async () => {
    const openCharacterSheet = vi.fn();
    mockCharacters([
      makeCharacter({ id: 4, name: 'Someone Else', user_id: 99, username: 'dana', user_role: 'gm' }),
    ]);

    renderWithProviders(<CharacterSheetPanel ctx={makeGlobalCtx(openCharacterSheet)} />);

    expect(await screen.findByTestId('global-character-sheet-picker')).toBeInTheDocument();
    expect(openCharacterSheet).not.toHaveBeenCalled();
  });

  /** A player's own single character still opens straight away. */
  it('still auto-opens a sole character the user actually plays', async () => {
    const openCharacterSheet = vi.fn();
    mockCharacters([makeCharacter({ id: 7, user_role: 'player' })]);

    renderWithProviders(<CharacterSheetPanel ctx={makeGlobalCtx(openCharacterSheet)} />);

    await waitFor(() => expect(openCharacterSheet).toHaveBeenCalledWith(7, expect.anything()));
  });

  it('opens an overseen character with GM stat-editing rights', async () => {
    const openCharacterSheet = vi.fn();
    mockCharacters(gmCast);

    renderWithProviders(<CharacterSheetPanel ctx={makeGlobalCtx(openCharacterSheet)} />);

    fireEvent.click(await screen.findByTestId('character-sheet-open-2'));
    expect(openCharacterSheet).toHaveBeenCalledWith(2, {
      canEdit: true,
      canEditStats: true,
      isAnonymous: false,
      userRole: 'gm',
      gameState: 'in_progress',
      portraitAvatars: false,
    });
  });

  /** Widening the GM's list must not widen anyone else's. */
  it('shows no owner line for a character in someone else\'s game', async () => {
    mockCharacters([
      makeCharacter({ id: 1, name: 'Kael Vance', user_role: 'player', username: 'me' }),
      makeCharacter({ id: 2, name: 'Mira Oduya', game_id: 11, game_title: 'Beta', user_role: 'player' }),
    ]);

    renderWithProviders(<CharacterSheetPanel ctx={makeGlobalCtx()} />);

    await screen.findByTestId('global-character-sheet-picker');
    // A player already knows these are theirs, and in an anonymous game the line
    // must not become a way to see who else is playing.
    expect(screen.queryByTestId('character-sheet-owner-1')).not.toBeInTheDocument();
    // No cast to filter, so no control.
    expect(screen.queryByTestId('cast-filter')).not.toBeInTheDocument();
  });
});

describe('CharacterSheetPanel — the All / Mine filter', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockCurrentUser();
    localStorage.clear();
  });

  /**
   * currentUser id 42 is the GM here. `mine` keeps their own player character
   * and every NPC (they can step in for any of them) and drops the two
   * characters belonging to other players.
   */
  const mixedCast = [
    makeCharacter({ id: 1, name: 'Tavern Keeper', character_type: 'npc', user_role: 'gm' }),
    makeCharacter({ id: 2, name: 'My Own Hero', user_id: 42, user_role: 'gm' }),
    makeCharacter({ id: 3, name: 'Zara Quill', user_id: 99, username: 'dana', user_role: 'gm' }),
    makeCharacter({ id: 4, name: 'Alden Roe', user_id: 98, username: 'sam', user_role: 'gm' }),
  ];

  it('offers the filter to a GM and defaults to the whole cast', async () => {
    mockCharacters(mixedCast);

    renderWithProviders(<CharacterSheetPanel ctx={makeGlobalCtx()} />);

    expect(await screen.findByTestId('cast-filter')).toBeInTheDocument();
    expect(screen.getByTestId('cast-filter-all')).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByTestId('character-sheet-open-3')).toBeInTheDocument();
  });

  it('narrows the list to what the GM plays when switched to Mine', async () => {
    mockCharacters(mixedCast);

    renderWithProviders(<CharacterSheetPanel ctx={makeGlobalCtx()} />);

    fireEvent.click(await screen.findByTestId('cast-filter-mine'));

    // Kept: any NPC, plus the GM's own player character.
    expect(screen.getByTestId('character-sheet-open-1')).toBeInTheDocument();
    expect(screen.getByTestId('character-sheet-open-2')).toBeInTheDocument();
    // Dropped: other players' characters.
    expect(screen.queryByTestId('character-sheet-open-3')).not.toBeInTheDocument();
    expect(screen.queryByTestId('character-sheet-open-4')).not.toBeInTheDocument();
  });

  it('keeps the filter on screen after filtering to nothing', async () => {
    // A GM who runs a game but plays nothing in it: only other players' PCs.
    mockCharacters([
      makeCharacter({ id: 3, name: 'Zara Quill', user_id: 99, username: 'dana', user_role: 'gm' }),
      makeCharacter({ id: 4, name: 'Alden Roe', user_id: 98, username: 'sam', user_role: 'gm' }),
    ]);

    renderWithProviders(<CharacterSheetPanel ctx={makeGlobalCtx()} />);

    fireEvent.click(await screen.findByTestId('cast-filter-mine'));

    // Without the control still rendered there is no way back to the cast.
    expect(screen.getByTestId('cast-filter')).toBeInTheDocument();
    expect(screen.getByText(/switch to "all"/i)).toBeInTheDocument();
  });

  it('remembers the choice across remounts', async () => {
    mockCharacters(mixedCast);

    const { unmount } = renderWithProviders(<CharacterSheetPanel ctx={makeGlobalCtx()} />);
    fireEvent.click(await screen.findByTestId('cast-filter-mine'));
    unmount();

    // Reopening the drawer must not silently revert to the full cast.
    renderWithProviders(<CharacterSheetPanel ctx={makeGlobalCtx()} />);
    expect(await screen.findByTestId('cast-filter-mine')).toHaveAttribute('aria-pressed', 'true');
    expect(screen.queryByTestId('character-sheet-open-3')).not.toBeInTheDocument();
  });

  it('is not offered to a player, who has nothing to filter', async () => {
    mockCharacters([
      makeCharacter({ id: 1, name: 'Kael Vance', user_id: 42, user_role: 'player' }),
      makeCharacter({ id: 2, name: 'Mira Oduya', user_id: 42, game_id: 11, user_role: 'player' }),
    ]);

    renderWithProviders(<CharacterSheetPanel ctx={makeGlobalCtx()} />);

    await screen.findByTestId('global-character-sheet-picker');
    expect(screen.queryByTestId('cast-filter')).not.toBeInTheDocument();
  });

  it('filters the in-game GM list too', () => {
    renderWithProviders(
      <CharacterSheetPanel
        ctx={makeInGameCtx({
          isGM: true,
          userRole: 'gm',
          userCharacters: [],
          allGameCharacters: [
            makeGameCharacter({ id: 1, name: 'Tavern Keeper', character_type: 'npc' }),
            makeGameCharacter({ id: 2, name: 'Zara Quill', username: 'dana' }),
          ],
        })}
      />
    );

    fireEvent.click(screen.getByTestId('cast-filter-mine'));

    expect(screen.getByTestId('character-sheet-open-1')).toBeInTheDocument();
    expect(screen.queryByTestId('character-sheet-open-2')).not.toBeInTheDocument();
  });

  it('is not offered to an in-game player', () => {
    renderWithProviders(
      <CharacterSheetPanel
        ctx={makeInGameCtx({
          isGM: false,
          userRole: 'player',
          userCharacters: [
            makeGameCharacter({ id: 1, name: 'Kael Vance' }),
            makeGameCharacter({ id: 2, name: 'Second Character' }),
          ],
        })}
      />
    );

    expect(screen.queryByTestId('cast-filter')).not.toBeInTheDocument();
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
      portraitAvatars: false,
    });
  });

  it('passes the game\'s portrait-avatar setting to the sheet', () => {
    const openCharacterSheet = vi.fn();
    renderWithProviders(
      <CharacterSheetPanel
        ctx={makeInGameCtx(
          {
            isGM: true,
            userRole: 'gm',
            allGameCharacters: cast,
            portraitAvatars: true,
          },
          openCharacterSheet
        )}
      />
    );

    fireEvent.click(screen.getByTestId('character-sheet-open-2'));
    expect(openCharacterSheet).toHaveBeenCalledWith(
      2,
      expect.objectContaining({ portraitAvatars: true })
    );
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
