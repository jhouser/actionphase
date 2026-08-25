import { ConfirmActionDialog } from './ConfirmActionDialog';

interface DeleteGameConfirmationDialogProps {
  isOpen: boolean;
  onClose: () => void;
  onConfirm: () => Promise<void>;
  gameTitle: string;
}

/**
 * DeleteGameConfirmationDialog - Confirmation dialog for deleting a cancelled game
 *
 * Deleting a game:
 * - Permanently removes the game from the system
 * - Cannot be undone
 * - Only available for games in cancelled state
 * - Only available to the Game Master
 *
 * Dismissal is locked while the delete is in flight: on success the parent
 * navigates away, and a backdrop click mid-request would race that.
 */
export function DeleteGameConfirmationDialog({
  isOpen,
  onClose,
  onConfirm,
  gameTitle,
}: DeleteGameConfirmationDialogProps) {
  return (
    <ConfirmActionDialog
      isOpen={isOpen}
      onClose={onClose}
      onConfirm={onConfirm}
      title="Delete Game"
      headline="⚠️ Permanent Deletion"
      intro="Deleting this game will:"
      consequences={[
        'Permanently remove all game data',
        'Delete all associated characters and content',
        'Cannot be recovered or undone',
      ]}
      subjectLabel="You are about to permanently delete:"
      subject={gameTitle}
      tone="danger"
      confirmLabel="Delete Game"
      confirmPendingLabel="Deleting..."
      confirmVariant="danger"
      confirmTestId="delete-game-confirm-button"
      cancelLabel="Keep Game"
      cancelTestId="delete-game-cancel-button"
      lockWhileSubmitting
    />
  );
}
