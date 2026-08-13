import { Page, expect } from '@playwright/test';

/**
 * Game Management Helper Functions for E2E Tests
 */

/**
 * Well-known test fixture games
 * Use these constants with getFixtureGameId() to avoid brittle hardcoded IDs
 */
export const FIXTURE_GAMES = {
  // Shared fixtures (READ-ONLY - do not modify in tests)
  HEIST: 'The Heist at Goldstone Bank',           // In-progress, has phases, characters
  WESTMARCH: 'Chronicles of Westmarch',           // In-progress, lots of phases (pagination test)
  SHADOWS: 'Shadows Over Innsmouth',              // In-progress, common room phase
  DRAGON: 'The Dragon of Mount Krag',             // In-progress, complex history
  MANOR: 'The Mystery of Blackwood Manor',        // Recruitment state

  // Dedicated E2E fixtures (STATE-MODIFYING - safe to complete/cancel/modify)
  E2E_COMPLETE: 'E2E Test: Game to Complete',     // For testing game completion
  E2E_CANCEL: 'E2E Test: Game to Cancel',         // For testing game cancellation
  E2E_PAUSE: 'E2E Test: Game to Pause',           // For testing pause/resume
  E2E_ACTION: 'E2E Test: Action Submission',      // For testing action submissions
  E2E_ACTION_RESULTS: 'E2E Test: Action Results', // For testing action results viewing (completed phase + common room)
  E2E_GM_EDITING_RESULTS: 'E2E Test: GM Editing Results', // For testing GM editing unpublished results (active action phase)
  E2E_LIFECYCLE: 'E2E Test: Phase Lifecycle',     // For testing complete phase lifecycle
  E2E_DRAFT_POSTS: 'E2E Test: Draft Posts',       // For testing draft post create/edit/publish flow
  E2E_MESSAGES: 'E2E Test: Private Messages',     // For testing private messages (dedicated game)
  E2E_PM: 'E2E Test: Private Messages',           // Alias for E2E_MESSAGES
  E2E_CHARACTER_SHEETS: 'E2E Test: Character Sheets', // For testing character sheet management
  E2E_LOOT_TABLES: 'E2E Test: Loot Tables',       // For testing loot tables and rolling loot onto a character
  E2E_GAME_SETTINGS: 'E2E Test: Game Settings',   // For testing game settings modifications
  CO_GM_MANAGEMENT: 'E2E Test: Co-GM Management',  // For testing co-GM promotion/demotion
  CO_GM_ACTION_RESULTS: 'E2E Test: Co-GM Action Results',  // Co-GM game with active action phase for action result editing
  E2E_GAME_APPLICATION_SUBMIT: 'E2E Test: Game Application - Submit', // Fresh game for testing player application submission
  E2E_GAME_APPLICATION_VIEW: 'E2E Test: Game Application - View', // Game with pending application for GM to view
  E2E_GAME_APPLICATION_APPROVE: 'E2E Test: Game Application - Approve', // Game with pending application for GM to approve
  E2E_GAME_APPLICATION_REJECT: 'E2E Test: Game Application - Reject', // Game with pending application for GM to reject
  E2E_GAME_APPLICATION_DUPLICATE: 'E2E Test: Game Application - Duplicate', // Game with existing application for duplicate prevention test
  E2E_GAME_APPLICATION_PUBLIC_LIST: 'E2E Test: Game Application - Public List', // Game with pre-seeded applications for public applicant list test
  E2E_GAME_CHARACTER_CREATION_AUDIENCE: 'E2E Test: Character Creation Audience', // Game in character_creation state for testing audience joining
  E2E_GAME_LIFECYCLE_START: 'E2E Test: Game Lifecycle - Start', // Game in recruitment ready to start
  E2E_GAME_LIFECYCLE_PAUSE: 'E2E Test: Game Lifecycle - Pause', // Active game ready to pause
  E2E_GAME_LIFECYCLE_RESUME: 'E2E Test: Game Lifecycle - Resume', // Paused game ready to resume
  E2E_GAME_LIFECYCLE_COMPLETE: 'E2E Test: Game Lifecycle - Complete', // Active game ready to complete
  E2E_GAME_LIFECYCLE_CANCEL: 'E2E Test: Game Lifecycle - Cancel', // Active game ready to cancel

  // Isolated Common Room games for parallel E2E testing (one per test file)
  COMMON_ROOM_POSTS: 'E2E Common Room - Posts',           // Game #164 - for common-room.spec.ts
  COMMON_ROOM_MENTIONS: 'E2E Common Room - Mentions',     // Game #165 - for character-mentions.spec.ts
  COMMON_ROOM_NOTIFICATIONS: 'E2E Common Room - Notifications', // Game #166 - for notification-flow.spec.ts
  COMMON_ROOM_MISC: 'E2E Common Room - Misc',             // Game #167 - for misc tests
  CHARACTER_AVATARS: 'E2E Character Avatars',             // Game #168 - for character-avatar.spec.ts
  COMMON_ROOM_POLLS: 'E2E Common Room - Polls',           // Game #169 - for polls-flow.spec.ts

  // Isolated games for common-room.spec.ts individual tests (605-610)
  COMMON_ROOM_CREATE_POST: 'E2E Common Room - Create Post',       // Game #605 - "GM can create a post"
  COMMON_ROOM_VIEW_POSTS: 'E2E Common Room - View Posts',         // Game #606 - "Player can view GM posts"
  COMMON_ROOM_COMMENT: 'E2E Common Room - Comment',               // Game #607 - "Player can comment on GM post"
  COMMON_ROOM_NESTED_REPLIES: 'E2E Common Room - Nested Replies', // Game #608 - "Players can reply to each others comments"
  COMMON_ROOM_MULTIPLE_REPLIES: 'E2E Common Room - Multiple Replies', // Game #609 - "Multiple players can reply to the same comment"
  COMMON_ROOM_DEEP_NESTING: 'E2E Common Room - Deep Nesting',     // Game #610 - "Deep nesting shows Continue this thread button"

  // Deep linking test (701)
  DEEP_LINKING_TEST: 'E2E Deep Linking Test',                      // Game #701 - "Deep linking regression tests"

  // Infinite scroll test (710) — shared, read-only
  INFINITE_SCROLL: 'E2E Test: Infinite Scroll',                    // Game #710 - "infinite-scroll.spec.ts"

  // Manual read tracking test (702)
  MANUAL_READ_TRACKING: 'E2E Test: Manual Read Tracking',          // Game #702 - "manual-read-tracking.spec.ts"

  // Unread comment badge test (703)
  UNREAD_TRACKING: 'E2E Test: Unread Tracking',                    // Game #703 - "unread-tracking.spec.ts"

  // Notification flow tests (704, 705)
  NOTIFICATION_FLOW: 'E2E Test: Notification Flow',                // Game #704 - reply/mention notification tests
  NOTIFICATION_PHASE: 'E2E Test: Notification Phase',              // Game #705 - phase-activation notification test

  // Dashboard unread inbox test (706)
  UNREAD_INBOX: 'E2E Test: Unread Inbox',                          // Game #706 - unread-inbox.spec.ts (inline reply)

  // Player multiple characters test (340-345, worker-specific)
  PLAYER_MULTIPLE_CHARACTERS: 'E2E Test: Player Multiple Characters', // Game #340 - "player-multiple-characters.spec.ts"

  // Character workflow games (isolated for parallel testing)
  E2E_CHARACTER_CREATION: 'E2E Test: Character Creation',  // character_creation state with approved player (no character)
  E2E_CHARACTER_PENDING_STATE: 'E2E Test: Character Approval - Pending State',  // For "character starts in pending state" test
  E2E_CHARACTER_VIEW_PENDING: 'E2E Test: Character Approval - View Pending',   // For "GM can view pending characters" test
  E2E_CHARACTER_APPROVE: 'E2E Test: Character Approval - Approve',             // For "GM can approve character" test
  E2E_CHARACTER_REJECT: 'E2E Test: Character Approval - Reject',               // For "GM can reject character" test
  E2E_CHARACTER_RESUBMIT: 'E2E Test: Character Approval - Resubmit',           // For "rejected character can be resubmitted" test
  E2E_CHARACTER_IN_GAME: 'E2E Test: Character Approval - In Game',             // For "approved characters appear in active game" test
  E2E_CHARACTER_APPROVAL: 'E2E Test: Character Approval - Pending State',      // Deprecated alias - use specific fixtures instead
  E2E_GM_MESSAGING: 'E2E Test: GM Messaging',              // in_progress with GM having multiple NPCs for messaging tests
  E2E_AUDIENCE_PM: 'E2E Test: Audience Private Messages',  // Game #360 - audience view of all private messages

  // Player-to-audience (permadeath) transition tests (370, worker-specific)
  PLAYER_TO_AUDIENCE: 'E2E Test: Player to Audience',

  // Legacy alias (deprecated - use COMMON_ROOM_POSTS instead)
  COMMON_ROOM_TEST: 'E2E Common Room - Posts',    // Alias for Game #164
} as const;

/**
 * Get worker index for parallel test execution
 * @returns Worker index (0-5)
 */
function getWorkerIndex(): number {
  const workerIndex = process.env.TEST_PARALLEL_INDEX
    ? parseInt(process.env.TEST_PARALLEL_INDEX, 10)
    : 0;
  return workerIndex;
}

/**
 * Get worker-specific username for test assertions
 * @param baseUsername - Base username (e.g., "TestPlayer4", "TestGM")
 * @returns Worker-specific username (e.g., "TestPlayer4" for worker 0, "TestPlayer4_1" for worker 1)
 */
export function getWorkerUsername(baseUsername: string): string {
  const workerIndex = getWorkerIndex();
  return workerIndex === 0 ? baseUsername : `${baseUsername}_${workerIndex}`;
}

/**
 * Calculate worker-specific game ID
 * Each worker gets games with IDs offset by worker_index * 10000
 * @param baseGameId - Base game ID from fixture (e.g., 200 for E2E_ACTION)
 * @returns Worker-specific game ID (e.g., 200 for worker 0, 10200 for worker 1)
 */
export function getWorkerGameId(baseGameId: number): number {
  const workerIndex = getWorkerIndex();
  const gameIdOffset = workerIndex * 10000;
  return baseGameId + gameIdOffset;
}

/**
 * Get a game ID by its title (resilient to fixture resets)
 * For parallel execution, this searches only the current worker's games
 * @param page - Playwright page object
 * @param title - Game title to search for
 * @returns Game ID or null if not found
 */
export async function getGameIdByTitle(page: Page, title: string): Promise<number | null> {
  // Calculate worker-specific game ID range
  const workerIndex = getWorkerIndex();
  const minId = workerIndex * 10000;
  const maxId = minId + 9999;

  // Wait for network to be idle to ensure page has fully loaded and cookies are set
  await page.waitForLoadState('networkidle');

  // Use page.evaluate to run fetch in the browser context where cookies are available
  // This ensures HTTP-only JWT cookies are automatically sent with the request
  // Use search parameter to filter for the specific game title (API searches title and description)
  const responseData = await page.evaluate(async (searchTitle) => {
    const response = await fetch(`/api/v1/games?page_size=100&search=${encodeURIComponent(searchTitle)}`, {
      credentials: 'include',
    });

    if (!response.ok) {
      throw new Error(`Failed to fetch games: ${response.status}`);
    }

    return response.json();
  }, title);

  // Extract games array from response (endpoint returns { games: [...], metadata: {...} })
  const games = responseData.games || [];

  // Filter to only this worker's games (by ID range) and match title
  const game = games.find((g: { title: string; id: number }) =>
    g.title === title &&
    g.id >= minId &&
    g.id <= maxId
  );

  return game ? game.id : null;
}

/**
 * Get the ID for a well-known fixture game
 * @param page - Playwright page object
 * @param gameKey - Key from FIXTURE_GAMES
 * @returns Game ID
 * @throws Error if game not found
 */
export async function getFixtureGameId(
  page: Page,
  gameKey: keyof typeof FIXTURE_GAMES
): Promise<number> {
  const title = FIXTURE_GAMES[gameKey];
  const gameId = await getGameIdByTitle(page, title);

  if (gameId === null) {
    throw new Error(`Fixture game not found: ${title}. Did you apply test fixtures?`);
  }

  return gameId;
}

/**
 * Fetch comment IDs for the deep-linking test fixture (game #701).
 *
 * Queries the API for the single post in the game, then its flat comment list,
 * and returns the IDs of the Level 3 and Level 5 comments by their known content.
 * This avoids DOM-scraping in tests and survives fixture resets.
 *
 * @param page - Playwright page (must be logged in)
 * @param gameId - Worker-adjusted game ID for the deep-linking fixture
 */
export async function getDeepLinkingCommentIds(
  page: Page,
  gameId: number
): Promise<{ shallowCommentId: number; deepCommentId: number }> {
  const result = await page.evaluate(async (gid: number) => {
    const postsResp = await fetch(`/api/v1/games/${gid}/posts`, { credentials: 'include' });
    if (!postsResp.ok) throw new Error(`Failed to fetch posts: ${postsResp.status}`);
    const posts: Array<{ id: number }> = await postsResp.json();
    if (!posts.length) throw new Error('No posts found in deep-linking fixture');

    const postId = posts[0].id;
    const commentsResp = await fetch(`/api/v1/games/${gid}/posts/${postId}/comments-with-threads?limit=100`, { credentials: 'include' });
    if (!commentsResp.ok) throw new Error(`Failed to fetch comments: ${commentsResp.status}`);
    const commentsData = await commentsResp.json();
    const comments: Array<{ id: number; content: string }> = commentsData.comments ?? commentsData;

    const shallow = comments.find(c => c.content.startsWith('Level 3 comment'));
    const deep    = comments.find(c => c.content.startsWith('Level 5 comment'));
    if (!shallow) throw new Error('Level 3 comment not found in deep-linking fixture');
    if (!deep)    throw new Error('Level 5 comment not found in deep-linking fixture');

    return { shallowCommentId: shallow.id, deepCommentId: deep.id };
  }, gameId);

  return result;
}

/**
 * Set the comment_read_mode preference for the currently logged-in user via the API.
 * Faster and more reliable than navigating to Settings and clicking through the UI.
 *
 * @param page - Playwright page (must be logged in)
 * @param mode - 'auto' | 'manual'
 */
export async function setCommentReadMode(page: Page, mode: 'auto' | 'manual'): Promise<void> {
  const status = await page.evaluate(async (m: string) => {
    const resp = await fetch('/api/v1/auth/preferences', {
      method: 'PUT',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ preferences: { comment_read_mode: m } }),
    });
    return resp.status;
  }, mode);

  if (status !== 200) {
    throw new Error(`setCommentReadMode failed: PUT /api/v1/me/preferences returned ${status}`);
  }
}

/**
 * Fetch the ID of the first post whose content matches a given string.
 * Used to get the post ID needed for API-level comment creation.
 *
 * @param page - Playwright page (must be logged in)
 * @param gameId - Worker-adjusted game ID
 * @param content - Exact post content to match
 */
export async function getPostIdByContent(page: Page, gameId: number, content: string): Promise<number> {
  const postId = await page.evaluate(async (args: { gameId: number; content: string }) => {
    const resp = await fetch(`/api/v1/games/${args.gameId}/posts`, { credentials: 'include' });
    if (!resp.ok) throw new Error(`Failed to fetch posts: ${resp.status}`);
    const posts: Array<{ id: number; content: string }> = await resp.json();
    const match = posts.find(p => p.content === args.content);
    if (!match) throw new Error(`Post not found: "${args.content}"`);
    return match.id;
  }, { gameId, content });
  return postId;
}

/**
 * Add a comment to a post via the API, using the current page's auth session.
 * Fetches the logged-in user's first controllable character in the game to use
 * as the comment author.
 *
 * @param page - Playwright page (must be logged in as the commenter)
 * @param gameId - Worker-adjusted game ID
 * @param postId - ID of the post to comment on
 * @param content - Comment text
 */
export async function addCommentViaApi(
  page: Page,
  gameId: number,
  postId: number,
  content: string,
): Promise<void> {
  const status = await page.evaluate(async (args: {
    gameId: number; postId: number; content: string;
  }) => {
    const charsResp = await fetch(`/api/v1/games/${args.gameId}/characters/controllable`, {
      credentials: 'include',
    });
    if (!charsResp.ok) throw new Error(`Failed to fetch controllable characters: ${charsResp.status}`);
    const chars: Array<{ id: number }> = await charsResp.json();
    if (!chars.length) throw new Error('No controllable characters found');

    const resp = await fetch(`/api/v1/games/${args.gameId}/posts/${args.postId}/comments`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ character_id: chars[0].id, content: args.content }),
    });
    return resp.status;
  }, { gameId, postId, content });

  if (status !== 200 && status !== 201) {
    throw new Error(`addCommentViaApi failed: POST comments returned ${status}`);
  }
}

/**
 * Create a draft action result for a player via the API (GM must be logged in).
 * Returns the new result's ID.
 *
 * @param page - Playwright page (must be logged in as GM)
 * @param gameId - Game ID
 * @param userId - Numeric user ID of the player to create the result for
 * @param content - Draft result content
 */
export async function createDraftResultViaApi(
  page: Page,
  gameId: number,
  userId: number,
  content: string,
): Promise<number> {
  const result = await page.evaluate(async (args: { gameId: number; userId: number; content: string }) => {
    const resp = await fetch(`/api/v1/games/${args.gameId}/results`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ user_id: args.userId, content: args.content, is_published: false }),
    });
    if (!resp.ok) throw new Error(`Failed to create draft result: ${resp.status}`);
    const data = await resp.json();
    return data.id as number;
  }, { gameId, userId, content });

  return result;
}

export interface CreateGameOptions {
  title: string;
  description?: string;
  maxPlayers?: number;
  isPublic?: boolean;
}

/**
 * Create a new game
 * @param page - Playwright page object
 * @param options - Game creation options
 * @returns Object with gameId
 */
export async function createGame(
  page: Page,
  options: CreateGameOptions
): Promise<{ gameId: number }> {
  await page.goto('/games');

  // Click create game button
  await page.click('text=Create Game');

  // Fill in game details
  await page.fill('input[name="title"]', options.title);

  if (options.description) {
    await page.fill('textarea[name="description"]', options.description);
  }

  if (options.maxPlayers) {
    await page.fill('input[name="maxPlayers"]', options.maxPlayers.toString());
  }

  // Submit form
  await page.click('button:has-text("Create")');

  // Wait for navigation to game details page
  await page.waitForURL(/\/games\/\d+/, { timeout: 10000 });

  // Extract game ID from URL
  const url = page.url();
  const gameId = parseInt(url.match(/\/games\/(\d+)/)?.[1] || '0');

  return { gameId };
}

/**
 * Start recruitment for a game
 * @param page - Playwright page object
 * @param gameId - Game ID
 */
export async function startRecruitment(page: Page, gameId: number) {
  await page.goto(`/games/${gameId}`);

  // Click start recruitment button
  await page.click('button:has-text("Start Recruitment")');

  // Wait for state change confirmation
  await expect(page.locator('text=Recruitment')).toBeVisible({ timeout: 5000 });
}

/**
 * Apply to join a game
 * @param page - Playwright page object
 * @param gameId - Game ID
 */
export async function applyToGame(page: Page, gameId: number) {
  await page.goto(`/games/${gameId}`);

  // Click apply to join button
  await page.click('button:has-text("Apply to Join")');

  // Wait for confirmation
  await expect(page.locator('text=Application Submitted')).toBeVisible({ timeout: 5000 });
}

/**
 * Approve a player's application to join a game
 * @param page - Playwright page object (must be logged in as GM)
 * @param gameId - Game ID
 * @param playerUsername - Username of player to approve
 */
export async function approveApplication(
  page: Page,
  gameId: number,
  playerUsername: string
) {
  await page.goto(`/games/${gameId}`);

  // Open applications tab/section
  await page.click('text=Applications');

  // Find the player's application row and approve
  const applicationRow = page.locator(`tr:has-text("${playerUsername}")`);
  await applicationRow.locator('button:has-text("Approve")').click();

  // Wait for confirmation
  await expect(page.locator(`text=${playerUsername}`).first()).toBeVisible();
}

/**
 * Navigate to game details page
 * @param page - Playwright page object
 * @param gameId - Game ID
 */
export async function goToGame(page: Page, gameId: number) {
  await page.goto(`/games/${gameId}`);
  await page.waitForLoadState('networkidle');
}

/**
 * Navigate to games list page
 * @param page - Playwright page object
 */
export async function goToGamesList(page: Page) {
  await page.goto('/games');
  await page.waitForLoadState('networkidle');
}

/**
 * Check if game is visible in games list
 * @param page - Playwright page object
 * @param gameTitle - Title of the game to find
 */
export async function isGameVisible(page: Page, gameTitle: string): Promise<boolean> {
  await goToGamesList(page);

  try {
    await page.waitForSelector(`text=${gameTitle}`, { timeout: 2000 });
    return true;
  } catch {
    return false;
  }
}

/**
 * Create a phase for a game
 * @param page - Playwright page object (must be logged in as GM)
 * @param gameId - Game ID
 * @param phaseType - Type of phase ('common_room', 'action', 'results')
 * @param options - Phase creation options
 */
export async function createPhase(
  page: Page,
  gameId: number,
  phaseType: 'common_room' | 'action' | 'interlude',
  options: {
    title: string;
    description?: string;
    deadline?: Date;
  }
) {
  await page.goto(`/games/${gameId}`);

  // Open phase management tab
  await page.click('[role="tab"]:has-text("Phase Management")');

  // Click create phase button
  await page.click('button:has-text("Create Phase")');

  // Select phase type
  await page.selectOption('select[name="phaseType"]', phaseType);

  // Fill in phase details
  await page.fill('input[name="title"]', options.title);

  if (options.description) {
    await page.fill('textarea[name="description"]', options.description);
  }

  if (options.deadline) {
    // Format date for input (YYYY-MM-DDTHH:mm)
    const formatted = options.deadline.toISOString().slice(0, 16);
    await page.fill('input[name="deadline"]', formatted);
  }

  // Submit form
  await page.click('button:has-text("Create")');

  // Wait for phase to appear in list
  await expect(page.locator(`text=${options.title}`)).toBeVisible({ timeout: 5000 });
}

/**
 * Get the numeric user ID for a participant in a game by username.
 * Makes an authenticated API call using the page's cookie context.
 *
 * @param page - Playwright page (must be logged in as GM)
 * @param gameId - Game ID to query participants for
 * @param username - Username to look up (e.g., "TestAudience2" or "TestAudience2_1")
 * @returns Numeric user ID
 */
export async function getParticipantUserId(
  page: Page,
  gameId: number,
  username: string
): Promise<number> {
  const data = await page.evaluate(async (args: { gameId: number; username: string }) => {
    const response = await fetch(`/api/v1/games/${args.gameId}/participants`, {
      credentials: 'include',
    });
    if (!response.ok) throw new Error(`Failed to fetch participants: ${response.status}`);
    return response.json();
  }, { gameId, username });

  // API returns a flat array: [{ user_id, username, role, ... }, ...]
  const participants: Array<{ user_id: number; username: string }> = Array.isArray(data) ? data : (data.participants ?? []);
  const match = participants.find((p) => p.username === username);
  if (!match) throw new Error(`Participant "${username}" not found in game ${gameId}`);
  return match.user_id;
}

/**
 * Demote whoever is currently the co-GM in a game (if any).
 * Fetches participants, finds the co_gm role holder, and demotes them.
 * No-op if no co-GM exists.
 *
 * @param page - Playwright page (must be logged in as GM)
 * @param gameId - Game ID
 */
export async function demoteCurrentCoGM(page: Page, gameId: number): Promise<void> {
  const data = await page.evaluate(async (args: { gameId: number }) => {
    const response = await fetch(`/api/v1/games/${args.gameId}/participants`, {
      credentials: 'include',
    });
    if (!response.ok) throw new Error(`Failed to fetch participants: ${response.status}`);
    return response.json();
  }, { gameId });

  const participants: Array<{ user_id: number; role: string }> = Array.isArray(data) ? data : (data.participants ?? []);
  const coGM = participants.find((p) => p.role === 'co_gm');
  if (coGM) {
    await demoteFromCoGM(page, gameId, coGM.user_id);
  }
}

/**
 * Promote a user to co-GM via the API (GM must be logged in).
 *
 * @param page - Playwright page (must be logged in as GM)
 * @param gameId - Game ID
 * @param userId - Numeric user ID to promote
 */
export async function promoteToCoGM(page: Page, gameId: number, userId: number): Promise<void> {
  const result = await page.evaluate(async (args: { gameId: number; userId: number }) => {
    const response = await fetch(`/api/v1/games/${args.gameId}/participants/${args.userId}/promote-to-co-gm`, {
      method: 'POST',
      credentials: 'include',
    });
    const body = response.status !== 204 ? await response.text() : '';
    return { status: response.status, body };
  }, { gameId, userId });

  if (result.status !== 200 && result.status !== 204) {
    // 400 "already has a co-GM" is idempotent — user is already promoted, treat as success
    if (result.status === 400 && result.body.includes('already has a co-GM')) return;
    throw new Error(`promoteToCoGM failed with status ${result.status}: ${result.body}`);
  }
}

/**
 * Demote a user from co-GM via the API (GM must be logged in).
 * Does nothing (and does not throw) if the user is not currently a co-GM.
 *
 * @param page - Playwright page (must be logged in as GM)
 * @param gameId - Game ID
 * @param userId - Numeric user ID to demote
 */
export async function demoteFromCoGM(page: Page, gameId: number, userId: number): Promise<void> {
  const result = await page.evaluate(async (args: { gameId: number; userId: number }) => {
    const response = await fetch(`/api/v1/games/${args.gameId}/participants/${args.userId}/demote-from-co-gm`, {
      method: 'POST',
      credentials: 'include',
    });
    const body = response.status !== 204 ? await response.text() : '';
    return { status: response.status, body };
  }, { gameId, userId });

  if (result.status !== 200 && result.status !== 204) {
    // 400 "not currently a co-GM" is idempotent — user is already audience, treat as success for cleanup
    if (result.status === 400 && result.body.includes('can only demote co-GMs')) return;
    throw new Error(`demoteFromCoGM failed with status ${result.status}: ${result.body}`);
  }
}

/**
 * Transition a player to audience via the API (primary GM must be logged in).
 * This is the permadeath transition — it sets role='audience' and is_former_player=true.
 * NOTE: This operation is irreversible via the API. Reset with fixture re-application.
 *
 * @param page - Playwright page (must be logged in as the primary GM)
 * @param gameId - Game ID
 * @param userId - Numeric user ID of the player to transition
 */
export async function transitionPlayerToAudience(page: Page, gameId: number, userId: number): Promise<void> {
  const result = await page.evaluate(async (args: { gameId: number; userId: number }) => {
    const response = await fetch(`/api/v1/games/${args.gameId}/participants/${args.userId}/to-audience`, {
      method: 'POST',
      credentials: 'include',
    });
    const body = response.status !== 204 ? await response.text() : '';
    return { status: response.status, body };
  }, { gameId, userId });

  if (result.status !== 200 && result.status !== 204) {
    throw new Error(`transitionPlayerToAudience failed with status ${result.status}: ${result.body}`);
  }
}
