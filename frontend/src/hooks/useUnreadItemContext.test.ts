import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement } from 'react';
import { useUnreadItemContext } from './useUnreadItemContext';
import * as unreadInboxApi from '@/utils/unreadInboxApi';
import type { UnreadCommentItem, UnreadPrivateMessageItem } from '@/types/unreadInbox';
import type { Notification } from '@/types/notifications';
import type { PrivateMessage } from '@/types/conversations';
import type { Message } from '@/types/messages';

vi.mock('@/utils/unreadInboxApi', () => ({
  fetchCommentContext: vi.fn(),
  fetchPmContext: vi.fn(),
  fetchConversationParticipantCharacterIds: vi.fn(),
  fetchAllGameCharacters: vi.fn(),
  resolveReplyCharacters: vi.fn(),
}));

function makeNotification(overrides: Partial<Notification> = {}): Notification {
  return {
    id: 1,
    user_id: 1,
    game_id: 12,
    type: 'private_message',
    title: 'New message',
    is_read: false,
    created_at: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

function makeMessage(overrides: Partial<PrivateMessage> = {}): PrivateMessage {
  return {
    id: 1,
    conversation_id: 34,
    sender_user_id: 5,
    sender_character_id: 20,
    content: 'default content',
    created_at: '2026-01-01T00:00:00Z',
    sender_username: 'gm',
    sender_character_name: 'GM Character',
    ...overrides,
  };
}

function makeComment(overrides: Partial<Message> = {}): Message {
  return {
    id: 1,
    game_id: 12,
    author_id: 5,
    character_id: 20,
    content: 'default comment',
    message_type: 'comment',
    thread_depth: 1,
    author_username: 'player',
    character_name: 'Some Character',
    is_edited: false,
    is_deleted: false,
    is_draft: false,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

describe('useUnreadItemContext', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(unreadInboxApi.resolveReplyCharacters).mockResolvedValue([]);
    vi.mocked(unreadInboxApi.fetchAllGameCharacters).mockResolvedValue([]);
    vi.mocked(unreadInboxApi.fetchConversationParticipantCharacterIds).mockResolvedValue([]);
  });

  it('previews the specific message the notification was for, not just the last message in the conversation', async () => {
    // Same sender sent two messages in the same conversation; each produced its
    // own notification. The server scopes the response to the requested message
    // and its predecessor, so this notification's fetch ends at message 101.
    const zerothMessage = makeMessage({ id: 100, content: 'Zeroth message' });
    const firstMessage = makeMessage({ id: 101, content: 'First message' });
    vi.mocked(unreadInboxApi.fetchPmContext).mockResolvedValue([zerothMessage, firstMessage]);

    const itemForFirstMessage: UnreadPrivateMessageItem = {
      kind: 'private_message',
      notification: makeNotification({ id: 1 }),
      gameId: 12,
      conversationId: 34,
      messageId: 101,
    };

    const { result } = renderHook(() => useUnreadItemContext(itemForFirstMessage, true), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.previewMessage.content).toBe('First message');
    // The fetch must be scoped to this notification's message, otherwise the
    // whole conversation would be pulled just to preview one message.
    expect(unreadInboxApi.fetchPmContext).toHaveBeenCalledWith(12, 34, 101);
  });

  it('previews the second message when its notification is expanded, from the same fetched conversation', async () => {
    const firstMessage = makeMessage({ id: 101, content: 'First message' });
    const secondMessage = makeMessage({ id: 102, content: 'Second message' });
    vi.mocked(unreadInboxApi.fetchPmContext).mockResolvedValue([firstMessage, secondMessage]);

    const itemForSecondMessage: UnreadPrivateMessageItem = {
      kind: 'private_message',
      notification: makeNotification({ id: 2 }),
      gameId: 12,
      conversationId: 34,
      messageId: 102,
    };

    const { result } = renderHook(() => useUnreadItemContext(itemForSecondMessage, true), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.previewMessage.content).toBe('Second message');
  });

  it('falls back to the most recent message if the notification message id is missing from the conversation', async () => {
    const firstMessage = makeMessage({ id: 101, content: 'First message' });
    const secondMessage = makeMessage({ id: 102, content: 'Second message' });
    vi.mocked(unreadInboxApi.fetchPmContext).mockResolvedValue([firstMessage, secondMessage]);

    const itemForDeletedMessage: UnreadPrivateMessageItem = {
      kind: 'private_message',
      notification: makeNotification({ id: 3 }),
      gameId: 12,
      conversationId: 34,
      messageId: 999, // e.g. the message was deleted and no longer appears
    };

    const { result } = renderHook(() => useUnreadItemContext(itemForDeletedMessage, true), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.previewMessage.content).toBe('Second message');
  });

  describe('parent context', () => {
    it('exposes the message preceding the unread one as parent context', async () => {
      const earlier = makeMessage({
        id: 101,
        content: 'What did you find in the archive?',
        sender_character_name: 'Inquisitor',
      });
      const unread = makeMessage({ id: 102, content: 'A sealed letter.' });
      vi.mocked(unreadInboxApi.fetchPmContext).mockResolvedValue([earlier, unread]);

      const item: UnreadPrivateMessageItem = {
        kind: 'private_message',
        notification: makeNotification({ id: 1 }),
        gameId: 12,
        conversationId: 34,
        messageId: 102,
      };

      const { result } = renderHook(() => useUnreadItemContext(item, true), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(result.current.data?.previewMessage.content).toBe('A sealed letter.');
      expect(result.current.data?.parentMessage?.content).toBe('What did you find in the archive?');
      expect(result.current.data?.parentMessage?.characterName).toBe('Inquisitor');
    });

    it('returns a null parent when the unread message opened the conversation', async () => {
      const first = makeMessage({ id: 101, content: 'Opening message' });
      vi.mocked(unreadInboxApi.fetchPmContext).mockResolvedValue([first]);

      const item: UnreadPrivateMessageItem = {
        kind: 'private_message',
        notification: makeNotification({ id: 1 }),
        gameId: 12,
        conversationId: 34,
        messageId: 101,
      };

      const { result } = renderHook(() => useUnreadItemContext(item, true), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(result.current.data?.parentMessage).toBeNull();
    });

    it('picks the parent relative to the notified message, not the end of the thread', async () => {
      // The notification is for the middle message of a longer thread. The
      // server returns only that message and its predecessor, so the parent is
      // the message before *it*, never the newest message in the conversation.
      const first = makeMessage({ id: 101, content: 'First' });
      const second = makeMessage({ id: 102, content: 'Second' });
      vi.mocked(unreadInboxApi.fetchPmContext).mockResolvedValue([first, second]);

      const item: UnreadPrivateMessageItem = {
        kind: 'private_message',
        notification: makeNotification({ id: 1 }),
        gameId: 12,
        conversationId: 34,
        messageId: 102,
      };

      const { result } = renderHook(() => useUnreadItemContext(item, true), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(result.current.data?.previewMessage.content).toBe('Second');
      expect(result.current.data?.parentMessage?.content).toBe('First');
    });

    it('exposes the parent comment for comment items without an extra fetch', async () => {
      const parent = makeComment({
        id: 200,
        content: 'The door was already open when we arrived.',
        character_name: 'Scout',
      });
      const comment = makeComment({ id: 201, content: 'Then someone beat us to it.' });
      vi.mocked(unreadInboxApi.fetchCommentContext).mockResolvedValue({
        comment,
        parent,
        rootPostId: 90,
      });

      const item: UnreadCommentItem = {
        kind: 'comment',
        notification: makeNotification({ id: 1, type: 'comment_reply' }),
        gameId: 12,
        commentId: 201,
      };

      const { result } = renderHook(() => useUnreadItemContext(item, true), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(result.current.data?.parentMessage?.content).toBe(
        'The door was already open when we arrived.'
      );
      expect(result.current.data?.parentMessage?.characterName).toBe('Scout');
      // The parent comes from the already-fetched comment context, so no
      // additional network call is made to retrieve it.
      expect(unreadInboxApi.fetchCommentContext).toHaveBeenCalledTimes(1);
    });

    it('marks a deleted parent comment so the UI can render it as removed', async () => {
      const parent = makeComment({ id: 200, content: '', is_deleted: true });
      const comment = makeComment({ id: 201, content: 'Reply to a since-deleted comment' });
      vi.mocked(unreadInboxApi.fetchCommentContext).mockResolvedValue({
        comment,
        parent,
        rootPostId: 90,
      });

      const item: UnreadCommentItem = {
        kind: 'comment',
        notification: makeNotification({ id: 1, type: 'comment_reply' }),
        gameId: 12,
        commentId: 201,
      };

      const { result } = renderHook(() => useUnreadItemContext(item, true), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(result.current.data?.parentMessage?.isDeleted).toBe(true);
    });

    it('returns a null parent for a comment replying directly to a post', async () => {
      const comment = makeComment({ id: 201, content: 'Top-level reply' });
      vi.mocked(unreadInboxApi.fetchCommentContext).mockResolvedValue({
        comment,
        parent: null,
        rootPostId: 90,
      });

      const item: UnreadCommentItem = {
        kind: 'comment',
        notification: makeNotification({ id: 1, type: 'comment_reply' }),
        gameId: 12,
        commentId: 201,
      };

      const { result } = renderHook(() => useUnreadItemContext(item, true), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(result.current.data?.parentMessage).toBeNull();
    });
  });
});
