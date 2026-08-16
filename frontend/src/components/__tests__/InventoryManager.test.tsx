import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { InventoryManager } from '../InventoryManager';
import type { InventoryItem, CurrencyEntry } from '../../types/characters';
import { logger } from '@/services/LoggingService';
import { ToastProvider } from '@/contexts/ToastContext';

vi.mock('@/services/LoggingService', () => ({
  logger: { warn: vi.fn(), error: vi.fn(), debug: vi.fn(), info: vi.fn() }
}));

// Mock the modals since they're not relevant for these tests
vi.mock('../AddItemModal', () => ({
  AddItemModal: () => <div data-testid="add-item-modal">Add Item Modal</div>
}));

vi.mock('../AddCurrencyModal', () => ({
  AddCurrencyModal: () => <div data-testid="add-currency-modal">Add Currency Modal</div>
}));

vi.mock('../ItemCard', () => ({
  ItemCard: ({ item, onRemove }: { item: InventoryItem; onRemove: () => void }) => (
    <div data-testid={`item-card-${item.id}`}>
      {item.name}
      <button onClick={onRemove} data-testid={`remove-item-${item.id}`}>Remove</button>
    </div>
  )
}));

vi.mock('../CurrencyCard', () => ({
  CurrencyCard: ({ currency, onRemove }: { currency: CurrencyEntry; onRemove: () => void }) => (
    <div data-testid={`currency-card-${currency.id}`}>
      {currency.type}
      <button onClick={onRemove} data-testid={`remove-currency-${currency.id}`}>Remove</button>
    </div>
  )
}));

// InventoryManager calls useToast (to report loot-roll failures), which throws
// outside a ToastProvider. It deliberately does NOT require a GameProvider — it
// renders in the character sheet editor too — so no game context is provided here.
const renderInventoryManager = (props: Partial<React.ComponentProps<typeof InventoryManager>>) =>
  render(
    <ToastProvider>
      <InventoryManager
        characterId={1}
        items={[]}
        currency={[]}
        canEdit={true}
        onItemsChange={vi.fn()}
        onCurrencyChange={vi.fn()}
        {...props}
      />
    </ToastProvider>
  );

describe('InventoryManager - Data Corruption Handling', () => {
  // Regression test for bug where draft merge stripped ID fields from currencies/items
  // causing deletion to fail catastrophically (all items deleted instead of one)

  describe('handles corrupted currency data without IDs', () => {
    it('generates IDs defensively and logs warning', () => {
      vi.mocked(logger.warn).mockClear();

      // Corrupted data from backend (missing id fields)
      const corruptedCurrency = [
        { type: 'Gold', amount: 100 } as CurrencyEntry, // Missing id!
        { type: 'Silver', amount: 50 } as CurrencyEntry, // Missing id!
      ];

      const mockOnChange = vi.fn();

      renderInventoryManager({
        items: [],
        currency: corruptedCurrency,
        canEdit: true,
        onItemsChange: vi.fn(),
        onCurrencyChange: mockOnChange,
      });

      // Verify warning was logged for corrupted data
      expect(logger.warn).toHaveBeenCalledWith(
        expect.stringContaining('Currency missing id field'),
        expect.any(Object)
      );
      expect(logger.warn).toHaveBeenCalledTimes(2); // Once per corrupted item

      // Switch to currency tab
      fireEvent.click(screen.getByText(/Currency/));

      // Verify currencies are still displayed (defensive IDs generated)
      expect(screen.getByText('Gold')).toBeInTheDocument();
      expect(screen.getByText('Silver')).toBeInTheDocument();

    });

    it('deletion still works correctly after defensive ID generation', () => {
      // Corrupted data from backend (missing id fields)
      const corruptedCurrency = [
        { type: 'Gold', amount: 100 } as CurrencyEntry,
        { type: 'Silver', amount: 50 } as CurrencyEntry,
        { type: 'Bronze', amount: 25 } as CurrencyEntry,
      ];

      const mockOnChange = vi.fn();

      renderInventoryManager({
        items: [],
        currency: corruptedCurrency,
        canEdit: true,
        onItemsChange: vi.fn(),
        onCurrencyChange: mockOnChange,
      });

      // Switch to currency tab
      fireEvent.click(screen.getByText(/Currency/));

      // All 3 currencies should be displayed
      expect(screen.getByText('Gold')).toBeInTheDocument();
      expect(screen.getByText('Silver')).toBeInTheDocument();
      expect(screen.getByText('Bronze')).toBeInTheDocument();

      // Find and click remove button for Silver
      // Note: The defensive code will have generated IDs, so we need to find by content
      const silverCard = screen.getByText('Silver').closest('[data-testid^="currency-card-"]');
      const removeButton = silverCard?.querySelector('button');
      expect(removeButton).toBeInTheDocument();

      if (removeButton) {
        fireEvent.click(removeButton);
      }

      // Verify onCurrencyChange was called with Silver removed
      expect(mockOnChange).toHaveBeenCalledTimes(1);
      const updatedCurrency = mockOnChange.mock.calls[0][0];

      // CRITICAL: Only Silver should be removed, Gold and Bronze should remain
      expect(updatedCurrency).toHaveLength(2);
      const types = updatedCurrency.map((c: CurrencyEntry) => c.type);
      expect(types).toContain('Gold');
      expect(types).toContain('Bronze');
      expect(types).not.toContain('Silver');

    });
  });

  describe('handles corrupted item data without IDs', () => {
    it('generates IDs defensively and logs warning', () => {
      vi.mocked(logger.warn).mockClear();

      // Corrupted data from backend (missing id fields)
      const corruptedItems = [
        { name: 'Sword', quantity: 1 } as InventoryItem, // Missing id!
        { name: 'Shield', quantity: 1 } as InventoryItem, // Missing id!
      ];

      const mockOnChange = vi.fn();

      renderInventoryManager({
        items: corruptedItems,
        currency: [],
        canEdit: true,
        onItemsChange: mockOnChange,
        onCurrencyChange: vi.fn(),
      });

      // Verify warning was logged for corrupted data
      expect(logger.warn).toHaveBeenCalledWith(
        expect.stringContaining('Item missing id field'),
        expect.any(Object)
      );
      expect(logger.warn).toHaveBeenCalledTimes(2); // Once per corrupted item

      // Verify items are still displayed (defensive IDs generated)
      expect(screen.getByText('Sword')).toBeInTheDocument();
      expect(screen.getByText('Shield')).toBeInTheDocument();

    });

    it('deletion still works correctly after defensive ID generation', () => {
      // Corrupted data from backend (missing id fields)
      const corruptedItems = [
        { name: 'Sword', quantity: 1 } as InventoryItem,
        { name: 'Shield', quantity: 1 } as InventoryItem,
        { name: 'Potion', quantity: 5 } as InventoryItem,
      ];

      const mockOnChange = vi.fn();

      renderInventoryManager({
        items: corruptedItems,
        currency: [],
        canEdit: true,
        onItemsChange: mockOnChange,
        onCurrencyChange: vi.fn(),
      });

      // All 3 items should be displayed
      expect(screen.getByText('Sword')).toBeInTheDocument();
      expect(screen.getByText('Shield')).toBeInTheDocument();
      expect(screen.getByText('Potion')).toBeInTheDocument();

      // Find and click remove button for Shield
      const shieldCard = screen.getByText('Shield').closest('[data-testid^="item-card-"]');
      const removeButton = shieldCard?.querySelector('button');
      expect(removeButton).toBeInTheDocument();

      if (removeButton) {
        fireEvent.click(removeButton);
      }

      // Verify onItemsChange was called with Shield removed
      expect(mockOnChange).toHaveBeenCalledTimes(1);
      const updatedItems = mockOnChange.mock.calls[0][0];

      // CRITICAL: Only Shield should be removed, Sword and Potion should remain
      expect(updatedItems).toHaveLength(2);
      const names = updatedItems.map((item: InventoryItem) => item.name);
      expect(names).toContain('Sword');
      expect(names).toContain('Potion');
      expect(names).not.toContain('Shield');
    });
  });

  describe('handles valid data (with IDs) normally', () => {
    it('does not log warnings when data has IDs', () => {
      vi.mocked(logger.warn).mockClear();

      const validCurrency: CurrencyEntry[] = [
        { id: 'currency-1', type: 'Gold', amount: 100 },
        { id: 'currency-2', type: 'Silver', amount: 50 },
      ];

      renderInventoryManager({
        items: [],
        currency: validCurrency,
        canEdit: true,
        onItemsChange: vi.fn(),
        onCurrencyChange: vi.fn(),
      });

      // No warnings should be logged for valid data
      expect(logger.warn).not.toHaveBeenCalled();
    });
  });
});
