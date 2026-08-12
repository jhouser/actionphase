import React, { useMemo, useState } from 'react';
import { Select, Alert } from './ui';
import { CreateActionResultForm } from './CreateActionResultForm';
import { useGameContext } from '../contexts/GameContext';

interface StandaloneResultComposerProps {
  gameId: number;
  onSuccess?: () => void;
}

/**
 * GM-facing composer for a result that answers no action submission.
 *
 * The regular result flow hangs off an expanded submission card, which forces
 * every result to have a parent submission. Games that collect actions offsite
 * (e.g. a Google Form feeding a spreadsheet) have no submission to hang it on,
 * and previously had to ask players to file blank submissions just to unblock
 * the GM. This picks a recipient directly instead.
 *
 * The result is created with no `action_submission_id`, which the schema and
 * the API have always allowed — see `action_results.action_submission_id`.
 */
export const StandaloneResultComposer: React.FC<StandaloneResultComposerProps> = ({
  gameId,
  onSuccess,
}) => {
  const { allGameCharacters } = useGameContext();
  const [selectedCharacterId, setSelectedCharacterId] = useState<string>('');

  // A result is delivered to a *user*, so only characters with a controlling
  // user are addressable. NPCs assigned to a player count — that player
  // receives the result — so fall back to the assignment before excluding.
  //
  // `is_active` is false only for characters orphaned when their player was
  // removed from the game; there is no one left to deliver a result to.
  const recipients = useMemo(
    () =>
      allGameCharacters
        .filter(character => character.is_active && character.status === 'approved')
        .map(character => ({
          character,
          userId: character.user_id ?? character.assigned_user_id,
        }))
        .filter((entry): entry is { character: typeof entry.character; userId: number } =>
          entry.userId !== undefined
        ),
    [allGameCharacters]
  );

  const selected = recipients.find(
    entry => entry.character.id.toString() === selectedCharacterId
  );

  if (recipients.length === 0) {
    return (
      <Alert variant="info">
        No approved characters with an assigned player yet. Standalone results are
        delivered to the player controlling a character.
      </Alert>
    );
  }

  return (
    <div className="space-y-4">
      <Select
        label="Recipient"
        value={selectedCharacterId}
        onChange={e => setSelectedCharacterId(e.target.value)}
        data-testid="standalone-result-recipient"
      >
        <option value="">Select a character…</option>
        {recipients.map(({ character }) => (
          <option key={character.id} value={character.id}>
            {character.name}
            {character.username || character.assigned_username
              ? ` (${character.username ?? character.assigned_username})`
              : ''}
          </option>
        ))}
      </Select>

      {selected ? (
        <CreateActionResultForm
          gameId={gameId}
          userId={selected.userId}
          userName={
            selected.character.username ??
            selected.character.assigned_username ??
            'Unknown User'
          }
          characterId={selected.character.id}
          characterName={selected.character.name}
          onSuccess={onSuccess}
        />
      ) : (
        <p className="text-sm text-content-tertiary">
          Choose a recipient to write their result.
        </p>
      )}
    </div>
  );
};
