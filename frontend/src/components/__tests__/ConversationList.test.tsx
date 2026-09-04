import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen, fireEvent, waitFor } from '@testing-library/react'
import { renderWithProviders } from '../../test-utils'
import { ConversationList } from '../ConversationList'
import { http, HttpResponse } from 'msw'
import { server } from '../../mocks/server'
import type { useAuth } from '../../contexts/AuthContext'
import type { ConversationListItem } from '../../types/conversations'

// Mock the auth hook
vi.mock('../../hooks/useAuth', () => ({
  useAuth: vi.fn(),
}))

import { useAuth } from '../../hooks/useAuth'

describe('ConversationList', () => {
  const mockOnSelectConversation = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()

    // Set up authenticated user
    vi.mocked(useAuth).mockReturnValue({
      currentUser: { id: 1, username: 'testuser', email: 'test@example.com', created_at: '', updated_at: '' },
      isAuthenticated: true,
      isCheckingAuth: false,
      isLoading: false,
      login: vi.fn(),
      register: vi.fn(),
      logout: vi.fn(),
      error: null,
    } as Partial<ReturnType<typeof useAuth>>)

    // Mock localStorage
    const mockLocalStorage = {
      getItem: vi.fn((key: string) => key === 'auth_token' ? 'mock-token' : null),
      setItem: vi.fn(),
      removeItem: vi.fn(),
      clear: vi.fn(),
      length: 0,
      key: vi.fn(),
    }
    Object.defineProperty(window, 'localStorage', {
      value: mockLocalStorage,
      writable: true,
    })
  })

  describe('Conversation Deduplication Bug - Regression Test', () => {
    it('should deduplicate conversations when user has multiple characters in same conversation', async () => {
      // CONTEXT: If a user has multiple characters (e.g., "Character A" and "Character B")
      // participating in the same conversation, the backend API returns the same conversation
      // twice (once for each character). The frontend must deduplicate these by conversation ID.

      const duplicatedConversation: ConversationListItem = {
        id: 1,
        title: 'Shared Conversation',
        game_id: 123,
        conversation_type: 'group',
        participant_count: 3,
        participant_names: 'Alice, Bob, Charlie',
        last_message: 'Hello everyone!',
        last_message_at: '2025-01-15T10:00:00Z',
        unread_count: 2,
        created_at: '2025-01-01T00:00:00Z',
      }

      // Simulate backend returning the same conversation twice
      const apiResponse = {
        conversations: [
          duplicatedConversation,  // First entry (via Character A)
          duplicatedConversation,  // Duplicate entry (via Character B)
        ]
      }

      // Mock API to return duplicated conversations
      server.use(
        http.get('http://localhost:3000/api/v1/auth/me', () => {
          return HttpResponse.json({
            id: 1,
            username: 'testuser',
            email: 'test@example.com',
            created_at: '',
            updated_at: '',
          })
        }),
        http.get('http://localhost:3000/api/v1/games/:gameId/conversations', () => {
          return HttpResponse.json(apiResponse)
        })
      )

      renderWithProviders(
        <ConversationList
          gameId={123}
          onSelectConversation={mockOnSelectConversation}
        />
      )

      // Wait for conversations to load
      await waitFor(() => {
        expect(screen.getAllByText('Shared Conversation')[0]).toBeInTheDocument()
      })

      // CRITICAL: Should only render ONE conversation, not two duplicates
      const conversationButtons = screen.getAllByRole('link')
      expect(conversationButtons).toHaveLength(1)

      // Verify the conversation has the correct data
      expect(screen.getAllByText('Shared Conversation')[0]).toBeInTheDocument()
      expect(screen.getAllByText('Alice, Bob, Charlie')[0]).toBeInTheDocument()
      expect(screen.getAllByText('Hello everyone!')[0]).toBeInTheDocument()
    })

    it('should handle multiple distinct conversations correctly', async () => {
      const conversation1: ConversationListItem = {
        id: 1,
        title: 'Conversation 1',
        game_id: 123,
        conversation_type: 'direct',
        participant_count: 2,
        participant_names: 'Alice, Bob',
        last_message: 'First message',
        last_message_at: '2025-01-15T10:00:00Z',
        unread_count: 1,
        created_at: '2025-01-01T00:00:00Z',
      }

      const conversation2: ConversationListItem = {
        id: 2,
        title: 'Conversation 2',
        game_id: 123,
        conversation_type: 'group',
        participant_count: 3,
        participant_names: 'Alice, Bob, Charlie',
        last_message: 'Second message',
        last_message_at: '2025-01-15T11:00:00Z',
        unread_count: 2,
        created_at: '2025-01-01T00:00:00Z',
      }

      const apiResponse = {
        conversations: [conversation1, conversation2]
      }

      server.use(
        http.get('http://localhost:3000/api/v1/auth/me', () => {
          return HttpResponse.json({
            id: 1,
            username: 'testuser',
            email: 'test@example.com',
            created_at: '',
            updated_at: '',
          })
        }),
        http.get('http://localhost:3000/api/v1/games/:gameId/conversations', () => {
          return HttpResponse.json(apiResponse)
        })
      )

      renderWithProviders(
        <ConversationList
          gameId={123}
          onSelectConversation={mockOnSelectConversation}
        />
      )

      await waitFor(() => {
        expect(screen.getAllByText('Conversation 1')[0]).toBeInTheDocument()
      })

      // Should render both distinct conversations
      const conversationButtons = screen.getAllByRole('link')
      expect(conversationButtons).toHaveLength(2)
      expect(screen.getAllByText('Conversation 1')[0]).toBeInTheDocument()
      expect(screen.getAllByText('Conversation 2')[0]).toBeInTheDocument()
    })

    it('should deduplicate mixed duplicates and distinct conversations', async () => {
      // Realistic scenario: 3 distinct conversations, but one appears twice
      const conversation1: ConversationListItem = {
        id: 1,
        title: 'Conversation 1',
        game_id: 123,
        conversation_type: 'direct',
        participant_count: 2,
        participant_names: 'Alice, Bob',
        last_message: 'Message 1',
        last_message_at: '2025-01-15T10:00:00Z',
        unread_count: 0,
        created_at: '2025-01-01T00:00:00Z',
      }

      const conversation2: ConversationListItem = {
        id: 2,
        title: 'Duplicated Conversation',
        game_id: 123,
        conversation_type: 'group',
        participant_count: 3,
        participant_names: 'Alice, Bob, Charlie',
        last_message: 'Message 2',
        last_message_at: '2025-01-15T11:00:00Z',
        unread_count: 1,
        created_at: '2025-01-01T00:00:00Z',
      }

      const conversation3: ConversationListItem = {
        id: 3,
        title: 'Conversation 3',
        game_id: 123,
        conversation_type: 'direct',
        participant_count: 2,
        participant_names: 'Alice, Dave',
        last_message: 'Message 3',
        last_message_at: '2025-01-15T12:00:00Z',
        unread_count: 0,
        created_at: '2025-01-01T00:00:00Z',
      }

      // API returns: [conv1, conv2, conv2 (duplicate), conv3]
      const apiResponse = {
        conversations: [conversation1, conversation2, conversation2, conversation3]
      }

      server.use(
        http.get('http://localhost:3000/api/v1/auth/me', () => {
          return HttpResponse.json({
            id: 1,
            username: 'testuser',
            email: 'test@example.com',
            created_at: '',
            updated_at: '',
          })
        }),
        http.get('http://localhost:3000/api/v1/games/:gameId/conversations', () => {
          return HttpResponse.json(apiResponse)
        })
      )

      renderWithProviders(
        <ConversationList
          gameId={123}
          onSelectConversation={mockOnSelectConversation}
        />
      )

      await waitFor(() => {
        expect(screen.getAllByText('Conversation 1')[0]).toBeInTheDocument()
      })

      // Should render only 3 distinct conversations, not 4
      const conversationButtons = screen.getAllByRole('link')
      expect(conversationButtons).toHaveLength(3)
    })

    it('should handle empty conversations list', async () => {
      server.use(
        http.get('http://localhost:3000/api/v1/auth/me', () => {
          return HttpResponse.json({
            id: 1,
            username: 'testuser',
            email: 'test@example.com',
            created_at: '',
            updated_at: '',
          })
        }),
        http.get('http://localhost:3000/api/v1/games/:gameId/conversations', () => {
          return HttpResponse.json({ conversations: [] })
        })
      )

      renderWithProviders(
        <ConversationList
          gameId={123}
          onSelectConversation={mockOnSelectConversation}
        />
      )

      await waitFor(() => {
        expect(screen.getByText('No conversations yet')).toBeInTheDocument()
      })

      expect(screen.queryByRole('link')).not.toBeInTheDocument()
    })
  })

  describe('Basic functionality', () => {
    const singleConversation: ConversationListItem = {
      id: 1,
      title: 'Test Conversation',
      game_id: 123,
      conversation_type: 'direct',
      participant_count: 2,
      participant_names: 'Alice, Bob',
      last_message: 'Test message',
      last_message_at: '2025-01-15T10:00:00Z',
      unread_count: 1,
      created_at: '2025-01-01T00:00:00Z',
    }

    it('should render loading state initially', () => {
      server.use(
        http.get('http://localhost:3000/api/v1/games/:gameId/conversations', async () => {
          // Delay response to test loading state
          await new Promise(resolve => setTimeout(resolve, 100))
          return HttpResponse.json({ conversations: [] })
        })
      )

      renderWithProviders(
        <ConversationList
          gameId={123}
          onSelectConversation={mockOnSelectConversation}
        />
      )

      expect(screen.getByText('Loading conversations...')).toBeInTheDocument()
    })

    it('should call onSelectConversation when clicking a conversation', async () => {
      server.use(
        http.get('http://localhost:3000/api/v1/auth/me', () => {
          return HttpResponse.json({
            id: 1,
            username: 'testuser',
            email: 'test@example.com',
            created_at: '',
            updated_at: '',
          })
        }),
        http.get('http://localhost:3000/api/v1/games/:gameId/conversations', () => {
          return HttpResponse.json({ conversations: [singleConversation] })
        })
      )

      renderWithProviders(
        <ConversationList
          gameId={123}
          onSelectConversation={mockOnSelectConversation}
        />
      )

      await waitFor(() => {
        expect(screen.getAllByText('Test Conversation')[0]).toBeInTheDocument()
      })

      const conversationButton = screen.getByRole('link')
      fireEvent.click(conversationButton)

      expect(mockOnSelectConversation).toHaveBeenCalledWith(1)
    })

    it('should highlight selected conversation', async () => {
      server.use(
        http.get('http://localhost:3000/api/v1/auth/me', () => {
          return HttpResponse.json({
            id: 1,
            username: 'testuser',
            email: 'test@example.com',
            created_at: '',
            updated_at: '',
          })
        }),
        http.get('http://localhost:3000/api/v1/games/:gameId/conversations', () => {
          return HttpResponse.json({ conversations: [singleConversation] })
        })
      )

      renderWithProviders(
        <ConversationList
          gameId={123}
          onSelectConversation={mockOnSelectConversation}
          selectedConversationId={1}
        />
      )

      await waitFor(() => {
        expect(screen.getAllByText('Test Conversation')[0]).toBeInTheDocument()
      })

      const conversationButton = screen.getByRole('link')
      expect(conversationButton).toHaveClass('bg-interactive-primary-subtle', 'border-l-4', 'border-l-interactive-primary')
    })

    it('should handle API errors gracefully', async () => {
      server.use(
        http.get('http://localhost:3000/api/v1/games/:gameId/conversations', () => {
          return HttpResponse.json(
            { detail: 'Internal server error' },
            { status: 500 }
          )
        })
      )

      renderWithProviders(
        <ConversationList
          gameId={123}
          onSelectConversation={mockOnSelectConversation}
        />
      )

      // Error handling is now done via toast notifications in the context
      // The component should show the empty state when there are no conversations
      await waitFor(() => {
        expect(screen.getByText('No conversations yet')).toBeInTheDocument()
      })
    })
  })
})
