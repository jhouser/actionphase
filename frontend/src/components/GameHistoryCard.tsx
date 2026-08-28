import { Link } from 'react-router-dom';
import type { UserGame } from '../types/user-profiles';
import { Card, CardBody, Badge } from './ui';

interface GameHistoryCardProps {
  game: UserGame;
}

/**
 * Get badge variant based on game state
 */
function getStateBadgeVariant(state: string): 'success' | 'warning' | 'danger' | 'neutral' {
  switch (state) {
    case 'in_progress':
      return 'success';
    case 'recruitment':
    case 'character_creation':
      return 'warning';
    case 'completed':
      return 'neutral';
    case 'cancelled':
    case 'paused':
      return 'danger';
    default:
      return 'neutral';
  }
}

/**
 * Format game state for display
 */
function formatGameState(state: string): string {
  return state
    .split('_')
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(' ');
}

/**
 * Get badge variant based on user role
 */
function getRoleBadgeVariant(role: string): 'primary' | 'success' | 'secondary' {
  switch (role) {
    case 'gm':
      return 'primary';
    case 'co_gm':
      return 'success';
    default:
      return 'secondary';
  }
}

/**
 * Format user role for display
 */
function formatUserRole(role: string): string {
  if (role === 'gm') return 'GM';
  if (role === 'co_gm') return 'Co-GM';
  return role.charAt(0).toUpperCase() + role.slice(1);
}

/**
 * GameHistoryCard - Individual game card in user's game history
 *
 * Features:
 * - Game title links to game page
 * - State and role badges
 * - GM information
 * - Privacy-aware character display (hidden for anonymous games)
 * - Date range display
 * - Hover elevation effect
 */
export function GameHistoryCard({ game }: GameHistoryCardProps) {
  const formatMonthYear = (dateStr: string) =>
    new Date(dateStr).toLocaleDateString('en-US', { month: 'short', year: 'numeric' });

  // Use actual start/end dates when available; fall back to created_at/updated_at
  const startDateStr = game.start_date ?? game.created_at;
  const startLabel = formatMonthYear(startDateStr);

  // Only show an end label when the game is finished and has an explicit end_date
  const endLabel =
    game.end_date && (game.state === 'completed' || game.state === 'cancelled')
      ? formatMonthYear(game.end_date)
      : null;

  return (
    <Card
      variant="bordered"
      padding="md"
      className="transition-all hover:shadow-lg hover:-translate-y-1"
    >
      <CardBody>
        {/* Game Title */}
        <Link
          to={`/games/${game.game_id}`}
          className="block group"
        >
          <h3 className="text-lg font-bold text-content-primary group-hover:text-interactive-primary transition-colors line-clamp-2">
            {game.title}
          </h3>
        </Link>

        {/* Badges Row */}
        <div className="flex flex-wrap gap-2 mt-3">
          <Badge variant={getStateBadgeVariant(game.state)}>
            {formatGameState(game.state)}
          </Badge>

          <Badge variant={getRoleBadgeVariant(game.user_role)}>
            {formatUserRole(game.user_role)}
          </Badge>

          {game.is_anonymous && (
            <Badge variant="warning">
              Anonymous Game
            </Badge>
          )}
        </div>

        {/* GM Information */}
        <div className="mt-4 text-sm text-content-secondary">
          <span className="font-medium">GM:</span> @{game.gm_username}
        </div>

        {/* Characters (only for non-anonymous games) */}
        {!game.is_anonymous && game.characters.length > 0 && (
          <div className="mt-4">
            <div className="text-sm font-medium text-content-primary mb-2">
              {game.characters.length === 1 ? 'Character:' : 'Characters:'}
            </div>
            <div className="flex flex-wrap gap-3">
              {game.characters.map((character) => (
                <div
                  key={character.id}
                  className="flex items-center gap-2"
                >
                  {character.avatar_url ? (
                    <img
                      src={character.avatar_url}
                      alt={character.name}
                      className="w-8 h-8 rounded-full object-cover ring-2 ring-border-primary"
                    />
                  ) : (
                    <div className="w-8 h-8 rounded-full bg-surface-secondary flex items-center justify-center ring-2 ring-border-primary">
                      <span className="text-xs font-bold text-content-secondary">
                        {character.name.charAt(0).toUpperCase()}
                      </span>
                    </div>
                  )}
                  <span className="text-sm text-content-primary font-medium">
                    {character.name}
                  </span>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Date Range */}
        <div className="mt-4 pt-4 border-t border-theme-default text-xs text-content-secondary">
          {endLabel ? `${startLabel} → ${endLabel}` : startLabel}
        </div>
      </CardBody>
    </Card>
  );
}
