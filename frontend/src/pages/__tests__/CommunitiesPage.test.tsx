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

  // Descriptions are markdown. A plain <p> printed the raw syntax, so a blurb
  // written as "**dusk**" showed the asterisks to every browsing user.
  it('renders the description as markdown rather than raw syntax', async () => {
    listActiveCommunities.mockResolvedValue({
      data: [{ ...community, description: 'We fly at **dusk**' }],
    })
    renderWithProviders(<CommunitiesPage />)

    const strong = await screen.findByText('dusk')
    expect(strong.tagName).toBe('STRONG')
    expect(screen.queryByText(/\*\*dusk\*\*/)).not.toBeInTheDocument()
  })

  // The card link is an overlay SIBLING, not a wrapper. Nesting the card's <a>
  // around the description put markdown links inside an anchor -- invalid HTML
  // that browsers unnest, so a link in the blurb hijacked the card click.
  it('does not nest a description link inside the card link', async () => {
    listActiveCommunities.mockResolvedValue({
      data: [{ ...community, description: 'See [our charter](https://example.com)' }],
    })
    renderWithProviders(<CommunitiesPage />)

    const card = await screen.findByTestId('community-card-midnight-ravens')
    const charter = screen.getByRole('link', { name: 'our charter' })

    expect(charter).toHaveAttribute('href', 'https://example.com')
    expect(card.contains(charter)).toBe(false)
  })

  // prose styles `a strong` as link-coloured. When the card wrapped the
  // description, every bold word inherited that and looked clickable.
  it('does not render bold text inside the card link', async () => {
    listActiveCommunities.mockResolvedValue({
      data: [{ ...community, description: 'We fly at **dusk**' }],
    })
    renderWithProviders(<CommunitiesPage />)

    const card = await screen.findByTestId('community-card-midnight-ravens')
    const strong = screen.getByText('dusk')

    expect(strong.tagName).toBe('STRONG')
    expect(card.contains(strong)).toBe(false)
  })

  // The overlay anchor has no text of its own, so it needs a label or it
  // reaches screen readers as an unnamed link.
  it('gives the overlay card link an accessible name', async () => {
    renderWithProviders(<CommunitiesPage />)

    expect(await screen.findByRole('link', { name: 'Midnight Ravens' })).toHaveAttribute(
      'href',
      '/communities/midnight-ravens'
    )
  })

  // The card is a link, so the expand toggle is suppressed -- a button inside
  // an anchor would swallow the click and nest interactive elements.
  it('does not put an expand toggle inside the card link', async () => {
    listActiveCommunities.mockResolvedValue({
      data: [{ ...community, description: 'line\n\n'.repeat(40) + 'end' }],
    })
    renderWithProviders(<CommunitiesPage />)

    await screen.findByTestId('community-card-midnight-ravens')
    expect(screen.queryByRole('button', { name: /show full content/i })).not.toBeInTheDocument()
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
