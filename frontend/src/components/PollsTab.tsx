import { useState, useEffect, useRef } from 'react';
import { useSearchParams } from 'react-router-dom';
import { usePollsByPhase } from '../hooks';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '../lib/api';
import { Button, Alert, Spinner } from './ui';
import { CreatePollForm } from './CreatePollForm';
import { PollCard } from './PollCard';
import { useGameContext } from '../contexts/GameContext';
import type { CreatePollRequest } from '../types/polls';

interface PollsTabProps {
  gameId: number;
  phaseId?: number; // Optional: if provided, shows polls for this specific phase
  isGM: boolean;
  isCurrentPhase: boolean;
  isAudience?: boolean;
  gameState?: string;
}

export function PollsTab({ gameId, phaseId, isGM, isCurrentPhase, isAudience = false, gameState }: PollsTabProps) {
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [includeExpired, setIncludeExpired] = useState(false);
  const { currentPhaseId } = useGameContext();
  const [searchParams, setSearchParams] = useSearchParams();
  const queryClient = useQueryClient();

  // Use provided phaseId or fall back to current phase from context
  const effectivePhaseId = phaseId ?? currentPhaseId;

  // Use phase-specific polls instead of game-wide polls
  const { data: polls, isLoading } = usePollsByPhase(gameId, effectivePhaseId || 0);

  // Create poll mutation with phase-specific cache invalidation
  const createPollMutation = useMutation({
    mutationFn: (data: CreatePollRequest) => apiClient.polls.createPoll(gameId, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['polls', 'by-phase', gameId, effectivePhaseId] });
    }
  });

  const activePolls = polls.filter(poll => !poll.is_expired);
  const expiredPolls = polls.filter(poll => poll.is_expired);

  // Deep-link from a poll deadline card: scroll to the poll and highlight it.
  // Mirrors the Common Room's ?comment= handling.
  const pollIdParam = searchParams.get('poll');
  const scrollAttemptedRef = useRef<string | null>(null);

  useEffect(() => {
    if (!pollIdParam || isLoading || scrollAttemptedRef.current === pollIdParam) {
      return;
    }

    // An expired poll is collapsed behind the toggle, so reveal it before
    // scrolling — otherwise the anchor isn't in the DOM at all. Derived here
    // rather than from the render-scope `expiredPolls`, which is a fresh array
    // identity every render and would churn this effect as a dependency.
    const targetIsExpired = polls.some(
      poll => String(poll.id) === pollIdParam && poll.is_expired
    );
    if (targetIsExpired && !includeExpired) {
      setIncludeExpired(true);
      return; // Re-runs once the expired section renders.
    }

    scrollAttemptedRef.current = pollIdParam;

    requestAnimationFrame(() => {
      const element = document.getElementById(`poll-${pollIdParam}`);
      if (element) {
        if (typeof element.scrollIntoView === 'function') {
          element.scrollIntoView({ behavior: 'smooth', block: 'center' });
        }
        element.classList.add('ring-2', 'ring-interactive-primary');
        setTimeout(() => {
          element.classList.remove('ring-2', 'ring-interactive-primary');
        }, 5000);
      }

      // Clear the param so a later re-render (or a browser refresh) doesn't
      // re-trigger the scroll, matching the comment deep-link behavior. The
      // functional form reads the params live: a tab switch landing in the same
      // frame would otherwise be reverted by a stale snapshot.
      setSearchParams(current => {
        const newParams = new URLSearchParams(current);
        newParams.delete('poll');
        return newParams;
      }, { replace: true });
    });
  }, [pollIdParam, isLoading, polls, includeExpired, setSearchParams]);

  if (isLoading) {
    return (
      <div className="flex justify-center items-center py-12">
        <Spinner size="lg" label="Loading polls..." />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header Section */}
      <div className="flex justify-between items-start">
        <div>
          <h3 className="text-lg font-semibold text-text-heading">Polls</h3>
          <p className="text-sm text-text-secondary mt-1">
            {isGM
              ? 'Create polls to gather input from players and characters.'
              : 'Vote on active polls to share your input.'}
          </p>
        </div>

        {/* GM Create Poll Button */}
        {isGM && isCurrentPhase && (
          <Button
            variant="primary"
            onClick={() => setShowCreateForm(!showCreateForm)}
          >
            {showCreateForm ? 'Cancel' : 'Create Poll'}
          </Button>
        )}
      </div>

      {/* Create Poll Form (GM only) */}
      {isGM && showCreateForm && (
        <CreatePollForm
          gameId={gameId}
          currentPhaseId={effectivePhaseId || undefined}
          onSuccess={() => setShowCreateForm(false)}
          onCancel={() => setShowCreateForm(false)}
        />
      )}

      {/* Show Expired Toggle */}
      {expiredPolls.length > 0 && (
        <div className="flex items-center gap-2">
          <input
            type="checkbox"
            id="show-expired"
            checked={includeExpired}
            onChange={(e) => setIncludeExpired(e.target.checked)}
            className="rounded border-border-primary text-accent-primary focus:ring-accent-primary"
          />
          <label htmlFor="show-expired" className="text-sm text-text-secondary cursor-pointer">
            Show expired polls ({expiredPolls.length})
          </label>
        </div>
      )}

      {/* Active Polls Section */}
      {activePolls.length === 0 && expiredPolls.length === 0 ? (
        <div className="bg-bg-secondary border border-border-primary rounded-lg p-8 text-center">
          <svg
            className="mx-auto h-12 w-12 text-text-tertiary mb-3"
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
          <h3 className="text-lg font-medium text-text-heading mb-1">No polls yet</h3>
          <p className="text-text-secondary">
            {isGM ? 'Create a poll to gather input from your players.' : 'Check back later for polls from the GM.'}
          </p>
        </div>
      ) : (
        <>
          {/* Active Polls */}
          {activePolls.length > 0 && (
            <div className="space-y-4">
              <h4 className="text-sm font-semibold text-text-secondary uppercase tracking-wide">
                Active Polls ({activePolls.length})
              </h4>
              {activePolls.map((poll) => (
                <PollCard key={poll.id} poll={poll} gameId={gameId} isGM={isGM} isAudience={isAudience} gameState={gameState} />
              ))}
            </div>
          )}

          {/* Expired Polls (if showing) */}
          {includeExpired && expiredPolls.length > 0 && (
            <div className="space-y-4">
              <h4 className="text-sm font-semibold text-text-secondary uppercase tracking-wide">
                Expired Polls ({expiredPolls.length})
              </h4>
              {expiredPolls.map((poll) => (
                <PollCard key={poll.id} poll={poll} gameId={gameId} isGM={isGM} isAudience={isAudience} gameState={gameState} />
              ))}
            </div>
          )}
        </>
      )}

      {/* Error Handling */}
      {createPollMutation.isError && (
        <Alert variant="danger" title="Error Creating Poll">
          {createPollMutation.error instanceof Error
            ? createPollMutation.error.message
            : 'Failed to create poll. Please try again.'}
        </Alert>
      )}
    </div>
  );
}
