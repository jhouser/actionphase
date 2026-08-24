import { describe, it, expect } from 'vitest';
import { screen, waitFor, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { renderWithProviders } from '../../test-utils';
import { AudienceConversationCard } from './AudienceConversationCard';
import type { AudienceConversationListItem } from '../../types/conversations';

describe('AudienceConversationCard', () => {
  const mockConversation: AudienceConversationListItem = {
    conversation_id: 1,
    subject: 'Test Conversation',
    conversation_type: 'group',
    created_at: '2025-01-01T10:00:00Z',
    message_count: 10,
    last_message_at: '2025-01-15T14:30:00Z',
    participant_names: ['Alice', 'Bob', 'Charlie'],
    participant_usernames: ['alice', 'bob', 'charlie'],
    last_message_content: 'This is the last message',
    last_sender_name: 'Alice',
    last_sender_username: 'alice',
  };

  it('renders conversation subject', () => {
    renderWithProviders(
      <AudienceConversationCard
        conversation={mockConversation}
      />,
    { gameId: 1 }
    );

    expect(screen.getByText('Test Conversation')).toBeInTheDocument();
  });

  it('renders participant names', () => {
    renderWithProviders(
      <AudienceConversationCard
        conversation={mockConversation}
      />,
    { gameId: 1 }
    );

    expect(screen.getByText('Alice, Bob, Charlie')).toBeInTheDocument();
  });

  it('renders last message preview', () => {
    renderWithProviders(
      <AudienceConversationCard
        conversation={mockConversation}
      />,
    { gameId: 1 }
    );

    expect(screen.getByText(/Alice:/)).toBeInTheDocument();
    expect(screen.getByText(/This is the last message/)).toBeInTheDocument();
  });

  // Navigation is the href alone. The card previously ALSO fired an onClick that
  // set the same param, pushing a second identical history entry and making the
  // first browser Back press appear to do nothing.
  it('navigates by link href, adding exactly one history entry', async () => {
    const user = userEvent.setup();

    const { router } = renderWithProviders(
      <AudienceConversationCard conversation={mockConversation} />,
      { gameId: 1, initialEntries: ['/?tab=audience'] }
    );

    const card = screen.getByTestId('conversation-item');
    expect(card.getAttribute('href')).toContain(
      `audienceConversation=${mockConversation.conversation_id}`
    );

    await user.click(card);

    await waitFor(() => {
      expect(router.state.location.search).toContain(
        `audienceConversation=${mockConversation.conversation_id}`
      );
    });

    // A single Back press must undo the click.
    await act(async () => { await router.navigate(-1); });
    await waitFor(() => {
      expect(router.state.location.search).not.toContain('audienceConversation');
    });
  });

  it('shows message count badge', () => {
    const { rerender } = renderWithProviders(
      <AudienceConversationCard
        conversation={{ ...mockConversation, message_count: 3 }}
      />,
    { gameId: 1 }
    );

    expect(screen.getByText('3 messages')).toBeInTheDocument();

    rerender(
      <AudienceConversationCard
        conversation={{ ...mockConversation, message_count: 25 }}
      />
    );
    expect(screen.getByText('25 messages')).toBeInTheDocument();
  });

  it('shows recent activity indicator for messages within 24 hours', () => {
    const now = new Date();
    const recentTime = new Date(now.getTime() - 1000 * 60 * 60).toISOString(); // 1 hour ago

    renderWithProviders(
      <AudienceConversationCard
        conversation={{ ...mockConversation, last_message_at: recentTime }}
      />,
    { gameId: 1 }
    );

    expect(screen.getByText('Recent')).toBeInTheDocument();
  });

  it('does not show recent activity indicator for old messages', () => {
    const oldTime = new Date('2025-01-01T10:00:00Z').toISOString(); // Old date

    renderWithProviders(
      <AudienceConversationCard
        conversation={{ ...mockConversation, last_message_at: oldTime }}
      />,
    { gameId: 1 }
    );

    expect(screen.queryByText('Recent')).not.toBeInTheDocument();
  });

  it('renders avatars for up to 4 participants', () => {
    renderWithProviders(
      <AudienceConversationCard
        conversation={mockConversation}
      />,
    { gameId: 1 }
    );

    // Should render 3 CharacterAvatar components (one for each participant)
    const avatars = screen.getAllByTestId('character-avatar');
    expect(avatars).toHaveLength(3);
  });

  it('shows "+X more" indicator when more than 4 participants', () => {
    const manyParticipants = {
      ...mockConversation,
      participant_names: ['Alice', 'Bob', 'Charlie', 'Dave', 'Eve', 'Frank'],
    };

    renderWithProviders(
      <AudienceConversationCard
        conversation={manyParticipants}
      />,
    { gameId: 1 }
    );

    expect(screen.getByText('+2')).toBeInTheDocument();
  });

  it('applies selected styling when isSelected is true', () => {
    const { container } = renderWithProviders(
      <AudienceConversationCard
        conversation={mockConversation}
        isSelected={true}
      />,
    { gameId: 1 }
    );

    // Check for selected styling classes
    const card = container.querySelector('.border-l-4');
    expect(card).toBeInTheDocument();
  });

  it('uses default subject when subject is null', () => {
    renderWithProviders(
      <AudienceConversationCard
        conversation={{ ...mockConversation, subject: null }}
      />,
    { gameId: 1 }
    );

    expect(screen.getByText('Conversation')).toBeInTheDocument();
  });

  it('handles missing last message gracefully', () => {
    const noLastMessage = {
      ...mockConversation,
      last_message_content: null,
      last_sender_name: null,
    };

    renderWithProviders(
      <AudienceConversationCard
        conversation={noLastMessage}
      />,
    { gameId: 1 }
    );

    // Should still render the card without errors
    expect(screen.getByText('Test Conversation')).toBeInTheDocument();
    expect(screen.queryByText(/:/)).not.toBeInTheDocument(); // No last message preview
  });

  it('renders relative timestamp', () => {
    renderWithProviders(
      <AudienceConversationCard
        conversation={mockConversation}
      />,
    { gameId: 1 }
    );

    // Should show a relative time like "X days ago"
    expect(screen.getByText(/ago/)).toBeInTheDocument();
  });

  it('handles empty participant list', () => {
    const noParticipants = {
      ...mockConversation,
      participant_names: [],
    };

    renderWithProviders(
      <AudienceConversationCard
        conversation={noParticipants}
      />,
    { gameId: 1 }
    );

    expect(screen.getByText('No participants')).toBeInTheDocument();
  });
});
