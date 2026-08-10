import React, { useState, useEffect, useCallback, useMemo, useRef, lazy, Suspense } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useSearchParams, useNavigate } from 'react-router-dom';
import { apiClient } from '../lib/api';
import { useAuth } from '../contexts/AuthContext';
import { useGameContext } from '../contexts/GameContext';
import { Button, Alert, Spinner, Card } from './ui';
import type { Message } from '../types/messages';
import type { GamePhase } from '../types/phases';
import { CreatePostForm } from './CreatePostForm';
import { PostCard } from './PostCard';
import { ThreadViewModal } from './ThreadViewModal';
import { NewCommentsView } from './NewCommentsView';
import { MarkdownPreview } from './MarkdownPreview';
import { RecentResultsSection } from './RecentResultsSection';
import type { GameUtilityContext } from './utility-drawer/types';
import { useProvideGameUtilityContext } from '../contexts/UtilityDrawerContext';
import { usePreviousPhaseResults } from '../hooks/usePreviousPhaseResults';
import { usePollsByPhase, useDraftPost } from '../hooks';
import { useToggleCommentRead, usePostManualReadCommentIDs } from '../hooks/useReadTracking';
import { useCommentReadMode } from '../hooks/useUserPreferences';
import { logger } from '@/services/LoggingService';
import { parentContextForViewport } from '@/config/comments';

// Lazy load PollsTab component
const PollsTab = lazy(() => import('./PollsTab').then(m => ({ default: m.PollsTab })));

interface CommonRoomProps {
  gameId: number;
  phaseId?: number;
  phaseTitle?: string;
  phaseDescription?: string;
  currentPhase?: GamePhase | null;
  isCurrentPhase?: boolean;
  isGM?: boolean;
  isAudience?: boolean;
  isGameCompleted?: boolean;
}

// Inner component so hooks run unconditionally with a known postId
function ThreadViewModalWithReadTracking(props: React.ComponentProps<typeof ThreadViewModal> & { gameId: number; postId: number }) {
  const { gameId, postId, ...rest } = props;
  const commentReadMode = useCommentReadMode();
  const manualReadCommentIDs = usePostManualReadCommentIDs(gameId, postId);
  const toggleMutation = useToggleCommentRead();
  const handleToggleRead = useCallback(
    (commentId: number, currentlyRead: boolean) => {
      toggleMutation.mutate({ gameId, postId, commentId, read: !currentlyRead });
    },
    [gameId, postId, toggleMutation]
  );

  return (
    <ThreadViewModal
      {...rest}
      gameId={gameId}
      postId={postId}
      commentReadMode={commentReadMode}
      manualReadCommentIDs={manualReadCommentIDs}
      onToggleRead={handleToggleRead}
    />
  );
}

export function CommonRoom({ gameId, phaseId, phaseTitle, phaseDescription, currentPhase, isCurrentPhase = true, isGM = false, isAudience = false, isGameCompleted = false }: CommonRoomProps) {
  // Get current user from AuthContext
  const { currentUser } = useAuth();
  const currentUserId = currentUser?.id;

  const queryClient = useQueryClient();
  const commentReadMode = useCommentReadMode();
  const toggleCommentReadMutation = useToggleCommentRead();
  const allowReadTracking = !isGameCompleted;

  // Read character data and game settings from GameContext — single source of truth
  const { userCharacters, allGameCharacters, userRole, game } = useGameContext();
  const gameState = game?.state ?? '';

  // URL search params for deep linking to comments and sub-tab navigation
  const [searchParams, setSearchParams] = useSearchParams();
  const commentIdParam = searchParams.get('comment');
  const viewParam = searchParams.get('view') as 'posts' | 'newComments' | 'polls' | null;

  const [posts, setPosts] = useState<Message[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isCreatingPost, setIsCreatingPost] = useState(false);
  const [threadModalComment, setThreadModalComment] = useState<Message | null>(null);
  const [threadModalContext, setThreadModalContext] = useState<{
    parentChain: Message[];
    hasFullThread: boolean;
    targetCommentId: number;
    postId: number;
  } | null>(null);
  // Initialize activeTab from URL parameter, default to 'posts'
  const [activeTab, setActiveTab] = useState<'posts' | 'newComments' | 'polls'>(viewParam || 'posts');
  const navigate = useNavigate();

  const isAnonymous = game?.is_anonymous ?? false;
  const portraitAvatars = game?.portrait_avatars ?? false;

  // The drawer itself lives at the app root, opened from the global nav, so
  // it's reachable from every page; the game page contributes the game-scoped
  // half of its context on every tab. This room publishes over the top of that
  // for as long as it's mounted, because it knows something the page doesn't:
  // which phase is being viewed. In History that's a past phase, not the game's
  // current one, and phase-scoped utilities (Mark All Read) must act on the
  // phase on screen. Memoized because useProvideGameUtilityContext republishes
  // whenever the object identity changes.
  const gameUtilityContext = useMemo<GameUtilityContext>(
    () => ({
      gameId,
      currentPhase,
      isGM,
      isAudience,
      isGameCompleted,
      userRole,
      gameState,
      isAnonymous,
      portraitAvatars,
      userCharacters,
      allGameCharacters,
      commentReadMode,
    }),
    [
      gameId,
      currentPhase,
      isGM,
      isAudience,
      isGameCompleted,
      userRole,
      gameState,
      isAnonymous,
      portraitAvatars,
      userCharacters,
      allGameCharacters,
      commentReadMode,
    ]
  );
  useProvideGameUtilityContext('common-room', gameUtilityContext);

  // Ref to track scroll attempts (prevents duplicate attempts for same comment)
  const scrollAttemptedRef = useRef<string | null>(null);

  // State to show loading indicator while fetching deeply nested comments
  const [fetchingComment, setFetchingComment] = useState(false);

  // Fetch polls to calculate unvoted count for badge (phase-specific)
  const { data: polls = [], isLoading: pollsLoading } = usePollsByPhase(gameId, phaseId || 0);
  const unvotedPollsCount = polls.filter(poll => !poll.user_has_voted).length;

  // Fetch previous phase results (if applicable)
  const previousPhaseResults = usePreviousPhaseResults(gameId, currentPhase, isGM);

  // Fetch draft post for GM preview on pending phases
  const isPendingPhase = isGM && phaseId && currentPhase && !currentPhase.is_active && currentPhase.phase_type === 'common_room';
  const { data: draftPost } = useDraftPost(isPendingPhase ? phaseId : undefined);

  // Sync activeTab state with URL parameter
  useEffect(() => {
    const currentView = searchParams.get('view') as 'posts' | 'newComments' | 'polls' | null;
    if (currentView && currentView !== activeTab) {
      setActiveTab(currentView);
    } else if (!currentView && activeTab !== 'posts') {
      // Default to 'posts' if no view parameter
      setActiveTab('posts');
    }
  }, [searchParams, activeTab]);

  // Auto-scroll to comment from URL parameter
  useEffect(() => {
    // Don't attempt scroll if:
    // 1. No comment param
    // 2. Still loading initial data
    // 3. Already attempted scroll for this comment
    if (!commentIdParam || loading || scrollAttemptedRef.current === commentIdParam) {
      return;
    }

    // If there's a comment parameter, ensure we're on the 'posts' tab
    if (activeTab !== 'posts') {
      // Update both state and URL parameter (replace current URL to avoid extra history entry)
      setActiveTab('posts');
      const newParams = new URLSearchParams(searchParams);
      newParams.set('view', 'posts');
      setSearchParams(newParams, { replace: true }); // Replace to avoid extra history entry
      return; // Let the tab switch complete, then the effect will re-run
    }

    // Mark this comment as having been attempted
    scrollAttemptedRef.current = commentIdParam;

    // Use requestAnimationFrame to ensure DOM has rendered
    requestAnimationFrame(() => {
      // Shorter timeout since we know data is loaded (loading=false)
      const timer = setTimeout(async () => {
        // Try to find comment with various ID patterns (base, -desktop, -mobile)
        // Root comments use base ID, nested comments may have -desktop/-mobile suffix
        const baseEl = document.getElementById(`comment-${commentIdParam}`);
        const desktopEl = document.getElementById(`comment-${commentIdParam}-desktop`);
        const mobileEl = document.getElementById(`comment-${commentIdParam}-mobile`);
        // Prefer the visible element so scrollIntoView works (hidden elements don't scroll)
        const element = [baseEl, mobileEl, desktopEl].find(
          el => el && el.offsetParent !== null
        ) || baseEl || desktopEl || mobileEl;

        if (element) {
          // Comment is visible in the DOM - scroll to it
          element.scrollIntoView({ behavior: 'smooth', block: 'center', inline: 'nearest' });

          // Add bordered box styling to match modal appearance
          element.classList.add('ring-2', 'ring-interactive-primary', 'rounded-lg', 'p-1');

          // Remove after 5 seconds
          setTimeout(() => {
            element.classList.remove('ring-2', 'ring-interactive-primary', 'rounded-lg', 'p-1');
          }, 5000);

          // Clear the comment parameter from URL after scrolling
          const newParams = new URLSearchParams(searchParams);
          newParams.delete('comment');
          setSearchParams(newParams, { replace: true });
        } else {
          // Comment not found in DOM - fetch it to determine where it lives
          logger.debug('Comment not found in DOM, fetching comment metadata', { commentId: commentIdParam, gameId });

          const fetchAndShowComment = async () => {
            setFetchingComment(true);
            try {
              // Fetch the target comment plus a bounded slice of its ancestor
              // chain and the true root post ID, in a single request. chain is
              // ordered parent-to-child (ancestor → target).
              //
              // The parent count depends on the viewport: the modal renders the
              // chain nested from depth 0, so requesting more parents than the
              // deepest visible level pushes the target behind a "Continue this
              // thread" button. Mobile's shallower max depth means fewer parents.
              const isMobile = typeof window !== 'undefined'
                && typeof window.matchMedia === 'function'
                && window.matchMedia('(max-width: 767px)').matches;
              const contextResponse = await apiClient.messages.getMessageThreadContext(
                gameId,
                parseInt(commentIdParam),
                parentContextForViewport(isMobile)
              );
              const { chain, root_post_id, has_full_thread } = contextResponse.data;

              if (chain.length === 0) {
                throw new Error('No messages fetched');
              }

              // The target comment is the last one in the chain.
              const targetComment = chain[chain.length - 1];

              // If the comment belongs to a different phase, redirect to History.
              // Only redirect when phaseId is known (defined) — if CommonRoom has no phase
              // context, we can't know whether the comment is "elsewhere", so fall through
              // to the thread modal as before.
              if (phaseId !== undefined && targetComment.phase_id && targetComment.phase_id !== phaseId) {
                logger.debug('Comment is in a different phase, redirecting to History', {
                  commentId: commentIdParam,
                  commentPhaseId: targetComment.phase_id,
                  currentPhaseId: phaseId,
                });
                navigate(`/games/${gameId}?tab=history&phase=${targetComment.phase_id}&comment=${commentIdParam}`, { replace: true });
                return;
              }

              // Store the comment and its context for the modal. root_post_id is the
              // true top-level post even if the chain was trimmed above the target.
              setThreadModalComment(targetComment);
              setThreadModalContext({
                parentChain: chain,
                hasFullThread: has_full_thread,
                targetCommentId: parseInt(commentIdParam),
                postId: root_post_id,
              });

              // Clear the comment parameter from URL
              const newParams = new URLSearchParams(searchParams);
              newParams.delete('comment');
              setSearchParams(newParams, { replace: true });
            } catch (_err) {
              logger.error('Failed to fetch comment', { error: _err, commentId: commentIdParam, gameId });
              // If fetch fails, clear the comment parameter and show error
              const newParams = new URLSearchParams(searchParams);
              newParams.delete('comment');
              setSearchParams(newParams, { replace: true });
              setError('Failed to load comment. The comment may have been deleted.');
            } finally {
              setFetchingComment(false);
            }
          };

          fetchAndShowComment();
        }
      }, 100); // Shorter timeout - DOM should be ready since loading=false

      return () => clearTimeout(timer);
    });

    // Cleanup: Reset scroll attempt tracking on unmount or when comment changes
    return () => {
      scrollAttemptedRef.current = null;
    };
  }, [commentIdParam, loading, searchParams, setSearchParams, gameId, navigate, activeTab, phaseId]);

  const loadData = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);

      const postsResponse = await apiClient.messages.getGamePosts(gameId, { phase_id: phaseId, limit: 50, offset: 0 });
      setPosts(postsResponse.data);
    } catch (err) {
      logger.error('Failed to load Common Room data', { error: err, gameId, phaseId });
      setError('Failed to load Common Room. Please try again.');
    } finally {
      setLoading(false);
    }
  }, [gameId, phaseId]);

  // Load data when component mounts or dependencies change
  useEffect(() => {
    loadData();
  }, [loadData]);

  const handleCreatePost = async (characterId: number, content: string) => {
    try {
      setIsCreatingPost(true);
      await apiClient.messages.createPost(gameId, {
        character_id: characterId,
        content,
        phase_id: phaseId
      });
      // Reload posts to show the new one
      await loadData();
    } catch (_err) {
      logger.error('Failed to create post', { error: _err, gameId, characterId, phaseId });
      throw new Error('Failed to create post. Please try again.');
    } finally {
      setIsCreatingPost(false);
    }
  };

  const handleCreateComment = async (parentId: number, characterId: number, content: string, rootPostId: number) => {
    try {
      const response = await apiClient.messages.createComment(gameId, parentId, {
        character_id: characterId,
        content,
        phase_id: phaseId,
        root_post_id: rootPostId,
      });
      // Don't reload all posts - let the individual PostCard/ThreadedComment handle the update
      // This prevents jarring full-page reloads when commenting deep in a thread

      // In manual read mode, the backend already marks the author's own comment as read.
      // Call toggle-read so the frontend cache reflects this immediately.
      const newCommentId = response.data?.id;
      if (newCommentId && commentReadMode === 'manual') {
        // Optimistically update the cache so the UI shows the comment as read right away
        queryClient.setQueryData<import('../types/messages').ManualCommentReads[]>(
          ['manualReadCommentIDs', gameId],
          (prev = []) => {
            const existing = prev.find(r => r.post_id === rootPostId);
            if (existing) {
              return prev.map(r =>
                r.post_id === rootPostId
                  ? { ...r, read_comment_ids: [...r.read_comment_ids, newCommentId] }
                  : r
              );
            }
            return [...prev, { post_id: rootPostId, read_comment_ids: [newCommentId] }];
          }
        );
        // Also fire the API call so the backend state and cache refetch stay in sync
        toggleCommentReadMutation.mutate({ gameId, postId: rootPostId, commentId: newCommentId, read: true });
      }
    } catch (_err) {
      logger.error('Failed to create comment', { error: _err, gameId, parentId, characterId });
      throw new Error('Failed to create comment. Please try again.');
    }
  };

  const handlePostUpdated = (updatedPost: Message) => {
    // Update the post in the local state to reflect the edit
    setPosts(prevPosts =>
      prevPosts.map(post =>
        post.id === updatedPost.id ? updatedPost : post
      )
    );
  };

  if (loading) {
    return (
      <div className="flex justify-center items-center py-12">
        <Spinner size="lg" label="Loading Common Room..." />
      </div>
    );
  }

  if (fetchingComment) {
    return (
      <div className="flex justify-center items-center py-12">
        <Spinner size="lg" />
        <p className="ml-3 text-text-secondary">Loading comment...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="space-y-4">
        <Alert variant="danger" title="Error">
          {error}
        </Alert>
        <Button variant="danger" onClick={loadData}>
          Try Again
        </Button>
      </div>
    );
  }

  return (
    <div className="max-w-full" data-testid="common-room-container">
      {/* Sticky header bar — keeps the phase title visible while scrolling a
          long thread. Pins under the global nav (h-16), which is where the
          Utilities button now lives. */}
      <div className="sticky top-16 z-30 -mx-4 mb-4 px-4 py-3 surface-base border-b-2 border-theme-strong shadow-sm">
        <h2 className="text-xl md:text-2xl font-bold text-content-primary truncate">
          Common Room{phaseTitle && ` - ${phaseTitle}`}
        </h2>
      </div>

      <div className="mb-6">
        <p className="text-content-secondary">
          {isCurrentPhase
            ? isGM
              ? 'Create GM posts to share information, updates, and phase details with all players. Players can comment and discuss below your posts.'
              : 'View GM posts and join the discussion. Comment on posts to interact with other players.'
            : 'Historical discussions from this phase. New posts can only be created by the GM in the current phase.'}
        </p>

        {/* Phase Description */}
        {phaseDescription && (
          <Card variant="bordered" padding="sm" className="mt-4">
            <h3 className="text-sm font-semibold text-content-primary mb-2">Phase Description</h3>
            <MarkdownPreview content={phaseDescription} />
          </Card>
        )}
      </div>

      {/* Tab Navigation */}
      <div className="border-b border-border-primary mb-6">
        <nav className="flex space-x-6 md:space-x-8">
          <button
            onClick={() => {
              const newParams = new URLSearchParams(searchParams);
              newParams.set('view', 'posts');
              setSearchParams(newParams, { replace: false });
            }}
            className={`py-3 md:py-2 px-1 border-b-[3px] md:border-b-2 font-semibold md:font-medium text-base md:text-sm transition-colors ${
              activeTab === 'posts'
                ? 'border-accent-primary text-interactive-primary'
                : 'border-transparent text-text-secondary hover:text-text-primary hover:border-border-secondary'
            }`}
          >
            Posts
          </button>
          <button
            onClick={() => {
              const newParams = new URLSearchParams(searchParams);
              newParams.set('view', 'newComments');
              setSearchParams(newParams, { replace: false });
            }}
            className={`py-3 md:py-2 px-1 border-b-[3px] md:border-b-2 font-semibold md:font-medium text-base md:text-sm transition-colors ${
              activeTab === 'newComments'
                ? 'border-accent-primary text-interactive-primary'
                : 'border-transparent text-text-secondary hover:text-text-primary hover:border-border-secondary'
            }`}
          >
            New Comments
          </button>
          <button
            onClick={() => {
              const newParams = new URLSearchParams(searchParams);
              newParams.set('view', 'polls');
              setSearchParams(newParams, { replace: false });
            }}
            className={`py-3 md:py-2 px-1 border-b-[3px] md:border-b-2 font-semibold md:font-medium text-base md:text-sm transition-colors ${
              activeTab === 'polls'
                ? 'border-accent-primary text-interactive-primary'
                : 'border-transparent text-text-secondary hover:text-text-primary hover:border-border-secondary'
            }`}
          >
            Polls {unvotedPollsCount > 0 && !pollsLoading && (
              <span className="ml-1 inline-flex items-center justify-center px-2 py-0.5 text-xs font-medium rounded-full bg-accent-primary text-white">
                {unvotedPollsCount}
              </span>
            )}
          </button>
        </nav>
      </div>

      {/* Recent Results Section - only for current phase and players (not GMs) */}
      {isCurrentPhase && !isGM && previousPhaseResults.shouldShowResults && (
        <RecentResultsSection
          gameId={gameId}
          results={previousPhaseResults.results}
          previousPhaseId={previousPhaseResults.previousPhaseId!}
          previousPhaseTitle={previousPhaseResults.previousPhaseTitle!}
        />
      )}

      {/* Tab Content */}
      {activeTab === 'posts' ? (
        <>
          {/* Draft Post Preview — GM only, pending phase only */}
          {isPendingPhase && draftPost && (
            <div className="border border-dashed border-border-default rounded-lg p-4 mb-4 bg-bg-secondary" data-testid="draft-post-preview">
              <div className="flex items-center gap-2 mb-3">
                <span className="text-xs font-medium uppercase tracking-wide text-content-tertiary bg-bg-tertiary px-2 py-0.5 rounded">
                  DRAFT — not visible to players
                </span>
              </div>
              <MarkdownPreview content={draftPost.content} />
            </div>
          )}

          {/* Create Post Form - only show for current phase and GM */}
          {isCurrentPhase && isGM && (
            <CreatePostForm
              gameId={gameId}
              characters={userCharacters}
              allCharacters={allGameCharacters}
              onSubmit={handleCreatePost}
              isSubmitting={isCreatingPost}
              shouldStartCollapsed={posts.length > 0}
            />
          )}

          {/* Posts Feed */}
          {posts.length === 0 ? (
            <div className="surface-raised border border-theme-default rounded-lg p-8 text-center">
              <svg
                className="mx-auto h-12 w-12 text-content-tertiary mb-3"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z"
                />
              </svg>
              <h3 className="text-lg font-medium text-content-primary mb-1">No posts yet</h3>
              <p className="text-content-secondary">Be the first to start a conversation!</p>
            </div>
          ) : (
            <div className="space-y-4">
              {posts.map((post) => (
                <PostCard
                  key={post.id}
                  post={post}
                  gameId={gameId}
                  characters={allGameCharacters}
                  controllableCharacters={userCharacters}
                  onCreateComment={handleCreateComment}
                  onPostUpdated={handlePostUpdated}
                  currentUserId={currentUserId}
                  data-testid={`post-${post.id}`}
                  readOnly={!isCurrentPhase}
                  allowReadTracking={allowReadTracking}
                />
              ))}
            </div>
          )}
        </>
      ) : activeTab === 'newComments' ? (
        /* New Comments View */
        <NewCommentsView gameId={gameId} />
      ) : (
        /* Polls Tab */
        <Suspense fallback={<div className="flex justify-center py-8"><Spinner size="lg" label="Loading polls..." /></div>}>
          {/* gameState gates PollCard's Delete button — a completed or cancelled
              game is an immutable archive. Omitting it left the button live here. */}
          <PollsTab gameId={gameId} phaseId={phaseId} isGM={isGM} isCurrentPhase={isCurrentPhase} isAudience={isAudience} gameState={gameState} />
        </Suspense>
      )}

      {/* Thread View Modal for deep-linked comments */}
      {threadModalComment && threadModalContext && (
        <ThreadViewModalWithReadTracking
          gameId={gameId}
          postId={threadModalContext.postId}
          comment={threadModalComment}
          characters={allGameCharacters}
          controllableCharacters={userCharacters}
          onClose={() => {
            setThreadModalComment(null);
            setThreadModalContext(null);
          }}
          onCreateReply={handleCreateComment}
          currentUserId={currentUserId}
          parentChain={threadModalContext.parentChain}
          hasFullThread={threadModalContext.hasFullThread}
          targetCommentId={threadModalContext.targetCommentId}
          readOnly={!isCurrentPhase}
          allowReadTracking={allowReadTracking}
        />
      )}

      {/* The Utility Drawer and the character-sheet modal it launches are
          rendered globally by GlobalUtilityDrawer, so they outlive this room. */}
    </div>
  );
}
