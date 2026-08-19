import { Page } from '@playwright/test';
import { waitForModal } from '../utils/waits';

/**
 * Page Object for Game Settings Modal
 *
 * Handles the Edit Game modal workflow including:
 * - Opening/closing the edit modal
 * - Updating game fields (title, description, genre, max_players, dates)
 * - Toggling settings (is_anonymous)
 * - Saving/canceling changes
 */
export class GameSettingsPage {
  readonly page: Page;

  constructor(page: Page) {
    this.page = page;
  }

  /**
   * Open the Edit Game modal
   */
  async openEditModal() {
    // First open the game actions kebab menu
    await this.page.getByLabel('Game actions').click();
    // Then click "Edit Game" from the dropdown
    await this.page.getByRole('button', { name: 'Edit Game' }).click();
    await waitForModal(this.page, 'Edit Game');
  }

  /**
   * Open one of the form's tabs.
   *
   * The Edit Game form is tab-split. Inactive panels stay mounted (hidden with
   * display:none) so the whole form validates and submits from any tab, and
   * read-only checks like inputValue()/isChecked() still work on them — but
   * Playwright will not fill or click a control it cannot see, so any
   * interaction has to open the owning tab first.
   */
  async openTab(tab: 'info' | 'schedule' | 'rules' | 'appearance') {
    // `game-form-` prefixed: the game page behind this modal has its own tab bar
    // with an `info` tab, so a bare `tab-info` matches two elements.
    await this.page.getByTestId(`tab-game-form-${tab}`).click();
  }

  /**
   * Update game title
   */
  async updateTitle(newTitle: string) {
    await this.openTab('info');
    await this.page.fill('#title', newTitle);
  }

  /**
   * Update game description
   */
  async updateDescription(newDescription: string) {
    await this.openTab('info');
    await this.page.fill('#description', newDescription);
  }

  /**
   * Update game genre
   */
  async updateGenre(newGenre: string) {
    await this.openTab('info');
    await this.page.fill('#genre', newGenre);
  }

  /**
   * Update max players
   */
  async updateMaxPlayers(maxPlayers: number) {
    await this.openTab('info');
    await this.page.fill('#max_players', maxPlayers.toString());
  }

  /**
   * Set a date/time using the DateTimeInput (react-datepicker)
   * @param fieldId - ID of the field ('recruitment_deadline', 'start_date', 'end_date')
   * @param dateTimeString - datetime-local format string (YYYY-MM-DDTHH:mm)
   */
  private async setDateTimeField(fieldId: string, dateTimeString: string) {
    await this.openTab('schedule');
    const futureDate = new Date(dateTimeString);

    // Round to nearest 15 minutes (datepicker uses 15-minute intervals)
    futureDate.setMinutes(Math.round(futureDate.getMinutes() / 15) * 15);
    futureDate.setSeconds(0);
    futureDate.setMilliseconds(0);

    // Click the input to open the date picker
    await this.page.locator(`#${fieldId}`).click();

    // Wait for the date picker dialog to appear
    await this.page.waitForSelector('[role="dialog"]', { state: 'visible', timeout: 5000 });
    await this.page.waitForTimeout(300); // Give picker time to fully render

    // Get the date components
    const dayOfMonth = futureDate.getDate();
    const monthName = futureDate.toLocaleString('en-US', { month: 'long' });
    const year = futureDate.getFullYear();
    const ordinalDay = this.getOrdinalSuffix(dayOfMonth);

    // Try to find the date - if not found, navigate months
    const datePattern = new RegExp(`Choose.*${monthName} ${ordinalDay}, ${year}`, 'i');
    const dateCell = this.page.getByRole('gridcell', { name: datePattern });

    // Check if date is visible, if not, navigate to correct month
    let attempts = 0;
    while (!(await dateCell.isVisible().catch(() => false)) && attempts < 12) {
      // Click next month button
      await this.page.getByRole('button', { name: /Next Month/i }).click();
      await this.page.waitForTimeout(200);
      attempts++;
    }

    // Click the date
    await dateCell.click();

    // Select the time from the dropdown
    const hours = String(futureDate.getHours()).padStart(2, '0');
    const minutes = String(futureDate.getMinutes()).padStart(2, '0');
    const timeString = `${hours}:${minutes}`;

    await this.page.getByRole('option', { name: timeString }).click();

    // Wait for picker to close
    await this.page.waitForSelector('[role="dialog"]', { state: 'hidden', timeout: 2000 });
  }

  /**
   * Helper to get ordinal suffix for date (1st, 2nd, 3rd, 4th, etc.)
   */
  private getOrdinalSuffix(day: number): string {
    if (day > 3 && day < 21) return `${day}th`;
    switch (day % 10) {
      case 1: return `${day}st`;
      case 2: return `${day}nd`;
      case 3: return `${day}rd`;
      default: return `${day}th`;
    }
  }

  /**
   * Update start date
   * @param date - Date string in YYYY-MM-DDTHH:mm format (datetime-local)
   */
  async updateStartDate(date: string) {
    await this.setDateTimeField('start_date', date);
  }

  /**
   * Update end date
   * @param date - Date string in YYYY-MM-DDTHH:mm format (datetime-local)
   */
  async updateEndDate(date: string) {
    await this.setDateTimeField('end_date', date);
  }

  /**
   * Update recruitment deadline
   * @param date - Date string in YYYY-MM-DDTHH:mm format (datetime-local)
   */
  async updateRecruitmentDeadline(date: string) {
    await this.setDateTimeField('recruitment_deadline', date);
  }

  /**
   * Toggle anonymous mode
   */
  async toggleAnonymous() {
    await this.openTab('rules');
    await this.page.locator('#is_anonymous').click();
  }

  /**
   * Toggle auto accept audience
   */
  async toggleAutoAcceptAudience() {
    await this.openTab('rules');
    await this.page.locator('#auto_accept_audience').click();
  }

  /**
   * Save changes and close modal
   */
  async saveChanges() {
    await this.page.getByRole('button', { name: 'Save Changes' }).click();
    // Wait for the modal to close before proceeding
    await this.page.getByRole('heading', { name: 'Edit Game', level: 2 }).waitFor({ state: 'hidden' });
    await this.page.waitForLoadState('networkidle');
  }

  /**
   * Cancel and close modal, discarding any unsaved edits.
   *
   * Cancelling a form with unsaved edits raises an inline confirmation rather
   * than closing outright, so the discard has to be confirmed. The bar is only
   * shown when the form is actually dirty — hence the isVisible() check rather
   * than an unconditional click, which would time out on a clean form.
   */
  async cancel() {
    await this.page.getByRole('button', { name: 'Cancel' }).click();

    const discardButton = this.page.getByTestId('confirm-close-discard');
    if (await discardButton.isVisible()) {
      await discardButton.click();
    }

    // Wait for the modal to close before proceeding
    await this.page.getByRole('heading', { name: 'Edit Game', level: 2 }).waitFor({ state: 'hidden' });
    await this.page.waitForLoadState('networkidle');
  }

  /**
   * Get current anonymous checkbox state
   */
  async isAnonymous(): Promise<boolean> {
    return await this.page.locator('#is_anonymous').isChecked();
  }

  /**
   * Get current auto accept audience checkbox state
   */
  async isAutoAcceptAudience(): Promise<boolean> {
    return await this.page.locator('#auto_accept_audience').isChecked();
  }

  /**
   * Get current title value from form
   */
  async getTitle(): Promise<string> {
    return await this.page.locator('#title').inputValue();
  }

  /**
   * Get current description value from form
   */
  async getDescription(): Promise<string> {
    return await this.page.locator('#description').inputValue();
  }

  /**
   * Get current genre value from form
   */
  async getGenre(): Promise<string> {
    return await this.page.locator('#genre').inputValue();
  }

  /**
   * Get current max players value from form
   */
  async getMaxPlayers(): Promise<string> {
    return await this.page.locator('#max_players').inputValue();
  }

  /**
   * Get current start date value from form
   */
  async getStartDate(): Promise<string> {
    return await this.page.locator('#start_date').inputValue();
  }

  /**
   * Get current end date value from form
   */
  async getEndDate(): Promise<string> {
    return await this.page.locator('#end_date').inputValue();
  }

  /**
   * Get current recruitment deadline value from form
   */
  async getRecruitmentDeadline(): Promise<string> {
    return await this.page.locator('#recruitment_deadline').inputValue();
  }
}
