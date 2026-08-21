import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server';
import { renderWithProviders } from '../../test-utils/render';
import { ThreadViewModal } from '../ThreadViewModal';
import type { Message } from '../../types/messages';
import type { Character } from '../../types/characters';

describe('ThreadViewModal', () => {
  const mockGameId = 1;
  const mockPostId = 100;
  const mockOnClose = vi.fn();
  const mockOnCreateReply = vi.fn();
  const mockCurrentUserId = 200;

  const mockCharacters: Character[] = [
    {
      id: 1,
      game_id: mockGameId,
      user_id: mockCurrentUserId,
      username: 'testuser',
      name: 'Hero',
      character_type: 'player_character',
      status: 'active',
      created_at: '2025-01-01T00:00:00Z',
      updated_at: '2025-01-01T00:00:00Z',
    },
  ];

  const mockComment: Message = {
    id: 1,
    game_id: mockGameId,
    author_id: mockCurrentUserId,
    character_id: 1,
    content: 'This is a test comment in modal',
    message_type: 'comment',
    thread_depth: 0,
    author_username: 'testuser',
    character_name: 'Hero',
    reply_count: 0,
    is_edited: false,
    is_deleted: false,
    created_at: '2025-01-15T10:30:00Z',
    updated_at: '2025-01-15T10:30:00Z',
  };

  beforeEach(() => {
    mockOnClose.mockClear();
    mockOnCreateReply.mockClear();
    // Setup MSW handlers for API calls made by components
    server.use(
      http.get('/api/v1/games/:gameId/details', () => {
        return HttpResponse.json({
          id: 1,
          title: 'Test Game',
          state: 'in_progress',
        });
      }),
      http.get('/api/v1/games/:gameId/participants', () => {
        return HttpResponse.json([]);
      })
    );
  });

  describe('Read-Only Mode', () => {
    it('should pass readOnly prop to ThreadedComment', async () => {
      renderWithProviders(
        <ThreadViewModal
          gameId={mockGameId}
          postId={mockPostId}
          comment={mockComment}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onClose={mockOnClose}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
          readOnly={true}
        />
      );

      // Comment content should be visible
      expect(screen.getByText('This is a test comment in modal')).toBeInTheDocument();

      // Edit/delete/reply buttons should NOT be visible (readOnly propagated to ThreadedComment)
      expect(screen.queryByRole('button', { name: /edit/i })).not.toBeInTheDocument();
      expect(screen.queryByRole('button', { name: /delete/i })).not.toBeInTheDocument();
      expect(screen.queryByRole('button', { name: /reply/i })).not.toBeInTheDocument();
    });

    it('should allow interactions when readOnly=false', async () => {
      renderWithProviders(
        <ThreadViewModal
          gameId={mockGameId}
          postId={mockPostId}
          comment={mockComment}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onClose={mockOnClose}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
          readOnly={false}
        />
      );

      // Comment content should be visible
      expect(screen.getByText('This is a test comment in modal')).toBeInTheDocument();

      // Reply button SHOULD be visible (readOnly=false allows interactions)
      expect(screen.getByRole('button', { name: /reply/i })).toBeInTheDocument();
    });

    it('should propagate readOnly to parent chain comments', async () => {
      // Create a parent chain (3 levels deep)
      const parentComment1: Message = {
        ...mockComment,
        id: 10,
        content: 'Parent comment 1',
        thread_depth: 0,
      };

      const parentComment2: Message = {
        ...mockComment,
        id: 11,
        parent_id: 10,
        content: 'Parent comment 2',
        thread_depth: 1,
      };

      const targetComment: Message = {
        ...mockComment,
        id: 12,
        parent_id: 11,
        content: 'Target comment (deepest)',
        thread_depth: 2,
      };

      renderWithProviders(
        <ThreadViewModal
          gameId={mockGameId}
          postId={mockPostId}
          comment={targetComment}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onClose={mockOnClose}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
          parentChain={[parentComment1, parentComment2, targetComment]}
          hasFullThread={true}
          targetCommentId={12}
          readOnly={true}
        />
      );

      // All comments should be visible
      expect(screen.queryAllByText('Parent comment 1').length).toBeGreaterThanOrEqual(1);
      expect(screen.queryAllByText('Parent comment 2').length).toBeGreaterThanOrEqual(1);
      expect(screen.queryAllByText('Target comment (deepest)').length).toBeGreaterThanOrEqual(1);

      // No "Reply" action buttons should be visible (readOnly=true)
      // Note: This looks for the "Reply" button to create a new reply, not the collapse buttons
      const replyActionButtons = screen.queryAllByRole('button', { name: /^reply$/i });
      expect(replyActionButtons).toHaveLength(0);
    });

    it('should default readOnly to false when not provided', async () => {
      renderWithProviders(
        <ThreadViewModal
          gameId={mockGameId}
          postId={mockPostId}
          comment={mockComment}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onClose={mockOnClose}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
          // readOnly not provided - should default to false
        />
      );

      // Reply button should be visible (readOnly defaults to false)
      expect(screen.getByRole('button', { name: /reply/i })).toBeInTheDocument();
    });
  });

  describe('Unsaved reply protection', () => {
    // A comment with a pre-loaded child — this is the scenario that was broken:
    // the user types a reply on the NESTED comment, not the root. The dirty state
    // must propagate up through onDirtyStateChange to ThreadViewModal.
    const mockChildComment: Message = {
      id: 2,
      game_id: mockGameId,
      author_id: 999, // different user — gives us a reply button
      character_id: 2,
      content: 'A nested child comment',
      message_type: 'comment',
      thread_depth: 1,
      author_username: 'otheruser',
      character_name: 'Sidekick',
      reply_count: 0,
      is_edited: false,
      is_deleted: false,
      created_at: '2025-01-15T11:00:00Z',
      updated_at: '2025-01-15T11:00:00Z',
    };

    const mockCommentWithChild = {
      ...mockComment,
      reply_count: 1,
      children: [mockChildComment],
    };

    it('should show confirmation when closed with pending reply on a nested comment', async () => {
      // This is the actual failure case: dirty state originates from a child
      // ThreadedComment (id=2), not the root (id=1). Before the fix, the root's
      // onDirtyStateChange was not propagated to children, so hasDirtyReply stayed false.
      const user = userEvent.setup();
      renderWithProviders(
        <ThreadViewModal
          gameId={mockGameId}
          postId={mockPostId}
          comment={mockCommentWithChild}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onClose={mockOnClose}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      // Open the reply form on the nested child comment
      const replyButtons = screen.getAllByRole('button', { name: /reply to this comment/i });
      // Last reply button belongs to the deepest (child) comment
      await user.click(replyButtons[replyButtons.length - 1]);

      const textarea = screen.getByPlaceholderText('Write a reply...');
      await user.type(textarea, 'Half-written reply text');

      // Close via the X — should NOT close immediately
      await user.click(screen.getByRole('button', { name: /close thread view/i }));

      expect(mockOnClose).not.toHaveBeenCalled();
      expect(screen.getByText(/discard unsaved reply/i)).toBeInTheDocument();
    });

    it('should ignore a backdrop click entirely while a reply is pending', async () => {
      // A backdrop click is usually a slip rather than a decision, so once there is
      // something to lose it does nothing at all — matching the character sheet.
      // Asserting "still open" alone would also pass against the old behaviour, where
      // the backdrop raised the confirm dialog; the absence of the dialog is what
      // distinguishes inert from confirming.
      const user = userEvent.setup();
      renderWithProviders(
        <ThreadViewModal
          gameId={mockGameId}
          postId={mockPostId}
          comment={mockCommentWithChild}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onClose={mockOnClose}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      const replyButtons = screen.getAllByRole('button', { name: /reply to this comment/i });
      await user.click(replyButtons[replyButtons.length - 1]);
      const textarea = screen.getByPlaceholderText('Write a reply...');
      await user.clear(textarea);
      await user.type(textarea, 'Half-written reply text');

      await user.click(document.querySelector('.fixed.inset-0') as HTMLElement);

      expect(mockOnClose).not.toHaveBeenCalled();
      expect(screen.queryByText(/discard unsaved reply/i)).not.toBeInTheDocument();
      expect(screen.getByRole('heading', { name: /thread view/i })).toBeInTheDocument();
      // The typed reply survives untouched — the point of the guard.
      expect(screen.getByPlaceholderText('Write a reply...')).toHaveValue('Half-written reply text');
    });

    it('should close without confirmation when no reply content is pending', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <ThreadViewModal
          gameId={mockGameId}
          postId={mockPostId}
          comment={mockComment}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onClose={mockOnClose}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      const backdrop = document.querySelector('.fixed.inset-0') as HTMLElement;
      await user.click(backdrop);

      expect(mockOnClose).toHaveBeenCalledOnce();
    });

    it('should close when user confirms discard', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <ThreadViewModal
          gameId={mockGameId}
          postId={mockPostId}
          comment={mockCommentWithChild}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onClose={mockOnClose}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      const replyButtons = screen.getAllByRole('button', { name: /reply to this comment/i });
      await user.click(replyButtons[replyButtons.length - 1]);
      await user.type(screen.getByPlaceholderText('Write a reply...'), 'Half-written reply text');

      await user.click(screen.getByRole('button', { name: /close thread view/i }));
      await user.click(screen.getByRole('button', { name: /discard/i }));

      expect(mockOnClose).toHaveBeenCalledOnce();
    });

    it('should keep modal open when user cancels the discard', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <ThreadViewModal
          gameId={mockGameId}
          postId={mockPostId}
          comment={mockCommentWithChild}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onClose={mockOnClose}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      const replyButtons = screen.getAllByRole('button', { name: /reply to this comment/i });
      await user.click(replyButtons[replyButtons.length - 1]);
      await user.type(screen.getByPlaceholderText('Write a reply...'), 'Half-written reply text');

      await user.click(screen.getByRole('button', { name: /close thread view/i }));
      await user.click(screen.getByRole('button', { name: /keep editing/i }));

      expect(mockOnClose).not.toHaveBeenCalled();
      expect(screen.getByRole('heading', { name: /thread view/i })).toBeInTheDocument();
    });

    it('should not block close after a reply is cancelled (unmount clears dirty state)', async () => {
      // Regression for the stale-Set bug: if a ThreadedComment unmounts while its
      // replyContent is non-empty, the cleanup effect must clear the dirty entry.
      // If it doesn't, hasDirtyReply stays true and the modal can never be closed.
      const user = userEvent.setup();
      renderWithProviders(
        <ThreadViewModal
          gameId={mockGameId}
          postId={mockPostId}
          comment={mockComment}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onClose={mockOnClose}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      // Open reply, type content, then cancel (clears content and hides form)
      await user.click(screen.getByRole('button', { name: /reply to this comment/i }));
      await user.type(screen.getByPlaceholderText('Write a reply...'), 'Some text');
      await user.click(screen.getByRole('button', { name: /cancel/i }));

      // Now clicking backdrop should close immediately — no stale dirty entry
      await user.click(document.querySelector('.fixed.inset-0') as HTMLElement);
      expect(mockOnClose).toHaveBeenCalledOnce();
    });
  });

  describe('Backdrop drag-close guard', () => {
    it('should not close when mousedown is inside modal but click fires on backdrop (drag simulation)', async () => {
      // Reproduces the Firefox-on-Windows behaviour: a resize-drag starts inside the
      // modal content area (mousedown on the inner div), but mouseup happens over the
      // backdrop, so Firefox synthesises a click on the backdrop directly.
      // The guard must ignore that spurious click.
      const user = userEvent.setup();
      renderWithProviders(
        <ThreadViewModal
          gameId={mockGameId}
          postId={mockPostId}
          comment={mockComment}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onClose={mockOnClose}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      const backdrop = document.querySelector('.fixed.inset-0.bg-black\\/60') as HTMLElement;
      const modalContent = backdrop.querySelector('.surface-base') as HTMLElement;

      // mousedown fires on the inner modal content (simulating drag start inside)
      await user.pointer({ target: modalContent, keys: '[MouseLeft>]' });
      // click fires on the backdrop (simulating Firefox synthesising click on backdrop at mouseup)
      await user.pointer({ target: backdrop, keys: '[/MouseLeft]' });

      expect(mockOnClose).not.toHaveBeenCalled();
    });

    it('should close when both mousedown and click occur on the backdrop', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <ThreadViewModal
          gameId={mockGameId}
          postId={mockPostId}
          comment={mockComment}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onClose={mockOnClose}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      const backdrop = document.querySelector('.fixed.inset-0.bg-black\\/60') as HTMLElement;
      await user.click(backdrop);

      expect(mockOnClose).toHaveBeenCalledOnce();
    });
  });

  describe('Modal Behavior', () => {
    it('should display Thread View heading', () => {
      renderWithProviders(
        <ThreadViewModal
          gameId={mockGameId}
          postId={mockPostId}
          comment={mockComment}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onClose={mockOnClose}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      expect(screen.getByRole('heading', { name: /thread view/i })).toBeInTheDocument();
    });

    it('should have a close button', () => {
      renderWithProviders(
        <ThreadViewModal
          gameId={mockGameId}
          postId={mockPostId}
          comment={mockComment}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onClose={mockOnClose}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      const closeButton = screen.getByRole('button', { name: /close/i });
      expect(closeButton).toBeInTheDocument();
    });

    it('should call onClose when close button is clicked', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <ThreadViewModal
          gameId={mockGameId}
          postId={mockPostId}
          comment={mockComment}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onClose={mockOnClose}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      const closeButton = screen.getByRole('button', { name: /close/i });
      await user.click(closeButton);
      expect(mockOnClose).toHaveBeenCalledOnce();
    });
  });
});
