import { Page } from '@playwright/test';
import { waitForVisible } from '../utils/waits';
import { assertTextVisible } from '../utils/assertions';
import { navigateToGameTab } from '../utils/navigation';

/**
 * Page Object Model for Audience Tab
 *
 * Encapsulates all audience-related interactions including:
 * - Viewing "All Private Messages" list
 * - Filtering conversations by participants
 * - Viewing individual conversations
 * - Verifying read-only access
 */
export class AudiencePage {
  constructor(private page: Page) {}

  /**
   * Navigate to the Audience tab
   * The "All Private Messages" view is displayed automatically
   */
  async goToAudience(gameId: number) {
    await this.page.goto(`/games/${gameId}`);
    await this.page.waitForLoadState('networkidle');

    // Navigate to Audience tab (handles mobile select and desktop tabs)
    await navigateToGameTab(this.page, 'Audience');

    // Click "Private Messages" sub-tab (default is already selected, but ensure we're there)
    const privateMessagesTab = this.page.getByRole('button', { name: 'Private Messages' });
    await privateMessagesTab.click();
    await this.page.waitForLoadState('networkidle');
    // Wait for the conversation list to render (React updates after networkidle)
    await this.page.locator('[data-testid="conversation-item"]').first().waitFor({ state: 'visible', timeout: 10000 }).catch(() => {
      // No conversations in this game — that's valid for some tests
    });
  }

  /**
   * Verify "All Private Messages" view is displayed
   */
  async verifyAllPrivateMessagesView() {
    await assertTextVisible(this.page, 'All Private Messages');
    await assertTextVisible(this.page, 'Read-Only');
  }

  /**
   * Verify a conversation card exists in the list
   */
  async verifyConversationExists(subject: string) {
    const card = this.page.getByText(subject);
    await waitForVisible(card);
  }

  /**
   * Click a conversation to open it
   */
  async openConversation(subject: string) {
    const card = this.page.locator('[class*="cursor-pointer"]').filter({ hasText: subject });
    await card.click();
    await this.page.waitForLoadState('networkidle');
  }

  /**
   * Verify conversation header is displayed
   */
  async verifyConversationHeader(subject: string) {
    await assertTextVisible(this.page, subject);
    await assertTextVisible(this.page, 'Read-Only');
  }

  /**
   * Verify participant filter section exists
   */
  async verifyParticipantFilter() {
    await assertTextVisible(this.page, 'Filter by Participants');
  }

  /**
   * Click a participant to filter conversations
   */
  async filterByParticipant(participantName: string) {
    const button = this.page.getByRole('button', { name: participantName });
    await button.click();
    await this.page.waitForLoadState('networkidle');
  }

  /**
   * Verify conversation count
   */
  async verifyConversationCount(expected: string) {
    await assertTextVisible(this.page, expected);
  }

  /**
   * Clear all filters
   */
  async clearFilters() {
    const button = this.page.getByRole('button', { name: /Clear filters/i });
    await button.click();
    await this.page.waitForLoadState('networkidle');
  }

  /**
   * Verify message exists in conversation view
   */
  async verifyMessageExists(content: string) {
    await assertTextVisible(this.page, content);
  }

  /**
   * Verify date divider exists
   */
  async verifyDateDivider(dateText: string) {
    await assertTextVisible(this.page, dateText);
  }

  /**
   * Verify message grouping by checking sender name appears only once per group
   */
  async verifySenderNameInGroup(senderName: string) {
    const senderElements = this.page.getByText(senderName, { exact: true });
    const count = await senderElements.count();
    // Sender name should appear at most twice (mobile + desktop layouts)
    return count <= 2;
  }

  /**
   * Go back to conversation list
   */
  async goBackToConversationList() {
    const backButton = this.page.getByRole('button', { name: /Back/i }).first();
    await backButton.click();
    await this.page.waitForLoadState('networkidle');
  }

  /**
   * Verify read-only state (no message input)
   */
  async verifyReadOnly() {
    // Should NOT have a message input/textarea
    const messageInput = this.page.getByPlaceholder(/Type your message/i);
    const messageInputCount = await messageInput.count();
    return messageInputCount === 0;
  }

  /**
   * Verify participant avatars are displayed
   */
  async verifyParticipantAvatars() {
    const avatars = this.page.getByTestId('character-avatar');
    const count = await avatars.count();
    return count > 0;
  }

  /**
   * Verify last message preview is displayed on conversation card
   */
  async verifyLastMessagePreview(conversationSubject: string, previewText: string) {
    const card = this.page.locator('[class*="cursor-pointer"]').filter({ hasText: conversationSubject });
    const preview = card.locator('text=' + previewText);
    await waitForVisible(preview);
  }

  /**
   * Verify activity badge is displayed (for conversations with messages)
   */
  async verifyActivityBadge() {
    const badge = this.page.locator('[class*="message"]');
    const count = await badge.count();
    return count > 0;
  }
}
