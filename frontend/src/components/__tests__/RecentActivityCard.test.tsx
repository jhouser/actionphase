import { describe, it, expect } from 'vitest';
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { renderWithProviders } from '../../test-utils';
import { RecentActivityCard } from '../RecentActivityCard';
import type { DashboardMessage } from '../../types/dashboard';

// Mock react-router-dom Link
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom');
  return {
    ...actual,
    Link: ({ to, children, className }: unknown) => (
      <a href={to} className={className}>{children}</a>
    ),
  };
});

describe('RecentActivityCard', () => {
  const baseMessage: DashboardMessage = {
    message_id: 1,
    game_id: 1,
    game_title: 'Test Game',
    author_name: 'TestAuthor',
    content: 'This is a test message',
    message_type: 'post',
    created_at: new Date().toISOString(),
  };

  it('returns null when messages array is empty', () => {
    const { container } = renderWithProviders(<RecentActivityCard messages={[]} />);

    expect(container.firstChild).toBeNull();
  });

  it('displays Recent Activity header when messages exist', () => {
    renderWithProviders(<RecentActivityCard messages={[baseMessage]} />);

    expect(screen.getByText('Recent Activity')).toBeInTheDocument();
  });

  it('displays message game title', () => {
    const message: DashboardMessage = {
      ...baseMessage,
      game_title: 'Epic Adventure Game',
    };

    renderWithProviders(<RecentActivityCard messages={[message]} />);

    expect(screen.getByText('Epic Adventure Game')).toBeInTheDocument();
  });

  it('displays message content', () => {
    const message: DashboardMessage = {
      ...baseMessage,
      content: 'The dragon swoops down from the mountain!',
    };

    renderWithProviders(<RecentActivityCard messages={[message]} />);

    expect(screen.getByText('The dragon swoops down from the mountain!')).toBeInTheDocument();
  });

  it('shows author name when no character name', () => {
    const message: DashboardMessage = {
      ...baseMessage,
      author_name: 'JohnDoe',
      character_name: undefined,
    };

    renderWithProviders(<RecentActivityCard messages={[message]} />);

    expect(screen.getByText('JohnDoe')).toBeInTheDocument();
  });

  it('shows author name with character name when available', () => {
    const message: DashboardMessage = {
      ...baseMessage,
      author_name: 'JohnDoe',
      character_name: 'Sir Galahad',
    };

    renderWithProviders(<RecentActivityCard messages={[message]} />);

    expect(screen.getByText('JohnDoe as Sir Galahad')).toBeInTheDocument();
  });

  it('displays Post type for post messages', () => {
    const message: DashboardMessage = {
      ...baseMessage,
      message_type: 'post',
    };

    renderWithProviders(<RecentActivityCard messages={[message]} />);

    expect(screen.getByText('Post')).toBeInTheDocument();
  });

  it('displays Comment type for comment messages', () => {
    const message: DashboardMessage = {
      ...baseMessage,
      message_type: 'comment',
    };

    renderWithProviders(<RecentActivityCard messages={[message]} />);

    expect(screen.getByText('Comment')).toBeInTheDocument();
  });

  it('displays Private message type for private messages', () => {
    const message: DashboardMessage = {
      ...baseMessage,
      message_type: 'private',
    };

    renderWithProviders(<RecentActivityCard messages={[message]} />);

    expect(screen.getByText('Private message')).toBeInTheDocument();
  });

  it('generates deep link for post messages', () => {
    const message: DashboardMessage = {
      ...baseMessage,
      message_id: 123,
      game_id: 42,
      message_type: 'post',
    };

    renderWithProviders(<RecentActivityCard messages={[message]} />);

    const link = screen.getByRole('link');
    expect(link).toHaveAttribute('href', '/games/42?tab=common-room&comment=123');
  });

  it('generates deep link for comment messages', () => {
    const message: DashboardMessage = {
      ...baseMessage,
      message_id: 456,
      game_id: 99,
      message_type: 'comment',
    };

    renderWithProviders(<RecentActivityCard messages={[message]} />);

    const link = screen.getByRole('link');
    expect(link).toHaveAttribute('href', '/games/99?tab=common-room&comment=456');
  });

  it('generates link to messages tab for private messages', () => {
    const message: DashboardMessage = {
      ...baseMessage,
      message_id: 789,
      game_id: 10,
      message_type: 'private_message',
    };

    renderWithProviders(<RecentActivityCard messages={[message]} />);

    const link = screen.getByRole('link');
    expect(link).toHaveAttribute('href', '/games/10?tab=messages');
  });

  it('displays multiple messages', () => {
    const messages: DashboardMessage[] = [
      {
        ...baseMessage,
        message_id: 1,
        content: 'First message',
      },
      {
        ...baseMessage,
        message_id: 2,
        content: 'Second message',
      },
      {
        ...baseMessage,
        message_id: 3,
        content: 'Third message',
      },
    ];

    renderWithProviders(<RecentActivityCard messages={messages} />);

    expect(screen.getByText('First message')).toBeInTheDocument();
    expect(screen.getByText('Second message')).toBeInTheDocument();
    expect(screen.getByText('Third message')).toBeInTheDocument();
  });

  it('formats time as "Just now" for very recent messages', () => {
    const message: DashboardMessage = {
      ...baseMessage,
      created_at: new Date(Date.now() - 30 * 1000).toISOString(), // 30 seconds ago
    };

    renderWithProviders(<RecentActivityCard messages={[message]} />);

    expect(screen.getByText('Just now')).toBeInTheDocument();
  });

  it('formats time in minutes when less than 1 hour', () => {
    const message: DashboardMessage = {
      ...baseMessage,
      created_at: new Date(Date.now() - 15 * 60 * 1000).toISOString(), // 15 minutes ago
    };

    renderWithProviders(<RecentActivityCard messages={[message]} />);

    // Should show "15m ago" (or 14m depending on timing)
    expect(screen.getByText(/\d+m ago/)).toBeInTheDocument();
  });

  it('formats time in hours when less than 24 hours', () => {
    const message: DashboardMessage = {
      ...baseMessage,
      created_at: new Date(Date.now() - 3 * 60 * 60 * 1000).toISOString(), // 3 hours ago
    };

    renderWithProviders(<RecentActivityCard messages={[message]} />);

    // Should show "3h ago" (or 2h depending on timing)
    expect(screen.getByText(/\d+h ago/)).toBeInTheDocument();
  });

  it('formats time in days when 24+ hours', () => {
    const message: DashboardMessage = {
      ...baseMessage,
      created_at: new Date(Date.now() - 2 * 24 * 60 * 60 * 1000).toISOString(), // 2 days ago
    };

    renderWithProviders(<RecentActivityCard messages={[message]} />);

    // Should show "2d ago"
    expect(screen.getByText(/\d+d ago/)).toBeInTheDocument();
  });

  it('displays all messages with their respective details', () => {
    const messages: DashboardMessage[] = [
      {
        message_id: 1,
        game_id: 1,
        game_title: 'Game One',
        author_name: 'Author1',
        character_name: 'Character1',
        content: 'Message 1 content',
        message_type: 'post',
        created_at: new Date(Date.now() - 5 * 60 * 1000).toISOString(),
      },
      {
        message_id: 2,
        game_id: 2,
        game_title: 'Game Two',
        author_name: 'Author2',
        content: 'Message 2 content',
        message_type: 'comment',
        created_at: new Date(Date.now() - 2 * 60 * 60 * 1000).toISOString(),
      },
    ];

    renderWithProviders(<RecentActivityCard messages={messages} />);

    // First message
    expect(screen.getByText('Game One')).toBeInTheDocument();
    expect(screen.getByText('Author1 as Character1')).toBeInTheDocument();
    expect(screen.getByText('Message 1 content')).toBeInTheDocument();
    expect(screen.getByText('Post')).toBeInTheDocument();

    // Second message
    expect(screen.getByText('Game Two')).toBeInTheDocument();
    expect(screen.getByText('Author2')).toBeInTheDocument();
    expect(screen.getByText('Message 2 content')).toBeInTheDocument();
    expect(screen.getByText('Comment')).toBeInTheDocument();
  });

  describe('defaultCollapsed', () => {
    it('hides message content behind the header when collapsed', () => {
      renderWithProviders(
        <RecentActivityCard messages={[baseMessage]} defaultCollapsed />
      );

      expect(screen.getByText('Recent Activity')).toBeInTheDocument();
      expect(screen.queryByText('This is a test message')).not.toBeInTheDocument();
    });

    it('shows the message count while collapsed', () => {
      const messages = [1, 2, 3].map((id) => ({ ...baseMessage, message_id: id }));

      renderWithProviders(<RecentActivityCard messages={messages} defaultCollapsed />);

      expect(screen.getByText('3')).toBeInTheDocument();
    });

    it('expands and re-collapses on header click', async () => {
      const user = userEvent.setup();
      renderWithProviders(
        <RecentActivityCard messages={[baseMessage]} defaultCollapsed />
      );

      const header = screen.getByRole('button', { name: /recent activity/i });
      expect(header).toHaveAttribute('aria-expanded', 'false');

      await user.click(header);
      expect(screen.getByText('This is a test message')).toBeInTheDocument();
      expect(header).toHaveAttribute('aria-expanded', 'true');

      await user.click(header);
      expect(screen.queryByText('This is a test message')).not.toBeInTheDocument();
    });

    it('renders expanded with no toggle button by default', () => {
      renderWithProviders(<RecentActivityCard messages={[baseMessage]} />);

      expect(screen.getByText('This is a test message')).toBeInTheDocument();
      expect(screen.queryByRole('button', { name: /recent activity/i })).not.toBeInTheDocument();
    });
  });

  it('handles message with null character_name correctly', () => {
    const message: DashboardMessage = {
      ...baseMessage,
      author_name: 'TestUser',
      character_name: null,
    };

    renderWithProviders(<RecentActivityCard messages={[message]} />);

    // Should show only author name, not "as null"
    expect(screen.getByText('TestUser')).toBeInTheDocument();
    expect(screen.queryByText(/as null/i)).not.toBeInTheDocument();
  });
});
