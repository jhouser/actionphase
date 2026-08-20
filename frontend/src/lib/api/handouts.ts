import { BaseApiClient } from './client';
import type {
  Handout,
  HandoutWithGame,
  HandoutComment,
  CreateHandoutRequest,
  UpdateHandoutRequest,
  CreateHandoutCommentRequest,
  UpdateHandoutCommentRequest
} from '../../types/handouts';

/**
 * Handouts API client
 * Handles handout and handout comment management
 */
export class HandoutsApi extends BaseApiClient {
  // Handout endpoints
  async createHandout(gameId: number, data: CreateHandoutRequest) {
    return this.client.post<Handout>(`/api/v1/games/${gameId}/handouts`, data);
  }

  async getHandout(gameId: number, handoutId: number) {
    return this.client.get<Handout>(`/api/v1/games/${gameId}/handouts/${handoutId}`);
  }

  async listHandouts(gameId: number) {
    return this.client.get<Handout[]>(`/api/v1/games/${gameId}/handouts`);
  }

  async updateHandout(gameId: number, handoutId: number, data: UpdateHandoutRequest) {
    return this.client.put<Handout>(`/api/v1/games/${gameId}/handouts/${handoutId}`, data);
  }

  async deleteHandout(gameId: number, handoutId: number) {
    return this.client.delete(`/api/v1/games/${gameId}/handouts/${handoutId}`);
  }

  async publishHandout(gameId: number, handoutId: number) {
    return this.client.post<Handout>(`/api/v1/games/${gameId}/handouts/${handoutId}/publish`);
  }

  async unpublishHandout(gameId: number, handoutId: number) {
    return this.client.post<Handout>(`/api/v1/games/${gameId}/handouts/${handoutId}/unpublish`);
  }

  /**
   * Published handouts across every in_progress game the current user takes
   * part in. Backs the global Utility Drawer, which has no game in scope; each
   * entry carries its game title so the list can be grouped without a request
   * per game.
   */
  async listHandoutsAcrossGames() {
    return this.client.get<HandoutWithGame[]>('/api/v1/handouts');
  }

  // Handout comment endpoints
  async createHandoutComment(gameId: number, handoutId: number, data: CreateHandoutCommentRequest) {
    return this.client.post<HandoutComment>(`/api/v1/games/${gameId}/handouts/${handoutId}/comments`, data);
  }

  async listHandoutComments(gameId: number, handoutId: number) {
    return this.client.get<HandoutComment[]>(`/api/v1/games/${gameId}/handouts/${handoutId}/comments`);
  }

  async updateHandoutComment(gameId: number, handoutId: number, commentId: number, data: UpdateHandoutCommentRequest) {
    return this.client.patch<HandoutComment>(`/api/v1/games/${gameId}/handouts/${handoutId}/comments/${commentId}`, data);
  }

  async deleteHandoutComment(gameId: number, handoutId: number, commentId: number) {
    return this.client.delete(`/api/v1/games/${gameId}/handouts/${handoutId}/comments/${commentId}`);
  }
}
