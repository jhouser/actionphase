import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { MemoryRouter, Link } from 'react-router-dom';
import { PrivateMessages } from './PrivateMessages';

// Regression: clicking a notification for the conversation you are ALREADY
// viewing did nothing. The link_url is identical to the current URL, so the
// `conversation` search param never changed and the sync effect — which keyed
// only off that param — never re-ran. The new reply stayed invisible until a
// manual refresh.

vi.mock('./ConversationList', () => ({
  ConversationList: () => <div data-testid="conversation-list" />,
}));
vi.mock('./MessageThread', () => ({
  MessageThread: () => <div data-testid="message-thread" />,
}));
vi.mock('./NewConversationModal', () => ({
  NewConversationModal: () => <div data-testid="new-conversation-modal" />,
}));

// The context is stateful here: selectConversation records the id the way the
// real provider does, so the "already selected" precondition is genuine rather
// than assumed.
const refreshConversation = vi.fn();
const selectConversation = vi.fn();
let selectedConversationId: number | null = null;

vi.mock('../contexts/ConversationContext', () => ({
  ConversationProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  useConversation: () => ({
    selectedConversationId,
    loadingConversations: false,
    selectConversation: (id: number | null) => {
      selectedConversationId = id;
      selectConversation(id);
    },
    loadConversations: vi.fn(),
    refreshConversation,
  }),
}));

vi.mock('../contexts/GameContext', () => ({
  useGameContext: () => ({ allGameCharacters: [] }),
}));

/**
 * Stands in for the notification dropdown: a Link pointing at the conversation
 * currently on screen, exactly as a private_message notification's link_url does.
 */
function renderWithNotificationLink(conversationId: number) {
  return render(
    <MemoryRouter initialEntries={[`/games/7?tab=messages&conversation=${conversationId}`]}>
      <Link to={`/games/7?tab=messages&conversation=${conversationId}`}>notification</Link>
      <PrivateMessages
        gameId={7}
        characters={[]}
        isAnonymous={false}
        allowGroupConversations
        currentPhaseType="common_room"
      />
    </MemoryRouter>
  );
}

describe('PrivateMessages — notification for the conversation already open', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    selectedConversationId = null;
  });

  it('reloads the thread when the notification points at the open conversation', async () => {
    const user = userEvent.setup();
    renderWithNotificationLink(5);

    // Precondition: conversation 5 is the one on screen.
    await waitFor(() => expect(selectedConversationId).toBe(5));
    refreshConversation.mockClear();

    await user.click(screen.getByRole('link', { name: 'notification' }));

    // The whole point of the fix: the new reply gets fetched.
    await waitFor(() => expect(refreshConversation).toHaveBeenCalledWith(7, 5));
  });

  it('does not refetch on the initial load of a conversation', async () => {
    renderWithNotificationLink(5);

    await waitFor(() => expect(selectedConversationId).toBe(5));

    // Opening a conversation already loads it; refreshing on top of that would
    // be a duplicate request and a spurious "1 new message" toast.
    expect(refreshConversation).not.toHaveBeenCalled();
  });
});
