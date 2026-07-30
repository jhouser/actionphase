import { Page, Locator, expect } from '@playwright/test';
import { navigateToGameAndTab, navigateToGameTab } from '../utils/navigation';
import { waitForVisible } from '../utils/waits';

/**
 * Page Object Model for Private Messaging
 *
 * Encapsulates all private messaging interactions including:
 * - Creating conversations
 * - Sending messages
 * - Managing participants
 * - Conversation navigation
 */
export class MessagingPage {
  constructor(private page: Page) {}

  /**
   * Navigate to the Messages tab for a specific game
   */
  async goto(gameId: number) {
    await navigateToGameAndTab(this.page, gameId, 'Messages');
  }

  /**
   * Get the Messages heading
   */
  get heading(): Locator {
    return this.page.getByRole('heading', { name: 'Private Messages' });
  }

  /**
   * Get the New Conversation button
   */
  get newConversationButton(): Locator {
    return this.page.getByRole('button', { name: '+ New' });
  }

  /**
   * Get the conversation title input
   */
  get conversationTitleInput(): Locator {
    return this.page.getByPlaceholder(/Planning the heist/i).or(
      this.page.getByRole('textbox', { name: /title/i })
    );
  }

  /**
   * Get the message textarea
   */
  get messageTextarea(): Locator {
    return this.page.getByPlaceholder(/Type your message/i);
  }

  /**
   * Get the Send button
   */
  get sendButton(): Locator {
    // Use type="submit" to specifically target the message send button
    // and avoid conflicts with "Resend Email" button from email verification banner
    return this.page.locator('button[type="submit"]', { hasText: 'Send' });
  }

  /**
   * Get the Create Conversation button
   */
  get createConversationButton(): Locator {
    return this.page.getByRole('button', { name: 'Create Conversation' });
  }

  /**
   * Open the new conversation form
   */
  async openNewConversationForm() {
    await this.newConversationButton.click();
    await waitForVisible(this.conversationTitleInput);
  }

  /**
   * Select which character to send messages as (for users with multiple characters)
   * @param characterName - Character name to select as sender
   */
  async selectSendingCharacter(characterName: string) {
    // Look for character select dropdown using the placeholder text to distinguish from tab select
    // Filter to only visible select - works for both mobile and desktop viewports
    const characterSelect = this.page.locator('select').filter({
      has: this.page.locator('option', { hasText: 'Select your character' })
    }).locator('visible=true').first();

    const selectCount = await this.page.locator('select').filter({
      has: this.page.locator('option', { hasText: 'Select your character' })
    }).count();

    if (selectCount > 0) {
      // Multiple characters - select from dropdown
      const options = await characterSelect.locator('option').all();
      for (const option of options) {
        const text = await option.textContent();
        if (text && text.includes(characterName)) {
          const value = await option.getAttribute('value');
          if (value) {
            await characterSelect.selectOption(value);
            return;
          }
        }
      }
      throw new Error(`Could not find character "${characterName}" in dropdown`);
    } else {
      // Single character - verify it's the right one (it's auto-selected)
      const displayedCharacter = await this.page.getByText(characterName).locator('visible=true').first();
      if (await displayedCharacter.count() === 0) {
        throw new Error(`Expected character "${characterName}" to be auto-selected, but it's not displayed`);
      }
      // Character is already selected, nothing to do
    }
  }

  /**
   * Select a character as participant
   * @param characterName - Character name to select
   */
  async selectParticipant(characterName: string) {
    const label = this.page.getByLabel(characterName, { exact: false });
    await label.click();
  }

  /**
   * Create a new conversation
   * @param title - Conversation title
   * @param participants - Array of character names to include
   * @param sendingCharacter - (Optional) Character to send as (for users with multiple characters)
   */
  async createConversation(title: string, participants: string[], sendingCharacter?: string) {
    await this.openNewConversationForm();

    // Fill in title
    await this.conversationTitleInput.fill(title);

    // Select sending character if specified (for GMs with multiple NPCs, etc.)
    if (sendingCharacter) {
      await this.selectSendingCharacter(sendingCharacter);
    }

    // Select participants
    for (const participant of participants) {
      await this.selectParticipant(participant);
    }

    // Submit form
    await this.createConversationButton.click();
    await this.page.waitForLoadState('networkidle');

    // Verify conversation was created - filter to visible element (viewport-agnostic)
    await expect(this.page.getByText(title).locator('visible=true').first()).toBeVisible({ timeout: 5000 });
  }

  /**
   * Send a message in the current conversation
   * @param message - Message text
   */
  async sendMessage(message: string) {
    // The composer is collapsed behind a "Reply" button at all viewport widths
    // until the user opens it (see MessageThread). When messaging is allowed the
    // Reply button is present and the textarea is not yet mounted; open it first.
    // We still tolerate an already-visible textarea (e.g. reply box left open) so
    // callers that send several messages in a row don't have to re-open it.
    const replyButton = this.page.getByRole('button', { name: 'Reply' }).locator('visible=true');
    const visibleTextarea = this.page.locator('textarea[placeholder*="Type your message"]').locator('visible=true');
    await expect(replyButton.or(visibleTextarea).first()).toBeVisible({ timeout: 5000 });
    if (await replyButton.count() > 0) {
      await replyButton.first().click();
      await this.messageTextarea.waitFor({ state: 'visible', timeout: 5000 });
    }

    await this.messageTextarea.fill(message);
    await this.sendButton.click();
    await this.page.waitForLoadState('networkidle');

    // Verify message appears - filter to visible element (viewport-agnostic)
    await expect(this.page.getByText(message).locator('visible=true').first()).toBeVisible({ timeout: 5000 });
  }

  /**
   * Open a conversation by title
   * @param conversationTitle - Title of the conversation to open
   */
  async openConversation(conversationTitle: string) {
    const conversation = this.page.getByText(conversationTitle).locator('visible=true').first();
    await conversation.click();
    await this.page.waitForLoadState('networkidle');
  }

  /**
   * Verify a conversation exists in the list
   * @param conversationTitle - Title to verify
   */
  async verifyConversationExists(conversationTitle: string) {
    // Filter to visible element (viewport-agnostic)
    await expect(this.page.getByText(conversationTitle).locator('visible=true').first()).toBeVisible({ timeout: 5000 });
  }

  /**
   * Verify a conversation does NOT exist in the list
   * @param conversationTitle - Title to verify is not visible
   */
  async verifyConversationNotVisible(conversationTitle: string) {
    const conversation = this.page.getByText(conversationTitle);
    await expect(conversation).not.toBeVisible();
  }

  /**
   * Verify a message exists in the conversation thread
   * @param messageContent - Message content to verify
   */
  async verifyMessageExists(messageContent: string) {
    // Filter to visible element (viewport-agnostic)
    const message = this.page.getByText(messageContent).locator('visible=true').first();
    await expect(message).toBeVisible({ timeout: 5000 });
  }

  /**
   * Verify a message does NOT exist
   * @param messageContent - Message content to verify is not visible
   */
  async verifyMessageNotVisible(messageContent: string) {
    const message = this.page.getByText(messageContent);
    await expect(message).not.toBeVisible();
  }

  /**
   * Navigate to Messages tab using button/tab click
   */
  async navigateToMessages() {
    // Navigate to Messages tab (handles mobile select and desktop tabs)
    await navigateToGameTab(this.page, 'Messages');
  }

  /**
   * Click the edit button on a message
   * @param messageLocator - Locator for the message element
   */
  async clickEditButton(messageLocator: Locator) {
    await messageLocator.hover();
    const editButton = messageLocator.locator('button[title="Edit message"]');
    await expect(editButton).toBeVisible();
    await editButton.click();
  }

  /**
   * Edit a message's content via the inline editor
   * @param messageLocator - Locator for the message element
   * @param newContent - New content to replace with
   */
  async editMessage(messageLocator: Locator, newContent: string) {
    await this.clickEditButton(messageLocator);
    const textarea = this.page.getByTestId('edit-message-textarea');
    await expect(textarea).toBeVisible();
    await textarea.clear();
    await textarea.fill(newContent);
    await this.page.getByTestId('save-edit-button').click();
    await this.page.waitForLoadState('networkidle');
  }
}
