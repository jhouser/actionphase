import { Modal } from './Modal';
import { Button } from './ui';

interface ConfirmModalProps {
  isOpen: boolean;
  onClose: () => void;
  onConfirm: () => void | Promise<void>;
  title: string;
  message: string;
  confirmText?: string;
  cancelText?: string;
  variant?: 'danger' | 'warning' | 'primary';
  isLoading?: boolean;
  /**
   * Overrides the panel's `data-testid`, default `confirm-modal`.
   *
   * Rarely needed — one confirmation is up at a time. Pass it when a test has
   * to tell two apart on the same screen.
   */
  testId?: string;
}

/**
 * ConfirmModal - Reusable confirmation dialog
 *
 * Replaces browser confirm() dialogs with a consistent,
 * theme-aware modal component.
 *
 * @example
 * ```tsx
 * const [showConfirm, setShowConfirm] = useState(false);
 *
 * <ConfirmModal
 *   isOpen={showConfirm}
 *   onClose={() => setShowConfirm(false)}
 *   onConfirm={handleDelete}
 *   title="Delete Comment"
 *   message="Are you sure you want to delete this comment? This action cannot be undone."
 *   confirmText="Delete"
 *   variant="danger"
 * />
 * ```
 */
export const ConfirmModal = ({
  isOpen,
  onClose,
  onConfirm,
  title,
  message,
  confirmText = 'Confirm',
  cancelText = 'Cancel',
  variant = 'primary',
  isLoading = false,
  testId = 'confirm-modal',
}: ConfirmModalProps) => {
  const handleConfirm = async () => {
    await onConfirm();
    onClose();
  };

  return (
    <Modal isOpen={isOpen} onClose={onClose} title={title} testId={testId}>
      <div className="space-y-4">
        <p className="text-content-primary" data-testid="confirm-modal-message">
          {message}
        </p>

        {/* Fixed testids rather than role+name lookups. The confirm button is
            usually labelled for the action it completes -- "Delete" -- which is
            the SAME label as the row button that opened this dialog, so a
            `getByRole('button', { name: 'Delete' })` matches both and a test
            cannot say which one it meant. */}
        <div className="flex justify-end gap-3">
          <Button
            variant="secondary"
            onClick={onClose}
            disabled={isLoading}
            data-testid="confirm-modal-cancel"
          >
            {cancelText}
          </Button>
          <Button
            variant={variant}
            onClick={handleConfirm}
            loading={isLoading}
            disabled={isLoading}
            data-testid="confirm-modal-confirm"
          >
            {confirmText}
          </Button>
        </div>
      </div>
    </Modal>
  );
};
