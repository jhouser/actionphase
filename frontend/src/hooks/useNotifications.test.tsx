import { describe, it, expect, beforeEach, beforeAll, afterEach, afterAll, vi } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import {
  useNotifications,
  useUnreadCount,
  useMarkNotificationAsRead,
  useMarkAllAsRead,
  useDeleteNotification,
  useAutoMarkNotificationRead,
} from './useNotifications';
import type { Notification } from '../types/notifications';
import { AuthProvider } from '../contexts/AuthContext'
import { ToastProvider } from '../contexts/ToastContext'
import { MemoryRouter } from 'react-router-dom';

// Setup MSW server
const server = setupServer();

beforeAll(() => server.listen());
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

describe('useNotifications hooks', () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
          // Disable refetch intervals for tests
          refetchInterval: false,
          refetchOnWindowFocus: false,
        },
        mutations: {
          retry: false,
        },
      },
    });
  });

  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <MemoryRouter>
      <QueryClientProvider client={queryClient}>
        <ToastProvider>
          <AuthProvider>
            {children}
          </AuthProvider>
        </ToastProvider>
      </QueryClientProvider>
    </MemoryRouter>
  );

  const createMockNotifications = (count: number): Notification[] => {
    return Array.from({ length: count }, (_, i) => ({
      id: i + 1,
      user_id: 1,
      game_id: 10,
      type: 'private_message',
      title: `Notification ${i + 1}`,
      is_read: false,
      created_at: new Date().toISOString(),
    }));
  };

  describe('useNotifications', () => {
    it('fetches notifications successfully', async () => {
      const mockNotifications = createMockNotifications(5);

      server.use(
        http.get('http://localhost:3000/api/v1/notifications', () => {
          return HttpResponse.json({
            data: mockNotifications,
            pagination: { total: 5, limit: 20, offset: 0 },
          });
        })
      );

      const { result } = renderHook(() => useNotifications(), { wrapper });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.data?.data).toHaveLength(5);
      expect(result.current.data?.data[0].title).toBe('Notification 1');
    });

    it('fetches notifications with custom params', async () => {
      let requestParams: URLSearchParams | null = null;

      server.use(
        http.get('http://localhost:3000/api/v1/notifications', ({ request }) => {
          const url = new URL(request.url);
          requestParams = url.searchParams;

          return HttpResponse.json({
            data: [],
            pagination: { total: 0, limit: 10, offset: 0 },
          });
        })
      );

      renderHook(() => useNotifications({ limit: 10, unread: true }), { wrapper });

      await waitFor(() => {
        expect(requestParams?.get('limit')).toBe('10');
        expect(requestParams?.get('unread')).toBe('true');
        // Note: offset=0 is omitted from query params by the API client
      });
    });

    it('handles API errors', async () => {
      server.use(
        http.get('http://localhost:3000/api/v1/notifications', () => {
          return HttpResponse.error();
        })
      );

      const { result } = renderHook(() => useNotifications(), { wrapper });

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });
    });

  });

  describe('useUnreadCount', () => {
    it('fetches unread count successfully', async () => {
      server.use(
        http.get('http://localhost:3000/api/v1/notifications/unread-count', () => {
          return HttpResponse.json({ unread_count: 7 });
        })
      );

      const { result } = renderHook(() => useUnreadCount(), { wrapper });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.data).toBe(7);
    });

    it('returns 0 when no unread notifications', async () => {
      server.use(
        http.get('http://localhost:3000/api/v1/notifications/unread-count', () => {
          return HttpResponse.json({ unread_count: 0 });
        })
      );

      const { result } = renderHook(() => useUnreadCount(), { wrapper });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.data).toBe(0);
    });

    it('handles API errors', async () => {
      server.use(
        http.get('http://localhost:3000/api/v1/notifications/unread-count', () => {
          return HttpResponse.error();
        })
      );

      const { result } = renderHook(() => useUnreadCount(), { wrapper });

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });
    });
  });

  describe('useMarkNotificationAsRead', () => {
    it('marks notification as read successfully', async () => {
      server.use(
        http.put('http://localhost:3000/api/v1/notifications/:id/mark-read', () => {
          return HttpResponse.json({ success: true });
        })
      );

      const { result } = renderHook(() => useMarkNotificationAsRead(), { wrapper });

      result.current.mutate(1);

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });
    });

    it('invalidates queries on success', async () => {
      server.use(
        http.put('http://localhost:3000/api/v1/notifications/:id/mark-read', () => {
          return HttpResponse.json({ success: true });
        })
      );

      const { result } = renderHook(() => useMarkNotificationAsRead(), { wrapper });

      const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');

      result.current.mutate(1);

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['notifications'] });
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['dashboard'] });
    });

    it('handles API errors', async () => {
      server.use(
        http.put('http://localhost:3000/api/v1/notifications/:id/mark-read', () => {
          return HttpResponse.error();
        })
      );

      const { result } = renderHook(() => useMarkNotificationAsRead(), { wrapper });

      result.current.mutate(1);

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });
    });
  });

  describe('useMarkAllAsRead', () => {
    it('marks all notifications as read successfully', async () => {
      server.use(
        http.put('http://localhost:3000/api/v1/notifications/mark-all-read', () => {
          return HttpResponse.json({ marked_count: 5 });
        })
      );

      const { result } = renderHook(() => useMarkAllAsRead(), { wrapper });

      result.current.mutate();

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.data?.marked_count).toBe(5);
    });

    it('invalidates queries on success', async () => {
      server.use(
        http.put('http://localhost:3000/api/v1/notifications/mark-all-read', () => {
          return HttpResponse.json({ marked_count: 3 });
        })
      );

      const { result } = renderHook(() => useMarkAllAsRead(), { wrapper });

      const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');

      result.current.mutate();

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['notifications'] });
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['dashboard'] });
    });

    it('handles API errors', async () => {
      server.use(
        http.put('http://localhost:3000/api/v1/notifications/mark-all-read', () => {
          return HttpResponse.error();
        })
      );

      const { result } = renderHook(() => useMarkAllAsRead(), { wrapper });

      result.current.mutate();

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });
    });
  });

  describe('useDeleteNotification', () => {
    it('deletes notification successfully', async () => {
      server.use(
        http.delete('http://localhost:3000/api/v1/notifications/:id', () => {
          return new HttpResponse(null, { status: 204 });
        })
      );

      const { result } = renderHook(() => useDeleteNotification(), { wrapper });

      result.current.mutate(1);

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });
    });

    it('invalidates queries on success', async () => {
      server.use(
        http.delete('http://localhost:3000/api/v1/notifications/:id', () => {
          return new HttpResponse(null, { status: 204 });
        })
      );

      const { result } = renderHook(() => useDeleteNotification(), { wrapper });

      const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');

      result.current.mutate(1);

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['notifications'] });
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['dashboard'] });
    });

    it('handles API errors', async () => {
      server.use(
        http.delete('http://localhost:3000/api/v1/notifications/:id', () => {
          return HttpResponse.error();
        })
      );

      const { result } = renderHook(() => useDeleteNotification(), { wrapper });

      result.current.mutate(1);

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });
    });
  });

  describe('Query keys', () => {
    it('uses different query keys for different hook types', async () => {
      server.use(
        http.get('http://localhost:3000/api/v1/notifications', () => {
          return HttpResponse.json({
            data: [],
            pagination: { total: 0, limit: 20, offset: 0 },
          });
        }),
        http.get('http://localhost:3000/api/v1/notifications/unread-count', () => {
          return HttpResponse.json({ unread_count: 0 });
        })
      );

      // Render both hooks
      const { result: _notificationsResult } = renderHook(() => useNotifications(), { wrapper });
      const { result: _unreadCountResult } = renderHook(() => useUnreadCount(), { wrapper });

      await waitFor(() => {
        expect(_notificationsResult.current.isSuccess).toBe(true);
        expect(_unreadCountResult.current.isSuccess).toBe(true);
      });

      // They should use different query keys — verified by inspecting the cache
      const cache = queryClient.getQueryCache();
      const queries = cache.getAll();
      const keys = queries.map(q => JSON.stringify(q.queryKey));
      // The two query keys must be distinct
      expect(new Set(keys).size).toBe(keys.length);
    });

    it('uses different query keys for different params', async () => {
      server.use(
        http.get('http://localhost:3000/api/v1/notifications', () => {
          return HttpResponse.json({
            data: [],
            pagination: { total: 0, limit: 20, offset: 0 },
          });
        })
      );

      // Render with different params
      const { result: _result1 } = renderHook(
        () => useNotifications({ limit: 10 }),
        { wrapper }
      );
      const { result: _result2 } = renderHook(
        () => useNotifications({ limit: 20 }),
        { wrapper }
      );

      await waitFor(() => {
        expect(_result1.current.isSuccess).toBe(true);
        expect(_result2.current.isSuccess).toBe(true);
      });

      // They should be cached separately — two distinct query entries
      const cache = queryClient.getQueryCache();
      const queries = cache.getAll();
      expect(queries.length).toBeGreaterThanOrEqual(2);
      const keys = queries.map(q => JSON.stringify(q.queryKey));
      expect(new Set(keys).size).toBe(keys.length);
    });
  });

  describe('useAutoMarkNotificationRead', () => {
    const makeWrapper = (initialUrl: string) => {
      const qc = new QueryClient({
        defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
      });
      return ({ children }: { children: React.ReactNode }) => (
        <MemoryRouter initialEntries={[initialUrl]}>
          <QueryClientProvider client={qc}>
            <ToastProvider>
              <AuthProvider>{children}</AuthProvider>
            </ToastProvider>
          </QueryClientProvider>
        </MemoryRouter>
      );
    };

    it('calls mark-as-read when ?notif param is present', async () => {
      let markedId: number | null = null;
      server.use(
        http.put('/api/v1/notifications/:id/mark-read', ({ params }) => {
          markedId = parseInt(params.id as string, 10);
          return HttpResponse.json({ success: true });
        })
      );

      renderHook(() => useAutoMarkNotificationRead(), {
        wrapper: makeWrapper('/games/1?tab=messages&notif=42'),
      });

      await waitFor(() => {
        expect(markedId).toBe(42);
      });
    });

    it('does not call mark-as-read when ?notif param is absent', async () => {
      let markCalled = false;
      server.use(
        http.put('/api/v1/notifications/:id/mark-read', () => {
          markCalled = true;
          return HttpResponse.json({ success: true });
        })
      );

      renderHook(() => useAutoMarkNotificationRead(), {
        wrapper: makeWrapper('/games/1?tab=messages'),
      });

      // This hook returns early with no observable effect when ?notif is absent,
      // so there's no positive signal to wait on — allow real time to pass and
      // confirm no request was made. The sleep runs inside act() so provider
      // state settling meanwhile doesn't trigger a React act warning. A bare
      // `await waitFor(() => expect(markCalled).toBe(false))` would be vacuous:
      // it passes on the first poll, before any request could land.
      await act(async () => {
        await new Promise(r => setTimeout(r, 50));
      });
      expect(markCalled).toBe(false);
    });

    it('ignores non-numeric notif param', async () => {
      let markCalled = false;
      server.use(
        http.put('/api/v1/notifications/:id/mark-read', () => {
          markCalled = true;
          return HttpResponse.json({ success: true });
        })
      );

      renderHook(() => useAutoMarkNotificationRead(), {
        wrapper: makeWrapper('/games/1?notif=abc'),
      });

      // Allow real time to pass inside act() — see note above for why waitFor
      // on the negative assertion alone would be vacuous here.
      await act(async () => {
        await new Promise(r => setTimeout(r, 50));
      });
      expect(markCalled).toBe(false);
    });
  });
});
