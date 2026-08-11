import { BaseApiClient } from './client';
import type {
  Game,
  GameWithDetails,
  GameListItem,
  GameParticipant,
  CreateGameRequest,
  UpdateGameRequest,
  UpdateGameStateRequest,
  ApplyToGameRequest,
  GameApplication,
  PublicGameApplicant,
  ReviewApplicationRequest,
  GameListingFilters,
  GameListingResponse,
  GameLog,
  LootTable,
  LootTableContent,
  CreateLootTableRequest,
  UpdateLootTableRequest
} from '../../types/games';
import type {
  AudienceConversationListItem,
  AudienceConversationMessage
} from '../../types/conversations';
import type { ActionSubmission } from '../../types/phases';
import type { GameStats } from '../../types/gameStats';

/**
 * Games API client
 * Handles game CRUD, applications, and participants
 */
export class GamesApi extends BaseApiClient {
  // Game CRUD endpoints
  async getAllGames() {
    return this.client.get<GameListItem[]>('/api/v1/games/public');
  }

  async getRecruitingGames() {
    return this.client.get<GameListItem[]>('/api/v1/games/recruiting');
  }

  async getFilteredGames(filters?: GameListingFilters) {
    // Build query string from filters
    const params = new URLSearchParams();

    if (filters?.search && filters.search.trim()) {
      params.append('search', filters.search.trim());
    }
    if (filters?.states && filters.states.length > 0) {
      params.append('states', filters.states.join(','));
    }
    if (filters?.participation) {
      params.append('participation', filters.participation);
    }
    if (filters?.has_open_spots !== undefined) {
      params.append('has_open_spots', filters.has_open_spots.toString());
    }
    if (filters?.sort_by) {
      params.append('sort_by', filters.sort_by);
    }
    if (filters?.admin_mode === true) {
      params.append('admin_mode', 'true');
    }
    if (filters?.page) {
      params.append('page', filters.page.toString());
    }
    if (filters?.page_size) {
      params.append('page_size', filters.page_size.toString());
    }

    const queryString = params.toString();
    const url = queryString ? `/api/v1/games/?${queryString}` : '/api/v1/games/';

    return this.client.get<GameListingResponse>(url);
  }

  async getGame(id: number) {
    return this.client.get<Game>(`/api/v1/games/${id}`);
  }

  async getGameWithDetails(id: number) {
    return this.client.get<GameWithDetails>(`/api/v1/games/${id}/details`);
  }

  async getGameParticipants(id: number) {
    return this.client.get<GameParticipant[]>(`/api/v1/games/${id}/participants`);
  }

  async createGame(data: CreateGameRequest) {
    return this.client.post<Game>('/api/v1/games', data);
  }

  async updateGame(id: number, data: UpdateGameRequest) {
    return this.client.put<Game>(`/api/v1/games/${id}`, data);
  }

  async deleteGame(id: number) {
    return this.client.delete(`/api/v1/games/${id}`);
  }

  async updateGameState(id: number, data: UpdateGameStateRequest) {
    return this.client.put<Game>(`/api/v1/games/${id}/state`, data);
  }

  async leaveGame(id: number) {
    return this.client.delete(`/api/v1/games/${id}/leave`);
  }

  // Game Application endpoints
  async applyToGame(id: number, data: ApplyToGameRequest) {
    return this.client.post<GameApplication>(`/api/v1/games/${id}/apply`, data);
  }

  async getGameApplications(id: number) {
    return this.client.get<GameApplication[]>(`/api/v1/games/${id}/applications`);
  }

  async getMyGameApplication(id: number) {
    return this.client.get<GameApplication>(`/api/v1/games/${id}/application/mine`);
  }

  async reviewGameApplication(gameId: number, applicationId: number, data: ReviewApplicationRequest) {
    return this.client.put<GameApplication>(`/api/v1/games/${gameId}/applications/${applicationId}/review`, data);
  }

  async withdrawGameApplication(id: number) {
    return this.client.delete(`/api/v1/games/${id}/application`);
  }

  async getPublicGameApplicants(id: number) {
    return this.client.get<PublicGameApplicant[]>(`/api/v1/games/${id}/applicants`);
  }

  // Player Management endpoints (GM only)
  async removePlayer(gameId: number, userId: number) {
    return this.client.delete(`/api/v1/games/${gameId}/participants/${userId}`);
  }

  async addParticipantDirectly(gameId: number, data: { user_id: number; role: 'player' | 'audience' }) {
    return this.client.post<GameParticipant>(`/api/v1/games/${gameId}/participants/direct-add`, data);
  }

  // Co-GM Management endpoints (GM only)
  async promoteToCoGM(gameId: number, userId: number) {
    return this.client.post(`/api/v1/games/${gameId}/participants/${userId}/promote-to-co-gm`);
  }

  async demoteFromCoGM(gameId: number, userId: number) {
    return this.client.post(`/api/v1/games/${gameId}/participants/${userId}/demote-from-co-gm`);
  }

  async transitionPlayerToAudience(gameId: number, userId: number) {
    return this.client.post(`/api/v1/games/${gameId}/participants/${userId}/to-audience`);
  }

  // Audience Participation endpoints
  async listAudienceMembers(gameId: number) {
    return this.client.get<{ audience_members: GameParticipant[] }>(`/api/v1/games/${gameId}/audience`);
  }

  async setAutoAcceptAudience(gameId: number, autoAccept: boolean) {
    return this.client.put(`/api/v1/games/${gameId}/settings/auto-accept-audience`, {
      auto_accept_audience: autoAccept
    });
  }

  async listAllPrivateConversations(gameId: number, options?: { limit?: number; offset?: number; participantNames?: string[] }) {
    const params = new URLSearchParams();
    if (options?.limit) params.append('limit', options.limit.toString());
    if (options?.offset) params.append('offset', options.offset.toString());
    // Add each participant name as a separate parameter
    if (options?.participantNames && options.participantNames.length > 0) {
      options.participantNames.forEach(name => {
        params.append('participant_names', name);
      });
    }

    const queryString = params.toString();
    const url = queryString
      ? `/api/v1/games/${gameId}/private-messages/all?${queryString}`
      : `/api/v1/games/${gameId}/private-messages/all`;

    return this.client.get<{ conversations: AudienceConversationListItem[]; total: number }>(url);
  }

  async getConversationParticipants(gameId: number, selectedNames?: string[]) {
    const params = new URLSearchParams();
    if (selectedNames && selectedNames.length > 0) {
      selectedNames.forEach(name => params.append('selected[]', name));
    }
    const queryString = params.toString();
    const url = queryString
      ? `/api/v1/games/${gameId}/private-messages/participants?${queryString}`
      : `/api/v1/games/${gameId}/private-messages/participants`;
    return this.client.get<{ participants: string[] }>(url);
  }

  async getAudienceConversationMessages(gameId: number, conversationId: string) {
    return this.client.get<{ messages: AudienceConversationMessage[] }>(`/api/v1/games/${gameId}/private-messages/conversations/${conversationId}`);
  }

  async uploadGameBanner(gameId: number, file: File) {
    const formData = new FormData();
    formData.append('banner', file);
    return this.client.post<{ banner_url: string }>(
      `/api/v1/games/${gameId}/banner`,
      formData,
      {
        headers: {
          'Content-Type': undefined, // Let axios set multipart boundary
        },
      }
    );
  }

  async deleteGameBanner(gameId: number) {
    return this.client.delete(`/api/v1/games/${gameId}/banner`);
  }

  async listAllActionSubmissions(gameId: number, options?: { limit?: number; offset?: number; phaseId?: number }) {
    const params = new URLSearchParams();
    if (options?.limit) params.append('limit', options.limit.toString());
    if (options?.offset) params.append('offset', options.offset.toString());
    if (options?.phaseId) params.append('phase_id', options.phaseId.toString());

    const queryString = params.toString();
    const url = queryString
      ? `/api/v1/games/${gameId}/action-submissions/all?${queryString}`
      : `/api/v1/games/${gameId}/action-submissions/all`;

    return this.client.get<{ action_submissions: ActionSubmission[]; total: number }>(url);
  }

  

  async getGameLogs(id: number) {
    return this.client.get<GameLog[]>(`/api/v1/games/${id}/logs`);
  }

  async getLootTables(gameId: number) {
    return this.client.get<LootTable[]>(`/api/v1/games/${gameId}/loot-tables`);
  }

  async createLootTable(gameId: number, data: CreateLootTableRequest) {
    return this.client.post<LootTable>(`/api/v1/games/${gameId}/loot-tables`, { name: data.name, items: data.items });
  }

  async updateLootTable(gameId: number, lootTableId: number, data: UpdateLootTableRequest) {
    return this.client.put(`/api/v1/games/${gameId}/loot-tables/${lootTableId}`, { name: data.name });
  }

  async deleteLootTable(gameId: number, lootTableId: number) {
    return this.client.delete(`/api/v1/games/${gameId}/loot-tables/${lootTableId}`);
  }

  async getLootTableContents(gameId: number, tableId: number) {
    return this.client.get<LootTableContent[]>(`/api/v1/games/${gameId}/loot-tables/${tableId}/contents`);
  }

  async setLootTableContents(gameId: number, tableId: number, contents: LootTableContent[]) {
    return this.client.post(`/api/v1/games/${gameId}/loot-tables/${tableId}/contents`, { items: contents });
  }

  async giveRandomLootTableContent(gameId: number, tableId: number, characterId: number) {
    return this.client.post<LootTableContent>(`/api/v1/games/${gameId}/loot-tables/${tableId}/random/${characterId}`, { });
  }
  
  // Post-game statistics. Returns 409 unless the game is completed, so only
  // call this for completed games.
  async getGameStats(id: number) {
    return this.client.get<GameStats>(`/api/v1/games/${id}/stats`);
  }
}
