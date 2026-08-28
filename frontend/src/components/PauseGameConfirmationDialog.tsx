import { ConfirmActionDialog } from './ConfirmActionDialog';

interface PauseGameConfirmationDialogProps {
  isOpen: boolean;
  onClose: () => void;
  onConfirm: () => Promise<void>;
  gameTitle: string;
}

/**
 * PauseGameConfirmationDialog - Confirmation dialog for pausing a game
 *
 * Pausing a game:
 * - Temporarily stops gameplay
 * - Can be resumed at any time
 * - Useful for breaks or waiting for players
 */
export function PauseGameConfirmationDialog({
  isOpen,
  onClose,
  onConfirm,
  gameTitle,
}: PauseGameConfirmationDialogProps) {
  return (
    <ConfirmActionDialog
      isOpen={isOpen}
      onClose={onClose}
      onConfirm={onConfirm}
      title="Pause Game"
      headline="⏸️ Pause Gameplay"
      intro="Pausing this game will:"
      consequences={[
        'Temporarily stop active gameplay',
        'Prevent phase transitions and new actions',
        'Allow you to resume at any time',
      ]}
      subjectLabel="You are about to pause:"
      subject={gameTitle}
      tone="warning"
      confirmLabel="Pause Game"
      confirmPendingLabel="Pausing..."
      confirmVariant="primary"
      confirmClassName="bg-semantic-warning hover:bg-semantic-warning-hover text-white"
      confirmTestId="pause-game-confirm-button"
      cancelLabel="Cancel"
      cancelTestId="pause-game-cancel-button"
    />
  );
}
