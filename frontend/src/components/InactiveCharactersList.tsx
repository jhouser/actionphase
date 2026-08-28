/**
 * Inactive Characters List Component
 *
 * Displays characters whose owners have been removed from the game.
 * Allows GM to reassign these characters to new owners.
 */

import { useState } from 'react';
import { Card, CardHeader, CardBody, Button, Badge, Spinner } from './ui';
import { useInactiveCharacters } from '../hooks/usePlayerManagement';
import { ReassignCharacterModal } from './ReassignCharacterModal';
import type { Character } from '../types/characters';

interface InactiveCharactersListProps {
  gameId: number;
}

export function InactiveCharactersList({ gameId }: InactiveCharactersListProps) {
  const { data: characters, isLoading, error } = useInactiveCharacters(gameId);
  const [selectedCharacter, setSelectedCharacter] = useState<Character | null>(null);

  if (isLoading) {
    return (
      <Card variant="default">
        <CardBody>
          <div className="flex items-center justify-center py-8">
            <Spinner size="lg" />
          </div>
        </CardBody>
      </Card>
    );
  }

  if (error) {
    return (
      <Card variant="default">
        <CardBody>
          <div className="p-4 rounded-lg bg-semantic-danger-subtle border border-semantic-danger">
            <p className="text-sm text-semantic-danger">
              Failed to load inactive characters. Please try again.
            </p>
          </div>
        </CardBody>
      </Card>
    );
  }

  if (!characters || characters.length === 0) {
    return (
      <Card variant="default">
        <CardHeader>
          <h3 className="text-lg font-semibold text-content-primary">Inactive Characters</h3>
        </CardHeader>
        <CardBody>
          <p className="text-sm text-content-secondary">
            No inactive characters. Characters become inactive when their owners are removed from the game.
          </p>
        </CardBody>
      </Card>
    );
  }

  return (
    <>
      <Card variant="default">
        <CardHeader>
          <div className="flex items-center justify-between">
            <h3 className="text-lg font-semibold text-content-primary">Inactive Characters</h3>
            <Badge variant="warning">{characters.length}</Badge>
          </div>
          <p className="text-sm text-content-secondary mt-1">
            Characters from removed players that can be reassigned
          </p>
        </CardHeader>
        <CardBody>
          <div className="space-y-3">
            {characters.map((character) => (
              <div
                key={character.id}
                className="p-4 rounded-lg border border-theme-default bg-surface-raised"
              >
                <div className="flex items-start justify-between gap-4">
                  <div className="flex-1">
                    <div className="flex items-center gap-2 mb-1">
                      <h4 className="font-medium text-content-primary">{character.name}</h4>
                      <Badge variant="warning" size="sm">Inactive</Badge>
                    </div>
                    <div className="text-sm text-content-secondary space-y-1">
                      <p>
                        <span className="font-medium">Original Owner:</span>{' '}
                        {character.original_owner_username || 'Unknown'}
                      </p>
                      {character.current_owner_username && (
                        <p>
                          <span className="font-medium">Current Owner:</span>{' '}
                          {character.current_owner_username}
                        </p>
                      )}
                      <p>
                        <span className="font-medium">Type:</span>{' '}
                        {character.character_type === 'player_character' ? 'Player Character' : 'NPC'}
                      </p>
                    </div>
                  </div>
                  <Button
                    variant="primary"
                    size="sm"
                    onClick={() => setSelectedCharacter(character)}
                  >
                    Reassign
                  </Button>
                </div>
              </div>
            ))}
          </div>
        </CardBody>
      </Card>

      {selectedCharacter && (
        <ReassignCharacterModal
          character={selectedCharacter}
          gameId={gameId}
          isOpen={!!selectedCharacter}
          onClose={() => setSelectedCharacter(null)}
          onSuccess={() => setSelectedCharacter(null)}
        />
      )}
    </>
  );
}
