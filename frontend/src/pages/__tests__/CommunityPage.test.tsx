import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithProviders } from '../../test-utils'
import { CommunityPage } from '../CommunityPage'

const getCommunity = vi.fn()

vi.mock('../../lib/api', () => ({
  apiClient: {
    communities: {
      getCommunity: (slug: string) => getCommunity(slug),
    },
  },
}))

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return { ...actual, useParams: () => ({ slug: 'midnight-ravens' }) }
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
  description: 'Rules live **here**',
  banner_url: null,
  owner_user_id: 7,
  owner_username: 'corvid',
  is_active: true,
  created_at: '2026-08-01T00:00:00Z',
  updated_at: '2026-08-01T00:00:00Z',
}

beforeEach(() => {
  vi.clearAllMocks()
  currentUser = null
  adminModeEnabled = false
  getCommunity.mockResolvedValue({ data: community })
})

describe('CommunityPage', () => {
  it('renders the profile with its owner', async () => {
    currentUser = { id: 99, username: 'visitor' }
    renderWithProviders(<CommunityPage />)

    expect(await screen.findByText('Midnight Ravens')).toBeInTheDocument()
    expect(screen.getByText('corvid')).toBeInTheDocument()
  })

  it('offers Manage to the owner', async () => {
    currentUser = { id: 7, username: 'corvid' }
    renderWithProviders(<CommunityPage />)

    const manage = await screen.findByTestId('manage-community')
    expect(manage.closest('a')).toHaveAttribute(
      'href',
      '/communities/midnight-ravens/manage/moderators'
    )
  })

  it('does not offer Manage to an ordinary visitor', async () => {
    currentUser = { id: 99, username: 'visitor' }
    renderWithProviders(<CommunityPage />)

    expect(await screen.findByText('Midnight Ravens')).toBeInTheDocument()
    expect(screen.queryByTestId('manage-community')).not.toBeInTheDocument()
  })

  // Admin mode is the same convention as GM override: an admin browsing
  // normally is an ordinary visitor, so they do not get moderation affordances
  // by accident.
  it('does not offer Manage to an admin who has not enabled admin mode', async () => {
    currentUser = { id: 99, username: 'root', is_admin: true }
    adminModeEnabled = false
    renderWithProviders(<CommunityPage />)

    expect(await screen.findByText('Midnight Ravens')).toBeInTheDocument()
    expect(screen.queryByTestId('manage-community')).not.toBeInTheDocument()
  })

  it('offers Manage to an admin with admin mode enabled', async () => {
    currentUser = { id: 99, username: 'root', is_admin: true }
    adminModeEnabled = true
    renderWithProviders(<CommunityPage />)

    expect(await screen.findByTestId('manage-community')).toBeInTheDocument()
  })

  it('reports a missing community rather than rendering a blank profile', async () => {
    getCommunity.mockRejectedValue(new Error('404'))
    renderWithProviders(<CommunityPage />)

    expect(await screen.findByText('Community not found')).toBeInTheDocument()
  })
})
