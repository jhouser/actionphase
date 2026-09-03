import { test, expect } from '@playwright/test';
import { loginAs } from '../fixtures/auth-helpers';
import { tagTest, tags } from '../fixtures/test-tags';

/**
 * Smoke Tests: Application Health Checks
 *
 * Quick tests to verify basic application functionality.
 * Run before every deployment to catch critical failures.
 */
test.describe('Smoke: Application Health', () => {
  test(tagTest([tags.SMOKE], 'API health endpoint responds'), async ({ request }) => {
    // Relative path resolves against baseURL and is proxied to the backend,
    // so this works whether the app runs on the host or in the container stack.
    const response = await request.get('/health');
    expect(response.status()).toBe(200);
  });

  test(tagTest([tags.SMOKE], 'Dashboard requires authentication'), async ({ page }) => {
    await page.goto('/dashboard');
    await expect(page).toHaveURL(/\/login/);
  });

  test(tagTest([tags.SMOKE], 'Games list page requires authentication'), async ({ page }) => {
    await page.goto('/games');
    await expect(page).toHaveURL(/\/login/);
  });

  test(tagTest([tags.SMOKE], 'Logged-in user can reach dashboard and see games'), async ({ page }) => {
    await loginAs(page, 'PLAYER_1');
    await expect(page).toHaveURL(/\/dashboard/);
    await expect(page.getByRole('heading', { name: /dashboard/i })).toBeVisible();
    await expect(page.locator('[data-testid="game-card"]').first()).toBeVisible();
  });

  test(tagTest([tags.SMOKE], 'Notification bell is visible after login'), async ({ page }) => {
    await loginAs(page, 'PLAYER_5');
    await expect(page.locator('[data-testid="notification-bell"]')).toBeVisible();
  });

  // The guidelines live at /site-guidelines, renamed from /community-guidelines
  // once communities became a real entity and the old name read as one
  // community's rules rather than the site-wide floor.
  //
  // Tested here rather than in a component test because the redirect only
  // exists in the real router: App.test.tsx asserts against a hand-written copy
  // of the route table, so a redirect added there would pass without the
  // application having one.
  test(tagTest([tags.SMOKE], 'Site guidelines are readable without logging in'), async ({ page }) => {
    await page.goto('/site-guidelines');
    await expect(page).toHaveURL(/\/site-guidelines/);
    await expect(page.getByRole('heading', { name: 'Site Guidelines' })).toBeVisible();
  });

  test(tagTest([tags.SMOKE], 'Old community-guidelines URL redirects to site-guidelines'), async ({ page }) => {
    // The old path is linked from outside the app, so it has to keep working.
    // Asserting the heading too, not just the URL: a redirect that lands on a
    // route rendering nothing would still satisfy a URL-only check.
    await page.goto('/community-guidelines');
    await expect(page).toHaveURL(/\/site-guidelines/);
    await expect(page.getByRole('heading', { name: 'Site Guidelines' })).toBeVisible();
  });
});
