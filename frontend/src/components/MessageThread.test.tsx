import { render, screen, waitFor, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { MessageThread } from './MessageThread';
import type { PrivateMessage } from '../types/conversations';

vi.mock('react-router-dom', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router-dom')>();
  return {
    ...actual,
    useBlocker: () => ({ state: 'unblocked', reset: undefined, proceed: undefined }),
  };
});

// Mock the ConversationContext
const mockEditMessage = vi.fn();
const mockDeleteMessage = vi.fn();
const mockSendMessage = vi.fn();
const mockLoadConversation = vi.fn();
const mockLoadMessages = vi.fn();
const mockMarkAsRead = vi.fn();
const mockRefreshConversation = vi.fn();

const baseConversationContext = {
  conversations: [],
  selectedConversationId: 1,
  selectedConversationInfo: undefined,
  conversation: {
    conversation: { id: 1, game_id: 1, title: 'Test Chat', conversation_type: 'direct', created_by_user_id: 1, created_at: '2026-01-01', updated_at: '2026-01-01' },
    participants: [],
  },
  messages: [] as PrivateMessage[],
  loadingConversations: false,
  loadingMessages: false,
  loadingConversation: false,
  isRefreshing: false,
  selectConversation: vi.fn(),
  loadConversations: vi.fn(),
  loadConversation: mockLoadConversation,
  loadMessages: mockLoadMessages,
  refreshConversation: mockRefreshConversation,
  markAsRead: mockMarkAsRead,
  sendMessage: mockSendMessage,
  deleteMessage: mockDeleteMessage,
  editMessage: mockEditMessage,
  createConversation: vi.fn(),
  resetConversationState: vi.fn(),
};

vi.mock('../contexts/ConversationContext', () => ({
  useConversation: () => baseConversationContext,
  ConversationProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

// Mock AuthContext
vi.mock('../contexts/AuthContext', () => ({
  useAuth: () => ({
    currentUser: { id: 1, username: 'testuser' },
  }),
}));

// Mock LoggingService
vi.mock('@/services/LoggingService', () => ({
  logger: { debug: vi.fn(), error: vi.fn(), warn: vi.fn() },
}));

const makeMessage = (overrides: Partial<PrivateMessage> = {}): PrivateMessage => ({
  id: 1,
  conversation_id: 1,
  sender_user_id: 1,
  content: 'Hello world',
  created_at: '2026-01-01T00:00:00Z',
  sender_username: 'testuser',
  sender_character_name: 'TestChar',
  is_deleted: false,
  ...overrides,
});

const defaultProps = {
  gameId: 1,
  conversationId: 1,
  characters: [{ id: 10, name: 'TestChar', game_id: 1, user_id: 1, character_type: 'player_character', status: 'active', created_at: '2026-01-01', updated_at: '2026-01-01' }],
  currentPhaseType: 'common_room',
};

const defaultConversation = {
  conversation: { id: 1, game_id: 1, title: 'Test Chat', conversation_type: 'direct', created_by_user_id: 1, created_at: '2026-01-01', updated_at: '2026-01-01' },
  participants: [],
};

beforeEach(() => {
  vi.clearAllMocks();
  mockLoadMessages.mockResolvedValue([]);
  mockLoadConversation.mockResolvedValue(undefined);
  mockMarkAsRead.mockResolvedValue(undefined);
  mockRefreshConversation.mockResolvedValue(false);
  baseConversationContext.messages = [];
  baseConversationContext.conversation = { ...defaultConversation, participants: [] };
});

describe('MessageThread draft clearing on conversation change', () => {
  it('clears the message draft when switching to a different conversation', async () => {
    const user = userEvent.setup();

    // Set up participants so the message form renders
    baseConversationContext.conversation = {
      conversation: { id: 1, game_id: 1, title: 'Test Chat', conversation_type: 'direct', created_by_user_id: 1, created_at: '2026-01-01', updated_at: '2026-01-01' },
      participants: [{ id: 1, conversation_id: 1, character_id: 10, character_name: 'TestChar', user_id: 1, username: 'testuser', joined_at: '2026-01-01' }],
    };

    const { rerender } = render(<MessageThread {...defaultProps} conversationId={1} />);

    // Composer is collapsed by default; open it via the Reply button.
    await user.click(screen.getByRole('button', { name: 'Reply' }));

    const textarea = screen.getByPlaceholderText('Type your message...');
    await user.type(textarea, 'drafted text for wrong recipient');
    expect(textarea).toHaveValue('drafted text for wrong recipient');

    // Simulate navigating to a different conversation (e.g. via a notification)
    act(() => {
      rerender(<MessageThread {...defaultProps} conversationId={2} />);
    });

    expect(screen.getByPlaceholderText('Type your message...')).toHaveValue('');
  });
});

describe('MessageThread observability', () => {
  it('names the private-message send button for Faro user-action attribution', async () => {
    // Participants must be present for the send form (and its button) to render.
    baseConversationContext.conversation = {
      conversation: { id: 1, game_id: 1, title: 'Test Chat', conversation_type: 'direct', created_by_user_id: 1, created_at: '2026-01-01', updated_at: '2026-01-01' },
      participants: [{ id: 1, conversation_id: 1, character_id: 10, character_name: 'TestChar', user_id: 1, username: 'testuser', joined_at: '2026-01-01' }],
    };

    const user = userEvent.setup();
    render(<MessageThread {...defaultProps} />);

    // Composer is collapsed by default; open it via the Reply button.
    await user.click(screen.getByRole('button', { name: 'Reply' }));

    expect(
      screen.getByRole('button', { name: 'Send' })
    ).toHaveAttribute('data-faro-user-action-name', 'send-private-message');
  });
});

describe('MessageThread edit functionality', () => {
  it('shows edit button on hover for sender messages during common_room phase', async () => {
    baseConversationContext.messages = [makeMessage({ sender_user_id: 1 })];

    render(<MessageThread {...defaultProps} />);

    expect(screen.getByTestId('edit-message-button')).toBeInTheDocument();
  });

  it('does not show edit button for other users messages', () => {
    baseConversationContext.messages = [makeMessage({ sender_user_id: 99 })];

    render(<MessageThread {...defaultProps} />);

    expect(screen.queryByTestId('edit-message-button')).not.toBeInTheDocument();
  });

  it('does not show edit button outside common_room phase', () => {
    baseConversationContext.messages = [makeMessage({ sender_user_id: 1 })];

    render(<MessageThread {...defaultProps} currentPhaseType="action" />);

    expect(screen.queryByTestId('edit-message-button')).not.toBeInTheDocument();
  });

  it('shows inline editor when edit button is clicked', async () => {
    const user = userEvent.setup();
    baseConversationContext.messages = [makeMessage({ sender_user_id: 1, content: 'Original text' })];

    render(<MessageThread {...defaultProps} />);

    await user.click(screen.getByTestId('edit-message-button'));

    expect(screen.getByRole('textbox')).toBeInTheDocument();
    expect(screen.getByTestId('save-edit-button')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /cancel/i })).toBeInTheDocument();
  });

  it('calls editMessage and closes editor on save', async () => {
    const user = userEvent.setup();
    mockEditMessage.mockResolvedValue(undefined);
    baseConversationContext.messages = [makeMessage({ id: 42, sender_user_id: 1, content: 'Original' })];

    render(<MessageThread {...defaultProps} />);

    await user.click(screen.getByTestId('edit-message-button'));

    const textarea = screen.getByRole('textbox');
    await user.clear(textarea);
    await user.type(textarea, 'Edited content');

    await user.click(screen.getByTestId('save-edit-button'));

    await waitFor(() => {
      expect(mockEditMessage).toHaveBeenCalledWith(1, 1, 42, 'Edited content');
    });
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
  });

  it('cancels editing without saving', async () => {
    const user = userEvent.setup();
    baseConversationContext.messages = [makeMessage({ sender_user_id: 1 })];

    render(<MessageThread {...defaultProps} />);

    await user.click(screen.getByTestId('edit-message-button'));
    expect(screen.getByRole('textbox')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /cancel/i }));
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
    expect(mockEditMessage).not.toHaveBeenCalled();
  });

  it('shows (edited) label for edited messages', () => {
    baseConversationContext.messages = [makeMessage({ sender_user_id: 1, is_edited: true })];

    render(<MessageThread {...defaultProps} />);

    expect(screen.getByTestId('edited-label')).toBeInTheDocument();
    expect(screen.getByTestId('edited-label')).toHaveTextContent('(edited)');
  });

  it('does not show (edited) label for unedited messages', () => {
    baseConversationContext.messages = [makeMessage({ sender_user_id: 1, is_edited: false })];

    render(<MessageThread {...defaultProps} />);

    expect(screen.queryByTestId('edited-label')).not.toBeInTheDocument();
  });

  it('edit and delete buttons have explicit text color class for dark mode visibility', () => {
    baseConversationContext.messages = [makeMessage({ sender_user_id: 1 })];

    render(<MessageThread {...defaultProps} />);

    const editButton = screen.getByTestId('edit-message-button');
    expect(editButton).toHaveClass('text-content-secondary');

    const deleteButton = editButton.nextElementSibling as HTMLElement;
    expect(deleteButton).toHaveClass('text-content-secondary');
  });
});
