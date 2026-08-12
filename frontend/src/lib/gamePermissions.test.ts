import { describe, it, expect } from 'vitest';
import {
  computeGamePermissions,
  isPublicArchive,
  resolveUserRole,
  type UserGameRole,
} from './gamePermissions';
import type { GameParticipant } from '../types/games';

// These rules are consumed by both GameProvider and useGamePermissions, so a
// change here moves the whole UI's permission surface. They mirror backend
// authorization (CanUserViewGame, CanUserManagePhases, CanUserDeleteComment,
// checkPollViewAccess) — the UI must be neither stricter nor looser.

const participant = (userId: number, role: string): GameParticipant => ({
  id: userId * 10,
  game_id: 1,
  user_id: userId,
  role,
  joined_at: new Date().toISOString(),
  username: `user${userId}`,
});

describe('resolveUserRole', () => {
  it('treats game ownership as GM regardless of any participant row', () => {
    // A GM who also holds a participant row must not be downgraded by it.
    expect(resolveUserRole(1, 1, [participant(1, 'player')])).toBe('gm');
  });

  it('reads the role from the participant list for non-owners', () => {
    expect(resolveUserRole(2, 1, [participant(2, 'co_gm')])).toBe('co_gm');
    expect(resolveUserRole(2, 1, [participant(2, 'audience')])).toBe('audience');
  });

  it('returns none for a non-member, a logged-out viewer, or an unloaded game', () => {
    expect(resolveUserRole(3, 1, [participant(2, 'player')])).toBe('none');
    expect(resolveUserRole(null, 1, [])).toBe('none');
    expect(resolveUserRole(1, undefined, [])).toBe('none');
    expect(resolveUserRole(1, 1, undefined)).toBe('gm');
  });
});

describe('isPublicArchive', () => {
  it('is true only for completed games', () => {
    expect(isPublicArchive('completed')).toBe(true);
    // Cancelled games are explicitly NOT public — they keep play-time rules.
    expect(isPublicArchive('cancelled')).toBe(false);
    expect(isPublicArchive('in_progress')).toBe(false);
    expect(isPublicArchive(undefined)).toBe(false);
  });
});

describe('computeGamePermissions', () => {
  const forRole = (userRole: UserGameRole, gameState = 'in_progress') =>
    computeGamePermissions({ userRole, gameState });

  describe('GM authority vs GM identity', () => {
    it('separates the primary GM from the co-GM', () => {
      const gm = forRole('gm');
      expect(gm.isGM).toBe(true);
      expect(gm.hasGMPowers).toBe(true);
      expect(gm.canEditGame).toBe(true);

      const coGM = forRole('co_gm');
      // Same authority over gameplay...
      expect(coGM.hasGMPowers).toBe(true);
      expect(coGM.canManagePhases).toBe(true);
      expect(coGM.canViewAllActions).toBe(true);
      // ...but not the owner's identity, so no settings editing.
      expect(coGM.isGM).toBe(false);
      expect(coGM.canEditGame).toBe(false);
    });

    it('grants no GM authority to players or audience', () => {
      for (const role of ['player', 'audience', 'none'] as UserGameRole[]) {
        const p = forRole(role);
        expect(p.hasGMPowers).toBe(false);
        expect(p.canManagePhases).toBe(false);
        expect(p.canEditGame).toBe(false);
      }
    });
  });

  describe('admin mode', () => {
    it('confers full GM authority, including settings editing', () => {
      const p = computeGamePermissions({
        userRole: 'none',
        gameState: 'in_progress',
        isAdminActingAsGM: true,
      });
      expect(p.isGM).toBe(true);
      expect(p.hasGMPowers).toBe(true);
      expect(p.canEditGame).toBe(true);
    });

    it('defaults off so it is never granted implicitly', () => {
      expect(forRole('none').isGM).toBe(false);
    });
  });

  describe('membership', () => {
    it('counts players and co-GMs as participants, but not audience', () => {
      expect(forRole('player').isParticipant).toBe(true);
      expect(forRole('co_gm').isParticipant).toBe(true);
      expect(forRole('audience').isParticipant).toBe(false);
      expect(forRole('none').isParticipant).toBe(false);
    });

    it('counts any role, audience included, as being in the game', () => {
      expect(forRole('audience').isInGame).toBe(true);
      expect(forRole('none').isInGame).toBe(false);
    });
  });

  describe('audience access in a completed game (public archive)', () => {
    it('grants read access to players and non-participants alike', () => {
      expect(forRole('player', 'completed').hasAudienceAccess).toBe(true);
      expect(forRole('none', 'completed').hasAudienceAccess).toBe(true);
    });

    it('does not demote a GM or co-GM to spectator', () => {
      expect(forRole('gm', 'completed').hasAudienceAccess).toBe(false);
      expect(forRole('co_gm', 'completed').hasAudienceAccess).toBe(false);
      // They keep their authoring authority in the archive.
      expect(forRole('co_gm', 'completed').canManagePhases).toBe(true);
    });

    it('does not demote an admin acting as GM', () => {
      const p = computeGamePermissions({
        userRole: 'none',
        gameState: 'completed',
        isAdminActingAsGM: true,
      });
      expect(p.hasAudienceAccess).toBe(false);
    });

    it('withholds it while the game is live or once cancelled', () => {
      expect(forRole('player', 'in_progress').hasAudienceAccess).toBe(false);
      expect(forRole('player', 'cancelled').hasAudienceAccess).toBe(false);
      expect(forRole('none', 'cancelled').hasAudienceAccess).toBe(false);
    });

    it('keeps the audience role distinct from archive access', () => {
      const audienceLive = forRole('audience', 'in_progress');
      expect(audienceLive.isAudienceRole).toBe(true);
      expect(audienceLive.hasAudienceAccess).toBe(true);

      // A player in the archive gets the access without becoming the role —
      // identity still drives "Leave Game" and the public-viewer banner.
      const playerArchive = forRole('player', 'completed');
      expect(playerArchive.isAudienceRole).toBe(false);
      expect(playerArchive.hasAudienceAccess).toBe(true);
      expect(playerArchive.isPlayer).toBe(true);
    });
  });
});
