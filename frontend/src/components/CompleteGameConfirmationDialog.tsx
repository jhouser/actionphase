import { ConfirmActionDialog } from './ConfirmActionDialog';

interface CompleteGameConfirmationDialogProps {
  isOpen: boolean;
  onClose: () => void;
  onConfirm: () => Promise<void>;
  gameTitle: string;
}

/**
 * CompleteGameConfirmationDialog - Confirmation dialog for completing a game
 *
 * Requires the GM to type "completed" to prevent accidental game completion.
 * Once a game is completed:
 * - It becomes a read-only public archive
 * - No new content can be created
 * - Anyone can view the game's history
 */
export function CompleteGameConfirmationDialog({
  isOpen,
  onClose,
  onConfirm,
  gameTitle,
}: CompleteGameConfirmationDialogProps) {
  return (
    <ConfirmActionDialog
      isOpen={isOpen}
      onClose={onClose}
      onConfirm={onConfirm}
      title="Complete Game"
      headline="⚠️ This action cannot be undone"
      intro="Completing this game will:"
      consequences={[
        'Make the game read-only (no new posts, actions, or content)',
        'Make it publicly viewable as an archive (anyone can read it)',
        'Prevent any further state changes',
      ]}
      subjectLabel="You are about to complete:"
      subject={gameTitle}
      tone="success"
      requireTypedConfirmation="completed"
      confirmLabel="Complete Game"
      confirmPendingLabel="Completing..."
      confirmVariant="primary"
      confirmClassName="bg-semantic-success hover:bg-semantic-success-hover text-white"
      confirmTestId="complete-game-confirm-button"
      cancelLabel="Cancel"
      cancelTestId="complete-game-cancel-button"
    />
  );
}
