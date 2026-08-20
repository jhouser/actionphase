import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, fireEvent, waitFor } from '@testing-library/react';
import { renderWithProviders } from '../../../test-utils';
import { HandoutsPanel } from '../panels/HandoutsPanel';
import type { GameUtilityContext, UtilityContext } from '../types';
import type { HandoutWithGame } from '../../../types/handouts';

vi.mock('../../../lib/api', () => ({
  apiClient: {
    handouts: {
      listHandouts: vi.fn(),
      listHandoutsAcrossGames: vi.fn(),
    },
    auth: {
      getCurrentUser: vi.fn().mockResolvedValue({
        data: { id: 42, username: 'user-under-test', email: 'user@example.com' },
      }),
    },
    setAuthToken: vi.fn(),
  },
}));

import { apiClient } from '../../../lib/api';

function makeHandout(overrides: Partial<HandoutWithGame> = {}): HandoutWithGame {
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

/** Global (no game in scope) drawer context. */
function makeGlobalCtx(openHandout = vi.fn()): UtilityContext {
  return {
    game: null,
    openCharacterSheet: vi.fn(),
    openHandout,
    closeDrawer: vi.fn(),
  };
}

/** An in-game drawer context. */
function makeInGameCtx(
  gameOverrides: Partial<GameUtilityContext> = {},
  openHandout = vi.fn()
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
      portraitAvatars: false,
      userCharacters: [],
      allGameCharacters: [],
      commentReadMode: 'auto',
      ...gameOverrides,
    },
    openCharacterSheet: vi.fn(),
    openHandout,
    closeDrawer: vi.fn(),
  };
}

describe('HandoutsPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('inside a game', () => {
    it("lists the game's published handouts", async () => {
      vi.mocked(apiClient.handouts.listHandouts).mockResolvedValue({
        data: [
          makeHandout({ id: 1, title: 'Tavern Rules' }),
          makeHandout({ id: 2, title: 'World Lore' }),
        ],
      } as never);

      renderWithProviders(<HandoutsPanel ctx={makeInGameCtx()} />);

      expect(await screen.findByText('Tavern Rules')).toBeInTheDocument();
      expect(screen.getByText('World Lore')).toBeInTheDocument();
      // Scoped to the game on screen, so no game grouping.
      expect(screen.queryByTestId('global-handout-picker')).not.toBeInTheDocument();
    });

    it('fetches handouts for the game currently in scope', async () => {
      vi.mocked(apiClient.handouts.listHandouts).mockResolvedValue({ data: [] } as never);

      renderWithProviders(<HandoutsPanel ctx={makeInGameCtx({ gameId: 77 })} />);

      await waitFor(() => expect(apiClient.handouts.listHandouts).toHaveBeenCalledWith(77));
      // The cross-game endpoint is for the no-game case only; hitting it here
      // would pull in handouts from unrelated games.
      expect(apiClient.handouts.listHandoutsAcrossGames).not.toHaveBeenCalled();
    });

    /**
     * The per-game endpoint hands a GM their drafts, since it also backs the
     * Handouts tab where drafts are edited. The drawer only reads, so a draft
     * must not appear even for the GM who wrote it.
     */
    it('hides draft handouts from the GM', async () => {
      vi.mocked(apiClient.handouts.listHandouts).mockResolvedValue({
        data: [
          makeHandout({ id: 1, title: 'Tavern Rules', status: 'published' }),
          makeHandout({ id: 2, title: 'Secret Map', status: 'draft' }),
        ],
      } as never);

      renderWithProviders(<HandoutsPanel ctx={makeInGameCtx({ isGM: true })} />);

      expect(await screen.findByText('Tavern Rules')).toBeInTheDocument();
      expect(screen.queryByText('Secret Map')).not.toBeInTheDocument();
    });

    it('opens the clicked handout', async () => {
      const openHandout = vi.fn();
      const handout = makeHandout({ id: 5, title: 'Tavern Rules' });
      vi.mocked(apiClient.handouts.listHandouts).mockResolvedValue({
        data: [handout],
      } as never);

      renderWithProviders(
        <HandoutsPanel ctx={makeInGameCtx({ isGM: true }, openHandout)} />
      );

      fireEvent.click(await screen.findByTestId('handout-open-5'));

      // No role or options: the drawer's modal is read-only for everyone, so
      // there is nothing role-dependent left to pass.
      expect(openHandout).toHaveBeenCalledWith(
        expect.objectContaining({ id: 5, title: 'Tavern Rules' })
      );
    });

    /**
     * Unlike the character-sheet panel, a lone handout must still be presented
     * as a list rather than flung open — handouts are reference material whose
     * count changes as the GM publishes more.
     */
    it('shows a list rather than auto-opening a single handout', async () => {
      const openHandout = vi.fn();
      vi.mocked(apiClient.handouts.listHandouts).mockResolvedValue({
        data: [makeHandout({ id: 1, title: 'Tavern Rules' })],
      } as never);

      renderWithProviders(<HandoutsPanel ctx={makeInGameCtx({}, openHandout)} />);

      expect(await screen.findByTestId('handout-picker')).toBeInTheDocument();
      expect(screen.getByText('Tavern Rules')).toBeInTheDocument();
      expect(openHandout).not.toHaveBeenCalled();
    });

    it('reports a game with no published handouts', async () => {
      vi.mocked(apiClient.handouts.listHandouts).mockResolvedValue({ data: [] } as never);

      renderWithProviders(<HandoutsPanel ctx={makeInGameCtx()} />);

      expect(
        await screen.findByText('This game has no published handouts yet.')
      ).toBeInTheDocument();
    });
  });

  describe('outside a game', () => {
    it('groups handouts from every game under its title', async () => {
      vi.mocked(apiClient.handouts.listHandoutsAcrossGames).mockResolvedValue({
        data: [
          makeHandout({ id: 1, game_id: 10, title: 'Tavern Rules', game_title: 'Alpha Game' }),
          makeHandout({ id: 2, game_id: 11, title: 'Faction Primer', game_title: 'Beta Game' }),
        ],
      } as never);

      renderWithProviders(<HandoutsPanel ctx={makeGlobalCtx()} />);

      expect(await screen.findByText('Alpha Game')).toBeInTheDocument();
      expect(screen.getByText('Beta Game')).toBeInTheDocument();
      expect(screen.getByText('Tavern Rules')).toBeInTheDocument();
      expect(screen.getByText('Faction Primer')).toBeInTheDocument();
    });

    it('collapses and re-expands a game group', async () => {
      vi.mocked(apiClient.handouts.listHandoutsAcrossGames).mockResolvedValue({
        data: [
          makeHandout({ id: 1, game_id: 10, title: 'Tavern Rules', game_title: 'Alpha Game' }),
          makeHandout({ id: 2, game_id: 11, title: 'Faction Primer', game_title: 'Beta Game' }),
        ],
      } as never);

      renderWithProviders(<HandoutsPanel ctx={makeGlobalCtx()} />);

      // Groups start expanded.
      const toggle = await screen.findByTestId('handout-group-toggle-10');
      expect(toggle).toHaveAttribute('aria-expanded', 'true');
      expect(screen.getByText('Tavern Rules')).toBeInTheDocument();

      fireEvent.click(toggle);

      expect(toggle).toHaveAttribute('aria-expanded', 'false');
      expect(screen.queryByText('Tavern Rules')).not.toBeInTheDocument();
      // Collapsing one game must not touch another.
      expect(screen.getByText('Faction Primer')).toBeInTheDocument();

      fireEvent.click(toggle);

      expect(toggle).toHaveAttribute('aria-expanded', 'true');
      expect(screen.getByText('Tavern Rules')).toBeInTheDocument();
    });

    it('opens a handout from its own game', async () => {
      const openHandout = vi.fn();
      vi.mocked(apiClient.handouts.listHandoutsAcrossGames).mockResolvedValue({
        data: [makeHandout({ id: 7, game_id: 11, title: 'Faction Primer', game_title: 'Beta Game' })],
      } as never);

      renderWithProviders(<HandoutsPanel ctx={makeGlobalCtx(openHandout)} />);

      fireEvent.click(await screen.findByTestId('handout-open-7'));

      expect(openHandout).toHaveBeenCalledWith(
        expect.objectContaining({ id: 7, game_id: 11 })
      );
    });

    it('reports when no active game has published handouts', async () => {
      vi.mocked(apiClient.handouts.listHandoutsAcrossGames).mockResolvedValue({
        data: [],
      } as never);

      renderWithProviders(<HandoutsPanel ctx={makeGlobalCtx()} />);

      expect(
        await screen.findByText('None of your active games have published handouts.')
      ).toBeInTheDocument();
    });

    it('surfaces a load failure', async () => {
      vi.mocked(apiClient.handouts.listHandoutsAcrossGames).mockRejectedValue(
        new Error('network down')
      );

      renderWithProviders(<HandoutsPanel ctx={makeGlobalCtx()} />);

      expect(await screen.findByTestId('global-handouts-error')).toBeInTheDocument();
    });
  });
});
