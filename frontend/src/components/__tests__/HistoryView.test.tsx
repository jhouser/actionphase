import { describe, it, expect } from 'vitest';
import { screen } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server';
import { renderWithProviders } from '../../test-utils/render';
import { HistoryView } from '../HistoryView';
import { UtilityDrawerHarness } from '../../test-utils/utilityDrawer';
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

    /**
     * Documents the API contract this resolver depends on.
     *
     * The resolver can only pick a phase if the comment tells it which one. The
     * API serialises phase_id with `omitempty`, so a NULL column disappears from
     * the response entirely rather than arriving as null — and the resolver's
     * `if (message.phase_id)` guard then returns without selecting anything. The
     * user lands on the History tab with no phase and no comment loaded, which is
     * exactly the reported notification bug.
     *
     * Replies posted from the Dashboard unread inbox and the New Comments view
     * used to store phase_id = NULL because neither surface has a phase in scope.
     * The backend now derives it from the parent message, so this response shape
     * should no longer occur in practice. This test pins the frontend's half of
     * that contract: the resolver degrades gracefully (phase list stays visible,
     * no crash, no bogus phase selected) rather than doing something worse.
     */
    it('falls back to the phase list when the comment has no phase_id', async () => {
      const commentWithoutPhase = {
        id: 99,
        game_id: mockGameId,
        // phase_id intentionally absent — what `omitempty` produces for a NULL.
        author_id: 1,
        character_id: 1,
        content: 'A comment with no phase',
        message_type: 'comment',
        parent_id: 42,
        thread_depth: 5,
        author_username: 'testuser',
        character_name: 'TestChar',
        comment_count: 0,
        is_edited: false,
        is_deleted: false,
        created_at: '2025-01-01T00:00:00Z',
        updated_at: '2025-01-01T00:00:00Z',
      };

      server.use(
        http.get('/api/v1/games/:gameId/messages/:messageId', () =>
          HttpResponse.json(commentWithoutPhase)
        )
      );

      renderWithProviders(
        <HistoryView
          gameId={mockGameId}
          currentPhaseId={mockCurrentPhaseId}
          isGM={false}
        />,
        { initialRoute: '/games/1?tab=history&comment=99', gameId: mockGameId }
      );

      // The phase list remains, and no phase is auto-selected.
      const phases = await screen.findAllByText('Opening Ceremony');
      expect(phases.length).toBeGreaterThan(0);
      expect(screen.queryByText('Back to History')).not.toBeInTheDocument();
    });
  });

  describe('character avatars on action phases', () => {
    /**
     * Regression guard: HistoryView rendered CharacterAvatar with only
     * `characterName`, never passing `avatarUrl`. CharacterAvatar falls back to
     * initials whenever `avatarUrl` is absent, so every submission and result in
     * the History tab showed a coloured initials bubble even for characters that
     * had uploaded a portrait — while the same character rendered correctly in
     * ActionsList and AllActionSubmissionsView, which look the URL up from the
     * game context by character_id.
     */
    const avatarCharacter = {
      id: 7,
      game_id: mockGameId,
      user_id: 1,
      name: 'Portrait Pat',
      avatar_url: 'https://cdn.example.com/avatars/pat.jpg',
      status: 'active',
      created_at: '2025-01-01T00:00:00Z',
      updated_at: '2025-01-01T00:00:00Z',
    };

    const setupAvatarHandlers = () => {
      server.use(
        // Populates GameContext.allGameCharacters, the avatar_url source.
        http.get('/api/v1/games/:gameId/characters', () =>
          HttpResponse.json([avatarCharacter])
        ),
        http.get('/api/v1/games/:gameId/actions/mine', () =>
          HttpResponse.json([
            {
              id: 501,
              game_id: mockGameId,
              phase_id: 2,
              user_id: 1,
              character_id: avatarCharacter.id,
              character_name: avatarCharacter.name,
              content: 'I scale the north wall under cover of dark.',
              is_draft: false,
              submitted_at: '2025-01-03T18:00:00Z',
              created_at: '2025-01-03T18:00:00Z',
              updated_at: '2025-01-03T18:00:00Z',
            },
          ])
        )
      );
    };

    it('shows the uploaded portrait for an action submission', async () => {
      setupAvatarHandlers();

      renderWithProviders(
        <HistoryView gameId={mockGameId} currentPhaseId={mockCurrentPhaseId} isGM={false} />,
        // Phase 2 is the action phase; submissions is the default sub-tab.
        { initialRoute: '/games/1?tab=history&phase=2&subTab=submissions', gameId: mockGameId }
      );

      // The submission renders, proving we are on the right phase and tab.
      expect(
        await screen.findByText('I scale the north wall under cover of dark.')
      ).toBeInTheDocument();

      // The avatar must be the real image, not the initials fallback ("PP").
      const avatar = await screen.findByRole('img', { name: avatarCharacter.name });
      expect(avatar).toHaveAttribute('src', avatarCharacter.avatar_url);
      expect(screen.queryByText('PP')).not.toBeInTheDocument();
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
        <>
          <HistoryView gameId={mockGameId} currentPhaseId={mockCurrentPhaseId} isGM={false} />
          {/* The drawer is mounted at the app root now, and opened from the
              global nav, so both come from the harness alongside the view
              that contributes its game context. */}
          <UtilityDrawerHarness />
        </>,
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
        <>
          <HistoryView gameId={mockGameId} currentPhaseId={mockCurrentPhaseId} isGM={false} />
          {/* The drawer is mounted at the app root now, and opened from the
              global nav, so both come from the harness alongside the view
              that contributes its game context. */}
          <UtilityDrawerHarness />
        </>,
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

  // Action submissions and results used to live in a separate Audience tab.
  // Folding them in here means HistoryView must pick each role's endpoint:
  // audience members have no characters of their own, so the per-user
  // endpoints the player path uses return nothing for them, and the GM
  // endpoint rejects them outright.
  describe('audience data routing', () => {
    const renderForAudience = () =>
      renderWithProviders(
        <HistoryView
          gameId={mockGameId}
          currentPhaseId={mockCurrentPhaseId}
          isGM={false}
          isAudience={true}
        />,
        { initialRoute: `/?phase=2&subTab=submissions` }
      );

    it('reads submissions from the audience endpoint, not the GM or per-user one', async () => {
      const calls: string[] = [];
      server.use(
        http.get('/api/v1/games/:gameId/action-submissions/all', ({ request }) => {
          calls.push('audience');
          const url = new URL(request.url);
          expect(url.searchParams.get('phase_id')).toBe('2');
          return HttpResponse.json({
            action_submissions: [
              {
                id: 91,
                game_id: mockGameId,
                user_id: 7,
                phase_id: 2,
                content: 'The whole cast can be watched.',
                character_name: 'Vera',
                username: 'vera_player',
                submitted_at: '2025-01-03T13:00:00Z',
                updated_at: '2025-01-03T13:00:00Z',
                status: 'submitted',
              },
            ],
            total: 1,
          });
        }),
        http.get('/api/v1/games/:gameId/actions', () => {
          calls.push('gm');
          return HttpResponse.json([]);
        }),
        http.get('/api/v1/games/:gameId/actions/mine', () => {
          calls.push('user');
          return HttpResponse.json([]);
        }),
      );

      renderForAudience();

      expect(await screen.findByText('The whole cast can be watched.')).toBeInTheDocument();
      expect(calls).toContain('audience');
      expect(calls).not.toContain('gm');
      expect(calls).not.toContain('user');
    });

    it('reads results from the game-wide endpoint rather than the per-user one', async () => {
      const calls: string[] = [];
      server.use(
        http.get('/api/v1/games/:gameId/action-submissions/all', () =>
          HttpResponse.json({ action_submissions: [], total: 0 })
        ),
        http.get('/api/v1/games/:gameId/results', () => {
          calls.push('game');
          return HttpResponse.json([
            {
              id: 500,
              game_id: mockGameId,
              user_id: 7,
              phase_id: 2,
              gm_user_id: 1,
              content: 'A result the audience may read.',
              is_published: true,
              character_name: 'Vera',
              sent_at: '2025-01-03T14:00:00Z',
            },
          ]);
        }),
        http.get('/api/v1/games/:gameId/results/mine', () => {
          calls.push('mine');
          return HttpResponse.json([]);
        }),
      );

      renderWithProviders(
        <HistoryView
          gameId={mockGameId}
          currentPhaseId={mockCurrentPhaseId}
          isGM={false}
          isAudience={true}
        />,
        { initialRoute: `/?phase=2&subTab=results` }
      );

      expect(await screen.findByText('A result the audience may read.')).toBeInTheDocument();
      expect(calls).toContain('game');
      expect(calls).not.toContain('mine');
    });

    // Audience is a trusted spectator role and deliberately sees GM drafts, so
    // spectators can watch results take shape. The badge matters as much as the
    // content — an audience member must be able to tell a work in progress from
    // something the GM has actually sent.
    it('shows unpublished drafts to audience, marked as drafts', async () => {
      server.use(
        http.get('/api/v1/games/:gameId/action-submissions/all', () =>
          HttpResponse.json({ action_submissions: [], total: 0 })
        ),
        http.get('/api/v1/games/:gameId/results', () =>
          HttpResponse.json([
            {
              id: 503,
              game_id: mockGameId,
              user_id: 7,
              phase_id: 2,
              gm_user_id: 1,
              content: 'Still being written by the GM.',
              is_published: false,
              character_name: 'Vera',
            },
          ])
        ),
      );

      renderWithProviders(
        <HistoryView
          gameId={mockGameId}
          currentPhaseId={mockCurrentPhaseId}
          isGM={false}
          isAudience={true}
        />,
        { initialRoute: `/?phase=2&subTab=results` }
      );

      expect(await screen.findByText('Still being written by the GM.')).toBeInTheDocument();
      expect(screen.getByText(/draft \(unpublished\)/i)).toBeInTheDocument();
    });

    // A standalone result answers no submission. History filters results by
    // phase alone, so these render without special handling — the property the
    // retired AllActionSubmissionsView needed extra code to preserve.
    it('renders results that have no parent submission', async () => {
      server.use(
        http.get('/api/v1/games/:gameId/action-submissions/all', () =>
          HttpResponse.json({ action_submissions: [], total: 0 })
        ),
        http.get('/api/v1/games/:gameId/results', () =>
          HttpResponse.json([
            {
              id: 501,
              game_id: mockGameId,
              user_id: 7,
              phase_id: 2,
              gm_user_id: 1,
              content: 'A raven arrives, unprompted.',
              is_published: true,
              character_name: 'Cass',
              sent_at: '2025-01-03T15:00:00Z',
            },
            {
              id: 502,
              game_id: mockGameId,
              user_id: 7,
              phase_id: 2,
              gm_user_id: 1,
              action_submission_id: 91,
              content: 'And a second, answering the first.',
              is_published: true,
              character_name: 'Cass',
              sent_at: '2025-01-03T16:00:00Z',
            },
          ])
        ),
      );

      renderWithProviders(
        <HistoryView
          gameId={mockGameId}
          currentPhaseId={mockCurrentPhaseId}
          isGM={false}
          isAudience={true}
        />,
        { initialRoute: `/?phase=2&subTab=results` }
      );

      expect(await screen.findByText('A raven arrives, unprompted.')).toBeInTheDocument();
      expect(screen.getByText('And a second, answering the first.')).toBeInTheDocument();
    });
  });

  // Submissions and results for the whole cast now share one phase drill-down,
  // which can run to dozens of entries. The character filter narrows it, using
  // OR semantics: selecting two characters shows both, not their intersection
  // (which would always be empty — a row belongs to exactly one character).
  describe('character filter', () => {
    const VERA = 11;
    const CASS = 12;
    const RUE = 13;

    // Rue appears only in results, so the chip row can only be complete if it is
    // built from submissions and results together.
    const setupFilterHandlers = () => {
      server.use(
        http.get('/api/v1/games/:gameId/actions', () =>
          HttpResponse.json([
            {
              id: 201,
              game_id: mockGameId,
              phase_id: 2,
              user_id: 7,
              character_id: VERA,
              character_name: 'Vera',
              content: 'Vera picks the lock.',
              is_draft: false,
              submitted_at: '2025-01-03T13:00:00Z',
            },
            {
              id: 202,
              game_id: mockGameId,
              phase_id: 2,
              user_id: 8,
              character_id: CASS,
              character_name: 'Cass',
              content: 'Cass keeps watch.',
              is_draft: false,
              submitted_at: '2025-01-03T13:05:00Z',
            },
          ])
        ),
        http.get('/api/v1/games/:gameId/results', () =>
          HttpResponse.json([
            {
              id: 301,
              game_id: mockGameId,
              phase_id: 2,
              user_id: 7,
              gm_user_id: 1,
              character_id: VERA,
              character_name: 'Vera',
              content: 'The lock gives way.',
              is_published: true,
              sent_at: '2025-01-03T14:00:00Z',
            },
            {
              id: 302,
              game_id: mockGameId,
              phase_id: 2,
              user_id: 9,
              gm_user_id: 1,
              character_id: RUE,
              character_name: 'Rue',
              content: 'Rue hears footsteps.',
              is_published: true,
              sent_at: '2025-01-03T14:05:00Z',
            },
          ])
        )
      );
    };

    const renderAsGM = (subTab: 'submissions' | 'results' = 'submissions') =>
      renderWithProviders(
        <HistoryView gameId={mockGameId} currentPhaseId={mockCurrentPhaseId} isGM={true} />,
        { initialRoute: `/?phase=2&subTab=${subTab}`, gameId: mockGameId }
      );

    it('offers a chip for every character in the phase, from submissions and results alike', async () => {
      setupFilterHandlers();
      renderAsGM();

      // Rue has no submission at all — only a result — so a chip row derived from
      // the active tab alone would omit them.
      expect(await screen.findByRole('button', { name: 'Rue' })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Vera' })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Cass' })).toBeInTheDocument();
    });

    it('shows only the selected character once a chip is picked', async () => {
      const user = (await import('@testing-library/user-event')).default.setup();
      setupFilterHandlers();
      renderAsGM();

      // Both submissions render before any filter is applied.
      expect(await screen.findByText('Vera picks the lock.')).toBeInTheDocument();
      expect(screen.getByText('Cass keeps watch.')).toBeInTheDocument();

      await user.click(screen.getByRole('button', { name: 'Vera' }));

      expect(screen.getByText('Vera picks the lock.')).toBeInTheDocument();
      expect(screen.queryByText('Cass keeps watch.')).not.toBeInTheDocument();
    });

    it('ORs the selection rather than intersecting it', async () => {
      const user = (await import('@testing-library/user-event')).default.setup();
      setupFilterHandlers();
      renderAsGM();

      expect(await screen.findByText('Vera picks the lock.')).toBeInTheDocument();

      await user.click(screen.getByRole('button', { name: 'Vera' }));
      await user.click(screen.getByRole('button', { name: 'Cass' }));

      // An AND filter would show nothing here: no submission belongs to both.
      expect(screen.getByText('Vera picks the lock.')).toBeInTheDocument();
      expect(screen.getByText('Cass keeps watch.')).toBeInTheDocument();
    });

    it('deselects a chip when it is clicked again', async () => {
      const user = (await import('@testing-library/user-event')).default.setup();
      setupFilterHandlers();
      renderAsGM();

      expect(await screen.findByText('Cass keeps watch.')).toBeInTheDocument();

      await user.click(screen.getByRole('button', { name: 'Vera' }));
      expect(screen.queryByText('Cass keeps watch.')).not.toBeInTheDocument();

      await user.click(screen.getByRole('button', { name: 'Vera' }));
      expect(screen.getByText('Cass keeps watch.')).toBeInTheDocument();
    });

    it('restores every row when the filter is cleared', async () => {
      const user = (await import('@testing-library/user-event')).default.setup();
      setupFilterHandlers();
      renderAsGM();

      expect(await screen.findByText('Vera picks the lock.')).toBeInTheDocument();
      await user.click(screen.getByRole('button', { name: 'Vera' }));
      expect(screen.queryByText('Cass keeps watch.')).not.toBeInTheDocument();

      await user.click(screen.getByRole('button', { name: /clear filters/i }));

      expect(screen.getByText('Vera picks the lock.')).toBeInTheDocument();
      expect(screen.getByText('Cass keeps watch.')).toBeInTheDocument();
    });

    // The chip row is deliberately shared across the two sub-tabs: a GM narrowing
    // to one character wants to follow that character from their submission to
    // the result answering it, without re-picking the filter on each tab.
    it('keeps the selection when switching between Submissions and Results', async () => {
      const user = (await import('@testing-library/user-event')).default.setup();
      setupFilterHandlers();
      renderAsGM();

      expect(await screen.findByText('Vera picks the lock.')).toBeInTheDocument();
      await user.click(screen.getByRole('button', { name: 'Vera' }));

      await user.click(screen.getByRole('button', { name: 'Results' }));

      // Vera's result survives the filter; Rue's is excluded by it.
      expect(await screen.findByText('The lock gives way.')).toBeInTheDocument();
      expect(screen.queryByText('Rue hears footsteps.')).not.toBeInTheDocument();
    });

    it('explains an empty list caused by the filter rather than by an empty phase', async () => {
      const user = (await import('@testing-library/user-event')).default.setup();
      setupFilterHandlers();
      renderAsGM();

      // Rue has no submissions, so filtering to Rue empties the Submissions tab.
      expect(await screen.findByText('Vera picks the lock.')).toBeInTheDocument();
      await user.click(screen.getByRole('button', { name: 'Rue' }));

      expect(screen.getByText(/no action submissions match the selected characters/i))
        .toBeInTheDocument();
    });

    it('applies a filter supplied in the URL', async () => {
      setupFilterHandlers();

      renderWithProviders(
        <HistoryView gameId={mockGameId} currentPhaseId={mockCurrentPhaseId} isGM={true} />,
        { initialRoute: `/?phase=2&subTab=submissions&characters=${CASS}`, gameId: mockGameId }
      );

      expect(await screen.findByText('Cass keeps watch.')).toBeInTheDocument();
      expect(screen.queryByText('Vera picks the lock.')).not.toBeInTheDocument();
    });

    // The param is user-editable, so a malformed value must not wipe the view.
    it('ignores unparseable ids in the URL param', async () => {
      setupFilterHandlers();

      renderWithProviders(
        <HistoryView gameId={mockGameId} currentPhaseId={mockCurrentPhaseId} isGM={true} />,
        { initialRoute: `/?phase=2&subTab=submissions&characters=abc,${VERA}`, gameId: mockGameId }
      );

      // "abc" is dropped; the valid id still filters.
      expect(await screen.findByText('Vera picks the lock.')).toBeInTheDocument();
      expect(screen.queryByText('Cass keeps watch.')).not.toBeInTheDocument();
    });

    it('drops the filter when leaving the phase drill-down', async () => {
      const user = (await import('@testing-library/user-event')).default.setup();
      setupFilterHandlers();
      renderAsGM();

      expect(await screen.findByText('Vera picks the lock.')).toBeInTheDocument();
      await user.click(screen.getByRole('button', { name: 'Vera' }));
      expect(screen.queryByText('Cass keeps watch.')).not.toBeInTheDocument();

      await user.click(screen.getByRole('button', { name: /back to history/i }));

      // Re-entering the same phase shows everything again: a character id is only
      // meaningful within the phase it was picked in.
      await user.click((await screen.findAllByRole('heading', { name: /action phase/i }))[0]);

      expect(await screen.findByText('Vera picks the lock.')).toBeInTheDocument();
      expect(screen.getByText('Cass keeps watch.')).toBeInTheDocument();
    });

    it('shows no chip row when the phase has nothing in it', async () => {
      server.use(
        http.get('/api/v1/games/:gameId/actions', () => HttpResponse.json([])),
        http.get('/api/v1/games/:gameId/results', () => HttpResponse.json([]))
      );

      renderAsGM();

      expect(await screen.findByText(/no action submissions for this phase/i)).toBeInTheDocument();
      expect(screen.queryByText(/filter by character/i)).not.toBeInTheDocument();
    });
  });
});
