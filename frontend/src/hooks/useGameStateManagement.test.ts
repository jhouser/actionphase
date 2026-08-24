import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useGameStateManagement } from './useGameStateManagement';

// Mock dependencies
vi.mock('../lib/api', () => ({
  apiClient: {
    games: {
      updateGameState: vi.fn(),
      leaveGame: vi.fn(),
    },
  },
}));

vi.mock('../contexts/AuthContext', () => ({
  useAuth: () => ({
    currentUser: { id: 1, username: 'gm', email: 'gm@example.com' },
    isAuthenticated: true,
  }),
}));

vi.mock('../contexts/ToastContext', () => ({
  useToast: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
  }),
}));

vi.mock('@/services/LoggingService', () => ({
  logger: {
    info: vi.fn(),
    error: vi.fn(),
  },
}));

import { apiClient } from '../lib/api';

describe('useGameStateManagement', () => {
  const refetchGameData = vi.fn().mockResolvedValue(undefined);

  beforeEach(() => {
    vi.clearAllMocks();
  });

  function renderHookWithOptions() {
    return renderHook(() =>
      useGameStateManagement({ gameId: 1, refetchGameData })
    );
  }

  // -----------------------------------------------------------------
  // getStateActions — this is the core V&V target.
  // If a state is missing or transitions are wrong, GMs get wrong buttons.
  // -----------------------------------------------------------------

  describe('getStateActions', () => {
    it('setup state offers Start Recruitment and Cancel Game', () => {
      // A GM most often abandons a game before recruiting anyone, so cancel
      // must be reachable from setup without first opening recruitment.
      const { result } = renderHookWithOptions();
      const actions = result.current.getStateActions('setup');
      const states = actions.map(a => a.state);
      expect(states).toContain('recruitment');
      expect(states).toContain('cancelled');
      expect(actions).toHaveLength(2);
    });

    it('recruitment state offers Start Character Creation and Cancel Game', () => {
      const { result } = renderHookWithOptions();
      const actions = result.current.getStateActions('recruitment');
      const states = actions.map(a => a.state);
      expect(states).toContain('character_creation');
      expect(states).toContain('cancelled');
      expect(actions).toHaveLength(2);
    });

    it('character_creation state offers only Start Game', () => {
      const { result } = renderHookWithOptions();
      const actions = result.current.getStateActions('character_creation');
      expect(actions).toHaveLength(1);
      expect(actions[0].state).toBe('in_progress');
    });

    it('in_progress state offers Pause Game and Complete Game', () => {
      const { result } = renderHookWithOptions();
      const actions = result.current.getStateActions('in_progress');
      const states = actions.map(a => a.state);
      expect(states).toContain('paused');
      expect(states).toContain('completed');
      expect(actions).toHaveLength(2);
    });

    it('paused state offers only Resume Game', () => {
      const { result } = renderHookWithOptions();
      const actions = result.current.getStateActions('paused');
      expect(actions).toHaveLength(1);
      expect(actions[0].state).toBe('in_progress');
    });

    it('completed and cancelled states offer no actions', () => {
      const { result } = renderHookWithOptions();
      expect(result.current.getStateActions('completed')).toHaveLength(0);
      expect(result.current.getStateActions('cancelled')).toHaveLength(0);
    });
  });

  // -----------------------------------------------------------------
  // Dialog guards — completing/pausing/cancelling should open a dialog,
  // NOT immediately call the API. Silent removal of this guard would
  // result in accidental game state changes.
  // -----------------------------------------------------------------

  describe('handleStateChange — confirmation dialogs', () => {
    it('complete shows confirmation dialog instead of calling API', async () => {
      const { result } = renderHookWithOptions();
      expect(result.current.showCompleteDialog).toBe(false);

      await act(async () => {
        await result.current.handleStateChange('completed');
      });

      expect(result.current.showCompleteDialog).toBe(true);
      expect(apiClient.games.updateGameState).not.toHaveBeenCalled();
    });

    it('pause shows confirmation dialog instead of calling API', async () => {
      const { result } = renderHookWithOptions();
      expect(result.current.showPauseDialog).toBe(false);

      await act(async () => {
        await result.current.handleStateChange('paused');
      });

      expect(result.current.showPauseDialog).toBe(true);
      expect(apiClient.games.updateGameState).not.toHaveBeenCalled();
    });

    it('cancel shows confirmation dialog instead of calling API', async () => {
      const { result } = renderHookWithOptions();
      expect(result.current.showCancelDialog).toBe(false);

      await act(async () => {
        await result.current.handleStateChange('cancelled');
      });

      expect(result.current.showCancelDialog).toBe(true);
      expect(apiClient.games.updateGameState).not.toHaveBeenCalled();
    });

    it('non-destructive state change (e.g. recruitment) calls API directly', async () => {
      vi.mocked(apiClient.games.updateGameState).mockResolvedValue({ data: {} } as never);
      const { result } = renderHookWithOptions();

      await act(async () => {
        await result.current.handleStateChange('recruitment');
      });

      expect(apiClient.games.updateGameState).toHaveBeenCalledWith(1, { state: 'recruitment' });
    });
  });
});
