import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, waitFor, act } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createElement, type ReactNode } from 'react'
import {
  useCommunityDocuments,
  useManageCommunityDocuments,
} from '../useCommunities'
import { apiClient } from '../../lib/api'
import type { CommunityDocument } from '../../types/communities'

vi.mock('../../lib/api', () => ({
  apiClient: {
    communities: {
      listDocuments: vi.fn(),
      listAllDocuments: vi.fn(),
      createDocument: vi.fn(),
      updateDocument: vi.fn(),
      deleteDocument: vi.fn(),
      listGameCommunityDocuments: vi.fn(),
    },
  },
}))

const doc = (id: number, status: 'draft' | 'published'): CommunityDocument => ({
  id,
  community_id: 1,
  title: `doc-${id}`,
  content: 'body',
  status,
  sort_order: id,
  created_at: '2026-08-01T00:00:00Z',
  updated_at: '2026-08-01T00:00:00Z',
})

let client: QueryClient

function wrapper({ children }: { children: ReactNode }) {
  return createElement(QueryClientProvider, { client }, children)
}

const listDocuments = vi.mocked(apiClient.communities.listDocuments)
const listAllDocuments = vi.mocked(apiClient.communities.listAllDocuments)
const createDocument = vi.mocked(apiClient.communities.createDocument)

beforeEach(() => {
  vi.clearAllMocks()
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  listDocuments.mockResolvedValue({ data: [doc(20, 'published')] } as never)
  listAllDocuments.mockResolvedValue({
    data: [doc(20, 'published'), doc(21, 'draft')],
  } as never)
  createDocument.mockResolvedValue({ data: doc(22, 'draft') } as never)
})

describe('community document hooks', () => {
  it('reads the public list from the public endpoint', async () => {
    const { result } = renderHook(() => useCommunityDocuments('midnight-ravens'), {
      wrapper,
    })

    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(listDocuments).toHaveBeenCalledWith('midnight-ravens')
    expect(listAllDocuments).not.toHaveBeenCalled()
  })

  it('reads the manage list from the privileged endpoint', async () => {
    const { result } = renderHook(
      () => useManageCommunityDocuments('midnight-ravens'),
      { wrapper }
    )

    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(listAllDocuments).toHaveBeenCalledWith('midnight-ravens')
    expect(result.current.documents).toHaveLength(2)
  })

  // The two lists are SEPARATE cache keys on purpose: they return different
  // data for the same community, so sharing a key would let whichever resolved
  // last serve the other's consumer -- leaking a draft into the public page.
  it('keeps the public and manage lists on separate cache keys', async () => {
    const pub = renderHook(() => useCommunityDocuments('midnight-ravens'), { wrapper })
    const mgmt = renderHook(() => useManageCommunityDocuments('midnight-ravens'), {
      wrapper,
    })

    await waitFor(() => expect(pub.result.current.isLoading).toBe(false))
    await waitFor(() => expect(mgmt.result.current.isLoading).toBe(false))

    expect(pub.result.current.documents).toHaveLength(1)
    expect(mgmt.result.current.documents).toHaveLength(2)
  })

  // Publishing changes what an ordinary visitor sees, and that is a different
  // cache key. Without this invalidation a moderator publishes a document and
  // the public page still shows the old set.
  it('refreshes the public list after a write', async () => {
    const { result } = renderHook(
      () => useManageCommunityDocuments('midnight-ravens'),
      { wrapper }
    )
    await waitFor(() => expect(result.current.isLoading).toBe(false))

    const spy = vi.spyOn(client, 'invalidateQueries')
    await act(async () => {
      await result.current.createDocument.mutateAsync({
        title: 'New',
        content: 'text',
      })
    })

    const keys = spy.mock.calls
      .map((c) => (c[0] as { queryKey?: unknown })?.queryKey)
      .filter(Boolean)
    expect(keys).toContainEqual(['communities', 'midnight-ravens', 'documents'])
  })

  // A published document appears on the Info tab of EVERY game in the
  // community. Those lists are keyed by game id, so the invalidation uses a
  // predicate rather than needing to know which games exist.
  it('refreshes every per-game list after a write', async () => {
    const { result } = renderHook(
      () => useManageCommunityDocuments('midnight-ravens'),
      { wrapper }
    )
    await waitFor(() => expect(result.current.isLoading).toBe(false))

    const spy = vi.spyOn(client, 'invalidateQueries')
    await act(async () => {
      await result.current.createDocument.mutateAsync({
        title: 'New',
        content: 'text',
      })
    })

    const predicates = spy.mock.calls
      .map((c) => (c[0] as { predicate?: (q: unknown) => boolean })?.predicate)
      .filter(Boolean) as ((q: unknown) => boolean)[]
    expect(predicates.length).toBeGreaterThan(0)

    // The predicate must match a per-game document list and nothing else.
    expect(
      predicates.some((p) =>
        p({ queryKey: ['games', 5, 'community-documents'] })
      )
    ).toBe(true)
    expect(
      predicates.some((p) => p({ queryKey: ['games', 5, 'phases'] }))
    ).toBe(false)
  })

  it('does not fetch the manage list when the caller cannot moderate', async () => {
    renderHook(() => useManageCommunityDocuments('midnight-ravens', false), {
      wrapper,
    })

    await waitFor(() => expect(listAllDocuments).not.toHaveBeenCalled())
  })
})
