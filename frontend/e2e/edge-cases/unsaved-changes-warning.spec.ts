import { test, expect } from '@playwright/test';
import { loginAs } from '../fixtures/auth-helpers';
import { getFixtureGameId } from '../fixtures/game-helpers';
import { navigateToGameAndTab, navigateViaNavLink } from '../utils/navigation';
import { MessagingPage } from '../pages/MessagingPage';

/**
 * E2E Tests: Unsaved Changes Navigation Warning
 *
 * Verifies that CommentEditor shows a confirmation dialog when the user
 * tries to navigate away (via React Router links) while unsaved text is
 * present. Uses warnOnUnsavedChanges={true} prop which is enabled on:
 * - Common room new post (GM only)
 * - Action submission
 * - Private messages (new message)
 *
 * Tests use GM's CreatePostForm for Common Room tests — it always has
 * a visible textarea without needing pre-seeded posts.
 */
test.describe('@mobile Unsaved Changes Warning', () => {

  test.describe('Common Room - New Post (GM)', () => {
    // Game 605 has no pre-seeded posts, so CreatePostForm starts expanded (not collapsed).
    // We use the GM's new-post textarea which always has warnOnUnsavedChanges enabled.

    test('shows warning dialog when navigating away with unsaved post', async ({ page }) => {
      await loginAs(page, 'GM');

      const gameId = await getFixtureGameId(page, 'COMMON_ROOM_CREATE_POST');
      await navigateToGameAndTab(page, gameId, 'Common Room');

      // The GM post form starts expanded when there are no posts
      const postTextarea = page.locator('textarea[placeholder*="Phase Title"]');
      await expect(postTextarea).toBeVisible({ timeout: 5000 });
      await postTextarea.fill('This is an unsaved GM post draft');

      // Navigate away via nav link (React Router in-app navigation)
      await navigateViaNavLink(page, 'Dashboard');

      // Warning dialog should appear
      await expect(page.getByText('Leave page?')).toBeVisible({ timeout: 5000 });
      await expect(page.getByText(/you have unsaved text/i)).toBeVisible();
    });

    test('Stay button keeps user on page with content intact', async ({ page }) => {
      await loginAs(page, 'GM');

      const gameId = await getFixtureGameId(page, 'COMMON_ROOM_CREATE_POST');
      await navigateToGameAndTab(page, gameId, 'Common Room');

      const postTextarea = page.locator('textarea[placeholder*="Phase Title"]');
      await expect(postTextarea).toBeVisible({ timeout: 5000 });

      const draftText = 'My unsaved draft text that should survive';
      await postTextarea.fill(draftText);

      // Attempt navigation
      await navigateViaNavLink(page, 'Dashboard');
      await expect(page.getByText('Leave page?')).toBeVisible({ timeout: 5000 });

      // Click Stay
      await page.getByRole('button', { name: 'Stay' }).click();

      // Dialog dismissed
      await expect(page.getByText('Leave page?')).not.toBeVisible();

      // Still on game page
      await expect(page).toHaveURL(new RegExp(`/games/${gameId}`));

      // Textarea content preserved
      await expect(postTextarea).toHaveValue(draftText);
    });

    test('Leave button proceeds with navigation', async ({ page }) => {
      await loginAs(page, 'GM');

      const gameId = await getFixtureGameId(page, 'COMMON_ROOM_CREATE_POST');
      await navigateToGameAndTab(page, gameId, 'Common Room');

      const postTextarea = page.locator('textarea[placeholder*="Phase Title"]');
      await expect(postTextarea).toBeVisible({ timeout: 5000 });
      await postTextarea.fill('Text I am OK abandoning');

      // Attempt navigation
      await navigateViaNavLink(page, 'Dashboard');
      await expect(page.getByText('Leave page?')).toBeVisible({ timeout: 5000 });

      // Click Leave
      await page.getByRole('button', { name: 'Leave' }).click();

      // Should have navigated
      await page.waitForLoadState('networkidle');
      await expect(page).toHaveURL('/dashboard');
    });

    test('no dialog shown when post textarea is empty', async ({ page }) => {
      await loginAs(page, 'GM');

      const gameId = await getFixtureGameId(page, 'COMMON_ROOM_CREATE_POST');
      await navigateToGameAndTab(page, gameId, 'Common Room');

      // Form is visible but empty
      await expect(page.locator('textarea[placeholder*="Phase Title"]')).toBeVisible({ timeout: 5000 });

      // Navigate away without typing
      await navigateViaNavLink(page, 'Dashboard');
      await page.waitForLoadState('networkidle');

      // Should navigate directly with no dialog
      await expect(page.getByText('Leave page?')).not.toBeVisible();
      await expect(page).toHaveURL('/dashboard');
    });
  });

  test.describe('Action Submission', () => {
    // The player-facing tab is "Submit Action", not "Actions" (that's the GM view)

    test('shows warning dialog when navigating away with unsaved action', async ({ page }) => {
      await loginAs(page, 'PLAYER_2');

      const gameId = await getFixtureGameId(page, 'E2E_ACTION');
      await navigateToGameAndTab(page, gameId, 'Submit Action');

      const actionTextarea = page.locator('textarea[placeholder*="Describe what your character"]');
      await expect(actionTextarea).toBeVisible({ timeout: 5000 });
      await actionTextarea.fill('My brilliant plan that I have not yet saved');

      // Navigate away
      await navigateViaNavLink(page, 'Dashboard');

      await expect(page.getByText('Leave page?')).toBeVisible({ timeout: 5000 });
    });

    test('Stay preserves action content', async ({ page }) => {
      await loginAs(page, 'PLAYER_3');

      const gameId = await getFixtureGameId(page, 'E2E_ACTION');
      await navigateToGameAndTab(page, gameId, 'Submit Action');

      const actionTextarea = page.locator('textarea[placeholder*="Describe what your character"]');
      await expect(actionTextarea).toBeVisible({ timeout: 5000 });

      const draftAction = 'My carefully drafted action plan';
      await actionTextarea.fill(draftAction);

      await navigateViaNavLink(page, 'Dashboard');
      await expect(page.getByText('Leave page?')).toBeVisible({ timeout: 5000 });

      await page.getByRole('button', { name: 'Stay' }).click();

      await expect(page.getByText('Leave page?')).not.toBeVisible();
      await expect(page).toHaveURL(new RegExp(`/games/${gameId}`));
      await expect(actionTextarea).toHaveValue(draftAction);
    });
  });

  test.describe('Private Messages', () => {
    // The message textarea only appears inside an open conversation thread.
    // We create a conversation first, then test the unsaved-changes warning.

    test('shows warning dialog when navigating away with unsaved message', async ({ page }) => {
      await loginAs(page, 'PLAYER_1');

      const gameId = await getFixtureGameId(page, 'E2E_PM');
      const messaging = new MessagingPage(page);
      await messaging.goto(gameId);

      // Create a conversation to get a message textarea
      const conversationTitle = `Unsaved Warning Test ${Date.now()}`;
      await messaging.createConversation(conversationTitle, ['E2E Test Char 2']);

      // The reply form is collapsed behind a "Reply" button at all widths —
      // click it to expand the composer and reveal the textarea.
      const replyButton = page.getByRole('button', { name: 'Reply' }).locator('visible=true').first();
      if (await replyButton.isVisible({ timeout: 1000 }).catch(() => false)) {
        await replyButton.click();
      }

      // The textarea should now be visible in the open conversation
      const messageTextarea = page.getByPlaceholder(/Type your message/i);
      await expect(messageTextarea).toBeVisible({ timeout: 5000 });
      await messageTextarea.fill('An unsaved private message draft');

      // Navigate away
      await navigateViaNavLink(page, 'Dashboard');

      await expect(page.getByText('Leave page?')).toBeVisible({ timeout: 5000 });
    });
  });
});
