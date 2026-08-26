import { ConfirmActionDialog } from './ConfirmActionDialog';

interface EpilogueGameConfirmationDialogProps {
  isOpen: boolean;
  onClose: () => void;
  onConfirm: () => Promise<void>;
  gameTitle: string;
}

/**
 * EpilogueGameConfirmationDialog - Confirmation dialog for moving a game to epilogue.
 *
 * Requires the GM to type "epilogue", matching the completed dialog. The typing
 * gate matters more here than the wording suggests: from in_progress the GM sees
 * two endgame options side by side, and this one is the less obviously final of
 * the two while being just as irreversible in the way that counts — it discloses
 * the entire game and there is no transition back.
 *
 * Moving to epilogue:
 * - Reveals the whole archive (private messages, action submissions, poll votes)
 * - Keeps the game WRITABLE, unlike completing it
 * - Cannot be undone; the only way out is to complete or cancel
 */
export function EpilogueGameConfirmationDialog({
  isOpen,
  onClose,
  onConfirm,
  gameTitle,
}: EpilogueGameConfirmationDialogProps) {
  return (
    <ConfirmActionDialog
      isOpen={isOpen}
      onClose={onClose}
      onConfirm={onConfirm}
      title="Move to Epilogue"
      headline="⚠️ This reveals everything, and cannot be undone"
      intro="Moving this game to Epilogue will:"
      consequences={[
        'Reveal the entire game to everyone — private messages, action submissions, and poll votes become readable by all players',
        'Keep the game writable, so you can post epilogue and meta-discussion threads and players can reply',
        'Reveal player identities if this is an anonymous game',
      ]}
      subjectLabel="You are about to move to epilogue:"
      subject={gameTitle}
      tone="warning"
      requireTypedConfirmation="epilogue"
      confirmLabel="Move to Epilogue"
      confirmPendingLabel="Moving to epilogue..."
      confirmVariant="primary"
      confirmTestId="epilogue-game-confirm-button"
      cancelLabel="Cancel"
      cancelTestId="epilogue-game-cancel-button"
    />
  );
}
