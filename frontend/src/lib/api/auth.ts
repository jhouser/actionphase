import { BaseApiClient } from './client';
import type {
  AuthResponse,
  LoginRequest,
  RegisterRequest,
  ChangePasswordRequest,
  ChangePasswordResponse,
  SessionsListResponse,
  User
} from '../../types/auth';

type Theme = 'light' | 'dark' | 'highContrast' | 'highContrastDark' | 'colorblind' | 'auto';
export type CommentReadMode = 'auto' | 'manual';
export type FontSize = 'small' | 'medium' | 'large';

export type NotificationTypePref =
  | 'private_message'
  | 'comment_reply'
  | 'character_mention'
  | 'action_submitted'
  | 'action_result'
  | 'common_room_post'
  | 'phase_created'
  | 'application_submitted'
  | 'application_approved'
  | 'character_approved'
  | 'game_state_changed'
  | 'handout_published';

export interface UserPreferences {
  theme: Theme;
  comment_read_mode: CommentReadMode;
  font_size: FontSize;
  discord_notifications?: Partial<Record<NotificationTypePref, boolean>>;
}

export interface DiscordStatus {
  linked: boolean;
  discord_username?: string;
}

interface DiscordConnectURL {
  url: string;
}

interface PreferencesResponse {
  preferences: UserPreferences;
}

/**
 * Authentication API client
 * Handles login, registration, token refresh, and user info
 */
export class AuthApi extends BaseApiClient {
  async login(data: LoginRequest) {
    return this.client.post<AuthResponse>('/api/v1/auth/login', data);
  }

  async register(data: RegisterRequest) {
    return this.client.post<AuthResponse>('/api/v1/auth/register', data);
  }

  async logout() {
    return this.client.post<void>('/api/v1/auth/logout', {});
  }

  async refreshToken() {
    const token = localStorage.getItem('auth_token');
    return this.refreshClient.get<{ token: string }>('/api/v1/auth/refresh', {
      headers: {
        Authorization: `Bearer ${token}`,
      },
    });
  }

  async getCurrentUser() {
    return this.client.get<User | { user: null }>('/api/v1/auth/me');
  }

  async getPreferences() {
    return this.client.get<PreferencesResponse>('/api/v1/auth/preferences');
  }

  async updatePreferences(preferences: UserPreferences) {
    return this.client.put<PreferencesResponse>('/api/v1/auth/preferences', {
      preferences,
    });
  }

  async searchUsers(query: string) {
    return this.client.get<{
      users: Array<{
        id: number;
        username: string;
        email: string;
        created_at: string;
      }>;
    }>(`/api/v1/auth/users/search?q=${encodeURIComponent(query)}`);
  }

  async changePassword(data: ChangePasswordRequest) {
    return this.client.post<ChangePasswordResponse>('/api/v1/auth/change-password', data);
  }

  async getSessions() {
    return this.client.get<SessionsListResponse>('/api/v1/auth/sessions');
  }

  async revokeSession(sessionId: number) {
    return this.client.delete(`/api/v1/auth/sessions/${sessionId}`);
  }

  async changeUsername(data: { new_username: string; current_password: string }) {
    return this.client.post<{ message: string }>('/api/v1/auth/change-username', data);
  }

  async requestEmailChange(data: { new_email: string; current_password: string }) {
    return this.client.post<{ message: string }>('/api/v1/auth/request-email-change', data);
  }

  async revokeAllSessions() {
    return this.client.post<{ message: string }>('/api/v1/auth/revoke-all-sessions', {});
  }

  async resendVerificationEmail() {
    return this.client.post<{ message: string }>('/api/v1/auth/resend-verification', {});
  }

  async requestPasswordReset(email: string) {
    return this.client.post<{ message: string }>('/api/v1/auth/request-password-reset', { email });
  }

  async validateResetToken(token: string) {
    return this.client.get<{ valid: boolean }>(`/api/v1/auth/validate-reset-token?token=${encodeURIComponent(token)}`);
  }

  async resetPassword(data: { token: string; new_password: string; confirm_password: string }) {
    return this.client.post<{ message: string }>('/api/v1/auth/reset-password', data);
  }

  async verifyEmail(token: string) {
    return this.client.post<{ message: string }>('/api/v1/auth/verify-email', { token });
  }

  async getDiscordStatus() {
    return this.client.get<DiscordStatus>('/api/v1/auth/discord/status');
  }

  async getDiscordConnectURL() {
    return this.client.get<DiscordConnectURL>('/api/v1/auth/discord/connect');
  }

  async disconnectDiscord() {
    return this.client.delete('/api/v1/auth/discord/disconnect');
  }
}
