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

// The page no longer derives the viewer's standing -- the server reports it as
// your_role, which already folds in moderator rows AND admin mode. So these
// tests vary the payload rather than the auth context.
const community = {
  id: 1,
  name: 'Midnight Ravens',
  slug: 'midnight-ravens',
  description: 'Rules live **here**',
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

beforeEach(() => {
  vi.clearAllMocks()
  getCommunity.mockResolvedValue(asRole(''))
})

describe('CommunityPage', () => {
  it('renders the profile with its owner', async () => {
    renderWithProviders(<CommunityPage />)

    expect(await screen.findByText('Midnight Ravens')).toBeInTheDocument()
    expect(screen.getByText('corvid')).toBeInTheDocument()
  })

  it('offers Manage to the owner', async () => {
    getCommunity.mockResolvedValue(asRole('owner'))
    renderWithProviders(<CommunityPage />)

    const manage = await screen.findByTestId('manage-community')
    expect(manage.closest('a')).toHaveAttribute(
      'href',
      '/communities/midnight-ravens/manage/moderators'
    )
  })

  // Previously impossible to express: the page compared against owner_user_id,
  // so a moderator was indistinguishable from a visitor.
  it('offers Manage to a moderator', async () => {
    getCommunity.mockResolvedValue(asRole('moderator'))
    renderWithProviders(<CommunityPage />)

    expect(await screen.findByTestId('manage-community')).toBeInTheDocument()
  })

  it('does not offer Manage to an ordinary visitor', async () => {
    renderWithProviders(<CommunityPage />)

    expect(await screen.findByText('Midnight Ravens')).toBeInTheDocument()
    expect(screen.queryByTestId('manage-community')).not.toBeInTheDocument()
  })

  // Admin mode is the same convention as GM override: an admin browsing
  // normally is an ordinary visitor, so they do not get moderation affordances
  // by accident. The SERVER enforces that -- it reports '' for an admin without
  // admin mode -- which is why this case now looks like any other visitor here.
  it('does not offer Manage when the server reports no standing', async () => {
    renderWithProviders(<CommunityPage />)

    expect(await screen.findByText('Midnight Ravens')).toBeInTheDocument()
    expect(screen.queryByTestId('manage-community')).not.toBeInTheDocument()
  })

  it('reports a missing community rather than rendering a blank profile', async () => {
    getCommunity.mockRejectedValue(new Error('404'))
    renderWithProviders(<CommunityPage />)

    expect(await screen.findByText('Community not found')).toBeInTheDocument()
  })
})
