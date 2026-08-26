import { useState, useRef } from 'react';
import { Modal } from './Modal';
import { Button, Input } from './ui';
import { logger } from '@/services/LoggingService';

/**
 * Visual weight of the explanatory banner. Maps to the semantic colour tokens
 * rather than raw Tailwind colours so the banner adapts to dark mode.
 */
type ConfirmTone = 'danger' | 'warning' | 'success';

const TONE_BANNER: Record<ConfirmTone, string> = {
  danger: 'bg-semantic-error/10 border-semantic-error',
  warning: 'bg-semantic-warning/10 border-semantic-warning',
  success: 'bg-semantic-warning/10 border-semantic-warning',
};

interface ConfirmActionDialogProps {
  isOpen: boolean;
  onClose: () => void;
  onConfirm: () => Promise<void>;
  /** Modal heading, e.g. "Complete Game". */
  title: string;
  /** Banner heading, e.g. "⚠️ This action cannot be undone". */
  headline: string;
  /** Lead-in above the bullet list, e.g. "Completing this game will:". */
  intro: string;
  /** Bullets describing the consequences. */
  consequences: React.ReactNode[];
  /** Sentence introducing the subject, e.g. "You are about to complete:". */
  subjectLabel: string;
  /** The thing being acted on — normally the game title. */
  subject: string;
  tone: ConfirmTone;
  /**
   * When set, the confirm button stays disabled until the user types this word
   * (compared case-insensitively). Reserved for irreversible actions where a
   * misclick is unrecoverable.
   */
  requireTypedConfirmation?: string;
  confirmLabel: string;
  /** Label while the promise is in flight, e.g. "Completing...". */
  confirmPendingLabel: string;
  confirmVariant?: 'primary' | 'danger';
  /** Extra classes on the confirm button, for tones Button has no variant for. */
  confirmClassName?: string;
  confirmTestId: string;
  cancelLabel: string;
  cancelTestId: string;
  /**
   * Block backdrop/X dismissal while the confirm promise is in flight. Used by
   * destructive actions whose parent unmounts the page on success.
   */
  lockWhileSubmitting?: boolean;
}

/**
 * ConfirmActionDialog - shared confirmation modal for game state changes.
 *
 * Consolidates what were four near-identical dialogs (Complete, Pause, Cancel,
 * Delete). Each of those had independently drifted on details like whether the
 * cancel button said "Cancel" or "Keep Game" and whether the input reset on
 * close; centralising them means a fix lands everywhere at once.
 *
 * Submission state is owned here rather than passed in: every caller was
 * duplicating the same try/finally around `onConfirm`. Callers that need to
 * drive `isSubmitting` externally (LeaveGame, WithdrawApplication) keep their
 * own components — they also use a different, prose-led layout.
 *
 * `onConfirm` is expected to re-throw on failure; the error is logged and the
 * dialog stays open so the user can retry or cancel.
 */
export function ConfirmActionDialog({
  isOpen,
  onClose,
  onConfirm,
  title,
  headline,
  intro,
  consequences,
  subjectLabel,
  subject,
  tone,
  requireTypedConfirmation,
  confirmLabel,
  confirmPendingLabel,
  confirmVariant = 'primary',
  confirmClassName,
  confirmTestId,
  cancelLabel,
  cancelTestId,
  lockWhileSubmitting = false,
}: ConfirmActionDialogProps) {
  const [confirmText, setConfirmText] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  // Mirrors isSubmitting for the backdrop handler: Modal's onClose fires from an
  // event handler that closes over the render's state, so reading the ref avoids
  // a stale value letting a backdrop click through mid-submit.
  const submittingRef = useRef(false);

  const isConfirmEnabled =
    !requireTypedConfirmation ||
    confirmText.trim().toLowerCase() === requireTypedConfirmation.toLowerCase();

  const handleConfirm = async () => {
    if (!isConfirmEnabled) return;

    try {
      submittingRef.current = true;
      setIsSubmitting(true);
      await onConfirm();
      submittingRef.current = false;
      onClose();
    } catch (error) {
      submittingRef.current = false;
      // The parent surfaces the error to the user (toast); this is for tracing.
      logger.error('Confirmation action failed', { error, title, subject });
    } finally {
      setIsSubmitting(false);
      setConfirmText('');
    }
  };

  const handleClose = () => {
    setConfirmText('');
    onClose();
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={() => {
        if (lockWhileSubmitting && submittingRef.current) return;
        handleClose();
      }}
      title={title}
    >
      <div className="space-y-4">
        <div className={`${TONE_BANNER[tone]} border rounded-lg p-4`}>
          <h3 className="font-semibold text-content-primary mb-2">{headline}</h3>
          <p className="text-content-secondary text-sm">{intro}</p>
          <ul className="list-disc list-inside text-content-secondary text-sm mt-2 space-y-1">
            {consequences.map((item, i) => (
              <li key={i}>{item}</li>
            ))}
          </ul>
        </div>

        <div>
          <p className="text-content-secondary text-sm mb-2">{subjectLabel}</p>
          <p className="font-semibold text-content-primary">{subject}</p>
        </div>

        {requireTypedConfirmation && (
          <div>
            <Input
              label={`Type '${requireTypedConfirmation}' to confirm`}
              type="text"
              value={confirmText}
              onChange={(e) => setConfirmText(e.target.value)}
              placeholder={requireTypedConfirmation}
              autoFocus
              disabled={isSubmitting}
            />
          </div>
        )}

        <div className="flex gap-3 justify-end pt-4">
          <Button
            variant="secondary"
            onClick={handleClose}
            disabled={isSubmitting}
            data-testid={cancelTestId}
          >
            {cancelLabel}
          </Button>
          <Button
            variant={confirmVariant}
            onClick={handleConfirm}
            disabled={!isConfirmEnabled || isSubmitting}
            loading={isSubmitting}
            className={confirmClassName}
            data-testid={confirmTestId}
          >
            {isSubmitting ? confirmPendingLabel : confirmLabel}
          </Button>
        </div>
      </div>
    </Modal>
  );
}
