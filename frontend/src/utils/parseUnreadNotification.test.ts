import { describe, it, expect } from 'vitest';
import { classifyNotification, collapseInboxItems, parseConversationIdFromLinkUrl } from './parseUnreadNotification';
import type { Notification } from '@/types/notifications';
import type { UnreadInboxItem } from '@/types/unreadInbox';

function makeNotification(overrides: Partial<Notification> = {}): Notification {
  return {
    id: 1,
    user_id: 1,
    game_id: 12,
    type: 'comment_reply',
    title: 'Someone replied',
    is_read: false,
    created_at: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

describe('parseConversationIdFromLinkUrl', () => {
  it('extracts the conversation id from a well-formed link_url', () => {
    expect(parseConversationIdFromLinkUrl('/games/12?tab=messages&conversation=34')).toBe(34);
  });

  it('returns null when link_url is missing', () => {
    expect(parseConversationIdFromLinkUrl(undefined)).toBeNull();
  });

  it('returns null when the conversation param is missing', () => {
    expect(parseConversationIdFromLinkUrl('/games/12?tab=messages')).toBeNull();
  });

  it('returns null when the conversation param is not a number', () => {
    expect(parseConversationIdFromLinkUrl('/games/12?tab=messages&conversation=abc')).toBeNull();
  });

  it('returns null for a malformed url', () => {
    expect(parseConversationIdFromLinkUrl('::not a url::')).toBeNull();
  });
});

describe('classifyNotification', () => {
  it('classifies a comment_reply notification as a comment item', () => {
    const notification = makeNotification({
      type: 'comment_reply',
      related_type: 'comment',
      related_id: 99,
    });

    expect(classifyNotification(notification)).toEqual({
      kind: 'comment',
      notification,
      gameId: 12,
      commentId: 99,
    });
  });

  it('classifies a comment_reply notification even when related_type is "message"', () => {
    // classifyNotification doesn't need related_type at all — `type` alone is
    // sufficient to know this points at a comment/reply message. Regression
    // coverage for a real bug in a test fixture that mis-set related_type to
    // "message" on a comment_reply row (backend/pkg/db/test_fixtures/e2e/25_notification_flow.sql).
    const notification = makeNotification({
      type: 'comment_reply',
      related_type: 'message',
      related_id: 35815,
    });

    expect(classifyNotification(notification)).toEqual({
      kind: 'comment',
      notification,
      gameId: 12,
      commentId: 35815,
    });
  });

  it('classifies a character_mention notification as a comment item', () => {
    const notification = makeNotification({
      type: 'character_mention',
      related_type: 'comment',
      related_id: 55,
    });

    expect(classifyNotification(notification)).toEqual({
      kind: 'comment',
      notification,
      gameId: 12,
      commentId: 55,
    });
  });

  it('classifies a private_message notification with a valid link_url as a PM item', () => {
    const notification = makeNotification({
      type: 'private_message',
      related_type: 'message',
      related_id: 77,
      link_url: '/games/12?tab=messages&conversation=34',
    });

    expect(classifyNotification(notification)).toEqual({
      kind: 'private_message',
      notification,
      gameId: 12,
      conversationId: 34,
      messageId: 77,
      unreadCount: 1,
    });
  });

  it('returns null for a private_message notification with no parseable conversation id', () => {
    const notification = makeNotification({
      type: 'private_message',
      related_type: 'message',
      related_id: 77,
      link_url: undefined,
    });

    expect(classifyNotification(notification)).toBeNull();
  });

  it('returns null for a private_message notification missing related_id', () => {
    const notification = makeNotification({
      type: 'private_message',
      related_type: 'message',
      related_id: undefined,
      link_url: '/games/12?tab=messages&conversation=34',
    });

    expect(classifyNotification(notification)).toBeNull();
  });

  it('returns null for a comment notification missing related_id', () => {
    const notification = makeNotification({
      type: 'comment_reply',
      related_type: 'comment',
      related_id: undefined,
    });

    expect(classifyNotification(notification)).toBeNull();
  });

  it('returns null for non-repliable notification types', () => {
    const types = ['common_room_post', 'action_result', 'phase_created', 'handout_published', 'character_approved'];
    for (const type of types) {
      expect(classifyNotification(makeNotification({ type }))).toBeNull();
    }
  });

  it('returns null when game_id is missing', () => {
    const notification = makeNotification({
      type: 'comment_reply',
      related_type: 'comment',
      related_id: 99,
      game_id: undefined,
    });

    expect(classifyNotification(notification)).toBeNull();
  });
});

describe('classifyNotification conversation resolution', () => {
  it('prefers context_id over link_url for the conversation id', () => {
    // link_url disagrees with context_id; context_id is what the backend uses
    // to clear the conversation, so the inbox must group by the same value.
    const notification = makeNotification({
      type: 'private_message',
      related_type: 'message',
      related_id: 77,
      link_url: '/games/12?tab=messages&conversation=34',
      context_type: 'conversation',
      context_id: 99,
    });

    expect(classifyNotification(notification)).toMatchObject({
      kind: 'private_message',
      conversationId: 99,
    });
  });

  it('falls back to link_url for notifications created before context tracking', () => {
    const notification = makeNotification({
      type: 'private_message',
      related_type: 'message',
      related_id: 77,
      link_url: '/games/12?tab=messages&conversation=34',
    });

    expect(classifyNotification(notification)).toMatchObject({
      kind: 'private_message',
      conversationId: 34,
    });
  });
});

describe('collapseInboxItems', () => {
  function pmItem(conversationId: number, messageId: number, notificationId: number): UnreadInboxItem {
    return {
      kind: 'private_message',
      notification: makeNotification({ id: notificationId, type: 'private_message' }),
      gameId: 12,
      conversationId,
      messageId,
      unreadCount: 1,
    };
  }

  function commentItem(commentId: number): UnreadInboxItem {
    return {
      kind: 'comment',
      notification: makeNotification({ id: commentId }),
      gameId: 12,
      commentId,
    };
  }

  it('collapses a busy conversation into one row carrying the count', () => {
    // The reported scenario: an overnight group conversation should occupy one
    // inbox row, not one per message.
    const collapsed = collapseInboxItems([
      pmItem(34, 3, 103),
      pmItem(34, 2, 102),
      pmItem(34, 1, 101),
    ]);

    expect(collapsed).toHaveLength(1);
    expect(collapsed[0]).toMatchObject({
      kind: 'private_message',
      conversationId: 34,
      unreadCount: 3,
      // Notifications arrive newest-first, so the newest message is previewed.
      messageId: 3,
    });
  });

  it('keeps separate conversations as separate rows', () => {
    const collapsed = collapseInboxItems([
      pmItem(34, 3, 103),
      pmItem(99, 9, 109),
      pmItem(34, 2, 102),
    ]);

    expect(collapsed).toHaveLength(2);
    expect(collapsed.map((i) => i.kind === 'private_message' && i.unreadCount)).toEqual([2, 1]);
  });

  it('preserves the position of each conversation by its newest message', () => {
    const collapsed = collapseInboxItems([
      pmItem(99, 9, 109),
      pmItem(34, 3, 103),
      pmItem(34, 2, 102),
    ]);

    expect(collapsed.map((i) => i.kind === 'private_message' && i.conversationId)).toEqual([99, 34]);
  });

  it('leaves comment items untouched', () => {
    // Each comment is a distinct thing to reply to, not repetition of one thing.
    const collapsed = collapseInboxItems([commentItem(1), commentItem(2)]);

    expect(collapsed).toHaveLength(2);
  });

  it('does not mutate the items it was given', () => {
    const items = [pmItem(34, 2, 102), pmItem(34, 1, 101)];

    collapseInboxItems(items);

    expect(items[0]).toMatchObject({ unreadCount: 1 });
  });
});
