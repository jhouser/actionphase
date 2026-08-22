import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server';
import { renderWithProviders } from '../../test-utils/render';
import { ActionSubmission } from '../ActionSubmission';
import type { GamePhase, ActionWithDetails } from '../../types/phases';
import type { Character } from '../../types/characters';
import { postCachingService } from '../../services/PostCachingService'

// Mock CountdownTimer component
vi.mock('../CountdownTimer', () => ({
  CountdownTimer: ({ deadline }: { deadline: string }) => (
    <div data-testid="countdown-timer">Time remaining for {deadline}</div>
  ),
}));

// Mock useAuth hook
const mockCurrentUser = { id: 100, username: 'testuser', email: 'test@example.com' };
vi.mock('../../contexts/AuthContext', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../contexts/AuthContext')>();
  return {
    ...actual,
    useAuth: () => ({
      currentUser: mockCurrentUser,
      isAuthenticated: true,
      isCheckingAuth: false,
      login: vi.fn(),
      logout: vi.fn(),
      register: vi.fn(),
    }),
  };
});

describe('ActionSubmission', () => {
  const mockActionPhase: GamePhase = {
    id: 1,
    game_id: 1,
    phase_type: 'action',
    phase_number: 2,
    title: 'First Action Phase',
    description: 'Submit your actions',
    start_time: '2025-01-01T00:00:00Z',
    deadline: '2026-12-31T23:59:59Z',
    is_active: true,
    is_published: false,
    created_at: '2025-01-01T00:00:00Z',
  };

  const mockCommonRoomPhase: GamePhase = {
    id: 2,
    game_id: 1,
    phase_type: 'common_room',
    phase_number: 1,
    title: 'Opening Discussion',
    start_time: '2025-01-01T00:00:00Z',
    is_active: true,
    is_published: false,
    created_at: '2025-01-01T00:00:00Z',
  };

  const mockCharacters: Character[] = [
    {
      id: 1,
      game_id: 1,
      name: 'Hero Character',
      character_type: 'player_character',
      user_id: 100,
      status: 'approved',
      created_at: '2025-01-01T00:00:00Z',
      updated_at: '2025-01-01T00:00:00Z',
    },
    {
      id: 2,
      game_id: 1,
      name: 'Villain Character',
      character_type: 'player_character',
      user_id: 100,
      status: 'approved',
      created_at: '2025-01-01T00:00:00Z',
      updated_at: '2025-01-01T00:00:00Z',
    },
  ];

  const mockCurrentAction: ActionWithDetails = {
    id: 1,
    game_id: 1,
    user_id: 100,
    phase_id: 1,
    character_id: 1,
    character_name: 'Hero Character',
    content: 'I investigate the mysterious door.',
    submitted_at: '2025-01-10T12:00:00Z',
    updated_at: '2025-01-10T12:00:00Z',
    phase_type: 'action',
    phase_number: 2,
  };

  const mockPreviousActions: ActionWithDetails[] = [
    {
      id: 2,
      game_id: 1,
      user_id: 100,
      phase_id: 5,
      character_id: 1,
      character_name: 'Hero Character',
      content: 'I searched the room thoroughly.',
      submitted_at: '2025-01-05T12:00:00Z',
      updated_at: '2025-01-05T12:00:00Z',
      phase_type: 'action',
      phase_number: 1,
    },
  ];

  const setupDefaultHandlers = (
    characters: Character[] = mockCharacters,
    userActions: ActionWithDetails[] = []
  ) => {
    server.use(
      http.get('/api/v1/games/:gameId/characters/controllable', () => {
        return HttpResponse.json(characters);
      }),
      http.get('/api/v1/games/:gameId/actions/mine', () => {
        return HttpResponse.json(userActions);
      }),
      http.post('/api/v1/games/:gameId/actions', async () => {
        return HttpResponse.json(
          {
            id: 10,
            game_id: 1,
            user_id: 100,
            phase_id: 1,
            content: 'Test action',
            submitted_at: new Date().toISOString(),
            updated_at: new Date().toISOString(),
          },
          { status: 201 }
        );
      })
    );
  };

  beforeEach(() => {
    server.resetHandlers();
    vi.clearAllMocks();
    localStorage.clear();
  });

  describe('Non-Action Phase', () => {
    it('shows message when phase is not an action phase', async () => {
      setupDefaultHandlers();

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockCommonRoomPhase} />
      , { gameId: 1 });

      await waitFor(() => {
        expect(screen.getByText(/no action phase active/i)).toBeInTheDocument();
      });
    });

    it('shows explanatory text for non-action phase', async () => {
      setupDefaultHandlers();

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockCommonRoomPhase} />
      , { gameId: 1 });

      await waitFor(() => {
        expect(
          screen.getByText(/action submissions are only available during action phases/i)
        ).toBeInTheDocument();
      });
    });

    it('does not show submission form for non-action phase', async () => {
      setupDefaultHandlers();

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockCommonRoomPhase} />
      , { gameId: 1 });

      await waitFor(() => {
        expect(screen.getByText(/no action phase active/i)).toBeInTheDocument();
      });

      expect(screen.queryByLabelText(/your action/i)).not.toBeInTheDocument();
    });
  });

  describe('Rendering', () => {
    it('renders action submission heading', async () => {
      setupDefaultHandlers();

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      await waitFor(() => {
        expect(screen.getByText('Action Submission')).toBeInTheDocument();
      });
    });

    it('renders countdown timer when deadline exists', async () => {
      setupDefaultHandlers();

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      await waitFor(() => {
        expect(screen.getByTestId('countdown-timer')).toBeInTheDocument();
      });
    });

    it('does not render countdown timer when no deadline', async () => {
      setupDefaultHandlers();
      const phaseWithoutDeadline = { ...mockActionPhase, deadline: undefined };

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={phaseWithoutDeadline} />
      , { gameId: 1 });

      await waitFor(() => {
        expect(screen.getByText('Action Submission')).toBeInTheDocument();
      });

      expect(screen.queryByTestId('countdown-timer')).not.toBeInTheDocument();
    });

    it('applies custom className', async () => {
      setupDefaultHandlers();

      const { container } = renderWithProviders(
        <ActionSubmission
          gameId={1}
          currentPhase={mockActionPhase}
          className="custom-class"
        />
      , { gameId: 1 });

      await waitFor(() => {
        expect(screen.getByText('Action Submission')).toBeInTheDocument();
      });

      expect(container.querySelector('.custom-class')).toBeInTheDocument();
    });
  });

  describe('Character Selection', () => {
    it('auto-selects single character', async () => {
      setupDefaultHandlers([mockCharacters[0]]);

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      await waitFor(() => {
        expect(screen.getByText(/acting as:/i)).toBeInTheDocument();
        expect(screen.getByText('Hero Character')).toBeInTheDocument();
      });
    });

    it('shows character dropdown when multiple characters', async () => {
      setupDefaultHandlers();

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      await waitFor(() => {
        expect(screen.getByText(/acting as character/i)).toBeInTheDocument();
      });

      // Verify the select is present
      const selects = screen.getAllByRole('combobox');
      expect(selects.length).toBeGreaterThan(0);
    });

    it('displays all available characters in dropdown', async () => {
      setupDefaultHandlers();

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      await waitFor(() => {
        expect(screen.getByText(/acting as character/i)).toBeInTheDocument();
      });

      const select = screen.getByRole('combobox');
      expect(select).toHaveTextContent('Hero Character');
      expect(select).toHaveTextContent('Villain Character');
    });

    it('allows changing selected character', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers();

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      await waitFor(() => {
        expect(screen.getByText(/acting as character/i)).toBeInTheDocument();
      });

      const select = screen.getByRole('combobox') as HTMLSelectElement;

      await user.selectOptions(select, '2');

      expect(select.value).toBe('2');
    });

    it('allows deselecting character (optional)', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers();

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      await waitFor(() => {
        expect(screen.getByText(/acting as character/i)).toBeInTheDocument();
      });

      const select = screen.getByRole('combobox') as HTMLSelectElement;

      await user.selectOptions(select, '');

      expect(select.value).toBe('');
    });
  });

  describe('Action Form', () => {
    it('renders action textarea', async () => {
      setupDefaultHandlers();

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      await waitFor(() => {
        expect(screen.getByRole('textbox')).toBeInTheDocument();
      });
    });

    it('shows privacy notice', async () => {
      setupDefaultHandlers();

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      await waitFor(() => {
        expect(
          screen.getByText(/this action is private and will only be visible to the gm/i)
        ).toBeInTheDocument();
      });
    });

    it('updates content when user types', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers();

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      const textarea = await screen.findByRole('textbox');
      await user.type(textarea, 'I open the mysterious door');

      expect(textarea).toHaveValue('I open the mysterious door');
    });

    it('shows submit button', async () => {
      setupDefaultHandlers();

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      await waitFor(() => {
        expect(
          screen.getByRole('button', { name: /submit action/i })
        ).toBeInTheDocument();
      });
    });

    it('disables submit button when content is empty', async () => {
      setupDefaultHandlers();

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      await waitFor(() => {
        const submitButton = screen.getByRole('button', {
          name: /submit action/i,
        });
        expect(submitButton).toBeDisabled();
      });
    });

    it('enables submit button when content is provided', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers();

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      const textarea = await screen.findByRole('textbox');
      await user.type(textarea, 'Test action');

      const submitButton = screen.getByRole('button', {
        name: /submit action/i,
      });
      expect(submitButton).not.toBeDisabled();
    });

    it('saves post to localstorage cache with the proper tag', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers();

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      const autosaveId = postCachingService.createAutosaveId('action', mockActionPhase.id);
      const textarea = await screen.findByRole('textbox');
      await user.type(textarea, 'I open the mysterious door');

      expect(localStorage.getItem(autosaveId)).toBeDefined();
      expect(localStorage.getItem(autosaveId)).toContain('I open the mysterious door');
    });
  });

  describe('Action Submission', () => {
    it('submits action with content', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers();

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      const textarea = await screen.findByRole('textbox');
      await user.type(textarea, 'I investigate the room');

      const submitButton = screen.getByRole('button', {
        name: /submit action/i,
      });
      await user.click(submitButton);

      await waitFor(() => {
        expect(textarea).toHaveValue('');
      });
    });

    it('trims whitespace from content before submitting', async () => {
      const user = userEvent.setup();
      let submittedData: unknown = null;

      setupDefaultHandlers();
      server.use(
        http.post('/api/v1/games/:gameId/actions', async ({ request }) => {
          submittedData = await request.json();
          return HttpResponse.json({ id: 1 }, { status: 201 });
        })
      );

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      const textarea = await screen.findByRole('textbox');
      await user.type(textarea, '  Test action  ');

      const submitButton = screen.getByRole('button', {
        name: /submit action/i,
      });
      await user.click(submitButton);

      await waitFor(() => {
        expect(submittedData?.content).toBe('Test action');
      });
    });

    it('submits with selected character', async () => {
      const user = userEvent.setup();
      let submittedData: unknown = null;

      setupDefaultHandlers();
      server.use(
        http.post('/api/v1/games/:gameId/actions', async ({ request }) => {
          submittedData = await request.json();
          return HttpResponse.json({ id: 1 }, { status: 201 });
        })
      );

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      const select = await screen.findByRole('combobox');
      await user.selectOptions(select, '2');

      const textarea = screen.getByRole('textbox');
      await user.type(textarea, 'Test action');

      const submitButton = screen.getByRole('button', {
        name: /submit action/i,
      });
      await user.click(submitButton);

      await waitFor(() => {
        expect(submittedData?.character_id).toBe(2);
      });
    });

    it('clears form after successful submission', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers();

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      const textarea = await screen.findByRole('textbox');
      await user.type(textarea, 'Test action');

      const submitButton = screen.getByRole('button', {
        name: /submit action/i,
      });
      await user.click(submitButton);

      await waitFor(() => {
        expect(textarea).toHaveValue('');
      });
    });

    it('shows loading state while submitting', async () => {
      const user = userEvent.setup();

      setupDefaultHandlers();
      server.use(
        http.post('/api/v1/games/:gameId/actions', async () => {
          await new Promise((resolve) => setTimeout(resolve, 100));
          return HttpResponse.json({ id: 1 }, { status: 201 });
        })
      );

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      const textarea = await screen.findByRole('textbox');
      await user.type(textarea, 'Test action');

      const submitButton = screen.getByRole('button', {
        name: /submit action/i,
      });
      await user.click(submitButton);

      expect(
        await screen.findByRole('button', { name: /submitting/i })
      ).toBeInTheDocument();
    });

    it('disables form fields while submitting', async () => {
      const user = userEvent.setup();

      setupDefaultHandlers();
      server.use(
        http.post('/api/v1/games/:gameId/actions', async () => {
          await new Promise((resolve) => setTimeout(resolve, 100));
          return HttpResponse.json({ id: 1 }, { status: 201 });
        })
      );

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      const textarea = await screen.findByRole('textbox');
      await user.type(textarea, 'Test action');

      const submitButton = screen.getByRole('button', {
        name: /submit action/i,
      });
      await user.click(submitButton);

      await waitFor(() => {
        expect(textarea).toBeDisabled();
      });
    });

    it('clears localstorage cache after submission', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers();

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      const autosaveId = postCachingService.createAutosaveId('action', mockActionPhase.id);
      const textarea = await screen.findByRole('textbox');
      await user.type(textarea, 'I open the mysterious door');

      expect(localStorage.getItem(autosaveId)).toBeDefined();

      const submitButton = screen.getByRole('button', {
        name: /submit action/i,
      });
      await user.click(submitButton);

      expect(localStorage.getItem(autosaveId)).toBeNull();
    });
  });

  describe('Edit Existing Action', () => {
    it('displays current action when exists', async () => {
      setupDefaultHandlers(mockCharacters, [mockCurrentAction]);

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      await waitFor(() => {
        expect(screen.getByText('Your Current Action')).toBeInTheDocument();
        expect(
          screen.getByText('I investigate the mysterious door.')
        ).toBeInTheDocument();
      });
    });

    it('shows character name for current action', async () => {
      setupDefaultHandlers(mockCharacters, [mockCurrentAction]);

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      await waitFor(() => {
        expect(screen.getByText(/acting as:/i)).toBeInTheDocument();
        expect(screen.getByText('Hero Character')).toBeInTheDocument();
      });
    });

    it('shows last updated timestamp', async () => {
      setupDefaultHandlers(mockCharacters, [mockCurrentAction]);

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      await waitFor(() => {
        expect(screen.getByText(/last updated:/i)).toBeInTheDocument();
      });
    });

    it('shows edit button for current action', async () => {
      setupDefaultHandlers(mockCharacters, [mockCurrentAction]);

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      await waitFor(() => {
        expect(
          screen.getByRole('button', { name: /^edit$/i })
        ).toBeInTheDocument();
      });
    });

    it('opens edit form when edit button clicked', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers(mockCharacters, [mockCurrentAction]);

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      const editButton = await screen.findByRole('button', { name: /^edit$/i });
      await user.click(editButton);

      await waitFor(() => {
        expect(screen.getByRole('textbox')).toHaveValue(
          'I investigate the mysterious door.'
        );
      });
    });

    it('pre-populates form with current action content', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers(mockCharacters, [mockCurrentAction]);

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      const editButton = await screen.findByRole('button', { name: /^edit$/i });
      await user.click(editButton);

      const textarea = await screen.findByRole('textbox');
      expect(textarea).toHaveValue('I investigate the mysterious door.');
    });

    it('shows update button when editing', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers(mockCharacters, [mockCurrentAction]);

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      const editButton = await screen.findByRole('button', { name: /^edit$/i });
      await user.click(editButton);

      await waitFor(() => {
        expect(
          screen.getByRole('button', { name: /update action/i })
        ).toBeInTheDocument();
      });
    });

    it('shows cancel button when editing', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers(mockCharacters, [mockCurrentAction]);

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      const editButton = await screen.findByRole('button', { name: /^edit$/i });
      await user.click(editButton);

      await waitFor(() => {
        expect(
          screen.getByRole('button', { name: /cancel/i })
        ).toBeInTheDocument();
      });
    });

    it('restores original content when cancel clicked', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers(mockCharacters, [mockCurrentAction]);

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      const editButton = await screen.findByRole('button', { name: /^edit$/i });
      await user.click(editButton);

      const textarea = await screen.findByRole('textbox');
      await user.clear(textarea);
      await user.type(textarea, 'Different action');

      const cancelButton = screen.getByRole('button', { name: /cancel/i });
      await user.click(cancelButton);

      await waitFor(() => {
        expect(screen.getByText('Your Current Action')).toBeInTheDocument();
        expect(
          screen.getByText('I investigate the mysterious door.')
        ).toBeInTheDocument();
      });
    });

    it('hides current action display when editing', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers(mockCharacters, [mockCurrentAction]);

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      const editButton = await screen.findByRole('button', { name: /^edit$/i });
      await user.click(editButton);

      await waitFor(() => {
        expect(
          screen.queryByText('Your Current Action')
        ).not.toBeInTheDocument();
      });
    });

    it('updates action successfully', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers(mockCharacters, [mockCurrentAction]);

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      const editButton = await screen.findByRole('button', { name: /^edit$/i });
      await user.click(editButton);

      const textarea = await screen.findByRole('textbox');
      await user.clear(textarea);
      await user.type(textarea, 'Updated action');

      const updateButton = screen.getByRole('button', {
        name: /update action/i,
      });
      await user.click(updateButton);

      // After successful update, edit form should collapse and current action should be visible
      await waitFor(() => {
        expect(screen.getByText('Your Current Action')).toBeInTheDocument();
      });
    });
  });

  describe('Deadline Warnings', () => {
    it('shows warning when phase is not active', async () => {
      setupDefaultHandlers();
      const inactivePhase = { ...mockActionPhase, is_active: false };

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={inactivePhase} />
      , { gameId: 1 });

      await waitFor(() => {
        expect(
          screen.getByText(/action submission is not currently available/i)
        ).toBeInTheDocument();
      });
    });

    it('shows warning when deadline has passed', async () => {
      setupDefaultHandlers();
      const expiredPhase = {
        ...mockActionPhase,
        deadline: '2020-01-01T00:00:00Z',
      };

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={expiredPhase} />
      , { gameId: 1 });

      await waitFor(() => {
        expect(
          screen.getByText(/the action submission deadline has passed/i)
        ).toBeInTheDocument();
      });
    });

    it('hides edit button when phase is not active', async () => {
      setupDefaultHandlers(mockCharacters, [mockCurrentAction]);
      const inactivePhase = { ...mockActionPhase, is_active: false };

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={inactivePhase} />
      , { gameId: 1 });

      await waitFor(() => {
        expect(screen.getByText('Your Current Action')).toBeInTheDocument();
      });

      expect(screen.queryByRole('button', { name: /^edit$/i })).not.toBeInTheDocument();
    });

    it('hides edit button when deadline has passed', async () => {
      setupDefaultHandlers(mockCharacters, [mockCurrentAction]);
      const expiredPhase = {
        ...mockActionPhase,
        deadline: '2020-01-01T00:00:00Z',
      };

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={expiredPhase} />
      , { gameId: 1 });

      await waitFor(() => {
        expect(screen.getByText('Your Current Action')).toBeInTheDocument();
      });

      expect(screen.queryByRole('button', { name: /^edit$/i })).not.toBeInTheDocument();
    });
  });

  describe('Previous Actions', () => {
    it('shows previous actions section when actions exist', async () => {
      setupDefaultHandlers(mockCharacters, [
        mockCurrentAction,
        ...mockPreviousActions,
      ]);

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      await waitFor(() => {
        expect(
          screen.getByText(/your previous actions/i)
        ).toBeInTheDocument();
      });
    });

    it('shows count of previous actions', async () => {
      setupDefaultHandlers(mockCharacters, [
        mockCurrentAction,
        ...mockPreviousActions,
      ]);

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      await waitFor(() => {
        expect(screen.getByText(/your previous actions \(1\)/i)).toBeInTheDocument();
      });
    });

    it('expands previous actions when clicked', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers(mockCharacters, [
        mockCurrentAction,
        ...mockPreviousActions,
      ]);

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      const toggleButton = await screen.findByText(/your previous actions/i);
      await user.click(toggleButton);

      await waitFor(() => {
        expect(
          screen.getByText('I searched the room thoroughly.')
        ).toBeInTheDocument();
      });
    });

    it('collapses previous actions when clicked again', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers(mockCharacters, [
        mockCurrentAction,
        ...mockPreviousActions,
      ]);

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      const toggleButton = await screen.findByText(/your previous actions/i);
      await user.click(toggleButton);

      await waitFor(() => {
        expect(
          screen.getByText('I searched the room thoroughly.')
        ).toBeInTheDocument();
      });

      await user.click(toggleButton);

      await waitFor(() => {
        expect(
          screen.queryByText('I searched the room thoroughly.')
        ).not.toBeInTheDocument();
      });
    });

    it('displays phase number in previous actions', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers(mockCharacters, [
        mockCurrentAction,
        ...mockPreviousActions,
      ]);

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      const toggleButton = await screen.findByText(/your previous actions/i);
      await user.click(toggleButton);

      await waitFor(() => {
        expect(screen.getByText(/phase 1/i)).toBeInTheDocument();
      });
    });

    it('displays character name in previous actions', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers(mockCharacters, [
        mockCurrentAction,
        ...mockPreviousActions,
      ]);

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      const toggleButton = await screen.findByText(/your previous actions/i);
      await user.click(toggleButton);

      await waitFor(() => {
        expect(screen.getByText(/as hero character/i)).toBeInTheDocument();
      });
    });

    it('does not show previous actions section when no previous actions', async () => {
      setupDefaultHandlers(mockCharacters, [mockCurrentAction]);

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      await waitFor(() => {
        expect(screen.getByText('Your Current Action')).toBeInTheDocument();
      });

      expect(
        screen.queryByText(/your previous actions/i)
      ).not.toBeInTheDocument();
    });
  });

  describe('Error Handling', () => {
    it('displays error message when submission fails', async () => {
      const user = userEvent.setup();

      setupDefaultHandlers();
      server.use(
        http.post('/api/v1/games/:gameId/actions', () => {
          return HttpResponse.json(
            { message: 'Failed to submit action' },
            { status: 500 }
          );
        })
      );

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      const textarea = await screen.findByRole('textbox');
      await user.type(textarea, 'Test action');

      const submitButton = screen.getByRole('button', {
        name: /submit action/i,
      });
      await user.click(submitButton);

      await waitFor(() => {
        expect(
          screen.getByText(/failed to submit action/i)
        ).toBeInTheDocument();
      });
    });

    it('does not clear content when submission fails', async () => {
      const user = userEvent.setup();

      setupDefaultHandlers();
      server.use(
        http.post('/api/v1/games/:gameId/actions', () => {
          return HttpResponse.json({ message: 'Error' }, { status: 500 });
        })
      );

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      const textarea = await screen.findByRole('textbox');
      await user.type(textarea, 'Test action');

      const submitButton = screen.getByRole('button', {
        name: /submit action/i,
      });
      await user.click(submitButton);

      await waitFor(() => {
        expect(
          screen.getByText(/failed to submit action/i)
        ).toBeInTheDocument();
      });

      expect(textarea).toHaveValue('Test action');
    });
  });

  describe('Integration', () => {
    it('handles complete submission workflow', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers();

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      // Select character
      const select = await screen.findByRole('combobox');
      await user.selectOptions(select, '1');

      // Type action
      const textarea = screen.getByRole('textbox');
      await user.type(textarea, 'I investigate the mysterious artifact');

      // Submit
      const submitButton = screen.getByRole('button', {
        name: /submit action/i,
      });
      await user.click(submitButton);

      // Verify submission
      await waitFor(() => {
        expect(textarea).toHaveValue('');
      });
    });

    it('handles complete edit workflow', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers(mockCharacters, [mockCurrentAction]);

      renderWithProviders(
        <ActionSubmission gameId={1} currentPhase={mockActionPhase} />
      , { gameId: 1 });

      // Click edit
      const editButton = await screen.findByRole('button', { name: /^edit$/i });
      await user.click(editButton);

      // Modify content
      const textarea = await screen.findByRole('textbox');
      await user.clear(textarea);
      await user.type(textarea, 'I carefully examine the door for traps');

      // Submit update
      const updateButton = screen.getByRole('button', {
        name: /update action/i,
      });
      await user.click(updateButton);

      // Verify update - edit form should collapse and current action should be visible
      await waitFor(() => {
        expect(screen.getByText('Your Current Action')).toBeInTheDocument();
      });
    });
  });
});
