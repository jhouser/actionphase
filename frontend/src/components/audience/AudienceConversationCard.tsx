import React from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { formatDistanceToNow } from 'date-fns';
import type { AudienceConversationListItem } from '../../types/conversations';
import CharacterAvatar from '../CharacterAvatar';
import { useGameContext } from '../../contexts/GameContext';
import { useParticipantFit } from '../../hooks/useParticipantFit';

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

  const participantNames = conversation.participant_names ?? [];

  // How many avatars and names fit is measured from the real row rather than
  // chosen by breakpoint: a card narrowed by a sidebar, and a card holding one
  // unusually long character name, both need to collapse even though the
  // viewport says otherwise. Avatars are kept in preference to names.
  // No reserve: participants now own the full row width.
  const { containerRef, visibleAvatars, visibleNames } = useParticipantFit({
    names: participantNames,
  });

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

  const messageCountLabel = `${conversation.message_count} ${conversation.message_count === 1 ? 'message' : 'messages'}`;

  // Names that do not fit collapse into a "+N" suffix rather than truncating
  // mid-word, which reads as broken rather than deliberate.
  const getParticipantDisplay = () => {
    if (participantNames.length === 0) {
      return 'No participants';
    }
    const remaining = participantNames.length - visibleNames;
    // No name fits at all: the avatars still identify everyone, so a bare count
    // is more use than a name cut mid-word. Kept short ("6 people", not
    // "6 participants") because this renders in the narrowest rows there are —
    // a truncated fallback is worse than the truncation it replaces.
    if (visibleNames === 0) {
      return `${participantNames.length} people`;
    }
    const shown = participantNames.slice(0, visibleNames).join(', ');
    return remaining > 0 ? `${shown} +${remaining}` : shown;
  };

  const avatarNames = participantNames.slice(0, visibleAvatars);
  const additionalParticipants = Math.max(0, participantNames.length - visibleAvatars);

  return (
    <Link
      to={href}
      data-testid="conversation-item"
      className={`
        block
        cursor-pointer
        rounded-lg
        p-4
        surface-raised
        transition-all
        duration-200
        hover:shadow-lg
        hover:border-theme-strong
        ${isSelected
          ? 'border border-theme-default border-l-4 border-l-interactive-primary'
          : 'border border-theme-default'}
      `}
    >
      <div className="flex flex-col gap-2">
        {/* Who: avatars and names describe the same thing, so they share a row —
            and they get the row to themselves. The timestamp lives in the footer
            with the other stats, which leaves the full width for participants
            (the difference between naming everyone and "6 people" on mobile). */}
        <div ref={containerRef} className="flex items-center gap-3">
          <div className="flex items-center gap-3 min-w-0">
            {/* -space-x-2 pulls each avatar over its neighbour, which also eats
                into the gap before the names; mr-1 restores a visible break. */}
            <div className={`flex items-center flex-shrink-0 mr-1 ${portraitAvatars ? 'gap-1' : '-space-x-2'}`}>
              {avatarNames.map((name, index) => (
                <div
                  key={index}
                  className={`${portraitAvatars ? 'rounded' : 'rounded-full'} border-2 border-theme-default shadow-sm`}
                  style={{ zIndex: avatarNames.length - index }}
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
              {/* Sized to match a size="sm" avatar (w-8 h-8) so the stack keeps
                  one consistent height. */}
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
            <p data-participant-names className="text-sm text-content-secondary truncate">
              {getParticipantDisplay()}
            </p>
          </div>
        </div>

        {/* Subject and preview: the only high-contrast text on the card. */}
        <h3 className="text-base font-bold text-content-primary line-clamp-1">
          {conversation.subject || 'Conversation'}
        </h3>

        {conversation.last_message_content && conversation.last_sender_name && (
          <p className="text-sm text-content-primary line-clamp-2">
            <span className="font-medium text-content-primary">
              {conversation.last_sender_name}:
            </span>{' '}
            {conversation.last_message_content}
          </p>
        )}

        {/* Stats footer: quiet, and last in reading order. */}
        <div className="flex items-center flex-wrap gap-x-2 gap-y-1 text-xs text-content-tertiary">
          <span>{messageCountLabel}</span>
          {conversation.last_message_at && (
            <>
              <span aria-hidden="true">·</span>
              <span>
                {formatDistanceToNow(new Date(conversation.last_message_at), { addSuffix: true })}
              </span>
            </>
          )}
        </div>
      </div>
    </Link>
  );
};
