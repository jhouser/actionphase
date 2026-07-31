/* eslint-disable react-refresh/only-export-components */
import { createContext, useContext, useState, useCallback, useRef } from 'react';
import type { ReactNode } from 'react';
import { apiClient } from '../lib/api';
import { useToast } from './ToastContext';
import { logger } from '@/services/LoggingService';
import type { ConversationListItem, ConversationWithDetails, PrivateMessage, CreateConversationRequest } from '../types/conversations';

interface ConversationContextType {
  // State
  conversations: ConversationListItem[];
  selectedConversationId: number | null;
  selectedConversationInfo: ConversationListItem | undefined;
  conversation: ConversationWithDetails | null;
  messages: PrivateMessage[];
  /**
   * The conversation `messages` actually belongs to, or null when nothing is
   * loaded. Consumers must check this against their own conversationId before
   * acting on `messages` — during a switch, `messages` still holds the previous
   * conversation's data until the new fetch resolves.
   */
  loadedMessagesConversationId: number | null;

  // Loading states
  loadingConversations: boolean;
  loadingMessages: boolean;
  loadingConversation: boolean;
  isRefreshing: boolean;

  // Actions
  selectConversation: (conversationId: number | null) => void;
  loadConversations: (gameId: number) => Promise<void>;
  loadConversation: (gameId: number, conversationId: number) => Promise<void>;
  loadMessages: (gameId: number, conversationId: number) => Promise<PrivateMessage[] | null>;
  refreshConversation: (gameId: number, conversationId: number) => Promise<boolean>;
  markAsRead: (gameId: number, conversationId: number) => Promise<void>;
  sendMessage: (gameId: number, conversationId: number, data: { character_id: number; content: string }) => Promise<void>;
  deleteMessage: (gameId: number, conversationId: number, messageId: number) => Promise<void>;
  editMessage: (gameId: number, conversationId: number, messageId: number, content: string) => Promise<void>;
  createConversation: (gameId: number, data: CreateConversationRequest) => Promise<number>;

  // Utility
  resetConversationState: () => void;
}

const ConversationContext = createContext<ConversationContextType | undefined>(undefined);

export function useConversation() {
  const context = useContext(ConversationContext);
  if (context === undefined) {
    throw new Error('useConversation must be used within a ConversationProvider');
  }
  return context;
}

interface ConversationProviderProps {
  children: ReactNode;
}

export function ConversationProvider({ children }: ConversationProviderProps) {
  const { showError, showSuccess } = useToast();

  // State
  const [conversations, setConversations] = useState<ConversationListItem[]>([]);
  const [selectedConversationId, setSelectedConversationId] = useState<number | null>(null);
  const [conversation, setConversation] = useState<ConversationWithDetails | null>(null);
  const [messages, setMessages] = useState<PrivateMessage[]>([]);
  const [loadedMessagesConversationId, setLoadedMessagesConversationId] = useState<number | null>(null);

  // Loading states
  const latestMessagesRequestRef = useRef(0);

  const [loadingConversations, setLoadingConversations] = useState(false);
  const [loadingMessages, setLoadingMessages] = useState(false);
  const [loadingConversation, setLoadingConversation] = useState(false);
  const [isRefreshing, setIsRefreshing] = useState(false);

  // Computed: Get info about selected conversation from conversations list
  const selectedConversationInfo = selectedConversationId
    ? conversations.find(c => c.id === selectedConversationId)
    : undefined;

  // Actions
  const selectConversation = useCallback((conversationId: number | null) => {
    setSelectedConversationId(conversationId);
    if (conversationId === null) {
      // Clear conversation data when deselecting
      setConversation(null);
      setMessages([]);
      setLoadedMessagesConversationId(null);
    }
  }, []);

  const loadConversations = useCallback(async (gameId: number) => {
    try {
      setLoadingConversations(true);
      const response = await apiClient.conversations.getUserConversations(gameId);

      // Deduplicate by ID
      const conversationMap = new Map<number, ConversationListItem>();
      (response.data.conversations || []).forEach(conv => {
        if (!conversationMap.has(conv.id)) {
          conversationMap.set(conv.id, conv);
        }
      });

      setConversations(Array.from(conversationMap.values()));
      logger.debug('Loaded conversations', { gameId, count: conversationMap.size });
    } catch (_err) {
      logger.error('Failed to load conversations', { error: _err, gameId });
      showError('Failed to load conversations. Please try again.');
    } finally {
      setLoadingConversations(false);
    }
  }, [showError]);

  const loadConversation = useCallback(async (gameId: number, conversationId: number) => {
    try {
      setLoadingConversation(true);
      const response = await apiClient.conversations.getConversation(gameId, conversationId);
      setConversation(response.data);
      logger.debug('Loaded conversation details', { gameId, conversationId });
    } catch (_err) {
      logger.error('Failed to load conversation', { error: _err, gameId, conversationId });
      showError('Failed to load conversation details.');
    } finally {
      setLoadingConversation(false);
    }
  }, [showError]);

  const loadMessages = useCallback(async (gameId: number, conversationId: number) => {
    // Track the most recent request so out-of-order responses can be discarded.
    // Without this, a slow fetch for conversation A can resolve after a fast
    // fetch for B and overwrite B's messages with A's.
    const requestId = ++latestMessagesRequestRef.current;
    try {
      setLoadingMessages(true);
      const response = await apiClient.conversations.getConversationMessages(gameId, conversationId);
      const newMessages = response.data.messages || [];

      if (requestId !== latestMessagesRequestRef.current) {
        logger.debug('Discarded stale messages response', { gameId, conversationId });
        return newMessages;
      }

      setMessages(newMessages);
      setLoadedMessagesConversationId(conversationId);
      logger.debug('Loaded messages', { gameId, conversationId, count: newMessages.length });
      return newMessages;
    } catch (_err) {
      logger.error('Failed to load messages', { error: _err, gameId, conversationId });
      showError('Failed to load messages.');
      return null;
    } finally {
      if (requestId === latestMessagesRequestRef.current) {
        setLoadingMessages(false);
      }
    }
  }, [showError]);

  const refreshConversation = useCallback(async (gameId: number, conversationId: number) => {
    // Store current message count before refresh
    const previousMessageCount = messages.length;

    setIsRefreshing(true);
    try {
      // Refresh both conversations list (for updated unread counts) and messages
      const results = await Promise.all([
        loadConversations(gameId),
        loadMessages(gameId, conversationId),
        loadConversation(gameId, conversationId)
      ]);

      // Get the new messages from loadMessages result
      const newMessages = results[1];
      const hasNewMessages = !!(newMessages && newMessages.length > previousMessageCount);

      if (hasNewMessages) {
        const messageCountDiff = newMessages.length - previousMessageCount;
        showSuccess(`${messageCountDiff} new message${messageCountDiff === 1 ? '' : 's'}`);
      }

      logger.debug('Refreshed conversation', {
        gameId,
        conversationId,
        hasNewMessages,
        previousMessageCount,
        newMessageCount: newMessages?.length || 0
      });

      return hasNewMessages;
    } catch (_err) {
      logger.error('Failed to refresh conversation', { error: _err, gameId, conversationId });
      showError('Failed to refresh conversation.');
      return false;
    } finally {
      setIsRefreshing(false);
    }
  }, [messages.length, loadConversations, loadMessages, loadConversation, showSuccess, showError]);

  const markAsRead = useCallback(async (gameId: number, conversationId: number) => {
    try {
      await apiClient.conversations.markConversationAsRead(gameId, conversationId);
      logger.debug('Marked conversation as read', { gameId, conversationId });

      // Refresh conversations list to update unread counts
      await loadConversations(gameId);
    } catch (_err) {
      logger.error('Failed to mark conversation as read', { error: _err, gameId, conversationId });
      // Don't show error to user - this is a background operation
    }
  }, [loadConversations]);

  const sendMessage = useCallback(async (
    gameId: number,
    conversationId: number,
    data: { character_id: number; content: string }
  ) => {
    try {
      await apiClient.conversations.sendMessage(gameId, conversationId, data);
      logger.debug('Sent message', { gameId, conversationId, characterId: data.character_id });

      // Reload messages to show the new message
      await loadMessages(gameId, conversationId);

      // Mark as read immediately after sending
      await markAsRead(gameId, conversationId);
    } catch (_err) {
      logger.error('Failed to send message', { error: _err, gameId, conversationId });
      showError('Failed to send message. Please try again.');
      throw _err; // Re-throw so caller knows it failed
    }
  }, [loadMessages, markAsRead, showError]);

  const deleteMessage = useCallback(async (
    gameId: number,
    conversationId: number,
    messageId: number
  ) => {
    try {
      await apiClient.conversations.deleteMessage(gameId, conversationId, messageId);
      logger.debug('Deleted message', { gameId, conversationId, messageId });

      // Reload messages to show updated state
      await loadMessages(gameId, conversationId);
    } catch (_err) {
      logger.error('Failed to delete message', { error: _err, gameId, conversationId, messageId });
      showError('Failed to delete message. Please try again.');
      throw _err;
    }
  }, [loadMessages, showError]);

  const editMessage = useCallback(async (
    gameId: number,
    conversationId: number,
    messageId: number,
    content: string
  ) => {
    try {
      await apiClient.conversations.updateMessage(gameId, conversationId, messageId, { content });
      logger.debug('Edited message', { gameId, conversationId, messageId });

      // Reload messages to show updated content
      await loadMessages(gameId, conversationId);
    } catch (_err) {
      logger.error('Failed to edit message', { error: _err, gameId, conversationId, messageId });
      showError('Failed to edit message. Please try again.');
      throw _err;
    }
  }, [loadMessages, showError]);

  const createConversation = useCallback(async (gameId: number, data: CreateConversationRequest) => {
    try {
      const response = await apiClient.conversations.createConversation(gameId, data);
      const newConversationId = response.data.id;
      logger.debug('Created conversation', { gameId, conversationId: newConversationId });

      // Refresh conversations list
      await loadConversations(gameId);

      return newConversationId;
    } catch (_err) {
      logger.error('Failed to create conversation', { error: _err, gameId });
      showError('Failed to create conversation. Please try again.');
      throw _err;
    }
  }, [loadConversations, showError]);

  const resetConversationState = useCallback(() => {
    setConversations([]);
    setSelectedConversationId(null);
    setConversation(null);
    setMessages([]);
    setLoadedMessagesConversationId(null);
  }, []);

  const value: ConversationContextType = {
    // State
    conversations,
    selectedConversationId,
    selectedConversationInfo,
    conversation,
    messages,
    loadedMessagesConversationId,

    // Loading states
    loadingConversations,
    loadingMessages,
    loadingConversation,
    isRefreshing,

    // Actions
    selectConversation,
    loadConversations,
    loadConversation,
    loadMessages,
    refreshConversation,
    markAsRead,
    sendMessage,
    deleteMessage,
    editMessage,
    createConversation,

    // Utility
    resetConversationState,
  };

  return (
    <ConversationContext.Provider value={value}>
      {children}
    </ConversationContext.Provider>
  );
}
