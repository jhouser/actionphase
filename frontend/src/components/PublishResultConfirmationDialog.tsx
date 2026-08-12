import { Modal } from './Modal';
import { Button, Alert } from './ui';
import { useDraftCharacterUpdates } from '../hooks';

interface PublishResultConfirmationDialogProps {
  isOpen: boolean;
  onConfirm: () => void;
  onCancel: () => void;
  gameId: number;
  actionResultId: number;
  isPublishing?: boolean;
  /**
   * True when another unpublished result for the same character also has staged
   * sheet updates, meaning publishing both will overwrite the earlier snapshot.
   */
  hasConflictingSheetDrafts?: boolean;
}

export const PublishResultConfirmationDialog: React.FC<PublishResultConfirmationDialogProps> = ({
  isOpen,
  onConfirm,
  onCancel,
  gameId,
  actionResultId,
  isPublishing = false,
  hasConflictingSheetDrafts = false,
}) => {
  const { data: drafts, isPending: isDraftsPending } = useDraftCharacterUpdates(gameId, actionResultId);
  const draftCount = drafts?.length || 0;

  // Publishing is irreversible and the draft-count notice below starts undefined, so a
  // fast click could confirm before it rendered. Hold the button until it settles.
  //
  // This does NOT gate the conflict warning: that comes from `hasConflictingSheetDrafts`,
  // computed by the parent from a different query. It needs no gate — the parent's
  // sibling-count hook already reports pending counts as a conflict, so the warning
  // fails safe (shown, possibly spuriously) rather than silently absent.
  const isCheckingDrafts = isOpen && isDraftsPending;

  return (
    <Modal isOpen={isOpen} onClose={onCancel} title="Publish Action Result?">
      <div className="space-y-4">
        <p className="text-content-primary">
          This will publish the action result and make it visible to the player.
        </p>

        {draftCount > 0 && (
          <Alert variant="warning">
            This will also publish {draftCount} character sheet update{draftCount !== 1 ? 's' : ''} to the player&apos;s character.
          </Alert>
        )}

        {hasConflictingSheetDrafts && (
          <Alert variant="danger" data-testid="publish-sheet-conflict-warning">
            Another unpublished result for this character also has staged sheet
            updates. Each result stores a complete snapshot of the character sheet,
            so publishing both will overwrite one set of changes with the other.
            Move all of this phase&apos;s sheet updates into a single result first.
          </Alert>
        )}

        <p className="text-sm text-content-secondary">
          This action cannot be undone. The result will be visible immediately.
        </p>

        <div className="flex justify-end space-x-3 pt-4">
          <Button
            variant="secondary"
            onClick={onCancel}
            disabled={isPublishing}
          >
            Cancel
          </Button>
          <Button
            variant="primary"
            onClick={onConfirm}
            disabled={isPublishing || isCheckingDrafts}
          >
            {isPublishing ? 'Publishing...' : 'Publish'}
          </Button>
        </div>
      </div>
    </Modal>
  );
};
