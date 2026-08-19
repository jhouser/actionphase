import { useState } from 'react';
import { Button, Input } from '../ui';
import { CommentEditor } from '../CommentEditor';
import { useReportDirty } from '@/hooks/useReportDirty';

export interface SkillFormData {
  name: string;
  rank?: string;
  description?: string;
  category?: string;
}

interface SkillFormProps {
  onSubmit: (data: SkillFormData) => void;
  onCancel: () => void;
  initialValues?: Partial<SkillFormData>;
  submitLabel?: string;
  variant?: 'modal' | 'inline';
  submitButtonTestId?: string;
  /** Reports whether the form holds edits that Save has not yet committed. */
  onDirtyChange?: (isDirty: boolean) => void;
}

/**
 * Shared form component for adding/editing character skills.
 * Used in both AddSkillModal and SkillCard's inline editor to ensure consistency.
 */
export const SkillForm: React.FC<SkillFormProps> = ({
  onSubmit,
  onCancel,
  initialValues,
  submitLabel = 'Add',
  variant = 'modal',
  submitButtonTestId,
  onDirtyChange,
}) => {
  const [name, setName] = useState(initialValues?.name || '');
  const [rank, setRank] = useState(initialValues?.rank || '');
  const [description, setDescription] = useState(initialValues?.description || '');
  const [category, setCategory] = useState(initialValues?.category || '');

  // Compared trimmed, because handleSubmit submits trimmed. An untrimmed
  // comparison reports dirty for a change that Save would discard, which
  // soft-locks the form: the tab stays locked with nothing left to commit.
  useReportDirty(
    name.trim() !== (initialValues?.name || '').trim() ||
      rank.trim() !== (initialValues?.rank || '').trim() ||
      description.trim() !== (initialValues?.description || '').trim() ||
      category.trim() !== (initialValues?.category || '').trim(),
    onDirtyChange,
  );

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) return;

    onSubmit({
      name: name.trim(),
      rank: rank.trim() || undefined,
      description: description.trim() || undefined,
      category: category.trim() || undefined,
    });
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <Input
        id="skill-name"
        label="Name *"
        type="text"
        value={name}
        onChange={(e) => setName(e.target.value)}
        placeholder="e.g., Sword Fighting, Lockpicking"
        required
      />

      <Input
        id="skill-rank"
        label="Rank"
        type="text"
        value={rank}
        onChange={(e) => setRank(e.target.value)}
        placeholder="e.g., Expert, 5, Advanced"
      />

      <Input
        id="skill-category"
        label="Category"
        type="text"
        value={category}
        onChange={(e) => setCategory(e.target.value)}
        placeholder="e.g., Combat, Social, Academic"
      />

      <div>
        <label htmlFor="skill-description" className="block text-sm font-medium text-content-primary mb-2">
          Description <span className="text-xs text-content-tertiary font-normal">(Markdown supported)</span>
        </label>
        <CommentEditor
          id="skill-description"
          value={description}
          onChange={setDescription}
          placeholder="Describe this skill..."
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
