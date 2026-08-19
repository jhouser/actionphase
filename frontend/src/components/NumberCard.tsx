import { useState } from 'react';
import type { NumberEntry } from '../types/characters';
import { numberEntryName, isBoundedTrack } from '../types/characters';
import { Button } from './ui';
import { MarkdownPreview } from './MarkdownPreview';
import { NumberForm, type NumberFormData } from './character-updates/NumberForm';

interface NumberCardProps {
  entry: NumberEntry;
  canEdit: boolean;
  onUpdate: (updates: Partial<NumberEntry>) => void;
  onRemove: () => void;
  /** Reports whether this card's inline editor holds uncommitted edits. */
  onDirtyChange?: (isDirty: boolean) => void;
}

/** How many boxes to draw before falling back to a bar. */
const MAX_RENDERED_BOXES = 20;

/**
 * A bounded entry drawn as filled/empty boxes — the notation most narrative
 * systems use for stress, harm, and clocks.
 *
 * Falls back to a bar past MAX_RENDERED_BOXES: twenty is already a wide row on a
 * phone, and a hundred boxes is unreadable rather than merely long.
 */
const BoxTrack: React.FC<{ filled: number; total: number; label: string }> = ({ filled, total, label }) => (
  <div className="flex items-center gap-1 flex-wrap" role="img" aria-label={`${label}: ${filled} of ${total}`}>
    {Array.from({ length: total }, (_, i) => (
      <span
        key={i}
        className={`inline-block w-4 h-4 rounded-sm border ${
          i < filled ? 'bg-interactive-primary border-interactive-primary' : 'border-theme-default'
        }`}
      />
    ))}
  </div>
);

const BarTrack: React.FC<{ filled: number; total: number; label: string }> = ({ filled, total, label }) => {
  // Clamped because an entry can exceed its maximum — overfilled stress is a
  // real state in several systems, and a 140%-wide bar would break the layout.
  const percent = Math.min(100, Math.max(0, (filled / total) * 100));
  return (
    <div
      // Bordered like BoxTrack's empty cells: without an outline the trough
      // blends into the card and the bar's full extent — and so the value it
      // encodes — is unreadable at anything under a full fill.
      className="w-full h-2 rounded-full surface-secondary border border-theme-default overflow-hidden"
      role="img"
      aria-label={`${label}: ${filled} of ${total}`}
    >
      <div className="h-full bg-interactive-primary transition-all" style={{ width: `${percent}%` }} />
    </div>
  );
};

export const NumberCard: React.FC<NumberCardProps> = ({ entry, canEdit, onUpdate, onRemove, onDirtyChange }) => {
  const [isEditing, setIsEditing] = useState(false);
  const name = numberEntryName(entry);

  const handleSave = (data: NumberFormData) => {
    onUpdate({
      name: data.name,
      amount: data.amount,
      max: data.max,
      display: data.display,
      description: data.description,
      // Clear the legacy key so an edited row stops carrying both spellings of
      // its name. Rows nobody edits keep it and read through the fallback,
      // which is why this is not a migration.
      type: undefined,
    });
    setIsEditing(false);
  };

  // Swaps the whole card for the shared form while editing, as SkillCard and
  // ItemCard do. This tab used to hand-roll its own inline editor with ✓/✕ icon
  // buttons, which was the only editor on the sheet that did not present a
  // labelled Cancel/Save pair.
  if (isEditing) {
    return (
      <div className="border border-theme-default rounded-lg p-4 surface-base">
        <NumberForm
          initialValues={{
            name,
            amount: entry.amount,
            max: entry.max,
            display: entry.display,
            description: entry.description,
          }}
          onSubmit={handleSave}
          onCancel={() => setIsEditing(false)}
          submitLabel="Save"
          variant="inline"
          onDirtyChange={onDirtyChange}
        />
      </div>
    );
  }

  const bounded = isBoundedTrack(entry);
  const max = entry.max ?? 0;
  const showBoxes = bounded && entry.display === 'boxes' && Number.isInteger(max) && max <= MAX_RENDERED_BOXES;
  const showBar = bounded && !showBoxes;

  return (
    <div className="border border-theme-default rounded-lg p-4 surface-base">
      <div className="flex justify-between items-center">
        <div className="flex-1">
          <div className="flex items-center justify-between">
            <span className="font-medium text-content-primary">{name}</span>
            <span className="text-lg font-semibold text-semantic-success">
              {entry.amount.toLocaleString()}
              {entry.max !== undefined && (
                <span className="text-content-tertiary font-normal"> / {entry.max.toLocaleString()}</span>
              )}
            </span>
          </div>
        </div>

        {canEdit && (
          <div className="flex space-x-1 ml-4">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setIsEditing(true)}
              className="p-1 text-interactive-primary hover:text-interactive-primary-hover"
              aria-label="Edit entry"
            >
              ✎
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={onRemove}
              className="p-1 text-semantic-danger hover:text-semantic-danger"
              aria-label="Remove entry"
            >
              🗑
            </Button>
          </div>
        )}
      </div>

      {(showBoxes || showBar) && (
        <div className="mt-2">
          {showBoxes ? (
            <BoxTrack filled={entry.amount} total={max} label={name} />
          ) : (
            <BarTrack filled={entry.amount} total={max} label={name} />
          )}
        </div>
      )}

      {entry.description && (
        <div className="mt-2">
          <div className="text-sm">
            <MarkdownPreview content={entry.description} />
          </div>
        </div>
      )}
    </div>
  );
};
