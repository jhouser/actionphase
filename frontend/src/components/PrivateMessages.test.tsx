import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { MemoryRouter, useLocation } from 'react-router-dom';
import { PrivateMessages } from './PrivateMessages';
import type { Character } from '../types/characters';

// The conversation list, thread, and context all reach for the network. This
// suite is about one thing: whether the ?newConversationWith= shortcut opens
// the New Conversation form with that character pre-selected.
vi.mock('./ConversationList', () => ({
  ConversationList: () => <div data-testid="conversation-list" />,
}));
vi.mock('./MessageThread', () => ({
  MessageThread: () => <div data-testid="message-thread" />,
}));
vi.mock('./NewConversationModal', () => ({
  NewConversationModal: ({
    initialParticipantIds,
    onClose,
  }: {
    initialParticipantIds?: number[];
    onClose: () => void;
  }) => (
    <div data-testid="new-conversation-modal">
      <span data-testid="prefilled">{JSON.stringify(initialParticipantIds ?? [])}</span>
      <button onClick={onClose}>close-modal</button>
    </div>
  ),
}));

vi.mock('../contexts/ConversationContext', () => ({
  ConversationProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  useConversation: () => ({
    selectedConversationId: null,
    loadingConversations: false,
    selectConversation: vi.fn(),
    loadConversations: vi.fn(),
  }),
}));

const allGameCharacters: Character[] = [
  {
    id: 42,
    game_id: 7,
    name: 'Vesper',
    status: 'approved',
    is_active: true,
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T00:00:00Z',
  },
];

vi.mock('../contexts/GameContext', () => ({
  useGameContext: () => ({ allGameCharacters }),
}));

/** Surfaces the current query string so tests can assert the param is cleared. */
function LocationProbe() {
  const location = useLocation();
  return <span data-testid="search">{location.search}</span>;
}

function renderMessages(entry: string, currentPhaseType = 'common_room') {
  return render(
    <MemoryRouter initialEntries={[entry]}>
      <LocationProbe />
      <PrivateMessages
        gameId={7}
        characters={[]}
        isAnonymous={false}
        allowGroupConversations
        currentPhaseType={currentPhaseType}
      />
    </MemoryRouter>
  );
}

describe('PrivateMessages new-conversation shortcut', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('opens the form pre-selecting the character named in the URL', async () => {
    renderMessages('/games/7?tab=messages&newConversationWith=42');

    const modal = await screen.findByTestId('new-conversation-modal');
    expect(modal).toBeInTheDocument();
    expect(screen.getByTestId('prefilled')).toHaveTextContent('[42]');
  });

  it('clears the param so the form does not reopen on the next render', async () => {
    renderMessages('/games/7?tab=messages&newConversationWith=42');

    await screen.findByTestId('new-conversation-modal');
    await waitFor(() =>
      expect(screen.getByTestId('search')).not.toHaveTextContent('newConversationWith')
    );
    // The form stays open even though the param is gone.
    expect(screen.getByTestId('new-conversation-modal')).toBeInTheDocument();
  });

  it('does not open the form during a phase that disallows new conversations', async () => {
    renderMessages('/games/7?tab=messages&newConversationWith=42', 'action');

    await waitFor(() =>
      expect(screen.getByTestId('search')).not.toHaveTextContent('newConversationWith')
    );
    expect(screen.queryByTestId('new-conversation-modal')).not.toBeInTheDocument();
  });

  it('does not open the form without the param', () => {
    renderMessages('/games/7?tab=messages');
    expect(screen.queryByTestId('new-conversation-modal')).not.toBeInTheDocument();
  });

  it('drops the pre-selection once the form is dismissed', async () => {
    const user = userEvent.setup();
    renderMessages('/games/7?tab=messages&newConversationWith=42');

    await screen.findByTestId('new-conversation-modal');
    await user.click(screen.getByText('close-modal'));
    expect(screen.queryByTestId('new-conversation-modal')).not.toBeInTheDocument();

    // Reopening via the "+ New" button must start from an empty selection.
    await user.click(screen.getByRole('button', { name: '+ New' }));
    expect(screen.getByTestId('prefilled')).toHaveTextContent('[]');
  });
});
