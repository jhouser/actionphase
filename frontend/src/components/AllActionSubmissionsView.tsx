import { useState, useEffect, useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useAllActionSubmissions } from '../hooks/useAudience';
import { Card, CardHeader, CardBody } from './ui/Card';
import { Badge } from './ui/Badge';
import { Spinner } from './ui/Spinner';
import { Alert } from './ui/Alert';
import { MarkdownPreview } from './MarkdownPreview';
import CharacterAvatar from './CharacterAvatar';
import { useGameContext } from '../contexts/GameContext';
import { useCharacterSheetItems } from '../hooks/useCharacterSheetItems';
import { apiClient } from '../lib/api';
import { logger } from '@/services/LoggingService';

interface AllActionSubmissionsViewProps {
  gameId: number;
}

interface ActionSubmissionType {
  id: number;
  status: string;
  action_result_id?: number;
  character_id?: number | null;
  character_name: string;
  username: string;
  submission_number: number;
  created_at: string;
  submitted_at: string;
  last_updated: string;
  content: string;
}

interface ActionResultType {
  id: number;
  action_submission_id?: number;
  phase_id: number;
  content: string;
  character_name?: string | null;
  username?: string;
  is_published?: boolean;
}

/**
 * Read-only view of all action submissions for audience members and GM
 * Features phase grouping, collapsible submissions, and action results display
 */
export function AllActionSubmissionsView({ gameId }: AllActionSubmissionsViewProps) {
  const [selectedPhaseId, setSelectedPhaseId] = useState<number | undefined>();

  // Fetch all game phases for the phase switcher
  const { data: phasesData, isLoading: phasesLoading } = useQuery({
    queryKey: ['gamePhases', gameId],
    queryFn: () => apiClient.phases.getGamePhases(gameId).then(res => res.data),
    enabled: !!gameId,
  });

  const phases = useMemo(() => phasesData || [], [phasesData]);

  // Set initial phase to the most recent action phase (highest phase number)
  useEffect(() => {
    if (phases.length > 0 && selectedPhaseId === undefined) {
      const actionPhases = phases.filter(p => p.phase_type === 'action');
      if (actionPhases.length > 0) {
        const mostRecentPhase = actionPhases.reduce((prev, current) =>
          (current.phase_number > prev.phase_number) ? current : prev
        );
        setSelectedPhaseId(mostRecentPhase.id);
      }
    }
  }, [phases, selectedPhaseId]);

  const {
    data,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
    isLoading,
    error,
  } = useAllActionSubmissions(gameId, { phaseId: selectedPhaseId });

  // Results that answer no submission (actions collected offsite) have no
  // submission card to nest under, so they are listed on their own below.
  const { data: gameResults } = useQuery({
    queryKey: ['actionResults', 'game', gameId],
    queryFn: () => apiClient.phases.getGameResults(gameId).then((res: { data: ActionResultType[] }) => res.data),
    enabled: !!gameId,
  });

  const standaloneResults = useMemo(
    () =>
      (gameResults || []).filter(
        result =>
          !result.action_submission_id &&
          result.is_published !== false &&
          (selectedPhaseId === undefined || result.phase_id === selectedPhaseId)
      ),
    [gameResults, selectedPhaseId]
  );

  // Infinite scroll handler
  useEffect(() => {
    const handleScroll = () => {
      if (
        window.innerHeight + window.scrollY >= document.documentElement.scrollHeight - 500 &&
        hasNextPage &&
        !isFetchingNextPage
      ) {
        fetchNextPage();
      }
    };

    window.addEventListener('scroll', handleScroll);
    return () => window.removeEventListener('scroll', handleScroll);
  }, [hasNextPage, isFetchingNextPage, fetchNextPage]);

  if (phasesLoading || isLoading) {
    return (
      <div className="flex justify-center items-center py-12">
        <Spinner size="lg" />
      </div>
    );
  }

  if (error) {
    return (
      <Alert variant="danger">
        Failed to load action submissions: {(error as Error).message}
      </Alert>
    );
  }

  const allSubmissions = data?.pages.flatMap((page) => page.action_submissions || []) || [];
  const total = data?.pages[0]?.total || 0;
  const selectedPhase = phases.find(p => p.id === selectedPhaseId);

  return (
    <div className="space-y-4">
      {/* Header with Read-Only Badge */}
      {/* Mobile: Vertical stack */}
      <div className="md:hidden space-y-2">
        <div className="flex items-center gap-2 flex-wrap">
          <h2 className="text-lg font-semibold text-content-primary">
            All Action Submissions
          </h2>
          <Badge variant="primary" size="sm">
            Read-Only
          </Badge>
        </div>
        <div className="text-sm text-content-secondary">
          {total} submission{total !== 1 ? 's' : ''} in {selectedPhase?.title || 'this phase'}
        </div>
      </div>
      {/* Desktop: Horizontal layout */}
      <div className="hidden md:flex items-center justify-between">
        <div className="flex items-center gap-3">
          <h2 className="text-xl font-semibold text-content-primary">
            All Action Submissions
          </h2>
          <Badge variant="primary" size="sm">
            Read-Only
          </Badge>
        </div>
        <div className="text-sm text-content-secondary">
          {total} submission{total !== 1 ? 's' : ''} in {selectedPhase?.title || 'this phase'}
        </div>
      </div>

      {/* Info Alert */}
      <Alert variant="info">
        As an audience member, you can view all action submissions and results to follow the story as it unfolds.
      </Alert>

      {/* Phase Switcher - Only show action phases */}
      {phases.filter(p => p.phase_type === 'action').length > 0 && (
        <div className="border border-border-primary rounded-lg p-4 bg-bg-secondary">
          <h3 className="text-sm font-semibold text-content-primary mb-3">Select Action Phase</h3>
          <div className="flex flex-wrap gap-2">
            {phases
              .filter(p => p.phase_type === 'action') // Only show action phases
              .sort((a, b) => b.phase_number - a.phase_number) // Most recent first
              .map((phase) => {
                const isSelected = phase.id === selectedPhaseId;
                return (
                  <button
                    key={phase.id}
                    onClick={() => setSelectedPhaseId(phase.id)}
                    className={`
                      px-3 py-1.5 rounded-lg text-sm font-medium transition-colors
                      ${isSelected
                        ? 'bg-interactive-primary text-white'
                        : 'bg-bg-primary border border-border-primary text-content-primary hover:bg-bg-tertiary'
                      }
                    `}
                  >
                    Phase {phase.phase_number}: {phase.title}
                    {phase.is_active && <span className="ml-1">●</span>}
                  </button>
                );
              })}
          </div>
        </div>
      )}

      {/* Submissions List */}
      {allSubmissions.length === 0 ? (
        <Card variant="default">
          <CardBody>
            <div className="text-center py-8 text-content-secondary">
              No action submissions in this phase yet.
            </div>
          </CardBody>
        </Card>
      ) : (
        <div className="space-y-4">
          {(allSubmissions as unknown as ActionSubmissionType[]).map((submission) => (
            <ActionSubmissionCard
              key={submission.id}
              gameId={gameId}
              submission={submission}
            />
          ))}

          {/* Load More Indicator */}
          {isFetchingNextPage && (
            <div className="flex justify-center py-4">
              <Spinner size="md" />
            </div>
          )}

          {!hasNextPage && allSubmissions.length > 0 && (
            <div className="text-center py-4 text-sm text-content-secondary">
              End of action submissions
            </div>
          )}
        </div>
      )}

      {standaloneResults.length > 0 && (
        <div className="space-y-4">
          <h3 className="text-sm font-semibold text-content-primary">
            Results without a submission
          </h3>
          {standaloneResults.map(result => (
            <StandaloneResultCard key={result.id} result={result} />
          ))}
        </div>
      )}
    </div>
  );
}

/**
 * A published result with no parent submission. Rendered on its own because
 * there is no submission card to nest it under.
 */
function StandaloneResultCard({ result }: { result: ActionResultType }) {
  const [isExpanded, setIsExpanded] = useState(false);

  return (
    <Card variant="default">
      <CardHeader>
        <button
          onClick={() => setIsExpanded(!isExpanded)}
          className="w-full text-left cursor-pointer"
        >
          <div className="flex items-center gap-2">
            <span className="text-lg">{isExpanded ? '▼' : '▶'}</span>
            <span className="font-semibold text-content-primary">
              {result.character_name || result.username || 'Unknown'}
            </span>
            <Badge variant="success" size="sm">Result</Badge>
          </div>
        </button>
      </CardHeader>

      {isExpanded && (
        <CardBody>
          <div className="bg-bg-tertiary p-3 rounded-lg border border-border-primary">
            <MarkdownPreview content={result.content} fullWidth />
          </div>
        </CardBody>
      )}
    </Card>
  );
}

/**
 * Individual action submission card component with collapsible content and result display
 */
function ActionSubmissionCard({ gameId, submission }: { gameId: number; submission: ActionSubmissionType }) {
  const [isExpanded, setIsExpanded] = useState(false);

  const { allGameCharacters, game } = useGameContext();

  // Lazy-fetch the submitting character's sheet items for [[item]] tooltip resolution
  // when expanded. Audience members (and any participant in a completed game) are
  // authorized for full sheet data by the backend; see characters.GetCharacterData.
  // Keyed per-character in the React Query cache, so repeat views cost no extra request.
  const sheetItems = useCharacterSheetItems(isExpanded && submission.character_id ? submission.character_id : null);

  const portraitAvatars = game?.portrait_avatars ?? false;
  const avatarUrl = submission.character_id
    ? (allGameCharacters.find(c => c.id === submission.character_id)?.avatar_url ?? null)
    : null;

  // Fetch this submission's results when expanded. A submission may receive
  // more than one result — the GM can stage a reveal across several — so this
  // collects every match rather than the first, and renders them in send order.
  const { data: gameResults, isLoading: loadingResults, error: resultsError } = useQuery({
    queryKey: ['actionResults', 'game', gameId],
    queryFn: () => apiClient.phases.getGameResults(gameId).then((res: { data: ActionResultType[] }) => res.data),
    enabled: isExpanded && submission.status === 'result_posted',
  });

  useEffect(() => {
    if (resultsError) {
      logger.error('Failed to load action results', {
        error: resultsError,
        gameId,
        submissionId: submission.id,
      });
    }
  }, [resultsError, gameId, submission.id]);

  const actionResults = useMemo(
    () => (gameResults || []).filter(r => r.action_submission_id === submission.id),
    [gameResults, submission.id]
  );

  const getStatusBadge = (status: string) => {
    if (status === 'submitted') {
      return <Badge variant="primary" size="sm">Submitted</Badge>;
    }
    if (status === 'result_posted') {
      return <Badge variant="success" size="sm">Result Posted</Badge>;
    }
    return <Badge variant="neutral" size="sm">Draft</Badge>;
  };

  const formatDate = (dateString: string) => {
    if (!dateString) return 'Not submitted';
    const date = new Date(dateString);
    return date.toLocaleDateString('en-US', {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  return (
    <Card variant="default">
      <CardHeader>
        <button
          onClick={() => setIsExpanded(!isExpanded)}
          className="w-full text-left cursor-pointer"
        >
          <div className="flex items-start justify-between">
            <div className="space-y-1 flex-1">
              <div className="flex items-center gap-2">
                <span className="text-lg">{isExpanded ? '▼' : '▶'}</span>
                <CharacterAvatar
                  characterName={submission.character_name}
                  avatarUrl={avatarUrl}
                  size="sm"
                  shape={portraitAvatars ? 'portrait' : 'circle'}
                />
                <span className="font-semibold text-content-primary">
                  {submission.character_name}
                </span>
                {getStatusBadge(submission.status || 'draft')}
              </div>
              <div className="text-sm text-content-secondary ml-6">
                {submission.username}
              </div>
            </div>
            <div className="text-xs text-content-secondary">
              {formatDate(submission.submitted_at)}
            </div>
          </div>
        </button>
      </CardHeader>

      {isExpanded && (
        <CardBody>
          {/* Action Submission */}
          <div className="mb-6">
            <h4 className="text-sm font-semibold text-content-primary mb-2">Action Submitted:</h4>
            <div className="bg-bg-secondary p-3 rounded-lg border border-border-primary">
              <MarkdownPreview content={submission.content} fullWidth sheetItemRefs={sheetItems} />
            </div>
          </div>

          {/* Action Result(s) */}
          {submission.status === 'result_posted' && (
            <div>
              <h4 className="text-sm font-semibold text-content-primary mb-2">
                {actionResults.length > 1 ? 'Results:' : 'Result:'}
              </h4>
              {loadingResults ? (
                <div className="flex items-center justify-center py-4">
                  <Spinner size="sm" />
                </div>
              ) : actionResults.length > 0 ? (
                <div className="space-y-3">
                  {actionResults.map(result => (
                    <div key={result.id} className="bg-bg-tertiary p-3 rounded-lg border border-border-primary">
                      <MarkdownPreview content={result.content} fullWidth sheetItemRefs={sheetItems} />
                    </div>
                  ))}
                </div>
              ) : (
                <div className="text-sm text-content-secondary italic">
                  Result not found
                </div>
              )}
            </div>
          )}

          {submission.status !== 'result_posted' && (
            <div className="text-sm text-content-secondary italic">
              No result posted yet
            </div>
          )}
        </CardBody>
      )}
    </Card>
  );
}
