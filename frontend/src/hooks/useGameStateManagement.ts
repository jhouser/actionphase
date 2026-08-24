import { useState } from 'react';
import { apiClient } from '../lib/api';
import type { GameState } from '../types/games';
import { useToast } from '../contexts/ToastContext';
import { logger } from '@/services/LoggingService';

interface UseGameStateManagementOptions {
  gameId: number;
  refetchGameData: () => Promise<void>;
}

interface StateAction {
  label: string;
  state: GameState;
  color: string;
}

export function useGameStateManagement({
  gameId,
  refetchGameData,
}: UseGameStateManagementOptions) {
  const { showError } = useToast();
  const [actionLoading, setActionLoading] = useState(false);
  const [showCompleteDialog, setShowCompleteDialog] = useState(false);
  const [showPauseDialog, setShowPauseDialog] = useState(false);
  const [showCancelDialog, setShowCancelDialog] = useState(false);
  const [showLeaveDialog, setShowLeaveDialog] = useState(false);

  const handleStateChange = async (newState: GameState) => {
    // Show confirmation dialog for completing game
    if (newState === 'completed') {
      logger.info('Showing complete game confirmation dialog', {
        gameId,
        targetState: newState,
      });
      setShowCompleteDialog(true);
      return;
    }

    // Show confirmation dialog for pausing game
    if (newState === 'paused') {
      logger.info('Showing pause game confirmation dialog', {
        gameId,
        targetState: newState,
      });
      setShowPauseDialog(true);
      return;
    }

    // Show confirmation dialog for cancelling game
    if (newState === 'cancelled') {
      logger.info('Showing cancel game confirmation dialog', {
        gameId,
        targetState: newState,
      });
      setShowCancelDialog(true);
      return;
    }

    try {
      setActionLoading(true);
      logger.info('Game state change attempt', {
        gameId,
        targetState: newState,
      });
      await apiClient.games.updateGameState(gameId, { state: newState });
      await refetchGameData();
      logger.info('Game state change successful', {
        gameId,
        newState,
      });
    } catch (_err) {
      logger.error('Game state change failed', {
        gameId,
        targetState: newState,
        error: _err,
      });
      showError(_err instanceof Error ? _err.message : 'Failed to update game state');
    } finally {
      setActionLoading(false);
    }
  };

  const handleConfirmComplete = async () => {
    try {
      setActionLoading(true);
      logger.info('Completing game confirmed', {
        gameId,
        targetState: 'completed',
      });
      await apiClient.games.updateGameState(gameId, { state: 'completed' });
      await refetchGameData();
      logger.info('Game completed successfully', {
        gameId,
      });
    } catch (_err) {
      logger.error('Failed to complete game', {
        gameId,
        error: _err,
      });
      showError(_err instanceof Error ? _err.message : 'Failed to complete game');
      throw _err; // Re-throw so dialog can handle it
    } finally {
      setActionLoading(false);
    }
  };

  const handleConfirmPause = async () => {
    try {
      setActionLoading(true);
      logger.info('Pausing game confirmed', {
        gameId,
        targetState: 'paused',
      });
      await apiClient.games.updateGameState(gameId, { state: 'paused' });
      await refetchGameData();
      logger.info('Game paused successfully', {
        gameId,
      });
    } catch (_err) {
      logger.error('Failed to pause game', {
        gameId,
        error: _err,
      });
      showError(_err instanceof Error ? _err.message : 'Failed to pause game');
      throw _err; // Re-throw so dialog can handle it
    } finally {
      setActionLoading(false);
    }
  };

  const handleConfirmCancel = async () => {
    try {
      setActionLoading(true);
      logger.info('Cancelling game confirmed', {
        gameId,
        targetState: 'cancelled',
      });
      await apiClient.games.updateGameState(gameId, { state: 'cancelled' });
      await refetchGameData();
      logger.info('Game cancelled successfully', {
        gameId,
      });
    } catch (_err) {
      logger.error('Failed to cancel game', {
        gameId,
        error: _err,
      });
      showError(_err instanceof Error ? _err.message : 'Failed to cancel game');
      throw _err; // Re-throw so dialog can handle it
    } finally {
      setActionLoading(false);
    }
  };

  const handleLeaveGame = () => {
    logger.info('Showing leave game confirmation dialog', {
      gameId,
    });
    setShowLeaveDialog(true);
  };

  const handleConfirmLeave = async () => {
    try {
      setActionLoading(true);
      logger.info('Leaving game confirmed', {
        gameId,
      });
      await apiClient.games.leaveGame(gameId);
      await refetchGameData();
      logger.info('Left game successfully', {
        gameId,
      });
    } catch (_err) {
      logger.error('Failed to leave game', {
        gameId,
        error: _err,
      });
      showError(_err instanceof Error ? _err.message : 'Failed to leave game');
      throw _err; // Re-throw so dialog can handle it
    } finally {
      setActionLoading(false);
    }
  };

  const getStateActions = (currentState: GameState): StateAction[] => {
    const actions: StateAction[] = [];

    switch (currentState) {
      case 'setup':
        actions.push({ label: 'Start Recruitment', state: 'recruitment', color: 'bg-semantic-success hover:bg-semantic-success-hover text-white' });
        // Setup is the likeliest point for a GM to abandon a game — before
        // anyone has been recruited. The backend has always allowed
        // setup → cancelled; only this menu was forcing a detour through
        // recruitment to reach it.
        actions.push({ label: 'Cancel Game', state: 'cancelled', color: 'bg-semantic-danger hover:bg-semantic-danger-hover text-white' });
        break;
      case 'recruitment':
        actions.push({ label: 'Start Character Creation', state: 'character_creation', color: 'bg-interactive-primary hover:bg-interactive-primary-hover text-white' });
        actions.push({ label: 'Cancel Game', state: 'cancelled', color: 'bg-semantic-danger hover:bg-semantic-danger-hover text-white' });
        break;
      case 'character_creation':
        actions.push({ label: 'Start Game', state: 'in_progress', color: 'bg-interactive-primary hover:bg-interactive-primary-hover text-white' });
        break;
      case 'in_progress':
        actions.push({ label: 'Pause Game', state: 'paused', color: 'bg-semantic-warning hover:bg-semantic-warning-hover text-white' });
        actions.push({ label: 'Complete Game', state: 'completed', color: 'bg-semantic-success hover:bg-semantic-success-hover text-white' });
        break;
      case 'paused':
        actions.push({ label: 'Resume Game', state: 'in_progress', color: 'bg-interactive-primary hover:bg-interactive-primary-hover text-white' });
        break;
    }

    return actions;
  };

  return {
    actionLoading,
    handleStateChange,
    handleLeaveGame,
    getStateActions,
    showCompleteDialog,
    setShowCompleteDialog,
    handleConfirmComplete,
    showPauseDialog,
    setShowPauseDialog,
    handleConfirmPause,
    showCancelDialog,
    setShowCancelDialog,
    handleConfirmCancel,
    showLeaveDialog,
    setShowLeaveDialog,
    handleConfirmLeave,
  };
}
