import { test, expect } from '@playwright/test';
import { loginAs } from '../fixtures/auth-helpers';
import { getWorkerUsername, getWorkerIndex } from '../fixtures/game-helpers';
import {
  COMMUNITY_NAMES,
  getCommunitySlug,
  getUserIdByUsername,
} from '../fixtures/community-helpers';
import { CommunityPage } from '../pages/CommunityPage';
import { LONG_TIMEOUT } from '../config/test-timeouts';

/**
 * E2E Tests for Community Moderation Tools
 *
 * Covers requirement 3: an owner or moderator can ban users, write documents,
 * and edit settings.
 *
 * These are the upkeep powers -- the tier BELOW roster management. A moderator
 * must be able to do all of them, so every flow here is driven as TestPlayer1
 * (a moderator, not the owner); driving them as the owner would leave the
 * moderator tier untested.
 *
 * This spec owns the E2E Mod Tools Community outright
 * (01_communities_e2e_owned.sql). Everything below mutates it -- issuing bans,
 * publishing documents, RENAMING it -- and Playwright shards by file, so a
 * shared community would let another spec observe those changes mid-flight.
 * It did: renaming Midnight Ravens here raced a spec asserting its card still
 * read "Midnight Ravens". Restoring state afterwards cannot close that window;
 * owning the fixture can.
 */

test.describe('Community Moderation', () => {
  test.describe('Bans', () => {
    // Lifting the ban is part of what this test ASSERTS, but it is also what
    // returns the fixture to its starting shape. In the test body alone, a
    // failure on any earlier assertion skips it and leaves TestAudience banned
    // -- so the next run starts dirty and fails somewhere else entirely. An
    // afterEach runs either way, which is the whole point of per-spec fixture
    // ownership being re-runnable rather than one-shot.
    test.afterEach(async ({ page }) => {
      // Best-effort, silent, and BOUNDED: this exists to stop ONE failure
      // becoming a dirty fixture, so it must never fail the run, and must never
      // outlive the budget of the test it is tidying up after.
      //
      // The bound is the part that matters. When the page crashed mid-test, an
      // unbounded cleanup sat waiting on that dead page until Playwright
      // reported "Test timeout exceeded while running afterEach hook" -- which
      // REPLACED the real error and sent the reader to the cleanup code instead
      // of the crash. A cleanup that hides the failure it follows is worse than
      // no cleanup, so this races a short timer and gives up quietly.
      if (page.isClosed()) return;

      const cleanup = (async () => {
        const targetId = await getUserIdByUsername(
          page,
          getWorkerUsername('TestAudience')
        );

        await page.evaluate(
          async ({ communitySlug, userId }) => {
            await fetch(`/api/v1/communities/${communitySlug}/bans/${userId}`, {
              method: 'DELETE',
              credentials: 'include',
            });
          },
          { communitySlug: getCommunitySlug('MODTOOLS'), userId: targetId }
        );
      })();

      await Promise.race([
        cleanup.catch(() => undefined),
        new Promise((resolve) => setTimeout(resolve, 5000)),
      ]);
    });

    test('moderator can ban a user and then lift the ban', async ({ page }) => {
      await loginAs(page, 'PLAYER_1');

      const community = new CommunityPage(page, getCommunitySlug('MODTOOLS'));
      await community.gotoManage('bans');

      // TestAudience has no ban in this community, so they start clean and can
      // be banned without disturbing the expiry and audit scenarios below.
      const target = getWorkerUsername('TestAudience');
      const targetId = await getUserIdByUsername(page, target);

      await expect(community.banForm).toBeVisible({ timeout: LONG_TIMEOUT });
      expect(await community.getBanStatus(targetId)).toBeNull();

      await community.banUser(target, 'E2E: disruptive behaviour in the common room');

      const row = page.getByTestId(`ban-row-${targetId}`);
      await expect(row).toBeVisible({ timeout: LONG_TIMEOUT });
      await expect(row).toContainText(target);
      await expect(row).toContainText('E2E: disruptive behaviour in the common room');
      expect(await community.getBanStatus(targetId)).toBe('Banned');

      // Lifting the ban is asserted here as behaviour in its own right; the
      // afterEach above is the safety net for when we never reach this line.
      await community.unbanUser(targetId);
      await expect(page.getByTestId(`ban-row-${targetId}`)).toBeHidden({ timeout: LONG_TIMEOUT });

      await community.gotoManage('bans');
      await expect(community.banForm).toBeVisible({ timeout: LONG_TIMEOUT });
      expect(await community.getBanStatus(targetId)).toBeNull();
    });

    test('an expired ban is listed but reads as Expired, not Banned', async ({ page }) => {
      // The guard for "a row's presence never means banned". TestPlayer4 has a
      // ban row here whose expiry has passed; it must still be visible to the
      // moderator while enforcing nothing.
      await loginAs(page, 'PLAYER_1');

      const community = new CommunityPage(page, getCommunitySlug('MODTOOLS'));
      await community.gotoManage('bans');

      const expiredId = await getUserIdByUsername(page, getWorkerUsername('TestPlayer4'));
      await expect(page.getByTestId(`ban-row-${expiredId}`)).toBeVisible({ timeout: LONG_TIMEOUT });
      expect(await community.getBanStatus(expiredId)).toBe('Expired');

      // While the permanent ban on TestPlayer5 in the same list reads as active.
      const activeId = await getUserIdByUsername(page, getWorkerUsername('TestPlayer5'));
      expect(await community.getBanStatus(activeId)).toBe('Banned');
    });

    test('ban history keeps lifted bans that the ban list no longer shows', async ({
      page,
    }) => {
      // The audit log is deliberately separate: lifting a ban clears the ban
      // list row but must NOT erase the record. TestPlayer3 was banned,
      // extended, then unbanned, and so appears only in the history.
      await loginAs(page, 'PLAYER_1');

      const community = new CommunityPage(page, getCommunitySlug('MODTOOLS'));

      const liftedId = await getUserIdByUsername(page, getWorkerUsername('TestPlayer3'));

      await community.gotoManage('bans');
      await expect(community.banForm).toBeVisible({ timeout: LONG_TIMEOUT });
      await expect(page.getByTestId(`ban-row-${liftedId}`)).toBeHidden();

      await community.gotoManage('history');
      await expect(community.banEventList).toBeVisible({ timeout: LONG_TIMEOUT });

      const history = community.banEventList;
      await expect(history).toContainText(getWorkerUsername('TestPlayer3'));
      await expect(history).toContainText('Appeal accepted; matter resolved.');
    });

    test('an ordinary member cannot see who is banned', async ({ page }) => {
      // The ban list names people and gives reasons -- moderators only.
      await loginAs(page, 'PLAYER_4');

      const community = new CommunityPage(page, getCommunitySlug('MODTOOLS'));
      await community.gotoManage('bans');

      // The refusal notice is the anchor: it proves the tab RENDERED and chose
      // to withhold the list, rather than the page having failed to load (which
      // would satisfy the toBeHidden() assertions below just as well).
      await expect(page.getByText('Only this community')).toBeVisible({ timeout: LONG_TIMEOUT });

      await expect(community.banForm).toBeHidden();
      await expect(community.banList).toBeHidden();
    });
  });

  test.describe('Documents', () => {
    test('moderator can create a draft and publish it to the community page', async ({
      page,
    }) => {
      await loginAs(page, 'PLAYER_1');

      const community = new CommunityPage(page, getCommunitySlug('MODTOOLS'));
      await community.gotoManage('documents');

      // Unique per run: documents are created, not reset between runs, so a
      // fixed title would match rows left by earlier runs.
      const title = `E2E House Rules ${getWorkerIndex()}-${Date.now()}`;

      await community.createDocument(title, 'Be **kind** to each other.', false);

      const row = community.documentRowByTitle(title);
      await expect(row).toBeVisible({ timeout: LONG_TIMEOUT });
      // New documents start as drafts -- publishing is deliberate.
      await expect(row).toContainText('Draft');

      // A draft is invisible on the public page. Asserted against the page body
      // rather than the documents list: the Documents section is not rendered
      // at all when nothing is published, so the list may not exist to query.
      await community.goto();
      await expect(page.locator('body')).not.toContainText(title);

      // Publish it, and it appears.
      await community.gotoManage('documents');
      await community.documentRowByTitle(title).getByTestId(/^toggle-publish-/).click();
      await page.waitForLoadState('networkidle');
      await expect(community.documentRowByTitle(title)).toContainText('Published', {
        timeout: LONG_TIMEOUT,
      });

      await community.goto();
      await expect(page.getByTestId('community-documents')).toContainText(title, {
        timeout: LONG_TIMEOUT,
      });
    });

    test('a member sees published documents but no editing controls', async ({ page }) => {
      // Both halves, because the name promises both. Asserting only that the
      // controls are hidden would pass just as well if documents had been made
      // invisible to members altogether -- which is the likelier bug when a
      // permission check is tightened, and the one a member would actually
      // notice.
      await loginAs(page, 'PLAYER_4');

      const community = new CommunityPage(page, getCommunitySlug('MODTOOLS'));

      // The read target is a PUBLISHED document seeded by the fixture
      // (01_communities_e2e_owned.sql), not one this test publishes for itself.
      // Publishing it here would mean a moderator login purely as setup, and
      // that extra login was where this test kept timing out under parallel
      // load -- failing in its scaffolding rather than its subject. The
      // moderator-side publish flow is covered by the test above, which owns it.
      const fixtureDocTitle = 'E2E Fixture House Rules';

      // The published document IS visible on the public page...
      await community.goto();
      await expect(page.getByTestId('community-documents')).toContainText(
        fixtureDocTitle,
        { timeout: LONG_TIMEOUT }
      );

      // ...while the management tab renders its refusal and withholds every
      // editing control. The notice anchors the negative assertions to a page
      // that actually loaded.
      await community.gotoManage('documents');
      await expect(page.getByText("Only this community's moderators")).toBeVisible({
        timeout: LONG_TIMEOUT,
      });
      await expect(community.newDocumentButton).toBeHidden();
      await expect(community.documentList).toBeHidden();
    });
  });

  test.describe('Settings', () => {
    // The rename is restored in the test body too, where it doubles as proof
    // that a second rename takes effect. This is the safety net: a failure
    // between the two renames would otherwise leave the community carrying a
    // timestamped name forever, and every later run comparing against
    // COMMUNITY_NAMES.MODTOOLS would fail for a reason unrelated to its own
    // assertions. Silent and best-effort -- it must not mask the real failure.
    test.afterEach(async ({ page }) => {
      // Bounded for the same reason as the ban cleanup above: an unbounded wait
      // on a crashed page replaces the test's real error with a timeout raised
      // inside this hook.
      if (page.isClosed()) return;

      const restore = page
        .evaluate(
          async ({ slug, name }) => {
            await fetch(`/api/v1/communities/${slug}`, {
              method: 'PATCH',
              credentials: 'include',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ name }),
            });
          },
          { slug: getCommunitySlug('MODTOOLS'), name: COMMUNITY_NAMES.MODTOOLS }
        )
        .catch(() => undefined);

      await Promise.race([
        restore,
        new Promise((resolve) => setTimeout(resolve, 5000)),
      ]);
    });

    test('moderator can edit the community name', async ({ page }) => {
      await loginAs(page, 'PLAYER_1');

      const slug = getCommunitySlug('MODTOOLS');
      const community = new CommunityPage(page, slug);
      await community.gotoManage('settings');

      await expect(community.settingsForm).toBeVisible({ timeout: LONG_TIMEOUT });

      const renamed = `E2E Mod Tools Renamed ${Date.now()}`;
      await community.renameCommunity(renamed);

      // The new name reaches the public page...
      await community.goto();
      await expect(page.getByRole('heading', { name: renamed, level: 1 })).toBeVisible({
        timeout: LONG_TIMEOUT,
      });

      // ...at the SAME address. The slug is immutable precisely so that links
      // shared elsewhere keep working through a rename.
      expect(page.url()).toContain(`/communities/${slug}`);

      // Restore the fixture name -- asserted here as a second successful
      // rename, and backstopped by the afterEach above when we never arrive.
      await community.gotoManage('settings');
      await community.renameCommunity(COMMUNITY_NAMES.MODTOOLS);
      await community.goto();
      await expect(
        page.getByRole('heading', { name: COMMUNITY_NAMES.MODTOOLS, level: 1 })
      ).toBeVisible({ timeout: LONG_TIMEOUT });
    });

    test('an ordinary member cannot edit settings', async ({ page }) => {
      await loginAs(page, 'PLAYER_4');

      const community = new CommunityPage(page, getCommunitySlug('MODTOOLS'));
      await community.gotoManage('settings');

      // gotoManage() has already waited for the shell heading, so reaching here
      // means the page rendered and the settings form specifically was withheld
      // -- rather than nothing having rendered at all.
      await expect(community.settingsForm).toBeHidden();
      await expect(community.settingsSave).toBeHidden();
    });
  });
});
