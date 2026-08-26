import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { useUrlParam } from '../hooks/useUrlParam';
import { CharactersList } from './CharactersList';
import CharacterAvatar from './CharacterAvatar';
import { ParticipantActionsMenu } from './ParticipantActionsMenu';
import { AddPlayerModal } from './AddPlayerModal';
import { AddAudienceMemberModal } from './AddAudienceMemberModal';
import { InactiveCharactersList } from './InactiveCharactersList';
import { AudienceMemberBadge } from './AudienceMemberBadge';
import { Button } from './ui';
import { apiClient } from '../lib/api';
import type { GameParticipant, GameApplication } from '../types/games';
import { isGameWritable } from '@/lib/gamePermissions';

interface PeopleViewProps {
  gameId: number;
  participants: GameParticipant[];
  isGM: boolean;
  currentUserId: number | null;
  gmUserId?: number;
  gameState?: string;
  isAnonymous?: boolean;
  onLeaveGame?: () => void;
  onRefreshData?: () => Promise<void>;
  actionLoading?: boolean;
}

type SubTab = 'characters' | 'participants';

// Role label mapping for participant sections
const ROLE_LABELS: Record<string, string> = {
  player: 'Players',
  co_gm: 'Co-GMs',
  audience: 'Audience',
};

/**
 * Combined view for Characters and GameParticipants
 * Reduces tab clutter by grouping related people management features
 */
export function PeopleView({
  gameId,
  participants,
  isGM,
  currentUserId,
  gmUserId,
  gameState,
  isAnonymous = false,
  onLeaveGame,
  onRefreshData,
  actionLoading = false
}: PeopleViewProps) {
  const [activeSubTab, setActiveSubTab] = useUrlParam<SubTab>('peopleTab', 'characters', { replace: true });

  // Determine user's role from participants list for anonymous mode handling
  const currentUserRole = (() => {
    if (isGM) return 'gm';
    const userParticipant = participants.find(p => p.user_id === currentUserId);
    return userParticipant?.role || 'player';
  })();

  // Check if current user is an active participant (not just viewing the game)
  const isParticipant = participants.some(
    p => p.user_id === currentUserId && p.status === 'active'
  );

  const [showAddPlayerModal, setShowAddPlayerModal] = useState(false);
  const [showAddAudienceModal, setShowAddAudienceModal] = useState(false);
  const [pendingAudienceApplications, setPendingAudienceApplications] = useState<GameApplication[]>([]);

  // Fetch pending audience applications for GMs
  useEffect(() => {
    const fetchApplications = async () => {
      if (!isGM) return;

      try {
        const response = await apiClient.games.getGameApplications(gameId);
        // Filter for pending audience applications
        const audienceApps = response.data.filter(
          (app: GameApplication) => app.role === 'audience' && app.status === 'pending'
        );
        setPendingAudienceApplications(audienceApps);
      } catch (_err) {
        // Silently fail - not critical for UI
        // Error is intentionally swallowed as audience applications are optional data
      }
    };

    fetchApplications();
  }, [gameId, isGM]);

  return (
    <div className="space-y-6">
      {/* Sub-tab navigation */}
      <div className="border-b border-theme-default">
        <nav className="flex gap-4">
          <button
            onClick={() => setActiveSubTab('characters')}
            className={`
              pb-3 px-2 border-b-2 font-medium text-sm transition-colors
              ${activeSubTab === 'characters'
                ? 'border-interactive-primary text-interactive-primary'
                : 'border-transparent text-content-secondary hover:text-content-primary'
              }
            `}
          >
            Characters
          </button>
          <button
            onClick={() => setActiveSubTab('participants')}
            className={`
              pb-3 px-2 border-b-2 font-medium text-sm transition-colors
              ${activeSubTab === 'participants'
                ? 'border-interactive-primary text-interactive-primary'
                : 'border-transparent text-content-secondary hover:text-content-primary'
              }
            `}
          >
            Game Participants ({participants.length})
          </button>
        </nav>
      </div>

      {/* Characters sub-tab */}
      {activeSubTab === 'characters' && (
        <CharactersList
          gameId={gameId}
          userRole={currentUserRole}
          currentUserId={currentUserId || undefined}
          gameState={gameState}
          isAnonymous={isAnonymous}
          isParticipant={isParticipant}
        />
      )}

      {/* Game Participants sub-tab */}
      {activeSubTab === 'participants' && (
        <>
          <div className="flex items-center justify-between">
            <h2 className="text-2xl font-bold text-content-primary">Game Participants</h2>
            {isGM && isGameWritable(gameState) && (
              <div className="flex gap-2">
                <Button
                  variant="secondary"
                  onClick={() => setShowAddAudienceModal(true)}
                >
                  Add Audience Member
                </Button>
                <Button
                  variant="primary"
                  onClick={() => setShowAddPlayerModal(true)}
                >
                  Add Player
                </Button>
              </div>
            )}
          </div>

          {participants.length === 0 && pendingAudienceApplications.length === 0 ? (
            <p className="text-content-tertiary">No participants yet.</p>
          ) : (
            <div className="space-y-4">
              {/* Pending Audience Applications (GM only) */}
              {isGM && pendingAudienceApplications.length > 0 && (
                <div key="pending-audience">
                  <h3 className="font-semibold text-content-primary mb-2">
                    Pending Audience Applications ({pendingAudienceApplications.length})
                  </h3>
                  <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                    {pendingAudienceApplications.map((application) => (
                      <div key={application.id} className="border border-theme-default rounded-lg p-4 surface-raised">
                        <div className="flex items-start justify-between gap-3">
                          <div className="flex items-start gap-3 flex-1">
                            {/* Avatar */}
                            <CharacterAvatar
                              avatarUrl={application.avatar_url}
                              characterName={application.username || 'User'}
                              size="lg"
                            />

                            {/* Content */}
                            <div className="flex-1">
                              <div className="font-medium text-content-primary">{application.username}</div>
                              <div className="text-sm text-content-tertiary">
                                Applied {new Date(application.applied_at).toLocaleDateString()}
                              </div>
                              {application.message && (
                                <div className="text-sm text-content-secondary mt-2 italic">
                                  "{application.message}"
                                </div>
                              )}
                            </div>
                          </div>
                          <ParticipantActionsMenu
                            gameId={gameId}
                            application={application}
                            isPrimaryGM={currentUserId === gmUserId}
                            onSuccess={async () => {
                              // Refetch applications after approval/rejection
                              const response = await apiClient.games.getGameApplications(gameId);
                              const audienceApps = response.data.filter(
                                (app: GameApplication) => app.role === 'audience' && app.status === 'pending'
                              );
                              setPendingAudienceApplications(audienceApps);

                              // Refetch participants to show newly approved audience member
                              await onRefreshData?.();
                            }}
                          />
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {/* Active Participants */}
              {['player', 'co_gm'].map((role) => {
                const roleGameParticipants = participants.filter(p => {
                  if (p.status !== 'active') return false;
                  if (p.role === role) return true;
                  // In anonymous games, former players are blended into the Players list for
                  // anyone who isn't a GM or audience member (who already know player identities).
                  const viewerCanSeeFormerPlayers = currentUserRole === 'gm' || currentUserRole === 'audience';
                  if (isAnonymous && !viewerCanSeeFormerPlayers && role === 'player' && p.role === 'audience' && p.is_former_player) return true;
                  return false;
                });
                if (roleGameParticipants.length === 0) return null;
                return (
                  <div key={role}>
                    <h3 className="font-semibold text-content-primary mb-2">
                      {ROLE_LABELS[role]} ({roleGameParticipants.length})
                    </h3>
                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                      {roleGameParticipants.map((participant) => {
                        const isCurrentUser = participant.user_id === currentUserId;
                        const canLeaveGame = !isGM && isCurrentUser && onLeaveGame && isGameWritable(gameState);

                        return (
                          <div key={participant.id} className="border border-theme-default rounded-lg p-4 surface-raised" data-testid="participant-card">
                            <div className="flex items-start justify-between gap-3">
                              <div className="flex items-start gap-3 flex-1">
                                <CharacterAvatar
                                  avatarUrl={participant.avatar_url}
                                  characterName={participant.username}
                                  size="lg"
                                />
                                <div className="flex-1">
                                  <div className="flex items-center gap-2">
                                    <Link to={`/users/${participant.username}`} className="font-medium text-content-primary hover:underline">{participant.username}</Link>
                                    {isCurrentUser && <span className="text-xs text-content-tertiary">(You)</span>}
                                  </div>
                                  <div className="text-sm text-content-tertiary">
                                    Joined {new Date(participant.joined_at).toLocaleDateString()}
                                  </div>
                                  {canLeaveGame && (
                                    <Button
                                      variant="danger"
                                      size="sm"
                                      onClick={onLeaveGame}
                                      disabled={actionLoading}
                                      className="mt-2 text-content-primary hover:text-semantic-danger"
                                    >
                                      Leave Game
                                    </Button>
                                  )}
                                </div>
                              </div>
                              {isGM && !isCurrentUser && isGameWritable(gameState) && (
                                <ParticipantActionsMenu
                                  gameId={gameId}
                                  participant={participant}
                                  isPrimaryGM={currentUserId === gmUserId}
                                />
                              )}
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  </div>
                );
              })}

              {/* Former Players (transitioned to audience via permadeath) */}
              {/* In anonymous games, only GMs and audience members can see this section */}
              {(() => {
                const viewerCanSeeFormerPlayers = currentUserRole === 'gm' || currentUserRole === 'audience';
                if (isAnonymous && !viewerCanSeeFormerPlayers) return null;
                const formerPlayers = participants.filter(p => p.role === 'audience' && p.is_former_player && p.status === 'active');
                if (formerPlayers.length === 0) return null;
                return (
                  <div>
                    <h3 className="font-semibold text-content-primary mb-2">
                      Former Players ({formerPlayers.length})
                    </h3>
                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                      {formerPlayers.map((participant) => {
                        const isCurrentUser = participant.user_id === currentUserId;
                        const canLeaveGame = !isGM && isCurrentUser && onLeaveGame && isGameWritable(gameState);
                        return (
                          <div key={participant.id} className="border border-theme-default rounded-lg p-4 surface-raised" data-testid="participant-card">
                            <div className="flex items-start justify-between gap-3">
                              <div className="flex items-start gap-3 flex-1">
                                <CharacterAvatar
                                  avatarUrl={participant.avatar_url}
                                  characterName={participant.username}
                                  size="lg"
                                />
                                <div className="flex-1">
                                  <div className="flex items-center gap-2">
                                    <Link to={`/users/${participant.username}`} className="font-medium text-content-primary hover:underline">{participant.username}</Link>
                                    {isCurrentUser && <span className="text-xs text-content-tertiary">(You)</span>}
                                  </div>
                                  <div className="text-sm text-content-tertiary">
                                    Joined {new Date(participant.joined_at).toLocaleDateString()}
                                  </div>
                                  {canLeaveGame && (
                                    <Button
                                      variant="danger"
                                      size="sm"
                                      onClick={onLeaveGame}
                                      disabled={actionLoading}
                                      className="mt-2 text-content-primary hover:text-semantic-danger"
                                    >
                                      Leave Game
                                    </Button>
                                  )}
                                </div>
                              </div>
                              {isGM && !isCurrentUser && isGameWritable(gameState) && (
                                <ParticipantActionsMenu
                                  gameId={gameId}
                                  participant={participant}
                                  isPrimaryGM={currentUserId === gmUserId}
                                />
                              )}
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  </div>
                );
              })()}

              {/* Audience */}
              {(() => {
                // In anonymous games, former players are shown in the Players section, not here
                const audienceMembers = participants.filter(p => p.role === 'audience' && !p.is_former_player && p.status === 'active');
                if (audienceMembers.length === 0) return null;
                return (
                  <div>
                    <h3 className="font-semibold text-content-primary mb-2">
                      Audience ({audienceMembers.length})
                    </h3>
                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                      {audienceMembers.map((participant) => {
                        const isCurrentUser = participant.user_id === currentUserId;
                        const canLeaveGame = !isGM && isCurrentUser && onLeaveGame && isGameWritable(gameState);
                        return (
                          <div key={participant.id} className="border border-theme-default rounded-lg p-4 surface-raised" data-testid="participant-card">
                            <div className="flex items-start justify-between gap-3">
                              <div className="flex items-start gap-3 flex-1">
                                <CharacterAvatar
                                  avatarUrl={participant.avatar_url}
                                  characterName={participant.username}
                                  size="lg"
                                />
                                <div className="flex-1">
                                  <div className="flex items-center gap-2">
                                    <Link to={`/users/${participant.username}`} className="font-medium text-content-primary hover:underline">{participant.username}</Link>
                                    <AudienceMemberBadge />
                                    {isCurrentUser && <span className="text-xs text-content-tertiary">(You)</span>}
                                  </div>
                                  <div className="text-sm text-content-tertiary">
                                    Joined {new Date(participant.joined_at).toLocaleDateString()}
                                  </div>
                                  {canLeaveGame && (
                                    <Button
                                      variant="danger"
                                      size="sm"
                                      onClick={onLeaveGame}
                                      disabled={actionLoading}
                                      className="mt-2 text-content-primary hover:text-semantic-danger"
                                    >
                                      Leave Game
                                    </Button>
                                  )}
                                </div>
                              </div>
                              {isGM && !isCurrentUser && isGameWritable(gameState) && (
                                <ParticipantActionsMenu
                                  gameId={gameId}
                                  participant={participant}
                                  isPrimaryGM={currentUserId === gmUserId}
                                />
                              )}
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  </div>
                );
              })()}
            </div>
          )}

          {isGM && isGameWritable(gameState) && (
            <div className="mt-8">
              <InactiveCharactersList gameId={gameId} />
            </div>
          )}

          <AddPlayerModal
            gameId={gameId}
            isOpen={showAddPlayerModal}
            onClose={() => setShowAddPlayerModal(false)}
            excludeUserIds={participants.map(p => p.user_id)}
          />
          <AddAudienceMemberModal
            gameId={gameId}
            isOpen={showAddAudienceModal}
            onClose={() => setShowAddAudienceModal(false)}
            excludeUserIds={participants.map(p => p.user_id)}
          />
        </>
      )}
    </div>
  );
}
