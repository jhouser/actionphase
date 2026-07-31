import { BaseApiClient } from './client';
import type {
  Conversation,
  ConversationListItem,
  ConversationWithDetails,
  PrivateMessage,
  CreateConversationRequest,
  SendMessageRequest,
  AddParticipantRequest,
  UpdateMessageRequest
} from '../../types/conversations';

/**
 * Conversations API client
 * Handles private messaging between characters
 */
export class ConversationsApi extends BaseApiClient {
  async createConversation(gameId: number, data: CreateConversationRequest) {
    return this.client.post<Conversation>(`/api/v1/games/${gameId}/conversations`, data);
  }

  async getUserConversations(gameId: number, options?: { unreadOnly?: boolean; limit?: number }) {
    const params = new URLSearchParams();
    if (options?.unreadOnly) params.set('unread_only', 'true');
    if (options?.limit) params.set('limit', String(options.limit));
    const qs = params.toString();
    return this.client.get<{ conversations: ConversationListItem[] }>(
      `/api/v1/games/${gameId}/conversations${qs ? `?${qs}` : ''}`
    );
  }

  async getConversation(gameId: number, conversationId: number) {
    return this.client.get<ConversationWithDetails>(`/api/v1/games/${gameId}/conversations/${conversationId}`);
  }

  /**
   * Deletes a conversation that has no messages in it. Rejected with 409 by the
   * server if any message exists, or 403 if the caller is not the creator/GM.
   */
  async deleteConversation(gameId: number, conversationId: number) {
    return this.client.delete<{ message: string; id: number }>(`/api/v1/games/${gameId}/conversations/${conversationId}`);
  }

  async getConversationMessages(gameId: number, conversationId: number) {
    return this.client.get<{ messages: PrivateMessage[] }>(`/api/v1/games/${gameId}/conversations/${conversationId}/messages`);
  }

  async sendMessage(gameId: number, conversationId: number, data: SendMessageRequest) {
    return this.client.post<PrivateMessage>(`/api/v1/games/${gameId}/conversations/${conversationId}/messages`, data);
  }

  async markConversationAsRead(gameId: number, conversationId: number) {
    return this.client.post<{ success: boolean }>(`/api/v1/games/${gameId}/conversations/${conversationId}/read`);
  }

  async addParticipant(gameId: number, conversationId: number, data: AddParticipantRequest) {
    return this.client.post<{ success: boolean }>(`/api/v1/games/${gameId}/conversations/${conversationId}/participants`, data);
  }

  async deleteMessage(gameId: number, conversationId: number, messageId: number) {
    return this.client.delete<{ message: string; id: number }>(`/api/v1/games/${gameId}/conversations/${conversationId}/messages/${messageId}`);
  }

  async updateMessage(gameId: number, conversationId: number, messageId: number, data: UpdateMessageRequest) {
    return this.client.patch<PrivateMessage>(`/api/v1/games/${gameId}/conversations/${conversationId}/messages/${messageId}`, data);
  }
}
