import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { QueryClientProvider } from '@tanstack/react-query';
import { createTestQueryClient } from '@/test-utils/render';
import { ConversationProvider, useConversation } from './ConversationContext';

vi.mock('@/services/LoggingService', () => ({
  logger: { debug: vi.fn(), error: vi.fn(), warn: vi.fn(), info: vi.fn() },
}));

const showError = vi.fn();
const showSuccess = vi.fn();
vi.mock('./ToastContext', () => ({
  useToast: () => ({ showError, showSuccess }),
}));

const getConversation = vi.fn();
const getConversationMessages = vi.fn();

vi.mock('../lib/api', () => ({
  apiClient: {
    conversations: {
      getConversation: (...args: unknown[]) => getConversation(...args),
      getConversationMessages: (...args: unknown[]) => getConversationMessages(...args),
    },
  },
}));

const detailsFor = (id: number) => ({
  data: {
    conversation: {
      id,
      game_id: 1,
      title: `Conversation ${id}`,
      created_by_user_id: id, // creator differs per conversation
    },
    participants: [],
  },
});

// ConversationProvider invalidates notification queries when a conversation is
// marked read, so it needs a QueryClient in scope.
const wrapper = ({ children }: { children: React.ReactNode }) => (
  <QueryClientProvider client={createTestQueryClient()}>
    <ConversationProvider>{children}</ConversationProvider>
  </QueryClientProvider>
);

beforeEach(() => {
  vi.clearAllMocks();
});

describe('ConversationContext out-of-order response handling', () => {
  // Switching conversations dispatches both fetches at once. When the first
  // conversation's response is slower than the second's, the late arrival must
  // not overwrite the newer conversation's state — otherwise the thread renders
  // one conversation's details over another's messages, and anything derived
  // from `conversation` (such as who may delete it) reads the wrong row.
  it('discards a slow conversation-details response that resolves after a newer one', async () => {
    let resolveSlow!: (value: unknown) => void;
    getConversation.mockImplementation((_gameId: number, id: number) =>
      id === 1
        ? new Promise((resolve) => {
            resolveSlow = resolve;
          })
        : Promise.resolve(detailsFor(id))
    );

    const { result } = renderHook(() => useConversation(), { wrapper });

    // Open conversation 1; its details request hangs.
    act(() => {
      result.current.loadConversation(1, 1);
    });

    // Switch to conversation 2 before 1 resolves; 2 resolves immediately.
    await act(async () => {
      await result.current.loadConversation(1, 2);
    });

    expect(result.current.conversation?.conversation.id).toBe(2);

    // Conversation 1's response finally lands, out of order.
    await act(async () => {
      resolveSlow(detailsFor(1));
      await Promise.resolve();
    });

    expect(result.current.conversation?.conversation.id).toBe(2);
    expect(result.current.conversation?.conversation.created_by_user_id).toBe(2);
  });

  it('keeps details and messages agreeing on the same conversation after a switch', async () => {
    // The delete button is gated on messages being empty AND the details' creator
    // matching the current user. Both must describe the same conversation.
    let resolveSlowDetails!: (value: unknown) => void;
    getConversation.mockImplementation((_gameId: number, id: number) =>
      id === 1
        ? new Promise((resolve) => {
            resolveSlowDetails = resolve;
          })
        : Promise.resolve(detailsFor(id))
    );
    getConversationMessages.mockImplementation((_gameId: number, id: number) =>
      Promise.resolve({ data: { messages: [{ id: id * 10, conversation_id: id }] } })
    );

    const { result } = renderHook(() => useConversation(), { wrapper });

    act(() => {
      result.current.loadConversation(1, 1);
      result.current.loadMessages(1, 1);
    });

    await act(async () => {
      await Promise.all([
        result.current.loadConversation(1, 2),
        result.current.loadMessages(1, 2),
      ]);
    });

    await act(async () => {
      resolveSlowDetails(detailsFor(1));
      await Promise.resolve();
    });

    expect(result.current.loadedMessagesConversationId).toBe(2);
    expect(result.current.conversation?.conversation.id).toBe(2);
  });
});
