import React, { useState, useEffect, useRef, memo, useCallback } from 'react';
import { Link } from 'react-router-dom';
import { formatDistanceToNow } from 'date-fns';
import type { Message } from '../types/messages';
import type { Character } from '../types/characters';
import { ThreadedComment } from './ThreadedComment';
import { ThreadViewModal } from './ThreadViewModal';
import { apiClient } from '../lib/api';
import { CommentEditor } from './CommentEditor';
import CharacterAvatar from './CharacterAvatar';
import { MarkdownPreview } from './MarkdownPreview';
import { useMarkPostAsRead, usePostUnreadCommentIDs, usePostManualReadCommentIDs, useToggleCommentRead } from '../hooks/useReadTracking';
import { useCommentReadMode } from '../hooks/useUserPreferences';
import { useUpdatePost } from '../hooks';
import { Button, Select } from './ui';
import { logger } from '@/services/LoggingService';
import { buildCommentTree, pruneDeletedLeaves, type CommentTreeNode } from '../lib/utils/commentTree';
import { COMMENT_MAX_DEPTH } from '@/config/comments';
import { usePostCollapseState } from '../hooks/usePostCollapseState';
import { useInfiniteScrollSentinel } from '../hooks/useInfiniteScrollSentinel';
import { useOptionalGameContext } from '../contexts/GameContext';
import { useScreenshotMode } from '../hooks/useScreenshotMode';

interface PostCardProps {
  post: Message;
  gameId: number;
  characters: Character[]; // All game characters (for autocomplete)
  controllableCharacters: Character[]; // Characters the user can control (for "Reply as" dropdown)
  onCreateComment: (parentId: number, characterId: number, content: string, rootPostId: number) => Promise<void>;
  onPostUpdated?: (updatedPost: Message) => void; // Callback when post is edited
  currentUserId?: number;
  'data-testid'?: string;
  readOnly?: boolean; // Disable all interactive features (for history view)
  allowReadTracking?: boolean; // Show faded read state and toggle button (default true)
}

// Memoized comment list that only re-renders when commentTree changes
// This prevents re-renders when typing in the reply box (replyContent state changes)
const CommentList = memo(function CommentList({
  commentTree,
  gameId,
  postId,
  characters,
  controllableCharacters,
  onCreateComment,
  loadComments,
  currentUserId,
  localUnreadCommentIDs,
  manualReadCommentIDs,
  commentReadMode,
  onToggleRead,
  onOpenThread,
  readOnly,
  allowReadTracking,
}: {
  commentTree: CommentTreeNode[];
  gameId: number;
  postId: number;
  characters: Character[];
  controllableCharacters: Character[];
  onCreateComment: (parentId: number, characterId: number, content: string, rootPostId: number) => Promise<void>;
  loadComments: () => Promise<void>;
  currentUserId?: number;
  localUnreadCommentIDs: number[];
  manualReadCommentIDs: number[];
  commentReadMode: 'auto' | 'manual';
  onToggleRead: (commentId: number, currentlyRead: boolean) => void;
  onOpenThread: (comment: Message) => void;
  readOnly: boolean;
  allowReadTracking: boolean;
}) {
  return (
    <div className="space-y-4">
      {commentTree.map((commentNode) => (
        <ThreadedComment
          key={commentNode.id}
          comment={commentNode}
          gameId={gameId}
          postId={postId}
          characters={characters}
          controllableCharacters={controllableCharacters}
          onCreateReply={onCreateComment}
          onCommentDeleted={loadComments}
          currentUserId={currentUserId}
          depth={0}
          maxDepth={COMMENT_MAX_DEPTH}
          unreadCommentIDs={localUnreadCommentIDs}
          manualReadCommentIDs={manualReadCommentIDs}
          commentReadMode={commentReadMode}
          onToggleRead={onToggleRead}
          onOpenThread={onOpenThread}
          readOnly={readOnly}
          allowReadTracking={allowReadTracking}
          parentComment={null}
        />
      ))}
    </div>
  );
});

export const PostCard = React.memo(function PostCard({ post, gameId, characters, controllableCharacters, onCreateComment, onPostUpdated, currentUserId, 'data-testid': dataTestId, readOnly = false, allowReadTracking = true }: PostCardProps) {
  const [showComments, setShowComments] = useState(true);
  const [isCommenting, setIsCommenting] = useState(false);
  const [commentTree, setCommentTree] = useState<CommentTreeNode[]>([]);
  const [loadingComments, setLoadingComments] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [replyContent, setReplyContent] = useState('');
  const [selectedCharacterId, setSelectedCharacterId] = useState<number | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isPostCollapsed, setIsPostCollapsed] = usePostCollapseState(post.id);
  const [threadModalComment, setThreadModalComment] = useState<Message | null>(null);
  const gameContext = useOptionalGameContext();
  const portraitAvatars = gameContext?.game?.portrait_avatars ?? false;
  const { screenshotModeEnabled } = useScreenshotMode();

  // Pagination state. `offset` lives in a ref, not state: nothing renders it,
  // and reading it through the ref means loadComments/loadMoreComments always
  // see the current value even when a fetch was started from an older render.
  const offsetRef = useRef(0);
  const [hasMore, setHasMore] = useState(false);
  const [totalTopLevel, setTotalTopLevel] = useState(0);
  const [returnedTopLevel, setReturnedTopLevel] = useState(0);
  const [initialLoadFailed, setInitialLoadFailed] = useState(false);
  const [initialLoadAttempt, setInitialLoadAttempt] = useState(0);
  const THREADS_PER_PAGE = 5;

  // Edit state
  const [isEditing, setIsEditing] = useState(false);
  const [editContent, setEditContent] = useState(post.content);

  // Get unread comment IDs for this post from the query
  const unreadCommentIDs = usePostUnreadCommentIDs(gameId, post.id);

  // Local state to preserve unread IDs and prevent auto-clearing
  const [localUnreadCommentIDs, setLocalUnreadCommentIDs] = useState<number[]>([]);

  const commentReadModeRaw = useCommentReadMode();
  const commentReadMode = allowReadTracking ? commentReadModeRaw : 'auto';
  const manualReadCommentIDsRaw = usePostManualReadCommentIDs(gameId, post.id);
  const manualReadCommentIDs = allowReadTracking ? manualReadCommentIDsRaw : [];
  const toggleCommentReadMutation = useToggleCommentRead();

  const handleToggleRead = useCallback((commentId: number, currentlyRead: boolean) => {
    toggleCommentReadMutation.mutate({
      gameId,
      postId: post.id,
      commentId,
      read: !currentlyRead,
    });
  }, [toggleCommentReadMutation, gameId, post.id]);

  // Mutation for marking post as read
  const markAsReadMutation = useMarkPostAsRead();

  // Ref for the post container (for intersection observer)
  const postRef = useRef<HTMLDivElement>(null);

  // Serializes silent refreshes: a refresh only applies its results if no
  // newer refresh has started since (prevents a stale response from resurrecting
  // just-deleted comments or hiding a just-posted one).
  const refreshSeqRef = useRef(0);
  // Dedupes the initial comments fetch. A ref (not state) so it survives
  // StrictMode's dev-only unmount/remount cycle, which otherwise fires the
  // effect — and the network request — twice. Keyed so a different post/game
  // on the same mounted component still triggers a fresh load.
  const initialLoadKeyRef = useRef<string | null>(null);

  // Track if we've already marked this post as read in this session
  const hasMarkedAsRead = useRef(false);

  // Auto-select first controllable character
  useEffect(() => {
    if (controllableCharacters.length > 0 && selectedCharacterId === null) {
      setSelectedCharacterId(controllableCharacters[0].id);
    }
  }, [controllableCharacters, selectedCharacterId]);

  // Initialize local unread IDs when query result changes (first load only)
  useEffect(() => {
    if (unreadCommentIDs.length > 0 && localUnreadCommentIDs.length === 0) {
      setLocalUnreadCommentIDs(unreadCommentIDs);
    }
  }, [unreadCommentIDs, localUnreadCommentIDs.length]);

  // Load paginated comments with all nested replies when showing comments.
  // No isMounted guard: the single deduped fetch must apply its results even
  // when StrictMode has already "cleaned up" the effect run that started it
  // (setState after unmount is a no-op in React 18, not an error).
  useEffect(() => {
    if (!showComments) return;

    const loadKey = `${gameId}:${post.id}`;
    if (initialLoadKeyRef.current === loadKey) return;
    initialLoadKeyRef.current = loadKey;

    const loadInitialComments = async () => {
      try {
        setLoadingComments(true);
        setInitialLoadFailed(false);
        const response = await apiClient.messages.getPostCommentsWithThreads(
          gameId,
          post.id,
          THREADS_PER_PAGE,
          0,
          5 // max_depth
        );
        const tree = pruneDeletedLeaves(buildCommentTree(response.data.comments));
        setCommentTree(tree);
        setTotalTopLevel(response.data.total_top_level);
        setReturnedTopLevel(response.data.returned_top_level);
        setHasMore(response.data.has_more);
        offsetRef.current = THREADS_PER_PAGE;
      } catch (_err) {
        logger.error('Failed to load comments', { error: _err, gameId, postId: post.id });
        // Surface the failure (instead of a false "No comments yet" empty state)
        // and clear the dedupe key so the Retry button's attempt counter can
        // re-run this effect. Re-renders alone do NOT retry — the deps below
        // only change on retry, post/game change, or collapse/re-expand.
        setInitialLoadFailed(true);
        initialLoadKeyRef.current = null;
      } finally {
        setLoadingComments(false);
      }
    };

    loadInitialComments();
  }, [showComments, gameId, post.id, initialLoadAttempt]);

  // Mark post as read immediately when user views it (on page load)
  useEffect(() => {
    // Always mark as read on first view to establish a read marker
    // This ensures that future comments will be correctly detected as "new"
    // Without this, users who view a post before any comments exist will never get
    // a read marker, and thus will never see new comments highlighted
    if (!hasMarkedAsRead.current) {
      markAsReadMutation.mutate({
        gameId,
        postId: post.id,
        data: {} // Mark as read with current timestamp
      });

      hasMarkedAsRead.current = true;
    }
  }, [gameId, post.id, markAsReadMutation]);

  // Window-preserving silent refresh — re-fetches the currently-loaded window without
  // collapsing the list or resetting scroll position.
  const loadComments = useCallback(async (delayMs: number = 0) => {
    const seq = ++refreshSeqRef.current;
    try {
      if (delayMs > 0) {
        await new Promise(resolve => setTimeout(resolve, delayMs));
      }
      // Re-fetch the entire currently-loaded window in one request.
      // Cap at backend max of 500 to avoid exceeding server limits.
      // If a concurrent loadMoreComments lands while we're fetching, the
      // window we asked for no longer covers [0, offset) — re-fetch with the
      // grown window so applying the result can't drop the appended page.
      // Terminates: offset only grows and the window is capped at 500.
      let windowSize: number;
      let response;
      do {
        windowSize = Math.min(Math.max(offsetRef.current, THREADS_PER_PAGE), 500);
        response = await apiClient.messages.getPostCommentsWithThreads(
          gameId,
          post.id,
          windowSize,
          0,
          5 // max_depth
        );
      } while (windowSize < Math.min(Math.max(offsetRef.current, THREADS_PER_PAGE), 500));
      // A newer refresh started while this one was in flight — let it win.
      if (seq !== refreshSeqRef.current) return;
      const tree = pruneDeletedLeaves(buildCommentTree(response.data.comments));
      setCommentTree(tree);
      setTotalTopLevel(response.data.total_top_level);
      setReturnedTopLevel(response.data.returned_top_level);
      setHasMore(response.data.has_more);
      // Keep `offset` unchanged — the window covers [0, offset), so the next
      // sentinel-triggered page continues from where it did before.
    } catch (_err) {
      logger.error('Failed to refresh comments', { error: _err, gameId, postId: post.id });
    }
  }, [gameId, post.id]);

  // Load more comments (append to existing tree)
  const loadMoreComments = useCallback(async () => {
    if (loadingMore || !hasMore) return;
    try {
      setLoadingMore(true);
      const response = await apiClient.messages.getPostCommentsWithThreads(
        gameId,
        post.id,
        THREADS_PER_PAGE,
        offsetRef.current,
        5 // max_depth
      );
      const newTree = pruneDeletedLeaves(buildCommentTree(response.data.comments));
      setCommentTree(prev => {
        const existingIds = new Set(prev.map(node => node.id));
        const uniqueNew = newTree.filter(node => !existingIds.has(node.id));
        return [...prev, ...uniqueNew];
      });
      setReturnedTopLevel(prev => prev + response.data.returned_top_level);
      // Refresh the total too: concurrent posting shifts it, and a stale total
      // makes the "N remaining" label drift wrong (even negative).
      setTotalTopLevel(response.data.total_top_level);
      setHasMore(response.data.has_more);
      offsetRef.current += THREADS_PER_PAGE;
    } catch (_err) {
      logger.error('Failed to load more comments', { error: _err, gameId, postId: post.id, offset: offsetRef.current });
    } finally {
      setLoadingMore(false);
    }
  // The loadingMore dep doubles as the sentinel re-arm signal: a new callback
  // identity re-creates the observer below, which re-checks intersection so
  // back-to-back pages keep loading while the sentinel stays visible.
  }, [loadingMore, hasMore, gameId, post.id]);

  // Auto-load the next page when the sentinel scrolls within 800px of the viewport.
  const sentinelRef = useInfiniteScrollSentinel({
    enabled: hasMore,
    onIntersect: loadMoreComments,
    rootMargin: '800px',
  });

  const handleShowComments = () => {
    setShowComments(!showComments);
  };

  // Stable callback for opening thread modal (prevents CommentList re-renders)
  const handleOpenThread = useCallback((comment: Message) => {
    setThreadModalComment(comment);
  }, []);

  const handleSubmitComment = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!selectedCharacterId || !replyContent.trim()) {
      return;
    }

    try {
      setIsSubmitting(true);
      await onCreateComment(post.id, selectedCharacterId, replyContent.trim(), post.id);
      setReplyContent('');
      setIsCommenting(false);
      // Ensure comments are shown and reload to display the new one
      setShowComments(true);
      await loadComments();

    } catch (_err) {
      logger.error('Failed to submit comment', { error: _err, gameId, postId: post.id, characterId: selectedCharacterId });
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

  const isAuthor = currentUserId === post.author_id;
  const updatePostMutation = useUpdatePost();

  // Edit handlers
  const handleEdit = () => {
    setEditContent(post.content);
    setIsEditing(true);
  };

  const handleCancelEdit = () => {
    setEditContent(post.content);
    setIsEditing(false);
  };

  const handleSaveEdit = async () => {
    if (!editContent.trim() || editContent === post.content) {
      setIsEditing(false);
      return;
    }

    try {
      const updatedPost = await updatePostMutation.mutateAsync({
        gameId,
        postId: post.id,
        content: editContent.trim()
      });
      setIsEditing(false);
      // Notify parent component of the update so it can update its local state
      onPostUpdated?.(updatedPost);
    } catch (_err) {
      logger.error('Failed to update post', { error: _err, gameId, postId: post.id });
    }
  };

  const postContentBody = isEditing ? (
    <div className="space-y-3">
      <CommentEditor
        value={editContent}
        onChange={setEditContent}
        placeholder="Edit your post..."
        disabled={updatePostMutation.isPending}
        characters={characters}
        maxLength={50000}
        showCharacterCount={true}
      />
      <div className="flex gap-2">
        <Button
          variant="primary"
          onClick={handleSaveEdit}
          disabled={updatePostMutation.isPending || !editContent.trim() || editContent === post.content}
        >
          {updatePostMutation.isPending ? 'Saving...' : 'Save'}
        </Button>
        <Button
          variant="ghost"
          onClick={handleCancelEdit}
          disabled={updatePostMutation.isPending}
        >
          Cancel
        </Button>
      </div>
    </div>
  ) : (
    <MarkdownPreview
      content={post.content}
      mentionedCharacters={characters}
      fullWidth
    />
  );

  // Determine if post content is long (more than 500 characters)
  const isLongContent = post.content.length > 500;

  return (
    <div ref={postRef} id={`comment-${post.id}`} data-testid={dataTestId || "post-card"} className="mb-8">
      {/* Post Card - Contains both post and comments */}
      <div className="surface-base md:rounded-xl shadow-lg border border-theme-default overflow-hidden">
      {/* GM Post Header Section */}
      <div className="bg-interactive-primary-subtle border-b-2 border-interactive-primary">
        {/* Post Header - Always visible */}
        <div className="py-3 px-3 md:p-4 surface-base bg-opacity-90 border-b border-interactive-primary">
          {portraitAvatars ? (
            /* Portrait mode: avatar floats left, name + content flow around it */
            <div className="overflow-hidden">
              <div className="float-left mr-3 mb-2">
                <CharacterAvatar
                  avatarUrl={post.character_avatar_url}
                  characterName={post.character_name}
                  shape="portrait"
                />
              </div>
              <div>
                <div className="flex items-start justify-between mb-1">
                  <div>
                    <Link to={`/characters/${post.character_id}`} className="font-bold text-xl text-content-primary hover:underline">{post.character_name}</Link>
                    <p className="text-sm text-content-secondary">
                      {post.author_username && !screenshotModeEnabled ? `Posted by @${post.author_username} · ` : 'Posted '}{formatDate(post.created_at)}
                      {post.is_edited && <span className="ml-1 text-content-tertiary">(edited)</span>}
                      {isAuthor && !screenshotModeEnabled && (
                        <span className="ml-2 text-xs bg-interactive-primary-subtle text-interactive-primary px-2 py-0.5 rounded">You</span>
                      )}
                      {isAuthor && !readOnly && !isEditing && !screenshotModeEnabled && (
                        <button
                          onClick={handleEdit}
                          className="ml-2 text-xs text-interactive-primary hover:text-interactive-primary-hover underline"
                        >
                          Edit
                        </button>
                      )}
                    </p>
                  </div>
                  {localUnreadCommentIDs.length > 0 && (
                    <span className="px-2 py-1 text-xs font-semibold bg-interactive-primary-subtle text-interactive-primary rounded">
                      {localUnreadCommentIDs.length} new {localUnreadCommentIDs.length === 1 ? 'comment' : 'comments'}
                    </span>
                  )}
                </div>

                {/* Action Buttons */}
                <div className="flex items-center gap-2 mb-2">
                  {isLongContent && (
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => setIsPostCollapsed(!isPostCollapsed)}
                      className="text-interactive-primary hover:text-interactive-primary-hover"
                    >
                    {isPostCollapsed ? (
                      <>
                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                        </svg>
                        Show Full Post
                      </>
                    ) : (
                      <>
                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 15l7-7 7 7" />
                        </svg>
                        Collapse Post
                      </>
                    )}
                    </Button>
                  )}
                </div>

                {/* Post content flows around the floating portrait */}
                {(!isLongContent || !isPostCollapsed) && postContentBody}
              </div>
            </div>
          ) : (
            /* Standard circular avatar mode */
            <>
              <div className="flex items-start justify-between mb-2">
                <div className="flex-1">
                  <div className="flex items-center gap-3">
                    <CharacterAvatar
                      avatarUrl={post.character_avatar_url}
                      characterName={post.character_name}
                      size="lg"
                      className="md:w-16 md:h-16"
                    />
                    <div className="flex-1">
                      <Link to={`/characters/${post.character_id}`} className="font-bold text-xl text-content-primary hover:underline">{post.character_name}</Link>
                      <p className="text-sm text-content-secondary">
                        {post.author_username && !screenshotModeEnabled ? `Posted by @${post.author_username} · ` : 'Posted '}{formatDate(post.created_at)}
                        {post.is_edited && <span className="ml-1 text-content-tertiary">(edited)</span>}
                        {isAuthor && !screenshotModeEnabled && (
                          <span className="ml-2 text-xs bg-interactive-primary-subtle text-interactive-primary px-2 py-0.5 rounded">You</span>
                        )}
                        {isAuthor && !readOnly && !isEditing && !screenshotModeEnabled && (
                          <button
                            onClick={handleEdit}
                            className="ml-2 text-xs text-interactive-primary hover:text-interactive-primary-hover underline"
                          >
                            Edit
                          </button>
                        )}
                      </p>
                    </div>
                    {localUnreadCommentIDs.length > 0 && (
                      <span className="px-2 py-1 text-xs font-semibold bg-interactive-primary-subtle text-interactive-primary rounded">
                        {localUnreadCommentIDs.length} new {localUnreadCommentIDs.length === 1 ? 'comment' : 'comments'}
                      </span>
                    )}
                  </div>
                </div>
              </div>

              {/* Action Buttons */}
              <div className="flex items-center gap-2 mt-2">
                {isLongContent && (
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => setIsPostCollapsed(!isPostCollapsed)}
                    className="text-interactive-primary hover:text-interactive-primary-hover"
                  >
                  {isPostCollapsed ? (
                    <>
                      <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                      </svg>
                      Show Full Post
                    </>
                  ) : (
                    <>
                      <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 15l7-7 7 7" />
                      </svg>
                      Collapse Post
                    </>
                  )}
                  </Button>
                )}
              </div>
            </>
          )}
        </div>

        {/* Post Content - Only rendered in standard (non-portrait) mode */}
        {!portraitAvatars && (!isLongContent || !isPostCollapsed) && (
          <div className="py-4 px-3 md:p-6 surface-base">
            {postContentBody}
          </div>
        )}
      </div>

      {/* Comments Section - Inside the card */}
      <div className="surface-raised border-t border-theme-default" data-comments-section="true">
        <div className="py-4 px-3 md:p-4 flex items-center gap-4 text-sm text-content-secondary flex-wrap border-b border-theme-default">
          <Button
            variant="ghost"
            onClick={handleShowComments}
            className="flex items-center gap-2"
          >
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
            </svg>
            <span>
              {showComments ? 'Collapse' : 'Expand'} Comments ({post.comment_count || 0})
            </span>
          </Button>

          {!isCommenting && !readOnly && controllableCharacters.length > 0 && (
            <Button
              variant="primary"
              onClick={() => setIsCommenting(true)}
              className="flex items-center gap-2"
            >
              <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 10h10a8 8 0 018 8v2M3 10l6 6m-6-6l6-6" />
              </svg>
              <span>Add Comment</span>
            </Button>
          )}
        </div>

        {/* Inline Reply Form (at top level) */}
        {isCommenting && !readOnly && (
          <div className="px-3 md:px-4 pb-4">
            <form onSubmit={handleSubmitComment} className="surface-base rounded-lg py-3 px-3 md:p-4 border border-theme-default shadow-sm">
            {controllableCharacters.length > 0 ? (
              <>
                {controllableCharacters.length > 1 && (
                  <Select
                    value={selectedCharacterId || ''}
                    onChange={(e) => setSelectedCharacterId(Number(e.target.value))}
                    className="mb-3"
                    disabled={isSubmitting}
                  >
                    {controllableCharacters.map((char) => (
                      <option key={char.id} value={char.id}>
                        Reply as {char.name}
                      </option>
                    ))}
                  </Select>
                )}

                <div className="mb-3">
                  <CommentEditor
                    value={replyContent}
                    onChange={setReplyContent}
                    placeholder="Write a comment..."
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
                    disabled={isSubmitting || !replyContent.trim()}
                  >
                    {isSubmitting ? 'Posting...' : 'Comment'}
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    onClick={() => {
                      setIsCommenting(false);
                      setReplyContent('');
                    }}
                    disabled={isSubmitting}
                  >
                    Cancel
                  </Button>
                </div>
              </>
            ) : (
              <p className="text-sm text-content-secondary">You need a character to comment.</p>
            )}
          </form>
          </div>
        )}

        {/* Threaded Comments */}
        {showComments && (
          loadingComments ? (
            <div className="text-sm text-content-secondary text-center py-4">Loading comments...</div>
          ) : initialLoadFailed ? (
            <div className="flex flex-col items-center gap-2 py-4">
              <span className="text-sm text-content-secondary">Failed to load comments.</span>
              <Button variant="ghost" onClick={() => setInitialLoadAttempt(attempt => attempt + 1)}>
                Retry
              </Button>
            </div>
          ) : commentTree.length === 0 ? (
            <p className="text-sm text-content-secondary italic text-center py-4">No comments yet. Be the first to reply!</p>
          ) : (
            <>
              <CommentList
                commentTree={commentTree}
                gameId={gameId}
                postId={post.id}
                characters={characters}
                controllableCharacters={controllableCharacters}
                onCreateComment={onCreateComment}
                loadComments={loadComments}
                currentUserId={currentUserId}
                localUnreadCommentIDs={localUnreadCommentIDs}
                manualReadCommentIDs={manualReadCommentIDs}
                commentReadMode={commentReadMode}
                onToggleRead={handleToggleRead}
                onOpenThread={handleOpenThread}
                readOnly={readOnly}
                allowReadTracking={allowReadTracking}
              />

              {/* Sentinel for infinite scroll auto-load */}
              <div ref={sentinelRef} data-testid="comments-sentinel" />

              {/* Load More Button */}
              {hasMore && (
                <div className="mt-6 flex justify-center border-t border-theme-default pt-4">
                  <Button
                    variant="ghost"
                    onClick={loadMoreComments}
                    disabled={loadingMore}
                    className="flex items-center gap-2"
                  >
                    {loadingMore ? (
                      <>
                        <svg className="animate-spin h-5 w-5" fill="none" viewBox="0 0 24 24">
                          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                        </svg>
                        <span>Loading...</span>
                      </>
                    ) : (
                      <>
                        <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                        </svg>
                        <span>
                          Load More Comments ({totalTopLevel - returnedTopLevel} remaining)
                        </span>
                      </>
                    )}
                  </Button>
                </div>
              )}
            </>
          )
        )}
      </div>
      </div>
      {/* End of Post Card */}

      {/* Thread View Modal */}
      {threadModalComment !== null && (
        <ThreadViewModal
          gameId={gameId}
          postId={post.id} // Pass the root post ID
          comment={threadModalComment}
          characters={characters}
          controllableCharacters={controllableCharacters}
          onClose={() => setThreadModalComment(null)}
          onCreateReply={onCreateComment}
          currentUserId={currentUserId}
          unreadCommentIDs={localUnreadCommentIDs}
          manualReadCommentIDs={manualReadCommentIDs}
          commentReadMode={commentReadMode}
          onToggleRead={handleToggleRead}
          readOnly={readOnly}
          allowReadTracking={allowReadTracking}
        />
      )}
    </div>
  );
});
