import { describe, it, expect, beforeEach } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { AllPrivateMessagesView } from '../AllPrivateMessagesView'
import { renderWithProviders } from '../../test-utils/render'
import { server } from '../../mocks/server'

const gameId = 1

// Characters are identified by ID; the UI shows names but filters by ID.
const ALPHA = { id: 101, name: 'Alpha' }
const BETA = { id: 102, name: 'Beta' }
const GAMMA = { id: 103, name: 'Gamma' }

// Simulate the participants endpoint:
// - No selection: Alpha, Beta, Gamma all appear (all have conversations)
// - Alpha selected: Alpha, Beta, Gamma (both co-appear with Alpha)
// - Beta selected: Alpha, Beta only (Gamma has no conversation with Beta)
function participantsHandler(selected: number[]) {
  if (selected.length === 0) return [ALPHA, BETA, GAMMA]
  if (selected.includes(BETA.id)) return [ALPHA, BETA]
  return [ALPHA, BETA, GAMMA]
}

beforeEach(() => {
  server.use(
    http.get('/api/v1/games/:gameId/private-messages/participants', ({ request }) => {
      const url = new URL(request.url)
      const selected = url.searchParams.getAll('selected[]').map(Number)
      return HttpResponse.json({ participants: participantsHandler(selected) })
    }),
    http.get('/api/v1/games/:gameId/private-messages/all', () => {
      return HttpResponse.json({ conversations: [], total: 0 })
    }),
    http.get('/api/v1/games/:gameId/characters', () => HttpResponse.json([])),
    http.get('/api/v1/games/:gameId/characters/controllable', () => HttpResponse.json([])),
    http.get('/api/v1/games/:gameId', () => HttpResponse.json({
      id: gameId, title: 'Test Game', state: 'in_progress',
      gm_user_id: 99, created_at: new Date().toISOString(), updated_at: new Date().toISOString(),
    })),
    http.get('/api/v1/games/:gameId/participants', () => HttpResponse.json([])),
    http.get('/api/v1/games/:gameId/current-phase', () => HttpResponse.json(null, { status: 404 })),
  )
})

const mockConversation = {
  conversation_id: 7,
  subject: 'Secret Plans',
  conversation_type: 'direct',
  participant_names: ['Alice', 'Bob'],
  participant_usernames: ['alice', 'bob'],
  last_message_at: '2025-01-15T10:00:00Z',
  message_count: 3,
  created_at: '2025-01-01T00:00:00Z',
}

describe('AllPrivateMessagesView - URL sync', () => {
  it('opens a conversation from audienceConversation URL param on mount', async () => {
    server.use(
      http.get('/api/v1/games/:gameId/private-messages/all', () => {
        return HttpResponse.json({ conversations: [mockConversation], total: 1 })
      }),
      http.get('/api/v1/games/:gameId/private-messages/conversations/:conversationId', () => {
        return HttpResponse.json({ messages: [] })
      })
    )

    renderWithProviders(
      <AllPrivateMessagesView gameId={gameId} />,
      { gameId, initialEntries: [`/?tab=audience&audienceConversation=7`] }
    )

    // Should show the message viewer (back button) not the conversation list
    await waitFor(() => {
      expect(screen.getAllByRole('button', { name: /back/i })[0]).toBeInTheDocument()
    })
  })

  it('shows conversation list when no audienceConversation param is present', async () => {
    renderWithProviders(<AllPrivateMessagesView gameId={gameId} />, { gameId })

    await waitFor(() => {
      expect(screen.getAllByText(/all private messages/i)[0]).toBeInTheDocument()
      expect(screen.queryByRole('button', { name: /back/i })).not.toBeInTheDocument()
    })
  })

  it('selecting a conversation card sets the audienceConversation param', async () => {
    const user = userEvent.setup()
    server.use(
      http.get('/api/v1/games/:gameId/private-messages/all', () => {
        return HttpResponse.json({ conversations: [mockConversation], total: 1 })
      }),
      http.get('/api/v1/games/:gameId/private-messages/conversations/:conversationId', () => {
        return HttpResponse.json({ messages: [] })
      })
    )

    renderWithProviders(<AllPrivateMessagesView gameId={gameId} />, { gameId })

    await waitFor(() => {
      expect(screen.getAllByText('Secret Plans')[0]).toBeInTheDocument()
    })

    await user.click(screen.getAllByText('Secret Plans')[0])

    await waitFor(() => {
      expect(screen.getAllByRole('button', { name: /back/i })[0]).toBeInTheDocument()
    })
  })

  it('clicking back returns to conversation list and clears the param', async () => {
    const user = userEvent.setup()
    server.use(
      http.get('/api/v1/games/:gameId/private-messages/all', () => {
        return HttpResponse.json({ conversations: [mockConversation], total: 1 })
      }),
      http.get('/api/v1/games/:gameId/private-messages/conversations/:conversationId', () => {
        return HttpResponse.json({ messages: [] })
      })
    )

    renderWithProviders(
      <AllPrivateMessagesView gameId={gameId} />,
      { gameId, initialEntries: [`/?tab=audience&audienceConversation=7`] }
    )

    await waitFor(() => {
      expect(screen.getAllByRole('button', { name: /back/i })[0]).toBeInTheDocument()
    })

    await user.click(screen.getAllByRole('button', { name: /back/i })[0])

    await waitFor(() => {
      // After going back, the back button disappears (we're on the list view)
      expect(screen.queryByRole('button', { name: /back/i })).not.toBeInTheDocument()
      // The list header appears (multiple renders due to responsive layout)
      expect(screen.getAllByText(/all private messages/i)[0]).toBeInTheDocument()
    })
  })
})

describe('AllPrivateMessagesView - conversation count display', () => {
  it('shows total count from API, not the count of loaded conversations', async () => {
    // Page returns 2 conversations but total is 50 (more pages exist)
    server.use(
      http.get('/api/v1/games/:gameId/private-messages/all', () => {
        return HttpResponse.json({
          conversations: [
            { ...mockConversation, conversation_id: 1, subject: 'Conv 1' },
            { ...mockConversation, conversation_id: 2, subject: 'Conv 2' },
          ],
          total: 50,
        })
      })
    )

    renderWithProviders(<AllPrivateMessagesView gameId={gameId} />, { gameId })

    await waitFor(() => {
      // Should show 50 (total), not 2 (loaded page size)
      expect(screen.getAllByText('50 conversations')[0]).toBeInTheDocument()
    })
  })

  it('shows the count with correct singular form when total is 1', async () => {
    server.use(
      http.get('/api/v1/games/:gameId/private-messages/all', () => {
        return HttpResponse.json({
          conversations: [mockConversation],
          total: 1,
        })
      })
    )

    renderWithProviders(<AllPrivateMessagesView gameId={gameId} />, { gameId })

    await waitFor(() => {
      expect(screen.getAllByText('1 conversation')[0]).toBeInTheDocument()
    })
  })
})

describe('AllPrivateMessagesView - participant filter', () => {
  it('shows all conversation participants as filter options on initial load', async () => {
    renderWithProviders(<AllPrivateMessagesView gameId={gameId} />, { gameId })

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Alpha' })).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Beta' })).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Gamma' })).toBeInTheDocument()
    })
  })

  it('narrows filter options to co-participants when a name is selected', async () => {
    const user = userEvent.setup()
    renderWithProviders(<AllPrivateMessagesView gameId={gameId} />, { gameId })

    // Wait for initial filter list
    await waitFor(() => expect(screen.getByRole('button', { name: 'Beta' })).toBeInTheDocument())

    // Select Beta — Gamma should disappear (no shared conversation with Beta)
    await user.click(screen.getByRole('button', { name: 'Beta' }))

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Alpha' })).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Beta' })).toBeInTheDocument()
      expect(screen.queryByRole('button', { name: 'Gamma' })).not.toBeInTheDocument()
    })
  })
})

describe('AllPrivateMessagesView - filters by character ID, not name', () => {
  // Regression: the filter round-tripped display NAMES through the query string, so
  // renaming a character (or two characters sharing a name) silently changed which
  // conversations matched. Selection now round-trips character IDs.
  it('sends character IDs, so a renamed character still matches its conversations', async () => {
    const conversationRequests: URL[] = []

    server.use(
      http.get('/api/v1/games/:gameId/private-messages/participants', () =>
        // The character was renamed since these conversations were created.
        HttpResponse.json({ participants: [{ id: 101, name: 'Cassandra the Renamed' }] })
      ),
      http.get('/api/v1/games/:gameId/private-messages/all', ({ request }) => {
        const url = new URL(request.url)
        conversationRequests.push(url)
        // Backend matches on ID regardless of the character's current name.
        const ids = url.searchParams.getAll('participant_ids')
        const matched = ids.length === 0 || ids.includes('101')
        return HttpResponse.json({
          conversations: matched
            ? [{ ...mockConversation, conversation_id: 55, subject: 'Found by ID' }]
            : [],
          total: matched ? 1 : 0,
        })
      }),
    )

    renderWithProviders(<AllPrivateMessagesView gameId={gameId} />, { gameId })

    const chip = await screen.findByRole('button', { name: 'Cassandra the Renamed' })
    await userEvent.click(chip)

    // The filtered request must carry the ID, not the display name.
    await waitFor(() => {
      const last = conversationRequests[conversationRequests.length - 1]
      expect(last.searchParams.getAll('participant_ids')).toEqual(['101'])
      expect(last.searchParams.getAll('participant_names')).toEqual([])
    })

    // And the conversation is still found despite the rename.
    await waitFor(() => {
      expect(screen.getByText('Found by ID')).toBeInTheDocument()
    })
  })
})
