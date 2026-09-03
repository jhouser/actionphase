import { test, expect } from '@playwright/test';
import { loginAs } from '../fixtures/auth-helpers';
import { getFixtureGameId } from '../fixtures/game-helpers';
import { getCommunitySlug, getCommunityBySlug } from '../fixtures/community-helpers';
import { GameApplicationsPage } from '../pages/GameApplicationsPage';
import type { Page } from '@playwright/test';
import { LONG_TIMEOUT } from '../config/test-timeouts';

/**
 * E2E Tests for Community Ban Enforcement
 *
 * Covers requirement 4: a banned player cannot join a game belonging to the
 * community they are banned from.
 *
 * This is the whole point of a ban, and it is the case most likely to break
 * quietly: the enforcement queries inner-join through games.community_id, so a
 * refactor that loses the join fails OPEN -- the banned user simply gets in and
 * nothing errors. So the positive controls matter as much as the refusal:
 *
 *   - TestPlayer5 is permanently banned from Midnight Ravens
 *   - ...but NOT from Harbor Lights, and must still play there (bans are scoped
 *     to one community, never to the site)
 *   - TestPlayer4's ban on Midnight Ravens has EXPIRED and must not block them,
 *     even though the ban row still exists
 *
 * Fixture layout: this worker's GM games belong to Midnight Ravens, except
 * game 811, which the backfill moves to Harbor Lights. Games 810-812 belong to
 * this spec alone (31_community_ban_enforcement.sql), so the withdrawals below
 * cannot disturb another spec's fixture.
 */

/**
 * Open a game page and wait for it to actually render.
 *
 * Deliberately not the GameDetailsPage page object: its goto() routes through
 * navigateToGame(), whose 5s wait for the heading is the first thing to give
 * out when six workers hit the Vite dev server at once. The failure then reads
 * as "game did not load" inside a ban test, pointing the reader at the ban
 * logic rather than at the dev server. Same navigation, a timeout budget that
 * suits parallel runs.
 */
async function openGame(page: Page, gameId: number) {
  await page.goto(`/games/${gameId}`);
  await page.waitForLoadState('networkidle');
  await expect(page.locator('h1, h2').first()).toBeVisible({ timeout: LONG_TIMEOUT });
}

/**
 * Withdraw any application this user already has on the game.
 *
 * The three positive-control tests below JOIN a game, so they leave an
 * application behind. Without this they would pass once and then fail on every
 * re-run against the same fixture load -- the Apply button is gone because the
 * user already applied, which looks identical to being blocked by a ban. That
 * is the worst possible failure mode for a ban test, so the starting state is
 * made explicit rather than assumed.
 */
async function clearExistingApplication(page: Page, gameId: number) {
  // Done through the API rather than the Withdraw button. The button is the
  // right thing to TEST elsewhere, but it is the wrong thing to rely on for
  // setup: it renders only after the application-status request resolves, so a
  // DOM check races that request and intermittently decides there is nothing to
  // withdraw. DELETE is idempotent here -- 404 simply means no application --
  // which makes the starting state deterministic instead of timing-dependent.
  await page.goto(`/games/${gameId}`);
  await page.waitForLoadState('networkidle');

  const result = await page.evaluate(async (id) => {
    const response = await fetch(`/api/v1/games/${id}/application`, {
      method: 'DELETE',
      credentials: 'include',
    });
    return { status: response.status, ok: response.ok };
  }, gameId);

  // 404 is the expected "there was nothing to withdraw" answer and is fine.
  // Anything else is NOT: an unchecked failure here leaves the old application
  // in place, the Apply button stays hidden, and the assertion further down
  // reads as "the ban blocked this user" -- diagnosing a ban bug that does not
  // exist. Failing loudly at the point of breakage is the whole reason this
  // helper goes through the API instead of the UI.
  if (!result.ok && result.status !== 404) {
    throw new Error(
      `clearExistingApplication: DELETE /games/${gameId}/application returned ` +
        `${result.status}; cannot establish a known starting state`
    );
  }

  // Reload so the UI reflects the cleared state; the Apply button does not
  // come back on its own after the application goes away.
  await page.goto(`/games/${gameId}`);
  await page.waitForLoadState('networkidle');
}

test.describe('Community Ban Enforcement', () => {
  test('banned player is refused when applying to a game in that community', async ({
    page,
  }) => {
    await loginAs(page, 'PLAYER_5');

    // A recruitment game belonging to Midnight Ravens, where PLAYER_5 is banned.
    const gameId = await getFixtureGameId(page, 'E2E_BAN_BLOCKED');
    await openGame(page, gameId);

    // The ban is not a filter -- the game is still browsable. It blocks joining.
    await expect(
      page.locator('text=E2E Test: Ban Enforcement - Blocked')
    ).toBeVisible({ timeout: LONG_TIMEOUT });

    const applications = new GameApplicationsPage(page, gameId);
    await applications.applyButton.click();

    const form = page.getByTestId('application-form');
    await expect(form).toBeVisible({ timeout: LONG_TIMEOUT });
    await page.getByTestId('application-message').fill('Let me in');
    await page.getByTestId('submit-application').click();

    // The refusal is shown in the form, which stays open -- the user is told
    // why rather than being bounced with a generic failure.
    await expect(page.getByTestId('application-error')).toContainText(
      /banned from this community/i,
      { timeout: LONG_TIMEOUT }
    );
    await expect(form).toBeVisible();
  });

  test('the same banned player can still join another community\'s game', async ({
    page,
  }) => {
    // The positive control for scope. Game 811 is moved to Harbor Lights by the
    // fixture backfill, and PLAYER_5 has no ban there. If this failed, a ban
    // would be acting as a site ban.
    await loginAs(page, 'PLAYER_5');

    const gameId = await getFixtureGameId(page, 'E2E_BAN_OTHER_COMMUNITY');

    // Confirm the premise rather than trusting it: this game must really belong
    // to a DIFFERENT community than the one the player is banned from.
    const harbor = await getCommunityBySlug(page, getCommunitySlug('HARBOR'));
    const ravens = await getCommunityBySlug(page, getCommunitySlug('RAVENS'));
    expect(harbor.id).not.toBe(ravens.id);

    const applications = new GameApplicationsPage(page, gameId);
    await openGame(page, gameId);
    await clearExistingApplication(page, gameId);

    expect(await applications.hasApplyButton()).toBe(true);
    await applications.submitApplication('Cross-community control', 'player');

    // submitApplication waits for the modal to close, which only happens on
    // success -- a refusal would have kept it open with an error.
    await openGame(page, gameId);
    expect(await applications.hasApplyButton()).toBe(false);
  });

  test('an expired ban does not block joining', async ({ page }) => {
    // TestPlayer4 still has a ban ROW in Midnight Ravens; its expiry has passed.
    // Expiry is evaluated at query time, so nobody has to lift it -- and a
    // presence check written instead of an is-active check would block here.
    await loginAs(page, 'PLAYER_4');

    const gameId = await getFixtureGameId(page, 'E2E_BAN_EXPIRED');
    await openGame(page, gameId);

    const applications = new GameApplicationsPage(page, gameId);
    await clearExistingApplication(page, gameId);

    expect(await applications.hasApplyButton()).toBe(true);
    await applications.submitApplication('Expired ban control', 'player');

    await openGame(page, gameId);
    expect(await applications.hasApplyButton()).toBe(false);
  });

  test('ban blocks joining as audience, not only as a player', async ({ page }) => {
    // Audience applications are exempt from the recruitment-state check, so it
    // would be easy for them to inherit that exemption and skip the ban check
    // too -- letting a banned user watch the community's games from the stands.
    await loginAs(page, 'PLAYER_5');

    const gameId = await getFixtureGameId(page, 'E2E_BAN_BLOCKED');
    await openGame(page, gameId);

    const applications = new GameApplicationsPage(page, gameId);
    await applications.applyButton.click();

    const form = page.getByTestId('application-form');
    await expect(form).toBeVisible({ timeout: LONG_TIMEOUT });
    await page.getByTestId('application-role-select').selectOption('audience');
    await page.getByTestId('application-message').fill('Just watching');
    await page.getByTestId('submit-application').click();

    await expect(page.getByTestId('application-error')).toContainText(
      /banned from this community/i,
      { timeout: LONG_TIMEOUT }
    );
  });

  test('the community page tells a banned user they are banned', async ({ page }) => {
    // is_banned is computed per request. The user should learn their status
    // from the community itself, not only by being refused at a game.
    await loginAs(page, 'PLAYER_5');

    const slug = getCommunitySlug('RAVENS');
    await page.goto(`/communities/${slug}`);
    await page.waitForLoadState('networkidle');

    const community = await getCommunityBySlug(page, slug);
    expect(community.is_banned).toBe(true);

    // A ban does not hide the community -- it is still browsable, which is how
    // a banned user reads the rules they fell foul of.
    await expect(page.getByRole('heading', { level: 1 })).toBeVisible({ timeout: LONG_TIMEOUT });
  });
});
