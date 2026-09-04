/**
 * Unified API Client
 *
 * Provides a single apiClient instance with all API endpoints organized by domain.
 *
 * Usage:
 *   import { apiClient } from '@/lib/api';
 *
 *   // Auth
 *   apiClient.auth.login({ username, password });
 *
 *   // Games
 *   apiClient.games.getGame(id);
 *
 *   // Characters
 *   apiClient.characters.createCharacter(gameId, data);
 *
 *   // Phases
 *   apiClient.phases.createPhase(gameId, data);
 *   apiClient.phases.submitAction(gameId, data);
 *
 *   // Messages
 *   apiClient.messages.createPost(gameId, data);
 *
 *   // Conversations
 *   apiClient.conversations.createConversation(gameId, data);
 *
 *   // Notifications
 *   apiClient.notifications.getNotifications();
 *
 *   // Polls
 *   apiClient.polls.createPoll(gameId, data);
 *   apiClient.polls.submitVote(pollId, data);
 *
 *   // Users
 *   apiClient.users.getUserProfile(userId);
 *   apiClient.users.updateUserProfile(data);
 *
 *   // Exports (completed games only)
 *   apiClient.exports.requestExport(gameId);
 *   apiClient.exports.getLatestExport(gameId);
 *
 *   // Utility methods
 *   apiClient.setAuthToken(token);
 *   apiClient.getAuthToken();
 *   apiClient.removeAuthToken();
 *   apiClient.ping();
 */

import { BaseApiClient } from './client';
import { AuthApi } from './auth';
import { GamesApi } from './games';
import { CharactersApi } from './characters';
import { PhasesApi } from './phases';
import { MessagesApi } from './messages';
import { ConversationsApi } from './conversations';
import { NotificationsApi } from './notifications';
import { AdminApi } from './admin';
import { CommunitiesApi } from './communities';
import { HandoutsApi } from './handouts';
import { DeadlinesApi } from './deadlines';
import { PollsApi } from './polls';
import { UsersApi } from './users';
import { ExportsApi } from './exports';

class ApiClient extends BaseApiClient {
  public auth: AuthApi;
  public games: GamesApi;
  public characters: CharactersApi;
  public phases: PhasesApi;
  public messages: MessagesApi;
  public conversations: ConversationsApi;
  public notifications: NotificationsApi;
  public admin: AdminApi;
  public communities: CommunitiesApi;
  public handouts: HandoutsApi;
  public deadlines: DeadlinesApi;
  public polls: PollsApi;
  public users: UsersApi;
  public exports: ExportsApi;

  constructor() {
    super();

    // Initialize domain-specific APIs
    this.auth = new AuthApi();
    this.games = new GamesApi();
    this.characters = new CharactersApi();
    this.phases = new PhasesApi();
    this.messages = new MessagesApi();
    this.conversations = new ConversationsApi();
    this.notifications = new NotificationsApi();
    this.admin = new AdminApi();
    this.communities = new CommunitiesApi();
    this.handouts = new HandoutsApi();
    this.deadlines = new DeadlinesApi();
    this.polls = new PollsApi();
    this.users = new UsersApi();
    this.exports = new ExportsApi();
  }
}

export const apiClient = new ApiClient();
