import { Page, Locator, expect } from '@playwright/test';

export class DashboardPage {
  readonly page: Page;
  readonly myGamesSection: Locator;
  readonly notificationBadge: Locator;
  readonly gamesListContainer: Locator;
  readonly inbox: Locator;
  readonly inboxItems: Locator;

  constructor(page: Page) {
    this.page = page;
    this.myGamesSection = page.locator('[data-testid="my-games-section"]');
    this.notificationBadge = page.locator('[data-testid="notification-badge"]');
    this.gamesListContainer = page.locator('[data-testid="games-list"]');
    // The dashboard renders a mobile and a desktop DOM tree; scope to whichever
    // is actually visible so these locators work at any viewport.
    this.inbox = page.locator('[data-testid="unread-inbox"]').locator('visible=true').first();
    this.inboxItems = page.locator('[data-testid="unread-inbox-item"]').locator('visible=true');
  }

  async goto() {
    await this.page.goto('/dashboard');
    await this.page.waitForLoadState('networkidle');
  }

  async getGameCount(): Promise<number> {
    const games = await this.page.locator('[data-testid^="game-card-"]').count();
    return games;
  }

  async navigateToGame(gameId: number) {
    await this.page.click(`[data-testid="game-card-${gameId}"]`);
    await this.page.waitForURL(`**/games/${gameId}**`);
  }

  async getGameCardByStatus(status: string): Promise<Locator> {
    // Filter to visible element (viewport-agnostic for dual-DOM pattern)
    return this.page.locator(`[data-testid="game-status-${status}"]`).locator('visible=true').first();
  }

  async hasUnreadNotifications(): Promise<boolean> {
    return await this.notificationBadge.isVisible();
  }

  /**
   * Locate an inbox item by the notification title text shown on its header row.
   * @param titleText - Substring of the notification title (e.g. 'replied')
   */
  getInboxItem(titleText: string): Locator {
    return this.inboxItems.filter({ hasText: titleText }).first();
  }

  /**
   * Wait for an inbox item matching the given title text to appear, reloading
   * the dashboard until it does.
   *
   * The inbox is driven by a polled notifications query, so a notification
   * created moments earlier (by another user's action) may not be in the cache
   * yet. Reloading is more reliable than waiting out the poll interval.
   */
  async waitForInboxItem(titleText: string, timeout = 30000): Promise<Locator> {
    const item = this.getInboxItem(titleText);
    await expect(async () => {
      if (!(await item.isVisible().catch(() => false))) {
        await this.goto();
      }
      await expect(item).toBeVisible({ timeout: 2000 });
    }).toPass({ timeout, intervals: [1000] });
    return item;
  }

  /**
   * Expand an inbox item and wait for its context (the quoted source message)
   * to finish loading.
   *
   * Expanding is what triggers the item's context fetch, so callers that assert
   * on quoted content must go through here rather than clicking directly.
   */
  async expandInboxItem(titleText: string): Promise<Locator> {
    const item = await this.waitForInboxItem(titleText);
    const toggle = item.locator('[data-testid="unread-inbox-item-toggle"]');
    if ((await toggle.getAttribute('aria-expanded')) !== 'true') {
      await toggle.click();
    }
    await expect(item.locator('[data-testid="unread-inbox-item-context"]')).toBeVisible({
      timeout: 15000,
    });
    return item;
  }

  /**
   * Reply to an inbox item inline, from the dashboard.
   * Assumes the item is already expanded via expandInboxItem().
   *
   * Waits for the comment POST to actually come back before returning. Without
   * this the caller can navigate away while the request is still in flight,
   * which cancels it and loses the reply.
   */
  async replyToInboxItem(item: Locator, replyText: string) {
    await item.locator('[data-testid="unread-reply-textarea"]').fill(replyText);
    const sendButton = item.locator('[data-testid="unread-reply-send"]');
    await expect(sendButton).toBeEnabled();

    const replyPosted = this.page.waitForResponse(
      (response) =>
        /\/api\/v1\/games\/\d+\/posts\/\d+\/comments$/.test(new URL(response.url()).pathname) &&
        response.request().method() === 'POST',
      { timeout: 15000 }
    );
    await sendButton.click();
    const response = await replyPosted;
    expect(response.status(), 'inbox reply POST should succeed').toBeLessThan(400);
  }
}
