import { test, expect } from '@playwright/test';
import { loginAs } from '../fixtures/auth-helpers';
import {
  createStagedChainViaApi,
  getFixtureGameId,
  getParticipantUserId,
  getWorkerUsername,
} from '../fixtures/game-helpers';
import { GameDetailsPage } from '../pages/GameDetailsPage';

/**
 * E2E Tests for Staged (Timed Multi-Part) Result Reveals
 *
 * A GM splits one result into parts revealed on a timer, so a player cannot
 * skim ahead to find out whether they lived.
 *
 * ## Why these tests do not wait for a reveal
 *
 * The release worker ticks once a minute and the minimum delay is one minute,
 * so observing a real release takes up to ~2 minutes of wall clock. Waiting for
 * one would make this the slowest suite in the project and would flake whenever
 * a tick landed badly.
 *
 * Instead every test asserts against a state that already exists when the page
 * loads. The locked state is just data — a published part with released_at NULL
 * — so seeding a chain with a long delay produces it instantly and permanently.
 * Nothing here sleeps or polls.
 *
 * The countdown's arithmetic (mm:ss, "Unlocking…", refetch on zero) is covered
 * by component tests with mocked clocks, where it belongs. Re-testing it in a
 * browser would cost seconds per assertion and prove nothing extra.
 *
 * ## What these tests are, and are NOT, for
 *
 * They are NOT the leak guarantee. "The server must not blank-fail" is owned by
 * `TestPhaseAPI_StagedReadShape` in pkg/phases, which greps the serialized
 * response bytes and fails when the SQL blanking is removed. Re-asserting that
 * here would be a slower, weaker duplicate — and a DOM assertion cannot even
 * see a server leak, because the locked branch never passes content to the
 * placeholder.
 *
 * What these DO cover is the thing no lower layer can: a real user driving real
 * UI across the whole stack. The composer posts a chain through the actual form
 * and the GM view reads it back; cancelling a part round-trips a mutation and
 * updates the list. Component tests mock the API, so they cannot show those
 * wirings hold against the real backend.
 */

// If this string ever reaches the page, a player with devtools defeats the
// feature. It is deliberately distinctive so a substring search cannot miss it.
const SPOILER = 'SPOILER-E2E-the-blow-lands-and-you-die';

test.describe('Staged Result Reveals', () => {
  test.describe.configure({ mode: 'serial' });

  // The weakest of the three, kept for one reason: it is the only check that a
  // real player, hitting the real backend, gets a rendered placeholder rather
  // than a blank card or a crash. The component test proves the placeholder
  // renders from mocked props; this proves the real API's shape actually drives
  // it — part_count, released_at and unlocks_at all arriving as the component
  // expects. That wiring has broken before (Phase 4b's field names).
  test('player sees a locked placeholder driven by real API data', async ({ page }) => {
    // Seed as GM, then read as the recipient.
    await loginAs(page, 'GM');
    const gameId = await getFixtureGameId(page, 'E2E_GM_EDITING_RESULTS');
    const playerUserId = await getParticipantUserId(page, gameId, getWorkerUsername('TestPlayer1'));

    // 1440 minutes keeps part 2 pending for the life of the run — the worker can
    // never release it mid-test, so the locked assertions cannot flake.
    await createStagedChainViaApi(page, gameId, playerUserId, [
      { content: 'The sword whooshes toward your head...', delay_minutes: 0 },
      { content: SPOILER, delay_minutes: 1440 },
    ]);

    await loginAs(page, 'PLAYER_1');

    // ?tab=actions rather than clicking the tab: for a player the label is
    // dynamic ('Submit Action' / 'Action Submitted ✓') while the GM sees
    // 'Actions', so matching by name is state-dependent. See the note in
    // e2e/utils/navigation.ts.
    await page.goto(`/games/${gameId}?tab=actions`);
    await page.waitForLoadState('networkidle');

    // The released head reads normally.
    await expect(page.getByText('The sword whooshes toward your head...').first())
      .toBeVisible({ timeout: 10000 });

    // The pending part renders as a placeholder telling the player what is coming.
    const placeholder = page.locator('[data-testid^="staged-part-placeholder"]').first();
    await expect(placeholder).toBeVisible({ timeout: 10000 });
    await expect(placeholder).toContainText('Part 2 of 2');

    // Belt-and-braces, and deliberately NOT the feature's primary guarantee.
    //
    // "The server must not leak" is owned by
    // TestPhaseAPI_StagedReadShape/locked_text_appears_nowhere_in_the_raw_player_response,
    // which greps the serialized response bytes. That test fails when the SQL
    // blanking is removed; this one does not, because ActionResultsList renders
    // StagedPartPlaceholder without passing it `content` — so the spoiler cannot
    // reach the DOM even from a leaking server. (Confirmed by mutation.)
    //
    // What this DOES prove is the second layer: whatever the page received, the
    // rendered document does not contain the locked text — not hidden by CSS,
    // not in a data attribute, not parked in a script tag. Cheap to keep, and it
    // fails if someone "simplifies" the locked branch into rendering content.
    expect(await page.content()).not.toContain(SPOILER);
  });

  test('GM composes a staged chain and sees its schedule', async ({ page }) => {
    await loginAs(page, 'GM');
    const gameId = await getFixtureGameId(page, 'E2E_GM_EDITING_RESULTS');
    const gamePage = new GameDetailsPage(page);

    await gamePage.goto(gameId);
    await gamePage.goToActions();

    // The standalone composer is the deterministic way in: one button, a modal,
    // and the same CreateActionResultForm the submission cards use. Going
    // through a submission card would first require expanding whichever card
    // happens to be first, which depends on fixture ordering.
    await page.getByTestId('standalone-result-button').click();
    await expect(page.getByRole('heading', { name: 'Send Standalone Result' })).toBeVisible({ timeout: 10000 });

    // Recipient must be chosen before the form appears.
    const recipient = page.getByTestId('standalone-result-recipient');
    await expect(recipient).toBeVisible({ timeout: 5000 });
    await recipient.selectOption({ index: 1 });

    // Unique per run: this test creates drafts that persist in the fixture
    // game, so fixed strings would match a previous run's rows.
    const stamp = Date.now();
    const headText = `E2E composed part one ${stamp}`;
    const followUpText = `E2E composed part two ${stamp}`;

    const contentBox = page.getByTestId('result-content');
    await expect(contentBox).toBeVisible({ timeout: 10000 });
    await contentBox.fill(headText);

    // Staging is opt-in: the follow-up controls appear only after this click.
    // Before it, this form is byte-for-byte the one GMs already used.
    await page.getByTestId('add-staged-part').click();

    // The follow-up editor renders below the fold inside the modal, so scroll
    // it into view before asserting visibility.
    const followUp = page.getByTestId('staged-part-content-2');
    await followUp.scrollIntoViewIfNeeded();
    await expect(followUp).toBeVisible({ timeout: 5000 });
    await followUp.fill(followUpText);

    await page.getByTestId('staged-part-delay-2').selectOption('30');

    // The button counts the whole chain, not just the head — the clearest
    // signal the form switched to the staged path.
    const submit = page.getByRole('button', { name: /Create Draft Result \(2 parts\)/ });
    await submit.scrollIntoViewIfNeeded();
    await expect(submit).toBeVisible();
    await submit.click();

    // The modal closes on success, returning to the results list.
    await expect(page.getByRole('heading', { name: 'Send Standalone Result' }))
      .toHaveCount(0, { timeout: 10000 });

    // Both parts landed, and the chain is labelled with each part's position —
    // proof the whole chain posted as one request and the GM view read it back.
    await expect(page.getByText(headText).first()).toBeVisible({ timeout: 10000 });
    await expect(page.getByText(followUpText).first()).toBeVisible({ timeout: 10000 });
    await expect(page.locator('[data-testid^="staged-status-"]').first())
      .toContainText(/Part \d of 2/, { timeout: 10000 });
  });

  test('GM cancels a pending part and the released part survives', async ({ page }) => {
    await loginAs(page, 'GM');
    const gameId = await getFixtureGameId(page, 'E2E_GM_EDITING_RESULTS');
    const playerUserId = await getParticipantUserId(page, gameId, getWorkerUsername('TestPlayer2'));

    const marker = `CANCEL-E2E-${Date.now()}`;
    const ids = await createStagedChainViaApi(page, gameId, playerUserId, [
      { content: `${marker}-head`, delay_minutes: 0 },
      { content: `${marker}-pending`, delay_minutes: 1440 },
    ]);

    const gamePage = new GameDetailsPage(page);
    await gamePage.goto(gameId);
    await gamePage.goToActions();

    // Cancel is offered only for a published-but-unreleased part.
    const cancelButton = page.getByTestId(`cancel-staged-part-${ids[1]}`);
    await expect(cancelButton).toBeVisible({ timeout: 10000 });

    // The released head must NOT offer cancel — once a part is out, it stays out.
    await expect(page.getByTestId(`cancel-staged-part-${ids[0]}`)).toHaveCount(0);

    await cancelButton.click();
    await page.getByRole('button', { name: 'Yes, Cancel This Part' }).click();

    // The pending part is gone; the part already delivered is untouched.
    await expect(page.getByText(`${marker}-pending`)).toHaveCount(0, { timeout: 10000 });
    await expect(page.getByText(`${marker}-head`).first()).toBeVisible();
  });
});
