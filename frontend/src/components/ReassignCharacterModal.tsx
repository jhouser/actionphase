/**
 * Reassign Character Modal Component
 *
 * Allows GMs to reassign inactive characters to new owners.
 * For now, uses a simple user ID input. Can be enhanced with participant list later.
 */

import { useState } from 'react';
import { Button, Select } from './ui';
import { Modal } from './Modal';
import { useReassignCharacter, useGameParticipants } from '../hooks/usePlayerManagement';
import { useAuth } from '../contexts/AuthContext';
import type { Character } from '../types/characters';
import { logger } from '@/services/LoggingService';

interface ReassignCharacterModalProps {
  character: Character;
  gameId: number;
  isOpen: boolean;
  onClose: () => void;
  onSuccess?: () => void;
}

export function ReassignCharacterModal({
  character,
  gameId,
  isOpen,
  onClose,
  onSuccess,
}: ReassignCharacterModalProps) {
  const { currentUser: user } = useAuth();
  const [selectedUserId, setSelectedUserId] = useState<number | null>(null);
  const [assignToSelf, setAssignToSelf] = useState(false);
  const reassignCharacter = useReassignCharacter();
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
      await reassignCharacter.mutateAsync({
        characterId: character.id,
        newOwnerUserId: targetUserId,
      });
      setSelectedUserId(null);
      setAssignToSelf(false);
      onClose();
      onSuccess?.();
    } catch (error) {
      logger.error('Failed to reassign character', { error, gameId, characterId: character.id, characterName: character.name, targetUserId });
    }
  };

  const handleClose = () => {
    if (!reassignCharacter.isPending) {
      setSelectedUserId(null);
      setAssignToSelf(false);
      reassignCharacter.reset();
      onClose();
    }
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={handleClose}
      title={`Reassign ${character.name}`}
    >
      <form onSubmit={handleSubmit} className="space-y-4">
        <div className="p-4 rounded-lg bg-surface-raised border border-theme-default">
          <h4 className="font-medium text-content-primary mb-2">Character Details</h4>
          <div className="text-sm text-content-secondary space-y-1">
            <p><span className="font-medium">Name:</span> {character.name}</p>
            <p><span className="font-medium">Original Owner:</span> {character.original_owner_username || 'Unknown'}</p>
            <p><span className="font-medium">Type:</span> {character.character_type === 'player_character' ? 'Player Character' : 'NPC'}</p>
          </div>
        </div>

        <div className="p-4 rounded-lg bg-semantic-info-subtle border border-theme-default">
          <p className="text-sm text-content-primary">
            Reassigning this character will make it active again and transfer control to the new owner.
            The original owner information will be preserved for the audit trail.
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
              disabled={reassignCharacter.isPending}
              className="rounded border-theme-default"
            />
            <label htmlFor="assign-to-self" className="text-sm text-content-primary cursor-pointer">
              Assign to myself (make this character an NPC I control)
            </label>
          </div>
        )}

        {!assignToSelf && (
          <Select
            label="New Owner"
            value={selectedUserId?.toString() || ''}
            onChange={(e) => setSelectedUserId(e.target.value ? parseInt(e.target.value, 10) : null)}
            required={!assignToSelf}
            disabled={reassignCharacter.isPending}
            helperText="Select a game participant to assign this character to"
          >
            <option value="">-- Select a participant --</option>
            {participants?.map((p) => (
              <option key={p.id} value={p.user_id}>
                {p.username} ({p.role})
              </option>
            ))}
          </Select>
        )}

        {reassignCharacter.isError && (
          <div className="p-3 rounded-lg bg-semantic-danger-subtle border border-semantic-danger">
            <p className="text-sm text-semantic-danger">
              Failed to reassign character. The user may not exist or may not be in the game.
            </p>
          </div>
        )}

        <div className="flex justify-end gap-3">
          <Button
            type="button"
            variant="secondary"
            onClick={handleClose}
            disabled={reassignCharacter.isPending}
          >
            Cancel
          </Button>
          <Button
            type="submit"
            variant="primary"
            loading={reassignCharacter.isPending}
            disabled={(assignToSelf ? false : !selectedUserId) || reassignCharacter.isPending}
          >
            Reassign Character
          </Button>
        </div>
      </form>
    </Modal>
  );
}
