import React, { useState } from 'react';
import { useCreateActionResult, useCreateStagedResultChain } from '../hooks/useActionResults';
import { useToast } from '../contexts/ToastContext';
import { Button, Alert } from './ui';
import { CommentEditor } from './CommentEditor';
import { StagedPartsEditor } from './StagedPartsEditor';
import type { StagedResultPart } from '../types/phases';
import { DEFAULT_DELAY_MINUTES, MAX_CHAIN_PARTS } from '../lib/stagedDelays';
import { logger } from '@/services/LoggingService';

interface CreateActionResultFormProps {
  gameId: number;
  userId: number;
  userName: string;
  characterId?: number;
  characterName?: string;
  actionSubmissionId?: number;
  onSuccess?: () => void;
}

export const CreateActionResultForm: React.FC<CreateActionResultFormProps> = ({
  gameId,
  userId,
  userName,
  characterId,
  characterName,
  actionSubmissionId,
  onSuccess,
}) => {
  const { showWarning } = useToast();
  const [content, setContent] = useState('');
  // Follow-up parts only; the head is `content` above. Empty means this is an
  // ordinary single-part result and the staged path is never taken.
  const [followUpParts, setFollowUpParts] = useState<StagedResultPart[]>([]);
  const createResult = useCreateActionResult(gameId);
  const createChain = useCreateStagedResultChain(gameId);

  const isStaged = followUpParts.length > 0;
  const activeMutation = isStaged ? createChain : createResult;

  // The head counts toward the server's cap, so the follow-up allowance is one
  // less. Enforced here as well as server-side because this form holds unsaved
  // text: letting the GM write an 11th part and then rejecting it on submit
  // would throw away everything they had typed.
  const canAddPart = followUpParts.length + 1 < MAX_CHAIN_PARTS;

  const addFollowUpPart = () => {
    if (!canAddPart) return;
    setFollowUpParts(parts => [...parts, { content: '', delay_minutes: DEFAULT_DELAY_MINUTES }]);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!content.trim()) {
      showWarning('Please enter result content');
      return;
    }

    if (isStaged && followUpParts.some(part => !part.content.trim())) {
      showWarning('Every part needs content before you can create the chain');
      return;
    }

    if (isStaged && followUpParts.some(part => !Number.isFinite(part.delay_minutes) || part.delay_minutes < 1)) {
      showWarning('Each follow-up part needs a delay of at least 1 minute');
      return;
    }

    try {
      if (isStaged) {
        await createChain.mutateAsync({
          user_id: userId,
          character_id: characterId,
          action_submission_id: actionSubmissionId,
          // The head's delay is always 0 — it releases on publish — and the API
          // rejects a non-zero head delay rather than ignoring it.
          parts: [
            { content: content.trim(), delay_minutes: 0 },
            ...followUpParts.map(part => ({
              content: part.content.trim(),
              delay_minutes: part.delay_minutes,
            })),
          ],
          is_published: false, // Always create as draft
        });
      } else {
        await createResult.mutateAsync({
          user_id: userId,
          character_id: characterId,
          action_submission_id: actionSubmissionId,
          content: content.trim(),
          is_published: false, // Always create as draft
        });
      }

      setContent('');
      setFollowUpParts([]);
      onSuccess?.();
    } catch (error) {
      logger.error('Failed to create action result', { error, gameId, userId, userName, characterId, characterName, actionSubmissionId, isStaged });
    }
  };

  const recipientLabel = characterName ? `${characterName} (${userName})` : userName;

  return (
    <form onSubmit={handleSubmit} className="p-4 surface-base border border-theme-default rounded shadow-sm">
      <h4 className="font-semibold text-content-primary mb-2">Send Result to {recipientLabel}</h4>

      <div className="mb-4">
        <label className="block text-sm font-medium text-content-primary mb-1">
          {isStaged ? 'Result Content 1' : 'Result Content'}
        </label>
        <CommentEditor
          id="content"
          textareaTestId="result-content"
          value={content}
          onChange={setContent}
          rows={4}
          placeholder="Enter the result of the player's action..."
          maxLength={100000}
          warnOnUnsavedChanges
          showCharacterCount={true}
        />
        <p className="mt-1 text-xs text-content-tertiary">Maximum 100,000 characters. Result will be created as a draft.</p>
      </div>

      {isStaged && (
        <div className="mb-4">
          <StagedPartsEditor
            parts={followUpParts}
            onChange={setFollowUpParts}
            disabled={activeMutation.isPending}
          />
        </div>
      )}

      <div className="mb-4">
        <Button
          type="button"
          variant="secondary"
          size="sm"
          onClick={addFollowUpPart}
          disabled={activeMutation.isPending || !canAddPart}
          data-testid="add-staged-part"
          data-faro-user-action-name="add-staged-result-part"
        >
          + Add a timed follow-up
        </Button>
        {isStaged && (
          <p className="mt-1 text-xs text-content-tertiary">
            {canAddPart
              ? `Parts are revealed one at a time. Each timer starts when the previous part is revealed, not when you publish.`
              : `A chain can hold at most ${MAX_CHAIN_PARTS} parts.`}
          </p>
        )}
      </div>

      <Button
        type="submit"
        variant="primary"
        disabled={activeMutation.isPending}
        className="w-full"
        data-faro-user-action-name="create-action-result"
      >
        {activeMutation.isPending
          ? 'Creating...'
          : isStaged
            ? `Create Draft Result (${followUpParts.length + 1} parts)`
            : 'Create Draft Result'}
      </Button>

      {activeMutation.isError && (
        <Alert variant="danger" className="mt-2">
          Failed to create result. Please try again.
        </Alert>
      )}

      {activeMutation.isSuccess && (
        <Alert variant="success" className="mt-2">
          Draft result created! Add character updates and publish when ready.
        </Alert>
      )}
    </form>
  );
};
