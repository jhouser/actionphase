import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement } from 'react';
import { useGamePermissions } from './useGamePermissions';
import { useAuth } from '../contexts/AuthContext';
import { useAdminMode } from '../contexts/AdminModeContext';
import { apiClient } from '../lib/api';
import type { GameWithDetails } from '../types/games';
import type { GameParticipant } from '../types/games';

vi.mock('../lib/api', () => ({
  apiClient: {
    games: {
      getGameWithDetails: vi.fn(),
      getGameParticipants: vi.fn(),
    },
  },
}));

vi.mock('../contexts/AuthContext', () => ({
  useAuth: vi.fn(() => ({
    currentUser: { id: 1, username: 'testuser', email: 'test@example.com' },
  })),
}));

// Admin mode off by default — it is an explicit escalation, so no test gets it
// implicitly. The one test that exercises it overrides this.
vi.mock('../contexts/AdminModeContext', () => ({
  useAdminMode: vi.fn(() => ({ adminModeEnabled: false, isAdmin: false })),
}));

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
};

const MOCK_GAME: GameWithDetails = {
  id: 10,
  title: 'Test Game',
  description: 'A game',
  gm_user_id: 1, // current user is GM
  state: 'in_progress',
  max_players: 5,
  is_public: true,
  is_anonymous: false,
  game_config: {},
  created_at: new Date().toISOString(),
  updated_at: new Date().toISOString(),
};

const makeParticipant = (userId: number, role: string): GameParticipant => ({
  id: userId * 10,
  game_id: 10,
  user_id: userId,
  role,
  joined_at: new Date().toISOString(),
  username: `user${userId}`,
});

describe('useGamePermissions', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(apiClient.games.getGameParticipants).mockResolvedValue({
      data: [],
    } as never);
    // Restore defaults explicitly: individual tests override these, and
    // clearAllMocks does not undo a mockReturnValue from a previous test.
    vi.mocked(useAuth).mockReturnValue({
      currentUser: { id: 1, username: 'testuser', email: 'test@example.com' },
    } as never);
    vi.mocked(useAdminMode).mockReturnValue({ adminModeEnabled: false, isAdmin: false } as never);
  });

  // -----------------------------------------------------------------
  // Role derivation — if these flags are wrong, GMs see player UI and
  // players can see GM controls. Silent and harmful.
  // -----------------------------------------------------------------

  it('identifies the GM correctly when gm_user_id matches current user', async () => {
    vi.mocked(apiClient.games.getGameWithDetails).mockResolvedValue({
      data: { ...MOCK_GAME, gm_user_id: 1 },
    } as never);

    const { result } = renderHook(() => useGamePermissions(10), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(result.current.userRole).toBe('gm');
    expect(result.current.isGM).toBe(true);
    expect(result.current.isPlayer).toBe(false);
    expect(result.current.isAudience).toBe(false);
    expect(result.current.canEditGame).toBe(true);
    expect(result.current.canManagePhases).toBe(true);
    expect(result.current.canViewAllActions).toBe(true);
  });

  it('identifies a player when current user is a participant with player role', async () => {
    vi.mocked(apiClient.games.getGameWithDetails).mockResolvedValue({
      data: { ...MOCK_GAME, gm_user_id: 99 }, // someone else is GM
    } as never);
    vi.mocked(apiClient.games.getGameParticipants).mockResolvedValue({
      data: [makeParticipant(1, 'player')], // current user (id=1) is a player
    } as never);

    const { result } = renderHook(() => useGamePermissions(10), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(result.current.userRole).toBe('player');
    expect(result.current.isPlayer).toBe(true);
    expect(result.current.isGM).toBe(false);
    expect(result.current.isParticipant).toBe(true);
    expect(result.current.canEditGame).toBe(false);
    expect(result.current.canManagePhases).toBe(false);
    expect(result.current.canViewAllActions).toBe(false);
  });

  it('identifies a co-GM — can manage phases but not edit game settings', async () => {
    vi.mocked(apiClient.games.getGameWithDetails).mockResolvedValue({
      data: { ...MOCK_GAME, gm_user_id: 99 },
    } as never);
    vi.mocked(apiClient.games.getGameParticipants).mockResolvedValue({
      data: [makeParticipant(1, 'co_gm')],
    } as never);

    const { result } = renderHook(() => useGamePermissions(10), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(result.current.userRole).toBe('co_gm');
    expect(result.current.isCoGM).toBe(true);
    expect(result.current.isGM).toBe(false);             // isGM is primary-GM identity only
    expect(result.current.hasGMPowers).toBe(true);       // but the co-GM holds GM authority
    expect(result.current.canEditGame).toBe(false);      // co-GM cannot edit game settings
    expect(result.current.canManagePhases).toBe(true);   // but can manage phases
    expect(result.current.canViewAllActions).toBe(true);
  });

  it('identifies an audience member', async () => {
    vi.mocked(apiClient.games.getGameWithDetails).mockResolvedValue({
      data: { ...MOCK_GAME, gm_user_id: 99 },
    } as never);
    vi.mocked(apiClient.games.getGameParticipants).mockResolvedValue({
      data: [makeParticipant(1, 'audience')],
    } as never);

    const { result } = renderHook(() => useGamePermissions(10), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(result.current.userRole).toBe('audience');
    expect(result.current.isAudience).toBe(true);
    expect(result.current.isParticipant).toBe(false); // audience is NOT a participant
    expect(result.current.canManagePhases).toBe(false);
  });

  // A COMPLETED game is a public archive: every authenticated viewer gets
  // audience-level access to all aspects of it. isAudience is the access level
  // the view components gate on, so it widens on completion — while userRole
  // stays the user's real identity, which "Leave Game" and the public-viewer
  // banner still depend on.
  describe('completed games grant audience-level access to everyone', () => {
    const completedGameWithRole = async (role: string | null) => {
      vi.mocked(apiClient.games.getGameWithDetails).mockResolvedValue({
        data: { ...MOCK_GAME, gm_user_id: 99, state: 'completed' },
      } as never);
      vi.mocked(apiClient.games.getGameParticipants).mockResolvedValue({
        data: role ? [makeParticipant(1, role)] : [],
      } as never);

      const { result } = renderHook(() => useGamePermissions(10), {
        wrapper: createWrapper(),
      });
      await waitFor(() => expect(result.current.isLoading).toBe(false));
      return result;
    };

    it('grants audience access to a player once the game completes', async () => {
      const result = await completedGameWithRole('player');

      expect(result.current.isAudience).toBe(true);
      // Identity is unchanged — the player is still a player, just with
      // archive-level reach.
      expect(result.current.userRole).toBe('player');
      expect(result.current.isPlayer).toBe(true);
      expect(result.current.isGM).toBe(false);
      expect(result.current.canManagePhases).toBe(false);
    });

    it('grants audience access to a non-participant viewing the archive', async () => {
      const result = await completedGameWithRole(null);

      expect(result.current.isAudience).toBe(true);
      expect(result.current.userRole).toBe('none');
      expect(result.current.isGM).toBe(false);
      expect(result.current.canEditGame).toBe(false);
    });

    it('does not grant audience access to a player while the game is in progress', async () => {
      vi.mocked(apiClient.games.getGameWithDetails).mockResolvedValue({
        data: { ...MOCK_GAME, gm_user_id: 99, state: 'in_progress' },
      } as never);
      vi.mocked(apiClient.games.getGameParticipants).mockResolvedValue({
        data: [makeParticipant(1, 'player')],
      } as never);

      const { result } = renderHook(() => useGamePermissions(10), {
        wrapper: createWrapper(),
      });
      await waitFor(() => expect(result.current.isLoading).toBe(false));

      expect(result.current.isAudience).toBe(false);
    });

    // Cancelled games are NOT a public archive — the play-time rules hold.
    it('does not grant audience access in a cancelled game', async () => {
      vi.mocked(apiClient.games.getGameWithDetails).mockResolvedValue({
        data: { ...MOCK_GAME, gm_user_id: 99, state: 'cancelled' },
      } as never);
      vi.mocked(apiClient.games.getGameParticipants).mockResolvedValue({
        data: [makeParticipant(1, 'player')],
      } as never);

      const { result } = renderHook(() => useGamePermissions(10), {
        wrapper: createWrapper(),
      });
      await waitFor(() => expect(result.current.isLoading).toBe(false));

      expect(result.current.isAudience).toBe(false);
    });

    // A co-GM holds the same authoring authority as the GM, so completion must
    // not flatten them into a spectator either. Guarding on isGM alone (which is
    // primary-GM identity) would have marked them isAudience while they still
    // reported canManagePhases — a contradictory state.
    it('leaves a co-GM with GM powers in a completed game', async () => {
      const result = await completedGameWithRole('co_gm');

      expect(result.current.isAudience).toBe(false);
      expect(result.current.hasGMPowers).toBe(true);
      expect(result.current.isCoGM).toBe(true);
      expect(result.current.canManagePhases).toBe(true);
      // Still not the primary GM: game settings remain the owner's alone.
      expect(result.current.isGM).toBe(false);
      expect(result.current.canEditGame).toBe(false);
    });

    // The GM must keep authoring powers in a completed game rather than being
    // flattened into a spectator.
    it('leaves the GM as GM in a completed game', async () => {
      vi.mocked(apiClient.games.getGameWithDetails).mockResolvedValue({
        data: { ...MOCK_GAME, gm_user_id: 1, state: 'completed' },
      } as never);

      const { result } = renderHook(() => useGamePermissions(10), {
        wrapper: createWrapper(),
      });
      await waitFor(() => expect(result.current.isLoading).toBe(false));

      expect(result.current.isGM).toBe(true);
      expect(result.current.userRole).toBe('gm');
      expect(result.current.canManagePhases).toBe(true);
    });
  });

  // Admin mode is a deliberate escalation to full GM authority, and GameProvider
  // has always honoured it. This hook did not until the permission rules were
  // shared, so a component rendered outside the provider disagreed with one
  // inside it about what an admin could do.
  describe('admin mode', () => {
    const renderAsAdmin = async (adminModeEnabled: boolean) => {
      vi.mocked(useAuth).mockReturnValue({
        currentUser: { id: 1, username: 'admin', email: 'a@example.com', is_admin: true },
      } as never);
      vi.mocked(useAdminMode).mockReturnValue({ adminModeEnabled, isAdmin: true } as never);
      vi.mocked(apiClient.games.getGameWithDetails).mockResolvedValue({
        data: { ...MOCK_GAME, gm_user_id: 99 },
      } as never);

      const { result } = renderHook(() => useGamePermissions(10), {
        wrapper: createWrapper(),
      });
      await waitFor(() => expect(result.current.isLoading).toBe(false));
      return result;
    };

    it('grants full GM authority to an admin with admin mode on', async () => {
      const result = await renderAsAdmin(true);

      expect(result.current.isGM).toBe(true);
      expect(result.current.hasGMPowers).toBe(true);
      expect(result.current.canEditGame).toBe(true);
      expect(result.current.canManagePhases).toBe(true);
      // Identity is untouched — they hold no actual role in this game.
      expect(result.current.userRole).toBe('none');
    });

    // Merely being an admin is not enough; the escalation must be switched on.
    it('grants nothing to an admin with admin mode off', async () => {
      const result = await renderAsAdmin(false);

      expect(result.current.isGM).toBe(false);
      expect(result.current.hasGMPowers).toBe(false);
      expect(result.current.canEditGame).toBe(false);
    });
  });

  it('returns role=none for a non-member who is not GM', async () => {
    vi.mocked(apiClient.games.getGameWithDetails).mockResolvedValue({
      data: { ...MOCK_GAME, gm_user_id: 99 },
    } as never);
    vi.mocked(apiClient.games.getGameParticipants).mockResolvedValue({
      data: [], // current user is not in participants
    } as never);

    const { result } = renderHook(() => useGamePermissions(10), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(result.current.userRole).toBe('none');
    expect(result.current.isGM).toBe(false);
    expect(result.current.isPlayer).toBe(false);
    expect(result.current.isParticipant).toBe(false);
    expect(result.current.canEditGame).toBe(false);
  });
});
