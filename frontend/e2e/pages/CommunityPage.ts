import { Page, Locator, expect } from '@playwright/test';
import { MEDIUM_TIMEOUT, DEFAULT_TIMEOUT, LONG_TIMEOUT } from '../config/test-timeouts';

/** The tabs on the community management shell, in the order they render. */
export type CommunityManageTab =
  | 'moderators'
  | 'bans'
  | 'history'
  | 'documents'
  | 'webhooks'
  | 'settings';

/**
 * Page Object for a community's public page and its management shell.
 *
 * Rows in the manage UI are keyed by USER ID rather than username
 * (`moderator-row-{id}`, `unban-{id}`), because a username is not unique across
 * workers. Resolve ids with getUserIdByUsername() from community-helpers.
 */
export class CommunityPage {
  readonly page: Page;
  readonly slug: string;

  // Public page
  readonly manageButton: Locator;
  readonly gamesLink: Locator;
  readonly documentsList: Locator;

  // Moderators tab
  readonly moderatorList: Locator;
  readonly moderatorSearch: Locator;
  readonly addModeratorSubmit: Locator;

  // Bans tab
  readonly banForm: Locator;
  readonly banSearch: Locator;
  readonly banReason: Locator;
  readonly banExpiresAt: Locator;
  readonly banSubmit: Locator;
  readonly banList: Locator;
  readonly bansEmpty: Locator;

  // Ban history tab
  readonly banEventList: Locator;

  // Documents tab
  readonly newDocumentButton: Locator;
  readonly documentForm: Locator;
  readonly documentTitle: Locator;
  readonly documentContent: Locator;
  readonly documentPublishNow: Locator;
  readonly documentSubmit: Locator;
  readonly documentList: Locator;

  // Settings tab
  readonly settingsForm: Locator;
  readonly settingsName: Locator;
  readonly settingsSave: Locator;

  constructor(page: Page, slug: string) {
    this.page = page;
    this.slug = slug;

    this.manageButton = page.getByTestId('manage-community');
    this.gamesLink = page.getByTestId('community-games-link');
    this.documentsList = page.getByTestId('community-documents');

    this.moderatorList = page.getByTestId('moderator-list');
    this.moderatorSearch = page.getByTestId('moderator-user-search');
    this.addModeratorSubmit = page.getByTestId('add-moderator-submit');

    this.banForm = page.getByTestId('ban-user-form');
    this.banSearch = page.getByTestId('ban-user-search');
    this.banReason = page.getByTestId('ban-reason');
    this.banExpiresAt = page.getByTestId('ban-expires-at');
    this.banSubmit = page.getByTestId('ban-user-submit');
    this.banList = page.getByTestId('ban-list');
    this.bansEmpty = page.getByTestId('bans-empty');

    this.banEventList = page.getByTestId('ban-event-list');

    this.newDocumentButton = page.getByTestId('new-document');
    this.documentForm = page.getByTestId('document-form');
    this.documentTitle = page.getByTestId('document-title');
    this.documentContent = page.getByTestId('document-content');
    this.documentPublishNow = page.getByTestId('document-publish-now');
    this.documentSubmit = page.getByTestId('document-submit');
    this.documentList = page.getByTestId('document-list');

    this.settingsForm = page.getByTestId('community-settings-form');
    this.settingsName = page.getByTestId('community-settings-name');
    this.settingsSave = page.getByTestId('community-settings-save');
  }

  /** Open the community's public page. */
  async goto() {
    await this.page.goto(`/communities/${this.slug}`);
    await this.page.waitForLoadState('networkidle');

    // Same reasoning as gotoManage(): a hidden spinner does not mean the page
    // rendered, so wait for the community's own <h1> before any caller reads
    // meaning into an element being absent.
    await expect(this.page.getByTestId('community-loading')).toBeHidden({
      timeout: LONG_TIMEOUT,
    });
    await expect(
      this.page.getByRole('heading', { level: 1 })
    ).toBeVisible({ timeout: LONG_TIMEOUT });
  }

  /**
   * Open one tab of the management shell directly.
   *
   * Navigating by URL rather than clicking the tab: each tab is its own route,
   * so a deep link is what a moderator actually uses, and it skips a click that
   * would otherwise have to be repeated in every spec.
   */
  async gotoManage(tab: CommunityManageTab = 'moderators') {
    await this.page.goto(`/communities/${this.slug}/manage/${tab}`);
    await this.page.waitForLoadState('networkidle');

    // Wait for the shell's HEADING, not merely for the spinner to vanish.
    //
    // "Spinner hidden" is not "page arrived": community-manage-loading is tied
    // to the query's isLoading, which is already false in the window before the
    // tab subtree renders. Callers then queried for a tab element that was
    // legitimately not there YET and got an immediate "element(s) not found" --
    // a race that reads exactly like a permissions bug, since a withheld
    // control is also absent.
    //
    // The heading renders for every signed-in visitor as soon as the community
    // resolves, whatever their role, so it marks "the shell is up" without
    // presuming anything about what this user is allowed to see inside it.
    await expect(this.page.getByTestId('community-manage-loading')).toBeHidden({
      timeout: LONG_TIMEOUT,
    });
    await expect(
      this.page.getByRole('heading', { name: /^Manage /, level: 1 })
    ).toBeVisible({ timeout: LONG_TIMEOUT });
  }

  /**
   * Pick a user out of a UserSearchSelect dropdown.
   *
   * The dropdown renders through a portal on document.body, so it is NOT a
   * descendant of the input — it has to be located from the page root by its
   * dropdownId. Options commit on mousedown (the component preventDefaults to
   * keep input focus), which is what click() dispatches anyway.
   */
  async selectUserInSearch(
    searchInput: Locator,
    dropdownId: string,
    username: string
  ) {
    await searchInput.fill(username);

    const dropdown = this.page.locator(`#${dropdownId}`);
    await expect(dropdown).toBeVisible({ timeout: MEDIUM_TIMEOUT });

    // Exact match: searching "TestPlayer1" also returns "TestPlayer1_2" etc.,
    // so a substring match could select another worker's user.
    const option = dropdown.locator('button').filter({
      has: this.page.locator(`div.font-medium:text-is("${username}")`),
    });
    await option.first().click();

    // The component writes "Selected: <username>" into the input's helper text,
    // which is a sibling of the input inside the Input wrapper. Waiting on it
    // proves the parent form holds the user before we submit -- the submit
    // button stays disabled until it does.
    await expect(searchInput.locator('..')).toContainText(`Selected: ${username}`, {
      timeout: DEFAULT_TIMEOUT,
    });
  }

  /** Add a moderator by username. Owner-only; the form is absent otherwise. */
  async addModerator(username: string) {
    await this.selectUserInSearch(
      this.moderatorSearch,
      'add-community-moderator',
      username
    );
    await this.addModeratorSubmit.click();
    await this.page.waitForLoadState('networkidle');
  }

  /** Remove a moderator by user id. There is no confirmation step. */
  async removeModerator(userId: number) {
    await this.page.getByTestId(`remove-moderator-${userId}`).click();
    await this.page.waitForLoadState('networkidle');
  }

  /** True when the roster shows this user as a moderator. */
  async hasModerator(userId: number): Promise<boolean> {
    return this.page.getByTestId(`moderator-row-${userId}`).isVisible();
  }

  /**
   * Ban a user.
   *
   * @param expiresAt - datetime-local value (e.g. '2027-01-01T12:00'). Omit for
   *                    a permanent ban.
   */
  async banUser(username: string, reason: string, expiresAt?: string) {
    await this.selectUserInSearch(this.banSearch, 'ban-community-user', username);
    await this.banReason.fill(reason);
    if (expiresAt) {
      await this.banExpiresAt.fill(expiresAt);
    }
    await this.banSubmit.click();
    await this.page.waitForLoadState('networkidle');
  }

  /** Lift a ban by user id. */
  async unbanUser(userId: number) {
    await this.page.getByTestId(`unban-${userId}`).click();
    await this.page.waitForLoadState('networkidle');
  }

  /**
   * The ban's rendered status, or null when the user has no ban row.
   *
   * A row's presence does NOT mean banned — an expired ban keeps its row and
   * renders as 'Expired'. Specs must assert on this text, not on the row.
   */
  async getBanStatus(userId: number): Promise<string | null> {
    // The bans panel must have RESOLVED before absence means anything: on a page
    // that never rendered, EVERY badge is missing, and a bare visibility check
    // would report that as "this user has no ban" -- passing a toBeNull()
    // assertion for entirely the wrong reason.
    //
    // Waiting on ban-list alone will not do: it and bans-empty are mutually
    // exclusive, and lifting the last ban legitimately leaves the empty state.
    // So wait for EITHER, which is what "the tab finished loading" means here.
    // Errors are deliberately not swallowed; a panel that never resolves should
    // surface as itself rather than as a missing ban.
    await expect(this.banList.or(this.bansEmpty)).toBeVisible({ timeout: LONG_TIMEOUT });

    const badge = this.page.getByTestId(`ban-status-${userId}`);
    if ((await badge.count()) === 0) {
      return null;
    }
    return (await badge.textContent())?.trim() ?? null;
  }

  /** Create a document, optionally publishing it immediately. */
  async createDocument(title: string, body: string, publish = false) {
    await this.newDocumentButton.click();
    await expect(this.documentForm).toBeVisible({ timeout: MEDIUM_TIMEOUT });

    await this.documentTitle.fill(title);

    // CommentEditor (the shared markdown editor) surfaces its textarea under
    // this testid rather than exposing one on the wrapper.
    await this.documentContent.fill(body);

    if (publish) {
      await this.documentPublishNow.click();
    }

    await this.documentSubmit.click();
    await expect(this.documentForm).toBeHidden({ timeout: MEDIUM_TIMEOUT });
    await this.page.waitForLoadState('networkidle');
  }

  /** Locate a document row by its visible title. */
  documentRowByTitle(title: string): Locator {
    return this.documentList.locator('li').filter({ hasText: title });
  }

  /** Rename the community from the Settings tab. */
  async renameCommunity(name: string) {
    await this.settingsName.fill(name);
    await this.settingsSave.click();
    await this.page.waitForLoadState('networkidle');
  }
}
