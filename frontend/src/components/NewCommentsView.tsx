import { useState, useCallback, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { RefreshCw } from 'lucide-react';
import { useRecentComments } from '../hooks/useRecentComments';
import { CommentWithParentCard } from './CommentWithParentCard';
import { Spinner, Alert, Button, Toggle } from './ui';
import { useManualReadCommentIDs, useToggleCommentRead } from '../hooks/useReadTracking';
import { useInfiniteScrollSentinel } from '../hooks/useInfiniteScrollSentinel';
import { useCommentReadMode } from '../hooks/useUserPreferences';
import { useUnreadOnlyFilter } from '../hooks/useUnreadOnlyFilter';

interface NewCommentsViewProps {
  gameId: number;
}

/**
 * Displays a paginated list of recent comments with their parent context.
 * Supports infinite scrolling to load more comments as the user scrolls.
 */
export function NewCommentsView({ gameId }: NewCommentsViewProps) {
  const navigate = useNavigate();

  const commentReadMode = useCommentReadMode();
  const isManualMode = commentReadMode === 'manual';

  // Unread-only filtering is a manual-read-mode concept: it hides comments the
  // user explicitly marked as read. The filter is only offered (and only
  // applied) in manual mode. The choice persists per game across refreshes.
  const [unreadOnly, setUnreadOnly] = useUnreadOnlyFilter(gameId);
  const showUnreadOnly = isManualMode && unreadOnly;

  const {
    data,
    isLoading,
    isError,
    error,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
    refetch,
  } = useRecentComments(gameId, showUnreadOnly);
  const { data: manualReads = [], refetch: refetchManualReads } = useManualReadCommentIDs(gameId);
  const toggleReadMutation = useToggleCommentRead();

  // Flatten all manually-read comment IDs across all posts into a single Set for O(1) lookup
  const readCommentIdSet = useMemo(() => {
    const ids = new Set<number>();
    for (const entry of manualReads) {
      for (const id of entry.read_comment_ids) {
        ids.add(id);
      }
    }
    return ids;
  }, [manualReads]);

  const handleToggleRead = useCallback((commentId: number, postId: number, currentlyRead: boolean) => {
    toggleReadMutation.mutate({
      gameId,
      postId,
      commentId,
      read: !currentlyRead,
    });
  }, [gameId, toggleReadMutation]);

  // Refresh state
  const [isRefreshing, setIsRefreshing] = useState(false);

  // Infinite scroll sentinel
  const sentinelRef = useInfiniteScrollSentinel({
    enabled: hasNextPage && !isFetchingNextPage,
    onIntersect: fetchNextPage,
    threshold: 0.1,
  });

  // Loading state
  if (isLoading) {
    return (
      <div className="flex justify-center items-center py-12">
        <Spinner size="lg" />
      </div>
    );
  }

  // Error state
  if (isError) {
    return (
      <Alert variant="danger">
        <p>Failed to load recent comments</p>
        <p className="text-sm text-text-muted mt-1">
          {error instanceof Error ? error.message : 'Unknown error'}
        </p>
      </Alert>
    );
  }

  // Flatten all pages of comments into a single array
  const allComments = data?.pages.flatMap((page) => page.comments) ?? [];

  // Empty state — distinguish "nothing here at all" from "everything is read"
  if (allComments.length === 0) {
    return (
      <div className="text-center py-12">
        {showUnreadOnly ? (
          <>
            <p className="text-content-secondary">No unread comments</p>
            <p className="text-sm text-text-secondary mt-2">
              You've read everything here. Turn off "Unread only" to see all comments.
            </p>
            <Button
              variant="secondary"
              size="sm"
              className="mt-4"
              onClick={() => setUnreadOnly(false)}
            >
              Show all comments
            </Button>
          </>
        ) : (
          <>
            <p className="text-content-secondary">No comments yet</p>
            <p className="text-sm text-text-secondary mt-2">
              Be the first to start a conversation in the Common Room!
            </p>
          </>
        )}
      </div>
    );
  }

  // Navigate to the comment's parent message in the Common Room
  const handleNavigateToParent = (comment: typeof allComments[0]) => {
    if (!comment.parent_id) return;

    // Navigate to Common Room with deep link to parent comment
    navigate(`/games/${gameId}?tab=common-room&comment=${comment.parent_id}`);
  };

  // Navigate to the comment itself in the Common Room
  const handleNavigateToComment = (comment: typeof allComments[0]) => {
    // Navigate to Common Room with deep link to this comment
    navigate(`/games/${gameId}?tab=common-room&comment=${comment.id}`);
  };

  // Handle refresh button click
  const handleRefresh = async () => {
    setIsRefreshing(true);
    try {
      await Promise.all([refetch(), refetchManualReads()]);
    } finally {
      setIsRefreshing(false);
    }
  };

  return (
    <div className="space-y-4">
      {/* Header with unread filter + refresh button */}
      <div className="flex justify-between items-center gap-3 flex-wrap">
        <h3 className="text-lg font-semibold text-content-primary">Recent Comments</h3>
        <div className="flex items-center gap-3">
          {isManualMode && (
            <label className="flex items-center gap-2 cursor-pointer">
              <span className="text-sm text-content-secondary select-none">Unread only</span>
              <Toggle
                size="sm"
                checked={unreadOnly}
                onChange={setUnreadOnly}
                aria-label="Unread only"
                data-testid="unread-only-toggle"
              />
            </label>
          )}
          <Button
            variant="ghost"
            size="sm"
            onClick={handleRefresh}
            disabled={isRefreshing || isLoading}
            className="flex items-center gap-2"
          >
            <RefreshCw className={`w-4 h-4 ${isRefreshing ? 'animate-spin' : ''}`} />
            <span>{isRefreshing ? 'Refreshing...' : 'Refresh'}</span>
          </Button>
        </div>
      </div>

      {/* Comments list */}
      {allComments.map((comment) => (
        <CommentWithParentCard
          key={comment.id}
          comment={comment}
          gameId={gameId}
          onNavigateToParent={() => handleNavigateToParent(comment)}
          onNavigateToComment={() => handleNavigateToComment(comment)}
          commentReadMode={commentReadMode}
          isRead={readCommentIdSet.has(comment.id)}
          onToggleRead={comment.post_id ? (currentlyRead) => handleToggleRead(comment.id, comment.post_id!, currentlyRead) : undefined}
        />
      ))}

      {/* Infinite scroll sentinel */}
      <div ref={sentinelRef} className="h-20 flex items-center justify-center">
        {isFetchingNextPage && <Spinner size="md" />}
        {!hasNextPage && allComments.length > 0 && (
          <p className="text-sm text-content-tertiary">No more comments to load</p>
        )}
      </div>
    </div>
  );
}
