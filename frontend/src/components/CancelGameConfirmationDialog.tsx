import { ConfirmActionDialog } from './ConfirmActionDialog';

interface CancelGameConfirmationDialogProps {
  isOpen: boolean;
  onClose: () => void;
  onConfirm: () => Promise<void>;
  gameTitle: string;
}

/**
 * CancelGameConfirmationDialog - Confirmation dialog for cancelling a game
 *
 * Cancelling a game:
 * - Permanently archives the game
 * - Cannot be undone or resumed
 * - Only available during setup/recruitment states
 */
export function CancelGameConfirmationDialog({
  isOpen,
  onClose,
  onConfirm,
  gameTitle,
}: CancelGameConfirmationDialogProps) {
  return (
    <ConfirmActionDialog
      isOpen={isOpen}
      onClose={onClose}
      onConfirm={onConfirm}
      title="Cancel Game"
      headline="⚠️ Permanent Action"
      intro="Cancelling this game will:"
      consequences={[
        'Permanently archive the game',
        'Prevent any further gameplay',
        'Mark the game as cancelled (cannot be undone)',
      ]}
      subjectLabel="You are about to cancel:"
      subject={gameTitle}
      tone="danger"
      confirmLabel="Cancel Game"
      confirmPendingLabel="Cancelling..."
      confirmVariant="danger"
      confirmTestId="cancel-game-confirm-button"
      cancelLabel="Keep Game"
      cancelTestId="cancel-game-keep-button"
    />
  );
}
