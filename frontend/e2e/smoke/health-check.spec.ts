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
});
