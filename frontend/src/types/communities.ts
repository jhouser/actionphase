/**
 * Community types.
 *
 * Communities are tenant-like groupings that own games, a moderator roster, a
 * banlist, and their own documentation. Membership is deliberately OPEN --
 * there is no roster or allowlist; the banlist is the whole access-control
 * mechanism.
 */

export interface Community {
  id: number;
  name: string;
  /** URL identifier. Immutable after creation. */
  slug: string;
  description: string | null;
  /**
   * Read-only here. Banners are uploaded objects, not typed-in URLs, so they
   * are written through a dedicated upload/delete endpoint rather than the
   * general update request.
   */
  banner_url: string | null;
  owner_user_id: number;
  /** Populated by list endpoints, absent on single-record reads. */
  owner_username?: string;
  /** Inactive communities accept no new games. */
  is_active: boolean;
  /**
   * The REQUESTING user's standing in this community -- '' | 'moderator' |
   * 'owner'. A property of the response, not of the community.
   *
   * Gate moderation UI on this rather than comparing against owner_user_id:
   * that comparison misses moderators entirely, and misses a site admin with
   * admin mode on. The server recomputes it per request, so it tracks the
   * admin-mode toggle that a cached login payload could not.
   *
   * Optional in the type only so fixtures and older cached payloads that
   * predate the field still typecheck; treat a missing value as ''.
   */
  your_role?: CommunityRole;
  created_at: string;
  updated_at: string;
}

/**
 * A user's standing within one community.
 *
 * Owner and moderator are two tiers, not one ranked enum: moderators may do
 * everything an owner can EXCEPT manage the moderator roster.
 */
export type CommunityRole = '' | 'moderator' | 'owner';

export interface CommunityModerator {
  id: number;
  community_id: number;
  user_id: number;
  username: string;
  display_name?: string;
  avatar_url?: string;
  granted_by_user_id?: number;
  granted_by_username?: string;
  granted_at: string;
}

/** Admin-only: creating a community and assigning its owner. */
export interface CreateCommunityRequest {
  name: string;
  slug: string;
  description?: string;
  owner_user_id: number;
}

/**
 * Partial update. An omitted field is left unchanged.
 * `slug` is absent on purpose -- it is immutable after creation.
 */
export interface UpdateCommunityRequest {
  name?: string;
  description?: string;
  owner_user_id?: number;
  is_active?: boolean;
}

/**
 * Moderator-level profile edit: name and description only.
 *
 * Deliberately NOT `Pick<UpdateCommunityRequest, ...>`. The server rejects
 * unknown properties on this route with a 400, so owner_user_id and is_active
 * must be unreachable here rather than merely discouraged -- reassigning
 * ownership and deactivating a community stay site-admin acts.
 *
 * `description: ''` CLEARS the blurb; omitting the key leaves it unchanged.
 */
export interface UpdateCommunityProfileRequest {
  name?: string;
  description?: string;
}


/** Owner-only: granting a user moderation powers. */
export interface AddModeratorRequest {
  user_id: number;
}
