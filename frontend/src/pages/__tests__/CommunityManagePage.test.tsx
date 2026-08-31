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

let currentUser: { id: number; username: string; is_admin?: boolean } | null = null
vi.mock('../../contexts/AuthContext', async () => {
  const actual = await vi.importActual('../../contexts/AuthContext')
  return { ...actual, useAuth: () => ({ currentUser, isCheckingAuth: false }) }
})

let adminModeEnabled = false
vi.mock('../../contexts/AdminModeContext', async () => {
  const actual = await vi.importActual('../../contexts/AdminModeContext')
  return { ...actual, useAdminMode: () => ({ adminModeEnabled, setAdminModeEnabled: vi.fn() }) }
})

const community = {
  id: 1,
  name: 'Midnight Ravens',
  slug: 'midnight-ravens',
  description: null,
  banner_url: null,
  owner_user_id: 7,
  owner_username: 'corvid',
  is_active: true,
  created_at: '2026-08-01T00:00:00Z',
  updated_at: '2026-08-01T00:00:00Z',
}

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
  currentUser = null
  adminModeEnabled = false
  getCommunity.mockResolvedValue({ data: community })
  listModerators.mockResolvedValue({ data: [moderator] })
})

describe('CommunityManagePage', () => {
  it('renders the moderators tab for the owner', async () => {
    currentUser = { id: 7, username: 'corvid' }
    renderWithProviders(<CommunityManagePage />)

    expect(await screen.findByText('Manage Midnight Ravens')).toBeInTheDocument()
    expect(await screen.findByTestId('moderator-user-search')).toBeInTheDocument()
  })

  // A moderator legitimately reaches this page -- later phases give them bans
  // and documents here. What they must not get is roster control.
  it('lets a moderator in but withholds the roster controls', async () => {
    currentUser = { id: 22, username: 'rook' }
    renderWithProviders(<CommunityManagePage />)

    expect(await screen.findByText('Manage Midnight Ravens')).toBeInTheDocument()
    expect(await screen.findByText('rook')).toBeInTheDocument()

    expect(screen.queryByTestId('moderator-user-search')).not.toBeInTheDocument()
    expect(screen.queryByTestId(`remove-moderator-${moderator.user_id}`)).not.toBeInTheDocument()
  })

  it('grants roster control to an admin only with admin mode enabled', async () => {
    currentUser = { id: 99, username: 'root', is_admin: true }
    adminModeEnabled = true
    renderWithProviders(<CommunityManagePage />)

    expect(await screen.findByTestId('moderator-user-search')).toBeInTheDocument()
  })

  it('withholds roster control from an admin browsing normally', async () => {
    currentUser = { id: 99, username: 'root', is_admin: true }
    adminModeEnabled = false
    renderWithProviders(<CommunityManagePage />)

    expect(await screen.findByText('rook')).toBeInTheDocument()
    expect(screen.queryByTestId('moderator-user-search')).not.toBeInTheDocument()
  })

  // An unrecognised tab falls back rather than rendering nothing, so a stale or
  // hand-edited URL still lands somewhere useful.
  it('falls back to the moderators tab for an unknown tab', async () => {
    currentUser = { id: 7, username: 'corvid' }
    routeParams = { slug: 'midnight-ravens', tab: 'not-a-tab' }
    renderWithProviders(<CommunityManagePage />)

    expect(await screen.findByTestId('moderator-user-search')).toBeInTheDocument()
  })

  it('reports a missing community', async () => {
    currentUser = { id: 7, username: 'corvid' }
    getCommunity.mockRejectedValue(new Error('404'))
    renderWithProviders(<CommunityManagePage />)

    expect(await screen.findByText('Community not found')).toBeInTheDocument()
  })
})
