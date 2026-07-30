import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, waitFor, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server';
import { renderWithProviders } from '../../test-utils/render';
import { CreateGameForm } from '../CreateGameForm';

// Mock ResizeObserver for react-datepicker
global.ResizeObserver = vi.fn().mockImplementation(() => ({
  observe: vi.fn(),
  unobserve: vi.fn(),
  disconnect: vi.fn(),
}));

describe('CreateGameForm', () => {
  describe('Rendering', () => {
    it('renders all form fields', () => {
      renderWithProviders(<CreateGameForm />);

      // Required fields
      expect(screen.getByLabelText(/game title/i)).toBeInTheDocument();
      expect(screen.getByLabelText(/description/i)).toBeInTheDocument();

      // Optional fields
      expect(screen.getByLabelText(/genre/i)).toBeInTheDocument();
      expect(screen.getByLabelText(/maximum players/i)).toBeInTheDocument();
      expect(screen.getByLabelText(/recruitment deadline/i)).toBeInTheDocument();
      expect(screen.getByLabelText(/start date/i)).toBeInTheDocument();
      expect(screen.getByLabelText(/end date/i)).toBeInTheDocument();

      // Submit button
      expect(screen.getByRole('button', { name: /create game/i })).toBeInTheDocument();
    });

    it('renders with default values', () => {
      renderWithProviders(<CreateGameForm />);

      const titleInput = screen.getByLabelText(/game title/i) as HTMLInputElement;
      const descInput = screen.getByLabelText(/description/i) as HTMLTextAreaElement;
      const maxPlayersInput = screen.getByLabelText(/maximum players/i) as HTMLInputElement;

      expect(titleInput.value).toBe('');
      expect(descInput.value).toBe('');
      expect(maxPlayersInput.value).toBe('6'); // Default max_players
    });

    it('renders cancel button when onCancel provided', () => {
      const onCancel = vi.fn();
      renderWithProviders(<CreateGameForm onCancel={onCancel} />);

      expect(screen.getByRole('button', { name: /cancel/i })).toBeInTheDocument();
    });

    it('does not render cancel button when onCancel not provided', () => {
      renderWithProviders(<CreateGameForm />);

      expect(screen.queryByRole('button', { name: /cancel/i })).not.toBeInTheDocument();
    });

    it('shows game creation process info', () => {
      renderWithProviders(<CreateGameForm />);

      expect(screen.getByText(/game creation process/i)).toBeInTheDocument();
      expect(screen.getByText(/start in "setup" mode/i)).toBeInTheDocument();
      expect(screen.getByText(/switch to "recruitment"/i)).toBeInTheDocument();
    });
  });

  describe('Form Input', () => {
    it('updates title field when user types', async () => {
      const user = userEvent.setup();
      renderWithProviders(<CreateGameForm />);

      const titleInput = screen.getByLabelText(/game title/i);
      await user.type(titleInput, 'Epic Adventure');

      expect(titleInput).toHaveValue('Epic Adventure');
    });

    it('updates description field when user types', async () => {
      const user = userEvent.setup();
      renderWithProviders(<CreateGameForm />);

      const descInput = screen.getByLabelText(/description/i);
      await user.type(descInput, 'A thrilling journey awaits');

      expect(descInput).toHaveValue('A thrilling journey awaits');
    });

    it('updates genre field when user types', async () => {
      const user = userEvent.setup();
      renderWithProviders(<CreateGameForm />);

      const genreInput = screen.getByLabelText(/genre/i);
      await user.type(genreInput, 'Fantasy');

      expect(genreInput).toHaveValue('Fantasy');
    });

    it('updates max players field when user enters number', async () => {
      const user = userEvent.setup();
      renderWithProviders(<CreateGameForm />);

      const maxPlayersInput = screen.getByLabelText(/maximum players/i);
      await user.clear(maxPlayersInput);
      await user.type(maxPlayersInput, '8');

      expect(maxPlayersInput).toHaveValue(8);
    });
  });

  describe('Form Validation', () => {
    it('has required attribute on title field', () => {
      renderWithProviders(<CreateGameForm />);

      const titleInput = screen.getByLabelText(/game title/i);
      expect(titleInput).toBeRequired();
    });

    it('has required attribute on description field', () => {
      renderWithProviders(<CreateGameForm />);

      const descInput = screen.getByLabelText(/description/i);
      expect(descInput).toBeRequired();
    });

    it('shows error when title is only whitespace', async () => {
      const user = userEvent.setup();
      renderWithProviders(<CreateGameForm />);

      const submitButton = screen.getByRole('button', { name: /create game/i });
      const titleInput = screen.getByLabelText(/game title/i);
      const descInput = screen.getByLabelText(/description/i);

      await user.type(titleInput, '   '); // Only spaces
      await user.type(descInput, 'A description');
      await user.click(submitButton);

      await waitFor(() => {
        expect(screen.getAllByText(/game title is required/i).length).toBeGreaterThan(0);
      });
    });

    it('shows error when description is only whitespace', async () => {
      const user = userEvent.setup();
      renderWithProviders(<CreateGameForm />);

      const submitButton = screen.getByRole('button', { name: /create game/i });
      const titleInput = screen.getByLabelText(/game title/i);
      const descInput = screen.getByLabelText(/description/i);

      await user.type(titleInput, 'Epic Game');
      await user.type(descInput, '   '); // Only spaces
      await user.click(submitButton);

      await waitFor(() => {
        expect(screen.getAllByText(/game description is required/i).length).toBeGreaterThan(0);
      });
    });

    it('enforces max length on title field', () => {
      renderWithProviders(<CreateGameForm />);

      const titleInput = screen.getByLabelText(/game title/i) as HTMLInputElement;
      expect(titleInput.maxLength).toBe(255);
    });

    it('enforces max length on genre field', () => {
      renderWithProviders(<CreateGameForm />);

      const genreInput = screen.getByLabelText(/genre/i) as HTMLInputElement;
      expect(genreInput.maxLength).toBe(100);
    });

    it('sets min and max constraints on max players field', () => {
      renderWithProviders(<CreateGameForm />);

      const maxPlayersInput = screen.getByLabelText(/maximum players/i) as HTMLInputElement;
      expect(maxPlayersInput.min).toBe('1');
      expect(maxPlayersInput.max).toBe('20');
    });
  });

  describe('Form Submission', () => {
    beforeEach(() => {
      // Setup successful game creation mock with small delay for realistic timing
      server.use(
        http.post('/api/v1/games', async ({ request }) => {
          const body = await request.json();
          await new Promise(resolve => setTimeout(resolve, 10));
          return HttpResponse.json({
            id: 123,
            ...body,
            state: 'setup',
            created_at: new Date().toISOString(),
          });
        })
      );
    });

    it('submits form with required fields only', async () => {
      const user = userEvent.setup();
      const onSuccess = vi.fn();
      renderWithProviders(<CreateGameForm onSuccess={onSuccess} />);

      // Fill required fields
      await user.type(screen.getByLabelText(/game title/i), 'Test Game');
      await user.type(screen.getByLabelText(/description/i), 'Test Description');

      // Submit
      await user.click(screen.getByRole('button', { name: /create game/i }));

      await waitFor(() => {
        expect(onSuccess).toHaveBeenCalledWith(123);
      });
    });

    it('submits form with all fields filled', async () => {
      const user = userEvent.setup();
      const onSuccess = vi.fn();
      renderWithProviders(<CreateGameForm onSuccess={onSuccess} />);

      // Fill all fields. fireEvent.change commits the whole value in one update
      // instead of one re-render per keystroke across seven fields, which is what
      // pushed this test past the timeout under parallel load.
      fireEvent.change(screen.getByLabelText(/game title/i), { target: { value: 'Epic Adventure' } });
      fireEvent.change(screen.getByLabelText(/description/i), { target: { value: 'An amazing journey' } });
      fireEvent.change(screen.getByLabelText(/genre/i), { target: { value: 'Fantasy' } });
      fireEvent.change(screen.getByLabelText(/maximum players/i), { target: { value: '10' } });
      fireEvent.change(screen.getByLabelText(/recruitment deadline/i), { target: { value: '2025-12-31T23:59' } });
      fireEvent.change(screen.getByLabelText(/start date/i), { target: { value: '2026-01-01T00:00' } });
      fireEvent.change(screen.getByLabelText(/end date/i), { target: { value: '2026-06-30T23:59' } });

      // Submit
      await user.click(screen.getByRole('button', { name: /create game/i }));

      await waitFor(() => {
        expect(onSuccess).toHaveBeenCalledWith(123);
      });
    });

    it('trims whitespace from title and description', async () => {
      const user = userEvent.setup();
      let submittedData: unknown = null;
      const onSuccess = vi.fn();

      server.use(
        http.post('/api/v1/games', async ({ request }) => {
          submittedData = await request.json();
          return HttpResponse.json({ id: 123, ...submittedData });
        })
      );

      renderWithProviders(<CreateGameForm onSuccess={onSuccess} />);

      const titleInput = screen.getByLabelText(/game title/i);
      const descriptionInput = screen.getByLabelText(/description/i);

      await user.click(titleInput);
      await user.paste('  Spaced Title  ');
      await user.click(descriptionInput);
      await user.paste('  Spaced Description  ');

      // Guard against submitting a partially-committed controlled input.
      await waitFor(() => {
        expect(titleInput).toHaveValue('  Spaced Title  ');
        expect(descriptionInput).toHaveValue('  Spaced Description  ');
      });

      await user.click(screen.getByRole('button', { name: /create game/i }));

      await waitFor(() => {
        expect(onSuccess).toHaveBeenCalled();
      });

      expect(submittedData).not.toBeNull();
      expect(submittedData.title).toBe('Spaced Title');
      expect(submittedData.description).toBe('Spaced Description');
    });

    it('converts empty date strings to undefined', async () => {
      const user = userEvent.setup();
      let submittedData: unknown = null;
      const onSuccess = vi.fn();

      server.use(
        http.post('/api/v1/games', async ({ request }) => {
          submittedData = await request.json();
          return HttpResponse.json({ id: 123, ...submittedData });
        })
      );

      renderWithProviders(<CreateGameForm onSuccess={onSuccess} />);

      await user.type(screen.getByLabelText(/game title/i), 'Test Game');
      await user.type(screen.getByLabelText(/description/i), 'Test Description');
      // Don't fill any date fields
      await user.click(screen.getByRole('button', { name: /create game/i }));

      await waitFor(() => {
        expect(onSuccess).toHaveBeenCalled();
      });

      expect(submittedData).not.toBeNull();
      expect(submittedData.start_date).toBeUndefined();
      expect(submittedData.end_date).toBeUndefined();
      expect(submittedData.recruitment_deadline).toBeUndefined();
    });

    it('converts empty genre to undefined', async () => {
      const user = userEvent.setup();
      let submittedData: unknown = null;
      const onSuccess = vi.fn();

      server.use(
        http.post('/api/v1/games', async ({ request }) => {
          submittedData = await request.json();
          return HttpResponse.json({ id: 123, ...submittedData });
        })
      );

      renderWithProviders(<CreateGameForm onSuccess={onSuccess} />);

      await user.type(screen.getByLabelText(/game title/i), 'Test Game');
      await user.type(screen.getByLabelText(/description/i), 'Test Description');
      // Don't fill genre field
      await user.click(screen.getByRole('button', { name: /create game/i }));

      await waitFor(() => {
        expect(onSuccess).toHaveBeenCalled();
      });

      expect(submittedData).not.toBeNull();
      expect(submittedData.genre).toBeUndefined();
    });

    it('shows loading state while submitting', async () => {
      const user = userEvent.setup();

      // Delay the response to see loading state
      server.use(
        http.post('/api/v1/games', async () => {
          await new Promise(resolve => setTimeout(resolve, 100));
          return HttpResponse.json({ id: 123 });
        })
      );

      renderWithProviders(<CreateGameForm />);

      await user.type(screen.getByLabelText(/game title/i), 'Test Game');
      await user.type(screen.getByLabelText(/description/i), 'Test Description');

      const submitButton = screen.getByRole('button', { name: /create game/i });
      await user.click(submitButton);

      // Should show loading text
      expect(screen.getByText(/creating game\.\.\./i)).toBeInTheDocument();

      // Button should be disabled
      expect(submitButton).toBeDisabled();

      // Wait for submission to complete
      await waitFor(() => {
        expect(screen.getByText(/create game/i)).toBeInTheDocument();
      });
    });

    it('disables submit button while submitting', async () => {
      const user = userEvent.setup();
      renderWithProviders(<CreateGameForm onSuccess={vi.fn()} />);

      await user.type(screen.getByLabelText(/game title/i), 'Test Game');
      await user.type(screen.getByLabelText(/description/i), 'Test Description');

      const submitButton = screen.getByRole('button', { name: /create game/i });
      await user.click(submitButton);

      expect(submitButton).toBeDisabled();

      await waitFor(() => {
        expect(submitButton).not.toBeDisabled();
      });
    });
  });

  describe('Error Handling', () => {
    it('displays API error message', async () => {
      const user = userEvent.setup();

      server.use(
        http.post('/api/v1/games', () => {
          return HttpResponse.json(
            { error: 'Game title already exists' },
            { status: 400 }
          );
        })
      );

      renderWithProviders(<CreateGameForm />);

      await user.type(screen.getByLabelText(/game title/i), 'Duplicate Game');
      await user.type(screen.getByLabelText(/description/i), 'Test Description');
      await user.click(screen.getByRole('button', { name: /create game/i }));

      await waitFor(() => {
        expect(screen.getAllByText(/game title already exists/i).length).toBeGreaterThan(0);
      });
    });

    it('displays generic error for network failures', async () => {
      const user = userEvent.setup();

      server.use(
        http.post('/api/v1/games', () => {
          return HttpResponse.error();
        })
      );

      renderWithProviders(<CreateGameForm />);

      await user.type(screen.getByLabelText(/game title/i), 'Test Game');
      await user.type(screen.getByLabelText(/description/i), 'Test Description');
      await user.click(screen.getByRole('button', { name: /create game/i }));

      await waitFor(() => {
        expect(screen.getAllByText(/failed to create game/i).length).toBeGreaterThan(0);
      });
    });

    it('clears error when form is resubmitted', async () => {
      const user = userEvent.setup();

      // First submission fails
      server.use(
        http.post('/api/v1/games', () => {
          return HttpResponse.json(
            { error: 'Validation error' },
            { status: 400 }
          );
        })
      );

      renderWithProviders(<CreateGameForm />);

      await user.type(screen.getByLabelText(/game title/i), 'Test Game');
      await user.type(screen.getByLabelText(/description/i), 'Test Description');
      await user.click(screen.getByRole('button', { name: /create game/i }));

      // Wait for error
      await waitFor(() => {
        expect(screen.getAllByText(/validation error/i).length).toBeGreaterThan(0);
      });

      // Fix the API to succeed
      server.use(
        http.post('/api/v1/games', () => {
          return HttpResponse.json({ id: 123 });
        })
      );

      // Resubmit
      await user.click(screen.getByRole('button', { name: /create game/i }));

      // Error should be cleared
      await waitFor(() => {
        expect(screen.queryByText(/validation error/i)).not.toBeInTheDocument();
      });
    });

    it('re-enables submit button after error', async () => {
      const user = userEvent.setup();

      server.use(
        http.post('/api/v1/games', () => {
          return HttpResponse.json(
            { error: 'Error' },
            { status: 500 }
          );
        })
      );

      renderWithProviders(<CreateGameForm />);

      await user.type(screen.getByLabelText(/game title/i), 'Test Game');
      await user.type(screen.getByLabelText(/description/i), 'Test Description');

      const submitButton = screen.getByRole('button', { name: /create game/i });
      await user.click(submitButton);

      // Wait for error
      await waitFor(() => {
        expect(screen.getAllByText(/error/i).length).toBeGreaterThan(0);
      });

      // Button should be re-enabled
      expect(submitButton).not.toBeDisabled();
    });
  });

  describe('Callbacks', () => {
    it('calls onSuccess with game ID when creation succeeds', async () => {
      const user = userEvent.setup();
      const onSuccess = vi.fn();

      server.use(
        http.post('/api/v1/games', () => {
          return HttpResponse.json({ id: 456 });
        })
      );

      renderWithProviders(<CreateGameForm onSuccess={onSuccess} />);

      await user.type(screen.getByLabelText(/game title/i), 'Test Game');
      await user.type(screen.getByLabelText(/description/i), 'Test Description');
      await user.click(screen.getByRole('button', { name: /create game/i }));

      await waitFor(() => {
        expect(onSuccess).toHaveBeenCalledWith(456);
        expect(onSuccess).toHaveBeenCalledTimes(1);
      });
    });

    it('does not call onSuccess when onSuccess is not provided', async () => {
      const user = userEvent.setup();

      server.use(
        http.post('/api/v1/games', () => {
          return HttpResponse.json({ id: 123 });
        })
      );

      renderWithProviders(<CreateGameForm />);

      await user.type(screen.getByLabelText(/game title/i), 'Test Game');
      await user.type(screen.getByLabelText(/description/i), 'Test Description');

      // Should not throw error
      await user.click(screen.getByRole('button', { name: /create game/i }));

      // Just wait for submission to complete
      await waitFor(() => {
        expect(screen.getByRole('button', { name: /create game/i })).not.toBeDisabled();
      });
    });

    it('calls onCancel when cancel button is clicked', async () => {
      const user = userEvent.setup();
      const onCancel = vi.fn();

      renderWithProviders(<CreateGameForm onCancel={onCancel} />);

      const cancelButton = screen.getByRole('button', { name: /cancel/i });
      await user.click(cancelButton);

      expect(onCancel).toHaveBeenCalledTimes(1);
    });

    it('does not submit form when cancel is clicked', async () => {
      const user = userEvent.setup();
      const onCancel = vi.fn();
      const onSuccess = vi.fn();

      renderWithProviders(<CreateGameForm onCancel={onCancel} onSuccess={onSuccess} />);

      await user.type(screen.getByLabelText(/game title/i), 'Test Game');
      await user.type(screen.getByLabelText(/description/i), 'Test Description');

      const cancelButton = screen.getByRole('button', { name: /cancel/i });
      await user.click(cancelButton);

      expect(onCancel).toHaveBeenCalled();
      expect(onSuccess).not.toHaveBeenCalled();
    });
  });
});
