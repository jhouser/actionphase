import { BaseApiClient } from './client';
import type {
  AddModeratorRequest,
  Community,
  CommunityModerator,
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
 * Bans, documents, and webhooks arrive in later phases.
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
}
