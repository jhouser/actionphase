import { Page, expect } from '@playwright/test';
import { TEST_USERS } from './test-users';
import { LoginPage } from '../pages/LoginPage';

/**
 * Authentication Helper Functions for E2E Tests
 */

/**
 * Get username for parallel test execution
 * Uses worker-specific usernames to prevent race conditions between parallel workers.
 * Each worker gets dedicated users and fixture data with isolated game IDs.
 *
 * @param baseUsername - Base username (e.g., 'TestGM', 'TestPlayer1')
 * @returns Worker-specific username (e.g., 'TestGM_1' for worker 1)
 */
function getWorkerSpecificUsername(baseUsername: string): string {
  // Get Playwright worker index from environment variable
  const workerIndex = process.env.TEST_PARALLEL_INDEX
    ? parseInt(process.env.TEST_PARALLEL_INDEX, 10)
    : 0;

  // Worker 0 uses base username (no suffix), others get _N suffix
  return workerIndex === 0 ? baseUsername : `${baseUsername}_${workerIndex}`;
}

/**
 * Login as a specific test user
 * @param page - Playwright page object
 * @param userKey - Key from TEST_USERS object (e.g., 'GM', 'PLAYER_1')
 * @returns Object with user info and token
 */
export async function loginAs(page: Page, userKey: keyof typeof TEST_USERS) {
  const user = TEST_USERS[userKey];

  // Get worker-specific username
  const workerUsername = getWorkerSpecificUsername(user.username);

  // Check if already logged in by checking for JWT cookie - if so, logout first
  const isLoggedIn = await isAuthenticated(page);
  if (isLoggedIn) {
    await logout(page);
    // After logout, we're already on /login page, so no need to navigate again
  }

  // Use LoginPage POM for login
  const loginPage = new LoginPage(page);

  // Always navigate to login page to ensure clean state
  // Even if we're already at /login (e.g., after logout), this ensures
  // the auth state has stabilized and network is idle before attempting login
  await loginPage.goto();

  await loginPage.login(workerUsername, user.password);

  // Authentication is now handled via HTTP-only cookies
  // No need to extract token from localStorage
  return { user, token: null };
}

/**
 * Login with custom credentials (for testing error cases)
 * @param page - Playwright page object
 * @param username - Custom username
 * @param password - Custom password
 * @param expectSuccess - Whether login should succeed (default: true)
 */
export async function login(
  page: Page,
  username: string,
  password: string,
  expectSuccess: boolean = true
) {
  const loginPage = new LoginPage(page);
  await loginPage.goto();
  await loginPage.login(username, password, expectSuccess);
}

/**
 * Logout the current user
 * Handles both mobile (hamburger menu) and desktop (hover user menu) navigation.
 * @param page - Playwright page object
 */
export async function logout(page: Page) {
  // Detect viewport: mobile shows hamburger (md:hidden), desktop shows user-menu-trigger (hidden md:block)
  const hamburger = page.locator('button[aria-label="Menu"]');
  const isMobile = await hamburger.isVisible({ timeout: 2000 }).catch(() => false);

  const logoutButton = page.locator('button:has-text("Logout")').locator('visible=true').first();

  if (isMobile) {
    // Mobile: click hamburger to open the mobile drawer
    await hamburger.click();
    await logoutButton.waitFor({ state: 'visible', timeout: 5000 });
  } else {
    // Desktop: open the user menu by clicking its button.
    //
    // The menu also opens on hover, but hover is a poor fit for a test helper:
    // it depends on where the pointer already is (re-hovering without leaving
    // is a no-op, so a retry can't recover), and under parallel load the first
    // hover can land before React has attached onMouseEnter. Clicking is a
    // discrete, retriable action with no such dependency.
    const userMenuTrigger = page.getByTestId('user-menu-trigger');
    const userMenuButton = userMenuTrigger.locator('button').first();
    await expect(userMenuButton).toBeVisible({ timeout: 10000 });

    await expect(async () => {
      await userMenuButton.click();
      await logoutButton.waitFor({ state: 'visible', timeout: 2000 });
    }).toPass({ timeout: 20000, intervals: [500] });
  }

  // Use Promise.all to handle the logout click and response concurrently
  // This ensures we catch the response even if navigation happens immediately
  await Promise.all([
    page.waitForResponse(
      response => response.url().includes('/api/v1/auth/logout') && response.status() === 200
    ),
    logoutButton.click(),
  ]);

  // Wait for redirect to login page
  await page.waitForURL('/login', { timeout: 5000 });

  // Clear the JWT cookie for clean state
  const cookies = await page.context().cookies();
  const jwtCookie = cookies.find(cookie => cookie.name === 'jwt');
  if (jwtCookie) {
    await page.context().clearCookies({ name: 'jwt' });
  }
}

/**
 * Check if user is authenticated by verifying presence of JWT cookie
 * @param page - Playwright page object
 */
export async function isAuthenticated(page: Page): Promise<boolean> {
  // Check for the JWT cookie (HTTP-only cookie named 'jwt')
  const cookies = await page.context().cookies();
  const jwtCookie = cookies.find(cookie => cookie.name === 'jwt');

  // User is authenticated if JWT cookie exists and is not expired
  if (jwtCookie) {
    // Check if cookie is expired (expires is in seconds since epoch)
    const now = Date.now() / 1000; // Convert to seconds
    if (jwtCookie.expires === -1 || jwtCookie.expires > now) {
      return true;
    }
  }

  return false;
}

/**
 * Get the current user's token from localStorage
 * @deprecated Authentication now uses HTTP-only cookies. This function always returns null.
 */
export async function getAuthToken(): Promise<string | null> {
  // Authentication is now cookie-based, no token in localStorage
  return null;
}

/**
 * Clear authentication state (logout without UI interaction)
 * @deprecated Use the logout() function instead. HTTP-only cookies cannot be cleared from JavaScript.
 */
export async function clearAuth() {
  // HTTP-only cookies cannot be cleared from JavaScript
  // Use the logout() function instead for proper logout
  // This function is kept for backwards compatibility but does nothing
}
