/**
 * Regression test: PostCard must request the *configured* comment depth.
 *
 * PostCard previously passed a hardcoded `5` as max_depth to all three
 * getPostCommentsWithThreads call sites. That silently defeated
 * VITE_COMMENT_MAX_DEPTH: raising the limit changed the render depth but not the
 * fetch depth, so the extra levels were never returned by the API and the
 * deepest visible comment showed "Continue thread" with nothing behind it.
 *
 * The config module is mocked to values that differ from the defaults so a
 * regression to a literal `5` fails here. A test that imported the real config
 * would pass against a hardcoded 5 whenever the config also happened to be 5.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, waitFor } from '@testing-library/react';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { AxiosResponse } from 'axios';
import { PostCard } from './PostCard';
import { ToastProvider } from '../contexts/ToastContext';
import { stubIntersectionObserver } from '../test-utils/mockIntersectionObserver';
import type { Message, PaginatedCommentsResponse } from '@/types/messages';
import type { Character } from '@/types/characters';

// Deliberately non-default depths: desktop deeper than the 5 that used to be
// hardcoded, mobile shallower, so the fetch depth must be max(8, 2) === 8.
const MOCK_DESKTOP_DEPTH = 8;
const MOCK_MOBILE_DEPTH = 2;

// Literals are inlined here rather than referencing the consts above: vi.mock is
// hoisted above them, so a reference would throw a TDZ ReferenceError.
vi.mock('@/config/comments', () => ({
  COMMENT_MAX_DEPTH: 8,
  COMMENT_MAX_DEPTH_MOBILE: 2,
  COMMENT_FETCH_MAX_DEPTH: 8,
  THREAD_VIEW_MAX_DEPTH: 10,
  parentContextForDepth: (maxDepth: number) => Math.min(3, maxDepth - 1),
  parentContextForViewport: () => 3,
}));

vi.mock('../lib/api', () => ({
  apiClient: {
    messages: {
      getPostCommentsWithThreads: vi.fn(),
    },
  },
}));

import { apiClient } from '../lib/api';

vi.mock('../hooks/useCommentMutations', () => ({
  useCreateComment: () => ({ mutateAsync: vi.fn() }),
  useUpdateComment: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useDeleteComment: () => ({ mutateAsync: vi.fn() }),
}));

vi.mock('../hooks/useAdminMode', () => ({
  useAdminMode: () => ({ adminModeEnabled: false }),
}));

vi.mock('../hooks/useScreenshotMode', () => ({
  useScreenshotMode: () => ({
    screenshotModeEnabled: false,
    toggleScreenshotMode: vi.fn(),
  }),
}));

vi.mock('../hooks/useGamePermissions', () => ({
  useGamePermissions: () => ({ isGM: false }),
}));

vi.mock('../hooks', () => ({
  useUpdatePost: () => ({ mutateAsync: vi.fn() }),
}));

vi.mock('../hooks/useReadTracking', () => ({
  useMarkPostAsRead: () => ({
    mutate: vi.fn(),
    mutateAsync: vi.fn(),
    isPending: false,
    isError: false,
    error: null,
  }),
  usePostUnreadCommentIDs: () => [],
  usePostManualReadCommentIDs: () => [],
  useToggleCommentRead: () => ({ mutate: vi.fn(), isPending: false }),
}));

vi.mock('../hooks/useUserPreferences', () => ({
  useCommentReadMode: () => 'auto',
}));

const mockPost: Message = {
  id: 1,
  game_id: 1,
  author_id: 1,
  character_id: 1,
  content: 'Test post content',
  message_type: 'post',
  thread_depth: 0,
  author_username: 'testuser',
  character_name: 'Test Character',
  character_avatar_url: null,
  comment_count: 3,
  reply_count: 0,
  is_edited: false,
  is_deleted: false,
  created_at: '2024-01-01T12:00:00Z',
  updated_at: '2024-01-01T12:00:00Z',
};

const mockCharacters: Character[] = [
  {
    id: 1,
    name: 'Test Character',
    username: 'testuser',
    character_type: 'player_character',
    avatar_url: null,
  } as Character,
];

const emptyResponse: PaginatedCommentsResponse = {
  comments: [],
  total_top_level: 0,
  returned_top_level: 0,
  returned_total: 0,
  has_more: false,
  limit: 5,
  offset: 0,
};

describe('PostCard - comment fetch depth', () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    stubIntersectionObserver();
    queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    });
    vi.clearAllMocks();
    vi.mocked(apiClient.messages.getPostCommentsWithThreads).mockReset();
    vi.mocked(apiClient.messages.getPostCommentsWithThreads).mockResolvedValue({
      data: emptyResponse,
    } as AxiosResponse<PaginatedCommentsResponse>);
  });

  const renderPostCard = () => {
    const router = createMemoryRouter([
      {
        path: '/',
        element: (
          <ToastProvider>
            <PostCard
              post={mockPost}
              gameId={1}
              characters={mockCharacters}
              controllableCharacters={mockCharacters}
              onCreateComment={vi.fn().mockResolvedValue(undefined)}
              currentUserId={1}
            />
          </ToastProvider>
        ),
      },
    ]);
    return render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    );
  };

  it('requests the configured depth, not a hardcoded 5, on initial load', async () => {
    renderPostCard();

    await waitFor(() => {
      expect(apiClient.messages.getPostCommentsWithThreads).toHaveBeenCalled();
    });

    // 5th arg (index 4) is max_depth: (gameId, postId, limit, offset, maxDepth).
    const maxDepthArg = vi.mocked(
      apiClient.messages.getPostCommentsWithThreads
    ).mock.calls[0][4];
    expect(maxDepthArg).toBe(MOCK_DESKTOP_DEPTH);
    expect(maxDepthArg).not.toBe(5);
  });

  it('fetches deep enough for whichever viewport limit is larger', async () => {
    // Desktop (8) and mobile (2) render from the SAME fetched tree — only
    // Tailwind md: classes hide one. Fetching the mobile depth would starve
    // desktop, so the request must cover the max of the two.
    renderPostCard();

    await waitFor(() => {
      expect(apiClient.messages.getPostCommentsWithThreads).toHaveBeenCalled();
    });

    const maxDepthArg = vi.mocked(
      apiClient.messages.getPostCommentsWithThreads
    ).mock.calls[0][4];
    expect(maxDepthArg).toBe(Math.max(MOCK_DESKTOP_DEPTH, MOCK_MOBILE_DEPTH));
  });
});
