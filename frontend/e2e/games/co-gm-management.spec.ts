import { test, expect } from '@playwright/test';
import { loginAs } from '../fixtures/auth-helpers';
import {
  getWorkerGameId,
  getWorkerUsername,
  getParticipantUserId,
  getFixtureGameId,
  promoteToCoGM,
  demoteFromCoGM,
  demoteCurrentCoGM,
} from '../fixtures/game-helpers';
import { AudiencePage } from '../pages/AudiencePage';
import { GameDetailsPage } from '../pages/GameDetailsPage';
import { MessagingPage } from '../pages/MessagingPage';
import { PhaseManagementPage } from '../pages/PhaseManagementPage';
import { navigateToGameTab, assertTabVisible, assertTabNotVisible } from '../utils/navigation';

/**
 * Co-GM Management E2E Tests
 *
 * Tests the full co-GM lifecycle:
 * 1. Primary GM promotes audience member to co-GM
 * 2. Co-GM has GM permissions (can manage phases, view actions, etc.)
 * 3. Co-GM cannot edit game settings or promote others
 * 4. Primary GM demotes co-GM back to audience
 * 5. Only one co-GM allowed per game
 *
 * Uses worker-specific game ID 339 for testing.
 *
 * Structure:
 * - "UI Lifecycle" group: tests the promotion/demotion UI flow (serial — depends on state)
 * - "Functional Capabilities" group: tests what a co-GM can do (each test sets up via API)
 */

const gameId = getWorkerGameId(339);
const audience2Username = getWorkerUsername('TestAudience2');

// ============================================================================
// GROUP 1: UI Lifecycle (promote/demote flow through the interface)
// ============================================================================
// These tests validate the actual promotion/demotion UI flow.
// They run serially because each test depends on the state left by the previous one.

test.describe.serial('@mobile Co-GM Management — UI Lifecycle', () => {
  // Start with a clean slate: ensure no co-GM exists (uses API so no "Setup" test needed)
  test.beforeAll(async ({ browser }) => {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();
    try {
      await loginAs(page, 'GM');
      // Demote whoever is currently co-GM (if any) so the first UI test starts clean
      await demoteCurrentCoGM(page, gameId);
    } finally {
      await ctx.close();
    }
  });

  test('Primary GM can promote audience member to co-GM', async ({ page }) => {
    await loginAs(page, 'GM');

    const gamePage = new GameDetailsPage(page);
    await gamePage.goto(gameId);

    await navigateToGameTab(page, 'People');
    await page.getByRole('button', { name: /Game Participants/ }).click();

    await expect(page.getByRole('heading', { name: /Audience/ })).toBeVisible();

    const audience2Card = page.getByTestId('participant-card').filter({ hasText: audience2Username });
    await audience2Card.getByRole('button', { name: 'Participant actions' }).click();

    const promoteMenuItem = page.getByRole('menuitem', { name: 'Promote to Co-GM' });
    await promoteMenuItem.waitFor({ state: 'visible', timeout: 5000 });
    await promoteMenuItem.click();

    await expect(page.getByRole('heading', { name: 'Promote to Co-GM?' })).toBeVisible();
    await expect(page.getByText(/Co-GMs can do everything you can except/)).toBeVisible();
    await page.getByRole('button', { name: 'Promote to Co-GM' }).click();

    await page.waitForLoadState('networkidle');

    await expect(page.getByRole('heading', { name: 'Promote to Co-GM?' })).not.toBeVisible({ timeout: 10000 });
    await expect(page.getByRole('heading', { name: /Co-GMs/ })).toBeVisible();
  });

  test('Co-GM appears in game header', async ({ page }) => {
    await loginAs(page, 'GM');

    const gamePage = new GameDetailsPage(page);
    await gamePage.goto(gameId);

    await page.waitForLoadState('networkidle');

    await expect(page.getByText(/Co-GM:/).locator('visible=true').first()).toBeVisible({ timeout: 10000 });
    await expect(page.getByText(new RegExp(`Co-GM: ${audience2Username}`)).locator('visible=true').first()).toBeVisible({ timeout: 10000 });
  });

  test('Co-GM can access GM features (phase management)', async ({ page }) => {
    await loginAs(page, 'AUDIENCE_2');

    const gamePage = new GameDetailsPage(page);
    await gamePage.goto(gameId);

    await assertTabVisible(page, 'Phases');

    await navigateToGameTab(page, 'Phases');
    await expect(page.getByRole('heading', { name: 'Phase Management' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'New Phase' })).toBeVisible();
  });

  test('Co-GM cannot edit game settings', async ({ page }) => {
    await loginAs(page, 'AUDIENCE_2');

    const gamePage = new GameDetailsPage(page);
    await gamePage.goto(gameId);

    await expect(page.getByRole('button', { name: 'Edit Game' })).not.toBeVisible();
  });

  test('Co-GM cannot promote others to co-GM', async ({ page }) => {
    await loginAs(page, 'AUDIENCE_2');

    const gamePage = new GameDetailsPage(page);
    await gamePage.goto(gameId);

    await navigateToGameTab(page, 'People');
    await page.getByRole('button', { name: /Game Participants/ }).click();

    // At least one audience member exists in this game (audience1 is a known participant)
    const actionsButton = page.getByRole('button', { name: 'Participant actions' }).first();
    await expect(actionsButton).toBeVisible({ timeout: 5000 });
    await actionsButton.click();

    // Co-GMs cannot promote others — the promote menu item must not appear
    await expect(page.getByRole('menuitem', { name: 'Promote to Co-GM' })).not.toBeVisible();
  });

  test('Primary GM can demote co-GM back to audience', async ({ page }) => {
    await loginAs(page, 'GM');

    const gamePage = new GameDetailsPage(page);
    await gamePage.goto(gameId);

    await navigateToGameTab(page, 'People');
    await page.getByRole('button', { name: /Game Participants/ }).click();

    await expect(page.getByRole('heading', { name: /Co-GMs/ })).toBeVisible({ timeout: 10000 });

    const coGmSection = page.locator('div:has(> h3:has-text("Co-GMs"))').first();
    const coGmActionsButton = coGmSection.getByRole('button', { name: 'Participant actions' }).first();
    await coGmActionsButton.click();

    await page.getByRole('menuitem', { name: 'Demote from Co-GM' }).click();

    await expect(page.getByRole('heading', { name: 'Demote from Co-GM?' })).toBeVisible();
    await page.getByRole('button', { name: 'Demote to Audience' }).click();

    await page.waitForLoadState('networkidle');

    await expect(page.getByRole('heading', { name: 'Demote from Co-GM?' })).not.toBeVisible();
    await expect(page.locator('h3:has-text("Co-GMs")')).not.toBeVisible();
  });

  test('Demoted co-GM loses GM permissions', async ({ page }) => {
    await loginAs(page, 'AUDIENCE_2');

    const gamePage = new GameDetailsPage(page);
    await gamePage.goto(gameId);

    await assertTabNotVisible(page, 'Phases');
    await assertTabVisible(page, 'People');
    await assertTabVisible(page, 'Audience');
  });

  test('Co-GM badge removed from header after demotion', async ({ page }) => {
    await loginAs(page, 'GM');

    const gamePage = new GameDetailsPage(page);
    await gamePage.goto(gameId);

    await expect(page.getByText(/Co-GM:/)).not.toBeVisible();
  });
});

// ============================================================================
// GROUP 2: Functional Capabilities (what a co-GM can actually do)
// ============================================================================
// Each test is independent: beforeEach promotes via API, afterEach demotes via API.
// These tests run in parallel (no serial constraint).

test.describe('Co-GM Management — Functional Capabilities', () => {
  let audience2UserId: number;

  test.beforeAll(async ({ browser }) => {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();
    try {
      await loginAs(page, 'GM');
      audience2UserId = await getParticipantUserId(page, gameId, audience2Username);
    } finally {
      await ctx.close();
    }
  });

  test.beforeEach(async ({ browser }) => {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();
    try {
      await loginAs(page, 'GM');
      // Demote whoever is currently co-GM (could be audience1 left over from UI Lifecycle group)
      // then promote audience2 fresh for this test
      await demoteCurrentCoGM(page, gameId);
      await promoteToCoGM(page, gameId, audience2UserId);
    } finally {
      await ctx.close();
    }
  });

  test.afterEach(async ({ browser }) => {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();
    try {
      await loginAs(page, 'GM');
      await demoteFromCoGM(page, gameId, audience2UserId);
    } finally {
      await ctx.close();
    }
  });

  test('Co-GM can create a new phase', async ({ page }) => {
    await loginAs(page, 'AUDIENCE_2');

    const gamePage = new GameDetailsPage(page);
    await gamePage.goto(gameId);

    await assertTabVisible(page, 'Phases');
    await navigateToGameTab(page, 'Phases');

    const phasePage = new PhaseManagementPage(page);
    await phasePage.createPhase({
      type: 'action',
      title: 'Co-GM Test Phase',
      description: 'Phase created by co-GM to test functionality',
    });

    await expect(page.getByRole('heading', { name: 'Co-GM Test Phase' }).first()).toBeVisible();
  });

  test('Co-GM can edit phase details', async ({ page }) => {
    await loginAs(page, 'AUDIENCE_2');

    const gamePage = new GameDetailsPage(page);
    await gamePage.goto(gameId);

    await navigateToGameTab(page, 'Phases');

    await expect(page.getByRole('button', { name: 'Edit phase details' }).first()).toBeVisible();
    await page.getByRole('button', { name: 'Edit phase details' }).first().click();
    await expect(page.getByRole('heading', { name: 'Edit Phase' })).toBeVisible();

    await page.getByRole('button', { name: 'Cancel' }).click();
  });

  test('Co-GM can send private messages as GM NPC', async ({ page }) => {
    await loginAs(page, 'AUDIENCE_2');

    const messagingPage = new MessagingPage(page);
    await messagingPage.goto(gameId);

    await messagingPage.navigateToMessages();

    await expect(messagingPage.newConversationButton).toBeVisible();
    await expect(messagingPage.newConversationButton).toBeEnabled();

    await messagingPage.openNewConversationForm();
    await expect(messagingPage.conversationTitleInput).toBeVisible();

    await page.keyboard.press('Escape');
  });

  test('Co-GM can view player actions on Actions tab', async ({ page }) => {
    // NOTE: This test activates Phase 2 as GM, which mutates shared game state.
    // 'Co-GM has full access to phase management' relies on a phase being active —
    // run this test first or ensure a phase is already active in the fixture.
    // First activate the action phase as GM
    await loginAs(page, 'GM');
    const gamePage = new GameDetailsPage(page);
    await gamePage.goto(gameId);

    await navigateToGameTab(page, 'Phases');

    const phaseCard = page.locator('div').filter({ hasText: 'Phase 2' }).filter({ has: page.getByRole('heading', { name: 'Test Phase 1', exact: true }) });
    await phaseCard.getByRole('button', { name: 'Activate', exact: true }).first().click();

    await expect(page.getByRole('heading', { name: 'Activate Phase 2?' })).toBeVisible();
    await page.getByRole('button', { name: 'Activate Phase' }).click();
    await page.waitForLoadState('networkidle');
    await expect(page.getByRole('heading', { name: 'Activate Phase 2?' })).not.toBeVisible();

    // Now check as co-GM
    await loginAs(page, 'AUDIENCE_2');
    await gamePage.goto(gameId);

    await navigateToGameTab(page, 'Actions');
    await expect(page.getByRole('heading', { name: 'Submitted Actions' })).toBeVisible();
    await expect(page.getByText(/\d+ Actions?/).first()).toBeVisible();
  });

  test('Co-GM has full access to phase management', async ({ page }) => {
    await loginAs(page, 'AUDIENCE_2');

    const gamePage = new GameDetailsPage(page);
    await gamePage.goto(gameId);

    await navigateToGameTab(page, 'Phases');

    await expect(page.getByRole('heading', { name: 'Phase Management' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'New Phase' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Test Phase 1' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Currently Active' })).toBeVisible();
  });

  test('Co-GM can create handouts', async ({ page }) => {
    await loginAs(page, 'AUDIENCE_2');

    const gamePage = new GameDetailsPage(page);
    await gamePage.goto(gameId);

    await navigateToGameTab(page, 'Handouts');

    const createButton = page.getByRole('button', { name: 'Create Handout' });
    await expect(createButton).toBeVisible();
    await createButton.click();

    await expect(page.getByRole('heading', { name: 'Create New Handout' })).toBeVisible();
    await expect(page.getByLabel('Title')).toBeVisible();
    await expect(page.getByTestId('handout-content-input')).toBeVisible();
    await expect(page.getByLabel('Status')).toBeVisible();

    await page.getByLabel('Status').selectOption('published');
    await expect(page.getByLabel('Status')).toHaveValue('published');

    await page.getByRole('button', { name: 'Cancel' }).click();
  });

  test('Co-GM can manage participants', async ({ page }) => {
    await loginAs(page, 'AUDIENCE_2');

    const gamePage = new GameDetailsPage(page);
    await gamePage.goto(gameId);

    await navigateToGameTab(page, 'People');
    await page.getByRole('button', { name: /Game Participants/ }).click();

    await expect(page.getByRole('heading', { name: /Audience/i })).toBeVisible();

    // Co-GM should see participant action buttons (same as primary GM)
    await expect(page.getByRole('button', { name: 'Participant actions' }).first()).toBeVisible();
  });

  test('Co-GM can create NPCs', async ({ page }) => {
    await loginAs(page, 'AUDIENCE_2');

    const gamePage = new GameDetailsPage(page);
    await gamePage.goto(gameId);

    await navigateToGameTab(page, 'People');
    await page.getByRole('button', { name: 'Characters' }).click();

    const createCharacterButton = page.getByRole('button', { name: 'Create Character' });
    await expect(createCharacterButton).toBeVisible();
    await createCharacterButton.click();

    const characterForm = page.getByTestId('character-form');
    await expect(characterForm).toBeVisible();

    const npcName = `Co-GM Test NPC ${Date.now()}`;
    await page.getByTestId('character-name-input').fill(npcName);
    await page.getByLabel('Character Type').selectOption('npc');

    const submitButton = page.getByTestId('character-submit-button');
    await expect(submitButton).toBeEnabled();
    await submitButton.click();

    await expect(characterForm).toBeHidden({ timeout: 5000 });
    await page.waitForLoadState('networkidle');

    await expect(page.getByText(npcName).locator('visible=true').first()).toBeVisible({ timeout: 5000 });
  });

  test('Co-GM can access Audience tab', async ({ page }) => {
    await loginAs(page, 'AUDIENCE_2');

    const gamePage = new GameDetailsPage(page);
    await gamePage.goto(gameId);

    await assertTabVisible(page, 'Audience');

    // The Audience tab renders the "All Private Messages" view directly. It no
    // longer has "Private Messages"/"Action Submissions" sub-tabs — action
    // submissions moved to the History tab for every role (see AudienceView).
    const audiencePage = new AudiencePage(page);
    await audiencePage.goToAudience(gameId);
    await audiencePage.verifyAllPrivateMessagesView();
  });

  test('Co-GM can edit Action Results', async ({ page, browser }) => {
    // Uses a dedicated fixture (game 349) that has an active action phase from the start,
    // so this test never mutates the shared CO_GM_MANAGEMENT game state.
    const actionGameId = await (async () => {
      const ctx = await browser.newContext();
      const p = await ctx.newPage();
      try {
        await loginAs(p, 'GM');
        const id = await getFixtureGameId(p, 'CO_GM_ACTION_RESULTS');
        // Ensure audience2 is co-GM in this game too
        await demoteCurrentCoGM(p, id);
        const uid = await getParticipantUserId(p, id, audience2Username);
        await promoteToCoGM(p, id, uid);
        return id;
      } finally {
        await ctx.close();
      }
    })();

    await loginAs(page, 'AUDIENCE_2');
    const gamePage = new GameDetailsPage(page);
    await gamePage.goto(actionGameId);

    await navigateToGameTab(page, 'Actions');
    await expect(page.getByRole('heading', { name: 'Action Results', exact: true })).toBeVisible({ timeout: 10000 });
  });

  test('Co-GM can access character management', async ({ page }) => {
    await loginAs(page, 'AUDIENCE_2');
    const gamePage = new GameDetailsPage(page);
    await gamePage.goto(gameId);

    await navigateToGameTab(page, 'People');
    await page.getByRole('button', { name: 'Characters' }).click();

    await expect(page.getByRole('heading', { name: 'Characters', exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Create Character' })).toBeVisible();
  });
});
