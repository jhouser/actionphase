import { test, expect } from '@playwright/test';
import { loginAs } from '../fixtures/auth-helpers';
import { getFixtureGameId } from '../fixtures/game-helpers';
import { CharacterWorkflowPage } from '../pages/CharacterWorkflowPage';
import { CharacterSheetPage } from '../pages/CharacterSheetPage';

/**
 * E2E Tests for Character Sheet Management
 *
 * Tests the complete character sheet management workflow including:
 * - Adding/viewing skills with ranks and descriptions
 * - Adding/removing inventory items
 * - Viewing numbers, including bounded tracks
 * - Permission boundaries (bio public, stat tabs private and GM-edit-only)
 * - GM can view all character sheets
 *
 * Uses dedicated E2E fixture (E2E_CHARACTER_SHEETS) which includes:
 * - Character 1: skills (4), items (2), numbers (2, one a bounded track)
 * - Character 2: different data for comparison, incl. one unmigrated `level` row
 * - Character 3: Empty sheet for fresh additions
 *
 * CRITICAL: This tests CORE player engagement mechanics
 */

test.describe('Character Sheet Management', () => {

  // Close any open modals after each test so the next test starts clean.
  // afterEach runs while still on the game page, so Escape reliably dismisses the modal.
  // beforeEach was wrong for this: it fired before loginAs/goto, so the page was
  // on '/' and Escape had nothing to close.
  test.afterEach(async ({ page }) => {
    await page.keyboard.press('Escape');
  });

  test('player can view existing skills, items, and numbers on their character sheet', async ({ page }) => {
    await loginAs(page, 'PLAYER_1');

    const gameId = await getFixtureGameId(page, 'E2E_CHARACTER_SHEETS');

    // Navigate to Characters tab and open character sheet
    const characterPage = new CharacterWorkflowPage(page, gameId);
    await characterPage.goto();

    // Open character sheet using POM
    await characterPage.openCharacterSheet('Sheet Test Char 1');

    // Wait for sheet to load
    await expect(page.getByRole('heading', { name: 'Sheet Test Char 1', level: 2 })).toBeVisible({ timeout: 10000 });

    // Initialize CharacterSheetPage
    const sheetPage = new CharacterSheetPage(page);

    // ===== Test Skills =====
    // All four entries live on one tab now. Keen Eye and Quick Draw were
    // abilities before the refactor retired that module and folded its content
    // into skills, so they are asserted here rather than in a separate block.
    await sheetPage.goToSkillsTab();

    const expectSkillWithDescription = async (name: string, description: string) => {
      await expect(page.getByRole('heading', { name })).toBeVisible();
      // Filtered on both the heading and the expand button rather than picking
      // a div by position: `.locator('div')` matches nested wrappers, so
      // .last() lands on the heading's own wrapper, which holds no button.
      const card = sheetPage.skillsSection
        .locator('div')
        .filter({ has: page.getByRole('heading', { name }) })
        .filter({ has: page.getByRole('button', { name: /expand description/i }) })
        .last();
      // Descriptions render collapsed.
      await card.getByRole('button', { name: /expand description/i }).first().click();
      await expect(page.locator(`text=${description}`)).toBeVisible();
    };

    await expectSkillWithDescription('Archery', 'Master archer');
    await expectSkillWithDescription('Tracking', 'Can track creatures');
    await expectSkillWithDescription('Keen Eye', 'Can spot hidden details');
    await expectSkillWithDescription('Quick Draw', 'Fast weapon draw');

    // Rank replaced the old `level` field. The fixture's skill-8 deliberately
    // still uses the legacy key, but these four are on the current shape.
    await expect(sheetPage.skillsSection.getByText('Rank: Expert').first()).toBeVisible();

    // ===== Test Inventory =====
    await sheetPage.goToInventoryTab();

    await expect(page.getByRole('heading', { name: 'Longbow' })).toBeVisible();
    const longbowCard = sheetPage.itemsSection
      .locator('div')
      .filter({ has: page.getByRole('heading', { name: 'Longbow' }) })
      .filter({ has: page.getByRole('button', { name: /expand description/i }) })
      .last();
    await longbowCard.getByRole('button', { name: /expand description/i }).first().click();
    await expect(page.locator('text=Masterwork longbow')).toBeVisible();

    await expect(page.getByRole('heading', { name: 'Arrows' })).toBeVisible();
    const arrowsCard = sheetPage.itemsSection
      .locator('div')
      .filter({ has: page.getByRole('heading', { name: 'Arrows' }) })
      .filter({ has: page.getByRole('button', { name: /expand description/i }) })
      .last();
    await arrowsCard.getByRole('button', { name: /expand description/i }).first().click();
    await expect(page.locator('text=Steel-tipped arrows')).toBeVisible();

    // No item sets weight or value, so the totals line stays hidden rather than
    // reporting a fabricated "0.0".
    await expect(sheetPage.itemsSection.getByText(/Total Weight/)).not.toBeVisible();

    // ===== Test Numbers =====
    // Promoted out of Inventory's Currency sub-tab and renamed: the tab holds
    // arbitrary numeric tracks, not only money.
    await sheetPage.goToNumbersTab();

    await expect(sheetPage.numbersSection.getByText('Gold')).toBeVisible();
    await expect(sheetPage.numbersSection.getByText('50')).toBeVisible();

    // Stress carries a maximum, which renders as a bounded track rather than a
    // bare count — the structure that justified giving Numbers its own tab.
    await expect(sheetPage.numbersSection.getByText('Stress')).toBeVisible();
    await expect(sheetPage.numbersSection.getByRole('img', { name: 'Stress: 4 of 9' })).toBeVisible();
  });

  test('GM can view all character sheets', async ({ page }) => {
    await loginAs(page, 'GM');

    const gameId = await getFixtureGameId(page, 'E2E_CHARACTER_SHEETS');

    // Navigate to Characters tab
    const characterPage = new CharacterWorkflowPage(page, gameId);
    await characterPage.goto();

    // Verify GM sees all characters.
    // Each character card renders two data-testid="character-name" elements — one for mobile
    // layout (hidden on desktop) and one for desktop layout (hidden on mobile). Filter to the
    // visible one to avoid strict mode violations and hidden-element false negatives.
    await expect(page.getByTestId('character-name').filter({ hasText: 'Sheet Test Char 1' }).locator('visible=true').first()).toBeVisible({ timeout: 10000 });
    await expect(page.getByTestId('character-name').filter({ hasText: 'Sheet Test Char 2' }).locator('visible=true').first()).toBeVisible();
    await expect(page.getByTestId('character-name').filter({ hasText: 'Empty Sheet Char' }).locator('visible=true').first()).toBeVisible();

    // GM should be able to view any character (open char 2, owned by PLAYER_2)
    await characterPage.openCharacterSheet('Sheet Test Char 2');

    // Verify GM can see character sheet modal
    await expect(page.getByRole('heading', { name: 'Sheet Test Char 2', level: 2 })).toBeVisible({ timeout: 10000 });

    // Initialize CharacterSheetPage
    const sheetPage = new CharacterSheetPage(page);

    await sheetPage.goToSkillsTab();

    // Verify the GM sees the mage's skills. Fireball and Shield were abilities
    // before that module was retired and folded into skills.
    await expect(page.getByRole('heading', { name: 'Arcana' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Fireball' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Shield' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Arcane Knowledge' })).toBeVisible();

    // Arcane Knowledge deliberately still carries the pre-rename `level` key in
    // the fixture: the rename is resolved on read, not migrated, so an
    // unmigrated row must still render its rank.
    const arcaneCard = sheetPage.skillsSection
      .locator('div')
      .filter({ has: page.getByRole('heading', { name: 'Arcane Knowledge' }) })
      .filter({ has: page.getByText('Rank:') })
      .last();
    await expect(arcaneCard.getByText('Rank: Expert')).toBeVisible();
  });

  test('bio tab is public, stat tabs are private', async ({ page }) => {
    // Verify PLAYER_2 can only see the bio of PLAYER_1's character
    await loginAs(page, 'PLAYER_2');

    const gameId = await getFixtureGameId(page, 'E2E_CHARACTER_SHEETS');

    // Navigate to Characters tab
    const characterPage = new CharacterWorkflowPage(page, gameId);
    await characterPage.goto();

    // Click on another player's character (Sheet Test Char 1, owned by PLAYER_1)
    await characterPage.openCharacterSheet('Sheet Test Char 1');

    // Wait for sheet modal to open
    await expect(page.getByRole('heading', { name: 'Sheet Test Char 1', level: 2 })).toBeVisible({ timeout: 10000 });

    // Initialize CharacterSheetPage
    const sheetPage = new CharacterSheetPage(page);

    // Should see the Public Profile tab (public)
    expect(await sheetPage.isModuleVisible('Public Profile')).toBe(true);

    // Should NOT see Private Notes or any stat tab (owner/GM only)
    expect(await sheetPage.isModuleVisible('Private Notes')).toBe(false);
    expect(await sheetPage.isModuleVisible('Skills')).toBe(false);
    expect(await sheetPage.isModuleVisible('Inventory')).toBe(false);
    expect(await sheetPage.isModuleVisible('Numbers')).toBe(false);

    // Verify bio content is visible
    await expect(page.locator('text=A weathered ranger with keen eyes')).toBeVisible();
  });

  test('player cannot edit skills, inventory, or numbers', async ({ page }) => {
    // Verify a player CANNOT add or edit stats on their own character
    await loginAs(page, 'PLAYER_1');

    const gameId = await getFixtureGameId(page, 'E2E_CHARACTER_SHEETS');

    // Navigate to Characters tab and open character sheet
    const characterPage = new CharacterWorkflowPage(page, gameId);
    await characterPage.goto();
    await characterPage.openCharacterSheet('Sheet Test Char 1');

    // Wait for sheet to load
    await expect(page.getByRole('heading', { name: 'Sheet Test Char 1', level: 2 })).toBeVisible({ timeout: 10000 });

    // Initialize CharacterSheetPage
    const sheetPage = new CharacterSheetPage(page);

    // ===== Skills - No Edit UI =====
    // Stat tabs are GM-edit-only by design: players cannot touch their own
    // numbers. Enforced server-side too (api_data.go's isStatField).
    await sheetPage.goToSkillsTab();

    expect(await sheetPage.canAddSkill()).toBe(false);
    await expect(page.getByRole('button', { name: 'Edit skill' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Remove skill' })).toHaveCount(0);

    // ===== Inventory - No Edit UI =====
    await sheetPage.goToInventoryTab();

    expect(await sheetPage.canAddItem()).toBe(false);
    await expect(page.getByRole('button', { name: 'Edit item' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Remove item' })).toHaveCount(0);

    // ===== Numbers - No Edit UI =====
    await sheetPage.goToNumbersTab();

    expect(await sheetPage.canAddNumber()).toBe(false);
    await expect(page.getByRole('button', { name: 'Edit entry' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Remove entry' })).toHaveCount(0);
  });

  test('GM can edit skills, inventory, and numbers', async ({ page }) => {
    // CharacterWorkflowPage now supports both character_creation and in_progress game states
    // Verify a GM CAN add and edit stats on any character
    await loginAs(page, 'GM');

    const gameId = await getFixtureGameId(page, 'E2E_CHARACTER_SHEETS');

    // Navigate to Characters tab and open character sheet
    const characterPage = new CharacterWorkflowPage(page, gameId);
    await characterPage.goto();
    await characterPage.openCharacterSheet('Sheet Test Char 1');

    // Wait for sheet to load
    await expect(page.getByRole('heading', { name: 'Sheet Test Char 1', level: 2 })).toBeVisible({ timeout: 10000 });

    // Initialize CharacterSheetPage
    const sheetPage = new CharacterSheetPage(page);

    // ===== Skills - GM can add =====
    await sheetPage.goToSkillsTab();

    expect(await sheetPage.canAddSkill()).toBe(true);
    await sheetPage.addSkill('Test Skill', 'Test description');
    await expect(page.getByRole('heading', { name: 'Test Skill' })).toBeVisible();

    // ===== Inventory - GM can add =====
    await sheetPage.goToInventoryTab();

    expect(await sheetPage.canAddItem()).toBe(true);

    // ===== Numbers - GM can add =====
    await sheetPage.goToNumbersTab();

    expect(await sheetPage.canAddNumber()).toBe(true);
  });
});
