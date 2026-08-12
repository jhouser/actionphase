import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server';
import { renderWithProviders, createTestQueryClient } from '../../test-utils';
import { ActionsList } from '../ActionsList';
import type { GamePhase, ActionWithDetails } from '../../types/phases';
import { useCharacterSheetItems } from '../../hooks/useCharacterSheetItems';
import { useGameActionResults } from '../../hooks/useActionResults';

/**
 * Stands in for the results list rendered next to ActionsList in the GM view,
 * so tests can observe whether publishing invalidates the results cache.
 */
function ResultsQueryProbe({ gameId }: { gameId: number }) {
  useGameActionResults(gameId);
  return null;
}

vi.mock('../../hooks/useCharacterSheetItems', () => ({
  useCharacterSheetItems: vi.fn(() => []),
}));

// Mock CreateActionResultForm component
vi.mock('../CreateActionResultForm', () => ({
  CreateActionResultForm: ({ gameId, userId, userName, onSuccess }: unknown) => (
    <div data-testid="create-action-result-form">
      <div>Create Action Result Form</div>
      <div>Game ID: {gameId}</div>
      <div>User ID: {userId}</div>
      <div>User Name: {userName}</div>
      <button onClick={onSuccess}>Mock Submit</button>
    </div>
  ),
}));

describe('ActionsList', () => {
  const mockActionPhase1: GamePhase = {
    id: 1,
    game_id: 1,
    phase_type: 'action',
    phase_number: 1,
    title: 'First Action Phase',
    description: 'Submit your first actions',
    start_time: '2025-01-01T00:00:00Z',
    deadline: '2025-12-31T23:59:59Z',
    is_active: true,
    is_published: false,
    created_at: '2025-01-01T00:00:00Z',
  };

  const mockActionPhase2: GamePhase = {
    id: 2,
    game_id: 1,
    phase_type: 'action',
    phase_number: 2,
    title: 'Second Action Phase',
    description: 'Submit your second actions',
    start_time: '2025-01-02T00:00:00Z',
    deadline: '2025-12-31T23:59:59Z',
    is_active: true,
    is_published: false,
    created_at: '2025-01-02T00:00:00Z',
  };

  const mockCommonRoomPhase: GamePhase = {
    id: 3,
    game_id: 1,
    phase_type: 'common_room',
    phase_number: 3,
    title: 'Common Room Discussion',
    start_time: '2025-01-03T00:00:00Z',
    is_active: true,
    is_published: false,
    created_at: '2025-01-03T00:00:00Z',
  };

  const mockActions: ActionWithDetails[] = [
    {
      id: 1,
      game_id: 1,
      user_id: 100,
      phase_id: 1,
      character_id: 1,
      character_name: 'Hero Character',
      username: 'player1',
      content: 'I investigate the mysterious door.',
      submitted_at: '2025-01-10T12:00:00Z',
      updated_at: '2025-01-10T12:00:00Z',
      phase_type: 'action',
      phase_number: 1,
    },
    {
      id: 2,
      game_id: 1,
      user_id: 101,
      phase_id: 1,
      character_id: 2,
      character_name: 'Villain Character',
      username: 'player2',
      content: 'I search the ancient library for clues.',
      submitted_at: '2025-01-10T13:00:00Z',
      updated_at: '2025-01-10T13:00:00Z',
      phase_type: 'action',
      phase_number: 1,
    },
    {
      id: 3,
      game_id: 1,
      user_id: 102,
      phase_id: 2,
      character_id: 3,
      character_name: 'Rogue Character',
      username: 'player3',
      content: 'I sneak past the guards.',
      submitted_at: '2025-01-11T14:00:00Z',
      updated_at: '2025-01-11T14:00:00Z',
      phase_type: 'action',
      phase_number: 2,
    },
  ];

  const setupDefaultHandlers = (
    actions: ActionWithDetails[] = mockActions,
    phases: GamePhase[] = [mockActionPhase1, mockActionPhase2, mockCommonRoomPhase],
    unpublishedCount: number = 0
  ) => {
    server.use(
      http.get('/api/v1/games/:gameId/actions', () => {
        return HttpResponse.json(actions);
      }),
      http.get('/api/v1/games/:gameId/phases', () => {
        return HttpResponse.json(phases);
      }),
      http.get('/api/v1/games/:gameId/phases/:phaseId/results/unpublished-count', () => {
        return HttpResponse.json({ count: unpublishedCount });
      }),
      http.post('/api/v1/games/:gameId/phases/:phaseId/results/publish', () => {
        return HttpResponse.json({}, { status: 200 });
      })
    );
  };

  beforeEach(() => {
    server.resetHandlers();
    vi.clearAllMocks();
  });

  describe('Loading State', () => {
    it('displays loading skeleton while fetching data', async () => {
      server.use(
        http.get('/api/v1/games/:gameId/actions', async () => {
          await new Promise((resolve) => setTimeout(resolve, 100));
          return HttpResponse.json(mockActions);
        }),
        http.get('/api/v1/games/:gameId/phases', async () => {
          await new Promise((resolve) => setTimeout(resolve, 100));
          return HttpResponse.json([mockActionPhase1]);
        })
      );

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      const skeleton = document.querySelector('.animate-pulse');
      expect(skeleton).toBeInTheDocument();

      await waitFor(() => {
        expect(screen.getByText('Submitted Actions')).toBeInTheDocument();
      });
    });

    it('shows multiple skeleton items', async () => {
      server.use(
        http.get('/api/v1/games/:gameId/actions', async () => {
          await new Promise((resolve) => setTimeout(resolve, 100));
          return HttpResponse.json(mockActions);
        }),
        http.get('/api/v1/games/:gameId/phases', async () => {
          await new Promise((resolve) => setTimeout(resolve, 100));
          return HttpResponse.json([mockActionPhase1]);
        })
      );

      const { container } = renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      const skeleton = container.querySelector('.animate-pulse');
      expect(skeleton).toBeInTheDocument();

      await waitFor(() => {
        expect(screen.getByText('Submitted Actions')).toBeInTheDocument();
      });
    });
  });

  describe('Empty States', () => {
    it('returns null when there are no action phases', async () => {
      setupDefaultHandlers([], [mockCommonRoomPhase]);

      const { container } = renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      await waitFor(() => {
        expect(container.firstChild).toBeNull();
      });
    });

    it('does not render component when only common room phases exist', async () => {
      setupDefaultHandlers(mockActions, [mockCommonRoomPhase]);

      const { container } = renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      await waitFor(() => {
        expect(container.firstChild).toBeNull();
      });
    });

    it('shows empty state when no actions exist', async () => {
      setupDefaultHandlers([]);

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      await waitFor(() => {
        expect(screen.getByText('No actions submitted yet')).toBeInTheDocument();
      });
    });

    it('shows empty state message for no actions', async () => {
      setupDefaultHandlers([]);

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      await waitFor(() => {
        expect(
          screen.getByText('Actions will appear here once players submit them')
        ).toBeInTheDocument();
      });
    });

    it('shows phase-specific empty state when filtering', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers([mockActions[0]]);

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      await waitFor(() => {
        expect(screen.getByText('Submitted Actions')).toBeInTheDocument();
      });

      const select = screen.getByRole('combobox');
      await user.selectOptions(select, '2');

      await waitFor(() => {
        expect(screen.getByText('No actions for this phase')).toBeInTheDocument();
      });
    });
  });

  describe('Basic Rendering', () => {
    it('renders heading', async () => {
      setupDefaultHandlers();

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      await waitFor(() => {
        expect(screen.getByText('Submitted Actions')).toBeInTheDocument();
      });
    });

    it('renders description', async () => {
      setupDefaultHandlers();

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      await waitFor(() => {
        expect(
          screen.getByText('View and manage player action submissions')
        ).toBeInTheDocument();
      });
    });

    it('displays total action count', async () => {
      setupDefaultHandlers();

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      await waitFor(() => {
        expect(screen.getByText('3 Actions')).toBeInTheDocument();
      });
    });

    it('displays singular action count when one action', async () => {
      setupDefaultHandlers([mockActions[0]]);

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      await waitFor(() => {
        expect(screen.getByText('1 Action')).toBeInTheDocument();
      });
    });

    it('lists all actions when no filter selected', async () => {
      setupDefaultHandlers();

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      await waitFor(() => {
        // Using getAllByText because dual-DOM renders username in both mobile and desktop views
        expect(screen.getAllByText('Hero Character')[0]).toBeInTheDocument();
        expect(screen.getAllByText('Villain Character')[0]).toBeInTheDocument();
        expect(screen.getAllByText('Rogue Character')[0]).toBeInTheDocument();
      });
    });

    it('applies custom className', () => {
      setupDefaultHandlers();

      const { container } = renderWithProviders(
        <ActionsList gameId={1} className="custom-test-class" />,
        { gameId: 1 }
      );

      expect(container.querySelector('.custom-test-class')).toBeInTheDocument();
    });
  });

  describe('Phase name display', () => {
    it('shows phase title in filter dropdown when phase has a title', async () => {
      setupDefaultHandlers();

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      await waitFor(() => {
        const select = screen.getByRole('combobox');
        expect(select).toHaveTextContent('First Action Phase');
        expect(select).toHaveTextContent('Second Action Phase');
      });
    });

    it('shows phase title in action card phase info', async () => {
      const actionsWithTitle: ActionWithDetails[] = [
        {
          ...mockActions[0],
          phase_title: 'The Great Heist',
        },
      ];
      setupDefaultHandlers(actionsWithTitle);

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      await waitFor(() => {
        expect(screen.getAllByText(/The Great Heist/i)[0]).toBeInTheDocument();
      });
    });

    it('falls back to "Action Phase" in dropdown when phase has no title', async () => {
      const phaseWithoutTitle: GamePhase = { ...mockActionPhase1, title: undefined };
      setupDefaultHandlers(mockActions, [phaseWithoutTitle]);

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      await waitFor(() => {
        const select = screen.getByRole('combobox');
        expect(select).toHaveTextContent('Phase 1 - Action Phase');
      });
    });

    it('falls back to "action phase" in action card when action has no phase_title', async () => {
      setupDefaultHandlers([mockActions[0]]);

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      await waitFor(() => {
        expect(screen.getAllByText(/Phase 1 - action/i)[0]).toBeInTheDocument();
      });
    });
  });

  describe('Phase Filtering', () => {
    it('renders phase filter dropdown', async () => {
      setupDefaultHandlers();

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      await waitFor(() => {
        expect(screen.getByText('Filter by Action Phase')).toBeInTheDocument();
      });
    });

    it('shows only action phases in filter dropdown', async () => {
      setupDefaultHandlers();

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      await waitFor(() => {
        const select = screen.getByRole('combobox');
        expect(select).toHaveTextContent('Phase 1 - First Action Phase');
        expect(select).toHaveTextContent('Phase 2 - Second Action Phase');
        expect(select).not.toHaveTextContent('Common Room Discussion');
      });
    });

    it('shows action count per phase in dropdown', async () => {
      setupDefaultHandlers();

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      await waitFor(() => {
        const select = screen.getByRole('combobox');
        expect(select).toHaveTextContent('Phase 1 - First Action Phase (2)');
        expect(select).toHaveTextContent('Phase 2 - Second Action Phase (1)');
      });
    });

    it('shows zero count for phases with no actions', async () => {
      setupDefaultHandlers([mockActions[0]]);

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      await waitFor(() => {
        const select = screen.getByRole('combobox');
        expect(select).toHaveTextContent('Phase 1 - First Action Phase (1)');
        expect(select).toHaveTextContent('Phase 2 - Second Action Phase (0)');
      });
    });

    it('filters actions by selected phase', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers();

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      await waitFor(() => {
        expect(screen.getAllByText('Hero Character')[0]).toBeInTheDocument();
        expect(screen.getAllByText('Rogue Character')[0]).toBeInTheDocument();
      });

      const select = screen.getByRole('combobox');
      await user.selectOptions(select, '1');

      await waitFor(() => {
        expect(screen.getAllByText('Hero Character')[0]).toBeInTheDocument();
        expect(screen.getAllByText('Villain Character')[0]).toBeInTheDocument();
        expect(screen.queryByText('Rogue Character')).not.toBeInTheDocument();
      });
    });

    it('updates action count when filtering by phase', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers();

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      await waitFor(() => {
        expect(screen.getByText('3 Actions')).toBeInTheDocument();
      });

      const select = screen.getByRole('combobox');
      await user.selectOptions(select, '1');

      await waitFor(() => {
        expect(screen.getByText('2 Actions')).toBeInTheDocument();
      });
    });

    it('shows all actions when "All Action Phases" is selected', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers();

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      await waitFor(() => {
        expect(screen.getAllByText('Hero Character')[0]).toBeInTheDocument();
      });

      const select = screen.getByRole('combobox');
      await user.selectOptions(select, '1');

      await waitFor(() => {
        expect(screen.queryByText('Rogue Character')).not.toBeInTheDocument();
      });

      await user.selectOptions(select, '');

      await waitFor(() => {
        expect(screen.getAllByText('Hero Character')[0]).toBeInTheDocument();
        expect(screen.getAllByText('Rogue Character')[0]).toBeInTheDocument();
      });
    });

    it('uses currentPhase as default filter when it is an action phase', async () => {
      setupDefaultHandlers();

      renderWithProviders(
        <ActionsList gameId={1} currentPhase={mockActionPhase1} />,
        { gameId: 1 }
      );

      await waitFor(() => {
        expect(screen.getAllByText('Hero Character')[0]).toBeInTheDocument();
        expect(screen.getAllByText('Villain Character')[0]).toBeInTheDocument();
        expect(screen.queryByText('Rogue Character')).not.toBeInTheDocument();
      });
    });

    it('filters by currentPhase even when it is a common room phase', async () => {
      // Note: The component uses currentPhase?.id for filtering regardless of phase type
      // This means even common_room phases will filter the actions
      setupDefaultHandlers();

      renderWithProviders(
        <ActionsList gameId={1} currentPhase={mockCommonRoomPhase} />,
        { gameId: 1 }
      );

      await waitFor(() => {
        // Since mockCommonRoomPhase has id=3 and no actions have phase_id=3,
        // we should see the empty state
        expect(screen.getByText('No actions submitted yet')).toBeInTheDocument();
      });
    });
  });

  describe('Action Cards', () => {
    it('displays character name as primary label for each action', async () => {
      setupDefaultHandlers();

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      await waitFor(() => {
        expect(screen.getAllByText('Hero Character')[0]).toBeInTheDocument();
        expect(screen.getAllByText('Villain Character')[0]).toBeInTheDocument();
        expect(screen.getAllByText('Rogue Character')[0]).toBeInTheDocument();
      });
    });

    it('displays character name when present', async () => {
      setupDefaultHandlers();

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      await waitFor(() => {
        expect(screen.getAllByText('Hero Character')[0]).toBeInTheDocument();
        expect(screen.getAllByText('Villain Character')[0]).toBeInTheDocument();
        expect(screen.getAllByText('Rogue Character')[0]).toBeInTheDocument();
      });
    });

    it('displays phase information', async () => {
      setupDefaultHandlers();

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      await waitFor(() => {
        const phaseInfo = screen.getAllByText(/Phase \d+ - action/i);
        expect(phaseInfo.length).toBeGreaterThan(0);
      });
    });

    it('displays submission timestamp', async () => {
      setupDefaultHandlers();

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      await waitFor(() => {
        expect(screen.getAllByText('Hero Character')[0]).toBeInTheDocument();
      });

      const timestamps = screen.getAllByText(/\d{1,2}\/\d{1,2}\/\d{4}/);
      expect(timestamps.length).toBeGreaterThan(0);
    });

    it('expands action card on click', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers();

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      const actionCards = await screen.findAllByText('Hero Character');
      await user.click(actionCards[0]);

      await waitFor(() => {
        expect(
          screen.getByText('I investigate the mysterious door.')
        ).toBeInTheDocument();
      });
    });

    it('collapses action card when clicked again', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers();

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      const actionCards = await screen.findAllByText('Hero Character');
      await user.click(actionCards[0]);

      await waitFor(() => {
        expect(
          screen.getByText('I investigate the mysterious door.')
        ).toBeInTheDocument();
      });

      await user.click(actionCards[0]);

      await waitFor(() => {
        expect(
          screen.queryByText('I investigate the mysterious door.')
        ).not.toBeInTheDocument();
      });
    });

    it('shows action content when expanded', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers();

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      const actionCards = await screen.findAllByText('Hero Character');
      await user.click(actionCards[0]);

      await waitFor(() => {
        expect(
          screen.getByText('I investigate the mysterious door.')
        ).toBeInTheDocument();
      });
    });

    it('shows updated timestamp when expanded', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers();

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      const actionCards = await screen.findAllByText('Hero Character');
      await user.click(actionCards[0]);

      await waitFor(() => {
        expect(screen.getByText(/Last updated:/i)).toBeInTheDocument();
      });
    });

    it('shows "Send Result" button when expanded', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers();

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      const actionCards = await screen.findAllByText('Hero Character');
      await user.click(actionCards[0]);

      await waitFor(() => {
        expect(
          screen.getByRole('button', { name: /Send Result to Hero Character/i })
        ).toBeInTheDocument();
      });
    });

    it('toggles CreateActionResultForm when "Send Result" clicked', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers();

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      const actionCards = await screen.findAllByText('Hero Character');
      await user.click(actionCards[0]);

      const sendResultButton = await screen.findByRole('button', {
        name: /Send Result to Hero Character/i,
      });
      await user.click(sendResultButton);

      await waitFor(() => {
        expect(screen.getByTestId('create-action-result-form')).toBeInTheDocument();
      });
    });

    it('passes correct props to CreateActionResultForm', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers();

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      const actionCards = await screen.findAllByText('Hero Character');
      await user.click(actionCards[0]);

      const sendResultButton = await screen.findByRole('button', {
        name: /Send Result to Hero Character/i,
      });
      await user.click(sendResultButton);

      await waitFor(() => {
        expect(screen.getByText('Game ID: 1')).toBeInTheDocument();
        expect(screen.getByText('User ID: 100')).toBeInTheDocument();
        expect(screen.getByText('User Name: player1')).toBeInTheDocument();
      });
    });

    it('shows cancel button when result form is open', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers();

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      const actionCards = await screen.findAllByText('Hero Character');
      await user.click(actionCards[0]);

      const sendResultButton = await screen.findByRole('button', {
        name: /Send Result to Hero Character/i,
      });
      await user.click(sendResultButton);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /Cancel/i })).toBeInTheDocument();
      });
    });

    it('hides result form when cancel clicked', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers();

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      const actionCards = await screen.findAllByText('Hero Character');
      await user.click(actionCards[0]);

      const sendResultButton = await screen.findByRole('button', {
        name: /Send Result to Hero Character/i,
      });
      await user.click(sendResultButton);

      await waitFor(() => {
        expect(screen.getByTestId('create-action-result-form')).toBeInTheDocument();
      });

      const cancelButton = screen.getByRole('button', { name: /Cancel/i });
      await user.click(cancelButton);

      await waitFor(() => {
        expect(
          screen.queryByTestId('create-action-result-form')
        ).not.toBeInTheDocument();
      });
    });

    it('hides result form on successful submission', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers();

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      const actionCards = await screen.findAllByText('Hero Character');
      await user.click(actionCards[0]);

      const sendResultButton = await screen.findByRole('button', {
        name: /Send Result to Hero Character/i,
      });
      await user.click(sendResultButton);

      const mockSubmitButton = await screen.findByRole('button', {
        name: /Mock Submit/i,
      });
      await user.click(mockSubmitButton);

      await waitFor(() => {
        expect(
          screen.queryByTestId('create-action-result-form')
        ).not.toBeInTheDocument();
      });
    });

    it('only expands one action at a time', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers();

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      const player1Cards = await screen.findAllByText('Hero Character');
      await user.click(player1Cards[0]);

      await waitFor(() => {
        expect(
          screen.getByText('I investigate the mysterious door.')
        ).toBeInTheDocument();
      });

      const player2Cards = await screen.findAllByText('player2');
      await user.click(player2Cards[0]);

      await waitFor(() => {
        expect(
          screen.getByText('I search the ancient library for clues.')
        ).toBeInTheDocument();
      });

      expect(
        screen.queryByText('I investigate the mysterious door.')
      ).not.toBeInTheDocument();
    });
  });

  describe('Publish Results', () => {
    it('hides publish button when no unpublished results', async () => {
      setupDefaultHandlers(mockActions, [mockActionPhase1], 0);

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      await waitFor(() => {
        expect(screen.getByText('Submitted Actions')).toBeInTheDocument();
      });

      expect(
        screen.queryByRole('button', { name: /Publish All Results/i })
      ).not.toBeInTheDocument();
    });

    it('shows publish button when unpublished results exist', async () => {
      setupDefaultHandlers(mockActions, [mockActionPhase1], 3);

      renderWithProviders(
        <ActionsList gameId={1} currentPhase={mockActionPhase1} />,
        { gameId: 1 }
      );

      await waitFor(() => {
        expect(
          screen.getByRole('button', { name: /Publish All Results/i })
        ).toBeInTheDocument();
      });
    });

    it('shows unpublished count', async () => {
      setupDefaultHandlers(mockActions, [mockActionPhase1], 5);

      renderWithProviders(
        <ActionsList gameId={1} currentPhase={mockActionPhase1} />,
        { gameId: 1 }
      );

      await waitFor(() => {
        expect(screen.getByText('5 unpublished results')).toBeInTheDocument();
      });
    });

    it('shows singular text for one unpublished result', async () => {
      setupDefaultHandlers(mockActions, [mockActionPhase1], 1);

      renderWithProviders(
        <ActionsList gameId={1} currentPhase={mockActionPhase1} />,
        { gameId: 1 }
      );

      await waitFor(() => {
        expect(screen.getByText('1 unpublished result')).toBeInTheDocument();
      });
    });

    it('opens confirmation dialog when publish button clicked', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers(mockActions, [mockActionPhase1], 3);

      renderWithProviders(
        <ActionsList gameId={1} currentPhase={mockActionPhase1} />,
        { gameId: 1 }
      );

      const publishButton = await screen.findByRole('button', {
        name: /Publish All Results/i,
      });
      await user.click(publishButton);

      await waitFor(() => {
        expect(screen.getByText('Publish All Results?')).toBeInTheDocument();
      });
    });

    it('shows count in confirmation dialog', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers(mockActions, [mockActionPhase1], 3);

      renderWithProviders(
        <ActionsList gameId={1} currentPhase={mockActionPhase1} />,
        { gameId: 1 }
      );

      const publishButton = await screen.findByRole('button', {
        name: /Publish All Results/i,
      });
      await user.click(publishButton);

      await waitFor(() => {
        expect(
          screen.getByText(/This will publish 3 results/i)
        ).toBeInTheDocument();
      });
    });

    it('shows warning about irreversibility in confirmation dialog', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers(mockActions, [mockActionPhase1], 3);

      renderWithProviders(
        <ActionsList gameId={1} currentPhase={mockActionPhase1} />,
        { gameId: 1 }
      );

      const publishButton = await screen.findByRole('button', {
        name: /Publish All Results/i,
      });
      await user.click(publishButton);

      await waitFor(() => {
        expect(
          screen.getByText(/This action cannot be undone/i)
        ).toBeInTheDocument();
      });
    });

    it('closes dialog when cancel clicked', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers(mockActions, [mockActionPhase1], 3);

      renderWithProviders(
        <ActionsList gameId={1} currentPhase={mockActionPhase1} />,
        { gameId: 1 }
      );

      const publishButton = await screen.findByRole('button', {
        name: /Publish All Results/i,
      });
      await user.click(publishButton);

      await waitFor(() => {
        expect(screen.getByText('Publish All Results?')).toBeInTheDocument();
      });

      const cancelButton = screen.getByRole('button', { name: /^Cancel$/i });
      await user.click(cancelButton);

      await waitFor(() => {
        expect(
          screen.queryByText('Publish All Results?')
        ).not.toBeInTheDocument();
      });
    });

    it('publishes results when confirmed', async () => {
      const user = userEvent.setup();
      let publishCalled = false;

      setupDefaultHandlers(mockActions, [mockActionPhase1], 3);
      server.use(
        http.post('/api/v1/games/:gameId/phases/:phaseId/results/publish', () => {
          publishCalled = true;
          return HttpResponse.json({}, { status: 200 });
        })
      );

      renderWithProviders(
        <ActionsList gameId={1} currentPhase={mockActionPhase1} />,
        { gameId: 1 }
      );

      const publishButton = await screen.findByRole('button', {
        name: /Publish All Results/i,
      });
      await user.click(publishButton);

      const confirmButton = await screen.findByRole('button', {
        name: /Confirm & Publish/i,
      });
      await user.click(confirmButton);

      await waitFor(() => {
        expect(publishCalled).toBe(true);
      });
    });

    it('refetches action results after publishing so drafts stop showing as unpublished', async () => {
      const user = userEvent.setup();
      let gameResultsFetches = 0;

      setupDefaultHandlers(mockActions, [mockActionPhase1], 3);
      server.use(
        http.get('/api/v1/games/:gameId/results', () => {
          gameResultsFetches++;
          return HttpResponse.json([]);
        })
      );

      // Prime the results query so it is cached and observable, mirroring the
      // results list rendered alongside ActionsList in the real GM view.
      const queryClient = createTestQueryClient();
      renderWithProviders(
        <>
          <ActionsList gameId={1} currentPhase={mockActionPhase1} />
          <ResultsQueryProbe gameId={1} />
        </>,
        { gameId: 1, queryClient }
      );

      await waitFor(() => {
        expect(gameResultsFetches).toBe(1);
      });

      const publishButton = await screen.findByRole('button', {
        name: /Publish All Results/i,
      });
      await user.click(publishButton);

      const confirmButton = await screen.findByRole('button', {
        name: /Confirm & Publish/i,
      });
      await user.click(confirmButton);

      await waitFor(() => {
        expect(gameResultsFetches).toBeGreaterThan(1);
      });
    });

    // Minimal ActionResult shape for the results endpoint; only the fields the
    // conflict detector reads (id, character_id, user_id, phase_id, is_published)
    // matter here.
    const baseResult = {
      game_id: 1,
      user_id: 100,
      phase_id: 1,
      gm_user_id: 1,
      content: 'A result',
      sent_at: '2025-01-15T10:00:00Z',
    };

    it('warns in the confirm dialog when one character has updates on several results', async () => {
      // Two unpublished results for character 500, both staging sheet updates:
      // publishing them together applies one snapshot over the other.
      const user = userEvent.setup();
      setupDefaultHandlers(mockActions, [mockActionPhase1], 2);
      server.use(
        http.get('/api/v1/games/:gameId/results', () =>
          HttpResponse.json([
            { ...baseResult, id: 20, character_id: 500, is_published: false },
            { ...baseResult, id: 21, character_id: 500, is_published: false },
          ])
        ),
        http.get('/api/v1/games/:gameId/results/:resultId/character-updates/count', () =>
          HttpResponse.json({ count: 1 })
        )
      );

      renderWithProviders(
        <ActionsList gameId={1} currentPhase={mockActionPhase1} />,
        { gameId: 1 }
      );

      const publishButton = await screen.findByRole('button', {
        name: /Publish All Results/i,
      });
      await user.click(publishButton);

      expect(
        await screen.findByTestId('publish-all-sheet-conflict-warning')
      ).toBeInTheDocument();
    });

    it('warns while the draft counts are still in flight', async () => {
      // Regression: the phase-wide hook used to read a pending count as "no drafts", so
      // a GM who opened Publish All before the counts resolved saw no warning — on the
      // very path where the clobber is guaranteed, since bulk publish applies every
      // conflicting result in one transaction. Pending must count as possibly-staged,
      // matching the per-result hook.
      const user = userEvent.setup();
      setupDefaultHandlers(mockActions, [mockActionPhase1], 2);
      server.use(
        http.get('/api/v1/games/:gameId/results', () =>
          HttpResponse.json([
            { ...baseResult, id: 20, character_id: 500, is_published: false },
            { ...baseResult, id: 21, character_id: 500, is_published: false },
          ])
        ),
        http.get('/api/v1/games/:gameId/results/:resultId/character-updates/count', async () => {
          await new Promise(() => {}); // never resolves
          return HttpResponse.json({ count: 1 });
        })
      );

      renderWithProviders(
        <ActionsList gameId={1} currentPhase={mockActionPhase1} />,
        { gameId: 1 }
      );

      const publishButton = await screen.findByRole('button', {
        name: /Publish All Results/i,
      });
      await user.click(publishButton);

      expect(
        await screen.findByTestId('publish-all-sheet-conflict-warning')
      ).toBeInTheDocument();
    });

    it('shows no conflict warning when each character has updates on one result', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers(mockActions, [mockActionPhase1], 2);
      server.use(
        http.get('/api/v1/games/:gameId/results', () =>
          HttpResponse.json([
            { ...baseResult, id: 20, character_id: 500, is_published: false },
            { ...baseResult, id: 21, character_id: 501, is_published: false },
          ])
        ),
        http.get('/api/v1/games/:gameId/results/:resultId/character-updates/count', () =>
          HttpResponse.json({ count: 1 })
        )
      );

      renderWithProviders(
        <ActionsList gameId={1} currentPhase={mockActionPhase1} />,
        { gameId: 1 }
      );

      const publishButton = await screen.findByRole('button', {
        name: /Publish All Results/i,
      });
      await user.click(publishButton);

      expect(await screen.findByText(/Publish All Results\?/i)).toBeInTheDocument();
      expect(
        screen.queryByTestId('publish-all-sheet-conflict-warning')
      ).not.toBeInTheDocument();
    });

    it('shows loading state while publishing', async () => {
      const user = userEvent.setup();
      let resolvePublish: () => void;
      const publishPromise = new Promise<void>((resolve) => {
        resolvePublish = resolve;
      });

      setupDefaultHandlers(mockActions, [mockActionPhase1], 3);
      server.use(
        http.post('/api/v1/games/:gameId/phases/:phaseId/results/publish', async () => {
          await publishPromise;
          return HttpResponse.json({}, { status: 200 });
        })
      );

      renderWithProviders(
        <ActionsList gameId={1} currentPhase={mockActionPhase1} />,
        { gameId: 1 }
      );

      const publishButton = await screen.findByRole('button', {
        name: /Publish All Results/i,
      });
      await user.click(publishButton);

      const confirmButton = await screen.findByRole('button', {
        name: /Confirm & Publish/i,
      });
      await user.click(confirmButton);

      await waitFor(() => {
        const publishingButtons = screen.getAllByRole('button', { name: /Publishing.../i });
        expect(publishingButtons.length).toBeGreaterThan(0);
      });

      resolvePublish!();
    });

    it('disables buttons while publishing', async () => {
      const user = userEvent.setup();
      let resolvePublish: () => void;
      const publishPromise = new Promise<void>((resolve) => {
        resolvePublish = resolve;
      });

      setupDefaultHandlers(mockActions, [mockActionPhase1], 3);
      server.use(
        http.post('/api/v1/games/:gameId/phases/:phaseId/results/publish', async () => {
          await publishPromise;
          return HttpResponse.json({}, { status: 200 });
        })
      );

      renderWithProviders(
        <ActionsList gameId={1} currentPhase={mockActionPhase1} />,
        { gameId: 1 }
      );

      const publishButton = await screen.findByRole('button', {
        name: /Publish All Results/i,
      });
      await user.click(publishButton);

      const confirmButton = await screen.findByRole('button', {
        name: /Confirm & Publish/i,
      });
      await user.click(confirmButton);

      await waitFor(() => {
        const publishingButtons = screen.getAllByRole('button', {
          name: /Publishing.../i,
        });
        publishingButtons.forEach(button => {
          expect(button).toBeDisabled();
        });
      });

      resolvePublish!();
    });

    it('closes dialog after successful publish', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers(mockActions, [mockActionPhase1], 3);

      renderWithProviders(
        <ActionsList gameId={1} currentPhase={mockActionPhase1} />,
        { gameId: 1 }
      );

      const publishButton = await screen.findByRole('button', {
        name: /Publish All Results/i,
      });
      await user.click(publishButton);

      const confirmButton = await screen.findByRole('button', {
        name: /Confirm & Publish/i,
      });
      await user.click(confirmButton);

      await waitFor(() => {
        expect(
          screen.queryByText('Publish All Results?')
        ).not.toBeInTheDocument();
      });
    });

    it('updates unpublished count for selected phase', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers(mockActions, [mockActionPhase1, mockActionPhase2], 2);

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      await waitFor(() => {
        expect(screen.getByText('Submitted Actions')).toBeInTheDocument();
      });

      const select = screen.getByRole('combobox');
      await user.selectOptions(select, '1');

      await waitFor(() => {
        expect(
          screen.getByRole('button', { name: /Publish All Results/i })
        ).toBeInTheDocument();
      });
    });
  });

  describe('ActionCard sheet item lazy-fetch', () => {
    it('calls useCharacterSheetItems with null when card is collapsed', async () => {
      setupDefaultHandlers([mockActions[0]]);

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      await waitFor(() => {
        expect(screen.getAllByText('Hero Character')[0]).toBeInTheDocument();
      });

      expect(vi.mocked(useCharacterSheetItems)).toHaveBeenCalledWith(null);
    });

    it('calls useCharacterSheetItems with character_id when card is expanded', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers([mockActions[0]]);

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      const cards = await screen.findAllByText('Hero Character');
      await user.click(cards[0]);

      await waitFor(() => {
        expect(screen.getByText('I investigate the mysterious door.')).toBeInTheDocument();
      });

      expect(vi.mocked(useCharacterSheetItems)).toHaveBeenCalledWith(1);
    });

    it('calls useCharacterSheetItems with null for action without character_id', async () => {
      const actionWithoutCharacter: ActionWithDetails = {
        ...mockActions[0],
        character_id: undefined,
      };
      setupDefaultHandlers([actionWithoutCharacter]);

      renderWithProviders(<ActionsList gameId={1} />, { gameId: 1 });

      await waitFor(() => {
        expect(screen.getAllByText('Hero Character')[0]).toBeInTheDocument();
      });

      expect(vi.mocked(useCharacterSheetItems)).toHaveBeenCalledWith(null);
    });
  });
});
