import { Page, Locator } from '@playwright/test';
import { LONG_TIMEOUT } from '../config/test-timeouts';

/**
 * Page Object for User Login
 *
 * Handles user authentication and login flows
 */
export class LoginPage {
  readonly page: Page;

  // Locators
  readonly usernameInput: Locator;
  readonly passwordInput: Locator;
  readonly loginButton: Locator;
  readonly signUpButton: Locator;
  readonly errorMessage: Locator;

  constructor(page: Page) {
    this.page = page;

    // Define locators
    this.usernameInput = page.locator('[data-testid="login-username"]');
    this.passwordInput = page.locator('[data-testid="login-password"]');
    this.loginButton = page.locator('[data-testid="login-submit"]');
    this.signUpButton = page.locator('button:has-text("Don\'t have an account? Sign up")');
    this.errorMessage = page.locator('[data-testid="error-message"]');
  }

  /**
   * Navigate to login page
   */
  async goto() {
    // Navigate with retry logic to handle race conditions during user switching
    let retries = 3;
    while (retries > 0) {
      try {
        await this.page.goto('/login', { waitUntil: 'domcontentloaded' });
        break;
      } catch (error) {
        const isNavigationError = error.message.includes('interrupted by another navigation') ||
                                 error.message.includes('ERR_ABORTED');
        if (isNavigationError && retries > 1) {
          // Wait a bit and retry
          await this.page.waitForTimeout(300);
          retries--;
        } else {
          throw error;
        }
      }
    }
    // Wait for the login form to be visible and network to settle
    // (ensures any in-flight auth requests from prior tests have completed)
    //
    // LONG_TIMEOUT, not the 5s default. Login is the first thing all ~51 specs
    // do, so this wait competes with five other workers cold-loading the Vite
    // dev server at once. A budget that only suits a quiet machine turns dev
    // server contention into "login timed out" failures scattered across
    // unrelated specs -- the symptom points at whatever the spec was testing
    // rather than at the shared bottleneck it actually hit.
    await this.usernameInput.waitFor({ state: 'visible', timeout: LONG_TIMEOUT });
    await this.page.waitForLoadState('networkidle');
  }

  /**
   * Login with credentials
   *
   * @param username - Username
   * @param password - Password
   * @param expectSuccess - Whether login should succeed (default: true)
   * @returns Promise that resolves when login completes
   */
  async login(username: string, password: string, expectSuccess: boolean = true) {
    await this.usernameInput.fill(username);
    await this.passwordInput.fill(password);
    await this.loginButton.click();

    if (expectSuccess) {
      // Wait to land ANYWHERE but /login -- not specifically on /dashboard.
      //
      // /dashboard is only the FALLBACK destination. LoginPage.tsx redirects to
      // `location.state.from.pathname` when it is set, which ProtectedRoute
      // sets whenever it bounces an unauthenticated user off a protected URL.
      // React Router keeps that state in SPA history across a page.goto('/login'),
      // so a spec that deep-links to a protected route and then logs in as
      // someone else legitimately lands back on THAT route instead.
      //
      // Waiting for '/dashboard' therefore timed out on a login that had in fact
      // succeeded -- and it read as a slow dev server, because the URL bar was
      // the only place the difference showed. Leaving /login is the real
      // success signal; where we land afterwards is the app's business.
      //
      // LONG_TIMEOUT, not the 10s default: the POST and the subsequent
      // /auth/me still queue behind other workers on a busy run.
      await this.page.waitForURL((url) => !url.pathname.startsWith('/login'), {
        timeout: LONG_TIMEOUT,
      });
      await this.page.waitForLoadState('networkidle');
    }
  }

  /**
   * Attempt login with invalid credentials (for testing validation)
   */
  async loginInvalid(username: string, password: string) {
    await this.usernameInput.fill(username);
    await this.passwordInput.fill(password);
    await this.loginButton.click();
  }

  /**
   * Get error message text
   */
  async getErrorMessage(): Promise<string> {
    return await this.errorMessage.textContent() || '';
  }

  /**
   * Check if login button is disabled
   */
  async isLoginButtonDisabled(): Promise<boolean> {
    return await this.loginButton.isDisabled();
  }

  /**
   * Navigate to registration page
   */
  async goToSignUp() {
    await this.signUpButton.click();
    await this.page.waitForTimeout(500); // Wait for form to toggle
  }

  /**
   * Fill login form but don't submit
   */
  async fillForm(username: string, password: string) {
    await this.usernameInput.fill(username);
    await this.passwordInput.fill(password);
  }

  /**
   * Check if user is logged in (authenticated)
   */
  async isLoggedIn(): Promise<boolean> {
    try {
      // Check for authenticated navbar (Dashboard or Games link)
      await this.page.waitForSelector('nav a[href="/dashboard"]', { timeout: 2000, state: 'attached' });
      return true;
    } catch {
      return false;
    }
  }
}
