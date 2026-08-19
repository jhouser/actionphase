import { describe, it, expect, beforeEach, vi } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server';
import { renderWithProviders } from '../../test-utils/render';
import { CommonRoom } from '../CommonRoom';
import { UtilityDrawerHarness } from '../../test-utils/utilityDrawer';
import type { Character } from '../../types/characters';

/**
 * Integration tests for the Utility Drawer wired into CommonRoom, focused on the
 * decisions whose failure would be silent or harmful (V&V):
 *
 *  - the drawer opens from the Common Room header (wiring / regression guard),
 *  - opening a character sheet from the drawer launches the CharacterSheet for
 *    the CORRECT character, with the CORRECT edit permission — a player editing
 *    their own sheet vs. a locked (completed-game) sheet. This proves the
 *    permission computed by useCharacterSheetPermissions actually reaches the
 *    component, rather than being hardcoded, and
 *  - opening the sheet closes the drawer so the modal stacks over the room.
 *
 * The drawer itself is mounted globally now; what CommonRoom contributes is the
 * game-scoped context (role, game state, controlled characters) those
 * permissions are computed from. That contribution is what's under test here.
 *
 * WHY THE REAL CharacterSheet IS NOT RENDERED HERE:
 * What these tests validate is the WIRING CommonRoom performs: which
 * `characterId` it passes, and what `canEdit` value it computes and hands to the
 * sheet. Those are props, and `canEdit=false` in particular has no reliable
 * rendered consequence to assert on — so a real mount would make the permission
 * check unobservable, which is the one thing here that would break silently on a
 * refactor. The probe reports the received props via the DOM instead.
 *
 * The sheet's own behaviour is covered by CharacterSheet.test.tsx. Note that this
 * mock previously existed because the real component's render tree hung under
 * jsdom; that loop is fixed, and the mock is kept for the scoping reason above.
 */

// Probe standing in for the real CharacterSheet. It records the props CommonRoom
// passes so the tests can assert on the wired characterId and edit permission.
vi.mock('../CharacterSheet', () => ({
  CharacterSheet: ({ characterId, canEdit }: { characterId: number; canEdit?: boolean }) => (
    <div
      data-testid="character-sheet-probe"
      data-character-id={String(characterId)}
      data-can-edit={String(!!canEdit)}
    />
  ),
}));

// The signed-in user (auth/me → id 1). In these tests they are a PLAYER, so the
// game is owned by a different GM (gm_user_id 2) and the user is a participant.
const PLAYER_USER_ID = 1;
const GM_USER_ID = 2;

const myCharacter: Character = {
  id: 1,
  game_id: 1,
  name: 'Kael',
  character_type: 'player_character',
  user_id: PLAYER_USER_ID,
  assigned_user_id: PLAYER_USER_ID,
  status: 'approved',
  created_at: '2024-01-01T00:00:00Z',
};

/**
 * Wire up a game the signed-in user plays in as a normal player, controlling a
 * single approved character. `gameState` lets a test lock the game (completed).
 */
function setupPlayerGame(gameState: 'in_progress' | 'completed' = 'in_progress') {
  server.use(
    http.get('/api/v1/auth/me', () =>
      HttpResponse.json({ id: PLAYER_USER_ID, username: 'player', email: 'player@example.com' })
    ),
    http.get('/api/v1/games/:gameId/details', ({ params }) =>
      HttpResponse.json({
        id: Number(params.gameId),
        title: 'Test Game',
        description: 'A test game',
        gm_user_id: GM_USER_ID,
        gm_username: 'thegm',
        state: gameState,
        max_players: 4,
        is_public: true,
        is_anonymous: false,
        auto_accept_audience: false,
        game_config: {},
        created_at: '2024-01-01T00:00:00Z',
        updated_at: '2024-01-01T00:00:00Z',
      })
    ),
    http.get('/api/v1/games/:gameId/participants', () =>
      HttpResponse.json([
        { id: 1, user_id: GM_USER_ID, username: 'thegm', role: 'gm', status: 'active' },
        { id: 2, user_id: PLAYER_USER_ID, username: 'player', role: 'player', status: 'active' },
      ])
    ),
    // userCharacters (drawer availability + sole-character auto-open + ownership)
    http.get('/api/v1/games/:gameId/characters/controllable', () =>
      HttpResponse.json([myCharacter])
    ),
    // allGameCharacters (permission lookup for the sheet)
    http.get('/api/v1/games/:gameId/characters', () => HttpResponse.json([myCharacter])),
    // Common room content — no posts needed for these tests.
    http.get('/api/v1/games/:gameId/posts', () => HttpResponse.json([])),
    http.get('/api/v1/games/:gameId/unread-comment-ids', () => HttpResponse.json([])),
    http.get('/api/v1/games/:gameId/phases/:phaseId/polls', () => HttpResponse.json([]))
  );
}

/**
 * Render CommonRoom for the wired-up player game and open the Utility Drawer.
 *
 * The drawer and the sheet modal it launches now live at the app root rather
 * than inside CommonRoom, and the button that opens them lives in the global
 * nav. UtilityDrawerHarness supplies both, mirroring how App composes them —
 * the provider itself comes from renderWithProviders. CommonRoom contributes
 * its game context to that provider while mounted.
 */
async function renderAndOpenDrawer() {
  const user = userEvent.setup();
  renderWithProviders(
    <>
      <CommonRoom gameId={1} phaseId={1} isCurrentPhase={true} />
      <UtilityDrawerHarness />
    </>,
    { gameId: 1 }
  );
  await waitFor(() =>
    expect(screen.getByTestId('utility-drawer-toggle')).toBeInTheDocument()
  );
  await user.click(screen.getByTestId('utility-drawer-toggle'));
  return user;
}

describe('CommonRoom — Utility Drawer integration', () => {
  beforeEach(() => {
    // Sheet modal renders into a portal; jsdom needs a stub for this.
    Element.prototype.scrollIntoView = () => {};
  });

  it('offers the room-appropriate utilities when the drawer is opened', async () => {
    setupPlayerGame();
    await renderAndOpenDrawer();

    // The utility list appears with the registry-driven utilities.
    expect(await screen.findByTestId('utility-list')).toBeInTheDocument();
    expect(screen.getByTestId('utility-dice-roller')).toBeInTheDocument();
    expect(screen.getByTestId('utility-character-sheet')).toBeInTheDocument();
  });

  it('opens the sheet for the correct character and, for a player, allows editing it', async () => {
    setupPlayerGame('in_progress');
    const user = await renderAndOpenDrawer();

    // Sole controlled character → selecting the utility opens that sheet directly.
    await user.click(await screen.findByTestId('utility-character-sheet'));

    // The sheet is launched for Kael (id 1)...
    const probe = await screen.findByTestId('character-sheet-probe');
    expect(probe).toHaveAttribute('data-character-id', '1');

    // ...and it is editable: a player may edit their own approved character, so
    // CommonRoom computed canEdit=true and passed it through.
    expect(probe).toHaveAttribute('data-can-edit', 'true');
  });

  it('opens the sheet read-only when the game is completed (permission flows through)', async () => {
    setupPlayerGame('completed');
    const user = await renderAndOpenDrawer();

    await user.click(await screen.findByTestId('utility-character-sheet'));

    // Sheet still opens for the character...
    const probe = await screen.findByTestId('character-sheet-probe');
    expect(probe).toHaveAttribute('data-character-id', '1');

    // ...but editing is disabled in a completed game. This proves canEdit is
    // computed from game state and passed through, not hardcoded to true.
    expect(probe).toHaveAttribute('data-can-edit', 'false');
  });

  it('closes the drawer when a sheet is opened, so the modal stacks over the room', async () => {
    setupPlayerGame('in_progress');
    const user = await renderAndOpenDrawer();

    await user.click(await screen.findByTestId('utility-character-sheet'));
    await screen.findByTestId('character-sheet-probe');

    // The drawer's utility list is no longer present once the sheet is open.
    await waitFor(() =>
      expect(screen.queryByTestId('utility-list')).not.toBeInTheDocument()
    );
  });
});
