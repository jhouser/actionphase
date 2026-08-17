import { useEffect, useState } from 'react';
import type { CurrencyEntry } from '../types/characters';
import { Button, Input } from './ui';
import { MarkdownPreview } from './MarkdownPreview';
import { CommentEditor } from './CommentEditor';
import { useReportDirty } from '@/hooks/useReportDirty';

interface CurrencyCardProps {
  currency: CurrencyEntry;
  canEdit: boolean;
  onUpdate: (updates: Partial<CurrencyEntry>) => void;
  onRemove: () => void;
  /** Reports whether this card's inline editor holds uncommitted edits. */
  onDirtyChange?: (isDirty: boolean) => void;
}

export const CurrencyCard: React.FC<CurrencyCardProps> = ({ currency, canEdit, onUpdate, onRemove, onDirtyChange }) => {
  const [isEditing, setIsEditing] = useState(false);
  const [editType, setEditType] = useState(currency.type);
  const [editAmount, setEditAmount] = useState(currency.amount);
  const [editDescription, setEditDescription] = useState(currency.description || '');

  // Unlike the other cards the editor here is inline rather than a separate form
  // component, so the dirty comparison lives with the state it watches.
  //
  // Deliberately not gated on isEditing. handleSave clears isEditing in the same tick
  // it calls onUpdate, but that write is async upstream (saveJsonField debounces to
  // the network), so the currency prop still holds the old value for a while after.
  // Gating on isEditing would report clean across that window and let a close slip
  // through with the save still in flight. handleCancel resets the fields to match
  // the prop, so a cancelled editor compares equal and reports clean on its own.
  useReportDirty(
    editType !== currency.type ||
      editAmount !== currency.amount ||
      editDescription !== (currency.description || ''),
    onDirtyChange,
  );

  const handleSave = () => {
    onUpdate({
      type: editType,
      amount: editAmount,
      description: editDescription || undefined
    });
    setIsEditing(false);
  };

  const handleCancel = () => {
    setEditType(currency.type);
    setEditAmount(currency.amount);
    setEditDescription(currency.description || '');
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
    setEditType(currency.type);
    setEditAmount(currency.amount);
    setEditDescription(currency.description || '');
    // Deliberately not depending on isEditing: this must run when the prop moves, not
    // when the editor closes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [currency.type, currency.amount, currency.description]);

  return (
    <div className="border border-theme-default rounded-lg p-4 surface-base">
      <div className="flex justify-between items-center">
        <div className="flex-1">
          {isEditing ? (
            <div className="flex items-center space-x-3">
              <Input
                type="text"
                value={editType}
                onChange={(e) => setEditType(e.target.value)}
                placeholder="Currency type..."
                className="font-medium"
              />
              <Input
                type="number"
                value={editAmount}
                onChange={(e) => setEditAmount(parseFloat(e.target.value) || 0)}
                className="w-24 text-right"
                step="any"
              />
            </div>
          ) : (
            <div className="flex items-center justify-between">
              <span className="font-medium text-content-primary">{currency.type}</span>
              <span className="text-lg font-semibold text-semantic-success">{currency.amount.toLocaleString()}</span>
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

      {(currency.description || isEditing) && (
        <div className="mt-2">
          {isEditing ? (
            <CommentEditor
              value={editDescription}
              onChange={setEditDescription}
              placeholder="Notes about this currency... (Markdown supported)"
              rows={2}
              showPreviewByDefault={false}
            />
          ) : (
            <div className="text-sm">
              <MarkdownPreview content={currency.description || ''} />
            </div>
          )}
        </div>
      )}
    </div>
  );
};
