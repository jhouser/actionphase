import { useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { apiClient } from '../lib/api';
import { useCharacterComments } from '../hooks/useCharacterComments';
import { useInfiniteScrollSentinel } from '../hooks/useInfiniteScrollSentinel';
import { useCharacterStats } from '../hooks/useCharacterStats';
import CharacterAvatar from '../components/CharacterAvatar';
import { useOptionalGameContext } from '../contexts/GameContext';
import { ParentCommentPreview } from '../components/ParentCommentPreview';
import { MarkdownPreview } from '../components/MarkdownPreview';
import { CollapsibleMarkdown } from '../components/CollapsibleMarkdown';
import { Spinner, Alert, Badge, Card, CardBody } from '../components/ui';
import { formatDistanceToNow } from 'date-fns';
import type { CharacterMessage } from '../types/messages';
import { CharacterActivityStats } from '../components/CharacterActivityStats';
import { MessageCharacterButton } from '../components/MessageCharacterButton';

/**
 * CharacterPage - Displays a character's profile and public activity feed
 *
 * Features:
 * - Character name and avatar
 * - Paginated feed of all public posts and comments by that character
 * - Infinite scroll to load more
 * - Links to navigate to each message in context
 *
 * Route: /characters/:characterId
 */
export function CharacterPage() {
  const { characterId } = useParams<{ characterId: string }>();
  const navigate = useNavigate();
  const gameContext = useOptionalGameContext();

  useEffect(() => {
    window.scrollTo(0, 0);
  }, []);

  const characterIdNum = characterId ? parseInt(characterId, 10) : undefined;

  const {
    data: characterData,
    isLoading: isLoadingCharacter,
    isError: isCharacterError,
  } = useQuery({
    // Singular 'character' is the shared key for a single character record —
    // CharacterSheet reads it and the avatar/update mutations invalidate it, so
    // this page picks up those changes instead of serving a stale copy.
    queryKey: ['character', characterIdNum],
    queryFn: () => apiClient.characters.getCharacter(characterIdNum!).then(res => res.data),
    enabled: !!characterIdNum && !isNaN(characterIdNum),
  });

  const { data: gameData } = useQuery({
    queryKey: ['games', characterData?.game_id],
    queryFn: () => apiClient.games.getGame(characterData!.game_id).then(res => res.data),
    enabled: !!characterData?.game_id && !gameContext,
  });

  const portraitAvatars = gameContext?.game?.portrait_avatars ?? gameData?.portrait_avatars ?? false;

  // Same query key as CharacterSheet so the two share a cache entry.
  const { data: characterFields } = useQuery({
    queryKey: ['characterData', characterIdNum],
    queryFn: () => apiClient.characters.getCharacterData(characterIdNum!).then(res => res.data),
    enabled: !!characterIdNum && !isNaN(characterIdNum),
  });

  // The public bio is the `background` field of the `bio` module. The endpoint
  // already withholds private fields from ordinary viewers, but editors, the
  // audience and everyone in a completed game receive the private ones too —
  // so filter on is_public here as well rather than trusting the payload to
  // contain only public data.
  const publicBio = characterFields?.find(
    (field) =>
      field.module_type === 'bio' &&
      field.field_name === 'background' &&
      field.is_public
  )?.field_value?.trim();

  const { data: statsData } = useCharacterStats(characterIdNum);

  const {
    data,
    isLoading: isLoadingMessages,
    isError: isMessagesError,
    error: messagesError,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
  } = useCharacterComments(characterIdNum);

  // Infinite scroll
  const sentinelRef = useInfiniteScrollSentinel({
    enabled: hasNextPage && !isFetchingNextPage,
    onIntersect: fetchNextPage,
    threshold: 0.1,
  });

  if (!characterId || isNaN(parseInt(characterId, 10))) {
    return (
      <div className="min-h-screen bg-surface-sunken py-8">
        <div className="max-w-5xl mx-auto px-4">
          <Alert variant="danger">Invalid character ID.</Alert>
        </div>
      </div>
    );
  }

  const allMessages = data?.pages.flatMap((page) => page.messages) ?? [];

  return (
    <div className="min-h-screen bg-surface-sunken py-8">
      <div className="max-w-5xl mx-auto px-4 sm:px-6">
        {/* Character Header */}
        <div className="mb-8">
          {isLoadingCharacter ? (
            <div className="flex items-center gap-4">
              <div className="w-20 h-20 rounded-full bg-bg-secondary animate-pulse" />
              <div className="space-y-2">
                <div className="h-7 w-48 bg-bg-secondary rounded animate-pulse" />
                <div className="h-5 w-32 bg-bg-secondary rounded animate-pulse" />
              </div>
            </div>
          ) : isCharacterError ? (
            <Alert variant="danger">Failed to load character.</Alert>
          ) : characterData ? (
            <div className="flex items-center gap-4">
              <CharacterAvatar
                avatarUrl={characterData.avatar_url}
                characterName={characterData.name}
                size="xl"
                shape={portraitAvatars ? 'portrait' : 'circle'}
              />
              <div className="min-w-0">
                <div className="flex items-center gap-2">
                  <h1 className="min-w-0 text-2xl font-bold text-text-heading break-words">
                    {characterData.name}
                  </h1>
                  <MessageCharacterButton character={characterData} />
                </div>
                <div className="flex items-center gap-2 mt-1">
                  {characterData.character_type && (
                    <Badge variant="secondary">{characterData.character_type === 'npc' ? 'NPC' : 'Player Character'}</Badge>
                  )}
                  {characterData.status && (
                    <Badge variant={characterData.status === 'approved' ? 'success' : 'secondary'}>
                      {characterData.status}
                    </Badge>
                  )}
                </div>
                {characterData.username && (
                  <p className="text-sm text-content-tertiary mt-1">
                    Played by{' '}
                    <a
                      href={`/users/${characterData.username}`}
                      className="text-interactive-primary hover:text-accent-secondary"
                    >
                      @{characterData.username}
                    </a>
                  </p>
                )}
              </div>
            </div>
          ) : null}

          {/* Activity Stats */}
          {statsData && (
            <CharacterActivityStats stats={statsData} className="mt-4" />
          )}
        </div>

        {/* Public Bio */}
        {publicBio && (
          <div className="mb-8">
            <h2 className="text-lg font-semibold text-text-heading mb-4">About</h2>
            <Card variant="default" padding="md">
              <CardBody>
                {/* Card's default variant paints surface-base, so the fade
                    blends to that token. */}
                <CollapsibleMarkdown
                  content={publicBio}
                  fullWidth
                  fadeSurface="surface-base"
                  expandLabel="Show More"
                  collapseLabel="Show Less"
                  data-testid="character-bio"
                />
              </CardBody>
            </Card>
          </div>
        )}

        {/* Activity Feed */}
        <div>
          <h2 className="text-lg font-semibold text-text-heading mb-4">Activity</h2>

          {isLoadingMessages && (
            <div className="flex justify-center py-12">
              <Spinner size="lg" />
            </div>
          )}

          {isMessagesError && (
            <Alert variant="danger">
              <p>Failed to load activity</p>
              <p className="text-sm mt-1">
                {messagesError instanceof Error ? messagesError.message : 'Unknown error'}
              </p>
            </Alert>
          )}

          {!isLoadingMessages && !isMessagesError && allMessages.length === 0 && (
            <div className="text-center py-12">
              <p className="text-content-secondary">No public activity yet.</p>
            </div>
          )}

          {allMessages.length > 0 && (
            <div className="space-y-4">
              {allMessages.map((message) => (
                <CharacterMessageCard
                  key={message.id}
                  message={message}
                  portraitAvatars={portraitAvatars}
                  onNavigate={() => {
                    if (!characterData) return;
                    const url = `/games/${message.game_id}?tab=common-room&comment=${message.id}`;
                    navigate(url);
                  }}
                />
              ))}

              {/* Infinite scroll sentinel */}
              <div ref={sentinelRef} className="h-16 flex items-center justify-center">
                {isFetchingNextPage && <Spinner size="md" />}
                {!hasNextPage && allMessages.length > 0 && (
                  <p className="text-sm text-content-tertiary">No more activity to load</p>
                )}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

interface CharacterMessageCardProps {
  message: CharacterMessage;
  onNavigate: () => void;
  portraitAvatars: boolean;
}

function CharacterMessageCard({ message, onNavigate, portraitAvatars }: CharacterMessageCardProps) {
  const utcDateString = message.created_at.endsWith('Z')
    ? message.created_at
    : `${message.created_at}Z`;
  const timeAgo = formatDistanceToNow(new Date(utcDateString), { addSuffix: true });
  const isEdited = message.edit_count > 0;

  return (
    <Card className="hover:shadow-md transition-shadow">
      <CardBody>
        {/* If this is a comment, show parent context */}
        {message.message_type === 'comment' && message.parent && (
          <ParentCommentPreview
            content={message.parent.content}
            createdAt={message.parent.created_at}
            isDeleted={message.parent.is_deleted}
            messageType={message.parent.message_type}
            authorUsername={message.parent.author_username}
            characterName={message.parent.character_name}
            characterAvatarUrl={message.parent.character_avatar_url}
            portraitAvatars={portraitAvatars}
          />
        )}

        {/* Message header */}
        <div className="flex items-center gap-3 mb-2">
          <CharacterAvatar
            avatarUrl={message.character_avatar_url}
            characterName={message.character_name || message.author_username}
            size="sm"
            shape={portraitAvatars ? 'portrait' : 'circle'}
          />
          <div className="flex flex-col min-w-0">
            <span className="font-medium text-text-heading leading-tight">{message.character_name || message.author_username}</span>
            <div className="flex items-center gap-2">
              {message.author_username && (
                <>
                  <span className="text-sm text-content-tertiary">@{message.author_username}</span>
                  <span className="text-sm text-content-tertiary">·</span>
                </>
              )}
              <span className="text-sm text-content-tertiary">{timeAgo}</span>
              {isEdited && <span className="text-sm text-content-tertiary">(edited)</span>}
            </div>
          </div>
        </div>

        {/* Message content */}
        <div>
          {message.is_deleted ? (
            <p className="text-content-tertiary italic">[deleted]</p>
          ) : (
            <MarkdownPreview content={message.content} />
          )}
        </div>

        {/* Link to view in context */}
        {!message.is_deleted && (
          <div className="mt-3 pt-3 border-t border-border-primary">
            <a
              href={`/games/${message.game_id}?tab=common-room&comment=${message.id}`}
              onClick={(e) => {
                e.preventDefault();
                onNavigate();
              }}
              className="text-sm text-interactive-primary hover:text-accent-secondary font-medium"
            >
              View in thread →
            </a>
          </div>
        )}
      </CardBody>
    </Card>
  );
}
