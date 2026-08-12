import { useState, useEffect, useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useSearchParams } from 'react-router-dom';
import { apiClient } from '../lib/api';
import { getActionPhaseLabel, getActionPhaseColor } from '../types/phases';
import { CommonRoom } from './CommonRoom';
import { PhaseHistoryPolls } from './PhaseHistoryPolls';
import { Button, Alert } from './ui';
import { MarkdownPreview } from './MarkdownPreview';
import CharacterAvatar from './CharacterAvatar';
import { useOptionalGameContext } from '../contexts/GameContext';
import type { ActionWithDetails } from '../types/phases';
import { useUrlParam } from '../hooks/useUrlParam';

const phaseParamOptions = {
  deserialize: (s: string) => parseInt(s, 10) || null,
  serialize: (v: number | null) => (v === null || v === undefined ? '' : String(v)),
} as const;

// The character filter is an OR filter over character ids, stored in the URL as a
// comma-separated list so a filtered view can be linked or survive a refresh.
// NaN entries are dropped rather than trusted — the param is user-editable.
const characterFilterParamOptions = {
  deserialize: (s: string) =>
    new Set(
      s
        .split(',')
        .map(part => parseInt(part, 10))
        .filter(id => !Number.isNaN(id))
    ),
  serialize: (v: Set<number>) => [...v].join(','),
} as const;

interface HistoryViewProps {
  gameId: number;
  currentPhaseId?: number;
  isGM?: boolean;
  isAudience?: boolean;
  isGameCompleted?: boolean;
}

export function HistoryView({ gameId, currentPhaseId, isGM = false, isAudience = false, isGameCompleted = false }: HistoryViewProps) {
  const gameContext = useOptionalGameContext();
  const portraitAvatars = gameContext?.game?.portrait_avatars ?? false;
  const allGameCharacters = gameContext?.allGameCharacters;

  // Submissions and results carry only character_id/character_name, so the
  // avatar URL is looked up from the game's character list — same as ActionsList.
  const getAvatarUrl = (characterId?: number | null) =>
    characterId
      ? (allGameCharacters?.find(c => c.id === characterId)?.avatar_url ?? null)
      : null;
  const [selectedPhaseId, setSelectedPhaseId] = useUrlParam<number | null>(
    'phase',
    null,
    { ...phaseParamOptions, replace: false }
  );
  const [activeTab, setActiveTab] = useUrlParam<'submissions' | 'results' | 'polls'>(
    'subTab',
    'submissions',
    { replace: false }
  );
  const [selectedCharacterIds, setSelectedCharacterIds] = useUrlParam<Set<number>>(
    'characters',
    new Set<number>(),
    { ...characterFilterParamOptions, replace: true }
  );
  const [expandedSubmissions, setExpandedSubmissions] = useState<Set<number>>(new Set());
  const [expandedResults, setExpandedResults] = useState<Set<number>>(new Set());

  const [searchParams, setSearchParams] = useSearchParams();
  const commentParam = searchParams.get('comment');

  // Leaving the drill-down clears the phase and the character filter together.
  // These cannot be two useUrlParam setters: each one's functional update reads
  // the committed search params, not the other's pending write, so the second
  // call would clobber the first and the filter would survive the navigation.
  const exitPhaseDrillDown = () => {
    setSearchParams(prev => {
      const next = new URLSearchParams(prev);
      next.delete('phase');
      next.delete('characters');
      return next;
    });
  };

  const { data: phasesData, isLoading } = useQuery({
    queryKey: ['gamePhases', gameId],
    queryFn: () => apiClient.phases.getGamePhases(gameId).then(res => res.data),
    enabled: !!gameId,
  });

  // Only show phases that have been activated. activated_at is set the moment a
  // phase is activated and never cleared, making it the reliable signal for history.
  // is_active is a fallback for phases activated before the activated_at column was added.
  const phases = useMemo(
    () => (phasesData || []).filter(p => !!p.activated_at || p.is_active),
    [phasesData]
  );

  // Fetch action results (use appropriate endpoint based on isGM)
  // Only fetch results/submissions once a phase is selected — avoids loading all data on tab open
  const hasSelectedPhase = selectedPhaseId !== null;

  // A COMPLETED game is a public archive: the backend serves the whole cast's
  // submissions and results to any authenticated user, not just to spectators
  // (CanUserViewGame, and the "completed" arm of GetGameActionResults). A player
  // looking back at a finished game therefore gets the same reach here as an
  // audience member — reading their own rows only would hide the story they just
  // finished playing through. Cancelled games are NOT public and keep the
  // play-time rule, which is why this keys on completion rather than "not active".
  const hasArchiveAccess = isAudience || isGameCompleted;

  const { data: userActionResults, isLoading: isLoadingUserResults, error: userResultsError } = useQuery({
    queryKey: ['actionResults', 'user', gameId],
    queryFn: () => apiClient.phases.getUserResults(gameId).then(res => res.data),
    enabled: !!gameId && !isGM && !hasArchiveAccess && hasSelectedPhase,
  });

  // Audience uses the same game-wide endpoint as the GM: it admits audience
  // members, so a spectator sees the whole cast's results rather than their own
  // (they have no characters, so the per-user endpoint returns nothing). Players
  // in a completed game join them via hasArchiveAccess.
  //
  // Drafts are NOT withheld from audience. Spectators are trusted with the
  // game's private traffic already, and watching results take shape is part of
  // what they are here for — see TestPhaseAPI_AudienceSeesUnpublishedResults.
  // Players in a LIVE game are the ones excluded: /results/mine serves only
  // published results, so a draft never reaches the character it answers. Once
  // the game completes there is nothing left to spoil, and the backend serves
  // drafts to everyone on this endpoint anyway.
  const seesAllResults = isGM || hasArchiveAccess;

  const { data: gmActionResults, isLoading: isLoadingGMResults, error: gmResultsError } = useQuery({
    queryKey: ['actionResults', 'game', gameId],
    queryFn: () => apiClient.phases.getGameResults(gameId).then(res => res.data),
    enabled: !!gameId && seesAllResults && hasSelectedPhase,
  });

  const actionResults = seesAllResults ? gmActionResults : userActionResults;
  const isLoadingResults = seesAllResults ? isLoadingGMResults : isLoadingUserResults;
  const resultsError = seesAllResults ? gmResultsError : userResultsError;

  // Fetch action submissions for the game (use appropriate endpoint based on isGM)
  const { data: userActionSubmissionsData, isLoading: isLoadingUserSubmissions, error: userSubmissionsError } = useQuery<ActionWithDetails[]>({
    queryKey: ['userActions', gameId],
    queryFn: () => apiClient.phases.getUserActions(gameId).then(res => res.data),
    enabled: !!gameId && !isGM && !hasArchiveAccess && hasSelectedPhase,
  });

  const { data: gmActionSubmissionsData, isLoading: isLoadingGMSubmissions, error: gmSubmissionsError } = useQuery<ActionWithDetails[]>({
    queryKey: ['gameActions', gameId],
    queryFn: () => apiClient.phases.getGameActions(gameId).then(res => res.data),
    enabled: !!gameId && isGM && hasSelectedPhase,
  });

  // Audience members — and players in a completed game — cannot use the GM
  // submissions endpoint, which rejects non-GMs outright. They read the
  // game-wide listing instead, which authorizes on "can view this game" and so
  // admits both. It is paginated, but this view drills into a single phase at a
  // time, so one generous page covers the phase rather than dragging infinite
  // scroll into a drill-down.
  const {
    data: audienceSubmissionsData,
    isLoading: isLoadingAudienceSubmissions,
    error: audienceSubmissionsError,
  } = useQuery<ActionWithDetails[]>({
    queryKey: ['audienceActions', gameId, selectedPhaseId],
    queryFn: () =>
      apiClient.games
        .listAllActionSubmissions(gameId, { phaseId: selectedPhaseId ?? undefined, limit: 200 })
        .then(res => (res.data.action_submissions ?? []) as unknown as ActionWithDetails[]),
    enabled: !!gameId && hasArchiveAccess && !isGM && hasSelectedPhase,
  });

  // Memoised because the `|| []` fallbacks mint a new array on every render, which
  // would invalidate the phase/filter memos downstream each time.
  const actionSubmissions = useMemo(
    () =>
      isGM
        ? (gmActionSubmissionsData || [])
        : hasArchiveAccess
          ? (audienceSubmissionsData || [])
          : (userActionSubmissionsData || []),
    [isGM, hasArchiveAccess, gmActionSubmissionsData, audienceSubmissionsData, userActionSubmissionsData]
  );
  const isLoadingSubmissions = isGM
    ? isLoadingGMSubmissions
    : hasArchiveAccess
      ? isLoadingAudienceSubmissions
      : isLoadingUserSubmissions;
  const submissionsError = isGM
    ? gmSubmissionsError
    : hasArchiveAccess
      ? audienceSubmissionsError
      : userSubmissionsError;

  // Get the selected phase details
  const selectedPhase = phases.find(p => p.id === selectedPhaseId);

  const phaseSubmissions = useMemo(
    () => actionSubmissions.filter(s => s.phase_id === selectedPhaseId),
    [actionSubmissions, selectedPhaseId]
  );
  const phaseResults = useMemo(
    () => (actionResults || []).filter(r => r.phase_id === selectedPhaseId),
    [actionResults, selectedPhaseId]
  );

  // Filter chips are derived from the rows actually present in this phase rather
  // than from the game's full cast: offering a character with nothing in the phase
  // would only ever produce an empty list. Submissions and results are pooled so
  // the chip row stays identical across the two sub-tabs — switching tabs with a
  // filter applied should narrow the other tab, not silently drop the selection.
  const characterFilterOptions = useMemo(() => {
    const byId = new Map<number, string>();
    for (const entry of [...phaseSubmissions, ...phaseResults]) {
      if (entry.character_id) {
        byId.set(entry.character_id, entry.character_name || 'Unknown Character');
      }
    }
    return [...byId.entries()]
      .map(([id, name]) => ({ id, name }))
      .sort((a, b) => a.name.localeCompare(b.name));
  }, [phaseSubmissions, phaseResults]);

  // OR filter: a row survives if its character is any of the selected ones. An
  // empty selection means "no filter" rather than "match nothing".
  const matchesCharacterFilter = (characterId?: number | null) =>
    selectedCharacterIds.size === 0 ||
    (characterId !== null && characterId !== undefined && selectedCharacterIds.has(characterId));

  const visibleSubmissions = phaseSubmissions.filter(s => matchesCharacterFilter(s.character_id));
  const visibleResults = phaseResults.filter(r => matchesCharacterFilter(r.character_id));

  const toggleCharacterFilter = (characterId: number) => {
    const next = new Set(selectedCharacterIds);
    if (next.has(characterId)) {
      next.delete(characterId);
    } else {
      next.add(characterId);
    }
    setSelectedCharacterIds(next);
  };

  const toggleSubmissionExpanded = (submissionId: number) => {
    const newExpanded = new Set(expandedSubmissions);
    if (newExpanded.has(submissionId)) {
      newExpanded.delete(submissionId);
    } else {
      newExpanded.add(submissionId);
    }
    setExpandedSubmissions(newExpanded);
  };

  const toggleResultExpanded = (resultId: number) => {
    const newExpanded = new Set(expandedResults);
    if (newExpanded.has(resultId)) {
      newExpanded.delete(resultId);
    } else {
      newExpanded.add(resultId);
    }
    setExpandedResults(newExpanded);
  };

  // Reset to 'submissions' tab when switching to Action phase while 'polls' is selected
  // (Action phases don't have polls)
  useEffect(() => {
    if (selectedPhase && selectedPhase.phase_type === 'action' && activeTab === 'polls') {
      setActiveTab('submissions');
    }
  }, [selectedPhase, activeTab, setActiveTab]);

  // Auto-navigate to the correct phase when a ?comment deep-link is present.
  // This happens when a notification URL like ?tab=history&comment=99 lands here.
  // We fetch the comment to determine its phase_id, then select that phase so
  // CommonRoom can scroll to the comment.
  useEffect(() => {
    if (!commentParam || selectedPhaseId !== null || isLoading || phases.length === 0) {
      return;
    }

    const resolveCommentPhase = async () => {
      try {
        const response = await apiClient.messages.getMessage(gameId, parseInt(commentParam, 10));
        const message = response.data;
        if (message.phase_id) {
          // Only select if this phase exists in history
          const phaseExists = phases.some(p => p.id === message.phase_id);
          if (phaseExists) {
            setSelectedPhaseId(message.phase_id);
          }
        }
      } catch {
        // If we can't resolve the comment's phase, leave the phase list visible
      }
    };

    resolveCommentPhase();
  }, [commentParam, selectedPhaseId, isLoading, phases, gameId, setSelectedPhaseId]);

  if (isLoading) {
    return (
      <div className="surface-base rounded-lg shadow-md p-6">
        <div className="animate-pulse">
          <div className="h-6 surface-raised rounded mb-4 w-1/3"></div>
          <div className="space-y-3">
            {[...Array(3)].map((_, i) => (
              <div key={i} className="h-16 surface-raised rounded"></div>
            ))}
          </div>
        </div>
      </div>
    );
  }

  if (selectedPhaseId && selectedPhase) {
    // Show Common Room or Action Results for selected phase
    return (
      <div>
        <Button
          variant="ghost"
          // Character ids are phase-scoped in practice — a cast member with rows in
          // this phase may have none in the next — so leaving the drill-down clears
          // the filter rather than carrying a stale selection into a phase where it
          // would silently hide everything.
          onClick={exitPhaseDrillDown}
          className="mb-4 flex items-center text-interactive-primary hover:text-interactive-primary-hover"
        >
          <svg className="w-5 h-5 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
          </svg>
          Back to History
        </Button>
        {selectedPhase.phase_type === 'common_room' ? (
          <CommonRoom
            gameId={gameId}
            phaseId={selectedPhaseId}
            phaseTitle={selectedPhase.title || getActionPhaseLabel(selectedPhase)}
            phaseDescription={selectedPhase.description}
            // The phase being viewed, not necessarily the game's active phase.
            // Utility Drawer panels (e.g. Mark All Read) need the phase object,
            // not just its id, to scope their actions.
            currentPhase={selectedPhase}
            isCurrentPhase={false} // Always read-only in history view
            isGM={isGM}
            isAudience={isAudience}
            isGameCompleted={isGameCompleted}
          />
        ) : (
          <div className="surface-base rounded-lg shadow-md p-6">
            <h3 className="text-xl font-bold text-content-primary mb-4">
              {selectedPhase.title || getActionPhaseLabel(selectedPhase)}
            </h3>

            {/* Tab Navigation */}
            <div className="flex border-b border-border-primary mb-6">
              <button
                onClick={() => setActiveTab('submissions')}
                className={`px-4 py-2 font-medium transition-colors ${
                  activeTab === 'submissions'
                    ? 'text-interactive-primary border-b-2 border-interactive-primary'
                    : 'text-content-secondary hover:text-content-primary'
                }`}
              >
                Submissions
              </button>
              <button
                onClick={() => setActiveTab('results')}
                className={`px-4 py-2 font-medium transition-colors ${
                  activeTab === 'results'
                    ? 'text-interactive-primary border-b-2 border-interactive-primary'
                    : 'text-content-secondary hover:text-content-primary'
                }`}
              >
                Results
              </button>
              {/* Polls tab is not shown for Action phases (handled by the outer if/else at line 105) */}
            </div>

            {/* Character filter — applies to both Submissions and Results */}
            {activeTab !== 'polls' && characterFilterOptions.length > 0 && (
              <div className="border border-border-primary rounded-lg p-4 bg-bg-secondary mb-6">
                <div className="flex items-center justify-between mb-3">
                  <h4 className="text-sm font-semibold text-content-primary">Filter by Character</h4>
                  {selectedCharacterIds.size > 0 && (
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => setSelectedCharacterIds(new Set())}
                      className="text-xs"
                    >
                      Clear filters
                    </Button>
                  )}
                </div>
                <div className="flex flex-wrap gap-2">
                  {characterFilterOptions.map(({ id, name }) => {
                    const isSelected = selectedCharacterIds.has(id);
                    return (
                      <button
                        key={id}
                        onClick={() => toggleCharacterFilter(id)}
                        aria-pressed={isSelected}
                        className={`
                          px-3 py-1.5 rounded-full text-sm font-medium transition-colors
                          ${isSelected
                            ? 'bg-interactive-primary text-white'
                            : 'bg-bg-primary border border-border-primary text-content-primary hover:bg-bg-tertiary'
                          }
                        `}
                      >
                        {name}
                      </button>
                    );
                  })}
                </div>
              </div>
            )}

            {/* Tab Content */}
            {activeTab === 'submissions' ? (
              // Submissions Tab
              <>
                {isLoadingSubmissions ? (
                  <div className="p-4">
                    <p className="text-content-secondary">Loading action submissions...</p>
                  </div>
                ) : submissionsError ? (
                  <Alert variant="danger">Error loading action submissions</Alert>
                ) : (() => {
                  if (visibleSubmissions.length === 0) {
                    return (
                      <div className="p-4 surface-raised border border-theme-default rounded">
                        <p className="text-content-secondary">
                          {selectedCharacterIds.size > 0
                            ? 'No action submissions match the selected characters.'
                            : 'No action submissions for this phase.'}
                        </p>
                      </div>
                    );
                  }

                  return (
                    <div className="space-y-4">
                      {visibleSubmissions.map((submission) => {
                        const isExpanded = expandedSubmissions.has(submission.id);
                        const isCollapsible = submission.content.length > 200;
                        const previewContent = submission.content.substring(0, 200) + '...';

                        return (
                          <div key={submission.id} className="p-4 surface-raised border border-theme-default rounded shadow-sm">
                            <div className="flex items-start gap-3 mb-3">
                              <CharacterAvatar
                                characterName={submission.character_name || 'Unknown'}
                                avatarUrl={getAvatarUrl(submission.character_id)}
                                size="md"
                                shape={portraitAvatars ? 'portrait' : 'circle'}
                              />
                              <div className="flex-1 min-w-0">
                                <div className="flex justify-between items-start">
                                  <div>
                                    <span className="font-semibold text-content-primary">{submission.character_name}</span>
                                    {submission.is_draft && (
                                      <span className="inline-block px-2 py-1 text-xs bg-semantic-warning-subtle text-content-primary rounded ml-2">
                                        Draft
                                      </span>
                                    )}
                                  </div>
                                  {submission.submitted_at && (
                                    <span className="text-xs text-content-tertiary">
                                      {new Date(submission.submitted_at).toLocaleString()}
                                    </span>
                                  )}
                                </div>
                              </div>
                            </div>
                            <MarkdownPreview
                              content={isCollapsible && !isExpanded ? previewContent : submission.content}
                              fullWidth
                            />
                            {isCollapsible && (
                              <button
                                onClick={() => toggleSubmissionExpanded(submission.id)}
                                className="mt-2 text-sm text-interactive-primary hover:text-interactive-primary-hover font-medium flex items-center"
                              >
                                {isExpanded ? (
                                  <>
                                    <svg className="w-4 h-4 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 15l7-7 7 7" />
                                    </svg>
                                    Show less
                                  </>
                                ) : (
                                  <>
                                    <svg className="w-4 h-4 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                                    </svg>
                                    Show full content
                                  </>
                                )}
                              </button>
                            )}
                          </div>
                        );
                      })}
                    </div>
                  );
                })()}
              </>
            ) : activeTab === 'results' ? (
              // Results Tab
              <>
                {isLoadingResults ? (
                  <div className="p-4">
                    <p className="text-content-secondary">Loading action results...</p>
                  </div>
                ) : resultsError ? (
                  <Alert variant="danger">Error loading action results</Alert>
                ) : (() => {
                  if (visibleResults.length === 0) {
                    return (
                      <div className="p-4 surface-raised border border-theme-default rounded">
                        <p className="text-content-secondary">
                          {selectedCharacterIds.size > 0
                            ? 'No action results match the selected characters.'
                            : 'No action results for this phase.'}
                        </p>
                      </div>
                    );
                  }

                  return (
                    <div className="space-y-4">
                      {visibleResults.map((result) => {
                        const isExpanded = expandedResults.has(result.id);
                        const isCollapsible = result.content.length > 200;
                        const previewContent = result.content.substring(0, 200) + '...';

                        return (
                          <div key={result.id} className="p-4 surface-raised border border-theme-default rounded shadow-sm">
                            <div className="flex items-start gap-3 mb-3">
                              <CharacterAvatar
                                characterName={result.character_name || 'Unknown'}
                                avatarUrl={getAvatarUrl(result.character_id)}
                                size="md"
                                shape={portraitAvatars ? 'portrait' : 'circle'}
                              />
                              <div className="flex-1 min-w-0">
                                <div className="flex justify-between items-start mb-1">
                                  <div className="flex items-center gap-2 flex-wrap">
                                    <span className="font-semibold text-content-primary">
                                      {result.character_name || 'Unknown Character'}
                                    </span>
                                    {!result.is_published && (
                                      <span className="inline-block px-2 py-1 text-xs bg-semantic-warning-subtle text-content-primary rounded">
                                        Draft (Unpublished)
                                      </span>
                                    )}
                                  </div>
                                  {result.sent_at && (
                                    <span className="text-xs text-content-tertiary whitespace-nowrap">
                                      {new Date(result.sent_at).toLocaleString()}
                                    </span>
                                  )}
                                </div>
                                {result.gm_username && (
                                  <p className="text-xs text-content-tertiary">From: {result.gm_username}</p>
                                )}
                              </div>
                            </div>
                            <MarkdownPreview
                              content={isCollapsible && !isExpanded ? previewContent : result.content}
                              fullWidth
                            />
                            {isCollapsible && (
                              <button
                                onClick={() => toggleResultExpanded(result.id)}
                                className="mt-2 text-sm text-interactive-primary hover:text-interactive-primary-hover font-medium flex items-center"
                              >
                                {isExpanded ? (
                                  <>
                                    <svg className="w-4 h-4 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 15l7-7 7 7" />
                                    </svg>
                                    Show less
                                  </>
                                ) : (
                                  <>
                                    <svg className="w-4 h-4 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                                    </svg>
                                    Show full content
                                  </>
                                )}
                              </button>
                            )}
                          </div>
                        );
                      })}
                    </div>
                  );
                })()}
              </>
            ) : (
              // Polls Tab
              <PhaseHistoryPolls gameId={gameId} phaseId={selectedPhaseId} isGM={isGM} isAudience={isAudience} />
            )}
          </div>
        )}
      </div>
    );
  }

  return (
    <div className="surface-base rounded-lg shadow-md p-6">
      <div className="mb-6">
        <h2 className="text-2xl font-bold text-content-primary mb-2">History</h2>
        <p className="text-content-secondary">
          View Common Room discussions from previous phases
        </p>
      </div>

      {phases.length === 0 ? (
        <div className="text-center py-8 text-content-tertiary">
          <svg className="w-12 h-12 mx-auto mb-3 opacity-50" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <p>No phases yet</p>
          <p className="text-sm">Phases will appear here as they are created</p>
        </div>
      ) : (
        <div className="space-y-3">
          {phases.map((phase) => {
            const phaseLabel = getActionPhaseLabel(phase);
            const phaseColorClass = getActionPhaseColor(phase);
            const isActive = phase.id === currentPhaseId;

            // All phases use the same card layout
            return (
              <Button
                key={phase.id}
                variant="ghost"
                onClick={() => setSelectedPhaseId(phase.id)}
                className={`w-full justify-start text-left border rounded-lg p-4 hover:border-theme-subtle ${
                  isActive ? 'border-interactive-primary bg-interactive-primary-subtle' : 'border-theme-default'
                }`}
              >
                {/* Mobile: Vertical Stack Layout */}
                <div className="md:hidden flex flex-col items-start gap-3">
                  {/* Badge + Active indicator */}
                  <div className="flex items-center gap-2 flex-wrap">
                    <span className={`px-2.5 py-1 text-xs rounded-full font-medium border whitespace-nowrap ${phaseColorClass}`}>
                      Phase {phase.phase_number}
                    </span>
                    {isActive && (
                      <span className="px-2 py-1 text-xs bg-interactive-primary-subtle text-interactive-primary rounded-full font-medium whitespace-nowrap">
                        Active
                      </span>
                    )}
                  </div>

                  {/* Title + Description */}
                  <div className="w-full">
                    <h4 className="font-semibold text-base text-content-primary mb-1 text-left">
                      {phase.title || phaseLabel}
                    </h4>
                    {phase.description && (
                      <p className="text-sm text-content-secondary leading-relaxed text-left">{phase.description}</p>
                    )}
                  </div>
                </div>

                {/* Desktop: Grid Layout for consistent alignment */}
                <div className="hidden md:grid md:grid-cols-[auto_1fr_auto] md:gap-4 md:items-start">
                  {/* Badge - fixed width column */}
                  <span className={`px-2 py-1 text-xs rounded-full font-medium border whitespace-nowrap ${phaseColorClass}`}>
                    Phase {phase.phase_number}
                  </span>

                  {/* Title + Description - flexible column */}
                  <div>
                    <h4 className="font-medium text-content-primary">{phase.title || phaseLabel}</h4>
                    {phase.description && (
                      <p className="text-sm text-content-secondary mt-1">{phase.description}</p>
                    )}
                  </div>

                  {/* Active indicator - fixed width column */}
                  <div className="flex justify-end">
                    {isActive && (
                      <span className="px-2 py-1 text-xs bg-interactive-primary-subtle text-interactive-primary rounded-full font-medium whitespace-nowrap">
                        Active
                      </span>
                    )}
                  </div>
                </div>
              </Button>
            );
          })}
        </div>
      )}
    </div>
  );
}
