import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ActivityTabs } from '../ActivityTabs';
import type { DashboardDeadline, DashboardMessage } from '../../../types/dashboard';

// Mock the child components
vi.mock('../../UpcomingDeadlinesCard', () => ({
  UpcomingDeadlinesCard: ({ deadlines }: { deadlines: DashboardDeadline[] }) => (
    <div data-testid="upcoming-deadlines-card">
      Upcoming Deadlines: {deadlines.length} items
    </div>
  ),
}));

vi.mock('../../RecentActivityCard', () => ({
  RecentActivityCard: ({ messages }: { messages: DashboardMessage[] }) => (
    <div data-testid="recent-activity-card">
      Recent Activity: {messages.length} items
    </div>
  ),
}));

describe('ActivityTabs', () => {
  const mockDeadlines: DashboardDeadline[] = [
    {
      deadline_type: 'phase',
      source_id: 1,
      phase_id: 1,
      game_id: 1,
      game_title: 'Test Game 1',
      title: 'Phase 1',
      phase_type: 'action',
      phase_title: 'Phase 1',
      phase_number: 1,
      end_time: new Date(Date.now() + 12 * 60 * 60 * 1000).toISOString(),
      has_pending_submission: true,
      hours_remaining: 12,
    },
    {
      deadline_type: 'phase',
      source_id: 2,
      phase_id: 2,
      game_id: 2,
      game_title: 'Test Game 2',
      title: 'Phase 2',
      phase_type: 'discussion',
      phase_title: 'Phase 2',
      phase_number: 2,
      end_time: new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString(),
      has_pending_submission: false,
      hours_remaining: 24,
    },
  ];

  const mockMessages: DashboardMessage[] = [
    {
      message_id: 1,
      game_id: 1,
      game_title: 'Test Game 1',
      author_name: 'Test Author',
      character_name: 'Test Character',
      content: 'This is a test message',
      message_type: 'post',
      phase_id: 1,
      created_at: new Date().toISOString(),
    },
    {
      message_id: 2,
      game_id: 2,
      game_title: 'Test Game 2',
      author_name: 'Another Author',
      character_name: null,
      content: 'Another test message',
      message_type: 'comment',
      phase_id: null,
      created_at: new Date().toISOString(),
    },
  ];

  describe('with both deadlines and messages', () => {
    it('renders tabbed interface when both have content', () => {
      render(<ActivityTabs deadlines={mockDeadlines} messages={mockMessages} />);

      // Tab buttons should be present
      expect(screen.getByRole('button', { name: 'Deadlines' })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Activity' })).toBeInTheDocument();
    });

    it('defaults to showing deadlines tab first', () => {
      render(<ActivityTabs deadlines={mockDeadlines} messages={mockMessages} />);

      // Deadlines card should be visible
      expect(screen.getByTestId('upcoming-deadlines-card')).toBeInTheDocument();
      // Activity card should not be visible initially
      expect(screen.queryByTestId('recent-activity-card')).not.toBeInTheDocument();
    });

    it('applies active styling to deadlines tab by default', () => {
      render(<ActivityTabs deadlines={mockDeadlines} messages={mockMessages} />);

      const deadlinesTab = screen.getByRole('button', { name: 'Deadlines' });
      const activityTab = screen.getByRole('button', { name: 'Activity' });

      // Check for active styling classes
      expect(deadlinesTab.className).toContain('text-interactive-primary');
      expect(deadlinesTab.className).toContain('border-interactive-primary');

      // Check for inactive styling classes
      expect(activityTab.className).toContain('text-content-secondary');
      expect(activityTab.className).not.toContain('border-interactive-primary');
    });

    it('switches to activity tab when clicked', async () => {
      const user = userEvent.setup();

      render(<ActivityTabs deadlines={mockDeadlines} messages={mockMessages} />);

      const activityTab = screen.getByRole('button', { name: 'Activity' });
      await user.click(activityTab);

      // Activity card should now be visible
      expect(screen.getByTestId('recent-activity-card')).toBeInTheDocument();
      // Deadlines card should not be visible
      expect(screen.queryByTestId('upcoming-deadlines-card')).not.toBeInTheDocument();
    });

    it('applies active styling to activity tab when clicked', async () => {
      const user = userEvent.setup();

      render(<ActivityTabs deadlines={mockDeadlines} messages={mockMessages} />);

      const deadlinesTab = screen.getByRole('button', { name: 'Deadlines' });
      const activityTab = screen.getByRole('button', { name: 'Activity' });

      await user.click(activityTab);

      // Activity tab should now have active styling
      expect(activityTab.className).toContain('text-interactive-primary');
      expect(activityTab.className).toContain('border-interactive-primary');

      // Deadlines tab should have inactive styling
      expect(deadlinesTab.className).toContain('text-content-secondary');
      expect(deadlinesTab.className).not.toContain('border-interactive-primary');
    });

    it('can toggle between tabs multiple times', async () => {
      const user = userEvent.setup();

      render(<ActivityTabs deadlines={mockDeadlines} messages={mockMessages} />);

      const deadlinesTab = screen.getByRole('button', { name: 'Deadlines' });
      const activityTab = screen.getByRole('button', { name: 'Activity' });

      // Click activity tab
      await user.click(activityTab);
      expect(screen.getByTestId('recent-activity-card')).toBeInTheDocument();

      // Click deadlines tab
      await user.click(deadlinesTab);
      expect(screen.getByTestId('upcoming-deadlines-card')).toBeInTheDocument();

      // Click activity tab again
      await user.click(activityTab);
      expect(screen.getByTestId('recent-activity-card')).toBeInTheDocument();
    });
  });

  describe('with only deadlines', () => {
    it('renders only deadlines card without tabs', () => {
      render(<ActivityTabs deadlines={mockDeadlines} messages={[]} />);

      // Deadlines card should be visible
      expect(screen.getByTestId('upcoming-deadlines-card')).toBeInTheDocument();

      // Tab buttons should not be present
      expect(screen.queryByRole('button', { name: 'Deadlines' })).not.toBeInTheDocument();
      expect(screen.queryByRole('button', { name: 'Activity' })).not.toBeInTheDocument();
    });

    it('passes deadlines prop correctly to child component', () => {
      render(<ActivityTabs deadlines={mockDeadlines} messages={[]} />);

      expect(screen.getByText('Upcoming Deadlines: 2 items')).toBeInTheDocument();
    });
  });

  describe('with only messages', () => {
    it('renders only activity card without tabs', () => {
      render(<ActivityTabs deadlines={[]} messages={mockMessages} />);

      // Activity card should be visible
      expect(screen.getByTestId('recent-activity-card')).toBeInTheDocument();

      // Tab buttons should not be present
      expect(screen.queryByRole('button', { name: 'Deadlines' })).not.toBeInTheDocument();
      expect(screen.queryByRole('button', { name: 'Activity' })).not.toBeInTheDocument();
    });

    it('passes messages prop correctly to child component', () => {
      render(<ActivityTabs deadlines={[]} messages={mockMessages} />);

      expect(screen.getByText('Recent Activity: 2 items')).toBeInTheDocument();
    });
  });

  describe('with no deadlines or messages', () => {
    it('renders nothing when both are empty', () => {
      const { container } = render(<ActivityTabs deadlines={[]} messages={[]} />);

      // Component should render null
      expect(container.firstChild).toBeNull();
    });
  });

  describe('UI structure', () => {
    it('has correct card container styling', () => {
      const { container } = render(<ActivityTabs deadlines={mockDeadlines} messages={mockMessages} />);

      // Check for card container classes
      const cardContainer = container.querySelector('.surface-base');
      expect(cardContainer).toBeInTheDocument();
      expect(cardContainer?.className).toContain('rounded-lg');
      expect(cardContainer?.className).toContain('shadow-md');
    });

    it('has border between tab headers and content', () => {
      const { container } = render(<ActivityTabs deadlines={mockDeadlines} messages={mockMessages} />);

      // Check for border classes on tab header container
      const tabHeaderContainer = container.querySelector('.border-b');
      expect(tabHeaderContainer).toBeInTheDocument();
      expect(tabHeaderContainer?.className).toContain('border-theme-default');
    });

    it('wraps tab content in padding container', () => {
      const { container } = render(<ActivityTabs deadlines={mockDeadlines} messages={mockMessages} />);

      // Check for padding on content wrapper
      const contentWrapper = container.querySelector('.p-6');
      expect(contentWrapper).toBeInTheDocument();
    });
  });

  describe('edge cases', () => {
    it('handles single deadline', () => {
      const singleDeadline = [mockDeadlines[0]];

      render(<ActivityTabs deadlines={singleDeadline} messages={[]} />);

      expect(screen.getByTestId('upcoming-deadlines-card')).toBeInTheDocument();
      expect(screen.getByText('Upcoming Deadlines: 1 items')).toBeInTheDocument();
    });

    it('handles single message', () => {
      const singleMessage = [mockMessages[0]];

      render(<ActivityTabs deadlines={[]} messages={singleMessage} />);

      expect(screen.getByTestId('recent-activity-card')).toBeInTheDocument();
      expect(screen.getByText('Recent Activity: 1 items')).toBeInTheDocument();
    });

    it('handles many deadlines', () => {
      const manyDeadlines = Array.from({ length: 10 }, (_, i) => ({
        ...mockDeadlines[0],
        phase_id: i + 1,
        phase_title: `Phase ${i + 1}`,
      }));

      render(<ActivityTabs deadlines={manyDeadlines} messages={[]} />);

      expect(screen.getByText('Upcoming Deadlines: 10 items')).toBeInTheDocument();
    });

    it('handles many messages', () => {
      const manyMessages = Array.from({ length: 15 }, (_, i) => ({
        ...mockMessages[0],
        message_id: i + 1,
        content: `Message ${i + 1}`,
      }));

      render(<ActivityTabs deadlines={[]} messages={manyMessages} />);

      expect(screen.getByText('Recent Activity: 15 items')).toBeInTheDocument();
    });
  });

  describe('responsive behavior', () => {
    it('maintains tab state when switching between tabs', async () => {
      const user = userEvent.setup();

      render(<ActivityTabs deadlines={mockDeadlines} messages={mockMessages} />);

      // Switch to activity tab
      await user.click(screen.getByRole('button', { name: 'Activity' }));
      expect(screen.getByTestId('recent-activity-card')).toBeInTheDocument();

      // Switch back to deadlines tab
      await user.click(screen.getByRole('button', { name: 'Deadlines' }));
      expect(screen.getByTestId('upcoming-deadlines-card')).toBeInTheDocument();

      // Both child components should have received correct props
      // (This is implicit from the mocks showing correct item counts)
    });
  });
});
