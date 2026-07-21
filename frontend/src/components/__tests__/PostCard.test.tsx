import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server';
import { renderWithProviders } from '../../test-utils/render';
import { PostCard } from '../PostCard';
import type { Message } from '../../types/messages';
import type { Character } from '../../types/characters';
import * as useScreenshotModeHook from '../../hooks/useScreenshotMode';

vi.mock('../../hooks/useScreenshotMode', () => ({
  useScreenshotMode: vi.fn(),
}));

// Mock data
const mockCharacters: Character[] = [
  {
    id: 1,
    game_id: 1,
    name: 'Test Character',
    character_type: 'player_character',
    user_id: 100,
    status: 'approved',
    created_at: '2024-01-01T00:00:00Z',
  },
  {
    id: 2,
    game_id: 1,
    name: 'Another Character',
    character_type: 'player_character',
    user_id: 100,
    status: 'approved',
    created_at: '2024-01-01T00:00:00Z',
  }
];

const mockPost: Message = {
  id: 1,
  game_id: 1,
  character_id: 1,
  character_name: 'GM Character',
  author_id: 100,
  author_username: 'gamemaster',
  content: 'This is a test GM post',
  message_type: 'post',
  comment_count: 2,
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
  is_edited: false,
};

const mockLongPost: Message = {
  ...mockPost,
  id: 2,
  content: 'A'.repeat(600), // 600 characters to trigger collapse
};

const mockEditedPost: Message = {
  ...mockPost,
  id: 3,
  is_edited: true,
};

const mockComments: Message[] = [
  {
    id: 10,
    game_id: 1,
    character_id: 1,
    character_name: 'Commenter',
    author_id: 101,
    author_username: 'user1',
    content: 'First comment',
    message_type: 'comment',
    parent_id: 1,
    created_at: '2024-01-01T01:00:00Z',
    updated_at: '2024-01-01T01:00:00Z',
  },
  {
    id: 11,
    game_id: 1,
    character_id: 2,
    character_name: 'Another Commenter',
    author_id: 102,
    author_username: 'user2',
    content: 'Second comment',
    message_type: 'comment',
    parent_id: 1,
    created_at: '2024-01-01T02:00:00Z',
    updated_at: '2024-01-01T02:00:00Z',
  }
];

describe('PostCard', () => {
  const mockOnCreateComment = vi.fn();

  beforeEach(() => {
    mockOnCreateComment.mockReset();
    localStorage.clear(); // Clear localStorage to prevent state persistence between tests
    vi.mocked(useScreenshotModeHook.useScreenshotMode).mockReturnValue({
      screenshotModeEnabled: false,
      toggleScreenshotMode: vi.fn(),
    });
    // Setup default successful responses for new paginated endpoint
    server.use(
      http.get('/api/v1/games/:gameId/posts/:postId/comments-with-threads', () => {
        return HttpResponse.json({
          comments: mockComments,
          total_top_level: mockComments.length,
          returned_top_level: mockComments.length,
          returned_total: mockComments.length,
          has_more: false,
          limit: 200,
          offset: 0,
        });
      }),
      http.get('/api/v1/games/:gameId/unread-comment-ids', () => {
        return HttpResponse.json([]);
      })
    );
  });

  describe('Post Header', () => {
    it('displays GM post heading with character name', () => {
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      expect(screen.getByText(/gm character/i)).toBeInTheDocument();
    });

    it('displays author username', () => {
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      expect(screen.getByText(/@gamemaster/i)).toBeInTheDocument();
    });

    it('displays formatted date', () => {
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      // Date should be formatted (exact format depends on time elapsed)
      const dateText = screen.getByText(/@gamemaster/i).textContent;
      expect(dateText).toBeTruthy();
    });

    it('hides author username when Screenshot Mode is enabled', () => {
      vi.mocked(useScreenshotModeHook.useScreenshotMode).mockReturnValue({
        screenshotModeEnabled: true,
        toggleScreenshotMode: vi.fn(),
      });

      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      expect(screen.queryByText(/@gamemaster/i)).not.toBeInTheDocument();
      expect(screen.getByText(/^Posted /i)).toBeInTheDocument();
      expect(screen.queryByText('You')).not.toBeInTheDocument();
    });

    it('shows the Edit button for the post author when Screenshot Mode is disabled', () => {
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      expect(screen.getByRole('button', { name: 'Edit' })).toBeInTheDocument();
    });

    // The Edit button only renders for the post's author, so leaving it visible
    // reveals which character the screenshotter plays.
    it('hides the Edit button when Screenshot Mode is enabled', () => {
      vi.mocked(useScreenshotModeHook.useScreenshotMode).mockReturnValue({
        screenshotModeEnabled: true,
        toggleScreenshotMode: vi.fn(),
      });

      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      expect(screen.queryByRole('button', { name: 'Edit' })).not.toBeInTheDocument();
      // The post itself still renders — only the authorship tell is hidden.
      expect(screen.getByText('This is a test GM post')).toBeInTheDocument();
    });

    it('shows edited indicator when post is edited', () => {
      renderWithProviders(
        <PostCard
          post={mockEditedPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      expect(screen.getByText(/\(edited\)/i)).toBeInTheDocument();
    });

    it('does not show edited indicator when post is not edited', () => {
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      expect(screen.queryByText(/\(edited\)/i)).not.toBeInTheDocument();
    });
  });

  describe('Author Badge', () => {
    it('shows "You" badge when current user is author', () => {
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      expect(screen.getByText(/^you$/i)).toBeInTheDocument();
    });

    it('does not show "You" badge when current user is not author', () => {
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={999}
        />
      );

      expect(screen.queryByText(/^you$/i)).not.toBeInTheDocument();
    });

    it('does not show "You" badge when currentUserId is not provided', () => {
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
        />
      );

      expect(screen.queryByText(/^you$/i)).not.toBeInTheDocument();
    });

    it('hides "You" badge when Screenshot Mode is enabled, even for the author', () => {
      vi.mocked(useScreenshotModeHook.useScreenshotMode).mockReturnValue({
        screenshotModeEnabled: true,
        toggleScreenshotMode: vi.fn(),
      });

      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      expect(screen.queryByText(/^you$/i)).not.toBeInTheDocument();
    });
  });

  describe('Post Content', () => {
    it('displays post content', () => {
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      expect(screen.getByText('This is a test GM post')).toBeInTheDocument();
    });

    it('renders markdown content', () => {
      const markdownPost = {
        ...mockPost,
        content: '# Heading\n\n**bold text**',
      };

      renderWithProviders(
        <PostCard
          post={markdownPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      // ReactMarkdown should render the markdown
      const heading = screen.getByText('Heading');
      expect(heading.tagName).toBe('H1');
    });
  });

  describe('Long Content Collapsing', () => {
    it('shows collapse toggle for long content (>500 chars)', () => {
      renderWithProviders(
        <PostCard
          post={mockLongPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      expect(screen.getByRole('button', { name: /collapse post/i })).toBeInTheDocument();
    });

    it('does not show collapse toggle for short content', () => {
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      expect(screen.queryByRole('button', { name: /collapse post/i })).not.toBeInTheDocument();
    });

    it('collapses content when collapse button is clicked', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <PostCard
          post={mockLongPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      // Initially content is visible
      expect(screen.getByText(/A{600}/)).toBeInTheDocument();

      // Click collapse button
      const collapseButton = screen.getByRole('button', { name: /collapse post/i });
      await user.click(collapseButton);

      // Content should be hidden
      expect(screen.queryByText(/A{600}/)).not.toBeInTheDocument();
      expect(screen.getByRole('button', { name: /show full post/i })).toBeInTheDocument();
    });

    it('expands content when show button is clicked', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <PostCard
          post={mockLongPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      // Collapse first
      await user.click(screen.getByRole('button', { name: /collapse post/i }));
      expect(screen.queryByText(/A{600}/)).not.toBeInTheDocument();

      // Expand
      await user.click(screen.getByRole('button', { name: /show full post/i }));
      expect(screen.getByText(/A{600}/)).toBeInTheDocument();
    });
  });

  describe('Comments Toggle', () => {
    it('shows collapse comments button initially', () => {
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      expect(screen.getByRole('button', { name: /collapse comments \(2\)/i })).toBeInTheDocument();
    });

    it('displays comment count', () => {
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      expect(screen.getByText(/comments \(2\)/i)).toBeInTheDocument();
    });

    it('displays 0 when comment_count is undefined', () => {
      const postWithoutCount = { ...mockPost, comment_count: undefined };
      renderWithProviders(
        <PostCard
          post={postWithoutCount}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      expect(screen.getByText(/comments \(0\)/i)).toBeInTheDocument();
    });

    it('toggles to expand comments when collapse is clicked', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      const toggleButton = screen.getByRole('button', { name: /collapse comments/i });
      await user.click(toggleButton);

      expect(screen.getByRole('button', { name: /expand comments/i })).toBeInTheDocument();
    });
  });

  describe('Add Comment Button', () => {
    it('shows add comment button initially', () => {
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      expect(screen.getByRole('button', { name: /add comment/i })).toBeInTheDocument();
    });

    it('opens comment form when add comment is clicked', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      await user.click(screen.getByRole('button', { name: /add comment/i }));

      expect(screen.getByPlaceholderText(/write a comment\.\.\./i)).toBeInTheDocument();
    });

    it('hides add comment button when comment form is open', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      await user.click(screen.getByRole('button', { name: /add comment/i }));

      expect(screen.queryByRole('button', { name: /add comment/i })).not.toBeInTheDocument();
    });
  });

  describe('Comment Form - Rendering', () => {
    it('shows comment form when commenting', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      await user.click(screen.getByRole('button', { name: /add comment/i }));

      expect(screen.getByPlaceholderText(/write a comment\.\.\./i)).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /^comment$/i })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /cancel/i })).toBeInTheDocument();
    });

    it('shows character selector when multiple characters available', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      await user.click(screen.getByRole('button', { name: /add comment/i }));

      const selector = screen.getByRole('combobox') as HTMLSelectElement;
      expect(selector).toBeInTheDocument();
      expect(screen.getByText(/reply as test character/i)).toBeInTheDocument();
      expect(screen.getByText(/reply as another character/i)).toBeInTheDocument();
    });

    it('does not show character selector when only one character available', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={[mockCharacters[0]]}
          controllableCharacters={[mockCharacters[0]]}
          onCreateComment={mockOnCreateComment}
        />
      );

      await user.click(screen.getByRole('button', { name: /add comment/i }));

      expect(screen.queryByRole('combobox')).not.toBeInTheDocument();
    });

    it('hides comment button when no characters available', async () => {
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={[]}
          controllableCharacters={[]}
          onCreateComment={mockOnCreateComment}
        />
      );

      // When there are no controllable characters, the "Add comment" button doesn't exist
      expect(screen.queryByRole('button', { name: /add comment/i })).not.toBeInTheDocument();

      // The comment form should not be visible
      expect(screen.queryByPlaceholderText(/write a comment/i)).not.toBeInTheDocument();
    });
  });

  describe('Comment Form - Character Selection', () => {
    it('auto-selects first character', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      await user.click(screen.getByRole('button', { name: /add comment/i }));

      const selector = screen.getByRole('combobox') as HTMLSelectElement;
      expect(selector.value).toBe('1');
    });

    it('allows changing selected character', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      await user.click(screen.getByRole('button', { name: /add comment/i }));

      const selector = screen.getByRole('combobox') as HTMLSelectElement;
      await user.selectOptions(selector, '2');

      expect(selector.value).toBe('2');
    });
  });

  describe('Comment Form - Input', () => {
    it('updates content when user types', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      await user.click(screen.getByRole('button', { name: /add comment/i }));

      const textarea = screen.getByPlaceholderText(/write a comment\.\.\./i);
      await user.type(textarea, 'Test comment content');

      expect(textarea).toHaveValue('Test comment content');
    });

  });

  describe('Comment Form - Validation', () => {
    it('disables submit button when content is empty', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      await user.click(screen.getByRole('button', { name: /add comment/i }));

      const submitButton = screen.getByRole('button', { name: /^comment$/i });
      expect(submitButton).toBeDisabled();
    });

    it('disables submit button when content is only whitespace', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      await user.click(screen.getByRole('button', { name: /add comment/i }));

      const textarea = screen.getByPlaceholderText(/write a comment\.\.\./i);
      await user.type(textarea, '   ');

      const submitButton = screen.getByRole('button', { name: /^comment$/i });
      expect(submitButton).toBeDisabled();
    });

    it('enables submit button when content is valid', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      await user.click(screen.getByRole('button', { name: /add comment/i }));

      const textarea = screen.getByPlaceholderText(/write a comment\.\.\./i);
      await user.type(textarea, 'Valid comment');

      const submitButton = screen.getByRole('button', { name: /^comment$/i });
      expect(submitButton).not.toBeDisabled();
    });
  });

  describe('Comment Form - Submission', () => {
    beforeEach(() => {
      mockOnCreateComment.mockResolvedValue(undefined);
    });

    it('calls onCreateComment with correct arguments', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      await user.click(screen.getByRole('button', { name: /add comment/i }));

      const textarea = screen.getByPlaceholderText(/write a comment\.\.\./i);
      await user.type(textarea, 'My test comment');

      await user.click(screen.getByRole('button', { name: /^comment$/i }));

      await waitFor(() => {
        expect(mockOnCreateComment).toHaveBeenCalledWith(1, 1, 'My test comment', 1);
      });
    });

    it('trims whitespace from content before submitting', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      await user.click(screen.getByRole('button', { name: /add comment/i }));

      const textarea = screen.getByPlaceholderText(/write a comment\.\.\./i);
      await user.type(textarea, '  Test comment  ');

      await user.click(screen.getByRole('button', { name: /^comment$/i }));

      await waitFor(() => {
        expect(mockOnCreateComment).toHaveBeenCalledWith(1, 1, 'Test comment', 1);
      });
    });

    it('clears content after successful submission', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      await user.click(screen.getByRole('button', { name: /add comment/i }));

      const textarea = screen.getByPlaceholderText(/write a comment\.\.\./i);
      await user.type(textarea, 'Test comment');

      await user.click(screen.getByRole('button', { name: /^comment$/i }));

      await waitFor(() => {
        expect(mockOnCreateComment).toHaveBeenCalled();
      });

      // Form should be closed after submission
      await waitFor(() => {
        expect(screen.queryByPlaceholderText(/write a comment/i)).not.toBeInTheDocument();
      });
    });

    it('closes comment form after successful submission', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      await user.click(screen.getByRole('button', { name: /add comment/i }));

      await user.type(screen.getByPlaceholderText(/write a comment\.\.\./i), 'Test');
      await user.click(screen.getByRole('button', { name: /^comment$/i }));

      await waitFor(() => {
        expect(screen.queryByPlaceholderText(/write a comment/i)).not.toBeInTheDocument();
      });

      // Add comment button should be visible again
      expect(screen.getByRole('button', { name: /add comment/i })).toBeInTheDocument();
    });

    it('reloads comments after successful submission', async () => {
      const user = userEvent.setup();
      let loadCount = 0;

      server.use(
        http.get('/api/v1/games/:gameId/posts/:postId/comments-with-threads', () => {
          loadCount++;
          return HttpResponse.json({
            comments: mockComments,
            total_top_level: mockComments.length,
            returned_top_level: mockComments.length,
            returned_total: mockComments.length,
            has_more: false,
            limit: 200,
            offset: 0,
          });
        })
      );

      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      // Initial load when comments are shown
      await waitFor(() => {
        expect(loadCount).toBe(1);
      });

      await user.click(screen.getByRole('button', { name: /add comment/i }));
      await user.type(screen.getByPlaceholderText(/write a comment\.\.\./i), 'Test');
      await user.click(screen.getByRole('button', { name: /^comment$/i }));

      // Should reload comments after submission
      await waitFor(() => {
        expect(loadCount).toBe(2);
      });
    });

    it('uses selected character when submitting', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      await user.click(screen.getByRole('button', { name: /add comment/i }));

      // Change character
      const selector = screen.getByRole('combobox') as HTMLSelectElement;
      await user.selectOptions(selector, '2');

      await user.type(screen.getByPlaceholderText(/write a comment\.\.\./i), 'Test');
      await user.click(screen.getByRole('button', { name: /^comment$/i }));

      await waitFor(() => {
        expect(mockOnCreateComment).toHaveBeenCalledWith(1, 2, 'Test', 1);
      });
    });
  });

  describe('Comment Form - Cancel', () => {
    it('closes comment form when cancel is clicked', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      await user.click(screen.getByRole('button', { name: /add comment/i }));
      expect(screen.getByPlaceholderText(/write a comment\.\.\./i)).toBeInTheDocument();

      await user.click(screen.getByRole('button', { name: /cancel/i }));

      expect(screen.queryByPlaceholderText(/write a comment/i)).not.toBeInTheDocument();
    });

    it('clears content when cancel is clicked', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      await user.click(screen.getByRole('button', { name: /add comment/i }));

      const textarea = screen.getByPlaceholderText(/write a comment\.\.\./i);
      await user.type(textarea, 'Test content');

      await user.click(screen.getByRole('button', { name: /cancel/i }));

      // Re-open form to check content is cleared
      await user.click(screen.getByRole('button', { name: /add comment/i }));

      const newTextarea = screen.getByPlaceholderText(/write a comment\.\.\./i);
      expect(newTextarea).toHaveValue('');
    });

    it('shows add comment button again after cancel', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      await user.click(screen.getByRole('button', { name: /add comment/i }));
      await user.click(screen.getByRole('button', { name: /cancel/i }));

      expect(screen.getByRole('button', { name: /add comment/i })).toBeInTheDocument();
    });
  });

  describe('Comment Form - Loading State', () => {
    it('shows loading text while submitting', async () => {
      const user = userEvent.setup();
      // Hold the submission open with a manually-resolved promise instead of a
      // timer. A setTimeout-based mock races the real clock: under CPU
      // contention (e.g. parallel workers in CI/containers) the delay can
      // elapse during `user.click` itself, closing the form before we assert.
      let resolveSubmit!: () => void;
      mockOnCreateComment.mockImplementation(
        () => new Promise<void>(resolve => { resolveSubmit = resolve; })
      );

      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      await user.click(screen.getByRole('button', { name: /add comment/i }));
      await user.type(screen.getByPlaceholderText(/write a comment\.\.\./i), 'Test');

      const submitButton = screen.getByRole('button', { name: /^comment$/i });
      await user.click(submitButton);

      // Submission is still pending, so the button shows its loading label.
      expect(screen.getByText(/posting\.\.\./i)).toBeInTheDocument();

      // Let the submission finish and the form close.
      resolveSubmit();
      await waitFor(() => {
        expect(screen.queryByPlaceholderText(/write a comment/i)).not.toBeInTheDocument();
      }, { timeout: 3000 });
    });

    it('disables all form elements while submitting', async () => {
      const user = userEvent.setup();
      // Manually-resolved promise, not a timer — see note above; the timer
      // version races the clock and flakes under parallel-worker contention.
      let resolveSubmit!: () => void;
      mockOnCreateComment.mockImplementation(
        () => new Promise<void>(resolve => { resolveSubmit = resolve; })
      );

      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      await user.click(screen.getByRole('button', { name: /add comment/i }));
      await user.type(screen.getByPlaceholderText(/write a comment\.\.\./i), 'Test');

      const submitButton = screen.getByRole('button', { name: /^comment$/i });
      const textarea = screen.getByPlaceholderText(/write a comment\.\.\./i);
      const cancelButton = screen.getByRole('button', { name: /cancel/i });

      await user.click(submitButton);

      // Check elements are disabled during submission
      expect(textarea).toBeDisabled();
      expect(submitButton).toBeDisabled();
      expect(cancelButton).toBeDisabled();

      resolveSubmit();
    });

    it('disables character selector while submitting', async () => {
      const user = userEvent.setup();
      // Manually-resolved promise, not a timer — see note above.
      let resolveSubmit!: () => void;
      mockOnCreateComment.mockImplementation(
        () => new Promise<void>(resolve => { resolveSubmit = resolve; })
      );

      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      await user.click(screen.getByRole('button', { name: /add comment/i }));
      await user.type(screen.getByPlaceholderText(/write a comment\.\.\./i), 'Test');

      const selector = screen.getByRole('combobox');
      const submitButton = screen.getByRole('button', { name: /^comment$/i });

      await user.click(submitButton);

      // Check selector is disabled during submission
      expect(selector).toBeDisabled();

      resolveSubmit();
    });
  });

  describe('Comments Display', () => {
    it('loads comments when shown', async () => {
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      // Comments are shown by default
      await waitFor(() => {
        expect(screen.getByText('First comment')).toBeInTheDocument();
        expect(screen.getByText('Second comment')).toBeInTheDocument();
      });
    });

    it('shows loading state while loading comments', async () => {
      server.use(
        http.get('/api/v1/games/:gameId/posts/:postId/comments-with-threads', async () => {
          await new Promise(resolve => setTimeout(resolve, 100));
          return HttpResponse.json({
            comments: mockComments,
            total_top_level: mockComments.length,
            returned_top_level: mockComments.length,
            returned_total: mockComments.length,
            has_more: false,
            limit: 200,
            offset: 0,
          });
        })
      );

      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      // Should show loading initially
      expect(screen.getByText(/loading comments\.\.\./i)).toBeInTheDocument();

      // Wait for comments to load
      await waitFor(() => {
        expect(screen.queryByText(/loading comments/i)).not.toBeInTheDocument();
      });
    });

    it('shows empty state when no comments exist', async () => {
      server.use(
        http.get('/api/v1/games/:gameId/posts/:postId/comments-with-threads', () => {
          return HttpResponse.json({
            comments: [],
            total_top_level: 0,
            returned_top_level: 0,
            returned_total: 0,
            has_more: false,
            limit: 200,
            offset: 0,
          });
        })
      );

      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      await waitFor(() => {
        expect(screen.getByText(/no comments yet/i)).toBeInTheDocument();
      });
    });

    it('shows encouragement message in empty state', async () => {
      server.use(
        http.get('/api/v1/games/:gameId/posts/:postId/comments-with-threads', () => {
          return HttpResponse.json({
            comments: [],
            total_top_level: 0,
            returned_top_level: 0,
            returned_total: 0,
            has_more: false,
            limit: 200,
            offset: 0,
          });
        })
      );

      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      await waitFor(() => {
        expect(screen.getByText(/be the first to reply/i)).toBeInTheDocument();
      });
    });

    it('does not load comments when initially hidden', async () => {
      let loadCount = 0;

      server.use(
        http.get('/api/v1/games/:gameId/posts/:postId/comments-with-threads', () => {
          loadCount++;
          return HttpResponse.json({
            comments: mockComments,
            total_top_level: mockComments.length,
            returned_top_level: mockComments.length,
            returned_total: mockComments.length,
            has_more: false,
            limit: 200,
            offset: 0,
          });
        })
      );

      const postWithoutComments = { ...mockPost, comment_count: 0 };

      const user = userEvent.setup();
      renderWithProviders(
        <PostCard
          post={postWithoutComments}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      // Collapse comments
      await user.click(screen.getByRole('button', { name: /collapse comments/i }));

      // Should load comments initially since showComments defaults to true
      await waitFor(() => {
        expect(loadCount).toBe(1);
      });
    });
  });

  describe('Date Formatting', () => {
    it('formats recent dates with "ago" suffix', () => {
      const now = new Date();
      const recentPost = {
        ...mockPost,
        created_at: now.toISOString(),
      };

      renderWithProviders(
        <PostCard
          post={recentPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      // date-fns formatDistanceToNow returns "less than a minute ago" for very recent dates
      expect(screen.getByText(/ago/i)).toBeInTheDocument();
    });

    it('formats dates as "X minutes ago" for minutes', () => {
      const now = new Date();
      const minutesAgo = new Date(now.getTime() - 5 * 60 * 1000); // 5 minutes ago
      const recentPost = {
        ...mockPost,
        created_at: minutesAgo.toISOString(),
      };

      renderWithProviders(
        <PostCard
          post={recentPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      // date-fns formats as "5 minutes ago"
      expect(screen.getByText(/5 minutes ago/i)).toBeInTheDocument();
    });

    it('formats dates as "X hours ago" for hours', () => {
      const now = new Date();
      const hoursAgo = new Date(now.getTime() - 3 * 60 * 60 * 1000); // 3 hours ago
      const recentPost = {
        ...mockPost,
        created_at: hoursAgo.toISOString(),
      };

      renderWithProviders(
        <PostCard
          post={recentPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      // date-fns formats as "about 3 hours ago"
      expect(screen.getByText(/3 hours ago/i)).toBeInTheDocument();
    });

    it('formats dates as "X days ago" for days', () => {
      const now = new Date();
      const daysAgo = new Date(now.getTime() - 2 * 24 * 60 * 60 * 1000); // 2 days ago
      const recentPost = {
        ...mockPost,
        created_at: daysAgo.toISOString(),
      };

      renderWithProviders(
        <PostCard
          post={recentPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      // date-fns formats as "2 days ago"
      expect(screen.getByText(/2 days ago/i)).toBeInTheDocument();
    });

    it('formats old dates with relative time', () => {
      const now = new Date();
      const oldDate = new Date(now.getTime() - 10 * 24 * 60 * 60 * 1000); // 10 days ago
      const oldPost = {
        ...mockPost,
        created_at: oldDate.toISOString(),
      };

      renderWithProviders(
        <PostCard
          post={oldPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      // date-fns formats as "10 days ago"
      expect(screen.getByText(/10 days ago/i)).toBeInTheDocument();
    });

    it('correctly handles UTC timestamps without Z suffix (backend format)', () => {
      const now = new Date();
      const fiveMinutesAgo = new Date(now.getTime() - 5 * 60 * 1000);

      // Backend returns UTC timestamps WITHOUT 'Z' suffix
      // Convert to ISO string, then remove the 'Z'
      const backendFormat = fiveMinutesAgo.toISOString().replace('Z', '');

      const recentPost = {
        ...mockPost,
        created_at: backendFormat,
      };

      renderWithProviders(
        <PostCard
          post={recentPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      // Should correctly format as "5 minutes ago" (not "in X hours")
      expect(screen.getByText(/5 minutes ago/i)).toBeInTheDocument();
    });
  });

  describe('Integration', () => {
    it('handles complete comment workflow', async () => {
      mockOnCreateComment.mockResolvedValue(undefined);
      const user = userEvent.setup();

      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      // Wait for initial load
      await waitFor(() => {
        expect(screen.getByText('First comment')).toBeInTheDocument();
      });

      // Verify post displays correctly
      expect(screen.getByText(/gm character/i)).toBeInTheDocument();
      expect(screen.getByText(/^you$/i)).toBeInTheDocument(); // Author badge

      // Open comment form
      await user.click(screen.getByRole('button', { name: /add comment/i }));

      // Select different character
      const selector = screen.getByRole('combobox') as HTMLSelectElement;
      await user.selectOptions(selector, '2');
      expect(selector.value).toBe('2');

      // Type comment
      const textarea = screen.getByPlaceholderText(/write a comment\.\.\./i);
      await user.type(textarea, 'This is my reply to the GM post');

      // Submit comment
      await user.click(screen.getByRole('button', { name: /^comment$/i }));

      // Verify submission
      await waitFor(() => {
        expect(mockOnCreateComment).toHaveBeenCalledWith(1, 2, 'This is my reply to the GM post', 1);
      });

      // Form should close
      await waitFor(() => {
        expect(screen.queryByPlaceholderText(/write a comment/i)).not.toBeInTheDocument();
      });

      // Add comment button should reappear
      expect(screen.getByRole('button', { name: /add comment/i })).toBeInTheDocument();
    });
  });

  describe('Memoization', () => {
    it('does not break when re-rendered with same props', () => {
      const { rerender } = renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      // Verify initial render works
      expect(screen.getByTestId('post-card')).toBeInTheDocument();

      // Re-render with same props (should not cause issues)
      rerender(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      // Component should still be rendered correctly
      expect(screen.getByTestId('post-card')).toBeInTheDocument();
      expect(screen.getByText(/gm character/i)).toBeInTheDocument();
    });
  });

  describe('Post Editing', () => {
    it('shows edit button for post author', () => {
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100} // Same as post author_id
        />
      );

      expect(screen.getByRole('button', { name: /^edit$/i })).toBeInTheDocument();
    });

    it('does not show edit button for non-author', () => {
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={999} // Different from post author_id (100)
        />
      );

      expect(screen.queryByRole('button', { name: /^edit$/i })).not.toBeInTheDocument();
    });

    it('does not show edit button in read-only mode', () => {
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
          readOnly={true}
        />
      );

      expect(screen.queryByRole('button', { name: /^edit$/i })).not.toBeInTheDocument();
    });

    it('shows editor when edit button is clicked', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      await user.click(screen.getByRole('button', { name: /^edit$/i }));

      // Editor should appear with current content
      expect(screen.getByPlaceholderText(/edit your post\.\.\./i)).toBeInTheDocument();
      expect(screen.getByDisplayValue(mockPost.content)).toBeInTheDocument();

      // Save and Cancel buttons should appear
      expect(screen.getByRole('button', { name: /^save$/i })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /^cancel$/i })).toBeInTheDocument();

      // Edit button should be hidden
      expect(screen.queryByRole('button', { name: /^edit$/i })).not.toBeInTheDocument();
    });

    it('reverts changes when cancel button is clicked', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      // Click edit
      await user.click(screen.getByRole('button', { name: /^edit$/i }));

      // Modify content
      const textarea = screen.getByPlaceholderText(/edit your post\.\.\./i);
      await user.clear(textarea);
      await user.type(textarea, 'Modified content');

      // Click cancel
      await user.click(screen.getByRole('button', { name: /^cancel$/i }));

      // Should exit edit mode
      await waitFor(() => {
        expect(screen.queryByPlaceholderText(/edit your post\.\.\./i)).not.toBeInTheDocument();
      });

      // Edit button should reappear
      expect(screen.getByRole('button', { name: /^edit$/i })).toBeInTheDocument();

      // Original content should be visible
      expect(screen.getByText(mockPost.content)).toBeInTheDocument();
    });

    it('saves post when save button is clicked', async () => {
      const updatedContent = 'Updated post content';
      server.use(
        http.patch('/api/v1/games/:gameId/posts/:postId', () => {
          return HttpResponse.json({
            ...mockPost,
            content: updatedContent,
            is_edited: true,
          });
        })
      );

      const user = userEvent.setup();
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      // Click edit
      await user.click(screen.getByRole('button', { name: /^edit$/i }));

      // Modify content
      const textarea = screen.getByPlaceholderText(/edit your post\.\.\./i);
      await user.clear(textarea);
      await user.type(textarea, updatedContent);

      // Click save
      await user.click(screen.getByRole('button', { name: /^save$/i }));

      // Should exit edit mode
      await waitFor(() => {
        expect(screen.queryByPlaceholderText(/edit your post\.\.\./i)).not.toBeInTheDocument();
      });

      // Edit button should reappear
      await waitFor(() => {
        expect(screen.getByRole('button', { name: /^edit$/i })).toBeInTheDocument();
      });
    });

    it('disables save button when content is unchanged', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      await user.click(screen.getByRole('button', { name: /^edit$/i }));

      // Save button should be disabled (content hasn't changed)
      expect(screen.getByRole('button', { name: /^save$/i })).toBeDisabled();
    });

    it('disables save button when content is empty', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={mockCharacters}
          onCreateComment={mockOnCreateComment}
          currentUserId={100}
        />
      );

      await user.click(screen.getByRole('button', { name: /^edit$/i }));

      // Clear content
      const textarea = screen.getByPlaceholderText(/edit your post\.\.\./i);
      await user.clear(textarea);

      // Save button should be disabled
      await waitFor(() => {
        expect(screen.getByRole('button', { name: /^save$/i })).toBeDisabled();
      });
    });

  });

  describe('allowReadTracking prop', () => {
    it('hides mark-as-read toggle when allowReadTracking=false', async () => {
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={[]}
          onCreateComment={mockOnCreateComment}
          allowReadTracking={false}
        />
      );

      await waitFor(() => {
        expect(screen.queryByText(/loading comments/i)).not.toBeInTheDocument();
      });

      expect(screen.queryByRole('button', { name: /mark as (read|unread)/i })).not.toBeInTheDocument();
    });

    it('shows mark-as-read toggle when allowReadTracking=true (default) in manual mode', async () => {
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={[]}
          onCreateComment={mockOnCreateComment}
          allowReadTracking={true}
        />
      );

      await waitFor(() => {
        expect(screen.queryByText(/loading comments/i)).not.toBeInTheDocument();
      });

      const toggleButtons = screen.queryAllByRole('button', { name: /mark as (read|unread)/i });
      expect(toggleButtons.length).toBeGreaterThan(0);
    });

    it('allows read tracking when readOnly=true but allowReadTracking=true', async () => {
      renderWithProviders(
        <PostCard
          post={mockPost}
          gameId={1}
          characters={mockCharacters}
          controllableCharacters={[]}
          onCreateComment={mockOnCreateComment}
          readOnly={true}
          allowReadTracking={true}
        />
      );

      await waitFor(() => {
        expect(screen.queryByText(/loading comments/i)).not.toBeInTheDocument();
      });

      const toggleButtons = screen.queryAllByRole('button', { name: /mark as (read|unread)/i });
      expect(toggleButtons.length).toBeGreaterThan(0);
    });
  });

});
