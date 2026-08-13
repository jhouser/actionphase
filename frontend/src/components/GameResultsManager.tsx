import { useState } from 'react';
import { useGameActionResults, useUpdateActionResult, usePublishActionResult, useDeleteActionResult, useCancelPendingStagedPart, useAppendStagedPart, useUpdateStagedPartDelay } from '../hooks/useActionResults';
import type { ActionResult, GamePhase } from '../types/phases';
import { Button, Badge, Alert, Select } from './ui';
import { AppendStagedPartForm } from './AppendStagedPartForm';
import { DELAY_PRESETS, formatDelayLabel, isPresetDelay } from '../lib/stagedDelays';
import { UpdateCharacterSheetModal } from './UpdateCharacterSheetModal';
import { PublishResultConfirmationDialog } from './PublishResultConfirmationDialog';
import { ConfirmModal } from './ConfirmModal';
import { MarkdownPreview } from './MarkdownPreview';
import { CommentEditor } from './CommentEditor';
import { useDraftUpdateCount } from '../hooks';
import { useConflictingSheetDrafts } from '../hooks/useConflictingSheetDrafts';
import { logger } from '@/services/LoggingService';
import { useToast } from '../contexts/ToastContext';

interface GameResultsManagerProps {
  gameId: number;
  currentPhase?: GamePhase | null;
  className?: string;
}

export function GameResultsManager({ gameId, currentPhase, className = '' }: GameResultsManagerProps) {
  const { data: results, isLoading } = useGameActionResults(gameId);
  const [editingResultId, setEditingResultId] = useState<number | null>(null);

  if (isLoading) {
    return (
      <div className={`surface-base rounded-lg border border-theme-default p-6 ${className}`}>
        <div className="animate-pulse">
          <div className="h-6 surface-sunken rounded mb-4 w-1/3"></div>
          <div className="space-y-3">
            {[...Array(3)].map((_, i) => (
              <div key={i} className="h-24 surface-sunken rounded"></div>
            ))}
          </div>
        </div>
      </div>
    );
  }

  // Filter results to only show those from the current phase (if provided).
  //
  // Newest first, unlike everywhere else results are shown: this is the composing
  // view, so a GM who just wrote a result needs it at the top to confirm it landed
  // and to edit it. GetGameResults returns oldest-first to keep the History tab
  // chronological for every role, so the order is flipped back here rather than in
  // SQL — the two views share one endpoint and want opposite orders.
  //
  // Sorting a copy: `results` is React Query's cached array and sort() mutates.
  const allResults = [...(results || [])]
    .filter(r => !currentPhase?.id || r.phase_id === currentPhase.id)
    .sort((a, b) => b.id - a.id);
  const unpublishedResults = allResults.filter(r => !r.is_published);
  const publishedResults = allResults.filter(r => r.is_published);

  if (allResults.length === 0) {
    return (
      <div className={`surface-base rounded-lg border border-theme-default p-6 ${className}`}>
        <h2 className="text-xl font-semibold text-content-primary mb-2">Action Results</h2>
        <p className="text-sm text-content-secondary">No results have been created yet.</p>
      </div>
    );
  }

  return (
    <div className={`surface-base rounded-lg border border-theme-default ${className}`}>
      <div className="p-6">
        <div className="flex items-center justify-between mb-6">
          <div>
            <h2 className="text-xl font-semibold text-content-primary">Action Results</h2>
            <p className="text-sm text-content-secondary mt-1">
              Manage results sent to players
            </p>
          </div>
          <div className="flex items-center space-x-2">
            <Badge variant="warning">
              {unpublishedResults.length} Unpublished
            </Badge>
            <Badge variant="success">
              {publishedResults.length} Published
            </Badge>
          </div>
        </div>

        {/* Unpublished Results Section */}
        {unpublishedResults.length > 0 && (
          <div className="mb-6" data-testid="unpublished-results-section">
            <h3 className="text-lg font-semibold text-content-primary mb-3 flex items-center">
              <svg className="w-5 h-5 text-semantic-warning mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
              Unpublished Results (Editable)
            </h3>
            <div className="space-y-3">
              {unpublishedResults.map((result) => (
                <ResultCard
                  key={result.id}
                  result={result}
                  gameId={gameId}
                  isEditing={editingResultId === result.id}
                  onStartEdit={() => setEditingResultId(result.id)}
                  onCancelEdit={() => setEditingResultId(null)}
                  phaseResults={allResults}
                />
              ))}
            </div>
          </div>
        )}

        {/* Published Results Section */}
        {publishedResults.length > 0 && (
          <div data-testid="published-results-section">
            <h3 className="text-lg font-semibold text-content-primary mb-3 flex items-center">
              <svg className="w-5 h-5 text-semantic-success mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              Published Results
            </h3>
            <div className="space-y-3">
              {publishedResults.map((result) => (
                <ResultCard
                  key={result.id}
                  result={result}
                  gameId={gameId}
                  isEditing={false}
                  onStartEdit={() => {}}
                  onCancelEdit={() => {}}
                  phaseResults={allResults}
                />
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

interface ResultCardProps {
  result: ActionResult;
  gameId: number;
  isEditing: boolean;
  onStartEdit: () => void;
  onCancelEdit: () => void;
  /** All results in this phase, used to detect conflicting sheet drafts on siblings. */
  phaseResults: ActionResult[];
}

function ResultCard({ result, gameId, isEditing, onStartEdit, onCancelEdit, phaseResults }: ResultCardProps) {
  const [editedContent, setEditedContent] = useState(result.content);
  const [isExpanded, setIsExpanded] = useState(false);
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [isPublishDialogOpen, setIsPublishDialogOpen] = useState(false);
  const [isDeleteConfirmOpen, setIsDeleteConfirmOpen] = useState(false);
  const [isCancelPartConfirmOpen, setIsCancelPartConfirmOpen] = useState(false);
  const [publishSuccess, setPublishSuccess] = useState(false);
  const updateMutation = useUpdateActionResult(gameId);
  const publishMutation = usePublishActionResult(gameId);
  const deleteMutation = useDeleteActionResult(gameId);
  const cancelPartMutation = useCancelPendingStagedPart(gameId);
  const appendPartMutation = useAppendStagedPart(gameId);
  const updateDelayMutation = useUpdateStagedPartDelay(gameId);
  const [isAppendingPart, setIsAppendingPart] = useState(false);
  const { data: draftCount, isPending: isDraftCountPending } = useDraftUpdateCount(gameId, result.id);
  const { hasConflict } = useConflictingSheetDrafts(gameId, result, phaseResults);
  const { showError } = useToast();

  // A clobber needs two competing snapshots, so warn only when THIS result also has
  // staged updates — a result with none can neither overwrite nor be overwritten.
  // Published results are excluded too: their drafts are already applied and deleted.
  //
  // `draftCount` is undefined while the query is in flight. Treating that as "no
  // drafts" lets a GM who clicks straight through reach the publish confirmation
  // without ever seeing the warning, so the unresolved case counts as a possible
  // conflict and the dialogs below hold their confirm until it settles.
  const hasStagedOrUnknown = isDraftCountPending || (draftCount ?? 0) > 0;
  const showSheetConflictWarning =
    !result.is_published && hasConflict && hasStagedOrUnknown;

  // Determine if content should be collapsible (long results)
  const isCollapsible = result.content.length > 200;
  const previewContent = result.content.substring(0, 200) + '...';

  // Staged chain status. The GM always sees every part's real content — only
  // the recipient's copy is blanked — so this is purely a schedule readout.
  //
  // A part is cancellable only while it is published-but-unreleased. An
  // unpublished chain is deleted through the ordinary Delete control, and a
  // released part cannot be recalled.
  const isStagedPart = result.part_count !== undefined && result.part_count > 1;
  // Cancel removes an unreleased part and cascades the parts behind it. Guarded
  // on release, not publication, matching DeleteStagedPart's SQL: a draft
  // follower is equally removable, and it is the *only* removal control such a
  // part gets, since Delete is a chain-head action (see showLifecycleControls).
  const isPendingPart = isStagedPart && result.part_number !== 1 && !result.released_at;

  // A chain member that is not the head. Its lifecycle belongs to the chain,
  // not to itself: it is published with the head, released by the worker, and
  // removed with Cancel rather than Delete.
  const isFollowerPart = isStagedPart && result.part_number !== 1;

  // A follow-up can be added only while the whole chain is still a draft: the
  // chain must be complete before it goes out, since appending afterwards would
  // extend a scene the player has already started reading. The server enforces
  // this too (409); this just avoids offering a control that would fail.
  //
  // Offered on the chain TAIL only — the part the new one will actually follow.
  // A new part always lands at the end, so a button on part 1 of a 3-part chain
  // would read as "add a follow-up to this part" while silently appending after
  // part 3. An unstaged draft is a chain of one and is therefore its own tail,
  // which is how a GM stages a result they saved earlier.
  const isChainTail = !isStagedPart || result.part_number === result.part_count;
  const canAppendPart = !result.is_published && isChainTail;

  // Publishing is a chain-level action — PublishActionResult publishes the
  // whole chain — so a follower must not carry its own Publish button. Offering
  // one per part implies each can go out separately, which is exactly what the
  // feature prevents. The same goes for Delete: removing a middle part is
  // Cancel's job (it cascades correctly); Delete on a follower would strand or
  // silently cascade the parts behind it.
  const showLifecycleControls = !result.is_published && !isFollowerPart;

  // Character-sheet updates go on the chain TAIL, not the head, for two reasons.
  //
  // Narrative: drafts apply when the chain is published, so hanging them off an
  // early part would grant the reward as the scene opens — the player sees the
  // loot before reading whether they survived to earn it. The tail is the beat
  // the outcome belongs to.
  //
  // Structural: every part of a chain shares one recipient, so sheet drafts on
  // two parts are precisely the clobber useConflictingSheetDrafts warns about
  // ("all sheet updates for a character in a phase belong in exactly ONE
  // result"). One control per chain makes that impossible rather than merely
  // discouraged.
  const showSheetUpdateControl = !result.is_published && isChainTail;

  // Retiming is guarded on release, not publication: the most common reason to
  // change a delay is that the scene is already running and the players need
  // longer. A head has no parent to wait for, so it has no delay to edit.
  const canEditDelay = isStagedPart && !result.released_at && result.part_number !== 1;

  const handleCancelPart = async () => {
    try {
      await cancelPartMutation.mutateAsync(result.id);
    } catch (error) {
      logger.error('Failed to cancel pending staged part', { error, resultId: result.id, gameId });
      showError('Failed to cancel this part. Please try again.');
    }
  };

  const handleAppendPart = async (part: { content: string; delay_minutes: number }) => {
    try {
      await appendPartMutation.mutateAsync({ resultId: result.id, part });
      setIsAppendingPart(false);
    } catch (error) {
      logger.error('Failed to append staged part', { error, resultId: result.id, gameId });
      showError('Failed to add this part. Please try again.');
    }
  };

  const handleDelayChange = async (delayMinutes: number) => {
    try {
      await updateDelayMutation.mutateAsync({ resultId: result.id, delayMinutes });
    } catch (error) {
      logger.error('Failed to update staged part delay', { error, resultId: result.id, gameId });
      showError('Failed to change this timer. Please try again.');
    }
  };

  const handleSave = async () => {
    if (editedContent.trim() === result.content) {
      onCancelEdit();
      return;
    }

    try {
      await updateMutation.mutateAsync({
        resultId: result.id,
        content: editedContent.trim(),
      });
      onCancelEdit();
    } catch (error) {
      logger.error('Failed to update result', { error, resultId: result.id, gameId });
    }
  };

  const handleCancel = () => {
    setEditedContent(result.content);
    onCancelEdit();
  };

  const handlePublish = async () => {
    try {
      await publishMutation.mutateAsync(result.id);
      setPublishSuccess(true);
      setIsPublishDialogOpen(false);

      // Hide success message after 5 seconds
      setTimeout(() => setPublishSuccess(false), 5000);
    } catch (error) {
      logger.error('Failed to publish result', { error, resultId: result.id, gameId });
    }
  };

  const handleDelete = async () => {
    try {
      await deleteMutation.mutateAsync(result.id);
      setIsDeleteConfirmOpen(false);
    } catch (error) {
      logger.error('Failed to delete result', { error, resultId: result.id, gameId });
      showError('Failed to delete result. Please try again.');
    }
  };

  return (
    <div className={`border rounded-lg overflow-hidden ${result.is_published ? 'border-semantic-success bg-semantic-success-subtle' : 'border-semantic-warning bg-semantic-warning-subtle'}`}>
      <div className="p-4">
        <div className="flex items-center justify-between mb-3">
          <div className="flex items-center space-x-3">
            <div className="flex-shrink-0">
              <div className={`w-10 h-10 rounded-full flex items-center justify-center ${result.is_published ? 'bg-semantic-success-subtle' : 'bg-semantic-warning-subtle'}`}>
                <svg className={`w-5 h-5 ${result.is_published ? 'text-semantic-success' : 'text-semantic-warning'}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
                </svg>
              </div>
            </div>
            <div>
              <h4 className="font-medium text-content-primary">
                To: {result.character_name ? `${result.character_name} (${result.username})` : (result.username || `User #${result.user_id}`)}
              </h4>
              <div className="flex items-center space-x-2 text-xs text-content-secondary mt-0.5">
                {result.phase_type && result.phase_number && (
                  <>
                    <span>Phase {result.phase_number}</span>
                    <span>•</span>
                  </>
                )}
                {result.is_published && result.sent_at && (
                  <span>Sent: {new Date(result.sent_at).toLocaleString()}</span>
                )}
                {!result.is_published && (
                  <span className="font-medium text-semantic-warning">Draft</span>
                )}
              </div>
              {isStagedPart && (
                <div
                  className="flex items-center space-x-2 mt-1"
                  data-testid={`staged-status-${result.id}`}
                >
                  <Badge variant="primary">
                    Part {result.part_number} of {result.part_count}
                  </Badge>
                  {result.released_at ? (
                    <span className="text-xs text-content-secondary">
                      Revealed {new Date(result.released_at).toLocaleString()}
                    </span>
                  ) : result.unlocks_at ? (
                    <span className="text-xs text-content-secondary">
                      Reveals {new Date(result.unlocks_at).toLocaleString()}
                    </span>
                  ) : (
                    <span className="text-xs text-content-secondary">
                      Reveals after the previous part
                    </span>
                  )}
                  {/* Retiming stays available on a published pending part, not
                      just a draft: the usual reason to move a timer is that the
                      scene is live and the players need longer. A custom delay
                      that is not one of the presets is shown as its own option
                      so the selector never misrepresents the stored value. */}
                  {canEditDelay && (
                    <Select
                      aria-label={`Delay before part ${result.part_number}`}
                      selectSize="sm"
                      value={String(result.reveal_delay_minutes ?? '')}
                      onChange={e => handleDelayChange(Number(e.target.value))}
                      disabled={updateDelayMutation.isPending}
                      data-testid={`edit-staged-delay-${result.id}`}
                    >
                      {/* An empty placeholder when the delay is unknown, so a
                          missing field reads as "unknown" instead of quietly
                          rendering the first preset as though it were the real
                          setting. The server always sends it; this exists so
                          the failure is visible if that ever regresses. */}
                      {result.reveal_delay_minutes === undefined && (
                        <option value="">Delay unavailable</option>
                      )}
                      {result.reveal_delay_minutes !== undefined
                        && !isPresetDelay(result.reveal_delay_minutes) && (
                        <option value={result.reveal_delay_minutes}>
                          {formatDelayLabel(result.reveal_delay_minutes)}
                        </option>
                      )}
                      {DELAY_PRESETS.map(minutes => (
                        <option key={minutes} value={minutes}>
                          {formatDelayLabel(minutes)}
                        </option>
                      ))}
                    </Select>
                  )}
                </div>
              )}
            </div>
          </div>
          {!result.is_published && !isEditing && (
            <div className="flex gap-2">
              {/* Delete and Publish belong to the chain as a whole, so they
                  live on the head. Character-sheet updates go on the TAIL
                  instead — they apply at publish, and the reward belongs to the
                  beat that earns it. Every part keeps Edit, since its text is
                  its own. */}
              {showSheetUpdateControl && (
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => setIsModalOpen(true)}
                >
                  Update Character Sheet
                  {draftCount !== undefined && draftCount > 0 && (
                    <Badge variant="warning" className="ml-2" data-testid={`draft-update-count-${result.id}`}>{draftCount}</Badge>
                  )}
                </Button>
              )}
              <Button
                variant="primary"
                size="sm"
                onClick={onStartEdit}
              >
                Edit
              </Button>
              {canAppendPart && !isAppendingPart && (
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => setIsAppendingPart(true)}
                  data-testid={`add-staged-part-${result.id}`}
                  data-faro-user-action-name="add-staged-result-part"
                >
                  + Add a timed follow-up
                </Button>
              )}
              {showLifecycleControls && (
                <Button
                  variant="danger"
                  size="sm"
                  onClick={() => setIsDeleteConfirmOpen(true)}
                  disabled={deleteMutation.isPending}
                  data-testid={`delete-result-${result.id}`}
                >
                  Delete
                </Button>
              )}
              {showLifecycleControls && (
                <Button
                  variant="primary"
                  size="sm"
                  onClick={() => setIsPublishDialogOpen(true)}
                  disabled={publishMutation.isPending}
                  data-testid={`publish-result-${result.id}`}
                >
                  {publishMutation.isPending
                    ? 'Publishing...'
                    : isStagedPart
                      ? `Publish Chain (${result.part_count} parts)`
                      : 'Publish Result'}
                </Button>
              )}
            </div>
          )}
          {isPendingPart && (
            <Button
              variant="danger"
              size="sm"
              onClick={() => setIsCancelPartConfirmOpen(true)}
              disabled={cancelPartMutation.isPending}
              data-testid={`cancel-staged-part-${result.id}`}
            >
              {cancelPartMutation.isPending ? 'Cancelling...' : 'Cancel This Part'}
            </Button>
          )}
        </div>

        {/* danger, not warning: the draft card is already a field of yellow (Draft
            label, update-count badge, section header), so a warning-toned alert blends
            in — and the consequence here is silent data loss, not caution. */}
        {showSheetConflictWarning && (
          <Alert variant="danger" className="mt-3" data-testid={`sheet-conflict-warning-${result.id}`}>
            Another unpublished result for this character also has staged character
            sheet updates. Publishing both will overwrite the earlier one — keep all
            of this phase&apos;s sheet updates in a single result.
          </Alert>
        )}

        {isEditing ? (
          <div className="space-y-3">
            <CommentEditor
              value={editedContent}
              onChange={setEditedContent}
              rows={6}
              placeholder="Enter result content..."
              disabled={updateMutation.isPending}
              maxLength={100000}
              showCharacterCount
            />
            <div className="flex justify-end space-x-2">
              <Button
                variant="ghost"
                size="sm"
                onClick={handleCancel}
                disabled={updateMutation.isPending}
              >
                Cancel
              </Button>
              <Button
                variant="primary"
                size="sm"
                onClick={handleSave}
                disabled={updateMutation.isPending || !editedContent.trim()}
              >
                {updateMutation.isPending ? 'Saving...' : 'Save Changes'}
              </Button>
            </div>
            {updateMutation.isError && (
              <p className="text-sm text-semantic-danger">
                Failed to update result. Please try again.
              </p>
            )}
          </div>
        ) : (
          <>
            <div className="surface-base p-4 rounded border border-theme-default">
              <MarkdownPreview
                content={isCollapsible && !isExpanded ? previewContent : result.content}
                mentionedCharacters={[]}
                fullWidth
              />
            </div>
            {/* Below the content, so the GM writes the follow-up while reading
                the beat it follows.

                part_count is the whole chain's length on every member, so
                +1 names the appended part correctly no matter which card the
                GM clicked — the server always appends to the tail, not after
                the anchor. It is absent on an unstaged draft, which is a chain
                of one, making the next part number 2. */}
            {isAppendingPart && (
              <AppendStagedPartForm
                resultId={result.id}
                nextPartNumber={(result.part_count ?? 1) + 1}
                onSubmit={handleAppendPart}
                onCancel={() => setIsAppendingPart(false)}
                isPending={appendPartMutation.isPending}
                isError={appendPartMutation.isError}
              />
            )}
            {isCollapsible && (
              <button
                onClick={() => setIsExpanded(!isExpanded)}
                className="mt-2 text-sm text-interactive-primary hover:text-interactive-primary-hover font-medium flex items-center"
              >
                {isExpanded ? (
                  <>
                    <svg className="w-4 h-4 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 15l7-7 7 7" />
                    </svg>
                    Show less
                  </>
                ) : (
                  <>
                    <svg className="w-4 h-4 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                    </svg>
                    Show full content
                  </>
                )}
              </button>
            )}
          </>
        )}

        {/* Success Notification */}
        {publishSuccess && (
          <Alert
            variant="success"
            title="Result Published Successfully!"
            dismissible
            onDismiss={() => setPublishSuccess(false)}
            className="mt-3"
          >
            The action result{draftCount !== undefined && draftCount > 0 ? ` and ${draftCount} character sheet update${draftCount !== 1 ? 's' : ''}` : ''} has been published and is now visible to the player.
          </Alert>
        )}

        {/* Update Character Sheet Modal */}
        <UpdateCharacterSheetModal
          isOpen={isModalOpen}
          onClose={() => setIsModalOpen(false)}
          gameId={gameId}
          actionResultId={result.id}
          characterId={result.character_id || result.user_id}
          characterName={result.character_name || result.username || `User #${result.user_id}`}
        />

        {/* Publish Confirmation Dialog */}
        <PublishResultConfirmationDialog
          isOpen={isPublishDialogOpen}
          onConfirm={handlePublish}
          onCancel={() => setIsPublishDialogOpen(false)}
          gameId={gameId}
          actionResultId={result.id}
          isPublishing={publishMutation.isPending}
          hasConflictingSheetDrafts={showSheetConflictWarning}
        />

        {/* Delete Confirmation Modal */}
        <ConfirmModal
          isOpen={isDeleteConfirmOpen}
          onClose={() => setIsDeleteConfirmOpen(false)}
          onConfirm={handleDelete}
          title="Delete Draft Result"
          message={`This will permanently delete the draft result for ${result.character_name ? `${result.character_name} (${result.username})` : (result.username || `User #${result.user_id}`)}, including any pending character sheet updates.`}
          confirmText="Yes, Delete Draft"
          variant="danger"
          isLoading={deleteMutation.isPending}
        />

        {/* Cancel Pending Part Confirmation */}
        <ConfirmModal
          isOpen={isCancelPartConfirmOpen}
          onClose={() => setIsCancelPartConfirmOpen(false)}
          onConfirm={async () => {
            await handleCancelPart();
            setIsCancelPartConfirmOpen(false);
          }}
          title="Cancel Pending Part"
          message={`Part ${result.part_number} has not been revealed yet. Cancelling removes it permanently — the player will see the chain end at the previous part.`}
          confirmText="Yes, Cancel This Part"
          variant="danger"
          isLoading={cancelPartMutation.isPending}
        />
      </div>
    </div>
  );
}
