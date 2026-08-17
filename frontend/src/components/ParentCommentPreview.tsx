import { useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { formatDistanceToNow } from 'date-fns';
import { CollapsibleMarkdown } from './CollapsibleMarkdown';
import CharacterAvatar from './CharacterAvatar';
import { useOptionalGameContext } from '../contexts/GameContext';
import type { Character } from '../types/characters';

interface ParentCommentPreviewProps {
  content?: string | null;
  createdAt?: string | null;
  isDeleted?: boolean | null;
  messageType?: string | null;
  authorUsername?: string | null;
  characterId?: number | null;
  characterName?: string | null;
  characterAvatarUrl?: string | null;
  onNavigateToParent?: () => void;
  mentionedCharacters?: Character[];
  defaultExpanded?: boolean;
  hideViewInThread?: boolean;
  portraitAvatars?: boolean;
}

/**
 * Shows a preview of the parent message (post or comment) that was replied to.
 * Can be expanded to show the full content, or collapsed to show just a preview.
 */
export function ParentCommentPreview({
  content,
  createdAt,
  isDeleted,
  messageType: _messageType,
  authorUsername,
  characterId,
  characterName,
  characterAvatarUrl,
  onNavigateToParent,
  mentionedCharacters = [],
  defaultExpanded = false,
  hideViewInThread = false,
  portraitAvatars: portraitAvatarsProp,
}: ParentCommentPreviewProps) {
  const gameContext = useOptionalGameContext();
  const portraitAvatars = portraitAvatarsProp ?? gameContext?.game?.portrait_avatars ?? false;
  const [isExpanded, setIsExpanded] = useState(defaultExpanded);

  // Memoized (and declared above the early return, so hook order stays stable):
  // a fresh array each render busts MarkdownPreview's React.memo boundary, which
  // re-runs dangerouslySetInnerHTML and leaves mention tooltips stuck open.
  const mentions = useMemo(
    () => mentionedCharacters.map(char => ({
      id: char.id,
      name: char.name,
      username: char.username,
      character_type: char.character_type,
      avatar_url: char.avatar_url ?? undefined,
    })),
    [mentionedCharacters]
  );

  // If there's no parent content, don't render anything
  if (!content && !isDeleted) {
    return null;
  }

  const timeAgo = createdAt
    ? formatDistanceToNow(new Date(createdAt), { addSuffix: true })
    : null;

  return (
    <div className="border-l-2 border-border-secondary pl-3 mb-3 opacity-70">
      <div className="flex items-start justify-between mb-2 gap-2">
        <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-sm min-w-0">
          {characterName && (
            <CharacterAvatar
              avatarUrl={characterAvatarUrl}
              characterName={characterName}
              size="xs"
              shape={portraitAvatars ? 'portrait' : 'circle'}
            />
          )}
          {characterName ? (
            characterId ? (
              <Link to={`/characters/${characterId}`} className="font-medium text-text-heading hover:underline">{characterName}</Link>
            ) : (
              <span className="font-medium text-text-heading">{characterName}</span>
            )
          ) : authorUsername ? (
            <span className="text-content-secondary">@{authorUsername}</span>
          ) : null}
          {timeAgo && (
            <span className="text-content-tertiary">{timeAgo}</span>
          )}
        </div>

        {!isDeleted && (
          <button
            onClick={() => setIsExpanded(!isExpanded)}
            className="text-xs text-interactive-primary hover:text-interactive-secondary flex items-center gap-1 shrink-0"
          >
            <svg
              className={`w-4 h-4 transition-transform ${isExpanded ? 'rotate-180' : ''}`}
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
            </svg>
            {isExpanded ? 'Collapse' : 'Expand'}
          </button>
        )}
      </div>

      {isDeleted ? (
        <div className="text-sm text-content-tertiary italic">[deleted]</div>
      ) : (
        <div className={isExpanded ? 'text-sm' : 'text-sm text-content-secondary [&_p]:my-0'}>
          {/* Collapsed height stands in for the old line-clamp-2: ~2 lines of
              text-sm. The toggle lives in the header row above, so this renders
              the clip and fade only. */}
          <CollapsibleMarkdown
            content={content || ''}
            mentionedCharacters={mentions}
            fullWidth
            collapsedMaxHeight={44}
            expanded={isExpanded}
            showToggle={false}
          />
        </div>
      )}

      {onNavigateToParent && !isDeleted && !hideViewInThread && (
        <button
          onClick={onNavigateToParent}
          className="text-xs text-interactive-primary hover:text-accent-secondary mt-2 flex items-center gap-1"
        >
          <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
          </svg>
          View in thread
        </button>
      )}
    </div>
  );
}
