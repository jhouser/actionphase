import { test, expect } from '@playwright/test';
import { loginAs } from '../fixtures/auth-helpers';
import { assertUrl } from '../utils/assertions';
import { SettingsPage } from '../pages/SettingsPage';
import { DEFAULT_TIMEOUT } from '../config/test-timeouts';

/**
 * Journey: User Account Security Management
 *
 * Tests the account security features available in Settings:
 * - Change Username
 * - Change Email (with verification)
 *
 * NOTE: The username-change test intercepts the API call at the network layer
 * so it never mutates the shared test user's username in the DB. The backend
 * handler tests own the DB-mutation assertion; this test owns the UI flow
 * (form renders, submits, toast appears, display updates). Because username
 * changes have a 30-day cooldown, a real mutation would be non-restorable
 * within a run — so the intercept (not any DB reset) is what keeps the fixture
 * user clean. The remaining tests only trigger validation / wrong-password
 * paths or the email form, none of which mutate the username. No DB teardown
 * is needed, so this suite does not touch the database directly.
 */

test.describe('Account Security Management', () => {
  test('should successfully change username', async ({ page }) => {
    await loginAs(page, 'PLAYER_1');
    await assertUrl(page, '/dashboard');

    // Intercept the change-username API call — return synthetic success without
    // hitting the DB so the shared test user's username is never mutated.
    await page.route('**/api/v1/me/change-username', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ message: 'Username changed successfully' }),
      }), { times: 1 }
    );

    // After the change, AuthContext re-fetches /api/v1/me. Intercept that once
    // to return the updated username so the display assertion can pass.
    await page.route('**/api/v1/me', async route => {
      const original = await route.fetch();
      const body = await original.json();
      if (body?.user) {
        body.user.username = 'TestPlayer1Updated';
      }
      await route.fulfill({ response: original, json: body });
    }, { times: 1 });

    const settingsPage = new SettingsPage(page);
    await settingsPage.goto();
    await assertUrl(page, '/settings');
    await settingsPage.clickAccountInformation();

    await expect(settingsPage.getCurrentUsernameDisplay()).toBeVisible();
    await expect(settingsPage.getCurrentUsernameDisplay()).toContainText('TestPlayer1');

    await settingsPage.changeUsername('TestPlayer1Updated', 'testpassword123');

    await expect(page.locator('text=/Username changed successfully/i')).toBeVisible({ timeout: DEFAULT_TIMEOUT });
    await expect(settingsPage.getCurrentUsernameDisplay()).toContainText('TestPlayer1Updated', { timeout: DEFAULT_TIMEOUT });
  });

  test('should show validation error when changing username without password', async ({ page }) => {
    // Login as Player 2
    await loginAs(page, 'PLAYER_2');

    const settingsPage = new SettingsPage(page);
    await settingsPage.goto();
    await settingsPage.clickAccountInformation();

    // Fill in only the new username (no password)
    await settingsPage.getNewUsernameInput().fill('TestPlayer2Updated');
    await settingsPage.getChangeUsernameSubmit().click();

    // Should show validation error
    await expect(settingsPage.getChangeUsernameForm().locator('text=/Current password is required/i')).toBeVisible();
  });

  test('should successfully request email change', async ({ page }) => {
    // Login as Player 3
    await loginAs(page, 'PLAYER_3');

    const settingsPage = new SettingsPage(page);
    await settingsPage.goto();
    await settingsPage.clickAccountInformation();

    // Scroll email form into view
    await settingsPage.getChangeEmailForm().scrollIntoViewIfNeeded();
    await expect(settingsPage.getChangeEmailForm()).toBeVisible();

    // Verify current email is displayed
    await expect(settingsPage.getCurrentEmailDisplay()).toBeVisible();
    await expect(settingsPage.getCurrentEmailDisplay()).toContainText('test_player3');

    // Verify info alert about verification is shown
    await expect(settingsPage.getEmailVerificationInfo()).toBeVisible();
    await expect(settingsPage.getEmailVerificationInfo()).toContainText('verification email will be sent');

    // Request email change
    await settingsPage.requestEmailChange('player3_updated@example.com', 'testpassword123');

    // Wait for success toast
    await expect(page.locator('text=/Verification email sent/i')).toBeVisible({ timeout: DEFAULT_TIMEOUT });

    // Form should be cleared
    await expect(settingsPage.getNewEmailInput()).toHaveValue('');
    await expect(settingsPage.getEmailPasswordInput()).toHaveValue('');
  });

  test('should show validation error for invalid email format', async ({ page }) => {
    // Login as Player 4
    await loginAs(page, 'PLAYER_4');

    const settingsPage = new SettingsPage(page);
    await settingsPage.goto();
    await settingsPage.clickAccountInformation();

    // Fill in with invalid email
    const newEmailInput = settingsPage.getNewEmailInput();
    await newEmailInput.fill('invalid-email');
    await settingsPage.getEmailPasswordInput().fill('testpassword123');
    await settingsPage.getChangeEmailSubmit().click();

    // HTML5 validation should prevent submission
    const isInvalid = await newEmailInput.evaluate((el: HTMLInputElement) => !el.validity.valid);
    expect(isInvalid).toBe(true);
  });

  test('should prevent username change with incorrect password', async ({ page }) => {
    // Login as GM
    await loginAs(page, 'GM');

    const settingsPage = new SettingsPage(page);
    await settingsPage.goto();
    await settingsPage.clickAccountInformation();

    // Fill in the form with wrong password
    await settingsPage.changeUsername('TestGMUpdated', 'wrongpassword');

    // Should show error (toast or alert)
    await expect(page.locator('text=/incorrect password|invalid password|authentication failed/i').first()).toBeVisible({ timeout: DEFAULT_TIMEOUT });
  });

  test('should prevent email change with incorrect password', async ({ page }) => {
    // Login as GM
    await loginAs(page, 'GM');

    const settingsPage = new SettingsPage(page);
    await settingsPage.goto();
    await settingsPage.clickAccountInformation();

    // Fill in the form with wrong password
    await settingsPage.requestEmailChange('gm_updated@example.com', 'wrongpassword');

    // Should show error (toast or alert)
    await expect(page.locator('text=/incorrect password|invalid password|authentication failed/i').first()).toBeVisible({ timeout: DEFAULT_TIMEOUT });
  });
});
