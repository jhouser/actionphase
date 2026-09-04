import { describe, it, expect, beforeEach } from 'vitest'
import { screen, waitFor, fireEvent } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { GameDetailsPage } from '../GameDetailsPage'
import { GameProvider } from '../../contexts/GameContext'
import { renderWithProviders } from '../../test-utils/render'
import { server } from '../../mocks/server'
import type { GameWithDetails, GameParticipant } from '../../types/games'

describe('GameDetailsPage', () => {
  const mockGame: GameWithDetails = {
    id: 1,
    title: 'Test Game',
    description: 'A test game description',
    gm_user_id: 999,
    gm_username: 'game_master',
    state: 'recruitment',
    genre: 'Fantasy',
    max_players: 5,
    current_players: 2,
    is_public: true,
    is_anonymous: false,
    recruitment_deadline: '2025-12-31T23:59:59Z',
    start_date: '2026-01-01T00:00:00Z',
    end_date: '2026-06-01T00:00:00Z',
    created_at: '2025-01-01T00:00:00Z',
    updated_at: '2025-01-01T00:00:00Z',
  }

  const mockParticipants: GameParticipant[] = [
    {
      id: 1,
      game_id: 1,
      user_id: 2,
      username: 'player1',
      email: 'player1@example.com',
      role: 'player',
      status: 'active',
      joined_at: '2025-01-02T00:00:00Z',
    },
    {
      id: 2,
      game_id: 1,
      user_id: 3,
      username: 'player2',
      email: 'player2@example.com',
      role: 'player',
      status: 'active',
      joined_at: '2025-01-03T00:00:00Z',
    },
  ]

  // Helper function to set up default MSW handlers
  const setupDefaultHandlers = (
    game: GameWithDetails = mockGame,
    participants: GameParticipant[] = mockParticipants,
    currentUserId: number = 1,
    hasApplication: boolean = false
  ) => {
    server.use(
      http.get('http://localhost:3000/api/v1/games/:id/details', () => {
        return HttpResponse.json(game)
      }),
      http.get('http://localhost:3000/api/v1/games/:id/participants', () => {
        return HttpResponse.json(participants)
      }),
      http.get('http://localhost:3000/api/v1/auth/me', () => {
        return HttpResponse.json({
          id: currentUserId,
          username: currentUserId === 999 ? 'game_master' : `player${currentUserId}`,
          email: `user${currentUserId}@example.com`,
        })
      }),
      http.get('http://localhost:3000/api/v1/games/:id/application/mine', () => {
        if (hasApplication) {
          return HttpResponse.json({
            id: 1,
            game_id: 1,
            user_id: currentUserId,
            role: 'player',
            status: 'pending',
            message: 'I would love to join!',
            created_at: '2025-01-05T00:00:00Z',
          })
        }
        return HttpResponse.json(null)
      }),
      http.get('http://localhost:3000/api/v1/games/:id/current-phase', () => {
        if (game.state === 'in_progress') {
          return HttpResponse.json({
            phase: {
              id: 1,
              game_id: 1,
              phase_number: 1,
              phase_type: 'action',
              title: 'Phase 1: Planning',
              description: 'Plan your actions',
              deadline: '2026-01-15T23:59:59Z',
              is_active: true,
              created_at: '2026-01-01T00:00:00Z',
              updated_at: '2026-01-01T00:00:00Z',
            },
          })
        }
        return HttpResponse.json({ phase: null })
      }),
      http.get('http://localhost:3000/api/v1/games/:id/characters/controllable', () => {
        return HttpResponse.json([])
      }),
      // Action results endpoints (needed by HistoryView component when displayed)
      http.get('http://localhost:3000/api/v1/games/:id/results/mine', () => {
        return HttpResponse.json([])
      }),
      http.get('http://localhost:3000/api/v1/games/:id/results', () => {
        return HttpResponse.json([])
      }),
      // Actions endpoint (needed by action-related hooks)
      http.get('http://localhost:3000/api/v1/games/:id/actions/mine', () => {
        return HttpResponse.json([])
      }),
      // Inactive characters endpoint
      http.get('http://localhost:3000/api/v1/games/:id/characters/inactive', () => {
        return HttpResponse.json([])
      }),
      // User controllable characters endpoint (needed by GameContext)
      http.get('http://localhost:3000/api/v1/games/:id/characters/controllable', () => {
        return HttpResponse.json([])
      }),
      // Deadlines endpoint (needed by GameDetailsPage)
      http.get('http://localhost:3000/api/v1/games/:id/deadlines', () => {
        return HttpResponse.json([])
      }),
      // Polls endpoint (needed by usePolls hook)
      http.get('http://localhost:3000/api/v1/games/:id/polls', () => {
        return HttpResponse.json([])
      })
    )
  }

  // Helper to render GameDetailsPage with GameProvider
  const renderGameDetailsPage = (gameId: number = 1, props?: Partial<React.ComponentProps<typeof GameDetailsPage>>) => {
    return renderWithProviders(
      <GameProvider gameId={gameId}>
        <GameDetailsPage gameId={gameId} {...props} />
      </GameProvider>
    )
  }

  beforeEach(() => {
    server.resetHandlers()
    localStorage.clear()
    // Set auth token so AuthContext will fetch user
    localStorage.setItem('auth_token', 'test-token')
  })

  describe('Error States', () => {
    it('should show error message when game fetch fails', async () => {
      server.use(
        http.get('http://localhost:3000/api/v1/games/:id/details', () => {
          return HttpResponse.json(
            { detail: 'Game not found' },
            { status: 404 }
          )
        }),
        http.get('http://localhost:3000/api/v1/games/:id/participants', () => {
          return HttpResponse.json([])
        })
      )

      renderGameDetailsPage(1)

      await waitFor(() => {
        expect(screen.getByText('Error')).toBeInTheDocument()
        expect(screen.getByRole('button', { name: /go back/i })).toBeInTheDocument()
      })
    })
  })

  describe('Game Display', () => {
    it('should display game title, description, and basic info', async () => {
      setupDefaultHandlers()

      renderGameDetailsPage(1)

      await waitFor(() => {
        expect(screen.getByText('Test Game')).toBeInTheDocument()
        expect(screen.getByText('A test game description')).toBeInTheDocument()
        expect(screen.getAllByRole('link', { name: 'game_master' }).length).toBeGreaterThanOrEqual(1)
        expect(screen.getAllByText(/Genre: Fantasy/i).length).toBeGreaterThanOrEqual(1)
      })
    })

    it('should display participant count', async () => {
      setupDefaultHandlers()

      renderGameDetailsPage(1)

      await waitFor(() => {
        const playerCounts = screen.getAllByText(/2 \/ 5/)
        expect(playerCounts.length).toBeGreaterThan(0)
      })
    })
  })

  describe('Visitor View', () => {
    it('should show Apply to Join button for non-GM during recruitment', async () => {
      setupDefaultHandlers(mockGame, mockParticipants, 100) // Not GM, not participant

      renderGameDetailsPage(1)

      await waitFor(() => {
        expect(screen.getAllByTestId('apply-button-1').length).toBeGreaterThanOrEqual(1)
      })
    })

    it('should not show GM controls for visitors', async () => {
      setupDefaultHandlers(mockGame, mockParticipants, 100)

      renderGameDetailsPage(1)

      await waitFor(() => {
        expect(screen.getByText('Test Game')).toBeInTheDocument()
      })

      expect(screen.queryByRole('button', { name: /edit game/i })).not.toBeInTheDocument()
    })
  })

  describe('User with Pending Application', () => {
    it('should display application status and withdraw button', async () => {
      setupDefaultHandlers(mockGame, mockParticipants, 100, true) // Has application

      renderGameDetailsPage(1)

      await waitFor(() => {
        expect(screen.getByText('Test Game')).toBeInTheDocument()
      })

      await waitFor(() => {
        expect(screen.getByText(/your application status/i)).toBeInTheDocument()
        expect(screen.getByText(/applied as player/i)).toBeInTheDocument()
        expect(screen.getByText(/I would love to join!/i)).toBeInTheDocument()
        expect(screen.getAllByTestId('withdraw-application-button').length).toBeGreaterThanOrEqual(1)
      }, { timeout: 3000 })
    })

    it('should not show Apply to Join button when application exists', async () => {
      setupDefaultHandlers(mockGame, mockParticipants, 100, true)

      renderGameDetailsPage(1)

      await waitFor(() => {
        expect(screen.getByText(/your application status/i)).toBeInTheDocument()
      }, { timeout: 3000 })

      expect(screen.queryByRole('button', { name: /apply to join/i })).not.toBeInTheDocument()
    })
  })

  describe('GM View', () => {
    it('should show Edit Game button for GM', async () => {
      setupDefaultHandlers(mockGame, mockParticipants, 999) // GM user

      renderGameDetailsPage(1)

      // Wait for game to load first
      await waitFor(() => {
        expect(screen.getByText('Test Game')).toBeInTheDocument()
      })

      // Wait for the game actions menu button to appear
      const menuButton = await waitFor(() => {
        return screen.getByTestId('game-actions-menu')
      }, { timeout: 3000 })

      // Click the menu to open it
      fireEvent.click(menuButton)

      // Then wait for Edit Game button to appear in the menu
      await waitFor(() => {
        expect(screen.getByRole('button', { name: /edit game/i })).toBeInTheDocument()
      })
    })

    it('should show state transition buttons for GM during recruitment', async () => {
      setupDefaultHandlers(mockGame, mockParticipants, 999)

      renderGameDetailsPage(1)

      await waitFor(() => {
        expect(screen.getByText('Test Game')).toBeInTheDocument()
      })

      // Wait for the game actions menu button and click it
      const menuButton = await waitFor(() => {
        return screen.getByTestId('game-actions-menu')
      }, { timeout: 3000 })

      fireEvent.click(menuButton)

      // Then check for state transition buttons in the opened menu
      await waitFor(() => {
        expect(screen.getByRole('button', { name: /start character creation/i })).toBeInTheDocument()
        expect(screen.getByRole('button', { name: /cancel game/i })).toBeInTheDocument()
      })
    })

    it('should not show Apply to Join button for GM', async () => {
      setupDefaultHandlers(mockGame, mockParticipants, 999)

      renderGameDetailsPage(1)

      await waitFor(() => {
        expect(screen.getByText('Test Game')).toBeInTheDocument()
      })

      // Wait for the game actions menu button to appear (confirms GM has access)
      await waitFor(() => {
        expect(screen.getByTestId('game-actions-menu')).toBeInTheDocument()
      }, { timeout: 3000 })

      // Ensure Apply to Join button is not visible
      expect(screen.queryByRole('button', { name: /apply to join/i })).not.toBeInTheDocument()
    })
  })

  describe('Participant View', () => {
    it('should not show GM controls for participants', async () => {
      setupDefaultHandlers(mockGame, mockParticipants, 2)

      renderGameDetailsPage(1)

      await waitFor(() => {
        expect(screen.getByText('Test Game')).toBeInTheDocument()
      })

      expect(screen.queryByRole('button', { name: /edit game/i })).not.toBeInTheDocument()
    })
  })

  describe('Tab Navigation - URL preservation on page load/refresh', () => {
    // These tests cover the integration between GameDetailsPage (which passes game?.state)
    // and useGameTabs (which must not redirect valid URL tabs before game data arrives).
    // Bugs in this area were previously only caught by manual testing.

    it('should respect ?tab=info on refresh for a recruitment game (non-default tab)', async () => {
      // Default tab for a non-GM in recruitment is 'info', so also test handouts
      setupDefaultHandlers(mockGame, mockParticipants, 100)

      renderWithProviders(
        <GameProvider gameId={1}>
          <GameDetailsPage gameId={1} />
        </GameProvider>,
        { initialEntries: ['/games/1?tab=handouts'] }
      )

      await waitFor(() => {
        expect(screen.getByText('Test Game')).toBeInTheDocument()
      })

      // The handouts tab must be active — NOT redirected to info (the default)
      await waitFor(() => {
        const handoutsTab = screen.getByRole('tab', { name: /handouts/i })
        expect(handoutsTab).toHaveAttribute('aria-selected', 'true')
      }, { timeout: 3000 })
    })

    it('should respect ?tab=handouts on refresh for a character_creation game (non-default tab)', async () => {
      // Default for character_creation is 'people'. This test verifies handouts URL is preserved.
      const charCreationGame = { ...mockGame, state: 'character_creation' as const }
      setupDefaultHandlers(charCreationGame, mockParticipants, 999)

      renderWithProviders(
        <GameProvider gameId={1}>
          <GameDetailsPage gameId={1} />
        </GameProvider>,
        { initialEntries: ['/games/1?tab=handouts'] }
      )

      await waitFor(() => {
        expect(screen.getByText('Test Game')).toBeInTheDocument()
      })

      // Must stay on handouts, not redirect to people
      await waitFor(() => {
        const handoutsTab = screen.getByRole('tab', { name: /handouts/i })
        expect(handoutsTab).toHaveAttribute('aria-selected', 'true')
      }, { timeout: 3000 })
    })

    it('should respect ?tab=people on refresh for a character_creation game', async () => {
      // This is the exact scenario reported: refreshing with ?tab=people redirected to handouts
      const charCreationGame = { ...mockGame, state: 'character_creation' as const }
      setupDefaultHandlers(charCreationGame, mockParticipants, 999)

      renderWithProviders(
        <GameProvider gameId={1}>
          <GameDetailsPage gameId={1} />
        </GameProvider>,
        { initialEntries: ['/games/1?tab=people'] }
      )

      await waitFor(() => {
        expect(screen.getByText('Test Game')).toBeInTheDocument()
      })

      // Must stay on people, not redirect to handouts or any other default
      await waitFor(() => {
        const peopleTab = screen.getByRole('tab', { name: /people/i })
        expect(peopleTab).toHaveAttribute('aria-selected', 'true')
      }, { timeout: 3000 })
    })

    it('should respect ?tab=info on refresh for a setup game (non-default tab)', async () => {
      // Default for setup is 'handouts'. Verify info URL is preserved.
      const setupGame = { ...mockGame, state: 'setup' as const }
      setupDefaultHandlers(setupGame, [], 999)

      renderWithProviders(
        <GameProvider gameId={1}>
          <GameDetailsPage gameId={1} />
        </GameProvider>,
        { initialEntries: ['/games/1?tab=info'] }
      )

      await waitFor(() => {
        expect(screen.getByText('Test Game')).toBeInTheDocument()
      })

      await waitFor(() => {
        const infoTab = screen.getByRole('tab', { name: /game info/i })
        expect(infoTab).toHaveAttribute('aria-selected', 'true')
      }, { timeout: 3000 })
    })

    it('should respect ?tab=people on refresh for an in_progress game while phase data is loading', async () => {
      // in_progress has a phase-loading guard — people must not be redirected to phase default
      // while currentPhase is still fetching
      const inProgressGame = { ...mockGame, state: 'in_progress' as const }

      // Delay the phase response to simulate loading
      server.use(
        http.get('http://localhost:3000/api/v1/games/:id/details', () =>
          HttpResponse.json(inProgressGame)
        ),
        http.get('http://localhost:3000/api/v1/games/:id/participants', () =>
          HttpResponse.json(mockParticipants)
        ),
        http.get('http://localhost:3000/api/v1/auth/me', () =>
          HttpResponse.json({ id: 2, username: 'player1', email: 'player1@example.com' })
        ),
        http.get('http://localhost:3000/api/v1/games/:id/application/mine', () =>
          HttpResponse.json(null)
        ),
        http.get('http://localhost:3000/api/v1/games/:id/current-phase', async () => {
          // Simulate a slow phase fetch
          await new Promise(resolve => setTimeout(resolve, 50))
          return HttpResponse.json({
            phase: { id: 1, game_id: 1, phase_number: 1, phase_type: 'action',
              title: 'Phase 1', description: '', deadline: null,
              is_active: true, created_at: '', updated_at: '' }
          })
        }),
        http.get('http://localhost:3000/api/v1/games/:id/characters/controllable', () =>
          HttpResponse.json([])
        ),
        http.get('http://localhost:3000/api/v1/games/:id/deadlines', () =>
          HttpResponse.json([])
        ),
        http.get('http://localhost:3000/api/v1/games/:id/polls', () =>
          HttpResponse.json([])
        ),
      )

      renderWithProviders(
        <GameProvider gameId={1}>
          <GameDetailsPage gameId={1} />
        </GameProvider>,
        { initialEntries: ['/games/1?tab=people'] }
      )

      await waitFor(() => {
        expect(screen.getByText('Test Game')).toBeInTheDocument()
      })

      // Once phase loads and tabs stabilise, people tab must still be active
      await waitFor(() => {
        const peopleTab = screen.getByRole('tab', { name: /people/i })
        expect(peopleTab).toHaveAttribute('aria-selected', 'true')
      }, { timeout: 3000 })
    })
  })

  describe('Tab Navigation - Link-style switching updates view', () => {
    // Clicking a tab renders an <a> / <Link> — it changes the URL without calling
    // setActiveTab directly. The view must update from URL alone.

    it('should switch view when clicking from default tab to another tab', async () => {
      const charCreationGame = { ...mockGame, state: 'character_creation' as const }
      setupDefaultHandlers(charCreationGame, mockParticipants, 999)

      renderWithProviders(
        <GameProvider gameId={1}>
          <GameDetailsPage gameId={1} />
        </GameProvider>,
        { initialEntries: ['/games/1'] }
      )

      await waitFor(() => {
        expect(screen.getByText('Test Game')).toBeInTheDocument()
      })

      // Wait for tabs to appear
      const handoutsTab = await waitFor(() =>
        screen.getByRole('tab', { name: /handouts/i }), { timeout: 3000 }
      )

      // Click handouts tab (Link navigation — updates URL, not direct state)
      fireEvent.click(handoutsTab)

      // Handouts tab must become active
      await waitFor(() => {
        expect(handoutsTab).toHaveAttribute('aria-selected', 'true')
      })
    })

    it('should switch back after clicking a second tab', async () => {
      const charCreationGame = { ...mockGame, state: 'character_creation' as const }
      setupDefaultHandlers(charCreationGame, mockParticipants, 999)

      renderWithProviders(
        <GameProvider gameId={1}>
          <GameDetailsPage gameId={1} />
        </GameProvider>,
        { initialEntries: ['/games/1?tab=people'] }
      )

      await waitFor(() => {
        expect(screen.getByText('Test Game')).toBeInTheDocument()
      })

      const handoutsTab = await waitFor(() =>
        screen.getByRole('tab', { name: /handouts/i }), { timeout: 3000 }
      )
      const peopleTab = screen.getByRole('tab', { name: /people/i })

      // Start on people
      await waitFor(() => {
        expect(peopleTab).toHaveAttribute('aria-selected', 'true')
      })

      // Click handouts
      fireEvent.click(handoutsTab)
      await waitFor(() => {
        expect(handoutsTab).toHaveAttribute('aria-selected', 'true')
        expect(peopleTab).toHaveAttribute('aria-selected', 'false')
      })

      // Click back to people
      fireEvent.click(peopleTab)
      await waitFor(() => {
        expect(peopleTab).toHaveAttribute('aria-selected', 'true')
        expect(handoutsTab).toHaveAttribute('aria-selected', 'false')
      })
    })
  })

  describe('Tab Navigation - Recruitment State', () => {
    it('should show correct tabs for GM during recruitment', async () => {
      setupDefaultHandlers(mockGame, mockParticipants, 999)

      renderGameDetailsPage(1)

      await waitFor(() => {
        expect(screen.getByText('Test Game')).toBeInTheDocument()
      })

      // During recruitment, GMs see Applications and Game Info tabs
      // No Participants tab since participants aren't confirmed yet
      await waitFor(() => {
        expect(screen.getByRole('tab', { name: /applications/i })).toBeInTheDocument()
        expect(screen.getByRole('tab', { name: /game info/i })).toBeInTheDocument()
        expect(screen.queryByRole('tab', { name: /participants/i })).not.toBeInTheDocument()
      }, { timeout: 3000 })
    })

    it('should display participants list when clicking People tab in character creation', async () => {
      // Use character_creation state which has a People tab
      const characterCreationGame = { ...mockGame, state: 'character_creation' as const }
      setupDefaultHandlers(characterCreationGame, mockParticipants, 999)

      renderGameDetailsPage(1)

      // Click the People tab
      await waitFor(() => {
        const peopleTab = screen.getByRole('tab', { name: /people/i })
        fireEvent.click(peopleTab)
      })

      // Click the "Game Participants" sub-tab within the People view
      await waitFor(() => {
        const participantsSubTab = screen.getByRole('button', { name: /game participants/i })
        fireEvent.click(participantsSubTab)
      })

      // Verify participants are displayed
      await waitFor(() => {
        expect(screen.getByText('player1')).toBeInTheDocument()
        expect(screen.getByText('player2')).toBeInTheDocument()
      })
    })
  })

  describe('Tab Navigation - In Progress State', () => {
    it('should show game tabs when game is in progress', async () => {
      const inProgressGame = { ...mockGame, state: 'in_progress' as const }
      setupDefaultHandlers(inProgressGame, mockParticipants, 2) // player

      renderGameDetailsPage(1)

      // Wait for phase-independent tabs first (these load immediately)
      await waitFor(() => {
        expect(screen.getByRole('tab', { name: /people/i })).toBeInTheDocument()
      })

      // Then wait for phase-dependent tab (requires currentPhase query to complete)
      await waitFor(() => {
        expect(screen.getByRole('tab', { name: /submit action/i })).toBeInTheDocument()
      }, { timeout: 3000 })

      // Verify all tabs are present
      expect(screen.getByRole('tab', { name: /^messages$/i })).toBeInTheDocument()
      expect(screen.getByRole('tab', { name: /history/i })).toBeInTheDocument()
    })

    it('should display current phase information', async () => {
      const inProgressGame = { ...mockGame, state: 'in_progress' as const }
      setupDefaultHandlers(inProgressGame, mockParticipants, 2)

      renderGameDetailsPage(1)

      await waitFor(() => {
        expect(screen.getByText('Test Game')).toBeInTheDocument()
      })

      // Wait for the Submit Action tab to appear (indicates phase data has loaded)
      // The presence of this tab confirms the game is in an action phase
      await waitFor(() => {
        expect(screen.getByRole('tab', { name: /submit action/i })).toBeInTheDocument()
      }, { timeout: 3000 })
    })
  })

  describe('State-Specific Buttons', () => {
    it('should show Start Recruitment button for setup state', async () => {
      const setupGame = { ...mockGame, state: 'setup' as const }
      setupDefaultHandlers(setupGame, [], 999)

      renderGameDetailsPage(1)

      await waitFor(() => {
        expect(screen.getByText('Test Game')).toBeInTheDocument()
      })

      // Click the menu to see state buttons
      const menuButton = await waitFor(() => {
        return screen.getByTestId('game-actions-menu')
      }, { timeout: 3000 })

      fireEvent.click(menuButton)

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /start recruitment/i })).toBeInTheDocument()
      })
    })

    it('should show Start Game button for character_creation state', async () => {
      const charCreationGame = { ...mockGame, state: 'character_creation' as const }
      setupDefaultHandlers(charCreationGame, mockParticipants, 999)

      renderGameDetailsPage(1)

      await waitFor(() => {
        expect(screen.getByText('Test Game')).toBeInTheDocument()
      })

      // Click the menu to see state buttons
      const menuButton = await waitFor(() => {
        return screen.getByTestId('game-actions-menu')
      }, { timeout: 3000 })

      fireEvent.click(menuButton)

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /start game/i })).toBeInTheDocument()
      })
    })

    it('should show Pause and Complete buttons for in_progress state', async () => {
      const inProgressGame = { ...mockGame, state: 'in_progress' as const }
      setupDefaultHandlers(inProgressGame, mockParticipants, 999)

      renderGameDetailsPage(1)

      await waitFor(() => {
        expect(screen.getByText('Test Game')).toBeInTheDocument()
      })

      // Click the menu to see state buttons
      const menuButton = await waitFor(() => {
        return screen.getByTestId('game-actions-menu')
      }, { timeout: 3000 })

      fireEvent.click(menuButton)

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /pause game/i })).toBeInTheDocument()
        expect(screen.getByRole('button', { name: /complete game/i })).toBeInTheDocument()
      })
    })
  })
})
