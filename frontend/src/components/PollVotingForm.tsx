import { useState } from 'react';
import { useSubmitVote } from '../hooks';
import { Button, Alert, Input } from './ui';
import type { PollWithOptions, SubmitVoteRequest } from '../types/polls';
import { logger } from '@/services/LoggingService';

interface PollVotingFormProps {
  poll: PollWithOptions;
  onSuccess: () => void;
  onCancel: () => void;
}

export function PollVotingForm({ poll, onSuccess, onCancel }: PollVotingFormProps) {
  const submitVoteMutation = useSubmitVote(poll.id, poll.game_id);

  const [selectedOptionId, setSelectedOptionId] = useState<number | null>(null);
  const [otherText, setOtherText] = useState('');
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (!selectedOptionId && !otherText.trim()) {
      setError('Please select an option or provide a custom response');
      return;
    }

    try {
      const voteData: Partial<SubmitVoteRequest> = {};

      if (selectedOptionId !== null && selectedOptionId !== -1) {
        voteData.selected_option_id = selectedOptionId;
      }

      if (otherText.trim()) {
        voteData.other_response = otherText.trim();
      }

      await submitVoteMutation.mutateAsync(voteData);
      onSuccess();
    } catch (_err) {
      logger.error('Failed to submit vote', { error: _err, pollId: poll.id, gameId: poll.game_id });
      setError(_err instanceof Error ? _err.message : 'Failed to submit vote');
    }
  };

  const isOtherSelected = selectedOptionId === -1;

  return (
    <form onSubmit={handleSubmit} className="space-y-4 border-t border-theme-default pt-4">
      {/* Options */}
      <div>
        <label className="block text-sm font-medium text-content-secondary mb-2">
          Select your response
        </label>
        <div className="space-y-2">
          {poll.options.map((option) => (
            <label
              key={option.id}
              className={`flex items-start gap-3 p-3 rounded-lg border cursor-pointer transition-colors ${
                selectedOptionId === option.id
                  ? 'border-interactive-primary bg-interactive-primary-subtle'
                  : 'border-theme-default hover:border-theme-strong surface-raised'
              }`}
            >
              <input
                type="radio"
                name="poll-option"
                value={option.id}
                checked={selectedOptionId === option.id}
                onChange={() => {
                  setSelectedOptionId(option.id);
                  setOtherText('');
                }}
                className="mt-1 text-accent-primary focus:ring-accent-primary"
              />
              <span className="flex-1 text-sm text-content-secondary">{option.option_text}</span>
            </label>
          ))}

          {/* "Other" Option */}
          {poll.allow_other_option && (
            <label
              className={`flex items-start gap-3 p-3 rounded-lg border cursor-pointer transition-colors ${
                isOtherSelected
                  ? 'border-interactive-primary bg-interactive-primary-subtle'
                  : 'border-theme-default hover:border-theme-strong surface-raised'
              }`}
            >
              <input
                type="radio"
                name="poll-option"
                value="-1"
                checked={isOtherSelected}
                onChange={() => {
                  setSelectedOptionId(-1);
                }}
                className="mt-1 text-accent-primary focus:ring-accent-primary"
              />
              <div className="flex-1 space-y-2">
                <span className="text-sm text-content-secondary">Other (specify below)</span>
                {isOtherSelected && (
                  <Input
                    value={otherText}
                    onChange={(e) => setOtherText(e.target.value)}
                    placeholder="Enter your custom response..."
                    required={isOtherSelected}
                    className="mt-2"
                    onClick={(e) => e.stopPropagation()}
                  />
                )}
              </div>
            </label>
          )}
        </div>
      </div>

      {/* Error Display */}
      {error && (
        <Alert variant="danger" title="Error">
          {error}
        </Alert>
      )}

      {/* Form Actions */}
      <div className="flex gap-2 justify-end pt-2">
        <Button
          type="button"
          variant="secondary"
          onClick={onCancel}
          disabled={submitVoteMutation.isPending}
        >
          Cancel
        </Button>
        <Button
          type="submit"
          variant="primary"
          loading={submitVoteMutation.isPending}
          disabled={!selectedOptionId && !otherText.trim()}
        >
          Submit Vote
        </Button>
      </div>
    </form>
  );
}
