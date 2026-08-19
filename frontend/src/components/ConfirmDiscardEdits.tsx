import { Button } from './ui';

interface ConfirmDiscardEditsProps {
  /** Proceed with the close, abandoning the open editor's uncommitted text. */
  onDiscard: () => void;
  /** Dismiss the prompt and leave the editor as it was. */
  onKeepEditing: () => void;
  /**
   * Hide the explanatory text below the `sm` breakpoint. For bars that sit in a
   * cramped header, where the two buttons already carry the meaning.
   */
  hideTextOnMobile?: boolean;
}

/**
 * Confirmation shown when closing a surface that has an editor open with edits its own
 * Save has not committed.
 *
 * Those edits live in the child editor's local state and never reach the surface being
 * closed, so closing discards them outright with nothing to undo. Shared by the
 * character sheet and the update modal so both ask in the same words — the prompt is
 * the same question, and phrasing drift between them reads as two different features.
 */
export function ConfirmDiscardEdits({
  onDiscard,
  onKeepEditing,
  hideTextOnMobile = false,
}: ConfirmDiscardEditsProps) {
  return (
    <div className="flex items-center gap-2 flex-shrink-0" data-testid="confirm-close-unsaved">
      <span
        className={`text-sm text-content-secondary ${hideTextOnMobile ? 'hidden sm:inline' : ''}`}
      >
        Unsaved edits open. Close anyway?
      </span>
      {/* type="button" because this bar can render inside a <form> (the game
          create/edit modals). The shared Button sets no default type, so without
          it these act as submit buttons and closing would also fire the form. */}
      <Button
        type="button"
        variant="danger"
        size="sm"
        onClick={onDiscard}
        data-testid="confirm-close-discard"
      >
        Close without saving
      </Button>
      <Button type="button" variant="secondary" size="sm" onClick={onKeepEditing}>
        Keep editing
      </Button>
    </div>
  );
}
