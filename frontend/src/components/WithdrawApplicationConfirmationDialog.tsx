import { Modal } from './Modal';
import { Button } from './ui';
import { logger } from '@/services/LoggingService';

interface WithdrawApplicationConfirmationDialogProps {
  isOpen: boolean;
  onClose: () => void;
  onConfirm: () => Promise<void>;
  gameTitle: string;
  isSubmitting: boolean;
  role: 'player' | 'audience';
}

/**
 * WithdrawApplicationConfirmationDialog - Confirmation dialog for withdrawing a game application
 *
 * Warns the user that withdrawing will remove their application from consideration
 */
export function WithdrawApplicationConfirmationDialog({
  isOpen,
  onClose,
  onConfirm,
  gameTitle,
  isSubmitting,
  role,
}: WithdrawApplicationConfirmationDialogProps) {
  const handleConfirm = async () => {
    try {
      await onConfirm();
      onClose();
    } catch (error) {
      // Error handling is done by the parent component
      logger.error('Failed to withdraw application', { error, gameTitle, role });
    }
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Withdraw Application"
    >
      <div className="space-y-4">
        <p className="text-content-primary">
          Are you sure you want to withdraw your {role} application for <strong>{gameTitle}</strong>?
        </p>

        <div className="surface-raised border border-semantic-warning rounded-md p-4">
          <p className="text-sm text-content-secondary">
            <strong>Note:</strong> Withdrawing your application will:
          </p>
          <ul className="list-disc list-inside text-sm text-content-secondary mt-2 space-y-1">
            <li>Remove your application from the GM's review queue</li>
            <li>Allow you to submit a new application if you change your mind</li>
            {role === 'player' && (
              <li>You can only apply again while recruitment is open</li>
            )}
          </ul>
        </div>

        <div className="flex justify-end gap-3 pt-2">
          <Button
            variant="ghost"
            onClick={onClose}
            disabled={isSubmitting}
          >
            Cancel
          </Button>
          <Button
            variant="danger"
            onClick={handleConfirm}
            disabled={isSubmitting}
            loading={isSubmitting}
          >
            Withdraw Application
          </Button>
        </div>
      </div>
    </Modal>
  );
}
