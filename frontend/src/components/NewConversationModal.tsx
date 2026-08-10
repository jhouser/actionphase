import { useState, useEffect } from 'react';
import { apiClient } from '../lib/api';
import type { Character } from '../types/characters';
import { Button, Input, Select, Checkbox, Alert } from './ui';
import { Modal } from './Modal';
import { logger } from '@/services/LoggingService';

interface NewConversationModalProps {
  gameId: number;
  characters: Character[]; // User's controllable characters
  allCharacters: Character[]; // All game characters (from GameContext)
  isAnonymous: boolean;
  allowGroupConversations: boolean;
  /**
   * Character IDs to pre-select as participants, e.g. when the modal was opened
   * from the envelope shortcut on a character sheet or profile. Only the first
   * is used when the game disallows group conversations.
   */
  initialParticipantIds?: number[];
  onClose: () => void;
  onConversationCreated: (conversationId: number) => void;
}

export function NewConversationModal({ gameId, characters, allCharacters, isAnonymous, allowGroupConversations, initialParticipantIds, onClose, onConversationCreated }: NewConversationModalProps) {
  const [title, setTitle] = useState('');
  const [yourCharacterId, setYourCharacterId] = useState<number | null>(null);
  const [selectedParticipants, setSelectedParticipants] = useState<Set<number>>(
    () => new Set(initialParticipantIds ?? [])
  );
  const [singleParticipant, setSingleParticipant] = useState<number | null>(
    () => initialParticipantIds?.[0] ?? null
  );
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Debug logging
  logger.debug('NewConversationModal state', {
    gameId,
    controllableCharactersCount: characters.length,
    allCharactersCount: allCharacters.length,
    yourCharacterId,
  });

  useEffect(() => {
    // Auto-select if user only has one character
    if (characters.length === 1 && !yourCharacterId) {
      setYourCharacterId(characters[0].id);
    }
  }, [characters, yourCharacterId]);

  // A pre-selected participant that turns out to be the sending character would
  // be submitted twice; drop it. The list already hides it, so without this the
  // stale selection would be invisible but still sent.
  useEffect(() => {
    if (yourCharacterId === null) return;
    setSelectedParticipants((prev) => {
      if (!prev.has(yourCharacterId)) return prev;
      const next = new Set(prev);
      next.delete(yourCharacterId);
      return next;
    });
    setSingleParticipant((prev) => (prev === yourCharacterId ? null : prev));
  }, [yourCharacterId]);

  const handleToggleParticipant = (characterId: number) => {
    const newSelected = new Set(selectedParticipants);
    if (newSelected.has(characterId)) {
      newSelected.delete(characterId);
    } else {
      newSelected.add(characterId);
    }
    setSelectedParticipants(newSelected);
  };

  const handleYourCharacterChange = (characterId: number) => {
    // Set the new character
    setYourCharacterId(characterId);

    // Remove this character from selected participants if it was selected
    if (selectedParticipants.has(characterId)) {
      const newSelected = new Set(selectedParticipants);
      newSelected.delete(characterId);
      setSelectedParticipants(newSelected);
    }
  };

  const handleCreate = async () => {
    if (!title.trim()) {
      setError('Please enter a conversation title');
      return;
    }

    if (!yourCharacterId) {
      setError('Please select your character');
      return;
    }

    if (allowGroupConversations && selectedParticipants.size === 0) {
      setError('Please select at least one participant');
      return;
    }

    if (!allowGroupConversations && !singleParticipant) {
      setError('Please select a participant');
      return;
    }

    try {
      setCreating(true);
      setError(null);

      // Include your character + all selected participants
      const allParticipantIds = allowGroupConversations
        ? [yourCharacterId, ...Array.from(selectedParticipants)]
        : [yourCharacterId, singleParticipant!];

      const response = await apiClient.conversations.createConversation(gameId, {
        title: title.trim(),
        character_ids: allParticipantIds,
      });

      onConversationCreated(response.data.id);
      onClose();
    } catch (_err) {
      logger.error('Failed to create conversation', { error: _err, gameId, title, participantCount: selectedParticipants.size + 1 });
      setError(_err instanceof Error ? _err.message : 'Failed to create conversation');
    } finally {
      setCreating(false);
    }
  };

  // Get available participants for the conversation
  // Exclude the character being used to send FROM (already auto-included)
  // Exclude pending/rejected characters
  // Allow user's other controllable characters (e.g., GM can include multiple NPCs)
  const availableParticipants = allCharacters.filter(
    char =>
      char.id !== yourCharacterId &&  // Don't show the character we're sending from
      char.status === 'approved'
  );

  return (
    <Modal isOpen={true} onClose={onClose} title="New Conversation">
      <div>
        {error && (
          <Alert variant="danger" className="mb-4">
            {error}
          </Alert>
        )}

        <div className="mb-4">
          <Input
            id="conversation-title"
            label="Conversation Title *"
            type="text"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder="e.g., Planning the heist"
            disabled={creating}
            required
          />
        </div>

        {/* Your Character Section */}
        <div className="mb-4">
          <label className="block text-sm font-medium text-content-primary mb-2">
            Your Character
          </label>
          {characters.length === 0 ? (
            <p className="text-sm text-content-tertiary p-3 surface-raised rounded border border-theme-default">
              You need at least one character to create conversations
            </p>
          ) : characters.length === 1 ? (
            <div className="p-3 bg-interactive-primary-subtle border border-interactive-primary rounded">
              <div className="font-medium text-content-primary">{characters[0].name}</div>
              {!isAnonymous && (
                <div className="text-xs text-content-secondary">{characters[0].character_type?.replace('_', ' ')}</div>
              )}
              {!isAnonymous && characters[0].username && (
                <div className="text-xs text-content-tertiary">Played by {characters[0].username}</div>
              )}
            </div>
          ) : (
            <Select
              value={yourCharacterId || ''}
              onChange={(e) => handleYourCharacterChange(Number(e.target.value))}
              disabled={creating}
            >
              <option value="">Select your character...</option>
              {characters.map((char) => (
                <option key={char.id} value={char.id}>
                  {char.name}{!isAnonymous ? ` (${char.character_type?.replace('_', ' ')})` : ''}{!isAnonymous && char.username ? ` - ${char.username}` : ''}
                </option>
              ))}
            </Select>
          )}
        </div>

        {/* Participants Section */}
        <div className="mb-4">
          {allowGroupConversations ? (
            <>
              <label className="block text-sm font-medium text-content-primary mb-2">
                Participants (select at least 1)
              </label>
              <div className="space-y-2 max-h-60 overflow-y-auto border border-theme-default rounded-md p-2">
                {availableParticipants.length === 0 ? (
                  <p className="text-sm text-content-tertiary text-center py-4">
                    No other characters available
                  </p>
                ) : (
                  availableParticipants.map((character) => (
                    <label
                      key={character.id}
                      className={`flex items-start gap-3 p-2 rounded transition-colors cursor-pointer ${
                        selectedParticipants.has(character.id)
                          ? 'bg-interactive-primary-subtle border border-interactive-primary'
                          : 'hover:surface-raised border border-transparent'
                      }`}
                    >
                      <Checkbox
                        id={`participant-${character.id}`}
                        checked={selectedParticipants.has(character.id)}
                        onChange={() => handleToggleParticipant(character.id)}
                        disabled={creating}
                        className="mt-1"
                      />
                      <div className="flex-1">
                        <div className="font-medium text-content-primary">{character.name}</div>
                        {!isAnonymous && (
                          <div className="text-xs text-content-tertiary">{character.character_type?.replace('_', ' ')}</div>
                        )}
                        {!isAnonymous && character.username && (
                          <div className="text-xs text-content-tertiary">Played by {character.username}</div>
                        )}
                      </div>
                    </label>
                  ))
                )}
              </div>
              {selectedParticipants.size > 0 && (
                <p className="text-xs text-content-tertiary mt-2">
                  {selectedParticipants.size} participant{selectedParticipants.size !== 1 ? 's' : ''} selected
                </p>
              )}
            </>
          ) : (
            <Select
              id="conversation-participant"
              label="Participant *"
              value={singleParticipant ?? ''}
              onChange={(e) => setSingleParticipant(Number(e.target.value) || null)}
              disabled={creating}
            >
              <option value="">Select a participant...</option>
              {availableParticipants.map((char) => (
                <option key={char.id} value={char.id}>
                  {char.name}{!isAnonymous ? ` (${char.character_type?.replace('_', ' ')})` : ''}{!isAnonymous && char.username ? ` - ${char.username}` : ''}
                </option>
              ))}
            </Select>
          )}
        </div>

        <div className="flex gap-3">
          <Button
            variant="ghost"
            onClick={onClose}
            disabled={creating}
            className="flex-1"
          >
            Cancel
          </Button>
          <Button
            variant="primary"
            onClick={handleCreate}
            disabled={creating || !title.trim() || !yourCharacterId || (allowGroupConversations ? selectedParticipants.size === 0 : !singleParticipant) || characters.length === 0}
            className="flex-1"
          >
            {creating ? 'Creating...' : 'Create Conversation'}
          </Button>
        </div>
      </div>
    </Modal>
  );
}
