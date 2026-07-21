import { useState, useRef } from 'react';
import { Link } from 'react-router-dom';
import { formatDistanceToNow } from 'date-fns';
import type { CommentWithParent } from '../types/messages';
import { ParentCommentPreview } from './ParentCommentPreview';
import { MarkdownPreview } from './MarkdownPreview';
import { CommentEditor } from './CommentEditor';
import { Card, CardBody, Badge, Button, Select } from './ui';
import CharacterAvatar from './CharacterAvatar';
import { useGameContext } from '../contexts/GameContext';
import { useAuth } from '../contexts/AuthContext';
import { useAdminMode } from '../hooks/useAdminMode';
import { useScreenshotMode } from '../hooks/useScreenshotMode';
import { useUpdateComment, useDeleteComment } from '../hooks/useCommentMutations';
import { ConfirmModal } from './ConfirmModal';
import { useToast } from '../contexts/ToastContext';
import { apiClient } from '../lib/api';
import { logger } from '@/services/LoggingService';
import type { CommentReadMode } from '../lib/api/auth';

interface CommentWithParentCardProps {
  comment: CommentWithParent;
  gameId: number;
  onNavigateToParent?: () => void;
  onNavigateToComment?: () => void;
  commentReadMode?: CommentReadMode;
  isRead?: boolean;
  onToggleRead?: (currentlyRead: boolean) => void;
}

/**
 * Displays a comment with its parent context in a card.
 * Used in the "New Comments" view to show recent activity.
 */
export function CommentWithParentCard({
  comment,
  gameId,
  onNavigateToParent,
  onNavigateToComment,
  commentReadMode,
  isRead = false,
  onToggleRead,
}: CommentWithParentCardProps) {
  const { allGameCharacters, game, isGM, userCharacters } = useGameContext();
  const { currentUser } = useAuth();
  const { adminModeEnabled } = useAdminMode();
  const { screenshotModeEnabled } = useScreenshotMode();
  const { showError } = useToast();
  const updateCommentMutation = useUpdateComment();
  const deleteCommentMutation = useDeleteComment();
  const isMountedRef = useRef(true);

  const [isEditing, setIsEditing] = useState(false);
  const [editContent, setEditContent] = useState(comment.content);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const [linkCopied, setLinkCopied] = useState(false);
  const [isReplying, setIsReplying] = useState(false);
  const [replyContent, setReplyContent] = useState('');
  const [selectedCharacterId, setSelectedCharacterId] = useState<number | null>(
    () => userCharacters[0]?.id ?? null
  );
  const [isSubmittingReply, setIsSubmittingReply] = useState(false);
  const [replyPostedId, setReplyPostedId] = useState<number | null>(null);

  const portraitAvatars = game?.portrait_avatars ?? false;
  const isAuthor = currentUser?.id === comment.author_id;
  // Screenshot Mode hides Edit/Delete too: they only render for the author (or
  // the GM/admin), so a visible control gives away who's behind the character
  // just as plainly as the username would.
  const canDelete = (isAuthor || isGM || adminModeEnabled) && !comment.is_deleted && !!comment.post_id && !screenshotModeEnabled;
  const canEdit = isAuthor && !comment.is_deleted && !!comment.post_id && !screenshotModeEnabled;
  const canReply = !comment.is_deleted && userCharacters.length > 0 && !!comment.post_id;

  const handleCopyLink = async () => {
    const url = `${window.location.origin}/games/${gameId}?tab=common-room&comment=${comment.id}`;
    try {
      if (navigator.clipboard && window.isSecureContext) {
        await navigator.clipboard.writeText(url);
      } else {
        const textArea = document.createElement('textarea');
        textArea.value = url;
        textArea.style.position = 'fixed';
        textArea.style.left = '-999999px';
        document.body.appendChild(textArea);
        textArea.focus();
        textArea.select();
        try {
          document.execCommand('copy');
          textArea.remove();
        } catch (execErr) {
          textArea.remove();
          throw execErr;
        }
      }
      if (isMountedRef.current) {
        setLinkCopied(true);
        setTimeout(() => { if (isMountedRef.current) setLinkCopied(false); }, 2000);
      }
    } catch (_err) {
      logger.error('Failed to copy link', { error: _err, commentId: comment.id });
      showError(`Failed to copy. Link: ${url}`);
    }
  };

  const handleEdit = () => {
    setEditContent(comment.content);
    setIsEditing(true);
  };

  const handleCancelEdit = () => {
    setEditContent(comment.content);
    setIsEditing(false);
  };

  const handleSaveEdit = async () => {
    if (!editContent.trim() || editContent === comment.content || !comment.post_id) {
      setIsEditing(false);
      return;
    }
    try {
      await updateCommentMutation.mutateAsync({
        gameId,
        postId: comment.post_id,
        commentId: comment.id,
        data: { content: editContent.trim() },
      });
      setIsEditing(false);
    } catch (_err) {
      logger.error('Failed to update comment', { error: _err, commentId: comment.id, gameId });
      showError('Failed to update comment. Please try again.');
    }
  };

  const handleConfirmDelete = async () => {
    if (!comment.post_id) return;
    try {
      setIsDeleting(true);
      await deleteCommentMutation.mutateAsync({ gameId, postId: comment.post_id, commentId: comment.id });
      setShowDeleteConfirm(false);
    } catch (_err) {
      logger.error('Failed to delete comment', { error: _err, commentId: comment.id, gameId });
      showError('Failed to delete comment. Please try again.');
    } finally {
      setIsDeleting(false);
    }
  };

  const handleSubmitReply = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedCharacterId || !replyContent.trim() || !comment.post_id) return;
    try {
      setIsSubmittingReply(true);
      const response = await apiClient.messages.createComment(gameId, comment.id, {
        character_id: selectedCharacterId,
        content: replyContent.trim(),
        root_post_id: comment.post_id,
      });
      setReplyContent('');
      setIsReplying(false);
      setReplyPostedId(response.data.id);
      setTimeout(() => { if (isMountedRef.current) setReplyPostedId(null); }, 4000);
    } catch (_err) {
      logger.error('Failed to post reply', { error: _err, commentId: comment.id, gameId });
      showError('Failed to post reply. Please try again.');
    } finally {
      setIsSubmittingReply(false);
    }
  };

  const parentCharacterId = comment.parent_character_name
    ? allGameCharacters.find(c => c.name === comment.parent_character_name)?.id ?? null
    : null;

  // Backend returns UTC timestamps without 'Z' suffix
  // Append 'Z' to ensure proper UTC parsing
  const utcDateString = comment.created_at.endsWith('Z') ? comment.created_at : `${comment.created_at}Z`;
  const timeAgo = formatDistanceToNow(new Date(utcDateString), {
    addSuffix: true,
  });

  const isEdited = comment.edit_count > 0;
  const showReadButton = commentReadMode === 'manual' && !comment.is_deleted && onToggleRead;

  return (
    <Card className={`hover:shadow-md transition-shadow${isRead ? ' opacity-50' : ''}`}>
      <CardBody>
        {/* Parent context preview */}
        <ParentCommentPreview
          content={comment.parent_content}
          createdAt={comment.parent_created_at}
          isDeleted={comment.parent_is_deleted}
          messageType={comment.parent_message_type}
          authorUsername={screenshotModeEnabled ? null : comment.parent_author_username}
          characterId={parentCharacterId}
          characterName={comment.parent_character_name}
          characterAvatarUrl={comment.parent_character_avatar_url}
          onNavigateToParent={onNavigateToParent}
          hideViewInThread
        />

        {/* Comment header + body */}
        <div className="bg-bg-secondary rounded-lg p-3">
          <div className="flex items-start gap-3 mb-2">
            <CharacterAvatar
              avatarUrl={comment.character_avatar_url}
              characterName={comment.character_name || (screenshotModeEnabled ? '' : comment.author_username)}
              size="sm"
              shape={portraitAvatars ? 'portrait' : 'circle'}
            />
            <div className="flex flex-col flex-1">
              <div className="flex items-center gap-2 flex-wrap">
                <Link to={`/characters/${comment.character_id}`} className="font-medium text-text-heading hover:underline">
                  {comment.character_name || 'Unknown'}
                </Link>
                {!screenshotModeEnabled && (
                  <span className="text-sm text-content-tertiary">
                    @{comment.author_username}
                  </span>
                )}
                <span className="text-sm text-content-tertiary">{timeAgo}</span>
                {isEdited && (
                  <Badge variant="secondary" size="sm">
                    Edited
                  </Badge>
                )}
              </div>
            </div>
          </div>

          {comment.is_deleted ? (
            <p className="text-content-tertiary italic">[deleted]</p>
          ) : isEditing ? (
            <div className="mb-3">
              <CommentEditor
                value={editContent}
                onChange={setEditContent}
                placeholder="Edit comment..."
                disabled={updateCommentMutation.isPending}
                characters={allGameCharacters}
                maxLength={10000}
                showCharacterCount
              />
              <div className="flex gap-2 mt-2">
                <Button
                  variant="primary"
                  size="sm"
                  onClick={handleSaveEdit}
                  disabled={updateCommentMutation.isPending || !editContent.trim()}
                >
                  {updateCommentMutation.isPending ? 'Saving...' : 'Save'}
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={handleCancelEdit}
                  disabled={updateCommentMutation.isPending}
                >
                  Cancel
                </Button>
              </div>
            </div>
          ) : (
            <MarkdownPreview content={comment.content} fullWidth />
          )}
        </div>

        {/* Action buttons */}
        {!isEditing && (
          <div className="mt-2 flex items-center flex-wrap gap-1 text-xs text-content-secondary">
            {canReply && (
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setIsReplying(!isReplying)}
                className="h-auto text-xs"
                aria-label="Reply to this comment"
              >
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 10h10a8 8 0 018 8v2M3 10l6 6m-6-6l6-6" />
                </svg>
                <span className="hidden md:inline">Reply</span>
              </Button>
            )}

            <Button
              variant="ghost"
              size="sm"
              onClick={handleCopyLink}
              className="h-auto text-xs"
              title="Copy link to this comment"
              aria-label="Copy link to this comment"
            >
              {linkCopied ? (
                <>
                  <svg className="w-4 h-4 text-semantic-success" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                  </svg>
                  <span className="text-semantic-success hidden md:inline">Copied!</span>
                </>
              ) : (
                <>
                  <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" />
                  </svg>
                  <span className="hidden md:inline">Link</span>
                </>
              )}
            </Button>

            {showReadButton && (
              <Button
                variant="ghost"
                size="sm"
                onClick={() => onToggleRead(isRead)}
                className="h-auto text-xs"
                aria-label={isRead ? 'Mark as unread' : 'Mark as read'}
                data-testid="toggle-read-button"
              >
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  {isRead ? (
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21" />
                  ) : (
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
                  )}
                </svg>
                <span className="hidden md:inline">{isRead ? 'Unread' : 'Read'}</span>
              </Button>
            )}

            {canEdit && (
              <Button
                variant="ghost"
                size="sm"
                onClick={handleEdit}
                className="h-auto text-xs"
                title="Edit this comment"
                aria-label="Edit this comment"
              >
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                </svg>
                <span className="hidden md:inline">Edit</span>
              </Button>
            )}

            {canDelete && (
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setShowDeleteConfirm(true)}
                disabled={isDeleting}
                className="h-auto text-xs text-semantic-danger hover:text-semantic-danger"
                title="Delete this comment"
                aria-label="Delete this comment"
              >
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                </svg>
                <span className="hidden md:inline">{isDeleting ? 'Deleting...' : 'Delete'}</span>
              </Button>
            )}

            {onNavigateToComment && !comment.is_deleted && (
              <a
                href={`/games/${gameId}?tab=common-room&comment=${comment.id}`}
                onClick={(e) => {
                  e.preventDefault();
                  onNavigateToComment();
                }}
                className="flex items-center gap-1 px-2 py-1 text-xs text-interactive-primary hover:text-accent-secondary font-medium"
              >
                <span className="hidden md:inline">View in thread →</span>
                <svg className="w-4 h-4 md:hidden" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                </svg>
              </a>
            )}
          </div>
        )}

        {/* Reply posted confirmation */}
        {replyPostedId && (
          <div className="mt-3 p-3 bg-bg-secondary rounded-lg border border-semantic-success flex items-center justify-between gap-3">
            <div className="flex items-center gap-2 text-sm text-semantic-success">
              <svg className="w-4 h-4 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
              </svg>
              Reply posted!
            </div>
            <a
              href={`/games/${gameId}?tab=common-room&comment=${replyPostedId}`}
              onClick={(e) => { e.preventDefault(); onNavigateToComment?.(); }}
              className="text-sm text-interactive-primary hover:text-accent-secondary font-medium shrink-0"
            >
              View in thread →
            </a>
          </div>
        )}

        {/* Reply form */}
        {isReplying && (
          <div className="mt-3 p-3 bg-bg-secondary rounded-lg border border-border-primary">
            <form onSubmit={handleSubmitReply}>
              {userCharacters.length > 1 && (
                <Select
                  value={selectedCharacterId || ''}
                  onChange={(e) => setSelectedCharacterId(Number(e.target.value))}
                  className="mb-2"
                  disabled={isSubmittingReply}
                >
                  {userCharacters.map((char) => (
                    <option key={char.id} value={char.id}>
                      Reply as {char.name}
                    </option>
                  ))}
                </Select>
              )}
              <CommentEditor
                value={replyContent}
                onChange={setReplyContent}
                placeholder="Write a reply..."
                disabled={isSubmittingReply}
                characters={allGameCharacters}
                maxLength={10000}
                showCharacterCount
              />
              <div className="flex gap-2 mt-2">
                <Button
                  type="submit"
                  variant="primary"
                  size="sm"
                  disabled={isSubmittingReply || !replyContent.trim()}
                >
                  {isSubmittingReply ? 'Posting...' : 'Reply'}
                </Button>
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  onClick={() => { setIsReplying(false); setReplyContent(''); }}
                  disabled={isSubmittingReply}
                >
                  Cancel
                </Button>
              </div>
            </form>
          </div>
        )}
      </CardBody>

      <ConfirmModal
        isOpen={showDeleteConfirm}
        onClose={() => setShowDeleteConfirm(false)}
        onConfirm={handleConfirmDelete}
        title="Delete Comment"
        message="Are you sure you want to delete this comment? This action cannot be undone."
        confirmText="Delete"
        variant="danger"
        isLoading={isDeleting}
      />
    </Card>
  );
}
