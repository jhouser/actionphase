import { useState, useRef, useEffect } from 'react';
import type { Game, GameApplication, GameState } from '../types/games';
import type { UserGameRole } from '../contexts/GameContext';
import { Button } from './ui';

interface StateAction {
  label: string;
  state: GameState;
  color: string;
}

interface GameActionsProps {
  game: Game;
  isGM: boolean;
  canEditGame: boolean;
  isCheckingAuth: boolean;
  isParticipant: boolean;
  isInGame: boolean; // Whether user has any role in the game (including audience)
  userRole: UserGameRole;
  userApplication: GameApplication | null;
  hasPendingAudienceApplication?: boolean; // Whether user has a pending audience application
  actionLoading: boolean;
  stateActions: StateAction[];
  onEditGame: () => void;
  onStateChange: (state: GameState) => void;
  onApplyToGame: () => void;
  onWithdrawApplication: () => void;
  onLeaveGame: () => void;
  onDeleteGame?: () => void;
  // Controls which portion to render — allows splitting player CTAs from the GM kebab menu
  slot?: 'player-actions' | 'menu-actions' | 'all';
}

/**
 * GameActions - Compact kebab menu for game actions
 *
 * Shows important player actions as buttons (Apply/Withdraw)
 * and editor/GM actions in a dropdown menu
 */
export function GameActions({
  game,
  isGM,
  canEditGame,
  isCheckingAuth,
  isParticipant: _isParticipant,
  isInGame,
  userRole: _userRole,
  userApplication,
  hasPendingAudienceApplication: _hasPendingAudienceApplication = false,
  actionLoading,
  stateActions,
  onEditGame,
  onStateChange,
  onApplyToGame,
  onWithdrawApplication,
  onLeaveGame: _onLeaveGame,
  onDeleteGame,
  slot = 'all',
}: GameActionsProps) {
  const [showMenu, setShowMenu] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  // Close menu when clicking outside
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        setShowMenu(false);
      }
    }

    if (showMenu) {
      document.addEventListener('mousedown', handleClickOutside);
      return () => document.removeEventListener('mousedown', handleClickOutside);
    }
  }, [showMenu]);

  // Count menu items to determine if we should show the menu
  const hasEditAction = canEditGame && game.state !== 'completed' && game.state !== 'cancelled';
  const hasStateActions = isGM && stateActions.length > 0;
  const hasDeleteAction = isGM && game.state === 'cancelled' && onDeleteGame;
  const hasMenuItems = hasEditAction || hasStateActions || hasDeleteAction;

  // A stale application is a leftover 'approved' record with no active membership behind it —
  // e.g. (for accounts affected before the backend fix that now deletes an audience application
  // on approval) an approved audience row left behind after the member left. It represents no
  // real relationship, so treat it as if there's no application: don't let it block re-applying,
  // and let it be withdrawn/cleared.
  //
  // A 'rejected' application is NOT stale — it's a terminal GM decision. It must keep blocking
  // re-applying (otherwise a rejected user could re-apply repeatedly) and must not be withdrawable
  // (a user cannot un-reject themselves). The backend enforces both; we mirror it here so the UI
  // never offers an action that would just fail.
  const isStaleApplication = !!userApplication && !isInGame && userApplication.status === 'approved';
  const hasBlockingApplication = !!userApplication && !isStaleApplication;

  // Player action buttons (always visible when applicable)
  const showApplyButton = !isGM && !isCheckingAuth && !isInGame && game.state === 'recruitment' && !hasBlockingApplication;
  const showWithdrawButton = !isGM && userApplication &&
    ((userApplication.status === 'pending' && game.state === 'recruitment') || isStaleApplication);

  // Audience can join during active game phases, but not after completion (completed games are open to all)
  const showJoinAsAudienceButton = !isGM && !isCheckingAuth && !isInGame &&
    (game.state === 'character_creation' || game.state === 'in_progress') &&
    !hasBlockingApplication;

  const showPlayerActions = slot === 'all' || slot === 'player-actions';
  const showMenuActions = slot === 'all' || slot === 'menu-actions';

  const hasPlayerActions = showApplyButton || showWithdrawButton || showJoinAsAudienceButton;
  if (showPlayerActions && !showMenuActions && !hasPlayerActions) return null;
  if (showMenuActions && !showPlayerActions && !hasMenuItems) return null;

  return (
    <div className="flex items-center gap-2">
      {/* Player Actions */}
      {showPlayerActions && showApplyButton && (
        <Button
          variant="primary"
          size="sm"
          onClick={onApplyToGame}
          disabled={actionLoading}
          data-testid={`apply-button-${game.id}`}
        >
          Apply to Join
        </Button>
      )}

      {showPlayerActions && showWithdrawButton && (
        <Button
          variant="warning"
          size="sm"
          onClick={onWithdrawApplication}
          disabled={actionLoading}
          data-testid="withdraw-application-button"
        >
          Withdraw Application
        </Button>
      )}

      {showPlayerActions && showJoinAsAudienceButton && (
        <Button
          variant="warning"
          size="sm"
          onClick={onApplyToGame}
          disabled={actionLoading}
          data-testid="join-as-audience-button"
        >
          Join as Audience
        </Button>
      )}

      {/* Kebab Menu for GM/Editor Actions */}
      {showMenuActions && hasMenuItems && (
        <div className="relative" ref={menuRef}>
          <button
            onClick={() => setShowMenu(!showMenu)}
            className="p-2 rounded hover:bg-surface-raised transition-colors text-content-primary"
            aria-label="Game actions"
            data-testid="game-actions-menu"
          >
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 5v.01M12 12v.01M12 19v.01M12 6a1 1 0 110-2 1 1 0 010 2zm0 7a1 1 0 110-2 1 1 0 010 2zm0 7a1 1 0 110-2 1 1 0 010 2z" />
            </svg>
          </button>

          {showMenu && (
            <div className="absolute right-0 top-full mt-1 w-48 rounded-lg border border-theme-default surface-raised shadow-xl py-1 z-50">
              {/* State Change Actions - Phase transitions first */}
              {hasStateActions && stateActions.map((action) => (
                <button
                  key={action.state}
                  onClick={() => {
                    onStateChange(action.state);
                    setShowMenu(false);
                  }}
                  disabled={actionLoading}
                  className="w-full text-left px-4 py-2 text-sm text-content-primary hover:bg-surface-raised transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                  data-testid={`${action.state}-button`}
                >
                  {action.label}
                </button>
              ))}

              {/* Edit Game - After phase transitions */}
              {hasEditAction && (
                <>
                  {hasStateActions && (
                    <div className="border-t border-border-primary my-1" />
                  )}
                  <button
                    onClick={() => {
                      onEditGame();
                      setShowMenu(false);
                    }}
                    disabled={actionLoading}
                    className="w-full text-left px-4 py-2 text-sm text-content-primary hover:bg-surface-raised transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    Edit Game
                  </button>
                </>
              )}

              {/* Delete Game */}
              {hasDeleteAction && (
                <>
                  {(hasEditAction || hasStateActions) && (
                    <div className="border-t border-border-primary my-1" />
                  )}
                  <button
                    onClick={() => {
                      onDeleteGame?.();
                      setShowMenu(false);
                    }}
                    disabled={actionLoading}
                    className="w-full text-left px-4 py-2 text-sm text-semantic-danger hover:bg-semantic-danger-subtle transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                    data-testid="delete-game-button"
                  >
                    Delete Game
                  </button>
                </>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
