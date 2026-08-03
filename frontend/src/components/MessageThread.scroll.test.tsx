import { render, screen, waitFor, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { MessageThread } from './MessageThread';
import type { PrivateMessage, ConversationListItem } from '../types/conversations';

vi.mock('react-router-dom', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router-dom')>();
  return {
    ...actual,
    useBlocker: () => ({ state: 'unblocked', reset: undefined, proceed: undefined }),
  };
});

vi.mock('../contexts/AuthContext', () => ({
  useAuth: () => ({ currentUser: { id: 1, username: 'testuser' } }),
}));

vi.mock('@/services/LoggingService', () => ({
  logger: { debug: vi.fn(), error: vi.fn(), warn: vi.fn() },
}));

// The scroll anchors are what we assert on: which element the component chose
// to scroll to is the observable behavior users experience as "it opened in the
// right place". We tag each scrollIntoView call with the anchor's identity.
const scrollCalls: string[] = [];
// Records the ScrollBehavior of each call, parallel to scrollCalls. Initial
// positioning must be instant; a user-initiated jump should animate.
const scrollBehaviors: (string | undefined)[] = [];

beforeEach(() => {
  scrollCalls.length = 0;
  scrollBehaviors.length = 0;
  Element.prototype.scrollIntoView = function (this: Element, arg?: boolean | ScrollIntoViewOptions) {
    const text = this.textContent || '';
    if (text.includes('New messages')) {
      scrollCalls.push('FIRST_UNREAD');
    } else if (this.childNodes.length === 0) {
      scrollCalls.push('BOTTOM');
    } else {
      scrollCalls.push('OTHER');
    }
    scrollBehaviors.push(typeof arg === 'object' ? arg?.behavior : undefined);
  };
});

const LAST_READ = '2026-01-01T00:10:00Z';

// 5 messages: indices 0-1 are before last_read_at, 2-4 are after.
// So the "New messages" divider belongs at index 2.
const makeMessages = (conversationId: number): PrivateMessage[] =>
  [
    '2026-01-01T00:00:00Z',
    '2026-01-01T00:05:00Z',
    '2026-01-01T00:15:00Z',
    '2026-01-01T00:20:00Z',
    '2026-01-01T00:25:00Z',
  ].map((created_at, i) => ({
    id: conversationId * 100 + i,
    conversation_id: conversationId,
    sender_user_id: 99,
    content: `msg ${i} in conv ${conversationId}`,
    created_at,
    sender_username: 'other',
    sender_character_name: 'Other',
    is_deleted: false,
  }));

const makeInfo = (id: number, unread: number): ConversationListItem =>
  ({
    id,
    game_id: 1,
    title: `Conversation ${id}`,
    unread_count: unread,
    last_read_at: LAST_READ,
  }) as ConversationListItem;

const conversationDetails = (id: number) => ({
  conversation: {
    id,
    game_id: 1,
    title: `Conversation ${id}`,
    conversation_type: 'direct',
    created_by_user_id: 1,
    created_at: '2026-01-01',
    updated_at: '2026-01-01',
  },
  participants: [],
});

// Mutable context object — tests reassign fields to simulate async arrival
// order, then re-render.
const ctx = {
  conversations: [] as ConversationListItem[],
  selectedConversationId: 1 as number | null,
  selectedConversationInfo: undefined as ConversationListItem | undefined,
  conversation: conversationDetails(1),
  messages: [] as PrivateMessage[],
  // Mirrors the context: which conversation `messages` actually belongs to.
  loadedMessagesConversationId: null as number | null,
  loadingConversations: false,
  loadingMessages: false,
  loadingConversation: false,
  isRefreshing: false,
  selectConversation: vi.fn(),
  loadConversations: vi.fn(),
  loadConversation: vi.fn().mockResolvedValue(undefined),
  loadMessages: vi.fn().mockResolvedValue([]),
  refreshConversation: vi.fn().mockResolvedValue(false),
  markAsRead: vi.fn().mockResolvedValue(undefined),
  sendMessage: vi.fn(),
  deleteMessage: vi.fn(),
  editMessage: vi.fn(),
  createConversation: vi.fn(),
  resetConversationState: vi.fn(),
};

vi.mock('../contexts/ConversationContext', () => ({
  useConversation: () => ctx,
  ConversationProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

const props = {
  gameId: 1,
  characters: [],
  currentPhaseType: 'common_room',
};

// Reset the mutable context between tests so loading flags and the
// loaded-conversation marker never leak across cases.
beforeEach(() => {
  ctx.loadedMessagesConversationId = null;
  ctx.loadingConversations = false;
  ctx.loadingMessages = false;
  ctx.loadingConversation = false;
});

describe('MessageThread initial scroll positioning', () => {
  it('scrolls to the first unread message when unread info is already loaded', async () => {
    // Baseline: everything has arrived before the thread renders. This is the
    // ordering that works today.
    ctx.messages = makeMessages(1);
    ctx.loadedMessagesConversationId = 1;
    ctx.selectedConversationInfo = makeInfo(1, 3);
    ctx.conversation = conversationDetails(1);

    render(<MessageThread {...props} conversationId={1} />);

    await waitFor(() => expect(scrollCalls.length).toBeGreaterThan(0));
    expect(scrollCalls).toContain('FIRST_UNREAD');
    expect(scrollCalls).not.toContain('BOTTOM');
  });

  it('scrolls to the first unread message even when the conversations list resolves after the messages', async () => {
    // RACE: messages arrive first; selectedConversationInfo (which carries
    // unread_count/last_read_at) is still undefined because the conversations
    // list request has not resolved yet. The component must not decide where to
    // scroll until that information is available.
    ctx.messages = makeMessages(1);
    ctx.loadedMessagesConversationId = 1;
    ctx.selectedConversationInfo = undefined;
    ctx.loadingConversations = true;
    ctx.conversation = conversationDetails(1);

    const { rerender } = render(<MessageThread {...props} conversationId={1} />);

    // Messages are on screen, but unread info has not arrived.
    await waitFor(() => expect(screen.getAllByTestId('message').length).toBe(5));

    // Let any deferred scroll (the bottom branch defers by 50ms) actually fire.
    // Without this drain the assertion can pass spuriously, observing only a
    // later correct scroll while the premature one is still queued.
    await act(async () => {
      await new Promise((r) => setTimeout(r, 100));
    });

    // Now the conversations list resolves, revealing 3 unread messages.
    await act(async () => {
      ctx.selectedConversationInfo = makeInfo(1, 3);
      ctx.loadingConversations = false;
      rerender(<MessageThread {...props} conversationId={1} />);
    });

    await act(async () => {
      await new Promise((r) => setTimeout(r, 100));
    });

    await waitFor(() => expect(scrollCalls.length).toBeGreaterThan(0));
    // The user must land on the unread divider, not at the bottom past it.
    expect(scrollCalls).toContain('FIRST_UNREAD');
    expect(scrollCalls).not.toContain('BOTTOM');
  });

  it('does not scroll using the previous conversation data when switching conversations', async () => {
    // Conversation 1 is fully loaded and already scrolled.
    ctx.messages = makeMessages(1);
    ctx.loadedMessagesConversationId = 1;
    ctx.selectedConversationInfo = makeInfo(1, 3);
    ctx.conversation = conversationDetails(1);

    const { rerender } = render(<MessageThread {...props} conversationId={1} />);
    await waitFor(() => expect(scrollCalls.length).toBeGreaterThan(0));
    scrollCalls.length = 0;

    // Switch to conversation 2. Its fetch is in flight, so `messages` still
    // holds conversation 1's data and selectedConversationInfo still describes
    // conversation 1. Nothing here describes conversation 2 yet.
    await act(async () => {
      rerender(<MessageThread {...props} conversationId={2} />);
      // Drain deferred scrolls so a premature one is observed, not missed.
      await new Promise((r) => setTimeout(r, 100));
    });

    // The component must not have scrolled off stale data.
    expect(scrollCalls).toEqual([]);

    // Conversation 2's data arrives: it has unreads too.
    await act(async () => {
      ctx.messages = makeMessages(2);
      ctx.loadedMessagesConversationId = 2;
      ctx.selectedConversationInfo = makeInfo(2, 3);
      ctx.conversation = conversationDetails(2);
      rerender(<MessageThread {...props} conversationId={2} />);
    });

    await waitFor(() => expect(scrollCalls.length).toBeGreaterThan(0));
    expect(scrollCalls).toContain('FIRST_UNREAD');
    expect(scrollCalls).not.toContain('BOTTOM');
  });

  it('scrolls to the bottom when the conversation genuinely has no unread messages', async () => {
    ctx.messages = makeMessages(1);
    ctx.loadedMessagesConversationId = 1;
    ctx.selectedConversationInfo = { ...makeInfo(1, 0), last_read_at: '2026-01-02T00:00:00Z' } as ConversationListItem;
    ctx.conversation = conversationDetails(1);

    render(<MessageThread {...props} conversationId={1} />);

    await waitFor(() => expect(scrollCalls.length).toBeGreaterThan(0));
    expect(scrollCalls).toContain('BOTTOM');
    expect(scrollCalls).not.toContain('FIRST_UNREAD');
  });

  it('positions instantly on load so a reflowing thread cannot leave it short', async () => {
    ctx.messages = makeMessages(1);
    ctx.loadedMessagesConversationId = 1;
    ctx.selectedConversationInfo = makeInfo(1, 3);
    ctx.conversation = conversationDetails(1);

    render(<MessageThread {...props} conversationId={1} />);

    await waitFor(() => expect(scrollCalls.length).toBeGreaterThan(0));
    // Smooth animates toward a target that images/markdown are still moving.
    expect(scrollBehaviors[0]).toBe('auto');
  });
});

describe('MessageThread jump-to-latest control', () => {
  it('scrolls to the newest message when the Latest button is clicked', async () => {
    const user = userEvent.setup();
    // Already read, so the initial positioning is a BOTTOM scroll; clear the
    // record afterwards to isolate the click's effect.
    ctx.messages = makeMessages(1);
    ctx.loadedMessagesConversationId = 1;
    ctx.selectedConversationInfo = { ...makeInfo(1, 0), last_read_at: '2026-01-02T00:00:00Z' } as ConversationListItem;
    ctx.conversation = conversationDetails(1);

    render(<MessageThread {...props} conversationId={1} />);
    await waitFor(() => expect(scrollCalls.length).toBeGreaterThan(0));
    scrollCalls.length = 0;
    scrollBehaviors.length = 0;

    await user.click(screen.getByTestId('jump-to-latest-button'));

    expect(scrollCalls).toEqual(['BOTTOM']);
    // User-initiated, so the motion should be visible rather than instant.
    expect(scrollBehaviors[0]).toBe('smooth');
  });

  it('disables the Latest button when the conversation has no messages', async () => {
    ctx.messages = [];
    ctx.loadedMessagesConversationId = 1;
    ctx.selectedConversationInfo = makeInfo(1, 0);
    ctx.conversation = conversationDetails(1);

    render(<MessageThread {...props} conversationId={1} />);

    expect(screen.getByTestId('jump-to-latest-button')).toBeDisabled();
  });
});
