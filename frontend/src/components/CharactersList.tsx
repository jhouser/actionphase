import { useState, useEffect, useRef } from 'react';
import { useUrlParam } from '../hooks/useUrlParam';
import { useNavigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '../lib/api';
import { useGameCharacterStats } from '../hooks/useCharacterStats';
import type { Character, CharacterActivityStats as CharacterActivityStatsData } from '../types/characters';
import { CharacterActivityStats } from './CharacterActivityStats';
import { useGameContext } from '../contexts/GameContext';
import { CreateCharacterModal } from './CreateCharacterModal';
import { CharacterSheet } from './CharacterSheet';
import { AssignNPCModal } from './AssignNPCModal';
import { Modal } from './Modal';
import { Card, Button, Badge, Spinner, type BadgeVariant } from './ui';
import CharacterAvatar from './CharacterAvatar';
import { logger } from '@/services/LoggingService';
import { useCharacterOwnership } from '../hooks/useCharacterOwnership';
import { useCharacterSheetPermissions } from '../hooks/useCharacterSheetPermissions';

interface CharactersListProps {
  gameId: number;
  userRole?: string; // 'gm', 'player', 'audience'
  currentUserId?: number;
  gameState?: string;
  isAnonymous?: boolean;
  isParticipant?: boolean; // Whether the user is an active participant in this game
}

export function CharactersList({
  gameId,
  userRole = 'player',
  gameState = 'setup',
  isAnonymous = false,
  isParticipant = false
}: CharactersListProps) {
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false);
  const [selectedCharacterId, setSelectedCharacterId] = useUrlParam<number | null>('character', null, {
    deserialize: (s) => parseInt(s, 10) || null,
    serialize: (v) => v === null || v === undefined ? '' : String(v),
    replace: true,
  });
  const [npcToAssign, setNpcToAssign] = useState<Character | null>(null);
  const [characterToDelete, setCharacterToDelete] = useState<Character | null>(null);
  const deletionSubmittedRef = useRef(false);
  const queryClient = useQueryClient();

  // Use ownership hook to check character ownership (works in anonymous mode)
  const { isUserCharacter: isUserCharacterById } = useCharacterOwnership(gameId);

  // Shared character-sheet permission logic (see useCharacterSheetPermissions).
  const {
    isUserCharacter,
    canView: canViewCharacterSheet,
    canEdit: canEditCharacterSheet,
    canEditStats: canEditCharacterStats,
  } = useCharacterSheetPermissions(gameId, userRole, gameState);

  // Read from GameContext — single source of truth for all game characters
  const { allGameCharacters, isLoadingAllCharacters, refetchAllGameCharacters } = useGameContext();

  // Fetch stats for the whole roster in one request rather than letting each
  // CharacterCard fetch its own (see useGameCharacterStats for why).
  const { data: statsByCharacterId } = useGameCharacterStats(gameId);

  // Refresh characters when visiting the People tab (mount refresh)
  useEffect(() => {
    refetchAllGameCharacters();
  }, [gameId]); // eslint-disable-line react-hooks/exhaustive-deps

  // Fetch game participants for user assignment (when GM creates player characters)
  const { data: participantsData } = useQuery({
    queryKey: ['gameParticipants', gameId],
    queryFn: () => apiClient.games.getGameParticipants(gameId).then(res => res.data || []),
    enabled: userRole === 'gm' // Only fetch for GMs
  });

  // Alias for readability
  const characters = allGameCharacters;
  const isLoading = isLoadingAllCharacters;
  const participants = participantsData || [];

  const approveCharacterMutation = useMutation({
    mutationFn: ({ characterId, status }: { characterId: number; status: 'approved' }) =>
      apiClient.characters.approveCharacter(characterId, { status }),
    onSuccess: () => {
      refetchAllGameCharacters();
      queryClient.invalidateQueries({ queryKey: ['userControllableCharacters', gameId] });
    }
  });

  const deleteCharacterMutation = useMutation({
    mutationFn: (characterId: number) => apiClient.characters.deleteCharacter(characterId),
    onSuccess: () => {
      deletionSubmittedRef.current = false;
      refetchAllGameCharacters();
      queryClient.invalidateQueries({ queryKey: ['userControllableCharacters', gameId] });
      setCharacterToDelete(null);
    },
    onError: (error: unknown) => {
      // Error will be displayed in the confirmation modal
      logger.error('Failed to delete character', { error, gameId, characterId: characterToDelete?.id, characterName: characterToDelete?.name });
    }
  });

  const handleApproveCharacter = (characterId: number, status: 'approved') => {
    approveCharacterMutation.mutate({ characterId, status });
  };

  const handleDeleteCharacter = () => {
    if (characterToDelete) {
      deletionSubmittedRef.current = true;
      deleteCharacterMutation.mutate(characterToDelete.id);
    }
  };

  // Filter characters based on user role, game state, and status
  // GM sees all characters
  // Players see approved characters + their own characters (regardless of status)
  const visibleCharacters = userRole === 'gm' || userRole === 'audience'
    ? characters
    : characters.filter(char => {
        // Always show approved characters
        if (char.status === 'approved') return true;
        // Always show user's own characters (even if pending)
        // Use ownership hook instead of user_id comparison (works in anonymous mode)
        if (isUserCharacterById(char.id)) return true;
        // Hide other players' pending characters
        return false;
      });

  // For non-GM players/audience: split out their own characters to show at top
  // Use isUserCharacterById directly here (isUserCharacter is defined below)
  const myCharacters = userRole !== 'gm'
    ? visibleCharacters.filter(char => isUserCharacterById(char.id))
    : [];
  const otherVisibleCharacters = myCharacters.length > 0
    ? visibleCharacters.filter(char => !isUserCharacterById(char.id))
    : visibleCharacters;

  // Group characters by type
  const playerCharacters = otherVisibleCharacters.filter(char => char.character_type === 'player_character');
  const npcs = otherVisibleCharacters.filter(char => char.character_type === 'npc');

  // Check if user can create characters
  const canCreateCharacter = () => {
    if (gameState === 'completed' || gameState === 'cancelled') return false;
    if (userRole === 'gm') return true;
    // Players must be active participants to create characters
    if (userRole === 'player' && isParticipant && (gameState === 'character_creation' || gameState === 'in_progress')) return true;
    return false;
  };

  // Check if user can delete characters
  // This is GM-only functionality
  const canDeleteCharacter = () => {
    // No deleting in completed or cancelled games
    if (gameState === 'completed' || gameState === 'cancelled') return false;
    return userRole === 'gm';
  };

  // Get character status badge variant
  const getStatusBadgeVariant = (status: string): BadgeVariant => {
    switch (status) {
      case 'approved':
        return 'success';
      case 'pending':
        return 'warning';
      default:
        return 'neutral';
    }
  };

  if (isLoading) {
    return (
      <Card variant="elevated" padding="lg">
        <h2 className="text-lg font-semibold text-content-primary mb-4">Characters</h2>
        <div className="flex items-center justify-center py-8">
          <Spinner size="lg" label="Loading characters..." />
        </div>
      </Card>
    );
  }

  return (
    <Card variant="elevated" padding="lg" data-testid="characters-list">
      <div className="flex justify-between items-center mb-4">
        <h2 className="text-lg font-semibold text-content-primary">Characters</h2>
        {canCreateCharacter() && (
          <Button
            variant="primary"
            size="sm"
            onClick={() => setIsCreateModalOpen(true)}
            data-testid="create-character-button"
          >
            Create Character
          </Button>
        )}
      </div>

      {characters.length === 0 ? (
        <div className="text-center py-8 text-content-secondary">
          <p>No characters created yet.</p>
          {canCreateCharacter() && (
            <p className="mt-1 text-sm">Click "Create Character" to get started.</p>
          )}
        </div>
      ) : (
        <div className="space-y-6">
          {/* Anonymous mode: Show all characters in one unified list */}
          {isAnonymous && userRole !== 'gm' ? (
              <>
                {/* My Characters section (anonymous mode) */}
                {myCharacters.length > 0 && (
                  <div>
                    <h3 className="text-md font-medium text-content-primary mb-3">My Characters</h3>
                    <div className="space-y-3">
                      {myCharacters.map((character) => (
                        <CharacterCard
                          key={character.id}
                          character={character}
                          isOwner={true}
                          userRole={userRole}
                          isAnonymous={isAnonymous}
                          onApprove={handleApproveCharacter}
                          onAssignNPC={setNpcToAssign}
                          onDelete={canDeleteCharacter() ? setCharacterToDelete : undefined}
                          getStatusBadgeVariant={getStatusBadgeVariant}
                          canViewSheet={canViewCharacterSheet(character)}
                          canEditSheet={canEditCharacterSheet(character)}
                          onViewSheet={() => setSelectedCharacterId(character.id)}
                          stats={statsByCharacterId?.[String(character.id)]}
                        />
                      ))}
                    </div>
                  </div>
                )}
                {/* All other characters sorted alphabetically */}
                {otherVisibleCharacters.length > 0 && myCharacters.length > 0 && (
                  <h3 className="text-md font-medium text-content-primary mb-3">Characters</h3>
                )}
                <div className="space-y-3">
                  {[...otherVisibleCharacters]
                    .sort((a, b) => a.name.localeCompare(b.name))
                    .map((character) => (
                    <CharacterCard
                      key={character.id}
                      character={character}
                      isOwner={isUserCharacter(character)}
                      userRole={userRole}
                      isAnonymous={isAnonymous}
                      onApprove={handleApproveCharacter}
                      onAssignNPC={setNpcToAssign}
                      onDelete={canDeleteCharacter() ? setCharacterToDelete : undefined}
                      getStatusBadgeVariant={getStatusBadgeVariant}
                      canViewSheet={canViewCharacterSheet(character)}
                      canEditSheet={canEditCharacterSheet(character)}
                      onViewSheet={() => setSelectedCharacterId(character.id)}
                      stats={statsByCharacterId?.[String(character.id)]}
                    />
                  ))}
                </div>
              </>
            ) : (
              <>
                {/* Non-anonymous mode: Show characters grouped by type */}
                {/* My Characters (non-GM players only) */}
                {myCharacters.length > 0 && (
                  <div>
                    <h3 className="text-md font-medium text-content-primary mb-3">My Characters</h3>
                    <div className="space-y-3">
                      {myCharacters.map((character) => (
                        <CharacterCard
                          key={character.id}
                          character={character}
                          isOwner={true}
                          userRole={userRole}
                          isAnonymous={isAnonymous}
                          onApprove={handleApproveCharacter}
                          onAssignNPC={setNpcToAssign}
                          onDelete={canDeleteCharacter() ? setCharacterToDelete : undefined}
                          getStatusBadgeVariant={getStatusBadgeVariant}
                          canViewSheet={canViewCharacterSheet(character)}
                          canEditSheet={canEditCharacterSheet(character)}
                          onViewSheet={() => setSelectedCharacterId(character.id)}
                          stats={statsByCharacterId?.[String(character.id)]}
                        />
                      ))}
                    </div>
                  </div>
                )}

                {/* Player Characters */}
                {playerCharacters.length > 0 && (
                  <div>
                    <h3 className="text-md font-medium text-content-primary mb-3">Player Characters</h3>
                    <div className="space-y-3">
                      {playerCharacters.map((character) => (
                        <CharacterCard
                          key={character.id}
                          character={character}
                          isOwner={isUserCharacter(character)}
                          userRole={userRole}
                          isAnonymous={isAnonymous}
                          onApprove={handleApproveCharacter}
                          onAssignNPC={setNpcToAssign}
                          onDelete={canDeleteCharacter() ? setCharacterToDelete : undefined}
                          getStatusBadgeVariant={getStatusBadgeVariant}
                          canViewSheet={canViewCharacterSheet(character)}
                          canEditSheet={canEditCharacterSheet(character)}
                          onViewSheet={() => {
                      setSelectedCharacterId(character.id);
                    }}
                          stats={statsByCharacterId?.[String(character.id)]}
                        />
                      ))}
                    </div>
                  </div>
                )}

                {/* NPCs */}
                {npcs.length > 0 && (
                  <div>
                    <h3 className="text-md font-medium text-content-primary mb-3">NPCs</h3>
                    <div className="space-y-3">
                      {npcs.map((character) => (
                        <CharacterCard
                          key={character.id}
                          character={character}
                          isOwner={isUserCharacter(character)}
                          userRole={userRole}
                          isAnonymous={isAnonymous}
                          onApprove={handleApproveCharacter}
                          onAssignNPC={setNpcToAssign}
                          onDelete={canDeleteCharacter() ? setCharacterToDelete : undefined}
                          getStatusBadgeVariant={getStatusBadgeVariant}
                          canViewSheet={canViewCharacterSheet(character)}
                          canEditSheet={canEditCharacterSheet(character)}
                          onViewSheet={() => {
                      setSelectedCharacterId(character.id);
                    }}
                          stats={statsByCharacterId?.[String(character.id)]}
                        />
                      ))}
                    </div>
                  </div>
                )}
            </>
          )}
        </div>
      )}

      <CreateCharacterModal
        isOpen={isCreateModalOpen}
        onClose={() => setIsCreateModalOpen(false)}
        gameId={gameId}
        userRole={userRole}
        participants={participants}
      />

      {/* Character Sheet Modal */}
      {selectedCharacterId && (() => {
        const character = characters.find(c => c.id === selectedCharacterId);
        // Safety check: only show sheet if character exists
        if (!character) return null;

        return (
          <Modal
            isOpen={true}
            onClose={() => setSelectedCharacterId(null)}
            title=""
          >
            <CharacterSheet
              characterId={selectedCharacterId}
              canEdit={canEditCharacterSheet(character)}
              canEditStats={canEditCharacterStats()}
              onClose={() => setSelectedCharacterId(null)}
              isAnonymous={isAnonymous}
              userRole={userRole}
              gameState={gameState}
            />
          </Modal>
        );
      })()}

      {/* Assign NPC Modal */}
      {npcToAssign && (
        <AssignNPCModal
          character={npcToAssign}
          gameId={gameId}
          isOpen={true}
          onClose={() => setNpcToAssign(null)}
          onSuccess={() => {
            setNpcToAssign(null);
            refetchAllGameCharacters();
          }}
        />
      )}

      {/* Delete Character Confirmation Modal */}
      {characterToDelete && (
        <Modal
          isOpen={true}
          onClose={() => { if (!deletionSubmittedRef.current) setCharacterToDelete(null); }}
          title="Delete Character?"
        >
          <div className="space-y-4">
            <p className="text-content-primary">
              Are you sure you want to delete <strong>{characterToDelete.name}</strong>?
            </p>
            <p className="text-sm text-content-secondary">
              This action cannot be undone. Characters with existing messages or action submissions cannot be deleted.
            </p>

            {deleteCharacterMutation.isError && (
              <div className="p-3 bg-danger/10 border border-danger rounded-md">
                <p className="text-sm text-danger">
                  {(deleteCharacterMutation.error as Error & { response?: { data?: { error?: string } } })?.response?.data?.error ||
                   'Failed to delete character. The character may have existing activity.'}
                </p>
              </div>
            )}

            <div className="flex justify-end space-x-2">
              <Button
                variant="secondary"
                onClick={() => { deletionSubmittedRef.current = false; setCharacterToDelete(null); }}
                disabled={deleteCharacterMutation.isPending}
              >
                Cancel
              </Button>
              <Button
                variant="danger"
                onClick={handleDeleteCharacter}
                loading={deleteCharacterMutation.isPending}
                data-testid="confirm-delete-character-button"
              >
                Delete Character
              </Button>
            </div>
          </div>
        </Modal>
      )}
    </Card>
  );
}

// Helper to determine if user can see player names in anonymous mode
// GMs, co-GMs, and audience can always see player names
// Only players have names hidden from them in anonymous mode
const canSeePlayerNames = (isAnonymous: boolean, userRole: string): boolean => {
  if (!isAnonymous) return true;
  return userRole === 'gm' || userRole === 'co_gm' || userRole === 'audience';
};

// Helper to format character type for display
// Handles special case of "NPC" being fully capitalized
const formatCharacterType = (characterType: string | undefined): string => {
  if (characterType === 'npc') {
    return 'NPC';
  }
  if (!characterType) return '';
  // Convert snake_case to Title Case (e.g., "player_character" -> "Player Character")
  return characterType
    .split('_')
    .map(word => word.charAt(0).toUpperCase() + word.slice(1))
    .join(' ');
};

interface CharacterCardProps {
  character: Character;
  isOwner: boolean;
  userRole: string;
  isAnonymous?: boolean;
  onApprove: (characterId: number, status: 'approved') => void;
  onAssignNPC?: (character: Character) => void;
  onDelete?: (character: Character) => void;
  getStatusBadgeVariant: (status: string) => BadgeVariant;
  canViewSheet: boolean;
  canEditSheet: boolean;
  onViewSheet: () => void;
  stats?: CharacterActivityStatsData;
}

function CharacterCard({
  character,
  isOwner,
  userRole,
  isAnonymous = false,
  onApprove,
  onAssignNPC,
  onDelete,
  getStatusBadgeVariant,
  canViewSheet,
  canEditSheet,
  onViewSheet,
  stats
}: CharacterCardProps) {
  const navigate = useNavigate();
  const { game } = useGameContext();
  const portraitAvatars = game?.portrait_avatars ?? false;

  // Public message counts are visible to everyone. The private-message count is
  // only present in `stats` when the backend authorizes this viewer to see it
  // (GM/co-GM, audience, the character's owner, or a completed game), and
  // CharacterActivityStats only renders the private line when that field is set
  // — so visibility is driven entirely by what the server returned.
  const statsData = stats;

  return (
    <div className="border border-theme-default rounded-lg p-3 md:p-4 surface-base hover:shadow-sm transition-shadow" data-testid="character-card">
      {/* Mobile: Vertical Stack Layout */}
      <div className="md:hidden space-y-3">
        {/* Avatar + Name + Badges */}
        <div className="flex gap-3">
          <CharacterAvatar
            avatarUrl={character.avatar_url}
            characterName={character.name}
            size="xl"
            shape={portraitAvatars ? 'portrait' : 'circle'}
          />
          <div className="flex-grow min-w-0">
            <h4 className="font-semibold text-base text-content-primary mb-1.5" data-testid="character-name">
              {character.name}
            </h4>
            <div className="flex flex-wrap gap-1.5">
              {character.status !== 'approved' && (
                <Badge variant={getStatusBadgeVariant(character.status)} size="sm">
                  <span data-testid="character-status-badge">{character.status}</span>
                </Badge>
              )}
              {isOwner && (
                <Badge variant="secondary" size="sm">
                  Your Character
                </Badge>
              )}
            </div>
          </div>
        </div>

        {/* Character Info */}
        <div className="text-sm text-content-primary space-y-1">
          {canSeePlayerNames(isAnonymous || false, userRole) && (
            <div>
              Type: {formatCharacterType(character.character_type)}
            </div>
          )}
          {character.character_type === 'npc' && character.assigned_username && canSeePlayerNames(isAnonymous || false, userRole) && (
            <div>Assigned to: {character.assigned_username}</div>
          )}
          {character.character_type === 'player_character' && character.username && canSeePlayerNames(isAnonymous || false, userRole) && (
            <div>Player: {character.username}</div>
          )}
          {/* Activity stats (owner, GM, audience, or completed game) */}
          {statsData && (
            <CharacterActivityStats stats={statsData} />
          )}
        </div>

        {/* Action Buttons */}
        <div className="flex flex-wrap gap-2 pt-2">
          {canViewSheet && (
            <Button
              variant={canEditSheet ? 'primary' : 'secondary'}
              size="sm"
              onClick={onViewSheet}
            >
              <span data-testid="edit-character-button">{canEditSheet ? 'Edit Sheet' : 'View Sheet'}</span>
            </Button>
          )}

          <Button
            variant="secondary"
            size="sm"
            onClick={() => navigate(`/characters/${character.id}`)}
          >
            Profile Page
          </Button>

          {userRole === 'gm' && character.character_type === 'npc' && onAssignNPC && (
            <Button
              variant="secondary"
              size="sm"
              onClick={() => onAssignNPC(character)}
            >
              Assign NPC
            </Button>
          )}

          {userRole === 'gm' && character.status === 'pending' && (
            <Button
              variant="success"
              size="sm"
              onClick={() => onApprove(character.id, 'approved')}
            >
              <span data-testid="approve-character-button">Publish</span>
            </Button>
          )}

          {userRole === 'gm' && onDelete && (
            <Button
              variant="danger"
              size="sm"
              onClick={() => onDelete(character)}
            >
              <span data-testid="delete-character-button">Delete</span>
            </Button>
          )}
        </div>
      </div>

      {/* Desktop: Horizontal Layout (Original) */}
      <div className="hidden md:flex justify-between items-start">
        <div className="flex gap-3 flex-grow">
          <CharacterAvatar
            avatarUrl={character.avatar_url}
            characterName={character.name}
            size="xl"
            shape={portraitAvatars ? 'portrait' : 'circle'}
          />
          <div className="flex-grow">
            <div className="flex items-center space-x-2 mb-1">
              <h4 className="font-medium text-content-primary" data-testid="character-name">{character.name}</h4>
              {character.status !== 'approved' && (
                <Badge variant={getStatusBadgeVariant(character.status)} size="sm">
                  <span data-testid="character-status-badge">{character.status}</span>
                </Badge>
              )}
              {/* Show ownership badge - always visible for owned characters */}
              {isOwner && (
                <Badge variant="secondary" size="sm">
                  Your Character
                </Badge>
              )}
            </div>

            <div className="text-sm text-content-primary space-y-1">
              {/* Show character type if not anonymous mode OR if user can see player names (GM/co-GM/audience) */}
              {canSeePlayerNames(isAnonymous || false, userRole) && (
                <div>
                  Type: {formatCharacterType(character.character_type)}
                </div>
              )}
              {/* For NPCs, show assignment info */}
              {character.character_type === 'npc' && character.assigned_username && canSeePlayerNames(isAnonymous || false, userRole) && (
                <div>Assigned to: {character.assigned_username}</div>
              )}
              {/* For player characters, show player name if not anonymous mode OR if user can see player names (GM/co-GM/audience) */}
              {character.character_type === 'player_character' && character.username && canSeePlayerNames(isAnonymous || false, userRole) && (
                <div>Player: {character.username}</div>
              )}
              {/* Activity stats (owner, GM, audience, or completed game) */}
              {statsData && (
                <CharacterActivityStats stats={statsData} />
              )}
            </div>
          </div>
        </div>

        <div className="flex flex-col space-y-2 ml-4">
          {/* View Character Sheet Button */}
          {canViewSheet && (
            <Button
              variant={canEditSheet ? 'primary' : 'secondary'}
              size="sm"
              onClick={onViewSheet}
            >
              <span data-testid="edit-character-button">{canEditSheet ? 'Edit Sheet' : 'View Sheet'}</span>
            </Button>
          )}

          <Button
            variant="secondary"
            size="sm"
            onClick={() => navigate(`/characters/${character.id}`)}
          >
            Profile Page
          </Button>

          {/* Assign NPC Button (GM only, for NPCs) */}
          {userRole === 'gm' && character.character_type === 'npc' && onAssignNPC && (
            <Button
              variant="secondary"
              size="sm"
              onClick={() => onAssignNPC(character)}
            >
              Assign NPC
            </Button>
          )}

          {/* GM Actions */}
          {userRole === 'gm' && character.status === 'pending' && (
            <Button
              variant="success"
              size="sm"
              onClick={() => onApprove(character.id, 'approved')}
            >
              <span data-testid="approve-character-button">Publish</span>
            </Button>
          )}

          {/* Delete Character Button (GM only) */}
          {userRole === 'gm' && onDelete && (
            <Button
              variant="danger"
              size="sm"
              onClick={() => onDelete(character)}
            >
              <span data-testid="delete-character-button">Delete</span>
            </Button>
          )}
        </div>
      </div>
    </div>
  );
}
