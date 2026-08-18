import { test, expect } from '@playwright/test';
import { loginAs } from '../fixtures/auth-helpers';
import { createDraftResultViaApi, getFixtureGameId, getParticipantUserId, getWorkerUsername } from '../fixtures/game-helpers';
import { GameDetailsPage } from '../pages/GameDetailsPage';
import { ActionResultsPage } from '../pages/ActionResultsPage';

/**
 * E2E Tests for Action Results Flow
 *
 * Tests the complete action results workflow including:
 * - Player views published action results via History tab
 * - Player cannot see unpublished results
 * - Results display with markdown content and character mentions
 * - Multiple results display correctly
 *
 * Uses dedicated E2E fixture (E2E_ACTION_RESULTS) which includes:
 * - Game with completed action phase (Phase 1)
 * - Published results for Player 1 and Player 2
 * - Unpublished result for Player 3 (should not be visible to players)
 * - Player 4 has no result (for empty state testing)
 *
 * CRITICAL: This tests CORE game mechanic - GM providing feedback to players
 */

test.describe('@mobile Action Results Flow', () => {

  test('player can view their published action results', async ({ page }) => {
    await loginAs(page, 'PLAYER_1');
    const gameId = await getFixtureGameId(page, 'E2E_ACTION_RESULTS');
    const resultsPage = new ActionResultsPage(page, gameId);

    // Navigate to History tab and view Phase 1 results using POM
    await resultsPage.goto();
    await resultsPage.viewPhaseResults(1);

    // Should see action results heading for the phase
    await expect(page.getByRole('heading', { name: /Completed Action Phase/ })).toBeVisible({ timeout: 10000 });

    // Should see Player 1's published result
    await expect(page.getByText('Basement Investigation Results').first()).toBeVisible({ timeout: 10000 });

    // Long results collapse behind a toggle; short ones render in full. Which
    // applies depends on rendered height, so expand only if there is a toggle.
    await resultsPage.expandResultsIfCollapsed();

    // Should see result content with markdown
    await expect(page.locator('text=You descend into the basement')).toBeVisible();

    // Should see discovery outcome
    await expect(page.locator('text=You discovered')).toBeVisible();
    await expect(page.locator('text=A secret passage!')).toBeVisible();

    // Should see GM attribution (TestGM or TestGM_N for worker N)
    await expect(page.getByText(`From: ${getWorkerUsername('TestGM')}`).first()).toBeVisible();
  });

  test('player can see character mentions in results', async ({ page }) => {
    await loginAs(page, 'PLAYER_1');
    const gameId = await getFixtureGameId(page, 'E2E_ACTION_RESULTS');
    const resultsPage = new ActionResultsPage(page, gameId);

    // Navigate to History tab and view Phase 1 results using POM
    await resultsPage.goto();
    await resultsPage.viewPhaseResults(1);

    // Wait for results to load
    await expect(page.getByText('Basement Investigation Results').first()).toBeVisible({ timeout: 10000 });

    // Long results collapse behind a toggle; short ones render in full. Which
    // applies depends on rendered height, so expand only if there is a toggle.
    await resultsPage.expandResultsIfCollapsed();

    // Player 1's result contains a character mention: "@Result Test Char 2"
    await expect(page.locator('text=@Result Test Char 2')).toBeVisible({ timeout: 10000 });
    await expect(page.locator('text=might want to know about this')).toBeVisible();
  });

  test('player sees multiple results if they have multiple', async ({ page }) => {
    // Player 2 should have published results (based on fixture)
    await loginAs(page, 'PLAYER_2');
    const gameId = await getFixtureGameId(page, 'E2E_ACTION_RESULTS');
    const resultsPage = new ActionResultsPage(page, gameId);

    // Navigate to History tab and view Phase 1 results using POM
    await resultsPage.goto();
    await resultsPage.viewPhaseResults(1);

    // Should see action results heading
    await expect(page.getByRole('heading', { name: /Completed Action Phase/ })).toBeVisible({ timeout: 10000 });

    // Should see Player 2's result about library research
    await expect(page.getByText('Library Research Results').first()).toBeVisible({ timeout: 10000 });

    // Long results collapse behind a toggle; short ones render in full. Which
    // applies depends on rendered height, so expand only if there is a toggle.
    await resultsPage.expandResultsIfCollapsed();

    // Should see result content
    await expect(page.locator('text=dusty tomes')).toBeVisible();
    await expect(page.locator('text=Order of the Crimson Moon')).toBeVisible();

    // Should see knowledge gained
    await expect(page.locator('text=Knowledge Gained')).toBeVisible();
    await expect(page.locator('text=+1 Occult Lore')).toBeVisible();
  });

  test('player cannot see unpublished results', async ({ page }) => {
    // Player 3 has an UNPUBLISHED result in the fixture
    await loginAs(page, 'PLAYER_3');
    const gameId = await getFixtureGameId(page, 'E2E_ACTION_RESULTS');
    const resultsPage = new ActionResultsPage(page, gameId);

    // Navigate to History tab and view Phase 1 results using POM
    await resultsPage.goto();
    await resultsPage.viewPhaseResults(1);

    // Should NOT see unpublished result content
    await expect(page.locator('text=DRAFT: The symbols appear to be a warning')).not.toBeVisible();

    // Should see empty state message since unpublished results don't show
    const noResultsMessage = page.locator('text=No action results for this phase');
    await expect(noResultsMessage).toBeVisible({ timeout: 10000 });
  });

  test('player with no results sees empty state', async ({ page }) => {
    // Player 4 has no results in the fixture
    await loginAs(page, 'PLAYER_4');
    const gameId = await getFixtureGameId(page, 'E2E_ACTION_RESULTS');
    const resultsPage = new ActionResultsPage(page, gameId);

    // Navigate to History tab and view Phase 1 results using POM
    await resultsPage.goto();
    await resultsPage.viewPhaseResults(1);

    // Should see empty state message
    await expect(page.locator('text=No action results for this phase')).toBeVisible({ timeout: 10000 });
  });
});

/**
 * E2E Tests for GM Editing Action Results
 *
 * Tests the complete GM result editing workflow including:
 * - GM can view unpublished results in GameResultsManager
 * - GM can edit unpublished result content
 * - GM can save changes to unpublished results
 * - Edited results remain unpublished until explicitly published
 * - Players still cannot see unpublished results after edit
 *
 * Uses dedicated E2E fixture (E2E_GM_EDITING_RESULTS) which includes:
 * - Game #327 with ACTIVE action phase (deadline passed, GM writing results)
 * - Unpublished result for Player 3 (GM can edit this)
 * - Published results for Player 1 and Player 2 (cannot be edited)
 *
 * CRITICAL: This tests CORE GM workflow - editing draft results before publishing
 */
test.describe('@mobile GM Action Results Editing', () => {
  test.describe.configure({ mode: 'serial' });

  test('GM can view unpublished results in GameResultsManager', async ({ page }) => {
    await loginAs(page, 'GM');
    const gameId = await getFixtureGameId(page, 'E2E_GM_EDITING_RESULTS');
    const gamePage = new GameDetailsPage(page);

    // Navigate to game and go to Actions tab
    await gamePage.goto(gameId);
    await gamePage.goToActions();

    // Should see GameResultsManager heading
    await expect(page.getByRole('heading', { name: 'Action Results', exact: true })).toBeVisible({ timeout: 10000 });

    // Should see Unpublished Results section
    await expect(page.getByText('Unpublished Results (Editable)')).toBeVisible({ timeout: 10000 });

    // Should see the unpublished result for TestPlayer3
    await expect(page.locator('text=DRAFT: The symbols appear to be a warning')).toBeVisible({ timeout: 10000 });

    // Should see Draft badge
    await expect(page.locator('text=Draft').locator('visible=true').first()).toBeVisible();
  });

  test('GM can save edited result successfully', async ({ page }) => {
    await loginAs(page, 'GM');
    const gameId = await getFixtureGameId(page, 'E2E_GM_EDITING_RESULTS');
    const gamePage = new GameDetailsPage(page);

    // Navigate to game and go to Actions tab
    await gamePage.goto(gameId);
    await gamePage.goToActions();

    // Wait for GameResultsManager to load
    await expect(page.getByText('Unpublished Results (Editable)')).toBeVisible({ timeout: 10000 });

    // Click Edit button
    await page.getByRole('button', { name: 'Edit', exact: true }).click();

    // Modify content
    const textarea = page.getByRole('textbox');
    await textarea.clear();
    const updatedContent = 'FINAL EDIT: The investigation reveals crucial evidence about the cult\'s plans. Time: ' + Date.now();
    await textarea.fill(updatedContent);

    // Click Save Changes
    await page.getByRole('button', { name: 'Save Changes' }).click();

    // Edit form should close
    await expect(page.getByRole('textbox')).not.toBeVisible({ timeout: 10000 });

    // Updated content should be visible
    await expect(page.locator(`text=${updatedContent.substring(0, 50)}`)).toBeVisible({ timeout: 10000 });

    // Should still see Draft badge (result remains unpublished)
    await expect(page.locator('text=Draft').locator('visible=true').first()).toBeVisible();
  });

  test('GM can cancel editing without saving changes', async ({ page }) => {
    await loginAs(page, 'GM');
    const gameId = await getFixtureGameId(page, 'E2E_GM_EDITING_RESULTS');
    const gamePage = new GameDetailsPage(page);

    // Navigate to game and go to Actions tab
    await gamePage.goto(gameId);
    await gamePage.goToActions();

    // Wait for GameResultsManager
    await expect(page.getByText('Unpublished Results (Editable)')).toBeVisible({ timeout: 10000 });

    // Click Edit button
    await page.getByRole('button', { name: 'Edit', exact: true }).click();

    // Capture the current content before editing, so we can assert it's restored after cancel
    const textarea = page.getByRole('textbox');
    const originalContent = await textarea.inputValue();
    await textarea.clear();
    await textarea.fill('This change should be cancelled and not saved');

    // Click Cancel
    await page.getByRole('button', { name: 'Cancel' }).click();

    // Edit form should close
    await expect(page.getByRole('textbox')).not.toBeVisible({ timeout: 10000 });

    // Original content should still be visible (changes were discarded)
    await expect(page.locator(`text=${originalContent.substring(0, 50)}`)).toBeVisible();
  });

  test('players cannot see unpublished results even after GM edits them', async ({ page }) => {
    // First, GM edits the unpublished result
    await loginAs(page, 'GM');
    const gameId = await getFixtureGameId(page, 'E2E_GM_EDITING_RESULTS');
    const gamePage = new GameDetailsPage(page);

    await gamePage.goto(gameId);
    await gamePage.goToActions();

    await expect(page.getByText('Unpublished Results (Editable)')).toBeVisible({ timeout: 10000 });
    await page.getByRole('button', { name: 'Edit', exact: true }).click();

    const textarea = page.getByRole('textbox');
    await textarea.clear();
    await textarea.fill('EDITED BY GM: This content has been updated but not published yet');
    await page.getByRole('button', { name: 'Save Changes' }).click();

    await expect(page.getByRole('textbox')).not.toBeVisible({ timeout: 10000 });

    // Now login as Player 3 and verify they still cannot see the edited result
    await loginAs(page, 'PLAYER_3');
    const resultsPage = new ActionResultsPage(page, gameId);

    // Navigate to History tab and view Phase 1 results
    await resultsPage.goto();
    await resultsPage.viewPhaseResults(1);

    // Should NOT see the edited unpublished result
    await expect(page.locator('text=EDITED BY GM')).not.toBeVisible();
    await expect(page.locator('text=DRAFT: The symbols')).not.toBeVisible();

    // Should see empty state message since unpublished results don't show
    await expect(page.locator('text=No action results for this phase')).toBeVisible({ timeout: 10000 });
  });

  test('GM cannot edit published results', async ({ page }) => {
    await loginAs(page, 'GM');
    const gameId = await getFixtureGameId(page, 'E2E_GM_EDITING_RESULTS');
    const gamePage = new GameDetailsPage(page);

    // Navigate to game and go to Actions tab
    await gamePage.goto(gameId);
    await gamePage.goToActions();

    // Wait for GameResultsManager to load
    await expect(page.getByTestId('published-results-section')).toBeVisible({ timeout: 10000 });

    // Published results section should NOT have Edit buttons
    const publishedSection = page.getByTestId('published-results-section');
    await expect(publishedSection.getByRole('button', { name: 'Edit', exact: true })).not.toBeVisible();

    // Unpublished results section SHOULD have Edit buttons
    const unpublishedSection = page.getByTestId('unpublished-results-section');
    await expect(unpublishedSection.getByRole('button', { name: 'Edit', exact: true })).toBeVisible();
  });

  test('GM can delete a draft result', async ({ page }) => {
    await loginAs(page, 'GM');
    const gameId = await getFixtureGameId(page, 'E2E_GM_EDITING_RESULTS');

    // Create a fresh draft result via API so this test doesn't depend on fixture state
    const player4Id = await getParticipantUserId(page, gameId, getWorkerUsername('TestPlayer4'));
    const resultId = await createDraftResultViaApi(page, gameId, player4Id, 'E2E delete test draft result');
    expect(resultId).toBeTruthy();

    const gamePage = new GameDetailsPage(page);
    await gamePage.goto(gameId);
    await gamePage.goToActions();

    // Wait for unpublished results section to be visible (includes our new result)
    await expect(page.getByText('Unpublished Results (Editable)')).toBeVisible({ timeout: 10000 });

    // Find and delete our specific result using its testid
    await page.getByTestId(`delete-result-${resultId}`).click();

    // Confirmation modal should appear
    await expect(page.getByRole('heading', { name: 'Delete Draft Result' })).toBeVisible({ timeout: 5000 });

    // Confirm deletion
    await page.getByRole('button', { name: 'Yes, Delete Draft' }).click();

    // Our result should be gone — modal closes and result no longer in the list
    await expect(page.getByRole('heading', { name: 'Delete Draft Result' })).not.toBeVisible({ timeout: 10000 });
    await expect(page.getByTestId(`delete-result-${resultId}`)).not.toBeVisible();
  });
});
