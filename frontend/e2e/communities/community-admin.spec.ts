import { test, expect } from '@playwright/test';
import { loginAs } from '../fixtures/auth-helpers';
import { getWorkerUsername, getWorkerIndex } from '../fixtures/game-helpers';
import {
  COMMUNITY_NAMES,
  getCommunitySlug,
  getCommunityBySlug,
} from '../fixtures/community-helpers';
import { AdminCommunitiesPage } from '../pages/AdminCommunitiesPage';
import { LONG_TIMEOUT } from '../config/test-timeouts';

/**
 * E2E Tests for Site-Admin Community Administration
 *
 * Covers requirement 1: a site admin can create a community.
 *
 * Creating a community is deliberately NOT a self-serve action -- it lives on
 * /admin, behind the site-admin check, because a community carries moderation
 * powers over other people's games. The negative case matters as much as the
 * positive one, so the non-admin path is asserted too.
 *
 * TestGM is the fixture's site admin (see admin/admin-mode.spec.ts).
 */

test.describe('Community Administration (site admin)', () => {
  test('site admin can create a community and it becomes browsable', async ({ page }) => {
    await loginAs(page, 'GM');

    // Unique per worker AND per run: the slug carries a UNIQUE constraint, so a
    // fixed value would collide with the previous run's row on the second run.
    const stamp = `${getWorkerIndex()}-${Date.now()}`;
    const name = `E2E Created Community ${stamp}`;
    const slug = `e2e-created-${stamp}`;
    const owner = getWorkerUsername('TestPlayer2');

    const admin = new AdminCommunitiesPage(page);
    await admin.goto();
    await admin.createCommunity(name, slug, owner);

    // The new community appears in the admin list, owned by the chosen user.
    const row = admin.rowByName(name);
    await expect(row).toContainText(`/${slug}`);
    await expect(row).toContainText(owner);
    await expect(row).toContainText('Active');

    // And it is reachable at its own address, which is the point of creating it.
    await page.goto(`/communities/${slug}`);
    await page.waitForLoadState('networkidle');
    await expect(page.getByRole('heading', { name, level: 1 })).toBeVisible({
      timeout: LONG_TIMEOUT,
    });
  });

  test('created community assigns ownership to the chosen user, not the admin', async ({
    page,
  }) => {
    // Ownership is what grants the Manage button and the moderator roster. If
    // creation silently kept the admin as owner, the owner would be locked out
    // of their own community and only a site admin could fix it.
    await loginAs(page, 'GM');

    const stamp = `${getWorkerIndex()}-${Date.now()}`;
    const name = `E2E Ownership Community ${stamp}`;
    const slug = `e2e-ownership-${stamp}`;
    const owner = getWorkerUsername('TestPlayer3');

    const admin = new AdminCommunitiesPage(page);
    await admin.goto();
    await admin.createCommunity(name, slug, owner);

    // The server computes your_role per request; TestPlayer3 must read as owner.
    await loginAs(page, 'PLAYER_3');
    await page.goto(`/communities/${slug}`);
    await page.waitForLoadState('networkidle');

    const community = await getCommunityBySlug(page, slug);
    expect(community.your_role).toBe('owner');
    expect(community.owner_username).toBe(owner);

    // ...and the UI grants them the Manage button on the strength of it.
    await expect(page.getByTestId('manage-community')).toBeVisible({ timeout: LONG_TIMEOUT });
  });

  test('non-admin is redirected away from the admin panel entirely', async ({ page }) => {
    // TestPlayer1 moderates Midnight Ravens but is not a site admin: moderating
    // a community must not imply the power to mint new ones.
    await loginAs(page, 'PLAYER_1');

    const admin = new AdminCommunitiesPage(page);
    await admin.goto();

    // Assert the REDIRECT first, not merely that the form is missing. A bare
    // toBeHidden() passes just as happily on a page that 500'd, failed to
    // render, or bounced to /login -- none of which prove the admin gate did
    // its job. Landing somewhere else, still signed in, is the positive
    // evidence that a refusal is what hid the form.
    //
    // The destination is /dashboard, via two hops: ProtectedRoute sends a
    // non-admin to '/', and HomePage bounces an authenticated user on to
    // /dashboard. Asserting '/' alone fails -- nobody authenticated ever comes
    // to rest there.
    await expect(page).toHaveURL(/\/dashboard$/, { timeout: LONG_TIMEOUT });
    expect(page.url()).not.toContain('/admin');

    // ...and refused for lack of ADMIN, not for lack of a session -- an expired
    // login would also clear the form, and would also leave /admin. Asserted
    // through the API rather than a nav element: the user menu is `hidden
    // md:block`, so a DOM check would silently mean nothing on the mobile
    // project this spec also runs in.
    const me = await page.evaluate(async () => {
      const response = await fetch('/api/v1/auth/me', { credentials: 'include' });
      return { ok: response.ok, body: response.ok ? await response.json() : null };
    });
    expect(me.ok).toBe(true);
    expect(me.body.is_admin).toBe(false);

    // And with that established, the creation controls are genuinely absent.
    await expect(admin.nameInput).toBeHidden();
    await expect(admin.createButton).toBeHidden();
  });

  test('anyone signed in can browse the communities list', async ({ page }) => {
    // Browsing is open -- membership is not a roster, so the list is public to
    // authenticated users even for communities they have no role in.
    await loginAs(page, 'PLAYER_4');

    await page.goto('/communities');
    await page.waitForLoadState('networkidle');

    await expect(page.getByTestId('communities-list')).toBeVisible({ timeout: LONG_TIMEOUT });

    // community-card-{slug} is the anchor stretched over the card, not the card
    // itself -- it has no text of its own, so the name is asserted through its
    // aria-label. Names are shared across workers; the slug disambiguates.
    const ravensCard = page.getByTestId(`community-card-${getCommunitySlug('RAVENS')}`);
    await expect(ravensCard).toBeVisible();
    await expect(ravensCard).toHaveAttribute('aria-label', COMMUNITY_NAMES.RAVENS);

    await expect(
      page.getByTestId(`community-card-${getCommunitySlug('HARBOR')}`)
    ).toBeVisible();
  });
});
