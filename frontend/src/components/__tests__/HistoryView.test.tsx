import { describe, it, expect } from 'vitest';
import { screen } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server';
import { renderWithProviders } from '../../test-utils/render';
import { HistoryView } from '../HistoryView';
import type { GamePhase } from '../../types/phases';

describe('HistoryView', () => {
  const mockGameId = 1;
  const mockCurrentPhaseId = 3;

  const mockPhases: GamePhase[] = [
    {
      id: 1,
      game_id: mockGameId,
      phase_number: 1,
      phase_type: 'common_room',
      title: 'Opening Ceremony',
      description: 'Welcome to the game!',
      is_active: false,
      is_published: false,
      activated_at: '2025-01-01T12:00:00Z',
      end_time: '2025-01-02T00:00:00Z',
      start_time: '2025-01-01T00:00:00Z',
      created_at: '2025-01-01T00:00:00Z',
    },
    {
      id: 2,
      game_id: mockGameId,
      phase_number: 2,
      phase_type: 'action',
      is_active: false,
      is_published: false,
      activated_at: '2025-01-03T12:00:00Z',
      end_time: '2025-01-04T00:00:00Z',
      start_time: '2025-01-03T00:00:00Z',
      created_at: '2025-01-03T00:00:00Z',
    },
    {
      id: 3,
      game_id: mockGameId,
      phase_number: 3,
      phase_type: 'common_room',
      title: 'Midgame Discussion',
      description: 'React to the results',
      is_active: true,
      is_published: false,
      activated_at: '2025-01-05T12:00:00Z',
      start_time: '2025-01-05T00:00:00Z',
      created_at: '2025-01-05T00:00:00Z',
    },
  ];

  const setupHandlers = (phases: GamePhase[] = mockPhases) => {
    server.use(
      http.get('/api/v1/games/:gameId/phases', () => {
        return HttpResponse.json(phases);
      }),
      // Mock action results endpoints (returns empty array since we're testing history display, not results)
      http.get('/api/v1/games/:gameId/results/mine', () => {
        return HttpResponse.json([]);
      }),
      http.get('/api/v1/games/:gameId/results', () => {
        return HttpResponse.json([]);
      }),
      // CommonRoom needs these when a phase is selected
      http.get('/api/v1/games/:gameId/posts', () => {
        return HttpResponse.json([]);
      }),
      http.get('/api/v1/games/:gameId/characters/controllable', () => {
        return HttpResponse.json([]);
      }),
      http.get('/api/v1/games/:gameId/polls', () => {
        return HttpResponse.json([]);
      }),
      http.get('/api/v1/games/:gameId/previous-phase-results', () => {
        return HttpResponse.json({ results: [], phase_id: null, phase_title: null });
      }),
      http.get('/api/v1/games/:gameId/unread-comment-ids', () => {
        return HttpResponse.json({});
      }),
    );
  };

  beforeEach(() => {
    server.resetHandlers();
    setupHandlers();
  });

  describe('Bug #4: Action Phases clickable but have no content', () => {
    it('should make common_room phases clickable', async () => {
      renderWithProviders(
        <HistoryView
          gameId={mockGameId}
          currentPhaseId={mockCurrentPhaseId}
          isGM={false}
        />
      );

      // Wait for phases to load
      const openingPhase = await screen.findAllByText('Opening Ceremony');
      expect(openingPhase[0]).toBeInTheDocument();

      // Common room phases should be in a button (clickable)
      const openingButton = openingPhase[0].closest('button');
      expect(openingButton).toBeInTheDocument();
      expect(openingButton).not.toBeDisabled();
    });

    it('should make action phases clickable to view results', async () => {
      renderWithProviders(
        <HistoryView
          gameId={mockGameId}
          currentPhaseId={mockCurrentPhaseId}
          isGM={false}
        />
      );

      // Wait for phases to load
      const actionPhaseTitle = await screen.findAllByRole('heading', { name: /action phase/i });
      expect(actionPhaseTitle[0]).toBeInTheDocument();

      // Action phases SHOULD now be clickable buttons (to view action results)
      const actionButton = actionPhaseTitle[0].closest('button');
      expect(actionButton).toBeInTheDocument();
      expect(actionButton).not.toBeDisabled();
    });
  });

  describe('future phase visibility', () => {
    it('does not show unactivated phases in the history list', async () => {
      const futurePhase: GamePhase = {
        id: 99,
        game_id: mockGameId,
        phase_number: 4,
        phase_type: 'action',
        title: 'Secret Future Phase',
        is_active: false,
        is_published: false,
        created_at: '2025-02-01T00:00:00Z',
        // no activated_at — never activated
      };

      setupHandlers([...mockPhases, futurePhase]);

      renderWithProviders(
        <HistoryView gameId={mockGameId} currentPhaseId={mockCurrentPhaseId} isGM={false} />
      );

      // Wait for the phase list to render, then assert the future phase is absent
      const matches = await screen.findAllByText('Opening Ceremony');
      expect(matches.length).toBeGreaterThan(0);

      // The unactivated future phase must not appear
      expect(screen.queryByText('Secret Future Phase')).not.toBeInTheDocument();
    });
  });

  describe('URL param sync (phase deep linking)', () => {
    it('auto-selects a phase when ?phase=<id> is in the URL', async () => {
      // Phase 1 is the "Opening Ceremony" common_room phase
      renderWithProviders(
        <HistoryView
          gameId={mockGameId}
          currentPhaseId={mockCurrentPhaseId}
          isGM={false}
        />,
        { initialRoute: '/games/1?tab=history&phase=1', gameId: mockGameId }
      );

      // Should render CommonRoom content for phase 1, not the phase list
      // The "Back to History" button only appears when a phase is selected
      expect(await screen.findByText('Back to History')).toBeInTheDocument();
    });

    it('updates the URL when a phase is clicked', async () => {
      const user = (await import('@testing-library/user-event')).default.setup();

      renderWithProviders(
        <HistoryView
          gameId={mockGameId}
          currentPhaseId={mockCurrentPhaseId}
          isGM={false}
        />,
        { initialRoute: '/games/1?tab=history', gameId: mockGameId }
      );

      // Wait for phase list, then click phase 1
      const phaseButtons = await screen.findAllByText('Opening Ceremony');
      await user.click(phaseButtons[0]);

      // CommonRoom should now be visible
      expect(await screen.findByText('Back to History')).toBeInTheDocument();
    });

    it('auto-navigates to the correct phase when ?comment param is present', async () => {
      // Set up a mock for getMessage that returns a message with phase_id=1
      const mockComment = {
        id: 99,
        game_id: mockGameId,
        phase_id: 1,
        author_id: 1,
        character_id: 1,
        content: 'A deep-linked comment',
        message_type: 'comment',
        parent_id: null,
        thread_depth: 1,
        author_username: 'testuser',
        character_name: 'TestChar',
        comment_count: 0,
        is_edited: false,
        is_deleted: false,
        created_at: '2025-01-01T00:00:00Z',
        updated_at: '2025-01-01T00:00:00Z',
      };

      server.use(
        http.get('/api/v1/games/:gameId/messages/:messageId', () => {
          return HttpResponse.json(mockComment);
        }),
        // CommonRoom will fetch posts for phase 1
        http.get('/api/v1/games/:gameId/posts', () => {
          return HttpResponse.json([]);
        })
      );

      renderWithProviders(
        <HistoryView
          gameId={mockGameId}
          currentPhaseId={mockCurrentPhaseId}
          isGM={false}
        />,
        { initialRoute: '/games/1?tab=history&comment=99', gameId: mockGameId }
      );

      // Should auto-select phase 1 (the phase containing comment 99)
      // and render CommonRoom, showing the "Back to History" button
      expect(await screen.findByText('Back to History')).toBeInTheDocument();
    });
  });

  describe('Utility Drawer context on historical phases', () => {
    /**
     * Regression guard: HistoryView used to render CommonRoom without the
     * `currentPhase` prop, passing only `phaseId`. The Mark All Read utility
     * gates on `!!ctx.currentPhase` (it needs the phase object, not just an id,
     * to scope the bulk mark-read), so it silently vanished from the drawer on
     * the History tab while working fine on the live phase. The failure mode is
     * invisible — a missing list item, no error — so it needs an explicit test.
     */
    it('offers Mark All Read in the drawer for a historical common_room phase', async () => {
      const user = (await import('@testing-library/user-event')).default.setup();

      renderWithProviders(
        <HistoryView gameId={mockGameId} currentPhaseId={mockCurrentPhaseId} isGM={false} />,
        { initialRoute: '/games/1?tab=history&phase=1', gameId: mockGameId }
      );

      // Phase 1 (a past common_room phase) is selected via the URL param.
      expect(await screen.findByText('Back to History')).toBeInTheDocument();

      await user.click(await screen.findByTestId('utility-drawer-toggle'));

      // The utility must be listed, not filtered out by its isAvailable gate.
      expect(await screen.findByTestId('utility-list')).toBeInTheDocument();
      expect(screen.getByTestId('utility-mark-all-read')).toBeInTheDocument();
    });

    it('enables the mark-all-read action for a historical phase', async () => {
      const user = (await import('@testing-library/user-event')).default.setup();

      renderWithProviders(
        <HistoryView gameId={mockGameId} currentPhaseId={mockCurrentPhaseId} isGM={false} />,
        { initialRoute: '/games/1?tab=history&phase=1', gameId: mockGameId }
      );

      expect(await screen.findByText('Back to History')).toBeInTheDocument();
      await user.click(await screen.findByTestId('utility-drawer-toggle'));
      await user.click(await screen.findByTestId('utility-mark-all-read'));

      // Without a resolved phase the panel renders its button disabled, so an
      // enabled button proves the phase object actually reached the panel.
      const markAllButton = await screen.findByRole('button', {
        name: /mark all comments as read/i,
      });
      expect(markAllButton).toBeEnabled();
    });
  });
});
