# E2E Testing Guide

**Complete guide to writing, organizing, and running end-to-end tests for ActionPhase.**

## Table of Contents

- [Quick Start](#quick-start)
- [Test Organization](#test-organization)
- [Writing Tests](#writing-tests)
- [Test Data](#test-data)
- [Page Objects](#page-objects)
- [Running Tests](#running-tests)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)

---

## Quick Start

### Prerequisites

```bash
# Install dependencies
npm install

# Install Playwright browsers
npx playwright install chromium

# Apply test fixtures (backend must be running)
cd backend/pkg/db/test_fixtures
./apply_all.sh
```

### Run Tests

```bash
# Run all E2E tests
npm run test:e2e

# Run smoke tests only (quick validation)
npm run test:e2e:smoke

# Run critical tests only (deployment blockers)
npm run test:e2e:critical

# Run specific tag
npm run test:e2e:auth

# Run with UI (debugging)
npm run test:e2e:ui

# Run in headed mode (see browser)
npm run test:e2e:headed
```

---

## Test Organization

### Directory Structure

```
e2e/
├── fixtures/           # Test helpers and utilities
│   ├── auth-helpers.ts      # Login/auth helpers
│   ├── test-tags.ts         # Test categorization
│   └── test-data-factory.ts # Test data generation
│
├── pages/             # Page Object Model
│   ├── CharacterSheetPage.ts
│   ├── RegistrationPage.ts
│   ├── UserSettingsPage.ts
│   ├── AdminDashboardPage.ts
│   └── GameHandoutsPage.ts
│
├── smoke/             # Quick health checks (<5 min)
│   └── health-check.spec.ts
│
├── journeys/          # User journey tests
│   ├── critical/      # Must-pass for deployment
│   │   ├── game-lifecycle.spec.ts
│   │   ├── user-onboarding.spec.ts
│   │   └── multi-user-collaboration.spec.ts
│   │
│   └── standard/      # Important flows (not deployment-blocking)
│       ├── phase-management.spec.ts
│       └── action-submission.spec.ts
│
└── regression/        # Bug prevention tests (add as needed)
```

### Test Categories

**Smoke Tests** (`@smoke`):
- **Purpose**: Quick validation before deployment
- **Target**: < 5 minutes execution
- **Examples**: Health checks, page loads, basic navigation

**Critical Tests** (`@critical`):
- **Purpose**: Must pass for deployment
- **Examples**: User registration, game creation, core workflows
- **Failures block release**

**Standard Tests**:
- **Purpose**: Important user journeys
- **Examples**: Phase management, action submission, character creation
- **Failures require investigation but don't block release**

**Regression Tests**:
- **Purpose**: Prevent bugs from returning
- **Add when**: A bug is fixed
- **Examples**: Specific edge cases, past failures

---

## Writing Tests

### Basic Test Structure

```typescript
import { test, expect } from '@playwright/test';
import { tagTest, tags } from '../fixtures/test-tags';
import { loginAs } from '../fixtures/auth-helpers';

test.describe('Feature Name', () => {
  test(tagTest([tags.SMOKE, tags.AUTH], 'Test description'), async ({ page }) => {
    // Arrange: Setup test data and navigate
    await loginAs(page, 'GM');
    await page.goto('/games');

    // Act: Perform user actions
    await page.click('[data-testid="create-game"]');

    // Assert: Verify expected outcomes
    await expect(page).toHaveURL(/\\/games\\/\\d+/);
  });
});
```

### Using Test Tags

```typescript
// Single tag
test(tagTest([tags.SMOKE], 'Test name'), async ({ page }) => { });

// Multiple tags
test(tagTest([tags.CRITICAL, tags.GAME, tags.E2E], 'Test name'), async ({ page }) => { });

// Available tags
tags.SMOKE       // Quick health checks
tags.CRITICAL    // Must-pass for deployment
tags.AUTH        // Authentication tests
tags.GAME        // Game management
tags.CHARACTER   // Character system
tags.MESSAGE     // Messaging/communication
tags.PHASE       // Phase management
tags.SLOW        // Tests > 30s
tags.FLAKY       // Known flaky tests
tags.INTEGRATION // Integration tests
tags.E2E         // End-to-end flows
```

### Multi-User Testing

```typescript
test('Multiple users interact', async ({ browser }) => {
  // Create separate contexts for each user
  const gmContext = await browser.newContext();
  const playerContext = await browser.newContext();

  const gmPage = await gmContext.newPage();
  const playerPage = await playerContext.newPage();

  try {
    // Login as different users
    await loginAs(gmPage, 'GM');
    await loginAs(playerPage, 'PLAYER_1');

    // Both users interact with same game
    await gmPage.goto('/games/123');
    await playerPage.goto('/games/123');

    // Verify both see the same content
    const gmTitle = await gmPage.locator('h1').textContent();
    const playerTitle = await playerPage.locator('h1').textContent();
    expect(gmTitle).toBe(playerTitle);

  } finally {
    // Always cleanup contexts
    await gmContext.close();
    await playerContext.close();
  }
});
```

---

## Test Data

### Using Test Users

```typescript
import { TEST_USERS } from '../fixtures/test-data-factory';

// Login as predefined test user
await loginAs(page, 'GM');
await loginAs(page, 'PLAYER_1');

// Access user credentials
const { username, email, password } = TEST_USERS.GM;
```

**Available Test Users**:
- `GM` - Game Master
- `PLAYER_1` through `PLAYER_5` - Players
- `AUDIENCE` - Observer role

**All passwords**: `testpassword123`

### Using Fixture Games

```typescript
import { FIXTURE_GAMES } from '../fixtures/test-data-factory';

// Navigate to a game with specific state
const gameTitle = FIXTURE_GAMES.ACTION_PHASE.title;
await page.goto('/games');
await page.locator(`text=${gameTitle}`).first().click();

// Verify game properties
expect(FIXTURE_GAMES.ACTION_PHASE.expectedState).toBe('in_progress');
expect(FIXTURE_GAMES.ACTION_PHASE.hasActionSubmissions).toBe(true);
```

**Available Fixture Games**:
- `COMMON_ROOM` - Active common room phase
- `ACTION_PHASE` - Active action phase with submissions
- `RESULTS_PHASE` - Active results phase
- `PHASE_TRANSITION` - For testing transitions
- `COMPLEX_HISTORY` - 6 previous phases
- `PAGINATION` - 11 previous phases
- `RECRUITING` - Recruitment state
- `PAUSED` - Paused game
- `COMPLETED` - Completed game
- `PRIVATE` - Private game

### Generating New Test Data

```typescript
import {
  generateTestUser,
  generateTestGame,
  generateTestCharacter,
  generateTestPost
} from '../fixtures/test-data-factory';

// Generate unique user
const newUser = generateTestUser('my_test');
// => { username: 'my_test_1234567890_123', email: '...', password: '...' }

// Generate game with overrides
const gameData = generateTestGame({
  title: 'My Specific Test Game',
  max_players: 6
});

// Generate character
const charData = generateTestCharacter(gameId, {
  name: 'Test Hero',
  character_type: 'player_character'
});
```

---

## Page Objects

### What are Page Objects?

Page Objects encapsulate page interactions into reusable, maintainable classes.

### Using Existing Page Objects

```typescript
import { RegistrationPage } from '../pages/RegistrationPage';

test('User registration', async ({ page }) => {
  const registrationPage = new RegistrationPage(page);

  await registrationPage.goto();
  await registrationPage.register('user@example.com', 'username', 'password');

  // Page object handles all the selectors and interactions
});
```

### Creating a Page Object

```typescript
import { Page, Locator } from '@playwright/test';

export class MyFeaturePage {
  readonly page: Page;

  // Define locators as class properties
  readonly submitButton: Locator;
  readonly titleInput: Locator;

  constructor(page: Page) {
    this.page = page;

    // Define selectors once
    this.submitButton = page.locator('[data-testid="submit"]');
    this.titleInput = page.locator('[data-testid="title"]');
  }

  // Create methods for common actions
  async goto() {
    await this.page.goto('/feature');
    await this.page.waitForLoadState('networkidle');
  }

  async fillForm(title: string) {
    await this.titleInput.fill(title);
  }

  async submit() {
    await this.submitButton.click();
  }
}
```

---

## Running Tests

### Local Development

```bash
# Run all tests
npm run test:e2e

# Run specific file
npx playwright test e2e/smoke/health-check.spec.ts

# Run with UI for debugging
npm run test:e2e:ui

# Run in headed mode (see browser)
npm run test:e2e:headed

# Run in debug mode with pause
npm run test:e2e:debug
```

### Filtering Tests

```bash
# By tag
npm run test:e2e:smoke
npm run test:e2e:critical
npm run test:e2e:auth
npm run test:e2e:game

# By name pattern
npx playwright test --grep "user registration"

# Exclude tests
npx playwright test --grep-invert "@slow"
```

### Viewing Results

```bash
# Show test report
npm run test:e2e:report

# Test output includes:
# - Console logs from tests
# - Screenshots on failure
# - Video recordings (if enabled)
# - Trace files for debugging
```

---

## Best Practices

### 1. Use data-testid Selectors

✅ **Good**:
```typescript
await page.click('[data-testid="create-game"]');
```

❌ **Avoid**:
```typescript
await page.click('.btn.btn-primary.create-button'); // Fragile, breaks with CSS changes
```

### 2. Wait for Network Idle

```typescript
// After navigation
await page.goto('/games');
await page.waitForLoadState('networkidle');

// After actions that trigger network requests
await page.click('[data-testid="submit"]');
await page.waitForLoadState('networkidle');
```

### 3. Handle Visibility Gracefully

```typescript
// Check if element exists before interacting
if (await button.isVisible()) {
  await button.click();
} else {
  console.log('⚠ Button not found - may be optional');
}

// Use .catch() for conditional checks
const hasError = await page.locator('[data-testid="error"]').isVisible().catch(() => false);
```

### 4. Isolate Tests

```typescript
// ✅ Each test is independent
test('Test 1', async ({ page }) => {
  await loginAs(page, 'GM');
  // ... test actions
});

test('Test 2', async ({ page }) => {
  await loginAs(page, 'GM'); // Login again, don't rely on Test 1
  // ... test actions
});
```

### 5. Clean Up Resources

```typescript
// Always cleanup browser contexts in multi-user tests
try {
  // Test code
} finally {
  await gmContext.close();
  await playerContext.close();
}
```

### 6. Use Meaningful Console Logs

```typescript
console.log('✓ User logged in successfully');
console.log('⚠ Optional feature not found - skipping');
console.log(`Found ${count} games in listing`);
```

### 7. Test One Concern Per Test

```typescript
// ✅ Good: Tests specific functionality
test('User can register', async ({ page }) => {
  // Only test registration
});

test('User can login', async ({ page }) => {
  // Only test login
});

// ❌ Avoid: Tests too many things
test('Complete user workflow', async ({ page }) => {
  // Register + Login + Create Game + Invite Player + Start Game
  // If this fails, where did it break?
});
```

### 8. Don't Modify Shared Test Data

```typescript
// ❌ Avoid: Modifying fixture games
await page.goto(`/games/${FIXTURE_GAMES.ACTION_PHASE.id}`);
await page.click('[data-testid="delete-game"]'); // Breaks other tests!

// ✅ Good: Create test-specific data
const gameData = generateTestGame();
// Create via API, then test with it
```

### 9. Never Touch the Database Directly From a Spec

Specs run **inside the Playwright container**, which has no `psql`, no DB driver,
and no network route to the database host. Shelling out to the DB (`execSync`,
`child_process`, raw `psql`) will crash there — and even on a host it is a
fragile antipattern (raw SQL, string-interpolated values, ordering hazards).

```typescript
// ❌ Avoid: reaching into the DB from a test to set up or reset state
execSync(`psql postgres://.../actionphase -c "UPDATE users SET ..."`);

// ✅ Good: keep state out of the DB in the first place — intercept the write
await page.route('**/api/v1/me/change-username', route =>
  route.fulfill({ status: 200, body: JSON.stringify({ message: 'ok' }) })
);
// The API never fires, so the shared fixture user is never mutated and there
// is nothing to reset. The backend handler tests own DB-mutation assertions.
```

- **Fixture / DB setup belongs in `global-setup.ts`**, which is guarded by
  `E2E_SKIP_FIXTURE_SETUP` and defers to `just load-e2e` (that runs `psql`
  inside the *backend* container) in the containerized stack.
- **Per-test isolation** should use network interception (`page.route`) or
  test-specific data created via the API — not DB writes.
- If a test seems to *need* a DB reset, that usually means it is mutating shared
  state it shouldn't (see #8). Fix the mutation, not the cleanup.

---

## Troubleshooting

### Test Failures

**Problem**: Test fails intermittently
- **Solution**: Add `waitForLoadState('networkidle')` after navigation and actions
- **Solution**: Increase timeout: `{ timeout: 10000 }`
- **Solution**: Check for race conditions with `waitFor()`

**Problem**: Element not found
- **Solution**: Verify selector with `npx playwright codegen http://localhost:5173`
- **Solution**: Check if element is in correct tab/modal
- **Solution**: Use `.isVisible()` to check existence first

**Problem**: Authentication not working
- **Solution**: Verify test fixtures are loaded: `./backend/pkg/db/test_fixtures/apply_all.sh`
- **Solution**: Check test user credentials in `TEST_USERS`
- **Solution**: Ensure backend is running on correct port

### Debugging

```bash
# Run with Playwright Inspector
npm run test:e2e:debug

# Generate test code from recording
npx playwright codegen http://localhost:5173

# View trace after test failure
npx playwright show-trace trace.zip

# Run single test with headed mode
npx playwright test e2e/smoke/health-check.spec.ts --headed
```

### Common Issues

**"Page not found" errors**:
- Verify backend is running: `just dev`
- Verify frontend is running: `npm run dev`
- Check correct ports (backend: 3000, frontend: 5173)

**"User not found" errors**:
- Apply test fixtures: `./backend/pkg/db/test_fixtures/apply_all.sh`
- Verify database: `psql -U postgres -d actionphase`

**Slow tests**:
- Use `@slow` tag for tests >30s
- Run slow tests separately: `npx playwright test --grep "@slow"`
- Consider splitting into smaller tests

---

## Visual Regression Testing

### Overview

Visual regression tests capture screenshots of pages and components to detect unintended visual changes. We use Playwright's built-in screenshot functionality.

### Running Visual Tests

```bash
# Run all visual regression tests
npm run test:e2e -- visual/

# Run critical pages only
npm run test:e2e -- visual/critical-pages.spec.ts

# Run in light mode only
npm run test:e2e -- visual/ --grep "Light Mode"

# Run in dark mode only
npm run test:e2e -- visual/ --grep "Dark Mode"
```

### Updating Baselines

After intentional UI changes, update the baseline screenshots:

```bash
# Update all baselines
npm run test:e2e -- visual/ --update-snapshots

# Update specific test
npm run test:e2e -- visual/critical-pages.spec.ts --update-snapshots
```

### Writing Visual Tests

```typescript
test('Page visual snapshot', async ({ page }) => {
  // Set color scheme
  await page.emulateMedia({ colorScheme: 'light' });

  await page.goto('/my-page');
  await page.waitForLoadState('networkidle');

  // Take screenshot
  await expect(page).toHaveScreenshot('my-page-light.png', {
    fullPage: true,
    maxDiffPixels: 100,
    mask: [
      // Mask dynamic content
      page.locator('text=/\\d+[mhd] ago/'),
    ],
  });
});
```

### Best Practices

1. **Test both light and dark mode** for all critical pages
2. **Mask dynamic content** (timestamps, random IDs, animations)
3. **Set appropriate diff thresholds** (100-150px for dynamic pages)
4. **Wait for network idle** before capturing screenshots
5. **Use full-page screenshots** for complete coverage
6. **Group by color scheme** for better organization

### Current Coverage

- ✅ Home page (light + dark)
- ✅ Login page (light + dark)
- ✅ Registration page (light + dark)
- ✅ Dashboard (light + dark)
- ✅ Game details page (light + dark)
- ✅ Settings page (light + dark)

See `e2e/visual/` for all visual regression tests.

---

## Additional Resources

- **Playwright Docs**: https://playwright.dev/
- **Playwright Screenshots**: https://playwright.dev/docs/test-snapshots
- **Test Data Reference**: `.claude/context/TEST_DATA.md`
- **E2E Implementation Plan**: `.claude/planning/E2E_TESTING_PLAN.md`
- **Testing Philosophy**: `.claude/context/TESTING.md`
