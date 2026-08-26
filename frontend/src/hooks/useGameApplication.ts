import { useState, useEffect } from 'react';
import { apiClient } from '../lib/api';
import type { GameApplication } from '../types/games';
import { useToast } from '../contexts/ToastContext';
import { isPublicArchive } from '@/lib/gamePermissions';

interface UseGameApplicationOptions {
  gameId: number;
  isGM: boolean;
  isInGame: boolean; // True if user has any role in the game (including audience)
  currentUserId: number | null;
  isLoadingParticipants: boolean; // Prevent checking application while participants are loading
  refetchGameData: () => Promise<void>;
  gameState?: string;
}

export function useGameApplication({
  gameId,
  isGM,
  isInGame,
  currentUserId,
  isLoadingParticipants,
  refetchGameData,
  gameState,
}: UseGameApplicationOptions) {
  const { showError } = useToast();
  const [userApplication, setUserApplication] = useState<GameApplication | null>(null);
  const [actionLoading, setActionLoading] = useState(false);
  const [showApplyModal, setShowApplyModal] = useState(false);
  const [showWithdrawModal, setShowWithdrawModal] = useState(false);

  // Fetch user's application if not GM and not already in the game.
  // Skip once the game is a public archive (completed or epilogue) — a game
  // that is winding down is not accepting applications either way.
  // Wait for participants to load to avoid race condition.
  useEffect(() => {
    const fetchUserApplication = async () => {
      if (!isGM && !isInGame && !isLoadingParticipants && currentUserId && !isPublicArchive(gameState)) {
        try {
          const applicationResponse = await apiClient.games.getMyGameApplication(gameId);
          setUserApplication(applicationResponse.data);
        } catch {
          setUserApplication(null);
        }
      } else {
        setUserApplication(null);
      }
    };

    fetchUserApplication();
  }, [gameId, isGM, isInGame, isLoadingParticipants, currentUserId, gameState]);

  const refetchUserApplication = async () => {
    if (!isGM && !isInGame && currentUserId && !isPublicArchive(gameState)) {
      try {
        const applicationResponse = await apiClient.games.getMyGameApplication(gameId);
        setUserApplication(applicationResponse.data);
      } catch {
        setUserApplication(null);
      }
    }
  };

  const handleApplicationSubmitted = async () => {
    await refetchGameData();
    await refetchUserApplication();
  };

  const handleWithdrawApplication = () => {
    setShowWithdrawModal(true);
  };

  const confirmWithdrawApplication = async () => {
    try {
      setActionLoading(true);
      await apiClient.games.withdrawGameApplication(gameId);
      await refetchGameData();
      await refetchUserApplication();
    } catch (_err) {
      showError(_err instanceof Error ? _err.message : 'Failed to withdraw application');
    } finally {
      setActionLoading(false);
    }
  };

  return {
    userApplication,
    actionLoading,
    showApplyModal,
    setShowApplyModal,
    showWithdrawModal,
    setShowWithdrawModal,
    handleApplicationSubmitted,
    handleWithdrawApplication,
    confirmWithdrawApplication,
    refetchUserApplication,
  };
}
