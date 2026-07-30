import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, act } from '@testing-library/react';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { AxiosResponse } from 'axios';
import { PostCard } from './PostCard';
import { ToastProvider } from '../contexts/ToastContext';
import { stubIntersectionObserver } from '../test-utils/mockIntersectionObserver';
import type { Message, CommentWithDepth, PaginatedCommentsResponse } from '@/types/messages';
import type { Character } from '@/types/characters';

// Mock the API client
vi.mock('../lib/api', () => ({
  apiClient: {
    messages: {
      getPostCommentsWithThreads: vi.fn(),
    },
  },
}));

// Import the mocked API after the mock definition
import { apiClient } from '../lib/api';

// Mock other hooks
vi.mock('../hooks/useCommentMutations', () => ({
  useCreateComment: () => ({
    mutateAsync: vi.fn(),
  }),
  useUpdateComment: () => ({
    mutateAsync: vi.fn(),
    isPending: false,
  }),
  useDeleteComment: () => ({
    mutateAsync: vi.fn(),
  }),
}));

vi.mock('../hooks/useAdminMode', () => ({
  useAdminMode: () => ({
    adminModeEnabled: false,
  }),
}));

vi.mock('../hooks/useScreenshotMode', () => ({
  useScreenshotMode: () => ({
    screenshotModeEnabled: false,
    toggleScreenshotMode: vi.fn(),
  }),
}));

vi.mock('../hooks/useGamePermissions', () => ({
  useGamePermissions: () => ({
    isGM: false,
  }),
}));

vi.mock('../hooks', () => ({
  useUpdatePost: () => ({
    mutateAsync: vi.fn(),
  }),
}));

vi.mock('../hooks/useReadTracking', () => ({
  useMarkPostAsRead: () => ({
    mutate: vi.fn(),
    mutateAsync: vi.fn(),
    isPending: false,
    isError: false,
    error: null,
  }),
  usePostUnreadCommentIDs: () => [],
  usePostManualReadCommentIDs: () => [],
  useToggleCommentRead: () => ({ mutate: vi.fn(), isPending: false }),
}));

vi.mock('../hooks/useUserPreferences', () => ({
  useCommentReadMode: () => 'auto',
}));

// Callback-capturing IntersectionObserver stub (shared helper) so tests can
// simulate the infinite-scroll sentinel entering the viewport.
let io: ReturnType<typeof stubIntersectionObserver>;

beforeEach(() => {
  io = stubIntersectionObserver();
});

// Must match THREADS_PER_PAGE in PostCard.tsx
const THREADS_PER_PAGE = 5;

describe('PostCard - Load More Comments', () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    });
    vi.clearAllMocks();
    // Also drop queued mock*Once responses a previous (possibly failed) test
    // didn't consume — clearAllMocks alone leaves them to leak across tests.
    vi.mocked(apiClient.messages.getPostCommentsWithThreads).mockReset();
  });

  const mockPost: Message = {
    id: 1,
    game_id: 1,
    author_id: 1,
    character_id: 1,
    content: 'Test post content',
    message_type: 'post',
    thread_depth: 0,
    author_username: 'testuser',
    character_name: 'Test Character',
    character_avatar_url: null,
    comment_count: 30,
    reply_count: 0,
    is_edited: false,
    is_deleted: false,
    created_at: '2024-01-01T12:00:00Z',
    updated_at: '2024-01-01T12:00:00Z',
  };

  const mockCharacters: Character[] = [
    {
      id: 1,
      name: 'Test Character',
      username: 'testuser',
      character_type: 'player_character',
      avatar_url: null,
    } as Character,
  ];

  const createMockComment = (id: number, depth: number = 0, parentId?: number): CommentWithDepth => ({
    id,
    game_id: 1,
    author_id: 1,
    character_id: 1,
    content: `Comment ${id}`,
    message_type: 'comment',
    parent_id: parentId,
    thread_depth: depth + 1,
    depth,
    author_username: 'testuser',
    character_name: 'Test Character',
    character_avatar_url: null,
    reply_count: 0,
    is_edited: false,
    is_deleted: false,
    created_at: `2024-01-01T12:${String(id).padStart(2, '0')}:00Z`,
    updated_at: `2024-01-01T12:${String(id).padStart(2, '0')}:00Z`,
  });

  const renderPostCard = (
    post: Message,
    onCreateComment: (parentId: number, characterId: number, content: string, rootPostId: number) => Promise<void> = vi.fn().mockResolvedValue(undefined)
  ) => {
    // Data router (not <MemoryRouter>) — CommentEditor's unsaved-changes guard
    // uses useBlocker, which throws outside a data router.
    const router = createMemoryRouter([
      {
        path: '/',
        element: (
          <ToastProvider>
            <PostCard
              post={post}
              gameId={1}
              characters={mockCharacters}
              controllableCharacters={mockCharacters}
              onCreateComment={onCreateComment}
              currentUserId={1}
            />
          </ToastProvider>
        ),
      },
    ]);
    return render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    );
  };

  /** Submits a top-level comment through the form, which triggers the silent refresh. */
  const submitTopLevelComment = async (user: ReturnType<typeof userEvent.setup>) => {
    await user.click(screen.getByRole('button', { name: /Add Comment/i }));
    await user.type(screen.getByPlaceholderText(/Write a comment/i), 'A new comment');
    await user.click(screen.getByRole('button', { name: /^Comment$/ }));
  };

  const deferred = <T,>() => {
    let resolve!: (value: T) => void;
    let reject!: (reason?: unknown) => void;
    const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej; });
    return { promise, resolve, reject };
  };

  it('should show "Load More" button when there are more comments', async () => {
    // Arrange: Mock API to return one page of threads with 15 more remaining
    const mockComments = Array.from({ length: THREADS_PER_PAGE }, (_, i) => createMockComment(i + 1));
    const mockResponse: PaginatedCommentsResponse = {
      comments: mockComments,
      total_top_level: 20,
      returned_top_level: THREADS_PER_PAGE,
      returned_total: THREADS_PER_PAGE,
      has_more: true,
      limit: THREADS_PER_PAGE,
      offset: 0,
    };

    vi.mocked(apiClient.messages.getPostCommentsWithThreads).mockResolvedValue({
      data: mockResponse,
    } as Partial<AxiosResponse<PaginatedCommentsResponse>>);

    // Act
    renderPostCard(mockPost);

    // Assert: Wait for comments to load
    await waitFor(() => {
      expect(screen.getByText(/Load More Comments/i)).toBeInTheDocument();
    });

    // Should show remaining count
    expect(screen.getByText(/15 remaining/i)).toBeInTheDocument();
  });

  it('should hide "Load More" button when all comments are loaded', async () => {
    // Arrange: Mock API to return all comments (no more remaining)
    const mockComments = Array.from({ length: 5 }, (_, i) => createMockComment(i + 1));
    const mockResponse: PaginatedCommentsResponse = {
      comments: mockComments,
      total_top_level: 5,
      returned_top_level: 5,
      returned_total: 5,
      has_more: false,
      limit: THREADS_PER_PAGE,
      offset: 0,
    };

    vi.mocked(apiClient.messages.getPostCommentsWithThreads).mockResolvedValue({
      data: mockResponse,
    } as Partial<AxiosResponse<PaginatedCommentsResponse>>);

    // Act
    renderPostCard(mockPost);

    // Assert: Wait for comments to load
    await waitFor(() => {
      expect(screen.queryByText(/Load More Comments/i)).not.toBeInTheDocument();
    });
  });

  it('should load more comments when "Load More" button is clicked', async () => {
    const user = userEvent.setup();

    // Arrange: Initial load returns one page of threads
    const initialComments = Array.from({ length: THREADS_PER_PAGE }, (_, i) => createMockComment(i + 1));
    const initialResponse: PaginatedCommentsResponse = {
      comments: initialComments,
      total_top_level: 20,
      returned_top_level: THREADS_PER_PAGE,
      returned_total: THREADS_PER_PAGE,
      has_more: true,
      limit: THREADS_PER_PAGE,
      offset: 0,
    };

    // Second load returns remaining 5 comments
    const moreComments = Array.from({ length: 5 }, (_, i) => createMockComment(i + THREADS_PER_PAGE + 1));
    const secondResponse: PaginatedCommentsResponse = {
      comments: moreComments,
      total_top_level: 20,
      returned_top_level: 5,
      returned_total: 5,
      has_more: false,
      limit: THREADS_PER_PAGE,
      offset: THREADS_PER_PAGE,
    };

    vi.mocked(apiClient.messages.getPostCommentsWithThreads)
      .mockResolvedValueOnce({ data: initialResponse } as Partial<AxiosResponse<PaginatedCommentsResponse>>)
      .mockResolvedValueOnce({ data: secondResponse } as Partial<AxiosResponse<PaginatedCommentsResponse>>);

    // Act
    renderPostCard(mockPost);

    // Wait for initial load
    await waitFor(() => {
      expect(screen.getByText(/Load More Comments/i)).toBeInTheDocument();
    });

    // Click Load More button
    const loadMoreButton = screen.getByText(/Load More Comments/i);
    await user.click(loadMoreButton);

    // Assert: Second API call should be made with correct offset
    await waitFor(() => {
      expect(apiClient.messages.getPostCommentsWithThreads).toHaveBeenCalledTimes(2);
      expect(apiClient.messages.getPostCommentsWithThreads).toHaveBeenLastCalledWith(
        1, // gameId
        1, // postId
        THREADS_PER_PAGE, // limit
        THREADS_PER_PAGE, // offset (one page in for the second page)
        5 // maxDepth
      );
    });

    // Button should disappear after loading all comments
    await waitFor(() => {
      expect(screen.queryByText(/Load More Comments/i)).not.toBeInTheDocument();
    });
  });

  it('should show loading state while loading more comments', async () => {
    const user = userEvent.setup();

    // Arrange: Initial load
    const initialComments = Array.from({ length: THREADS_PER_PAGE }, (_, i) => createMockComment(i + 1));
    const initialResponse: PaginatedCommentsResponse = {
      comments: initialComments,
      total_top_level: 20,
      returned_top_level: THREADS_PER_PAGE,
      returned_total: THREADS_PER_PAGE,
      has_more: true,
      limit: THREADS_PER_PAGE,
      offset: 0,
    };

    // Second load with delay
    const moreComments = Array.from({ length: 5 }, (_, i) => createMockComment(i + THREADS_PER_PAGE + 1));
    const secondResponse: PaginatedCommentsResponse = {
      comments: moreComments,
      total_top_level: 20,
      returned_top_level: 5,
      returned_total: 5,
      has_more: false,
      limit: THREADS_PER_PAGE,
      offset: THREADS_PER_PAGE,
    };

    vi.mocked(apiClient.messages.getPostCommentsWithThreads)
      .mockResolvedValueOnce({ data: initialResponse } as Partial<AxiosResponse<PaginatedCommentsResponse>>)
      .mockImplementationOnce(() =>
        new Promise(resolve => setTimeout(() => resolve({ data: secondResponse } as Partial<AxiosResponse<PaginatedCommentsResponse>>), 100))
      );

    // Act
    renderPostCard(mockPost);

    // Wait for initial load
    await waitFor(() => {
      expect(screen.getByText(/Load More Comments/i)).toBeInTheDocument();
    });

    // Get the button element (not just the text)
    const loadMoreText = screen.getByText(/Load More Comments/i);
    const loadMoreButton = loadMoreText.closest('button');
    expect(loadMoreButton).not.toBeNull();

    // Click Load More button
    await user.click(loadMoreButton!);

    // Assert: Button should be disabled during loading
    await waitFor(() => {
      expect(loadMoreButton).toBeDisabled();
    });

    // Wait for loading to complete
    await waitFor(() => {
      expect(screen.queryByText(/Load More Comments/i)).not.toBeInTheDocument();
    });
  });

  it('should append new comments to existing tree when loading more', async () => {
    const user = userEvent.setup();

    // Arrange: Initial 3 comments
    const initialComments = [
      createMockComment(1),
      createMockComment(2),
      createMockComment(3),
    ];
    const initialResponse: PaginatedCommentsResponse = {
      comments: initialComments,
      total_top_level: 5,
      returned_top_level: 3,
      returned_total: 3,
      has_more: true,
      limit: THREADS_PER_PAGE,
      offset: 0,
    };

    // Load 2 more comments
    const moreComments = [
      createMockComment(4),
      createMockComment(5),
    ];
    const secondResponse: PaginatedCommentsResponse = {
      comments: moreComments,
      total_top_level: 5,
      returned_top_level: 2,
      returned_total: 2,
      has_more: false,
      limit: THREADS_PER_PAGE,
      offset: 3,
    };

    vi.mocked(apiClient.messages.getPostCommentsWithThreads)
      .mockResolvedValueOnce({ data: initialResponse } as Partial<AxiosResponse<PaginatedCommentsResponse>>)
      .mockResolvedValueOnce({ data: secondResponse } as Partial<AxiosResponse<PaginatedCommentsResponse>>);

    // Act
    renderPostCard(mockPost);

    // Wait for initial load
    await waitFor(() => {
      // Use queryAllByText to handle potential duplicates during rendering
      expect(screen.queryAllByText('Comment 1').length).toBeGreaterThanOrEqual(1);
      expect(screen.queryAllByText('Comment 2').length).toBeGreaterThanOrEqual(1);
      expect(screen.queryAllByText('Comment 3').length).toBeGreaterThanOrEqual(1);
    });

    // Click Load More
    const loadMoreButton = screen.getByText(/Load More Comments/i);
    await user.click(loadMoreButton);

    // Assert: All comments should be visible
    await waitFor(() => {
      expect(screen.getByText('Comment 1')).toBeInTheDocument();
      expect(screen.getByText('Comment 2')).toBeInTheDocument();
      expect(screen.getByText('Comment 3')).toBeInTheDocument();
      expect(screen.getByText('Comment 4')).toBeInTheDocument();
      expect(screen.getByText('Comment 5')).toBeInTheDocument();
    });
  });

  it('should preserve nested comment structure when loading more', async () => {
    const user = userEvent.setup();

    // Arrange: Top-level comment with nested replies
    const initialComments = [
      createMockComment(1, 0), // Top-level
      createMockComment(2, 1, 1), // Reply to comment 1
      createMockComment(3, 2, 2), // Reply to comment 2
    ];
    const initialResponse: PaginatedCommentsResponse = {
      comments: initialComments,
      total_top_level: 2,
      returned_top_level: 1,
      returned_total: 3,
      has_more: true,
      limit: THREADS_PER_PAGE,
      offset: 0,
    };

    // Load another top-level with its replies
    const moreComments = [
      createMockComment(4, 0), // Second top-level
      createMockComment(5, 1, 4), // Reply to comment 4
    ];
    const secondResponse: PaginatedCommentsResponse = {
      comments: moreComments,
      total_top_level: 2,
      returned_top_level: 1,
      returned_total: 2,
      has_more: false,
      limit: THREADS_PER_PAGE,
      offset: 1,
    };

    vi.mocked(apiClient.messages.getPostCommentsWithThreads)
      .mockResolvedValueOnce({ data: initialResponse } as Partial<AxiosResponse<PaginatedCommentsResponse>>)
      .mockResolvedValueOnce({ data: secondResponse } as Partial<AxiosResponse<PaginatedCommentsResponse>>);

    // Act
    renderPostCard(mockPost);

    // Wait for initial load
    await waitFor(() => {
      // Use queryAllByText to handle potential duplicates during rendering
      expect(screen.queryAllByText('Comment 1').length).toBeGreaterThanOrEqual(1);
      expect(screen.queryAllByText('Comment 2').length).toBeGreaterThanOrEqual(1);
      expect(screen.queryAllByText('Comment 3').length).toBeGreaterThanOrEqual(1);
    });

    // Click Load More
    const loadMoreButton = screen.getByText(/Load More Comments/i);
    await user.click(loadMoreButton);

    // Assert: Both trees should be preserved
    await waitFor(() => {
      // Use queryAllByText to handle potential duplicates during rendering
      expect(screen.queryAllByText('Comment 1').length).toBeGreaterThanOrEqual(1);
      expect(screen.queryAllByText('Comment 2').length).toBeGreaterThanOrEqual(1);
      expect(screen.queryAllByText('Comment 3').length).toBeGreaterThanOrEqual(1);
      expect(screen.queryAllByText('Comment 4').length).toBeGreaterThanOrEqual(1);
      expect(screen.queryAllByText('Comment 5').length).toBeGreaterThanOrEqual(1);
    });
  });

  it('should auto-load next page when sentinel scrolls into view', async () => {
    // Arrange: Initial load returns one page of threads with more available
    const initialComments = Array.from({ length: THREADS_PER_PAGE }, (_, i) => createMockComment(i + 1));
    const initialResponse: PaginatedCommentsResponse = {
      comments: initialComments,
      total_top_level: 20,
      returned_top_level: THREADS_PER_PAGE,
      returned_total: THREADS_PER_PAGE,
      has_more: true,
      limit: THREADS_PER_PAGE,
      offset: 0,
    };

    const moreComments = Array.from({ length: 5 }, (_, i) => createMockComment(i + THREADS_PER_PAGE + 1));
    const secondResponse: PaginatedCommentsResponse = {
      comments: moreComments,
      total_top_level: 20,
      returned_top_level: 5,
      returned_total: 5,
      has_more: false,
      limit: THREADS_PER_PAGE,
      offset: THREADS_PER_PAGE,
    };

    vi.mocked(apiClient.messages.getPostCommentsWithThreads)
      .mockResolvedValueOnce({ data: initialResponse } as Partial<AxiosResponse<PaginatedCommentsResponse>>)
      .mockResolvedValueOnce({ data: secondResponse } as Partial<AxiosResponse<PaginatedCommentsResponse>>);

    // Act: render
    renderPostCard(mockPost);

    // Wait for initial load and sentinel to be observed
    await waitFor(() => {
      expect(screen.getByText(`Comment 1`)).toBeInTheDocument();
    });

    // Simulate the sentinel scrolling into view
    expect(io.hasObserver()).toBe(true);
    act(() => {
      io.intersect();
    });

    // Assert: page-2 fetch fires with correct offset
    await waitFor(() => {
      expect(apiClient.messages.getPostCommentsWithThreads).toHaveBeenCalledTimes(2);
      expect(apiClient.messages.getPostCommentsWithThreads).toHaveBeenLastCalledWith(
        1, 1, THREADS_PER_PAGE, THREADS_PER_PAGE, 5
      );
    });

    // Threads from both pages should be visible
    await waitFor(() => {
      expect(screen.getByText(`Comment ${THREADS_PER_PAGE + 1}`)).toBeInTheDocument();
    });
  });

  it('should deduplicate threads when offset drift causes page overlap', async () => {
    // Arrange: page 1 returns threads 1-3
    const initialComments = [
      createMockComment(1),
      createMockComment(2),
      createMockComment(3),
    ];
    const initialResponse: PaginatedCommentsResponse = {
      comments: initialComments,
      total_top_level: 5,
      returned_top_level: 3,
      returned_total: 3,
      has_more: true,
      limit: THREADS_PER_PAGE,
      offset: 0,
    };

    // page 2 re-returns thread 3 (duplicate due to offset drift) plus threads 4-5
    const moreComments = [
      createMockComment(3), // duplicate
      createMockComment(4),
      createMockComment(5),
    ];
    const secondResponse: PaginatedCommentsResponse = {
      comments: moreComments,
      total_top_level: 5,
      returned_top_level: 3,
      returned_total: 3,
      has_more: false,
      limit: THREADS_PER_PAGE,
      offset: 3,
    };

    vi.mocked(apiClient.messages.getPostCommentsWithThreads)
      .mockResolvedValueOnce({ data: initialResponse } as Partial<AxiosResponse<PaginatedCommentsResponse>>)
      .mockResolvedValueOnce({ data: secondResponse } as Partial<AxiosResponse<PaginatedCommentsResponse>>);

    const user = userEvent.setup();
    renderPostCard(mockPost);

    await waitFor(() => {
      expect(screen.getByText('Comment 1')).toBeInTheDocument();
    });

    const loadMoreButton = screen.getByText(/Load More Comments/i);
    await user.click(loadMoreButton);

    await waitFor(() => {
      expect(screen.queryByText(/Load More Comments/i)).not.toBeInTheDocument();
    });

    // Thread 3 must appear exactly once
    expect(screen.getAllByText('Comment 3')).toHaveLength(1);

    // All other threads present
    expect(screen.getByText('Comment 1')).toBeInTheDocument();
    expect(screen.getByText('Comment 2')).toBeInTheDocument();
    expect(screen.getByText('Comment 4')).toBeInTheDocument();
    expect(screen.getByText('Comment 5')).toBeInTheDocument();
  });

  const makePageResponse = (
    comments: CommentWithDepth[],
    offset: number,
    hasMore: boolean,
    totalTopLevel: number = 20
  ): PaginatedCommentsResponse => ({
    comments,
    total_top_level: totalTopLevel,
    returned_top_level: comments.length,
    returned_total: comments.length,
    has_more: hasMore,
    limit: comments.length,
    offset,
  });

  it('should perform window-preserving silent refresh without collapsing the list', async () => {
    // Arrange: two pages loaded (window = 10 threads), then a top-level comment
    // is submitted, which triggers loadComments — the silent refresh. It must
    // re-fetch the whole loaded window in ONE request (limit=10, offset=0) and
    // must never swap the list for the "Loading comments..." placeholder.
    const page1 = Array.from({ length: THREADS_PER_PAGE }, (_, i) => createMockComment(i + 1));
    const page2 = Array.from({ length: THREADS_PER_PAGE }, (_, i) => createMockComment(i + THREADS_PER_PAGE + 1));

    // Refresh response is deferred so we can assert the list stays mounted
    // while the refresh is in flight.
    const refreshDeferred = deferred<Partial<AxiosResponse<PaginatedCommentsResponse>>>();

    vi.mocked(apiClient.messages.getPostCommentsWithThreads)
      .mockResolvedValueOnce({ data: makePageResponse(page1, 0, true) } as Partial<AxiosResponse<PaginatedCommentsResponse>>)
      .mockResolvedValueOnce({ data: makePageResponse(page2, THREADS_PER_PAGE, true) } as Partial<AxiosResponse<PaginatedCommentsResponse>>)
      // 3rd call = the silent refresh
      .mockImplementationOnce(() => refreshDeferred.promise as never);

    const user = userEvent.setup();
    renderPostCard(mockPost);

    // Load two pages
    await waitFor(() => expect(screen.getByText('Comment 1')).toBeInTheDocument());
    await user.click(screen.getByText(/Load More Comments/i));
    await waitFor(() => expect(screen.getByText(`Comment ${THREADS_PER_PAGE + 1}`)).toBeInTheDocument());

    // Act: submit a top-level comment → handleSubmitComment → loadComments()
    await submitTopLevelComment(user);

    // Assert: the refresh re-fetches the whole loaded window in one request
    await waitFor(() => {
      expect(apiClient.messages.getPostCommentsWithThreads).toHaveBeenCalledTimes(3);
      expect(apiClient.messages.getPostCommentsWithThreads).toHaveBeenLastCalledWith(
        1, 1, THREADS_PER_PAGE * 2, 0, 5
      );
    });

    // While the refresh is in flight the list must stay mounted — no collapse,
    // no "Loading comments..." placeholder
    expect(screen.getByText('Comment 1')).toBeInTheDocument();
    expect(screen.getByText(`Comment ${THREADS_PER_PAGE * 2}`)).toBeInTheDocument();
    expect(screen.queryByText(/Loading comments/i)).not.toBeInTheDocument();

    // Resolve the refresh; all threads remain rendered
    await act(async () => {
      refreshDeferred.resolve({ data: makePageResponse([...page1, ...page2], 0, true) } as Partial<AxiosResponse<PaginatedCommentsResponse>>);
    });
    expect(screen.getByText('Comment 1')).toBeInTheDocument();
    expect(screen.getByText(`Comment ${THREADS_PER_PAGE * 2}`)).toBeInTheDocument();
    expect(screen.queryByText(/Loading comments/i)).not.toBeInTheDocument();
  });

  it('should not lose a page loaded while a silent refresh is in flight (refresh/load-more race)', async () => {
    // Arrange: a load-more and a silent refresh overlap. The load-more lands
    // first (window grows to 10); the refresh was requested with the old
    // window (5). Applying the stale 5-thread response would drop threads
    // 6-10 while offset stays at 10 — a permanent gap. The component must
    // detect the grown window and re-fetch [0, 10) before applying.
    const page1 = Array.from({ length: THREADS_PER_PAGE }, (_, i) => createMockComment(i + 1));
    const page2 = Array.from({ length: THREADS_PER_PAGE }, (_, i) => createMockComment(i + THREADS_PER_PAGE + 1));

    const loadMoreDeferred = deferred<Partial<AxiosResponse<PaginatedCommentsResponse>>>();
    const staleRefreshDeferred = deferred<Partial<AxiosResponse<PaginatedCommentsResponse>>>();

    vi.mocked(apiClient.messages.getPostCommentsWithThreads)
      .mockResolvedValueOnce({ data: makePageResponse(page1, 0, true) } as Partial<AxiosResponse<PaginatedCommentsResponse>>)
      .mockImplementationOnce(() => loadMoreDeferred.promise as never)
      .mockImplementationOnce(() => staleRefreshDeferred.promise as never)
      // 4th call = the corrective re-fetch with the grown window
      .mockResolvedValueOnce({ data: makePageResponse([...page1, ...page2], 0, false) } as Partial<AxiosResponse<PaginatedCommentsResponse>>);

    const user = userEvent.setup();
    renderPostCard(mockPost);
    await waitFor(() => expect(screen.getByText('Comment 1')).toBeInTheDocument());

    // Start a load-more (stays in flight)…
    await user.click(screen.getByText(/Load More Comments/i));
    // …then trigger a silent refresh while the load-more is pending
    await submitTopLevelComment(user);

    // The refresh asked for the pre-load-more window
    await waitFor(() => expect(apiClient.messages.getPostCommentsWithThreads).toHaveBeenCalledTimes(3));
    expect(apiClient.messages.getPostCommentsWithThreads).toHaveBeenNthCalledWith(
      3, 1, 1, THREADS_PER_PAGE, 0, 5
    );

    // Load-more lands first: threads 6-10 appear, window is now 10
    await act(async () => {
      loadMoreDeferred.resolve({ data: makePageResponse(page2, THREADS_PER_PAGE, true) } as Partial<AxiosResponse<PaginatedCommentsResponse>>);
    });
    await waitFor(() => expect(screen.getByText(`Comment ${THREADS_PER_PAGE + 1}`)).toBeInTheDocument());

    // Stale refresh lands second — must trigger a re-fetch of the grown window
    await act(async () => {
      staleRefreshDeferred.resolve({ data: makePageResponse(page1, 0, true) } as Partial<AxiosResponse<PaginatedCommentsResponse>>);
    });
    await waitFor(() => {
      expect(apiClient.messages.getPostCommentsWithThreads).toHaveBeenCalledTimes(4);
      expect(apiClient.messages.getPostCommentsWithThreads).toHaveBeenLastCalledWith(
        1, 1, THREADS_PER_PAGE * 2, 0, 5
      );
    });

    // No page lost: every loaded thread is still rendered
    expect(screen.getByText('Comment 1')).toBeInTheDocument();
    expect(screen.getByText(`Comment ${THREADS_PER_PAGE * 2}`)).toBeInTheDocument();
  });

  it('should keep the remaining count accurate when pages overlap due to offset drift', async () => {
    // Arrange: initial total is 12 ("7 remaining" after page 1). While the
    // reader paginates, 2 new top-level comments arrive: the server total is
    // now 14 and page 2 re-returns threads 4-5 (drift duplicates) plus 6-8.
    // The label must use the fresh total (14 - 10 = 4 remaining), not the
    // stale one (12 - 10 = 2), which under heavier drift can even go negative.
    const page1 = Array.from({ length: THREADS_PER_PAGE }, (_, i) => createMockComment(i + 1));
    const page2 = [4, 5, 6, 7, 8].map(id => createMockComment(id));

    vi.mocked(apiClient.messages.getPostCommentsWithThreads)
      .mockResolvedValueOnce({ data: makePageResponse(page1, 0, true, 12) } as Partial<AxiosResponse<PaginatedCommentsResponse>>)
      .mockResolvedValueOnce({ data: makePageResponse(page2, THREADS_PER_PAGE, true, 14) } as Partial<AxiosResponse<PaginatedCommentsResponse>>);

    const user = userEvent.setup();
    renderPostCard(mockPost);

    await waitFor(() => expect(screen.getByText(/7 remaining/i)).toBeInTheDocument());

    await user.click(screen.getByText(/Load More Comments/i));

    await waitFor(() => expect(screen.getByText(/4 remaining/i)).toBeInTheDocument());
    // Drift duplicates render exactly once
    expect(screen.getAllByText('Comment 4')).toHaveLength(1);
    expect(screen.getAllByText('Comment 5')).toHaveLength(1);
  });

  it('should show an error state with a working retry when the initial load fails', async () => {
    // Arrange: initial fetch fails, retry succeeds
    const page1 = Array.from({ length: THREADS_PER_PAGE }, (_, i) => createMockComment(i + 1));

    vi.mocked(apiClient.messages.getPostCommentsWithThreads)
      .mockRejectedValueOnce(new Error('Network error'))
      .mockResolvedValueOnce({ data: makePageResponse(page1, 0, false) } as Partial<AxiosResponse<PaginatedCommentsResponse>>);

    const user = userEvent.setup();
    renderPostCard(mockPost);

    // The failure is surfaced — NOT the misleading "No comments yet" empty
    // state (the post advertises 30 comments)
    await waitFor(() => expect(screen.getByText(/Failed to load comments/i)).toBeInTheDocument());
    expect(screen.queryByText(/No comments yet/i)).not.toBeInTheDocument();

    // Retry recovers
    await user.click(screen.getByRole('button', { name: /Retry/i }));
    await waitFor(() => expect(screen.getByText('Comment 1')).toBeInTheDocument());
    expect(screen.queryByText(/Failed to load comments/i)).not.toBeInTheDocument();
  });

  it('should recover from a failed page-2 fetch and allow manual retry', async () => {
    const user = userEvent.setup();

    // Arrange: Initial load succeeds
    const initialComments = Array.from({ length: THREADS_PER_PAGE }, (_, i) => createMockComment(i + 1));
    const initialResponse: PaginatedCommentsResponse = {
      comments: initialComments,
      total_top_level: 20,
      returned_top_level: THREADS_PER_PAGE,
      returned_total: THREADS_PER_PAGE,
      has_more: true,
      limit: THREADS_PER_PAGE,
      offset: 0,
    };

    // Page 2 fails
    const networkError = new Error('Network error');

    // Retry succeeds
    const moreComments = Array.from({ length: 5 }, (_, i) => createMockComment(i + THREADS_PER_PAGE + 1));
    const retryResponse: PaginatedCommentsResponse = {
      comments: moreComments,
      total_top_level: 20,
      returned_top_level: 5,
      returned_total: 5,
      has_more: false,
      limit: THREADS_PER_PAGE,
      offset: THREADS_PER_PAGE,
    };

    vi.mocked(apiClient.messages.getPostCommentsWithThreads)
      .mockResolvedValueOnce({ data: initialResponse } as Partial<AxiosResponse<PaginatedCommentsResponse>>)
      .mockRejectedValueOnce(networkError)
      .mockResolvedValueOnce({ data: retryResponse } as Partial<AxiosResponse<PaginatedCommentsResponse>>);

    renderPostCard(mockPost);

    // Wait for initial load
    await waitFor(() => {
      expect(screen.getByText(/Load More Comments/i)).toBeInTheDocument();
    });

    // Click Load More — this will fail
    let btn = screen.getByText(/Load More Comments/i);
    await user.click(btn);

    // After failure, loadingMore returns false and the button must be clickable again
    await waitFor(() => {
      const retryBtn = screen.getByText(/Load More Comments/i);
      expect(retryBtn.closest('button')).not.toBeDisabled();
    });

    // Retry — this should succeed
    btn = screen.getByText(/Load More Comments/i);
    await user.click(btn);

    await waitFor(() => {
      expect(screen.queryByText(/Load More Comments/i)).not.toBeInTheDocument();
      expect(screen.getByText(`Comment ${THREADS_PER_PAGE + 1}`)).toBeInTheDocument();
    });
  });
});
