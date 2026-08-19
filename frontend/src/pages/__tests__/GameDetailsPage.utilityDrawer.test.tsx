import { describe, it, expect, beforeEach, vi } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { server } from '../../mocks/server'
import { renderWithProviders } from '../../test-utils/render'
import { UtilityDrawerHarness } from '../../test-utils/utilityDrawer'
import { GameProvider } from '../../contexts/GameContext'
import { GameDetailsPage } from '../GameDetailsPage'
import type { Character } from '../../types/characters'

/**
 * The Utility Drawer is mounted at the app root and reachable from every page,
 * so the game it acts on comes from whatever game surface is mounted. It used to
 * come from CommonRoom alone, which only mounts on the Common Room tab of a
 * common_room phase — so on a completed game's archive, or during an action
 * phase, the drawer had no game at all and silently fell back to cross-game
 * mode, opening a character from an UNRELATED active game.
 *
 * These tests pin the drawer to the game on screen for the surfaces CommonRoom
 * never covered. They render GameDetailsPage, which is mounted for every tab
 * and every game state, and assert on which character sheet the drawer opens.
 *
 * WHY CharacterSheet IS MOCKED: its internals are not what's under test — the
 * wiring is, i.e. WHICH character id the drawer reaches and with what
 * permissions. Those are props with no dependable rendered form, so the probe
 * reports them via the DOM. (The mock predates the fix for the sheet's render
 * loop; it is kept for scoping, not because the real one still hangs.)
 */
vi.mock('../../components/CharacterSheet', () => ({
  CharacterSheet: ({ characterId, canEdit }: { characterId: number; canEdit?: boolean }) => (
    <div
      data-testid="character-sheet-probe"
      data-character-id={String(characterId)}
      data-can-edit={String(!!canEdit)}
    />
  ),
}))

const USER_ID = 1
const GM_USER_ID = 2

/** The character the user controls in the game being VIEWED (game 7). */
const viewedGameCharacter: Character = {
  id: 70,
  game_id: 7,
  name: 'Archivist Vell',
  character_type: 'player_character',
  user_id: USER_ID,
  assigned_user_id: USER_ID,
  status: 'approved',
  created_at: '2024-01-01T00:00:00Z',
}

/**
 * The character the user controls in a DIFFERENT, still-active game. The
 * cross-game endpoint returns only this one, so if the drawer ever falls back
 * to global mode it will auto-open this sheet — which is exactly the bug. Its
 * appearance in these tests is a failure, never a pass.
 */
const otherGameCharacter = {
  id: 99,
  game_id: 42,
  name: 'Unrelated Hero',
  character_type: 'player_character',
  status: 'approved',
  game_title: 'Some Other Game',
  game_state: 'in_progress',
  game_is_anonymous: false,
  user_role: 'player',
}

interface GameSetup {
  /** Game state under test — 'completed' has no active phase. */
  state: 'in_progress' | 'completed'
  /** Phase type reported by the current-phase endpoint (in_progress only). */
  phaseType?: 'common_room' | 'action'
  /** Characters the user controls in THIS game. */
  characters?: Character[]
}

function setupGame({ state, phaseType = 'action', characters = [viewedGameCharacter] }: GameSetup) {
  server.use(
    http.get('*/api/v1/auth/me', () =>
      HttpResponse.json({ id: USER_ID, username: 'player', email: 'player@example.com' })
    ),
    http.get('*/api/v1/games/:id/details', () =>
      HttpResponse.json({
        id: 7,
        title: 'The Viewed Game',
        description: 'The game the user is actually looking at',
        gm_user_id: GM_USER_ID,
        gm_username: 'thegm',
        state,
        max_players: 4,
        current_players: 2,
        is_public: true,
        is_anonymous: false,
        created_at: '2024-01-01T00:00:00Z',
        updated_at: '2024-01-01T00:00:00Z',
      })
    ),
    http.get('*/api/v1/games/:id/participants', () =>
      HttpResponse.json([
        { id: 1, game_id: 7, user_id: GM_USER_ID, username: 'thegm', role: 'gm', status: 'active' },
        { id: 2, game_id: 7, user_id: USER_ID, username: 'player', role: 'player', status: 'active' },
      ])
    ),
    http.get('*/api/v1/games/:id/current-phase', () =>
      HttpResponse.json(
        state === 'in_progress'
          ? {
              phase: {
                id: 5,
                game_id: 7,
                phase_number: 3,
                phase_type: phaseType,
                title: 'Phase 3',
                is_active: true,
                created_at: '2024-01-01T00:00:00Z',
                updated_at: '2024-01-01T00:00:00Z',
              },
            }
          : { phase: null }
      )
    ),
    http.get('*/api/v1/games/:id/characters/controllable', () => HttpResponse.json(characters)),
    http.get('*/api/v1/games/:id/characters', () => HttpResponse.json(characters)),
    http.get('*/api/v1/games/:id/characters/inactive', () => HttpResponse.json([])),
    // The cross-game fallback. Reaching this at all, inside a game, is the bug.
    http.get('*/api/v1/characters/controllable', () =>
      HttpResponse.json([otherGameCharacter])
    ),
    http.get('*/api/v1/games/:id/application/mine', () => HttpResponse.json(null)),
    http.get('*/api/v1/games/:id/deadlines', () => HttpResponse.json([])),
    http.get('*/api/v1/games/:id/polls', () => HttpResponse.json([])),
    http.get('*/api/v1/games/:id/phases/:phaseId/polls', () => HttpResponse.json([])),
    http.get('*/api/v1/games/:id/actions/mine', () => HttpResponse.json([])),
    http.get('*/api/v1/games/:id/results/mine', () => HttpResponse.json([])),
    http.get('*/api/v1/games/:id/results', () => HttpResponse.json([])),
    http.get('*/api/v1/games/:id/posts', () => HttpResponse.json([])),
    http.get('*/api/v1/games/:id/unread-comment-ids', () => HttpResponse.json([]))
  )
}

/** Render the game page (on the given tab) and open the Utility Drawer. */
async function renderPageAndOpenDrawer(tab: string) {
  const user = userEvent.setup()
  renderWithProviders(
    <GameProvider gameId={7}>
      <GameDetailsPage gameId={7} />
      <UtilityDrawerHarness />
    </GameProvider>,
    { initialEntries: [`/games/7?tab=${tab}`] }
  )

  // Wait for the game itself to load, so the page has published its context
  // before the drawer is opened.
  await screen.findByText('The Viewed Game')
  await user.click(screen.getByTestId('utility-drawer-toggle'))
  return user
}

describe('GameDetailsPage — Utility Drawer is scoped to the game on screen', () => {
  beforeEach(() => {
    server.resetHandlers()
    localStorage.clear()
    localStorage.setItem('auth_token', 'test-token')
    Element.prototype.scrollIntoView = () => {}
  })

  it('opens the character from a COMPLETED game, not from an unrelated active game', async () => {
    setupGame({ state: 'completed' })
    const user = await renderPageAndOpenDrawer('people')

    await user.click(await screen.findByTestId('utility-character-sheet'))

    const probe = await screen.findByTestId('character-sheet-probe')
    // The character in the completed game being viewed — NOT id 99, the one the
    // cross-game fallback would have supplied.
    expect(probe).toHaveAttribute('data-character-id', '70')
    // ...and a completed game's sheet is read-only, proving permissions were
    // derived from THIS game's state rather than the unrelated in_progress one.
    expect(probe).toHaveAttribute('data-can-edit', 'false')
  })

  it('opens the character from the viewed game during an ACTION phase, where no common room is mounted', async () => {
    setupGame({ state: 'in_progress', phaseType: 'action' })
    const user = await renderPageAndOpenDrawer('actions')

    await user.click(await screen.findByTestId('utility-character-sheet'))

    const probe = await screen.findByTestId('character-sheet-probe')
    expect(probe).toHaveAttribute('data-character-id', '70')
    // The game is open, so the player may still edit their own sheet.
    expect(probe).toHaveAttribute('data-can-edit', 'true')
  })

  it('hides the Character Sheet utility in a game where the user controls nobody, rather than offering a character from elsewhere', async () => {
    setupGame({ state: 'completed', characters: [] })
    await renderPageAndOpenDrawer('people')

    // The drawer opened and listed its utilities...
    expect(await screen.findByTestId('utility-list')).toBeInTheDocument()
    expect(screen.getByTestId('utility-dice-roller')).toBeInTheDocument()

    // ...but the character sheet is not among them, and nothing auto-opened
    // from the user's other game.
    expect(screen.queryByTestId('utility-character-sheet')).not.toBeInTheDocument()
    await waitFor(() =>
      expect(screen.queryByTestId('character-sheet-probe')).not.toBeInTheDocument()
    )
  })
})
