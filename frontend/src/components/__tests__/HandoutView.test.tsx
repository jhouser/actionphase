import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server';
import { renderWithProviders } from '../../test-utils/render';
import { HandoutView } from '../HandoutView';
import type { Handout, HandoutComment } from '../../types/handouts';

const mockHandout: Handout = {
  id: 1,
  game_id: 1,
  title: 'Test Handout',
  content: '# Test Content\n\nThis is a test handout.',
  status: 'published',
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T12:00:00Z',
};

const mockComments: HandoutComment[] = [
  {
    id: 1,
    handout_id: 1,
    user_id: 1,
    content: 'First comment',
    edit_count: 0,
    created_at: '2024-01-01T10:00:00Z',
    updated_at: '2024-01-01T10:00:00Z',
    edited_at: null,
    deleted_at: null,
    deleted_by_user_id: null,
  },
  {
    id: 2,
    handout_id: 1,
    user_id: 1,
    content: 'Edited comment',
    edit_count: 2,
    created_at: '2024-01-01T11:00:00Z',
    updated_at: '2024-01-01T12:00:00Z',
    edited_at: '2024-01-01T12:00:00Z',
    deleted_at: null,
    deleted_by_user_id: null,
  },
  {
    id: 3,
    handout_id: 1,
    user_id: 1,
    content: 'Deleted comment',
    edit_count: 0,
    created_at: '2024-01-01T09:00:00Z',
    updated_at: '2024-01-01T09:00:00Z',
    edited_at: null,
    deleted_at: '2024-01-01T13:00:00Z',
    deleted_by_user_id: 1,
  },
];

describe('HandoutView', () => {
  const mockOnClose = vi.fn();
  const mockOnEdit = vi.fn();

  const setupDefaultHandlers = () => {
    // Track updated comments
    let updatedCommentContent: string | null = null;

    server.use(
      http.get('/api/v1/games/:gameId/handouts/:handoutId/comments', () => {
        // If a comment was updated, return the updated version
        if (updatedCommentContent) {
          const updatedComments = mockComments.map(c =>
            c.id === 1 ? { ...c, content: updatedCommentContent, edit_count: 1 } : c
          );
          return HttpResponse.json(updatedComments);
        }
        return HttpResponse.json(mockComments);
      }),
      http.post('/api/v1/games/:gameId/handouts/:handoutId/comments', async ({ request }) => {
        const body = await request.json() as { content: string };
        return HttpResponse.json({
          id: 4,
          handout_id: 1,
          user_id: 1,
          content: body.content,
          edit_count: 0,
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
          edited_at: null,
          deleted_at: null,
          deleted_by_user_id: null,
        });
      }),
      http.patch('/api/v1/games/:gameId/handouts/:handoutId/comments/:commentId', async ({ request }) => {
        const body = await request.json() as { content: string };
        // Store the updated content for subsequent GET requests
        updatedCommentContent = body.content;
        return HttpResponse.json({
          id: 1,
          handout_id: 1,
          user_id: 1,
          content: body.content,
          edit_count: 1,
          created_at: '2024-01-01T10:00:00Z',
          updated_at: new Date().toISOString(),
          edited_at: new Date().toISOString(),
          deleted_at: null,
          deleted_by_user_id: null,
        });
      }),
      http.delete('/api/v1/games/:gameId/handouts/:handoutId/comments/:commentId', () => {
        return HttpResponse.json(null, { status: 204 });
      })
    );
  };

  beforeEach(() => {
    server.resetHandlers();
    setupDefaultHandlers();
    mockOnClose.mockClear();
    mockOnEdit.mockClear();
  });

  describe('GM Authorization', () => {
    it('shows edit and delete buttons for GM', async () => {
      renderWithProviders(
        <HandoutView
          gameId={1}
          handout={mockHandout}
          isGM={true}
          onClose={mockOnClose}
          onEdit={mockOnEdit}
        />
      );

      // Wait for comments to load
      await waitFor(() => {
        expect(screen.getByText('First comment')).toBeInTheDocument();
      });

      // Check for edit buttons (should have at least 2, one for each visible comment)
      const editButtons = screen.getAllByRole('button', { name: /^edit$/i });
      expect(editButtons.length).toBeGreaterThanOrEqual(2);

      // Check for delete buttons
      const deleteButtons = screen.getAllByRole('button', { name: /delete/i });
      expect(deleteButtons.length).toBeGreaterThanOrEqual(2);
    });

    it('does not show edit and delete buttons for non-GM', async () => {
      renderWithProviders(
        <HandoutView
          gameId={1}
          handout={mockHandout}
          isGM={false}
          onClose={mockOnClose}
        />
      );

      // Wait for comments to load
      await waitFor(() => {
        expect(screen.getByText('First comment')).toBeInTheDocument();
      });

      // Should not find edit or delete buttons
      expect(screen.queryByRole('button', { name: /^edit$/i })).not.toBeInTheDocument();
      expect(screen.queryByRole('button', { name: /delete/i })).not.toBeInTheDocument();
    });
  });

  describe('Edit Mode', () => {
    it('shows inline editor when edit button clicked', async () => {
      const user = userEvent.setup();

      renderWithProviders(
        <HandoutView
          gameId={1}
          handout={mockHandout}
          isGM={true}
          onClose={mockOnClose}
          onEdit={mockOnEdit}
        />
      );

      // Wait for comments to load
      await screen.findByText('First comment');

      // Find edit buttons using text search
      const editButtons = screen.getAllByText('Edit');
      // Filter to only ghost buttons in comment sections (not the top handout edit button)
      const commentEditButtons = editButtons.filter(btn =>
        btn.closest('.bg-bg-secondary') !== null
      );

      await user.click(commentEditButtons[0]);

      // Should show Save and Cancel buttons (indicates edit mode is active)
      expect(await screen.findByText('Save')).toBeInTheDocument();
      expect(await screen.findByText('Cancel')).toBeInTheDocument();
    });

    it('reverts to view mode when cancel button clicked', async () => {
      const user = userEvent.setup();

      renderWithProviders(
        <HandoutView
          gameId={1}
          handout={mockHandout}
          isGM={true}
          onClose={mockOnClose}
          onEdit={mockOnEdit}
        />
      );

      // Wait for comments to load
      await screen.findByText('First comment');

      const editButtons = screen.getAllByText('Edit');
      const commentEditButtons = editButtons.filter(btn =>
        btn.closest('.bg-bg-secondary') !== null
      );
      const initialEditButtonCount = commentEditButtons.length;
      await user.click(commentEditButtons[0]);

      // Wait for edit mode
      const saveButton = await screen.findByText('Save');
      const cancelButton = await screen.findByText('Cancel');
      expect(saveButton).toBeInTheDocument();
      expect(cancelButton).toBeInTheDocument();

      // Click cancel
      await user.click(cancelButton);

      // Should go back to view mode (Save/Cancel buttons gone, edit buttons return)
      await waitFor(() => {
        expect(screen.queryByText('Save')).not.toBeInTheDocument();
        expect(screen.queryByText('Cancel')).not.toBeInTheDocument();
      });

      // Edit buttons should be back
      const editButtonsAfter = screen.getAllByText('Edit').filter(btn =>
        btn.closest('.bg-bg-secondary') !== null
      );
      expect(editButtonsAfter.length).toBe(initialEditButtonCount);
    });

    it('updates comment when save button clicked', async () => {
      const user = userEvent.setup();

      renderWithProviders(
        <HandoutView
          gameId={1}
          handout={mockHandout}
          isGM={true}
          onClose={mockOnClose}
          onEdit={mockOnEdit}
        />
      );

      // Wait for comments to load
      await screen.findByText('First comment');

      // Click edit on first comment
      const editButtons = screen.getAllByText('Edit');
      const commentEditButtons = editButtons.filter(btn =>
        btn.closest('.bg-bg-secondary') !== null
      );
      await user.click(commentEditButtons[0]);

      // Wait for edit mode (Save button appears)
      const saveButton = await screen.findByText('Save');
      expect(saveButton).toBeInTheDocument();

      // Find all textareas and get the last one (the edit textarea, not the new comment form)
      const textareas = screen.getAllByRole('textbox');
      const editTextarea = textareas[textareas.length - 1] as HTMLTextAreaElement;

      // Modify the content
      await user.clear(editTextarea);
      await user.type(editTextarea, 'Updated comment content');

      // Click save
      await user.click(saveButton);

      // Should return to view mode (Save button disappears)
      await waitFor(() => {
        expect(screen.queryByText('Save')).not.toBeInTheDocument();
      });

      // Comment should be updated (verify API was called by checking the mock response appears)
      await waitFor(() => {
        expect(screen.getByText('Updated comment content')).toBeInTheDocument();
      });
    });
  });

  describe('Delete Confirmation', () => {
    it('shows confirmation modal when delete button clicked', async () => {
      const user = userEvent.setup();

      renderWithProviders(
        <HandoutView
          gameId={1}
          handout={mockHandout}
          isGM={true}
          onClose={mockOnClose}
          onEdit={mockOnEdit}
        />
      );

      // Wait for comments to load
      await waitFor(() => {
        expect(screen.getByText('First comment')).toBeInTheDocument();
      });

      // Click delete on first comment
      const deleteButtons = screen.getAllByRole('button', { name: /delete/i });
      await user.click(deleteButtons[0]);

      // Should show confirmation modal
      await waitFor(() => {
        expect(screen.getByText('Delete Update')).toBeInTheDocument();
        expect(screen.getByText(/are you sure you want to delete this update/i)).toBeInTheDocument();
      });
    });

    it('deletes comment when deletion confirmed', async () => {
      const user = userEvent.setup();

      renderWithProviders(
        <HandoutView
          gameId={1}
          handout={mockHandout}
          isGM={true}
          onClose={mockOnClose}
          onEdit={mockOnEdit}
        />
      );

      // Wait for comments to load
      await waitFor(() => {
        expect(screen.getByText('First comment')).toBeInTheDocument();
      });

      // Get all buttons before opening modal
      const deleteButtons = screen.getAllByRole('button', { name: /delete/i });
      await user.click(deleteButtons[0]);

      // Wait for modal to appear
      await waitFor(() => {
        expect(screen.getByText('Delete Update')).toBeInTheDocument();
        expect(screen.getByText(/are you sure you want to delete this update/i)).toBeInTheDocument();
      });

      // Now there will be more Delete buttons (one in modal), get them all again
      const allDeleteButtons = screen.getAllByRole('button', { name: /^delete$/i });
      // The last one should be the modal's delete button
      const modalDeleteButton = allDeleteButtons[allDeleteButtons.length - 1];
      await user.click(modalDeleteButton);

      // Modal should close
      await waitFor(() => {
        expect(screen.queryByText('Delete Update')).not.toBeInTheDocument();
      });

      // Verify the modal closed (the API mock returned 204, query will refetch)
      expect(screen.queryByText('Delete Update')).not.toBeInTheDocument();
    });

    it('does not delete when modal is cancelled', async () => {
      const user = userEvent.setup();

      renderWithProviders(
        <HandoutView
          gameId={1}
          handout={mockHandout}
          isGM={true}
          onClose={mockOnClose}
          onEdit={mockOnEdit}
        />
      );

      // Wait for comments to load
      await waitFor(() => {
        expect(screen.getByText('First comment')).toBeInTheDocument();
      });

      // Click delete
      const deleteButtons = screen.getAllByRole('button', { name: /delete/i });
      await user.click(deleteButtons[0]);

      // Wait for modal
      await waitFor(() => {
        expect(screen.getByText('Delete Update')).toBeInTheDocument();
      });

      // Click cancel
      const cancelButton = screen.getByRole('button', { name: /cancel/i });
      await user.click(cancelButton);

      // Modal should close
      await waitFor(() => {
        expect(screen.queryByText('Delete Update')).not.toBeInTheDocument();
      });

      // Comment should still be visible
      expect(screen.getByText('First comment')).toBeInTheDocument();
    });
  });

  describe('Edit Indicator', () => {
    it('shows "edited at" timestamp for edited comments instead of "Edited N times" badge', async () => {
      // Bug: edited comments showed creation timestamp + useless "Edited N times" badge
      // Fix: show "Edited [updated_at]" timestamp, drop the badge
      renderWithProviders(
        <HandoutView
          gameId={1}
          handout={mockHandout}
          isGM={true}
          onClose={mockOnClose}
          onEdit={mockOnEdit}
        />
      );

      await waitFor(() => {
        expect(screen.getByText('Edited comment')).toBeInTheDocument();
      });

      // Should NOT show the useless "Edited N times" badge
      expect(screen.queryByText(/Edited \d+ times?/i)).not.toBeInTheDocument();

      // Should show the "edited at" timestamp for the edited comment (edit_count: 2, updated_at: '2024-01-01T12:00:00Z')
      // The timestamp text starts with "Edited " followed by the formatted date
      const editedTimestamps = screen.getAllByText(/^Edited /i);
      expect(editedTimestamps.length).toBeGreaterThanOrEqual(1);
    });

    it('shows creation timestamp for unedited comments', async () => {
      renderWithProviders(
        <HandoutView
          gameId={1}
          handout={mockHandout}
          isGM={true}
          onClose={mockOnClose}
          onEdit={mockOnEdit}
        />
      );

      await waitFor(() => {
        expect(screen.getByText('First comment')).toBeInTheDocument();
      });

      // Unedited comment (edit_count: 0) should show creation timestamp, not "Edited"
      // The creation timestamp for comment 1 is '2024-01-01T10:00:00Z'
      const createdAt = new Date('2024-01-01T10:00:00Z').toLocaleString();
      expect(screen.getByText(createdAt)).toBeInTheDocument();
    });

    it('does not show "Edited" timestamp for unedited comments', async () => {
      renderWithProviders(
        <HandoutView
          gameId={1}
          handout={mockHandout}
          isGM={false}
          onClose={mockOnClose}
        />
      );

      await waitFor(() => {
        expect(screen.getByText('First comment')).toBeInTheDocument();
      });

      // The unedited first comment should not show any "Edited" text in its timestamp area
      // (The handout itself doesn't show "Edited" either since updated_at != created_at for mockHandout
      // — but here we test comment-level edit indicators)
      expect(screen.queryByText(/Edited \d+ times?/i)).not.toBeInTheDocument();
    });
  });

  describe('Handout Edited Timestamp', () => {
    it('does not show edited timestamp when updated_at equals created_at', async () => {
      const unedited = { ...mockHandout, created_at: '2024-01-01T00:00:00Z', updated_at: '2024-01-01T00:00:00Z' };

      renderWithProviders(
        <HandoutView
          gameId={1}
          handout={unedited}
          isGM={false}
          onClose={mockOnClose}
        />
      );

      await waitFor(() => {
        expect(screen.getByText('Test Handout')).toBeInTheDocument();
      });

      expect(screen.queryByText(/^edited /i)).not.toBeInTheDocument();
    });

    it('shows edited timestamp when updated_at differs from created_at', async () => {
      const editedHandout = { ...mockHandout, created_at: '2024-01-01T00:00:00Z', updated_at: '2024-03-01T15:00:00Z' };

      renderWithProviders(
        <HandoutView
          gameId={1}
          handout={editedHandout}
          isGM={false}
          onClose={mockOnClose}
        />
      );

      await waitFor(() => {
        expect(screen.getByText('Test Handout')).toBeInTheDocument();
      });

      expect(screen.getByText(/^edited /i)).toBeInTheDocument();
    });
  });

  describe('Deleted Comments Filtering', () => {
    it('filters out deleted comments from display', async () => {
      renderWithProviders(
        <HandoutView
          gameId={1}
          handout={mockHandout}
          isGM={true}
          onClose={mockOnClose}
          onEdit={mockOnEdit}
        />
      );

      // Wait for comments to load
      await waitFor(() => {
        expect(screen.getByText('First comment')).toBeInTheDocument();
      });

      // Should show non-deleted comments
      expect(screen.getByText('First comment')).toBeInTheDocument();
      expect(screen.getByText('Edited comment')).toBeInTheDocument();

      // Should NOT show deleted comment
      expect(screen.queryByText('Deleted comment')).not.toBeInTheDocument();
    });
  });

  /**
   * The Utility Drawer renders this inside a Modal that supplies its own title
   * bar and X, so the tab's back-link and duplicate heading are turned off
   * there. The Handouts tab, which has no such chrome, keeps both.
   */
  describe('showHeader', () => {
    it('shows the back link and title heading by default', async () => {
      renderWithProviders(
        <HandoutView
          gameId={1}
          handout={mockHandout}
          isGM={false}
          onClose={mockOnClose}
        />
      );

      expect(
        await screen.findByRole('button', { name: /back to handouts/i })
      ).toBeInTheDocument();
      expect(
        screen.getByRole('heading', { name: 'Test Handout' })
      ).toBeInTheDocument();
    });

    it('hides the back link and title heading when showHeader is false', async () => {
      renderWithProviders(
        <HandoutView
          gameId={1}
          handout={mockHandout}
          isGM={false}
          onClose={mockOnClose}
          showHeader={false}
        />
      );

      // The content still renders, so this is not just an empty tree.
      await waitFor(() => {
        expect(screen.getByText('Updates & Comments')).toBeInTheDocument();
      });

      expect(
        screen.queryByRole('button', { name: /back to handouts/i })
      ).not.toBeInTheDocument();
      expect(
        screen.queryByRole('heading', { name: 'Test Handout' })
      ).not.toBeInTheDocument();
      // The draft/published badge is not part of the modal's title bar, so it
      // must survive.
      expect(screen.getByText('Published')).toBeInTheDocument();
    });

    it('still offers the GM Edit button with the header hidden', async () => {
      renderWithProviders(
        <HandoutView
          gameId={1}
          handout={mockHandout}
          isGM={true}
          onClose={mockOnClose}
          onEdit={mockOnEdit}
          showHeader={false}
        />
      );

      const user = userEvent.setup();
      const editButton = await screen.findByRole('button', { name: /^edit$/i });
      await user.click(editButton);
      expect(mockOnEdit).toHaveBeenCalled();
    });
  });

  /**
   * The Utility Drawer is reference material you pull up mid-action-submission,
   * so its modal must not offer *any* way to change things — not the handout,
   * and not the updates on it. Half-writable (updates but not the handout) was
   * the inconsistency this mode exists to remove.
   */
  describe('readOnly', () => {
    it('hides every GM write affordance from a GM', async () => {
      renderWithProviders(
        <HandoutView
          gameId={1}
          handout={mockHandout}
          isGM={true}
          onClose={mockOnClose}
          onEdit={mockOnEdit}
          readOnly
        />
      );

      // The updates themselves still render — this is read-only, not hidden.
      await waitFor(() => {
        expect(screen.getByText('First comment')).toBeInTheDocument();
      });

      // No composer for a new update.
      expect(
        screen.queryByPlaceholderText(/add an update or clarification/i)
      ).not.toBeInTheDocument();
      expect(
        screen.queryByRole('button', { name: /post update/i })
      ).not.toBeInTheDocument();

      // No editing or deleting existing updates, and no editing the handout.
      expect(screen.queryByRole('button', { name: /^edit$/i })).not.toBeInTheDocument();
      expect(screen.queryByRole('button', { name: /delete/i })).not.toBeInTheDocument();
    });

    it('keeps those affordances for a GM when not read-only', async () => {
      renderWithProviders(
        <HandoutView
          gameId={1}
          handout={mockHandout}
          isGM={true}
          onClose={mockOnClose}
          onEdit={mockOnEdit}
        />
      );

      await waitFor(() => {
        expect(screen.getByText('First comment')).toBeInTheDocument();
      });

      // Guards against the read-only assertions above passing for the wrong
      // reason (e.g. a renamed button that never renders in either mode).
      expect(
        screen.getByRole('button', { name: /post update/i })
      ).toBeInTheDocument();
      expect(
        screen.getAllByRole('button', { name: /^edit$/i }).length
      ).toBeGreaterThan(0);
      expect(
        screen.getAllByRole('button', { name: /delete/i }).length
      ).toBeGreaterThan(0);
    });

    it('reads as the player-facing view for a read-only GM', async () => {
      renderWithProviders(
        <HandoutView
          gameId={1}
          handout={mockHandout}
          isGM={true}
          onClose={mockOnClose}
          readOnly
        />
      );

      // Wording that offers to add updates would be a lie here.
      expect(
        await screen.findByText(/gm updates and additional information/i)
      ).toBeInTheDocument();
      expect(
        screen.queryByText(/add updates, clarifications/i)
      ).not.toBeInTheDocument();
    });
  });

});
