import { test, expect, Page } from '@playwright/test';
import { loginAs } from '../fixtures/auth-helpers';
import { getFixtureGameId } from '../fixtures/game-helpers';
import { GameDetailsPage } from '../pages/GameDetailsPage';

/**
 * E2E Tests for Deadline Management
 *
 * Tests the complete deadline lifecycle:
 * - GM can create deadlines
 * - GM can edit deadlines
 * - GM can extend deadlines
 * - GM can delete deadlines
 * - Players can view deadlines (read-only)
 * - Timer color coding works correctly
 *
 * Uses E2E_ACTION fixture game for testing (state-modifying tests allowed)
 */

/**
 * Helper function to set a date/time using the DateTimeInput picker
 * The DateTimeInput component uses react-datepicker which requires clicking on
 * calendar dates and selecting times from a dropdown, not using .fill()
 */
async function setDeadlineDateTime(page: Page, futureDate: Date) {
  // Click the deadline textbox to open the date/time picker
  await page.getByRole('textbox', { name: 'Deadline' }).click();

  // Wait for the date picker dialog to appear
  await expect(page.getByRole('dialog', { name: 'Choose Date and Time' })).toBeVisible();

  const targetMonth = futureDate.getMonth(); // 0-indexed
  const targetYear = futureDate.getFullYear();

  // Navigate to the correct month — react-datepicker may be on any month
  // Read the currently displayed month/year from the calendar header and navigate forward/back
  for (let attempts = 0; attempts < 24; attempts++) {
    const header = page.locator('.react-datepicker__current-month');
    const headerText = await header.textContent();
    if (!headerText) break;

    // Parse "March 2026" style header
    const displayedDate = new Date(`${headerText} 1`);
    const displayedMonth = displayedDate.getMonth();
    const displayedYear = displayedDate.getFullYear();

    if (displayedYear === targetYear && displayedMonth === targetMonth) break;

    const isAhead = displayedYear > targetYear || (displayedYear === targetYear && displayedMonth > targetMonth);
    if (isAhead) {
      await page.getByRole('button', { name: 'Previous Month' }).click();
    } else {
      await page.getByRole('button', { name: 'Next Month' }).click();
    }
  }

  // Click the correct date cell
  const dayOfMonth = futureDate.getDate();
  const monthName = futureDate.toLocaleString('en-US', { month: 'long' });
  const year = futureDate.getFullYear();
  const ordinalDay = getOrdinalSuffix(dayOfMonth);

  await page.getByRole('gridcell', { name: new RegExp(`Choose.*${monthName} ${ordinalDay}, ${year}`, 'i') }).click();

  // Select the time from the dropdown
  const hours = String(futureDate.getHours()).padStart(2, '0');
  const minutes = String(futureDate.getMinutes()).padStart(2, '0');
  const timeString = `${hours}:${minutes}`;

  await page.getByRole('option', { name: timeString }).click();

  // The picker should close automatically after selecting time
  await expect(page.getByRole('dialog', { name: 'Choose Date and Time' })).not.toBeVisible();
}

/**
 * Reveal every deadline in the widget.
 *
 * DeadlineStrip shows only the 3 soonest active deadlines and hides the rest
 * behind a "View All" toggle. Tests in this file share one fixture game and each
 * add a deadline, so a deadline created far in the future sorts past position 3
 * once a few have accumulated and is genuinely absent from the DOM. Expanding
 * first makes assertions independent of how many deadlines already exist.
 *
 * The toggle only renders when there is something to expand, so its absence is
 * fine — that means everything is already visible.
 */
async function expandAllDeadlines(page: Page) {
  const viewAllButton = page.getByRole('button', { name: /View All \(\d+\)/ }).locator('visible=true').first();
  if (await viewAllButton.isVisible({ timeout: 2000 }).catch(() => false)) {
    await viewAllButton.click();
  }
}

/**
 * Helper to get ordinal suffix for date (1st, 2nd, 3rd, 4th, etc.)
 */
function getOrdinalSuffix(day: number): string {
  if (day > 3 && day < 21) return `${day}th`;
  switch (day % 10) {
    case 1: return `${day}st`;
    case 2: return `${day}nd`;
    case 3: return `${day}rd`;
    default: return `${day}th`;
  }
}

test.describe('Deadline Management', () => {
  test.beforeEach(async ({ page }) => {
    // Login as GM
    await loginAs(page, 'GM');
  });

  test('GM can create a new deadline', async ({ page }) => {
    // Get game ID for E2E_ACTION fixture
    const gameId = await getFixtureGameId(page, 'E2E_ACTION');
    const gameDetailsPage = new GameDetailsPage(page);
    await gameDetailsPage.goto(gameId);

    // Verify Deadlines widget header exists
    await expect(page.getByText('Deadlines').first()).toBeVisible();

    // Click "Add Deadline" button
    await page.getByRole('button', { name: /Add Deadline/i }).click();

    // Fill in deadline form
    await page.getByLabel('Title').fill('Test Deadline');
    await page.getByRole('textbox', { name: 'Description' }).fill('This is a test deadline for E2E');

    // Set deadline to 48 hours from now (future, >24h = blue)
    // Round to nearest 15 minutes since the time picker uses 15-minute intervals
    const futureDate = new Date(Date.now() + 48 * 60 * 60 * 1000);
    futureDate.setMinutes(Math.ceil(futureDate.getMinutes() / 15) * 15);
    futureDate.setSeconds(0);
    futureDate.setMilliseconds(0);

    await setDeadlineDateTime(page, futureDate);

    // Submit form
    await page.getByRole('button', { name: /Create Deadline/i }).click();

    // Verify deadline appears in widget
    await expect(page.getByText('Test Deadline')).toBeVisible();
    // Note: Description is now in tooltip (info icon), not directly visible

    // Verify countdown timer is displayed (should show something like "2d 0h" for 48 hours)
    await expect(page.getByText(/\d+d \d+h/)).toBeVisible();
  });

  test('GM can edit an existing deadline', async ({ page }) => {
    // Get game ID
    const gameId = await getFixtureGameId(page, 'E2E_ACTION');
    const gameDetailsPage = new GameDetailsPage(page);
    await gameDetailsPage.goto(gameId);

    // Create a deadline first
    await page.getByRole('button', { name: /Add Deadline/i }).click();
    await page.getByLabel('Title').fill('Original Title');
    await page.getByRole('textbox', { name: 'Description' }).fill('Original description');

    const futureDate = new Date(Date.now() + 48 * 60 * 60 * 1000);
    futureDate.setMinutes(Math.ceil(futureDate.getMinutes() / 15) * 15);
    futureDate.setSeconds(0);
    futureDate.setMilliseconds(0);

    await setDeadlineDateTime(page, futureDate);
    await page.getByRole('button', { name: /Create Deadline/i }).click();

    // Wait for deadline to appear
    await expect(page.getByText('Original Title')).toBeVisible();

    // Hover over the deadline card to reveal action buttons
    const deadlineCard = page.locator('[class*="rounded-lg border-2"]').filter({ hasText: 'Original Title' }).first();
    await deadlineCard.hover();

    // Click edit button (pencil icon) - visible on hover
    await page.getByRole('button', { name: /Edit deadline/i }).first().click();

    // Update the deadline
    await page.getByLabel('Title').fill('Updated Title');
    await page.getByRole('textbox', { name: 'Description' }).fill('Updated description');

    // Submit changes
    await page.getByRole('button', { name: /Save Changes/i }).click();

    // Wait for modal to close
    await expect(page.getByRole('heading', { name: 'Edit Deadline' })).not.toBeVisible();

    // Verify changes appear in the deadline list
    // NOTE: DeadlineCard only shows title/countdown/date, not description
    await expect(page.getByText('Updated Title')).toBeVisible();
    await expect(page.getByText('Original Title')).not.toBeVisible();
  });

  test('GM can extend a deadline', async ({ page }) => {
    // Get game ID
    const gameId = await getFixtureGameId(page, 'E2E_ACTION');
    const gameDetailsPage = new GameDetailsPage(page);
    await gameDetailsPage.goto(gameId);

    // Create a deadline with a near-term date (12 hours = yellow warning)
    await page.getByRole('button', { name: /Add Deadline/i }).click();
    await page.getByLabel('Title').fill('Soon Deadline');
    await page.getByRole('textbox', { name: 'Description' }).fill('This deadline is coming soon');

    const soonDate = new Date(Date.now() + 12 * 60 * 60 * 1000); // 12 hours
    soonDate.setMinutes(Math.round(soonDate.getMinutes() / 15) * 15);
    soonDate.setSeconds(0);
    soonDate.setMilliseconds(0);

    await setDeadlineDateTime(page, soonDate);
    await page.getByRole('button', { name: /Create Deadline/i }).click();

    const deadlineCard = page.locator('[class*="rounded-lg border-2"]').filter({ hasText: 'Soon Deadline' }).first();
    await expect(deadlineCard).toBeVisible();

    // Hover to reveal action buttons and click extend
    await deadlineCard.hover();
    await page.getByRole('button', { name: /Extend deadline/i }).first().click();

    // Select 24 hours extension
    await page.getByRole('button', { name: /24 hours/i }).click();

    // Confirm extension (use exact match to avoid ambiguity with the icon button)
    await page.getByRole('button', { name: 'Extend Deadline', exact: true }).click();

    // Deadline should still be visible. After extending a 12h deadline by 24h it becomes
    // ~36h — the countdown should now show days ("1d Xh") rather than hours only.
    await expect(page.getByText('Soon Deadline')).toBeVisible();
    await expect(deadlineCard.getByText(/1d \d+h/)).toBeVisible({ timeout: 5000 });
  });

  test('GM can delete a deadline', async ({ page }) => {
    // Get game ID
    const gameId = await getFixtureGameId(page, 'E2E_ACTION');
    const gameDetailsPage = new GameDetailsPage(page);
    await gameDetailsPage.goto(gameId);

    // Create a deadline
    await page.getByRole('button', { name: /Add Deadline/i }).click();
    await page.getByLabel('Title').fill('Deadline to Delete');
    await page.getByRole('textbox', { name: 'Description' }).fill('This deadline will be deleted');

    const futureDate = new Date(Date.now() + 48 * 60 * 60 * 1000);
    futureDate.setMinutes(Math.ceil(futureDate.getMinutes() / 15) * 15);
    futureDate.setSeconds(0);
    futureDate.setMilliseconds(0);

    await setDeadlineDateTime(page, futureDate);
    await page.getByRole('button', { name: /Create Deadline/i }).click();

    await expect(page.getByText('Deadline to Delete')).toBeVisible();

    // Hover over the deadline card to reveal action buttons
    const deadlineCard = page.locator('[class*="rounded-lg border-2"]').filter({ hasText: 'Deadline to Delete' }).first();
    await deadlineCard.hover();

    // Click delete button (trash icon) - visible on hover
    await page.getByRole('button', { name: /Delete deadline/i }).first().click();

    // Confirm deletion in modal
    const deleteModal = page.getByRole('dialog', { name: 'Delete Deadline' });
    await expect(deleteModal.getByText(/Are you sure you want to delete/i)).toBeVisible();
    await deleteModal.getByRole('button', { name: 'Delete' }).click();

    // Wait for modal to close
    await expect(page.getByRole('heading', { name: 'Delete Deadline' })).not.toBeVisible();

    // The deadline should no longer be visible in the deadline list
    const deletedDeadlineCard = page.locator('[class*="rounded-lg border-2"]').filter({ hasText: 'Deadline to Delete' });
    await expect(deletedDeadlineCard).not.toBeVisible({ timeout: 15000 });
  });

  test('Player can view deadlines but cannot edit them', async ({ page }) => {
    // First, as GM, create a deadline
    const gameId = await getFixtureGameId(page, 'E2E_ACTION');
    const gameDetailsPage = new GameDetailsPage(page);
    await gameDetailsPage.goto(gameId);

    await page.getByRole('button', { name: /Add Deadline/i }).click();
    await page.getByLabel('Title').fill('Player View Test');
    await page.getByRole('textbox', { name: 'Description' }).fill('Players should see this read-only');

    const futureDate = new Date(Date.now() + 48 * 60 * 60 * 1000);
    futureDate.setMinutes(Math.ceil(futureDate.getMinutes() / 15) * 15);
    futureDate.setSeconds(0);
    futureDate.setMilliseconds(0);

    await setDeadlineDateTime(page, futureDate);
    await page.getByRole('button', { name: /Create Deadline/i }).click();

    // Wait for modal to close
    await expect(page.getByRole('heading', { name: 'Create New Deadline' })).not.toBeVisible();

    // Wait for network to be idle after deadline creation
    await page.waitForLoadState('networkidle');

    // Verify deadline appears in the list.
    // Scope to the visible copy: the widget renders a mobile and a desktop tree,
    // and re-runs against this shared fixture game can leave several deadlines
    // with this title, so a bare getByText would be a strict-mode violation.
    await expandAllDeadlines(page);
    await expect(page.getByText('Player View Test').locator('visible=true').first()).toBeVisible();

    // Login as player
    await loginAs(page, 'PLAYER_4');

    // Navigate to the same game
    await gameDetailsPage.goto(gameId);

    // Verify player can see the deadline title
    // NOTE: DeadlineCard only shows title/countdown/date, not description
    await expandAllDeadlines(page);
    await expect(page.getByText('Player View Test').locator('visible=true').first()).toBeVisible();

    // Verify "Add Deadline" button is NOT visible to players (permission boundary)
    await expect(page.getByRole('button', { name: /Add Deadline/i })).not.toBeVisible();
  });

  test('Deadline widget shows View All for multiple deadlines', async ({ page }) => {
    // Get game ID
    const gameId = await getFixtureGameId(page, 'E2E_ACTION');
    const gameDetailsPage = new GameDetailsPage(page);
    await gameDetailsPage.goto(gameId);

    // Use unique prefix to avoid conflicts with other tests
    const uniquePrefix = `ViewAll-${Date.now()}`;

    // Create 4 deadlines (more than the 3 shown by default)
    for (let i = 1; i <= 4; i++) {
      await page.getByRole('button', { name: /Add Deadline/i }).click();
      await page.getByLabel('Title').fill(`${uniquePrefix}-${i}`);
      await page.getByRole('textbox', { name: 'Description' }).fill(`Testing multiple deadlines ${i}`);

      // Set deadlines far enough in the future to account for test execution time
      // Start at 2 days from now, then add more days for each deadline
      const futureDate = new Date(Date.now() + ((i + 1) * 24) * 60 * 60 * 1000);
      futureDate.setMinutes(Math.ceil(futureDate.getMinutes() / 15) * 15);
      futureDate.setSeconds(0);
      futureDate.setMilliseconds(0);

      await setDeadlineDateTime(page, futureDate);
      // Wait for the API call to complete, then modal to close
      await Promise.all([
        page.waitForResponse(resp => resp.url().includes('/deadlines') && resp.request().method() === 'POST' && resp.status() === 201),
        page.getByRole('button', { name: /Create Deadline/i }).click(),
      ]);
      await expect(page.getByRole('heading', { name: 'Create Deadline' })).not.toBeVisible();
    }

    // After creating all 4 deadlines, verify "View All" button appears
    // (widget shows max 3 initially, so with our 4 + any existing, should have View All)
    const viewAllButton = page.getByRole('button', { name: /View All/i });
    await expect(viewAllButton).toBeVisible();

    // Click View All to show all deadlines
    await viewAllButton.click();

    // Verify we can see our newly created deadlines
    // Check that at least one of our uniquely-prefixed deadlines is visible
    await expect(page.locator('text=/ViewAll-\\d+/').first()).toBeVisible({ timeout: 5000 });

    // Button should now say "Show Less"
    await expect(page.getByRole('button', { name: /Show Less/i })).toBeVisible();
  });

});
