import { test, expect } from '@playwright/test';
import { loginAs } from '../fixtures/auth-helpers';
import { getWorkerUsername } from '../fixtures/game-helpers';
import { getCommunitySlug, getUserIdByUsername } from '../fixtures/community-helpers';
import { CommunityPage } from '../pages/CommunityPage';
import { LONG_TIMEOUT } from '../config/test-timeouts';

/**
 * E2E Tests for the Community Moderator Roster
 *
 * Covers requirement 2: the owner can add and remove moderators.
 *
 * The roster is the one power an owner does NOT share with their moderators:
 * a moderator can ban, write documents, and edit the profile, but cannot change
 * who else holds those powers. Both halves are asserted -- the owner's ability
 * and the moderator's inability -- because a permission check is only proven by
 * its refusal.
 *
 * Fixture state (01_communities_e2e_owned.sql): the E2E Roster Community is
 * owned by TestGM, with TestPlayer1 already a moderator.
 *
 * This spec has that community TO ITSELF. It promotes and demotes people, and
 * Playwright shards by file -- so doing that in a SHARED community meant another
 * spec could read the roster mid-change. It did: this file promoting TestPlayer4
 * raced another file asserting TestPlayer4 had no moderator controls.
 */

test.describe('Community Moderators', () => {
  test('owner can add and then remove a moderator', async ({ page }) => {
    await loginAs(page, 'GM');

    const community = new CommunityPage(page, getCommunitySlug('ROSTER'));
    await community.gotoManage('moderators');

    // TestAudience1 is not a moderator in this community and is not its owner --
    // both of which the picker would otherwise exclude.
    const target = getWorkerUsername('TestAudience1');
    const targetId = await getUserIdByUsername(page, target);

    await expect(community.moderatorList).toBeVisible({ timeout: LONG_TIMEOUT });
    expect(await community.hasModerator(targetId)).toBe(false);

    await community.addModerator(target);

    const row = page.getByTestId(`moderator-row-${targetId}`);
    await expect(row).toBeVisible({ timeout: LONG_TIMEOUT });
    await expect(row).toContainText(target);
    await expect(row).toContainText('Moderator');

    // Survives a reload: the roster is server state, not optimistic UI.
    await community.gotoManage('moderators');
    await expect(page.getByTestId(`moderator-row-${targetId}`)).toBeVisible({
      timeout: LONG_TIMEOUT,
    });

    // ...and now remove them again, returning the fixture to its original shape.
    await community.removeModerator(targetId);
    await expect(page.getByTestId(`moderator-row-${targetId}`)).toBeHidden({
      timeout: LONG_TIMEOUT,
    });

    await community.gotoManage('moderators');
    await expect(community.moderatorList).toBeVisible({ timeout: LONG_TIMEOUT });
    await expect(page.getByTestId(`moderator-row-${targetId}`)).toBeHidden();
  });

  test('roster lists the owner separately from the moderators', async ({ page }) => {
    // Ownership is not a moderator row -- the server's roster never contains
    // the owner, so the page renders them separately. If that were dropped, the
    // page would answer "who holds power here" incompletely.
    await loginAs(page, 'GM');

    const community = new CommunityPage(page, getCommunitySlug('ROSTER'));
    await community.gotoManage('moderators');

    await expect(community.moderatorList).toBeVisible({ timeout: LONG_TIMEOUT });
    await expect(community.moderatorList).toContainText(getWorkerUsername('TestGM'));
    await expect(community.moderatorList).toContainText('Owner');

    // The fixture moderator is present and labelled as a moderator, not owner.
    const moderatorId = await getUserIdByUsername(page, getWorkerUsername('TestPlayer1'));
    const row = page.getByTestId(`moderator-row-${moderatorId}`);
    await expect(row).toBeVisible();
    await expect(row).toContainText('Moderator');
  });

  test('a moderator sees the roster but cannot change it', async ({ page }) => {
    // TestPlayer1 moderates this community. They reach the manage page for the
    // tabs they can act on, so the page must load -- only the roster CONTROLS
    // are withheld.
    await loginAs(page, 'PLAYER_1');

    const community = new CommunityPage(page, getCommunitySlug('ROSTER'));
    await community.gotoManage('moderators');

    await expect(community.moderatorList).toBeVisible({ timeout: LONG_TIMEOUT });

    // No add form...
    await expect(community.moderatorSearch).toBeHidden();
    await expect(community.addModeratorSubmit).toBeHidden();

    // ...and no Remove button on any row, including their own.
    const selfId = await getUserIdByUsername(page, getWorkerUsername('TestPlayer1'));
    await expect(page.getByTestId(`moderator-row-${selfId}`)).toBeVisible();
    await expect(page.getByTestId(`remove-moderator-${selfId}`)).toBeHidden();
  });

  test('an ordinary member sees the roster but gets no controls over it', async ({ page }) => {
    // TestAudience2 has no role in this community. The Manage button is absent
    // from the community page, and the manage route withholds every control.
    await loginAs(page, 'AUDIENCE_2');

    const slug = getCommunitySlug('ROSTER');
    const community = new CommunityPage(page, slug);
    await community.goto();

    // goto() waits for the community's <h1>, so the Manage button being absent
    // here is a real refusal rather than a page that never rendered.
    await expect(community.manageButton).toBeHidden();

    // The manage route itself is open to any signed-in user -- permissions are
    // decided per tab, not at the route -- and gotoManage() waits for the shell
    // heading. So the missing controls below are withheld, not merely unloaded.
    await community.gotoManage('moderators');

    await expect(community.moderatorSearch).toBeHidden();
    await expect(community.addModeratorSubmit).toBeHidden();
  });
});
