import { test, expect } from '@playwright/test';
import { loginAs } from '../fixtures/auth-helpers';
import { getFixtureGameId } from '../fixtures/game-helpers';
import { CharacterWorkflowPage } from '../pages/CharacterWorkflowPage';
import { CharacterSheetPage } from '../pages/CharacterSheetPage';

/**
 * E2E Tests for Loot Tables
 *
 * Covers the one thing no lower layer can: the seam between the GM's loot table
 * and a character's inventory. Component tests mock
 * apiClient.games.giveRandomLootTableContent, and the Go handler tests never
 * touch the sheet, so nothing else proves a rolled item is persisted and
 * rendered.
 *
 * Deliberately NOT covered here (already covered cheaper and faster):
 * - loot mode gating, JSON unpacking, selection resets -> ItemForm /
 *   LootTableSelector component tests
 * - authorization, cross-game access, empty-table 400 -> pkg/games handler tests
 * - failed-roll error surfacing (empty table, network, malformed data) ->
 *   InventoryManager.loot.test.tsx. There was an E2E test for the empty-table
 *   case, but ItemForm now requests loot tables with exclude-empty=true, so an
 *   empty table can no longer be selected and the flow is unreachable from the
 *   UI. The 400 remains as a backstop, asserted by
 *   TestSetRandomLootForCharacterEmptyTable.
 *
 * Uses the dedicated E2E_LOOT_TABLES fixture (backend .../e2e/24_loot_tables.sql):
 * - 'E2E Single Item Table' holds exactly one item, so a random roll is
 *   deterministic and the test can assert on a specific name
 * - 'E2E Empty Table' holds none; it is no longer selectable in the UI, but is
 *   kept so the fixture still exercises the exclude-empty filter
 * - 'Loot Test Char' starts with an empty inventory
 */

const ROLLED_ITEM = 'Enchanted Compass';

test.describe('Loot Tables', () => {
  test('GM rolls random loot onto a character sheet and it persists', async ({ page }) => {
    await loginAs(page, 'GM');
    const gameId = await getFixtureGameId(page, 'E2E_LOOT_TABLES');

    const characterPage = new CharacterWorkflowPage(page, gameId);
    await characterPage.goto();
    await characterPage.openCharacterSheet('Loot Test Char');

    await expect(page.getByRole('heading', { name: 'Loot Test Char', level: 2 })).toBeVisible({ timeout: 10000 });

    const sheetPage = new CharacterSheetPage(page);
    // One call, not two: the sheet's tabs are flat now, so Inventory is a tab
    // rather than a module with an Items sub-tab.
    await sheetPage.goToInventoryTab();

    // The fixture character starts empty, so anything present afterwards came
    // from the roll rather than from seeded data.
    await expect(page.getByRole('heading', { name: ROLLED_ITEM })).toHaveCount(0);

    await page.getByRole('button', { name: /^add item$/i }).click();

    // Loot modes only appear inside a game context; their absence here would
    // mean the sheet lost its GameProvider.
    await page.getByLabel(/^mode$/i).selectOption('loot_table_random');
    await page.getByLabel(/^loot table$/i).selectOption({ label: 'E2E Single Item Table' });

    await page.getByRole('button', { name: /^add item$/i }).last().click();

    // The roll round-trips through the server, which persists the item and
    // returns its GM-authored JSON for the client to unpack.
    await expect(page.getByRole('heading', { name: ROLLED_ITEM })).toBeVisible({ timeout: 10000 });

    // Reload to prove the item was actually written, not just held in local
    // component state after the optimistic update. The open sheet is encoded in
    // the URL (?character=), so the reload reopens it — clicking the card again
    // would land on the still-open modal's backdrop.
    await page.reload();
    await expect(page.getByRole('heading', { name: 'Loot Test Char', level: 2 })).toBeVisible({ timeout: 10000 });
    // One call, not two: the sheet's tabs are flat now, so Inventory is a tab
    // rather than a module with an Items sub-tab.
    await sheetPage.goToInventoryTab();

    await expect(page.getByRole('heading', { name: ROLLED_ITEM })).toBeVisible({ timeout: 10000 });
  });
});
