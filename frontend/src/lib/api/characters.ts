import { BaseApiClient } from './client';
import { logger } from '@/services/LoggingService';
import type {
  Character,
  ControllableCharacterWithGame,
  CharacterData,
  CharacterActivityStats,
  CreateCharacterRequest,
  CharacterDataRequest,
  ApproveCharacterRequest,
  AssignNPCRequest
} from '../../types/characters';
import type { CharacterMessagesResponse } from '../../types/messages';

/**
 * Characters API client
 * Handles character creation, management, avatars, and data
 */
export class CharactersApi extends BaseApiClient {
  // Character CRUD endpoints
  async createCharacter(gameId: number, data: CreateCharacterRequest) {
    return this.client.post<Character>(`/api/v1/games/${gameId}/characters`, data);
  }

  async getGameCharacters(gameId: number) {
    return this.client.get<Character[]>(`/api/v1/games/${gameId}/characters`);
  }

  async getUserControllableCharacters(gameId: number) {
    return this.client.get<Character[]>(`/api/v1/games/${gameId}/characters/controllable`);
  }

  /**
   * Every character the current user controls across all their in_progress
   * games, each carrying its game context and the user's role in that game.
   * Backs surfaces with no game in scope, such as the global Utility Drawer.
   */
  async getControllableCharactersAcrossGames() {
    return this.client.get<ControllableCharacterWithGame[]>('/api/v1/characters/controllable');
  }

  async getCharacter(id: number) {
    return this.client.get<Character>(`/api/v1/characters/${id}`);
  }

  async approveCharacter(id: number, data: ApproveCharacterRequest) {
    return this.client.post<Character>(`/api/v1/characters/${id}/approve`, data);
  }

  async assignNPC(id: number, data: AssignNPCRequest) {
    return this.client.post(`/api/v1/characters/${id}/assign`, data);
  }

  async renameCharacter(id: number, data: { name: string }) {
    return this.client.put<Character>(`/api/v1/characters/${id}/rename`, data);
  }

  async deleteCharacter(id: number) {
    return this.client.delete(`/api/v1/characters/${id}`);
  }

  // Character Data endpoints
  async setCharacterData(id: number, data: CharacterDataRequest) {
    return this.client.post(`/api/v1/characters/${id}/data`, data);
  }

  async getCharacterData(id: number) {
    return this.client.get<CharacterData[]>(`/api/v1/characters/${id}/data`);
  }

  // Avatar endpoints
  async uploadCharacterAvatar(characterId: number, file: File) {
    logger.debug('Avatar upload starting', {
      characterId,
      fileName: file.name,
      fileSize: file.size,
      fileType: file.type,
    });

    const formData = new FormData();
    formData.append('avatar', file);

    logger.debug('Avatar upload FormData created', {
      characterId,
      fileName: file.name,
    });

    // CRITICAL: Must explicitly delete Content-Type header for multipart/form-data
    // The BaseApiClient sets a default 'Content-Type: application/json' header,
    // but for FormData uploads, axios needs to set the Content-Type itself with
    // the correct multipart boundary. We must delete the default header.
    try {
      const response = await this.client.post<{ avatar_url: string }>(
        `/api/v1/characters/${characterId}/avatar`,
        formData,
        {
          headers: {
            'Content-Type': undefined, // Remove default Content-Type, let axios set it
          },
        }
      );
      logger.debug('Avatar upload successful', { characterId });
      return response;
    } catch (error: unknown) {
      logger.error('Avatar upload failed', {
        characterId,
        message: (error as Error)?.message,
        status: (error as { response?: { status?: number } })?.response?.status,
      });
      throw error;
    }
  }

  async deleteCharacterAvatar(characterId: number) {
    return this.client.delete(`/api/v1/characters/${characterId}/avatar`);
  }

  // Player Management endpoints (GM only)
  async reassignCharacter(characterId: number, data: { new_owner_user_id: number }) {
    return this.client.put<Character>(`/api/v1/characters/${characterId}/reassign`, data);
  }

  async getInactiveCharacters(gameId: number) {
    return this.client.get<Character[]>(`/api/v1/games/${gameId}/characters/inactive`);
  }

  // Audience Participation endpoints
  async listAudienceNPCs(gameId: number) {
    return this.client.get<{ npcs: Character[] }>(`/api/v1/games/${gameId}/characters/audience-npcs`);
  }

  async getCharacterStats(characterId: number) {
    return this.client.get<CharacterActivityStats>(`/api/v1/characters/${characterId}/stats`);
  }

  // Stats for every character in a game in one request, keyed by character id
  // (as a string, since it comes back as JSON object keys). Used by roster
  // views instead of calling getCharacterStats once per character, which was
  // bursting the backend on large rosters.
  async getGameCharacterStats(gameId: number) {
    return this.client.get<Record<string, CharacterActivityStats>>(`/api/v1/games/${gameId}/characters/stats`);
  }

  async getCharacterComments(characterId: number, limit: number = 20, offset: number = 0) {
    const queryParams = new URLSearchParams();
    queryParams.append('limit', limit.toString());
    queryParams.append('offset', offset.toString());
    return this.client.get<CharacterMessagesResponse>(`/api/v1/characters/${characterId}/comments?${queryParams.toString()}`);
  }
}
