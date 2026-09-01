import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithProviders } from '../../test-utils'
import { CommunityManagePage } from '../CommunityManagePage'

const getCommunity = vi.fn()
const listModerators = vi.fn()

vi.mock('../../lib/api', () => ({
  apiClient: {
    communities: {
      getCommunity: (slug: string) => getCommunity(slug),
      listModerators: (slug: string) => listModerators(slug),
      addModerator: vi.fn(),
      removeModerator: vi.fn(),
    },
    auth: { searchUsers: vi.fn().mockResolvedValue({ data: { users: [] } }) },
  },
}))

let routeParams: { slug?: string; tab?: string } = { slug: 'midnight-ravens', tab: 'moderators' }
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return { ...actual, useParams: () => routeParams }
})

// Standing comes from the server as your_role -- which already accounts for
// moderator rows and for admin mode -- so these tests vary the payload rather
// than the auth context.

const community = {
  id: 1,
  name: 'Midnight Ravens',
  slug: 'midnight-ravens',
  description: null,
  banner_url: null,
  owner_user_id: 7,
  owner_username: 'corvid',
  is_active: true,
  your_role: '' as const,
  created_at: '2026-08-01T00:00:00Z',
  updated_at: '2026-08-01T00:00:00Z',
}

/** The same community as seen by a viewer with the given standing. */
const asRole = (your_role: '' | 'moderator' | 'owner') => ({
  data: { ...community, your_role },
})

const moderator = {
  id: 11,
  community_id: 1,
  user_id: 22,
  username: 'rook',
  granted_at: '2026-08-02T00:00:00Z',
}

beforeEach(() => {
  vi.clearAllMocks()
  routeParams = { slug: 'midnight-ravens', tab: 'moderators' }
  getCommunity.mockResolvedValue(asRole('owner'))
  listModerators.mockResolvedValue({ data: [moderator] })
})

describe('CommunityManagePage', () => {
  it('renders the moderators tab for the owner', async () => {
    renderWithProviders(<CommunityManagePage />)

    expect(await screen.findByText('Manage Midnight Ravens')).toBeInTheDocument()
    expect(await screen.findByTestId('moderator-user-search')).toBeInTheDocument()
  })

  // A moderator legitimately reaches this page -- later phases give them bans
  // and documents here. What they must not get is roster control.
  it('lets a moderator in but withholds the roster controls', async () => {
    getCommunity.mockResolvedValue(asRole('moderator'))
    renderWithProviders(<CommunityManagePage />)

    expect(await screen.findByText('Manage Midnight Ravens')).toBeInTheDocument()
    expect(await screen.findByText('rook')).toBeInTheDocument()

    expect(screen.queryByTestId('moderator-user-search')).not.toBeInTheDocument()
    expect(screen.queryByTestId(`remove-moderator-${moderator.user_id}`)).not.toBeInTheDocument()
  })

  // An admin WITH admin mode is reported as 'owner' by the server, so they get
  // roster control; without it the server reports '' and they do not. Both are
  // server-side decisions now, which is why they read as ordinary role cases.
  it('grants roster control when the server reports owner standing', async () => {
    renderWithProviders(<CommunityManagePage />)

    expect(await screen.findByTestId('moderator-user-search')).toBeInTheDocument()
  })

  it('withholds roster control when the server reports no standing', async () => {
    getCommunity.mockResolvedValue(asRole(''))
    renderWithProviders(<CommunityManagePage />)

    expect(await screen.findByText('rook')).toBeInTheDocument()
    expect(screen.queryByTestId('moderator-user-search')).not.toBeInTheDocument()
  })

  // An unrecognised tab falls back rather than rendering nothing, so a stale or
  // hand-edited URL still lands somewhere useful.
  it('falls back to the moderators tab for an unknown tab', async () => {
    routeParams = { slug: 'midnight-ravens', tab: 'not-a-tab' }
    renderWithProviders(<CommunityManagePage />)

    expect(await screen.findByTestId('moderator-user-search')).toBeInTheDocument()
  })

  // Profile editing is moderator-level, unlike the roster: a moderator gets the
  // real form, not the read-only notice.
  it('lets a moderator edit the profile on the settings tab', async () => {
    routeParams = { slug: 'midnight-ravens', tab: 'settings' }
    getCommunity.mockResolvedValue(asRole('moderator'))
    renderWithProviders(<CommunityManagePage />)

    expect(await screen.findByTestId('community-settings-form')).toBeInTheDocument()
  })

  it('shows the settings tab read-only to a viewer with no standing', async () => {
    routeParams = { slug: 'midnight-ravens', tab: 'settings' }
    getCommunity.mockResolvedValue(asRole(''))
    renderWithProviders(<CommunityManagePage />)

    expect(await screen.findByText(/only this community's moderators/i)).toBeInTheDocument()
    expect(screen.queryByTestId('community-settings-form')).not.toBeInTheDocument()
  })

  it('reports a missing community', async () => {
    getCommunity.mockRejectedValue(new Error('404'))
    renderWithProviders(<CommunityManagePage />)

    expect(await screen.findByText('Community not found')).toBeInTheDocument()
  })
})
