import { Modal } from './Modal';
import { Button } from './ui';
import { logger } from '@/services/LoggingService';

interface LeaveGameConfirmationDialogProps {
  isOpen: boolean;
  onClose: () => void;
  onConfirm: () => Promise<void>;
  gameTitle: string;
  isSubmitting: boolean;
}

/**
 * LeaveGameConfirmationDialog - Confirmation dialog for leaving a game
 *
 * Warns the player that leaving will:
 * - Remove them from the game
 * - Optionally mark their characters as inactive (if any exist)
 */
export function LeaveGameConfirmationDialog({
  isOpen,
  onClose,
  onConfirm,
  gameTitle,
  isSubmitting,
}: LeaveGameConfirmationDialogProps) {
  const handleConfirm = async () => {
    try {
      await onConfirm();
      onClose();
    } catch (error) {
      // Error handling is done by the parent component
      logger.error('Failed to leave game', { error, gameTitle });
    }
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Leave Game"
    >
      <div className="space-y-4">
        <p className="text-content-primary">
          Are you sure you want to leave <strong>{gameTitle}</strong>?
        </p>

        <div className="surface-raised border border-semantic-warning rounded-md p-4">
          <p className="text-sm text-content-secondary">
            <strong>Warning:</strong> Leaving this game will:
          </p>
          <ul className="list-disc list-inside text-sm text-content-secondary mt-2 space-y-1">
            <li>Remove you from the participants list</li>
            <li>Mark any of your characters as inactive</li>
            <li>Remove your access to game content</li>
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
          >
            {isSubmitting ? 'Leaving...' : 'Leave Game'}
          </Button>
        </div>
      </div>
    </Modal>
  );
}
