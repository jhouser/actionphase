import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithProviders } from '../../test-utils'
import { CommunitiesPage } from '../CommunitiesPage'

const listActiveCommunities = vi.fn()

vi.mock('../../lib/api', () => ({
  apiClient: {
    communities: {
      listActiveCommunities: () => listActiveCommunities(),
    },
  },
}))

const community = {
  id: 1,
  name: 'Midnight Ravens',
  slug: 'midnight-ravens',
  description: 'Rules live here',
  banner_url: null,
  owner_user_id: 7,
  owner_username: 'corvid',
  is_active: true,
  created_at: '2026-08-01T00:00:00Z',
  updated_at: '2026-08-01T00:00:00Z',
}

beforeEach(() => {
  vi.clearAllMocks()
  listActiveCommunities.mockResolvedValue({ data: [community] })
})

describe('CommunitiesPage', () => {
  it('lists communities and links to each profile', async () => {
    renderWithProviders(<CommunitiesPage />)

    expect(await screen.findByText('Midnight Ravens')).toBeInTheDocument()
    expect(screen.getByText('Rules live here')).toBeInTheDocument()
    expect(screen.getByTestId('community-card-midnight-ravens')).toHaveAttribute(
      'href',
      '/communities/midnight-ravens'
    )
  })

  it('shows an empty state when there are no communities', async () => {
    listActiveCommunities.mockResolvedValue({ data: [] })
    renderWithProviders(<CommunitiesPage />)

    expect(await screen.findByText('No communities yet')).toBeInTheDocument()
  })

  it('reports a failed load instead of rendering an empty list', async () => {
    listActiveCommunities.mockRejectedValue(new Error('boom'))
    renderWithProviders(<CommunitiesPage />)

    expect(await screen.findByText('Could not load communities')).toBeInTheDocument()
    // An error must not be mistaken for "there are none" -- they call for
    // different actions from the reader.
    expect(screen.queryByText('No communities yet')).not.toBeInTheDocument()
  })
})
