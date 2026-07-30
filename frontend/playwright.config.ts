import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright E2E Test Configuration
 *
 * See https://playwright.dev/docs/test-configuration.
 */

// App origin the tests drive. Host runs default to localhost:5173; the
// containerized Playwright service sets E2E_BASE_URL=http://frontend:5173.
const baseURL = process.env.E2E_BASE_URL ?? 'http://localhost:5173';

// When the app is already running (containerized stack, or CI), skip Playwright's
// managed webServer. E2E_NO_WEBSERVER=true is set by the Playwright container.
const manageWebServer = process.env.E2E_NO_WEBSERVER !== 'true';

export default defineConfig({
  testDir: './e2e',

  /* Global setup - runs once before all tests to reset fixtures */
  globalSetup: './e2e/global-setup.ts',

  /* Run test files in parallel - tests are isolated with dedicated fixture games */
  fullyParallel: true,

  /* Fail the build on CI if you accidentally left test.only in the source code. */
  forbidOnly: !!process.env.CI,

  /* Retry on CI only */
  retries: process.env.CI ? 2 : 0,

  /* Use 6 workers for parallel execution with worker-specific test users */
  workers: 6,

  /* Reporter to use. See https://playwright.dev/docs/test-reporters */
  reporter: [
    ['html'],
    ['junit', { outputFile: 'test-results/junit.xml' }],
    ['list']
  ],

  /* Shared settings for all the projects below. See https://playwright.dev/docs/api/class-testoptions. */
  use: {
    /* Base URL to use in actions like `await page.goto('/')`. */
    baseURL,

    /* Collect trace when retrying the failed test. See https://playwright.dev/docs/trace-viewer */
    trace: 'on-first-retry',

    /* Capture screenshot on failure */
    screenshot: 'only-on-failure',

    /* Capture video on failure */
    video: 'retain-on-failure',
  },

  /* Configure projects for major browsers */
  projects: [
    {
      name: 'smoke',
      use: { ...devices['Desktop Chrome'] },
      grep: /@smoke/,
    },
    {
      name: 'mobile-chrome',
      use: { ...devices['Pixel 5'] },
      grep: /@mobile/,
      dependencies: ['smoke'],
    },
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
      grepInvert: /@smoke/,
      dependencies: ['smoke'],
    },
    // {
    //   name: 'firefox',
    //   use: { ...devices['Desktop Firefox'] },
    // },
    // {
    //   name: 'webkit',
    //   use: { ...devices['Desktop Safari'] },
    // },
  ],

  /* Run your local dev server before starting the tests — unless the app is
     already up (containerized stack / CI), in which case we just target it. */
  webServer: manageWebServer
    ? {
        command: 'npm run dev',
        url: baseURL,
        reuseExistingServer: !process.env.CI,
        timeout: 120000,
      }
    : undefined,
});
