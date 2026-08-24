import React from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { formatDistanceToNow } from 'date-fns';
import type { AudienceConversationListItem } from '../../types/conversations';
import { Badge, CardBody } from '../ui';
import CharacterAvatar from '../CharacterAvatar';
import { useGameContext } from '../../contexts/GameContext';

interface AudienceConversationCardProps {
  conversation: AudienceConversationListItem;
  isSelected?: boolean;
}

export const AudienceConversationCard: React.FC<AudienceConversationCardProps> = ({
  conversation,
  isSelected = false
}) => {
  const { allGameCharacters, game } = useGameContext();
  const portraitAvatars = game?.portrait_avatars ?? false;
  const [searchParams] = useSearchParams();

  // Navigation happens purely through this href. The card previously also
  // called an onClick that set the same URL param, which pushed a second
  // identical history entry — so the first Back press appeared to do nothing.
  const href = (() => {
    const params = new URLSearchParams(searchParams);
    params.set('audienceConversation', String(conversation.conversation_id));
    return `?${params.toString()}`;
  })();

  // Look up avatar URL by character ID from global character data
  const getAvatarUrl = (characterId: number | null | undefined): string | null => {
    if (!characterId) return null;
    return allGameCharacters.find(c => c.id === characterId)?.avatar_url ?? null;
  };

  const messageCountLabel = `${conversation.message_count} messages`;

  // Check if conversation had activity in last 24 hours
  const isRecentActivity = () => {
    if (!conversation.last_message_at) return false;
    const lastMessageDate = new Date(conversation.last_message_at);
    const dayAgo = new Date(Date.now() - 24 * 60 * 60 * 1000);
    return lastMessageDate > dayAgo;
  };

  // Format participant names for display
  const getParticipantDisplay = () => {
    if (!conversation.participant_names || conversation.participant_names.length === 0) {
      return 'No participants';
    }
    return conversation.participant_names.join(', ');
  };

  // Get up to 4 participant initials for avatars
  const getParticipantAvatars = () => {
    if (!conversation.participant_names) return [];
    return conversation.participant_names.slice(0, 4);
  };

  const additionalParticipants = conversation.participant_names
    ? Math.max(0, conversation.participant_names.length - 4)
    : 0;

  const hasRecentActivity = isRecentActivity();

  return (
    <Link
      to={href}
      data-testid="conversation-item"
      className={`
        block
        cursor-pointer
        rounded-lg
        p-4
        transition-all
        duration-200
        hover:shadow-lg
        ${isSelected ? 'border border-border-primary border-l-4 border-l-blue-500 bg-bg-secondary' : 'border border-border-primary hover:bg-bg-secondary'}
      `}
    >
      <CardBody>
        <div className="flex flex-col gap-3">
          {/* Header: Avatars + Recent indicator */}
          <div className="flex items-start justify-between gap-2">
            {/* Participant Avatars (Slack-style overlap) */}
            <div className={`flex items-center ${portraitAvatars ? 'gap-1' : '-space-x-2'}`}>
              {getParticipantAvatars().map((name, index) => (
                <div
                  key={index}
                  className={`${portraitAvatars ? 'rounded' : 'rounded-full'} border-2 border-theme-default shadow-sm`}
                  style={{ zIndex: getParticipantAvatars().length - index }}
                  title={name}
                >
                  <CharacterAvatar
                    characterName={name}
                    avatarUrl={getAvatarUrl(conversation.participant_character_ids?.[index])}
                    size="sm"
                    shape={portraitAvatars ? 'portrait' : 'circle'}
                  />
                </div>
              ))}
              {additionalParticipants > 0 && (
                <div
                  className="h-10 w-10 rounded-full bg-content-tertiary text-white flex items-center justify-center text-xs font-medium border-2 border-theme-default shadow-sm"
                  style={{ zIndex: 0 }}
                  title={`+${additionalParticipants} more`}
                >
                  +{additionalParticipants}
                </div>
              )}
            </div>

            {/* Recent activity indicator */}
            {hasRecentActivity && (
              <div className="flex items-center gap-1.5" title="Active in last 24 hours">
                <div className="h-2 w-2 rounded-full bg-green-500 animate-pulse"></div>
                <span className="text-xs text-content-secondary hidden sm:inline">Recent</span>
              </div>
            )}
          </div>

          {/* Title/Subject */}
          <div className="flex items-start justify-between gap-2">
            <h3 className="text-base font-bold text-content-primary line-clamp-1">
              {conversation.subject || 'Conversation'}
            </h3>
            <Badge variant="neutral" size="sm">
              {messageCountLabel}
            </Badge>
          </div>

          {/* Participants */}
          <p className="text-sm text-content-secondary line-clamp-1">
            {getParticipantDisplay()}
          </p>

          {/* Last Message Preview */}
          {conversation.last_message_content && conversation.last_sender_name && (
            <div className="flex flex-col gap-1">
              <p className="text-sm text-content-primary line-clamp-2">
                <span className="font-medium text-content-primary">
                  {conversation.last_sender_name}:
                </span>{' '}
                {conversation.last_message_content}
              </p>
            </div>
          )}

          {/* Timestamp */}
          {conversation.last_message_at && (
            <p className="text-xs text-content-tertiary">
              {formatDistanceToNow(new Date(conversation.last_message_at), { addSuffix: true })}
            </p>
          )}
        </div>
      </CardBody>
    </Link>
  );
};
