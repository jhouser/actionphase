import { useEffect, useState } from 'react';
import type { NumberEntry, NumberEntryDisplay } from '../types/characters';
import { numberEntryName, isBoundedTrack } from '../types/characters';
import { Button, Input, Select } from './ui';
import { MarkdownPreview } from './MarkdownPreview';
import { CommentEditor } from './CommentEditor';
import { useReportDirty } from '@/hooks/useReportDirty';

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
      className="w-full h-2 rounded-full surface-secondary overflow-hidden"
      role="img"
      aria-label={`${label}: ${filled} of ${total}`}
    >
      <div className="h-full bg-interactive-primary transition-all" style={{ width: `${percent}%` }} />
    </div>
  );
};

/** Empty string for a cleared maximum, so "unbounded" survives a round trip. */
const maxToInput = (max: number | undefined): string => (max === undefined ? '' : max.toString());

export const NumberCard: React.FC<NumberCardProps> = ({ entry, canEdit, onUpdate, onRemove, onDirtyChange }) => {
  const [isEditing, setIsEditing] = useState(false);
  const name = numberEntryName(entry);

  const [editName, setEditName] = useState(name);
  const [editAmount, setEditAmount] = useState(entry.amount);
  const [editMax, setEditMax] = useState(maxToInput(entry.max));
  const [editDisplay, setEditDisplay] = useState<NumberEntryDisplay>(entry.display || 'number');
  const [editDescription, setEditDescription] = useState(entry.description || '');

  // A maximum is what makes a track possible, so the display choice is
  // meaningless without one and the control stays hidden until a max is set.
  const parsedMax = parseFloat(editMax);
  const hasMax = editMax.trim() !== '' && !Number.isNaN(parsedMax) && parsedMax > 0;

  // Unlike the other cards the editor here is inline rather than a separate form
  // component, so the dirty comparison lives with the state it watches.
  //
  // Deliberately not gated on isEditing. handleSave clears isEditing in the same tick
  // it calls onUpdate, but that write is async upstream (saveJsonField debounces to
  // the network), so the entry prop still holds the old value for a while after.
  // Gating on isEditing would report clean across that window and let a close slip
  // through with the save still in flight. handleCancel resets the fields to match
  // the prop, so a cancelled editor compares equal and reports clean on its own.
  useReportDirty(
    editName !== name ||
      editAmount !== entry.amount ||
      editMax !== maxToInput(entry.max) ||
      editDisplay !== (entry.display || 'number') ||
      editDescription !== (entry.description || ''),
    onDirtyChange,
  );

  const handleSave = () => {
    onUpdate({
      name: editName,
      amount: editAmount,
      max: hasMax ? parsedMax : undefined,
      // Never persist a display mode without the max it renders against, and
      // never persist the default: 'number' is what an absent key already means.
      display: hasMax && editDisplay !== 'number' ? editDisplay : undefined,
      description: editDescription || undefined,
      // Clear the legacy key so an edited row stops carrying both spellings of
      // its name. Rows nobody edits keep it and read through the fallback,
      // which is why this is not a migration.
      type: undefined,
    });
    setIsEditing(false);
  };

  const handleCancel = () => {
    setEditName(name);
    setEditAmount(entry.amount);
    setEditMax(maxToInput(entry.max));
    setEditDisplay(entry.display || 'number');
    setEditDescription(entry.description || '');
    setIsEditing(false);
  };

  // Adopt whatever the parent settles on once the editor is closed and the prop has
  // actually moved. Distinguishing "the write landed" from "the write is still in
  // flight" is the point: resyncing on isEditing alone would report clean across the
  // in-flight window the comparison above exists to cover, while never resyncing
  // leaves a dropped write reporting dirty forever with no editor on screen to clear
  // it — tabs locked, nothing to click. Keying on the prop changing splits the two.
  useEffect(() => {
    if (isEditing) return;
    setEditName(name);
    setEditAmount(entry.amount);
    setEditMax(maxToInput(entry.max));
    setEditDisplay(entry.display || 'number');
    setEditDescription(entry.description || '');
    // Deliberately not depending on isEditing: this must run when the prop moves, not
    // when the editor closes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [name, entry.amount, entry.max, entry.display, entry.description]);

  const bounded = isBoundedTrack(entry);
  const max = entry.max ?? 0;
  const showBoxes = bounded && entry.display === 'boxes' && Number.isInteger(max) && max <= MAX_RENDERED_BOXES;
  const showBar = bounded && !showBoxes;

  return (
    <div className="border border-theme-default rounded-lg p-4 surface-base">
      <div className="flex justify-between items-center">
        <div className="flex-1">
          {isEditing ? (
            <div className="space-y-3">
              <div className="flex items-center space-x-3">
                <Input
                  type="text"
                  value={editName}
                  onChange={(e) => setEditName(e.target.value)}
                  placeholder="Name..."
                  className="font-medium"
                  aria-label="Name"
                />
                <Input
                  type="number"
                  value={editAmount}
                  onChange={(e) => setEditAmount(parseFloat(e.target.value) || 0)}
                  className="w-24 text-right"
                  step="any"
                  aria-label="Current"
                />
                <Input
                  type="number"
                  value={editMax}
                  onChange={(e) => setEditMax(e.target.value)}
                  className="w-24 text-right"
                  placeholder="Max"
                  min={0}
                  step="any"
                  aria-label="Maximum"
                />
              </div>
              {hasMax && (
                <Select
                  value={editDisplay}
                  onChange={(e) => setEditDisplay(e.target.value as NumberEntryDisplay)}
                  aria-label="Display as"
                  selectSize="sm"
                >
                  <option value="number">Number (4 / 9)</option>
                  <option value="track">Bar</option>
                  <option value="boxes">Boxes</option>
                </Select>
              )}
            </div>
          ) : (
            <div className="flex items-center justify-between">
              <span className="font-medium text-content-primary">{name}</span>
              <span className="text-lg font-semibold text-semantic-success">
                {entry.amount.toLocaleString()}
                {entry.max !== undefined && (
                  <span className="text-content-tertiary font-normal"> / {entry.max.toLocaleString()}</span>
                )}
              </span>
            </div>
          )}
        </div>

        {canEdit && (
          <div className="flex space-x-1 ml-4">
            {isEditing ? (
              <>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={handleSave}
                  className="p-1 text-semantic-success hover:text-semantic-success"
                >
                  ✓
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={handleCancel}
                  className="p-1 text-content-secondary hover:text-content-primary"
                >
                  ✕
                </Button>
              </>
            ) : (
              <>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => setIsEditing(true)}
                  className="p-1 text-interactive-primary hover:text-interactive-primary-hover"
                >
                  ✎
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={onRemove}
                  className="p-1 text-semantic-danger hover:text-semantic-danger"
                >
                  🗑
                </Button>
              </>
            )}
          </div>
        )}
      </div>

      {!isEditing && (showBoxes || showBar) && (
        <div className="mt-2">
          {showBoxes ? (
            <BoxTrack filled={entry.amount} total={max} label={name} />
          ) : (
            <BarTrack filled={entry.amount} total={max} label={name} />
          )}
        </div>
      )}

      {(entry.description || isEditing) && (
        <div className="mt-2">
          {isEditing ? (
            <CommentEditor
              value={editDescription}
              onChange={setEditDescription}
              placeholder="Notes about this entry... (Markdown supported)"
              rows={2}
              showPreviewByDefault={false}
            />
          ) : (
            <div className="text-sm">
              <MarkdownPreview content={entry.description || ''} />
            </div>
          )}
        </div>
      )}
    </div>
  );
};
