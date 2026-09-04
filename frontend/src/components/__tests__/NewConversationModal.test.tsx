import { describe, it, expect, beforeEach } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { NewConversationModal } from '../NewConversationModal'
import { renderWithProviders } from '../../test-utils/render'
import { server } from '../../mocks/server'
import type { Character } from '../../types/characters'

describe('NewConversationModal', () => {
  const mockGameId = 1
  const mockOnClose = vi.fn()
  const mockOnConversationCreated = vi.fn()

  // Mock characters controlled by the user
  const userCharacters: Character[] = [
    {
      id: 1,
      game_id: mockGameId,
      user_id: 1,
      username: 'testuser',
      name: 'Alice',
      character_type: 'player_character',
      status: 'approved',
      is_active: true,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    },
    {
      id: 2,
      game_id: mockGameId,
      user_id: 1,
      username: 'testuser',
      name: 'Bob',
      character_type: 'player_character',
      status: 'approved',
      is_active: true,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    },
  ]

  // Mock all characters in the game (including NPCs and other player characters)
  const allCharacters: Character[] = [
    ...userCharacters,
    {
      id: 3,
      game_id: mockGameId,
      user_id: 2,
      username: 'otheruser',
      name: 'Charlie',
      character_type: 'player_character',
      status: 'approved',
      is_active: true,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    },
    {
      id: 4,
      game_id: mockGameId,
      user_id: undefined,
      username: undefined,
      name: 'The Mysterious NPC',
      character_type: 'npc',
      status: 'approved',
      is_active: true,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    },
  ]

  const setupDefaultHandlers = () => {
    server.use(
      http.post('/api/v1/games/:gameId/conversations', async ({ request }) => {
        const body = await request.json() as { title?: string; character_ids: number[] }
        return HttpResponse.json({
          id: 100,
          game_id: mockGameId,
          title: body.title || 'New Conversation',
          conversation_type: 'group',
          created_by_user_id: 1,
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        }, { status: 201 })
      })
    )
  }

  beforeEach(() => {
    server.resetHandlers()
    setupDefaultHandlers()
    mockOnClose.mockClear()
    mockOnConversationCreated.mockClear()
  })

  describe('Rendering', () => {
    it('renders modal with title and close button', () => {
      renderWithProviders(
        <NewConversationModal
          gameId={mockGameId}
          characters={userCharacters}
          allCharacters={allCharacters}
          isAnonymous={false}
          allowGroupConversations={true}
          onClose={mockOnClose}
          onConversationCreated={mockOnConversationCreated}
        />
      )

      expect(screen.getByText('New Conversation')).toBeInTheDocument()
      // The close button is the button with just an SVG (no text)
      const buttons = screen.getAllByRole('button')
      const closeButton = buttons.find(btn => btn.querySelector('svg') && !btn.textContent)
      expect(closeButton).toBeInTheDocument()
    })

    it('displays backdrop and modal container', () => {
      const { container } = renderWithProviders(
        <NewConversationModal
          gameId={mockGameId}
          characters={userCharacters}
          allCharacters={allCharacters}
          isAnonymous={false}
          allowGroupConversations={true}
          onClose={mockOnClose}
          onConversationCreated={mockOnConversationCreated}
        />
      )

      // Check for the Modal component structure (backdrop with blur)
      const backdrop = container.querySelector('.backdrop-blur-sm')
      expect(backdrop).toBeInTheDocument()

      const modalContainer = container.querySelector('.surface-raised.rounded-lg')
      expect(modalContainer).toBeInTheDocument()
    })
  })

  describe('Form Fields', () => {
    it('renders conversation title input field', () => {
      renderWithProviders(
        <NewConversationModal
          gameId={mockGameId}
          characters={userCharacters}
          allCharacters={allCharacters}
          isAnonymous={false}
          allowGroupConversations={true}
          onClose={mockOnClose}
          onConversationCreated={mockOnConversationCreated}
        />
      )

      const titleInput = screen.getByPlaceholderText('e.g., Planning the heist')
      expect(titleInput).toBeInTheDocument()
      expect(titleInput).toHaveAttribute('type', 'text')
      expect(titleInput).toHaveAttribute('required') // Verify required field
      expect(screen.getByText(/Conversation Title/i)).toBeInTheDocument()
    })

    it('renders your character section with select dropdown when multiple characters', () => {
      renderWithProviders(
        <NewConversationModal
          gameId={mockGameId}
          characters={userCharacters}
          allCharacters={allCharacters}
          isAnonymous={false}
          allowGroupConversations={true}
          onClose={mockOnClose}
          onConversationCreated={mockOnConversationCreated}
        />
      )

      expect(screen.getByText('Your Character')).toBeInTheDocument()
      const select = screen.getByRole('combobox')
      expect(select).toBeInTheDocument()
      expect(screen.getByText('Select your character...')).toBeInTheDocument()
    })

    it('auto-selects and displays single character when user has only one', () => {
      const singleCharacter = [userCharacters[0]]
      const { container } = renderWithProviders(
        <NewConversationModal
          gameId={mockGameId}
          characters={singleCharacter}
          allCharacters={allCharacters}
          isAnonymous={false}
          allowGroupConversations={true}
          onClose={mockOnClose}
          onConversationCreated={mockOnConversationCreated}
        />
      )

      // The auto-selected character display uses bg-interactive-primary-subtle
      const selectedCharDiv = container.querySelector('.bg-interactive-primary-subtle')
      expect(selectedCharDiv).toBeInTheDocument()
      expect(within(selectedCharDiv!).getByText('Alice')).toBeInTheDocument()
      expect(within(selectedCharDiv!).getByText('player character')).toBeInTheDocument()
      expect(within(selectedCharDiv!).getByText('Played by testuser')).toBeInTheDocument()
      // No select dropdown should be present
      expect(screen.queryByRole('combobox')).not.toBeInTheDocument()
    })

    it('does not show username in anonymous games', () => {
      const singleCharacter = [userCharacters[0]]
      renderWithProviders(
        <NewConversationModal
          gameId={mockGameId}
          characters={singleCharacter}
          allCharacters={allCharacters}
          isAnonymous={true}
          allowGroupConversations={true}
          onClose={mockOnClose}
          onConversationCreated={mockOnConversationCreated}
        />
      )

      expect(screen.queryByText('Played by testuser')).not.toBeInTheDocument()
    })

    it('shows message when user has no characters', () => {
      renderWithProviders(
        <NewConversationModal
          gameId={mockGameId}
          characters={[]}
          allCharacters={allCharacters}
          isAnonymous={false}
          allowGroupConversations={true}
          onClose={mockOnClose}
          onConversationCreated={mockOnConversationCreated}
        />
      )

      expect(screen.getByText('You need at least one character to create conversations')).toBeInTheDocument()
    })

    it('renders participants section', () => {
      renderWithProviders(
        <NewConversationModal
          gameId={mockGameId}
          characters={userCharacters}
          allCharacters={allCharacters}
          isAnonymous={false}
          allowGroupConversations={true}
          onClose={mockOnClose}
          onConversationCreated={mockOnConversationCreated}
        />
      )

      expect(screen.getByText(/Participants \(select at least 1\)/i)).toBeInTheDocument()
    })

    it('displays available participants from allCharacters prop', () => {
      renderWithProviders(
        <NewConversationModal
          gameId={mockGameId}
          characters={userCharacters}
          allCharacters={allCharacters}
          isAnonymous={false}
          allowGroupConversations={true}
          onClose={mockOnClose}
          onConversationCreated={mockOnConversationCreated}
        />
      )

      // Should show other characters (Charlie and NPC)
      expect(screen.getByText('Charlie')).toBeInTheDocument()
      expect(screen.getByText('The Mysterious NPC')).toBeInTheDocument()

      // Alice and Bob SHOULD be in the participants list (they can be selected as participants)
      const participantsList = screen.getByText('Charlie').closest('div[class*="space-y-2"]')
      expect(participantsList).toBeInTheDocument()

      const aliceInParticipants = within(participantsList!).queryByText('Alice')
      const bobInParticipants = within(participantsList!).queryByText('Bob')
      expect(aliceInParticipants).toBeInTheDocument()
      expect(bobInParticipants).toBeInTheDocument()
    })

    it('shows message when no other characters available', () => {
      // Use only ONE character so it gets auto-selected, leaving no other participants available
      const singleCharacter = [userCharacters[0]] // Just Alice

      renderWithProviders(
        <NewConversationModal
          gameId={mockGameId}
          characters={singleCharacter}
          allCharacters={singleCharacter} // Only Alice in the game
          isAnonymous={false}
          allowGroupConversations={true}
          onClose={mockOnClose}
          onConversationCreated={mockOnConversationCreated}
        />
      )

      // Alice gets auto-selected as "your character"
      // No other characters exist, so "No other characters available" should show
      expect(screen.getByText('No other characters available')).toBeInTheDocument()
    })
  })

  describe('User Interactions', () => {
    it('allows user to type in title field', async () => {
      const user = userEvent.setup()
      renderWithProviders(
        <NewConversationModal
          gameId={mockGameId}
          characters={userCharacters}
          allCharacters={allCharacters}
          isAnonymous={false}
          allowGroupConversations={true}
          onClose={mockOnClose}
          onConversationCreated={mockOnConversationCreated}
        />
      )

      const titleInput = screen.getByPlaceholderText('e.g., Planning the heist')
      await user.type(titleInput, 'Secret Meeting')

      expect(titleInput).toHaveValue('Secret Meeting')
    })

    it('allows user to select their character from dropdown', async () => {
      const user = userEvent.setup()
      renderWithProviders(
        <NewConversationModal
          gameId={mockGameId}
          characters={userCharacters}
          allCharacters={allCharacters}
          isAnonymous={false}
          allowGroupConversations={true}
          onClose={mockOnClose}
          onConversationCreated={mockOnConversationCreated}
        />
      )

      const select = screen.getByRole('combobox')
      await user.selectOptions(select, '1')

      expect(select).toHaveValue('1')
    })

    it('allows user to select and deselect participants', async () => {
      const user = userEvent.setup()
      renderWithProviders(
        <NewConversationModal
          gameId={mockGameId}
          characters={userCharacters}
          allCharacters={allCharacters}
          isAnonymous={false}
          allowGroupConversations={true}
          onClose={mockOnClose}
          onConversationCreated={mockOnConversationCreated}
        />
      )

      const charlieCheckbox = screen.getByRole('checkbox', { name: /Charlie/i })
      const npcCheckbox = screen.getByRole('checkbox', { name: /The Mysterious NPC/i })

      // Select Charlie
      await user.click(charlieCheckbox)
      expect(charlieCheckbox).toBeChecked()
      expect(screen.getByText('1 participant selected')).toBeInTheDocument()

      // Select NPC
      await user.click(npcCheckbox)
      expect(npcCheckbox).toBeChecked()
      expect(screen.getByText('2 participants selected')).toBeInTheDocument()

      // Deselect Charlie
      await user.click(charlieCheckbox)
      expect(charlieCheckbox).not.toBeChecked()
      expect(screen.getByText('1 participant selected')).toBeInTheDocument()
    })

    it('displays participant count correctly', async () => {
      const user = userEvent.setup()
      renderWithProviders(
        <NewConversationModal
          gameId={mockGameId}
          characters={userCharacters}
          allCharacters={allCharacters}
          isAnonymous={false}
          allowGroupConversations={true}
          onClose={mockOnClose}
          onConversationCreated={mockOnConversationCreated}
        />
      )

      // Initially no participants selected
      expect(screen.queryByText(/participant.* selected/i)).not.toBeInTheDocument()

      // Select one participant
      const charlieCheckbox = screen.getByRole('checkbox', { name: /Charlie/i })
      await user.click(charlieCheckbox)
      expect(screen.getByText('1 participant selected')).toBeInTheDocument()

      // Select another participant (plural form)
      const npcCheckbox = screen.getByRole('checkbox', { name: /The Mysterious NPC/i })
      await user.click(npcCheckbox)
      expect(screen.getByText('2 participants selected')).toBeInTheDocument()
    })
  })

  describe('Validation', () => {
    it('disables create button when title is empty', async () => {
      const user = userEvent.setup()
      renderWithProviders(
        <NewConversationModal
          gameId={mockGameId}
          characters={userCharacters}
          allCharacters={allCharacters}
          isAnonymous={false}
          allowGroupConversations={true}
          onClose={mockOnClose}
          onConversationCreated={mockOnConversationCreated}
        />
      )

      // Select character
      const select = screen.getByRole('combobox')
      await user.selectOptions(select, '1')

      // Select participant
      const charlieCheckbox = screen.getByRole('checkbox', { name: /Charlie/i })
      await user.click(charlieCheckbox)

      // Button should still be disabled without title
      const createButton = screen.getByRole('button', { name: 'Create Conversation' })
      expect(createButton).toBeDisabled()

      expect(mockOnConversationCreated).not.toHaveBeenCalled()
    })

    it('disables create button when your character is not selected', async () => {
      const user = userEvent.setup()
      renderWithProviders(
        <NewConversationModal
          gameId={mockGameId}
          characters={userCharacters}
          allCharacters={allCharacters}
          isAnonymous={false}
          allowGroupConversations={true}
          onClose={mockOnClose}
          onConversationCreated={mockOnConversationCreated}
        />
      )

      // Enter title
      const titleInput = screen.getByPlaceholderText('e.g., Planning the heist')
      await user.type(titleInput, 'Secret Meeting')

      // Select participant
      const charlieCheckbox = screen.getByRole('checkbox', { name: /Charlie/i })
      await user.click(charlieCheckbox)

      // Button should still be disabled without character selection
      const createButton = screen.getByRole('button', { name: 'Create Conversation' })
      expect(createButton).toBeDisabled()

      expect(mockOnConversationCreated).not.toHaveBeenCalled()
    })

    it('disables create button when no participants are selected', async () => {
      const user = userEvent.setup()
      renderWithProviders(
        <NewConversationModal
          gameId={mockGameId}
          characters={userCharacters}
          allCharacters={allCharacters}
          isAnonymous={false}
          allowGroupConversations={true}
          onClose={mockOnClose}
          onConversationCreated={mockOnConversationCreated}
        />
      )

      // Enter title
      const titleInput = screen.getByPlaceholderText('e.g., Planning the heist')
      await user.type(titleInput, 'Secret Meeting')

      // Select your character
      const select = screen.getByRole('combobox')
      await user.selectOptions(select, '1')

      // Button should still be disabled without participant selection
      const createButton = screen.getByRole('button', { name: 'Create Conversation' })
      expect(createButton).toBeDisabled()

      expect(mockOnConversationCreated).not.toHaveBeenCalled()
    })

    it('disables create button when title is only whitespace', async () => {
      const user = userEvent.setup()
      renderWithProviders(
        <NewConversationModal
          gameId={mockGameId}
          characters={userCharacters}
          allCharacters={allCharacters}
          isAnonymous={false}
          allowGroupConversations={true}
          onClose={mockOnClose}
          onConversationCreated={mockOnConversationCreated}
        />
      )

      // Enter only whitespace as title
      const titleInput = screen.getByPlaceholderText('e.g., Planning the heist')
      await user.type(titleInput, '   ')

      // Select your character
      const select = screen.getByRole('combobox')
      await user.selectOptions(select, '1')

      // Select participant
      const charlieCheckbox = screen.getByRole('checkbox', { name: /Charlie/i })
      await user.click(charlieCheckbox)

      // Button should be disabled due to whitespace-only title
      const createButton = screen.getByRole('button', { name: 'Create Conversation' })
      expect(createButton).toBeDisabled()
    })
  })

  describe('Form Submission', () => {
    it('successfully creates conversation and calls callbacks', async () => {
      const user = userEvent.setup()
      renderWithProviders(
        <NewConversationModal
          gameId={mockGameId}
          characters={userCharacters}
          allCharacters={allCharacters}
          isAnonymous={false}
          allowGroupConversations={true}
          onClose={mockOnClose}
          onConversationCreated={mockOnConversationCreated}
        />
      )

      // Fill in all required fields
      const titleInput = screen.getByPlaceholderText('e.g., Planning the heist')
      await user.type(titleInput, 'Secret Meeting')

      const select = screen.getByRole('combobox')
      await user.selectOptions(select, '1')

      const charlieCheckbox = screen.getByRole('checkbox', { name: /Charlie/i })
      await user.click(charlieCheckbox)

      // Submit
      const createButton = screen.getByRole('button', { name: 'Create Conversation' })
      await user.click(createButton)

      await waitFor(() => {
        expect(mockOnConversationCreated).toHaveBeenCalledWith(100)
        expect(mockOnClose).toHaveBeenCalled()
      })
    })

    it('sends correct data to API including your character and participants', async () => {
      const user = userEvent.setup()
      let requestBody: unknown = null

      server.use(
        http.post('/api/v1/games/:gameId/conversations', async ({ request }) => {
          requestBody = await request.json()
          return HttpResponse.json({
            id: 100,
            game_id: mockGameId,
            title: 'Secret Meeting',
            conversation_type: 'group',
            created_by_user_id: 1,
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString(),
          }, { status: 201 })
        })
      )

      renderWithProviders(
        <NewConversationModal
          gameId={mockGameId}
          characters={userCharacters}
          allCharacters={allCharacters}
          isAnonymous={false}
          allowGroupConversations={true}
          onClose={mockOnClose}
          onConversationCreated={mockOnConversationCreated}
        />
      )

      // Fill in all required fields
      const titleInput = screen.getByPlaceholderText('e.g., Planning the heist')
      await user.type(titleInput, 'Secret Meeting')

      const select = screen.getByRole('combobox')
      await user.selectOptions(select, '1') // Select Alice (id: 1)

      const charlieCheckbox = screen.getByRole('checkbox', { name: /Charlie/i }) // Charlie (id: 3)
      await user.click(charlieCheckbox)

      // Submit
      const createButton = screen.getByRole('button', { name: 'Create Conversation' })
      await user.click(createButton)

      await waitFor(() => {
        expect(requestBody).toEqual({
          title: 'Secret Meeting',
          character_ids: [1, 3], // Your character + selected participant
        })
      })
    })

    it('handles multiple participants correctly', async () => {
      const user = userEvent.setup()
      let requestBody: unknown = null

      server.use(
        http.post('/api/v1/games/:gameId/conversations', async ({ request }) => {
          requestBody = await request.json()
          return HttpResponse.json({
            id: 100,
            game_id: mockGameId,
            title: 'Group Chat',
            conversation_type: 'group',
            created_by_user_id: 1,
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString(),
          }, { status: 201 })
        })
      )

      renderWithProviders(
        <NewConversationModal
          gameId={mockGameId}
          characters={userCharacters}
          allCharacters={allCharacters}
          isAnonymous={false}
          allowGroupConversations={true}
          onClose={mockOnClose}
          onConversationCreated={mockOnConversationCreated}
        />
      )

      // Fill in all required fields
      const titleInput = screen.getByPlaceholderText('e.g., Planning the heist')
      await user.type(titleInput, 'Group Chat')

      const select = screen.getByRole('combobox')
      await user.selectOptions(select, '2') // Select Bob (id: 2)

      // Select multiple participants
      const charlieCheckbox = screen.getByRole('checkbox', { name: /Charlie/i }) // Charlie (id: 3)
      const npcCheckbox = screen.getByRole('checkbox', { name: /The Mysterious NPC/i }) // NPC (id: 4)
      await user.click(charlieCheckbox)
      await user.click(npcCheckbox)

      // Submit
      const createButton = screen.getByRole('button', { name: 'Create Conversation' })
      await user.click(createButton)

      await waitFor(() => {
        expect(requestBody).toEqual({
          title: 'Group Chat',
          character_ids: [2, 3, 4], // Your character + both selected participants
        })
      })
    })

    it('does not call callbacks when API errors occur during conversation creation', async () => {
      const user = userEvent.setup()

      server.use(
        http.post('/api/v1/games/:gameId/conversations', () => {
          return HttpResponse.json(
            { detail: 'Failed to create conversation' },
            { status: 500 }
          )
        })
      )

      renderWithProviders(
        <NewConversationModal
          gameId={mockGameId}
          characters={userCharacters}
          allCharacters={allCharacters}
          isAnonymous={false}
          allowGroupConversations={true}
          onClose={mockOnClose}
          onConversationCreated={mockOnConversationCreated}
        />
      )

      // Fill in all required fields
      const titleInput = screen.getByPlaceholderText('e.g., Planning the heist')
      await user.type(titleInput, 'Secret Meeting')

      const select = screen.getByRole('combobox')
      await user.selectOptions(select, '1')

      const charlieCheckbox = screen.getByRole('checkbox', { name: /Charlie/i })
      await user.click(charlieCheckbox)

      // Submit
      const createButton = screen.getByRole('button', { name: 'Create Conversation' })
      await user.click(createButton)

      // Wait for button to be re-enabled (indicating the request completed)
      await waitFor(() => {
        expect(createButton).not.toHaveTextContent('Creating...')
      }, { timeout: 1000 })

      // Callbacks should not have been called due to error
      expect(mockOnConversationCreated).not.toHaveBeenCalled()
      expect(mockOnClose).not.toHaveBeenCalled()
    })
  })

  describe('Close Behavior', () => {
    it('calls onClose when close button is clicked', async () => {
      const user = userEvent.setup()
      renderWithProviders(
        <NewConversationModal
          gameId={mockGameId}
          characters={userCharacters}
          allCharacters={allCharacters}
          isAnonymous={false}
          allowGroupConversations={true}
          onClose={mockOnClose}
          onConversationCreated={mockOnConversationCreated}
        />
      )

      // The close button is the button with just an SVG (no text)
      const buttons = screen.getAllByRole('button')
      const closeButton = buttons.find(btn => btn.querySelector('svg') && !btn.textContent)!
      await user.click(closeButton)

      expect(mockOnClose).toHaveBeenCalledTimes(1)
    })

    it('calls onClose when cancel button is clicked', async () => {
      const user = userEvent.setup()
      renderWithProviders(
        <NewConversationModal
          gameId={mockGameId}
          characters={userCharacters}
          allCharacters={allCharacters}
          isAnonymous={false}
          allowGroupConversations={true}
          onClose={mockOnClose}
          onConversationCreated={mockOnConversationCreated}
        />
      )

      const cancelButton = screen.getByRole('button', { name: 'Cancel' })
      await user.click(cancelButton)

      expect(mockOnClose).toHaveBeenCalledTimes(1)
    })
  })

  describe('Loading States', () => {
    it('shows loading state while creating conversation', async () => {
      const user = userEvent.setup()

      // Delay the response to see loading state
      server.use(
        http.post('/api/v1/games/:gameId/conversations', async () => {
          await new Promise(resolve => setTimeout(resolve, 100))
          return HttpResponse.json({
            id: 100,
            game_id: mockGameId,
            title: 'Secret Meeting',
            conversation_type: 'group',
            created_by_user_id: 1,
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString(),
          }, { status: 201 })
        })
      )

      renderWithProviders(
        <NewConversationModal
          gameId={mockGameId}
          characters={userCharacters}
          allCharacters={allCharacters}
          isAnonymous={false}
          allowGroupConversations={true}
          onClose={mockOnClose}
          onConversationCreated={mockOnConversationCreated}
        />
      )

      // Fill in all required fields
      const titleInput = screen.getByPlaceholderText('e.g., Planning the heist')
      await user.type(titleInput, 'Secret Meeting')

      const select = screen.getByRole('combobox')
      await user.selectOptions(select, '1')

      const charlieCheckbox = screen.getByRole('checkbox', { name: /Charlie/i })
      await user.click(charlieCheckbox)

      // Submit
      const createButton = screen.getByRole('button', { name: 'Create Conversation' })
      await user.click(createButton)

      // Should show loading text
      expect(screen.getByText('Creating...')).toBeInTheDocument()

      // Wait for completion
      await waitFor(() => {
        expect(mockOnConversationCreated).toHaveBeenCalled()
      })
    })

    it('disables form fields while creating conversation', async () => {
      const user = userEvent.setup()

      // Delay the response
      server.use(
        http.post('/api/v1/games/:gameId/conversations', async () => {
          await new Promise(resolve => setTimeout(resolve, 100))
          return HttpResponse.json({
            id: 100,
            game_id: mockGameId,
            title: 'Secret Meeting',
            conversation_type: 'group',
            created_by_user_id: 1,
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString(),
          }, { status: 201 })
        })
      )

      renderWithProviders(
        <NewConversationModal
          gameId={mockGameId}
          characters={userCharacters}
          allCharacters={allCharacters}
          isAnonymous={false}
          allowGroupConversations={true}
          onClose={mockOnClose}
          onConversationCreated={mockOnConversationCreated}
        />
      )

      // Fill in all required fields
      const titleInput = screen.getByPlaceholderText('e.g., Planning the heist') as HTMLInputElement
      await user.type(titleInput, 'Secret Meeting')

      const select = screen.getByRole('combobox') as HTMLSelectElement
      await user.selectOptions(select, '1')

      const charlieCheckbox = screen.getByRole('checkbox', { name: /Charlie/i }) as HTMLInputElement
      await user.click(charlieCheckbox)

      // Submit
      const createButton = screen.getByRole('button', { name: 'Create Conversation' })
      await user.click(createButton)

      // Fields should be disabled during submission
      expect(titleInput).toBeDisabled()
      expect(select).toBeDisabled()
      expect(screen.getByRole('button', { name: 'Cancel' })).toBeDisabled()
      expect(screen.getByRole('button', { name: 'Creating...' })).toBeDisabled()

      // Wait for completion
      await waitFor(() => {
        expect(mockOnConversationCreated).toHaveBeenCalled()
      })
    })

    it('disables create button when form is incomplete', () => {
      renderWithProviders(
        <NewConversationModal
          gameId={mockGameId}
          characters={userCharacters}
          allCharacters={allCharacters}
          isAnonymous={false}
          allowGroupConversations={true}
          onClose={mockOnClose}
          onConversationCreated={mockOnConversationCreated}
        />
      )

      const createButton = screen.getByRole('button', { name: 'Create Conversation' })

      // Should be disabled when form is empty
      expect(createButton).toBeDisabled()
    })

    it('enables create button when all required fields are filled', async () => {
      const user = userEvent.setup()
      renderWithProviders(
        <NewConversationModal
          gameId={mockGameId}
          characters={userCharacters}
          allCharacters={allCharacters}
          isAnonymous={false}
          allowGroupConversations={true}
          onClose={mockOnClose}
          onConversationCreated={mockOnConversationCreated}
        />
      )

      const createButton = screen.getByRole('button', { name: 'Create Conversation' })
      expect(createButton).toBeDisabled()

      // Fill in all required fields
      const titleInput = screen.getByPlaceholderText('e.g., Planning the heist')
      await user.type(titleInput, 'Secret Meeting')

      const select = screen.getByRole('combobox')
      await user.selectOptions(select, '1')

      const charlieCheckbox = screen.getByRole('checkbox', { name: /Charlie/i })
      await user.click(charlieCheckbox)

      // Now button should be enabled
      expect(createButton).toBeEnabled()
    })

    it('disables create button when user has no characters', () => {
      renderWithProviders(
        <NewConversationModal
          gameId={mockGameId}
          characters={[]}
          allCharacters={allCharacters}
          isAnonymous={false}
          allowGroupConversations={true}
          onClose={mockOnClose}
          onConversationCreated={mockOnConversationCreated}
        />
      )

      const createButton = screen.getByRole('button', { name: 'Create Conversation' })
      expect(createButton).toBeDisabled()
    })
  })

  describe('Character Filtering', () => {
    it('filters out user-controlled characters from participant list when selected as sender', async () => {
      const user = userEvent.setup()
      renderWithProviders(
        <NewConversationModal
          gameId={mockGameId}
          characters={userCharacters}
          allCharacters={allCharacters}
          isAnonymous={false}
          allowGroupConversations={true}
          onClose={mockOnClose}
          onConversationCreated={mockOnConversationCreated}
        />
      )

      // Select Alice as "your character"
      const select = screen.getByRole('combobox')
      await user.selectOptions(select, '1') // Select Alice (id: 1)

      // Alice should not appear in participant checkboxes
      const checkboxes = screen.getAllByRole('checkbox')
      checkboxes.forEach(checkbox => {
        const container = checkbox.parentElement
        if (container) {
          const containerText = container.textContent || ''
          expect(containerText).not.toContain('Alice')
        }
      })
    })

    it('displays all character types in participant list (PCs and NPCs)', () => {
      renderWithProviders(
        <NewConversationModal
          gameId={mockGameId}
          characters={userCharacters}
          allCharacters={allCharacters}
          isAnonymous={false}
          allowGroupConversations={true}
          onClose={mockOnClose}
          onConversationCreated={mockOnConversationCreated}
        />
      )

      // Should show both player character and NPC
      expect(screen.getByText('Charlie')).toBeInTheDocument() // player_character
      expect(screen.getByText('The Mysterious NPC')).toBeInTheDocument() // npc
    })

    it('shows character type for each participant', () => {
      renderWithProviders(
        <NewConversationModal
          gameId={mockGameId}
          characters={userCharacters}
          allCharacters={allCharacters}
          isAnonymous={false}
          allowGroupConversations={true}
          onClose={mockOnClose}
          onConversationCreated={mockOnConversationCreated}
        />
      )

      // Check that character types are displayed (with underscore replaced)
      const participantsList = screen.getAllByRole('checkbox')[0].closest('div[class*="space-y-2"]')
      expect(participantsList).toHaveTextContent('player character')
      expect(participantsList).toHaveTextContent('npc')
    })
  })

  describe('Group Conversations Disabled (allowGroupConversations=false)', () => {
    it('renders a single Select dropdown instead of checkboxes', () => {
      renderWithProviders(
        <NewConversationModal
          gameId={mockGameId}
          characters={userCharacters}
          allCharacters={allCharacters}
          isAnonymous={false}
          allowGroupConversations={false}
          onClose={mockOnClose}
          onConversationCreated={mockOnConversationCreated}
        />
      )

      // Should not render any participant checkboxes
      expect(screen.queryAllByRole('checkbox')).toHaveLength(0)

      // The participant dropdown should contain the available characters as options
      // (Charlie and NPC are available; Alice/Bob are in userCharacters and one will be auto-selected as sender)
      expect(screen.getByRole('option', { name: /Charlie/i })).toBeInTheDocument()
      expect(screen.getByRole('option', { name: /The Mysterious NPC/i })).toBeInTheDocument()
    })

    it('shows participant label for the single select', () => {
      renderWithProviders(
        <NewConversationModal
          gameId={mockGameId}
          characters={userCharacters}
          allCharacters={allCharacters}
          isAnonymous={false}
          allowGroupConversations={false}
          onClose={mockOnClose}
          onConversationCreated={mockOnConversationCreated}
        />
      )

      expect(screen.getByText(/Participant \*/i)).toBeInTheDocument()
    })

    it('disables create button when no participant is selected', async () => {
      const user = userEvent.setup()
      renderWithProviders(
        <NewConversationModal
          gameId={mockGameId}
          characters={[userCharacters[0]]}
          allCharacters={allCharacters}
          isAnonymous={false}
          allowGroupConversations={false}
          onClose={mockOnClose}
          onConversationCreated={mockOnConversationCreated}
        />
      )

      const titleInput = screen.getByPlaceholderText('e.g., Planning the heist')
      await user.type(titleInput, 'Direct Message')

      const createButton = screen.getByRole('button', { name: 'Create Conversation' })
      expect(createButton).toBeDisabled()
    })

    it('enables create button after selecting a single participant', async () => {
      const user = userEvent.setup()
      renderWithProviders(
        <NewConversationModal
          gameId={mockGameId}
          characters={[userCharacters[0]]}
          allCharacters={allCharacters}
          isAnonymous={false}
          allowGroupConversations={false}
          onClose={mockOnClose}
          onConversationCreated={mockOnConversationCreated}
        />
      )

      const titleInput = screen.getByPlaceholderText('e.g., Planning the heist')
      await user.type(titleInput, 'Direct Message')

      // Select a participant from the dropdown (the participant select, not the character select)
      const selects = screen.getAllByRole('combobox')
      const participantSelect = selects[selects.length - 1]
      await user.selectOptions(participantSelect, String(allCharacters[2].id)) // Charlie

      const createButton = screen.getByRole('button', { name: 'Create Conversation' })
      expect(createButton).toBeEnabled()
    })

    it('sends only two character IDs when group conversations are disabled', async () => {
      const user = userEvent.setup()
      let requestBody: unknown = null

      server.use(
        http.post('/api/v1/games/:gameId/conversations', async ({ request }) => {
          requestBody = await request.json()
          return HttpResponse.json({
            id: 101,
            game_id: mockGameId,
            title: 'Direct Message',
            conversation_type: 'direct',
            created_by_user_id: 1,
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString(),
          }, { status: 201 })
        })
      )

      renderWithProviders(
        <NewConversationModal
          gameId={mockGameId}
          characters={[userCharacters[0]]}
          allCharacters={allCharacters}
          isAnonymous={false}
          allowGroupConversations={false}
          onClose={mockOnClose}
          onConversationCreated={mockOnConversationCreated}
        />
      )

      const titleInput = screen.getByPlaceholderText('e.g., Planning the heist')
      await user.type(titleInput, 'Direct Message')

      const selects = screen.getAllByRole('combobox')
      const participantSelect = selects[selects.length - 1]
      await user.selectOptions(participantSelect, '3') // Charlie (id: 3)

      const createButton = screen.getByRole('button', { name: 'Create Conversation' })
      await user.click(createButton)

      await waitFor(() => {
        expect(requestBody).toEqual({
          title: 'Direct Message',
          character_ids: [1, 3], // Exactly 2: your character + 1 participant
        })
      })
    })
  })
})
