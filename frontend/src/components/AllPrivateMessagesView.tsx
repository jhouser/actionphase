import { useEffect, useMemo, useCallback } from 'react';
import { useUrlParam } from '../hooks/useUrlParam';
import { useAllPrivateConversations, useAudienceConversationMessages, useConversationParticipants } from '../hooks/useAudience';
import { Badge } from './ui/Badge';
import { Spinner } from './ui/Spinner';
import { Alert } from './ui/Alert';
import { Button } from './ui/Button';
import { MarkdownPreview } from './MarkdownPreview';
import CharacterAvatar from './CharacterAvatar';
import { AudienceConversationCard } from './audience/AudienceConversationCard';
import { AudienceConversationHeader } from './audience/AudienceConversationHeader';
import type { AudienceConversationListItem } from '../types/conversations';
import { useGameContext } from '../contexts/GameContext';
import { format, isToday, isYesterday, isSameDay } from 'date-fns';

interface AllPrivateMessagesViewProps {
  gameId: number;
}

// Stable identity: useUrlParam returns this exact object when the param is
// absent, so a fresh Set() here would change on every render and retrigger the
// memos and queries that depend on the selection.
const EMPTY_PARTICIPANTS: Set<number> = new Set();

// The participant filter is an AND filter over character ids, stored in the URL
// as a comma-separated list so a filtered view survives a refresh and can be
// linked. NaN entries are dropped rather than trusted — the param is
// user-editable. Mirrors HistoryView's `characters` filter.
const participantFilterParamOptions = {
  deserialize: (raw: string) =>
    new Set(
      raw
        .split(',')
        .map(part => parseInt(part, 10))
        .filter(id => !Number.isNaN(id))
    ),
  serialize: (value: Set<number>) => [...value].join(','),
} as const;


interface MessageType {
  id: number;
  created_at: string;
  content: string;
  sender_character_id?: number;
  sender_character_name?: string | null;
  sender_username: string;
}

/**
 * Read-only view of all private message conversations for audience members and GM
 * Features infinite scroll, participant filtering, and conversation browsing
 */
export function AllPrivateMessagesView({ gameId }: AllPrivateMessagesViewProps) {
  // Opening a conversation pushes a history entry so the browser Back button
  // returns to the list. It must not replace: AudienceConversationCard is a
  // <Link>, which has already pushed the entry by the time its onClick runs, so
  // replacing here overwrote that entry and left Back with nothing to undo.
  const [selectedConversationId, setSelectedConversationId] = useUrlParam<string | null>('audienceConversation', null, {
    deserialize: (s) => s || null,
    serialize: (v) => v ?? '',
    replace: false,
  });
  // Selection is tracked by character ID, not name: names are mutable and not unique
  // within a game, so a rename silently changed which conversations matched.
  // Kept in the URL so the filter survives a refresh and is restored when Back
  // returns here from a conversation. Replaces rather than pushes: toggling chips
  // is a refinement, not a navigation, and each toggle would otherwise cost a
  // separate Back press to escape.
  const [selectedParticipants, setSelectedParticipants] = useUrlParam<Set<number>>(
    'audienceParticipants',
    EMPTY_PARTICIPANTS,
    { ...participantFilterParamOptions, replace: true }
  );

  // Fetch messages for selected conversation
  const {
    data: messages,
    isLoading: messagesLoading,
    error: messagesError
  } = useAudienceConversationMessages(gameId, selectedConversationId);

  // Fetch conversations with server-side filtering
  const selectedIdsForConvs = useMemo(() => Array.from(selectedParticipants), [selectedParticipants]);
  const {
    data,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
    isLoading,
    error,
  } = useAllPrivateConversations(gameId, {
    participantCharacterIds: selectedIdsForConvs.length > 0 ? selectedIdsForConvs : undefined
  });

  // Infinite scroll handler
  useEffect(() => {
    const handleScroll = () => {
      if (
        window.innerHeight + window.scrollY >= document.documentElement.scrollHeight - 500 &&
        hasNextPage &&
        !isFetchingNextPage
      ) {
        fetchNextPage();
      }
    };

    window.addEventListener('scroll', handleScroll);
    return () => window.removeEventListener('scroll', handleScroll);
  }, [hasNextPage, isFetchingNextPage, fetchNextPage]);

  // Get all conversations (already filtered by backend)
  const allConversations = useMemo(() =>
    data?.pages.flatMap((page) => page.conversations || []) || [],
    [data?.pages]
  );

  // Total comes from the first page's total field (same count for all pages of same query)
  const totalConversations = data?.pages[0]?.total ?? allConversations.length;

  // Fetch valid filter options from the backend.
  // Returns all participants when nothing is selected; narrows to co-participants
  // of all selected characters when a filter is active. Backed by a dedicated SQL query
  // that scans ALL conversations — not just the paginated subset loaded so far.
  const { data: filterOptions = [] } = useConversationParticipants(gameId, selectedIdsForConvs);

  const toggleParticipant = (characterId: number) => {
    const next = new Set(selectedParticipants);
    if (next.has(characterId)) {
      next.delete(characterId);
    } else {
      next.add(characterId);
    }
    setSelectedParticipants(next);
  };

  const clearFilters = () => {
    setSelectedParticipants(EMPTY_PARTICIPANTS);
  };

  if (isLoading) {
    return (
      <div className="flex justify-center items-center py-12">
        <Spinner size="lg" />
      </div>
    );
  }

  if (error) {
    return (
      <Alert variant="danger">
        Failed to load private conversations: {(error as Error).message}
      </Alert>
    );
  }

  // If a conversation is selected, show the message viewer
  if (selectedConversationId) {
    const selectedConversation = allConversations.find(
      (conv: AudienceConversationListItem) => String(conv.conversation_id) === selectedConversationId
    );

    return (
      <MessageViewer
        gameId={gameId}
        conversationId={selectedConversationId}
        conversation={selectedConversation}
        messages={messages}
        isLoading={messagesLoading}
        error={messagesError}
        onBack={() => setSelectedConversationId(null)}
      />
    );
  }

  return (
    <div className="space-y-4">
      {/* Header with Read-Only Badge */}
      {/* Mobile: Vertical stack */}
      <div className="md:hidden space-y-2">
        <div className="flex items-center gap-2 flex-wrap">
          <h2 className="text-lg font-semibold text-content-primary">
            All Private Messages
          </h2>
          <Badge variant="primary" size="sm">
            Read-Only
          </Badge>
        </div>
        <div className="text-sm text-content-secondary">
          {totalConversations} conversation{totalConversations !== 1 ? 's' : ''}
          {selectedParticipants.size > 0 && ' (filtered)'}
        </div>
      </div>
      {/* Desktop: Horizontal layout */}
      <div className="hidden md:flex items-center justify-between">
        <div className="flex items-center gap-3">
          <h2 className="text-xl font-semibold text-content-primary">
            All Private Messages
          </h2>
          <Badge variant="primary" size="sm">
            Read-Only
          </Badge>
        </div>
        <div className="text-sm text-content-secondary">
          {totalConversations} conversation{totalConversations !== 1 ? 's' : ''}
          {selectedParticipants.size > 0 && ' (filtered)'}
        </div>
      </div>

      {/* Info Alert */}
      <Alert variant="info">
        As an audience member, you can view all private message conversations to follow the full story.
        You cannot send messages or participate in conversations.
      </Alert>

      {/* Participant Filter */}
      {filterOptions.length > 0 && (
        <div className="border border-border-primary rounded-lg p-4 bg-bg-secondary">
          <div className="flex items-center justify-between mb-3">
            <h3 className="text-sm font-semibold text-content-primary">Filter by Participants</h3>
            {selectedParticipants.size > 0 && (
              <Button
                variant="ghost"
                size="sm"
                onClick={clearFilters}
                className="text-xs"
              >
                Clear filters
              </Button>
            )}
          </div>
          <div className="flex flex-wrap gap-2">
            {filterOptions.map((participant) => {
              const isSelected = selectedParticipants.has(participant.id);
              return (
                <button
                  key={participant.id}
                  onClick={() => toggleParticipant(participant.id)}
                  className={`
                    px-3 py-1.5 rounded-full text-sm font-medium transition-colors
                    ${isSelected
                      ? 'bg-interactive-primary text-white'
                      : 'bg-bg-primary border border-border-primary text-content-primary hover:bg-bg-tertiary'
                    }
                  `}
                >
                  {participant.name}
                </button>
              );
            })}
          </div>
        </div>
      )}

      {/* Conversations List */}
      {allConversations.length === 0 ? (
        <div className="text-center py-12 text-content-secondary">
          {selectedParticipants.size > 0 ? (
            <>
              <p className="text-lg mb-2">No conversations found</p>
              <p className="text-sm">Try adjusting your filters</p>
            </>
          ) : (
            <>
              <p className="text-lg mb-2">No private conversations yet</p>
              <p className="text-sm">Conversations will appear here once players start messaging</p>
            </>
          )}
        </div>
      ) : (
        <div className="space-y-3">
          {allConversations.map((conversation: AudienceConversationListItem) => (
            <AudienceConversationCard
              key={conversation.conversation_id}
              conversation={conversation}
              isSelected={selectedConversationId === String(conversation.conversation_id)}
            />
          ))}

          {/* Load More Indicator */}
          {isFetchingNextPage && (
            <div className="flex justify-center py-4">
              <Spinner size="md" />
            </div>
          )}

          {!hasNextPage && allConversations.length > 0 && (
            <div className="text-center py-4 text-sm text-content-secondary">
              End of conversations
            </div>
          )}
        </div>
      )}
    </div>
  );
}

/**
 * Message viewer component - displays messages for a selected conversation
 * Features: Date dividers, message grouping, rich header
 */
function MessageViewer({
  gameId: _gameId,
  conversationId: _conversationId,
  conversation,
  messages,
  isLoading,
  error,
  onBack,
}: {
  gameId: number;
  conversationId: string;
  conversation?: AudienceConversationListItem;
  messages: MessageType[] | undefined;
  isLoading: boolean;
  error: Error | null;
  onBack: () => void;
}) {
  const { allGameCharacters, game } = useGameContext();
  const portraitAvatars = game?.portrait_avatars ?? false;

  const getAvatarUrl = useCallback((characterId: number | undefined): string | null =>
    allGameCharacters.find(c => c.id === characterId)?.avatar_url ?? null,
  [allGameCharacters]);

  // Format date for dividers
  const formatDateDivider = (date: Date): string => {
    if (isToday(date)) return 'Today';
    if (isYesterday(date)) return 'Yesterday';
    return format(date, 'MMMM d, yyyy');
  };

  // Format timestamp for message
  const formatTimestamp = (dateString: string) => {
    const date = new Date(dateString);
    return format(date, 'h:mm a');
  };

  // Group messages by date and consecutive sender
  const groupedMessages = useMemo(() => {
    if (!messages || messages.length === 0) return [];

    const groups: Array<{
      date: Date;
      messageGroups: Array<{
        senderId: number | undefined;
        senderName: string;
        senderUsername: string;
        senderAvatar: string | null;
        messages: MessageType[];
      }>;
    }> = [];

    let currentDate: Date | null = null;
    let currentSenderId: number | undefined = undefined;
    let currentMessageGroup: MessageType[] = [];
    let currentSenderName = '';
    let currentSenderUsername = '';
    let currentSenderAvatar: string | null = null;

    messages.forEach((message, index) => {
      const messageDate = new Date(message.created_at);
      const isNewDate = !currentDate || !isSameDay(currentDate, messageDate);
      // Compare character IDs instead of user IDs to properly group messages from different NPCs controlled by the same GM
      const isNewSender = message.sender_character_id !== currentSenderId;

      // Start new date group
      if (isNewDate) {
        // Save previous message group if exists
        if (currentMessageGroup.length > 0) {
          const lastGroup = groups[groups.length - 1];
          if (lastGroup) {
            lastGroup.messageGroups.push({
              senderId: currentSenderId,
              senderName: currentSenderName,
              senderUsername: currentSenderUsername,
              senderAvatar: currentSenderAvatar,
              messages: currentMessageGroup,
            });
          }
        }

        currentDate = messageDate;
        groups.push({
          date: messageDate,
          messageGroups: [],
        });
        currentMessageGroup = [message];
        currentSenderId = message.sender_character_id;
        currentSenderName = message.sender_character_name || 'Unknown Character';
        currentSenderUsername = message.sender_username;
        currentSenderAvatar = getAvatarUrl(message.sender_character_id);
      }
      // Start new sender group within same date
      else if (isNewSender) {
        // Save previous message group
        if (currentMessageGroup.length > 0) {
          groups[groups.length - 1].messageGroups.push({
            senderId: currentSenderId,
            senderName: currentSenderName,
            senderUsername: currentSenderUsername,
            senderAvatar: currentSenderAvatar,
            messages: currentMessageGroup,
          });
        }

        currentMessageGroup = [message];
        currentSenderId = message.sender_character_id;
        currentSenderName = message.sender_character_name || 'Unknown Character';
        currentSenderUsername = message.sender_username;
        currentSenderAvatar = getAvatarUrl(message.sender_character_id);
      }
      // Same sender, same date - add to current group
      else {
        currentMessageGroup.push(message);
      }

      // Handle last message
      if (index === messages.length - 1 && currentMessageGroup.length > 0) {
        groups[groups.length - 1].messageGroups.push({
          senderId: currentSenderId,
          senderName: currentSenderName,
          senderUsername: currentSenderUsername,
          senderAvatar: currentSenderAvatar,
          messages: currentMessageGroup,
        });
      }
    });

    return groups;
  }, [messages, getAvatarUrl]);

  // Loading state
  if (isLoading) {
    return (
      <div className="flex justify-center items-center py-12">
        <Spinner size="lg" />
      </div>
    );
  }

  // Error state
  if (error) {
    return (
      <div className="space-y-4">
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="sm" onClick={onBack}>
            ← Back to conversations
          </Button>
        </div>
        <Alert variant="danger">
          Failed to load messages: {error.message}
        </Alert>
      </div>
    );
  }

  // No conversation selected (shouldn't happen but handle gracefully)
  if (!conversation) {
    return (
      <div className="space-y-4">
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="sm" onClick={onBack}>
            ← Back to conversations
          </Button>
        </div>
        <Alert variant="warning">
          Conversation not found
        </Alert>
      </div>
    );
  }

  return (
    <div className="space-y-0">
      {/* Conversation Header */}
      <AudienceConversationHeader
        conversation={conversation}
        messageCount={messages?.length || 0}
        onBack={onBack}
      />

      {/* Messages */}
      <div className="p-4">
        {!messages || messages.length === 0 ? (
          <div className="text-center py-12 text-content-secondary">
            <p className="text-lg mb-2">No messages yet</p>
            <p className="text-sm">This conversation has no messages</p>
          </div>
        ) : (
          <div className="space-y-6">
            {groupedMessages.map((dateGroup, dateIndex) => (
              <div key={dateIndex}>
                {/* Date Divider */}
                <div className="flex items-center gap-3 mb-4">
                  <div className="flex-1 h-px bg-border-primary"></div>
                  <div className="text-xs font-semibold text-content-secondary uppercase tracking-wide">
                    {formatDateDivider(dateGroup.date)}
                  </div>
                  <div className="flex-1 h-px bg-border-primary"></div>
                </div>

                {/* Message Groups for this date */}
                <div className="space-y-4">
                  {dateGroup.messageGroups.map((messageGroup, groupIndex) => (
                    <div key={groupIndex} className="flex gap-3">
                      {/* Avatar (shown once per group) */}
                      <div className="flex-shrink-0">
                        <CharacterAvatar
                          avatarUrl={messageGroup.senderAvatar}
                          characterName={messageGroup.senderName}
                          size="md"
                          shape={portraitAvatars ? 'portrait' : 'circle'}
                        />
                      </div>

                      {/* Message group */}
                      <div className="flex-1 min-w-0">
                        {/* Sender info (shown once per group) */}
                        <div className="flex items-baseline gap-2 mb-2">
                          <span className="font-semibold text-content-primary">
                            {messageGroup.senderName}
                          </span>
                          <span className="text-xs text-content-secondary">
                            {messageGroup.senderUsername}
                          </span>
                          <span className="text-xs text-content-tertiary">
                            {formatTimestamp(messageGroup.messages[0].created_at)}
                          </span>
                        </div>

                        {/* Messages from this sender */}
                        <div className="space-y-2">
                          {messageGroup.messages.map((message: MessageType, msgIndex: number) => (
                            <div key={message.id}>
                              {/* Show timestamp for subsequent messages if more than 5 minutes apart */}
                              {msgIndex > 0 && (
                                (() => {
                                  const prevTime = new Date(messageGroup.messages[msgIndex - 1].created_at).getTime();
                                  const currTime = new Date(message.created_at).getTime();
                                  const minutesDiff = (currTime - prevTime) / (1000 * 60);

                                  return minutesDiff > 5 ? (
                                    <div className="text-[10px] text-content-tertiary opacity-60 mt-1 mb-0.5 pl-0.5">
                                      {formatTimestamp(message.created_at)}
                                    </div>
                                  ) : null;
                                })()
                              )}

                              {/* Message content */}
                              <div>
                                <MarkdownPreview content={message.content} fullWidth />
                              </div>
                            </div>
                          ))}
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
