import { test, expect } from '@playwright/test';
import { loginAs } from '../fixtures/auth-helpers';
import { CommonRoomPage } from '../pages/CommonRoomPage';
import { getFixtureGameId, getWorkerGameId } from '../fixtures/game-helpers';
import { deepNestingTargets } from '../fixtures/comment-depth';

/**
 * E2E Tests: Common Room Flow
 *
 * Tests GM post creation, player commenting, nested replies, NPC posting, and post editing.
 *
 * Fixtures used:
 *   COMMON_ROOM_CREATE_POST (#605) — GM creates posts, player cannot create
 *   COMMON_ROOM_COMMENT     (#607) — pre-seeded post: 'Let's plan our approach'
 *   COMMON_ROOM_NESTED_REPLIES (#608) — pre-seeded post: 'What should we do next?'
 *   COMMON_ROOM_MULTIPLE_REPLIES (#609) — pre-seeded post: 'Who wants to go north?'
 *   COMMON_ROOM_MISC        (#167) — GM + unassigned NPCs (Mysterious Stranger, Town Guard)
 *   COMMON_ROOM_DEEP_NESTING     — pre-seeded deep thread 'Deep Thread Test Post'
 *   game #347               — co-GM NPC messaging; pre-seeded post: 'Has anyone seen unusual activity?'
 */

const FIXTURE_POST_607 = "Let's plan our approach";
const FIXTURE_POST_608 = 'What should we do next?';
const FIXTURE_POST_609 = 'Who wants to go north?';
const FIXTURE_POST_347 = 'Has anyone seen unusual activity?';

test.describe('@mobile Common Room Flow', () => {

  test('GM can create a post in Common Room', async ({ page }) => {
    await loginAs(page, 'GM');

    const gameId = await getFixtureGameId(page, 'COMMON_ROOM_CREATE_POST');
    const commonRoom = new CommonRoomPage(page);
    await commonRoom.goto(gameId);

    await expect(commonRoom.heading).toBeVisible({ timeout: 5000 });

    const postContent = `GM Post ${Date.now()}: Important mission briefing!`;
    await commonRoom.createPost(postContent);

    await commonRoom.verifyPostExists(postContent);
  });

  test('Player can comment on GM post', async ({ page }) => {
    await loginAs(page, 'PLAYER_1');

    const gameId = await getFixtureGameId(page, 'COMMON_ROOM_COMMENT');
    const commonRoom = new CommonRoomPage(page);
    await commonRoom.goto(gameId);

    await commonRoom.verifyPostExists(FIXTURE_POST_607);

    const commentContent = `Comment ${Date.now()}: I agree with this plan`;
    await commonRoom.addComment(FIXTURE_POST_607, commentContent);

    await commonRoom.verifyCommentExists(commentContent);
  });

  test('Players can reply to each others comments (nested replies)', async ({ page }) => {
    await loginAs(page, 'PLAYER_2');

    const gameId = await getFixtureGameId(page, 'COMMON_ROOM_NESTED_REPLIES');
    const commonRoom = new CommonRoomPage(page);
    await commonRoom.goto(gameId);

    const player2Comment = `Comment ${Date.now()}: I think we should scout ahead`;
    await commonRoom.addComment(FIXTURE_POST_608, player2Comment);
    await commonRoom.verifyCommentExists(player2Comment);

    // Player 1 replies to Player 2's comment
    await loginAs(page, 'PLAYER_1');
    await commonRoom.goto(gameId);
    await commonRoom.expandComments(FIXTURE_POST_608);

    const player1Reply = `Reply ${Date.now()}: Good idea, let's do it`;
    await commonRoom.replyToComment(player2Comment, player1Reply);

    const nestedReply = page.locator('[data-testid="threaded-comment"]').filter({ hasText: player1Reply }).locator('visible=true').first();
    await expect(nestedReply).toBeVisible({ timeout: 10000 });
  });

  test('Multiple players can reply to the same comment', async ({ page }) => {
    // Player 1 comments, then Player 2 comments — both visible on reload
    await loginAs(page, 'PLAYER_1');

    const gameId = await getFixtureGameId(page, 'COMMON_ROOM_MULTIPLE_REPLIES');
    const commonRoom = new CommonRoomPage(page);
    await commonRoom.goto(gameId);

    await commonRoom.verifyPostExists(FIXTURE_POST_609);

    const player1Comment = `P1 Reply ${Date.now()}: I do!`;
    await commonRoom.addComment(FIXTURE_POST_609, player1Comment);

    await loginAs(page, 'PLAYER_2');
    await commonRoom.goto(gameId);

    const player2Comment = `P2 Reply ${Date.now()}: Me too!`;
    await commonRoom.addComment(FIXTURE_POST_609, player2Comment);

    // Both comments are visible in the same page context
    await commonRoom.verifyCommentExists(player1Comment);
    await commonRoom.verifyCommentExists(player2Comment);
  });

  test('Deep nesting truncates inline and shows Continue this thread button', async ({ page }) => {
    await loginAs(page, 'PLAYER_1');

    const gameId = await getFixtureGameId(page, 'COMMON_ROOM_DEEP_NESTING');
    const commonRoom = new CommonRoomPage(page);
    await commonRoom.goto(gameId);

    const postContent = 'Deep Thread Test Post';
    const isMobile = page.viewportSize()!.width < 768;
    // Derived from the configured depth limits, not hardcoded: these targets are
    // really (maxDepth - 1) and maxDepth, so literals break whenever the env vars
    // are retuned. See e2e/fixtures/comment-depth.ts.
    const { deepestInline } = deepNestingTargets(isMobile);

    await expect(page.getByText(postContent).first()).toBeVisible({ timeout: 15000 });
    await commonRoom.expandComments(postContent);

    const deepestCommentLocator = page
      .locator('[data-testid="threaded-comment"]:visible')
      .filter({ has: page.getByText(deepestInline, { exact: true }) })
      .last();
    await expect(deepestCommentLocator).toBeVisible({ timeout: 15000 });

    const continueButton = deepestCommentLocator
      .getByRole('button', { name: /Continue this thread/ })
      .locator('visible=true')
      .first();
    await expect(continueButton).toBeVisible({ timeout: 5000 });
  });

  test('Thread modal shows deeper replies and allows replying', async ({ page }) => {
    await loginAs(page, 'PLAYER_1');

    const gameId = await getFixtureGameId(page, 'COMMON_ROOM_DEEP_NESTING');
    const commonRoom = new CommonRoomPage(page);
    await commonRoom.goto(gameId);

    const postContent = 'Deep Thread Test Post';
    const isMobile = page.viewportSize()!.width < 768;
    // Derived from the configured depth limits — see e2e/fixtures/comment-depth.ts.
    const { deepestInline, behindContinueButton: modalComment } =
      deepNestingTargets(isMobile);

    await expect(page.getByText(postContent).first()).toBeVisible({ timeout: 15000 });
    await commonRoom.expandComments(postContent);

    const deepestCommentLocator = page
      .locator('[data-testid="threaded-comment"]:visible')
      .filter({ has: page.getByText(deepestInline, { exact: true }) })
      .last();
    await deepestCommentLocator
      .getByRole('button', { name: /Continue this thread/ })
      .locator('visible=true')
      .first()
      .click();

    const modal = page.locator('.fixed.inset-0').filter({ hasText: 'Thread View' });
    await expect(modal.getByText('Thread View')).toBeVisible({ timeout: 5000 });
    // exact: true — the chain now runs to "Level 11", so a substring match on
    // "Nested Reply Level 1" would also hit Levels 10 and 11 (strict-mode violation).
    await expect(modal.getByText(modalComment, { exact: true })).toBeVisible({ timeout: 5000 });

    // `has:` must take a PAGE-rooted locator: Playwright re-resolves it relative
    // to each candidate, so a modal-rooted one (modal.getByText(...)) matches
    // nothing and the filter silently yields zero elements.
    const modalCommentContainer = modal
      .locator('[data-testid="threaded-comment"]:visible')
      .filter({ has: page.getByText(modalComment, { exact: true }) })
      .last();
    // Accessible name is "Reply to this comment" (the visible "Reply" label is
    // `hidden md:inline`). .first() is this comment's own button — the container
    // also holds its descendants'.
    await modalCommentContainer
      .getByRole('button', { name: /reply to this comment/i })
      .first()
      .click();

    const modalReply = `Modal Reply - ${Date.now()}`;
    const modalReplyTextarea = modalCommentContainer.locator('textarea').locator('visible=true').first();
    await modalReplyTextarea.waitFor({ state: 'visible', timeout: 5000 });
    await modalReplyTextarea.fill(modalReply);
    await modalCommentContainer.locator('form').locator('visible=true').first().evaluate((f: HTMLFormElement) => f.requestSubmit());
    await page.waitForLoadState('networkidle');

    await expect(modal.getByText(modalReply).locator('visible=true').first()).toBeVisible({ timeout: 10000 });
  });

  test('GM can create posts as NPCs', async ({ page }) => {
    await loginAs(page, 'GM');

    const gameId = await getFixtureGameId(page, 'COMMON_ROOM_MISC');
    const commonRoom = new CommonRoomPage(page);
    await commonRoom.goto(gameId);

    await expect(commonRoom.heading).toBeVisible({ timeout: 5000 });

    const post1 = `GM Post 1 ${Date.now()}: Testing NPC posting.`;
    await commonRoom.createPost(post1);
    await commonRoom.verifyPostExists(post1);

    const npcPost = `NPC Message ${Date.now()}: I have information about the quest.`;
    await commonRoom.createPost(npcPost, 'Mysterious Stranger');
    await commonRoom.verifyPostExists(npcPost);
  });

  test('Co-GM can create posts as NPCs', async ({ page }) => {
    await loginAs(page, 'AUDIENCE_1');

    const gameId = getWorkerGameId(347);
    const commonRoom = new CommonRoomPage(page);
    await commonRoom.goto(gameId);

    await expect(commonRoom.heading).toBeVisible({ timeout: 5000 });

    const post1 = `Co-GM Post 1 ${Date.now()}: Testing co-GM NPC posting.`;
    await commonRoom.createPost(post1);
    await commonRoom.verifyPostExists(post1);

    const npcPost = `Co-GM NPC Post ${Date.now()}: I've been watching from the shadows.`;
    await commonRoom.createPost(npcPost, 'Town Guard');
    await commonRoom.verifyPostExists(npcPost);
  });

  test('Co-GM comments as NPC on GM post', async ({ page }) => {
    await loginAs(page, 'AUDIENCE_1');

    const gameId = getWorkerGameId(347);
    const commonRoom = new CommonRoomPage(page);
    await commonRoom.goto(gameId);

    await commonRoom.verifyPostExists(FIXTURE_POST_347);

    const commentText = `Strange figures were seen near the old mill last night. ${Date.now()}`;
    await commonRoom.addComment(FIXTURE_POST_347, commentText, { asCharacter: 'Mysterious Stranger' });

    await commonRoom.verifyCommentExists(commentText);
  });

  test('GM replies to co-GM NPC comment', async ({ page }) => {
    await loginAs(page, 'AUDIENCE_1');

    const gameId = getWorkerGameId(347);
    const commonRoom = new CommonRoomPage(page);
    await commonRoom.goto(gameId);

    const coGmComment = `Town Guard report ${Date.now()}: suspicious activity at the docks`;
    await commonRoom.addComment(FIXTURE_POST_347, coGmComment, { asCharacter: 'Town Guard' });
    await commonRoom.verifyCommentExists(coGmComment);

    await loginAs(page, 'GM');
    await commonRoom.goto(gameId);
    await commonRoom.expandComments(FIXTURE_POST_347);

    const gmReply = `Reply ${Date.now()}: Thank you for the report, keep me informed`;
    await commonRoom.replyToComment(coGmComment, gmReply);

    await expect(page.getByText(gmReply).locator('visible=true').first()).toBeVisible({ timeout: 10000 });
  });

  test('Co-GM replies as NPC in thread', async ({ page }) => {
    await loginAs(page, 'GM');

    const gameId = getWorkerGameId(347);
    const commonRoom = new CommonRoomPage(page);
    await commonRoom.goto(gameId);

    const gmComment = `GM Comment ${Date.now()}: Citizens are concerned about recent events`;
    await commonRoom.addComment(FIXTURE_POST_347, gmComment);
    await commonRoom.verifyCommentExists(gmComment);

    await loginAs(page, 'AUDIENCE_1');
    await commonRoom.goto(gameId);
    await commonRoom.expandComments(FIXTURE_POST_347);

    const coGmReply = `Reply ${Date.now()}: I have information that may shed light on these events`;
    await commonRoom.replyToComment(gmComment, coGmReply, { asCharacter: 'Mysterious Stranger' });

    await expect(page.getByText(coGmReply).locator('visible=true').first()).toBeVisible({ timeout: 10000 });
  });

  test('GM can edit a comment and change the character', async ({ page }) => {
    await loginAs(page, 'GM');

    const gameId = await getFixtureGameId(page, 'COMMON_ROOM_MISC');
    const commonRoom = new CommonRoomPage(page);
    await commonRoom.goto(gameId);

    await expect(commonRoom.heading).toBeVisible({ timeout: 5000 });

    const postContent = `Test Post ${Date.now()}: Information from the shadows`;
    await commonRoom.createPost(postContent, 'Mysterious Stranger');
    await commonRoom.verifyPostExists(postContent);

    const commentContent = `Comment ${Date.now()}: I saw something unusual`;
    await commonRoom.addComment(postContent, commentContent, { asCharacter: 'Town Guard' });
    await commonRoom.verifyCommentExists(commentContent);

    await commonRoom.expandComments(postContent);

    const commentContainer = page.locator('[data-testid="threaded-comment"]').filter({ hasText: commentContent }).locator('visible=true').first();
    await expect(commentContainer.getByTestId('comment-author').filter({ hasText: 'Town Guard' }).locator('visible=true').first()).toBeVisible({ timeout: 5000 });

    await commentContainer.getByRole('button', { name: 'Edit' }).locator('visible=true').first().click();

    const characterSelect = commentContainer.locator('select').locator('visible=true').first();
    await characterSelect.waitFor({ state: 'visible', timeout: 5000 });
    await characterSelect.selectOption({ label: 'Edit as Mysterious Stranger' });

    await commentContainer.getByRole('button', { name: 'Save' }).locator('visible=true').first().click();
    await page.waitForLoadState('networkidle');

    await expect(commentContainer.getByTestId('comment-author').filter({ hasText: 'Mysterious Stranger' }).locator('visible=true').first()).toBeVisible({ timeout: 5000 });
    await expect(commentContainer.getByTestId('comment-author').filter({ hasText: 'Town Guard' }).first()).not.toBeVisible();
    await expect(commentContainer.getByText('(edited)').locator('visible=true').first()).toBeVisible({ timeout: 3000 });
  });

  test('Player cannot create posts in Common Room', async ({ page }) => {
    await loginAs(page, 'PLAYER_1');
    const gameId = await getFixtureGameId(page, 'COMMON_ROOM_CREATE_POST');
    const commonRoom = new CommonRoomPage(page);
    await commonRoom.goto(gameId);

    await expect(commonRoom.createPostHeading).not.toBeVisible({ timeout: 10000 });
  });

  test('GM can edit their own post', async ({ page }) => {
    await loginAs(page, 'GM');

    const gameId = await getFixtureGameId(page, 'COMMON_ROOM_MISC');
    const commonRoom = new CommonRoomPage(page);
    await commonRoom.goto(gameId);

    await expect(commonRoom.heading).toBeVisible({ timeout: 5000 });

    const originalContent = `Original Post ${Date.now()}: This is the initial content`;
    await commonRoom.createPost(originalContent);
    await commonRoom.verifyPostExists(originalContent);

    const postCard = commonRoom.getPostCard(originalContent);
    await postCard.getByRole('button', { name: /^edit$/i }).locator('visible=true').first().click();

    const textarea = page.locator('textarea[placeholder*="Edit your post"]').locator('visible=true').first();
    await textarea.waitFor({ state: 'visible', timeout: 5000 });

    const updatedContent = `Updated Post ${Date.now()}: This content has been changed`;
    await textarea.fill(updatedContent);
    await expect(textarea).toHaveValue(updatedContent);

    const saveButton = page.getByRole('button', { name: 'Save' }).locator('visible=true').first();
    await expect(saveButton).toBeEnabled();
    await saveButton.click();

    await expect(page.getByRole('button', { name: /^edit$/i }).locator('visible=true').first()).toBeVisible({ timeout: 10000 });
    await expect(page.getByText(updatedContent)).toBeVisible({ timeout: 5000 });
    await expect(page.getByText(originalContent)).not.toBeVisible();
    await expect(page.getByText('(edited)').locator('visible=true').first()).toBeVisible({ timeout: 5000 });
  });
});
