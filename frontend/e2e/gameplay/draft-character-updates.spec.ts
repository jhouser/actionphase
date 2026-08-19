import { test, expect } from '@playwright/test';
import { loginAs } from '../fixtures/auth-helpers';
import { getFixtureGameId } from '../fixtures/game-helpers';
import { GameDetailsPage } from '../pages/GameDetailsPage';
import { CharacterSheetPage } from '../pages/CharacterSheetPage';

/**
 * E2E Tests for Draft Character Updates Feature
 *
 * Tests the complete workflow for GMs creating and managing character sheet updates
 * when writing action results, focusing on:
 * - Creating draft updates (skills as primary example)
 * - Publishing results with character updates
 * - Confirmation dialog showing pending updates
 * - Published skill appearing on the player's character sheet
 *
 * Skills were the abilities module before the character sheet refactor retired
 * it: abilities duplicated skills, which is strictly more featured, so every
 * stat feature had to be built twice. The workflow under test is unchanged.
 *
 * Uses dedicated E2E fixture (E2E_GM_EDITING_RESULTS) which includes:
 * - Game with active action phase
 * - Unpublished result for Player 3 (GM can add character updates)
 *
 * CRITICAL: Serial mode required — test 5 publishes the unpublished result,
 * consuming the shared fixture state. Tests build on each other.
 */
test.describe('Draft Character Updates - Core Workflow', () => {
  test.describe.configure({ mode: 'serial' });

  test('GM can add a skill and it persists after reopening', async ({ page }) => {
    await loginAs(page, 'GM');
    const gameId = await getFixtureGameId(page, 'E2E_GM_EDITING_RESULTS');
    const gamePage = new GameDetailsPage(page);

    await gamePage.goto(gameId);
    await gamePage.goToActions();
    await expect(page.getByText('Unpublished Results (Editable)')).toBeVisible({ timeout: 10000 });

    await page.getByRole('button', { name: 'Update Character Sheet' }).click();
    await expect(page.getByRole('heading', { name: 'Update Character Sheet' })).toBeVisible({ timeout: 5000 });

    await page.getByRole('button', { name: 'Add Skill' }).click();
    await expect(page.getByPlaceholder('e.g., Sword Fighting, Lockpicking')).toBeVisible({ timeout: 5000 });

    const skillName = `Persist Test ${Date.now()}`;
    await page.getByPlaceholder('e.g., Sword Fighting, Lockpicking').fill(skillName);
    await page.getByPlaceholder('Describe this skill...').fill('You can see in darkness within 60 feet');
    await page.getByRole('button', { name: 'Add Skill' }).last().click();
    await expect(page.getByRole('heading', { name: skillName })).toBeVisible({ timeout: 5000 });

    // Close and wait for debounced save to complete, then reopen
    await page.getByRole('button', { name: 'Done' }).click();
    await expect(page.getByRole('heading', { name: 'Update Character Sheet' })).not.toBeVisible();
    await page.waitForResponse(resp => resp.url().includes('/character-updates') && resp.status() === 200, { timeout: 5000 });

    await page.getByRole('button', { name: 'Update Character Sheet' }).click();
    await expect(page.getByRole('heading', { name: 'Update Character Sheet' })).toBeVisible({ timeout: 5000 });

    // Skill should still be present after closing and reopening (not just in-memory)
    await expect(page.getByRole('heading', { name: skillName })).toBeVisible({ timeout: 5000 });
  });

  test('GM can remove a skill', async ({ page }) => {
    await loginAs(page, 'GM');
    const gameId = await getFixtureGameId(page, 'E2E_GM_EDITING_RESULTS');
    const gamePage = new GameDetailsPage(page);

    await gamePage.goto(gameId);
    await gamePage.goToActions();
    await expect(page.getByText('Unpublished Results (Editable)')).toBeVisible({ timeout: 10000 });

    await page.getByRole('button', { name: 'Update Character Sheet' }).click();
    await expect(page.getByRole('heading', { name: 'Update Character Sheet' })).toBeVisible({ timeout: 5000 });

    await page.getByRole('button', { name: 'Add Skill' }).click();
    const uniqueSkillName = `Remove Test ${Date.now()}`;
    await page.getByPlaceholder('e.g., Sword Fighting, Lockpicking').fill(uniqueSkillName);
    await page.getByPlaceholder('Describe this skill...').fill('Test removal');
    await page.getByRole('button', { name: 'Add Skill' }).last().click();
    await expect(page.getByRole('heading', { name: uniqueSkillName })).toBeVisible({ timeout: 5000 });

    // Scoped to the card holding this skill's heading: every card renders a
    // "Remove skill" button and only this one may be clicked. Filtered on both
    // the heading and the button rather than picking a div by position —
    // `.locator('div')` matches nested wrappers, so .first()/.last() lands on
    // whichever nesting depth happens to win rather than on the card.
    const skillCard = page
      .getByTestId('skills-section')
      .locator('div')
      .filter({ has: page.getByRole('heading', { name: uniqueSkillName }) })
      .filter({ has: page.getByRole('button', { name: 'Remove skill' }) })
      .last();
    await skillCard.getByRole('button', { name: 'Remove skill' }).click();

    await expect(page.getByRole('heading', { name: uniqueSkillName })).not.toBeVisible();
  });

  test('GM sees draft count badge on Update Character Sheet button', async ({ page }) => {
    await loginAs(page, 'GM');
    const gameId = await getFixtureGameId(page, 'E2E_GM_EDITING_RESULTS');
    const gamePage = new GameDetailsPage(page);

    await gamePage.goto(gameId);
    await gamePage.goToActions();
    await expect(page.getByText('Unpublished Results (Editable)')).toBeVisible({ timeout: 10000 });

    await page.getByRole('button', { name: 'Update Character Sheet' }).click();
    await expect(page.getByRole('heading', { name: 'Update Character Sheet' })).toBeVisible({ timeout: 5000 });

    await page.getByRole('button', { name: 'Add Skill' }).click();
    const skillName = `Badge Test ${Date.now()}`;
    await page.getByPlaceholder('e.g., Sword Fighting, Lockpicking').fill(skillName);
    await page.getByPlaceholder('Describe this skill...').fill('Description');
    await page.getByRole('button', { name: 'Add Skill' }).last().click();
    await expect(page.getByRole('heading', { name: skillName })).toBeVisible({ timeout: 5000 });

    await Promise.all([
      page.waitForResponse(resp => resp.url().includes('/character-updates') && resp.status() === 200, { timeout: 5000 }),
      page.getByRole('button', { name: 'Done' }).click(),
    ]);

    // Button should show a badge with a count > 0
    const updateButton = page.getByRole('button', { name: /Update Character Sheet/ });
    await expect(updateButton.locator('span, div').filter({ hasText: /^\d+$/ })).toBeVisible({ timeout: 5000 });
  });

  test('publish confirmation dialog shows character update warning', async ({ page }) => {
    await loginAs(page, 'GM');
    const gameId = await getFixtureGameId(page, 'E2E_GM_EDITING_RESULTS');
    const gamePage = new GameDetailsPage(page);

    await gamePage.goto(gameId);
    await gamePage.goToActions();
    await expect(page.getByText('Unpublished Results (Editable)')).toBeVisible({ timeout: 10000 });

    await page.getByRole('button', { name: 'Update Character Sheet' }).click();
    await expect(page.getByRole('heading', { name: 'Update Character Sheet' })).toBeVisible({ timeout: 5000 });
    await page.getByRole('button', { name: 'Add Skill' }).click();
    const skillName = `Publish Dialog Test ${Date.now()}`;
    await page.getByPlaceholder('e.g., Sword Fighting, Lockpicking').fill(skillName);
    await page.getByPlaceholder('Describe this skill...').fill('This will be published');
    await page.getByRole('button', { name: 'Add Skill' }).last().click();
    await expect(page.getByRole('heading', { name: skillName })).toBeVisible({ timeout: 5000 });
    await Promise.all([
      page.waitForResponse(resp => resp.url().includes('/character-updates') && resp.status() === 200, { timeout: 5000 }),
      page.getByRole('button', { name: 'Done' }).click(),
    ]);

    await page.getByRole('button', { name: 'Publish Result' }).click();
    await expect(page.getByRole('heading', { name: 'Publish Action Result?' })).toBeVisible({ timeout: 5000 });
    await expect(page.getByText(/This will also publish \d+ character sheet update/)).toBeVisible();

    // Dismiss without publishing — test 5 needs the unpublished result intact
    await page.getByRole('button', { name: 'Cancel' }).click();
    await expect(page.getByRole('heading', { name: 'Publish Action Result?' })).not.toBeVisible();
  });

  test('publishing result applies skill to player character sheet', async ({ page }) => {
    await loginAs(page, 'GM');
    const gameId = await getFixtureGameId(page, 'E2E_GM_EDITING_RESULTS');
    const gamePage = new GameDetailsPage(page);

    await gamePage.goto(gameId);
    await gamePage.goToActions();
    await expect(page.getByText('Unpublished Results (Editable)')).toBeVisible({ timeout: 10000 });

    // Add the skill that will be published to the player's sheet
    await page.getByRole('button', { name: 'Update Character Sheet' }).click();
    await expect(page.getByRole('heading', { name: 'Update Character Sheet' })).toBeVisible({ timeout: 5000 });
    await page.getByRole('button', { name: 'Add Skill' }).click();
    const skillName = `Final Skill ${Date.now()}`;
    await page.getByPlaceholder('e.g., Sword Fighting, Lockpicking').fill(skillName);
    await page.getByPlaceholder('Describe this skill...').fill('Granted by GM on publish');
    await page.getByRole('button', { name: 'Add Skill' }).last().click();
    await expect(page.getByRole('heading', { name: skillName })).toBeVisible({ timeout: 5000 });
    await Promise.all([
      page.waitForResponse(resp => resp.url().includes('/character-updates') && resp.status() === 200, { timeout: 5000 }),
      page.getByRole('button', { name: 'Done' }).click(),
    ]);

    // Publish the result
    await page.getByRole('button', { name: 'Publish Result' }).click();
    await expect(page.getByRole('heading', { name: 'Publish Action Result?' })).toBeVisible({ timeout: 5000 });
    await page.getByRole('button', { name: 'Publish', exact: true }).click();
    await expect(page.getByRole('heading', { name: 'Publish Action Result?' })).not.toBeVisible({ timeout: 10000 });
    await expect(page.getByText('0 Unpublished')).toBeVisible({ timeout: 5000 });

    // Switch to Player 3 and verify the skill now appears on their character sheet
    await loginAs(page, 'PLAYER_3');
    await gamePage.goto(gameId);
    await gamePage.goToCharacters();

    await page.getByRole('button', { name: 'Edit Sheet' }).click();

    const characterSheet = new CharacterSheetPage(page);
    await characterSheet.goToSkillsTab();

    await expect(page.getByRole('heading', { name: skillName })).toBeVisible({ timeout: 5000 });
  });

  test('player cannot see Update Character Sheet button', async ({ page }) => {
    await loginAs(page, 'PLAYER_3');
    const gameId = await getFixtureGameId(page, 'E2E_GM_EDITING_RESULTS');
    const gamePage = new GameDetailsPage(page);

    await gamePage.goto(gameId);

    // Players should not see the Update Character Sheet button (GM-only)
    await expect(page.getByRole('button', { name: 'Update Character Sheet' })).not.toBeVisible({ timeout: 10000 });
  });
});
