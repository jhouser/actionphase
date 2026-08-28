import { usePollsByPhase } from '../hooks';
import { PollCard } from './PollCard';
import { Alert, Spinner } from './ui';

interface PhaseHistoryPollsProps {
  gameId: number;
  phaseId: number;
  isGM: boolean;
  isAudience?: boolean;
}

/**
 * Displays polls for a historical phase
 * Used in the History tab to show polls from past phases
 */
export function PhaseHistoryPolls({ gameId, phaseId, isGM, isAudience = false }: PhaseHistoryPollsProps) {
  const { data: polls, isLoading, error } = usePollsByPhase(gameId, phaseId);

  if (isLoading) {
    return (
      <div className="flex justify-center py-8">
        <Spinner size="md" label="Loading polls..." />
      </div>
    );
  }

  if (error) {
    return (
      <Alert variant="danger" title="Error Loading Polls">
        {error instanceof Error ? error.message : 'Failed to load polls for this phase.'}
      </Alert>
    );
  }

  if (polls.length === 0) {
    return (
      <div className="p-6 surface-raised border border-theme-default rounded-lg text-center">
        <svg
          className="mx-auto h-12 w-12 text-content-tertiary mb-3"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01"
          />
        </svg>
        <h3 className="text-lg font-medium text-content-primary mb-1">No polls for this phase</h3>
        <p className="text-content-secondary">
          There were no polls created during this phase.
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <p className="text-sm text-content-secondary mb-4">
        Showing {polls.length} {polls.length === 1 ? 'poll' : 'polls'} from this phase
      </p>
      {polls.map((poll) => (
        <PollCard key={poll.id} poll={poll} gameId={gameId} isGM={isGM} isAudience={isAudience} />
      ))}
    </div>
  );
}
