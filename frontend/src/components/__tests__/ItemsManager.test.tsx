import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { ItemsManager } from '../ItemsManager';
import type { InventoryItem } from '../../types/characters';
import { logger } from '@/services/LoggingService';
import { ToastProvider } from '@/contexts/ToastContext';

vi.mock('@/services/LoggingService', () => ({
  logger: { warn: vi.fn(), error: vi.fn(), debug: vi.fn(), info: vi.fn() }
}));

vi.mock('../AddItemModal', () => ({
  AddItemModal: () => <div data-testid="add-item-modal">Add Item Modal</div>
}));

vi.mock('../ItemCard', () => ({
  ItemCard: ({ item, onRemove }: { item: InventoryItem; onRemove: () => void }) => (
    <div data-testid={`item-card-${item.id}`}>
      {item.name}
      <button onClick={onRemove} data-testid={`remove-item-${item.id}`}>Remove</button>
    </div>
  )
}));

// ItemsManager calls useToast (to report loot-roll failures), which throws outside
// a ToastProvider. It deliberately does NOT require a GameProvider — it renders in
// the character sheet editor too — so no game context is provided here.
const renderItems = (props: Partial<React.ComponentProps<typeof ItemsManager>> = {}) =>
  render(
    <ToastProvider>
      <ItemsManager
        characterId={1}
        items={[]}
        canEdit={true}
        onItemsChange={vi.fn()}
        label="Inventory"
        {...props}
      />
    </ToastProvider>
  );

describe('ItemsManager - Data Corruption Handling', () => {
  // Regression: a draft merge stripped id fields, and because rows are removed by
  // id, deleting one item deleted every item that shared its (undefined) id.
  describe('handles corrupted item data without IDs', () => {
    it('generates IDs defensively and logs a warning', () => {
      vi.mocked(logger.warn).mockClear();

      renderItems({
        items: [
          { name: 'Sword', quantity: 1 } as InventoryItem, // Missing id!
          { name: 'Shield', quantity: 1 } as InventoryItem, // Missing id!
        ],
      });

      expect(logger.warn).toHaveBeenCalledWith(
        expect.stringContaining('Item missing id field'),
        expect.any(Object)
      );
      expect(logger.warn).toHaveBeenCalledTimes(2);

      expect(screen.getByText('Sword')).toBeInTheDocument();
      expect(screen.getByText('Shield')).toBeInTheDocument();
    });

    it('removes only the targeted item after defensive ID generation', () => {
      const onItemsChange = vi.fn();
      renderItems({
        items: [
          { name: 'Sword', quantity: 1 } as InventoryItem,
          { name: 'Shield', quantity: 1 } as InventoryItem,
          { name: 'Potion', quantity: 3 } as InventoryItem,
        ],
        onItemsChange,
      });

      const shieldCard = screen.getByText('Shield').closest('[data-testid^="item-card-"]');
      fireEvent.click(shieldCard!.querySelector('button')!);

      expect(onItemsChange).toHaveBeenCalledTimes(1);
      const updated = onItemsChange.mock.calls[0][0];

      expect(updated).toHaveLength(2);
      const names = updated.map((i: InventoryItem) => i.name);
      expect(names).toEqual(expect.arrayContaining(['Sword', 'Potion']));
      expect(names).not.toContain('Shield');
    });
  });

  it('does not warn when the data already has IDs', () => {
    vi.mocked(logger.warn).mockClear();
    renderItems({ items: [{ id: 'i1', name: 'Sword', quantity: 1 }] });
    expect(logger.warn).not.toHaveBeenCalled();
  });

  it('names the heading after the game label', () => {
    renderItems({ label: 'Load' });
    expect(screen.getByRole('heading', { name: 'Load' })).toBeInTheDocument();
  });
});

// `weight` and `value` are optional and no game in play sets them, so the summary
// used to render "Total Weight: 0.0 • Total Value: 0" under every inventory —
// a made-up number presented as a fact about the character.
describe('ItemsManager - weight/value summary', () => {
  it('hides the summary when no item sets weight or value', () => {
    renderItems({
      items: [
        { id: 'i1', name: 'Sword', quantity: 1 },
        { id: 'i2', name: 'Potion', quantity: 3 },
      ],
    });

    expect(screen.queryByText(/Total Weight/)).not.toBeInTheDocument();
  });

  it('shows the summary when at least one item sets weight', () => {
    renderItems({
      items: [
        { id: 'i1', name: 'Sword', quantity: 2, weight: 3 },
        { id: 'i2', name: 'Potion', quantity: 3 },
      ],
    });

    expect(screen.getByText(/Total Weight: 6\.0/)).toBeInTheDocument();
  });

  // A zero is a real opt-in, not an absent field: a weightless item in a game
  // that tracks weight should still get a summary.
  it('shows the summary when an item sets weight to zero', () => {
    renderItems({ items: [{ id: 'i1', name: 'Feather', quantity: 1, weight: 0 }] });

    expect(screen.getByText(/Total Weight: 0\.0/)).toBeInTheDocument();
  });

  it('shows the summary when only value is set', () => {
    renderItems({ items: [{ id: 'i1', name: 'Gem', quantity: 2, value: 50 }] });

    expect(screen.getByText(/Total Value: 100/)).toBeInTheDocument();
  });
});
