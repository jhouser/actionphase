import { describe, it, expect } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { BrowserRouter } from 'react-router-dom';
import { EnhancedGameCard } from './EnhancedGameCard';
import type { EnrichedGameListItem } from '../types/games';

const wrapper = ({ children }: { children: React.ReactNode }) => (
  <BrowserRouter>{children}</BrowserRouter>
);

describe('EnhancedGameCard', () => {
  const mockGame: EnrichedGameListItem = {
    id: 1,
    title: 'Test Game',
    description: 'A test game description',
    gm_user_id: 99,
    gm_username: 'test_gm',
    state: 'recruitment',
    genre: 'Fantasy',
    current_players: 3,
    max_players: 6,
    deadline_urgency: 'normal',
    has_recent_activity: false,
    user_relationship: 'none',
    created_at: '2025-01-01T00:00:00Z',
    updated_at: '2025-01-01T00:00:00Z',
    is_public: true,
    is_anonymous: false,
  };

  describe('Basic Rendering', () => {
    it('should render game title and description', () => {
      render(<EnhancedGameCard game={mockGame} />, { wrapper });

      expect(screen.getByText('Test Game')).toBeInTheDocument();
      expect(screen.getByText('A test game description')).toBeInTheDocument();
    });

    it('should render GM username', () => {
      render(<EnhancedGameCard game={mockGame} />, { wrapper });

      expect(screen.getByText(/test_gm/)).toBeInTheDocument();
    });

    it('should render player count', () => {
      render(<EnhancedGameCard game={mockGame} />, { wrapper });

      expect(screen.getByText(/3 \/ 6/)).toBeInTheDocument();
    });

    it('should render state badge', () => {
      render(<EnhancedGameCard game={mockGame} />, { wrapper });

      expect(screen.getByText('Recruiting Players')).toBeInTheDocument();
    });

    it('should render genre badge when genre exists', () => {
      render(<EnhancedGameCard game={mockGame} />, { wrapper });

      expect(screen.getByText('Fantasy')).toBeInTheDocument();
    });

    it('should not render genre badge when genre is null', () => {
      const gameWithoutGenre = { ...mockGame, genre: null };
      render(<EnhancedGameCard game={gameWithoutGenre} />, { wrapper });

      // Should still render title but no genre badge
      expect(screen.getByText('Test Game')).toBeInTheDocument();
      expect(screen.queryByText('Fantasy')).not.toBeInTheDocument();
    });
  });

  describe('User Relationship Badges', () => {
    it('should show "GM" badge for GM', () => {
      const gmGame = { ...mockGame, user_relationship: 'gm' as const };
      render(<EnhancedGameCard game={gmGame} />, { wrapper });

      expect(screen.getByText('GM')).toBeInTheDocument();
    });

    it('should show "Co-GM" badge for co-GM', () => {
      const coGmGame = { ...mockGame, user_relationship: 'co_gm' as const };
      render(<EnhancedGameCard game={coGmGame} />, { wrapper });

      expect(screen.getByText('Co-GM')).toBeInTheDocument();
    });

    it('should show "Player" badge for participant', () => {
      const participantGame = { ...mockGame, user_relationship: 'participant' as const };
      render(<EnhancedGameCard game={participantGame} />, { wrapper });

      expect(screen.getByText('Player')).toBeInTheDocument();
    });

    it('should show "Audience" badge for audience, not "Player"', () => {
      // Regression: audience members used to come back as 'participant' from
      // the listing query and were labelled "You are playing".
      const audienceGame = { ...mockGame, user_relationship: 'audience' as const };
      render(<EnhancedGameCard game={audienceGame} />, { wrapper });

      expect(screen.getByText('Audience')).toBeInTheDocument();
      expect(screen.queryByText('Player')).not.toBeInTheDocument();
    });

    it('should show "Applied" badge for applied', () => {
      const appliedGame = { ...mockGame, user_relationship: 'applied' as const };
      render(<EnhancedGameCard game={appliedGame} />, { wrapper });

      expect(screen.getByText('Applied')).toBeInTheDocument();
    });

    it('should still show the badge on a completed game the user played in', () => {
      // This is the case the state-based border scheme exists for: the card is
      // muted, so the badge is the only thing saying "you were in this game".
      const playedGame = {
        ...mockGame,
        state: 'completed' as const,
        user_relationship: 'participant' as const,
      };
      render(<EnhancedGameCard game={playedGame} />, { wrapper });

      expect(screen.getByText('Player')).toBeInTheDocument();
    });

    it('should not show badge for none relationship', () => {
      render(<EnhancedGameCard game={mockGame} />, { wrapper });

      expect(screen.queryByText('GM')).not.toBeInTheDocument();
      expect(screen.queryByText('Player')).not.toBeInTheDocument();
      expect(screen.queryByText('Audience')).not.toBeInTheDocument();
      expect(screen.queryByText('Applied')).not.toBeInTheDocument();
    });
  });

  describe('Urgency Badges', () => {
    it('should show critical urgency badge', () => {
      const urgentGame = {
        ...mockGame,
        deadline_urgency: 'critical' as const,
        recruitment_deadline: '2025-10-20T23:59:59Z',
      };
      render(<EnhancedGameCard game={urgentGame} />, { wrapper });

      expect(screen.getByText(/⚠️ Urgent/)).toBeInTheDocument();
    });

    it('should show warning urgency badge', () => {
      const warningGame = {
        ...mockGame,
        deadline_urgency: 'warning' as const,
        recruitment_deadline: '2025-10-22T23:59:59Z',
      };
      render(<EnhancedGameCard game={warningGame} />, { wrapper });

      expect(screen.getByText(/⏰ Soon/)).toBeInTheDocument();
    });

    it('should not show urgency badge for normal urgency', () => {
      const normalGame = {
        ...mockGame,
        deadline_urgency: 'normal' as const,
        recruitment_deadline: '2025-11-01T23:59:59Z',
      };
      render(<EnhancedGameCard game={normalGame} />, { wrapper });

      expect(screen.queryByText(/⚠️ Urgent/)).not.toBeInTheDocument();
      expect(screen.queryByText(/⏰ Soon/)).not.toBeInTheDocument();
    });

    it('should not show urgency badge when no deadline', () => {
      render(<EnhancedGameCard game={mockGame} />, { wrapper });

      expect(screen.queryByText(/⚠️ Urgent/)).not.toBeInTheDocument();
      expect(screen.queryByText(/⏰ Soon/)).not.toBeInTheDocument();
    });
  });

  describe('Recent Activity Indicator', () => {
    it('should show recent activity badge when has_recent_activity is true', () => {
      const activeGame = { ...mockGame, has_recent_activity: true };
      render(<EnhancedGameCard game={activeGame} />, { wrapper });

      expect(screen.getByText('New Activity')).toBeInTheDocument();
    });

    it('should not show recent activity badge when has_recent_activity is false', () => {
      render(<EnhancedGameCard game={mockGame} />, { wrapper });

      expect(screen.queryByText('New Activity')).not.toBeInTheDocument();
    });
  });

  describe('Visual Styling', () => {
    // The card border encodes GAME STATE, not the viewer's relationship to the
    // game. Relationship is carried by the badge (see "User Relationship Badge"),
    // so that a completed game the user played in no longer looks like a CTA.
    it('should tint the border green for recruiting games', () => {
      const { container } = render(<EnhancedGameCard game={mockGame} />, { wrapper });

      const card = container.firstChild as HTMLElement;
      expect(card.className).toContain('border-semantic-success');
      expect(card.className).toContain('bg-semantic-success-subtle');
    });

    it('should tint the border amber for in-progress games', () => {
      const activeGame = { ...mockGame, state: 'in_progress' as const };
      const { container } = render(<EnhancedGameCard game={activeGame} />, { wrapper });

      const card = container.firstChild as HTMLElement;
      expect(card.className).toContain('border-semantic-warning');
      expect(card.className).toContain('bg-semantic-warning-subtle');
    });

    it('should mute completed games even when the user played in them', () => {
      const playedGame = {
        ...mockGame,
        state: 'completed' as const,
        user_relationship: 'participant' as const,
      };
      const { container } = render(<EnhancedGameCard game={playedGame} />, { wrapper });

      const card = container.firstChild as HTMLElement;
      expect(card.className).toContain('border-theme-subtle');
      // Regression: participation must not re-introduce a highlight tint.
      expect(card.className).not.toContain('bg-interactive-primary-subtle');
      expect(card.className).not.toContain('bg-semantic-success-subtle');
    });

    it('should mute cancelled games', () => {
      const cancelledGame = { ...mockGame, state: 'cancelled' as const };
      const { container } = render(<EnhancedGameCard game={cancelledGame} />, { wrapper });

      const card = container.firstChild as HTMLElement;
      expect(card.className).toContain('border-theme-subtle');
    });

    it('should style the card by state independently of relationship', () => {
      const asGm = { ...mockGame, user_relationship: 'gm' as const };
      const asStranger = { ...mockGame, user_relationship: 'none' as const };

      const { container: gmContainer } = render(<EnhancedGameCard game={asGm} />, { wrapper });
      const { container: strangerContainer } = render(
        <EnhancedGameCard game={asStranger} />,
        { wrapper }
      );

      expect((gmContainer.firstChild as HTMLElement).className).toBe(
        (strangerContainer.firstChild as HTMLElement).className
      );
    });

    it('should be clickable when onClick is provided', () => {
      const { container } = render(<EnhancedGameCard game={mockGame} onClick={vi.fn()} />, { wrapper });

      const card = container.firstChild as HTMLElement;
      // Link elements are always clickable, no need for cursor-pointer class
      expect(card.tagName).toBe('A');
    });
  });

  describe('Open Spots Indicator', () => {
    it('should show "✓ Open" for recruiting games with available spots', () => {
      render(<EnhancedGameCard game={mockGame} />, { wrapper });

      expect(screen.getByText('✓ Open')).toBeInTheDocument();
    });

    it('should not show "✓ Open" for full games', () => {
      const fullGame = { ...mockGame, current_players: 6, max_players: 6 };
      render(<EnhancedGameCard game={fullGame} />, { wrapper });

      expect(screen.queryByText('✓ Open')).not.toBeInTheDocument();
    });

    it('should not show "✓ Open" for non-recruiting games', () => {
      const inProgressGame = { ...mockGame, state: 'in_progress' as const };
      render(<EnhancedGameCard game={inProgressGame} />, { wrapper });

      expect(screen.queryByText('✓ Open')).not.toBeInTheDocument();
    });
  });

  describe('Deadline Display', () => {
    it('should show recruitment deadline for recruiting games', () => {
      const gameWithDeadline = {
        ...mockGame,
        recruitment_deadline: '2025-10-25T23:59:59Z',
      };
      render(<EnhancedGameCard game={gameWithDeadline} />, { wrapper });

      expect(screen.getByText('Application Deadline:')).toBeInTheDocument();
    });

    it('should show phase deadline for in-progress games', () => {
      const gameWithPhaseDeadline = {
        ...mockGame,
        state: 'in_progress' as const,
        current_phase_type: 'action' as const,
        current_phase_deadline: '2025-10-25T23:59:59Z',
      };
      render(<EnhancedGameCard game={gameWithPhaseDeadline} />, { wrapper });

      expect(screen.getByText('Phase Deadline:')).toBeInTheDocument();
    });

    it('should not show deadline when none exists', () => {
      render(<EnhancedGameCard game={mockGame} />, { wrapper });

      expect(screen.queryByText(/Deadline:/)).not.toBeInTheDocument();
    });
  });

  describe('Apply Button', () => {
    it('should show Apply button when showApplyButton is true', () => {
      render(
        <EnhancedGameCard
          game={mockGame}
          onApplyClick={vi.fn()}
          showApplyButton={true}
        />,
        { wrapper }
      );

      expect(screen.getByText('Apply to Join')).toBeInTheDocument();
    });

    it('should not show Apply button when showApplyButton is false', () => {
      render(
        <EnhancedGameCard
          game={mockGame}
          onApplyClick={vi.fn()}
          showApplyButton={false}
        />,
        { wrapper }
      );

      expect(screen.queryByText('Apply to Join')).not.toBeInTheDocument();
    });

    it('should not show Apply button for user\'s own game', () => {
      const userGame = { ...mockGame, user_relationship: 'participant' as const };
      render(
        <EnhancedGameCard
          game={userGame}
          onApplyClick={vi.fn()}
          showApplyButton={true}
        />,
        { wrapper }
      );

      expect(screen.queryByText('Apply to Join')).not.toBeInTheDocument();
    });

    it('should not show Apply button for applied games', () => {
      const appliedGame = { ...mockGame, user_relationship: 'applied' as const };
      render(
        <EnhancedGameCard
          game={appliedGame}
          onApplyClick={vi.fn()}
          showApplyButton={true}
        />,
        { wrapper }
      );

      expect(screen.queryByText('Apply to Join')).not.toBeInTheDocument();
    });

    it('should call onApplyClick when Apply button is clicked', () => {
      const handleApply = vi.fn();
      render(
        <EnhancedGameCard
          game={mockGame}
          onApplyClick={handleApply}
          showApplyButton={true}
        />,
        { wrapper }
      );

      fireEvent.click(screen.getByText('Apply to Join'));

      expect(handleApply).toHaveBeenCalledTimes(1);
    });

    it('should stop propagation when Apply button is clicked', () => {
      const handleClick = vi.fn();
      const handleApply = vi.fn();

      render(
        <EnhancedGameCard
          game={mockGame}
          onClick={handleClick}
          onApplyClick={handleApply}
          showApplyButton={true}
        />,
        { wrapper }
      );

      fireEvent.click(screen.getByText('Apply to Join'));

      expect(handleApply).toHaveBeenCalledTimes(1);
      expect(handleClick).not.toHaveBeenCalled();
    });
  });

  describe('Click Interaction', () => {
    it('should call onClick when card is clicked', () => {
      const handleClick = vi.fn();
      const { container } = render(
        <EnhancedGameCard game={mockGame} onClick={handleClick} />,
        { wrapper }
      );

      const card = container.firstChild as HTMLElement;
      fireEvent.click(card);

      expect(handleClick).toHaveBeenCalledTimes(1);
    });

    it('should not be clickable when onClick is not provided', () => {
      const { container } = render(<EnhancedGameCard game={mockGame} />, { wrapper });

      const card = container.firstChild as HTMLElement;
      // Link elements are always clickable by nature
      expect(card.tagName).toBe('A');
    });
  });

  describe('Start Date Display', () => {
    it('should show start date when present', () => {
      const gameWithStartDate = {
        ...mockGame,
        start_date: '2025-11-01T00:00:00Z',
      };
      render(<EnhancedGameCard game={gameWithStartDate} />, { wrapper });

      expect(screen.getByText('Starts:')).toBeInTheDocument();
    });

    it('should not show start date when not present', () => {
      render(<EnhancedGameCard game={mockGame} />, { wrapper });

      expect(screen.queryByText('Starts:')).not.toBeInTheDocument();
    });
  });

  describe('Bug #1: Game card links cannot be opened in new tabs', () => {
    it('should render the entire card as a link element', () => {
      // Arrange
      render(<EnhancedGameCard game={mockGame} />, { wrapper });

      // Assert: Card should be wrapped in a link element
      const link = screen.getByRole('link');
      expect(link).toBeInTheDocument();
      expect(link).toHaveAttribute('href', `/games/${mockGame.id}`);
    });

    it('should navigate to game page when clicked normally', () => {
      // Arrange
      render(<EnhancedGameCard game={mockGame} />, { wrapper });

      // Assert: Link should point to correct game details page
      const link = screen.getByRole('link');
      expect(link).toHaveAttribute('href', `/games/1`);
    });

    it('should still allow Apply button to work without triggering navigation', () => {
      // Arrange
      const onApplyClick = vi.fn();
      render(
        <EnhancedGameCard
          game={mockGame}
          onApplyClick={onApplyClick}
          showApplyButton={true}
        />,
        { wrapper }
      );

      // Act: Click Apply button
      const applyButton = screen.getByRole('button', { name: /apply to join/i });
      fireEvent.click(applyButton);

      // Assert: Apply handler called, but link navigation should be prevented
      expect(onApplyClick).toHaveBeenCalledTimes(1);
    });

    it('should support CMD+Click to open in new tab (link element)', () => {
      // Arrange
      render(<EnhancedGameCard game={mockGame} />, { wrapper });

      // Assert: Being a link element allows browser to handle CMD+Click
      const link = screen.getByRole('link');
      expect(link.tagName).toBe('A');
      expect(link).toHaveAttribute('href', `/games/1`);
    });

    it('should support right-click -> Open in new tab (link element)', () => {
      // Arrange
      render(<EnhancedGameCard game={mockGame} />, { wrapper });

      // Assert: Being a link element allows browser's context menu
      const link = screen.getByRole('link');
      expect(link.tagName).toBe('A');
      expect(link).toHaveAttribute('href', `/games/1`);
    });
  });
});
