import { BaseApiClient } from './client';
import type {
  Community,
  CreateCommunityRequest,
  UpdateCommunityRequest,
} from '../../types/communities';

/**
 * Communities API client.
 *
 * Phase 1 covers the site-admin surface only: creating a community, assigning
 * its owner, listing, and editing. Moderator-facing endpoints (bans, documents,
 * webhooks) arrive in later phases under /api/v1/communities.
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

}
