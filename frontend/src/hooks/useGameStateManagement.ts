import { useCallback, useMemo, useState } from 'react';
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

/**
 * States whose transition is destructive or hard to reverse, and so must be
 * confirmed before the API is called. `handleStateChange` opens the matching
 * dialog for these and returns without touching the network; everything else
 * (e.g. recruitment, resuming from paused) applies immediately.
 *
 * Adding a state here is the only step needed to give it a confirmation gate —
 * the dialog state, confirm handler, and hook return fields are all derived.
 */
const CONFIRMED_STATES = ['completed', 'epilogue', 'paused', 'cancelled'] as const;

type ConfirmedState = (typeof CONFIRMED_STATES)[number];

function isConfirmedState(state: GameState): state is ConfirmedState {
  return (CONFIRMED_STATES as readonly string[]).includes(state);
}

/** Human-readable verb per confirmed state, for logs and error toasts. */
const STATE_VERBS: Record<ConfirmedState, { gerund: string; past: string }> = {
  completed: { gerund: 'complete', past: 'completed' },
  epilogue: { gerund: 'move to epilogue', past: 'moved to epilogue' },
  paused: { gerund: 'pause', past: 'paused' },
  cancelled: { gerund: 'cancel', past: 'cancelled' },
};

const BUTTON_COLORS = {
  primary: 'bg-interactive-primary hover:bg-interactive-primary-hover text-white',
  success: 'bg-semantic-success hover:bg-semantic-success-hover text-white',
  warning: 'bg-semantic-warning hover:bg-semantic-warning-hover text-white',
  danger: 'bg-semantic-danger hover:bg-semantic-danger-hover text-white',
} as const;

/**
 * The state-change actions a GM is offered, keyed by the game's current state.
 * Mirrors the backend's `allowedTransitions` (db/services/games.go) — the UI
 * must not offer a transition the API will reject, nor hide one it would honour.
 */
const STATE_ACTIONS: Partial<Record<GameState, StateAction[]>> = {
  // Setup is the likeliest point for a GM to abandon a game — before anyone has
  // been recruited. The backend has always allowed setup → cancelled; only this
  // menu was forcing a detour through recruitment to reach it.
  setup: [
    { label: 'Start Recruitment', state: 'recruitment', color: BUTTON_COLORS.success },
    { label: 'Cancel Game', state: 'cancelled', color: BUTTON_COLORS.danger },
  ],
  recruitment: [
    { label: 'Start Character Creation', state: 'character_creation', color: BUTTON_COLORS.primary },
    { label: 'Cancel Game', state: 'cancelled', color: BUTTON_COLORS.danger },
  ],
  character_creation: [
    { label: 'Start Game', state: 'in_progress', color: BUTTON_COLORS.primary },
  ],
  // Two endgame options sit side by side here, so they must not read as
  // interchangeable: epilogue takes 'primary' rather than 'success' to keep it
  // visually distinct from Complete Game.
  in_progress: [
    { label: 'Pause Game', state: 'paused', color: BUTTON_COLORS.warning },
    { label: 'Move to Epilogue', state: 'epilogue', color: BUTTON_COLORS.primary },
    { label: 'Complete Game', state: 'completed', color: BUTTON_COLORS.success },
  ],
  paused: [
    { label: 'Resume Game', state: 'in_progress', color: BUTTON_COLORS.primary },
  ],
  // No way back to in_progress: entering epilogue disclosed the whole game and
  // players cannot un-see it. Mirrors allowedTransitions on the backend.
  epilogue: [
    { label: 'Complete Game', state: 'completed', color: BUTTON_COLORS.success },
  ],
};

export function useGameStateManagement({
  gameId,
  refetchGameData,
}: UseGameStateManagementOptions) {
  const { showError } = useToast();
  const [actionLoading, setActionLoading] = useState(false);
  // One flag per confirmed state, plus the separate "leave game" flow (which is
  // not a state transition — it removes the current user from the game).
  const [pendingConfirmation, setPendingConfirmation] = useState<ConfirmedState | null>(null);
  const [showLeaveDialog, setShowLeaveDialog] = useState(false);

  /**
   * Apply a state transition against the API. Shared by the immediate path and
   * every confirmation handler, which previously each carried their own copy of
   * this try/catch/finally.
   *
   * Re-throws on failure so a dialog can keep itself open; the toast is raised
   * here so the caller does not have to.
   */
  const applyStateChange = useCallback(
    async (newState: GameState, verb: string) => {
      try {
        setActionLoading(true);
        logger.info('Game state change attempt', { gameId, targetState: newState });
        await apiClient.games.updateGameState(gameId, { state: newState });
        await refetchGameData();
        logger.info('Game state change successful', { gameId, newState });
      } catch (err) {
        logger.error(`Failed to ${verb} game`, { gameId, targetState: newState, error: err });
        showError(err instanceof Error ? err.message : `Failed to ${verb} game`);
        throw err;
      } finally {
        setActionLoading(false);
      }
    },
    [gameId, refetchGameData, showError]
  );

  const handleStateChange = useCallback(
    async (newState: GameState) => {
      if (isConfirmedState(newState)) {
        logger.info('Showing state change confirmation dialog', {
          gameId,
          targetState: newState,
        });
        setPendingConfirmation(newState);
        return;
      }

      // Unconfirmed transitions apply immediately. The throw from
      // applyStateChange is swallowed here: with no dialog open there is
      // nothing to keep open, and the toast has already been shown.
      await applyStateChange(newState, 'update').catch(() => {});
    },
    [applyStateChange, gameId]
  );

  /** Build the confirm handler for one state; the dialog re-throws on failure. */
  const confirmHandlerFor = useCallback(
    (state: ConfirmedState) => async () => {
      await applyStateChange(state, STATE_VERBS[state].gerund);
    },
    [applyStateChange]
  );

  const setShowDialog = useCallback(
    (state: ConfirmedState) => (show: boolean) => {
      setPendingConfirmation(show ? state : null);
    },
    []
  );

  const handleLeaveGame = useCallback(() => {
    logger.info('Showing leave game confirmation dialog', { gameId });
    setShowLeaveDialog(true);
  }, [gameId]);

  const handleConfirmLeave = useCallback(async () => {
    try {
      setActionLoading(true);
      logger.info('Leaving game confirmed', { gameId });
      await apiClient.games.leaveGame(gameId);
      await refetchGameData();
      logger.info('Left game successfully', { gameId });
    } catch (err) {
      logger.error('Failed to leave game', { gameId, error: err });
      showError(err instanceof Error ? err.message : 'Failed to leave game');
      throw err; // Re-throw so dialog can handle it
    } finally {
      setActionLoading(false);
    }
  }, [gameId, refetchGameData, showError]);

  const getStateActions = useCallback(
    (currentState: GameState): StateAction[] => STATE_ACTIONS[currentState] ?? [],
    []
  );

  // Per-state dialog bindings. Memoised so the identities stay stable across
  // renders for consumers that pass them into memoised children.
  const dialogs = useMemo(
    () => ({
      completed: {
        show: pendingConfirmation === 'completed',
        setShow: setShowDialog('completed'),
        confirm: confirmHandlerFor('completed'),
      },
      epilogue: {
        show: pendingConfirmation === 'epilogue',
        setShow: setShowDialog('epilogue'),
        confirm: confirmHandlerFor('epilogue'),
      },
      paused: {
        show: pendingConfirmation === 'paused',
        setShow: setShowDialog('paused'),
        confirm: confirmHandlerFor('paused'),
      },
      cancelled: {
        show: pendingConfirmation === 'cancelled',
        setShow: setShowDialog('cancelled'),
        confirm: confirmHandlerFor('cancelled'),
      },
    }),
    [pendingConfirmation, setShowDialog, confirmHandlerFor]
  );

  return {
    actionLoading,
    handleStateChange,
    handleLeaveGame,
    getStateActions,

    showCompleteDialog: dialogs.completed.show,
    setShowCompleteDialog: dialogs.completed.setShow,
    handleConfirmComplete: dialogs.completed.confirm,

    showEpilogueDialog: dialogs.epilogue.show,
    setShowEpilogueDialog: dialogs.epilogue.setShow,
    handleConfirmEpilogue: dialogs.epilogue.confirm,

    showPauseDialog: dialogs.paused.show,
    setShowPauseDialog: dialogs.paused.setShow,
    handleConfirmPause: dialogs.paused.confirm,

    showCancelDialog: dialogs.cancelled.show,
    setShowCancelDialog: dialogs.cancelled.setShow,
    handleConfirmCancel: dialogs.cancelled.confirm,

    showLeaveDialog,
    setShowLeaveDialog,
    handleConfirmLeave,
  };
}
