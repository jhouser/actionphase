import { useState } from 'react';
import { usePolls } from '../hooks';
import { Button, Input, Textarea, Card, CardBody, Alert, Checkbox, DateTimeInput } from './ui';
import type { CreatePollRequest } from '../types/polls';
import { logger } from '@/services/LoggingService';
import { localDateTimeToUTC } from '../utils/timezone';

interface CreatePollFormProps {
  gameId: number;
  currentPhaseId?: number;
  onSuccess: () => void;
  onCancel: () => void;
}

export function CreatePollForm({ gameId, currentPhaseId, onSuccess, onCancel }: CreatePollFormProps) {
  const { createPollMutation } = usePolls(gameId);

  const [question, setQuestion] = useState('');
  const [description, setDescription] = useState('');
  const [deadline, setDeadline] = useState('');
  const [showIndividualVotes, setShowIndividualVotes] = useState(false);
  const [allowOtherOption, setAllowOtherOption] = useState(false);
  const [hideResultsFromPlayers, setHideResultsFromPlayers] = useState(false);
  const [allowAudienceVoting, setAllowAudienceVoting] = useState(false);
  const [showRunningTotals, setShowRunningTotals] = useState(false);
  const [options, setOptions] = useState<string[]>(['', '']);
  const [error, setError] = useState<string | null>(null);

  const handleAddOption = () => {
    setOptions([...options, '']);
  };

  const handleRemoveOption = (index: number) => {
    if (options.length > 2) {
      setOptions(options.filter((_, i) => i !== index));
    }
  };

  // "Hide results from players" is mutually exclusive with both disclosure options:
  // "Show individual votes" reveals per-voter attribution and "Show running totals"
  // reveals results early, while hiding withholds results entirely. Selecting any of
  // them clears the conflicting one so an invalid combination is unreachable.
  const disclosesResultsToPlayers = showIndividualVotes || showRunningTotals;

  const handleShowIndividualVotesChange = (checked: boolean) => {
    setShowIndividualVotes(checked);
    if (checked) {
      setHideResultsFromPlayers(false);
    }
  };

  const handleShowRunningTotalsChange = (checked: boolean) => {
    setShowRunningTotals(checked);
    if (checked) {
      setHideResultsFromPlayers(false);
    }
  };

  const handleHideResultsChange = (checked: boolean) => {
    setHideResultsFromPlayers(checked);
    if (checked) {
      setShowIndividualVotes(false);
      setShowRunningTotals(false);
    }
  };

  const handleOptionChange = (index: number, value: string) => {
    const newOptions = [...options];
    newOptions[index] = value;
    setOptions(newOptions);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    // Validation
    if (!question.trim()) {
      setError('Question is required');
      return;
    }

    if (!deadline) {
      setError('Deadline is required');
      return;
    }

    const filledOptions = options.filter(opt => opt.trim() !== '');
    if (filledOptions.length < 2) {
      setError('At least 2 options are required');
      return;
    }

    // Build request
    // Convert datetime-local format from user's local time to UTC
    const deadlineISO = localDateTimeToUTC(deadline);

    const request: CreatePollRequest = {
      question: question.trim(),
      description: description.trim() || undefined,
      deadline: deadlineISO,
      show_individual_votes: showIndividualVotes,
      allow_other_option: allowOtherOption,
      hide_results_from_players: hideResultsFromPlayers,
      allow_audience_voting: allowAudienceVoting,
      show_running_totals_to_players: showRunningTotals,
      phase_id: currentPhaseId,
      options: filledOptions.map((text, index) => ({ text, display_order: index }))
    };

    try {
      await createPollMutation.mutateAsync(request);
      onSuccess();
    } catch (_err) {
      logger.error('Failed to create poll', { error: _err, gameId, question });
      setError(_err instanceof Error ? _err.message : 'Failed to create poll');
    }
  };

  return (
    <Card variant="bordered" padding="md">
      <CardBody>
        <form onSubmit={handleSubmit} className="space-y-4">
          <h4 className="text-lg font-semibold text-text-heading mb-4">Create New Poll</h4>

          {/* Question */}
          <Input
            label="Question"
            required
            value={question}
            onChange={(e) => setQuestion(e.target.value)}
            placeholder="What would you like to ask?"
            maxLength={500}
          />

          {/* Description */}
          <Textarea
            label="Description (Optional)"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Provide additional context or instructions..."
            rows={3}
          />

          {/* Deadline */}
          <DateTimeInput
            label="Deadline"
            required
            value={deadline}
            onChange={(e) => setDeadline(e.target.value)}
          />

          {/* Options */}
          <div className="space-y-2">
            <label className="block text-sm font-medium text-text-primary">
              Options (minimum 2)
            </label>
            {options.map((option, index) => (
              <div key={index} className="flex gap-2">
                <Input
                  value={option}
                  onChange={(e) => handleOptionChange(index, e.target.value)}
                  placeholder={`Option ${index + 1}`}
                  className="flex-1"
                />
                {options.length > 2 && (
                  <Button
                    type="button"
                    variant="danger"
                    onClick={() => handleRemoveOption(index)}
                  >
                    Remove
                  </Button>
                )}
              </div>
            ))}
            <Button
              type="button"
              variant="secondary"
              onClick={handleAddOption}
              className="w-full"
            >
              Add Option
            </Button>
          </div>

          {/* Settings */}
          <div className="space-y-3 border-t border-border-primary pt-4">
            <Checkbox
              id="poll-show-individual-votes"
              label="Show individual votes to all players"
              helperText={
                hideResultsFromPlayers
                  ? "Unavailable while results are hidden from players"
                  : undefined
              }
              disabled={hideResultsFromPlayers}
              checked={showIndividualVotes}
              onChange={(e) => handleShowIndividualVotesChange(e.target.checked)}
            />
            <Checkbox
              id="poll-show-running-totals"
              label="Show running totals to players"
              helperText={
                hideResultsFromPlayers
                  ? "Unavailable while results are hidden from players"
                  : showIndividualVotes
                    ? "Players see the live results, including who voted for what."
                    : "Players see the live vote counts before the poll closes, without seeing who voted for what."
              }
              disabled={hideResultsFromPlayers}
              checked={showRunningTotals}
              onChange={(e) => handleShowRunningTotalsChange(e.target.checked)}
            />
            <Checkbox
              id="poll-hide-results"
              label="Hide results from players"
              helperText="Players can vote but never see the results, even after the poll closes. You and the audience still see them."
              disabled={disclosesResultsToPlayers}
              checked={hideResultsFromPlayers}
              onChange={(e) => handleHideResultsChange(e.target.checked)}
            />
            <Checkbox
              id="poll-allow-audience-voting"
              label="Allow audience members to vote"
              helperText="Audience votes count toward the totals but are always shown as 'Anonymous'."
              checked={allowAudienceVoting}
              onChange={(e) => setAllowAudienceVoting(e.target.checked)}
            />
            <Checkbox
              id="poll-allow-other-option"
              label="Allow 'Other' text responses"
              checked={allowOtherOption}
              onChange={(e) => setAllowOtherOption(e.target.checked)}
            />
          </div>

          {/* Error Display */}
          {error && (
            <Alert variant="danger" title="Error">
              {error}
            </Alert>
          )}

          {/* Form Actions */}
          <div className="flex gap-2 justify-end pt-4">
            <Button
              type="button"
              variant="secondary"
              onClick={onCancel}
              disabled={createPollMutation.isPending}
            >
              Cancel
            </Button>
            <Button
              type="submit"
              variant="primary"
              loading={createPollMutation.isPending}
            >
              Create Poll
            </Button>
          </div>
        </form>
      </CardBody>
    </Card>
  );
}
