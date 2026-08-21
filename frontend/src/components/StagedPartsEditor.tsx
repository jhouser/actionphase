import React from 'react';
import { Select, Button } from './ui';
import { CommentEditor } from './CommentEditor';
import type { StagedResultPart } from '../types/phases';
import { DELAY_PRESETS, formatDelayLabel } from '../lib/stagedDelays';

interface StagedPartsEditorProps {
  /**
   * Follow-up parts only — the head lives in the parent form's own editor.
   * Index 0 here is therefore Part 2.
   */
  parts: StagedResultPart[];
  onChange: (parts: StagedResultPart[]) => void;
  disabled?: boolean;
  actionSubmissionId?: number;
}

/**
 * The follow-up half of the staged-result composer.
 *
 * Deliberately does not own the head part. The head is an ordinary result
 * whose content field already exists in CreateActionResultForm, and a GM who
 * never touches staging must see exactly the form they saw before — so the
 * head stays where it was and this component appends to it.
 */
export const StagedPartsEditor: React.FC<StagedPartsEditorProps> = ({
  parts,
  onChange,
  disabled = false,
  actionSubmissionId = undefined,
}) => {
  //Content autosave id for comment textbox
  const autosaveRefId = actionSubmissionId ? `action-result-${actionSubmissionId}` : undefined;
  
  const updatePart = (index: number, patch: Partial<StagedResultPart>) => {
    onChange(parts.map((part, i) => (i === index ? { ...part, ...patch } : part)));
  };

  const removePart = (index: number) => {
    onChange(parts.filter((_, i) => i !== index));
    if (autosaveRefId) {
      localStorage.removeItem(`${autosaveRefId}-part-${index + 2}`);
    }
  };


  return (
    <div className="space-y-4">
      {parts.map((part, index) => {
        // Part numbering is 1-based and counts the head, so the first follow-up
        // is Part 2 and its delay is measured from Part 1.
        const partNumber = index + 2;

        return (
          <div
            key={index}
            className="border border-theme-default rounded p-4 surface-sunken"
            data-testid={`staged-part-${partNumber}`}
          >
            <div className="flex items-end gap-2 mb-3">
              <div className="flex-1">
                <Select
                  label={`Send out ___ after Part ${partNumber - 1}`}
                  selectSize="sm"
                  value={String(part.delay_minutes)}
                  onChange={e => updatePart(index, { delay_minutes: Number(e.target.value) })}
                  disabled={disabled}
                  data-testid={`staged-part-delay-${partNumber}`}
                >
                  {DELAY_PRESETS.map(minutes => (
                    <option key={minutes} value={minutes}>
                      {formatDelayLabel(minutes)}
                    </option>
                  ))}
                </Select>
              </div>
              <Button
                type="button"
                variant="danger"
                size="sm"
                onClick={() => removePart(index)}
                disabled={disabled}
                data-testid={`remove-staged-part-${partNumber}`}
              >
                Remove
              </Button>
            </div>

            <label className="block text-sm font-medium text-content-primary mb-1">
              Result Content {partNumber}
            </label>
            <CommentEditor
              id={`staged-part-content-${partNumber}`}
              textareaTestId={`staged-part-content-${partNumber}`}
              value={part.content}
              onChange={value => updatePart(index, { content: value })}
              rows={4}
              placeholder={`Part ${partNumber} — revealed after the timer...`}
              maxLength={100000}
              showCharacterCount
              autosaveRefId={autosaveRefId ? `${autosaveRefId}-part-${partNumber}` : undefined}
            />
          </div>
        );
      })}
    </div>
  );
};
