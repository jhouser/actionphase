import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server';
import { renderWithProviders } from '../../test-utils/render';
import { CommonRoom } from '../CommonRoom';
import {
  parentContextForDepth,
  parentContextForViewport,
  COMMENT_MAX_DEPTH_MOBILE,
} from '../../config/comments';
import type { Message } from '../../types/messages';

/**
 * Regression tests for deep-linked (notification) comment fetching.
 *
 * When a user opens a notification for a deeply nested comment, CommonRoom
 * fetches the target comment plus a slice of its ancestor chain via
 * getMessageThreadContext. The modal renders that chain nested from depth 0,
 * so the number of parents requested must not exceed the deepest visible depth
 * for the current viewport — otherwise the target comment ends up buried below
 * its parents (past the "Continue this thread" boundary in the main view).
 */

describe('parentContextForDepth', () => {
  // The deepest visible depth is (maxDepth - 1), so at most that many parents.
  it('fetches 2 parents when mobile max depth is 3 (current prod)', () => {
    expect(parentContextForDepth(3)).toBe(2);
  });

  it('fetches 3 parents when max depth is 4 (planned prod / current dev)', () => {
    expect(parentContextForDepth(4)).toBe(3);
  });

  it('caps at the preferred context (3) for deep desktop threads', () => {
    expect(parentContextForDepth(5)).toBe(3);
    expect(parentContextForDepth(10)).toBe(3);
  });

  it('never exceeds what the viewport can render inline', () => {
    // A degenerate shallow config must not push the target out of view.
    expect(parentContextForDepth(2)).toBe(1);
  });
});

// A post so CommonRoom finishes loading (loading=false) and runs the scroll/fetch effect.
const mockPosts: Message[] = [
  {
    id: 1,
    game_id: 1,
    character_id: 1,
    character_name: 'Test Character',
    content: 'This is a test post',
    message_type: 'post',
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T00:00:00Z',
  },
];

// The deep-linked target comment id (not present in the DOM, forcing a fetch).
const TARGET_COMMENT_ID = 999;

function mockMatchMedia(isMobile: boolean) {
  window.matchMedia = vi.fn().mockImplementation((query: string) => ({
    // '(max-width: 767px)' is the mobile breakpoint CommonRoom checks.
    matches: query.includes('max-width') ? isMobile : false,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })) as unknown as typeof window.matchMedia;
}

/**
 * Renders CommonRoom deep-linked to TARGET_COMMENT_ID and returns the
 * max_parents query value the thread-context endpoint received.
 */
async function captureRequestedParents(): Promise<string | null> {
  let capturedMaxParents: string | null = null;
  let requestReceived = false;

  server.use(
    http.get('/api/v1/games/:gameId/posts', () => HttpResponse.json(mockPosts)),
    http.get('/api/v1/games/:gameId/unread-comment-ids', () => HttpResponse.json([])),
    http.get(
      '/api/v1/games/:gameId/messages/:messageId/thread-context',
      ({ request }) => {
        requestReceived = true;
        capturedMaxParents = new URL(request.url).searchParams.get('max_parents');
        // Return the target comment as a single-message chain so the modal opens
        // without error.
        return HttpResponse.json({
          chain: [
            {
              id: TARGET_COMMENT_ID,
              game_id: 1,
              character_id: 1,
              character_name: 'Test Character',
              content: 'The comment the user clicked',
              message_type: 'comment',
              created_at: '2024-01-01T00:00:00Z',
              updated_at: '2024-01-01T00:00:00Z',
            },
          ],
          root_post_id: 1,
          has_full_thread: true,
        });
      }
    )
  );

  renderWithProviders(<CommonRoom gameId={1} />, {
    gameId: 1,
    initialEntries: [`/games/1?tab=common-room&comment=${TARGET_COMMENT_ID}`],
  });

  await waitFor(() => expect(requestReceived).toBe(true));
  return capturedMaxParents;
}

describe('CommonRoom deep-link parent fetch', () => {
  beforeEach(() => {
    server.use(
      http.get('/api/v1/auth/me', () =>
        HttpResponse.json({ id: 1, username: 'testuser', email: 'test@example.com' })
      ),
      http.get('/api/v1/auth/refresh', () =>
        HttpResponse.json({ Token: 'mock-jwt-token' }, { status: 200 })
      )
    );
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('requests the viewport-appropriate parent count on mobile (not a hardcoded 3)', async () => {
    mockMatchMedia(true);
    const maxParents = await captureRequestedParents();
    // Must match the mobile-aware helper. With mobile depth 3 this is 2; with 4
    // it is 3. Asserting against the helper keeps the test correct as the env
    // var is retuned, while still catching a regression to a viewport-blind value.
    expect(maxParents).toBe(String(parentContextForViewport(true)));
    // Sanity check: never more than the deepest visible mobile depth.
    expect(Number(maxParents)).toBeLessThanOrEqual(COMMENT_MAX_DEPTH_MOBILE - 1);
  });

  it('requests the desktop parent count on desktop', async () => {
    mockMatchMedia(false);
    const maxParents = await captureRequestedParents();
    expect(maxParents).toBe(String(parentContextForViewport(false)));
  });
});
