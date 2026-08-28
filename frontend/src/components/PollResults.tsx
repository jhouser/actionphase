import type { PollResults as PollResultsType, Poll } from '../types/polls';

interface PollResultsProps {
  results: PollResultsType;
  poll: Poll;
  isGM?: boolean;
  isAudience?: boolean;
  isExpired?: boolean;
}

export function PollResults({ results, poll, isGM = false, isAudience = false, isExpired = false }: PollResultsProps) {
  // Use total votes from backend (includes "other" votes)
  const totalVotes = results.total_votes;

  // Sort options by vote count (descending)
  const sortedOptions = [...results.option_results].sort((a, b) => b.vote_count - a.vote_count);

  // GMs and audience can see individual votes and other responses even when poll.show_individual_votes is false
  const canSeeDetails = poll.show_individual_votes || isGM || isAudience;

  // Detect ties: count how many options share the highest vote count
  const highestVoteCount = sortedOptions[0]?.vote_count || 0;
  const winnersCount = sortedOptions.filter(opt => opt.vote_count === highestVoteCount && highestVoteCount > 0).length;
  const isTie = winnersCount > 1;

  return (
    <div className="space-y-4">
      {/* Results Header */}
      <div className="flex justify-between items-center pb-2 border-b border-theme-default">
        <h5 className="font-semibold text-content-primary">Results</h5>
        <span className="text-sm text-content-secondary">
          {totalVotes} {totalVotes === 1 ? 'vote' : 'votes'}
        </span>
      </div>

      {/* No votes yet */}
      {totalVotes === 0 ? (
        <div className="text-center py-6 text-content-secondary">
          No votes yet
        </div>
      ) : (
        <div className="space-y-3">
          {sortedOptions.map((option) => {
            const percentage = totalVotes > 0 ? (option.vote_count / totalVotes) * 100 : 0;
            const isWinning = option.vote_count === sortedOptions[0].vote_count && option.vote_count > 0;

            return (
              <div key={option.poll_option_id || 'other'} className="space-y-2">
                {/* Option Header */}
                <div className="flex justify-between items-center">
                  <span className="text-sm font-medium text-content-secondary">
                    {option.option_text || 'Other responses'}
                    {isWinning && sortedOptions[0].vote_count > 0 && (
                      isExpired ? (
                        isTie ? (
                          <span className="ml-2 px-2 py-0.5 text-xs font-bold text-content-primary bg-semantic-warning-subtle rounded">
                            TIE
                          </span>
                        ) : (
                          <span className="ml-2 px-2 py-0.5 text-xs font-bold text-content-primary bg-semantic-success-subtle rounded">
                            WINNER
                          </span>
                        )
                      ) : (
                        <span className="ml-2 text-xs font-semibold text-semantic-success">● Leading</span>
                      )
                    )}
                  </span>
                  <span className="text-sm text-content-secondary">
                    {option.vote_count} ({percentage.toFixed(1)}%)
                  </span>
                </div>

                {/* Progress Bar */}
                <div className="w-full surface-sunken rounded-full h-2 overflow-hidden">
                  <div
                    className={`h-full transition-all duration-300 ${
                      isWinning ? 'bg-interactive-primary' : 'bg-interactive-secondary'
                    }`}
                    style={{ width: `${percentage}%` }}
                  />
                </div>

                {/* Individual Voters (if enabled or GM/Audience) */}
                {canSeeDetails && option.voters && option.voters.length > 0 && (
                  <div className="ml-4 text-xs text-content-secondary">
                    {option.voters.map((voter, idx) => (
                      <span key={idx}>
                        {voter.is_anonymous ? (
                          <span className="italic">Anonymous</span>
                        ) : (
                          voter.character_name
                        )}
                        {voter.other_response && (
                          <span className="italic"> - "{voter.other_response}"</span>
                        )}
                        {idx < (option.voters?.length ?? 0) - 1 && ', '}
                      </span>
                    ))}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}

      {/* Other Responses - Show full list to GM/Audience, count only to players */}
      {results.other_responses.length > 0 && (
        <div className="pt-4 border-t border-theme-default">
          {canSeeDetails ? (
            <div>
              <h6 className="text-sm font-semibold text-content-primary mb-2">
                Other Responses ({results.other_responses.length})
              </h6>
              <div className="space-y-2">
                {results.other_responses.map((response) => (
                  <div key={response.vote_id} className="text-sm text-content-secondary">
                    <span className={`font-medium text-content-secondary${response.is_anonymous ? ' italic' : ''}`}>
                      {response.is_anonymous ? 'Anonymous' : response.character_name}:
                    </span>{' '}
                    <span className="italic">"{response.other_text}"</span>
                  </div>
                ))}
              </div>
            </div>
          ) : (
            <div className="text-xs text-content-secondary italic">
              {results.other_responses.length} custom {results.other_responses.length === 1 ? 'response' : 'responses'}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
