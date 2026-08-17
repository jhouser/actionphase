import { useState, useEffect, useRef } from 'react';
import { RefreshCw } from 'lucide-react';
import { useLocation } from 'react-router-dom';
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
  const [prefilledParticipantIds, setPrefilledParticipantIds] = useState<number[]>([]);
  const { allGameCharacters } = useGameContext();
  const {
    selectedConversationId,
    loadingConversations,
    selectConversation,
    loadConversations,
    refreshConversation,
  } = useConversation();

  // Identifies each navigation, so re-entering the conversation already on
  // screen (e.g. via a notification link) is distinguishable from a re-render.
  const { key: locationKey } = useLocation();
  const initialLocationKeyRef = useRef(locationKey);

  const [conversationParam, setConversationParam] = useUrlParam<number | null>('conversation', null, {
    deserialize: (s) => parseInt(s, 10) || null,
    serialize: (v) => v === null || v === undefined ? '' : String(v),
    replace: true,
  });

  // Set by the envelope shortcut on a character sheet/profile: open the New
  // Conversation form with that character already selected as a participant.
  const [newConversationWith, setNewConversationWith] = useUrlParam<number | null>('newConversationWith', null, {
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

  // Sync URL param → context on mount and when param changes.
  //
  // Keyed on location.key, not just the param, so that navigating to the
  // conversation already on screen still does something. A notification for a
  // new reply links to the exact URL you are already viewing; the param is
  // unchanged, so a param-only dependency never re-runs and the click appears
  // dead. location.key is fresh on every navigation, including same-URL ones,
  // which lets us tell "the user arrived here again" from "this component
  // merely re-rendered" and refetch the thread to pull in the new reply.
  useEffect(() => {
    if (conversationParam !== selectedConversationId) {
      selectConversation(conversationParam);
      return;
    }
    // Same conversation, new navigation. Skip the very first render, where the
    // conversation is already being loaded by the selection above — refreshing
    // on top of that duplicates the request and fires a bogus "new message" toast.
    if (conversationParam === null || locationKey === initialLocationKeyRef.current) {
      return;
    }
    refreshConversation(gameId, conversationParam);
  }, [conversationParam, locationKey]); // eslint-disable-line react-hooks/exhaustive-deps

  // Honour the envelope shortcut once, then clear the param so a later refresh
  // or back-navigation doesn't reopen the form. The selected participant moves
  // into state first, since clearing the param unmounts nothing but does remove
  // the value the modal reads.
  useEffect(() => {
    if (newConversationWith === null) return;
    if (isMessagingAllowed) {
      setPrefilledParticipantIds([newConversationWith]);
      setShowNewConversationModal(true);
      setConversationParam(null);
    }
    setNewConversationWith(null);
  }, [newConversationWith, isMessagingAllowed]); // eslint-disable-line react-hooks/exhaustive-deps

  const handleCloseNewConversationModal = () => {
    setShowNewConversationModal(false);
    setPrefilledParticipantIds([]);
  };

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
          {/* min-h-0 (not overflow-hidden) lets this flex child shrink to its
              container without establishing a clipping context. overflow-hidden
              here would become the sticky containing block for the thread
              header and prevent it from sticking to the viewport. */}
          <div className="flex-1 min-h-0">
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
          initialParticipantIds={prefilledParticipantIds}
          onClose={handleCloseNewConversationModal}
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
