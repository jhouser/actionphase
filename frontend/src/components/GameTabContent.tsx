import { lazy, Suspense } from 'react';
import { formatScheduleDay } from '../lib/scheduleFormat';
import type { Character } from '../types/characters';
import { GameApplicationsList } from './GameApplicationsList';
import { PublicApplicantsList } from './PublicApplicantsList';
import { PhaseManagement } from './PhaseManagement';
import { ActionSubmission } from './ActionSubmission';
import { ActionsList } from './ActionsList';
import { ActionResultsList } from './ActionResultsList';
import { GameResultsManager } from './GameResultsManager';
import { CommonRoom } from './CommonRoom';
import { PrivateMessages } from './PrivateMessages';
import { HistoryView } from './HistoryView';
import { AudienceView } from './AudienceView';
import { PeopleView } from './PeopleView';
import { HandoutsList } from './HandoutsList';
import type { GameWithDetails, GameParticipant } from '../types/games';
import type { GamePhase } from '../types/phases';
import { GameLogsView } from './GameLogsView';

// Lazy load PollsTab to match CommonRoom's lazy loading and prevent duplicate chunks
const PollsTab = lazy(() => import('./PollsTab').then(m => ({ default: m.PollsTab })));

interface GameTabContentProps {
  activeTab: string;
  gameId: number;
  game: GameWithDetails;
  participants: GameParticipant[];
  currentPhaseData?: { phase: GamePhase | null };
  isLoadingPhase?: boolean;
  isGM: boolean;
  isParticipant: boolean;
  isAudience?: boolean;
  currentUserId: number | null;
  userCharacters: Character[];
  onLeaveGame?: () => void;
  onRefreshData?: () => Promise<void>;
  actionLoading?: boolean;
  applicationsRefreshTrigger?: number;
}

const formatDate = (dateString?: string) => {
  if (!dateString) return 'Not set';
  return new Date(dateString).toLocaleString();
};

export function GameTabContent({
  activeTab,
  gameId,
  game,
  participants,
  currentPhaseData,
  isLoadingPhase = false,
  isGM,
  isAudience = false,
  currentUserId,
  userCharacters,
  onLeaveGame,
  onRefreshData,
  actionLoading = false,
  applicationsRefreshTrigger,
}: GameTabContentProps) {

  // Applications Tab (Recruitment - GM only)
  if (activeTab === 'applications' && game.state === 'recruitment' && isGM) {
    return <GameApplicationsList gameId={gameId} isGM={isGM} gameState={game.state} refreshTrigger={applicationsRefreshTrigger} />;
  }

  // Applications Tab (Character Creation - GM only, collapsed)
  if (activeTab === 'applications' && game.state === 'character_creation' && isGM) {
    return <GameApplicationsList gameId={gameId} isGM={isGM} gameState={game.state} refreshTrigger={applicationsRefreshTrigger} />;
  }

  // People Tab (combines Characters and Participants) - used for character_creation, in_progress, and completed states
  if (activeTab === 'people') {
    return (
      <PeopleView
        gameId={gameId}
        participants={participants}
        isGM={isGM}
        currentUserId={currentUserId}
        gmUserId={game.gm_user_id}
        gameState={game.state}
        isAnonymous={game.is_anonymous || false}
        onLeaveGame={onLeaveGame}
        onRefreshData={onRefreshData}
        actionLoading={actionLoading}
      />
    );
  }

  // Game Info Tab (Recruitment & other states)
  if (activeTab === 'info') {
    const hasSchedule =
      game.common_room_open_day !== null && game.common_room_open_day !== undefined &&
      game.common_room_open_time !== null && game.common_room_open_time !== undefined &&
      game.common_room_close_day !== null && game.common_room_close_day !== undefined &&
      game.common_room_close_time !== null && game.common_room_close_time !== undefined &&
      game.schedule_timezone !== null && game.schedule_timezone !== undefined;

    return (
      <>
        <h2 className="text-2xl font-bold text-content-primary mb-6">Game Information</h2>

        <div className="grid grid-cols-2 gap-x-8 gap-y-5">
          <div>
            <p className="text-xs font-semibold uppercase tracking-wider text-content-secondary mb-1">Players</p>
            <p className="text-content-primary">{game.current_players} / {game.max_players || 'Unlimited'}</p>
          </div>

          {game.genre && (
            <div>
              <p className="text-xs font-semibold uppercase tracking-wider text-content-secondary mb-1">Genre</p>
              <p className="text-content-primary">{game.genre}</p>
            </div>
          )}

          {game.recruitment_deadline && (
            <div>
              <p className="text-xs font-semibold uppercase tracking-wider text-content-secondary mb-1">Recruitment Deadline</p>
              <p className="text-content-primary">{formatDate(game.recruitment_deadline)}</p>
            </div>
          )}

          {game.start_date && (
            <div>
              <p className="text-xs font-semibold uppercase tracking-wider text-content-secondary mb-1">Start Date</p>
              <p className="text-content-primary">{formatDate(game.start_date)}</p>
            </div>
          )}

          {game.end_date && (
            <div>
              <p className="text-xs font-semibold uppercase tracking-wider text-content-secondary mb-1">End Date</p>
              <p className="text-content-primary">{formatDate(game.end_date)}</p>
            </div>
          )}
        </div>

        {hasSchedule && (
          <>
            <div className="border-t border-border-primary my-6" />
            <div>
              <p className="text-xs font-semibold uppercase tracking-wider text-content-secondary mb-3">Common Room Schedule</p>
              <div className="grid grid-cols-2 gap-x-8 gap-y-3 max-w-sm">
                <p className="text-xs font-semibold uppercase tracking-wider text-content-secondary">Opens</p>
                <p className="text-content-primary text-sm">
                  {formatScheduleDay(game.common_room_open_day!, game.common_room_open_time!, game.schedule_timezone!)}
                </p>
                <p className="text-xs font-semibold uppercase tracking-wider text-content-secondary">Closes</p>
                <p className="text-content-primary text-sm">
                  {formatScheduleDay(game.common_room_close_day!, game.common_room_close_time!, game.schedule_timezone!)}
                </p>
              </div>
              <p className="text-content-tertiary text-xs mt-3">Times shown in your local timezone</p>
            </div>
          </>
        )}

        {/* Show public applicants list during recruitment */}
        {game.state === 'recruitment' && (
          <div className="mt-6 pt-6 border-t border-border-primary">
            <PublicApplicantsList gameId={gameId} />
          </div>
        )}
      </>
    );
  }

  // Common Room Tab (In Progress & Completed - common_room phases)
  if (activeTab === 'common-room' && (game.state === 'in_progress' || game.state === 'completed')) {
    // Show loading only on initial load (when we have no data yet)
    if (isLoadingPhase && !currentPhaseData) {
      return (
        <div className="flex justify-center items-center py-12">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-interactive-primary"></div>
        </div>
      );
    }

    // Render CommonRoom if phase is common_room type
    // This stays mounted during refetches since currentPhaseData persists
    if (currentPhaseData?.phase?.phase_type === 'common_room') {
      return (
        <CommonRoom
          gameId={gameId}
          phaseId={currentPhaseData.phase.id}
          phaseTitle={currentPhaseData.phase.title || `Phase ${currentPhaseData.phase.phase_number}`}
          phaseDescription={currentPhaseData.phase.description}
          currentPhase={currentPhaseData.phase}
          isCurrentPhase={game.state === 'in_progress'}
          isGM={isGM}
          isAudience={isAudience}
        />
      );
    }

    // Phase exists but is not common_room type
    return (
      <div className="text-center py-12">
        <p className="text-content-secondary">
          Common Room is only available during Discussion phases.
        </p>
        <p className="text-content-tertiary mt-2">
          Current phase: {currentPhaseData?.phase?.phase_type}
        </p>
      </div>
    );
  }

  // Polls Tab (In Progress - common_room phases)
  if (activeTab === 'polls' && game.state === 'in_progress') {
    // Show loading only on initial load (when we have no data yet)
    if (isLoadingPhase && !currentPhaseData) {
      return (
        <div className="flex justify-center items-center py-12">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-interactive-primary"></div>
        </div>
      );
    }

    // Render Polls if phase is common_room type
    if (currentPhaseData?.phase?.phase_type === 'common_room') {
      return (
        <Suspense fallback={<div className="flex justify-center py-8"><div className="animate-spin rounded-full h-8 w-8 border-b-2 border-interactive-primary"></div></div>}>
          <PollsTab
            gameId={gameId}
            isGM={isGM}
            isCurrentPhase={true}
            isAudience={isAudience}
            gameState={game.state}
          />
        </Suspense>
      );
    }

    // Phase exists but is not common_room type
    return (
      <div className="text-center py-12">
        <p className="text-content-secondary">
          Polls are only available during Discussion phases.
        </p>
        <p className="text-content-tertiary mt-2">
          Current phase: {currentPhaseData?.phase?.phase_type}
        </p>
      </div>
    );
  }

  // Phases Tab (In Progress - GM only)
  if (activeTab === 'phases' && game.state === 'in_progress' && isGM) {
    return <PhaseManagement gameId={gameId} />;
  }

  // Actions Tab (In Progress)
  if (activeTab === 'actions' && game.state === 'in_progress') {
    return (
      <>
        {isGM ? (
          <>
            <ActionsList
              gameId={gameId}
              currentPhase={currentPhaseData?.phase}
              className="mb-6"
            />
            <GameResultsManager
              gameId={gameId}
              currentPhase={currentPhaseData?.phase}
            />
          </>
        ) : (
          <>
            <div className="mb-6">
              <ActionSubmission
                gameId={gameId}
                currentPhase={currentPhaseData?.phase}
              />
            </div>
            <div className="mb-6">
              <ActionResultsList gameId={gameId} />
            </div>
          </>
        )}
      </>
    );
  }

  // History Tab (In Progress & Completed)
  if (activeTab === 'history' && (game.state === 'in_progress' || game.state === 'completed')) {
    return <HistoryView gameId={gameId} currentPhaseId={currentPhaseData?.phase?.id} isGM={isGM} isAudience={isAudience} isGameCompleted={game.state === 'completed'} />;
  }

  // Private Messages Tab (In Progress & Completed)
  if (activeTab === 'messages' && (game.state === 'in_progress' || game.state === 'completed')) {
    return (
      <div className="h-[600px] md:h-[85vh] md:min-h-[900px]">
        <PrivateMessages
          gameId={gameId}
          characters={userCharacters}
          isAnonymous={game.is_anonymous || false}
          allowGroupConversations={game.allow_group_conversations ?? true}
          currentPhaseType={currentPhaseData?.phase?.phase_type}
        />
      </div>
    );
  }

  // Handouts Tab (All States - tab is always visible per useGameTabs)
  if (activeTab === 'handouts') {
    return <HandoutsList gameId={gameId} isGM={isGM} gameState={game.state} />;
  }

  // Audience Tab (In Progress & Completed)
  if (activeTab === 'audience' && (game.state === 'in_progress' || game.state === 'completed')) {
    return <AudienceView gameId={gameId} />;
  }

  if (activeTab === 'logs') {
    return <GameLogsView gameId={gameId} />;
  }

  // Default fallback
  return null;
}
