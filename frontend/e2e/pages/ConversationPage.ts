import { Page, Locator } from '@playwright/test';
import { navigateToGameTab } from '../utils/navigation';

/**
 * Page Object for Conversation/Messaging interactions
 *
 * Handles viewing conversations, sending messages, and managing message threads
 */
export class ConversationPage {
  readonly page: Page;

  // Locators
  readonly conversationList: Locator;
  readonly conversationItem: Locator;
  readonly messagesList: Locator;
  readonly messageInput: Locator;
  readonly sendMessageButton: Locator;
  readonly newConversationButton: Locator;
  readonly conversationTitle: Locator;

  constructor(page: Page) {
    this.page = page;

    // Define locators
    this.conversationList = page.locator('.divide-y');
    this.conversationItem = page.locator('[class*="w-full"]').filter({ hasText: /Untitled|Conversation/ });
    this.messagesList = page.locator('[data-testid="messages-list"]');
    this.messageInput = page.locator('textarea[placeholder*="message"]');
    this.sendMessageButton = page.locator('button[type="submit"]:has-text("Send")');
    this.newConversationButton = page.locator('button:has-text("New Conversation")');
    // Filter to visible element (viewport-agnostic for dual-DOM pattern)
    this.conversationTitle = page.locator('h1, h2, h3').filter({ hasText: /Conversation/ }).locator('visible=true').first();
  }

  /**
   * Navigate to conversations page for a game
   */
  async goto(gameId: number): Promise<void> {
    await this.page.goto(`/games/${gameId}`);
    await this.page.waitForLoadState('networkidle');

    // Navigate to Messages tab first (handles mobile select and desktop tabs),
    // then click the Conversations sub-tab within the Messages view
    await navigateToGameTab(this.page, 'Messages');
    const conversationsTab = this.page.getByRole('button', { name: 'Conversations' }).locator('visible=true').first();
    await conversationsTab.waitFor({ state: 'visible', timeout: 5000 });
    await conversationsTab.click();
    await this.page.waitForLoadState('networkidle');
  }

  /**
   * Wait for conversations to load
   */
  async waitForConversationsToLoad(): Promise<void> {
    // Wait for either conversations or "No conversations" message
    await Promise.race([
      // Filter to visible element (viewport-agnostic for dual-DOM pattern)
      this.conversationItem.locator('visible=true').first().waitFor({ state: 'visible', timeout: 5000 }),
      this.page.locator('text=/No conversations/i').waitFor({ state: 'visible', timeout: 5000 })
    ]).catch(() => {
      // Timeout is OK - may not have conversations
    });
  }

  /**
   * Get list of conversation titles
   */
  async getConversationTitles(): Promise<string[]> {
    await this.waitForConversationsToLoad();

    // Check if there are no conversations
    const noConversations = await this.page.locator('text=/No conversations/i').isVisible().catch(() => false);
    if (noConversations) {
      return [];
    }

    const conversations = await this.conversationItem.all();
    const titles: string[] = [];

    for (const conv of conversations) {
      // Filter to visible element (viewport-agnostic for dual-DOM pattern)
      const titleElement = conv.locator('h3, .font-semibold').locator('visible=true').first();
      const titleText = await titleElement.textContent().catch(() => null);
      if (titleText) {
        titles.push(titleText.trim());
      }
    }

    return titles;
  }

  /**
   * Select a conversation by title
   */
  async selectConversation(title: string): Promise<void> {
    await this.waitForConversationsToLoad();

    const conversation = this.page.locator('button, a').filter({ hasText: title });
    await conversation.click();
    await this.page.waitForLoadState('networkidle');
  }

  /**
   * Select a conversation by index
   */
  async selectConversationByIndex(index: number): Promise<void> {
    await this.waitForConversationsToLoad();

    const conversations = await this.conversationItem.all();
    if (index >= conversations.length) {
      throw new Error(`Conversation index ${index} out of range (total: ${conversations.length})`);
    }

    await conversations[index].click();
    await this.page.waitForLoadState('networkidle');
  }

  /**
   * Get conversation count
   */
  async getConversationCount(): Promise<number> {
    await this.waitForConversationsToLoad();

    // Check if there are no conversations
    const noConversations = await this.page.locator('text=/No conversations/i').isVisible().catch(() => false);
    if (noConversations) {
      return 0;
    }

    return await this.conversationItem.count();
  }

  /**
   * Send a message in the current conversation
   */
  async sendMessage(content: string, characterName?: string): Promise<void> {
    // The composer is collapsed behind a "Reply" button (all widths) until opened.
    // Open it first so the character select and textarea are mounted.
    const replyButton = this.page.getByRole('button', { name: 'Reply' }).locator('visible=true');
    if (await replyButton.count() > 0) {
      await replyButton.first().click();
      await this.messageInput.waitFor({ state: 'visible', timeout: 5000 });
    }

    // Select character if specified and dropdown exists
    if (characterName) {
      // Filter to visible element (viewport-agnostic for dual-DOM pattern)
      const characterSelect = this.page.locator('select').locator('visible=true').first();
      const isVisible = await characterSelect.isVisible().catch(() => false);
      if (isVisible) {
        await characterSelect.selectOption({ label: characterName });
      }
    }

    await this.messageInput.fill(content);
    await this.sendMessageButton.click();
    await this.page.waitForLoadState('networkidle');
  }

  /**
   * Get all messages in current conversation
   */
  async getMessages(): Promise<string[]> {
    // Wait for messages to load
    await this.page.waitForTimeout(1000);

    const messages = this.page.locator('[data-testid="message-item"], .message-item, [class*="message"]');
    const count = await messages.count();

    if (count === 0) {
      return [];
    }

    const messageTexts = await messages.allTextContents();
    return messageTexts.filter((t): t is string => t !== null && t.trim() !== '');
  }

  /**
   * Get message count in current conversation
   */
  async getMessageCount(): Promise<number> {
    await this.page.waitForTimeout(1000);
    const messages = this.page.locator('[data-testid="message-item"], .message-item, [class*="message"]');
    return await messages.count();
  }

  /**
   * Check if conversation has unread badge
   */
  async hasUnreadBadge(conversationTitle: string): Promise<boolean> {
    const conversation = this.page.locator('button, a').filter({ hasText: conversationTitle });
    const unreadBadge = conversation.locator('[class*="unread"], span[class*="badge"]').filter({ hasText: /\d+/ });
    return await unreadBadge.isVisible().catch(() => false);
  }

  /**
   * Get unread count for a conversation
   */
  async getUnreadCount(conversationTitle: string): Promise<number> {
    const conversation = this.page.locator('button, a').filter({ hasText: conversationTitle });
    const unreadBadge = conversation.locator('span').filter({ hasText: /^\d+$/ });

    const isVisible = await unreadBadge.isVisible().catch(() => false);
    if (!isVisible) {
      return 0;
    }

    const text = await unreadBadge.textContent();
    return text ? parseInt(text, 10) : 0;
  }

  /**
   * Create a new conversation (if button exists)
   */
  async createConversation(title: string, participantNames: string[]): Promise<void> {
    await this.newConversationButton.click();
    await this.page.waitForLoadState('networkidle');

    // Fill in title
    const titleInput = this.page.locator('input[name="title"], input[placeholder*="title"]');
    await titleInput.fill(title);

    // Select participants (implementation depends on UI - this is a placeholder)
    for (const participant of participantNames) {
      const participantCheckbox = this.page.locator(`label:has-text("${participant}")`);
      await participantCheckbox.click();
    }

    // Submit form
    const createButton = this.page.locator('button[type="submit"]:has-text("Create")');
    await createButton.click();
    await this.page.waitForLoadState('networkidle');
  }

  /**
   * Mark conversation as read (if option exists)
   */
  async markAsRead(): Promise<void> {
    const markReadButton = this.page.locator('button:has-text("Mark as read")');
    const isVisible = await markReadButton.isVisible().catch(() => false);
    if (isVisible) {
      await markReadButton.click();
    }
  }

  /**
   * Get current conversation title
   */
  async getCurrentConversationTitle(): Promise<string> {
    return await this.conversationTitle.textContent() || '';
  }

  /**
   * Check if conversation list is empty
   */
  async isConversationListEmpty(): Promise<boolean> {
    return await this.page.locator('text=/No conversations/i').isVisible().catch(() => false);
  }
}
