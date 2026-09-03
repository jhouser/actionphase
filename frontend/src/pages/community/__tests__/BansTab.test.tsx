import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen, fireEvent, waitFor } from '@testing-library/react'
import { renderWithProviders } from '../../../test-utils'
import { BansTab } from '../BansTab'
import type { Community, CommunityBan } from '../../../types/communities'

const listBans = vi.fn()
const banUser = vi.fn()
const unbanUser = vi.fn()
const listModerators = vi.fn()

vi.mock('../../../lib/api', () => ({
  apiClient: {
    communities: {
      listBans: (slug: string) => listBans(slug),
      banUser: (slug: string, data: unknown) => banUser(slug, data),
      unbanUser: (slug: string, userId: number) => unbanUser(slug, userId),
      listModerators: (slug: string) => listModerators(slug),
    },
  },
}))

const showSuccess = vi.fn()
const showError = vi.fn()
vi.mock('../../../contexts/ToastContext', async () => {
  const actual = await vi.importActual('../../../contexts/ToastContext')
  return { ...actual, useToast: () => ({ showSuccess, showError }) }
})

// The user picker has its own tests; standing it in keeps these assertions
// about what the BAN FORM sends rather than about search behaviour. The
// exclusion list is captured because which users the form REFUSES to offer is
// part of the form's contract, not the picker's.
const pickerExcludes = vi.fn()
vi.mock('../../../components/UserSearchSelect', () => ({
  UserSearchSelect: ({
    onChange,
    excludeUserIds,
    'data-testid': testId,
  }: {
    onChange: (u: { id: number; username: string } | null) => void
    excludeUserIds?: number[]
    'data-testid'?: string
  }) => {
    pickerExcludes(excludeUserIds)
    return (
      <button
        type="button"
        data-testid={testId}
        onClick={() => onChange({ id: 42, username: 'griefer' })}
      >
        pick user
      </button>
    )
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

const activeBan: CommunityBan = {
  id: 10,
  community_id: 1,
  user_id: 42,
  username: 'griefer',
  reason: 'Repeated harassment',
  banned_by_username: 'corvid',
  banned_at: '2026-08-10T00:00:00Z',
  is_active: true,
}

// Retained on purpose: expired bans are not deleted, so the UI has to render
// one that exists but enforces nothing.
const expiredBan: CommunityBan = {
  id: 11,
  community_id: 1,
  user_id: 43,
  username: 'cooldown',
  reason: 'Two-week cooldown',
  banned_at: '2026-07-01T00:00:00Z',
  expires_at: '2026-07-15T00:00:00Z',
  is_active: false,
}

beforeEach(() => {
  vi.clearAllMocks()
  listBans.mockResolvedValue({ data: [activeBan] })
  banUser.mockResolvedValue({ data: activeBan })
  unbanUser.mockResolvedValue({ data: undefined })
  listModerators.mockResolvedValue({ data: [] })
})

describe('BansTab', () => {
  it('lists banned users with their reason', async () => {
    renderWithProviders(<BansTab community={community} canModerate />)

    expect(await screen.findByTestId('ban-row-42')).toBeInTheDocument()
    expect(screen.getByText('griefer')).toBeInTheDocument()
    expect(screen.getByText('Repeated harassment')).toBeInTheDocument()
  })

  // THE ASSERTION THAT MATTERS MOST ON THIS SCREEN.
  //
  // Expired bans are retained so a moderator sees a ban lapse rather than
  // vanish. If this ever fails, a two-week ban silently disappears from the
  // list when it runs out and the moderator has no way to tell it existed.
  it('shows an expired ban as expired rather than dropping it', async () => {
    listBans.mockResolvedValue({ data: [activeBan, expiredBan] })
    renderWithProviders(<BansTab community={community} canModerate />)

    expect(await screen.findByTestId('ban-row-43')).toBeInTheDocument()
    expect(screen.getByTestId('ban-status-43')).toHaveTextContent('Expired')
    expect(screen.getByTestId('ban-status-42')).toHaveTextContent('Banned')
  })

  // is_active is the only answer to "is this enforced". A row's presence is
  // not, because expired rows stay.
  it('drives status from is_active, not from the row existing', async () => {
    listBans.mockResolvedValue({ data: [{ ...activeBan, is_active: false }] })
    renderWithProviders(<BansTab community={community} canModerate />)

    expect(await screen.findByTestId('ban-status-42')).toHaveTextContent('Expired')
  })

  it('bans a user with a reason', async () => {
    renderWithProviders(<BansTab community={community} canModerate />)

    fireEvent.click(screen.getByTestId('ban-user-search'))
    fireEvent.change(screen.getByTestId('ban-reason'), {
      target: { value: 'spoilers' },
    })
    fireEvent.click(screen.getByTestId('ban-user-submit'))

    await waitFor(() => expect(banUser).toHaveBeenCalled())
    expect(banUser).toHaveBeenCalledWith('midnight-ravens', {
      user_id: 42,
      reason: 'spoilers',
      // Omitted, not empty: the server reads an absent expiry as permanent.
      expires_at: undefined,
    })
  })

  // An empty reason must not be sent as '', which would store a blank reason
  // rather than none at all.
  it('omits a blank reason instead of sending an empty string', async () => {
    renderWithProviders(<BansTab community={community} canModerate />)

    fireEvent.click(screen.getByTestId('ban-user-search'))
    fireEvent.change(screen.getByTestId('ban-reason'), { target: { value: '   ' } })
    fireEvent.click(screen.getByTestId('ban-user-submit'))

    await waitFor(() => expect(banUser).toHaveBeenCalled())
    expect(banUser.mock.calls[0][1]).toMatchObject({ reason: undefined })
  })

  it('cannot submit until a user is chosen', () => {
    renderWithProviders(<BansTab community={community} canModerate />)

    expect(screen.getByTestId('ban-user-submit')).toBeDisabled()
    fireEvent.click(screen.getByTestId('ban-user-search'))
    expect(screen.getByTestId('ban-user-submit')).toBeEnabled()
  })

  // The server's messages name the actual problem -- staff, unknown user, past
  // expiry -- so surfacing them beats a generic failure line.
  it('surfaces the server’s reason for a refused ban', async () => {
    banUser.mockRejectedValue({
      response: { data: { error: 'community staff cannot be banned' } },
    })
    renderWithProviders(<BansTab community={community} canModerate />)

    fireEvent.click(screen.getByTestId('ban-user-search'))
    fireEvent.click(screen.getByTestId('ban-user-submit'))

    await waitFor(() =>
      expect(showError).toHaveBeenCalledWith('community staff cannot be banned')
    )
  })

  it('lifts a ban', async () => {
    renderWithProviders(<BansTab community={community} canModerate />)

    fireEvent.click(await screen.findByTestId('unban-42'))

    await waitFor(() => expect(unbanUser).toHaveBeenCalledWith('midnight-ravens', 42))
  })

  it('reports an empty banlist', async () => {
    listBans.mockResolvedValue({ data: [] })
    renderWithProviders(<BansTab community={community} canModerate />)

    expect(await screen.findByTestId('bans-empty')).toBeInTheDocument()
  })

  // Community staff cannot be banned: the service refuses BOTH the owner and
  // every moderator row with ErrCannotBanCommunityStaff. The picker must
  // therefore withhold both, or it offers a choice the server answers with a
  // 400 -- and the moderator has no way to tell which names are eligible.
  it('withholds the owner and every moderator from the ban picker', async () => {
    listBans.mockResolvedValue({ data: [] })
    listModerators.mockResolvedValue({
      data: [
        { id: 1, community_id: 1, user_id: 11, username: 'rook', granted_at: '2026-08-01T00:00:00Z' },
        { id: 2, community_id: 1, user_id: 12, username: 'jackdaw', granted_at: '2026-08-01T00:00:00Z' },
      ],
    })

    renderWithProviders(<BansTab community={community} canModerate />)

    await waitFor(() => {
      const excluded = pickerExcludes.mock.calls.at(-1)?.[0] as number[]
      // Owner is community.owner_user_id (7); moderators are 11 and 12.
      expect([...excluded].sort((a, b) => a - b)).toEqual([7, 11, 12])
    })
  })

  // The roster is a separate request, so it may still be in flight when the
  // form first paints. The owner comes from the community record already in
  // hand and must be withheld from the very first render regardless.
  it('withholds the owner even before the moderator roster resolves', () => {
    listBans.mockResolvedValue({ data: [] })
    listModerators.mockReturnValue(new Promise(() => {}))

    renderWithProviders(<BansTab community={community} canModerate />)

    expect(pickerExcludes.mock.calls.at(-1)?.[0]).toContain(7)
  })

  // The endpoint 403s for a non-moderator, so the request must not be made at
  // all -- firing it would seed the cache with a guaranteed error.
  it('renders nothing and fetches nothing for a non-moderator', () => {
    renderWithProviders(<BansTab community={community} canModerate={false} />)

    expect(screen.queryByTestId('ban-user-form')).not.toBeInTheDocument()
    expect(listBans).not.toHaveBeenCalled()
    expect(screen.getByText(/only this community's moderators/i)).toBeInTheDocument()
  })
})
