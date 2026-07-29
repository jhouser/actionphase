import { describe, it, expect } from 'vitest';
import { screen } from '@testing-library/react';
import { renderWithProviders } from '../../test-utils';
import { NotificationDigest } from '../NotificationDigest';

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom');
  return {
    ...actual,
    Link: ({ to, children, className }: { to: string; children: React.ReactNode; className?: string }) => (
      <a href={to} className={className}>{children}</a>
    ),
  };
});

describe('NotificationDigest', () => {
  it('renders nothing when there are no notifications', () => {
    const { container } = renderWithProviders(
      <NotificationDigest notificationsByType={{}} />
    );
    expect(container.firstChild).toBeNull();
  });

  it('renders nothing when all counts are zero', () => {
    const { container } = renderWithProviders(
      <NotificationDigest notificationsByType={{ private_message: 0, comment_reply: 0 }} />
    );
    expect(container.firstChild).toBeNull();
  });

  it('shows singular label for count of 1', () => {
    renderWithProviders(
      <NotificationDigest notificationsByType={{ handout_published: 1 }} gameId={5} />
    );
    expect(screen.getByText('1 new handout')).toBeInTheDocument();
  });

  it('shows plural label for count > 1', () => {
    renderWithProviders(
      <NotificationDigest notificationsByType={{ handout_published: 3 }} gameId={5} />
    );
    expect(screen.getByText('3 new handouts')).toBeInTheDocument();
  });

  it('links player-facing notification to the correct game tab', () => {
    renderWithProviders(
      <NotificationDigest notificationsByType={{ action_result: 2 }} gameId={5} />
    );
    const link = screen.getByRole('link', { name: /2 action results published/i });
    expect(link).toHaveAttribute('href', '/games/5?tab=actions');
  });

  it('collapses GM-type notifications into "other" bucket', () => {
    renderWithProviders(
      <NotificationDigest notificationsByType={{ application_submitted: 4 }} gameId={5} />
    );
    expect(screen.getByText('4 other notifications')).toBeInTheDocument();
  });

  it('collapses unknown types into "other" bucket', () => {
    renderWithProviders(
      <NotificationDigest notificationsByType={{ some_unknown_type: 1 }} gameId={5} />
    );
    expect(screen.getByText('1 other notification')).toBeInTheDocument();
  });

  it('links to /notifications when no gameId provided', () => {
    renderWithProviders(
      <NotificationDigest notificationsByType={{ handout_published: 1 }} />
    );
    const link = screen.getByRole('link', { name: /1 new handout/i });
    expect(link).toHaveAttribute('href', '/notifications?tab=handouts');
  });

  it('renders multiple notification types in priority order', () => {
    renderWithProviders(
      <NotificationDigest
        notificationsByType={{ handout_published: 1, action_result: 2 }}
        gameId={5}
      />
    );
    const links = screen.getAllByRole('link');
    // action_result has higher priority than handout_published
    expect(links[0]).toHaveTextContent('2 action results published');
    expect(links[1]).toHaveTextContent('1 new handout');
  });

  it('omits types the Inbox card already lists as repliable rows', () => {
    renderWithProviders(
      <NotificationDigest
        notificationsByType={{ comment_reply: 1, character_mention: 2, private_message: 3 }}
        gameId={5}
      />
    );
    // All three are surfaced by the Inbox above; showing them again would
    // restate the same notification twice on one page.
    expect(screen.queryByText('1 reply to your comment')).not.toBeInTheDocument();
    expect(screen.queryByText('2 mentions')).not.toBeInTheDocument();
    expect(screen.queryByText('3 private messages')).not.toBeInTheDocument();
  });

  it('renders nothing when only Inbox-handled types are present', () => {
    const { container } = renderWithProviders(
      <NotificationDigest notificationsByType={{ comment_reply: 1 }} gameId={5} />
    );
    expect(container.firstChild).toBeNull();
  });
});
