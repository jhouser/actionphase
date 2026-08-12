import { test, expect } from '@playwright/test';
import { loginAs } from '../fixtures/auth-helpers';
import { getFixtureGameId } from '../fixtures/game-helpers';
import { CommonRoomPage } from '../pages/CommonRoomPage';
import { DashboardPage } from '../pages/DashboardPage';

/**
 * E2E Test for the dashboard Unread Inbox (inline reply).
 *
 * Fixture:
 *   UNREAD_INBOX (Game #706) — dedicated, because this test marks a
 *   notification read and posts a comment.
 *
 * Why this is worth an E2E test when the inbox has thorough unit coverage:
 * the inbox has no backend endpoint of its own. It is assembled client-side
 * from the unread notifications list, and each expanded item fans out to
 * several more endpoints to resolve its context — the comment, its parent, the
 * root post id, and the repliable characters. Replying then posts through the
 * normal comment API using a root_post_id derived by walking the thread.
 *
 * Every step of that chain is mocked in the unit tests, so a change to
 * notification titles, related_id semantics, or thread shape could break the
 * inbox in production with all unit tests still green. This test exercises the
 * real chain end to end.
 */

const PLAYER_1_COMMENT = 'Player 1 comment on inbox test post';

test.describe('Dashboard Unread Inbox', () => {
  test('player can reply to a comment reply from the dashboard inbox', async ({ browser }) => {
    const player1Context = await browser.newContext();
    const player2Context = await browser.newContext();
    const player1Page = await player1Context.newPage();
    const player2Page = await player2Context.newPage();

    try {
      await loginAs(player1Page, 'PLAYER_1');
      await loginAs(player2Page, 'PLAYER_2');

      const gameId = await getFixtureGameId(player1Page, 'UNREAD_INBOX');

      // Setup: Player 2 replies to Player 1's pre-seeded comment, which
      // generates the comment_reply notification Player 1's inbox should pick up.
      const player2CommonRoom = new CommonRoomPage(player2Page);
      await player2CommonRoom.goto(gameId);
      await player2CommonRoom.expandComments('Inbox Test Post');
      const replyFromPlayer2 = `Player 2 reply ${Date.now()}`;
      await player2CommonRoom.replyToComment(PLAYER_1_COMMENT, replyFromPlayer2);

      // Player 1 opens the dashboard and expands the inbox item.
      const dashboard = new DashboardPage(player1Page);
      await dashboard.goto();
      const item = await dashboard.expandInboxItem('replied');

      // The quoted context is the real reply text, which proves the whole
      // client-side context fan-out resolved against live data.
      await expect(
        item.locator('[data-testid="unread-inbox-item-context"]')
      ).toContainText(replyFromPlayer2);

      // Player 1 replies inline, without leaving the dashboard.
      const replyFromPlayer1 = `Inbox reply ${Date.now()}`;
      await dashboard.replyToInboxItem(item, replyFromPlayer1);

      // The reply landed in the actual thread, nested under Player 2's reply —
      // this is what verifies root_post_id/parent resolution was correct.
      const player1CommonRoom = new CommonRoomPage(player1Page);
      await player1CommonRoom.goto(gameId);
      await player1CommonRoom.expandComments('Inbox Test Post');
      await player1CommonRoom.verifyCommentExists(replyFromPlayer1);

      const parentComment = player1Page
        .locator('[data-testid="threaded-comment"]')
        .filter({ hasText: replyFromPlayer2 })
        .locator('visible=true')
        .first();
      await expect(parentComment).toContainText(replyFromPlayer1);
    } finally {
      await player1Context.close();
      await player2Context.close();
    }
  });
});
