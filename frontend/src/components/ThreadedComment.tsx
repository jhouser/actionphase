import { useState, useEffect, useRef, useCallback, memo, useMemo } from 'react';
import { Link } from 'react-router-dom';
import { formatDistanceToNow } from 'date-fns';
import { apiClient } from '../lib/api';
import { useToast } from '../contexts/ToastContext';
import type { Message } from '../types/messages';
import type { Character } from '../types/characters';
import { MarkdownPreview } from './MarkdownPreview';
import { CommentEditor } from './CommentEditor';
import CharacterAvatar from './CharacterAvatar';
import { Button, Select } from './ui';
import { useAdminMode } from '../hooks/useAdminMode';
import { useScreenshotMode } from '../hooks/useScreenshotMode';
import { useUpdateComment, useDeleteComment } from '../hooks/useCommentMutations';
import { useGamePermissions } from '../hooks/useGamePermissions';
import { useOptionalGameContext } from '../contexts/GameContext';
import { ConfirmModal } from './ConfirmModal';
import { logger } from '@/services/LoggingService';
import type { CommentTreeNode } from '../lib/utils/commentTree';
import { COMMENT_MAX_DEPTH_MOBILE } from '@/config/comments';

interface ThreadedCommentProps {
  comment: Message | CommentTreeNode; // Supports both individual messages and pre-loaded tree nodes
  gameId: number;
  postId: number; // The root post ID (required for API calls)
  characters: Character[]; // All game characters (for autocomplete)
  controllableCharacters: Character[]; // Characters the user can control (for "Reply as" dropdown)
  onCreateReply: (parentId: number, characterId: number, content: string, rootPostId: number) => Promise<void>;
  onCommentDeleted?: () => void; // Callback when a comment is deleted (to trigger parent reload)
  currentUserId?: number;
  depth?: number;
  maxDepth?: number; // Maximum nesting depth before showing "Continue thread" button
  unreadCommentIDs?: number[]; // IDs of comments that are "new since last visit" (auto mode)
  manualReadCommentIDs?: number[]; // IDs of comments explicitly marked as read (manual mode)
  commentReadMode?: 'auto' | 'manual'; // Which read tracking mode is active
  onToggleRead?: (commentId: number, currentlyRead: boolean) => void; // Callback to toggle manual read state
  onOpenThread?: (comment: Message) => void; // Callback to open thread modal with comment object
  readOnly?: boolean; // Disable all interactive features (for history view)
  allowReadTracking?: boolean; // Show faded read state and toggle button (default true)
  parentComment?: Message | CommentTreeNode | null; // Parent comment for smart character defaulting in nested replies
  variant?: 'desktop' | 'mobile'; // Used to create unique IDs for desktop vs mobile rendering
  onDirtyStateChange?: (commentId: number, isDirty: boolean) => void; // Fired when pending reply content goes from empty↔non-empty
}

export const ThreadedComment = memo(function ThreadedComment({
  comment: initialComment,
  gameId,
  postId,
  characters,
  controllableCharacters,
  onCreateReply,
  onCommentDeleted,
  currentUserId,
  depth = 0,
  maxDepth = 5,
  unreadCommentIDs = [],
  manualReadCommentIDs = [],
  commentReadMode = 'auto',
  onToggleRead,
  onOpenThread,
  readOnly = false,
  allowReadTracking = true,
  parentComment = null,
  variant,
  onDirtyStateChange,
}: ThreadedCommentProps) {
  const { showSuccess, showError } = useToast();
  // Use local state to track the current comment data (for immediate UI updates)
  // Initialize reply_count from preloaded children if not set
  const initializeComment = (c: Message | CommentTreeNode): Message | CommentTreeNode => {
    const hasPreloaded = 'children' in c && Array.isArray(c.children);
    if (hasPreloaded && (!c.reply_count || c.reply_count === 0)) {
      return { ...c, reply_count: (c as CommentTreeNode).children.length };
    }
    return c;
  };

  const [comment, setComment] = useState<Message | CommentTreeNode>(initializeComment(initialComment));
  const [replies, setReplies] = useState<Message[]>([]);
  const [loadingReplies, setLoadingReplies] = useState(false);
  const [showReplies, setShowReplies] = useState(true); // Start expanded
  const [isReplying, setIsReplying] = useState(false);
  const [replyContent, setReplyContent] = useState('');
  const [selectedCharacterId, setSelectedCharacterId] = useState<number | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const [isEditing, setIsEditing] = useState(false);
  const [editContent, setEditContent] = useState(comment.content);
  const [selectedEditCharacterId, setSelectedEditCharacterId] = useState<number | null>(null);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  const isMountedRef = useRef(true);
  const hasLoadedRef = useRef(false);

  // Notify parent (e.g. ThreadViewModal) when pending reply content transitions from empty↔non-empty.
  // Cleanup on unmount always signals false so the parent Set doesn't retain a stale dirty entry.
  const isReplyDirty = replyContent.trim().length > 0;
  useEffect(() => {
    if (isReplyDirty) onDirtyStateChange?.(comment.id, true);
    return () => onDirtyStateChange?.(comment.id, false);
  }, [isReplyDirty, onDirtyStateChange, comment.id]);

  // Check if this comment has pre-loaded children (from tree structure)
  const hasPreloadedChildren = 'children' in comment && Array.isArray(comment.children) && comment.children.length > 0;
  const preloadedChildren = hasPreloadedChildren ? (comment as CommentTreeNode).children : [];

  const gameContext = useOptionalGameContext();
  const portraitAvatars = gameContext?.game?.portrait_avatars ?? false;
  const { adminModeEnabled } = useAdminMode();
  const { screenshotModeEnabled } = useScreenshotMode();
  const { isGM } = useGamePermissions(gameId);
  const updateCommentMutation = useUpdateComment();
  const deleteCommentMutation = useDeleteComment();
  const isAuthor = currentUserId === comment.author_id;
  // Single source of truth: reply_count (initialized from preloaded children, updated on mutations)
  const hasReplies = (comment.reply_count || 0) > 0;

  // Stable reference required so MarkdownPreview's React.memo boundary isn't busted on
  // every ThreadedComment re-render (e.g. toggling replies, editing). Without this, the
  // dangerouslySetInnerHTML div re-renders and Chrome re-fires mouseover, causing the
  // mention tooltip to get stuck open.
  const mentionedCharacters = useMemo(
    () => comment.mentioned_character_ids?.flatMap(id => {
      const char = characters.find(c => c.id === id);
      if (!char) return [];
      return [{ id: char.id, name: char.name, username: char.username,
                character_type: char.character_type, avatar_url: char.avatar_url ?? undefined }];
    }) ?? [],
    [comment.mentioned_character_ids, characters]
  );
  const isManuallyRead = commentReadMode === 'manual' && manualReadCommentIDs.includes(comment.id);
  const isUnread = commentReadMode !== 'manual' && unreadCommentIDs.includes(comment.id);

  // Update local comment state when prop changes (from cache invalidation)
  useEffect(() => {
    setComment(initializeComment(initialComment));
  }, [initialComment]);
  // On mobile, show "Continue thread" button earlier to save space
  // On desktop, use the normal maxDepth (depth 5)
  // Use COMMENT_MAX_DEPTH_MOBILE from env var (VITE_COMMENT_MAX_DEPTH_MOBILE)
  const mobileMaxDepth = COMMENT_MAX_DEPTH_MOBILE;
  // True max depth - where we stop rendering entirely
  const isAtMaxDepth = depth >= maxDepth;
  const isAtMobileMaxDepth = depth >= mobileMaxDepth;
  // Show "Continue thread" button only when at exactly maxDepth - 1 (the last visible level)
  // At this depth, we stop rendering children and show the button instead
  const shouldShowContinueButton = depth === maxDepth - 1;
  const shouldShowMobileContinueButton = depth === mobileMaxDepth - 1;
  const [linkCopied, setLinkCopied] = useState(false);

  // Track component mount status
  useEffect(() => {
    isMountedRef.current = true; // Explicitly set to true on mount
    return () => {
      isMountedRef.current = false;
      hasLoadedRef.current = false; // Reset so component can reload if remounted
    };
  }, []);

  // Auto-select character - prefer grandparent comment's character for nested replies
  // This creates a natural conversation flow when GMs reply as NPCs in threaded conversations
  useEffect(() => {
    if (controllableCharacters.length > 0 && selectedCharacterId === null) {
      // For nested replies (when parentComment exists), use the parent's character
      // This maintains conversation continuity: if replying to a reply, continue as the original NPC
      const targetCharacterId = parentComment?.character_id || comment.character_id;
      const targetCharacter = controllableCharacters.find(c => c.id === targetCharacterId);

      if (targetCharacter) {
        // We control the target character - use it as default
        setSelectedCharacterId(targetCharacter.id);
      } else {
        // We don't control the target character - use first available
        setSelectedCharacterId(controllableCharacters[0].id);
      }
    }
  }, [controllableCharacters, selectedCharacterId, parentComment, comment.character_id]);

  // Define loadReplies with useCallback to satisfy exhaustive-deps
  const loadReplies = useCallback(async () => {
    // Skip if we have pre-loaded children (from tree structure)
    if (!isMountedRef.current || hasLoadedRef.current || hasPreloadedChildren) return;

    try {
      setLoadingReplies(true);
      const response = await apiClient.messages.getPostComments(gameId, comment.id);
      if (isMountedRef.current) {
        setReplies(response.data.filter(r => !r.is_deleted || (r.reply_count ?? 0) > 0));
        hasLoadedRef.current = true; // Only mark as loaded after successful state update
      }
    } catch (_err) {
      logger.error('Failed to load replies', { error: _err, commentId: comment.id, gameId, postId });
    } finally {
      if (isMountedRef.current) {
        setLoadingReplies(false);
      }
    }
  }, [gameId, comment.id, postId, hasPreloadedChildren]);

  // Load replies immediately when component mounts if there are replies (and no pre-loaded children)
  // Skip when shouldShowContinueButton is true: children at this depth are never rendered,
  // so fetching them would be wasted API calls (common in history view with deep threads).
  useEffect(() => {
    if (hasReplies && !hasLoadedRef.current && !hasPreloadedChildren && !shouldShowContinueButton) {
      loadReplies();
    }
  }, [hasReplies, hasPreloadedChildren, loadReplies, shouldShowContinueButton]);

  const handleCopyLink = async () => {
    const phaseId = 'phase_id' in comment ? comment.phase_id : undefined;
    const url = readOnly && phaseId
      ? `${window.location.origin}/games/${gameId}?tab=history&phase=${phaseId}&comment=${comment.id}`
      : `${window.location.origin}/games/${gameId}?tab=common-room&comment=${comment.id}`;

    try {
      // Check if Clipboard API is available (requires HTTPS or localhost)
      if (navigator.clipboard && window.isSecureContext) {
        await navigator.clipboard.writeText(url);
      } else {
        // Fallback for non-secure contexts (HTTP)
        const textArea = document.createElement('textarea');
        textArea.value = url;
        textArea.style.position = 'fixed';
        textArea.style.left = '-999999px';
        textArea.style.top = '-999999px';
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
        // Reset after 2 seconds
        setTimeout(() => {
          if (isMountedRef.current) {
            setLinkCopied(false);
          }
        }, 2000);
      }
    } catch (_err) {
      logger.error('Failed to copy link', { error: _err, commentId: comment.id });
      // Fallback: show toast with link if both methods fail
      if (isMountedRef.current) {
        showError(`Failed to copy. Link: ${url}`);
      }
    }
  };

  const handleEdit = () => {
    setEditContent(comment.content);
    setSelectedEditCharacterId(comment.character_id);
    setIsEditing(true);
  };

  const handleCancelEdit = () => {
    setEditContent(comment.content);
    setIsEditing(false);
  };

  const handleSaveEdit = async () => {
    if (!editContent.trim() || (editContent === comment.content && selectedEditCharacterId === comment.character_id)) {
      setIsEditing(false);
      return;
    }

    try {
      const updatedComment = await updateCommentMutation.mutateAsync({
        gameId,
        postId, // Use the root post ID passed as prop
        commentId: comment.id,
        data: {
          content: editContent.trim(),
          ...(selectedEditCharacterId !== comment.character_id && {
            character_id: selectedEditCharacterId ?? undefined
          })
        }
      });
      // Update local state immediately with the response from the server
      setComment(updatedComment);
      setIsEditing(false);
    } catch (_err) {
      logger.error('Failed to update comment', { error: _err, commentId: comment.id, gameId, postId });
      showError('Failed to update comment. Please try again.');
    }
  };

  const handleDeleteClick = () => {
    setShowDeleteConfirm(true);
  };

  const handleConfirmDelete = async () => {
    try {
      setIsDeleting(true);
      await deleteCommentMutation.mutateAsync({
        gameId,
        postId, // Use the root post ID passed as prop
        commentId: comment.id
      });
      // Success - the mutation will invalidate queries and trigger refetch
      setShowDeleteConfirm(false);
      // Notify parent to reload its replies
      onCommentDeleted?.();
    } catch (_err) {
      logger.error('Failed to delete comment', { error: _err, commentId: comment.id, gameId, postId });
      showError('Failed to delete comment. Please try again.');
    } finally {
      setIsDeleting(false);
    }
  };

  // Handler for when a nested comment is deleted - reload this comment's replies
  const handleNestedCommentDeleted = useCallback(() => {
    // Reset the loaded flag and reload replies
    hasLoadedRef.current = false;
    loadReplies();
  }, [loadReplies]);


  const handleSubmitReply = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedCharacterId || !replyContent.trim()) return;

    // Find the selected character for optimistic rendering
    const selectedCharacter = controllableCharacters.find(c => c.id === selectedCharacterId);
    if (!selectedCharacter) return;

    // Create optimistic reply
    const optimisticReply: Message = {
      id: Date.now(), // Temporary ID
      parent_id: comment.id,
      game_id: gameId,
      thread_depth: depth + 1,
      character_id: selectedCharacterId,
      author_id: currentUserId || 0,
      author_username: selectedCharacter.username || '',
      character_name: selectedCharacter.name,
      character_avatar_url: selectedCharacter.avatar_url || undefined,
      content: replyContent.trim(),
      message_type: 'comment',
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
      reply_count: 0,
      is_deleted: false,
      is_edited: false,
      is_draft: false,
    };

    try {
      setIsSubmitting(true);

      // Only add optimistic reply if children won't exceed max depth
      // If at depth maxDepth-1, children would be at maxDepth and won't render
      if (!shouldShowContinueButton) {
        // Add optimistic reply immediately
        setReplies(prev => [...prev, optimisticReply]);
        hasLoadedRef.current = true; // Mark as loaded so we use replies state instead of preloaded children
        setShowReplies(true);
      }
      // Always increment reply_count immediately for button visibility
      setComment(prev => ({ ...prev, reply_count: (prev.reply_count || 0) + 1 }));
      setReplyContent('');
      setIsReplying(false);

      logger.debug('Creating reply to comment', { commentId: comment.id, gameId, postId, characterId: selectedCharacterId });
      await onCreateReply(comment.id, selectedCharacterId, optimisticReply.content, postId);
      logger.debug('Reply created successfully', { commentId: comment.id, gameId, postId });

      // Show success toast
      showSuccess('Reply posted successfully');

      // Reload replies to get the real data (with proper ID, timestamps, etc.)
      // But skip reloading if at max depth (children won't render anyway)
      if (!shouldShowContinueButton) {
        logger.debug('Loading replies after reply creation', { commentId: comment.id, gameId, postId });

        // Force reload by fetching fresh data
        try {
          setLoadingReplies(true);
          const response = await apiClient.messages.getPostComments(gameId, comment.id);
          if (isMountedRef.current) {
            const visibleReplies = response.data.filter(r => !r.is_deleted || (r.reply_count ?? 0) > 0);
            setReplies(visibleReplies);
            // Update reply_count to match visible (non-deleted-leaf) count
            setComment(prev => ({ ...prev, reply_count: visibleReplies.length }));
            hasLoadedRef.current = true;
          }
        } catch (loadErr) {
          logger.error('Failed to load replies after creation', { error: loadErr, commentId: comment.id });
          // Keep optimistic reply on load error
        } finally {
          if (isMountedRef.current) {
            setLoadingReplies(false);
          }
        }
      }

      logger.debug('Replies loaded after reply creation', { commentId: comment.id, gameId, postId });
    } catch (_err) {
      logger.error('Failed to submit reply', { error: _err, commentId: comment.id, gameId, postId });
      // Remove optimistic reply on error
      setReplies(prev => prev.filter(r => r.id !== optimisticReply.id));
      // Decrement reply_count to rollback optimistic increment
      setComment(prev => ({ ...prev, reply_count: Math.max(0, (prev.reply_count || 0) - 1) }));
      hasLoadedRef.current = false; // Reset so we fall back to preloaded children if available
      showError('Failed to post reply. Please try again.');
      // Restore reply form state so user can retry
      setReplyContent(optimisticReply.content);
      setIsReplying(true);
    } finally {
      setIsSubmitting(false);
    }
  };

  const formatDate = (dateString: string | null | undefined) => {
    if (!dateString) return '';
    // Backend returns UTC timestamps without 'Z' suffix
    // Append 'Z' to ensure proper UTC parsing
    const utcDateString = dateString.endsWith('Z') ? dateString : `${dateString}Z`;
    const date = new Date(utcDateString);

    // Check if date is valid
    if (isNaN(date.getTime())) {
      return '';
    }

    return formatDistanceToNow(date, {
      addSuffix: true,
    });
  };

  // Per-depth left-rail accent so each nesting level is visually distinct in every
  // theme. Reuses avatar hues (theme-defined). depth 0 has no rail.
  const threadAccents = [
    'border-l-avatar-6', // depth 1 - blue
    'border-l-avatar-4', // depth 2 - green
    'border-l-avatar-7', // depth 3 - violet
    'border-l-avatar-2', // depth 4 - orange
  ];
  const borderColor = depth > 0 ? threadAccents[(depth - 1) % threadAccents.length] : '';

  // Alternating background colors for better visual separation between comment levels
  const backgroundColors = [
    'surface-raised', // depth 0 - raised surface
    'surface-base', // depth 1 - base surface
    'surface-raised', // depth 2 - raised surface
    'surface-base', // depth 3 - base surface
    'surface-raised', // depth 4 - raised surface
    'surface-base', // depth 5 - base surface
  ];
  const bgColor = backgroundColors[depth % backgroundColors.length];

  // Mobile-friendly indentation: minimal padding for maximum content width
  // Desktop keeps generous padding for better visual hierarchy
  // Max indentation prevents overly narrow content areas
  const getIndentPadding = () => {
    if (depth === 0) return 'px-1 md:px-3'; // Minimal 4px mobile padding, normal desktop
    if (depth === 1) return 'pl-1 pr-1 md:pl-6 md:pr-3'; // 4px mobile indent, 24px desktop
    if (depth === 2) return 'pl-1 pr-1 md:pl-6 md:pr-3'; // 4px mobile indent, 24px desktop
    if (depth === 3) return 'pl-1 pr-1 md:pl-6 md:pr-3'; // 4px mobile indent, 24px desktop
    // Depth 4+ (only visible in thread modal): cap at same level to prevent excessive narrowing
    return 'pl-1 pr-1 md:pl-6 md:pr-3';
  };

  return (
    <div
      id={`comment-${comment.id}${variant ? `-${variant}` : ''}`}
      data-testid="threaded-comment"
      className={`${getIndentPadding()} ${depth > 0 ? 'border-l-2 ' + borderColor : 'border-t border-theme-strong'} ${bgColor} ${depth > 0 ? 'py-3 my-2' : 'py-2'} border-b border-theme-subtle`}
    >
      {/* Comment Header and Content */}
      <div className={`${portraitAvatars ? 'overflow-hidden' : ''}${isUnread ? ' border border-semantic-warning rounded-lg p-3' : ''}${isManuallyRead ? ' opacity-50' : ''}`}>
        {portraitAvatars && (
          <div className="float-left mr-2 mb-1">
            <CharacterAvatar
              avatarUrl={comment.character_avatar_url}
              characterName={comment.character_name}
              size="md"
              shape="portrait"
            />
          </div>
        )}
        <div className={portraitAvatars ? '' : 'flex items-center gap-1.5 md:gap-2 mb-1'}>
          {!portraitAvatars && (
            <CharacterAvatar
              avatarUrl={comment.character_avatar_url}
              characterName={comment.character_name}
              size="md"
              className="md:w-12 md:h-12"
            />
          )}
          <div className="flex-1 min-w-0">
            {/* Desktop: horizontal layout */}
            <div className="hidden md:block">
              <Link to={`/characters/${comment.character_id}`} className="font-semibold text-sm text-content-primary hover:underline" data-testid="comment-author">{comment.character_name}</Link>
              <span className="text-xs text-content-secondary ml-2">
                {comment.author_username && !screenshotModeEnabled ? <><Link to={`/users/${comment.author_username}`} className="hover:underline">@{comment.author_username}</Link>{' · '}</> : ''}{formatDate(comment.created_at)}
                {comment.is_edited && !comment.is_deleted && (
                  <span className="ml-1 text-content-tertiary" title={comment.edited_at ? `Last edited ${formatDate(comment.edited_at)}` : undefined}>
                    (edited{comment.edit_count && comment.edit_count > 1 ? ` ${comment.edit_count}x` : ''})
                  </span>
                )}
              </span>
              {isAuthor && !screenshotModeEnabled && (
                <span className="ml-2 text-xs bg-interactive-primary-subtle text-interactive-primary px-1.5 py-0.5 rounded">You</span>
              )}
              {isUnread && (
                <span className="ml-2 text-xs bg-semantic-warning-subtle text-content-primary px-2 py-0.5 rounded font-semibold">NEW</span>
              )}
            </div>
            {/* Mobile: compact layout */}
            <div className="md:hidden">
              <div className="flex items-center gap-1 flex-wrap text-xs">
                <Link to={`/characters/${comment.character_id}`} className="font-semibold text-content-primary hover:underline" data-testid="comment-author">{comment.character_name}</Link>
                {isAuthor && !screenshotModeEnabled && (
                  <span className="bg-interactive-primary-subtle text-content-primary px-1 py-0.5 rounded">You</span>
                )}
                {isUnread && (
                  <span className="bg-semantic-warning-subtle text-content-primary px-1.5 py-0.5 rounded font-semibold">NEW</span>
                )}
              </div>
              <div className="text-xs text-content-secondary">
                {comment.author_username && !screenshotModeEnabled && <><Link to={`/users/${comment.author_username}`} className="hover:underline">@{comment.author_username}</Link>{' · '}</>}
                <span className="text-content-tertiary">{formatDate(comment.created_at)}</span>
                {comment.is_edited && !comment.is_deleted && (
                  <span className="ml-1 text-content-tertiary" title={comment.edited_at ? `Last edited ${formatDate(comment.edited_at)}` : undefined}>
                    (edited{comment.edit_count && comment.edit_count > 1 ? ` ${comment.edit_count}x` : ''})
                  </span>
                )}
              </div>
            </div>
          </div>
        </div>

        {/* Comment Content - Edit Mode, Deleted State, or Display Mode */}
        {comment.is_deleted ? (
          // Deleted comment - show placeholder to preserve thread structure
          <div className="text-sm text-content-tertiary italic mb-2 py-1">
            <span className="opacity-60">[Comment deleted]</span>
            {comment.deleted_at && (
              <span className="ml-2 text-xs opacity-50">
                {formatDate(comment.deleted_at)}
              </span>
            )}
          </div>
        ) : isEditing ? (
          <div className="mb-3">
            {controllableCharacters.length > 1 && (
              <Select
                value={selectedEditCharacterId || ''}
                onChange={(e) => setSelectedEditCharacterId(Number(e.target.value))}
                className="mb-2"
                disabled={updateCommentMutation.isPending}
              >
                {controllableCharacters.map((char) => (
                  <option key={char.id} value={char.id}>
                    Edit as {char.name}
                  </option>
                ))}
              </Select>
            )}
            <CommentEditor
              value={editContent}
              onChange={setEditContent}
              placeholder="Edit comment..."
              disabled={updateCommentMutation.isPending}
              characters={characters}
              maxLength={10000}
              showCharacterCount={true}
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
          <div className="mb-2 pl-1 md:pl-0 text-sm md:text-base">
            <MarkdownPreview
              content={comment.content}
              mentionedCharacters={mentionedCharacters}
              fullWidth
            />
          </div>
        )}

        {/* Action Buttons */}
        <div className="flex items-center flex-wrap gap-1 md:gap-3 text-xs text-content-secondary">
          {!isAtMaxDepth && !isEditing && !readOnly && !comment.is_deleted && controllableCharacters.length > 0 && (
            <Button
              variant="ghost"
              onClick={() => setIsReplying(!isReplying)}
              className="p-2 md:p-0 min-h-[44px] md:min-h-0 h-auto text-xs"
              aria-label="Reply to this comment"
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 10h10a8 8 0 018 8v2M3 10l6 6m-6-6l6-6" />
              </svg>
              <span className="hidden md:inline">Reply</span>
            </Button>
          )}

          {/* Show collapse/expand button only if NOT at mobile/desktop max depth */}
          {hasReplies && (
            <>
              {/* Mobile: hide if at mobile max depth */}
              <div className="md:hidden">
                {!isAtMobileMaxDepth && (
                  <Button
                    variant="ghost"
                    onClick={() => setShowReplies(!showReplies)}
                    className="p-2 min-h-[44px] h-auto text-xs"
                  >
                    <span>{showReplies ? '▼' : '▶'}</span>
                    <span>{comment.reply_count}</span>
                  </Button>
                )}
              </div>
              {/* Desktop: hide if at desktop max depth */}
              <div className="hidden md:block">
                {!isAtMaxDepth && (
                  <Button
                    variant="ghost"
                    onClick={() => setShowReplies(!showReplies)}
                    className="p-0 h-auto text-xs"
                  >
                    <span>{showReplies ? '▼' : '▶'}</span>
                    <span>{comment.reply_count} {comment.reply_count === 1 ? 'reply' : 'replies'}</span>
                  </Button>
                )}
              </div>
            </>
          )}

          <Button
            variant="ghost"
            onClick={handleCopyLink}
            className="p-2 md:p-0 min-h-[44px] md:min-h-0 h-auto text-xs"
            title="Copy link to this comment"
            aria-label="Copy link to this comment"
          >
            {linkCopied ? (
              <>
                <svg className="w-4 h-4 text-semantic-success" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                </svg>
                <span className="text-semantic-success">Copied!</span>
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

          {commentReadMode === 'manual' && allowReadTracking && !comment.is_deleted && (
            <Button
              variant="ghost"
              onClick={() => onToggleRead?.(comment.id, isManuallyRead)}
              className="p-2 md:p-0 min-h-[44px] md:min-h-0 h-auto text-xs"
              title={isManuallyRead ? 'Mark as unread' : 'Mark as read'}
              aria-label={isManuallyRead ? 'Mark as unread' : 'Mark as read'}
              data-testid="toggle-read-button"
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                {isManuallyRead ? (
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21" />
                ) : (
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
                )}
              </svg>
              <span className="hidden md:inline">{isManuallyRead ? 'Unread' : 'Read'}</span>
            </Button>
          )}

          {/* Screenshot Mode hides Edit/Delete too: they only render for the
              author (or the GM/admin), so a visible control gives away who's
              behind the character just as plainly as the username would. */}
          {isAuthor && !isEditing && !comment.is_deleted && !readOnly && !screenshotModeEnabled && (
            <Button
              variant="ghost"
              onClick={handleEdit}
              className="p-2 md:p-0 min-h-[44px] md:min-h-0 h-auto text-xs"
              title="Edit this comment"
              aria-label="Edit this comment"
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
              </svg>
              <span className="hidden md:inline">Edit</span>
            </Button>
          )}

          {(isAuthor || isGM || adminModeEnabled) && !isEditing && !comment.is_deleted && !readOnly && !screenshotModeEnabled && (
            <Button
              variant="ghost"
              onClick={handleDeleteClick}
              disabled={isDeleting}
              className="p-2 md:p-0 min-h-[44px] md:min-h-0 h-auto text-xs text-semantic-danger hover:text-semantic-danger"
              title={isAuthor ? "Delete this comment" : (isGM ? "Delete this comment (GM)" : "Delete this comment (admin)")}
              aria-label={isAuthor ? "Delete this comment" : (isGM ? "Delete this comment (GM)" : "Delete this comment (admin)")}
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
              </svg>
              <span className="hidden md:inline">{isDeleting ? 'Deleting...' : 'Delete'}</span>
            </Button>
          )}

          {comment.parent_id && (
            <a
              href={
                comment.thread_depth === 1
                  ? `/games/${gameId}?tab=common-room&postId=${comment.parent_id}`
                  : `/games/${gameId}?tab=common-room&comment=${comment.parent_id}`
              }
              className="flex items-center gap-1 p-2 md:p-0 min-h-[44px] md:min-h-0 text-xs text-content-secondary hover:text-interactive-primary-hover transition-colors"
              title={comment.thread_depth === 1 ? "Go to parent post" : "Go to parent comment"}
              aria-label={comment.thread_depth === 1 ? "Go to parent post" : "Go to parent comment"}
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 19l-7-7m0 0l7-7m-7 7h18" />
              </svg>
              <span className="hidden md:inline">Parent</span>
            </a>
          )}
        </div>
      </div>

      {/* Reply Form */}
      {isReplying && !readOnly && (
        <div className="mb-3 surface-raised rounded p-3 border border-theme-default">
          <form onSubmit={handleSubmitReply}>
            {controllableCharacters.length > 0 ? (
              <>
                {controllableCharacters.length > 1 && (
                  <Select
                    value={selectedCharacterId || ''}
                    onChange={(e) => setSelectedCharacterId(Number(e.target.value))}
                    className="mb-2"
                    disabled={isSubmitting}
                  >
                    {controllableCharacters.map((char) => (
                      <option key={char.id} value={char.id}>
                        Reply as {char.name}
                      </option>
                    ))}
                  </Select>
                )}

                <div className="mb-2">
                  <CommentEditor
                    value={replyContent}
                    onChange={setReplyContent}
                    placeholder="Write a reply..."
                    disabled={isSubmitting}
                    characters={characters}
                    maxLength={10000}
                    warnOnUnsavedChanges
                    showCharacterCount={true}
                  />
                </div>

                <div className="flex gap-2">
                  <Button
                    type="submit"
                    variant="primary"
                    size="sm"
                    disabled={isSubmitting || !replyContent.trim()}
                    data-faro-user-action-name="submit-comment"
                  >
                    {isSubmitting ? 'Posting...' : 'Reply'}
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    onClick={() => {
                      setIsReplying(false);
                      setReplyContent('');
                    }}
                    disabled={isSubmitting}
                  >
                    Cancel
                  </Button>
                </div>
              </>
            ) : (
              <p className="text-xs text-content-secondary">You need a character to reply.</p>
            )}
          </form>
        </div>
      )}

      {/* Continue Thread Button (if at max depth with replies) */}
      {/* Show on mobile at depth 3+, on desktop at depth 5+ */}
      {hasReplies && (
        <>
          {/* Mobile: Show at depth 3+ */}
          <div className="mt-2 ml-2 md:hidden">
            {shouldShowMobileContinueButton && (
              <Button
                variant="ghost"
                size="sm"
                onClick={() => onOpenThread?.(comment)}
                className="inline-flex items-center gap-1 text-sm font-medium text-interactive-primary hover:text-interactive-primary-hover h-auto p-0"
              >
                <span>Continue this thread</span>
                <span className="text-content-secondary">({comment.reply_count} {comment.reply_count === 1 ? 'reply' : 'replies'})</span>
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                </svg>
              </Button>
            )}
          </div>
          {/* Desktop: Show at depth 5+ */}
          <div className="hidden md:block mt-2 ml-6">
            {shouldShowContinueButton && (
              <Button
                variant="ghost"
                size="sm"
                onClick={() => onOpenThread?.(comment)}
                className="inline-flex items-center gap-1 text-sm font-medium text-interactive-primary hover:text-interactive-primary-hover h-auto p-0"
              >
                <span>Continue this thread</span>
                <span className="text-content-secondary">({comment.reply_count} {comment.reply_count === 1 ? 'reply' : 'replies'})</span>
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                </svg>
              </Button>
            )}
          </div>
        </>
      )}

      {/* Nested Replies */}
      {showReplies && (hasReplies || replies.length > 0 || preloadedChildren.length > 0) && (
          <>
            {/* Desktop: Show if not at or beyond max depth - 1 */}
            {/* Only render desktop section if we're not locked to mobile variant */}
            {variant !== 'mobile' && (
              <div className="hidden md:block">
                {depth < maxDepth - 1 && (
                  <div className="space-y-1">
                    {loadingReplies ? (
                        <div className="ml-2 md:ml-6 py-2 text-xs text-content-secondary">
                          Loading replies...
                        </div>
                    ) : (
                        // Use dynamically loaded replies if we have them (hasLoadedRef = true means we've loaded/reloaded)
                        // Otherwise fall back to pre-loaded children
                        // This allows optimistic updates and fresh data to override preloaded children
                        (hasLoadedRef.current ? replies : (hasPreloadedChildren ? preloadedChildren : replies)).map((reply) => (
                            <ThreadedComment
                                key={`${reply.id}-desktop`}
                                comment={reply}
                                gameId={gameId}
                                postId={postId}
                                characters={characters}
                                controllableCharacters={controllableCharacters}
                                onCreateReply={onCreateReply}
                                onCommentDeleted={handleNestedCommentDeleted}
                                currentUserId={currentUserId}
                                depth={depth + 1}
                                maxDepth={maxDepth}
                                unreadCommentIDs={unreadCommentIDs}
                                manualReadCommentIDs={manualReadCommentIDs}
                                commentReadMode={commentReadMode}
                                onToggleRead={onToggleRead}
                                onOpenThread={onOpenThread}
                                readOnly={readOnly}
                                allowReadTracking={allowReadTracking}
                                parentComment={comment}
                                variant="desktop"
                                onDirtyStateChange={onDirtyStateChange}
                            />
                        ))
                    )}
                  </div>
              )}
              </div>
            )}

            {/* Mobile: Show if not at or beyond mobile max depth - 1 */}
            {/* Only render mobile section if we're not locked to desktop variant */}
            {variant !== 'desktop' && (
              <div className="md:hidden">
                {depth < mobileMaxDepth - 1 && (
                  <div className="space-y-1">
                    {loadingReplies ? (
                        <div className="ml-2 md:ml-6 py-2 text-xs text-content-secondary">
                          Loading replies...
                        </div>
                    ) : (
                        // Use dynamically loaded replies if we have them (hasLoadedRef = true means we've loaded/reloaded)
                        // Otherwise fall back to pre-loaded children
                        // This allows optimistic updates and fresh data to override preloaded children
                        (hasLoadedRef.current ? replies : (hasPreloadedChildren ? preloadedChildren : replies)).map((reply) => (
                            <ThreadedComment
                                key={`${reply.id}-mobile`}
                                comment={reply}
                                gameId={gameId}
                                postId={postId}
                                characters={characters}
                                controllableCharacters={controllableCharacters}
                                onCreateReply={onCreateReply}
                                onCommentDeleted={handleNestedCommentDeleted}
                                currentUserId={currentUserId}
                                depth={depth + 1}
                                maxDepth={maxDepth}
                                unreadCommentIDs={unreadCommentIDs}
                                manualReadCommentIDs={manualReadCommentIDs}
                                commentReadMode={commentReadMode}
                                onToggleRead={onToggleRead}
                                onOpenThread={onOpenThread}
                                readOnly={readOnly}
                                allowReadTracking={allowReadTracking}
                                parentComment={comment}
                                variant="mobile"
                                onDirtyStateChange={onDirtyStateChange}
                            />
                        ))
                    )}
                  </div>
                )}
              </div>
            )}
          </>
      )}


      {/* Delete Confirmation Modal */}
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
    </div>
  );
});
