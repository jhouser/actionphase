import { useState } from 'react';
import { Button } from './ui';
import { Modal } from './Modal';
import { useAddParticipant } from '../hooks/usePlayerManagement';
import { UserSearchSelect, type SelectedUser } from './UserSearchSelect';
import { logger } from '@/services/LoggingService';

interface AddParticipantModalProps {
  gameId: number;
  role: 'player' | 'audience';
  isOpen: boolean;
  onClose: () => void;
  onSuccess?: () => void;
  excludeUserIds?: number[];
}

const CONFIG = {
  player: {
    title: 'Add Player Directly',
    description: 'Adding a player directly bypasses the application process and grants them immediate access to the game.',
    buttonLabel: 'Add Player',
    errorMessage: 'Failed to add player. They may already be in the game, or the user may be invalid.',
  },
  audience: {
    title: 'Add Audience Member Directly',
    description: 'Adding an audience member directly bypasses the application process and grants them immediate audience access.',
    buttonLabel: 'Add Audience Member',
    errorMessage: 'Failed to add audience member. They may already be in the game, or the user may be invalid.',
  },
} as const;

const EMPTY_ARRAY: number[] = [];

export function AddParticipantModal({ gameId, role, isOpen, onClose, onSuccess, excludeUserIds = EMPTY_ARRAY }: AddParticipantModalProps) {
  const [selectedUser, setSelectedUser] = useState<SelectedUser | null>(null);
  const addParticipant = useAddParticipant(gameId, role);
  const config = CONFIG[role];

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedUser) return;

    try {
      await addParticipant.mutateAsync(selectedUser.id);
      setSelectedUser(null);
      onClose();
      onSuccess?.();
    } catch (error) {
      logger.error(`Failed to add ${role}`, { error, gameId, userId: selectedUser.id, username: selectedUser.username });
    }
  };

  const handleClose = () => {
    if (!addParticipant.isPending) {
      setSelectedUser(null);
      addParticipant.reset();
      onClose();
    }
  };

  return (
    <Modal isOpen={isOpen} onClose={handleClose} title={config.title}>
      <form onSubmit={handleSubmit} className="space-y-4">
        <div className="p-4 rounded-lg bg-semantic-info-subtle border border-theme-default">
          <p className="text-sm text-content-primary">{config.description}</p>
        </div>

        <UserSearchSelect
          value={selectedUser}
          onChange={setSelectedUser}
          excludeUserIds={excludeUserIds}
          dropdownId="participant-search-dropdown"
          required
          disabled={addParticipant.isPending}
        />

        {addParticipant.isError && (
          <div className="p-3 rounded-lg bg-semantic-danger-subtle border border-semantic-danger">
            <p className="text-sm text-semantic-danger">{config.errorMessage}</p>
          </div>
        )}

        <div className="flex justify-end gap-3">
          <Button type="button" variant="secondary" onClick={handleClose} disabled={addParticipant.isPending}>
            Cancel
          </Button>
          <Button
            type="submit"
            variant="primary"
            loading={addParticipant.isPending}
            disabled={!selectedUser || addParticipant.isPending}
          >
            {config.buttonLabel}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
