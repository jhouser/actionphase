import { Page } from '@playwright/test';
import { navigateToGameTab } from '../utils/navigation';

/**
 * Page Object for Action Results (History)
 *
 * Handles viewing action results and game history
 * Results are accessed via the game's History tab
 */
export class ActionResultsPage {
  readonly page: Page;
  readonly gameId: number;

  constructor(page: Page, gameId: number) {
    this.page = page;
    this.gameId = gameId;
  }

  /**
   * Navigate to game's history tab
   */
  async goto() {
    await this.page.goto(`/games/${this.gameId}`);
    await this.page.waitForLoadState('networkidle');

    // Navigate to History tab (handles mobile select and desktop tabs)
    await navigateToGameTab(this.page, 'History');
  }

  /**
   * Reveal a result's full text, whether or not it is collapsed.
   *
   * Results collapse only when their rendered height overflows, so whether a
   * "Show full content" toggle exists depends on how tall the content actually
   * renders — not on its character count. Short-but-published results show no
   * toggle at all and are already fully visible, so this expands when there is
   * something to expand and is a no-op otherwise.
   */
  async expandResultsIfCollapsed() {
    const toggle = this.page.getByRole('button', { name: 'Show full content' }).first();
    if (await toggle.isVisible().catch(() => false)) {
      await toggle.click();
    }
  }

  /**
   * View results for a specific phase
   *
   * @param phaseNumber - Phase number to view
   */
  async viewPhaseResults(phaseNumber: number) {
    // Look for phase selector or phase link
    const phaseSelector = this.page.locator(`button:has-text("Phase ${phaseNumber}"), a:has-text("Phase ${phaseNumber}")`);

    await phaseSelector.waitFor({ state: 'visible', timeout: 5000 });
    await phaseSelector.click();
    await this.page.waitForLoadState('networkidle');

    // Click on the Results tab (phase view shows Submissions/Results tabs)
    await this.clickResultsTab();
  }

  /**
   * Click on the Results tab within a phase view
   */
  async clickResultsTab() {
    const resultsTab = this.page.getByRole('button', { name: 'Results', exact: true });
    await resultsTab.waitFor({ state: 'visible', timeout: 5000 });
    await resultsTab.click();
    await this.page.waitForLoadState('networkidle');
  }

  /**
   * Click on the Submissions tab within a phase view
   */
  async clickSubmissionsTab() {
    const submissionsTab = this.page.getByRole('button', { name: 'Submissions', exact: true });
    await submissionsTab.waitFor({ state: 'visible', timeout: 5000 });
    await submissionsTab.click();
    await this.page.waitForLoadState('networkidle');
  }

  /**
   * Get all results for the current view
   *
   * @returns Array of result texts
   */
  async getResults(): Promise<string[]> {
    // Wait for results to load
    await this.page.waitForLoadState('networkidle');

    // Look for result cards or sections
    const resultElements = await this.page
      .locator('[data-testid^="result-"], [data-testid^="action-result-"], .result-card, .action-result')
      .all();

    const results: string[] = [];
    for (const element of resultElements) {
      const content = await element.textContent();
      if (content) {
        results.push(content.trim());
      }
    }

    return results;
  }

  /**
   * Check if results exist for current view
   */
  async hasResults(): Promise<boolean> {
    try {
      const results = await this.getResults();
      return results.length > 0;
    } catch {
      return false;
    }
  }

  /**
   * Get result content for a specific character
   *
   * @param characterName - Character name to find results for
   * @returns Result content or null if not found
   */
  async getResultForCharacter(characterName: string): Promise<string | null> {
    const allResults = await this.page
      .locator('[data-testid^="result-"], .result-card')
      .all();

    for (const result of allResults) {
      const text = await result.textContent();
      if (text?.includes(characterName)) {
        return text.trim();
      }
    }

    return null;
  }

  /**
   * Get all phase numbers that have results
   *
   * @returns Array of phase numbers
   */
  async getAvailablePhases(): Promise<number[]> {
    const phaseButtons = await this.page
      .locator('button:has-text("Phase"), a:has-text("Phase")')
      .all();

    const phases: number[] = [];
    for (const button of phaseButtons) {
      const text = await button.textContent();
      const match = text?.match(/Phase (\d+)/);
      if (match) {
        phases.push(parseInt(match[1]));
      }
    }

    return phases.sort((a, b) => a - b);
  }

  /**
   * Check if a specific phase has published results
   *
   * @param phaseNumber - Phase number to check
   */
  async hasPhaseResults(phaseNumber: number): Promise<boolean> {
    const availablePhases = await this.getAvailablePhases();
    return availablePhases.includes(phaseNumber);
  }

  /**
   * Get count of results in current view
   */
  async getResultsCount(): Promise<number> {
    const results = await this.getResults();
    return results.length;
  }

  /**
   * Filter results by result type (if filtering is available)
   *
   * @param resultType - Type of result to filter by
   */
  async filterByResultType(resultType: string) {
    // Look for filter dropdown or buttons
    const filterSelect = this.page.locator('select[aria-label*="filter"], select[aria-label*="type"]');
    const isVisible = await filterSelect.isVisible().catch(() => false);

    if (isVisible) {
      await filterSelect.selectOption(resultType);
      await this.page.waitForLoadState('networkidle');
    }
  }

  /**
   * Search for specific text in results
   *
   * @param searchText - Text to search for
   * @returns Whether the text was found in any result
   */
  async searchResults(searchText: string): Promise<boolean> {
    const results = await this.getResults();
    return results.some(result => result.includes(searchText));
  }

  /**
   * Get results organized by phase
   *
   * @returns Map of phase number to results
   */
  async getResultsByPhase(): Promise<Map<number, string[]>> {
    const resultsByPhase = new Map<number, string[]>();
    const phases = await this.getAvailablePhases();

    for (const phaseNumber of phases) {
      await this.viewPhaseResults(phaseNumber);
      const results = await this.getResults();
      resultsByPhase.set(phaseNumber, results);
    }

    return resultsByPhase;
  }

  /**
   * Check if history tab is accessible (game has progressed)
   */
  async isHistoryAvailable(): Promise<boolean> {
    try {
      const mobileSelect = this.page.locator('select#tab-select');
      const isMobile = await mobileSelect.isVisible({ timeout: 2000 }).catch(() => false);
      if (isMobile) {
        return await mobileSelect.locator('option', { hasText: 'History' }).count() > 0;
      } else {
        await this.page.getByTestId('tab-history').waitFor({ state: 'visible', timeout: 3000 });
        return true;
      }
    } catch {
      return false;
    }
  }
}
