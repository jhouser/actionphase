import { useState, useEffect } from 'react';
import { usePoll, usePollResults, usePolls } from '../hooks';
import { Card, CardHeader, CardBody, Button, Badge, Spinner } from './ui';
import { MarkdownPreview } from './MarkdownPreview';
import { PollVotingForm } from './PollVotingForm';
import { PollResults } from './PollResults';
import { ConfirmModal } from './ConfirmModal';
import type { Poll } from '../types/polls';

interface PollCardProps {
  poll: Poll;
  gameId: number;
  isGM: boolean;
  isAudience?: boolean;
  gameState?: string;
}

export function PollCard({ poll, gameId, isGM, isAudience = false, gameState }: PollCardProps) {
  const [showVotingForm, setShowVotingForm] = useState(false);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);

  // Get delete mutation from usePolls hook
  const { deletePollMutation } = usePolls(gameId);

  // Compute is_expired client-side (backend doesn't provide it)
  const isExpired = new Date(poll.deadline) < new Date();

  // Results of a hidden-results poll are withheld from players permanently — the
  // backend rejects the request, so never ask for them. GM/audience are exempt.
  const resultsHiddenFromMe =
    poll.hide_results_from_players && !isGM && !isAudience;

  // Running totals let players view an active poll's results, the same live view
  // GMs and audience already have. The backend applies the same rule.
  const canViewLiveResults = isGM || isAudience || poll.show_running_totals_to_players;

  // Calculate initial showResults state with permission check
  // Players can only see results on expired polls, unless the GM enabled running totals
  // GMs and audience can see results anytime (even on active polls)
  const initialShowResults =
    !resultsHiddenFromMe &&
    (isExpired || poll.user_has_voted) && // User has voted or poll expired
    (isExpired || canViewLiveResults);    // AND user has permission to view

  const [showResults, setShowResults] = useState(initialShowResults);

  // Sync showResults state when poll data changes (after cache invalidation)
  useEffect(() => {
    // Determine if results should be shown based on user permissions
    const shouldShowResults =
      !resultsHiddenFromMe &&
      (isExpired || poll.user_has_voted) && // User has voted or poll expired
      (isExpired || canViewLiveResults);    // AND user has permission to view

    setShowResults(shouldShowResults);
  }, [isExpired, poll.user_has_voted, canViewLiveResults, resultsHiddenFromMe]);

  // Who may cast a vote: players always, audience only when the GM enabled it.
  // GMs never vote on their own polls.
  const canVote = !isGM && (!isAudience || poll.allow_audience_voting);

  // Fetch full poll details when showing vote form or when the voter has voted (to get their vote choice)
  const needFullPoll = showVotingForm || (canVote && !isExpired && poll.user_has_voted);
  const { data: fullPoll, isLoading: pollLoading } = usePoll(needFullPoll ? poll.id : null);

  // Fetch results when showing results
  const { data: results, isLoading: resultsLoading } = usePollResults(showResults ? poll.id : null);

  const formatDeadline = (deadline: string) => {
    const date = new Date(deadline);
    const now = new Date();
    const isExpired = date < now;

    return {
      text: date.toLocaleString('en-US', {
        month: 'short',
        day: 'numeric',
        year: 'numeric',
        hour: 'numeric',
        minute: '2-digit'
      }),
      isExpired
    };
  };

  const deadlineInfo = formatDeadline(poll.deadline);

  const handleDeletePoll = async () => {
    try {
      await deletePollMutation.mutateAsync(poll.id);
      setShowDeleteConfirm(false);
    } catch (_error) {
      // Error is handled by React Query mutation error state
      // User feedback is shown via the mutation's onError handler
    }
  };

  return (
    <Card variant="default" padding="md">
      <CardHeader>
        <div className="flex justify-between items-start gap-4">
          <div className="flex-1">
            <h4 className="text-lg font-semibold text-text-heading">{poll.question}</h4>

            {/* Metadata row */}
            <div className="flex items-center gap-3 mt-2 text-sm text-text-secondary">
              <span>
                {deadlineInfo.isExpired ? 'Ended' : 'Ends'}: {deadlineInfo.text}
              </span>
            </div>
          </div>

          {/* Status Badge */}
          <div className="flex items-center gap-2">
            {isExpired ? (
              <Badge variant="secondary">Expired</Badge>
            ) : poll.user_has_voted ? (
              <Badge variant="success">Voted</Badge>
            ) : canVote ? (
              <Badge variant="warning">Not Voted</Badge>
            ) : null}
          </div>
        </div>
      </CardHeader>

      <CardBody>
        {/* Description */}
        {poll.description && (
          <div className="mb-4">
            <MarkdownPreview content={poll.description} />
          </div>
        )}

        {/* Your vote summary: shown to anyone who has voted on an active poll */}
        {canVote && !isExpired && poll.user_has_voted && !showVotingForm && (
          <div className="mb-4 p-3 rounded-lg bg-bg-secondary border border-border-primary text-sm">
            <span className="font-medium text-text-secondary">Your vote: </span>
            {fullPoll?.user_vote_other_response ? (
              <span className="text-text-primary italic">"{fullPoll.user_vote_other_response}"</span>
            ) : fullPoll?.user_vote_option_id ? (
              <span className="text-text-primary">
                {fullPoll.options?.find(o => o.id === fullPoll.user_vote_option_id)?.option_text ?? '—'}
              </span>
            ) : null}
          </div>
        )}

        {/* Voting Form or Results */}
        {showVotingForm && !isExpired ? (
          pollLoading ? (
            <div className="flex justify-center py-8">
              <Spinner size="md" label="Loading poll options..." />
            </div>
          ) : fullPoll ? (
            <PollVotingForm
              poll={fullPoll}
              onSuccess={() => {
                setShowVotingForm(false);
                // Only show results if user is allowed to view them
                // Players can only see results on expired polls
                // GMs and audience can see results anytime
                if (!resultsHiddenFromMe && (isExpired || canViewLiveResults)) {
                  setShowResults(true);
                }
              }}
              onCancel={() => {
                setShowVotingForm(false);
              }}
            />
          ) : null
        ) : showResults ? (
          resultsLoading ? (
            <div className="flex justify-center py-8">
              <Spinner size="md" label="Loading results..." />
            </div>
          ) : results ? (
            <PollResults results={results} poll={poll} isGM={isGM} isAudience={isAudience} isExpired={isExpired} />
          ) : null
        ) : null}

        {/* Action Buttons */}
        {!showVotingForm && !isExpired && (
          <div className="flex gap-2 mt-4">
            {!poll.user_has_voted && canVote && (
              <Button
                variant="primary"
                onClick={() => setShowVotingForm(true)}
              >
                Vote Now
              </Button>
            )}
            {/* Show "Change Vote" button if user has already voted */}
            {poll.user_has_voted && canVote && (
              <Button
                variant="secondary"
                onClick={() => setShowVotingForm(true)}
              >
                Change Vote
              </Button>
            )}
            {/* Show Results button: GM and audience can always see; players only
                after expiration, or live when the GM enabled running totals */}
            {canViewLiveResults && !resultsHiddenFromMe && (
              <Button
                variant="secondary"
                onClick={() => setShowResults(!showResults)}
              >
                {showResults ? 'Hide Results' : 'Show Results'}
              </Button>
            )}
            {/* Delete button: GM only, not in completed/cancelled games */}
            {isGM && gameState !== 'completed' && gameState !== 'cancelled' && (
              <Button
                variant="danger"
                onClick={() => setShowDeleteConfirm(true)}
              >
                Delete Poll
              </Button>
            )}
          </div>
        )}

        {/* Hidden results: explain the absence rather than leaving a dead card */}
        {resultsHiddenFromMe && !showVotingForm && (
          <div
            className="mt-4 flex items-start gap-2 p-3 rounded-lg bg-bg-secondary border border-border-primary"
            data-testid="poll-results-hidden-notice"
          >
            <span aria-hidden="true">🔒</span>
            <p className="text-sm text-text-secondary">
              {isExpired
                ? 'This poll is closed. The GM has hidden the results.'
                : 'The GM has hidden the results of this poll.'}
            </p>
          </div>
        )}

        {isExpired && !showResults && !resultsHiddenFromMe && (
          <Button
            variant="secondary"
            onClick={() => setShowResults(true)}
            className="mt-4"
          >
            Show Results
          </Button>
        )}
      </CardBody>

      {/* Delete Confirmation Modal */}
      <ConfirmModal
        isOpen={showDeleteConfirm}
        onClose={() => setShowDeleteConfirm(false)}
        onConfirm={handleDeletePoll}
        title="Delete Poll"
        message="Are you sure you want to delete this poll? All votes and results will be permanently removed. This action cannot be undone."
        confirmText="Delete Poll"
        variant="danger"
        isLoading={deletePollMutation.isPending}
      />
    </Card>
  );
}
