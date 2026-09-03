import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import { renderWithProviders } from '../../test-utils'
import { GameCommunitySection } from '../GameCommunitySection'
import type { CommunityDocument } from '../../types/communities'

const listGameCommunityDocuments = vi.fn()

vi.mock('../../lib/api', () => ({
  apiClient: {
    communities: {
      listGameCommunityDocuments: (gameId: number) => listGameCommunityDocuments(gameId),
    },
  },
}))

const doc = (id: number, title: string): CommunityDocument => ({
  id,
  community_id: 1,
  title,
  content: 'body',
  status: 'published',
  sort_order: id,
  created_at: '2026-08-01T00:00:00Z',
  updated_at: '2026-08-01T00:00:00Z',
})

const RAVENS = { communityName: 'Midnight Ravens', communitySlug: 'midnight-ravens' }

beforeEach(() => {
  vi.clearAllMocks()
  listGameCommunityDocuments.mockResolvedValue({ data: [doc(20, 'House rules')] })
})

describe('GameCommunitySection', () => {
  it('names the community and lists its documents', async () => {
    renderWithProviders(<GameCommunitySection gameId={5} {...RAVENS} />)

    expect(screen.getByTestId('game-community-name')).toHaveTextContent('Midnight Ravens')
    expect(await screen.findByTestId('game-community-document-20')).toHaveTextContent(
      'House rules'
    )
  })

  // The bug this component shipped with: identity was read off the document
  // list, so a community that had published nothing showed no community at all.
  // Naming the community is not conditional on it having written anything.
  it('still names the community when it has published no documents', async () => {
    listGameCommunityDocuments.mockResolvedValue({ data: [] })
    renderWithProviders(<GameCommunitySection gameId={5} {...RAVENS} />)

    await waitFor(() => expect(listGameCommunityDocuments).toHaveBeenCalled())
    expect(screen.getByTestId('game-community-section')).toBeInTheDocument()
    expect(screen.getByTestId('game-community-name')).toHaveTextContent('Midnight Ravens')
  })

  it('links the community name to its page', () => {
    renderWithProviders(<GameCommunitySection gameId={5} {...RAVENS} />)

    expect(screen.getByTestId('game-community-name')).toHaveAttribute(
      'href',
      '/communities/midnight-ravens'
    )
  })

  // Titles LINK rather than embedding markdown: embedding would duplicate the
  // same rules across every game and make the tab unbounded in length.
  it('links document titles rather than embedding their content', async () => {
    renderWithProviders(<GameCommunitySection gameId={5} {...RAVENS} />)

    const link = await screen.findByTestId('game-community-document-20')
    expect(link).toHaveAttribute('href', '/communities/midnight-ravens')
    expect(screen.queryByText('body')).not.toBeInTheDocument()
  })

  // req 5 grandfathering: a game predating communities has no community, so
  // there is no identity to show and no placeholder worth inventing.
  it('renders nothing for a game with no community', () => {
    const { container } = renderWithProviders(<GameCommunitySection gameId={5} />)

    expect(screen.queryByTestId('game-community-section')).not.toBeInTheDocument()
    expect(container).toBeEmptyDOMElement()
  })

  // No point asking for documents belonging to a community that isn't there.
  it('does not fetch documents for a game with no community', async () => {
    renderWithProviders(<GameCommunitySection gameId={5} />)

    await waitFor(() => expect(listGameCommunityDocuments).not.toHaveBeenCalled())
  })

  // Documents are supplementary: the section stands on the name alone, and an
  // error banner would be noise on a tab whose primary content loaded fine.
  it('keeps the community name when the document request fails', async () => {
    listGameCommunityDocuments.mockRejectedValue(new Error('boom'))
    renderWithProviders(<GameCommunitySection gameId={5} {...RAVENS} />)

    await waitFor(() => expect(listGameCommunityDocuments).toHaveBeenCalled())
    expect(screen.getByTestId('game-community-name')).toHaveTextContent('Midnight Ravens')
    expect(screen.queryByTestId('game-community-document-20')).not.toBeInTheDocument()
  })
})
