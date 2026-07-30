import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { GameActions } from '../GameActions';
import type { Game, GameApplication } from '../../types/games';

const baseGame: Game = {
  id: 1,
  title: 'Test Game',
  description: '',
  gm_user_id: 99,
  state: 'in_progress',
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
};

const defaultProps = {
  game: baseGame,
  isGM: false,
  canEditGame: false,
  isCheckingAuth: false,
  isParticipant: false,
  isInGame: false,
  userRole: 'none' as const,
  userApplication: null,
  actionLoading: false,
  stateActions: [],
  onEditGame: vi.fn(),
  onStateChange: vi.fn(),
  onApplyToGame: vi.fn(),
  onWithdrawApplication: vi.fn(),
  onLeaveGame: vi.fn(),
};

describe('GameActions - Join as Audience button visibility', () => {
  it('shows Join as Audience button during character_creation', () => {
    render(<GameActions {...defaultProps} game={{ ...baseGame, state: 'character_creation' }} />);
    expect(screen.getByTestId('join-as-audience-button')).toBeInTheDocument();
  });

  it('shows Join as Audience button during in_progress', () => {
    render(<GameActions {...defaultProps} game={{ ...baseGame, state: 'in_progress' }} />);
    expect(screen.getByTestId('join-as-audience-button')).toBeInTheDocument();
  });

  it('does not show Join as Audience button on completed games', () => {
    render(<GameActions {...defaultProps} game={{ ...baseGame, state: 'completed' }} />);
    expect(screen.queryByTestId('join-as-audience-button')).not.toBeInTheDocument();
  });

  it('does not show Join as Audience button if user is already in game', () => {
    render(<GameActions {...defaultProps} isInGame game={{ ...baseGame, state: 'in_progress' }} />);
    expect(screen.queryByTestId('join-as-audience-button')).not.toBeInTheDocument();
  });

  it('does not show Join as Audience button for the GM', () => {
    render(<GameActions {...defaultProps} isGM game={{ ...baseGame, state: 'in_progress' }} />);
    expect(screen.queryByTestId('join-as-audience-button')).not.toBeInTheDocument();
  });

  it('renders Join as Audience button with warning variant', () => {
    render(<GameActions {...defaultProps} game={{ ...baseGame, state: 'in_progress' }} />);
    const button = screen.getByTestId('join-as-audience-button');
    // Warning variant applies bg-semantic-warning class (via tv() utility)
    expect(button.className).toMatch(/semantic-warning/);
  });
});

describe('GameActions - stale application after leaving (regression)', () => {
  // Regression test: a user whose audience application was approved and who then left the
  // game was left with an 'approved' application record but no active participant record
  // (isInGame: false). The UI hid both the Withdraw button (which required status ===
  // 'pending') and the Apply/Join buttons (which hid whenever any application existed,
  // regardless of status), leaving the user with no way to fix their own stuck state.
  const staleApprovedApplication: GameApplication = {
    id: 1,
    game_id: baseGame.id,
    user_id: 42,
    role: 'audience',
    status: 'approved',
    applied_at: '2024-01-01T00:00:00Z',
  };

  it('shows Withdraw button for a stale approved application when not in game', () => {
    render(
      <GameActions
        {...defaultProps}
        isInGame={false}
        userApplication={staleApprovedApplication}
        game={{ ...baseGame, state: 'in_progress' }}
      />
    );
    expect(screen.getByTestId('withdraw-application-button')).toBeInTheDocument();
  });

  it('shows Join as Audience button despite a stale approved application when not in game', () => {
    render(
      <GameActions
        {...defaultProps}
        isInGame={false}
        userApplication={staleApprovedApplication}
        game={{ ...baseGame, state: 'in_progress' }}
      />
    );
    expect(screen.getByTestId('join-as-audience-button')).toBeInTheDocument();
  });

  it('does not show Withdraw button for a live approved application while still in game', () => {
    render(
      <GameActions
        {...defaultProps}
        isInGame
        userApplication={staleApprovedApplication}
        game={{ ...baseGame, state: 'in_progress' }}
      />
    );
    expect(screen.queryByTestId('withdraw-application-button')).not.toBeInTheDocument();
  });

  it('still hides Join/Apply buttons for a genuinely pending application', () => {
    const pendingApplication: GameApplication = { ...staleApprovedApplication, status: 'pending' };
    render(
      <GameActions
        {...defaultProps}
        isInGame={false}
        userApplication={pendingApplication}
        game={{ ...baseGame, state: 'recruitment' }}
      />
    );
    expect(screen.queryByTestId('join-as-audience-button')).not.toBeInTheDocument();
    expect(screen.queryByTestId('apply-button-1')).not.toBeInTheDocument();
    expect(screen.getByTestId('withdraw-application-button')).toBeInTheDocument();
  });
});

describe('GameActions - rejected application is terminal, not stale', () => {
  // A rejection is a terminal GM decision, not a stale leftover. A rejected user must not be
  // able to re-apply (they'd just re-apply repeatedly) or withdraw the rejection (they can't
  // un-reject themselves). The backend enforces this; the UI must not offer either action.
  const rejectedApplication: GameApplication = {
    id: 1,
    game_id: baseGame.id,
    user_id: 42,
    role: 'audience',
    status: 'rejected',
    applied_at: '2024-01-01T00:00:00Z',
  };

  it('does not show Withdraw button for a rejected application', () => {
    render(
      <GameActions
        {...defaultProps}
        isInGame={false}
        userApplication={rejectedApplication}
        game={{ ...baseGame, state: 'in_progress' }}
      />
    );
    expect(screen.queryByTestId('withdraw-application-button')).not.toBeInTheDocument();
  });

  it('does not show Join as Audience button for a rejected application', () => {
    render(
      <GameActions
        {...defaultProps}
        isInGame={false}
        userApplication={rejectedApplication}
        game={{ ...baseGame, state: 'in_progress' }}
      />
    );
    expect(screen.queryByTestId('join-as-audience-button')).not.toBeInTheDocument();
  });

  it('does not show Apply button for a rejected application during recruitment', () => {
    render(
      <GameActions
        {...defaultProps}
        isInGame={false}
        userApplication={rejectedApplication}
        game={{ ...baseGame, state: 'recruitment' }}
      />
    );
    expect(screen.queryByTestId('apply-button-1')).not.toBeInTheDocument();
    expect(screen.queryByTestId('withdraw-application-button')).not.toBeInTheDocument();
  });
});
