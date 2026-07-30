import { useState, useEffect } from 'react';
import { RefreshCw } from 'lucide-react';
import { useUrlParam } from '../hooks/useUrlParam';
import { ConversationList } from './ConversationList';
import { MessageThread } from './MessageThread';
import { NewConversationModal } from './NewConversationModal';
import { ConversationProvider, useConversation } from '../contexts/ConversationContext';
import { useGameContext } from '../contexts/GameContext';
import type { Character } from '../types/characters';
import { Button, Alert } from './ui';
import { logger } from '@/services/LoggingService';

interface PrivateMessagesProps {
  gameId: number;
  characters: Character[];
  isAnonymous: boolean;
  allowGroupConversations: boolean;
  currentPhaseType?: string; // Current game phase type (common_room, action, results, etc.)
}

/**
 * Inner component that uses ConversationContext
 */
function PrivateMessagesInner({ gameId, characters, isAnonymous, allowGroupConversations, currentPhaseType }: PrivateMessagesProps) {
  const [showNewConversationModal, setShowNewConversationModal] = useState(false);
  const { allGameCharacters } = useGameContext();
  const {
    selectedConversationId,
    loadingConversations,
    selectConversation,
    loadConversations,
  } = useConversation();

  const [conversationParam, setConversationParam] = useUrlParam<number | null>('conversation', null, {
    deserialize: (s) => parseInt(s, 10) || null,
    serialize: (v) => v === null || v === undefined ? '' : String(v),
    replace: true,
  });

  const isMessagingAllowed = currentPhaseType === 'common_room' || currentPhaseType === 'interlude';

  logger.debug('PrivateMessages component state', {
    selectedConversationId,
    charactersCount: characters.length,
    gameId,
    currentPhaseType,
    isMessagingAllowed
  });

  // Load conversations on mount and when gameId changes
  useEffect(() => {
    loadConversations(gameId);
  }, [gameId, loadConversations]);

  // Sync URL param → context on mount and when param changes
  useEffect(() => {
    if (conversationParam !== selectedConversationId) {
      selectConversation(conversationParam);
    }
  }, [conversationParam]); // eslint-disable-line react-hooks/exhaustive-deps

  const handleConversationCreated = (conversationId: number) => {
    logger.debug('Conversation created', { conversationId, gameId });
    // Refresh conversations list to show the new conversation
    loadConversations(gameId);
    // Select the new conversation
    setConversationParam(conversationId);
  };

  const handleSelectConversation = (conversationId: number) => {
    logger.debug('Conversation selected', { conversationId, gameId });
    setConversationParam(conversationId);
  };

  const handleBackToList = () => {
    setConversationParam(null);
  };

  const handleRefreshConversations = async () => {
    await loadConversations(gameId);
    logger.debug('Refreshed conversation list', { gameId });
  };

  return (
    <div className="h-full">
      {!selectedConversationId ? (
        /* Conversation List (full screen) */
        <div className="h-full flex flex-col surface-base">
          <div className="p-4 border-b border-theme-default surface-raised">
            <div className="flex items-center justify-between mb-2">
              <h2 className="text-lg font-bold text-content-primary">Private Messages</h2>
              <div className="flex items-center gap-2">
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={handleRefreshConversations}
                  disabled={loadingConversations}
                  className="flex items-center gap-2"
                  aria-label="Refresh conversation list"
                >
                  <RefreshCw className={`w-4 h-4 ${loadingConversations ? 'animate-spin' : ''}`} />
                </Button>
                <Button
                  variant="primary"
                  size="sm"
                  onClick={() => setShowNewConversationModal(true)}
                  disabled={!isMessagingAllowed}
                  title={!isMessagingAllowed ? 'New conversations can only be started during Common Room or Interlude phases' : 'Start a new private conversation'}
                  data-faro-user-action-name="start-conversation"
                >
                  + New
                </Button>
              </div>
            </div>
            {!isMessagingAllowed && (
              <Alert variant="info" className="mt-2">
                You can read message history, but new messages can only be sent during Common Room or Interlude phases.
              </Alert>
            )}
          </div>

          <div className="flex-1 overflow-y-auto scrollbar-thin scrollbar-thumb-border-primary scrollbar-track-transparent hover:scrollbar-thumb-border-secondary">
            <ConversationList
              gameId={gameId}
              onSelectConversation={handleSelectConversation}
              selectedConversationId={selectedConversationId || undefined}
            />
          </div>
        </div>
      ) : (
        /* Message Thread (full screen with centered content on desktop) */
        <div className="h-full flex flex-col surface-base">
          {/* Thread - centered with max-width for better readability on desktop */}
          <div className="flex-1 overflow-hidden">
            <div className="h-full max-w-7xl mx-auto">
              <MessageThread
                gameId={gameId}
                conversationId={selectedConversationId}
                characters={characters}
                currentPhaseType={currentPhaseType}
                onBack={handleBackToList}
              />
            </div>
          </div>
        </div>
      )}

      {showNewConversationModal && (
        <NewConversationModal
          gameId={gameId}
          characters={characters}
          allCharacters={allGameCharacters}
          isAnonymous={isAnonymous}
          allowGroupConversations={allowGroupConversations}
          onClose={() => setShowNewConversationModal(false)}
          onConversationCreated={handleConversationCreated}
        />
      )}
    </div>
  );
}

/**
 * PrivateMessages - Full-screen messaging interface
 *
 * Uses a mobile-first full-screen pattern for all screen sizes:
 * - Conversation list OR message thread (not both simultaneously)
 * - Back button navigation from thread to list
 * - Maximum screen space for reading messages (primary use case)
 * - Consistent UX across mobile, tablet, and desktop
 *
 * Layout follows modern messaging apps (Slack, Discord, WhatsApp Web):
 * - List view: Full-width conversation cards
 * - Thread view: Full-screen messages with centered content on desktop
 */
export function PrivateMessages(props: PrivateMessagesProps) {
  return (
    <ConversationProvider>
      <PrivateMessagesInner {...props} />
    </ConversationProvider>
  );
}
