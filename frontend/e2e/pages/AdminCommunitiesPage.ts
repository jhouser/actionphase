import { Page, Locator, expect } from '@playwright/test';
import { LONG_TIMEOUT } from '../config/test-timeouts';
import { CommunityPage } from './CommunityPage';

/**
 * Page Object for the Communities tab of the site-admin panel (/admin/communities).
 *
 * Creating a community is site-admin-only, so this page is where communities
 * come from. The owner field is a UserSearchSelect rendering its dropdown
 * through a portal; picking out of it is fiddly enough that every spec doing it
 * inline would drift the moment that component changes. The selection itself is
 * delegated to CommunityPage.selectUserInSearch(), which already owns that
 * knowledge for the moderator and ban pickers.
 */
export class AdminCommunitiesPage {
  readonly page: Page;

  readonly nameInput: Locator;
  readonly slugInput: Locator;
  readonly ownerSearch: Locator;
  readonly createButton: Locator;
  readonly communitiesList: Locator;

  constructor(page: Page) {
    this.page = page;

    this.nameInput = page.getByTestId('community-name-input');
    this.slugInput = page.getByTestId('community-slug-input');
    this.ownerSearch = page.getByTestId('community-owner-search');
    this.createButton = page.getByTestId('create-community-button');
    this.communitiesList = page.getByTestId('communities-list');
  }

  /** Open the admin communities tab. */
  async goto() {
    await this.page.goto('/admin/communities');
    await this.page.waitForLoadState('networkidle');
  }

  /**
   * Create a community owned by `owner`.
   *
   * Waits for the new row to appear before returning, so callers can assert on
   * the result rather than on the act of submitting.
   */
  async createCommunity(name: string, slug: string, owner: string) {
    await expect(this.nameInput).toBeVisible({ timeout: LONG_TIMEOUT });

    await this.nameInput.fill(name);
    await this.slugInput.fill(slug);

    // Reuse the shared portal-dropdown handling rather than repeating it.
    const picker = new CommunityPage(this.page, slug);
    await picker.selectUserInSearch(
      this.ownerSearch,
      'community-owner-dropdown',
      owner
    );

    await this.createButton.click();
    await this.page.waitForLoadState('networkidle');

    await expect(this.rowByName(name)).toBeVisible({ timeout: LONG_TIMEOUT });
  }

  /** Locate a community's row in the admin list by its visible name. */
  rowByName(name: string): Locator {
    return this.communitiesList.locator('> div').filter({ hasText: name });
  }
}
