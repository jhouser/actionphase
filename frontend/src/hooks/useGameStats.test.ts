import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement } from 'react';
import { useGameStats } from './useGameStats';

// The fixture is built inside the factory because vi.mock is hoisted above any
// module-scope const it would otherwise close over.
vi.mock('../lib/api', () => ({
  apiClient: {
    games: {
      getGameStats: vi.fn().mockResolvedValue({
        data: {
          comments: {
            total_comments: 412,
            avg_comment_length: 340,
            longest_comment_length: 2100,
            max_thread_depth: 7,
            avg_thread_depth: 2.4,
          },
          private_messages: {
            total_private_messages: 1203,
            avg_private_message_length: 96,
            longest_private_message_length: 800,
            active_conversations: 18,
          },
          characters: [
            {
              character_id: 1,
              character_name: 'Alice',
              is_active: true,
              comment_count: 61,
              avg_comment_length: 402,
              private_message_count: 180,
              avg_private_message_length: 88,
            },
          ],
        },
      }),
    },
  },
}));

import { apiClient } from '../lib/api';

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

describe('useGameStats', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('fetches stats for a completed game', async () => {
    const { result } = renderHook(() => useGameStats(7, 'completed'), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(apiClient.games.getGameStats).toHaveBeenCalledWith(7);
    expect(result.current.data?.comments.total_comments).toBe(412);
    expect(result.current.data?.characters[0].character_name).toBe('Alice');
  });

  it('does not fetch while the game is still in progress', async () => {
    // The endpoint 409s for non-completed games, so the query must stay idle
    // rather than firing a request that is guaranteed to fail.
    const { result } = renderHook(() => useGameStats(7, 'in_progress'), {
      wrapper: createWrapper(),
    });

    expect(result.current.fetchStatus).toBe('idle');
    expect(apiClient.games.getGameStats).not.toHaveBeenCalled();
  });

  it('does not fetch without a game id', () => {
    const { result } = renderHook(() => useGameStats(undefined, 'completed'), {
      wrapper: createWrapper(),
    });

    expect(result.current.fetchStatus).toBe('idle');
    expect(apiClient.games.getGameStats).not.toHaveBeenCalled();
  });
});
