import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement } from 'react';
import {
  useManualReadCommentIDs,
  usePostManualReadCommentIDs,
  useToggleCommentRead,
  useMarkAllCommentsRead,
} from './useReadTracking';

const mockGetManualReadCommentIDs = vi.fn();
const mockToggleCommentRead = vi.fn();
const mockMarkAllCommentsRead = vi.fn();
const mockGetGamePosts = vi.fn();

vi.mock('../lib/api', () => ({
  apiClient: {
    messages: {
      getManualReadCommentIDs: (...args: unknown[]) => mockGetManualReadCommentIDs(...args),
      toggleCommentRead: (...args: unknown[]) => mockToggleCommentRead(...args),
      markAllCommentsRead: (...args: unknown[]) => mockMarkAllCommentsRead(...args),
      getGamePosts: (...args: unknown[]) => mockGetGamePosts(...args),
    },
  },
}));

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

describe('useManualReadCommentIDs', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetManualReadCommentIDs.mockResolvedValue({
      data: [{ post_id: 1, read_comment_ids: [5, 12] }],
    });
  });

  it('is disabled when gameId is undefined', () => {
    const wrapper = createWrapper();
    const { result } = renderHook(() => useManualReadCommentIDs(undefined), { wrapper });
    expect(result.current.data).toBeUndefined();
    expect(result.current.isPending).toBe(true);
  });

  it('fetches data for a valid gameId', async () => {
    const wrapper = createWrapper();
    const { result } = renderHook(() => useManualReadCommentIDs(1), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockGetManualReadCommentIDs).toHaveBeenCalledWith(1);
    expect(result.current.data).toEqual([{ post_id: 1, read_comment_ids: [5, 12] }]);
  });
});

describe('usePostManualReadCommentIDs', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetManualReadCommentIDs.mockResolvedValue({
      data: [
        { post_id: 1, read_comment_ids: [5, 12] },
        { post_id: 2, read_comment_ids: [33] },
      ],
    });
  });

  it('returns empty array when postId is undefined', () => {
    const wrapper = createWrapper();
    const { result } = renderHook(() => usePostManualReadCommentIDs(1, undefined), { wrapper });
    expect(result.current).toEqual([]);
  });

  it('filters read IDs for the specified post', async () => {
    const wrapper = createWrapper();
    const { result } = renderHook(() => usePostManualReadCommentIDs(1, 1), { wrapper });

    await waitFor(() => expect(result.current).toEqual([5, 12]));
  });

  it('returns empty array when post has no read entries', async () => {
    const wrapper = createWrapper();
    const { result } = renderHook(() => usePostManualReadCommentIDs(1, 99), { wrapper });

    await waitFor(() => expect(result.current).toEqual([]));
  });
});

describe('useToggleCommentRead', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockToggleCommentRead.mockResolvedValue({ data: null });
    // Also mock getManualReadCommentIDs so invalidation doesn't break
    mockGetManualReadCommentIDs.mockResolvedValue({ data: [] });
  });

  it('calls toggleCommentRead API with correct arguments', async () => {
    const wrapper = createWrapper();
    const { result } = renderHook(() => useToggleCommentRead(), { wrapper });

    result.current.mutate({ gameId: 1, postId: 2, commentId: 42, read: true });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockToggleCommentRead).toHaveBeenCalledWith(1, 2, 42, true);
  });
});

describe('useMarkAllCommentsRead', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockMarkAllCommentsRead.mockResolvedValue({ data: null });
    mockGetGamePosts.mockResolvedValue({ data: [{ id: 1 }, { id: 2 }] });
    mockGetManualReadCommentIDs.mockResolvedValue({ data: [] });
  });

  it('calls markAllCommentsRead API with correct arguments', async () => {
    const wrapper = createWrapper();
    const { result } = renderHook(() => useMarkAllCommentsRead(), { wrapper });

    result.current.mutate({ gameId: 1, phaseId: 5 });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockMarkAllCommentsRead).toHaveBeenCalledWith(1, 5);
  });

  it('optimistically clears unread comment IDs for cached posts', async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    queryClient.setQueryData(['unreadCommentIDs', 1], [
      { post_id: 1, unread_comment_ids: [10, 11] },
      { post_id: 2, unread_comment_ids: [20] },
    ]);

    const wrapper = ({ children }: { children: React.ReactNode }) =>
      createElement(QueryClientProvider, { client: queryClient }, children);
    const { result } = renderHook(() => useMarkAllCommentsRead(), { wrapper });

    result.current.mutate({ gameId: 1, phaseId: 5 });

    await waitFor(() => {
      const data = queryClient.getQueryData(['unreadCommentIDs', 1]) as {
        post_id: number;
        unread_comment_ids: number[];
      }[];
      expect(data.find((d) => d.post_id === 1)?.unread_comment_ids).toEqual([]);
      expect(data.find((d) => d.post_id === 2)?.unread_comment_ids).toEqual([]);
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
  });

  // The optimistic write must not depend on a posts fetch: that would make it
  // non-optimistic and let an unrelated read failure abort the mark-read write.
  it('does not fetch posts to scope the optimistic update', async () => {
    const wrapper = createWrapper();
    const { result } = renderHook(() => useMarkAllCommentsRead(), { wrapper });

    result.current.mutate({ gameId: 1, phaseId: 5 });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockGetGamePosts).not.toHaveBeenCalled();
  });

  it('rolls back the optimistic update on error', async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const original = [{ post_id: 1, unread_comment_ids: [10, 11] }];
    queryClient.setQueryData(['unreadCommentIDs', 1], original);
    mockGetGamePosts.mockResolvedValue({ data: [{ id: 1 }] });
    mockMarkAllCommentsRead.mockRejectedValue(new Error('boom'));

    const wrapper = ({ children }: { children: React.ReactNode }) =>
      createElement(QueryClientProvider, { client: queryClient }, children);
    const { result } = renderHook(() => useMarkAllCommentsRead(), { wrapper });

    result.current.mutate({ gameId: 1, phaseId: 5 });

    await waitFor(() => expect(result.current.isError).toBe(true));

    expect(queryClient.getQueryData(['unreadCommentIDs', 1])).toEqual(original);
  });
});
