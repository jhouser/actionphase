/**
 * Assign NPC Modal Component
 *
 * Allows GMs to assign NPCs to audience members or other players.
 */

import { useState } from 'react';
import { Button, Select } from './ui';
import { Modal } from './Modal';
import { useAssignNPC } from '../hooks/useCharacters';
import { useGameParticipants } from '../hooks/usePlayerManagement';
import { useAuth } from '../contexts/AuthContext';
import type { Character } from '../types/characters';
import { logger } from '@/services/LoggingService';

interface AssignNPCModalProps {
  character: Character;
  gameId: number;
  isOpen: boolean;
  onClose: () => void;
  onSuccess?: () => void;
}

export function AssignNPCModal({
  character,
  gameId,
  isOpen,
  onClose,
  onSuccess,
}: AssignNPCModalProps) {
  const { currentUser: user } = useAuth();
  const [selectedUserId, setSelectedUserId] = useState<number | null>(null);
  const [assignToSelf, setAssignToSelf] = useState(false);
  const assignNPC = useAssignNPC();
  const { data: participants } = useGameParticipants(gameId);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    const targetUserId = assignToSelf && user?.id
      ? user.id
      : selectedUserId;

    if (!targetUserId || targetUserId <= 0) {
      return;
    }

    try {
      await assignNPC.mutateAsync({
        characterId: character.id,
        assignedUserId: targetUserId,
        gameId: gameId,
      });
      setSelectedUserId(null);
      setAssignToSelf(false);
      onClose();
      onSuccess?.();
    } catch (error) {
      logger.error('Failed to assign NPC', { error, gameId, characterId: character.id, characterName: character.name, targetUserId });
    }
  };

  const handleClose = () => {
    if (!assignNPC.isPending) {
      setSelectedUserId(null);
      setAssignToSelf(false);
      assignNPC.reset();
      onClose();
    }
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={handleClose}
      title={`Assign ${character.name}`}
    >
      <form onSubmit={handleSubmit} className="space-y-4">
        <div className="p-4 rounded-lg bg-surface-raised border border-theme-default">
          <h4 className="font-medium text-content-primary mb-2">NPC Details</h4>
          <div className="text-sm text-content-secondary space-y-1">
            <p><span className="font-medium">Name:</span> {character.name}</p>
            <p><span className="font-medium">Type:</span> NPC</p>
          </div>
        </div>

        <div className="p-4 rounded-lg bg-semantic-info-subtle border border-theme-default">
          <p className="text-sm text-content-primary">
            NPCs can be assigned to audience members, allowing them to control this character in the game.
            They will be able to participate in conversations as this character.
          </p>
        </div>

        {user && (
          <div className="flex items-center gap-2">
            <input
              type="checkbox"
              id="assign-to-self"
              checked={assignToSelf}
              onChange={(e) => {
                setAssignToSelf(e.target.checked);
                if (e.target.checked) {
                  setSelectedUserId(null);
                }
              }}
              disabled={assignNPC.isPending}
              className="rounded border-theme-default"
            />
            <label htmlFor="assign-to-self" className="text-sm text-content-primary cursor-pointer">
              Assign to myself (take back control of this NPC)
            </label>
          </div>
        )}

        {!assignToSelf && (
          <Select
            label="Assign to Audience Member"
            value={selectedUserId?.toString() || ''}
            onChange={(e) => setSelectedUserId(e.target.value ? parseInt(e.target.value, 10) : null)}
            required={!assignToSelf}
            disabled={assignNPC.isPending}
            helperText="Select an audience member to assign this NPC to"
          >
            <option value="">-- Select an audience member --</option>
            {participants?.filter((p) => p.role === 'audience').map((p) => (
              <option key={p.id} value={p.user_id}>
                {p.username}
              </option>
            ))}
          </Select>
        )}

        {assignNPC.isError && (
          <div className="p-3 rounded-lg bg-semantic-danger-subtle border border-semantic-danger">
            <p className="text-sm text-semantic-danger">
              Failed to assign NPC. The user may not exist or may not be in the game.
            </p>
          </div>
        )}

        <div className="flex justify-end gap-3">
          <Button
            type="button"
            variant="secondary"
            onClick={handleClose}
            disabled={assignNPC.isPending}
          >
            Cancel
          </Button>
          <Button
            type="submit"
            variant="primary"
            loading={assignNPC.isPending}
            disabled={(assignToSelf ? false : !selectedUserId) || assignNPC.isPending}
          >
            Assign NPC
          </Button>
        </div>
      </form>
    </Modal>
  );
}
