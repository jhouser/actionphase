import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server';
import { renderWithProviders } from '../../test-utils/render';
import type { useAuth } from '../../contexts/AuthContext';
import { MessageThread } from '../MessageThread';
import type { Character } from '../../types/characters';

// Mock the auth hook
vi.mock('../../contexts/AuthContext', () => ({
  useAuth: vi.fn(),
  AuthProvider: ({ children }: { children: React.ReactNode }) => children,
}));

import { useAuth } from '../../contexts/AuthContext'

describe('MessageThread', () => {
  const mockCharacters: Character[] = [
    {
      id: 1,
      game_id: 1,
      name: 'Hero Character',
      character_type: 'player_character',
      user_id: 100,
      status: 'approved',
      created_at: '2024-01-01T00:00:00Z',
    },
    {
      id: 2,
      game_id: 1,
      name: 'Companion Character',
      character_type: 'player_character',
      user_id: 100,
      status: 'approved',
      created_at: '2024-01-01T00:00:00Z',
    },
  ];

  const mockConversation = {
    conversation: {
      id: 1,
      game_id: 1,
      title: 'Test Conversation',
      created_at: '2024-01-01T00:00:00Z',
    },
    participants: [
      { character_id: 1, character_name: 'Hero Character', username: 'player1' },
      { character_id: 2, character_name: 'Companion Character', username: 'player1' },
    ],
  };

  const mockMessages = [
    {
      id: 1,
      conversation_id: 1,
      sender_character_id: 1,
      sender_character_name: 'Hero Character',
      sender_username: 'player1',
      content: 'Hello! This is the first message.',
      created_at: '2024-01-01T10:00:00Z',
    },
    {
      id: 2,
      conversation_id: 1,
      sender_character_id: 2,
      sender_character_name: 'Companion Character',
      sender_username: 'player1',
      content: 'This is a reply with **markdown** formatting!',
      created_at: '2024-01-01T10:05:00Z',
    },
  ];

  beforeEach(() => {
    vi.clearAllMocks();

    // Set up authenticated user
    vi.mocked(useAuth).mockReturnValue({
      currentUser: { id: 100, username: 'player1', email: 'player1@example.com', created_at: '', updated_at: '' },
      isAuthenticated: true,
      isCheckingAuth: false,
      isLoading: false,
      login: vi.fn(),
      register: vi.fn(),
      logout: vi.fn(),
      error: null,
    } as Partial<ReturnType<typeof useAuth>>);

    // Setup default mocks
    server.use(
      http.get('/api/v1/games/:gameId/conversations/:conversationId', () => {
        return HttpResponse.json(mockConversation);
      }),
      http.get('/api/v1/games/:gameId/conversations/:conversationId/messages', () => {
        return HttpResponse.json({ messages: mockMessages });
      }),
      http.post('/api/v1/games/:gameId/conversations/:conversationId/read', () => {
        return HttpResponse.json({ success: true });
      })
    );
  });

  // The composer is collapsed behind a "Reply" button until the user opens it
  // (see MessageThread). Tests that interact with the textarea/character
  // select/Send button must open it first. Only valid when messaging is allowed
  // (common_room/interlude) — in other phases there is no Reply button and no
  // composer, just the phase-restriction alert.
  const openComposer = async (user: ReturnType<typeof userEvent.setup>) => {
    const replyButton = await screen.findByRole('button', { name: /^reply$/i });
    await user.click(replyButton);
    await screen.findByPlaceholderText(/type your message/i);
  };

  describe('Loading State', () => {
    it('shows loading indicator initially', () => {
      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      expect(screen.getByText(/loading messages/i)).toBeInTheDocument();
    });

    it('hides loading indicator after messages load', async () => {
      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      await waitFor(() => {
        expect(screen.queryByText(/loading messages/i)).not.toBeInTheDocument();
      });
    });
  });

  describe('Error Handling', () => {
    it('displays error when conversation fails to load', async () => {
      server.use(
        http.get('/api/v1/games/:gameId/conversations/:conversationId', () => {
          return HttpResponse.error();
        })
      );

      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      await waitFor(() => {
        expect(screen.getByText(/failed to load conversation/i)).toBeInTheDocument();
      });
    });

    it('displays error when messages fail to load', async () => {
      server.use(
        http.get('/api/v1/games/:gameId/conversations/:conversationId/messages', () => {
          return HttpResponse.error();
        })
      );

      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      await waitFor(() => {
        expect(screen.getByText(/failed to load messages/i)).toBeInTheDocument();
      });
    });
  });

  describe('Conversation Display', () => {
    it('displays conversation title', async () => {
      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      await waitFor(() => {
        expect(screen.getByText('Test Conversation')).toBeInTheDocument();
      });
    });

    it('displays participant list', async () => {
      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      await waitFor(() => {
        // Character names appear in the header participant line, selector, and messages
        const heroMatches = screen.getAllByText(/hero character/i);
        const companionMatches = screen.getAllByText(/companion character/i);
        expect(heroMatches.length).toBeGreaterThan(0);
        expect(companionMatches.length).toBeGreaterThan(0);
      });
    });

    it('deduplicates participant names when same character appears multiple times (e.g. GM + co-GM both added for NPC)', async () => {
      server.use(
        http.get('/api/v1/games/:gameId/conversations/:conversationId', () => {
          return HttpResponse.json({
            conversation: { id: 1, game_id: 1, title: 'Test Conversation', created_at: '2024-01-01T00:00:00Z' },
            participants: [
              { character_id: 10, user_id: 1, character_name: 'Captain Obed Marsh', username: 'TestGM' },
              { character_id: 10, user_id: 2, character_name: 'Captain Obed Marsh', username: 'TestCoGM' },
              { character_id: 20, user_id: 3, character_name: 'Detective Marcus Kane', username: 'TestPlayer1' },
            ],
          });
        })
      );

      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      await waitFor(() => {
        // The header participant line should show each character only once
        const header = screen.getByText(/captain obed marsh/i);
        // Should appear exactly once in header (not twice)
        expect(header.textContent).not.toMatch(/captain obed marsh.*captain obed marsh/i);
        expect(header.textContent).toMatch(/captain obed marsh/i);
        expect(header.textContent).toMatch(/detective marcus kane/i);
      });
    });

    it('shows untitled when conversation has no title', async () => {
      server.use(
        http.get('/api/v1/games/:gameId/conversations/:conversationId', () => {
          return HttpResponse.json({
            conversation: { id: 1, game_id: 1, title: null, created_at: '2024-01-01T00:00:00Z' },
            participants: mockConversation.participants,
          });
        })
      );

      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      await waitFor(() => {
        expect(screen.getByText(/untitled conversation/i)).toBeInTheDocument();
      });
    });
  });

  describe('Message Display', () => {
    it('displays all messages', async () => {
      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      await waitFor(() => {
        expect(screen.getByText(/hello! this is the first message/i)).toBeInTheDocument();
        expect(screen.getByText(/this is a reply/i)).toBeInTheDocument();
      });
    });

    it('displays sender names', async () => {
      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      await waitFor(() => {
        const senders = screen.getAllByText(/hero character|companion character/i);
        expect(senders.length).toBeGreaterThan(0);
      });
    });

    it('displays timestamps', async () => {
      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      await waitFor(() => {
        // Timestamps will be formatted, just check some exist
        const timeElements = screen.getAllByText(/jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec/i);
        expect(timeElements.length).toBeGreaterThan(0);
      });
    });

    it('renders markdown in messages', async () => {
      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      await waitFor(() => {
        // Check for markdown bold formatting (rendered as <strong>)
        const boldText = screen.getByText('markdown');
        expect(boldText.tagName).toBe('STRONG');
      });
    });

    it('shows empty state when no messages', async () => {
      server.use(
        http.get('/api/v1/games/:gameId/conversations/:conversationId/messages', () => {
          return HttpResponse.json({ messages: [] });
        })
      );

      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      await waitFor(() => {
        expect(screen.getByText(/no messages yet/i)).toBeInTheDocument();
        expect(screen.getByText(/start the conversation/i)).toBeInTheDocument();
      });
    });
  });

  describe('Message Input', () => {
    it('shows message input when user has participating characters', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      await openComposer(user);

      expect(screen.getByPlaceholderText(/type your message/i)).toBeInTheDocument();
    });

    it('shows character selector when user has multiple participants', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      await openComposer(user);

      expect(screen.getByText(/send as hero character/i)).toBeInTheDocument();
      expect(screen.getByText(/send as companion character/i)).toBeInTheDocument();
    });

    it('hides character selector when user has only one participant', async () => {
      const user = userEvent.setup();
      const singleCharacter = [mockCharacters[0]];

      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={singleCharacter} currentPhaseType="common_room" />
      );

      await openComposer(user);

      expect(screen.queryByText(/send as/i)).not.toBeInTheDocument();
    });

    it('shows help text about keyboard shortcut', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      await openComposer(user);

      expect(screen.getByText(/press ctrl\/cmd \+ enter to send/i)).toBeInTheDocument();
    });

    it('shows message when user has no characters', async () => {
      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={[]} currentPhaseType="common_room" />
      );

      await waitFor(() => {
        expect(screen.getByText(/you need a character to send messages/i)).toBeInTheDocument();
      });
    });

    it('shows message when user has no participating characters', async () => {
      const nonParticipantCharacter: Character = {
        id: 99,
        game_id: 1,
        name: 'Non-Participant',
        character_type: 'player_character',
        user_id: 100,
        status: 'approved',
        created_at: '2024-01-01T00:00:00Z',
      };

      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={[nonParticipantCharacter]} currentPhaseType="common_room" />
      );

      await waitFor(() => {
        expect(screen.getByText(/you don't have any characters participating/i)).toBeInTheDocument();
      });
    });
  });

  describe('Sending Messages', () => {
    it('allows typing in message input', async () => {
      const user = userEvent.setup();

      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      await openComposer(user);

      const textarea = screen.getByPlaceholderText(/type your message/i);
      await user.type(textarea, 'Test message');

      expect(textarea).toHaveValue('Test message');
    });

    it('sends message when form is submitted', async () => {
      const user = userEvent.setup();
      let sentMessage: unknown;

      server.use(
        http.post('/api/v1/games/:gameId/conversations/:conversationId/messages', async ({ request }) => {
          sentMessage = await request.json();
          return HttpResponse.json({
            id: 3, ...sentMessage, created_at: new Date().toISOString()
          });
        })
      );

      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      await openComposer(user);

      const textarea = screen.getByPlaceholderText(/type your message/i);
      await user.type(textarea, 'New test message');

      const sendButton = screen.getByRole('button', { name: /send/i });
      await user.click(sendButton);

      await waitFor(() => {
        expect(sentMessage).toBeDefined();
        expect(sentMessage.content).toBe('New test message');
      });
    });

    it('clears input after sending message', async () => {
      const user = userEvent.setup();

      server.use(
        http.post('/api/v1/games/:gameId/conversations/:conversationId/messages', () => {
          return HttpResponse.json({ id: 3 });
        })
      );

      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      await openComposer(user);

      const textarea = screen.getByPlaceholderText(/type your message/i);
      await user.type(textarea, 'Message to clear');
      await user.click(screen.getByRole('button', { name: /^send$/i }));

      // After sending, the composer collapses back to the Reply button; the
      // draft is discarded (reopening yields an empty textarea).
      await screen.findByRole('button', { name: /^reply$/i });
      await openComposer(user);
      expect(screen.getByPlaceholderText(/type your message/i)).toHaveValue('');
    });

    it('shows sending state while message is being sent', async () => {
      const user = userEvent.setup();

      server.use(
        http.post('/api/v1/games/:gameId/conversations/:conversationId/messages', async () => {
          await new Promise(resolve => setTimeout(resolve, 100));
          return HttpResponse.json({ id: 3 });
        })
      );

      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      await openComposer(user);

      await user.type(screen.getByPlaceholderText(/type your message/i), 'Test');
      await user.click(screen.getByRole('button', { name: /^send$/i }));

      // The button shows "Sending..." while the POST is in flight (mock delays
      // 100ms). Poll for it rather than asserting synchronously, since the click
      // await may already have advanced past the initial render.
      await screen.findByText(/sending\.\.\./i);

      await waitFor(() => {
        expect(screen.queryByText(/sending\.\.\./i)).not.toBeInTheDocument();
      });
    });

    it('disables send button when message is empty', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      await openComposer(user);

      const sendButton = screen.getByRole('button', { name: /^send$/i });
      expect(sendButton).toBeDisabled();
    });

    it('disables send button when message is only whitespace', async () => {
      const user = userEvent.setup();

      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      await openComposer(user);

      const textarea = screen.getByPlaceholderText(/type your message/i);
      await user.type(textarea, '   '); // Only spaces

      const sendButton = screen.getByRole('button', { name: /^send$/i });
      expect(sendButton).toBeDisabled();
    });

    it('trims whitespace from message before sending', async () => {
      const user = userEvent.setup();
      let sentMessage: unknown;

      server.use(
        http.post('/api/v1/games/:gameId/conversations/:conversationId/messages', async ({ request }) => {
          sentMessage = await request.json();
          return HttpResponse.json({ id: 3 });
        })
      );

      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      await openComposer(user);

      await user.type(screen.getByPlaceholderText(/type your message/i), '  Trimmed message  ');
      await user.click(screen.getByRole('button', { name: /^send$/i }));

      await waitFor(() => {
        expect(sentMessage.content).toBe('Trimmed message');
      });
    });

    it('sends message with selected character ID', async () => {
      const user = userEvent.setup();
      let sentMessage: unknown;

      server.use(
        http.post('/api/v1/games/:gameId/conversations/:conversationId/messages', async ({ request }) => {
          sentMessage = await request.json();
          return HttpResponse.json({ id: 3 });
        })
      );

      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      await openComposer(user);

      await user.type(screen.getByPlaceholderText(/type your message/i), 'Test');
      await user.click(screen.getByRole('button', { name: /^send$/i }));

      await waitFor(() => {
        expect(sentMessage.character_id).toBe(1); // First character auto-selected
      });
    });

    it('allows switching character before sending', async () => {
      const user = userEvent.setup();
      let sentMessage: unknown;

      server.use(
        http.post('/api/v1/games/:gameId/conversations/:conversationId/messages', async ({ request }) => {
          sentMessage = await request.json();
          return HttpResponse.json({ id: 3 });
        })
      );

      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      await openComposer(user);

      // Select second character
      const select = screen.getByRole('combobox');
      await user.selectOptions(select, '2');

      await user.type(screen.getByPlaceholderText(/type your message/i), 'Test');
      await user.click(screen.getByRole('button', { name: /^send$/i }));

      await waitFor(() => {
        expect(sentMessage.character_id).toBe(2);
      });
    });
  });

  describe('Marks Conversation as Read', () => {
    it('marks conversation as read when messages are loaded', async () => {
      let markedAsRead = false;

      server.use(
        http.post('/api/v1/games/:gameId/conversations/:conversationId/read', () => {
          markedAsRead = true;
          return HttpResponse.json({ success: true });
        })
      );

      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      await waitFor(() => {
        expect(markedAsRead).toBe(true);
      });
    });
  });

  describe('Character Selection Logic', () => {
    it('auto-selects first participating character', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      await openComposer(user);

      const select = screen.getByRole('combobox') as HTMLSelectElement;
      expect(select.value).toBe('1'); // First character
    });

    it('filters characters to only show conversation participants', async () => {
      const mixedCharacters: Character[] = [
        ...mockCharacters,
        {
          id: 99,
          game_id: 1,
          name: 'Non-Participant',
          character_type: 'player_character',
          user_id: 100,
          status: 'approved',
          created_at: '2024-01-01T00:00:00Z',
        },
      ];

      const user = userEvent.setup();
      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mixedCharacters} currentPhaseType="common_room" />
      );

      await openComposer(user);

      expect(screen.getByText(/send as hero character/i)).toBeInTheDocument();
      expect(screen.getByText(/send as companion character/i)).toBeInTheDocument();
      expect(screen.queryByText(/non-participant/i)).not.toBeInTheDocument();
    });
  });

  describe('Delete Message', () => {
    it('shows delete button for own messages', async () => {
      const messagesWithUserId = mockMessages.map(msg => ({
        ...msg,
        sender_user_id: 100, // Current user's ID
      }));

      server.use(
        http.get('/api/v1/games/:gameId/conversations/:conversationId/messages', () => {
          return HttpResponse.json({ messages: messagesWithUserId });
        })
      );

      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      await waitFor(() => {
        const deleteButtons = screen.getAllByTitle(/delete message/i);
        expect(deleteButtons.length).toBeGreaterThan(0);
      });
    });

    it('hides delete button for other users messages', async () => {
      const messagesWithDifferentUser = mockMessages.map(msg => ({
        ...msg,
        sender_user_id: 999, // Different user
      }));

      server.use(
        http.get('/api/v1/games/:gameId/conversations/:conversationId/messages', () => {
          return HttpResponse.json({ messages: messagesWithDifferentUser });
        })
      );

      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      await waitFor(() => {
        const deleteButtons = screen.queryAllByTitle(/delete message/i);
        expect(deleteButtons.length).toBe(0);
      });
    });

    it('hides delete button for already deleted messages', async () => {
      const messagesWithDeleted = [
        {
          ...mockMessages[0],
          sender_user_id: 100,
          is_deleted: true,
          deleted_at: '2024-01-01T11:00:00Z',
        },
      ];

      server.use(
        http.get('/api/v1/games/:gameId/conversations/:conversationId/messages', () => {
          return HttpResponse.json({ messages: messagesWithDeleted });
        })
      );

      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      await waitFor(() => {
        const deleteButtons = screen.queryAllByTitle(/delete message/i);
        expect(deleteButtons.length).toBe(0);
      });
    });

    it('shows confirmation modal when delete button is clicked', async () => {
      const user = userEvent.setup();
      const messagesWithUserId = mockMessages.map(msg => ({
        ...msg,
        sender_user_id: 100,
      }));

      server.use(
        http.get('/api/v1/games/:gameId/conversations/:conversationId/messages', () => {
          return HttpResponse.json({ messages: messagesWithUserId });
        })
      );

      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      await waitFor(() => {
        expect(screen.getAllByTitle(/delete message/i).length).toBeGreaterThan(0);
      });

      const deleteButton = screen.getAllByTitle(/delete message/i)[0];
      await user.click(deleteButton);

      await waitFor(() => {
        expect(screen.getByText(/delete message\?/i)).toBeInTheDocument();
        expect(screen.getByText(/this will permanently delete your message/i)).toBeInTheDocument();
      });
    });

    it('closes modal when cancel is clicked', async () => {
      const user = userEvent.setup();
      const messagesWithUserId = mockMessages.map(msg => ({
        ...msg,
        sender_user_id: 100,
      }));

      server.use(
        http.get('/api/v1/games/:gameId/conversations/:conversationId/messages', () => {
          return HttpResponse.json({ messages: messagesWithUserId });
        })
      );

      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      await waitFor(() => {
        expect(screen.getAllByTitle(/delete message/i).length).toBeGreaterThan(0);
      });

      // Open modal
      const deleteButton = screen.getAllByTitle(/delete message/i)[0];
      await user.click(deleteButton);

      await waitFor(() => {
        expect(screen.getByText(/delete message\?/i)).toBeInTheDocument();
      });

      // Click cancel
      const cancelButton = screen.getByRole('button', { name: /cancel/i });
      await user.click(cancelButton);

      await waitFor(() => {
        expect(screen.queryByText(/delete message\?/i)).not.toBeInTheDocument();
      });
    });

    it('deletes message when confirmed', async () => {
      const user = userEvent.setup();
      let deletedMessageId: number | null = null;

      const messagesWithUserId = mockMessages.map(msg => ({
        ...msg,
        sender_user_id: 100,
      }));

      server.use(
        http.get('/api/v1/games/:gameId/conversations/:conversationId/messages', () => {
          return HttpResponse.json({ messages: messagesWithUserId });
        }),
        http.delete('/api/v1/games/:gameId/conversations/:conversationId/messages/:messageId', ({ params }) => {
          deletedMessageId = Number(params.messageId);
          return HttpResponse.json({ message: 'Message deleted successfully', id: deletedMessageId });
        })
      );

      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      await waitFor(() => {
        expect(screen.getAllByTitle(/delete message/i).length).toBeGreaterThan(0);
      });

      // Open modal
      const deleteButton = screen.getAllByTitle(/delete message/i)[0];
      await user.click(deleteButton);

      await waitFor(() => {
        expect(screen.getByText(/delete message\?/i)).toBeInTheDocument();
      });

      // Confirm delete
      const confirmButton = screen.getByRole('button', { name: /^delete$/i });
      await user.click(confirmButton);

      await waitFor(() => {
        expect(deletedMessageId).toBe(1); // First message ID
      });
    });

    it('displays deleted messages with placeholder text', async () => {
      const deletedMessage = {
        id: 1,
        conversation_id: 1,
        sender_character_id: 1,
        sender_character_name: 'Hero Character',
        sender_username: 'player1',
        sender_user_id: 100,
        content: '[Message deleted]',
        created_at: '2024-01-01T10:00:00Z',
        is_deleted: true,
        deleted_at: '2024-01-01T11:00:00Z',
      };

      server.use(
        http.get('/api/v1/games/:gameId/conversations/:conversationId/messages', () => {
          return HttpResponse.json({ messages: [deletedMessage] });
        })
      );

      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      await waitFor(() => {
        expect(screen.getByText('[Message deleted]')).toBeInTheDocument();
      });
    });

    it('preserves sender name and timestamp for deleted messages', async () => {
      const deletedMessage = {
        id: 1,
        conversation_id: 1,
        sender_character_id: 1,
        sender_character_name: 'Hero Character',
        sender_username: 'player1',
        sender_user_id: 100,
        content: '[Message deleted]',
        created_at: '2024-01-01T10:00:00Z',
        is_deleted: true,
        deleted_at: '2024-01-01T11:00:00Z',
      };

      server.use(
        http.get('/api/v1/games/:gameId/conversations/:conversationId/messages', () => {
          return HttpResponse.json({ messages: [deletedMessage] });
        })
      );

      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      await waitFor(() => {
        expect(screen.getByText('Hero Character')).toBeInTheDocument();
        // Timestamp will be formatted - just verify it exists
        const timeElements = screen.getAllByText(/jan/i);
        expect(timeElements.length).toBeGreaterThan(0);
      });
    });

    it('shows loading state while deleting', async () => {
      const user = userEvent.setup();
      const messagesWithUserId = mockMessages.map(msg => ({
        ...msg,
        sender_user_id: 100,
      }));

      server.use(
        http.get('/api/v1/games/:gameId/conversations/:conversationId/messages', () => {
          return HttpResponse.json({ messages: messagesWithUserId });
        }),
        http.delete('/api/v1/games/:gameId/conversations/:conversationId/messages/:messageId', async () => {
          await new Promise(resolve => setTimeout(resolve, 100));
          return HttpResponse.json({ message: 'Message deleted successfully' });
        })
      );

      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      await waitFor(() => {
        expect(screen.getAllByTitle(/delete message/i).length).toBeGreaterThan(0);
      });

      const deleteButton = screen.getAllByTitle(/delete message/i)[0];
      await user.click(deleteButton);

      await waitFor(() => {
        expect(screen.getByText(/delete message\?/i)).toBeInTheDocument();
      });

      const confirmButton = screen.getByRole('button', { name: /^delete$/i });
      await user.click(confirmButton);

      // Check for loading state on button
      await waitFor(() => {
        const button = screen.getByRole('button', { name: /^delete$/i });
        expect(button).toBeDisabled();
      });
    });

    it('reloads messages after successful deletion', async () => {
      const user = userEvent.setup();
      let messagesFetchCount = 0;

      const messagesWithUserId = mockMessages.map(msg => ({
        ...msg,
        sender_user_id: 100,
      }));

      server.use(
        http.get('/api/v1/games/:gameId/conversations/:conversationId/messages', () => {
          messagesFetchCount++;
          return HttpResponse.json({ messages: messagesWithUserId });
        }),
        http.delete('/api/v1/games/:gameId/conversations/:conversationId/messages/:messageId', () => {
          return HttpResponse.json({ message: 'Message deleted successfully' });
        })
      );

      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      await waitFor(() => {
        expect(messagesFetchCount).toBeGreaterThan(0);
      });

      const initialFetchCount = messagesFetchCount;

      // Delete message
      const deleteButton = screen.getAllByTitle(/delete message/i)[0];
      await user.click(deleteButton);

      await waitFor(() => {
        expect(screen.getByText(/delete message\?/i)).toBeInTheDocument();
      });

      const confirmButton = screen.getByRole('button', { name: /^delete$/i });
      await user.click(confirmButton);

      await waitFor(() => {
        expect(messagesFetchCount).toBeGreaterThan(initialFetchCount);
      });
    });
  });

  describe('Refresh Functionality', () => {
    it('renders refresh button', async () => {
      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      await waitFor(() => {
        expect(screen.getByLabelText(/refresh messages/i)).toBeInTheDocument();
      });
    });

    it('shows refresh text on desktop', async () => {
      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      await waitFor(() => {
        expect(screen.getByText('Refresh')).toBeInTheDocument();
      });
    });

    it('enables refresh button after messages load', async () => {
      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      await waitFor(() => {
        const refreshButton = screen.getByLabelText(/refresh messages/i);
        expect(refreshButton).not.toBeDisabled();
      });
    });

    it('refreshes messages when refresh button is clicked', async () => {
      const user = userEvent.setup();
      let messagesFetchCount = 0;
      let conversationFetchCount = 0;

      server.use(
        http.get('/api/v1/games/:gameId/conversations/:conversationId', () => {
          conversationFetchCount++;
          return HttpResponse.json(mockConversation);
        }),
        http.get('/api/v1/games/:gameId/conversations/:conversationId/messages', () => {
          messagesFetchCount++;
          return HttpResponse.json({ messages: mockMessages });
        })
      );

      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      // Wait for initial load
      await waitFor(() => {
        expect(screen.getByText('Test Conversation')).toBeInTheDocument();
      });

      const initialMessagesFetch = messagesFetchCount;
      const initialConversationFetch = conversationFetchCount;

      // Click refresh button
      const refreshButton = screen.getByLabelText(/refresh messages/i);
      await user.click(refreshButton);

      // Verify both endpoints were called again
      await waitFor(() => {
        expect(messagesFetchCount).toBeGreaterThan(initialMessagesFetch);
        expect(conversationFetchCount).toBeGreaterThan(initialConversationFetch);
      });
    });

    it('handles refresh errors gracefully', async () => {
      const user = userEvent.setup();

      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      // Wait for initial load
      await waitFor(() => {
        expect(screen.getByText('Test Conversation')).toBeInTheDocument();
      });

      // Make refresh fail
      server.use(
        http.get('/api/v1/games/:gameId/conversations/:conversationId/messages', () => {
          return HttpResponse.error();
        })
      );

      // Click refresh button
      const refreshButton = screen.getByLabelText(/refresh messages/i);
      await user.click(refreshButton);

      // Wait for error handling to complete
      await waitFor(() => {
        expect(refreshButton).not.toBeDisabled();
      });
    });

    it('displays new messages after refresh', async () => {
      const user = userEvent.setup();
      let returnNewMessage = false;

      const newMessage = {
        id: 3,
        conversation_id: 1,
        sender_character_id: 1,
        sender_character_name: 'Hero Character',
        sender_username: 'player1',
        content: 'This is a brand new message!',
        created_at: '2024-01-01T10:10:00Z',
      };

      server.use(
        http.get('/api/v1/games/:gameId/conversations/:conversationId/messages', () => {
          const messages = returnNewMessage ? [...mockMessages, newMessage] : mockMessages;
          return HttpResponse.json({ messages });
        })
      );

      renderWithProviders(
        <MessageThread gameId={1} conversationId={1} characters={mockCharacters} currentPhaseType="common_room" />
      );

      // Wait for initial load
      await waitFor(() => {
        expect(screen.getByText(/hello! this is the first message/i)).toBeInTheDocument();
      });

      // Verify new message is not present
      expect(screen.queryByText(/this is a brand new message!/i)).not.toBeInTheDocument();

      // Add new message for next fetch
      returnNewMessage = true;

      // Click refresh button
      const refreshButton = screen.getByLabelText(/refresh messages/i);
      await user.click(refreshButton);

      // Verify new message appears
      await waitFor(() => {
        expect(screen.getByText(/this is a brand new message!/i)).toBeInTheDocument();
      });
    });
  });

  describe('Phase Restrictions', () => {
    it('disables messaging during action phase', async () => {
      renderWithProviders(
        <MessageThread
          gameId={1}
          conversationId={1}
          characters={mockCharacters}
          currentPhaseType="action"
        />
      );

      // In a non-messaging phase the composer is not rendered at all — only the
      // restriction alert. There is no Reply button, textarea, or Send button.
      await waitFor(() => {
        expect(screen.getByText(/new messages can only be sent during common room or interlude phases/i)).toBeInTheDocument();
      });

      expect(screen.queryByRole('button', { name: /^reply$/i })).not.toBeInTheDocument();
      expect(screen.queryByRole('button', { name: /^send$/i })).not.toBeInTheDocument();
      expect(screen.queryByPlaceholderText(/type your message/i)).not.toBeInTheDocument();
    });

    it('enables messaging during common_room phase', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <MessageThread
          gameId={1}
          conversationId={1}
          characters={mockCharacters}
          currentPhaseType="common_room"
        />
      );

      await waitFor(() => {
        expect(screen.queryByText(/new messages can only be sent during common room or interlude phases/i)).not.toBeInTheDocument();
      });

      await openComposer(user);

      const textarea = screen.getByPlaceholderText(/type your message/i);
      expect(textarea).not.toBeDisabled();
    });

    it('disables messaging during results phase', async () => {
      renderWithProviders(
        <MessageThread
          gameId={1}
          conversationId={1}
          characters={mockCharacters}
          currentPhaseType="results"
        />
      );

      await waitFor(() => {
        expect(screen.getByText(/new messages can only be sent during common room or interlude phases/i)).toBeInTheDocument();
      });

      // No composer is rendered in a non-messaging phase.
      expect(screen.queryByRole('button', { name: /^reply$/i })).not.toBeInTheDocument();
      expect(screen.queryByPlaceholderText(/type your message/i)).not.toBeInTheDocument();
    });

    it('disables messaging when phase type is undefined', async () => {
      renderWithProviders(
        <MessageThread
          gameId={1}
          conversationId={1}
          characters={mockCharacters}
          currentPhaseType={undefined}
        />
      );

      await waitFor(() => {
        expect(screen.getByText(/new messages can only be sent during common room or interlude phases/i)).toBeInTheDocument();
      });

      // No composer is rendered when the phase is unknown/non-messaging.
      expect(screen.queryByRole('button', { name: /^reply$/i })).not.toBeInTheDocument();
      expect(screen.queryByRole('button', { name: /^send$/i })).not.toBeInTheDocument();
    });

    // Note: in non-messaging phases the composer (character selector, Send
    // button, textarea) is no longer rendered at all — only the restriction
    // alert — so the former "disabled selector" and "tooltip on disabled Send"
    // tests describe UI that no longer exists. The blocked-phase behavior is
    // covered by the "disables messaging during <phase>" tests above.

    it('does not show tooltip on enabled send button during common_room phase', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <MessageThread
          gameId={1}
          conversationId={1}
          characters={mockCharacters}
          currentPhaseType="common_room"
        />
      );

      await openComposer(user);

      const sendButton = screen.getByRole('button', { name: /send/i });
      expect(sendButton).not.toHaveAttribute('title');
    });

    it('enables messaging during interlude phase', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <MessageThread
          gameId={1}
          conversationId={1}
          characters={mockCharacters}
          currentPhaseType="interlude"
        />
      );

      await waitFor(() => {
        expect(screen.queryByText(/new messages can only be sent during common room or interlude phases/i)).not.toBeInTheDocument();
      });

      await openComposer(user);

      const textarea = screen.getByPlaceholderText(/type your message/i);
      expect(textarea).not.toBeDisabled();
    });
  });
});
