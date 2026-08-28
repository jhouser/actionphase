import { useAuth } from '../contexts/AuthContext';
import type { EnrichedGameListItem } from '../types/games';
import { EnhancedGameCard } from './EnhancedGameCard';
import { Card, Alert } from './ui';

function GameCardSkeleton() {
  return (
    <div className="surface-base rounded-lg shadow-md border-2 border-theme-default animate-pulse">
      <div className="p-4 border-b border-theme-default">
        <div className="h-6 surface-raised rounded w-3/4 mb-3"></div>
        <div className="flex gap-2">
          <div className="h-5 surface-raised rounded-full w-16"></div>
          <div className="h-5 surface-raised rounded-full w-20"></div>
        </div>
      </div>
      <div className="p-4">
        <div className="h-4 surface-raised rounded w-full mb-2"></div>
        <div className="h-4 surface-raised rounded w-5/6 mb-4"></div>
        <div className="grid grid-cols-2 gap-2">
          <div className="h-4 surface-raised rounded w-24"></div>
          <div className="h-4 surface-raised rounded w-20"></div>
        </div>
      </div>
    </div>
  );
}

interface GamesListProps {
  games: EnrichedGameListItem[];
  loading: boolean;
  error: string | null;
  onGameClick?: (game: EnrichedGameListItem) => void;
  onApplyToGame?: (game: EnrichedGameListItem) => void;
}

export const GamesList = ({
  games,
  loading,
  error,
  onGameClick,
  onApplyToGame,
}: GamesListProps) => {
  const { currentUser, isCheckingAuth } = useAuth();

  if (loading) {
    return (
      <Card variant="elevated" padding="lg" data-testid="games-list">
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {Array.from({ length: 6 }).map((_, i) => (
            <GameCardSkeleton key={i} />
          ))}
        </div>
      </Card>
    );
  }

  if (error) {
    return (
      <Card variant="elevated" padding="lg">
        <Alert variant="danger" title="Error Loading Games">
          {error}
        </Alert>
      </Card>
    );
  }

  if (!games || games.length === 0) {
    return (
      <Card variant="elevated" padding="lg">
        <div className="text-center">
          <div className="text-content-tertiary text-4xl mb-4">🎲</div>
          <p className="text-content-primary text-lg">No games match your current filters.</p>
          <p className="text-content-tertiary text-sm mt-2">Try adjusting your filter criteria.</p>
        </div>
      </Card>
    );
  }

  // Determine if the Apply button should be shown for a game
  const shouldShowApplyButton = (game: EnrichedGameListItem) => {
    if (!onApplyToGame || isCheckingAuth) return false;
    if (game.state !== 'recruitment') return false;
    if (game.gm_user_id === currentUser?.id) return false;
    return true;
  };

  return (
    <Card variant="elevated" padding="lg" data-testid="games-list">
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        {games.map((game) => (
          <EnhancedGameCard
            key={game.id}
            game={game}
            onClick={onGameClick ? () => onGameClick(game) : undefined}
            onApplyClick={onApplyToGame ? () => onApplyToGame(game) : undefined}
            showApplyButton={shouldShowApplyButton(game)}
            data-testid={`game-card-${game.id}`}
          />
        ))}
      </div>
    </Card>
  );
};
