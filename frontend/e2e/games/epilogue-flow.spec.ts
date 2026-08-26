import { test, expect, type Page } from '@playwright/test';
import { loginAs } from '../fixtures/auth-helpers';
import { getFixtureGameId } from '../fixtures/game-helpers';
import { GameDetailsPage } from '../pages/GameDetailsPage';
import { CommonRoomPage } from '../pages/CommonRoomPage';
import { CharacterSheetPage } from '../pages/CharacterSheetPage';
import { CharacterWorkflowPage } from '../pages/CharacterWorkflowPage';
import { AudiencePage } from '../pages/AudiencePage';
import { assertTabVisible, assertTabNotVisible, navigateToGameTab } from '../utils/navigation';
import { assertTextVisible } from '../utils/assertions';

/**
 * E2E Tests: Epilogue Game State
 *
 * Epilogue is a writable public archive. It grants the READ permissions of
 * `completed` (the whole game opens to every authenticated user) while keeping
 * the WRITE permissions of `in_progress` (the GM can still run threads and
 * players can still post), so epilogue content can be written with the archive
 * open.
 *
 * Those are two separate mechanisms in the code — core.IsPublicArchive for
 * reads, core.ValidateGameNotCompleted for writes — coupled only by both
 * happening to trigger on `completed`. Every epilogue bug found by hand was a
 * site that conflated them, so this spec asserts each gate independently, at
 * the point where a real user would notice.
 *
 * Two fixture games, for two different kinds of question:
 *
 *   * E2E_EPILOGUE_STEADY (#349, seeded in epilogue, never mutated) backs the
 *     read-side tests. They run in any order, in parallel, and survive retries.
 *
 *   * E2E_EPILOGUE_TRANSITION (#348, starts in_progress) backs the transition
 *     tests, which must run serially because they advance shared state.
 *
 * Every probe targets content owned by TestPlayer2/3, never TestPlayer1 — a
 * player can always read their own messages, results, and sheet, so a
 * self-owned probe would pass identically in in_progress and prove nothing.
 */

const PRIVATE_MESSAGE = 'Only we know where the confession is hidden.';
const OTHER_PLAYERS_RESULT = 'The vault yields a sealed confession naming the magistrate.';
const SECRET_CONVERSATION = 'Epilogue Secret Conversation';
const SEEDED_POST = 'The dust settles over the magistrate';

/**
 * Open Epilogue Char 2's sheet (TestPlayer2's character) the way a user does:
 * People tab -> character card. Deliberately not a direct URL visit — the sheet
 * route is /characters/:characterId with no game segment, and going through the
 * game page also exercises the tab's own permissions.
 */
async function openOtherPlayersSheet(page: Page, gameId: number): Promise<CharacterSheetPage> {
  const people = new CharacterWorkflowPage(page, gameId);
  await people.goto();
  await people.openCharacterSheet('Epilogue Char 2');
  await expect(page.getByRole('heading', { name: 'Epilogue Char 2', level: 2 })).toBeVisible({ timeout: 10000 });
  return new CharacterSheetPage(page);
}

/**
 * Open the Results list for the archived action phase on the History tab.
 */
async function openArchivedResults(page: Page) {
  await navigateToGameTab(page, 'History');
  await page.getByRole('button', { name: 'Epilogue Fixture Action Phase' }).click();
  await page.waitForLoadState('networkidle');
  await page.getByRole('button', { name: 'Results' }).click();
  await page.waitForLoadState('networkidle');
}

test.describe('Epilogue Game State — read access (steady state)', () => {
  test('Player sees another players private messages via the Audience tab', async ({ page }) => {
    await loginAs(page, 'PLAYER_1');
    const gameId = await getFixtureGameId(page, 'E2E_EPILOGUE_STEADY');

    await new GameDetailsPage(page).goto(gameId);
    await expect(page.getByTestId('game-status-badge')).toContainText(/epilogue/i, { timeout: 10000 });

    // A plain player gets audience-level read access once the archive opens.
    await assertTabVisible(page, 'Audience');

    const audience = new AudiencePage(page);
    await audience.goToAudience(gameId);
    await audience.verifyConversationExists(SECRET_CONVERSATION);
    await audience.openConversation(SECRET_CONVERSATION);
    // A conversation TestPlayer1 was never a participant in.
    await audience.verifyMessageExists(PRIVATE_MESSAGE);
  });

  test('Player sees another players action results in History', async ({ page }) => {
    await loginAs(page, 'PLAYER_1');
    const gameId = await getFixtureGameId(page, 'E2E_EPILOGUE_STEADY');

    await new GameDetailsPage(page).goto(gameId);
    await openArchivedResults(page);

    // This is the whole-game results feed that returned 403 before epilogue was
    // taught to be a public archive — not /results/mine.
    await assertTextVisible(page, OTHER_PLAYERS_RESULT, { timeout: 10000 });
  });

  test('Player sees another players private character sheet modules', async ({ page }) => {
    await loginAs(page, 'PLAYER_1');
    const gameId = await getFixtureGameId(page, 'E2E_EPILOGUE_STEADY');

    const sheet = await openOtherPlayersSheet(page, gameId);
    expect(await sheet.isModuleVisible('Public Profile')).toBe(true);
    expect(await sheet.isModuleVisible('Private Notes')).toBe(true);
    expect(await sheet.isModuleVisible('Skills')).toBe(true);
  });

  test('Epilogue hides the Actions tab but keeps History', async ({ page }) => {
    await loginAs(page, 'PLAYER_1');
    const gameId = await getFixtureGameId(page, 'E2E_EPILOGUE_STEADY');

    await new GameDetailsPage(page).goto(gameId);

    // Play is over in epilogue: no submissions, no incoming results. The tab
    // previously rendered an empty pane labelled "Submit Action", which read as
    // broken. Archived submissions stay readable on History instead.
    await assertTabNotVisible(page, 'Submit Action');
    await assertTabNotVisible(page, 'Actions');
    await assertTabVisible(page, 'History');
  });

  test('Epilogue keeps the game writable: player can still post in the common room', async ({ page }) => {
    await loginAs(page, 'PLAYER_1');
    const gameId = await getFixtureGameId(page, 'E2E_EPILOGUE_STEADY');

    // The write gate is the half epilogue must NOT inherit from completed.
    // A player writes comments, not top-level posts (those are GM-only), so
    // commenting on the seeded thread is the real player-side write path.
    const commonRoom = new CommonRoomPage(page);
    await commonRoom.goto(gameId);

    const comment = `Epilogue reply from TestPlayer1 ${Date.now()}`;
    await commonRoom.addComment(SEEDED_POST, comment);
    await commonRoom.verifyCommentExists(comment);
  });
});

test.describe('Epilogue Game State — transitions', () => {
  // Serial: these walk one shared game through
  // in_progress -> epilogue -> completed. Order is load-bearing.
  test.describe.configure({ mode: 'serial' });

  test('Player cannot see other players private content while game is in progress', async ({ page }) => {
    await loginAs(page, 'PLAYER_1');
    const gameId = await getFixtureGameId(page, 'E2E_EPILOGUE_TRANSITION');

    await new GameDetailsPage(page).goto(gameId);

    // Baseline. Without this the "hidden" assertions below would also pass
    // against an already-open game, which is the bug they exist to catch.
    await expect(page.getByTestId('game-status-badge')).toContainText(/in.?progress/i, { timeout: 10000 });

    // No audience-level access mid-game, so no Audience tab.
    await assertTabNotVisible(page, 'Audience');

    // Another player's published result is not in this player's History.
    await openArchivedResults(page);
    await expect(page.getByText(OTHER_PLAYERS_RESULT)).toHaveCount(0);

    // Another player's private sheet modules are hidden.
    const sheet = await openOtherPlayersSheet(page, gameId);
    expect(await sheet.isModuleVisible('Public Profile')).toBe(true);
    expect(await sheet.isModuleVisible('Private Notes')).toBe(false);
  });

  test('GM moves the game to epilogue', async ({ page }) => {
    await loginAs(page, 'GM');
    const gameId = await getFixtureGameId(page, 'E2E_EPILOGUE_TRANSITION');

    const gamePage = new GameDetailsPage(page);
    await gamePage.goto(gameId);

    await expect(page.getByLabel('Game actions')).toBeVisible({ timeout: 10000 });
    await gamePage.moveToEpilogue();

    const statusBadge = page.getByTestId('game-status-badge');
    await expect(statusBadge).toContainText(/epilogue/i, { timeout: 10000 });

    // Reload to confirm the transition persisted, not just optimistic UI.
    await page.reload();
    await expect(statusBadge).toContainText(/epilogue/i, { timeout: 10000 });
  });

  test('Moving to epilogue discloses content that was hidden moments earlier', async ({ page }) => {
    await loginAs(page, 'PLAYER_1');
    const gameId = await getFixtureGameId(page, 'E2E_EPILOGUE_TRANSITION');

    await new GameDetailsPage(page).goto(gameId);
    await expect(page.getByTestId('game-status-badge')).toContainText(/epilogue/i, { timeout: 10000 });

    // The same three probes the in-progress test found hidden, now open —
    // against the same rows, so the transition is what changed.
    await assertTabVisible(page, 'Audience');

    await openArchivedResults(page);
    await assertTextVisible(page, OTHER_PLAYERS_RESULT, { timeout: 10000 });

    const sheet = await openOtherPlayersSheet(page, gameId);
    expect(await sheet.isModuleVisible('Private Notes')).toBe(true);
  });

  test('GM completes the game from epilogue', async ({ page }) => {
    await loginAs(page, 'GM');
    const gameId = await getFixtureGameId(page, 'E2E_EPILOGUE_TRANSITION');

    const gamePage = new GameDetailsPage(page);
    await gamePage.goto(gameId);
    await gamePage.completeGame();

    await page.reload();
    await page.waitForLoadState('networkidle');
    await expect(page.getByTestId('game-status-badge')).toContainText(/completed/i, { timeout: 10000 });
  });

  test('Completing closes writing but leaves the archive open', async ({ page }) => {
    await loginAs(page, 'PLAYER_1');
    const gameId = await getFixtureGameId(page, 'E2E_EPILOGUE_TRANSITION');

    await new GameDetailsPage(page).goto(gameId);
    await expect(page.getByTestId('game-status-badge')).toContainText(/completed/i, { timeout: 10000 });

    // Write surface is gone: a completed game drops the Common Room tab.
    await assertTabNotVisible(page, 'Common Room');

    // Reads stay open. This is what separates completed from cancelled, and
    // confirms locking writes did not require undoing the disclosure.
    await assertTabVisible(page, 'Audience');
    const audience = new AudiencePage(page);
    await audience.goToAudience(gameId);
    await audience.verifyConversationExists(SECRET_CONVERSATION);
  });
});
