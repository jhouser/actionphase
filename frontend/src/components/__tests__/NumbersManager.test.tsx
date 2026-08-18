import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { NumbersManager } from '../NumbersManager';
import type { NumberEntry } from '../../types/characters';
import { logger } from '@/services/LoggingService';

vi.mock('@/services/LoggingService', () => ({
  logger: { warn: vi.fn(), error: vi.fn(), debug: vi.fn(), info: vi.fn() }
}));

vi.mock('../AddNumberModal', () => ({
  AddNumberModal: () => <div data-testid="add-number-modal">Add Modal</div>
}));

vi.mock('../NumberCard', () => ({
  NumberCard: ({ entry, onRemove }: { entry: NumberEntry; onRemove: () => void }) => (
    <div data-testid={`number-card-${entry.id}`}>
      {entry.name}
      <button onClick={onRemove} data-testid={`remove-number-${entry.id}`}>Remove</button>
    </div>
  )
}));

const renderNumbers = (props: Partial<React.ComponentProps<typeof NumbersManager>> = {}) =>
  render(
    <NumbersManager
      numbers={[]}
      canEdit={true}
      onNumbersChange={vi.fn()}
      label="Numbers"
      {...props}
    />
  );

describe('NumbersManager - Data Corruption Handling', () => {
  // Regression: a draft merge stripped id fields, and because rows are removed by
  // id, deleting one row deleted every row that shared its (undefined) id. Ported
  // from the InventoryManager suite when the Currency sub-tab became this tab.
  describe('handles corrupted data without IDs', () => {
    it('generates IDs defensively and logs a warning', () => {
      vi.mocked(logger.warn).mockClear();

      renderNumbers({
        numbers: [
          { name: 'Gold', amount: 100 } as NumberEntry, // Missing id!
          { name: 'Silver', amount: 50 } as NumberEntry, // Missing id!
        ],
      });

      expect(logger.warn).toHaveBeenCalledWith(
        expect.stringContaining('Number missing id field'),
        expect.any(Object)
      );
      expect(logger.warn).toHaveBeenCalledTimes(2); // once per corrupted row

      expect(screen.getByText('Gold')).toBeInTheDocument();
      expect(screen.getByText('Silver')).toBeInTheDocument();
    });

    it('removes only the targeted row after defensive ID generation', () => {
      const onNumbersChange = vi.fn();
      renderNumbers({
        numbers: [
          { name: 'Gold', amount: 100 } as NumberEntry,
          { name: 'Silver', amount: 50 } as NumberEntry,
          { name: 'Bronze', amount: 25 } as NumberEntry,
        ],
        onNumbersChange,
      });

      const silverCard = screen.getByText('Silver').closest('[data-testid^="number-card-"]');
      fireEvent.click(silverCard!.querySelector('button')!);

      expect(onNumbersChange).toHaveBeenCalledTimes(1);
      const updated = onNumbersChange.mock.calls[0][0];

      // The whole point of the regression: exactly one row goes.
      expect(updated).toHaveLength(2);
      const names = updated.map((c: NumberEntry) => c.name);
      expect(names).toEqual(expect.arrayContaining(['Gold', 'Bronze']));
      expect(names).not.toContain('Silver');
    });
  });

  it('does not warn when the data already has IDs', () => {
    vi.mocked(logger.warn).mockClear();
    renderNumbers({ numbers: [{ id: 'n1', name: 'Gold', amount: 100 }] });
    expect(logger.warn).not.toHaveBeenCalled();
  });

  describe('game-specific labels', () => {
    it('names the heading and controls after the game label', () => {
      // The tab is renameable, and this manager's copy is generated from the
      // label rather than hardcoded — a game tracking stress must not read
      // "Add Numbers".
      renderNumbers({ label: 'Stress' });
      expect(screen.getByRole('heading', { name: 'Stress' })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Add Stress' })).toBeInTheDocument();
      expect(screen.getByText(/no stress tracked yet/i)).toBeInTheDocument();
    });

    it('hides the add control from viewers who cannot edit', () => {
      renderNumbers({ canEdit: false, label: 'Stress' });
      expect(screen.queryByRole('button', { name: 'Add Stress' })).not.toBeInTheDocument();
    });
  });
});
