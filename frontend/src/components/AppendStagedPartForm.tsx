import React, { useState } from 'react';
import { Select, Button, Input, Alert } from './ui';
import { CommentEditor } from './CommentEditor';
import { DELAY_PRESETS, CUSTOM_DELAY, DEFAULT_DELAY_MINUTES, formatDelayLabel, isPresetDelay } from '../lib/stagedDelays';

interface AppendStagedPartFormProps {
  /** Any member of the chain to append to; the server resolves the tail. */
  resultId: number;
  /** Human-facing number the new part will take, for labelling only. */
  nextPartNumber: number;
  onSubmit: (part: { content: string; delay_minutes: number }) => Promise<void>;
  onCancel: () => void;
  isPending?: boolean;
  isError?: boolean;
}

/**
 * Adds one follow-up part to a draft chain.
 *
 * Distinct from StagedPartsEditor, which edits a list of not-yet-created parts
 * inside the create form. This one appends a single part to a chain that
 * already exists in the database, which is what lets a GM write a staged result
 * across several sittings instead of composing the whole thing up front.
 *
 * Deliberately one part at a time: each append is its own request, so a GM who
 * adds three parts over an afternoon gets three saved parts rather than a
 * batch that is lost if the tab closes.
 */
export const AppendStagedPartForm: React.FC<AppendStagedPartFormProps> = ({
  resultId,
  nextPartNumber,
  onSubmit,
  onCancel,
  isPending = false,
  isError = false,
}) => {
  const [content, setContent] = useState('');
  const [delayMinutes, setDelayMinutes] = useState(DEFAULT_DELAY_MINUTES);

  const isCustom = !isPresetDelay(delayMinutes);
  const canSubmit = content.trim().length > 0 && delayMinutes >= 1 && !isPending;

  const handleSubmit = async () => {
    if (!canSubmit) return;
    await onSubmit({ content: content.trim(), delay_minutes: delayMinutes });
    // Cleared so the form is ready for the next part; the parent decides
    // whether to keep it open.
    setContent('');
    setDelayMinutes(DEFAULT_DELAY_MINUTES);
  };

  return (
    <div
      className="border border-theme-default rounded p-4 surface-sunken mt-3"
      data-testid={`append-staged-part-form-${resultId}`}
    >
      <div className="flex items-end gap-2 mb-3">
        <div className="flex-1">
          <Select
            label={`Send out ___ after Part ${nextPartNumber - 1}`}
            selectSize="sm"
            value={isCustom ? CUSTOM_DELAY : String(delayMinutes)}
            onChange={e => {
              const { value } = e.target;
              // Switching to custom keeps the current value, so changing modes
              // never silently changes the schedule.
              if (value !== CUSTOM_DELAY) setDelayMinutes(Number(value));
            }}
            disabled={isPending}
            data-testid={`append-staged-part-delay-${resultId}`}
          >
            {DELAY_PRESETS.map(minutes => (
              <option key={minutes} value={minutes}>
                {formatDelayLabel(minutes)}
              </option>
            ))}
            <option value={CUSTOM_DELAY}>Custom…</option>
          </Select>
        </div>
      </div>

      {isCustom && (
        <div className="mb-3 w-40">
          <Input
            label="Custom delay (minutes)"
            type="number"
            min={1}
            inputSize="sm"
            value={delayMinutes}
            onChange={e => setDelayMinutes(Number(e.target.value))}
            disabled={isPending}
            data-testid={`append-staged-part-custom-delay-${resultId}`}
          />
        </div>
      )}

      <label className="block text-sm font-medium text-content-primary mb-1">
        Result Content {nextPartNumber}
      </label>
      <CommentEditor
        id={`append-staged-part-content-${resultId}`}
        textareaTestId={`append-staged-part-content-${resultId}`}
        value={content}
        onChange={setContent}
        rows={4}
        placeholder={`Part ${nextPartNumber} — revealed after the timer...`}
        maxLength={100000}
        disabled={isPending}
        showCharacterCount
      />

      <div className="flex justify-end gap-2 mt-3">
        <Button variant="ghost" size="sm" onClick={onCancel} disabled={isPending}>
          Cancel
        </Button>
        <Button
          variant="primary"
          size="sm"
          onClick={handleSubmit}
          disabled={!canSubmit}
          data-testid={`save-staged-part-${resultId}`}
          data-faro-user-action-name="append-staged-result-part"
        >
          {isPending ? 'Adding...' : 'Add Part'}
        </Button>
      </div>

      {isError && (
        <Alert variant="danger" className="mt-2">
          Failed to add this part. Please try again.
        </Alert>
      )}
    </div>
  );
};
