import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { GAME_STATE_LABELS, GAME_STATE_COLORS } from '../types/games';
import type { GameListItem, GameWithDetails, GameParticipant } from '../types/games';
import { Badge } from './ui';
import { format } from 'date-fns';

interface GameHeaderProps {
  game: GameListItem | GameWithDetails;
  participants?: GameParticipant[];
  playerCount?: string; // e.g., "3/5" or "3" for current players / max
  pinnedAction?: React.ReactNode; // Always visible on mobile, even when collapsed (primary player CTAs)
  actionMenu?: React.ReactNode; // Hidden on mobile when collapsed (GM/editor actions)
  isCollapsed?: boolean; // External collapse state (for parent to control what gets hidden)
  onToggleCollapse?: () => void; // Callback when collapse button is clicked
}

/**
 * GameHeader - Compact single-line game header
 *
 * Displays:
 * - Line 1: Title + Status Badge + Action Menu (right)
 * - Line 2: GM • Genre • Players (compact metadata chips)
 */
export function GameHeader({
  game,
  participants = [],
  playerCount,
  pinnedAction,
  actionMenu,
  isCollapsed: externalIsCollapsed,
  onToggleCollapse
}: GameHeaderProps) {
  // Find co-GM from participants
  const coGM = participants.find(p => p.role === 'co_gm');

  // Internal collapsible state for mobile (persisted in localStorage)
  // Only used if parent doesn't provide isCollapsed/onToggleCollapse
  const [internalIsCollapsed, setInternalIsCollapsed] = useState(() => {
    const stored = localStorage.getItem('gameHeaderCollapsed');
    return stored === 'true';
  });

  // Use external state if provided, otherwise use internal state
  const isCollapsed = externalIsCollapsed !== undefined ? externalIsCollapsed : internalIsCollapsed;

  // Persist collapse state to localStorage (only for internal state)
  useEffect(() => {
    if (externalIsCollapsed === undefined) {
      localStorage.setItem('gameHeaderCollapsed', String(internalIsCollapsed));
    }
  }, [internalIsCollapsed, externalIsCollapsed]);

  const toggleCollapse = () => {
    if (onToggleCollapse) {
      onToggleCollapse();
    } else {
      setInternalIsCollapsed(!internalIsCollapsed);
    }
  };

  return (
    <div className="space-y-1">
      {/* Title + Status + Actions */}
      <div className="flex items-start md:items-center justify-between gap-4">
        {/* Mobile: Stack title and badge vertically, Desktop: Horizontal with truncate */}
        <div className="flex flex-col md:flex-row md:items-center gap-2 md:gap-3 min-w-0 flex-1">
          <div className="flex items-center gap-2 w-full md:w-auto">
            <h1 className="text-2xl font-bold text-content-primary break-words md:truncate flex-1 md:flex-none">{game.title}</h1>
          </div>
          {/* Badge - hidden when collapsed on mobile, always visible on desktop */}
          <div className={`self-start ${isCollapsed ? 'hidden md:block' : ''}`}>
            <Badge className={GAME_STATE_COLORS[game.state]} data-testid="game-status-badge" size="sm">
              {GAME_STATE_LABELS[game.state]}
            </Badge>
          </div>
        </div>
        {/* Pinned action - desktop only in title row (mobile renders below the collapsible block) */}
        {pinnedAction && (
          <div className="hidden md:block flex-shrink-0">
            {pinnedAction}
          </div>
        )}
        {/* Action menu + mobile collapse toggle, aligned on the same row */}
        <div className="flex items-center gap-1 flex-shrink-0">
          {actionMenu && (
            <div className={isCollapsed ? 'hidden md:block' : ''}>
              {actionMenu}
            </div>
          )}
          {/* Mobile-only collapse toggle button */}
          <button
            onClick={toggleCollapse}
            className="md:hidden flex-shrink-0 p-2 text-content-secondary hover:text-content-primary transition-colors"
            aria-label={isCollapsed ? 'Expand game details' : 'Collapse game details'}
          >
            <svg
              className={`w-5 h-5 transition-transform ${isCollapsed ? '' : 'rotate-180'}`}
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
            </svg>
          </button>
        </div>
      </div>

      {/* Metadata - Mobile: Stacked rows for better readability, Desktop: Single line */}
      {/* Mobile: Hidden when collapsed, Desktop: Always visible */}
      <div className={`text-sm text-content-secondary ${isCollapsed ? 'hidden md:block' : ''}`}>
          {/* Mobile: Stacked layout */}
          <div className="flex flex-col gap-1 md:hidden">
          {/* Row 1: GM info */}
          <div className="flex items-center gap-2 flex-wrap">
            <span>GM: <Link to={`/users/${game.gm_username}`} className="hover:underline">{game.gm_username}</Link></span>
            {coGM && (
              <>
                <span className="text-content-tertiary">•</span>
                <span>Co-GM: <Link to={`/users/${coGM.username}`} className="hover:underline">{coGM.username}</Link></span>
              </>
            )}
          </div>

          {/* Row 2: Game details */}
          {(game.genre || playerCount) && (
            <div className="flex items-center gap-2 flex-wrap">
              {game.genre && <span>Genre: {game.genre}</span>}
              {game.genre && playerCount && <span className="text-content-tertiary">•</span>}
              {playerCount && <span>{playerCount} Players</span>}
            </div>
          )}

          {/* Row 3: Dates */}
          {(game.start_date || game.end_date) && (
            <div className="flex items-center gap-2 flex-wrap">
              {game.start_date && <span>Start: {format(new Date(game.start_date), 'MMM d, yyyy')}</span>}
              {game.start_date && game.end_date && <span className="text-content-tertiary">•</span>}
              {game.end_date && <span>End: {format(new Date(game.end_date), 'MMM d, yyyy')}</span>}
            </div>
          )}
        </div>

        {/* Desktop: Single line (original layout) */}
        <div className="hidden md:flex md:items-center md:gap-2 md:flex-wrap">
          <span>GM: <Link to={`/users/${game.gm_username}`} className="hover:underline">{game.gm_username}</Link></span>
          {coGM && (
            <>
              <span className="text-content-tertiary">•</span>
              <span>Co-GM: <Link to={`/users/${coGM.username}`} className="hover:underline">{coGM.username}</Link></span>
            </>
          )}
          {game.genre && (
            <>
              <span className="text-content-tertiary">•</span>
              <span>Genre: {game.genre}</span>
            </>
          )}
          {playerCount && (
            <>
              <span className="text-content-tertiary">•</span>
              <span>{playerCount} Players</span>
            </>
          )}
          {game.start_date && (
            <>
              <span className="text-content-tertiary">•</span>
              <span>Start: {format(new Date(game.start_date), 'MMM d, yyyy')}</span>
            </>
          )}
          {game.end_date && (
            <>
              <span className="text-content-tertiary">•</span>
              <span>End: {format(new Date(game.end_date), 'MMM d, yyyy')}</span>
            </>
          )}
        </div>
      </div>

      {/* Pinned action - mobile only, always visible below header regardless of collapse state */}
      {/* On desktop, player CTAs are shown in the actionMenu slot in the title row */}
      {pinnedAction && (
        <div className="md:hidden mt-2">
          {pinnedAction}
        </div>
      )}
    </div>
  );
}
