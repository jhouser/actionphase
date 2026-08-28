import type { GameParticipant } from '../types/games';

/**
 * A user's role in a single game. Defined here, alongside the rules that
 * interpret it, rather than in GameContext — the shared helpers cannot import
 * from a context that imports them back.
 */
export type UserGameRole = 'gm' | 'player' | 'co_gm' | 'audience' | 'none';

/**
 * Single source of truth for "what may this user do in this game".
 *
 * Two callers derive permissions independently — GameProvider (the live path for
 * the game page) and useGamePermissions (a standalone hook for components
 * rendered outside that provider). They previously each reimplemented these
 * rules and had drifted apart, notably on whether a co-GM counts as a GM. The
 * rules live here so a change lands in both.
 *
 * These are UI affordances only. The backend re-checks every one of them; the
 * point of mirroring them is that the UI should not be stricter than the API
 * (hiding a control the server would honour) nor looser (offering one it will
 * reject).
 */

/** Roles that hold GM-level authority. */
const GM_POWER_ROLES: ReadonlySet<UserGameRole> = new Set<UserGameRole>(['gm', 'co_gm']);

export interface GameRoleInput {
  /** The viewer's role in this game, as resolved from GM ownership + participant rows. */
  userRole: UserGameRole;
  /** The game's lifecycle state, e.g. 'in_progress' | 'completed' | 'cancelled'. */
  gameState: string | null | undefined;
  /**
   * True when the viewer is a site admin AND has admin mode switched on.
   * Admin mode is an explicit, deliberate escalation — it is never implied by
   * merely holding the admin flag.
   */
  isAdminActingAsGM?: boolean;
}

export interface GamePermissionFlags {
  /**
   * Primary GM identity — the user in game.gm_user_id (or an admin in admin
   * mode). Deliberately narrower than GM authority: editing game settings and
   * promoting a co-GM belong to the owner alone. For anything the backend also
   * grants co-GMs, use hasGMPowers.
   */
  isGM: boolean;
  isCoGM: boolean;
  /**
   * GM-level authority: primary GM, co-GM, or admin in admin mode. The backend
   * consistently pairs these (CanUserManagePhases, CanUserDeleteComment,
   * core.IsUserCoGM), so GM-only affordances should gate on this.
   */
  hasGMPowers: boolean;
  isPlayer: boolean;
  /** Holds the audience ROLE. For read access to archived content use hasAudienceAccess. */
  isAudienceRole: boolean;
  /**
   * Audience-level read ACCESS, which is broader than the audience role.
   *
   * A public-archive game (COMPLETED or EPILOGUE) is readable in full: every
   * authenticated viewer — players, audience, and non-participants alike — may
   * read all of it. The backend already works this way (CanUserViewGame,
   * checkPollViewAccess, CanSeeUsernamesInAnonymousGame, all keyed on
   * core.IsPublicArchive).
   *
   * Anyone with GM powers is excluded, so opening the archive does not demote a
   * GM or co-GM to a spectator. Cancelled games are NOT public and keep the
   * play-time rules.
   */
  hasAudienceAccess: boolean;
  /** Actively playing the game: player or co-GM. Audience and non-members are not. */
  isParticipant: boolean;
  /** Has any role at all, audience included. */
  isInGame: boolean;
  canEditGame: boolean;
  canManagePhases: boolean;
  canViewAllActions: boolean;
}

/**
 * Read gate: the whole game is visible to any authenticated viewer.
 *
 * Both completed and epilogue games are public archives; a cancelled one is
 * not. Mirrors core.IsPublicArchive on the backend.
 *
 * This is a separate question from whether the game accepts writes — see
 * isGameWritable. Epilogue is both readable by everyone AND writable, which is
 * the whole reason the state exists.
 */
export function isPublicArchive(gameState: string | null | undefined): boolean {
  return gameState === 'completed' || gameState === 'epilogue';
}

/**
 * Write gate: the game still accepts new content.
 *
 * Mirrors core.ValidateGameNotCompleted on the backend, which rejects only
 * completed and cancelled. Epilogue IS writable — that is the point of it, so
 * the GM can run epilogue and meta-discussion threads with the archive open.
 */
export function isGameWritable(gameState: string | null | undefined): boolean {
  return gameState !== 'completed' && gameState !== 'cancelled';
}

/**
 * Resolve a viewer's role from game ownership and the participant list.
 * Shared so both callers agree on GM-ownership-beats-participant-row precedence.
 */
export function resolveUserRole(
  currentUserId: number | null | undefined,
  gmUserId: number | null | undefined,
  participants: readonly GameParticipant[] | null | undefined
): UserGameRole {
  if (!currentUserId || gmUserId === null || gmUserId === undefined) return 'none';
  if (gmUserId === currentUserId) return 'gm';

  const participant = participants?.find(p => p.user_id === currentUserId);
  return participant ? (participant.role as UserGameRole) : 'none';
}

/** Derive every permission flag from a viewer's role and the game's state. */
export function computeGamePermissions({
  userRole,
  gameState,
  isAdminActingAsGM = false,
}: GameRoleInput): GamePermissionFlags {
  const isPrimaryGM = userRole === 'gm';
  const isCoGM = userRole === 'co_gm';

  // Admin mode is a full GM escalation: it confers the owner's authority,
  // including settings editing, so it feeds isGM rather than only hasGMPowers.
  const isGM = isPrimaryGM || isAdminActingAsGM;
  const hasGMPowers = GM_POWER_ROLES.has(userRole) || isAdminActingAsGM;

  const isAudienceRole = userRole === 'audience';
  const hasAudienceAccess = isAudienceRole || (isPublicArchive(gameState) && !hasGMPowers);

  return {
    isGM,
    isCoGM,
    hasGMPowers,
    isPlayer: userRole === 'player',
    isAudienceRole,
    hasAudienceAccess,
    isParticipant: userRole === 'player' || isCoGM,
    isInGame: userRole !== 'none',
    canEditGame: isGM,
    canManagePhases: hasGMPowers,
    canViewAllActions: hasGMPowers,
  };
}
