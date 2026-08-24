import { useState, useEffect, useCallback, useMemo } from 'react';
import { useSearchParams, useNavigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '../lib/api';
import { useAuth } from '../contexts/AuthContext';
import { useGameContext } from '../contexts/GameContext';
import { useGameApplication } from '../hooks/useGameApplication';
import { useGameStateManagement } from '../hooks/useGameStateManagement';
import { useAutoMarkNotificationRead } from '../hooks/useNotifications';
import { useGameTabs } from '../hooks/useGameTabs';
import { usePollsByPhase } from '../hooks';
import { useCommentReadMode } from '../hooks/useUserPreferences';
import { useProvideGameUtilityContext } from '../contexts/UtilityDrawerContext';
import type { GameUtilityContext } from '../components/utility-drawer/types';
import { GameHeader } from '../components/GameHeader';
import { GameBanner } from '../components/GameBanner';
import { GameApplicationStatus } from '../components/GameApplicationStatus';
import { GameActions } from '../components/GameActions';
import { TabNavigation } from '../components/TabNavigation';
import { GameTabContent } from '../components/GameTabContent';
import { ApplyToGameModal } from '../components/ApplyToGameModal';
import { EditGameModal } from '../components/EditGameModal';
import { CompleteGameConfirmationDialog } from '../components/CompleteGameConfirmationDialog';
import { PauseGameConfirmationDialog } from '../components/PauseGameConfirmationDialog';
import { CancelGameConfirmationDialog } from '../components/CancelGameConfirmationDialog';
import { LeaveGameConfirmationDialog } from '../components/LeaveGameConfirmationDialog';
import { DeleteGameConfirmationDialog } from '../components/DeleteGameConfirmationDialog';
import { WithdrawApplicationConfirmationDialog } from '../components/WithdrawApplicationConfirmationDialog';
import { DeadlineStrip } from '../components/DeadlineStrip';
import type { CreateDeadlineRequest, UnifiedDeadline } from '../types/deadlines';
import { getDeadlineTarget } from '../utils/deadlineTarget';
import { clearForeignTabParams } from '../utils/tabParams';
import { logger } from '@/services/LoggingService';

interface GameDetailsPageProps {
  gameId: number;
  isGM?: boolean;
}

export const GameDetailsPage = ({ gameId }: GameDetailsPageProps) => {
  useAutoMarkNotificationRead();

  // Get data from contexts
  const { currentUser, isCheckingAuth } = useAuth();
  const {
    game,
    participants,
    isLoadingGame,
    isLoadingParticipants,
    isGM,
    isParticipant,
    isInGame,
    isAudience,
    canEditGame,
    userRole,
    userCharacters,
    allGameCharacters,
    refetchGameData,
  } = useGameContext();

  const currentUserId = currentUser?.id ?? null;
  const loading = isLoadingGame || isLoadingParticipants;

  // Get current phase data. Phases typically change on the order of hours/days
  // (GM activation or the server-side scheduler's time-based auto-activation),
  // not seconds, so a 3-minute poll is plenty responsive without generating
  // the request volume a 30s interval did. This was the single busiest polled
  // endpoint on the /games/* route.
  const { data: currentPhaseData, isLoading: isLoadingPhase } = useQuery({
    queryKey: ['currentPhase', gameId],
    queryFn: () => apiClient.phases.getCurrentPhase(gameId).then(res => res.data),
    enabled: !!gameId && game?.state === 'in_progress',
    refetchInterval: 180000,
  });

  // Custom hooks for application management
  const {
    userApplication,
    actionLoading: appActionLoading,
    showApplyModal,
    setShowApplyModal,
    showWithdrawModal,
    setShowWithdrawModal,
    handleApplicationSubmitted: hookHandleApplicationSubmitted,
    handleWithdrawApplication,
    confirmWithdrawApplication: hookConfirmWithdrawApplication,
  } = useGameApplication({
    gameId,
    isGM,
    isInGame,
    currentUserId,
    isLoadingParticipants,
    refetchGameData,
    gameState: game?.state,
  });

  // Wrap handlers to trigger applications list refresh
  const handleApplicationSubmitted = async () => {
    await hookHandleApplicationSubmitted();
    setApplicationsRefreshTrigger(prev => prev + 1);
  };

  const confirmWithdrawApplication = async () => {
    await hookConfirmWithdrawApplication();
    setApplicationsRefreshTrigger(prev => prev + 1);
  };

  // Custom hooks for state management
  const {
    actionLoading: stateActionLoading,
    handleStateChange,
    handleLeaveGame,
    getStateActions,
    showCompleteDialog,
    setShowCompleteDialog,
    handleConfirmComplete,
    showPauseDialog,
    setShowPauseDialog,
    handleConfirmPause,
    showCancelDialog,
    setShowCancelDialog,
    handleConfirmCancel,
    showLeaveDialog,
    setShowLeaveDialog,
    handleConfirmLeave,
  } = useGameStateManagement({
    gameId,
    refetchGameData,
  });

  // Fetch polls to calculate unvoted count for badge (phase-specific)
  const currentPhaseId = currentPhaseData?.phase?.id;
  const { data: polls = [] } = usePollsByPhase(gameId, currentPhaseId || 0);
  const unvotedPollsCount = polls.filter(poll => !poll.user_has_voted).length;

  // Check if player has already submitted an action for the current phase (served from cache)
  const { data: userActionsData } = useQuery({
    queryKey: ['userActions', gameId],
    queryFn: () => apiClient.phases.getUserActions(gameId).then(r => r.data),
    enabled: !!gameId && currentPhaseData?.phase?.phase_type === 'action' && isParticipant,
  });
  const hasSubmittedAction = !!userActionsData?.some(a => a.phase_id === currentPhaseData?.phase?.id);

  const [searchParams] = useSearchParams();
  const navigate = useNavigate();

  // Custom hooks for tab management
  const { tabs, activeTab, setActiveTab } = useGameTabs({
    gameState: game?.state,
    isGM,
    participantCount: participants.length,
    currentPhaseType: currentPhaseData?.phase?.phase_type,
    isPhaseLoading: isLoadingPhase,
    isAudience,
    isParticipant,
    hasCharacters: userCharacters.length > 0,
    unvotedPollsCount,
    hasSubmittedAction,
    isRoleLoading: isLoadingParticipants,
  });

  // The Utility Drawer lives at the app root and is reachable from every page,
  // so the game it should act on has to come from whatever game surface is
  // mounted. Publishing here rather than from a single tab means the drawer is
  // scoped to the game you are actually looking at on ALL of them — during an
  // action phase, in a completed game's archive, on People or History — instead
  // of falling back to cross-game mode and offering a character from an
  // unrelated game. CommonRoom republishes over this with the viewed phase.
  const commentReadMode = useCommentReadMode();
  const gameUtilityContext = useMemo<GameUtilityContext>(
    () => ({
      gameId,
      currentPhase: currentPhaseData?.phase ?? null,
      isGM,
      isAudience,
      isGameCompleted: game?.state === 'completed',
      userRole,
      gameState: game?.state ?? '',
      isAnonymous: game?.is_anonymous ?? false,
      portraitAvatars: game?.portrait_avatars ?? false,
      sheetConfig: game?.character_sheet,
      userCharacters,
      allGameCharacters,
      commentReadMode,
    }),
    [
      gameId,
      currentPhaseData?.phase,
      isGM,
      isAudience,
      userRole,
      game?.state,
      game?.is_anonymous,
      game?.portrait_avatars,
      game?.character_sheet,
      userCharacters,
      allGameCharacters,
      commentReadMode,
    ]
  );
  useProvideGameUtilityContext('game-page', gameUtilityContext);

  const getTabHref = useCallback((tabId: string) => {
    const params = new URLSearchParams(searchParams);
    params.set('tab', tabId);
    // Clear tab-specific sub-params when leaving their tab, so returning to a
    // tab later opens it fresh instead of resuming a stale drill-down.
    clearForeignTabParams(params, tabId);
    return `?${params.toString()}`;
  }, [searchParams]);

  // Deadline cards navigate to wherever the deadline is actually acted on:
  // the polls sub-tab for poll deadlines, the phase's tab for phase deadlines.
  const availableTabIds = useMemo(() => tabs.map(t => t.id), [tabs]);
  const getDeadlineHref = useCallback((deadline: UnifiedDeadline) => {
    const target = getDeadlineTarget(deadline, {
      currentPhaseType: currentPhaseData?.phase?.phase_type,
      availableTabIds,
    });
    if (!target) return null;

    const params = new URLSearchParams(searchParams);
    target.clearParams?.forEach(key => params.delete(key));
    Object.entries(target.params).forEach(([key, value]) => params.set(key, value));
    return `?${params.toString()}`;
  }, [searchParams, availableTabIds, currentPhaseData?.phase?.phase_type]);

  const handleDeadlineClick = useCallback((deadline: UnifiedDeadline) => {
    const href = getDeadlineHref(deadline);
    if (href) {
      navigate(href);
    }
  }, [getDeadlineHref, navigate]);

  const actionLoading = appActionLoading || stateActionLoading;
  const [showEditModal, setShowEditModal] = useState(false);
  const [showDeleteDialog, setShowDeleteDialog] = useState(false);
  const [isDescriptionExpanded, setIsDescriptionExpanded] = useState(false);
  const [applicationsRefreshTrigger, setApplicationsRefreshTrigger] = useState(0);

  // Header collapse state for mobile (persisted in localStorage)
  const [isHeaderCollapsed, setIsHeaderCollapsed] = useState(() => {
    const stored = localStorage.getItem('gameHeaderCollapsed');
    return stored === 'true';
  });

  // Persist header collapse state
  useEffect(() => {
    localStorage.setItem('gameHeaderCollapsed', String(isHeaderCollapsed));
  }, [isHeaderCollapsed]);

  const queryClient = useQueryClient();

  // Fetch deadlines
  const { data: deadlines = [], isLoading: isLoadingDeadlines } = useQuery({
    queryKey: ['deadlines', gameId],
    queryFn: () => apiClient.deadlines.getGameDeadlines(gameId, false).then(res => res.data),
    enabled: !!gameId,
  });

  // Create deadline mutation
  const createDeadlineMutation = useMutation({
    mutationFn: (data: CreateDeadlineRequest) =>
      apiClient.deadlines.createDeadline(gameId, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['deadlines', gameId] });
    },
  });

  // Update deadline mutation
  const updateDeadlineMutation = useMutation({
    mutationFn: ({ deadlineId, data }: { deadlineId: number; data: CreateDeadlineRequest }) =>
      apiClient.deadlines.updateDeadline(deadlineId, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['deadlines', gameId] });
    },
  });

  // Delete deadline mutation
  const deleteDeadlineMutation = useMutation({
    mutationFn: (deadlineId: number) =>
      apiClient.deadlines.deleteDeadline(deadlineId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['deadlines', gameId] });
    },
  });

  // Deadline handlers
  const handleCreateDeadline = async (data: CreateDeadlineRequest) => {
    await createDeadlineMutation.mutateAsync(data);
  };

  const handleUpdateDeadline = async (deadlineId: number, data: CreateDeadlineRequest) => {
    await updateDeadlineMutation.mutateAsync({ deadlineId, data });
  };

  const handleDeleteDeadline = async (deadlineId: number) => {
    await deleteDeadlineMutation.mutateAsync(deadlineId);
  };

  const handleExtendDeadline = async (deadlineId: number, hours: number) => {
    const deadline = deadlines.find(d => d.source_id === deadlineId);
    if (!deadline?.deadline) return;

    // Calculate new deadline by adding hours to current deadline
    const currentDate = new Date(deadline.deadline);
    const newDate = new Date(currentDate.getTime() + hours * 60 * 60 * 1000);

    await updateDeadlineMutation.mutateAsync({
      deadlineId,
      data: {
        title: deadline.title,
        description: deadline.description || '',
        deadline: newDate.toISOString(),
      },
    });
  };

  // Delete game handler
  const handleDeleteGame = () => {
    setShowDeleteDialog(true);
  };

  const handleConfirmDelete = async () => {
    try {
      await apiClient.games.deleteGame(gameId);
      // Redirect to games list after successful deletion
      window.location.href = '/games';
    } catch (error) {
      logger.error('Failed to delete game', { error });
      // Error will be shown by the API client
    }
  };

  if (loading) {
    return (
      <div className="min-h-screen surface-page">
        <div className="max-w-7xl mx-auto md:px-4 py-4 md:py-8">
          {/* Header skeleton */}
          <div className="surface-base shadow-md py-4 px-3 md:p-6 mb-6 md:rounded-lg animate-pulse">
            <div className="flex items-start justify-between">
              <div className="flex-1">
                <div className="h-7 bg-bg-secondary rounded w-2/3 mb-3"></div>
                <div className="flex gap-2">
                  <div className="h-5 bg-bg-secondary rounded-full w-20"></div>
                  <div className="h-5 bg-bg-secondary rounded-full w-16"></div>
                  <div className="h-5 bg-bg-secondary rounded-full w-24"></div>
                </div>
              </div>
              <div className="h-8 bg-bg-secondary rounded w-24 ml-4"></div>
            </div>
            <div className="mt-4 space-y-2">
              <div className="h-4 bg-bg-secondary rounded w-full"></div>
              <div className="h-4 bg-bg-secondary rounded w-5/6"></div>
            </div>
          </div>

          {/* Tab bar + content skeleton */}
          <div className="surface-base shadow-sm md:rounded-lg mb-6 animate-pulse">
            <div className="flex gap-1 p-2 border-b border-theme-default overflow-x-auto">
              {Array.from({ length: 4 }).map((_, i) => (
                <div key={i} className="h-8 bg-bg-secondary rounded w-20 flex-shrink-0"></div>
              ))}
            </div>
            <div className="p-4 md:p-6 space-y-4">
              <div className="h-4 bg-bg-secondary rounded w-full"></div>
              <div className="h-4 bg-bg-secondary rounded w-4/5"></div>
              <div className="h-4 bg-bg-secondary rounded w-3/5"></div>
              <div className="h-32 bg-bg-secondary rounded w-full mt-4"></div>
            </div>
          </div>
        </div>
      </div>
    );
  }

  if (!game) {
    return (
      <div className="min-h-screen surface-page flex items-center justify-center">
        <div className="surface-base p-8 rounded-lg shadow-md max-w-md w-full">
          <h2 className="text-xl font-semibold text-content-primary mb-4">Error</h2>
          <p className="text-content-secondary mb-4">Game not found</p>
          <button
            onClick={() => window.history.back()}
            className="w-full bg-interactive-primary hover:bg-interactive-primary-hover text-white py-2 px-4 rounded-lg transition-colors"
          >
            Go Back
          </button>
        </div>
      </div>
    );
  }

  const stateActions = isGM ? getStateActions(game.state) : [];

  // Check if user is viewing as public (completed game, not a participant)
  const isPublicViewer = game?.state === 'completed' && userRole === 'none';

  return (
    <div className="min-h-screen surface-page">
      {/* Banner: full-bleed on mobile, constrained on desktop */}
      {game.banner_url && (
        <div className="max-w-7xl mx-auto md:px-4 md:pt-4">
          <GameBanner bannerUrl={game.banner_url} />
        </div>
      )}
      <div className={`max-w-7xl mx-auto md:px-4 ${game.banner_url ? 'py-0 md:py-0' : 'py-4 md:py-8'}`}>
        {/* Public Archive Notice */}
        {isPublicViewer && (
          <div className="bg-interactive-primary/10 border border-interactive-primary md:rounded-lg py-3 px-3 md:p-4 mb-6">
            <div className="flex items-center gap-2">
              <svg className="w-5 h-5 text-interactive-primary" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
              </svg>
              <div>
                <p className="font-semibold text-content-primary">Public Archive</p>
                <p className="text-sm text-content-secondary">
                  This completed game is publicly viewable as a read-only archive. You can browse the game's history, but cannot create new content.
                </p>
              </div>
            </div>
          </div>
        )}

        {/* Audience Member Notice */}
        {!isPublicViewer && userRole === 'audience' && (
          <div className="bg-semantic-info-subtle border border-semantic-info md:rounded-lg py-3 px-3 md:p-4 mb-6">
            <div className="flex items-center gap-2">
              <svg className="w-5 h-5 text-semantic-info" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
              </svg>
              <div>
                <p className="font-semibold text-content-primary">Audience Member</p>
                <p className="text-sm text-content-secondary">
                  You are viewing this game as an audience member. You can observe the game's progress and interact in the Audience tab.
                </p>
              </div>
            </div>
          </div>
        )}

        {/* Header - Compact Layout */}
        <div className={`surface-base shadow-md py-4 px-3 md:p-6 mb-6 ${game.banner_url ? 'md:rounded-b-lg' : 'md:rounded-lg'}`}>
          {/* Compact Header with integrated action menu */}
          <GameHeader
            game={game}
            participants={participants}
            playerCount={`${game.current_players || 0}/${game.max_players || '∞'}`}
            isCollapsed={isHeaderCollapsed}
            onToggleCollapse={() => setIsHeaderCollapsed(!isHeaderCollapsed)}
            pinnedAction={
              <GameActions
                game={game}
                isGM={isGM}
                canEditGame={canEditGame}
                isCheckingAuth={isCheckingAuth}
                isParticipant={isParticipant}
                isInGame={isInGame}
                userRole={userRole}
                userApplication={userApplication}
                hasPendingAudienceApplication={
                  !!userApplication && userApplication.role === 'audience' && userApplication.status === 'pending'
                }
                actionLoading={actionLoading}
                stateActions={stateActions}
                onEditGame={() => setShowEditModal(true)}
                onStateChange={handleStateChange}
                onApplyToGame={() => setShowApplyModal(true)}
                onWithdrawApplication={handleWithdrawApplication}
                onLeaveGame={handleLeaveGame}
                onDeleteGame={handleDeleteGame}
                slot="player-actions"
              />
            }
            actionMenu={
              <GameActions
                game={game}
                isGM={isGM}
                canEditGame={canEditGame}
                isCheckingAuth={isCheckingAuth}
                isParticipant={isParticipant}
                isInGame={isInGame}
                userRole={userRole}
                userApplication={userApplication}
                hasPendingAudienceApplication={
                  !!userApplication && userApplication.role === 'audience' && userApplication.status === 'pending'
                }
                actionLoading={actionLoading}
                stateActions={stateActions}
                onEditGame={() => setShowEditModal(true)}
                onStateChange={handleStateChange}
                onApplyToGame={() => setShowApplyModal(true)}
                onWithdrawApplication={handleWithdrawApplication}
                onLeaveGame={handleLeaveGame}
                onDeleteGame={handleDeleteGame}
                slot="menu-actions"
              />
            }
          />

          {/* Description and Game Info - Hidden when collapsed on mobile, always visible on desktop */}
          <div className={isHeaderCollapsed ? 'hidden md:block' : ''}>
            {/* Description - Truncated with expand */}
            {game.description && (
              <div className="mt-3 mb-4">
                <p className={`text-content-secondary leading-relaxed ${!isDescriptionExpanded && game.description.length > 200 ? 'line-clamp-1' : ''}`}>
                  {game.description}
                </p>
                {game.description.length > 200 && (
                  <button
                    onClick={() => setIsDescriptionExpanded(!isDescriptionExpanded)}
                    className="text-sm text-interactive-primary hover:text-interactive-primary-hover font-medium mt-1 transition-colors inline-flex items-center gap-1"
                  >
                    {isDescriptionExpanded ? (
                      <>
                        <span>Show Less</span>
                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 15l7-7 7 7" />
                        </svg>
                      </>
                    ) : (
                      <>
                        <span>Show More</span>
                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                        </svg>
                      </>
                    )}
                  </button>
                )}
              </div>
            )}

          </div>

          {/* User Application Status - Show pending applications */}
          {!isGM && !isInGame && userApplication && (
            <div className="mb-4">
              <GameApplicationStatus application={userApplication} />
            </div>
          )}

          {/* Deadline Strip - Horizontal layout with cards */}
          <DeadlineStrip
            deadlines={deadlines}
            isLoading={isLoadingDeadlines}
            isGM={isGM}
            gameState={game.state}
            onDeadlineClick={handleDeadlineClick}
            getDeadlineHref={getDeadlineHref}
            onCreateDeadline={handleCreateDeadline}
            onUpdateDeadline={handleUpdateDeadline}
            onDeleteDeadline={handleDeleteDeadline}
            onExtendDeadline={handleExtendDeadline}
          />
        </div>

        {/* Tab Navigation */}
        {tabs.length > 0 && (
          <div className="mb-6">
            <TabNavigation
              tabs={tabs}
              activeTab={activeTab}
              onTabChange={setActiveTab}
              getTabHref={getTabHref}
              collapseOverflow
              sticky
            />

            {/* Tab Content */}
            <div className={`surface-base md:rounded-b-lg shadow-md py-4 md:p-6`}>
              <GameTabContent
                activeTab={activeTab}
                gameId={gameId}
                game={game}
                participants={participants}
                currentPhaseData={currentPhaseData}
                isLoadingPhase={isLoadingPhase}
                isGM={isGM}
                isParticipant={isParticipant}
                isAudience={isAudience}
                currentUserId={currentUserId}
                userCharacters={userCharacters}
                onLeaveGame={handleLeaveGame}
                onRefreshData={refetchGameData}
                actionLoading={actionLoading}
                applicationsRefreshTrigger={applicationsRefreshTrigger}
              />
            </div>
          </div>
        )}
      </div>

      {/* Apply to Game Modal */}
      {game && (
        <ApplyToGameModal
          gameId={gameId}
          gameTitle={game.title}
          autoAcceptAudience={game.auto_accept_audience}
          audienceOnly={game.state !== 'recruitment'} // Only audience can join after recruitment
          isOpen={showApplyModal}
          onClose={() => setShowApplyModal(false)}
          onApplicationSubmitted={handleApplicationSubmitted}
        />
      )}

      {/* Edit Game Modal */}
      {game && (
        <EditGameModal
          game={game}
          isOpen={showEditModal}
          onClose={() => setShowEditModal(false)}
          onGameUpdated={refetchGameData}
        />
      )}

      {/* Complete Game Confirmation Dialog */}
      {game && (
        <CompleteGameConfirmationDialog
          isOpen={showCompleteDialog}
          onClose={() => setShowCompleteDialog(false)}
          onConfirm={handleConfirmComplete}
          gameTitle={game.title}
        />
      )}

      {/* Pause Game Confirmation Dialog */}
      {game && (
        <PauseGameConfirmationDialog
          isOpen={showPauseDialog}
          onClose={() => setShowPauseDialog(false)}
          onConfirm={handleConfirmPause}
          gameTitle={game.title}
        />
      )}

      {/* Cancel Game Confirmation Dialog */}
      {game && (
        <CancelGameConfirmationDialog
          isOpen={showCancelDialog}
          onClose={() => setShowCancelDialog(false)}
          onConfirm={handleConfirmCancel}
          gameTitle={game.title}
        />
      )}

      {/* Withdraw Application Confirmation Dialog */}
      {game && userApplication && (
        <WithdrawApplicationConfirmationDialog
          isOpen={showWithdrawModal}
          onClose={() => setShowWithdrawModal(false)}
          onConfirm={confirmWithdrawApplication}
          gameTitle={game.title}
          isSubmitting={appActionLoading}
          role={userApplication.role as 'player' | 'audience'}
        />
      )}

      {/* Leave Game Confirmation Dialog */}
      {game && (
        <LeaveGameConfirmationDialog
          isOpen={showLeaveDialog}
          onClose={() => setShowLeaveDialog(false)}
          onConfirm={handleConfirmLeave}
          gameTitle={game.title}
          isSubmitting={actionLoading}
        />
      )}

      {/* Delete Game Confirmation Dialog */}
      {game && (
        <DeleteGameConfirmationDialog
          isOpen={showDeleteDialog}
          onClose={() => setShowDeleteDialog(false)}
          onConfirm={handleConfirmDelete}
          gameTitle={game.title}
        />
      )}
    </div>
  );
};
