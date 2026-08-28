import React from 'react';
import type { AudienceConversationListItem } from '../../types/conversations';
import { Badge, Button } from '../ui';
import CharacterAvatar from '../CharacterAvatar';
import { useOptionalGameContext } from '../../contexts/GameContext';

interface AudienceConversationHeaderProps {
  conversation: AudienceConversationListItem;
  messageCount: number;
  onBack: () => void;
}

/**
 * Header component for audience conversation detail view
 * Shows conversation metadata, participants, and navigation
 */
export const AudienceConversationHeader: React.FC<AudienceConversationHeaderProps> = ({
  conversation,
  messageCount,
  onBack,
}) => {
  const gameContext = useOptionalGameContext();
  const portraitAvatars = gameContext?.game?.portrait_avatars ?? false;

  const getAvatarUrl = (characterId: number | null | undefined): string | null => {
    if (!characterId || !gameContext) return null;
    return gameContext.allGameCharacters.find(c => c.id === characterId)?.avatar_url ?? null;
  };

  // Get up to 5 participant avatars for display
  const getParticipantAvatars = () => {
    if (!conversation.participant_names) return [];
    return conversation.participant_names.slice(0, 5);
  };

  const additionalParticipants = conversation.participant_names
    ? Math.max(0, conversation.participant_names.length - 5)
    : 0;

  const participantDisplay = conversation.participant_names && conversation.participant_names.length > 0
    ? conversation.participant_names.join(', ')
    : 'No participants';

  return (
    <div className="border-b border-theme-default surface-raised sticky top-0 z-10">
      {/* Mobile Layout */}
      <div className="md:hidden p-4 space-y-3">
        {/* Back button + Read-Only badge */}
        <div className="flex items-center justify-between gap-2">
          <Button variant="ghost" size="sm" onClick={onBack}>
            ← Back
          </Button>
          <Badge variant="primary" size="sm">
            Read-Only
          </Badge>
        </div>

        {/* Subject/Title */}
        <h2 className="text-lg font-bold text-content-primary">
          {conversation.subject || 'Conversation'}
        </h2>

        {/* Avatars + Message count */}
        <div className="flex items-center justify-between gap-3">
          {/* Participant Avatars */}
          <div className={`flex items-center ${portraitAvatars ? 'gap-1' : '-space-x-2'}`}>
            {getParticipantAvatars().map((name, index) => (
              <div
                key={index}
                className={`${portraitAvatars ? 'rounded' : 'rounded-full'} border-2 border-theme-default shadow-sm`}
                style={{ zIndex: getParticipantAvatars().length - index }}
                title={name}
              >
                <CharacterAvatar characterName={name} avatarUrl={getAvatarUrl(conversation.participant_character_ids?.[index])} size="xs" shape={portraitAvatars ? 'portrait' : 'circle'} />
              </div>
            ))}
            {additionalParticipants > 0 && (
              <div
                className="h-8 w-8 rounded-full surface-sunken text-content-secondary flex items-center justify-center text-xs font-medium border-2 border-theme-default shadow-sm"
                style={{ zIndex: 0 }}
                title={`+${additionalParticipants} more`}
              >
                +{additionalParticipants}
              </div>
            )}
          </div>

          {/* Message count */}
          <div className="text-sm text-content-secondary">
            {messageCount} {messageCount === 1 ? 'message' : 'messages'}
          </div>
        </div>

        {/* Participants list */}
        <p className="text-sm text-content-secondary line-clamp-2">
          {participantDisplay}
        </p>
      </div>

      {/* Desktop Layout */}
      <div className="hidden md:flex items-center justify-between p-4 gap-4">
        {/* Left side: Back button + Conversation info */}
        <div className="flex items-center gap-4 flex-1 min-w-0">
          <Button variant="ghost" size="sm" onClick={onBack}>
            ← Back
          </Button>

          {/* Participant Avatars */}
          <div className={`flex items-center flex-shrink-0 ${portraitAvatars ? 'gap-1' : '-space-x-2'}`}>
            {getParticipantAvatars().map((name, index) => (
              <div
                key={index}
                className={`${portraitAvatars ? 'rounded' : 'rounded-full'} border-2 border-theme-default shadow-sm`}
                style={{ zIndex: getParticipantAvatars().length - index }}
                title={name}
              >
                <CharacterAvatar characterName={name} avatarUrl={getAvatarUrl(conversation.participant_character_ids?.[index])} size="sm" shape={portraitAvatars ? 'portrait' : 'circle'} />
              </div>
            ))}
            {additionalParticipants > 0 && (
              <div
                className="h-10 w-10 rounded-full surface-sunken text-content-secondary flex items-center justify-center text-xs font-medium border-2 border-theme-default shadow-sm"
                style={{ zIndex: 0 }}
                title={`+${additionalParticipants} more`}
              >
                +{additionalParticipants}
              </div>
            )}
          </div>

          {/* Conversation metadata */}
          <div className="flex-1 min-w-0">
            <h2 className="text-xl font-bold text-content-primary truncate">
              {conversation.subject || 'Conversation'}
            </h2>
            <p className="text-sm text-content-secondary truncate">
              {participantDisplay}
            </p>
          </div>
        </div>

        {/* Right side: Message count + Read-Only badge */}
        <div className="flex items-center gap-3 flex-shrink-0">
          <div className="text-sm text-content-secondary">
            {messageCount} {messageCount === 1 ? 'message' : 'messages'}
          </div>
          <Badge variant="primary" size="sm">
            Read-Only
          </Badge>
        </div>
      </div>
    </div>
  );
};
