import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import React from 'react';
import { useGameExport } from './useGameExport';
import { apiClient } from '../lib/api';

vi.mock('../lib/api', () => ({
  apiClient: {
    exports: {
      requestExport: vi.fn(),
      getLatestExport: vi.fn(),
      getDownloadUrl: (id: number) => `/api/v1/exports/${id}/download`,
    },
  },
}));

const mockedApi = vi.mocked(apiClient.exports);

function wrapper({ children }: { children: React.ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

function notFoundError() {
  return Object.assign(new Error('Request failed'), { response: { status: 404 } });
}

describe('useGameExport', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers({ shouldAdvanceTime: true });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('treats a 404 as "no export yet" rather than an error', async () => {
    mockedApi.getLatestExport.mockRejectedValue(notFoundError());

    const { result } = renderHook(() => useGameExport(164), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.export).toBeUndefined();
    expect(result.current.isWorking).toBe(false);
  });

  it('does not query when disabled', () => {
    renderHook(() => useGameExport(164, { enabled: false }), { wrapper });
    expect(mockedApi.getLatestExport).not.toHaveBeenCalled();
  });

  it('does not query for an invalid game id', () => {
    renderHook(() => useGameExport(0), { wrapper });
    expect(mockedApi.getLatestExport).not.toHaveBeenCalled();
  });

  // The behavior that keeps an idle archive page from hammering the API.
  it('stops polling once the export completes', async () => {
    mockedApi.getLatestExport.mockResolvedValue({
      data: { id: 1, game_id: 164, status: 'complete', download_url: '/x' },
    } as never);

    const { result } = renderHook(() => useGameExport(164), { wrapper });
    await waitFor(() => expect(result.current.export?.status).toBe('complete'));

    const callsAfterLoad = mockedApi.getLatestExport.mock.calls.length;
    await act(async () => {
      await vi.advanceTimersByTimeAsync(15000);
    });

    expect(mockedApi.getLatestExport.mock.calls.length).toBe(callsAfterLoad);
  });

  it('stops polling once the export fails', async () => {
    mockedApi.getLatestExport.mockResolvedValue({
      data: { id: 1, game_id: 164, status: 'failed', error: 'boom' },
    } as never);

    const { result } = renderHook(() => useGameExport(164), { wrapper });
    await waitFor(() => expect(result.current.export?.status).toBe('failed'));

    const callsAfterLoad = mockedApi.getLatestExport.mock.calls.length;
    await act(async () => {
      await vi.advanceTimersByTimeAsync(15000);
    });

    expect(mockedApi.getLatestExport.mock.calls.length).toBe(callsAfterLoad);
  });

  it('keeps polling while the export is running', async () => {
    mockedApi.getLatestExport.mockResolvedValue({
      data: { id: 1, game_id: 164, status: 'running' },
    } as never);

    const { result } = renderHook(() => useGameExport(164), { wrapper });
    await waitFor(() => expect(result.current.isWorking).toBe(true));

    const callsAfterLoad = mockedApi.getLatestExport.mock.calls.length;
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10000);
    });

    expect(mockedApi.getLatestExport.mock.calls.length).toBeGreaterThan(callsAfterLoad);
  });

  it('seeds the cache from the request response so polling starts immediately', async () => {
    mockedApi.getLatestExport.mockRejectedValue(notFoundError());
    mockedApi.requestExport.mockResolvedValue({
      data: { id: 9, game_id: 164, status: 'pending' },
    } as never);

    const { result } = renderHook(() => useGameExport(164), { wrapper });
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    act(() => result.current.requestExport());

    await waitFor(() => expect(result.current.export?.id).toBe(9));
    expect(result.current.isWorking).toBe(true);
  });

  it('exposes and clears request errors', async () => {
    mockedApi.getLatestExport.mockRejectedValue(notFoundError());
    mockedApi.requestExport.mockRejectedValue(new Error('nope'));

    const { result } = renderHook(() => useGameExport(164), { wrapper });
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    act(() => result.current.requestExport());
    await waitFor(() => expect(result.current.requestError?.message).toBe('nope'));

    act(() => result.current.dismissError());
    expect(result.current.requestError).toBeNull();
  });
});
