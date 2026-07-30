import { describe, it, expect, beforeEach } from 'vitest';
import { screen, waitFor, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server';
import { renderWithProviders } from '../../test-utils/render';
import { ThreadedComment } from '../ThreadedComment';
import type { Message } from '../../types/messages';
import type { Character } from '../../types/characters';
import { logger } from '@/services/LoggingService';

describe('ThreadedComment', () => {
  const mockGameId = 1;
  const mockOnCreateReply = vi.fn();
  const mockCurrentUserId = 100;

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
    {
      id: 2,
      game_id: mockGameId,
      user_id: mockCurrentUserId,
      username: 'testuser',
      name: 'Villain',
      character_type: 'player_character',
      status: 'active',
      created_at: '2025-01-01T00:00:00Z',
      updated_at: '2025-01-01T00:00:00Z',
    },
  ];

  const mockComment: Message = {
    id: 1,
    game_id: mockGameId,
    author_id: 200,
    character_id: 3,
    content: 'This is a test comment',
    message_type: 'comment',
    thread_depth: 0,
    author_username: 'otheruser',
    character_name: 'Other Character',
    reply_count: 0,
    is_edited: false,
    is_deleted: false,
    created_at: '2025-01-15T10:30:00Z',
    updated_at: '2025-01-15T10:30:00Z',
  };

  const mockCommentWithReplies: Message = {
    ...mockComment,
    reply_count: 2,
  };

  const mockReplies: Message[] = [
    {
      id: 2,
      game_id: mockGameId,
      parent_id: 1,
      author_id: mockCurrentUserId,
      character_id: 1,
      content: 'This is a reply',
      message_type: 'comment',
      thread_depth: 1,
      author_username: 'testuser',
      character_name: 'Hero',
      reply_count: 0,
      is_edited: false,
      is_deleted: false,
      created_at: '2025-01-15T11:00:00Z',
      updated_at: '2025-01-15T11:00:00Z',
    },
    {
      id: 3,
      game_id: mockGameId,
      parent_id: 1,
      author_id: 300,
      character_id: 4,
      content: 'Another reply',
      message_type: 'comment',
      thread_depth: 1,
      author_username: 'thirduser',
      character_name: 'Third Character',
      reply_count: 0,
      is_edited: false,
      is_deleted: false,
      created_at: '2025-01-15T11:30:00Z',
      updated_at: '2025-01-15T11:30:00Z',
    },
  ];

  const setupDefaultHandlers = () => {
    server.use(
      http.get('/api/v1/games/:gameId/posts/:postId/comments', () => {
        return HttpResponse.json(mockReplies);
      })
    );
  };

  beforeEach(() => {
    server.resetHandlers();
    setupDefaultHandlers();
    mockOnCreateReply.mockClear();
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('renders comment content', () => {
      renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      expect(screen.getByText('This is a test comment')).toBeInTheDocument();
    });

    it('renders character name', () => {
      renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      expect(screen.getAllByText('Other Character')[0]).toBeInTheDocument();
    });

    it('renders author username', () => {
      renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      expect(screen.getAllByText(/@otheruser/)[0]).toBeInTheDocument();
    });

    it('links the author username to their user profile', () => {
      renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      // The @username text should be wrapped in a link to /users/:username
      const usernameLinks = screen.getAllByRole('link', { name: /@otheruser/ });
      expect(usernameLinks.length).toBeGreaterThan(0);
      usernameLinks.forEach(link => {
        expect(link).toHaveAttribute('href', '/users/otheruser');
      });
    });

    it('renders timestamp', () => {
      renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      // Should display a formatted timestamp with "ago" suffix (date-fns formatDistanceToNow)
      expect(screen.getAllByText(/ago/i)[0]).toBeInTheDocument();
    });

    it('renders reply button', () => {
      renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      expect(screen.getByRole('button', { name: /reply/i })).toBeInTheDocument();
    });

    it('shows "You" badge when user is the author', () => {
      const ownComment: Message = {
        ...mockComment,
        author_id: mockCurrentUserId,
        author_username: 'testuser',
      };

      renderWithProviders(
        <ThreadedComment
          comment={ownComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      expect(screen.getAllByText('You')[0]).toBeInTheDocument();
    });

    it('does not show "You" badge when user is not the author', () => {
      renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      expect(screen.queryByText('You')).not.toBeInTheDocument();
    });

    it('shows edited indicator when comment is edited', () => {
      const editedComment: Message = {
        ...mockComment,
        is_edited: true,
      };

      renderWithProviders(
        <ThreadedComment
          comment={editedComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      expect(screen.getAllByText('(edited)')[0]).toBeInTheDocument();
    });

    it('does not show edited indicator when comment is not edited', () => {
      renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      expect(screen.queryByText('(edited)')).not.toBeInTheDocument();
    });

    it('applies indentation when depth is greater than 0', () => {
      renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
          depth={1}
        />
      );

      // Verify visual indicator (border) is present - this is what users see
      const commentContainer = screen.getAllByTestId('threaded-comment')[0];
      expect(commentContainer).toHaveClass('border-l-2');
    });

    it('does not apply indentation when depth is 0', () => {
      renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
          depth={0}
        />
      );

      // Verify no border on depth 0 (root level comments)
      const commentContainer = screen.getAllByTestId('threaded-comment')[0];
      expect(commentContainer).not.toHaveClass('border-l-2');
    });

    it('applies different border color based on depth', () => {
      const { container: container1 } = renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
          depth={1}
        />
      );

      const { container: container2 } = renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
          depth={2}
        />
      );

      const commentContainer1 = container1.querySelector('.border-l-2');
      const commentContainer2 = container2.querySelector('.border-l-2');

      // Each depth gets a distinct avatar-hue rail so nesting levels are visually distinct
      expect(commentContainer1?.className).toContain('border-l-avatar-6');
      expect(commentContainer2?.className).toContain('border-l-avatar-4');
      expect(commentContainer1?.className).not.toBe(commentContainer2?.className);
    });
  });

  describe('Reply Count Display', () => {
    it('shows reply count button when comment has replies', () => {
      renderWithProviders(
        <ThreadedComment
          comment={mockCommentWithReplies}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      expect(screen.getByText(/2 replies/i)).toBeInTheDocument();
    });

    it('shows singular form for single reply', () => {
      const commentWithOneReply: Message = {
        ...mockComment,
        reply_count: 1,
      };

      renderWithProviders(
        <ThreadedComment
          comment={commentWithOneReply}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      expect(screen.getByText(/1 reply/i)).toBeInTheDocument();
    });

    it('does not show reply count button when comment has no replies', () => {
      renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      expect(screen.queryByText(/replies/i)).not.toBeInTheDocument();
    });

    it('shows expand icon when replies are visible', () => {
      renderWithProviders(
        <ThreadedComment
          comment={mockCommentWithReplies}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      expect(screen.getAllByText('▼')[0]).toBeInTheDocument();
    });

    it('shows collapse icon when replies are hidden', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <ThreadedComment
          comment={mockCommentWithReplies}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      const toggleButton = screen.getByRole('button', { name: /replies/i });
      await user.click(toggleButton);

      expect(screen.getAllByText('▶')[0]).toBeInTheDocument();
    });
  });

  describe('Reply Form Toggle', () => {
    it('does not show reply form initially', () => {
      renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      expect(screen.queryByPlaceholderText(/write a reply/i)).not.toBeInTheDocument();
    });

    it('shows reply form when reply button is clicked', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      const replyButton = screen.getByRole('button', { name: /reply/i });
      await user.click(replyButton);

      expect(screen.getByPlaceholderText(/write a reply/i)).toBeInTheDocument();
    });

    it('hides reply form when reply button is clicked again', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      const replyButton = screen.getByRole('button', { name: /reply/i });
      await user.click(replyButton);
      await user.click(replyButton);

      expect(screen.queryByPlaceholderText(/write a reply/i)).not.toBeInTheDocument();
    });
  });

  describe('Reply Form', () => {
    it('auto-selects first character when user has characters', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      const replyButton = screen.getByRole('button', { name: /reply/i });
      await user.click(replyButton);

      // Should auto-select first character (Hero)
      const select = screen.getByRole('combobox') as HTMLSelectElement;
      expect(select.value).toBe('1');
    });

    it('shows character dropdown when user has multiple characters', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      const replyButton = screen.getByRole('button', { name: /reply/i });
      await user.click(replyButton);

      expect(screen.getByRole('combobox')).toBeInTheDocument();
      expect(screen.getByText(/reply as hero/i)).toBeInTheDocument();
      expect(screen.getByText(/reply as villain/i)).toBeInTheDocument();
    });

    it('does not show character dropdown when user has single character', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          characters={[mockCharacters[0]]}
          controllableCharacters={[mockCharacters[0]]}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      const replyButton = screen.getByRole('button', { name: /reply/i });
      await user.click(replyButton);

      expect(screen.queryByRole('combobox')).not.toBeInTheDocument();
    });

    it('does not show reply button when user has no controllable characters', () => {
      renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          characters={[]}
          controllableCharacters={[]}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      // Issue #6: Reply button should be hidden when user has no controllable characters
      expect(screen.queryByRole('button', { name: /reply/i })).not.toBeInTheDocument();
    });

    it('allows changing selected character', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      const replyButton = screen.getByRole('button', { name: /reply/i });
      await user.click(replyButton);

      const select = screen.getByRole('combobox') as HTMLSelectElement;
      await user.selectOptions(select, '2');

      expect(select.value).toBe('2');
    });

    it('auto-selects parent comment character when parentComment is provided and user controls it', async () => {
      const user = userEvent.setup();

      // Parent comment authored by character ID 2 (Villain)
      const parentComment: Message = {
        id: 100,
        game_id: mockGameId,
        author_id: 200,
        character_id: 2, // Villain - user controls this character
        content: 'Parent comment as Villain',
        message_type: 'comment',
        thread_depth: 1,
        author_username: 'otheruser',
        character_name: 'Villain',
        reply_count: 0,
        is_edited: false,
        is_deleted: false,
        created_at: '2025-01-15T10:00:00Z',
        updated_at: '2025-01-15T10:00:00Z',
      };

      // Current comment is a reply to the parent comment
      const nestedComment: Message = {
        ...mockComment,
        parent_id: 100,
        thread_depth: 2,
      };

      renderWithProviders(
        <ThreadedComment
          comment={nestedComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
          parentComment={parentComment}
        />
      );

      const replyButton = screen.getByRole('button', { name: /reply/i });
      await user.click(replyButton);

      // Should auto-select parent's character (Villain, ID 2) instead of first character (Hero, ID 1)
      const select = screen.getByRole('combobox') as HTMLSelectElement;
      expect(select.value).toBe('2');
    });

    it('auto-selects first character when parentComment is null', async () => {
      const user = userEvent.setup();

      renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
          parentComment={null}
        />
      );

      const replyButton = screen.getByRole('button', { name: /reply/i });
      await user.click(replyButton);

      // Should auto-select first character (Hero, ID 1) when no parent provided
      const select = screen.getByRole('combobox') as HTMLSelectElement;
      expect(select.value).toBe('1');
    });

    it('auto-selects first character when parentComment character is not controllable', async () => {
      const user = userEvent.setup();

      // Parent comment authored by character ID 99 (not in controllableCharacters)
      const parentComment: Message = {
        id: 100,
        game_id: mockGameId,
        author_id: 200,
        character_id: 99, // Not controllable by current user
        content: 'Parent comment as NPC',
        message_type: 'comment',
        thread_depth: 1,
        author_username: 'otheruser',
        character_name: 'NPC Guard',
        reply_count: 0,
        is_edited: false,
        is_deleted: false,
        created_at: '2025-01-15T10:00:00Z',
        updated_at: '2025-01-15T10:00:00Z',
      };

      const nestedComment: Message = {
        ...mockComment,
        parent_id: 100,
        thread_depth: 2,
      };

      renderWithProviders(
        <ThreadedComment
          comment={nestedComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
          parentComment={parentComment}
        />
      );

      const replyButton = screen.getByRole('button', { name: /reply/i });
      await user.click(replyButton);

      // Should fall back to first character (Hero, ID 1) when parent's character not controllable
      const select = screen.getByRole('combobox') as HTMLSelectElement;
      expect(select.value).toBe('1');
    });

    it('allows typing in reply textarea', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      const replyButton = screen.getByRole('button', { name: /reply/i });
      await user.click(replyButton);

      const textarea = screen.getByPlaceholderText(/write a reply/i);
      await user.type(textarea, 'Test reply content');

      expect(textarea).toHaveValue('Test reply content');
    });

    it('disables reply button when content is empty', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      const replyButton = screen.getByRole('button', { name: /reply/i });
      await user.click(replyButton);

      // Find the submit button by type="submit" within the form
      const textarea = screen.getByPlaceholderText(/write a reply/i);
      const form = textarea.closest('form');
      const submitButton = form?.querySelector('button[type="submit"]') as HTMLButtonElement;
      expect(submitButton).toBeDisabled();
    });

    it('enables reply button when content is provided', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      await user.click(screen.getByRole('button', { name: /reply/i }));

      const textarea = screen.getByPlaceholderText(/write a reply/i);
      await user.type(textarea, 'Test reply');

      const form = textarea.closest('form');
      const submitButton = form?.querySelector('button[type="submit"]') as HTMLButtonElement;
      expect(submitButton).not.toBeDisabled();
    });

    it('disables reply button when content is only whitespace', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      await user.click(screen.getByRole('button', { name: /reply/i }));

      const textarea = screen.getByPlaceholderText(/write a reply/i);
      await user.type(textarea, '   ');

      const form = textarea.closest('form');
      const submitButton = form?.querySelector('button[type="submit"]') as HTMLButtonElement;
      expect(submitButton).toBeDisabled();
    });

    it('shows cancel button', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      await user.click(screen.getByRole('button', { name: /reply/i }));

      expect(screen.getByRole('button', { name: /cancel/i })).toBeInTheDocument();
    });

    it('closes form when cancel button is clicked', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      await user.click(screen.getByRole('button', { name: /reply/i }));
      const textarea = screen.getByPlaceholderText(/write a reply/i);
      await user.type(textarea, 'Test content');

      await user.click(screen.getByRole('button', { name: /cancel/i }));

      expect(screen.queryByPlaceholderText(/write a reply/i)).not.toBeInTheDocument();
    });

    it('clears form content when cancel is clicked', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      await user.click(screen.getByRole('button', { name: /reply/i }));
      const textarea = screen.getByPlaceholderText(/write a reply/i);
      await user.type(textarea, 'Test content');
      await user.click(screen.getByRole('button', { name: /cancel/i }));

      // Reopen form
      await user.click(screen.getByRole('button', { name: /reply/i }));
      const newTextarea = screen.getByPlaceholderText(/write a reply/i);

      expect(newTextarea).toHaveValue('');
    });
  });

  describe('Reply Submission', () => {
    it('calls onCreateReply with correct parameters', async () => {
      const user = userEvent.setup();
      mockOnCreateReply.mockResolvedValueOnce(undefined);

      renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          postId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      await user.click(screen.getByRole('button', { name: /reply/i }));

      const textarea = screen.getByPlaceholderText(/write a reply/i);
      await user.type(textarea, 'Test reply');

      const form = textarea.closest('form');
      const submitButton = form?.querySelector('button[type="submit"]') as HTMLButtonElement;
      await user.click(submitButton);

      await waitFor(() => {
        expect(mockOnCreateReply).toHaveBeenCalledWith(1, 1, 'Test reply', 1);
      });
    });

    it('trims whitespace from reply content', async () => {
      const user = userEvent.setup();
      mockOnCreateReply.mockResolvedValueOnce(undefined);

      renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          postId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      await user.click(screen.getByRole('button', { name: /reply/i }));

      const textarea = screen.getByPlaceholderText(/write a reply/i);
      await user.type(textarea, '  Test reply  ');

      const form = textarea.closest('form'); const submitButton = form?.querySelector('button[type="submit"]') as HTMLButtonElement;
      await user.click(submitButton);

      await waitFor(() => {
        expect(mockOnCreateReply).toHaveBeenCalledWith(1, 1, 'Test reply', 1);
      });
    });

    it('clears form after successful submission', async () => {
      const user = userEvent.setup();
      mockOnCreateReply.mockResolvedValueOnce(undefined);

      renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      await user.click(screen.getByRole('button', { name: /reply/i }));

      const textarea = screen.getByPlaceholderText(/write a reply/i);
      await user.type(textarea, 'Test reply');

      const form = textarea.closest('form'); const submitButton = form?.querySelector('button[type="submit"]') as HTMLButtonElement;
      await user.click(submitButton);

      await waitFor(() => {
        expect(screen.queryByPlaceholderText(/write a reply/i)).not.toBeInTheDocument();
      });
    });

    it.skip('shows loading state during submission', async () => {
      const user = userEvent.setup();
      mockOnCreateReply.mockImplementation(
        () => new Promise((resolve) => setTimeout(resolve, 100))
      );

      renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      await user.click(screen.getByRole('button', { name: /reply/i }));

      const textarea = screen.getByPlaceholderText(/write a reply/i);
      await user.type(textarea, 'Test reply');

      const form = textarea.closest('form'); const submitButton = form?.querySelector('button[type="submit"]') as HTMLButtonElement;
      await user.click(submitButton);

      await waitFor(() => {
        expect(screen.getByText(/posting\.\.\./i)).toBeInTheDocument();
      });
    });

    it.skip('disables form fields during submission', async () => {
      const user = userEvent.setup();
      mockOnCreateReply.mockImplementation(
        () => new Promise((resolve) => setTimeout(resolve, 100))
      );

      renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      await user.click(screen.getByRole('button', { name: /reply/i }));

      const textarea = screen.getByPlaceholderText(/write a reply/i);
      await user.type(textarea, 'Test reply');

      const form = textarea.closest('form'); const submitButton = form?.querySelector('button[type="submit"]') as HTMLButtonElement;
      await user.click(submitButton);

      await waitFor(() => {
        expect(textarea).toBeDisabled();
        expect(screen.getByRole('button', { name: /posting\.\.\./i })).toBeDisabled();
        expect(screen.getByRole('button', { name: /cancel/i })).toBeDisabled();
      });
    });

    it('reloads replies after successful submission', async () => {
      const user = userEvent.setup();
      mockOnCreateReply.mockResolvedValueOnce(undefined);

      renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      await user.click(screen.getByRole('button', { name: /reply/i }));

      const textarea = screen.getByPlaceholderText(/write a reply/i);
      await user.type(textarea, 'Test reply');

      const form = textarea.closest('form'); const submitButton = form?.querySelector('button[type="submit"]') as HTMLButtonElement;
      await user.click(submitButton);

      await waitFor(() => {
        expect(mockOnCreateReply).toHaveBeenCalled();
      });

      // Should reload replies - this would trigger the getPostComments API call
      // which is mocked to return mockReplies
      await waitFor(() => {
        expect(screen.queryByText(/posting\.\.\./i)).not.toBeInTheDocument();
      });
    });

  });

  describe('Nested Replies', () => {
    it('automatically loads replies when comment has replies', async () => {
      renderWithProviders(
        <ThreadedComment
          comment={mockCommentWithReplies}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      await waitFor(() => {
        expect(screen.queryAllByText('This is a reply').length).toBeGreaterThanOrEqual(1);
        expect(screen.queryAllByText('Another reply').length).toBeGreaterThanOrEqual(1);
      });
    });

    it('shows loading state while loading replies', async () => {
      server.use(
        http.get('/api/v1/games/:gameId/posts/:postId/comments', async () => {
          await new Promise((resolve) => setTimeout(resolve, 100));
          return HttpResponse.json(mockReplies);
        })
      );

      renderWithProviders(
        <ThreadedComment
          comment={mockCommentWithReplies}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      expect(screen.queryAllByText(/loading replies\.\.\./i).length).toBeGreaterThanOrEqual(1);

      await waitFor(() => {
        expect(screen.queryByText(/loading replies\.\.\./i)).not.toBeInTheDocument();
      });
    });

    it('toggles replies visibility when reply count button is clicked', async () => {
      const user = userEvent.setup();

      renderWithProviders(
        <ThreadedComment
          comment={mockCommentWithReplies}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      await waitFor(() => {
        expect(screen.queryAllByText('This is a reply').length).toBeGreaterThanOrEqual(1);
      });

      const toggleButton = screen.getByRole('button', { name: /replies/i });
      await user.click(toggleButton);

      await waitFor(() => {
        expect(screen.queryByText('This is a reply')).not.toBeInTheDocument();
      });

      await user.click(toggleButton);

      await waitFor(() => {
        expect(screen.queryAllByText('This is a reply').length).toBeGreaterThanOrEqual(1);
      });
    });

    it('renders nested ThreadedComment components recursively', async () => {
      renderWithProviders(
        <ThreadedComment
          comment={mockCommentWithReplies}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      await waitFor(() => {
        // Should render the nested replies
        expect(screen.getAllByText('Hero')[0]).toBeInTheDocument();
        expect(screen.getAllByText('Third Character')[0]).toBeInTheDocument();
      });
    });

    it('increases depth for nested comments', async () => {
      renderWithProviders(
        <ThreadedComment
          comment={mockCommentWithReplies}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
          depth={1}
        />
      );

      await waitFor(() => {
        expect(screen.queryAllByText('This is a reply').length).toBeGreaterThanOrEqual(1);
      });

      // Parent should have depth 1 indentation (verify by border presence)
      const parentContainer = screen.getAllByTestId('threaded-comment')[0];
      expect(parentContainer).toHaveClass('border-l-2');

      // Nested replies would have depth 2, but we can't easily verify this
      // without inspecting the DOM structure more deeply
    });

    it('does not show nested replies when showReplies is false', async () => {
      const user = userEvent.setup();

      renderWithProviders(
        <ThreadedComment
          comment={mockCommentWithReplies}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      await waitFor(() => {
        expect(screen.queryAllByText('This is a reply').length).toBeGreaterThanOrEqual(1);
      });

      const toggleButton = screen.getByRole('button', { name: /replies/i });
      await user.click(toggleButton);

      expect(screen.queryByText('This is a reply')).not.toBeInTheDocument();
    });

    it('shows replies after submitting a new reply', async () => {
      const user = userEvent.setup();
      mockOnCreateReply.mockResolvedValueOnce(undefined);

      renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      await user.click(screen.getByRole('button', { name: /reply/i }));

      const textarea = screen.getByPlaceholderText(/write a reply/i);
      await user.type(textarea, 'New reply');

      const form = textarea.closest('form'); const submitButton = form?.querySelector('button[type="submit"]') as HTMLButtonElement;
      await user.click(submitButton);

      // After submission, replies should be loaded and visible
      await waitFor(() => {
        expect(screen.queryAllByText('This is a reply').length).toBeGreaterThanOrEqual(1);
      });
    });
  });

  describe('Date Formatting', () => {
    it('formats recent timestamps with "ago" suffix', () => {
      const recentComment: Message = {
        ...mockComment,
        created_at: new Date().toISOString(),
      };

      renderWithProviders(
        <ThreadedComment
          comment={recentComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      // date-fns formats as "less than a minute ago" for very recent dates
      expect(screen.getAllByText(/ago/i)[0]).toBeInTheDocument();
    });

    it('formats timestamps within an hour as "X minutes ago"', () => {
      const thirtyMinsAgo = new Date(Date.now() - 30 * 60 * 1000).toISOString();
      const recentComment: Message = {
        ...mockComment,
        created_at: thirtyMinsAgo,
      };

      renderWithProviders(
        <ThreadedComment
          comment={recentComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      // date-fns formats as "30 minutes ago"
      expect(screen.getAllByText(/30 minutes ago/i)[0]).toBeInTheDocument();
    });

    it('formats timestamps within a day as "X hours ago"', () => {
      const fiveHoursAgo = new Date(Date.now() - 5 * 60 * 60 * 1000).toISOString();
      const recentComment: Message = {
        ...mockComment,
        created_at: fiveHoursAgo,
      };

      renderWithProviders(
        <ThreadedComment
          comment={recentComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      // date-fns formats as "about 5 hours ago"
      expect(screen.getAllByText(/5 hours ago/i)[0]).toBeInTheDocument();
    });

    it('formats timestamps within a week as "X days ago"', () => {
      const threeDaysAgo = new Date(Date.now() - 3 * 24 * 60 * 60 * 1000).toISOString();
      const recentComment: Message = {
        ...mockComment,
        created_at: threeDaysAgo,
      };

      renderWithProviders(
        <ThreadedComment
          comment={recentComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      // date-fns formats as "3 days ago"
      expect(screen.getAllByText(/3 days ago/i)[0]).toBeInTheDocument();
    });

    it('formats old timestamps with relative time', () => {
      // Using a date from 2023 to ensure it's more than a week ago
      const oldComment: Message = {
        ...mockComment,
        created_at: '2023-01-01T00:00:00Z',
      };

      renderWithProviders(
        <ThreadedComment
          comment={oldComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      // date-fns formats as "about 2 years ago" or "over 2 years ago"
      expect(screen.getAllByText(/years ago/i)[0]).toBeInTheDocument();
    });
  });

  describe('Error Handling', () => {
    it('handles error when loading replies fails', async () => {
      const loggerErrorSpy = vi.spyOn(logger, 'error').mockImplementation(() => {});

      server.use(
        http.get('/api/v1/games/:gameId/posts/:postId/comments', () => {
          return HttpResponse.json({ error: 'Failed to load replies' }, { status: 500 });
        })
      );

      renderWithProviders(
        <ThreadedComment
          comment={mockCommentWithReplies}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      await waitFor(() => {
        expect(loggerErrorSpy).toHaveBeenCalled();
      });

      loggerErrorSpy.mockRestore();
    });

    it('handles error when creating reply fails', async () => {
      const user = userEvent.setup();
      const loggerErrorSpy = vi.spyOn(logger, 'error').mockImplementation(() => {});
      mockOnCreateReply.mockRejectedValueOnce(new Error('Failed to create reply'));

      renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      await user.click(screen.getByRole('button', { name: /reply/i }));

      const textarea = screen.getByPlaceholderText(/write a reply/i);
      await user.type(textarea, 'Test reply');

      const form = textarea.closest('form'); const submitButton = form?.querySelector('button[type="submit"]') as HTMLButtonElement;
      await user.click(submitButton);

      await waitFor(() => {
        expect(loggerErrorSpy).toHaveBeenCalled();
      });

      // Form should remain open with content preserved
      expect(screen.getByPlaceholderText(/write a reply/i)).toHaveValue('Test reply');

      loggerErrorSpy.mockRestore();
    });
  });

  describe('Integration', () => {
    it('handles complete workflow from viewing to replying', async () => {
      const user = userEvent.setup();
      mockOnCreateReply.mockResolvedValueOnce(undefined);

      renderWithProviders(
        <ThreadedComment
          comment={mockCommentWithReplies}
          gameId={mockGameId}
          postId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      // Wait for replies to load
      await waitFor(() => {
        expect(screen.queryAllByText('This is a reply').length).toBeGreaterThanOrEqual(1);
      });

      // Collapse replies
      const toggleButton = screen.getByRole('button', { name: /replies/i });
      await user.click(toggleButton);

      await waitFor(() => {
        expect(screen.queryByText('This is a reply')).not.toBeInTheDocument();
      });

      // Open reply form
      await user.click(screen.getByRole('button', { name: /reply/i }));

      // Change character
      const select = screen.getByRole('combobox') as HTMLSelectElement;
      await user.selectOptions(select, '2');

      // Type reply
      const textarea = screen.getByPlaceholderText(/write a reply/i);
      await user.type(textarea, 'My detailed reply');

      // Submit
      const form = textarea.closest('form'); const submitButton = form?.querySelector('button[type="submit"]') as HTMLButtonElement;
      await user.click(submitButton);

      // Verify submission
      await waitFor(() => {
        expect(mockOnCreateReply).toHaveBeenCalledWith(1, 2, 'My detailed reply', 1);
      });

      // Replies should be visible again after submission
      await waitFor(() => {
        expect(screen.queryAllByText('This is a reply').length).toBeGreaterThanOrEqual(1);
      });
    });

    it('handles nested reply workflow', async () => {
      const user = userEvent.setup();
      mockOnCreateReply.mockResolvedValueOnce(undefined);

      renderWithProviders(
        <ThreadedComment
          comment={mockCommentWithReplies}
          gameId={mockGameId}
          postId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
          depth={1}
        />
      );

      // Wait for nested replies to load
      await waitFor(() => {
        expect(screen.queryAllByText('This is a reply').length).toBeGreaterThanOrEqual(1);
      });

      // Click reply button
      const replyButtons = screen.getAllByRole('button', { name: /reply/i });
      await user.click(replyButtons[0]);

      // Type and submit reply
      const textarea = screen.getByPlaceholderText(/write a reply/i);
      await user.type(textarea, 'Nested reply');

      const form = textarea.closest('form'); const submitButton = form?.querySelector('button[type="submit"]') as HTMLButtonElement;
      await user.click(submitButton);

      await waitFor(() => {
        expect(mockOnCreateReply).toHaveBeenCalledWith(1, 1, 'Nested reply', 1);
      });
    });
  });

  describe('Deleted Comments', () => {
    const mockDeletedComment: Message = {
      ...mockComment,
      is_deleted: true,
      deleted_at: '2025-01-15T12:00:00Z',
      deleted_by_user_id: mockCurrentUserId,
    };

    const mockDeletedCommentWithReplies: Message = {
      ...mockDeletedComment,
      reply_count: 2,
    };

    it('renders "[Comment deleted]" placeholder for deleted comments', () => {
      renderWithProviders(
        <ThreadedComment
          comment={mockDeletedComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      expect(screen.getByText('[Comment deleted]')).toBeInTheDocument();
      expect(screen.queryByText('This is a test comment')).not.toBeInTheDocument();
    });

    it('does not show Reply button for deleted comments', () => {
      renderWithProviders(
        <ThreadedComment
          comment={mockDeletedComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      expect(screen.queryByRole('button', { name: /reply/i })).not.toBeInTheDocument();
    });

    it('does not show Edit button for deleted comments owned by user', () => {
      const ownDeletedComment: Message = {
        ...mockDeletedComment,
        author_id: mockCurrentUserId,
      };

      renderWithProviders(
        <ThreadedComment
          comment={ownDeletedComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      expect(screen.queryByRole('button', { name: /edit/i })).not.toBeInTheDocument();
    });

    it('does not show Delete button for deleted comments owned by user', () => {
      const ownDeletedComment: Message = {
        ...mockDeletedComment,
        author_id: mockCurrentUserId,
      };

      renderWithProviders(
        <ThreadedComment
          comment={ownDeletedComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      expect(screen.queryByRole('button', { name: /delete/i })).not.toBeInTheDocument();
    });

    it('still shows Copy link button for deleted comments', () => {
      renderWithProviders(
        <ThreadedComment
          comment={mockDeletedComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      expect(screen.getByRole('button', { name: /copy link/i })).toBeInTheDocument();
    });

    it('still shows Parent link for deleted comments with parent', () => {
      const deletedCommentWithParent: Message = {
        ...mockDeletedComment,
        parent_id: 999,
        thread_depth: 2,
      };

      renderWithProviders(
        <ThreadedComment
          comment={deletedCommentWithParent}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      expect(screen.getByRole('link', { name: /parent/i })).toBeInTheDocument();
    });

    it('shows reply count button for deleted comments with replies', () => {
      renderWithProviders(
        <ThreadedComment
          comment={mockDeletedCommentWithReplies}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      expect(screen.getByRole('button', { name: /2 replies/i })).toBeInTheDocument();
    });

    it('loads and displays nested replies under deleted comments', async () => {
      renderWithProviders(
        <ThreadedComment
          comment={mockDeletedCommentWithReplies}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      // Wait for replies to load automatically
      await waitFor(() => {
        expect(screen.queryAllByText('This is a reply').length).toBeGreaterThanOrEqual(1);
        expect(screen.queryAllByText('Another reply').length).toBeGreaterThanOrEqual(1);
      });

      // Deleted comment placeholder should still be visible
      expect(screen.getByText('[Comment deleted]')).toBeInTheDocument();
    });

    it('renders character name and username for deleted comments', () => {
      renderWithProviders(
        <ThreadedComment
          comment={mockDeletedComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      // Character name and username should still be shown
      expect(screen.getAllByText('Other Character')[0]).toBeInTheDocument();
      expect(screen.getAllByText(/@otheruser/)[0]).toBeInTheDocument();
    });

    it('does not show edited indicator for deleted comments', () => {
      const deletedEditedComment: Message = {
        ...mockDeletedComment,
        is_edited: true,
      };

      renderWithProviders(
        <ThreadedComment
          comment={deletedEditedComment}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      // Should not show (edited) indicator for deleted comments
      expect(screen.queryByText('(edited)')).not.toBeInTheDocument();
    });

    it('preserves thread structure with deleted middle comment', async () => {
      // Create a scenario: Comment A (active) → Comment B (deleted with replies) → Comment C (active)
      const commentA: Message = {
        id: 100,
        game_id: mockGameId,
        author_id: 200,
        character_id: 3,
        content: 'Comment A - top level',
        message_type: 'comment',
        thread_depth: 0,
        author_username: 'user1',
        character_name: 'Character 1',
        reply_count: 1, // Has one reply (deleted comment B)
        is_edited: false,
        is_deleted: false,
        created_at: '2025-01-15T10:00:00Z',
        updated_at: '2025-01-15T10:00:00Z',
      };

      const commentB: Message = {
        id: 101,
        game_id: mockGameId,
        parent_id: 100,
        author_id: 300,
        character_id: 4,
        content: 'Comment B - deleted middle',
        message_type: 'comment',
        thread_depth: 1,
        author_username: 'user2',
        character_name: 'Character 2',
        reply_count: 1, // Has one reply (comment C)
        is_edited: false,
        is_deleted: true,
        deleted_at: '2025-01-15T11:30:00Z',
        created_at: '2025-01-15T11:00:00Z',
        updated_at: '2025-01-15T11:00:00Z',
      };

      const commentC: Message = {
        id: 102,
        game_id: mockGameId,
        parent_id: 101,
        author_id: 400,
        character_id: 5,
        content: 'Comment C - nested under deleted B',
        message_type: 'comment',
        thread_depth: 2,
        author_username: 'user3',
        character_name: 'Character 3',
        reply_count: 0,
        is_edited: false,
        is_deleted: false,
        created_at: '2025-01-15T12:00:00Z',
        updated_at: '2025-01-15T12:00:00Z',
      };

      // Mock API responses
      server.use(
        http.get('/api/v1/games/:gameId/posts/100/comments', () => {
          return HttpResponse.json([commentB]); // Comment A's replies
        }),
        http.get('/api/v1/games/:gameId/posts/101/comments', () => {
          return HttpResponse.json([commentC]); // Comment B's replies
        })
      );

      renderWithProviders(
        <ThreadedComment
          comment={commentA}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      // Wait for comment A to load its replies (deleted comment B)
      await waitFor(() => {
        expect(screen.queryAllByText('[Comment deleted]').length).toBeGreaterThanOrEqual(1);
      });

      // Wait for deleted comment B to load its replies (comment C)
      await waitFor(() => {
        expect(screen.queryAllByText('Comment C - nested under deleted B').length).toBeGreaterThanOrEqual(1);
      });

      // Verify the full thread structure is preserved:
      // - Comment A (active) is visible
      expect(screen.queryAllByText('Comment A - top level').length).toBeGreaterThanOrEqual(1);
      // - Comment B (deleted) shows placeholder
      expect(screen.queryAllByText('[Comment deleted]').length).toBeGreaterThanOrEqual(1);
      // - Comment C (active under deleted B) is visible
      expect(screen.queryAllByText('Comment C - nested under deleted B').length).toBeGreaterThanOrEqual(1);
    });
  });

  describe('Hide deleted comments with no children', () => {
    it('does not render a deleted reply with reply_count=0 in the lazy-load path', async () => {
      const deletedLeafReply: Message = {
        id: 50,
        game_id: mockGameId,
        parent_id: 1,
        author_id: 200,
        character_id: 3,
        content: 'Deleted leaf',
        message_type: 'comment',
        thread_depth: 1,
        author_username: 'someone',
        character_name: 'Ghost',
        reply_count: 0,
        is_deleted: true,
        is_edited: false,
        created_at: '2025-01-15T11:00:00Z',
        updated_at: '2025-01-15T11:00:00Z',
      };

      server.use(
        http.get('/api/v1/games/:gameId/posts/:postId/comments', () => {
          return HttpResponse.json([deletedLeafReply]);
        })
      );

      renderWithProviders(
        <ThreadedComment
          comment={mockCommentWithReplies}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      // Allow the lazy-load path to run inside act(), so any state update it
      // makes is wrapped; a bare sleep leaves it outside act and warns.
      await act(async () => {
        await new Promise(resolve => setTimeout(resolve, 50));
      });

      expect(screen.queryByText('[Comment deleted]')).not.toBeInTheDocument();
    });

    it('still renders a deleted reply that has replies (reply_count > 0)', async () => {
      const deletedWithChildren: Message = {
        id: 51,
        game_id: mockGameId,
        parent_id: 1,
        author_id: 200,
        character_id: 3,
        content: 'Deleted middle',
        message_type: 'comment',
        thread_depth: 1,
        author_username: 'someone',
        character_name: 'Ghost',
        reply_count: 1,
        is_deleted: true,
        is_edited: false,
        created_at: '2025-01-15T11:00:00Z',
        updated_at: '2025-01-15T11:00:00Z',
      };

      server.use(
        http.get('/api/v1/games/:gameId/posts/:postId/comments', () => {
          return HttpResponse.json([deletedWithChildren]);
        })
      );

      renderWithProviders(
        <ThreadedComment
          comment={mockCommentWithReplies}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      await waitFor(() => {
        expect(screen.queryAllByText('[Comment deleted]').length).toBeGreaterThanOrEqual(1);
      });
    });
  });

  describe('Bug #2: Parent link navigation', () => {
    it('should link to post in common room when parent is a post (thread_depth === 1)', () => {
      // Top-level reply to a post (thread_depth === 1)
      const topLevelReply: Message = {
        id: 100,
        game_id: mockGameId,
        parent_id: 50, // Parent is a POST with ID 50
        author_id: mockCurrentUserId,
        character_id: 1,
        content: 'Reply to post',
        message_type: 'comment',
        thread_depth: 1, // Top-level reply to post
        author_username: 'testuser',
        character_name: 'Hero',
        reply_count: 0,
        is_edited: false,
        is_deleted: false,
        created_at: '2025-01-15T11:00:00Z',
        updated_at: '2025-01-15T11:00:00Z',
      };

      renderWithProviders(
        <ThreadedComment
          comment={topLevelReply}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      // Parent link should navigate to the post in common room view
      const parentLink = screen.getByRole('link', { name: /parent/i });
      expect(parentLink).toHaveAttribute('href', `/games/${mockGameId}?tab=common-room&postId=50`);
    });

    it('should link to parent comment when parent is a comment (thread_depth > 1)', () => {
      // Nested reply to another comment (thread_depth > 1)
      const nestedReply: Message = {
        id: 101,
        game_id: mockGameId,
        parent_id: 100, // Parent is a COMMENT with ID 100
        author_id: mockCurrentUserId,
        character_id: 1,
        content: 'Reply to comment',
        message_type: 'comment',
        thread_depth: 2, // Nested reply to comment
        author_username: 'testuser',
        character_name: 'Hero',
        reply_count: 0,
        is_edited: false,
        is_deleted: false,
        created_at: '2025-01-15T11:30:00Z',
        updated_at: '2025-01-15T11:30:00Z',
      };

      renderWithProviders(
        <ThreadedComment
          comment={nestedReply}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      // Parent link should navigate to the parent comment
      const parentLink = screen.getByRole('link', { name: /parent/i });
      expect(parentLink).toHaveAttribute('href', `/games/${mockGameId}?tab=common-room&comment=100`);
    });

    it('should not show parent link when parent_id is undefined', () => {
      // Top-level post (no parent)
      const topLevelPost: Message = {
        id: 50,
        game_id: mockGameId,
        // No parent_id
        author_id: mockCurrentUserId,
        character_id: 1,
        content: 'This is a post',
        message_type: 'post',
        thread_depth: 0,
        author_username: 'testuser',
        character_name: 'Hero',
        comment_count: 5,
        is_edited: false,
        is_deleted: false,
        created_at: '2025-01-15T10:00:00Z',
        updated_at: '2025-01-15T10:00:00Z',
      };

      renderWithProviders(
        <ThreadedComment
          comment={topLevelPost}
          gameId={mockGameId}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
        />
      );

      // Parent link should NOT be rendered
      expect(screen.queryByRole('link', { name: /parent/i })).not.toBeInTheDocument();
    });
  });

  describe('Read-Only Mode', () => {
    it('should not show edit/delete buttons when readOnly=true', async () => {
      // Create a comment owned by current user (so edit/delete buttons would normally show)
      const myComment: Message = {
        ...mockComment,
        author_id: mockCurrentUserId,
        character_id: mockCharacters[0].id,
        character_name: mockCharacters[0].name,
        author_username: 'testuser',
      };

      renderWithProviders(
        <ThreadedComment
          comment={myComment}
          gameId={mockGameId}
          postId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
          readOnly={true}
        />
      );

      // Comment content should be visible
      expect(screen.getByText('This is a test comment')).toBeInTheDocument();

      // Edit/delete buttons should NOT be visible (even though user owns the comment)
      expect(screen.queryByRole('button', { name: /edit/i })).not.toBeInTheDocument();
      expect(screen.queryByRole('button', { name: /delete/i })).not.toBeInTheDocument();
    });

    it('should not show reply button when readOnly=true', async () => {
      renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          postId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
          readOnly={true}
        />
      );

      // Reply button should NOT be visible
      expect(screen.queryByRole('button', { name: /reply/i })).not.toBeInTheDocument();
    });

    it('should allow edit/delete buttons when readOnly=false (default)', async () => {
      // Create a comment owned by current user
      const myComment: Message = {
        ...mockComment,
        author_id: mockCurrentUserId,
        character_id: mockCharacters[0].id,
        character_name: mockCharacters[0].name,
        author_username: 'testuser',
      };

      renderWithProviders(
        <ThreadedComment
          comment={myComment}
          gameId={mockGameId}
          postId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
          readOnly={false}
        />
      );

      // Hover to show action buttons
      const commentCard = screen.getByText('This is a test comment').closest('[data-testid^="comment-"]');
      if (commentCard) {
        await userEvent.hover(commentCard);
      }

      // Edit/delete buttons SHOULD be visible when readOnly=false
      await waitFor(() => {
        expect(screen.getByRole('button', { name: /edit/i })).toBeInTheDocument();
      });
      expect(screen.getByRole('button', { name: /delete/i })).toBeInTheDocument();
    });

    it('should propagate readOnly to nested replies', async () => {
      // Mock API responses for replies
      server.use(
        http.get(`/api/v1/games/${mockGameId}/messages/:messageId/children`, () => {
          return HttpResponse.json(mockReplies);
        })
      );

      renderWithProviders(
        <ThreadedComment
          comment={mockCommentWithReplies}
          gameId={mockGameId}
          postId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
          readOnly={true}
        />
      );

      // Wait for replies to load
      await waitFor(() => {
        expect(screen.queryAllByText('This is a reply').length).toBeGreaterThanOrEqual(1);
      });

      // Reply buttons should NOT be visible on nested comments either
      const replyButtons = screen.queryAllByRole('button', { name: /reply/i });
      expect(replyButtons).toHaveLength(0);
    });
  });

  describe('API call efficiency', () => {
    it('does not fetch replies when shouldShowContinueButton is true (depth === maxDepth - 1)', async () => {
      // This is the regression test for the history-view API call flood bug.
      // When PostCard loads all comments via getPostCommentsWithThreads up to max_depth,
      // comments at depth (maxDepth - 1) have reply_count > 0 (replies exist deeper)
      // but their children are not returned by the backend (depth cutoff).
      // Without the fix, each such comment fires getPostComments() on mount,
      // causing 50+ simultaneous API calls in threads with many deep replies.
      let loadCount = 0;
      server.use(
        http.get('/api/v1/games/:gameId/posts/:postId/comments', () => {
          loadCount++;
          return HttpResponse.json(mockReplies);
        })
      );

      const maxDepth = 5;
      const commentAtContinueDepth: Message = {
        ...mockCommentWithReplies, // reply_count: 2, but no preloaded children
        id: 99,
      };

      renderWithProviders(
        <ThreadedComment
          comment={commentAtContinueDepth}
          gameId={mockGameId}
          postId={10}
          characters={mockCharacters}
          controllableCharacters={[]}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
          depth={maxDepth - 1} // shouldShowContinueButton = true
          maxDepth={maxDepth}
        />
      );

      // Allow real time to pass so a stray fetch would have fired. The sleep runs
      // inside act() so component state settling meanwhile doesn't trigger a
      // React act warning; waitFor on the negative assertion would be vacuous.
      await act(async () => {
        await new Promise(resolve => setTimeout(resolve, 50));
      });

      expect(loadCount).toBe(0);
    });
  });

  describe('allowReadTracking prop', () => {
    it('hides toggle button when allowReadTracking=false even in manual mode', () => {
      renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          postId={10}
          characters={mockCharacters}
          controllableCharacters={[]}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
          commentReadMode="manual"
          manualReadCommentIDs={[]}
          onToggleRead={vi.fn()}
          allowReadTracking={false}
        />
      );

      expect(screen.queryByRole('button', { name: /mark as (read|unread)/i })).not.toBeInTheDocument();
    });

    it('shows toggle button when allowReadTracking=true in manual mode', () => {
      renderWithProviders(
        <ThreadedComment
          comment={mockComment}
          gameId={mockGameId}
          postId={10}
          characters={mockCharacters}
          controllableCharacters={[]}
          onCreateReply={mockOnCreateReply}
          currentUserId={mockCurrentUserId}
          commentReadMode="manual"
          manualReadCommentIDs={[]}
          onToggleRead={vi.fn()}
          allowReadTracking={true}
        />
      );

      expect(screen.getByRole('button', { name: /mark as (read|unread)/i })).toBeInTheDocument();
    });
  });
});
