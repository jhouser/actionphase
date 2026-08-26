# Page Object Models (POMs)

This directory contains Page Object Models for E2E testing with Playwright.

## What are POMs?

Page Object Models (POMs) encapsulate page interactions into reusable, maintainable classes. They provide stable, semantic methods that make tests more readable and resilient to UI changes.

## POM-First Development Rule

**Always create or use POMs when writing tests. Never use inline selectors directly in tests.**

### Why?
- ✅ **Stable**: Uses semantic selectors (`getByRole`, `getByTestId`) instead of brittle class names
- ✅ **Maintainable**: UI changes only require updating the POM, not every test
- ✅ **Readable**: Tests read like user stories, not technical implementation
- ✅ **Reusable**: Share methods across multiple test files

## Available POMs

There are **26** page objects in this directory. The sections below document
seven of them in detail; the rest follow the same conventions — read the source,
which is short and self-describing.

<details>
<summary>Full list of page objects (26)</summary>

Documented in detail below: `CharacterSheetPage`, `GameSettingsPage`, `ParticipantsPage`, `AvatarManagementPage`, `GameDetailsPage`, `HistoryPage`, `MessagingPage`

Also available:

| File |
|---|
| `ActionResultsPage.ts` |
| `ActionSubmissionPage.ts` |
| `AdminDashboardPage.ts` |
| `AudiencePage.ts` |
| `CharacterWorkflowPage.ts` |
| `CommonRoomPage.ts` |
| `ConversationPage.ts` |
| `DashboardPage.ts` |
| `GameApplicationsPage.ts` |
| `GameHandoutsPage.ts` |
| `GamesListPage.ts` |
| `LoginPage.ts` |
| `NotificationsPage.ts` |
| `PhaseManagementPage.ts` |
| `PollsPage.ts` |
| `PostPage.ts` |
| `RegistrationPage.ts` |
| `SettingsPage.ts` |
| `UserSettingsPage.ts` |

</details>


### CharacterSheetPage
**File**: `CharacterSheetPage.ts`
**Purpose**: Character sheet viewing, editing, renaming, and tab navigation

**Constructor**: `new CharacterSheetPage(page: Page)`

**Key Methods**:
```typescript
// Navigation
async goto(gameId: number, characterId: number)
async goToBioModule()
async goToNotesTab()
async goToSkillsTab(label = 'Skills')
async goToInventoryTab(label = 'Inventory')
async goToNumbersTab(label = 'Numbers')

// Workflows
async editField(field: string, value: string)
async addSkill(name: string, description: string)
async uploadAvatar(filePath: string)
async deleteCharacter()

// Rename flow
async startRename()
getRenameInput(): Locator
async renameCharacter(newName: string)
async startAndCancelRename(tempName?: string)

// Permission / state checks
async canEdit(): Promise<boolean>
async canRename(): Promise<boolean>
async canAddSkill(): Promise<boolean>
async canAddItem(): Promise<boolean>
async canAddNumber(): Promise<boolean>
async isSaveButtonEnabled(): Promise<boolean>
async isModuleVisible(moduleName: string): Promise<boolean>

// Data Access
async getCharacterName(): Promise<string>
async getInventoryItems(): Promise<string[]>
```

> Tab methods take a **label string**, not a count, and handle the mobile
> `<select>` fallback internally — see `goToSkillsTab` in the source.

**Usage Example**:
```typescript
const sheetPage = new CharacterSheetPage(page);
await sheetPage.goto(2, 5);
await sheetPage.goToSkillsTab();
expect(await sheetPage.canAddSkill()).toBe(true);
await sheetPage.addSkill('Fireball', 'Powerful spell');
```

---

### GameSettingsPage
**File**: `GameSettingsPage.ts`
**Purpose**: Edit Game modal workflow and settings management

**Constructor**: `new GameSettingsPage(page: Page)`

**Key Methods**:
```typescript
// Modal Control
async openEditModal()
async saveChanges()
async cancel()

// Field Updates
async updateTitle(newTitle: string)
async updateDescription(newDescription: string)
async updateGenre(newGenre: string)
async updateMaxPlayers(maxPlayers: number)

// Toggle Settings
async toggleAnonymous()

// Getters
async isAnonymous(): Promise<boolean>
async getTitle(): Promise<string>
async getGenre(): Promise<string>
async getMaxPlayers(): Promise<string>
```

**Usage Example**:
```typescript
const settingsPage = new GameSettingsPage(page);
await settingsPage.openEditModal();
await settingsPage.updateTitle('New Game Title');
await settingsPage.updateMaxPlayers(6);
await settingsPage.saveChanges();
```

---

### ParticipantsPage
**File**: `ParticipantsPage.ts`
**Purpose**: Participant/people management and viewing

**Constructor**: `new ParticipantsPage(page: Page, gameId: number)`

**Key Methods**:
```typescript
// Navigation
async goto()
async goToParticipantsTab()

// List Operations
async getParticipantsList(): Promise<string[]>
async getParticipantsByRole(role: string): Promise<string[]>
async getParticipantsCount(): Promise<number>

// Search & Filter
async searchParticipants(searchTerm: string)
async hasParticipant(username: string): Promise<boolean>
async getParticipantRole(username: string): Promise<string>

// Permissions
async canManageParticipants(): Promise<boolean>
```

**Usage Example**:
```typescript
const participantsPage = new ParticipantsPage(page, gameId);
await participantsPage.goto();
const count = await participantsPage.getParticipantsCount();
const players = await participantsPage.getParticipantsByRole('player');
expect(await participantsPage.hasParticipant('testuser')).toBe(true);
```

---

### AvatarManagementPage
**File**: `AvatarManagementPage.ts`
**Purpose**: Avatar upload, preview, and management workflow

**Constructor**: `new AvatarManagementPage(page: Page)`

**Key Methods**:
```typescript
// Upload Workflow
async uploadAvatar(filePath: string)
async uploadAndSaveAvatar(filePath: string)
async waitForPreview()

// Actions
async saveAvatar()
async cancelUpload()
async deleteAvatar()

// State Checks
async hasPreview(): Promise<boolean>
async canUploadAvatar(): Promise<boolean>
async hasCurrentAvatar(): Promise<boolean>

// Error Handling
async getUploadError(): Promise<string | null>

// Data Access
async getAvatarSrc(): Promise<string | null>
```

**Usage Example**:
```typescript
const avatarPage = new AvatarManagementPage(page);
await avatarPage.uploadAvatar('/path/to/image.png');
await avatarPage.waitForPreview();
expect(await avatarPage.hasPreview()).toBe(true);
await avatarPage.saveAvatar();
```

---

### GameDetailsPage
**File**: `GameDetailsPage.ts`
**Purpose**: Master navigation POM for game detail pages and game state management

**Constructor**: `new GameDetailsPage(page: Page)`

**Key Methods**:
```typescript
// Navigation
async goto(gameId: number)
async goToTab(tabName: string)
async goToCommonRoom()
async goToPhases() // Alias for goToPhaseManagement()
async goToPhaseManagement()
async goToPeople()
async goToCharacters() // Handles both character_creation and in_progress states
async goToMessages()
async goToHistory()
async goToHandouts()
async goToAudience()
async goToGameInfo()
async goToSettings()
async goToApplications()
async goToParticipants()
async goToActions() // GM view
async goToSubmitAction() // Player view

// Game State Management (GM only)
async startRecruitment()
async startGame()
async pauseGame() // Handles confirmation modal
async resumeGame()
async completeGame() // Handles confirmation modal with text input
async endGame()
async cancelGame()

// Application Management (GM only)
async approveApplication(playerUsername: string)
async rejectApplication(playerUsername: string)

// Getters
get gameTitle(): Locator
get stateBadge(): Locator
getButton(text: string): Locator

// Verification
async verifyOnPage(gameId: number)
async verifyGameState(state: string)
async verifyActiveTab(tabName: string)
async getParticipantCount(): Promise<number>
async verifyParticipantExists(username: string)
async verifyApplicationStatus(username: string, status: string)
```

**Usage Example**:
```typescript
const gamePage = new GameDetailsPage(page);
await gamePage.goto(gameId);
await gamePage.goToPhaseManagement();
await gamePage.startGame();
expect(await gamePage.getParticipantCount()).toBe(4);
```

**CRITICAL**: All tab navigation uses `getByRole('tab')`, never `getByRole('button')` for tabs.

---

### HistoryPage
**File**: `HistoryPage.ts`
**Purpose**: View game history and phase results

**Constructor**: `new HistoryPage(page: Page, gameId: number)`

**Key Methods**:
```typescript
// Navigation
async goto()

// Phase History
async getPhaseHistory(): Promise<string[]>
async getPhaseNumbers(): Promise<string[]>
async viewPhaseDetails(phaseTitle: string)
async goBackToHistory()

// Verification
async verifyOnPage()
async verifyPhaseExists(phaseTitle: string)
async hasPublishedResults(): Promise<boolean>
async hasActivePhase(): Promise<boolean>
async hasCommonRoomContent(): Promise<boolean>

// Getters
async getPhaseStatus(phaseTitle: string): Promise<string>
```

**Usage Example**:
```typescript
const historyPage = new HistoryPage(page, gameId);
await historyPage.goto();
await historyPage.verifyPhaseExists('Phase 1: Planning');
await historyPage.viewPhaseDetails('Phase 1: Planning');
expect(await historyPage.hasPublishedResults()).toBe(true);
await historyPage.goBackToHistory();
```

---

### MessagingPage
**File**: `MessagingPage.ts`
**Purpose**: Private messaging, conversation creation, and message management

**Constructor**: `new MessagingPage(page: Page)`

**Key Methods**:
```typescript
// Navigation
async goto(gameId: number)
async navigateToMessages()

// Conversation Management
async openNewConversationForm()
async selectParticipant(characterName: string)
async createConversation(title: string, participants: string[])
async openConversation(conversationTitle: string)

// Messaging
async sendMessage(message: string)

// Verification
async verifyConversationExists(conversationTitle: string)
async verifyConversationNotVisible(conversationTitle: string)
async verifyMessageExists(messageContent: string)
async verifyMessageNotVisible(messageContent: string)

// Getters
get heading(): Locator
get newConversationButton(): Locator
get conversationTitleInput(): Locator
get messageTextarea(): Locator
get sendButton(): Locator
get createConversationButton(): Locator
```

**Usage Example**:
```typescript
const messaging = new MessagingPage(page);
await messaging.goto(gameId);
await messaging.createConversation('Planning Session', ['Player 1', 'Player 2']);
await messaging.sendMessage('Hello everyone!');
await messaging.verifyMessageExists('Hello everyone!');
```

---

## Creating New POMs

When you need to create a new POM:

### 1. Identify the Workflow
What user actions does the test cover? Group related actions together.

### 2. Create the POM Class
```typescript
import { Page } from '@playwright/test';

export class MyFeaturePage {
  readonly page: Page;

  constructor(page: Page) {
    this.page = page;
  }

  async goto() {
    await this.page.goto('/my-feature');
    await this.page.waitForLoadState('networkidle');
  }

  // Add your methods here
}
```

### 3. Use Stable Selectors
**Prefer** (in order):
1. `getByRole('button', { name: 'Submit' })`
2. `getByTestId('submit-button')`
3. `getByText('exact text')`
4. `getByPlaceholder('Search...')`

**Avoid**:
- ❌ `locator('.btn-primary')` - class names change
- ❌ `locator('button:has-text("Submit")')` - brittle
- ❌ `locator('xpath=//...')` - very brittle

### 4. Method Naming Conventions
- `goto()` - Navigate to page/tab
- `get*()` - Retrieve data (returns value)
- `has*()` - Boolean checks (returns boolean)
- `can*()` - Permission checks (returns boolean)
- `[action]*()` - Perform action (e.g., `updateTitle()`, `saveChanges()`)

### 5. Handle Waits Internally
```typescript
async saveChanges() {
  await this.page.getByRole('button', { name: 'Save' }).click();
  await this.page.waitForLoadState('networkidle');
}
```

### 6. Return Promises
All async methods should return Promises. Use appropriate return types.

## Testing Best Practices

### ✅ Good Test Structure
```typescript
test('user can edit game settings', async ({ page }) => {
  // Setup
  await loginAs(page, 'GM');
  const gameId = await getFixtureGameId(page, 'TEST_GAME');

  // Initialize POMs
  const gamePage = new GameDetailsPage(page);
  const settingsPage = new GameSettingsPage(page);

  // Navigate
  await gamePage.goto(gameId);

  // Act using POM methods
  await settingsPage.openEditModal();
  await settingsPage.updateTitle('New Title');
  await settingsPage.saveChanges();

  // Assert
  await expect(page.getByRole('heading', { name: 'New Title' })).toBeVisible();
});
```

### ❌ Avoid Inline Selectors
```typescript
// DON'T DO THIS:
await page.click('button:has-text("Edit Game")');
await page.fill('#title', 'New Title');
await page.click('button:has-text("Save")');

// DO THIS:
await settingsPage.openEditModal();
await settingsPage.updateTitle('New Title');
await settingsPage.saveChanges();
```

## Documentation

For more information:
- **E2E Quick Start**: `/docs-site/developer/testing/E2E_QUICK_START.md`
- **E2E Guide**: `/frontend/e2e/README.md`
- **Coverage status**: `/frontend/e2e/STATUS.md`
- **Playwright Docs**: https://playwright.dev/docs/pom

---

**Remember**: Always use POMs. Tests should read like user stories, not technical implementations!
