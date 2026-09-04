import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement, type ReactNode } from 'react';
import { useSelectableCommunities } from '../useCommunities';
import { apiClient } from '../../lib/api';
import type { Community } from '../../types/communities';

vi.mock('../../lib/api', () => ({
  apiClient: {
    communities: {
      listActiveCommunities: vi.fn(),
    },
  },
}));

function community(id: number, slug: string, isBanned?: boolean): Community {
  return {
    id,
    name: slug,
    slug,
    description: null,
    banner_url: null,
    owner_user_id: 1,
    is_active: true,
    ...(isBanned === undefined ? {} : { is_banned: isBanned }),
    created_at: '2026-08-01T00:00:00Z',
    updated_at: '2026-08-01T00:00:00Z',
  };
}

function wrapper({ children }: { children: ReactNode }) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return createElement(QueryClientProvider, { client }, children);
}

const listActive = vi.mocked(apiClient.communities.listActiveCommunities);

beforeEach(() => {
  vi.clearAllMocks();
  listActive.mockResolvedValue({
    data: [community(1, 'ravens', false), community(2, 'harbor', false)],
  } as never);
});

describe('useSelectableCommunities', () => {
  it('returns every active community when the user is banned from none', async () => {
    const { result } = renderHook(() => useSelectableCommunities(), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.communities.map((c) => c.slug)).toEqual(['ravens', 'harbor']);
  });

  // The point of the hook: the picker must not offer a community whose game
  // creation the server will refuse with a 403.
  it('excludes a community the user is banned from', async () => {
    listActive.mockResolvedValue({
      data: [community(1, 'ravens', true), community(2, 'harbor', false)],
    } as never);
    const { result } = renderHook(() => useSelectableCommunities(), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.communities.map((c) => c.slug)).toEqual(['harbor']);
  });

  it('can exclude every community', async () => {
    listActive.mockResolvedValue({
      data: [community(1, 'ravens', true), community(2, 'harbor', true)],
    } as never);
    const { result } = renderHook(() => useSelectableCommunities(), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.communities).toEqual([]);
  });

  // An older cached payload predating the field must not empty the picker. The
  // server refuses a banned choice regardless, so treating absent as "not
  // banned" costs at worst one rejected submit -- while treating it as banned
  // would leave the user with no community to pick at all.
  it('treats a missing is_banned as not banned', async () => {
    listActive.mockResolvedValue({
      data: [community(1, 'ravens'), community(2, 'harbor')],
    } as never);
    const { result } = renderHook(() => useSelectableCommunities(), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.communities.map((c) => c.slug)).toEqual(['ravens', 'harbor']);
  });

  // The flag rides on the community payload, so there is no second request to
  // outrace. CreateGameForm preselects when exactly one community is offered;
  // that decision is safe the moment this single query settles.
  it('reports ready off the one community request', async () => {
    let release: (v: unknown) => void = () => {};
    listActive.mockReturnValue(
      new Promise((resolve) => {
        release = resolve;
      }) as never
    );

    const { result } = renderHook(() => useSelectableCommunities(), { wrapper });
    expect(result.current.isLoading).toBe(true);

    release({ data: [community(1, 'ravens', true), community(2, 'harbor', false)] });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.communities.map((c) => c.slug)).toEqual(['harbor']);
  });
});
