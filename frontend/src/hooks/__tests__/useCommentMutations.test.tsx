import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement } from 'react';
import type { AxiosResponse } from 'axios';
import { useUpdateComment, useDeleteComment } from '../useCommentMutations';
import { apiClient } from '../../lib/api';
import type { Message } from '../../types/messages';

// Mock the API client
vi.mock('../../lib/api', () => ({
  apiClient: {
    messages: {
      updateComment: vi.fn(),
      deleteComment: vi.fn(),
    },
  },
}));

function makeMessage(overrides: Partial<Message> = {}): Message {
  return {
    id: 9010,
    parent_id: 9004,
    game_id: 12,
    thread_depth: 5,
    character_id: 1,
    author_id: 1,
    author_username: 'tester',
    character_name: 'Dr. Chen',
    content: 'updated content',
    message_type: 'comment',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    reply_count: 0,
    is_deleted: false,
    is_edited: true,
    is_draft: false,
    ...overrides,
  } as Message;
}

function axiosResponse<T>(data: T): AxiosResponse<T> {
  return { data, status: 200, statusText: 'OK', headers: {}, config: {} } as AxiosResponse<T>;
}

function createWrapper(queryClient: QueryClient) {
  return ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

describe('useUpdateComment', () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    vi.clearAllMocks();
    queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  });

  // Regression test: editing a comment from the "New Comments" tab left the card
  // showing stale text until a manual refresh. That list is backed by the
  // ['games', gameId, 'recentComments'] query, and the update mutation never
  // invalidated it, so the visible card never refetched.
  it('invalidates the recentComments query so the New Comments view refreshes', async () => {
    vi.mocked(apiClient.messages.updateComment).mockResolvedValue(
      axiosResponse(makeMessage())
    );
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');

    const { result } = renderHook(() => useUpdateComment(), {
      wrapper: createWrapper(queryClient),
    });

    result.current.mutate({
      gameId: 12,
      postId: 9000,
      commentId: 9010,
      data: { content: 'updated content' },
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: ['games', 12, 'recentComments'],
    });
  });
});

describe('useDeleteComment', () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    vi.clearAllMocks();
    queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  });

  // Same class of bug for deletion: a comment deleted from the New Comments view
  // must disappear without a refresh.
  it('invalidates the recentComments query so the New Comments view refreshes', async () => {
    vi.mocked(apiClient.messages.deleteComment).mockResolvedValue(
      axiosResponse({ message: 'deleted', id: 9010 })
    );
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');

    const { result } = renderHook(() => useDeleteComment(), {
      wrapper: createWrapper(queryClient),
    });

    result.current.mutate({ gameId: 12, postId: 9000, commentId: 9010 });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: ['games', 12, 'recentComments'],
    });
  });
});
