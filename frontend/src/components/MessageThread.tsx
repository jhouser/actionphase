import { useState, useEffect, useLayoutEffect, useRef, useMemo, useCallback } from 'react';
import { Trash2, RefreshCw, Pencil, ArrowDownToLine } from 'lucide-react';
import { useAuth } from '../contexts/AuthContext';
import { useConversation } from '../contexts/ConversationContext';
import { useOptionalGameContext } from '../contexts/GameContext';
import { Button, Select, Alert } from './ui';
import { CommentEditor } from './CommentEditor';
import CharacterAvatar from './CharacterAvatar';
import { MarkdownPreview } from './MarkdownPreview';
import { STICKY_BELOW_TABS } from './TabNavigation';
import type { Character } from '../types/characters';
import { logger } from '@/services/LoggingService';

interface MessageThreadProps {
  gameId: number;
  conversationId: number;
  characters: Character[];
  currentPhaseType?: string; // Current game phase type (common_room, action, results, etc.)
  onBack?: () => void;
}

export function MessageThread({ gameId, conversationId, characters, currentPhaseType, onBack }: MessageThreadProps) {
  const isMessagingAllowed = currentPhaseType === 'common_room' || currentPhaseType === 'interlude';
  const { currentUser } = useAuth();
  const gameContext = useOptionalGameContext();
  const portraitAvatars = gameContext?.game?.portrait_avatars ?? false;
  const isGM = gameContext?.isGM ?? false;

  // Get conversation data from context
  const {
    messages,
    conversation,
    selectedConversationInfo,
    loadedMessagesConversationId,
    loadingMessages,
    loadingConversation,
    loadingConversations,
    isRefreshing,
    loadConversation,
    loadMessages,
    refreshConversation,
    markAsRead,
    sendMessage,
    deleteMessage,
    editMessage,
    deleteConversation,
  } = useConversation();

  // UI-specific state
  const [newMessage, setNewMessage] = useState('');
  const [selectedCharacterId, setSelectedCharacterId] = useState<number | null>(null);
  const [sending, setSending] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const firstUnreadRef = useRef<HTMLDivElement>(null);
  const messagesContainerRef = useRef<HTMLDivElement>(null);
  // Latches once the thread has been positioned for the current conversation.
  // A ref rather than state: flipping state inside the scroll effect would
  // re-run it and cancel the markAsRead timer it just armed.
  const hasScrolledRef = useRef(false);
  const [deleteMessageId, setDeleteMessageId] = useState<number | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [confirmDeleteConversation, setConfirmDeleteConversation] = useState(false);
  const [deletingConversation, setDeletingConversation] = useState(false);
  const [replyOpen, setReplyOpen] = useState(false);
  const [editingMessageId, setEditingMessageId] = useState<number | null>(null);
  const [editContent, setEditContent] = useState('');
  const [saving, setSaving] = useState(false);
  const savedScrollPositionRef = useRef<number | null>(null);

  const loading = loadingMessages || loadingConversation;

  //Content autosave id for comment textbox
  const autosaveRefId = `conversation-${conversationId}`;

  // Filter characters to only show conversation participants
  const participantCharacters = useMemo(() => {
    if (!conversation || !conversation.participants) return characters;

    const participantCharacterIds = conversation.participants
      .map(p => p.character_id)
      .filter((id): id is number => id !== null);

    return characters.filter(char => participantCharacterIds.includes(char.id));
  }, [conversation, characters]);

  // Scroll functions
  //
  // Initial positioning uses `behavior: 'auto'` (instant) rather than 'smooth'.
  // Smooth scrolling animates toward a target that is still moving while
  // markdown and avatar images below it finish laying out, so it reliably
  // lands short on slower connections. An instant jump is immune to that
  // reflow and reads correctly as a starting position.
  const scrollToBottom = useCallback((behavior: ScrollBehavior = 'auto') => {
    if (typeof messagesEndRef.current?.scrollIntoView === 'function') {
      messagesEndRef.current.scrollIntoView({ behavior });
      logger.debug('Scrolled to bottom', { conversationId });
    } else {
      logger.warn('messagesEndRef not available for scrolling', { conversationId });
    }
  }, [conversationId]);

  const scrollToFirstUnread = useCallback(() => {
    if (typeof firstUnreadRef.current?.scrollIntoView === 'function') {
      firstUnreadRef.current.scrollIntoView({ behavior: 'auto', block: 'start' });
      logger.debug('Scrolled to first unread message', { conversationId });
    } else {
      // Fallback to bottom if ref not set
      scrollToBottom();
    }
  }, [conversationId, scrollToBottom]);

  // Load conversation and messages on mount or when conversationId changes
  useEffect(() => {
    loadConversation(gameId, conversationId);
    loadMessages(gameId, conversationId);
  }, [gameId, conversationId, loadConversation, loadMessages]);

  // Auto-select first character from participants
  useEffect(() => {
    if (participantCharacters.length > 0) {
      if (selectedCharacterId === null || !participantCharacters.some(c => c.id === selectedCharacterId)) {
        setSelectedCharacterId(participantCharacters[0].id);
      }
    }
  }, [participantCharacters, selectedCharacterId]);

  // Find the first unread message based on last_read_at timestamp.
  // This is the single source of truth for both the "New messages" divider and
  // the initial scroll target — deriving the scroll branch from unread_count
  // instead would let the two disagree, scrolling to a divider that never
  // rendered. Declared before the scroll effect because that effect's
  // dependency array reads it during render.
  const firstUnreadIndex = useMemo(() => {
    if (!selectedConversationInfo || !selectedConversationInfo.last_read_at) {
      return -1;
    }

    const lastReadTime = new Date(selectedConversationInfo.last_read_at).getTime();
    return messages.findIndex(msg => new Date(msg.created_at).getTime() > lastReadTime);
  }, [selectedConversationInfo, messages]);

  // Scroll to first unread message or bottom on initial load.
  //
  // This effect must not run until every input it reads has settled for THIS
  // conversation. It latches (hasScrolledRef) after a single run, so a
  // premature run based on stale or half-loaded data permanently scrolls to the
  // wrong place. The guards below cover each way that used to happen:
  //
  //   1. `messages` still belongs to the previously-viewed conversation while
  //      the new fetch is in flight — loadedMessagesConversationId proves whose
  //      data we are looking at.
  //   2. The conversations list (the only source of unread_count/last_read_at)
  //      is fetched separately from the messages, so it may not have arrived
  //      yet. Acting early makes every conversation look fully read.
  // useLayoutEffect: the messages we gated on are already laid out by the time
  // this runs, so the scroll anchors exist and are positioned. This also
  // positions the thread before the browser paints, so the user never sees it
  // at the top and then jump.
  useLayoutEffect(() => {
    // The latch is a ref, not state: setting state here would re-run this
    // effect and its cleanup would cancel the markAsRead timer we just armed.
    if (hasScrolledRef.current) return;

    // Guard 1: messages must belong to the conversation we are displaying.
    if (loadedMessagesConversationId !== conversationId) return;
    if (loadingMessages || loadingConversation) return;
    if (messages.length === 0) return;

    // Guard 2: unread tracking must have settled. `selectedConversationInfo`
    // being undefined while the list is still loading is indistinguishable from
    // "genuinely has no unreads", so wait it out.
    if (loadingConversations) return;

    const hasUnreads = firstUnreadIndex >= 0;

    logger.debug('Initial scroll effect running', {
      conversationId,
      messagesCount: messages.length,
      firstUnreadIndex,
      unreadCount: selectedConversationInfo?.unread_count,
    });

    if (hasUnreads) {
      scrollToFirstUnread();
    } else {
      scrollToBottom();
    }

    hasScrolledRef.current = true;

    // Mark as read AFTER a delay to give user time to see the "New messages" badge
    const delay = hasUnreads ? 2000 : 0;
    const timer = setTimeout(() => {
      markAsRead(gameId, conversationId);
    }, delay);

    return () => clearTimeout(timer);
  }, [
    loadedMessagesConversationId,
    loadingMessages,
    loadingConversation,
    loadingConversations,
    messages.length,
    firstUnreadIndex,
    selectedConversationInfo,
    conversationId,
    gameId,
    markAsRead,
    scrollToBottom,
    scrollToFirstUnread,
  ]);

  // Reset scroll state and draft when conversation changes.
  // useLayoutEffect (not useEffect) so the latch is cleared before the browser
  // paints the new conversation, rather than depending on this effect being
  // declared after the scroll effect to get the ordering right.
  useLayoutEffect(() => {
    hasScrolledRef.current = false;
    setNewMessage('');
  }, [conversationId]);

  // Restore scroll position after refresh or send completes
  useLayoutEffect(() => {
    if (!isRefreshing && !sending && savedScrollPositionRef.current !== null) {
      const container = messagesContainerRef.current;
      if (container) {
        container.scrollTop = savedScrollPositionRef.current;
        logger.debug('Restored scroll position', {
          scrollTop: savedScrollPositionRef.current,
          conversationId
        });
        savedScrollPositionRef.current = null;
      }
    }
  }, [isRefreshing, sending, conversationId]);

  // Jump to the newest message on demand.
  //
  // Two scroll containers are in play: the thread's own overflow area and the
  // page itself. scrollIntoView on the end marker settles both, so this works
  // whether the thread is scrolled internally, the page is scrolled, or both.
  // Smooth is right here (unlike initial positioning) — the user initiated it
  // and the movement communicates where they went.
  const handleJumpToLatest = useCallback(() => {
    scrollToBottom('smooth');
  }, [scrollToBottom]);

  const handleRefresh = async () => {
    // Store scroll position before refresh (in case there are no new messages)
    const container = messagesContainerRef.current;
    const currentScrollTop = container ? container.scrollTop : 0;

    // Refresh conversation (context detects new messages and returns boolean)
    const hasNewMessages = await refreshConversation(gameId, conversationId);

    if (hasNewMessages) {
      // New messages: re-arm the initial-scroll effect so it repositions on
      // them. refreshConversation has already updated messages/conversations,
      // so a re-render is pending and the effect will re-evaluate.
      savedScrollPositionRef.current = null;
      hasScrolledRef.current = false;
    } else {
      // No new messages: restore scroll position
      savedScrollPositionRef.current = currentScrollTop;
    }
  };

  const handleSendMessage = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedCharacterId || !newMessage.trim() || sending) return;

    // Save scroll position before sending
    const container = messagesContainerRef.current;
    if (container) {
      savedScrollPositionRef.current = container.scrollTop;
      logger.debug('Saved scroll position before send', {
        scrollTop: container.scrollTop,
        conversationId
      });
    }

    try {
      setSending(true);
      setNewMessage('');

      // Use context's sendMessage (it handles loadMessages and markAsRead)
      // Scroll position will be restored by useLayoutEffect after messages re-render
      await sendMessage(gameId, conversationId, {
        character_id: selectedCharacterId,
        content: newMessage.trim(),
      });

      // Collapse the composer only after a successful send, so the "Sending…"
      // state stays visible while the request is in flight and the composer
      // (with the user's draft) survives if the send fails.
      setReplyOpen(false);

      if (autosaveRefId){
        localStorage.removeItem(autosaveRefId);
      }
    } catch (_err) {
      // Error already handled by context
      logger.error('Failed to send message', { error: _err, gameId, conversationId });
      // Clear saved position on error
      savedScrollPositionRef.current = null;
    } finally {
      setSending(false);
    }
  };

  const handleStartEdit = (messageId: number, currentContent: string) => {
    setEditingMessageId(messageId);
    setEditContent(currentContent);
  };

  const handleCancelEdit = () => {
    setEditingMessageId(null);
    setEditContent('');
  };

  const handleSaveEdit = async () => {
    if (!editingMessageId || !editContent.trim() || saving) return;
    try {
      setSaving(true);
      await editMessage(gameId, conversationId, editingMessageId, editContent.trim());
      setEditingMessageId(null);
      setEditContent('');
    } catch (_err) {
      logger.error('Failed to save edit', { error: _err, gameId, conversationId, messageId: editingMessageId });
    } finally {
      setSaving(false);
    }
  };

  const handleDeleteMessage = async () => {
    if (!deleteMessageId) return;

    try {
      setDeleting(true);
      await deleteMessage(gameId, conversationId, deleteMessageId);
      setDeleteMessageId(null);
    } catch (_err) {
      // Error already handled by context
      logger.error('Failed to delete message', { error: _err, gameId, conversationId, messageId: deleteMessageId });
    } finally {
      setDeleting(false);
    }
  };

  // A conversation can only be deleted while it is empty, and only by the person
  // who created it or a GM. This mirrors the server rule; the server re-checks,
  // so this only governs whether the button is worth showing.
  const canDeleteConversation =
    !!conversation?.conversation &&
    loadedMessagesConversationId === conversationId &&
    messages.length === 0 &&
    (isGM || conversation.conversation.created_by_user_id === currentUser?.id);

  const handleDeleteConversation = async () => {
    try {
      setDeletingConversation(true);
      await deleteConversation(gameId, conversationId);
      setConfirmDeleteConversation(false);
      onBack?.();
    } catch (_err) {
      // Error already surfaced by the context as a toast.
      logger.error('Failed to delete conversation', { error: _err, gameId, conversationId });
      setConfirmDeleteConversation(false);
    } finally {
      setDeletingConversation(false);
    }
  };

  const formatTimestamp = (dateString: string) => {
    const date = new Date(dateString);
    return date.toLocaleString('en-US', {
      month: 'short',
      day: 'numeric',
      hour: 'numeric',
      minute: '2-digit',
    });
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center p-8">
        <div className="text-content-secondary">Loading messages...</div>
      </div>
    );
  }

  return (
    <div className="flex flex-col h-full">
      {/* Conversation Header.
          Sticky because the whole page scrolls (the thread is not the only
          scroll container), so on a long conversation the title, Back button
          and actions would otherwise scroll out of reach.
          It parks beneath the app navbar (h-16) plus the game tab bar, which is
          itself sticky and publishes its height as --game-tabbar-h (0 when there
          is no tab bar above us). z-10 keeps it above message content passing
          underneath while staying below the tab bar's z-30 and navbar's z-50. */}
      {conversation && conversation.conversation && (
        <div
          className="sticky z-10 surface-base border-b border-theme-default px-3 py-2"
          style={{ top: STICKY_BELOW_TABS }}
        >
          <div className="flex items-center gap-2">
            {onBack && (
              <button
                onClick={onBack}
                aria-label="Back to conversations"
                className="flex-shrink-0 p-1.5 rounded hover:bg-interactive-primary-subtle text-interactive-primary hover:text-interactive-primary-hover"
              >
                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
                </svg>
              </button>
            )}
            <div className="flex-1 min-w-0">
              <h2 className="text-base font-bold text-content-primary leading-tight truncate">
                {conversation.conversation.title || 'Untitled Conversation'}
              </h2>
              <p className="text-xs text-content-secondary truncate">
                {[...new Map(conversation.participants?.map(p => [p.character_id ?? `u${p.user_id}`, p]) ?? []).values()]
                  .map(p => p.character_name || p.username).join(', ') || 'None'}
              </p>
            </div>
            <Button
              variant="ghost"
              size="sm"
              onClick={handleJumpToLatest}
              disabled={messages.length === 0}
              className="flex items-center gap-2 flex-shrink-0"
              aria-label="Jump to latest message"
              title="Jump to latest message"
              data-testid="jump-to-latest-button"
            >
              <ArrowDownToLine className="w-4 h-4" />
              <span className="hidden sm:inline">Latest</span>
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={handleRefresh}
              disabled={isRefreshing || loading}
              className="flex items-center gap-2 flex-shrink-0"
              aria-label="Refresh messages"
            >
              <RefreshCw className={`w-4 h-4 ${isRefreshing ? 'animate-spin' : ''}`} />
              <span className="hidden sm:inline">
                {isRefreshing ? 'Refreshing...' : 'Refresh'}
              </span>
            </Button>
            {canDeleteConversation && (
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setConfirmDeleteConversation(true)}
                className="flex items-center gap-2 flex-shrink-0 text-semantic-danger"
                aria-label="Delete conversation"
                title="Delete this empty conversation"
                data-testid="delete-conversation-button"
                data-faro-user-action-name="delete-conversation"
              >
                <Trash2 className="w-4 h-4" />
                <span className="hidden sm:inline">Delete</span>
              </Button>
            )}
          </div>
        </div>
      )}

      {/* Messages */}
      <div ref={messagesContainerRef} className="flex-1 overflow-y-auto p-4 space-y-3">
        {messages.length === 0 ? (
          <div className="flex items-center justify-center h-full">
            <div className="text-center text-content-secondary">
              <p className="mb-2">No messages yet</p>
              <p className="text-sm">Start the conversation!</p>
            </div>
          </div>
        ) : (
          messages.map((message, index) => {
            const isFirstUnread = index === firstUnreadIndex;

            return (
              <div key={message.id}>
                {/* "New messages" divider */}
                {isFirstUnread && (
                  <div ref={firstUnreadRef} className="flex items-center gap-3 my-6">
                    <div className="flex-1 h-px bg-gradient-to-r from-transparent via-interactive-primary to-interactive-primary"></div>
                    <span className="text-sm font-semibold text-interactive-primary px-3 py-1 bg-interactive-primary-subtle rounded-full border border-interactive-primary">
                      New messages
                    </span>
                    <div className="flex-1 h-px bg-gradient-to-l from-transparent via-interactive-primary to-interactive-primary"></div>
                  </div>
                )}

                <div className="flex gap-3 group" data-testid="message">
                  <CharacterAvatar
                    avatarUrl={message.sender_avatar_url}
                    characterName={message.sender_character_name || message.sender_username}
                    size="md"
                    shape={portraitAvatars ? 'portrait' : 'circle'}
                  />
                  <div className="flex flex-col flex-1">
                    <div className="flex items-baseline gap-2 mb-1">
                      <span className="font-semibold text-content-primary" data-testid="message-sender">
                        {message.sender_character_name || message.sender_username}
                      </span>
                      <span className="text-xs text-content-tertiary">
                        {formatTimestamp(message.created_at)}
                      </span>
                      {/* Edit/Delete buttons - only show for sender's non-deleted messages */}
                      {currentUser && message.sender_user_id === currentUser.id && !message.is_deleted && isMessagingAllowed && (
                        <div className="ml-auto flex items-center gap-1 opacity-100 sm:opacity-0 sm:group-hover:opacity-100 transition-opacity">
                          <button
                            onClick={() => handleStartEdit(message.id, message.content)}
                            className="p-1 text-content-secondary hover:bg-interactive-primary-subtle hover:text-interactive-primary rounded"
                            title="Edit message"
                            data-testid="edit-message-button"
                          >
                            <Pencil className="w-4 h-4" />
                          </button>
                          <button
                            onClick={() => setDeleteMessageId(message.id)}
                            className="p-1 text-content-secondary hover:bg-semantic-danger hover:text-content-inverse rounded"
                            title="Delete message"
                          >
                            <Trash2 className="w-4 h-4" />
                          </button>
                        </div>
                      )}
                    </div>
                    {message.is_deleted ? (
                      <div className="surface-raised rounded-lg p-3 italic text-content-tertiary">
                        {message.content}
                      </div>
                    ) : editingMessageId === message.id ? (
                      <div className="surface-raised rounded-lg p-3">
                        <CommentEditor
                          value={editContent}
                          onChange={setEditContent}
                          rows={4}
                          maxLength={50000}
                          disabled={saving}
                          characters={participantCharacters}
                          textareaTestId="edit-message-textarea"
                        />
                        <div className="flex gap-2 mt-2">
                          <Button
                            variant="primary"
                            size="sm"
                            onClick={handleSaveEdit}
                            disabled={saving || !editContent.trim()}
                            loading={saving}
                            data-testid="save-edit-button"
                          >
                            Save
                          </Button>
                          <Button
                            variant="secondary"
                            size="sm"
                            onClick={handleCancelEdit}
                            disabled={saving}
                          >
                            Cancel
                          </Button>
                        </div>
                      </div>
                    ) : (
                      <div className="surface-raised rounded-lg p-3">
                        <MarkdownPreview
                          content={message.content}
                          mentionedCharacters={[]}
                          fullWidth
                        />
                        {message.is_edited && (
                          <span className="text-xs text-content-tertiary mt-1 block" data-testid="edited-label">(edited)</span>
                        )}
                      </div>
                    )}
                  </div>
                </div>
              </div>
            );
          })
        )}
        <div ref={messagesEndRef} />
      </div>

      {/* Message Input */}
      {/* min-h-0 + overflow-y-auto let this region scroll internally if the
          editor is resized taller than the available space, guaranteeing the
          Send button stays reachable instead of being pushed off screen. */}
      <div className="surface-base border-t border-theme-default flex-shrink min-h-0 overflow-y-auto">
        {/* Collapsed state: "Reply" button reclaims the composer strip until the
            user chooses to write. Applies at all widths so the reading area gets
            the full panel while idle. Only shown when the user can actually send
            (messaging allowed + has a participating character) — otherwise the
            informational states below render instead. */}
        {isMessagingAllowed && participantCharacters.length > 0 && !replyOpen && (
          <div className="p-2 flex justify-end">
            <Button
              variant="primary"
              size="sm"
              onClick={() => setReplyOpen(true)}
            >
              Reply
            </Button>
          </div>
        )}

        {/* Phase restriction alert — always visible (read-only users get no
            Reply button, so this must not be gated behind replyOpen). */}
        {!isMessagingAllowed && (
          <div className="p-4">
            <Alert variant="info">
              New messages can only be sent during Common Room or Interlude phases. You can read message history at any time.
            </Alert>
          </div>
        )}

        {/* No-character info — always visible when messaging is allowed but the
            user has no character to send as. */}
        {isMessagingAllowed && participantCharacters.length === 0 && (
          <p className="p-4 text-sm text-content-secondary">
            {characters.length === 0
              ? "You need a character to send messages."
              : "You don't have any characters participating in this conversation."}
          </p>
        )}

        {/* Composer — only mounted once the user opens the reply box. */}
        {isMessagingAllowed && participantCharacters.length > 0 && replyOpen && (
        <div className="p-4">
        <form onSubmit={handleSendMessage}>
              {participantCharacters.length > 1 && (
                <div className="mb-3">
                  <Select
                    value={selectedCharacterId?.toString() || ''}
                    onChange={(e) => setSelectedCharacterId(Number(e.target.value))}
                    disabled={sending || !isMessagingAllowed}
                  >
                    {participantCharacters.map((char) => (
                      <option key={char.id} value={char.id}>
                        Send as {char.name}
                      </option>
                    ))}
                  </Select>
                </div>
              )}

              <CommentEditor
                value={newMessage}
                onChange={setNewMessage}
                rows={4}
                placeholder={isMessagingAllowed ? "Type your message..." : "Messaging is only available during Common Room or Interlude phases"}
                disabled={sending || !isMessagingAllowed}
                maxLength={50000}
                warnOnUnsavedChanges
                showCharacterCount={true}
                characters={participantCharacters}
                autosaveRefId={autosaveRefId}
              />
              <div className="flex items-center gap-2 mt-2">
                <Button
                    type="submit"
                    variant="primary"
                    disabled={sending || !newMessage.trim() || !isMessagingAllowed}
                    title={!isMessagingAllowed ? 'Messages can only be sent during Common Room or Interlude phases' : undefined}
                    data-faro-user-action-name="send-private-message"
                >
                  {sending ? 'Sending...' : 'Send'}
                </Button>
                <p className="text-xs text-content-tertiary hidden sm:block">
                  Press Ctrl/Cmd + Enter to send
                </p>
                <button
                  type="button"
                  onClick={() => setReplyOpen(false)}
                  aria-label="Close reply"
                  className="ml-auto p-1.5 rounded text-content-tertiary hover:text-content-primary hover:bg-interactive-primary-subtle"
                >
                  <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              </div>
        </form>
        </div>
        )}
      </div>

      {/* Delete Confirmation Modal */}
      {deleteMessageId && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="surface-base border border-theme-default rounded-lg p-6 max-w-md w-full mx-4">
            <h3 className="text-lg font-semibold text-content-primary mb-4">Delete Message?</h3>
            <p className="text-content-secondary mb-6">
              This will permanently delete your message. Other participants will see "[Message deleted]" in its place.
            </p>
            <div className="flex gap-3 justify-end">
              <Button
                variant="secondary"
                onClick={() => setDeleteMessageId(null)}
                disabled={deleting}
              >
                Cancel
              </Button>
              <Button
                variant="danger"
                onClick={handleDeleteMessage}
                loading={deleting}
              >
                Delete
              </Button>
            </div>
          </div>
        </div>
      )}

      {/* Delete Conversation Confirmation Modal */}
      {confirmDeleteConversation && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="surface-base border border-theme-default rounded-lg p-6 max-w-md w-full mx-4">
            <h3 className="text-lg font-semibold text-content-primary mb-4">Delete Conversation?</h3>
            <p className="text-content-secondary mb-6">
              This will permanently remove this empty conversation for everyone in it. This cannot be undone.
            </p>
            <div className="flex gap-3 justify-end">
              <Button
                variant="secondary"
                onClick={() => setConfirmDeleteConversation(false)}
                disabled={deletingConversation}
              >
                Cancel
              </Button>
              <Button
                variant="danger"
                onClick={handleDeleteConversation}
                loading={deletingConversation}
                data-testid="confirm-delete-conversation-button"
              >
                Delete
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
