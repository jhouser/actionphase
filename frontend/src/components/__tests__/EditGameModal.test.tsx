import { describe, it, expect, beforeEach } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server';
import { renderWithProviders } from '../../test-utils/render';
import { EditGameModal } from '../EditGameModal';
import type { GameWithDetails } from '../../types/games';

// Mock ResizeObserver for react-datepicker
global.ResizeObserver = vi.fn().mockImplementation(() => ({
  observe: vi.fn(),
  unobserve: vi.fn(),
  disconnect: vi.fn(),
}));

describe('EditGameModal', () => {
  const mockOnClose = vi.fn();
  const mockOnGameUpdated = vi.fn();

  const mockGame: GameWithDetails = {
    id: 1,
    title: 'Test Game',
    description: 'A test game description',
    gm_user_id: 1,
    gm_username: 'testgm',
    state: 'setup',
    genre: 'Fantasy',
    max_players: 6,
    recruitment_deadline: '2025-12-31T23:59:00Z',
    start_date: '2026-01-01T00:00:00Z',
    end_date: '2026-12-31T23:59:00Z',
    is_anonymous: false,
    current_players: 3,
    created_at: '2025-01-01T00:00:00Z',
    updated_at: '2025-01-01T00:00:00Z',
  };

  const setupDefaultHandlers = () => {
    server.use(
      http.put('/api/v1/games/:id', async ({ request }) => {
        const body = await request.json();
        return HttpResponse.json({
          ...mockGame,
          ...body,
          updated_at: new Date().toISOString(),
        });
      })
    );
  };

  beforeEach(() => {
    server.resetHandlers();
    setupDefaultHandlers();
    mockOnClose.mockClear();
    mockOnGameUpdated.mockClear();
  });

  describe('Rendering', () => {
    it('does not render when isOpen is false', () => {
      renderWithProviders(
        <EditGameModal
          game={mockGame}
          isOpen={false}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      expect(screen.queryByText('Edit Game')).not.toBeInTheDocument();
    });

    it('renders modal with heading when isOpen is true', () => {
      renderWithProviders(
        <EditGameModal
          game={mockGame}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      expect(screen.getByText('Edit Game')).toBeInTheDocument();
    });

    it('displays backdrop and modal container', () => {
      const { container } = renderWithProviders(
        <EditGameModal
          game={mockGame}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      // Backdrop uses new theme tokens
      const backdrop = container.querySelector('.bg-black\\/60');
      expect(backdrop).toBeInTheDocument();

      // Modal container uses semantic surface class
      const modalContainer = container.querySelector('.surface-raised');
      expect(modalContainer).toBeInTheDocument();
    });
  });

  describe('Form Fields', () => {
    it('renders all form fields', () => {
      renderWithProviders(
        <EditGameModal
          game={mockGame}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      expect(screen.getByLabelText(/title/i)).toBeInTheDocument();
      expect(screen.getByLabelText(/description/i)).toBeInTheDocument();
      expect(screen.getByLabelText(/genre/i)).toBeInTheDocument();
      expect(screen.getByLabelText(/maximum players/i)).toBeInTheDocument();
      // DateTimeInput doesn't properly associate labels with inputs, so check for label text
      expect(screen.getByText(/recruitment deadline/i)).toBeInTheDocument();
      expect(screen.getByText(/start date/i)).toBeInTheDocument();
      expect(screen.getByText(/end date/i)).toBeInTheDocument();
      expect(screen.getByLabelText(/anonymous mode/i)).toBeInTheDocument();
      expect(screen.getByLabelText(/auto.*accept.*audience/i)).toBeInTheDocument();
    });

    it('marks required fields with asterisks', () => {
      renderWithProviders(
        <EditGameModal
          game={mockGame}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      // Title and Description should have required attribute (UI components handle visual asterisks)
      expect(screen.getByLabelText(/title/i)).toBeRequired();
      expect(screen.getByLabelText(/description/i)).toBeRequired();
    });

    it('has proper input types for each field', () => {
      renderWithProviders(
        <EditGameModal
          game={mockGame}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      const titleInput = screen.getByLabelText(/title/i);
      expect(titleInput).toHaveAttribute('type', 'text');

      const maxPlayersInput = screen.getByLabelText(/maximum players/i);
      expect(maxPlayersInput).toHaveAttribute('type', 'number');

      // DateTimeInput uses react-datepicker which doesn't have type="datetime-local"
      // Just verify the field exists via placeholder
      const dateInputs = screen.getAllByPlaceholderText(/select date and time/i);
      expect(dateInputs.length).toBeGreaterThan(0);

      const isAnonymousInput = screen.getByLabelText(/anonymous mode/i);
      expect(isAnonymousInput).toHaveAttribute('type', 'checkbox');
    });
  });

  describe('Initial Values', () => {
    it('populates form with game data when modal opens', () => {
      renderWithProviders(
        <EditGameModal
          game={mockGame}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      expect(screen.getByLabelText(/title/i)).toHaveValue('Test Game');
      expect(screen.getByLabelText(/description/i)).toHaveValue('A test game description');
      expect(screen.getByLabelText(/genre/i)).toHaveValue('Fantasy');
      expect(screen.getByLabelText(/maximum players/i)).toHaveValue(6);
    });

    it('handles games with missing optional fields', () => {
      const gameWithMinimalData: GameWithDetails = {
        ...mockGame,
        genre: undefined,
        max_players: undefined,
        recruitment_deadline: undefined,
        start_date: undefined,
        end_date: undefined,
      };

      renderWithProviders(
        <EditGameModal
          game={gameWithMinimalData}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      expect(screen.getByLabelText(/genre/i)).toHaveValue('');
      expect(screen.getByLabelText(/maximum players/i)).toHaveValue(null);
      // DateTimeInput fields don't have accessible labels, just verify they're empty via placeholder
      const dateInputs = screen.getAllByPlaceholderText(/select date and time/i);
      expect(dateInputs[0]).toHaveValue('');
    });

    it('sets anonymous mode checkbox correctly', () => {
      renderWithProviders(
        <EditGameModal
          game={mockGame}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      const anonymousCheckbox = screen.getByLabelText(/anonymous mode/i);
      expect(anonymousCheckbox).not.toBeChecked();
    });

    it('sets auto-accept audience checkbox correctly', () => {
      const gameWithAutoAccept: GameWithDetails = {
        ...mockGame,
        auto_accept_audience: true,
      };

      renderWithProviders(
        <EditGameModal
          game={gameWithAutoAccept}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      const autoAcceptCheckbox = screen.getByLabelText(/auto.*accept.*audience/i);
      expect(autoAcceptCheckbox).toBeChecked();
    });

    it('unchecks auto-accept audience when game has it disabled', () => {
      const gameWithoutAutoAccept: GameWithDetails = {
        ...mockGame,
        auto_accept_audience: false,
      };

      renderWithProviders(
        <EditGameModal
          game={gameWithoutAutoAccept}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      const autoAcceptCheckbox = screen.getByLabelText(/auto.*accept.*audience/i);
      expect(autoAcceptCheckbox).not.toBeChecked();
    });

    it('resets form when modal is reopened', () => {
      const { rerender } = renderWithProviders(
        <EditGameModal
          game={mockGame}
          isOpen={false}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      // Open modal
      rerender(
        <EditGameModal
          game={mockGame}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      expect(screen.getByLabelText(/title/i)).toHaveValue('Test Game');
    });
  });

  describe('Form Interactions', () => {
    it('allows user to type in title field', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <EditGameModal
          game={mockGame}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      const titleInput = screen.getByLabelText(/title/i);
      await user.clear(titleInput);
      await user.type(titleInput, 'Updated Game Title');

      expect(titleInput).toHaveValue('Updated Game Title');
    });

    it('allows user to type in description field', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <EditGameModal
          game={mockGame}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      const descriptionInput = screen.getByLabelText(/description/i);
      await user.clear(descriptionInput);
      await user.type(descriptionInput, 'Updated description');

      expect(descriptionInput).toHaveValue('Updated description');
    });

    it('allows user to change genre', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <EditGameModal
          game={mockGame}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      const genreInput = screen.getByLabelText(/genre/i);
      await user.clear(genreInput);
      await user.type(genreInput, 'Sci-Fi');

      expect(genreInput).toHaveValue('Sci-Fi');
    });

    it('allows user to change max players', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <EditGameModal
          game={mockGame}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      const maxPlayersInput = screen.getByLabelText(/maximum players/i);
      await user.clear(maxPlayersInput);
      await user.type(maxPlayersInput, '8');

      expect(maxPlayersInput).toHaveValue(8);
    });

    it('allows user to change datetime fields', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <EditGameModal
          game={mockGame}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      // DateTimeInput uses react-datepicker, access via placeholder
      const dateInputs = screen.getAllByPlaceholderText(/select date and time/i);
      const startDateInput = dateInputs[1]; // Second one is start date
      await user.clear(startDateInput);
      await user.type(startDateInput, '06/15/2026 02:30 PM');

      expect(startDateInput).toHaveValue('06/15/2026 02:30 PM');
    });

    it('allows user to toggle anonymous mode checkbox', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <EditGameModal
          game={mockGame}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      const anonymousCheckbox = screen.getByLabelText(/anonymous mode/i);
      expect(anonymousCheckbox).not.toBeChecked();

      await user.click(anonymousCheckbox);
      expect(anonymousCheckbox).toBeChecked();

      await user.click(anonymousCheckbox);
      expect(anonymousCheckbox).not.toBeChecked();
    });

    it('allows user to toggle auto-accept audience checkbox', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <EditGameModal
          game={mockGame}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      const autoAcceptCheckbox = screen.getByLabelText(/auto.*accept.*audience/i);
      expect(autoAcceptCheckbox).not.toBeChecked();

      await user.click(autoAcceptCheckbox);
      expect(autoAcceptCheckbox).toBeChecked();

      await user.click(autoAcceptCheckbox);
      expect(autoAcceptCheckbox).not.toBeChecked();
    });

    it('allows clearing max players field', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <EditGameModal
          game={mockGame}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      const maxPlayersInput = screen.getByLabelText(/maximum players/i);
      await user.clear(maxPlayersInput);

      expect(maxPlayersInput).toHaveValue(null);
    });
  });

  describe('Validation', () => {
    it('shows error when title contains only whitespace', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <EditGameModal
          game={mockGame}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      const titleInput = screen.getByLabelText(/title/i);
      await user.clear(titleInput);
      await user.type(titleInput, '   ');

      const saveButton = screen.getByRole('button', { name: /save changes/i });
      await user.click(saveButton);

      await waitFor(() => {
        expect(screen.getAllByText('Game title is required').length).toBeGreaterThan(0);
      });
    });

    it('shows error when description contains only whitespace', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <EditGameModal
          game={mockGame}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      const descriptionInput = screen.getByLabelText(/description/i);
      await user.clear(descriptionInput);
      await user.type(descriptionInput, '   ');

      const saveButton = screen.getByRole('button', { name: /save changes/i });
      await user.click(saveButton);

      await waitFor(() => {
        expect(screen.getAllByText('Game description is required').length).toBeGreaterThan(0);
      });
    });
  });

  describe('Form Submission', () => {
    it('successfully updates game with valid data', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <EditGameModal
          game={mockGame}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      const titleInput = screen.getByLabelText(/title/i);
      await user.clear(titleInput);
      await user.type(titleInput, 'Updated Title');

      const saveButton = screen.getByRole('button', { name: /save changes/i });
      await user.click(saveButton);

      await waitFor(() => {
        expect(mockOnGameUpdated).toHaveBeenCalled();
        expect(mockOnClose).toHaveBeenCalled();
      });
    });

    it('sends correct data to API', async () => {
      const user = userEvent.setup();
      let requestBody: unknown = null;

      server.use(
        http.put('/api/v1/games/:id', async ({ request }) => {
          requestBody = await request.json();
          return HttpResponse.json({ ...mockGame, ...requestBody });
        })
      );

      renderWithProviders(
        <EditGameModal
          game={mockGame}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      const titleInput = screen.getByLabelText(/title/i);
      await user.clear(titleInput);
      await user.type(titleInput, 'New Title');

      const descriptionInput = screen.getByLabelText(/description/i);
      await user.clear(descriptionInput);
      await user.type(descriptionInput, 'New Description');

      const maxPlayersInput = screen.getByLabelText(/maximum players/i);
      await user.clear(maxPlayersInput);
      await user.type(maxPlayersInput, '10');

      const anonymousCheckbox = screen.getByLabelText(/anonymous mode/i);
      await user.click(anonymousCheckbox);

      const autoAcceptCheckbox = screen.getByLabelText(/auto.*accept.*audience/i);
      await user.click(autoAcceptCheckbox);

      const saveButton = screen.getByRole('button', { name: /save changes/i });
      await user.click(saveButton);

      await waitFor(() => {
        expect(requestBody).toEqual({
          title: 'New Title',
          description: 'New Description',
          genre: 'Fantasy',
          max_players: 10,
          recruitment_deadline: expect.any(String),
          start_date: expect.any(String),
          end_date: expect.any(String),
          is_public: true,
          is_anonymous: true,
          auto_accept_audience: true,
          allow_group_conversations: expect.any(Boolean),
          portrait_avatars: expect.any(Boolean),
          common_room_open_day: null,
          common_room_open_time: null,
          common_room_close_day: null,
          common_room_close_time: null,
          schedule_timezone: null,
        });
        // banner_url must NOT be in the payload — the upload/delete mutations manage it
        // independently, and including it here would overwrite a freshly uploaded banner
        expect(requestBody).not.toHaveProperty('banner_url');
      });
    });

    it('trims whitespace from title and description', async () => {
      const user = userEvent.setup();
      let requestBody: unknown = null;

      server.use(
        http.put('/api/v1/games/:id', async ({ request }) => {
          requestBody = await request.json();
          return HttpResponse.json({ ...mockGame, ...requestBody });
        })
      );

      renderWithProviders(
        <EditGameModal
          game={mockGame}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      const titleInput = screen.getByLabelText(/title/i);
      await user.clear(titleInput);
      await user.type(titleInput, '  Trimmed Title  ');

      const descriptionInput = screen.getByLabelText(/description/i);
      await user.clear(descriptionInput);
      await user.type(descriptionInput, '  Trimmed Description  ');

      const saveButton = screen.getByRole('button', { name: /save changes/i });
      await user.click(saveButton);

      await waitFor(() => {
        expect(requestBody.title).toBe('Trimmed Title');
        expect(requestBody.description).toBe('Trimmed Description');
      });
    });

    it('sends undefined for empty optional fields', async () => {
      const user = userEvent.setup();
      let requestBody: unknown = null;

      server.use(
        http.put('/api/v1/games/:id', async ({ request }) => {
          requestBody = await request.json();
          return HttpResponse.json({ ...mockGame, ...requestBody });
        })
      );

      renderWithProviders(
        <EditGameModal
          game={mockGame}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      const genreInput = screen.getByLabelText(/genre/i);
      await user.clear(genreInput);

      const maxPlayersInput = screen.getByLabelText(/maximum players/i);
      await user.clear(maxPlayersInput);

      const saveButton = screen.getByRole('button', { name: /save changes/i });
      await user.click(saveButton);

      await waitFor(() => {
        expect(requestBody.genre).toBeUndefined();
        expect(requestBody.max_players).toBeUndefined();
      });
    });

    it('does not include banner_url in update payload even when game has an existing banner', async () => {
      const user = userEvent.setup();
      let requestBody: unknown = null;

      server.use(
        http.put('/api/v1/games/:id', async ({ request }) => {
          requestBody = await request.json();
          return HttpResponse.json({ ...mockGame, ...requestBody });
        })
      );

      const gameWithBanner: GameWithDetails = {
        ...mockGame,
        banner_url: 'https://example.com/banner.png',
      };

      renderWithProviders(
        <EditGameModal
          game={gameWithBanner}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      const saveButton = screen.getByRole('button', { name: /save changes/i });
      await user.click(saveButton);

      await waitFor(() => {
        // Saving the form must not touch banner_url — a banner uploaded just before
        // clicking Save would be overwritten if this field were included
        expect(requestBody).not.toHaveProperty('banner_url');
      });
    });

    it('handles empty genre by trimming and sending undefined', async () => {
      const user = userEvent.setup();
      let requestBody: unknown = null;

      server.use(
        http.put('/api/v1/games/:id', async ({ request }) => {
          requestBody = await request.json();
          return HttpResponse.json({ ...mockGame, ...requestBody });
        })
      );

      renderWithProviders(
        <EditGameModal
          game={mockGame}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      const genreInput = screen.getByLabelText(/genre/i);
      await user.clear(genreInput);
      await user.type(genreInput, '   ');

      const saveButton = screen.getByRole('button', { name: /save changes/i });
      await user.click(saveButton);

      await waitFor(() => {
        expect(requestBody.genre).toBeUndefined();
      });
    });
  });

  describe('Loading States', () => {
    it('shows loading state while submitting', async () => {
      const user = userEvent.setup();

      server.use(
        http.put('/api/v1/games/:id', async () => {
          await new Promise((resolve) => setTimeout(resolve, 100));
          return HttpResponse.json(mockGame);
        })
      );

      renderWithProviders(
        <EditGameModal
          game={mockGame}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      const saveButton = screen.getByRole('button', { name: /save changes/i });
      await user.click(saveButton);

      // Button component keeps text and adds spinner when loading
      expect(screen.getByText('Save Changes')).toBeInTheDocument();
      expect(saveButton).toBeDisabled();

      await waitFor(() => {
        expect(mockOnGameUpdated).toHaveBeenCalled();
      });
    });

    it('disables submit button while loading', async () => {
      const user = userEvent.setup();

      server.use(
        http.put('/api/v1/games/:id', async () => {
          await new Promise((resolve) => setTimeout(resolve, 100));
          return HttpResponse.json(mockGame);
        })
      );

      renderWithProviders(
        <EditGameModal
          game={mockGame}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      const saveButton = screen.getByRole('button', { name: /save changes/i });
      await user.click(saveButton);

      expect(saveButton).toBeDisabled();

      await waitFor(() => {
        expect(mockOnGameUpdated).toHaveBeenCalled();
      });
    });

    it('disables cancel button while loading', async () => {
      const user = userEvent.setup();

      server.use(
        http.put('/api/v1/games/:id', async () => {
          await new Promise((resolve) => setTimeout(resolve, 100));
          return HttpResponse.json(mockGame);
        })
      );

      renderWithProviders(
        <EditGameModal
          game={mockGame}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      const saveButton = screen.getByRole('button', { name: /save changes/i });
      await user.click(saveButton);

      const cancelButton = screen.getByRole('button', { name: /cancel/i });
      expect(cancelButton).toBeDisabled();

      await waitFor(() => {
        expect(mockOnGameUpdated).toHaveBeenCalled();
      });
    });
  });

  describe('Close Behavior', () => {
    it('calls onClose when cancel button is clicked', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <EditGameModal
          game={mockGame}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      const cancelButton = screen.getByRole('button', { name: /cancel/i });
      await user.click(cancelButton);

      expect(mockOnClose).toHaveBeenCalledTimes(1);
    });

    it('does not submit form when cancel is clicked', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <EditGameModal
          game={mockGame}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      const titleInput = screen.getByLabelText(/title/i);
      await user.clear(titleInput);
      await user.type(titleInput, 'Changed Title');

      const cancelButton = screen.getByRole('button', { name: /cancel/i });
      await user.click(cancelButton);

      expect(mockOnGameUpdated).not.toHaveBeenCalled();
    });

    it('closes modal and clears error after successful update', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <EditGameModal
          game={mockGame}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      const saveButton = screen.getByRole('button', { name: /save changes/i });
      await user.click(saveButton);

      await waitFor(() => {
        expect(mockOnClose).toHaveBeenCalled();
      });
    });
  });

  describe('Form Reset', () => {
    it('resets form to initial values when reopened', () => {
      const { rerender } = renderWithProviders(
        <EditGameModal
          game={mockGame}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      // Close modal
      rerender(
        <EditGameModal
          game={mockGame}
          isOpen={false}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      // Reopen modal
      rerender(
        <EditGameModal
          game={mockGame}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      expect(screen.getByLabelText(/title/i)).toHaveValue('Test Game');
      expect(screen.queryByText(/error/i)).not.toBeInTheDocument();
    });

    it('clears errors when modal is reopened', () => {
      const { rerender } = renderWithProviders(
        <EditGameModal
          game={mockGame}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      // Close and reopen
      rerender(
        <EditGameModal
          game={mockGame}
          isOpen={false}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      rerender(
        <EditGameModal
          game={mockGame}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      expect(screen.queryByText(/error/i)).not.toBeInTheDocument();
    });

    it('updates form when game prop changes', () => {
      const { rerender } = renderWithProviders(
        <EditGameModal
          game={mockGame}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      const updatedGame: GameWithDetails = {
        ...mockGame,
        title: 'Different Game',
        description: 'Different description',
      };

      rerender(
        <EditGameModal
          game={updatedGame}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      expect(screen.getByLabelText(/title/i)).toHaveValue('Different Game');
      expect(screen.getByLabelText(/description/i)).toHaveValue('Different description');
    });
  });

  describe('Accessibility', () => {
    it('associates labels with inputs correctly', () => {
      renderWithProviders(
        <EditGameModal
          game={mockGame}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      expect(screen.getByLabelText(/title/i)).toHaveAttribute('id', 'title');
      expect(screen.getByLabelText(/description/i)).toHaveAttribute('id', 'description');
      expect(screen.getByLabelText(/genre/i)).toHaveAttribute('id', 'genre');
      expect(screen.getByLabelText(/maximum players/i)).toHaveAttribute('id', 'max_players');
    });

    it('uses semantic button elements', () => {
      renderWithProviders(
        <EditGameModal
          game={mockGame}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      const saveButton = screen.getByRole('button', { name: /save changes/i });
      expect(saveButton).toHaveAttribute('type', 'submit');

      const cancelButton = screen.getByRole('button', { name: /cancel/i });
      expect(cancelButton).toHaveAttribute('type', 'button');
    });

    it('marks required fields properly', () => {
      renderWithProviders(
        <EditGameModal
          game={mockGame}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      expect(screen.getByLabelText(/title/i)).toBeRequired();
      expect(screen.getByLabelText(/description/i)).toBeRequired();
      expect(screen.getByLabelText(/genre/i)).not.toBeRequired();
    });
  });

  describe('Integration', () => {
    it('handles complete update workflow', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <EditGameModal
          game={mockGame}
          isOpen={true}
          onClose={mockOnClose}
          onGameUpdated={mockOnGameUpdated}
        />
      );

      // Update multiple fields
      const titleInput = screen.getByLabelText(/title/i);
      await user.clear(titleInput);
      await user.type(titleInput, 'Epic Adventure');

      const descriptionInput = screen.getByLabelText(/description/i);
      await user.clear(descriptionInput);
      await user.type(descriptionInput, 'An epic fantasy adventure awaits');

      const genreInput = screen.getByLabelText(/genre/i);
      await user.clear(genreInput);
      await user.type(genreInput, 'High Fantasy');

      const maxPlayersInput = screen.getByLabelText(/maximum players/i);
      await user.clear(maxPlayersInput);
      await user.type(maxPlayersInput, '8');

      const anonymousCheckbox = screen.getByLabelText(/anonymous mode/i);
      await user.click(anonymousCheckbox);

      // Submit
      const saveButton = screen.getByRole('button', { name: /save changes/i });
      await user.click(saveButton);

      // Verify callbacks
      await waitFor(() => {
        expect(mockOnGameUpdated).toHaveBeenCalledTimes(1);
        expect(mockOnClose).toHaveBeenCalledTimes(1);
      });
    });
  
  // Reassignment rides on the edit form (decision 4), setup-only.
  describe('Community reassignment', () => {
    it('offers the picker while the game is in setup', async () => {
      renderWithProviders(
        <EditGameModal game={mockGame} isOpen={true} onClose={() => {}} onGameUpdated={() => {}} />
      );

      expect(await screen.findByTestId('game-community')).toBeInTheDocument();
    });

    it('withholds the picker once the game has left setup', async () => {
      const recruiting = { ...mockGame, state: 'recruitment' as const };
      renderWithProviders(
        <EditGameModal game={recruiting} isOpen={true} onClose={() => {}} onGameUpdated={() => {}} />
      );

      // The title field proves the form rendered at all.
      await screen.findByDisplayValue('Test Game');
      expect(screen.queryByTestId('game-community')).not.toBeInTheDocument();
    });

    // THE FULL-REPLACE GUARD, at the wire level. UpdateGame replaces the fields
    // it is given, so an edit that says nothing about the community must send
    // no community_id at all -- otherwise it would clear it, and a game with no
    // community is one no ban can reach.
    it('omits community_id from an edit that did not touch it', async () => {
      const user = userEvent.setup();
      let sentBody: Record<string, unknown> | null = null;
      server.use(
        http.put('/api/v1/games/1', async ({ request }) => {
          sentBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json({ ...mockGame, title: 'Renamed' });
        })
      );

      renderWithProviders(
        <EditGameModal game={mockGame} isOpen={true} onClose={() => {}} onGameUpdated={() => {}} />
      );

      const title = await screen.findByDisplayValue('Test Game');
      await user.clear(title);
      await user.type(title, 'Renamed');
      await user.click(screen.getByRole('button', { name: /save/i }));

      await waitFor(() => expect(sentBody).not.toBeNull());
      expect(sentBody!).not.toHaveProperty('community_id');
    });
  });
});
});
