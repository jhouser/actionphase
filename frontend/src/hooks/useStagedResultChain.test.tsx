import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import React from 'react';
import { useCreateStagedResultChain, useCancelPendingStagedPart } from './useActionResults';
import { apiClient } from '../lib/api';

vi.mock('../lib/api', () => ({
  apiClient: {
    phases: {
      createStagedResultChain: vi.fn(),
      cancelPendingStagedPart: vi.fn(),
    },
  },
}));

const mockedApi = vi.mocked(apiClient.phases);

const GAME_ID = 164;

// Each test gets its own client so invalidation assertions cannot leak between
// tests through shared cache state.
function makeWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  return { wrapper, queryClient };
}

describe('useCreateStagedResultChain', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('posts the chain as a single request', async () => {
    mockedApi.createStagedResultChain.mockResolvedValue({ data: [] } as never);
    const { wrapper } = makeWrapper();

    const { result } = renderHook(() => useCreateStagedResultChain(GAME_ID), { wrapper });

    const payload = {
      user_id: 7,
      is_published: true,
      parts: [
        { content: 'The sword whooshes toward your head...', delay_minutes: 0 },
        { content: '...and misses!', delay_minutes: 15 },
      ],
    };

    await act(async () => {
      await result.current.mutateAsync(payload);
    });

    // One call, whole chain. Posting part-by-part would leave a partial chain
    // behind if a later request failed.
    expect(mockedApi.createStagedResultChain).toHaveBeenCalledTimes(1);
    expect(mockedApi.createStagedResultChain).toHaveBeenCalledWith(GAME_ID, payload);
  });

  it('preserves part order and per-part delays in the payload', async () => {
    mockedApi.createStagedResultChain.mockResolvedValue({ data: [] } as never);
    const { wrapper } = makeWrapper();

    const { result } = renderHook(() => useCreateStagedResultChain(GAME_ID), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({
        user_id: 7,
        parts: [
          { content: 'one', delay_minutes: 0 },
          { content: 'two', delay_minutes: 15 },
          { content: 'three', delay_minutes: 30 },
        ],
      });
    });

    const sent = mockedApi.createStagedResultChain.mock.calls[0][1];
    expect(sent.parts.map((p) => p.content)).toEqual(['one', 'two', 'three']);
    // The head releases on publish, so its delay is always 0; later delays are
    // measured from the previous part's reveal.
    expect(sent.parts.map((p) => p.delay_minutes)).toEqual([0, 15, 30]);
  });

  it('refreshes both result lists after creating a chain', async () => {
    mockedApi.createStagedResultChain.mockResolvedValue({ data: [] } as never);
    const { wrapper, queryClient } = makeWrapper();
    const invalidate = vi.spyOn(queryClient, 'invalidateQueries');

    const { result } = renderHook(() => useCreateStagedResultChain(GAME_ID), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({
        user_id: 7,
        parts: [{ content: 'one', delay_minutes: 0 }, { content: 'two', delay_minutes: 15 }],
      });
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    // The GM's composing view and the player's own list both show the new head.
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ['actionResults', 'game', GAME_ID] });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ['actionResults', 'user', GAME_ID] });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ['unpublishedResultsCount'] });
  });

  it('surfaces a rejected chain to the caller', async () => {
    // The API returns 400 with a message naming the offending part, which the
    // composer shows to the GM.
    mockedApi.createStagedResultChain.mockRejectedValue(
      Object.assign(new Error('Request failed'), { response: { status: 400 } })
    );
    const { wrapper } = makeWrapper();

    const { result } = renderHook(() => useCreateStagedResultChain(GAME_ID), { wrapper });

    await act(async () => {
      await result.current
        .mutateAsync({ user_id: 7, parts: [{ content: 'only one', delay_minutes: 0 }] })
        .catch(() => undefined);
    });

    await waitFor(() => expect(result.current.isError).toBe(true));
  });
});

describe('useCancelPendingStagedPart', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('cancels by result id', async () => {
    mockedApi.cancelPendingStagedPart.mockResolvedValue({ data: undefined } as never);
    const { wrapper } = makeWrapper();

    const { result } = renderHook(() => useCancelPendingStagedPart(GAME_ID), { wrapper });

    await act(async () => {
      await result.current.mutateAsync(42);
    });

    expect(mockedApi.cancelPendingStagedPart).toHaveBeenCalledWith(GAME_ID, 42);
  });

  it('refreshes result lists but not the unpublished count', async () => {
    mockedApi.cancelPendingStagedPart.mockResolvedValue({ data: undefined } as never);
    const { wrapper, queryClient } = makeWrapper();
    const invalidate = vi.spyOn(queryClient, 'invalidateQueries');

    const { result } = renderHook(() => useCancelPendingStagedPart(GAME_ID), { wrapper });

    await act(async () => {
      await result.current.mutateAsync(42);
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(invalidate).toHaveBeenCalledWith({ queryKey: ['actionResults', 'game', GAME_ID] });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ['actionResults', 'user', GAME_ID] });

    // A pending part is already published — it waits on its timer, not on the
    // GM — so cancelling it cannot change the unpublished count.
    expect(invalidate).not.toHaveBeenCalledWith({ queryKey: ['unpublishedResultsCount'] });
  });
});
