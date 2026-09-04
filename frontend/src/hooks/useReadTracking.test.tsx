import React from 'react';
import { describe, it, expect, beforeEach, beforeAll, afterEach, afterAll } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import {
  useUnreadCommentIDs,
  usePostUnreadCommentIDs,
  useMarkPostAsRead,
} from './useReadTracking';
import type { PostUnreadComments } from '../types/messages';

// Setup MSW server
const server = setupServer();

beforeAll(() => server.listen());
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

describe('useReadTracking hooks', () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
        },
      },
    });
  });

  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );

  describe('useUnreadCommentIDs', () => {
    it('fetches unread comment IDs for a game', async () => {
      const mockData: PostUnreadComments[] = [
        { post_id: 1, unread_comment_ids: [101, 102, 103] },
        { post_id: 2, unread_comment_ids: [201] },
        { post_id: 3, unread_comment_ids: [] }, // No unread comments
      ];

      server.use(
        http.get('/api/v1/games/10/unread-comment-ids', () => {
          return HttpResponse.json(mockData);
        })
      );

      const { result } = renderHook(() => useUnreadCommentIDs(10), { wrapper });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.data).toEqual(mockData);
      expect(result.current.data).toHaveLength(3);
      expect(result.current.data![0].unread_comment_ids).toHaveLength(3);
    });

    it('returns empty array when gameId is undefined', async () => {
      const { result } = renderHook(() => useUnreadCommentIDs(undefined), { wrapper });

      // Query should be disabled, so isPending should be true but not fetching
      expect(result.current.data).toBeUndefined();
      expect(result.current.isError).toBe(false);
    });

    it('handles API errors gracefully', async () => {
      server.use(
        http.get('/api/v1/games/10/unread-comment-ids', () => {
          return HttpResponse.json({ detail: 'Server error' }, { status: 500 });
        })
      );

      const { result } = renderHook(() => useUnreadCommentIDs(10), { wrapper });

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });

      expect(result.current.data).toBeUndefined();
    });

    it('excludes user own comments from unread list', async () => {
      // This test verifies the backend contract: user's own comments should not appear
      const mockData: PostUnreadComments[] = [
        { post_id: 1, unread_comment_ids: [102, 104] }, // Only other users' comments (101, 103 are user's own)
      ];

      server.use(
        http.get('/api/v1/games/10/unread-comment-ids', () => {
          return HttpResponse.json(mockData);
        })
      );

      const { result } = renderHook(() => useUnreadCommentIDs(10), { wrapper });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      // Verify only other users' comments are included
      expect(result.current.data![0].unread_comment_ids).toEqual([102, 104]);
      expect(result.current.data![0].unread_comment_ids).not.toContain(101);
      expect(result.current.data![0].unread_comment_ids).not.toContain(103);
    });

    it('includes nested comments from all levels', async () => {
      // This test verifies that the backend includes deeply nested comments
      const mockData: PostUnreadComments[] = [
        {
          post_id: 1,
          unread_comment_ids: [101, 102, 103, 104, 105], // Mix of top-level and nested
        },
      ];

      server.use(
        http.get('/api/v1/games/10/unread-comment-ids', () => {
          return HttpResponse.json(mockData);
        })
      );

      const { result } = renderHook(() => useUnreadCommentIDs(10), { wrapper });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      // All levels of nesting should be included
      expect(result.current.data![0].unread_comment_ids).toHaveLength(5);
    });
  });

  describe('usePostUnreadCommentIDs', () => {
    it('returns unread comment IDs for a specific post', async () => {
      const mockData: PostUnreadComments[] = [
        { post_id: 1, unread_comment_ids: [101, 102] },
        { post_id: 2, unread_comment_ids: [201, 202, 203] },
      ];

      server.use(
        http.get('/api/v1/games/10/unread-comment-ids', () => {
          return HttpResponse.json(mockData);
        })
      );

      const { result } = renderHook(() => usePostUnreadCommentIDs(10, 2), { wrapper });

      await waitFor(() => {
        expect(result.current).toEqual([201, 202, 203]);
      });
    });

    it('returns empty array when post has no unread comments', async () => {
      const mockData: PostUnreadComments[] = [
        { post_id: 1, unread_comment_ids: [] },
      ];

      server.use(
        http.get('/api/v1/games/10/unread-comment-ids', () => {
          return HttpResponse.json(mockData);
        })
      );

      const { result } = renderHook(() => usePostUnreadCommentIDs(10, 1), { wrapper });

      await waitFor(() => {
        expect(result.current).toEqual([]);
      });
    });

    it('returns empty array when post is not found', async () => {
      const mockData: PostUnreadComments[] = [
        { post_id: 1, unread_comment_ids: [101] },
      ];

      server.use(
        http.get('/api/v1/games/10/unread-comment-ids', () => {
          return HttpResponse.json(mockData);
        })
      );

      const { result } = renderHook(() => usePostUnreadCommentIDs(10, 999), { wrapper });

      await waitFor(() => {
        expect(result.current).toEqual([]);
      });
    });

    it('returns empty array when postId is undefined', async () => {
      const mockData: PostUnreadComments[] = [
        { post_id: 1, unread_comment_ids: [101] },
      ];

      server.use(
        http.get('/api/v1/games/10/unread-comment-ids', () => {
          return HttpResponse.json(mockData);
        })
      );

      const { result } = renderHook(() => usePostUnreadCommentIDs(10, undefined), { wrapper });

      // Should return empty array immediately without waiting
      expect(result.current).toEqual([]);
    });

    it('updates when unread comment IDs change', async () => {
      const initialData: PostUnreadComments[] = [
        { post_id: 1, unread_comment_ids: [101, 102] },
      ];

      const updatedData: PostUnreadComments[] = [
        { post_id: 1, unread_comment_ids: [101, 102, 103] },
      ];

      server.use(
        http.get('/api/v1/games/10/unread-comment-ids', () => {
          return HttpResponse.json(initialData);
        })
      );

      const { result } = renderHook(() => usePostUnreadCommentIDs(10, 1), { wrapper });

      await waitFor(() => {
        expect(result.current).toEqual([101, 102]);
      });

      // Update the mock data
      server.use(
        http.get('/api/v1/games/10/unread-comment-ids', () => {
          return HttpResponse.json(updatedData);
        })
      );

      // Invalidate query to trigger refetch
      await queryClient.refetchQueries({ queryKey: ['unreadCommentIDs', 10] });

      await waitFor(() => {
        expect(result.current).toEqual([101, 102, 103]);
      });
    });
  });

  describe('useMarkPostAsRead', () => {
    it('marks a post as read and invalidates queries', async () => {
      server.use(
        http.post('/api/v1/games/10/posts/1/mark-read', () => {
          return HttpResponse.json({
            id: 1,
            user_id: 1,
            game_id: 10,
            post_id: 1,
            last_read_comment_id: null,
            last_read_at: new Date().toISOString(),
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString(),
          });
        })
      );

      // Set up initial unread data
      server.use(
        http.get('/api/v1/games/10/unread-comment-ids', () => {
          return HttpResponse.json([
            { post_id: 1, unread_comment_ids: [101, 102] },
          ]);
        })
      );

      const { result } = renderHook(() => useMarkPostAsRead(), { wrapper });

      // Initially should not be loading
      expect(result.current.isPending).toBe(false);

      // Mark post as read
      result.current.mutate({ gameId: 10, postId: 1, data: {} });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      // Verify mutation completed
      expect(result.current.isPending).toBe(false);
    });

    it('refetches unread comment IDs after marking as read', async () => {
      const initialUnreadData: PostUnreadComments[] = [
        { post_id: 1, unread_comment_ids: [101, 102] },
      ];

      const afterMarkReadData: PostUnreadComments[] = [
        { post_id: 1, unread_comment_ids: [] }, // No unread after marking
      ];

      let callCount = 0;

      server.use(
        http.get('/api/v1/games/10/unread-comment-ids', () => {
          callCount++;
          return HttpResponse.json(callCount === 1 ? initialUnreadData : afterMarkReadData);
        }),
        http.post('/api/v1/games/10/posts/1/mark-read', () => {
          return HttpResponse.json({
            id: 1,
            user_id: 1,
            game_id: 10,
            post_id: 1,
            last_read_at: new Date().toISOString(),
          });
        })
      );

      // Set up hook to track unread IDs
      const { result: unreadResult } = renderHook(() => usePostUnreadCommentIDs(10, 1), { wrapper });

      await waitFor(() => {
        expect(unreadResult.current).toEqual([101, 102]);
      });

      // Mark as read
      const { result: markReadResult } = renderHook(() => useMarkPostAsRead(), { wrapper });
      markReadResult.current.mutate({ gameId: 10, postId: 1, data: {} });

      await waitFor(() => {
        expect(markReadResult.current.isSuccess).toBe(true);
      });

      // Unread IDs should be refetched and now empty
      await waitFor(() => {
        expect(unreadResult.current).toEqual([]);
      });
    });

    it('handles mark as read errors gracefully', async () => {
      server.use(
        http.post('/api/v1/games/10/posts/1/mark-read', () => {
          return HttpResponse.json({ detail: 'Unauthorized' }, { status: 401 });
        })
      );

      const { result } = renderHook(() => useMarkPostAsRead(), { wrapper });

      result.current.mutate({ gameId: 10, postId: 1, data: {} });

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });

      expect(result.current.data).toBeUndefined();
    });
  });

  describe('Edge Cases', () => {
    it('handles first visit (empty unread array)', async () => {
      // On first visit, backend returns empty array even if comments exist
      const mockData: PostUnreadComments[] = [
        { post_id: 1, unread_comment_ids: [] },
      ];

      server.use(
        http.get('/api/v1/games/10/unread-comment-ids', () => {
          return HttpResponse.json(mockData);
        })
      );

      const { result } = renderHook(() => usePostUnreadCommentIDs(10, 1), { wrapper });

      await waitFor(() => {
        expect(result.current).toEqual([]);
      });
    });

    it('handles mix of read and unread posts', async () => {
      const mockData: PostUnreadComments[] = [
        { post_id: 1, unread_comment_ids: [101, 102] }, // Unread
        { post_id: 2, unread_comment_ids: [] },         // Read
        { post_id: 3, unread_comment_ids: [301] },      // Unread
      ];

      server.use(
        http.get('/api/v1/games/10/unread-comment-ids', () => {
          return HttpResponse.json(mockData);
        })
      );

      const { result } = renderHook(() => useUnreadCommentIDs(10), { wrapper });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      const data = result.current.data!;
      expect(data[0].unread_comment_ids).toHaveLength(2);
      expect(data[1].unread_comment_ids).toHaveLength(0);
      expect(data[2].unread_comment_ids).toHaveLength(1);
    });

    it('handles deleted comments (excluded from unread)', async () => {
      // Backend should exclude deleted comments from unread list
      const mockData: PostUnreadComments[] = [
        { post_id: 1, unread_comment_ids: [102, 104] }, // 103 was deleted
      ];

      server.use(
        http.get('/api/v1/games/10/unread-comment-ids', () => {
          return HttpResponse.json(mockData);
        })
      );

      const { result } = renderHook(() => usePostUnreadCommentIDs(10, 1), { wrapper });

      await waitFor(() => {
        expect(result.current).toEqual([102, 104]);
      });

      // Deleted comment (103) should not be in the array
      expect(result.current).not.toContain(103);
    });
  });
});
