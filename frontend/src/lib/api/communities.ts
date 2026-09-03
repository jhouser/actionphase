import { BaseApiClient } from './client';
import type {
  AddModeratorRequest,
  Community,
  CommunityBan,
  CommunityBanEvent,
  CommunityDocument,
  CommunityModerator,
  CommunityWebhook,
  CreateCommunityBanRequest,
  CreateCommunityDocumentRequest,
  CreateCommunityRequest,
  CreateCommunityWebhookRequest,
  UpdateCommunityDocumentRequest,
  UpdateCommunityProfileRequest,
  UpdateCommunityRequest,
  UpdateCommunityWebhookRequest,
  WebhookTestResult,
} from '../../types/communities';

/**
 * Communities API client.
 *
 * Two surfaces, deliberately separate. The /admin routes are site-admin acts --
 * creating a community and assigning its owner, and listing every community
 * including inactive ones. The /communities routes are gated per-community on
 * the caller's own standing, so they list only ACTIVE communities.
 *
 * Webhook URLs are credentials: every response masks them, and no method here
 * can retrieve a real one.
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
   * Moderator: replace a community's banner image.
   *
   * Deliberately NOT a field on `updateCommunityProfile` -- a banner is an
   * uploaded object whose file and column must stay in sync, so the server
   * exposes it only through this endpoint and rejects `banner_url` in a PATCH.
   *
   * Returns the refreshed community rather than the URL alone, so callers can
   * replace their cached profile without a follow-up GET.
   */
  async uploadCommunityBanner(slug: string, file: File) {
    const formData = new FormData();
    formData.append('banner', file);
    return this.client.post<Community>(`/api/v1/communities/${slug}/banner`, formData, {
      headers: {
        // Undefined rather than a literal multipart type: axios has to set the
        // header itself so it can append the boundary it generated.
        'Content-Type': undefined,
      },
    });
  }

  /** Moderator: remove a community's banner. Succeeds when there is none. */
  async deleteCommunityBanner(slug: string) {
    return this.client.delete(`/api/v1/communities/${slug}/banner`);
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

  // ------------------------------------------------------------- documents

  /**
   * A community's PUBLISHED documents.
   *
   * Open to any authenticated user: a community's rules are what someone reads
   * before deciding whether to join, so gating them on membership would hide
   * them from exactly the person they inform. Drafts are never included --
   * moderators use `listAllDocuments` for those.
   */
  async listDocuments(slug: string) {
    return this.client.get<CommunityDocument[]>(`/api/v1/communities/${slug}/documents`);
  }

  /**
   * Moderator: every document INCLUDING drafts.
   *
   * A separate endpoint from `listDocuments` rather than a query flag on it, so
   * the privileged read carries its own permission gate on the server.
   */
  async listAllDocuments(slug: string) {
    return this.client.get<CommunityDocument[]>(
      `/api/v1/communities/${slug}/documents/manage`
    );
  }

  /**
   * One document.
   *
   * A draft answers 404 for anyone but a moderator -- not 403, which would
   * confirm the document exists and let unpublished work be enumerated by id.
   */
  async getDocument(slug: string, documentId: number) {
    return this.client.get<CommunityDocument>(
      `/api/v1/communities/${slug}/documents/${documentId}`
    );
  }

  /** Moderator: create a document. Omitting `status` creates a draft. */
  async createDocument(slug: string, data: CreateCommunityDocumentRequest) {
    return this.client.post<CommunityDocument>(
      `/api/v1/communities/${slug}/documents`,
      data
    );
  }

  /**
   * Moderator: partial update. Publishing and unpublishing both run through
   * here, since status sits on the same form as the body.
   */
  async updateDocument(
    slug: string,
    documentId: number,
    data: UpdateCommunityDocumentRequest
  ) {
    return this.client.patch<CommunityDocument>(
      `/api/v1/communities/${slug}/documents/${documentId}`,
      data
    );
  }

  /**
   * Moderator: delete a document.
   *
   * 404s if it does not exist rather than succeeding -- a delete that silently
   * matched nothing would leave a moderator believing published rules are gone.
   */
  async deleteDocument(slug: string, documentId: number) {
    return this.client.delete<void>(
      `/api/v1/communities/${slug}/documents/${documentId}`
    );
  }

  /**
   * The published documents of the community that owns a GAME -- the Info tab.
   *
   * Gated on access to the game, not on standing in the community: a player
   * reads their game's community rules without moderating it. A game with no
   * community returns an empty list, so legacy games render no section.
   */
  async listGameCommunityDocuments(gameId: number) {
    return this.client.get<CommunityDocument[]>(
      `/api/v1/games/${gameId}/community-documents`
    );
  }
  // ------------------------------------------------------------- webhooks

  /**
   * Moderator: list a community's Discord webhooks.
   *
   * URLs come back MASKED and disabled webhooks are included -- re-enabling
   * them is the point of the screen. There is no public read: unlike documents,
   * nothing here is visible to ordinary members.
   */
  async listWebhooks(slug: string) {
    return this.client.get<CommunityWebhook[]>(
      `/api/v1/communities/${slug}/webhooks`
    );
  }

  /**
   * Moderator: register a webhook.
   *
   * The server rejects any URL that is not an https Discord webhook endpoint --
   * it makes outbound requests to this address, so the check is a security
   * control rather than a formatting rule. The response masks the URL.
   */
  async createWebhook(slug: string, data: CreateCommunityWebhookRequest) {
    return this.client.post<CommunityWebhook>(
      `/api/v1/communities/${slug}/webhooks`,
      data
    );
  }

  /**
   * Moderator: partially update a webhook.
   *
   * 🔴 OMIT `url` unless the moderator typed a new one. The client only ever
   * holds a masked URL, and sending that mask back would overwrite the stored
   * credential with bullet characters, silently breaking delivery.
   */
  async updateWebhook(
    slug: string,
    webhookId: number,
    data: UpdateCommunityWebhookRequest
  ) {
    return this.client.patch<CommunityWebhook>(
      `/api/v1/communities/${slug}/webhooks/${webhookId}`,
      data
    );
  }

  /** Moderator: delete a webhook. 404s if it does not exist. */
  async deleteWebhook(slug: string, webhookId: number) {
    return this.client.delete<void>(
      `/api/v1/communities/${slug}/webhooks/${webhookId}`
    );
  }

  /**
   * Moderator: send a test message, SYNCHRONOUSLY.
   *
   * Unlike game-state announcements, which are fire-and-forget, this waits for
   * Discord and reports the outcome -- that answer is the whole point of the
   * button. A rejection surfaces as a 502 carrying Discord's reason.
   */
  async testWebhook(slug: string, webhookId: number) {
    return this.client.post<WebhookTestResult>(
      `/api/v1/communities/${slug}/webhooks/${webhookId}/test`,
      {}
    );
  }
}
