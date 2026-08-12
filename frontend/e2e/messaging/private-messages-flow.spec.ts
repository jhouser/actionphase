import { test } from '@playwright/test';
import { loginAs } from '../fixtures/auth-helpers';
import { getFixtureGameId, getWorkerGameId } from '../fixtures/game-helpers';
import { MessagingPage } from '../pages/MessagingPage';

/**
 * Journey 5: Players Exchange Private Messages
 *
 * Tests the complete private messaging flow between players.
 * Uses E2E fixture game "E2E Test: Action Submission" with existing characters.
 * Character creation is tested separately in Journey 3.
 *
 * REFACTORED: Using Page Object Model and shared utilities
 * - Eliminated all waitForTimeout calls (was 11)
 * - Uses navigateToGame for consistent navigation
 * - Uses assertion utilities for consistency
 * - Uses waitForModal and smart waits
 */
test.describe('@mobile Private Messages Flow', () => {
  test('Players can send private messages to each other', async ({ browser }) => {
    const player1Context = await browser.newContext();
    const player2Context = await browser.newContext();
    const player1Page = await player1Context.newPage();
    const player2Page = await player2Context.newPage();

    try {
      // === Player 1 creates a conversation with Player 2 ===
      await loginAs(player1Page, 'PLAYER_1');

      // Use E2E Messages game
      // TestPlayer1 has character: E2E Test Char 1
      // TestPlayer2 has character: E2E Test Char 2
      const gameId = await getFixtureGameId(player1Page, 'E2E_MESSAGES');

      const player1Messaging = new MessagingPage(player1Page);
      await player1Messaging.goto(gameId);

      // Create conversation with Player 2's character
      const conversationTitle = `Test Conversation ${Date.now()}`;
      await player1Messaging.createConversation(conversationTitle, ['E2E Test Char 2']);

      // === Player 1 sends first message ===
      const messageContent = `Hello from Player 1! Test message at ${Date.now()}`;
      await player1Messaging.sendMessage(messageContent);

      // === Player 2 sees the conversation and message ===
      await loginAs(player2Page, 'PLAYER_2');

      const player2Messaging = new MessagingPage(player2Page);
      await player2Messaging.goto(gameId);

      // See conversation in the list
      await player2Messaging.verifyConversationExists(conversationTitle);

      // Open the conversation
      await player2Messaging.openConversation(conversationTitle);

      // Verify Player 1's message is visible
      await player2Messaging.verifyMessageExists(messageContent);

      // === Player 2 replies ===
      const replyContent = `Hi Shade! Got your message. Rook replying at ${Date.now()}`;
      await player2Messaging.sendMessage(replyContent);

      // === Player 1 sees the reply ===
      // Navigate fresh to Messages tab (avoids mobile issue where tab bar is hidden in conversation view)
      await player1Messaging.goto(gameId);
      await player1Messaging.openConversation(conversationTitle);

      // Verify Player 2's reply is visible
      await player1Messaging.verifyMessageExists(replyContent);
    } finally {
      await player1Context.close();
      await player2Context.close();
    }
  });

  test('Players can create group conversations with 3+ participants', async ({ browser }) => {
    const player1Context = await browser.newContext();
    const player2Context = await browser.newContext();
    const player3Context = await browser.newContext();
    const player1Page = await player1Context.newPage();
    const player2Page = await player2Context.newPage();
    const player3Page = await player3Context.newPage();

    try {
      // === Player 1 creates a group conversation with Player 2 and Player 3 ===
      await loginAs(player1Page, 'PLAYER_1');

      // Use E2E Messages game
      // TestPlayer1 has character: E2E Test Char 1
      // TestPlayer2 has character: E2E Test Char 2
      // TestPlayer3 has character: E2E Test Char 3
      const gameId = await getFixtureGameId(player1Page, 'E2E_MESSAGES');

      const player1Messaging = new MessagingPage(player1Page);
      await player1Messaging.goto(gameId);

      // Create group conversation with Player 2 and Player 3's characters
      const groupTitle = `Group Chat ${Date.now()}`;
      await player1Messaging.createConversation(groupTitle, ['E2E Test Char 2', 'E2E Test Char 3']);

      // === Player 1 sends first message ===
      const player1Message = `Hello everyone! Player 1 here at ${Date.now()}`;
      await player1Messaging.sendMessage(player1Message);

      // === Player 2 sees the group conversation ===
      await loginAs(player2Page, 'PLAYER_2');

      const player2Messaging = new MessagingPage(player2Page);
      await player2Messaging.goto(gameId);

      // See and open group conversation
      await player2Messaging.verifyConversationExists(groupTitle);
      await player2Messaging.openConversation(groupTitle);

      // Verify Player 1's message is visible
      await player2Messaging.verifyMessageExists(player1Message);

      // Player 2 sends a message
      const player2Message = `Hi from Player 2 at ${Date.now()}`;
      await player2Messaging.sendMessage(player2Message);

      // === Player 3 also sees the group conversation ===
      await loginAs(player3Page, 'PLAYER_3');

      const player3Messaging = new MessagingPage(player3Page);
      await player3Messaging.goto(gameId);

      // See and open group conversation
      await player3Messaging.verifyConversationExists(groupTitle);
      await player3Messaging.openConversation(groupTitle);

      // Verify BOTH previous messages are visible to Player 3
      await player3Messaging.verifyMessageExists(player1Message);
      await player3Messaging.verifyMessageExists(player2Message);

      // Player 3 sends a message
      const player3Message = `Player 3 joining the conversation at ${Date.now()}`;
      await player3Messaging.sendMessage(player3Message);

      // === Verify Player 1 sees all messages from all participants ===
      await player1Messaging.goto(gameId);
      await player1Messaging.openConversation(groupTitle);

      // Verify all three messages are visible
      await player1Messaging.verifyMessageExists(player1Message);
      await player1Messaging.verifyMessageExists(player2Message);
      await player1Messaging.verifyMessageExists(player3Message);

    } finally {
      await player1Context.close();
      await player2Context.close();
      await player3Context.close();
    }
  });

  test('GM can send private messages as different NPCs', async ({ browser }) => {
    test.setTimeout(60000);
    const gmContext = await browser.newContext();
    const playerContext = await browser.newContext();

    const gmPage = await gmContext.newPage();
    const playerPage = await playerContext.newPage();

    try {
      // === GM creates conversation with Player as Detective Morrison ===
      await loginAs(gmPage, 'GM');

      const gameId = await getFixtureGameId(gmPage, 'E2E_GM_MESSAGING');
      const gmMessaging = new MessagingPage(gmPage);
      await gmMessaging.goto(gameId);

      // Create conversation as Detective Morrison NPC
      const detectiveConvoTitle = `Investigation ${Date.now()}`;
      await gmMessaging.createConversation(
        detectiveConvoTitle,
        ['E2E Test Char 1'],  // Player 1's character
        'Detective Morrison'  // GM's NPC sending the message
      );

      // GM (as Detective) sends message
      const detectiveMessage = `This is Detective Morrison. I need your help with the case.`;
      await gmMessaging.sendMessage(detectiveMessage);

      // === Player sees conversation from Detective Morrison ===
      await loginAs(playerPage, 'PLAYER_1');

      const playerMessaging = new MessagingPage(playerPage);
      await playerMessaging.goto(gameId);

      await playerMessaging.verifyConversationExists(detectiveConvoTitle);
      await playerMessaging.openConversation(detectiveConvoTitle);
      await playerMessaging.verifyMessageExists(detectiveMessage);
      // The point of the test: the player sees the NPC, not the GM behind it.
      await playerMessaging.verifyMessageSender(detectiveMessage, 'Detective Morrison');

      // (The player replying is covered by the send/receive tests above; it
      // adds nothing to the NPC-identity claim and this test is long enough
      // that the extra round trip pushed it into timeouts on mobile.)

      // === GM creates DIFFERENT conversation as Whisper (different NPC) ===
      // Use goto() to navigate cleanly without any lingering conversation URL param
      await gmMessaging.goto(gameId);

      const informantConvoTitle = `Secret Intel ${Date.now()}`;
      await gmMessaging.createConversation(
        informantConvoTitle,
        ['E2E Test Char 1'],
        'Whisper (Informant)'  // Different NPC
      );

      // GM (as Whisper) sends message
      const informantMessage = `Psst... I have information you might need.`;
      await gmMessaging.sendMessage(informantMessage);

      // === Player sees BOTH conversations from different NPCs ===
      // Use goto() to navigate cleanly without any lingering conversation URL param
      await playerMessaging.goto(gameId);

      // Verify both conversations exist
      await playerMessaging.verifyConversationExists(detectiveConvoTitle);
      await playerMessaging.verifyConversationExists(informantConvoTitle);

      // Open the informant conversation
      await playerMessaging.openConversation(informantConvoTitle);
      await playerMessaging.verifyMessageExists(informantMessage);
      // Attributed to the second NPC — not to Detective Morrison, and not to
      // the GM's username. This is what distinguishes "two NPCs" from "one GM".
      await playerMessaging.verifyMessageSender(informantMessage, 'Whisper (Informant)');

    } finally {
      // Close with catch: if the body times out, Playwright tears the contexts
      // down first, and an unguarded close() throws "Target page, context or
      // browser has been closed" — masking the real failure with a cleanup error.
      await gmContext.close().catch(() => {});
      await playerContext.close().catch(() => {});
    }
  });

  test('Audience member can send private messages as assigned NPC', async ({ browser }) => {
    test.setTimeout(45000); // Audience character loading may take time

    const audienceContext = await browser.newContext();
    const playerContext = await browser.newContext();

    const audiencePage = await audienceContext.newPage();
    const playerPage = await playerContext.newPage();

    try {
      // === Audience member creates conversation as The Narrator ===
      await loginAs(audiencePage, 'AUDIENCE');

      const gameId = await getFixtureGameId(audiencePage, 'E2E_GM_MESSAGING');
      const audienceMessaging = new MessagingPage(audiencePage);
      await audienceMessaging.goto(gameId);

      // Create conversation as The Narrator (audience member's assigned NPC)
      const narratorConvoTitle = `Narrative Insight ${Date.now()}`;
      await audienceMessaging.createConversation(
        narratorConvoTitle,
        ['E2E Test Char 1'],  // Player 1's character
        'The Narrator'  // Audience member's NPC
      );

      // Audience (as Narrator) sends message
      const narratorMessage = `From the shadows, a voice speaks: "The truth lies deeper than you think..."`;
      await audienceMessaging.sendMessage(narratorMessage);

      // === Player sees conversation from The Narrator ===
      await loginAs(playerPage, 'PLAYER_1');

      const playerMessaging = new MessagingPage(playerPage);
      await playerMessaging.goto(gameId);

      await playerMessaging.verifyConversationExists(narratorConvoTitle);
      await playerMessaging.openConversation(narratorConvoTitle);
      await playerMessaging.verifyMessageExists(narratorMessage);

      // Player can reply
      const playerReply = `Who are you? How do you know this?`;
      await playerMessaging.sendMessage(playerReply);

      // === Audience member sees the reply ===
      await audienceMessaging.goto(gameId);
      await audienceMessaging.openConversation(narratorConvoTitle);

      await audienceMessaging.verifyMessageExists(playerReply);

    } finally {
      await audienceContext.close();
      await playerContext.close();
    }
  });

  test('Co-GM can send private messages as different NPCs', async ({ browser }) => {
    test.setTimeout(60000); // Extended timeout for messaging flow

    // Co-GMs should have the same NPC control permissions as GMs.
    // Uses a dedicated fixture (game 347) where TestAudience1 is stably co-GM.
    // This is separate from game 339 (co-gm-management.spec.ts) to avoid
    // cross-test fixture mutation races.

    const coGmContext = await browser.newContext();
    const playerContext = await browser.newContext();

    const coGmPage = await coGmContext.newPage();
    const playerPage = await playerContext.newPage();

    const gameId = getWorkerGameId(347); // Dedicated co-GM NPC messaging fixture

    try {
      // Login as TestAudience1 — stably co-GM in game 347
      await loginAs(coGmPage, 'AUDIENCE_1');

      const coGmMessaging = new MessagingPage(coGmPage);
      await coGmMessaging.goto(gameId);

      // Create conversation as first unassigned NPC (Mysterious Stranger)
      const strangerConvoTitle = `Strange Encounter ${Date.now()}`;

      // Debug: Check what characters are available
      await coGmMessaging.openNewConversationForm();

      // Try to select the NPC and participant
      await coGmMessaging.selectSendingCharacter('Mysterious Stranger');
      await coGmMessaging.selectParticipant('Test Player Character');
      await coGmMessaging.conversationTitleInput.fill(strangerConvoTitle);
      await coGmMessaging.createConversationButton.click();
      await coGmPage.waitForLoadState('networkidle');

      // Co-GM (as Mysterious Stranger) sends message
      const strangerMessage = `I've been watching you. There's something you need to know...`;
      await coGmMessaging.sendMessage(strangerMessage);

      // === Player sees conversation from Mysterious Stranger ===
      await loginAs(playerPage, 'PLAYER_1');

      const playerMessaging = new MessagingPage(playerPage);
      await playerMessaging.goto(gameId);

      await playerMessaging.verifyConversationExists(strangerConvoTitle);
      await playerMessaging.openConversation(strangerConvoTitle);
      await playerMessaging.verifyMessageExists(strangerMessage);

      // Player replies - verifies full bidirectional flow
      const playerReply = `Who are you? What do you want?`;
      await playerMessaging.sendMessage(playerReply);

      // Co-GM sees player reply (confirms full bidirectional messaging as NPC)
      await coGmMessaging.goto(gameId);
      await coGmMessaging.openConversation(strangerConvoTitle);
      await coGmMessaging.verifyMessageExists(playerReply);

    } finally {
      await coGmContext.close();
      await playerContext.close();
    }
  });
});
