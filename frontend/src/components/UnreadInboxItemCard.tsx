import { useState } from 'react';
import { formatDistanceToNow } from 'date-fns';
import { MessageSquare, Reply, AtSign } from 'lucide-react';
import { Button, Spinner, Alert } from './ui';
import { MarkdownPreview } from './MarkdownPreview';
import CharacterAvatar from './CharacterAvatar';
import { UnreadReplyBox } from './UnreadReplyBox';
import { ParentCommentPreview } from './ParentCommentPreview';
import { useUnreadItemContext } from '../hooks/useUnreadItemContext';
import { useReplyToUnread } from '../hooks/useReplyToUnread';
import type { UnreadInboxItem } from '../types/unreadInbox';

interface UnreadInboxItemCardProps {
  item: UnreadInboxItem;
}

const TYPE_ICON: Record<string, React.ReactNode> = {
  private_message: <MessageSquare className="w-4 h-4" />,
  comment_reply: <Reply className="w-4 h-4" />,
  character_mention: <AtSign className="w-4 h-4" />,
};

export function UnreadInboxItemCard({ item }: UnreadInboxItemCardProps) {
  const [isExpanded, setIsExpanded] = useState(false);
  const { data: context, isLoading, isError } = useUnreadItemContext(item, isExpanded);
  const replyMutation = useReplyToUnread();

  //Content autosave id for comment textbox
  const autosaveRefId = item.kind === 'comment' ? `post-reply-${item.commentId}` : `conversation-${item.conversationId}`;

  const handleSubmit = (characterId: number, content: string) => {
    replyMutation.mutate({
      item,
      characterId,
      content,
      rootPostId: context?.kind === 'comment' ? context.rootPostId : undefined,
    });
  };

  return (
    <div className="border border-theme-default rounded-md" data-testid="unread-inbox-item">
      <Button
        variant="ghost"
        onClick={() => setIsExpanded((prev) => !prev)}
        className="w-full justify-start gap-3 p-3 text-left font-normal"
        aria-expanded={isExpanded}
        data-testid="unread-inbox-item-toggle"
      >
        <span className="text-interactive-primary flex-shrink-0">
          {TYPE_ICON[item.notification.type] ?? <MessageSquare className="w-4 h-4" />}
        </span>
        <span className="flex-1 text-sm text-content-primary">{item.notification.title}</span>
      </Button>

      {isExpanded && (
        <div className="px-3 pb-3 space-y-3">
          {isLoading && (
            <div className="flex items-center gap-2 text-content-secondary text-sm">
              <Spinner size="sm" /> Loading...
            </div>
          )}

          {isError && <Alert variant="danger">Couldn't load this item. Try refreshing.</Alert>}

          {context && (
            <>
              <div className="bg-bg-secondary rounded-lg p-3" data-testid="unread-inbox-item-context">
                {context.parentMessage && (
                  <ParentCommentPreview
                    content={context.parentMessage.content}
                    createdAt={context.parentMessage.createdAt}
                    isDeleted={context.parentMessage.isDeleted}
                    authorUsername={context.parentMessage.authorUsername}
                    characterId={context.parentMessage.characterId}
                    characterName={context.parentMessage.characterName}
                    characterAvatarUrl={context.parentMessage.characterAvatarUrl}
                    mentionedCharacters={context.mentionableCharacters}
                    hideViewInThread
                  />
                )}
                <div className="flex items-start gap-3 mb-2">
                  <CharacterAvatar
                    avatarUrl={context.previewMessage.characterAvatarUrl}
                    characterName={context.previewMessage.characterName || context.previewMessage.authorUsername || 'Unknown'}
                    size="sm"
                  />
                  <div className="flex flex-col flex-1">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span className="font-medium text-text-heading">
                        {context.previewMessage.characterName || 'Unknown'}
                      </span>
                      {context.previewMessage.authorUsername && (
                        <span className="text-sm text-content-tertiary">@{context.previewMessage.authorUsername}</span>
                      )}
                      {context.previewMessage.createdAt && (
                        <span className="text-sm text-content-tertiary">
                          {formatDistanceToNow(new Date(context.previewMessage.createdAt), { addSuffix: true })}
                        </span>
                      )}
                    </div>
                  </div>
                </div>
                <MarkdownPreview
                  content={context.previewMessage.content}
                  fullWidth
                  mentionedCharacters={context.mentionableCharacters}
                />
              </div>

              {context.kind === 'comment' && context.isReplyDisabled ? (
                <p className="text-sm text-content-tertiary italic">
                  This comment has been deleted, so you can't reply to it here.
                </p>
              ) : (
                <UnreadReplyBox
                  controllableCharacters={context.controllableCharacters}
                  mentionableCharacters={context.mentionableCharacters}
                  defaultCharacterId={context.defaultCharacterId}
                  onSubmit={handleSubmit}
                  isSubmitting={replyMutation.isPending}
                  error={replyMutation.isError ? 'Failed to send reply. Please try again.' : null}
                  autosaveRefId={autosaveRefId}
                />
              )}
            </>
          )}
        </div>
      )}
    </div>
  );
}
