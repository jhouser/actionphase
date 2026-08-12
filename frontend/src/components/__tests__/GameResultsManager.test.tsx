import { describe, it, expect, beforeEach } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server';
import { renderWithProviders } from '../../test-utils/render';
import { GameResultsManager } from '../GameResultsManager';
import type { ActionResult } from '../../types/phases';

describe('GameResultsManager', () => {
  const mockGameId = 1;

  const mockUnpublishedResult: ActionResult = {
    id: 1,
    game_id: mockGameId,
    user_id: 100,
    phase_id: 1,
    gm_user_id: 1,
    content: 'This is an unpublished result for the player',
    is_published: false,
    sent_at: '2025-01-15T10:00:00Z',
    phase_type: 'action',
    phase_number: 1,
    gm_username: 'testgm',
    username: 'player1',
  };

  const mockPublishedResult: ActionResult = {
    id: 2,
    game_id: mockGameId,
    user_id: 101,
    phase_id: 2,
    gm_user_id: 1,
    content: 'This is a published result that was sent',
    is_published: true,
    sent_at: '2025-01-16T14:30:00Z',
    phase_type: 'action',
    phase_number: 2,
    gm_username: 'testgm',
    username: 'player2',
  };

  const mockUnpublishedResult2: ActionResult = {
    id: 3,
    game_id: mockGameId,
    user_id: 102,
    phase_id: 1,
    gm_user_id: 1,
    content: 'Another unpublished draft result',
    is_published: false,
    sent_at: '2025-01-15T11:00:00Z',
    username: 'player3',
  };

  const setupDefaultHandlers = (results: ActionResult[] = []) => {
    server.use(
      http.get('/api/v1/games/:gameId/results', () => {
        return HttpResponse.json(results);
      }),
      http.put('/api/v1/games/:gameId/results/:resultId', async ({ request }) => {
        const body = await request.json() as { content: string };
        return HttpResponse.json({
          ...mockUnpublishedResult,
          content: body.content,
        });
      })
    );
  };

  beforeEach(() => {
    server.resetHandlers();
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('shows loading state initially', () => {
      server.use(
        http.get('/api/v1/games/:gameId/results', async () => {
          await new Promise((resolve) => setTimeout(resolve, 100));
          return HttpResponse.json([]);
        })
      );

      const { container } = renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      // Loading skeleton should be present (with animate-pulse class)
      const loadingContainer = container.querySelector('.animate-pulse');
      expect(loadingContainer).toBeInTheDocument();
    });

    it('renders empty state when no results exist', async () => {
      setupDefaultHandlers([]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByText('Action Results')).toBeInTheDocument();
        expect(screen.getByText('No results have been created yet.')).toBeInTheDocument();
      });
    });

    it('renders header with title and description', async () => {
      setupDefaultHandlers([mockUnpublishedResult]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByText('Action Results')).toBeInTheDocument();
        expect(screen.getByText('Manage results sent to players')).toBeInTheDocument();
      });
    });

    it('applies custom className when provided', async () => {
      setupDefaultHandlers([mockUnpublishedResult]);

      const { container } = renderWithProviders(
        <GameResultsManager gameId={mockGameId} className="custom-class" />
      );

      await waitFor(() => {
        const mainContainer = container.querySelector('.custom-class');
        expect(mainContainer).toBeInTheDocument();
      });
    });
  });

  describe('Result Counts Display', () => {
    it('displays correct count badges for unpublished and published results', async () => {
      setupDefaultHandlers([
        mockUnpublishedResult,
        mockUnpublishedResult2,
        mockPublishedResult,
      ]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByText('2 Unpublished')).toBeInTheDocument();
        expect(screen.getByText('1 Published')).toBeInTheDocument();
      });
    });

    it('shows zero count when no unpublished results', async () => {
      setupDefaultHandlers([mockPublishedResult]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByText('0 Unpublished')).toBeInTheDocument();
        expect(screen.getByText('1 Published')).toBeInTheDocument();
      });
    });

    it('shows zero count when no published results', async () => {
      setupDefaultHandlers([mockUnpublishedResult]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByText('1 Unpublished')).toBeInTheDocument();
        expect(screen.getByText('0 Published')).toBeInTheDocument();
      });
    });
  });

  describe('Unpublished Results Section', () => {
    it('displays unpublished results section when unpublished results exist', async () => {
      setupDefaultHandlers([mockUnpublishedResult, mockUnpublishedResult2]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByText('Unpublished Results (Editable)')).toBeInTheDocument();
      });
    });

    it('does not show unpublished section when no unpublished results', async () => {
      setupDefaultHandlers([mockPublishedResult]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.queryByText('Unpublished Results (Editable)')).not.toBeInTheDocument();
      });
    });

    it('displays unpublished result content', async () => {
      setupDefaultHandlers([mockUnpublishedResult]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByText('This is an unpublished result for the player')).toBeInTheDocument();
      });
    });

    it('displays username for unpublished results', async () => {
      setupDefaultHandlers([mockUnpublishedResult]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByText('To: player1')).toBeInTheDocument();
      });
    });

    it('displays user ID when username is not available', async () => {
      const resultWithoutUsername: ActionResult = {
        ...mockUnpublishedResult,
        username: undefined,
      };
      setupDefaultHandlers([resultWithoutUsername]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByText('To: User #100')).toBeInTheDocument();
      });
    });

    it('displays draft badge for unpublished results', async () => {
      setupDefaultHandlers([mockUnpublishedResult]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByText('Draft')).toBeInTheDocument();
      });
    });

    it('displays phase information when available', async () => {
      setupDefaultHandlers([mockUnpublishedResult]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByText('Phase 1')).toBeInTheDocument();
      });
    });

    it('shows Edit button for unpublished results', async () => {
      setupDefaultHandlers([mockUnpublishedResult]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /edit/i })).toBeInTheDocument();
      });
    });

    it('displays multiple unpublished results', async () => {
      setupDefaultHandlers([mockUnpublishedResult, mockUnpublishedResult2]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByText('This is an unpublished result for the player')).toBeInTheDocument();
        expect(screen.getByText('Another unpublished draft result')).toBeInTheDocument();
      });
    });
  });

  describe('Published Results Section', () => {
    it('displays published results section when published results exist', async () => {
      setupDefaultHandlers([mockPublishedResult]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByText('Published Results')).toBeInTheDocument();
      });
    });

    it('does not show published section when no published results', async () => {
      setupDefaultHandlers([mockUnpublishedResult]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.queryByText('Published Results')).not.toBeInTheDocument();
      });
    });

    it('displays published result content', async () => {
      setupDefaultHandlers([mockPublishedResult]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByText('This is a published result that was sent')).toBeInTheDocument();
      });
    });

    it('displays username for published results', async () => {
      setupDefaultHandlers([mockPublishedResult]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByText('To: player2')).toBeInTheDocument();
      });
    });

    it('displays sent timestamp for published results', async () => {
      setupDefaultHandlers([mockPublishedResult]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        // Date format can vary by locale, so just check for presence of date-like pattern
        const sentText = screen.getByText(/Sent:/);
        expect(sentText).toBeInTheDocument();
      });
    });

    it('does not show Edit button for published results', async () => {
      setupDefaultHandlers([mockPublishedResult]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.queryByRole('button', { name: /edit/i })).not.toBeInTheDocument();
      });
    });
  });

  describe('Edit Functionality', () => {
    it('shows edit form when Edit button is clicked', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers([mockUnpublishedResult]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /edit/i })).toBeInTheDocument();
      });

      const editButton = screen.getByRole('button', { name: /edit/i });
      await user.click(editButton);

      expect(screen.getByRole('textbox')).toBeInTheDocument();
      expect(screen.getByRole('textbox')).toHaveValue('This is an unpublished result for the player');
    });

    it('shows Save Changes and Cancel buttons in edit mode', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers([mockUnpublishedResult]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /edit/i })).toBeInTheDocument();
      });

      await user.click(screen.getByRole('button', { name: /edit/i }));

      expect(screen.getByRole('button', { name: /save changes/i })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /cancel/i })).toBeInTheDocument();
    });

    it('hides Edit button when in edit mode', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers([mockUnpublishedResult]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /edit/i })).toBeInTheDocument();
      });

      await user.click(screen.getByRole('button', { name: /edit/i }));

      // The Edit button should no longer be visible
      const editButtons = screen.queryAllByRole('button', { name: /^edit$/i });
      expect(editButtons).toHaveLength(0);
    });

    it('allows editing content in textarea', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers([mockUnpublishedResult]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /edit/i })).toBeInTheDocument();
      });

      await user.click(screen.getByRole('button', { name: /edit/i }));

      const textarea = screen.getByRole('textbox');
      await user.clear(textarea);
      await user.type(textarea, 'Updated content for the result');

      expect(textarea).toHaveValue('Updated content for the result');
    });

    it('closes edit form and reverts changes when Cancel is clicked', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers([mockUnpublishedResult]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /edit/i })).toBeInTheDocument();
      });

      await user.click(screen.getByRole('button', { name: /edit/i }));

      const textarea = screen.getByRole('textbox');
      await user.clear(textarea);
      await user.type(textarea, 'Changed content');

      await user.click(screen.getByRole('button', { name: /cancel/i }));

      // Edit form should be closed
      expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
      // Original content should still be displayed
      expect(screen.getByText('This is an unpublished result for the player')).toBeInTheDocument();
    });

    it('successfully saves changes when Save Changes is clicked', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers([mockUnpublishedResult]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /edit/i })).toBeInTheDocument();
      });

      await user.click(screen.getByRole('button', { name: /edit/i }));

      const textarea = screen.getByRole('textbox');
      await user.clear(textarea);
      await user.type(textarea, 'Updated result content');

      await user.click(screen.getByRole('button', { name: /save changes/i }));

      await waitFor(() => {
        // Edit form should be closed
        expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
      });
    });

    it('trims whitespace from content before saving', async () => {
      const user = userEvent.setup();
      let requestBody: unknown = null;

      server.use(
        http.get('/api/v1/games/:gameId/results', () => {
          return HttpResponse.json([mockUnpublishedResult]);
        }),
        http.put('/api/v1/games/:gameId/results/:resultId', async ({ request }) => {
          requestBody = await request.json();
          return HttpResponse.json({
            ...mockUnpublishedResult,
            content: requestBody.content,
          });
        })
      );

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /edit/i })).toBeInTheDocument();
      });

      await user.click(screen.getByRole('button', { name: /edit/i }));

      const textarea = screen.getByRole('textbox');
      await user.clear(textarea);
      await user.type(textarea, '  Trimmed content  ');

      await user.click(screen.getByRole('button', { name: /save changes/i }));

      await waitFor(() => {
        expect(requestBody).toEqual({ content: 'Trimmed content' });
      });
    });

    it('does not save when content is unchanged', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers([mockUnpublishedResult]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /edit/i })).toBeInTheDocument();
      });

      await user.click(screen.getByRole('button', { name: /edit/i }));

      // Don't change anything, just click save
      await user.click(screen.getByRole('button', { name: /save changes/i }));

      await waitFor(() => {
        // Edit form should be closed
        expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
      });
    });

    it('disables Save button when content is empty', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers([mockUnpublishedResult]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /edit/i })).toBeInTheDocument();
      });

      await user.click(screen.getByRole('button', { name: /edit/i }));

      const textarea = screen.getByRole('textbox');
      await user.clear(textarea);

      const saveButton = screen.getByRole('button', { name: /save changes/i });
      expect(saveButton).toBeDisabled();
    });

    it('disables Save button when content is only whitespace', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers([mockUnpublishedResult]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /edit/i })).toBeInTheDocument();
      });

      await user.click(screen.getByRole('button', { name: /edit/i }));

      const textarea = screen.getByRole('textbox');
      await user.clear(textarea);
      await user.type(textarea, '   ');

      const saveButton = screen.getByRole('button', { name: /save changes/i });
      expect(saveButton).toBeDisabled();
    });

    it('offers a markdown preview of the draft being edited', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers([mockUnpublishedResult]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /edit/i })).toBeInTheDocument();
      });

      await user.click(screen.getByRole('button', { name: /edit/i }));

      // Replace the draft with markdown, then switch to the Preview tab.
      const textarea = screen.getByRole('textbox');
      await user.clear(textarea);
      await user.type(textarea, '**Bolded result**');

      await user.click(screen.getByRole('button', { name: /preview/i }));

      // The markdown must be rendered, not shown as raw asterisks.
      const rendered = await screen.findByText('Bolded result');
      expect(rendered.tagName).toBe('STRONG');
      expect(screen.queryByText('**Bolded result**')).not.toBeInTheDocument();
    });

    it('keeps edited content when toggling between Write and Preview', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers([mockUnpublishedResult]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /edit/i })).toBeInTheDocument();
      });

      await user.click(screen.getByRole('button', { name: /edit/i }));

      await user.clear(screen.getByRole('textbox'));
      await user.type(screen.getByRole('textbox'), 'Draft in progress');

      await user.click(screen.getByRole('button', { name: /preview/i }));
      await user.click(screen.getByRole('button', { name: /write/i }));

      expect(screen.getByRole('textbox')).toHaveValue('Draft in progress');
    });

    it('can edit only one result at a time', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers([mockUnpublishedResult, mockUnpublishedResult2]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        const editButtons = screen.getAllByRole('button', { name: /edit/i });
        expect(editButtons).toHaveLength(2);
      });

      const editButtons = screen.getAllByRole('button', { name: /edit/i });
      await user.click(editButtons[0]);

      // Only one textarea should be visible
      const textareas = screen.getAllByRole('textbox');
      expect(textareas).toHaveLength(1);
    });
  });

  describe('Loading States', () => {
    it('shows loading text while saving changes', async () => {
      const user = userEvent.setup();

      // Hold the save open until the test releases it, rather than racing a
      // timer. This keeps the pending state observable for as long as we need
      // and lets the click be fully awaited, so no update escapes act(...).
      let releaseSave!: () => void;
      const savePending = new Promise<void>((resolve) => {
        releaseSave = resolve;
      });

      server.use(
        http.get('/api/v1/games/:gameId/results', () => {
          return HttpResponse.json([mockUnpublishedResult]);
        }),
        http.put('/api/v1/games/:gameId/results/:resultId', async () => {
          await savePending;
          return HttpResponse.json(mockUnpublishedResult);
        })
      );

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /edit/i })).toBeInTheDocument();
      });

      await user.click(screen.getByRole('button', { name: /edit/i }));

      const textarea = screen.getByRole('textbox');
      await user.clear(textarea);
      await user.type(textarea, 'Updated content');

      await user.click(screen.getByRole('button', { name: /save changes/i }));

      expect(await screen.findByText('Saving...')).toBeInTheDocument();

      // Release the save and let the component exit edit mode before the test
      // ends, so the resulting updates land inside act(...).
      releaseSave();
      await waitFor(() => {
        expect(screen.getByRole('button', { name: /edit/i })).toBeInTheDocument();
      });
    });

    it('disables form controls while saving', async () => {
      const user = userEvent.setup();

      // Hold the save open until the test releases it, rather than racing a
      // timer. This keeps the pending state observable for as long as we need
      // and lets the click be fully awaited, so no update escapes act(...).
      let releaseSave!: () => void;
      const savePending = new Promise<void>((resolve) => {
        releaseSave = resolve;
      });

      server.use(
        http.get('/api/v1/games/:gameId/results', () => {
          return HttpResponse.json([mockUnpublishedResult]);
        }),
        http.put('/api/v1/games/:gameId/results/:resultId', async () => {
          await savePending;
          return HttpResponse.json(mockUnpublishedResult);
        })
      );

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /edit/i })).toBeInTheDocument();
      });

      await user.click(screen.getByRole('button', { name: /edit/i }));

      const textarea = screen.getByRole('textbox');
      await user.clear(textarea);
      await user.type(textarea, 'Updated content');

      await user.click(screen.getByRole('button', { name: /save changes/i }));

      expect(await screen.findByRole('button', { name: /saving\.\.\./i })).toBeDisabled();
      expect(screen.getByRole('button', { name: /cancel/i })).toBeDisabled();

      // Release the save and let the component exit edit mode before the test
      // ends, so the resulting updates land inside act(...).
      releaseSave();
      await waitFor(() => {
        expect(screen.getByRole('button', { name: /edit/i })).toBeInTheDocument();
      });
    });
  });

  describe('Error Handling', () => {
    it('shows error message when save fails', async () => {
      const user = userEvent.setup();

      server.use(
        http.get('/api/v1/games/:gameId/results', () => {
          return HttpResponse.json([mockUnpublishedResult]);
        }),
        http.put('/api/v1/games/:gameId/results/:resultId', () => {
          return HttpResponse.json(
            { error: 'Failed to update result' },
            { status: 500 }
          );
        })
      );

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /edit/i })).toBeInTheDocument();
      });

      await user.click(screen.getByRole('button', { name: /edit/i }));

      const textarea = screen.getByRole('textbox');
      await user.clear(textarea);
      await user.type(textarea, 'Updated content');

      await user.click(screen.getByRole('button', { name: /save changes/i }));

      await waitFor(() => {
        expect(screen.getByText('Failed to update result. Please try again.')).toBeInTheDocument();
      });
    });

    it('keeps edit form open when save fails', async () => {
      const user = userEvent.setup();

      server.use(
        http.get('/api/v1/games/:gameId/results', () => {
          return HttpResponse.json([mockUnpublishedResult]);
        }),
        http.put('/api/v1/games/:gameId/results/:resultId', () => {
          return HttpResponse.json(
            { error: 'Failed to update result' },
            { status: 500 }
          );
        })
      );

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /edit/i })).toBeInTheDocument();
      });

      await user.click(screen.getByRole('button', { name: /edit/i }));

      const textarea = screen.getByRole('textbox');
      await user.clear(textarea);
      await user.type(textarea, 'Updated content');

      await user.click(screen.getByRole('button', { name: /save changes/i }));

      await waitFor(() => {
        expect(screen.getByText('Failed to update result. Please try again.')).toBeInTheDocument();
      });

      // Edit form should still be open with the content
      expect(screen.getByRole('textbox')).toBeInTheDocument();
      expect(screen.getByRole('textbox')).toHaveValue('Updated content');
    });
  });

  describe('Styling and Visual States', () => {
    it('applies different styling to unpublished results', async () => {
      setupDefaultHandlers([mockUnpublishedResult]);

      const { container } = renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        const unpublishedCard = container.querySelector('.border-semantic-warning');
        expect(unpublishedCard).toBeInTheDocument();
      });
    });

    it('applies different styling to published results', async () => {
      setupDefaultHandlers([mockPublishedResult]);

      const { container } = renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        const publishedCard = container.querySelector('.border-semantic-success');
        expect(publishedCard).toBeInTheDocument();
      });
    });

    it('displays warning icon for unpublished section', async () => {
      setupDefaultHandlers([mockUnpublishedResult]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByText('Unpublished Results (Editable)')).toBeInTheDocument();
      });

      // Check for warning icon (svg path)
      const heading = screen.getByText('Unpublished Results (Editable)');
      const svg = heading.closest('h3')?.querySelector('svg');
      expect(svg).toBeInTheDocument();
    });

    it('displays check icon for published section', async () => {
      setupDefaultHandlers([mockPublishedResult]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByText('Published Results')).toBeInTheDocument();
      });

      // Check for check icon (svg path)
      const heading = screen.getByText('Published Results');
      const svg = heading.closest('h3')?.querySelector('svg');
      expect(svg).toBeInTheDocument();
    });

    it('renders markdown content correctly', async () => {
      const resultWithMarkdown: ActionResult = {
        ...mockUnpublishedResult,
        content: '**Bold text**\n\nThis is a paragraph with *italic* text.\n\n- List item 1\n- List item 2',
      };
      setupDefaultHandlers([resultWithMarkdown]);

      const { container } = renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        // Check that MarkdownPreview component is rendering
        const markdownContainer = container.querySelector('.markdown-preview');
        expect(markdownContainer).toBeInTheDocument();

        // Check that markdown is rendered as HTML elements
        const boldElement = container.querySelector('strong');
        expect(boldElement).toBeInTheDocument();
        expect(boldElement).toHaveTextContent('Bold text');

        const italicElement = container.querySelector('em');
        expect(italicElement).toBeInTheDocument();
        expect(italicElement).toHaveTextContent('italic');

        const listItems = container.querySelectorAll('li');
        expect(listItems.length).toBeGreaterThanOrEqual(2);
      });
    });
  });

  describe('Collapse/Expand Functionality', () => {
    it('shows collapse button for long unpublished results (>200 characters)', async () => {
      const longContent = 'A'.repeat(250); // 250 characters
      const longResult: ActionResult = {
        ...mockUnpublishedResult,
        content: longContent,
      };
      setupDefaultHandlers([longResult]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByText(/show full content/i)).toBeInTheDocument();
      });
    });

    it('does NOT show collapse button for short unpublished results (<=200 characters)', async () => {
      const shortContent = 'A'.repeat(150); // 150 characters
      const shortResult: ActionResult = {
        ...mockUnpublishedResult,
        content: shortContent,
      };
      setupDefaultHandlers([shortResult]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByText(shortContent)).toBeInTheDocument();
      });

      expect(screen.queryByText(/show full content/i)).not.toBeInTheDocument();
    });

    it('shows collapse button for long published results', async () => {
      const longContent = 'A'.repeat(350);
      const longPublishedResult: ActionResult = {
        ...mockPublishedResult,
        content: longContent,
      };
      setupDefaultHandlers([longPublishedResult]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      // Should show truncated content initially
      await waitFor(() => {
        expect(screen.getByText(longContent.substring(0, 200) + '...')).toBeInTheDocument();
      });

      // Should have "Show full content" button
      const showMoreButton = screen.getByText(/show full content/i);
      expect(showMoreButton).toBeInTheDocument();

      // Click to expand
      await userEvent.click(showMoreButton);

      // Now full content should be visible
      await waitFor(() => {
        expect(screen.getByText(longContent)).toBeInTheDocument();
      });

      // Button text should change to "Show less"
      expect(screen.getByText(/show less/i)).toBeInTheDocument();
    });

    it('shows truncated preview when collapsed', async () => {
      const longContent = 'This is a very long unpublished result that exceeds the 200 character limit and should be collapsed by default. It contains important information that the GM is still drafting and needs to review before publishing to players. The content goes on and on with more details.';
      const longResult: ActionResult = {
        ...mockUnpublishedResult,
        content: longContent,
      };
      setupDefaultHandlers([longResult]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        // Should see first 200 characters + "..."
        const preview = longContent.substring(0, 200) + '...';
        expect(screen.getByText(preview)).toBeInTheDocument();
        // Should NOT see the full content initially
        expect(screen.queryByText(longContent)).not.toBeInTheDocument();
      });
    });

    it('expands to show full content when "Show full content" is clicked', async () => {
      const user = userEvent.setup();
      const longContent = 'This is a very long unpublished result that exceeds the 200 character limit. It contains important information that the GM is still drafting and needs to review before publishing to players. The content continues with more narrative details about the game.';
      const longResult: ActionResult = {
        ...mockUnpublishedResult,
        content: longContent,
      };
      setupDefaultHandlers([longResult]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      // Wait for truncated preview
      await waitFor(() => {
        const preview = longContent.substring(0, 200) + '...';
        expect(screen.getByText(preview)).toBeInTheDocument();
      });

      // Click expand button
      await user.click(screen.getByText(/show full content/i));

      // Should now see full content
      await waitFor(() => {
        expect(screen.getByText(longContent)).toBeInTheDocument();
        expect(screen.getByText(/show less/i)).toBeInTheDocument();
      });
    });

    it('collapses to show preview when "Show less" is clicked', async () => {
      const user = userEvent.setup();
      const longContent = 'This is a very long unpublished result that needs to be collapsed. It has lots of details about what happened during the action phase and the consequences of the player\'s choices. The GM is still working on perfecting this narrative before sending it.';
      const longResult: ActionResult = {
        ...mockUnpublishedResult,
        content: longContent,
      };
      setupDefaultHandlers([longResult]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      // Expand first
      await waitFor(() => {
        expect(screen.getByText(/show full content/i)).toBeInTheDocument();
      });
      await user.click(screen.getByText(/show full content/i));

      // Wait for expansion
      await waitFor(() => {
        expect(screen.getByText(longContent)).toBeInTheDocument();
      });

      // Collapse
      await user.click(screen.getByText(/show less/i));

      // Should see preview again
      await waitFor(() => {
        const preview = longContent.substring(0, 200) + '...';
        expect(screen.getByText(preview)).toBeInTheDocument();
        expect(screen.queryByText(longContent)).not.toBeInTheDocument();
      });
    });

    it('maintains separate collapse state for multiple long results', async () => {
      const user = userEvent.setup();
      const longResult1: ActionResult = {
        ...mockUnpublishedResult,
        id: 1,
        content: 'First result content that is very long and exceeds 200 characters. This contains the narrative about what happened to the first player during their investigation. The GM has drafted this carefully but hasn\'t published it yet. More content continues here.',
      };
      const longResult2: ActionResult = {
        ...mockUnpublishedResult,
        id: 2,
        content: 'Second result content that is also very long and exceeds 200 characters. This one describes what happened to the second player in a different part of the story. The consequences are significant and the GM wants to refine this before sending.',
      };
      setupDefaultHandlers([longResult1, longResult2]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      // Wait for both collapsed results
      await waitFor(() => {
        const buttons = screen.getAllByText(/show full content/i);
        expect(buttons).toHaveLength(2);
      });

      // Expand one result and assert the other is unaffected. This view sorts
      // newest first, so the topmost card is longResult2 (higher id) — hence
      // clicking index 0 expands that one, not longResult1.
      const expandButtons = screen.getAllByText(/show full content/i);
      await user.click(expandButtons[0]);

      // The clicked result expands; the other stays collapsed.
      await waitFor(() => {
        expect(screen.getByText(longResult2.content)).toBeInTheDocument();
        expect(screen.queryByText(longResult1.content)).not.toBeInTheDocument();
        expect(screen.getByText(/show less/i)).toBeInTheDocument();
        expect(screen.getByText(/show full content/i)).toBeInTheDocument(); // The other is still collapsed
      });
    });
  });

  describe('Ordering', () => {
    // This is the composing view: a GM who just wrote a result needs it at the
    // top to confirm it landed and to edit it. The endpoint returns oldest-first
    // so the History tab reads chronologically for every role, so the order is
    // reversed here — the two views share one query and want opposite orders.
    it('shows the newest result first, independent of the order the API returns', async () => {
      const older: ActionResult = {
        ...mockUnpublishedResult,
        id: 1,
        content: 'Older result written first.',
      };
      const newer: ActionResult = {
        ...mockUnpublishedResult,
        id: 2,
        content: 'Newer result written second.',
      };
      // Ascending, matching what GetGameResults actually sends.
      setupDefaultHandlers([older, newer]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByText(newer.content)).toBeInTheDocument();
      });

      const rendered = screen.getAllByText(/result written/i).map(el => el.textContent);
      expect(rendered).toEqual([newer.content, older.content]);
    });
  });

  describe('Delete Functionality', () => {
    it('shows a Delete button for unpublished results', async () => {
      setupDefaultHandlers([mockUnpublishedResult]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /delete/i })).toBeInTheDocument();
      });
    });

    it('does not show Delete button for published results', async () => {
      setupDefaultHandlers([mockPublishedResult]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByText('This is a published result that was sent')).toBeInTheDocument();
      });

      expect(screen.queryByRole('button', { name: /delete/i })).not.toBeInTheDocument();
    });

    it('shows confirmation dialog when Delete is clicked', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers([mockUnpublishedResult]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /delete/i })).toBeInTheDocument();
      });

      await user.click(screen.getByRole('button', { name: /delete/i }));

      expect(screen.getByText('Delete Draft Result')).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /yes, delete draft/i })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /cancel/i })).toBeInTheDocument();
    });

    it('hides confirmation dialog when Cancel is clicked', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers([mockUnpublishedResult]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /delete/i })).toBeInTheDocument();
      });

      await user.click(screen.getByRole('button', { name: /delete/i }));
      expect(screen.getByText('Delete Draft Result')).toBeInTheDocument();

      await user.click(screen.getByRole('button', { name: /cancel/i }));
      expect(screen.queryByText('Delete Draft Result')).not.toBeInTheDocument();
    });

    it('calls delete API and removes result when confirmed', async () => {
      const user = userEvent.setup();
      let deleteCalled = false;

      server.use(
        http.get('/api/v1/games/:gameId/results', () => {
          return HttpResponse.json([mockUnpublishedResult]);
        }),
        http.delete('/api/v1/games/:gameId/results/:resultId', () => {
          deleteCalled = true;
          return new HttpResponse(null, { status: 204 });
        })
      );

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /delete/i })).toBeInTheDocument();
      });

      await user.click(screen.getByRole('button', { name: /delete/i }));
      await user.click(screen.getByRole('button', { name: /yes, delete draft/i }));

      await waitFor(() => {
        expect(deleteCalled).toBe(true);
      });
    });

    it('leaves result in list when delete fails', async () => {
      const user = userEvent.setup();

      server.use(
        http.get('/api/v1/games/:gameId/results', () => {
          return HttpResponse.json([mockUnpublishedResult]);
        }),
        http.delete('/api/v1/games/:gameId/results/:resultId', () => {
          return HttpResponse.json({ error: 'Failed to delete' }, { status: 500 });
        })
      );

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /delete/i })).toBeInTheDocument();
      });

      await user.click(screen.getByRole('button', { name: /delete/i }));
      await user.click(screen.getByRole('button', { name: /yes, delete draft/i }));

      await waitFor(() => {
        expect(screen.getByText('This is an unpublished result for the player')).toBeInTheDocument();
      });
    });
  });

  describe('Conflicting character sheet drafts', () => {
    // Two unpublished results in the same phase for the SAME character. Each draft
    // sheet update is a whole-sheet snapshot seeded from published character data,
    // so publishing both silently overwrites the earlier one.
    const resultA: ActionResult = {
      ...mockUnpublishedResult,
      id: 10,
      character_id: 500,
      content: 'You find Item A',
    };
    const resultB: ActionResult = {
      ...mockUnpublishedResult,
      id: 11,
      character_id: 500,
      content: 'You find Item B',
    };
    // Same phase, different character — must NOT warn.
    const otherCharacterResult: ActionResult = {
      ...mockUnpublishedResult,
      id: 12,
      character_id: 501,
      content: 'A result for someone else',
    };

    /** Mock per-result draft counts; results absent from the map report zero drafts. */
    const setupDraftCounts = (counts: Record<number, number>) => {
      server.use(
        http.get('/api/v1/games/:gameId/results/:resultId/character-updates/count', ({ params }) => {
          const resultId = Number(params.resultId);
          return HttpResponse.json({ count: counts[resultId] ?? 0 });
        })
      );
    };

    it('warns on both results when two unpublished results stage sheet updates for one character', async () => {
      setupDefaultHandlers([resultA, resultB]);
      setupDraftCounts({ 10: 1, 11: 1 });

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByTestId('sheet-conflict-warning-10')).toBeInTheDocument();
      });
      expect(screen.getByTestId('sheet-conflict-warning-11')).toBeInTheDocument();
    });

    it('warns while a sibling draft count is still in flight', async () => {
      // Regression: a pending sibling count used to read as "no conflict", so a GM who
      // clicked straight through to Publish before the counts resolved got no warning
      // and clobbered the sibling's snapshot. Pending must count as a possible conflict,
      // matching how the local count is already hardened.
      setupDefaultHandlers([resultA, resultB]);
      server.use(
        http.get(
          '/api/v1/games/:gameId/results/:resultId/character-updates/count',
          async ({ params }) => {
            // Result 10 resolves; its sibling 11 never does.
            if (Number(params.resultId) === 11) {
              await new Promise(() => {});
            }
            return HttpResponse.json({ count: 1 });
          }
        )
      );

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByTestId('sheet-conflict-warning-10')).toBeInTheDocument();
      });
    });

    it('does not warn when only one result has staged sheet updates', async () => {
      setupDefaultHandlers([resultA, resultB]);
      setupDraftCounts({ 10: 2 });

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      // Wait on the draft-count badge, not the result content: content renders before
      // the count queries resolve, and an absence assertion passes on the first tick.
      // The badge only appears once a count has actually settled.
      await waitFor(() => {
        expect(screen.getByTestId('draft-update-count-10')).toHaveTextContent('2');
      });

      expect(screen.queryByTestId('sheet-conflict-warning-10')).not.toBeInTheDocument();
      expect(screen.queryByTestId('sheet-conflict-warning-11')).not.toBeInTheDocument();
    });

    it('warns only on the result that has staged updates, not its empty sibling', async () => {
      // Result 11 has no staged updates, so it can neither overwrite nor be
      // overwritten — warning there would be noise.
      setupDefaultHandlers([resultA, resultB]);
      setupDraftCounts({ 10: 1 });

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      // Result 10's badge proves the counts resolved; only then is 11's lack of a
      // warning meaningful rather than merely early.
      await waitFor(() => {
        expect(screen.getByTestId('draft-update-count-10')).toHaveTextContent('1');
      });

      expect(screen.queryByTestId('sheet-conflict-warning-11')).not.toBeInTheDocument();
    });

    it('does not warn when the staged updates target different characters', async () => {
      setupDefaultHandlers([resultA, otherCharacterResult]);
      setupDraftCounts({ 10: 1, 12: 1 });

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      // Both counts must have settled before absence proves anything.
      await waitFor(() => {
        expect(screen.getByTestId('draft-update-count-10')).toHaveTextContent('1');
        expect(screen.getByTestId('draft-update-count-12')).toHaveTextContent('1');
      });

      expect(screen.queryByTestId('sheet-conflict-warning-10')).not.toBeInTheDocument();
      expect(screen.queryByTestId('sheet-conflict-warning-12')).not.toBeInTheDocument();
    });

    it('does not warn when the sibling result is already published', async () => {
      // A published result's drafts have already been applied and deleted, so it
      // can no longer clobber anything.
      const publishedSibling: ActionResult = { ...resultB, is_published: true };
      setupDefaultHandlers([resultA, publishedSibling]);
      setupDraftCounts({ 10: 1, 11: 1 });

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByTestId('draft-update-count-10')).toHaveTextContent('1');
      });

      expect(screen.queryByTestId('sheet-conflict-warning-10')).not.toBeInTheDocument();
    });

    it('surfaces the conflict in the publish confirmation dialog', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers([resultA, resultB]);
      setupDraftCounts({ 10: 1, 11: 1 });
      server.use(
        http.get('/api/v1/games/:gameId/results/:resultId/character-updates', () =>
          HttpResponse.json([])
        )
      );

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByTestId('sheet-conflict-warning-10')).toBeInTheDocument();
      });

      const publishButtons = await screen.findAllByRole('button', { name: /publish result/i });
      await user.click(publishButtons[0]);

      await waitFor(() => {
        expect(screen.getByTestId('publish-sheet-conflict-warning')).toBeInTheDocument();
      });
    });
  });

  describe('Integration', () => {
    it('handles complete edit workflow', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers([mockUnpublishedResult]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      // Wait for results to load
      await waitFor(() => {
        expect(screen.getByText('This is an unpublished result for the player')).toBeInTheDocument();
      });

      // Click Edit
      await user.click(screen.getByRole('button', { name: /edit/i }));

      // Modify content
      const textarea = screen.getByRole('textbox');
      await user.clear(textarea);
      await user.type(textarea, 'Completely new content');

      // Save changes
      await user.click(screen.getByRole('button', { name: /save changes/i }));

      // Verify edit form is closed
      await waitFor(() => {
        expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
      });
    });

    it('handles editing multiple results sequentially', async () => {
      const user = userEvent.setup();
      setupDefaultHandlers([mockUnpublishedResult, mockUnpublishedResult2]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getAllByRole('button', { name: /edit/i })).toHaveLength(2);
      });

      // Edit first result
      const editButtons = screen.getAllByRole('button', { name: /edit/i });
      await user.click(editButtons[0]);

      let textarea = screen.getByRole('textbox');
      await user.clear(textarea);
      await user.type(textarea, 'Updated first result');

      await user.click(screen.getByRole('button', { name: /save changes/i }));

      await waitFor(() => {
        expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
      });

      // Edit second result
      const newEditButtons = screen.getAllByRole('button', { name: /edit/i });
      await user.click(newEditButtons[1]);

      textarea = screen.getByRole('textbox');
      await user.clear(textarea);
      await user.type(textarea, 'Updated second result');

      await user.click(screen.getByRole('button', { name: /save changes/i }));

      await waitFor(() => {
        expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
      });
    });

    it('displays both unpublished and published sections together', async () => {
      setupDefaultHandlers([
        mockUnpublishedResult,
        mockUnpublishedResult2,
        mockPublishedResult,
      ]);

      renderWithProviders(<GameResultsManager gameId={mockGameId} />);

      await waitFor(() => {
        expect(screen.getByText('Unpublished Results (Editable)')).toBeInTheDocument();
        expect(screen.getByText('Published Results')).toBeInTheDocument();
        expect(screen.getByText('This is an unpublished result for the player')).toBeInTheDocument();
        expect(screen.getByText('Another unpublished draft result')).toBeInTheDocument();
        expect(screen.getByText('This is a published result that was sent')).toBeInTheDocument();
      });
    });
  });
});
