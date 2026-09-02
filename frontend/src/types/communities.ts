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
  /**
   * Whether the REQUESTING user is currently banned from this community.
   * A property of the response, not of the community -- computed per request
   * like your_role.
   *
   * "Currently" matters: an expired ban leaves its row behind deliberately, so
   * this is false once a ban lapses. Never infer a ban from a row's presence.
   *
   * Use it to filter the game-creation picker, not the browse listing -- a ban
   * blocks joining, not looking. It is convenience only; the ban check on game
   * creation is the enforcement.
   *
   * Optional in the type only so fixtures and older cached payloads still
   * typecheck; treat a missing value as false.
   */
  is_banned?: boolean;
  created_at: string;
  updated_at: string;
}

/**
 * A user's standing within one community.
 *
 * Owner and moderator are two tiers, not one ranked enum: moderators may do
 * everything an owner can EXCEPT manage the moderator roster.
 */
type CommunityRole = '' | 'moderator' | 'owner';

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

/**
 * One user's exclusion from one community's games.
 *
 * Bans are NOT retroactive: a ban blocks a user from entering new games, but
 * never ejects them from games already in progress. Removing an existing
 * participant stays the GM's decision.
 */
export interface CommunityBan {
  id: number;
  community_id: number;
  user_id: number;
  username: string;
  display_name?: string;
  avatar_url?: string;
  reason?: string;
  banned_by_user_id?: number;
  banned_by_username?: string;
  banned_at: string;
  /**
   * Absent means PERMANENT. An expired ban is NOT deleted -- it stays on the
   * list so a moderator can see it lapsed rather than watching it vanish.
   */
  expires_at?: string;
  /**
   * Whether the ban is being ENFORCED right now. Computed server-side from the
   * clock, so never infer "banned" from a row's presence -- an expired ban is
   * still a row. Render from this field alone.
   */
  is_active: boolean;
}

/**
 * One entry in a community's append-only ban audit log.
 *
 * Separate from the banlist because lifting a ban DELETES its row: for an
 * unbanned user this log is the only surviving record the ban ever existed.
 * `reason` and `expires_at` are SNAPSHOTS as they stood at event time, not
 * live references to a row that may be gone.
 */
export interface CommunityBanEvent {
  id: number;
  community_id: number;
  target_user_id: number;
  target_username?: string;
  /** Nullable: a deleted moderator's events outlive them. */
  actor_user_id?: number;
  actor_username?: string;
  action: BanEventAction;
  reason?: string;
  expires_at?: string;
  created_at: string;
}

/**
 * What happened in a ban audit entry.
 *
 * 'modified' is a re-ban of an already-banned user -- an edited reason or
 * extended expiry. Distinguished from 'banned' so the log reads as a history of
 * decisions rather than implying the user was unbanned and re-banned between.
 */
export type BanEventAction = 'banned' | 'unbanned' | 'modified';

/**
 * Ban a user, or edit an existing ban in place.
 *
 * Re-banning an already-banned user is not an error: it updates the reason and
 * expiry while preserving the original `banned_at`.
 *
 * Omit `expires_at` for a PERMANENT ban -- the common case. It must be in the
 * future; the server rejects a past expiry rather than writing a ban that is
 * inert on arrival.
 */
export interface CreateCommunityBanRequest {
  user_id: number;
  reason?: string;
  expires_at?: string;
}

/**
 * One of a community's rules or reference pages.
 *
 * Addressed by ID, not a slug: a document's title is edited as its rules
 * evolve, and a slug frozen at creation would strand a renamed document at its
 * old URL. See the plan's §3.5 for the full reasoning.
 */
export interface CommunityDocument {
  id: number;
  community_id: number;
  title: string;
  /** Markdown, rendered through MarkdownPreview. */
  content: string;
  status: DocumentStatus;
  /** Display position, lowest first. Ties break by id. */
  sort_order: number;
  /** Nullable: the moderator who wrote it may since have been deleted. */
  created_by_user_id?: number;
  created_at: string;
  updated_at: string;
  // No community_name / community_slug. The per-game list used to carry them so
  // the Info tab could label its section, back when GET /games/{id}/details did
  // not join communities. It does now, so identity comes from the GAME -- which
  // it must, since the section has to name a community that has published no
  // documents at all.
}

/**
 * Whether a document is visible to anyone but a moderator.
 *
 * A draft is moderator-only, and the server answers 404 rather than 403 for
 * everyone else -- so an outsider cannot enumerate unpublished work by walking
 * ids. Drafts let a moderator write rules over several sittings before they
 * bind anyone.
 */
// Not exported: every consumer reaches it through CommunityDocument.status or
// the request types below. Exporting it would be an unused public surface.
type DocumentStatus = 'draft' | 'published';

/**
 * Create a document. Omit `status` for a draft, which is the default -- a
 * half-written page should bind nobody until its author says otherwise.
 */
export interface CreateCommunityDocumentRequest {
  title: string;
  content: string;
  status?: DocumentStatus;
  sort_order?: number;
}

/**
 * Partial update; an omitted field is left unchanged.
 *
 * Publishing and unpublishing both go through here rather than dedicated
 * endpoints, since status sits on the same form as the body.
 *
 * Unlike a community description, `content` is NOT tri-state: the column is NOT
 * NULL, so an empty string is a blank page rather than a clear.
 */
export interface UpdateCommunityDocumentRequest {
  title?: string;
  content?: string;
  status?: DocumentStatus;
  sort_order?: number;
}
