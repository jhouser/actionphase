import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement } from 'react';
import { useUnreadInbox } from './useUnreadInbox';
import type { Notification } from '../types/notifications';

const mockGetNotifications = vi.fn();

vi.mock('../lib/api', () => ({
  apiClient: {
    notifications: {
      getNotifications: (...args: unknown[]) => mockGetNotifications(...args),
    },
  },
}));

vi.mock('../contexts/AuthContext', () => ({
  useAuth: () => ({ isAuthenticated: true }),
}));

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

function makeNotification(overrides: Partial<Notification> = {}): Notification {
  return {
    id: 1,
    user_id: 1,
    game_id: 12,
    type: 'comment_reply',
    title: 'Someone replied',
    is_read: false,
    created_at: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

describe('useUnreadInbox', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('fetches unread notifications with the unread-inbox query key', async () => {
    mockGetNotifications.mockResolvedValue({ data: { data: [], pagination: { total: 0, limit: 100, offset: 0 } } });

    const { result } = renderHook(() => useUnreadInbox(), { wrapper: createWrapper() });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockGetNotifications).toHaveBeenCalledWith({ unread: true, limit: 100 });
  });

  it('classifies repliable notifications into inbox items', async () => {
    mockGetNotifications.mockResolvedValue({
      data: {
        data: [
          makeNotification({
            id: 1,
            type: 'comment_reply',
            related_type: 'comment',
            related_id: 99,
          }),
          makeNotification({
            id: 2,
            type: 'private_message',
            related_type: 'message',
            related_id: 55,
            link_url: '/games/12?tab=messages&conversation=34',
          }),
        ],
        pagination: { total: 2, limit: 100, offset: 0 },
      },
    });

    const { result } = renderHook(() => useUnreadInbox(), { wrapper: createWrapper() });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual([
      { kind: 'comment', notification: expect.objectContaining({ id: 1 }), gameId: 12, commentId: 99 },
      {
        kind: 'private_message',
        notification: expect.objectContaining({ id: 2 }),
        gameId: 12,
        conversationId: 34,
        messageId: 55,
        unreadCount: 1,
      },
    ]);
  });

  it('filters out non-repliable notification types', async () => {
    mockGetNotifications.mockResolvedValue({
      data: {
        data: [
          makeNotification({ id: 1, type: 'common_room_post' }),
          makeNotification({ id: 2, type: 'phase_created' }),
        ],
        pagination: { total: 2, limit: 100, offset: 0 },
      },
    });

    const { result } = renderHook(() => useUnreadInbox(), { wrapper: createWrapper() });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual([]);
  });
});
