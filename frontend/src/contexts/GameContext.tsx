import React, { createContext, useContext, useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { apiClient } from '../lib/api';
import type { GameWithDetails, GameParticipant } from '../types/games';
import type { Character } from '../types/characters';
import { useAuth } from './AuthContext';
import { useAdminMode } from './AdminModeContext';
import { computeGamePermissions, resolveUserRole } from '../lib/gamePermissions';
import type { UserGameRole } from '../lib/gamePermissions';
import { logger } from '@/services/LoggingService';

// Re-exported for the many components that already import it from here. The
// definition lives with the permission rules in lib/gamePermissions.
export type { UserGameRole } from '../lib/gamePermissions';

interface GameContextValue {
  // Game data
  gameId: number;
  game: GameWithDetails | null;
  participants: GameParticipant[];

  // Loading states
  isLoadingGame: boolean;
  isLoadingParticipants: boolean;
  isLoadingCharacters: boolean;
  isLoadingAllCharacters: boolean;

  // Current user's role and permissions
  userRole: UserGameRole;
  isGM: boolean;
  isParticipant: boolean;
  isInGame: boolean; // True if user has any role (including audience)
  isAudience: boolean; // Audience-level ACCESS, not identity — see below
  canEditGame: boolean;

  // User's characters
  userCharacters: Character[];

  // All game characters (all participants' characters, filtered by backend permissions)
  allGameCharacters: Character[];

  // Current phase ID
  currentPhaseId: number | null;

  // Character ownership checker
  isUserCharacter: (characterId: number) => boolean;

  // Refresh functions
  refetchGameData: () => Promise<void>;
  refetchAllGameCharacters: () => Promise<void>;
}

const GameContext = createContext<GameContextValue | undefined>(undefined);

interface GameProviderProps {
  gameId: number;
  children: React.ReactNode;
}

export function GameProvider({ gameId, children }: GameProviderProps) {
  const { currentUser } = useAuth();
  const { adminModeEnabled } = useAdminMode();
  const currentUserId = currentUser?.id;

  // Fetch game details
  const {
    data: game,
    isLoading: isLoadingGame,
    refetch: refetchGame,
  } = useQuery({
    queryKey: ['gameDetails', gameId],
    queryFn: async () => {
      logger.debug('Fetching game details', { gameId });
      const response = await apiClient.games.getGameWithDetails(gameId);
      logger.debug('Game details loaded', { gameId, gameTitle: response.data.title, state: response.data.state });
      return response.data;
    },
    enabled: !!gameId,
    staleTime: 30000, // Cache for 30 seconds
  });

  // Fetch game participants
  const {
    data: participants,
    isLoading: isLoadingParticipants,
    refetch: refetchParticipants,
  } = useQuery({
    queryKey: ['gameParticipants', gameId],
    queryFn: async () => {
      logger.debug('Fetching game participants', { gameId });
      const response = await apiClient.games.getGameParticipants(gameId);
      logger.debug('Participants loaded', { gameId, participantCount: response.data?.length || 0 });
      return response.data || [];
    },
    enabled: !!gameId,
    staleTime: 30000,
  });

  // Fetch user's controllable characters
  const {
    data: userCharacters,
    isLoading: isLoadingCharacters,
    refetch: refetchCharacters,
  } = useQuery({
    queryKey: ['userControllableCharacters', gameId],
    queryFn: async () => {
      logger.debug('Fetching controllable characters', { gameId, currentUserId });
      const response = await apiClient.characters.getUserControllableCharacters(gameId);
      logger.debug('User characters loaded', { gameId, characterCount: response.data?.length || 0 });
      return response.data || [];
    },
    enabled: !!gameId && !!currentUserId,
    staleTime: 30000,
  });

  // Fetch all game characters (backend applies permission filtering)
  const {
    data: allGameCharacters,
    isLoading: isLoadingAllCharacters,
    refetch: refetchAllCharacters,
  } = useQuery({
    queryKey: ['gameCharacters', gameId],
    queryFn: async () => {
      logger.debug('Fetching all game characters', { gameId });
      const response = await apiClient.characters.getGameCharacters(gameId);
      logger.debug('All game characters loaded', { gameId, characterCount: response.data?.length || 0 });
      return response.data || [];
    },
    enabled: !!gameId,
    staleTime: 30000,
  });

  // Compute user's role (shared with useGamePermissions — see lib/gamePermissions)
  const userRole: UserGameRole = useMemo(
    () => resolveUserRole(currentUserId, game?.gm_user_id, participants),
    [currentUserId, game?.gm_user_id, participants]
  );

  const isAdminActingAsGM = adminModeEnabled && !!currentUser?.is_admin;

  // All permission rules live in lib/gamePermissions so this provider and the
  // standalone useGamePermissions hook cannot drift apart.
  const permissions = useMemo(
    () => computeGamePermissions({ userRole, gameState: game?.state, isAdminActingAsGM }),
    [userRole, game?.state, isAdminActingAsGM]
  );

  // NOTE: this context has always exported isGM meaning "has GM authority",
  // co-GM included — its many consumers gate GM affordances on it. So it maps to
  // hasGMPowers, not the shared isGM (which is primary-GM identity). canEditGame
  // stays owner-only, which is the distinction that makes the two differ.
  const { hasGMPowers: isGM, isParticipant, isInGame, hasAudienceAccess: isAudience, canEditGame } = permissions;

  // Character ownership checker
  const isUserCharacter = useMemo(() => {
    const userCharacterIds = new Set(userCharacters?.map(c => c.id) || []);
    return (characterId: number) => userCharacterIds.has(characterId);
  }, [userCharacters]);

  // Refresh all game data
  const refetchGameData = async () => {
    logger.debug('Refetching all game data', { gameId });
    await Promise.all([
      refetchGame(),
      refetchParticipants(),
      refetchCharacters(),
      refetchAllCharacters(),
    ]);
  };

  const refetchAllGameCharacters = async () => {
    await refetchAllCharacters();
  };

  const value: GameContextValue = {
    gameId,
    game: game || null,
    participants: participants || [],
    isLoadingGame,
    isLoadingParticipants,
    isLoadingCharacters,
    isLoadingAllCharacters,
    userRole,
    isGM,
    isParticipant,
    isInGame,
    isAudience,
    canEditGame,
    userCharacters: userCharacters || [],
    allGameCharacters: allGameCharacters || [],
    currentPhaseId: null, // TODO: Get current phase ID from phases endpoint
    isUserCharacter,
    refetchGameData,
    refetchAllGameCharacters,
  };

  logger.debug('GameContext state updated', {
    gameId,
    hasGame: !!game,
    gameState: game?.state,
    participantCount: participants?.length || 0,
    userRole,
    isGM,
    isParticipant,
    userCharacterCount: userCharacters?.length || 0,
    allGameCharacterCount: allGameCharacters?.length || 0,
    currentUserId,
  });

  return <GameContext.Provider value={value}>{children}</GameContext.Provider>;
}

// eslint-disable-next-line react-refresh/only-export-components
export function useGameContext() {
  const context = useContext(GameContext);
  if (context === undefined) {
    throw new Error('useGameContext must be used within a GameProvider');
  }
  return context;
}

// Optional hook that returns null if not in a GameContext
// eslint-disable-next-line react-refresh/only-export-components
export function useOptionalGameContext() {
  return useContext(GameContext) || null;
}
