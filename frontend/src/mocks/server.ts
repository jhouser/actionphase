import { setupServer } from 'msw/node';
import { http, HttpResponse } from 'msw';
import type { UserProfileResponse } from '../types/user-profiles';

// Mock data
const mockUserProfile: UserProfileResponse = {
  user: {
    id: 1,
    username: 'testuser',
    display_name: 'Test User',
    bio: 'This is a test bio',
    avatar_url: 'http://localhost:3000/uploads/avatars/users/1/test.jpg',
    created_at: '2024-01-01T00:00:00Z',
    timezone: 'America/New_York',
    is_admin: false,
  },
  games: [
    {
      game_id: 1,
      title: 'Test Game 1',
      gm_username: 'gm1',
      state: 'recruiting',
      user_role: 'player',
      is_anonymous: false,
      start_date: null,
      end_date: null,
      created_at: '2024-01-01T00:00:00Z',
      updated_at: '2024-01-01T00:00:00Z',
      characters: [
        {
          id: 101,
          name: 'Test Character',
          avatar_url: null,
          character_type: 'warrior',
        },
      ],
    },
    {
      game_id: 2,
      title: 'Test Game 2',
      gm_username: 'gm2',
      state: 'active',
      user_role: 'gm',
      is_anonymous: false,
      start_date: null,
      end_date: null,
      created_at: '2024-01-02T00:00:00Z',
      updated_at: '2024-01-02T00:00:00Z',
      characters: [],
    },
  ],
  metadata: {
    page: 1,
    page_size: 12,
    total_count: 2,
    total_pages: 1,
    has_next_page: false,
    has_previous_page: false,
  },
};

// Request handlers
const handlers = [
  // Auth endpoints
  http.get('/api/v1/auth/me', () => {
    return HttpResponse.json({
      id: 1,
      username: 'testuser',
      email: 'test@example.com',
      display_name: 'Test User',
      avatar_url: 'http://localhost:3000/uploads/avatars/users/1/test.jpg',
      created_at: '2024-01-01T00:00:00Z',
      timezone: 'America/New_York',
      is_admin: false,
    });
  }),

  http.post('/api/v1/auth/refresh', () => {
    return HttpResponse.json(
      { Token: 'mock-jwt-token' },
      { status: 200 }
    );
  }),

  http.get('/api/v1/auth/refresh', () => {
    return HttpResponse.json(
      { Token: 'mock-jwt-token' },
      { status: 200 }
    );
  }),

  // User profile endpoints
  http.get('/api/v1/users/username/:username/profile', () => {
    return HttpResponse.json(mockUserProfile);
  }),

  http.get('/api/v1/users/:id/profile', () => {
    return HttpResponse.json(mockUserProfile);
  }),

  http.patch('/api/v1/users/me/profile', async ({ request }) => {
    const updates = await request.json() as Partial<UserProfileResponse['user']>;
    return HttpResponse.json({
      ...mockUserProfile,
      user: { ...mockUserProfile.user, ...updates },
    });
  }),

  // Game-related endpoints (fallback handlers to prevent 404s)
  http.get('/api/v1/games/:gameId/details', ({ params }) => {
    return HttpResponse.json({
      id: Number(params.gameId),
      title: 'Test Game',
      description: 'A test game',
      gm_user_id: 1,
      gm_username: 'testgm',
      state: 'setup',
      max_players: 4,
      is_public: true,
      is_anonymous: false,
      auto_accept_audience: false,
      game_config: {},
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    });
  }),

  http.get('/api/v1/games/:gameId/participants', () => {
    return HttpResponse.json([
      {
        id: 1,
        user_id: 1,
        username: 'testuser',
        role: 'player',
        status: 'active',
      },
    ]);
  }),

  http.get('/api/v1/games/:gameId/polls', () => {
    return HttpResponse.json([]);
  }),

  http.get('/api/v1/games/:gameId/phases', () => {
    return HttpResponse.json([]);
  }),

  http.get('/api/v1/games/:gameId/results', () => {
    return HttpResponse.json([]);
  }),

  http.get('/api/v1/games/:gameId/results/mine', () => {
    return HttpResponse.json([]);
  }),

  http.get('/api/v1/games/:gameId/results/:resultId/character-updates', () => {
    return HttpResponse.json([]);
  }),

  http.get('/api/v1/games/:gameId/results/:resultId/character-updates/count', () => {
    return HttpResponse.json({ count: 0 });
  }),

  http.get('/api/v1/games/:gameId/characters', () => {
    return HttpResponse.json([]);
  }),

  http.get('/api/v1/games/:gameId/characters/mine', () => {
    return HttpResponse.json([]);
  }),

  // Stub handlers for background requests made incidentally during component tests.
  // Do NOT add stubs here for routes that have their own isolated-server hook tests
  // (e.g. useNotifications, useReadTracking) — those tests create their own MSW server
  // and both run simultaneously, so a stub here will intercept before the test's handler.
  http.get('/api/v1/auth/preferences', () => {
    return HttpResponse.json({ preferences: {} });
  }),

  http.get('/api/v1/games/:gameId/manual-read-comment-ids', () => {
    return HttpResponse.json([]);
  }),

  http.get('/api/v1/characters/:id/stats', () => {
    return HttpResponse.json({ stats: {} });
  }),

  http.get('/api/v1/games/:gameId/posts/:postId/comments-with-threads', () => {
    return HttpResponse.json([]);
  }),

  http.get('/api/v1/games/:gameId/conversations', () => {
    return HttpResponse.json([]);
  }),

  http.get('/api/v1/games/:gameId/characters/controllable', () => {
    return HttpResponse.json([]);
  }),

  http.get('/api/v1/games/:gameId/characters/stats', () => {
    return HttpResponse.json({});
  }),

  http.get('/api/v1/characters/:characterId/data', () => {
    return HttpResponse.json([]);
  }),

  // Every new game requires a community (req 5), so the create form fetches
  // this list. One entry, so the form preselects it and tests that only care
  // about other fields do not have to pick one.
  http.get('/api/v1/communities', () => {
    return HttpResponse.json([
      {
        id: 1,
        name: 'Test Community',
        slug: 'test-community',
        description: null,
        banner_url: null,
        owner_user_id: 1,
        owner_username: 'testuser',
        is_active: true,
        created_at: '2024-01-01T00:00:00Z',
        updated_at: '2024-01-01T00:00:00Z',
      },
    ]);
  }),

];

// Setup server with handlers
export const server = setupServer(...handlers);
