import { Page, Locator } from '@playwright/test';

/**
 * Page Object for Character Sheet interactions
 *
 * Handles character viewing, editing, and management operations.
 *
 * The sheet's tabs are flat (Phase 4) and its three stat tabs are GM-renameable
 * (Phase 3), so navigation takes a label rather than assuming fixed wording.
 */
export class CharacterSheetPage {
  readonly page: Page;

  // Locators
  readonly characterName: Locator;
  readonly characterType: Locator;
  readonly characterDescription: Locator;
  readonly editButton: Locator;
  readonly saveButton: Locator;
  readonly cancelButton: Locator;
  readonly deleteButton: Locator;
  readonly avatarUploadButton: Locator;
  readonly inventorySection: Locator;
  readonly skillsSection: Locator;
  readonly itemsSection: Locator;
  readonly numbersSection: Locator;

  constructor(page: Page) {
    this.page = page;

    // Define locators
    this.characterName = page.locator('[data-testid="character-name"]');
    this.characterType = page.locator('[data-testid="character-type"]');
    this.characterDescription = page.locator('[data-testid="character-description"]');
    this.editButton = page.locator('[data-testid="edit-character"]');
    this.saveButton = page.locator('[data-testid="save-character"]');
    this.cancelButton = page.locator('[data-testid="cancel-edit"]');
    this.deleteButton = page.locator('[data-testid="delete-character"]');
    this.avatarUploadButton = page.locator('input[type="file"]');
    this.inventorySection = page.locator('[data-testid="items-section"]');
    this.skillsSection = page.locator('[data-testid="skills-section"]');
    this.itemsSection = page.locator('[data-testid="items-section"]');
    this.numbersSection = page.locator('[data-testid="numbers-section"]');
  }

  /**
   * Navigate to a specific character sheet
   */
  async goto(gameId: number, characterId: number) {
    await this.page.goto(`/games/${gameId}/characters/${characterId}`);
    await this.page.waitForLoadState('networkidle');
  }

  /**
   * Get character name text
   */
  async getCharacterName(): Promise<string> {
    return await this.characterName.textContent() || '';
  }

  /**
   * Edit a character field
   */
  async editField(field: string, value: string) {
    await this.editButton.click();
    await this.page.waitForLoadState('networkidle');

    await this.page.fill(`[data-testid="input-${field}"]`, value);
    await this.saveButton.click();
    await this.page.waitForLoadState('networkidle');
  }

  /**
   * Upload character avatar
   */
  async uploadAvatar(filePath: string) {
    await this.avatarUploadButton.setInputFiles(filePath);
    await this.page.waitForSelector('[data-testid="avatar-preview"]', { timeout: 5000 });
  }

  /**
   * Delete the character (with confirmation)
   */
  async deleteCharacter() {
    await this.deleteButton.click();

    // Handle confirmation dialog
    await this.page.click('[data-testid="confirm-delete"]');
    await this.page.waitForURL('**/games/*', { timeout: 5000 });
  }

  /**
   * Check if character is editable by current user
   */
  async canEdit(): Promise<boolean> {
    return await this.editButton.isVisible();
  }

  // ========== Character Rename Methods ==========

  /**
   * Click the rename button to enter rename mode
   */
  async startRename() {
    const renameButton = this.page.locator('button[title="Rename character"]');
    await renameButton.click();
    await this.page.waitForTimeout(300); // Wait for edit UI to appear
  }

  /**
   * Get the rename input field
   */
  getRenameInput(): Locator {
    return this.page.getByRole('textbox');
  }

  /**
   * Check if rename button is visible
   */
  async canRename(): Promise<boolean> {
    const renameButton = this.page.locator('button[title="Rename character"]');
    return await renameButton.isVisible();
  }

  /**
   * Rename character and save
   * @param newName - New character name
   */
  async renameCharacter(newName: string) {
    await this.startRename();
    const nameInput = this.getRenameInput();
    await nameInput.clear();
    await nameInput.fill(newName);

    const saveButton = this.page.getByRole('button', { name: 'Save' });
    await saveButton.click();
    await this.page.waitForLoadState('networkidle');
  }

  /**
   * Start rename and then cancel
   * @param tempName - Temporary name to type before canceling
   */
  async startAndCancelRename(tempName?: string) {
    await this.startRename();

    if (tempName) {
      const nameInput = this.getRenameInput();
      await nameInput.clear();
      await nameInput.fill(tempName);
    }

    const cancelButton = this.page.getByRole('button', { name: 'Cancel' });
    await cancelButton.click();
    await this.page.waitForTimeout(500);
  }

  /**
   * Check if save button is enabled during rename
   */
  async isSaveButtonEnabled(): Promise<boolean> {
    const saveButton = this.page.getByRole('button', { name: 'Save' });
    return await saveButton.isEnabled();
  }

  /**
   * Get all inventory items
   */
  async getInventoryItems(): Promise<string[]> {
    const items = await this.inventorySection.locator('[data-testid="inventory-item"]').all();
    return Promise.all(items.map(i => i.textContent())).then(texts =>
      texts.filter((t): t is string => t !== null)
    );
  }

  // ========== Character Sheet Tab Navigation ==========
  // The sheet is a FLAT list of five tabs: Public Profile, Private Notes, and
  // the three renameable stat tabs (Skills, Inventory, Numbers by default).
  //
  // It used to be two levels — "Abilities & Skills" and "Inventory" modules,
  // each with sub-tabs — which is why the old methods came in pairs
  // (goToAbilitiesModule + goToAbilitiesTab). There is no second level now, so
  // one method per tab is the whole API.
  //
  // Stat tab labels are GM-renameable per game, so every stat method takes an
  // optional label. The defaults match DEFAULT_SHEET_LABELS in
  // frontend/src/hooks/useSheetLabels.ts — the single source of those defaults.

  /**
   * Get the character sheet tab select dropdown (mobile).
   * Scoped via data-testid="character-sheet-module-tabs" to distinguish it from
   * the game-level tab select when both are present on the same page.
   */
  private get moduleSelect() {
    return this.page.getByTestId('character-sheet-module-tabs').locator('select#tab-select');
  }

  /**
   * Navigate to a tab by its storage id and visible label.
   *
   * Mobile selects by id; desktop clicks by label. The two differ because the
   * ids are stable (`skills`) while the labels are per-game (`Talents`), and
   * only the desktop tab strip renders the label.
   *
   * Falls back to the overflow "More" menu on desktop: at five tabs a narrow
   * viewport pushes the last ones out of the strip, where getByRole('tab') can
   * still find them but a plain click would miss.
   */
  private async goToTab(tabId: string, label: string) {
    const isMobile = await this.waitForModuleTabsReady();
    if (isMobile) {
      await this.moduleSelect.scrollIntoViewIfNeeded();
      await this.moduleSelect.selectOption(tabId);
    } else {
      const tab = this.page.getByRole('tab', { name: label });
      if (await tab.isVisible({ timeout: 1000 }).catch(() => false)) {
        await tab.click();
      } else {
        await this.page.getByRole('button', { name: /More/ }).click();
        await tab.click();
      }
    }
    await this.page.waitForLoadState('networkidle');
  }

  /**
   * Navigate to the Public Profile (bio) tab.
   */
  async goToBioModule() {
    await this.goToTab('bio', 'Public Profile');
  }

  /**
   * Navigate to the Private Notes tab.
   */
  async goToNotesTab() {
    await this.goToTab('notes', 'Private Notes');
  }

  /**
   * Navigate to the Skills tab.
   * @param label - The game's label for this tab, when renamed.
   */
  async goToSkillsTab(label = 'Skills') {
    await this.goToTab('skills', label);
  }

  /**
   * Navigate to the Inventory tab.
   * @param label - The game's label for this tab, when renamed.
   */
  async goToInventoryTab(label = 'Inventory') {
    await this.goToTab('inventory', label);
  }

  /**
   * Navigate to the Numbers tab.
   *
   * Replaces goToCurrencyTab: the tab was promoted out of Inventory and its
   * storage key renamed `currency` -> `numbers`, because it holds arbitrary
   * numeric tracks (stress, XP, clocks), not only money.
   *
   * @param label - The game's label for this tab, when renamed.
   */
  async goToNumbersTab(label = 'Numbers') {
    await this.goToTab('numbers', label);
  }

  /**
   * Wait for the character sheet tab container to appear in the DOM, then
   * report whether we are on a mobile viewport (select visible vs tabs visible).
   */
  private async waitForModuleTabsReady(): Promise<boolean> {
    await this.page.getByTestId('character-sheet-module-tabs').waitFor({ state: 'attached', timeout: 5000 });
    // The select is inside md:hidden — only visible on mobile viewports
    return await this.moduleSelect.isVisible({ timeout: 2000 }).catch(() => false);
  }

  /**
   * Add a skill (requires the "Add Skill" button to be visible).
   *
   * Replaces addAbility: abilities were retired because they duplicated skills,
   * which is strictly more featured (rank, category, markdown description).
   *
   * @param name - Skill name
   * @param description - Skill description
   */
  async addSkill(name: string, description: string) {
    await this.page.getByRole('button', { name: 'Add Skill' }).click();
    await this.page.waitForTimeout(500);

    await this.page.getByRole('textbox', { name: 'Skill Name *' }).fill(name);
    await this.page.getByRole('textbox', { name: 'Description' }).fill(description);

    // The second "Add Skill" button is the form's submit; the first opened the modal.
    await this.page.getByRole('button', { name: 'Add Skill' }).nth(1).click();
    await this.page.waitForLoadState('networkidle');
  }

  /**
   * Check if the "Add Skill" button is visible (GM/owner permission check).
   */
  async canAddSkill(): Promise<boolean> {
    try {
      await this.page.getByRole('button', { name: 'Add Skill' }).waitFor({ state: 'visible', timeout: 2000 });
      return true;
    } catch {
      return false;
    }
  }

  /**
   * Check if the "Add Item" button is visible (GM/owner permission check).
   */
  async canAddItem(): Promise<boolean> {
    try {
      await this.page.getByRole('button', { name: 'Add Item' }).waitFor({ state: 'visible', timeout: 2000 });
      return true;
    } catch {
      return false;
    }
  }

  /**
   * Check if the Numbers tab's add button is visible (GM/owner permission check).
   *
   * Replaces canAddCurrency. The button is labelled "Add {label}", following the
   * game's name for the tab, so the label has to be passed when renamed.
   *
   * @param label - The game's label for this tab, when renamed.
   */
  async canAddNumber(label = 'Numbers'): Promise<boolean> {
    try {
      await this.page.getByRole('button', { name: `Add ${label}` }).waitFor({ state: 'visible', timeout: 2000 });
      return true;
    } catch {
      return false;
    }
  }

  /**
   * Check if a specific tab is visible.
   * Used to verify permission boundaries (e.g. players shouldn't see Inventory).
   * Works with both the mobile dropdown and the desktop tab strip.
   */
  async isModuleVisible(
    moduleName: 'Public Profile' | 'Private Notes' | 'Skills' | 'Inventory' | 'Numbers' | (string & {}),
    tabId?: string,
  ): Promise<boolean> {
    // Map default label to storage id. Stat tabs are GM-renameable, so a game
    // using its own wording must pass tabId explicitly — the default labels are
    // only a convenience for games that never renamed anything.
    const moduleIdMap: Record<string, string> = {
      'Public Profile': 'bio',
      'Private Notes': 'notes',
      'Skills': 'skills',
      'Inventory': 'inventory',
      'Numbers': 'numbers'
    };
    const moduleId = tabId ?? moduleIdMap[moduleName];

    // Check mobile dropdown (scoped to character sheet module tabs to avoid matching game-level select)
    // Use isVisible() not count() — the select exists in DOM on desktop too but is hidden via md:hidden
    const mobileSelect = this.moduleSelect;
    if (await mobileSelect.isVisible({ timeout: 2000 }).catch(() => false)) {
      // Check if the option exists in the dropdown
      const option = mobileSelect.locator(`option[value="${moduleId}"]`);
      return await option.count() > 0;
    }

    // Check desktop tabs
    try {
      await this.page.getByRole('tab', { name: moduleName }).waitFor({ state: 'visible', timeout: 2000 });
      return true;
    } catch {
      return false;
    }
  }
}
