import { BaseApiClient } from './client';
import type {
  AddModeratorRequest,
  Community,
  CommunityBan,
  CommunityBanEvent,
  CommunityModerator,
  CreateCommunityBanRequest,
  CreateCommunityRequest,
  UpdateCommunityProfileRequest,
  UpdateCommunityRequest,
} from '../../types/communities';

/**
 * Communities API client.
 *
 * Two surfaces, deliberately separate. The /admin routes are site-admin acts --
 * creating a community and assigning its owner, and listing every community
 * including inactive ones. The /communities routes are gated per-community on
 * the caller's own standing, so they list only ACTIVE communities.
 *
 * Documents and webhooks arrive in later phases.
 */
export class CommunitiesApi extends BaseApiClient {
  /** Admin: create a community and assign its owner. */
  async createCommunity(data: CreateCommunityRequest) {
    return this.client.post<Community>('/api/v1/admin/communities', data);
  }

  /** Admin: every community, active or not. */
  async listCommunities() {
    return this.client.get<Community[]>('/api/v1/admin/communities');
  }

  /** Admin: partial update -- profile fields, owner reassignment, or deactivation. */
  async updateCommunity(id: number, data: UpdateCommunityRequest) {
    return this.client.patch<Community>(`/api/v1/admin/communities/${id}`, data);
  }

  /** Active communities only -- inactive ones accept no new games. */
  async listActiveCommunities() {
    return this.client.get<Community[]>('/api/v1/communities');
  }

  /** One community's public profile, by slug. */
  async getCommunity(slug: string) {
    return this.client.get<Community>(`/api/v1/communities/${slug}`);
  }

  /**
   * Moderator: edit a community's name and description.
   *
   * Distinct from `updateCommunity` above, which is the ADMIN route keyed by
   * id and able to reassign ownership or deactivate. This one is keyed by slug
   * and accepts only the two profile fields -- the server rejects anything
   * else outright.
   */
  async updateCommunityProfile(slug: string, data: UpdateCommunityProfileRequest) {
    return this.client.patch<Community>(`/api/v1/communities/${slug}`, data);
  }

  /**
   * A community's moderator roster. Requires moderation rights.
   *
   * The OWNER IS NOT IN THIS LIST -- ownership is not a moderator row. Render
   * the owner from the community's `owner_user_id` alongside these entries.
   */
  async listModerators(slug: string) {
    return this.client.get<CommunityModerator[]>(`/api/v1/communities/${slug}/moderators`);
  }

  /** Owner-only: grant a user moderation powers. */
  async addModerator(slug: string, data: AddModeratorRequest) {
    return this.client.post<CommunityModerator>(
      `/api/v1/communities/${slug}/moderators`,
      data
    );
  }

  /** Owner-only: revoke a user's moderation powers. */
  async removeModerator(slug: string, userId: number) {
    return this.client.delete<void>(
      `/api/v1/communities/${slug}/moderators/${userId}`
    );
  }

  /**
   * A community's banlist. Requires moderation rights.
   *
   * EXPIRED bans are included, each carrying `is_active`. Render from that
   * field rather than from the row's presence, and show a lapsed ban as lapsed
   * -- dropping it would make a ban appear to vanish.
   */
  async listBans(slug: string) {
    return this.client.get<CommunityBan[]>(`/api/v1/communities/${slug}/bans`);
  }

  /**
   * Moderator: ban a user, or edit an existing ban in place.
   *
   * Re-banning an already-banned user is a normal 200, not a conflict: it
   * updates the reason and expiry while preserving the original `banned_at`.
   */
  async banUser(slug: string, data: CreateCommunityBanRequest) {
    return this.client.post<CommunityBan>(`/api/v1/communities/${slug}/bans`, data);
  }

  /**
   * Moderator: lift a ban.
   *
   * 404s if the user is not banned -- unlike removing a moderator, which is
   * idempotent. This is reached from a banlist, so a missing row means the view
   * is stale and someone else lifted it first.
   */
  async unbanUser(slug: string, userId: number) {
    return this.client.delete<void>(`/api/v1/communities/${slug}/bans/${userId}`);
  }

  /**
   * A community's ban audit log, newest first. Requires moderation rights.
   *
   * Lifting a ban deletes its row, so for an unbanned user this log is the only
   * surviving record the ban existed.
   */
  async listBanEvents(slug: string, params?: { limit?: number; offset?: number }) {
    return this.client.get<CommunityBanEvent[]>(
      `/api/v1/communities/${slug}/ban-events`,
      { params }
    );
  }
}
