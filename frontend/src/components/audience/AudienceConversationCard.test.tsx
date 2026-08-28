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

  // Names sit inline beside the avatars. Desktop names everyone; only the narrow
  // variant collapses, to avoid truncating mid-word at ~390px.
  // How many names show is measured from the rendered row. jsdom reports a
  // 0-width container, which the hook treats as "no information yet" and leaves
  // everything expanded — the same fallback a real browser paints before its
  // first measurement, so nothing is hidden without a width to justify it.
  it('names every participant when width is unconstrained', () => {
    renderWithProviders(
      <AudienceConversationCard
        conversation={mockConversation}
      />,
    { gameId: 1 }
    );

    expect(screen.getByText('Alice, Bob, Charlie')).toBeInTheDocument();
  });

  it('renders participant names in full when they fit', () => {
    renderWithProviders(
      <AudienceConversationCard
        conversation={{ ...mockConversation, participant_names: ['Alice', 'Bob'] }}
      />,
    { gameId: 1 }
    );

    expect(screen.getByText('Alice, Bob')).toBeInTheDocument();
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

  it('pluralises the message count', () => {
    renderWithProviders(
      <AudienceConversationCard
        conversation={{ ...mockConversation, message_count: 1 }}
      />,
    { gameId: 1 }
    );

    expect(screen.getByText('1 message')).toBeInTheDocument();
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

  // The "Recent" badge was removed: a green dot meaning "within 24 hours" sat
  // beside a timestamp already reading "about 1 hour ago". The timestamp alone
  // now carries recency, and it must distinguish these cases on its own.
  it('shows a relative timestamp for recent activity', () => {
    const recentTime = new Date(Date.now() - 1000 * 60 * 60).toISOString(); // 1 hour ago

    renderWithProviders(
      <AudienceConversationCard
        conversation={{ ...mockConversation, last_message_at: recentTime }}
      />,
    { gameId: 1 }
    );

    expect(screen.getByText('about 1 hour ago')).toBeInTheDocument();
    expect(screen.queryByText('Recent')).not.toBeInTheDocument();
  });

  it('shows a relative timestamp for old activity', () => {
    const oldTime = new Date('2025-01-01T10:00:00Z').toISOString();

    renderWithProviders(
      <AudienceConversationCard
        conversation={{ ...mockConversation, last_message_at: oldTime }}
      />,
    { gameId: 1 }
    );

    expect(screen.getByText(/years? ago$/)).toBeInTheDocument();
  });

  it('renders an avatar for each participant when width is unconstrained', () => {
    renderWithProviders(
      <AudienceConversationCard
        conversation={mockConversation}
      />,
    { gameId: 1 }
    );

    const avatars = screen.getAllByTestId('character-avatar');
    expect(avatars).toHaveLength(3);
  });

  it('renders every avatar for a large conversation when width allows', () => {
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

    expect(screen.getAllByTestId('character-avatar')).toHaveLength(6);
    // Nothing is collapsed, so no overflow bubble.
    expect(screen.queryByTitle(/more$/)).not.toBeInTheDocument();
  });

  // Regression: the overflow bubble used bg-content-tertiary, which has no
  // corresponding background utility (only text-content-tertiary exists), so it
  // rendered as an empty circle with no fill. Driven here by a stubbed width,
  // since jsdom otherwise reports 0 and nothing collapses.
  it('renders the overflow bubble with a real background utility', () => {
    const manyParticipants = {
      ...mockConversation,
      participant_names: ['Alice', 'Bob', 'Charlie', 'Dave', 'Eve', 'Frank'],
    };

    // Narrow enough that the avatar stack itself must shed faces (all six cost
    // 152px; 120px fits three plus the bubble).
    const widthSpy = vi
      .spyOn(HTMLElement.prototype, 'getBoundingClientRect')
      .mockReturnValue({ width: 120, height: 32, top: 0, left: 0, bottom: 0, right: 0, x: 0, y: 0, toJSON: () => ({}) });

    try {
      renderWithProviders(
        <AudienceConversationCard conversation={manyParticipants} />,
        { gameId: 1 }
      );

      const bubble = screen.getByTitle(/more$/);
      expect(bubble).toHaveClass('surface-sunken');
      expect(bubble.className).not.toContain('bg-content-tertiary');
      // Must match a size="sm" avatar so the stack keeps one height.
      expect(bubble).toHaveClass('h-8', 'w-8');
    } finally {
      widthSpy.mockRestore();
    }
  });

  // The priority rule: a long name must never cost a face. At 330px (roughly the
  // metadata row inside a card at a 390px viewport) every avatar still fits, so
  // the names give way entirely rather than the stack shedding anyone.
  it('keeps every avatar when only the names are too wide', () => {
    const manyParticipants = {
      ...mockConversation,
      participant_names: ['Alice', 'Bob', 'Charlie', 'Dave', 'Eve', 'Frank'],
    };

    const widthSpy = vi
      .spyOn(HTMLElement.prototype, 'getBoundingClientRect')
      .mockReturnValue({ width: 330, height: 32, top: 0, left: 0, bottom: 0, right: 0, x: 0, y: 0, toJSON: () => ({}) });

    try {
      renderWithProviders(
        <AudienceConversationCard conversation={manyParticipants} />,
        { gameId: 1 }
      );

      // Every face survives...
      expect(screen.getAllByTestId('character-avatar')).toHaveLength(6);
      expect(screen.queryByTitle(/more$/)).not.toBeInTheDocument();
      // ...while the names give way — either collapsed to a "+N" suffix or, when
      // that is still too wide, to a bare participant count. Which of the two
      // depends on real text metrics, so assert the property, not the string.
      const nameLine = screen.getByTestId('conversation-item')
        .querySelector('[data-participant-names]')!;
      expect(nameLine.textContent).toMatch(/(\+\d+|\d+ people)$/);
    } finally {
      widthSpy.mockRestore();
    }
  });

  // Regression: the "+N" bubble is exactly one avatar wide, so hiding a single
  // avatar to make room for it saves nothing — a 2-person conversation rendered
  // "DK +1" where both faces fit.
  it('does not collapse a single avatar into an equally wide bubble', () => {
    // Both faces cost 56px here, well inside 120px — the old rule still dropped
    // one because it reserved room for a name first.
    const widthSpy = vi
      .spyOn(HTMLElement.prototype, 'getBoundingClientRect')
      .mockReturnValue({ width: 120, height: 32, top: 0, left: 0, bottom: 0, right: 0, x: 0, y: 0, toJSON: () => ({}) });

    try {
      renderWithProviders(
        <AudienceConversationCard
          conversation={{ ...mockConversation, participant_names: ['Alice', 'Bob'] }}
        />,
        { gameId: 1 }
      );

      expect(screen.getAllByTestId('character-avatar')).toHaveLength(2);
      expect(screen.queryByTitle(/more$/)).not.toBeInTheDocument();
    } finally {
      widthSpy.mockRestore();
    }
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
