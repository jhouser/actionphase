import { describe, it, expect } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { CommentWithParentCard } from '../CommentWithParentCard';
import type { CommentWithParent } from '../../types/messages';
import { renderWithProviders } from '../../test-utils/render';
import { useGameContext } from '../../contexts/GameContext';
import { useAuth } from '../../contexts/AuthContext';
import { apiClient } from '../../lib/api';
import * as useScreenshotModeHook from '../../hooks/useScreenshotMode';

vi.mock('../../hooks/useScreenshotMode', () => ({
  useScreenshotMode: vi.fn(),
}));

vi.mock('../MarkdownPreview', () => ({
  MarkdownPreview: ({ content }: { content: string }) => <div>{content}</div>,
}));

vi.mock('../../contexts/GameContext', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../contexts/GameContext')>();
  return {
    ...actual,
    useGameContext: vi.fn(),
  };
});

vi.mock('../../contexts/AuthContext', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../contexts/AuthContext')>();
  return {
    ...actual,
    useAuth: vi.fn(),
  };
});

vi.mock('../../lib/api', () => ({
  apiClient: {
    messages: {
      createComment: vi.fn(),
    },
  },
}));

const mockGameContext = {
  game: null,
  gameId: 1,
  participants: [],
  isLoadingGame: false,
  isLoadingParticipants: false,
  isLoadingCharacters: false,
  isLoadingAllCharacters: false,
  userRole: 'player' as const,
  isGM: false,
  isParticipant: true,
  isInGame: true,
  canEditGame: false,
  userCharacters: [],
  allGameCharacters: [],
  currentPhaseId: null,
  isUserCharacter: () => false,
  refetchGameData: vi.fn(),
  refetchAllGameCharacters: vi.fn(),
};

const mockControllableCharacter = {
  id: 99,
  name: 'My Character',
  username: 'myuser',
  character_type: 'player' as const,
  avatar_url: null,
};

describe('CommentWithParentCard', () => {
  beforeEach(() => {
    vi.mocked(useGameContext).mockReturnValue(mockGameContext as never);
    vi.mocked(useAuth).mockReturnValue({ currentUser: null } as never);
    vi.mocked(useScreenshotModeHook.useScreenshotMode).mockReturnValue({
      screenshotModeEnabled: false,
      toggleScreenshotMode: vi.fn(),
    });
  });

  const mockComment: CommentWithParent = {
    id: 1,
    game_id: 1,
    parent_id: 100,
    author_id: 10,
    character_id: 20,
    content: 'This is a test comment',
    created_at: '2025-10-22T10:00:00Z',
    updated_at: '2025-10-22T10:00:00Z',
    edited_at: null,
    edit_count: 0,
    deleted_at: null,
    is_deleted: false,
    author_username: 'testuser',
    character_name: 'Test Character',
    parent_content: 'Parent post content',
    parent_created_at: '2025-10-22T09:00:00Z',
    parent_deleted_at: null,
    parent_is_deleted: false,
    parent_message_type: 'post',
    parent_author_username: 'parentuser',
    parent_character_name: 'Parent Character',
  };

  it('renders comment with parent preview', () => {
    renderWithProviders(<CommentWithParentCard comment={mockComment} gameId={1} />, { gameId: 1 });

    // Check comment content
    expect(screen.getByText('This is a test comment')).toBeInTheDocument();
    expect(screen.getByText('Test Character')).toBeInTheDocument();
    expect(screen.getByText(/testuser/i)).toBeInTheDocument();

    // Check parent preview
    expect(screen.getByText('Parent post content')).toBeInTheDocument();
  });

  it('shows "Edited" badge when comment has been edited', () => {
    const editedComment = {
      ...mockComment,
      edit_count: 2,
      edited_at: '2025-10-22T11:00:00Z',
    };

    renderWithProviders(<CommentWithParentCard comment={editedComment} gameId={1} />, { gameId: 1 });

    expect(screen.getByText('Edited')).toBeInTheDocument();
  });

  it('does not show "Edited" badge when comment has not been edited', () => {
    renderWithProviders(<CommentWithParentCard comment={mockComment} gameId={1} />, { gameId: 1 });

    expect(screen.queryByText('Edited')).not.toBeInTheDocument();
  });

  it('shows deleted marker when comment is deleted', () => {
    const deletedComment = {
      ...mockComment,
      is_deleted: true,
      deleted_at: '2025-10-22T12:00:00Z',
    };

    renderWithProviders(<CommentWithParentCard comment={deletedComment} gameId={1} />, { gameId: 1 });

    expect(screen.getByText('[deleted]')).toBeInTheDocument();
    expect(screen.queryByText('This is a test comment')).not.toBeInTheDocument();
  });

  it('hides "View in thread" button when comment is deleted', () => {
    const deletedComment = {
      ...mockComment,
      is_deleted: true,
    };

    const mockNavigate = vi.fn();
    renderWithProviders(
      <CommentWithParentCard
        comment={deletedComment}
        gameId={1}
        onNavigateToComment={mockNavigate}
      />,
      { gameId: 1 }
    );

    expect(screen.queryByText(/view in thread/i)).not.toBeInTheDocument();
  });

  it('shows "View in thread" button when onNavigateToComment is provided', () => {
    const mockNavigate = vi.fn();
    renderWithProviders(
      <CommentWithParentCard
        comment={mockComment}
        gameId={1}
        onNavigateToComment={mockNavigate}
      />,
      { gameId: 1 }
    );

    expect(screen.getByText(/view in thread/i)).toBeInTheDocument();
  });

  it('hides "View in thread" button when onNavigateToComment is not provided', () => {
    renderWithProviders(<CommentWithParentCard comment={mockComment} gameId={1} />, { gameId: 1 });

    expect(screen.queryByText(/view in thread/i)).not.toBeInTheDocument();
  });

  it('calls onNavigateToComment when "View in thread" button is clicked', async () => {
    const user = userEvent.setup();
    const mockNavigate = vi.fn();

    renderWithProviders(
      <CommentWithParentCard
        comment={mockComment}
        gameId={1}
        onNavigateToComment={mockNavigate}
      />,
      { gameId: 1 }
    );

    const button = screen.getByText(/view in thread/i);
    await user.click(button);

    expect(mockNavigate).toHaveBeenCalledTimes(1);
  });

  it('does not show "view in thread" inside the parent preview (suppressed by hideViewInThread)', () => {
    const mockNavigate = vi.fn();

    renderWithProviders(
      <CommentWithParentCard
        comment={mockComment}
        gameId={1}
        onNavigateToParent={mockNavigate}
        onNavigateToComment={mockNavigate}
      />,
      { gameId: 1 }
    );

    // "View in thread" appears only once — at the card level, not inside the parent preview
    const links = screen.getAllByText(/view in thread/i);
    expect(links).toHaveLength(1);
  });

  it('formats timestamp as relative time', () => {
    const oneHourAgo = new Date(Date.now() - 60 * 60 * 1000).toISOString();
    const recentComment = {
      ...mockComment,
      created_at: oneHourAgo,
    };

    renderWithProviders(<CommentWithParentCard comment={recentComment} gameId={1} />, { gameId: 1 });

    expect(screen.getByText(/1 hour ago/i)).toBeInTheDocument();
  });

  it('handles missing character name gracefully', () => {
    const commentWithoutCharacter = {
      ...mockComment,
      character_name: null,
    };

    renderWithProviders(<CommentWithParentCard comment={commentWithoutCharacter} gameId={1} />, { gameId: 1 });

    expect(screen.getByText('Unknown')).toBeInTheDocument();
  });

  it('applies hover shadow effect class', () => {
    const { container } = renderWithProviders(<CommentWithParentCard comment={mockComment} gameId={1} />, { gameId: 1 });

    const card = container.querySelector('.hover\\:shadow-md');
    expect(card).toBeInTheDocument();
  });

  it('renders "View in thread" as proper anchor tag with href', () => {
    const mockNavigate = vi.fn();
    renderWithProviders(
      <CommentWithParentCard
        comment={mockComment}
        gameId={2}
        onNavigateToComment={mockNavigate}
      />,
      { gameId: 2 }
    );

    const link = screen.getByText(/view in thread/i).closest('a');
    expect(link).not.toBeNull();
    expect(link).toHaveAttribute('href', '/games/2?tab=common-room&comment=1');
  });

  describe('Reply button', () => {
    const commentWithPostId: CommentWithParent = { ...mockComment, post_id: 42 };

    it('shows Reply button when user has a controllable character and comment has post_id', () => {
      vi.mocked(useGameContext).mockReturnValue({
        ...mockGameContext,
        userCharacters: [mockControllableCharacter],
      } as never);

      renderWithProviders(<CommentWithParentCard comment={commentWithPostId} gameId={1} />, { gameId: 1 });

      expect(screen.getByRole('button', { name: /reply/i })).toBeInTheDocument();
    });

    it('hides Reply button when user has no controllable characters', () => {
      renderWithProviders(<CommentWithParentCard comment={commentWithPostId} gameId={1} />, { gameId: 1 });

      expect(screen.queryByRole('button', { name: /reply/i })).not.toBeInTheDocument();
    });

    it('hides Reply button when comment is deleted', () => {
      vi.mocked(useGameContext).mockReturnValue({
        ...mockGameContext,
        userCharacters: [mockControllableCharacter],
      } as never);
      const deletedComment = { ...commentWithPostId, is_deleted: true };

      renderWithProviders(<CommentWithParentCard comment={deletedComment} gameId={1} />, { gameId: 1 });

      expect(screen.queryByRole('button', { name: /reply/i })).not.toBeInTheDocument();
    });

    it('shows reply form when Reply button is clicked', async () => {
      const user = userEvent.setup();
      vi.mocked(useGameContext).mockReturnValue({
        ...mockGameContext,
        userCharacters: [mockControllableCharacter],
      } as never);

      renderWithProviders(<CommentWithParentCard comment={commentWithPostId} gameId={1} />, { gameId: 1 });

      await user.click(screen.getByRole('button', { name: /reply/i }));

      expect(screen.getByPlaceholderText('Write a reply...')).toBeInTheDocument();
    });

    it('submits reply via apiClient and resets form', async () => {
      const user = userEvent.setup();
      vi.mocked(useGameContext).mockReturnValue({
        ...mockGameContext,
        userCharacters: [mockControllableCharacter],
      } as never);
      vi.mocked(apiClient.messages.createComment).mockResolvedValue({ data: {} } as never);

      renderWithProviders(<CommentWithParentCard comment={commentWithPostId} gameId={1} />, { gameId: 1 });

      await user.click(screen.getByRole('button', { name: /reply/i }));
      await user.type(screen.getByPlaceholderText('Write a reply...'), 'Hello there');
      await user.click(screen.getByRole('button', { name: /^reply$/i }));

      await waitFor(() => {
        expect(apiClient.messages.createComment).toHaveBeenCalledWith(
          1,
          commentWithPostId.id,
          expect.objectContaining({
            character_id: mockControllableCharacter.id,
            content: 'Hello there',
            root_post_id: 42,
          })
        );
        expect(screen.queryByPlaceholderText('Write a reply...')).not.toBeInTheDocument();
      });
    });

    it('shows inline confirmation with "View in thread" link after successful reply', async () => {
      const user = userEvent.setup();
      vi.mocked(useGameContext).mockReturnValue({
        ...mockGameContext,
        userCharacters: [mockControllableCharacter],
      } as never);
      vi.mocked(apiClient.messages.createComment).mockResolvedValue({ data: { id: 999 } } as never);

      renderWithProviders(
        <CommentWithParentCard
          comment={commentWithPostId}
          gameId={1}
          onNavigateToComment={vi.fn()}
        />,
        { gameId: 1 }
      );

      await user.click(screen.getByRole('button', { name: /reply/i }));
      await user.type(screen.getByPlaceholderText('Write a reply...'), 'Hello there');
      await user.click(screen.getByRole('button', { name: /^reply$/i }));

      await waitFor(() => {
        expect(screen.getByText('Reply posted!')).toBeInTheDocument();
        // Confirmation banner adds a second "View in thread →" link
        expect(screen.getAllByText('View in thread →').length).toBeGreaterThanOrEqual(2);
      });
    });
  });

  describe('Edit button', () => {
    const commentWithPostId: CommentWithParent = { ...mockComment, post_id: 42 };

    it('shows Edit button when user is the author', () => {
      vi.mocked(useAuth).mockReturnValue({ currentUser: { id: mockComment.author_id } } as never);

      renderWithProviders(<CommentWithParentCard comment={commentWithPostId} gameId={1} />, { gameId: 1 });

      expect(screen.getByRole('button', { name: /edit this comment/i })).toBeInTheDocument();
    });

    it('hides Edit button for non-authors', () => {
      vi.mocked(useAuth).mockReturnValue({ currentUser: { id: 999 } } as never);

      renderWithProviders(<CommentWithParentCard comment={commentWithPostId} gameId={1} />, { gameId: 1 });

      expect(screen.queryByRole('button', { name: /edit this comment/i })).not.toBeInTheDocument();
    });

    it('shows edit form when Edit button is clicked', async () => {
      const user = userEvent.setup();
      vi.mocked(useAuth).mockReturnValue({ currentUser: { id: mockComment.author_id } } as never);

      renderWithProviders(<CommentWithParentCard comment={commentWithPostId} gameId={1} />, { gameId: 1 });

      await user.click(screen.getByRole('button', { name: /edit this comment/i }));

      expect(screen.getByRole('button', { name: /^save$/i })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /^cancel$/i })).toBeInTheDocument();
    });
  });

  describe('Delete button', () => {
    const commentWithPostId: CommentWithParent = { ...mockComment, post_id: 42 };

    it('shows Delete button when user is the author', () => {
      vi.mocked(useAuth).mockReturnValue({ currentUser: { id: mockComment.author_id } } as never);

      renderWithProviders(<CommentWithParentCard comment={commentWithPostId} gameId={1} />, { gameId: 1 });

      expect(screen.getByRole('button', { name: /delete this comment/i })).toBeInTheDocument();
    });

    it('shows Delete button when user is GM', () => {
      vi.mocked(useGameContext).mockReturnValue({
        ...mockGameContext,
        isGM: true,
      } as never);

      renderWithProviders(<CommentWithParentCard comment={commentWithPostId} gameId={1} />, { gameId: 1 });

      expect(screen.getByRole('button', { name: /delete this comment/i })).toBeInTheDocument();
    });

    it('hides Delete button for non-author non-GM users', () => {
      vi.mocked(useAuth).mockReturnValue({ currentUser: { id: 999 } } as never);

      renderWithProviders(<CommentWithParentCard comment={commentWithPostId} gameId={1} />, { gameId: 1 });

      expect(screen.queryByRole('button', { name: /delete this comment/i })).not.toBeInTheDocument();
    });

    it('shows confirmation modal when Delete button is clicked', async () => {
      const user = userEvent.setup();
      vi.mocked(useAuth).mockReturnValue({ currentUser: { id: mockComment.author_id } } as never);

      renderWithProviders(<CommentWithParentCard comment={commentWithPostId} gameId={1} />, { gameId: 1 });

      await user.click(screen.getByRole('button', { name: /delete this comment/i }));

      expect(screen.getByText('Delete Comment')).toBeInTheDocument();
    });
  });

  describe('Copy link button', () => {
    it('shows Copy link button on all non-deleted comments', () => {
      renderWithProviders(<CommentWithParentCard comment={mockComment} gameId={1} />, { gameId: 1 });

      expect(screen.getByRole('button', { name: /copy link to this comment/i })).toBeInTheDocument();
    });
  });

  describe('Screenshot Mode', () => {
    // ParentCommentPreview only falls back to showing @authorUsername when the
    // parent has no character name, so clear it to exercise that render path.
    const commentWithUsernameOnlyParent: CommentWithParent = {
      ...mockComment,
      parent_character_name: null,
    };

    it('shows the comment author and parent author usernames when disabled', () => {
      renderWithProviders(<CommentWithParentCard comment={commentWithUsernameOnlyParent} gameId={1} />, { gameId: 1 });

      expect(screen.getByText(/testuser/i)).toBeInTheDocument();
      expect(screen.getByText(/parentuser/i)).toBeInTheDocument();
    });

    it('hides the comment author and parent author usernames when enabled', () => {
      vi.mocked(useScreenshotModeHook.useScreenshotMode).mockReturnValue({
        screenshotModeEnabled: true,
        toggleScreenshotMode: vi.fn(),
      });

      renderWithProviders(<CommentWithParentCard comment={commentWithUsernameOnlyParent} gameId={1} />, { gameId: 1 });

      expect(screen.queryByText(/testuser/i)).not.toBeInTheDocument();
      expect(screen.queryByText(/parentuser/i)).not.toBeInTheDocument();
      // Character name remains visible — only the real username is hidden.
      expect(screen.getByText('Test Character')).toBeInTheDocument();
    });

    const commentWithPostId: CommentWithParent = { ...mockComment, post_id: 42 };

    it('shows the Edit and Delete buttons for the comment author when disabled', () => {
      vi.mocked(useAuth).mockReturnValue({ currentUser: { id: mockComment.author_id } } as never);

      renderWithProviders(<CommentWithParentCard comment={commentWithPostId} gameId={1} />, { gameId: 1 });

      expect(screen.getByRole('button', { name: /edit this comment/i })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /delete this comment/i })).toBeInTheDocument();
    });

    // Edit/Delete only render for the author, so leaving them visible identifies
    // which character the screenshotter plays just as plainly as the username.
    it('hides the Edit and Delete buttons when enabled', () => {
      vi.mocked(useAuth).mockReturnValue({ currentUser: { id: mockComment.author_id } } as never);
      vi.mocked(useScreenshotModeHook.useScreenshotMode).mockReturnValue({
        screenshotModeEnabled: true,
        toggleScreenshotMode: vi.fn(),
      });

      renderWithProviders(<CommentWithParentCard comment={commentWithPostId} gameId={1} />, { gameId: 1 });

      expect(screen.queryByRole('button', { name: /edit this comment/i })).not.toBeInTheDocument();
      expect(screen.queryByRole('button', { name: /delete this comment/i })).not.toBeInTheDocument();
      // The comment itself still renders — only the authorship tells are hidden.
      expect(screen.getByText('This is a test comment')).toBeInTheDocument();
    });
  });
});
