import React, { useState, useMemo } from 'react';
import type { CurrencyEntry } from '../types/characters';
import { CurrencyCard } from './CurrencyCard';
import { AddCurrencyModal } from './AddCurrencyModal';
import { Button } from './ui';
import { generateId } from '../utils/generateId';
import { ensureIds } from '../utils/ensureIds';
import { useDirtyChildren } from '@/hooks/useDirtyChildren';

interface NumbersManagerProps {
  numbers: CurrencyEntry[];
  canEdit: boolean;
  onNumbersChange: (numbers: CurrencyEntry[]) => void;
  /**
   * Reports whether any editor below holds edits that have not been committed
   * with Save. Ancestors use it to warn before closing the sheet.
   */
  onDirtyChange?: (isDirty: boolean) => void;
  /**
   * The tab's name in this game. Load-bearing here more than on the other tabs:
   * the default is the deliberately generic "Numbers", and a game that tracks
   * stress or clocks will have renamed it to something specific.
   */
  label: string;
}

/**
 * The Numbers tab of the character sheet: arbitrary per-game numeric tracks.
 *
 * Promoted out of the former InventoryManager's Currency sub-tab, where it was
 * buried under Inventory despite nothing about a number being an item. The tab
 * is deliberately general — money, stress, XP, heat, clocks — which is why the
 * storage key was renamed `currency` → `numbers` rather than kept.
 *
 * The row type is still `CurrencyEntry` and the card is still `CurrencyCard`;
 * renaming those is Phase 5's field work, not this phase's.
 */
export const NumbersManager: React.FC<NumbersManagerProps> = ({
  numbers,
  canEdit,
  onNumbersChange,
  onDirtyChange,
  label,
}) => {
  const { report: reportDirty } = useDirtyChildren(onDirtyChange);
  // Defensive: ensure every row has an ID (protects against draft-merge corruption)
  const validatedNumbers = useMemo(() => ensureIds(numbers, 'Number'), [numbers]);

  const [showAdd, setShowAdd] = useState(false);

  const addNumber = (data: Omit<CurrencyEntry, 'id'>) => {
    onNumbersChange([...validatedNumbers, { id: generateId(), ...data }]);
    setShowAdd(false);
  };

  const removeNumber = (id: string) => {
    onNumbersChange(validatedNumbers.filter(c => c.id !== id));
  };

  const updateNumber = (id: string, updates: Partial<CurrencyEntry>) => {
    onNumbersChange(validatedNumbers.map(c => c.id === id ? { ...c, ...updates } : c));
  };

  return (
    <div data-testid="numbers-section">
      <div className="flex justify-between items-center mb-4">
        <h3 className="text-lg font-medium text-content-primary">{label}</h3>
        {canEdit && (
          <Button variant="primary" size="sm" onClick={() => setShowAdd(true)}>
            Add {label}
          </Button>
        )}
      </div>

      {validatedNumbers.length === 0 ? (
        <div className="text-center py-8 text-content-secondary">
          <p>No {label.toLowerCase()} tracked yet.</p>
          {canEdit && <p className="text-sm mt-1">Click "Add {label}" to get started.</p>}
        </div>
      ) : (
        <div className="space-y-3">
          {validatedNumbers.map((entry) => (
            <CurrencyCard
              key={entry.id}
              currency={entry}
              canEdit={canEdit}
              onUpdate={(updates) => updateNumber(entry.id, updates)}
              onRemove={() => removeNumber(entry.id)}
              onDirtyChange={(isDirty) => reportDirty(`number:${entry.id}`, isDirty)}
            />
          ))}
        </div>
      )}

      {showAdd && (
        <AddCurrencyModal onAdd={addNumber} onCancel={() => setShowAdd(false)} />
      )}
    </div>
  );
};
