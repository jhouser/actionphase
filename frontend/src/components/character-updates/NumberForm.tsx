import { useState } from 'react';
import { Button, Input, Select } from '../ui';
import { CommentEditor } from '../CommentEditor';
import type { NumberEntryDisplay } from '../../types/characters';

export interface NumberFormData {
  name: string;
  amount: number;
  max?: number;
  display?: NumberEntryDisplay;
  description?: string;
}

interface NumberFormProps {
  onSubmit: (data: NumberFormData) => void;
  onCancel: () => void;
  initialValues?: Partial<NumberFormData>;
  submitLabel?: string;
  variant?: 'modal' | 'inline';
  submitButtonTestId?: string;
}

/**
 * Shared form for adding entries on the Numbers tab, used by AddNumberModal.
 *
 * NumberCard edits inline rather than reusing this form: its dirty reporting has
 * to survive the editor closing while the save is still in flight, which a form
 * that unmounts on submit cannot express.
 */
export const NumberForm: React.FC<NumberFormProps> = ({
  onSubmit,
  onCancel,
  initialValues,
  submitLabel = 'Add',
  variant = 'modal',
  submitButtonTestId,
}) => {
  const [name, setName] = useState(initialValues?.name || '');
  const [amount, setAmount] = useState(initialValues?.amount?.toString() || '');
  const [max, setMax] = useState(initialValues?.max?.toString() || '');
  const [display, setDisplay] = useState<NumberEntryDisplay>(initialValues?.display || 'number');
  const [description, setDescription] = useState(initialValues?.description || '');

  // A maximum is what makes a track possible, so the display choice is
  // meaningless without one and the control stays hidden until a max is set.
  const parsedMax = parseFloat(max);
  const hasMax = max.trim() !== '' && !Number.isNaN(parsedMax) && parsedMax > 0;

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) return;

    onSubmit({
      name: name.trim(),
      amount: parseFloat(amount) || 0,
      max: hasMax ? parsedMax : undefined,
      // Never persist a display mode without the max it renders against, and
      // never persist the default: 'number' is what an absent key already means.
      display: hasMax && display !== 'number' ? display : undefined,
      description: description.trim() || undefined,
    });
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <Input
        id="number-name"
        label="Name *"
        type="text"
        value={name}
        onChange={(e) => setName(e.target.value)}
        placeholder="e.g., Gold, Stress, XP, Clock"
        required
      />

      <div className="flex gap-3">
        <Input
          id="number-amount"
          label="Current"
          type="number"
          value={amount}
          onChange={(e) => setAmount(e.target.value)}
          placeholder="0"
          step="any"
          className="flex-1"
        />

        <Input
          id="number-max"
          label="Maximum"
          type="number"
          value={max}
          onChange={(e) => setMax(e.target.value)}
          placeholder="Optional"
          min={0}
          step="any"
          className="flex-1"
        />
      </div>

      {hasMax && (
        <Select
          id="number-display"
          label="Display as"
          value={display}
          onChange={(e) => setDisplay(e.target.value as NumberEntryDisplay)}
        >
          <option value="number">Number (4 / 9)</option>
          <option value="track">Bar</option>
          <option value="boxes">Boxes</option>
        </Select>
      )}

      <div>
        <label htmlFor="number-description" className="block text-sm font-medium text-content-primary mb-2">
          Description <span className="text-xs text-content-tertiary font-normal">(Markdown supported)</span>
        </label>
        <CommentEditor
          id="number-description"
          value={description}
          onChange={setDescription}
          placeholder="Optional notes..."
          rows={2}
          showPreviewByDefault={false}
        />
      </div>

      <div className={`flex justify-end gap-3 ${variant === 'modal' ? 'pt-4' : 'pt-2'}`}>
        <Button
          type="button"
          variant="secondary"
          onClick={onCancel}
        >
          Cancel
        </Button>
        <Button
          type="submit"
          variant="primary"
          data-testid={submitButtonTestId}
        >
          {submitLabel}
        </Button>
      </div>
    </form>
  );
};
