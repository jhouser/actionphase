import { useState } from 'react';
import { Card, Button, Badge, Alert, Spinner } from './ui';
import { MarkdownPreview } from './MarkdownPreview';
import { CommentEditor } from './CommentEditor';
import { ConfirmModal } from './ConfirmModal';
import { useHandoutComments } from '../hooks/useHandoutComments';
import type { Handout } from '../types/handouts';
import { logger } from '@/services/LoggingService';

interface HandoutViewProps {
  gameId: number;
  handout: Handout;
  isGM: boolean;
  onClose: () => void;
  onEdit?: () => void;
  /**
   * Whether to render the "Back to Handouts" link and the handout's own title
   * heading. On by default, for the Handouts tab, where this view *is* the page
   * and going back really does return you to the list.
   *
   * The Utility Drawer opens this inside a Modal, which supplies the standard
   * title bar and X. There the back-link is a second, differently-shaped way to
   * dismiss and the heading is a duplicate of the one above it, so the drawer
   * turns both off.
   */
  showHeader?: boolean;
  /**
   * Suppresses every way of changing anything — posting an update, editing or
   * deleting one, and the Edit button — regardless of `isGM`.
   *
   * For the Utility Drawer, which is reference material you pull up without
   * leaving what you were writing. Editing a handout mid-action-submission is
   * not what the drawer is for, and offering only *some* of the write actions
   * there (updates but not the handout itself) is worse than offering none.
   * `isGM` still selects the GM's wording, which stays correct for a GM reading
   * their own updates.
   */
  readOnly?: boolean;
}

export function HandoutView({
  gameId,
  handout,
  isGM,
  onClose,
  onEdit,
  showHeader = true,
  readOnly = false,
}: HandoutViewProps) {
  const {
    comments,
    isLoading: commentsLoading,
    createCommentMutation,
    updateCommentMutation,
    deleteCommentMutation,
  } = useHandoutComments(gameId, handout.id);

  const [newNoteContent, setNewNoteContent] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [editingCommentId, setEditingCommentId] = useState<number | null>(null);
  const [editContent, setEditContent] = useState('');
  const [deleteCommentId, setDeleteCommentId] = useState<number | null>(null);

  const statusBadgeVariant = handout.status === 'published' ? 'success' : 'warning';

  const handleSubmitNote = async () => {
    if (!newNoteContent.trim()) return;

    setIsSubmitting(true);
    try {
      await createCommentMutation.mutateAsync({ content: newNoteContent });
      setNewNoteContent(''); // Clear the form on success
    } catch (error) {
      logger.error('Failed to create GM note', { error, gameId, handoutId: handout.id, handoutTitle: handout.title });
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleStartEdit = (commentId: number, currentContent: string) => {
    setEditingCommentId(commentId);
    setEditContent(currentContent);
  };

  const handleCancelEdit = () => {
    setEditingCommentId(null);
    setEditContent('');
  };

  const handleSaveEdit = async (commentId: number) => {
    if (!editContent.trim()) return;

    try {
      await updateCommentMutation.mutateAsync({
        commentId,
        data: { content: editContent }
      });
      setEditingCommentId(null);
      setEditContent('');
    } catch (error) {
      logger.error('Failed to update comment', { error, gameId, handoutId: handout.id, commentId });
    }
  };

  const handleDeleteComment = async () => {
    if (!deleteCommentId) return;

    try {
      await deleteCommentMutation.mutateAsync(deleteCommentId);
      setDeleteCommentId(null);
    } catch (error) {
      logger.error('Failed to delete comment', { error, gameId, handoutId: handout.id, commentId: deleteCommentId });
    }
  };

  // Filter out deleted comments
  const visibleComments = comments.filter(comment => !comment.deleted_at);

  // Every write affordance keys off this rather than isGM directly, so a
  // read-only surface can't grow a new one by accident.
  const canWrite = isGM && !readOnly;

  return (
    <div className="space-y-6">
      <Card variant="elevated" padding="lg">
        {/* Hidden entirely when neither control would render, so the modal
            doesn't carry an empty row's worth of margin above the content. */}
        {(showHeader || (canWrite && onEdit)) && (
          <div className="flex items-center justify-between mb-4">
            {showHeader ? (
              <Button
                variant="ghost"
                size="sm"
                onClick={onClose}
                className="flex items-center gap-2"
              >
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 19l-7-7m0 0l7-7m-7 7h18" />
                </svg>
                Back to Handouts
              </Button>
            ) : (
              // Keeps the Edit button pinned right under justify-between.
              <span />
            )}

            {canWrite && onEdit && (
              <Button
                variant="secondary"
                size="sm"
                onClick={onEdit}
              >
                Edit
              </Button>
            )}
          </div>
        )}

        <div className="space-y-4">
          <div>
            <div className="flex items-center gap-3 mb-2">
              {/* Dropped in the modal, where the Modal's own title bar already
                  names the handout. The badge stays either way — draft vs
                  published is information the title bar doesn't carry. */}
              {showHeader && (
                <h1 className="text-3xl font-bold text-content-primary">{handout.title}</h1>
              )}
              <Badge variant={statusBadgeVariant}>
                {handout.status === 'published' ? 'Published' : 'Draft'}
              </Badge>
            </div>

            {handout.updated_at && handout.created_at && handout.updated_at !== handout.created_at && (
              <p className="text-sm text-content-tertiary">
                Edited {new Date(handout.updated_at).toLocaleString()}
              </p>
            )}
          </div>

          <div className="border-t border-border-primary pt-4">
            <MarkdownPreview content={handout.content} fullWidth />
          </div>
        </div>
      </Card>

      {/* Comments/Updates section - Public to all players */}
      <Card variant="elevated" padding="lg">
        <h2 className="text-xl font-bold text-content-primary mb-4">Updates & Comments</h2>
        <p className="text-sm text-content-secondary mb-4">
          {canWrite
            ? "Add updates, clarifications, or additional information about this handout. All players can see these comments."
            : "GM updates and additional information about this handout."}
        </p>

        {/* New Comment Form - GM Only, and never in read-only mode */}
        {canWrite && (
          <div className="mb-6 space-y-3">
            <CommentEditor
              value={newNoteContent}
              onChange={setNewNoteContent}
              placeholder="Add an update or clarification about this handout..."
              disabled={isSubmitting}
              rows={4}
              warnOnUnsavedChanges
            />
            <div className="flex justify-end gap-2">
              <Button
                variant="secondary"
                size="sm"
                onClick={() => setNewNoteContent('')}
                disabled={!newNoteContent.trim() || isSubmitting}
              >
                Clear
              </Button>
              <Button
                variant="primary"
                size="sm"
                onClick={handleSubmitNote}
                disabled={!newNoteContent.trim() || isSubmitting}
                loading={isSubmitting}
              >
                Post Update
              </Button>
            </div>
          </div>
        )}

        {/* Existing Comments */}
        <div className={isGM ? "border-t border-border-primary pt-4" : ""}>
          {isGM && <h3 className="text-sm font-semibold text-content-secondary mb-3">Previous Updates</h3>}
          {commentsLoading ? (
            <div className="flex justify-center py-8">
              <Spinner size="md" label="Loading updates..." />
            </div>
          ) : visibleComments.length === 0 ? (
            <Alert variant="info">
              {/* "yet" implies you could add one, so it follows write ability
                  rather than role — a GM reading in the drawer cannot. */}
              {canWrite ? "No updates posted yet." : "No additional information available for this handout."}
            </Alert>
          ) : (
            <div className="space-y-4">
              {visibleComments.map(comment => (
                <div key={comment.id} className="border border-border-primary rounded-lg p-4 bg-bg-secondary">
                  {/* Edit Mode */}
                  {editingCommentId === comment.id ? (
                    <div className="space-y-3">
                      <CommentEditor
                        value={editContent}
                        onChange={setEditContent}
                        placeholder="Edit your update..."
                        disabled={updateCommentMutation.isPending}
                        rows={4}
                      />
                      <div className="flex justify-end gap-2">
                        <Button
                          variant="secondary"
                          size="sm"
                          onClick={handleCancelEdit}
                          disabled={updateCommentMutation.isPending}
                        >
                          Cancel
                        </Button>
                        <Button
                          variant="primary"
                          size="sm"
                          onClick={() => handleSaveEdit(comment.id)}
                          disabled={!editContent.trim() || updateCommentMutation.isPending}
                          loading={updateCommentMutation.isPending}
                        >
                          Save
                        </Button>
                      </div>
                    </div>
                  ) : (
                    <>
                      {/* View Mode */}
                      <MarkdownPreview content={comment.content} />
                      <div className="flex items-center justify-between mt-2">
                        <div className="flex items-center gap-2">
                          <p className="text-sm text-content-tertiary">
                            {comment.edit_count > 0 && comment.updated_at
                              ? `Edited ${new Date(comment.updated_at).toLocaleString()}`
                              : new Date(comment.created_at || '').toLocaleString()
                            }
                          </p>
                        </div>
                        {/* Edit/Delete Buttons - GM Only, and never in read-only mode */}
                        {canWrite && (
                          <div className="flex gap-2">
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => handleStartEdit(comment.id, comment.content)}
                            >
                              Edit
                            </Button>
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => setDeleteCommentId(comment.id)}
                              className="text-semantic-danger"
                            >
                              Delete
                            </Button>
                          </div>
                        )}
                      </div>
                    </>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      </Card>

      {/* Delete Confirmation Modal */}
      <ConfirmModal
        isOpen={deleteCommentId !== null}
        onClose={() => setDeleteCommentId(null)}
        onConfirm={handleDeleteComment}
        title="Delete Update"
        message="Are you sure you want to delete this update? This action cannot be undone."
        confirmText="Delete"
        variant="danger"
        isLoading={deleteCommentMutation.isPending}
      />
    </div>
  );
}
