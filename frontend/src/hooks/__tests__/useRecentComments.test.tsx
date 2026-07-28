import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { AxiosResponse } from 'axios';
import { useRecentComments, useTotalCommentCount } from '../useRecentComments';
import { apiClient } from '../../lib/api';
import type { RecentCommentsResponse, CommentWithParent } from '../../types/messages';

// Mock the API client
vi.mock('../../lib/api', () => ({
  apiClient: {
    messages: {
      getRecentComments: vi.fn(),
      getTotalCommentCount: vi.fn(),
    },
  },
}));

describe('useRecentComments', () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    // Create a new QueryClient for each test
    queryClient = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false, // Disable retries for tests
        },
      },
    });
    vi.clearAllMocks();
  });

  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );

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
    parent_content: 'Parent post content',
    parent_created_at: '2025-10-22T09:00:00Z',
    parent_deleted_at: null,
    parent_is_deleted: false,
    parent_message_type: 'post',
    parent_author_username: 'parentuser',
    parent_character_name: 'Parent Character',
  };

  const mockResponse: RecentCommentsResponse = {
    comments: [mockComment],
    total: 1,
    limit: 20,
    offset: 0,
  };

  it('fetches recent comments successfully', async () => {
    vi.mocked(apiClient.messages.getRecentComments).mockResolvedValue({
      data: mockResponse,
    } as Partial<AxiosResponse<RecentCommentsResponse>>);

    const { result } = renderHook(() => useRecentComments(1), { wrapper });

    // Initially loading
    expect(result.current.isLoading).toBe(true);
    expect(result.current.data).toBeUndefined();

    // Wait for data to load
    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.data?.pages).toHaveLength(1);
    expect(result.current.data?.pages[0]).toEqual(mockResponse);
    expect(result.current.error).toBeNull();
    expect(apiClient.messages.getRecentComments).toHaveBeenCalledWith(1, 20, 0, false);
  });

  it('handles error state', async () => {
    const error = new Error('Failed to fetch comments');
    vi.mocked(apiClient.messages.getRecentComments).mockRejectedValue(error);

    const { result } = renderHook(() => useRecentComments(1), { wrapper });

    // Wait for error
    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toEqual(error);
  });

  it('does not fetch when gameId is undefined', () => {
    const { result } = renderHook(() => useRecentComments(undefined), { wrapper });

    expect(result.current.isLoading).toBe(false);
    expect(result.current.data).toBeUndefined();
    expect(apiClient.messages.getRecentComments).not.toHaveBeenCalled();
  });

  it('determines hasNextPage correctly when there are more pages', async () => {
    const fullPageResponse: RecentCommentsResponse = {
      comments: Array(20).fill(mockComment),
      total: 50,
      limit: 20,
      offset: 0,
    };

    vi.mocked(apiClient.messages.getRecentComments).mockResolvedValue({
      data: fullPageResponse,
    } as Partial<AxiosResponse<RecentCommentsResponse>>);

    const { result } = renderHook(() => useRecentComments(1), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    // Should have next page because we got a full page (20 items)
    expect(result.current.hasNextPage).toBe(true);
  });

  it('determines hasNextPage correctly when at end', async () => {
    const partialPageResponse: RecentCommentsResponse = {
      comments: Array(15).fill(mockComment),
      total: 15,
      limit: 20,
      offset: 0,
    };

    vi.mocked(apiClient.messages.getRecentComments).mockResolvedValue({
      data: partialPageResponse,
    } as Partial<AxiosResponse<RecentCommentsResponse>>);

    const { result } = renderHook(() => useRecentComments(1), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    // Should not have next page because we got less than a full page
    expect(result.current.hasNextPage).toBe(false);
  });

  it('fetches next page with correct offset', async () => {
    const firstPageResponse: RecentCommentsResponse = {
      comments: Array(20).fill({ ...mockComment, id: 1 }),
      total: 50,
      limit: 20,
      offset: 0,
    };

    const secondPageResponse: RecentCommentsResponse = {
      comments: Array(20).fill({ ...mockComment, id: 2 }),
      total: 50,
      limit: 20,
      offset: 20,
    };

    vi.mocked(apiClient.messages.getRecentComments)
      .mockResolvedValueOnce({ data: firstPageResponse } as Partial<AxiosResponse<RecentCommentsResponse>>)
      .mockResolvedValueOnce({ data: secondPageResponse } as Partial<AxiosResponse<RecentCommentsResponse>>);

    const { result } = renderHook(() => useRecentComments(1), { wrapper });

    // Wait for first page
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.pages).toHaveLength(1);
    expect(apiClient.messages.getRecentComments).toHaveBeenCalledWith(1, 20, 0, false);

    // Fetch next page
    result.current.fetchNextPage();

    // Wait for second page
    await waitFor(() => {
      expect(result.current.data?.pages).toHaveLength(2);
    });

    expect(apiClient.messages.getRecentComments).toHaveBeenCalledWith(1, 20, 20, false);
    expect(result.current.data?.pages[1]).toEqual(secondPageResponse);
  });

  it('uses correct query key', async () => {
    vi.mocked(apiClient.messages.getRecentComments).mockResolvedValue({
      data: mockResponse,
    } as Partial<AxiosResponse<RecentCommentsResponse>>);

    renderHook(() => useRecentComments(1), { wrapper });

    await waitFor(() => {
      const cachedData = queryClient.getQueryData(['games', 1, 'recentComments', { unreadOnly: false }]);
      expect(cachedData).toBeDefined();
    });
  });

  it('requests unread-only comments when unreadOnly is true', async () => {
    vi.mocked(apiClient.messages.getRecentComments).mockResolvedValue({
      data: mockResponse,
    } as Partial<AxiosResponse<RecentCommentsResponse>>);

    const { result } = renderHook(() => useRecentComments(1, true), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(apiClient.messages.getRecentComments).toHaveBeenCalledWith(1, 20, 0, true);
  });

  it('caches filtered and unfiltered lists under separate query keys', async () => {
    vi.mocked(apiClient.messages.getRecentComments).mockResolvedValue({
      data: mockResponse,
    } as Partial<AxiosResponse<RecentCommentsResponse>>);

    const unfiltered = renderHook(() => useRecentComments(1, false), { wrapper });
    await waitFor(() => expect(unfiltered.result.current.isSuccess).toBe(true));

    const filtered = renderHook(() => useRecentComments(1, true), { wrapper });
    await waitFor(() => expect(filtered.result.current.isSuccess).toBe(true));

    // Separate keys mean the filtered view refetches rather than reusing the
    // unfiltered cache, which would show already-read comments.
    expect(
      queryClient.getQueryData(['games', 1, 'recentComments', { unreadOnly: false }])
    ).toBeDefined();
    expect(
      queryClient.getQueryData(['games', 1, 'recentComments', { unreadOnly: true }])
    ).toBeDefined();
    expect(apiClient.messages.getRecentComments).toHaveBeenCalledWith(1, 20, 0, false);
    expect(apiClient.messages.getRecentComments).toHaveBeenCalledWith(1, 20, 0, true);
  });

  it('handles missing offset in API response', async () => {
    // Mock response without offset field (regression test for NaN bug)
    const responseWithoutOffset = {
      comments: Array(20).fill({ ...mockComment }),
      total: 50,
      limit: 20,
      // offset intentionally missing
    };

    const secondPageResponse: RecentCommentsResponse = {
      comments: Array(20).fill({ ...mockComment, id: 2 }),
      total: 50,
      limit: 20,
      offset: 20,
    };

    vi.mocked(apiClient.messages.getRecentComments)
      .mockResolvedValueOnce({ data: responseWithoutOffset } as Partial<AxiosResponse>)
      .mockResolvedValueOnce({ data: secondPageResponse } as Partial<AxiosResponse<RecentCommentsResponse>>);

    const { result } = renderHook(() => useRecentComments(1), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    // Should still be able to fetch next page
    result.current.fetchNextPage();

    // Verify the next call uses calculated offset (20), not NaN
    await waitFor(() => {
      expect(result.current.data?.pages).toHaveLength(2);
    });

    // Verify second API call used correct offset calculated from pages
    expect(apiClient.messages.getRecentComments).toHaveBeenCalledWith(1, 20, 20, false);
  });
});

describe('useTotalCommentCount', () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
        },
      },
    });
    vi.clearAllMocks();
  });

  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );

  it('fetches total comment count successfully', async () => {
    vi.mocked(apiClient.messages.getTotalCommentCount).mockResolvedValue(42);

    const { result } = renderHook(() => useTotalCommentCount(1), { wrapper });

    // Initially loading
    expect(result.current.isLoading).toBe(true);

    // Wait for data to load
    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.data).toBe(42);
    expect(result.current.error).toBeNull();
    expect(apiClient.messages.getTotalCommentCount).toHaveBeenCalledWith(1);
  });

  it('handles error state', async () => {
    const error = new Error('Failed to fetch count');
    vi.mocked(apiClient.messages.getTotalCommentCount).mockRejectedValue(error);

    const { result } = renderHook(() => useTotalCommentCount(1), { wrapper });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toEqual(error);
  });

  it('does not fetch when gameId is undefined', () => {
    const { result } = renderHook(() => useTotalCommentCount(undefined), { wrapper });

    expect(result.current.isLoading).toBe(false);
    expect(result.current.data).toBeUndefined();
    expect(apiClient.messages.getTotalCommentCount).not.toHaveBeenCalled();
  });

  it('uses correct query key', async () => {
    vi.mocked(apiClient.messages.getTotalCommentCount).mockResolvedValue(42);

    renderHook(() => useTotalCommentCount(1), { wrapper });

    await waitFor(() => {
      const cachedData = queryClient.getQueryData(['games', 1, 'totalCommentCount']);
      expect(cachedData).toBe(42);
    });
  });
});
