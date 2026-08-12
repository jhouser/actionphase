import { useQuery } from '@tanstack/react-query';
import { apiClient } from '../lib/api';
import { useAuth } from '../contexts/AuthContext';
import { useAdminMode } from '../contexts/AdminModeContext';
import { computeGamePermissions, resolveUserRole } from '../lib/gamePermissions';
import type { UserGameRole } from '../lib/gamePermissions';
import type { GameWithDetails, GameParticipant } from '../types/games';

export interface GamePermissions {
  // Game data
  game: GameWithDetails | null;
  participants: GameParticipant[];

  // Loading states
  isLoading: boolean;

  // User's role and permissions
  userRole: UserGameRole;
  /** Primary GM identity only (game.gm_user_id). For GM-level authority use hasGMPowers. */
  isGM: boolean;
  isCoGM: boolean;
  /** GM or co-GM — the authority level the backend grants to both. */
  hasGMPowers: boolean;
  isPlayer: boolean;
  /** Audience-level read ACCESS (hasAudienceAccess), which a completed game grants to everyone. */
  isAudience: boolean;
  isParticipant: boolean;
  canEditGame: boolean;
  canManagePhases: boolean;
  canViewAllActions: boolean;

  // User identification
  currentUserId: number | null;
}

/**
 * Hook to get game permissions for the current user.
 * This hook provides comprehensive permission checks and role information
 * for a specific game without requiring a GameContext.
 *
 * @param gameId - The ID of the game to check permissions for
 * @returns GamePermissions object with role and permission information
 */
export function useGamePermissions(gameId: number): GamePermissions {
  const { currentUser } = useAuth();
  const { adminModeEnabled } = useAdminMode();
  const currentUserId = currentUser?.id || null;

  // Fetch game details
  const {
    data: game,
    isLoading: isLoadingGame,
  } = useQuery({
    queryKey: ['gameDetails', gameId],
    queryFn: async () => {
      const response = await apiClient.games.getGameWithDetails(gameId);
      return response.data;
    },
    enabled: !!gameId,
    staleTime: 30000,
  });

  // Fetch participants
  const {
    data: participants,
    isLoading: isLoadingParticipants,
  } = useQuery({
    queryKey: ['gameParticipants', gameId],
    queryFn: async () => {
      const response = await apiClient.games.getGameParticipants(gameId);
      return response.data || [];
    },
    enabled: !!gameId,
    staleTime: 30000,
  });

  // Role resolution and every permission rule are shared with GameProvider via
  // lib/gamePermissions, so the two cannot drift apart.
  const userRole = resolveUserRole(currentUserId, game?.gm_user_id, participants);
  const isAdminActingAsGM = adminModeEnabled && !!currentUser?.is_admin;

  const {
    isGM,
    isCoGM,
    hasGMPowers,
    isPlayer,
    hasAudienceAccess,
    isParticipant,
    canEditGame,
    canManagePhases,
    canViewAllActions,
  } = computeGamePermissions({ userRole, gameState: game?.state, isAdminActingAsGM });

  return {
    game: game || null,
    participants: participants || [],
    isLoading: isLoadingGame || isLoadingParticipants,
    userRole,
    isGM,
    isCoGM,
    hasGMPowers,
    isPlayer,
    isAudience: hasAudienceAccess,
    isParticipant,
    canEditGame,
    canManagePhases,
    canViewAllActions,
    currentUserId,
  };
}
