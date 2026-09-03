import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen, fireEvent, waitFor } from '@testing-library/react'
import { renderWithProviders } from '../../../test-utils'
import { BanHistoryTab } from '../BanHistoryTab'
import type { Community, CommunityBanEvent } from '../../../types/communities'

const listBanEvents = vi.fn()

vi.mock('../../../lib/api', () => ({
  apiClient: {
    communities: {
      listBanEvents: (slug: string, params: unknown) => listBanEvents(slug, params),
    },
  },
}))

const community: Community = {
  id: 1,
  name: 'Midnight Ravens',
  slug: 'midnight-ravens',
  description: null,
  banner_url: null,
  owner_user_id: 7,
  owner_username: 'corvid',
  is_active: true,
  your_role: 'moderator',
  created_at: '2026-08-01T00:00:00Z',
  updated_at: '2026-08-01T00:00:00Z',
}

const event = (over: Partial<CommunityBanEvent> = {}): CommunityBanEvent => ({
  id: 1,
  community_id: 1,
  target_user_id: 42,
  target_username: 'griefer',
  actor_user_id: 7,
  actor_username: 'corvid',
  action: 'banned',
  reason: 'Repeated harassment',
  created_at: '2026-08-10T00:00:00Z',
  ...over,
})

beforeEach(() => {
  vi.clearAllMocks()
  listBanEvents.mockResolvedValue({ data: [event()] })
})

describe('BanHistoryTab', () => {
  it('lists audit entries with actor and target', async () => {
    renderWithProviders(<BanHistoryTab community={community} canModerate />)

    expect(await screen.findByTestId('ban-event-1')).toBeInTheDocument()
    expect(screen.getByText('griefer')).toBeInTheDocument()
    expect(screen.getByText(/by corvid/)).toBeInTheDocument()
    expect(screen.getByText('Repeated harassment')).toBeInTheDocument()
  })

  // 'modified' must not read as a fresh ban: it means an already-banned user's
  // reason or expiry changed, and calling it a ban implies they were unbanned
  // in between, which never happened.
  it('labels a modified ban as an edit, not a ban', async () => {
    listBanEvents.mockResolvedValue({ data: [event({ action: 'modified' })] })
    renderWithProviders(<BanHistoryTab community={community} canModerate />)

    expect(await screen.findByText('Ban edited')).toBeInTheDocument()
  })

  it('labels an unban', async () => {
    listBanEvents.mockResolvedValue({ data: [event({ action: 'unbanned' })] })
    renderWithProviders(<BanHistoryTab community={community} canModerate />)

    expect(await screen.findByText('Unbanned')).toBeInTheDocument()
  })

  // A deleted moderator's events survive them by design, so the actor really
  // can be absent -- it must not render as a blank or a crash.
  it('handles an entry whose actor has been deleted', async () => {
    listBanEvents.mockResolvedValue({
      data: [event({ actor_user_id: undefined, actor_username: undefined })],
    })
    renderWithProviders(<BanHistoryTab community={community} canModerate />)

    expect(await screen.findByText(/by a deleted user/)).toBeInTheDocument()
  })

  it('reports an empty log', async () => {
    listBanEvents.mockResolvedValue({ data: [] })
    renderWithProviders(<BanHistoryTab community={community} canModerate />)

    expect(await screen.findByTestId('ban-events-empty')).toBeInTheDocument()
  })

  it('does not fetch for a non-moderator', () => {
    renderWithProviders(<BanHistoryTab community={community} canModerate={false} />)

    expect(listBanEvents).not.toHaveBeenCalled()
    expect(screen.getByText(/only this community's moderators/i)).toBeInTheDocument()
  })

  it('requests the first page with a bounded limit', async () => {
    renderWithProviders(<BanHistoryTab community={community} canModerate />)

    await waitFor(() => expect(listBanEvents).toHaveBeenCalled())
    expect(listBanEvents).toHaveBeenCalledWith('midnight-ravens', {
      limit: 50,
      offset: 0,
    })
  })

  // Paging must move the OFFSET, not just the page number, or "Older" would
  // re-request the same rows.
  it('pages by offset', async () => {
    listBanEvents.mockResolvedValue({
      data: Array.from({ length: 50 }, (_, i) => event({ id: i + 1 })),
    })
    renderWithProviders(<BanHistoryTab community={community} canModerate />)

    fireEvent.click(await screen.findByTestId('ban-events-next'))

    await waitFor(() =>
      expect(listBanEvents).toHaveBeenLastCalledWith('midnight-ravens', {
        limit: 50,
        offset: 50,
      })
    )
  })

  // A short page cannot be followed by another, so offering "Older" would lead
  // to a guaranteed empty screen.
  it('disables paging forward on a short page', async () => {
    renderWithProviders(<BanHistoryTab community={community} canModerate />)

    await screen.findByTestId('ban-event-1')
    expect(screen.queryByTestId('ban-events-next')).not.toBeInTheDocument()
  })
})
