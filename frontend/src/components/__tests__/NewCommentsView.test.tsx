import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { BrowserRouter } from 'react-router-dom';
import type { UseInfiniteQueryResult } from '@tanstack/react-query';
import { NewCommentsView } from '../NewCommentsView';
import * as useRecentCommentsModule from '../../hooks/useRecentComments';
import type { CommentWithParent } from '../../types/messages';

// Mock the useRecentComments hook
vi.mock('../../hooks/useRecentComments');

// Mock read tracking hooks
vi.mock('../../hooks/useReadTracking', () => ({
  useManualReadCommentIDs: () => ({ data: [], refetch: vi.fn() }),
  useToggleCommentRead: () => ({ mutate: vi.fn() }),
}));

// Read mode is mutable per-test: the unread-only filter is manual-mode only.
let mockReadMode: 'auto' | 'manual' = 'auto';
vi.mock('../../hooks/useUserPreferences', () => ({
  useCommentReadMode: () => mockReadMode,
}));

// Mock navigate function
const mockNavigate = vi.fn();

// Mock react-router-dom
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

// Mock the CommentWithParentCard component to expose callback props
vi.mock('../CommentWithParentCard', () => ({
  CommentWithParentCard: ({
    comment,
    onNavigateToParent,
    onNavigateToComment
  }: {
    comment: CommentWithParent;
    onNavigateToParent: () => void;
    onNavigateToComment: () => void;
  }) => (
    <div data-testid={`comment-${comment.id}`}>
      {comment.content}
      <button data-testid={`navigate-to-comment-${comment.id}`} onClick={onNavigateToComment}>
        View Comment
      </button>
      <button data-testid={`navigate-to-parent-${comment.id}`} onClick={onNavigateToParent}>
        View Parent
      </button>
    </div>
  ),
}));

// Mock IntersectionObserver
const mockIntersectionObserver = vi.fn();
mockIntersectionObserver.mockReturnValue({
  observe: vi.fn(),
  unobserve: vi.fn(),
  disconnect: vi.fn(),
});
window.IntersectionObserver = mockIntersectionObserver as unknown as typeof IntersectionObserver;

describe('NewCommentsView', () => {
  const mockComment: CommentWithParent = {
    id: 1,
    game_id: 1,
    parent_id: 100,
    author_id: 10,
    character_id: 20,
    content: 'Test comment',
    created_at: '2025-10-22T10:00:00Z',
    updated_at: '2025-10-22T10:00:00Z',
    edited_at: null,
    edit_count: 0,
    deleted_at: null,
    is_deleted: false,
    author_username: 'testuser',
    character_name: 'Test Character',
    parent_content: 'Parent content',
    parent_created_at: '2025-10-22T09:00:00Z',
    parent_deleted_at: null,
    parent_is_deleted: false,
    parent_message_type: 'post',
    parent_author_username: 'parentuser',
    parent_character_name: 'Parent Character',
  };

  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <BrowserRouter>{children}</BrowserRouter>
  );

  beforeEach(() => {
    vi.clearAllMocks();
    mockReadMode = 'auto';
    // The unread-only filter persists to localStorage; clear it so each test
    // starts from the default rather than inheriting the previous test's toggle.
    localStorage.clear();
  });

  it('shows loading spinner while loading', () => {
    vi.mocked(useRecentCommentsModule.useRecentComments).mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      error: null,
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
      refetch: vi.fn(),
    } as Partial<UseInfiniteQueryResult<CommentWithParent[], Error>>);

    render(<NewCommentsView gameId={1} />, { wrapper });

    expect(screen.getByRole('status')).toBeInTheDocument();
  });

  it('shows error message when loading fails', () => {
    const error = new Error('Failed to load');
    vi.mocked(useRecentCommentsModule.useRecentComments).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      error,
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
      refetch: vi.fn(),
    } as Partial<UseInfiniteQueryResult<CommentWithParent[], Error>>);

    render(<NewCommentsView gameId={1} />, { wrapper });

    expect(screen.getByText(/failed to load recent comments/i)).toBeInTheDocument();
    expect(screen.getByText('Failed to load')).toBeInTheDocument();
  });

  it('shows empty state when there are no comments', () => {
    vi.mocked(useRecentCommentsModule.useRecentComments).mockReturnValue({
      data: { pages: [] },
      isLoading: false,
      isError: false,
      error: null,
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
      refetch: vi.fn(),
    } as Partial<UseInfiniteQueryResult<CommentWithParent[], Error>>);

    render(<NewCommentsView gameId={1} />, { wrapper });

    expect(screen.getByText(/no comments yet/i)).toBeInTheDocument();
    expect(screen.getByText(/be the first to start a conversation/i)).toBeInTheDocument();
  });

  it('renders list of comments', () => {
    const comments = [
      { ...mockComment, id: 1, content: 'Comment 1' },
      { ...mockComment, id: 2, content: 'Comment 2' },
      { ...mockComment, id: 3, content: 'Comment 3' },
    ];

    vi.mocked(useRecentCommentsModule.useRecentComments).mockReturnValue({
      data: {
        pages: [{ comments, total: 3, limit: 20, offset: 0 }],
      },
      isLoading: false,
      isError: false,
      error: null,
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
      refetch: vi.fn(),
    } as Partial<UseInfiniteQueryResult<CommentWithParent[], Error>>);

    render(<NewCommentsView gameId={1} />, { wrapper });

    expect(screen.getByTestId('comment-1')).toBeInTheDocument();
    expect(screen.getByTestId('comment-2')).toBeInTheDocument();
    expect(screen.getByTestId('comment-3')).toBeInTheDocument();
  });

  it('flattens multiple pages of comments', () => {
    const page1Comments = [
      { ...mockComment, id: 1, content: 'Comment 1' },
      { ...mockComment, id: 2, content: 'Comment 2' },
    ];

    const page2Comments = [
      { ...mockComment, id: 3, content: 'Comment 3' },
      { ...mockComment, id: 4, content: 'Comment 4' },
    ];

    vi.mocked(useRecentCommentsModule.useRecentComments).mockReturnValue({
      data: {
        pages: [
          { comments: page1Comments, total: 4, limit: 2, offset: 0 },
          { comments: page2Comments, total: 4, limit: 2, offset: 2 },
        ],
      },
      isLoading: false,
      isError: false,
      error: null,
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
      refetch: vi.fn(),
    } as Partial<UseInfiniteQueryResult<CommentWithParent[], Error>>);

    render(<NewCommentsView gameId={1} />, { wrapper });

    expect(screen.getByTestId('comment-1')).toBeInTheDocument();
    expect(screen.getByTestId('comment-2')).toBeInTheDocument();
    expect(screen.getByTestId('comment-3')).toBeInTheDocument();
    expect(screen.getByTestId('comment-4')).toBeInTheDocument();
  });

  it('shows "No more comments" when all pages loaded', () => {
    vi.mocked(useRecentCommentsModule.useRecentComments).mockReturnValue({
      data: {
        pages: [{ comments: [mockComment], total: 1, limit: 20, offset: 0 }],
      },
      isLoading: false,
      isError: false,
      error: null,
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
      refetch: vi.fn(),
    } as Partial<UseInfiniteQueryResult<CommentWithParent[], Error>>);

    render(<NewCommentsView gameId={1} />, { wrapper });

    expect(screen.getByText(/no more comments to load/i)).toBeInTheDocument();
  });

  it('shows loading spinner when fetching next page', () => {
    vi.mocked(useRecentCommentsModule.useRecentComments).mockReturnValue({
      data: {
        pages: [{ comments: [mockComment], total: 20, limit: 20, offset: 0 }],
      },
      isLoading: false,
      isError: false,
      error: null,
      fetchNextPage: vi.fn(),
      hasNextPage: true,
      isFetchingNextPage: true,
      refetch: vi.fn(),
    } as Partial<UseInfiniteQueryResult<CommentWithParent[], Error>>);

    render(<NewCommentsView gameId={1} />, { wrapper });

    // There should be 2 spinners: one for the sentinel and potentially one in the loading state
    const spinners = screen.getAllByRole('status');
    expect(spinners.length).toBeGreaterThan(0);
  });

  it('sets up intersection observer when hasNextPage is true', () => {
    const mockObserve = vi.fn();
    mockIntersectionObserver.mockReturnValue({
      observe: mockObserve,
      unobserve: vi.fn(),
      disconnect: vi.fn(),
    });

    vi.mocked(useRecentCommentsModule.useRecentComments).mockReturnValue({
      data: {
        pages: [{ comments: [mockComment], total: 20, limit: 20, offset: 0 }],
      },
      isLoading: false,
      isError: false,
      error: null,
      fetchNextPage: vi.fn(),
      hasNextPage: true,
      isFetchingNextPage: false,
      refetch: vi.fn(),
    } as Partial<UseInfiniteQueryResult<CommentWithParent[], Error>>);

    render(<NewCommentsView gameId={1} />, { wrapper });

    // Wait for useEffect to run
    waitFor(() => {
      expect(mockIntersectionObserver).toHaveBeenCalled();
    });
  });

  it('does not set up intersection observer when hasNextPage is false', () => {
    const mockObserve = vi.fn();
    mockIntersectionObserver.mockReturnValue({
      observe: mockObserve,
      unobserve: vi.fn(),
      disconnect: vi.fn(),
    });

    vi.mocked(useRecentCommentsModule.useRecentComments).mockReturnValue({
      data: {
        pages: [{ comments: [mockComment], total: 1, limit: 20, offset: 0 }],
      },
      isLoading: false,
      isError: false,
      error: null,
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
      refetch: vi.fn(),
    } as Partial<UseInfiniteQueryResult<CommentWithParent[], Error>>);

    render(<NewCommentsView gameId={1} />, { wrapper });

    // Observer should not be set up if there's no next page
    expect(mockObserve).not.toHaveBeenCalled();
  });

  it('handles unknown error gracefully', () => {
    vi.mocked(useRecentCommentsModule.useRecentComments).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      error: 'String error',
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
      refetch: vi.fn(),
    } as Partial<UseInfiniteQueryResult<CommentWithParent[], Error>>);

    render(<NewCommentsView gameId={1} />, { wrapper });

    expect(screen.getByText(/failed to load recent comments/i)).toBeInTheDocument();
    expect(screen.getByText('Unknown error')).toBeInTheDocument();
  });

  describe('Deep Linking', () => {
    beforeEach(() => {
      mockNavigate.mockClear();
    });

    it('generates correct deep link to comment when "View Comment" is clicked', () => {
      vi.mocked(useRecentCommentsModule.useRecentComments).mockReturnValue({
        data: {
          pages: [{ comments: [mockComment], total: 1, limit: 20, offset: 0 }],
        },
        isLoading: false,
        isError: false,
        error: null,
        fetchNextPage: vi.fn(),
        hasNextPage: false,
        isFetchingNextPage: false,
        refetch: vi.fn(),
      } as Partial<UseInfiniteQueryResult<CommentWithParent[], Error>>);

      render(<NewCommentsView gameId={1} />, { wrapper });

      // Click the "View Comment" button from the mocked card
      const viewCommentButton = screen.getByTestId('navigate-to-comment-1');
      viewCommentButton.click();

      // Verify navigate was called with correct deep link
      expect(mockNavigate).toHaveBeenCalledWith('/games/1?tab=common-room&comment=1');
    });

    it('generates correct deep link to parent comment when "View Parent" is clicked', () => {
      vi.mocked(useRecentCommentsModule.useRecentComments).mockReturnValue({
        data: {
          pages: [{ comments: [mockComment], total: 1, limit: 20, offset: 0 }],
        },
        isLoading: false,
        isError: false,
        error: null,
        fetchNextPage: vi.fn(),
        hasNextPage: false,
        isFetchingNextPage: false,
        refetch: vi.fn(),
      } as Partial<UseInfiniteQueryResult<CommentWithParent[], Error>>);

      render(<NewCommentsView gameId={1} />, { wrapper });

      // Click the "View Parent" button from the mocked card
      const viewParentButton = screen.getByTestId('navigate-to-parent-1');
      viewParentButton.click();

      // Verify navigate was called with correct deep link to parent
      expect(mockNavigate).toHaveBeenCalledWith('/games/1?tab=common-room&comment=100');
    });

    it('does not navigate to parent when parent_id is null', () => {
      const commentWithoutParent = { ...mockComment, parent_id: null };
      vi.mocked(useRecentCommentsModule.useRecentComments).mockReturnValue({
        data: {
          pages: [{ comments: [commentWithoutParent], total: 1, limit: 20, offset: 0 }],
        },
        isLoading: false,
        isError: false,
        error: null,
        fetchNextPage: vi.fn(),
        hasNextPage: false,
        isFetchingNextPage: false,
        refetch: vi.fn(),
      } as Partial<UseInfiniteQueryResult<CommentWithParent[], Error>>);

      render(<NewCommentsView gameId={1} />, { wrapper });

      // Click the "View Parent" button
      const viewParentButton = screen.getByTestId('navigate-to-parent-1');
      viewParentButton.click();

      // Verify navigate was NOT called when parent_id is null
      expect(mockNavigate).not.toHaveBeenCalled();
    });

    it('generates correct deep links for multiple comments', () => {
      const comments = [
        { ...mockComment, id: 10, parent_id: 100, content: 'First comment' },
        { ...mockComment, id: 20, parent_id: 200, content: 'Second comment' },
      ];

      vi.mocked(useRecentCommentsModule.useRecentComments).mockReturnValue({
        data: {
          pages: [{ comments, total: 2, limit: 20, offset: 0 }],
        },
        isLoading: false,
        isError: false,
        error: null,
        fetchNextPage: vi.fn(),
        hasNextPage: false,
        isFetchingNextPage: false,
        refetch: vi.fn(),
      } as Partial<UseInfiniteQueryResult<CommentWithParent[], Error>>);

      render(<NewCommentsView gameId={1} />, { wrapper });

      // Click first comment's "View Comment" button
      screen.getByTestId('navigate-to-comment-10').click();
      expect(mockNavigate).toHaveBeenCalledWith('/games/1?tab=common-room&comment=10');

      mockNavigate.mockClear();

      // Click second comment's "View Parent" button
      screen.getByTestId('navigate-to-parent-20').click();
      expect(mockNavigate).toHaveBeenCalledWith('/games/1?tab=common-room&comment=200');
    });
  });

  describe('Refresh Button', () => {
    it('renders refresh button with comments', () => {
      const refetch = vi.fn();
      vi.mocked(useRecentCommentsModule.useRecentComments).mockReturnValue({
        data: {
          pages: [{ comments: [mockComment], total: 1, limit: 20, offset: 0 }],
        },
        isLoading: false,
        isError: false,
        error: null,
        fetchNextPage: vi.fn(),
        hasNextPage: false,
        isFetchingNextPage: false,
        refetch,
      } as Partial<UseInfiniteQueryResult<CommentWithParent[], Error>>);

      render(<NewCommentsView gameId={1} />, { wrapper });

      expect(screen.getByRole('button', { name: /refresh/i })).toBeInTheDocument();
    });

    it('calls refetch when refresh button is clicked', async () => {
      const refetch = vi.fn().mockResolvedValue({});
      vi.mocked(useRecentCommentsModule.useRecentComments).mockReturnValue({
        data: {
          pages: [{ comments: [mockComment], total: 1, limit: 20, offset: 0 }],
        },
        isLoading: false,
        isError: false,
        error: null,
        fetchNextPage: vi.fn(),
        hasNextPage: false,
        isFetchingNextPage: false,
        refetch,
      } as Partial<UseInfiniteQueryResult<CommentWithParent[], Error>>);

      render(<NewCommentsView gameId={1} />, { wrapper });

      const refreshButton = screen.getByRole('button', { name: /refresh/i });
      refreshButton.click();

      await waitFor(() => {
        expect(refetch).toHaveBeenCalledTimes(1);
      });
    });

    it('disables refresh button while refreshing', async () => {
      const refetch = vi.fn().mockImplementation(
        () => new Promise((resolve) => setTimeout(resolve, 100))
      );
      vi.mocked(useRecentCommentsModule.useRecentComments).mockReturnValue({
        data: {
          pages: [{ comments: [mockComment], total: 1, limit: 20, offset: 0 }],
        },
        isLoading: false,
        isError: false,
        error: null,
        fetchNextPage: vi.fn(),
        hasNextPage: false,
        isFetchingNextPage: false,
        refetch,
      } as Partial<UseInfiniteQueryResult<CommentWithParent[], Error>>);

      render(<NewCommentsView gameId={1} />, { wrapper });

      const refreshButton = screen.getByRole('button', { name: /refresh/i });
      refreshButton.click();

      // Button should be disabled during refresh
      await waitFor(() => {
        expect(refreshButton).toBeDisabled();
      });

      // Wait for refresh to complete
      await waitFor(() => {
        expect(refreshButton).not.toBeDisabled();
      });
    });

    it('shows "Refreshing..." text while refreshing', async () => {
      const refetch = vi.fn().mockImplementation(
        () => new Promise((resolve) => setTimeout(resolve, 100))
      );
      vi.mocked(useRecentCommentsModule.useRecentComments).mockReturnValue({
        data: {
          pages: [{ comments: [mockComment], total: 1, limit: 20, offset: 0 }],
        },
        isLoading: false,
        isError: false,
        error: null,
        fetchNextPage: vi.fn(),
        hasNextPage: false,
        isFetchingNextPage: false,
        refetch,
      } as Partial<UseInfiniteQueryResult<CommentWithParent[], Error>>);

      render(<NewCommentsView gameId={1} />, { wrapper });

      const refreshButton = screen.getByRole('button', { name: /refresh/i });
      refreshButton.click();

      // Should show "Refreshing..." text
      await waitFor(() => {
        expect(screen.getByText(/refreshing/i)).toBeInTheDocument();
      });

      // Wait for refresh to complete and text to return to "Refresh"
      await waitFor(() => {
        expect(screen.getByRole('button', { name: /refresh/i })).toBeInTheDocument();
      });
    });

    it('disables refresh button while initially loading', () => {
      const refetch = vi.fn();
      vi.mocked(useRecentCommentsModule.useRecentComments).mockReturnValue({
        data: undefined,
        isLoading: true,
        isError: false,
        error: null,
        fetchNextPage: vi.fn(),
        hasNextPage: false,
        isFetchingNextPage: false,
        refetch,
      } as Partial<UseInfiniteQueryResult<CommentWithParent[], Error>>);

      render(<NewCommentsView gameId={1} />, { wrapper });

      // Loading spinner should be shown, but let's check if we can find any button
      // In this case, the component shows loading state before rendering the refresh button
      // So we expect the button to not be in the document during initial load
      expect(screen.queryByRole('button', { name: /refresh/i })).not.toBeInTheDocument();
    });
  });

  describe('unread-only filter', () => {
    const loadedComments = (comments: CommentWithParent[]) =>
      ({
        data: { pages: [{ comments, total: comments.length, limit: 20, offset: 0 }] },
        isLoading: false,
        isError: false,
        error: null,
        fetchNextPage: vi.fn(),
        hasNextPage: false,
        isFetchingNextPage: false,
        refetch: vi.fn(),
      }) as Partial<UseInfiniteQueryResult<CommentWithParent[], Error>>;

    it('does not offer the filter in auto read mode', () => {
      mockReadMode = 'auto';
      vi.mocked(useRecentCommentsModule.useRecentComments).mockReturnValue(
        loadedComments([{ ...mockComment, id: 1 }])
      );

      render(<NewCommentsView gameId={1} />, { wrapper });

      expect(screen.queryByRole('switch', { name: /unread only/i })).not.toBeInTheDocument();
    });

    it('offers the filter in manual read mode, off by default', () => {
      mockReadMode = 'manual';
      vi.mocked(useRecentCommentsModule.useRecentComments).mockReturnValue(
        loadedComments([{ ...mockComment, id: 1 }])
      );

      render(<NewCommentsView gameId={1} />, { wrapper });

      const toggle = screen.getByRole('switch', { name: /unread only/i });
      expect(toggle).toBeInTheDocument();
      expect(toggle).not.toBeChecked();
      // Unfiltered until the user opts in
      expect(useRecentCommentsModule.useRecentComments).toHaveBeenCalledWith(1, false);
    });

    it('restores a previously enabled filter on mount', () => {
      mockReadMode = 'manual';
      localStorage.setItem('newCommentsUnreadOnly', JSON.stringify({ 1: true }));
      vi.mocked(useRecentCommentsModule.useRecentComments).mockReturnValue(
        loadedComments([{ ...mockComment, id: 1 }])
      );

      render(<NewCommentsView gameId={1} />, { wrapper });

      // Filter is on without the user touching it, and the first fetch is filtered
      expect(screen.getByRole('switch', { name: /unread only/i })).toBeChecked();
      expect(useRecentCommentsModule.useRecentComments).toHaveBeenCalledWith(1, true);
    });

    it('does not apply another game\'s stored filter', () => {
      mockReadMode = 'manual';
      localStorage.setItem('newCommentsUnreadOnly', JSON.stringify({ 99: true }));
      vi.mocked(useRecentCommentsModule.useRecentComments).mockReturnValue(
        loadedComments([{ ...mockComment, id: 1 }])
      );

      render(<NewCommentsView gameId={1} />, { wrapper });

      expect(screen.getByRole('switch', { name: /unread only/i })).not.toBeChecked();
      expect(useRecentCommentsModule.useRecentComments).toHaveBeenCalledWith(1, false);
    });

    it('requests only unread comments once the filter is enabled', async () => {
      const user = userEvent.setup();
      mockReadMode = 'manual';
      vi.mocked(useRecentCommentsModule.useRecentComments).mockReturnValue(
        loadedComments([{ ...mockComment, id: 1 }])
      );

      render(<NewCommentsView gameId={1} />, { wrapper });

      await user.click(screen.getByLabelText(/unread only/i));

      expect(screen.getByLabelText(/unread only/i)).toBeChecked();
      expect(useRecentCommentsModule.useRecentComments).toHaveBeenLastCalledWith(1, true);
    });

    it('stays unfiltered in auto mode even though the filter state exists', () => {
      mockReadMode = 'auto';
      vi.mocked(useRecentCommentsModule.useRecentComments).mockReturnValue(
        loadedComments([{ ...mockComment, id: 1 }])
      );

      render(<NewCommentsView gameId={1} />, { wrapper });

      expect(useRecentCommentsModule.useRecentComments).toHaveBeenCalledWith(1, false);
    });

    it('shows an "all read" empty state when the filter hides everything', async () => {
      const user = userEvent.setup();
      mockReadMode = 'manual';

      // Comments exist unfiltered; once filtered, the server returns none.
      vi.mocked(useRecentCommentsModule.useRecentComments).mockImplementation(
        (_gameId, unreadOnly) =>
          loadedComments(
            unreadOnly ? [] : [{ ...mockComment, id: 1 }]
          ) as UseInfiniteQueryResult<CommentWithParent[], Error>
      );

      render(<NewCommentsView gameId={1} />, { wrapper });
      await user.click(screen.getByLabelText(/unread only/i));

      expect(screen.getByText(/no unread comments/i)).toBeInTheDocument();
      // Must not claim the game has no comments at all
      expect(screen.queryByText(/be the first to start a conversation/i)).not.toBeInTheDocument();
    });

    it('keeps the toggle and refresh button usable in the "all read" empty state', async () => {
      const user = userEvent.setup();
      mockReadMode = 'manual';

      vi.mocked(useRecentCommentsModule.useRecentComments).mockImplementation(
        (_gameId, unreadOnly) =>
          loadedComments(
            unreadOnly ? [] : [{ ...mockComment, id: 1, content: 'Only comment' }]
          ) as UseInfiniteQueryResult<CommentWithParent[], Error>
      );

      render(<NewCommentsView gameId={1} />, { wrapper });
      await user.click(screen.getByLabelText(/unread only/i));

      // Empty state, but the controls stay available: refresh again to poll for
      // newly-arrived unread comments without leaving the filter.
      expect(screen.getByText(/no unread comments/i)).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /refresh/i })).toBeEnabled();
      expect(screen.getByLabelText(/unread only/i)).toBeChecked();

      // The dead-end "escape hatch" button is gone; the toggle is the way back.
      expect(screen.queryByRole('button', { name: /show all comments/i })).not.toBeInTheDocument();
    });

    it('lets the user clear the filter from the empty state via the toggle', async () => {
      const user = userEvent.setup();
      mockReadMode = 'manual';

      vi.mocked(useRecentCommentsModule.useRecentComments).mockImplementation(
        (_gameId, unreadOnly) =>
          loadedComments(
            unreadOnly ? [] : [{ ...mockComment, id: 1, content: 'Only comment' }]
          ) as UseInfiniteQueryResult<CommentWithParent[], Error>
      );

      render(<NewCommentsView gameId={1} />, { wrapper });
      await user.click(screen.getByLabelText(/unread only/i));
      expect(screen.getByText(/no unread comments/i)).toBeInTheDocument();

      await user.click(screen.getByLabelText(/unread only/i));

      expect(screen.getByTestId('comment-1')).toBeInTheDocument();
      expect(screen.getByLabelText(/unread only/i)).not.toBeChecked();
    });

    it('refetches from the empty state and shows newly-arrived unread comments', async () => {
      const user = userEvent.setup();
      mockReadMode = 'manual';

      let unreadArrived = false;
      vi.mocked(useRecentCommentsModule.useRecentComments).mockImplementation(
        (_gameId, unreadOnly) => {
          const result = loadedComments(
            unreadOnly
              ? unreadArrived
                ? [{ ...mockComment, id: 2, content: 'Fresh unread' }]
                : []
              : [{ ...mockComment, id: 1, content: 'Only comment' }]
          ) as UseInfiniteQueryResult<CommentWithParent[], Error>;
          return {
            ...result,
            refetch: vi.fn(async () => {
              unreadArrived = true;
              return result;
            }),
          } as UseInfiniteQueryResult<CommentWithParent[], Error>;
        }
      );

      const { rerender } = render(<NewCommentsView gameId={1} />, { wrapper });
      await user.click(screen.getByLabelText(/unread only/i));
      expect(screen.getByText(/no unread comments/i)).toBeInTheDocument();

      await user.click(screen.getByRole('button', { name: /refresh/i }));
      rerender(<NewCommentsView gameId={1} />);

      await waitFor(() => expect(screen.getByTestId('comment-2')).toBeInTheDocument());
      expect(screen.queryByText(/no unread comments/i)).not.toBeInTheDocument();
    });

    it('renders the filtered list the server returns', async () => {
      const user = userEvent.setup();
      mockReadMode = 'manual';

      vi.mocked(useRecentCommentsModule.useRecentComments).mockImplementation(
        (_gameId, unreadOnly) =>
          loadedComments(
            unreadOnly
              ? [{ ...mockComment, id: 2, content: 'Unread' }]
              : [
                  { ...mockComment, id: 1, content: 'Read' },
                  { ...mockComment, id: 2, content: 'Unread' },
                ]
          ) as UseInfiniteQueryResult<CommentWithParent[], Error>
      );

      render(<NewCommentsView gameId={1} />, { wrapper });
      expect(screen.getByTestId('comment-1')).toBeInTheDocument();

      await user.click(screen.getByLabelText(/unread only/i));

      expect(screen.queryByTestId('comment-1')).not.toBeInTheDocument();
      expect(screen.getByTestId('comment-2')).toBeInTheDocument();
    });
  });
});
