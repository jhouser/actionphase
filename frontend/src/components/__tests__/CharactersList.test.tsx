import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen, fireEvent, waitFor } from '@testing-library/react'
import { renderWithProviders } from '../../test-utils'
import { CharactersList } from '../CharactersList'
import { http, HttpResponse } from 'msw'
import { server } from '../../mocks/server'
import type { Character } from '../../types/characters'

describe('CharactersList', () => {
  const mockCharacters: Character[] = [
    {
      id: 1,
      name: 'Hero Character',
      game_id: 123,
      user_id: 1,
      username: 'player1',
      character_type: 'player_character',
      status: 'approved',
      attributes: {},
      inventory: [],
      notes: '',
      created_at: '2025-01-01T00:00:00Z',
      updated_at: '2025-01-01T00:00:00Z',
    },
    {
      id: 2,
      name: 'Pending Character',
      game_id: 123,
      user_id: 2,
      username: 'player2',
      character_type: 'player_character',
      status: 'pending',
      attributes: {},
      inventory: [],
      notes: '',
      created_at: '2025-01-01T00:00:00Z',
      updated_at: '2025-01-01T00:00:00Z',
    },
    {
      id: 3,
      name: 'Villain NPC',
      game_id: 123,
      user_id: 1,
      username: 'gm',
      character_type: 'npc',
      status: 'approved',
      attributes: {},
      inventory: [],
      notes: '',
      created_at: '2025-01-01T00:00:00Z',
      updated_at: '2025-01-01T00:00:00Z',
    },
    {
      id: 4,
      name: 'Other Hero',
      game_id: 123,
      user_id: 2,
      username: 'player2',
      character_type: 'player_character',
      status: 'approved',
      attributes: {},
      inventory: [],
      notes: '',
      created_at: '2025-01-01T00:00:00Z',
      updated_at: '2025-01-01T00:00:00Z',
    },
  ]

  beforeEach(() => {
    vi.clearAllMocks()

    // Default API response - apiClient expects res.data to be the array directly
    server.use(
      http.get('http://localhost:3000/api/v1/games/:gameId/characters', () => {
        return HttpResponse.json(mockCharacters)
      }),
      // Mock controllable characters endpoint for ownership detection
      http.get('http://localhost:3000/api/v1/games/:gameId/characters/controllable', () => {
        // Return characters owned by user_id 1 (default test user)
        return HttpResponse.json(mockCharacters.filter(c => c.user_id === 1))
      }),
      // Mock game participants endpoint (used by GM for character assignment)
      http.get('http://localhost:3000/api/v1/games/:gameId/participants', () => {
        return HttpResponse.json([
          { id: 1, user_id: 1, username: 'player1', role: 'player', status: 'active', joined_at: '2025-01-01T00:00:00Z' },
          { id: 2, user_id: 2, username: 'player2', role: 'player', status: 'active', joined_at: '2025-01-01T00:00:00Z' },
        ])
      })
    )
  })

  describe('Loading and empty states', () => {
    it('should show empty state when no characters exist', async () => {
      server.use(
        http.get('http://localhost:3000/api/v1/games/:gameId/characters', () => {
          return HttpResponse.json([])
        }),
        http.get('http://localhost:3000/api/v1/games/:gameId/characters/controllable', () => {
          return HttpResponse.json([])
        })
      )

      renderWithProviders(
        <CharactersList gameId={123} userRole="player" currentUserId={1} gameState="in_progress" isParticipant={true} />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getByText('No characters created yet.')).toBeInTheDocument()
      })

      expect(screen.getByText(/Click "Create Character" to get started/)).toBeInTheDocument()
    })
  })

  describe('Character rendering', () => {
    it('should render character list when data is loaded', async () => {
      renderWithProviders(
        <CharactersList gameId={123} userRole="gm" currentUserId={1} />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getAllByText('Hero Character')[0]).toBeInTheDocument()
      })

      expect(screen.getAllByText('Pending Character')[0]).toBeInTheDocument()
      expect(screen.getAllByText('Villain NPC')[0]).toBeInTheDocument()
    })

    it('should group characters by type (Player Characters, NPCs)', async () => {
      renderWithProviders(
        <CharactersList gameId={123} userRole="gm" currentUserId={1} />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getByText('Player Characters')).toBeInTheDocument()
      })

      expect(screen.getByText('NPCs')).toBeInTheDocument()
    })

    it('should display character status badges for pending characters', async () => {
      renderWithProviders(
        <CharactersList gameId={123} userRole="gm" currentUserId={1} />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getAllByText('pending')[0]).toBeInTheDocument()
      })

      // Approved badge should NOT be shown
      expect(screen.queryByText('approved')).not.toBeInTheDocument()
    })

    it('should show ownership badge for user characters', async () => {
      renderWithProviders(
        <CharactersList gameId={123} userRole="player" currentUserId={1} />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getAllByText('Your Character').length).toBeGreaterThan(0)
      })
    })
  })

  describe('Role-based visibility (GM)', () => {
    it('GM should see all characters regardless of status', async () => {
      renderWithProviders(
        <CharactersList gameId={123} userRole="gm" currentUserId={1} />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getAllByText('Hero Character')[0]).toBeInTheDocument()
      })

      expect(screen.getAllByText('Pending Character')[0]).toBeInTheDocument()
      expect(screen.getAllByText('Villain NPC')[0]).toBeInTheDocument()
    })

    it('GM should see publish button for pending characters', async () => {
      renderWithProviders(
        <CharactersList gameId={123} userRole="gm" currentUserId={1} />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getAllByText('Publish')[0]).toBeInTheDocument()
      })
    })

    it('GM should NOT see publish button for approved characters', async () => {
      server.use(
        http.get('http://localhost:3000/api/v1/games/:gameId/characters', () => {
          return HttpResponse.json([mockCharacters[0]]) // Only approved character
        })
      )

      renderWithProviders(
        <CharactersList gameId={123} userRole="gm" currentUserId={1} />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getAllByText('Hero Character')[0]).toBeInTheDocument()
      })

      expect(screen.queryByText('Publish')).not.toBeInTheDocument()
    })
  })

  describe('Role-based visibility (Player)', () => {
    it('Player should see approved characters and their own characters', async () => {
      renderWithProviders(
        <CharactersList gameId={123} userRole="player" currentUserId={1} />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getAllByText('Hero Character')[0]).toBeInTheDocument()
      })

      // Should NOT see other players' pending characters
      expect(screen.queryByText('Pending Character')).not.toBeInTheDocument()

      // Should see GM NPC (approved)
      expect(screen.getAllByText('Villain NPC')[0]).toBeInTheDocument()
    })

    it('Player should NOT see publish button', async () => {
      renderWithProviders(
        <CharactersList gameId={123} userRole="player" currentUserId={1} />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getAllByText('Hero Character')[0]).toBeInTheDocument()
      })

      expect(screen.queryByText('Publish')).not.toBeInTheDocument()
    })

    it('Player should see their own pending character during character_creation', async () => {
      // Test for Issue 4.1: Player should be able to see their own pending character
      // Override controllable endpoint for user 2
      server.use(
        http.get('http://localhost:3000/api/v1/games/:gameId/characters/controllable', () => {
          // Return pending character owned by user_id 2
          return HttpResponse.json(mockCharacters.filter(c => c.user_id === 2))
        })
      )

      renderWithProviders(
        <CharactersList gameId={123} userRole="player" currentUserId={2} gameState="character_creation" />,
      { gameId: 123 })

      await waitFor(() => {
        // Should see their own pending character (user_id=2, status='pending')
        expect(screen.getAllByText('Pending Character')[0]).toBeInTheDocument()
      })

      // Should also see approved characters
      expect(screen.getAllByText('Hero Character')[0]).toBeInTheDocument()
    })

    it('Player should NOT see their own pending character during in_progress', async () => {
      // During in_progress, pending characters are hidden even from their owners
      renderWithProviders(
        <CharactersList gameId={123} userRole="player" currentUserId={2} gameState="in_progress" />,
      { gameId: 123 })

      await waitFor(() => {
        // Should see approved characters
        expect(screen.getAllByText('Hero Character')[0]).toBeInTheDocument()
      })

      // Should NOT see their own pending character during in_progress
      expect(screen.queryByText('Pending Character')).not.toBeInTheDocument()
    })
  })

  describe('Role-based visibility (Audience)', () => {
    const pendingAudienceNPC: Character = {
      id: 10,
      name: 'Audience NPC',
      game_id: 123,
      character_type: 'npc',
      status: 'pending',
      assigned_user_id: 5,
      assigned_username: 'audienceuser',
      attributes: {},
      inventory: [],
      notes: '',
      created_at: '2025-01-01T00:00:00Z',
      updated_at: '2025-01-01T00:00:00Z',
    }

    it('audience member with assigned pending NPC in in_progress game can edit the NPC sheet', async () => {
      server.use(
        http.get('http://localhost:3000/api/v1/games/:gameId/characters', () => {
          return HttpResponse.json([...mockCharacters, pendingAudienceNPC])
        }),
        http.get('http://localhost:3000/api/v1/games/:gameId/characters/controllable', () => {
          // Backend returns assigned pending NPCs to their assignee
          return HttpResponse.json([pendingAudienceNPC])
        })
      )

      renderWithProviders(
        <CharactersList gameId={123} userRole="audience" currentUserId={5} gameState="in_progress" />,
        { gameId: 123 }
      )

      // Wait for both the NPC to render AND the controllable-characters fetch to resolve
      // (which determines whether the audience member sees "Edit Sheet" vs "View Sheet")
      await waitFor(() => {
        const editButtons = screen.getAllByTestId('edit-character-button')
        const npcEditButton = editButtons.find(btn => btn.textContent === 'Edit Sheet')
        expect(npcEditButton).toBeTruthy()
      })
    })
  })

  describe('Create Character button', () => {
    it('should show create button for GM in setup state', async () => {
      renderWithProviders(
        <CharactersList gameId={123} userRole="gm" currentUserId={1} gameState="setup" />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getByRole('button', { name: 'Create Character' })).toBeInTheDocument()
      })
    })

    it('should show create button for player in character_creation state', async () => {
      renderWithProviders(
        <CharactersList gameId={123} userRole="player" currentUserId={1} gameState="character_creation" isParticipant={true} />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getByRole('button', { name: 'Create Character' })).toBeInTheDocument()
      })
    })

    it('should show create button for participant player in in_progress state', async () => {
      renderWithProviders(
        <CharactersList gameId={123} userRole="player" currentUserId={1} gameState="in_progress" isParticipant={true} />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getByRole('button', { name: 'Create Character' })).toBeInTheDocument()
      })
    })

    it('should NOT show create button for non-participant player in in_progress state', async () => {
      renderWithProviders(
        <CharactersList gameId={123} userRole="player" currentUserId={1} gameState="in_progress" isParticipant={false} />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getByText('Characters')).toBeInTheDocument()
      })

      expect(screen.queryByRole('button', { name: 'Create Character' })).not.toBeInTheDocument()
    })

    it('should NOT show create button for player in active state', async () => {
      renderWithProviders(
        <CharactersList gameId={123} userRole="player" currentUserId={1} gameState="active" />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getByText('Characters')).toBeInTheDocument()
      })

      expect(screen.queryByRole('button', { name: 'Create Character' })).not.toBeInTheDocument()
    })

    it('should NOT show create button in completed game', async () => {
      renderWithProviders(
        <CharactersList gameId={123} userRole="gm" currentUserId={1} gameState="completed" />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getByText('Characters')).toBeInTheDocument()
      })

      expect(screen.queryByRole('button', { name: 'Create Character' })).not.toBeInTheDocument()
    })
  })

  describe('Participant-only character creation (Issue #3 fix)', () => {
    it('should NOT show create button for non-participant player in character_creation', async () => {
      // Non-participant viewing the game during character creation
      renderWithProviders(
        <CharactersList
          gameId={123}
          userRole="player"
          currentUserId={1}
          gameState="character_creation"
          isParticipant={false}
        />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getByText('Characters')).toBeInTheDocument()
      })

      // Create button should NOT appear for non-participants
      expect(screen.queryByRole('button', { name: 'Create Character' })).not.toBeInTheDocument()
    })

    it('should NOT show create button for non-participant player in setup', async () => {
      // Non-participant viewing the game during setup
      renderWithProviders(
        <CharactersList
          gameId={123}
          userRole="player"
          currentUserId={1}
          gameState="setup"
          isParticipant={false}
        />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getByText('Characters')).toBeInTheDocument()
      })

      // Create button should NOT appear for non-participants
      expect(screen.queryByRole('button', { name: 'Create Character' })).not.toBeInTheDocument()
    })

    it('should show create button for participant player in character_creation', async () => {
      // Active participant in character creation phase
      renderWithProviders(
        <CharactersList
          gameId={123}
          userRole="player"
          currentUserId={1}
          gameState="character_creation"
          isParticipant={true}
        />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getByRole('button', { name: 'Create Character' })).toBeInTheDocument()
      })
    })

    it('should NOT show create button for participant player in setup (participants do not exist in setup)', async () => {
      // Participants are created on recruitment → character_creation transition,
      // so no player is ever a participant during setup.
      renderWithProviders(
        <CharactersList
          gameId={123}
          userRole="player"
          currentUserId={1}
          gameState="setup"
          isParticipant={true}
        />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getByText('Characters')).toBeInTheDocument()
      })

      expect(screen.queryByRole('button', { name: 'Create Character' })).not.toBeInTheDocument()
    })

    it('GM should see create button regardless of isParticipant prop', async () => {
      // GM with isParticipant=false should still see button
      renderWithProviders(
        <CharactersList
          gameId={123}
          userRole="gm"
          currentUserId={1}
          gameState="setup"
          isParticipant={false}
        />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getByRole('button', { name: 'Create Character' })).toBeInTheDocument()
      })
    })

    it('should NOT show create button for non-participant in active game', async () => {
      // Even participants can't create characters in active games
      renderWithProviders(
        <CharactersList
          gameId={123}
          userRole="player"
          currentUserId={1}
          gameState="active"
          isParticipant={true}
        />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getByText('Characters')).toBeInTheDocument()
      })

      expect(screen.queryByRole('button', { name: 'Create Character' })).not.toBeInTheDocument()
    })
  })

  describe('View/Edit Sheet permissions', () => {
    it('should show "Edit Sheet" button for user\'s own character', async () => {
      renderWithProviders(
        <CharactersList gameId={123} userRole="player" currentUserId={1} />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getAllByRole('button', { name: 'Edit Sheet' }).length).toBeGreaterThan(0)
      })
    })

    it('should show "View Sheet" button for other approved characters', async () => {
      // Override controllable endpoint for user 999 (doesn't own any characters)
      server.use(
        http.get('http://localhost:3000/api/v1/games/:gameId/characters/controllable', () => {
          return HttpResponse.json([])
        })
      )

      renderWithProviders(
        <CharactersList gameId={123} userRole="player" currentUserId={999} />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getAllByRole('button', { name: 'View Sheet' }).length).toBeGreaterThan(0)
      })
    })

    it('GM should be able to edit all character sheets', async () => {
      renderWithProviders(
        <CharactersList gameId={123} userRole="gm" currentUserId={999} />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getAllByRole('button', { name: 'Edit Sheet' }).length).toBeGreaterThan(0)
      })
    })

  })

  describe('Anonymous mode', () => {
    it('should show "Your Character" badge for owned characters even in anonymous mode', async () => {
      renderWithProviders(
        <CharactersList
          gameId={123}
          userRole="player"
          currentUserId={1}
          isAnonymous={true}
        />,
      { gameId: 123 })

      // Wait for both characters and ownership to load
      await waitFor(() => {
        expect(screen.getAllByText('Your Character').length).toBeGreaterThan(0)
      })
    })

    it('should hide character type in anonymous mode', async () => {
      renderWithProviders(
        <CharactersList
          gameId={123}
          userRole="player"
          currentUserId={1}
          isAnonymous={true}
        />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getAllByText('Hero Character')[0]).toBeInTheDocument()
      })

      expect(screen.queryByText(/Type:/)).not.toBeInTheDocument()
    })

    it('should NOT group by type in anonymous mode', async () => {
      renderWithProviders(
        <CharactersList
          gameId={123}
          userRole="player"
          currentUserId={1}
          isAnonymous={true}
        />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getAllByText('Hero Character')[0]).toBeInTheDocument()
      })

      expect(screen.queryByText('Player Characters')).not.toBeInTheDocument()
      expect(screen.queryByText('NPCs')).not.toBeInTheDocument()
    })

    it('GM should still see character details in anonymous mode', async () => {
      renderWithProviders(
        <CharactersList
          gameId={123}
          userRole="gm"
          currentUserId={1}
          isAnonymous={true}
        />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getAllByText('Your Character').length).toBeGreaterThan(0)
      })

      expect(screen.getAllByText(/Type:/).length).toBeGreaterThan(0)
    })
  })

  describe('Character actions', () => {
    it('should call approve mutation when publish button is clicked', async () => {
      let approvePayload: unknown = null

      server.use(
        http.post('http://localhost:3000/api/v1/characters/:id/approve', async ({ request }) => {
          approvePayload = await request.json()
          return HttpResponse.json({ success: true })
        })
      )

      renderWithProviders(
        <CharactersList gameId={123} userRole="gm" currentUserId={1} />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getAllByText('Publish')[0]).toBeInTheDocument()
      })

      const approveButton = screen.getAllByText('Publish')[0]
      fireEvent.click(approveButton)

      await waitFor(() => {
        expect(approvePayload).toEqual({ status: 'approved' })
      })
    })

    it('should call delete mutation when delete button is clicked for pending characters', async () => {
      let deletedCharacterId: string | null = null

      server.use(
        http.delete('http://localhost:3000/api/v1/characters/:id', ({ params }) => {
          deletedCharacterId = params.id as string
          return new HttpResponse(null, { status: 204 })
        })
      )

      renderWithProviders(
        <CharactersList gameId={123} userRole="gm" currentUserId={1} />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getAllByTestId('delete-character-button').length).toBeGreaterThanOrEqual(2)
      })

      // Click delete button for pending character (second character)
      // With dual-DOM: [0]=char1-mobile, [1]=char1-desktop, [2]=char2-mobile (pending), etc.
      const deleteButtons = screen.getAllByTestId('delete-character-button')
      fireEvent.click(deleteButtons[2])

      // Confirm deletion in modal
      await waitFor(() => {
        expect(screen.getByTestId('confirm-delete-character-button')).toBeInTheDocument()
      })

      const confirmButton = screen.getByTestId('confirm-delete-character-button')
      fireEvent.click(confirmButton)

      await waitFor(() => {
        expect(deletedCharacterId).toBe('2')
      })
    })
  })

  describe('My Characters section', () => {
    it('shows "My Characters" section at top for non-GM player with own character', async () => {
      renderWithProviders(
        <CharactersList gameId={123} userRole="player" currentUserId={1} />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getByText('My Characters')).toBeInTheDocument()
      })
    })

    it('"My Characters" section appears before "Player Characters" section', async () => {
      // Need another approved player character (not owned by user 1) to trigger "Player Characters" section
      server.use(
        http.get('http://localhost:3000/api/v1/games/:gameId/characters', () => {
          return HttpResponse.json([
            ...mockCharacters,
            { id: 4, name: 'Other Player Character', game_id: 123, user_id: 3, username: 'player3',
              character_type: 'player_character', status: 'approved', attributes: {}, inventory: [], notes: '',
              created_at: '2025-01-01T00:00:00Z', updated_at: '2025-01-01T00:00:00Z' },
          ])
        })
      )

      renderWithProviders(
        <CharactersList gameId={123} userRole="player" currentUserId={1} />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getByText('My Characters')).toBeInTheDocument()
      })

      const headings = screen.getAllByRole('heading', { level: 3 })
      const myCharIdx = headings.findIndex(h => h.textContent === 'My Characters')
      const playerCharIdx = headings.findIndex(h => h.textContent === 'Player Characters')
      expect(myCharIdx).toBeGreaterThanOrEqual(0)
      expect(playerCharIdx).toBeGreaterThanOrEqual(0)
      expect(myCharIdx).toBeLessThan(playerCharIdx)
    })

    it("player's own character appears in My Characters but not in Player Characters", async () => {
      // Need another approved player character to ensure "Player Characters" section renders
      server.use(
        http.get('http://localhost:3000/api/v1/games/:gameId/characters', () => {
          return HttpResponse.json([
            ...mockCharacters,
            { id: 4, name: 'Other Player Character', game_id: 123, user_id: 3, username: 'player3',
              character_type: 'player_character', status: 'approved', attributes: {}, inventory: [], notes: '',
              created_at: '2025-01-01T00:00:00Z', updated_at: '2025-01-01T00:00:00Z' },
          ])
        })
      )

      renderWithProviders(
        <CharactersList gameId={123} userRole="player" currentUserId={1} />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getByText('My Characters')).toBeInTheDocument()
      })

      const myCharSection = screen.getByText('My Characters').closest('div')!
      const playerCharSection = screen.getByText('Player Characters').closest('div')!

      // Hero Character (owned by user 1) should be in My Characters section
      expect(myCharSection.textContent).toContain('Hero Character')
      // Hero Character should NOT also be in the Player Characters section
      expect(playerCharSection.textContent).not.toContain('Hero Character')
      // Other player's character should be in Player Characters, not My Characters
      expect(playerCharSection.textContent).toContain('Other Player Character')
      expect(myCharSection.textContent).not.toContain('Other Player Character')
    })

    it('does NOT show "My Characters" section for GM', async () => {
      renderWithProviders(
        <CharactersList gameId={123} userRole="gm" currentUserId={1} />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getByText('Player Characters')).toBeInTheDocument()
      })

      expect(screen.queryByText('My Characters')).not.toBeInTheDocument()
    })

    it('shows "Characters" section header alongside "My Characters" in anonymous mode', async () => {
      // Default fixture: user=1 owns chars 1 and 3; char 4 (Other Hero, user=2) is the other approved char.
      renderWithProviders(
        <CharactersList gameId={123} userRole="player" currentUserId={1} isAnonymous={true} />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getByText('My Characters')).toBeInTheDocument()
        expect(screen.getAllByText('Other Hero').length).toBeGreaterThan(0)
      })

      const h3s = screen.getAllByRole('heading', { level: 3 })
      expect(h3s.some(h => h.textContent === 'Characters')).toBe(true)
    })

    it('shows "My Characters" section in anonymous mode too', async () => {
      renderWithProviders(
        <CharactersList gameId={123} userRole="player" currentUserId={1} isAnonymous={true} />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getByText('My Characters')).toBeInTheDocument()
      })
    })

    it('does NOT show "My Characters" section when player has no characters', async () => {
      server.use(
        http.get('http://localhost:3000/api/v1/games/:gameId/characters/controllable', () => {
          return HttpResponse.json([])
        })
      )

      renderWithProviders(
        <CharactersList gameId={123} userRole="player" currentUserId={999} />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getAllByText('Hero Character')[0]).toBeInTheDocument()
      })

      expect(screen.queryByText('My Characters')).not.toBeInTheDocument()
    })
  })

  describe('Status badges', () => {
    it('should NOT show badge for approved characters', async () => {
      renderWithProviders(
        <CharactersList gameId={123} userRole="gm" currentUserId={1} />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getAllByText('Hero Character')[0]).toBeInTheDocument()
      })

      // Approved badge should be hidden
      expect(screen.queryByText('approved')).not.toBeInTheDocument()
    })

    it('should show yellow badge for pending characters', async () => {
      renderWithProviders(
        <CharactersList gameId={123} userRole="gm" currentUserId={1} />,
      { gameId: 123 })

      await waitFor(() => {
        const pendingBadges = screen.getAllByText('pending')
        expect(pendingBadges.length).toBeGreaterThan(0)
        // Check that the badge's parent element has the correct classes
        const badgeElement = pendingBadges[0].closest('.bg-semantic-warning-subtle')
        expect(badgeElement).toBeInTheDocument()
      })
    })
  })

  describe('Delete character functionality', () => {
    it('GM should see delete button for all characters', async () => {
      renderWithProviders(
        <CharactersList gameId={123} userRole="gm" currentUserId={1} />,
      { gameId: 123 })

      await waitFor(() => {
        const deleteButtons = screen.getAllByTestId('delete-character-button')
        // Dual-DOM renders buttons in both mobile and desktop views (N characters × 2)
        expect(deleteButtons.length).toBe(mockCharacters.length * 2)
      })
    })

    it('Player should NOT see delete button', async () => {
      renderWithProviders(
        <CharactersList gameId={123} userRole="player" currentUserId={1} />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getAllByText('Hero Character')[0]).toBeInTheDocument()
      })

      expect(screen.queryByTestId('delete-character-button')).not.toBeInTheDocument()
    })

    it('clicking delete button opens confirmation modal', async () => {
      renderWithProviders(
        <CharactersList gameId={123} userRole="gm" currentUserId={1} />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getAllByTestId('delete-character-button').length).toBeGreaterThanOrEqual(2)
      })

      const deleteButtons = screen.getAllByTestId('delete-character-button')
      fireEvent.click(deleteButtons[0])

      await waitFor(() => {
        expect(screen.getByText('Delete Character?')).toBeInTheDocument()
      })

      expect(screen.getByText(/Are you sure you want to delete/)).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument()
      expect(screen.getByTestId('confirm-delete-character-button')).toBeInTheDocument()
    })

    it('confirmation modal displays character name', async () => {
      renderWithProviders(
        <CharactersList gameId={123} userRole="gm" currentUserId={1} />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getAllByTestId('delete-character-button').length).toBeGreaterThanOrEqual(2)
      })

      const deleteButtons = screen.getAllByTestId('delete-character-button')
      fireEvent.click(deleteButtons[0])

      await waitFor(() => {
        expect(screen.getByText(/Are you sure you want to delete/)).toBeInTheDocument()
      })

      // Verify character name appears in the confirmation text
      expect(screen.getByText(/Are you sure you want to delete/)).toHaveTextContent('Hero Character')
    })

    it('clicking confirm button calls delete API', async () => {
      let deletedCharacterId: string | null = null

      server.use(
        http.delete('http://localhost:3000/api/v1/characters/:id', ({ params }) => {
          deletedCharacterId = params.id as string
          return new HttpResponse(null, { status: 204 })
        })
      )

      renderWithProviders(
        <CharactersList gameId={123} userRole="gm" currentUserId={1} />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getAllByTestId('delete-character-button').length).toBeGreaterThanOrEqual(2)
      })

      const deleteButtons = screen.getAllByTestId('delete-character-button')
      fireEvent.click(deleteButtons[0])

      await waitFor(() => {
        expect(screen.getByTestId('confirm-delete-character-button')).toBeInTheDocument()
      })

      const confirmButton = screen.getByTestId('confirm-delete-character-button')
      fireEvent.click(confirmButton)

      await waitFor(() => {
        expect(deletedCharacterId).toBe('1')
      })
    })

    it('modal closes on successful deletion', async () => {
      server.use(
        http.delete('http://localhost:3000/api/v1/characters/:id', () => {
          return new HttpResponse(null, { status: 204 })
        })
      )

      renderWithProviders(
        <CharactersList gameId={123} userRole="gm" currentUserId={1} />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getAllByTestId('delete-character-button').length).toBeGreaterThanOrEqual(2)
      })

      const deleteButtons = screen.getAllByTestId('delete-character-button')
      fireEvent.click(deleteButtons[0])

      await waitFor(() => {
        expect(screen.getByTestId('confirm-delete-character-button')).toBeInTheDocument()
      })

      const confirmButton = screen.getByTestId('confirm-delete-character-button')
      fireEvent.click(confirmButton)

      await waitFor(() => {
        expect(screen.queryByText('Delete Character?')).not.toBeInTheDocument()
      })
    })

    it('displays error message if deletion fails', async () => {
      server.use(
        http.delete('http://localhost:3000/api/v1/characters/:id', () => {
          return HttpResponse.json(
            { detail: 'cannot delete character with existing messages' },
            { status: 400 }
          )
        })
      )

      renderWithProviders(
        <CharactersList gameId={123} userRole="gm" currentUserId={1} />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getAllByTestId('delete-character-button').length).toBeGreaterThanOrEqual(2)
      })

      const deleteButtons = screen.getAllByTestId('delete-character-button')
      fireEvent.click(deleteButtons[0])

      await waitFor(() => {
        expect(screen.getByTestId('confirm-delete-character-button')).toBeInTheDocument()
      })

      const confirmButton = screen.getByTestId('confirm-delete-character-button')
      fireEvent.click(confirmButton)

      await waitFor(() => {
        expect(screen.getByText(/cannot delete character with existing messages/)).toBeInTheDocument()
      })

      // Modal should remain open on error
      expect(screen.getByText('Delete Character?')).toBeInTheDocument()
    })

    it('cancel button closes modal without deleting', async () => {
      let deleteWasCalled = false

      server.use(
        http.delete('http://localhost:3000/api/v1/characters/:id', () => {
          deleteWasCalled = true
          return new HttpResponse(null, { status: 204 })
        })
      )

      renderWithProviders(
        <CharactersList gameId={123} userRole="gm" currentUserId={1} />,
      { gameId: 123 })

      await waitFor(() => {
        expect(screen.getAllByTestId('delete-character-button').length).toBeGreaterThanOrEqual(2)
      })

      const deleteButtons = screen.getAllByTestId('delete-character-button')
      fireEvent.click(deleteButtons[0])

      await waitFor(() => {
        expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument()
      })

      const cancelButton = screen.getByRole('button', { name: 'Cancel' })
      fireEvent.click(cancelButton)

      await waitFor(() => {
        expect(screen.queryByText('Delete Character?')).not.toBeInTheDocument()
      })

      expect(deleteWasCalled).toBe(false)
    })
  })

})
