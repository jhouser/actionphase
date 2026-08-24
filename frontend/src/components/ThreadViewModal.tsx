import { useState, useEffect, useCallback, useRef } from 'react';
import { createPortal } from 'react-dom';
import { ThreadedComment } from './ThreadedComment';
import type { Message } from '../types/messages';
import type { Character } from '../types/characters';
import { Button } from './ui';
import { UtilitiesButton } from './UtilitiesButton';
import { THREAD_VIEW_MAX_DEPTH } from '../config/comments';
import { LAYERS } from '../config/layers';
import { useBodyScrollLock } from '../hooks/useBodyScrollLock';
import { useDirtyChildren } from '../hooks/useDirtyChildren';

interface ThreadViewModalProps {
  gameId: number;
  postId: number; // The root post ID
  comment: Message; // Pass the comment object directly instead of just ID
  characters: Character[];
  controllableCharacters: Character[];
  onClose: () => void;
  onCreateReply: (parentId: number, characterId: number, content: string, rootPostId: number) => Promise<void>;
  currentUserId?: number;
  unreadCommentIDs?: number[];
  manualReadCommentIDs?: number[];
  commentReadMode?: 'auto' | 'manual';
  onToggleRead?: (commentId: number, currentlyRead: boolean) => void;
  // New props for parent chain context (deep-link enhancement)
  parentChain?: Message[]; // Array of parent messages (oldest → target)
  hasFullThread?: boolean; // Whether we fetched all the way to root
  targetCommentId?: number; // ID of the originally requested comment to highlight
  readOnly?: boolean; // Disable all interactive features (for history view)
  allowReadTracking?: boolean; // Show faded read state and toggle button (default true)
}

/**
 * Modal view for deeply nested comment threads
 * Shows the comment with its replies without navigating away from Common Room
 * Prevents accidental read-marking when users explore deep threads
 */
export function ThreadViewModal({
  gameId,
  postId,
  comment,
  characters,
  controllableCharacters,
  onClose,
  onCreateReply,
  currentUserId,
  unreadCommentIDs = [],
  manualReadCommentIDs = [],
  commentReadMode = 'auto',
  onToggleRead,
  parentChain,
  hasFullThread = true,
  targetCommentId,
  readOnly = false,
  allowReadTracking = true,
}: ThreadViewModalProps) {
  // State for nested modal (modal-within-modal for deeply nested threads)
  const [nestedModalComment, setNestedModalComment] = useState<Message | null>(null);
  // Track where mousedown originated so a drag that ends on the backdrop doesn't close the modal.
  // Firefox on Windows synthesizes a click on the backdrop when mouseup lands there after a drag
  // that started inside the modal (e.g. resizing the CommentEditor). We only close on a "true"
  // backdrop click where both mousedown and mouseup occurred on the backdrop itself.
  const backdropMouseDownTarget = useRef<EventTarget | null>(null);
  const [showDiscardConfirm, setShowDiscardConfirm] = useState(false);
  // Any comment in the thread may hold a pending reply, so dirty state is tracked per
  // comment id rather than as a single boolean — otherwise one comment's "clean" report
  // would erase another's "dirty".
  const { isAnyDirty: hasDirtyReply, report } = useDirtyChildren();

  // ThreadedComment reports (commentId, isDirty); useDirtyChildren keys on strings.
  // Wrapped rather than inlined because ThreadedComment renders itself recursively and
  // passes this straight down, so an unstable identity would churn every descendant.
  const handleDirtyStateChange = useCallback(
    (commentId: number, isDirty: boolean) => report(String(commentId), isDirty),
    [report],
  );

  const handleClose = useCallback(() => {
    if (hasDirtyReply) {
      setShowDiscardConfirm(true);
    } else {
      onClose();
    }
  }, [hasDirtyReply, onClose]);

  // Determine if we're showing parent chain context or single comment
  const showingContext = parentChain && parentChain.length > 1;

  // Lock background scroll while modal is open. Ref-counted, so the nested
  // thread views this component renders (and the drawer opening over it) each
  // hold a lock and the page unlocks only when the last one closes.
  useBodyScrollLock();

  // Auto-scroll to target comment when modal opens
  useEffect(() => {
    if (targetCommentId && showingContext) {
      // Wait for DOM to render, then scroll to target
      const timer = setTimeout(() => {
        // Try to find comment with various ID patterns (base, -desktop, -mobile)
        // Root comments use base ID, nested comments may have -desktop/-mobile suffix
        const baseEl = document.getElementById(`comment-${targetCommentId}`);
        const desktopEl = document.getElementById(`comment-${targetCommentId}-desktop`);
        const mobileEl = document.getElementById(`comment-${targetCommentId}-mobile`);
        // Prefer the visible element so scrollIntoView works (hidden elements don't scroll)
        const element = [baseEl, mobileEl, desktopEl].find(
          el => el && el.offsetParent !== null
        ) || baseEl || desktopEl || mobileEl;
        if (element) {
          element.scrollIntoView({ behavior: 'smooth', block: 'center' });
        }
      }, 100);
      return () => clearTimeout(timer);
    }
  }, [targetCommentId, showingContext]);

  // Strip children property from comment to force ThreadedComment to load fresh replies
  // Comments from main view have pre-loaded children with maxDepth=5, but in thread view we want THREAD_VIEW_MAX_DEPTH
  const stripChildren = (msg: Message): Message => {
    const { _children, ...rest } = msg as Message & { _children?: unknown };
    return rest;
  };

  return (
    <>
      <div
        className={`fixed inset-0 bg-black/60 backdrop-blur-sm ${LAYERS.modal} flex items-center justify-center p-4`}
        onMouseDown={(e) => { backdropMouseDownTarget.current = e.target; }}
        onClick={(e) => {
          // Matches the character sheet (UpdateCharacterSheetModal's dismissOnBackdrop):
          // once there is something to lose, a backdrop click does nothing at all. A click
          // out here is usually a slip rather than a decision — nothing was aimed at — and
          // answering a slip with a confirm dialog makes the user dismiss a question they
          // never asked. The X still closes, and still confirms.
          if (hasDirtyReply) return;
          if (backdropMouseDownTarget.current === e.currentTarget) handleClose();
        }}
      >
        <div
          className="surface-base rounded-lg shadow-xl max-w-7xl w-full max-h-[90vh] overflow-y-auto overscroll-contain"
          onClick={(e) => e.stopPropagation()}
        >
          {/* Header */}
          {/* Padding is inline because the mobile-only `[class*="surface-"] { padding: 5px }`
              rule in index.css is unlayered, so it overrides Tailwind's padding utilities
              here and squashes the header against the modal's left edge. */}
          <div
            className="sticky top-0 surface-base border-b border-theme-default z-10"
            style={{ paddingLeft: '1.5rem', paddingRight: '1.5rem', paddingTop: '0.75rem', paddingBottom: '0.5rem' }}
          >
            <div className="flex items-center justify-between">
              <h2 className="text-xl font-bold text-content-primary">Thread View</h2>
              <div className="flex items-center gap-3">
                {/* Reach the dice roller / character sheet without leaving the
                    thread — the nav's copy is behind this modal's blur. */}
                <UtilitiesButton />
              <Button
                variant="ghost"
                size="sm"
                onClick={handleClose}
                aria-label="Close thread view"
                className="text-content-tertiary hover:text-content-secondary h-auto p-0"
              >
                <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                </svg>
              </Button>
              </div>
            </div>

            {/* Context info */}
            {showingContext && (
              <div className="text-sm">
                <p className="text-content-secondary">
                  Showing {parentChain.length} {parentChain.length === 1 ? 'message' : 'messages'}
                  {!hasFullThread && ' (partial context)'}
                </p>
              </div>
            )}
          </div>

          {/* Content */}
          <div className="px-6 py-6">
            {showingContext ? (
              /* Render parent chain as nested structure */
              (() => {
                // Separate parents from target
                const parents = parentChain.slice(0, -1);
                const target = parentChain[parentChain.length - 1];

                // Reconstruct parents as nested structure, with target as the deepest child
                // Use reduceRight to build from deepest to shallowest
                const reconstructedRoot = parents.reduceRight((child, parent) => {
                  return { ...parent, children: [child] };
                }, stripChildren(target));

                // Render the root parent as ThreadedComment, which cascades down to target
                // The auto-scroll effect will handle highlighting the target
                return (
                  <ThreadedComment
                    comment={reconstructedRoot}
                    gameId={gameId}
                    postId={postId}
                    characters={characters}
                    controllableCharacters={controllableCharacters}
                    onCreateReply={onCreateReply}
                    onCommentDeleted={onClose}
                    currentUserId={currentUserId}
                    depth={0}
                    maxDepth={THREAD_VIEW_MAX_DEPTH}
                    unreadCommentIDs={unreadCommentIDs}
                    manualReadCommentIDs={manualReadCommentIDs}
                    commentReadMode={commentReadMode}
                    onToggleRead={onToggleRead}
                    onOpenThread={(nestedComment) => setNestedModalComment(nestedComment)}
                    readOnly={readOnly}
                    allowReadTracking={allowReadTracking}
                    onDirtyStateChange={handleDirtyStateChange}
                  />
                );
              })()
            ) : (
              /* Single comment view (original behavior) */
              <ThreadedComment
                comment={stripChildren(comment)}
                gameId={gameId}
                postId={postId}
                characters={characters}
                controllableCharacters={controllableCharacters}
                onCreateReply={onCreateReply}
                onCommentDeleted={onClose}
                currentUserId={currentUserId}
                readOnly={readOnly}
                allowReadTracking={allowReadTracking}
                depth={0}
                maxDepth={10}
                unreadCommentIDs={unreadCommentIDs}
                manualReadCommentIDs={manualReadCommentIDs}
                commentReadMode={commentReadMode}
                onToggleRead={onToggleRead}
                onOpenThread={(nestedComment) => setNestedModalComment(nestedComment)}
                onDirtyStateChange={handleDirtyStateChange}
              />
            )}
          </div>
        </div>
      </div>

      {/* Nested Modal - Recursively render another ThreadViewModal if user clicks "Continue thread" in this modal */}
      {nestedModalComment && (
        <ThreadViewModal
          gameId={gameId}
          postId={postId} // Pass through the root post ID
          comment={nestedModalComment}
          characters={characters}
          controllableCharacters={controllableCharacters}
          onClose={() => setNestedModalComment(null)}
          onCreateReply={onCreateReply}
          currentUserId={currentUserId}
          unreadCommentIDs={unreadCommentIDs}
          manualReadCommentIDs={manualReadCommentIDs}
          commentReadMode={commentReadMode}
          onToggleRead={onToggleRead}
          readOnly={readOnly}
          allowReadTracking={allowReadTracking}
        />
      )}

      {/* Discard confirmation — portaled to document.body so it escapes any parent stacking context */}
      {showDiscardConfirm && createPortal(
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/40">
          <div
            className="surface-raised rounded-lg shadow-xl border border-theme-default max-w-sm w-full p-6"
            onClick={(e) => e.stopPropagation()}
          >
            <h3 className="text-lg font-semibold text-content-primary mb-2">Discard unsaved reply?</h3>
            <p className="text-content-secondary text-sm mb-6">
              You have unsaved text in the reply editor. If you close this thread, your reply will be lost.
            </p>
            <div className="flex justify-end gap-3">
              <Button variant="secondary" onClick={() => setShowDiscardConfirm(false)}>
                Keep editing
              </Button>
              <Button variant="danger" onClick={onClose}>
                Discard
              </Button>
            </div>
          </div>
        </div>,
        document.body
      )}
    </>
  );
}
