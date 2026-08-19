import { describe, it, expect } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ItemCard } from '../ItemCard';
import type { InventoryItem } from '../../types/characters';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

const mockItem: InventoryItem = {
  id: '1',
  name: 'Iron Sword',
  description: 'A sturdy iron blade',
  quantity: 1,
  category: 'Weapon',
  value: 100,
  weight: 5,
  condition: 'Good',
};

/** Render inside a QueryClientProvider — LootTableSelector fetches via React Query. */
const renderWithQuery = (ui: React.ReactElement) => {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>);
};

describe('ItemCard', () => {
  describe('Display - Basic Info', () => {
    it('displays item name', () => {
      render(
        <ItemCard
          item={mockItem}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.getByText('Iron Sword')).toBeInTheDocument();
    });

    it('shows description button when description provided', () => {
      render(
        <ItemCard
          item={mockItem}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.getByText('Description')).toBeInTheDocument();
    });

    it('hides description button when not provided', () => {
      const itemWithoutDesc = { ...mockItem, description: undefined };
      render(
        <ItemCard
          item={itemWithoutDesc}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.queryByText('Description')).not.toBeInTheDocument();
      expect(screen.queryByText('A sturdy iron blade')).not.toBeInTheDocument();
    });
  });

  describe('Quantity Display', () => {
    it('shows quantity badge when quantity > 1', () => {
      const multipleItems = { ...mockItem, quantity: 5 };
      render(
        <ItemCard
          item={multipleItems}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.getByText('x5')).toBeInTheDocument();
    });

    it('hides quantity badge when quantity is 1', () => {
      render(
        <ItemCard
          item={mockItem}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.queryByText('x1')).not.toBeInTheDocument();
    });
  });

  // See SkillCard's equivalent: emoji-only buttons announce as the glyph unless
  // explicitly labelled.
  describe('Accessible names', () => {
    it('labels the edit and remove buttons', () => {
      render(
        <ItemCard item={mockItem} canEdit={true} onUpdate={vi.fn()} onRemove={vi.fn()} />
      );

      expect(screen.getByRole('button', { name: 'Edit item' })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Remove item' })).toBeInTheDocument();
    });
  });

  describe('Equipped Status (retired)', () => {
    // The badge was removed in the Phase 5 field pass: nothing could ever set
    // the flag true, so it never rendered in practice. Old rows still carry the
    // key, and they must not resurrect it.
    it('renders no equipped badge for a legacy row that still carries the key', () => {
      const legacyEquippedItem = { ...mockItem, equipped: true } as InventoryItem;
      render(
        <ItemCard
          item={legacyEquippedItem}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.queryByText('Equipped')).not.toBeInTheDocument();
    });
  });

  describe('Category Display', () => {
    it('displays category badge', () => {
      render(
        <ItemCard
          item={mockItem}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.getByText('Weapon')).toBeInTheDocument();
    });

    it('applies red color for weapon category', () => {
      render(
        <ItemCard
          item={mockItem}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      const badge = screen.getByText('Weapon');
      expect(badge).toHaveClass('bg-semantic-danger-subtle', 'border-semantic-danger');
    });

    it('applies blue color for armor category', () => {
      const armorItem = { ...mockItem, category: 'Armor' };
      render(
        <ItemCard
          item={armorItem}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      const badge = screen.getByText('Armor');
      expect(badge).toHaveClass('bg-semantic-info-subtle', 'border-semantic-info');
    });

    it('applies green color for consumable category', () => {
      const consumableItem = { ...mockItem, category: 'Consumable' };
      render(
        <ItemCard
          item={consumableItem}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      const badge = screen.getByText('Consumable');
      expect(badge).toHaveClass('bg-semantic-success-subtle', 'border-semantic-success');
    });

    it('applies yellow color for tool category', () => {
      const toolItem = { ...mockItem, category: 'Tool' };
      render(
        <ItemCard
          item={toolItem}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      const badge = screen.getByText('Tool');
      expect(badge).toHaveClass('bg-semantic-warning-subtle', 'border-semantic-warning');
    });

    it('applies gray color for unknown category', () => {
      const otherItem = { ...mockItem, category: 'Misc' };
      render(
        <ItemCard
          item={otherItem}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      const badge = screen.getByText('Misc');
      expect(badge).toHaveClass('inline-flex', 'items-center');
    });

    it('hides category badge when not provided', () => {
      const itemWithoutCategory = { ...mockItem, category: undefined };
      render(
        <ItemCard
          item={itemWithoutCategory}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.queryByText('Weapon')).not.toBeInTheDocument();
    });
  });

  describe('Item Stats', () => {
    it('displays weight when provided', () => {
      render(
        <ItemCard
          item={mockItem}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.getByText('Weight: 5.0')).toBeInTheDocument();
    });

    it('displays value when provided', () => {
      render(
        <ItemCard
          item={mockItem}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.getByText('Value: 100')).toBeInTheDocument();
    });

    it('displays condition when provided', () => {
      render(
        <ItemCard
          item={mockItem}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.getByText('Good')).toBeInTheDocument();
    });

    it('calculates total weight for multiple quantities', () => {
      const multipleItems = { ...mockItem, quantity: 3, weight: 5 };
      render(
        <ItemCard
          item={multipleItems}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.getByText('Weight: 15.0')).toBeInTheDocument();
    });

    it('calculates total value for multiple quantities', () => {
      const multipleItems = { ...mockItem, quantity: 2, value: 100 };
      render(
        <ItemCard
          item={multipleItems}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.getByText('Value: 200')).toBeInTheDocument();
    });

    it('hides weight when not provided', () => {
      const itemWithoutWeight = { ...mockItem, weight: undefined };
      render(
        <ItemCard
          item={itemWithoutWeight}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.queryByText(/Weight:/)).not.toBeInTheDocument();
    });
  });

  describe('Edit Controls', () => {
    it('hides edit buttons when canEdit is false', () => {
      render(
        <ItemCard
          item={mockItem}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.queryByText('✎')).not.toBeInTheDocument();
      expect(screen.queryByText('🗑')).not.toBeInTheDocument();
    });

    it('shows edit buttons when canEdit is true', () => {
      render(
        <ItemCard
          item={mockItem}
          canEdit={true}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.getByText('✎')).toBeInTheDocument();
      expect(screen.getByText('🗑')).toBeInTheDocument();
    });
  });

  describe('Edit Mode', () => {
    it('enters edit mode when edit button clicked', async () => {
      const user = userEvent.setup();
      renderWithQuery(
        <ItemCard
          item={mockItem}
          canEdit={true}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      await user.click(screen.getByText('✎'));

      expect(screen.getByDisplayValue('Iron Sword')).toBeInTheDocument();
      expect(screen.getByDisplayValue('A sturdy iron blade')).toBeInTheDocument();
      expect(screen.getByDisplayValue('Weapon')).toBeInTheDocument();
    });

    it('shows Save and Cancel buttons in edit mode', async () => {
      const user = userEvent.setup();
      renderWithQuery(
        <ItemCard
          item={mockItem}
          canEdit={true}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      await user.click(screen.getByText('✎'));

      expect(screen.getByRole('button', { name: 'Save' })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument();
    });

    it('allows editing item name', async () => {
      const user = userEvent.setup();
      renderWithQuery(
        <ItemCard
          item={mockItem}
          canEdit={true}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      await user.click(screen.getByText('✎'));
      const nameInput = screen.getByDisplayValue('Iron Sword');
      await user.clear(nameInput);
      await user.type(nameInput, 'Steel Sword');

      expect(screen.getByDisplayValue('Steel Sword')).toBeInTheDocument();
    });

    it('allows editing quantity', async () => {
      const user = userEvent.setup();
      renderWithQuery(
        <ItemCard
          item={mockItem}
          canEdit={true}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      await user.click(screen.getByText('✎'));
      const qtyInput = screen.getByDisplayValue('1') as HTMLInputElement;
      fireEvent.change(qtyInput, { target: { value: '3' } });

      expect(qtyInput.value).toBe('3');
    });

    it('allows editing category', async () => {
      const user = userEvent.setup();
      renderWithQuery(
        <ItemCard
          item={mockItem}
          canEdit={true}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      await user.click(screen.getByText('✎'));
      const categoryInput = screen.getByDisplayValue('Weapon');
      await user.clear(categoryInput);
      await user.type(categoryInput, 'Armor');

      expect(screen.getByDisplayValue('Armor')).toBeInTheDocument();
    });

    it('allows editing description', async () => {
      const user = userEvent.setup();
      renderWithQuery(
        <ItemCard
          item={mockItem}
          canEdit={true}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      await user.click(screen.getByText('✎'));
      const descInput = screen.getByDisplayValue('A sturdy iron blade');
      await user.clear(descInput);
      await user.type(descInput, 'A shiny new blade');

      expect(screen.getByDisplayValue('A shiny new blade')).toBeInTheDocument();
    });
  });

  describe('Save Functionality', () => {
    it('calls onUpdate with all field values when saved', async () => {
      const onUpdate = vi.fn();
      const user = userEvent.setup();
      renderWithQuery(
        <ItemCard
          item={mockItem}
          canEdit={true}
          onUpdate={onUpdate}
          onRemove={vi.fn()}
        />
      );

      await user.click(screen.getByText('✎'));

      const nameInput = screen.getByDisplayValue('Iron Sword');
      await user.clear(nameInput);
      await user.type(nameInput, 'Magic Sword');

      const categoryInput = screen.getByDisplayValue('Weapon');
      await user.clear(categoryInput);
      await user.type(categoryInput, 'Artifact');

      await user.click(screen.getByRole('button', { name: 'Save' }));

      expect(onUpdate).toHaveBeenCalledWith(
        expect.objectContaining({
          name: 'Magic Sword',
          category: 'Artifact',
          value: 100,
          weight: 5,
        })
      );
    });

    it('exits edit mode after save', async () => {
      const user = userEvent.setup();
      renderWithQuery(
        <ItemCard
          item={mockItem}
          canEdit={true}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      await user.click(screen.getByText('✎'));
      await user.click(screen.getByRole('button', { name: 'Save' }));

      expect(screen.queryByRole('button', { name: 'Save' })).not.toBeInTheDocument();
      expect(screen.getByText('✎')).toBeInTheDocument();
    });
  });

  describe('Cancel Functionality', () => {
    it('reverts to view mode without calling onUpdate when cancelled', async () => {
      const onUpdate = vi.fn();
      const user = userEvent.setup();
      renderWithQuery(
        <ItemCard
          item={mockItem}
          canEdit={true}
          onUpdate={onUpdate}
          onRemove={vi.fn()}
        />
      );

      await user.click(screen.getByText('✎'));

      const nameInput = screen.getByDisplayValue('Iron Sword');
      await user.clear(nameInput);
      await user.type(nameInput, 'Changed Name');

      await user.click(screen.getByRole('button', { name: 'Cancel' }));

      expect(onUpdate).not.toHaveBeenCalled();
      expect(screen.getByText('Iron Sword')).toBeInTheDocument();
      expect(screen.queryByRole('button', { name: 'Cancel' })).not.toBeInTheDocument();
    });
  });

  describe('Remove Functionality', () => {
    it('calls onRemove when delete button clicked', async () => {
      const onRemove = vi.fn();
      const user = userEvent.setup();
      render(
        <ItemCard
          item={mockItem}
          canEdit={true}
          onUpdate={vi.fn()}
          onRemove={onRemove}
        />
      );

      await user.click(screen.getByText('🗑'));

      expect(onRemove).toHaveBeenCalledTimes(1);
    });
  });

  describe('Collapsible Description', () => {
    it('shows description button when description exists', () => {
      render(
        <ItemCard
          item={mockItem}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.getByLabelText('Expand description')).toBeInTheDocument();
      expect(screen.getByText('Description')).toBeInTheDocument();
    });

    it('hides description button when no description', () => {
      const itemWithoutDesc = { ...mockItem, description: undefined };
      render(
        <ItemCard
          item={itemWithoutDesc}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.queryByLabelText('Expand description')).not.toBeInTheDocument();
      expect(screen.queryByText('Description')).not.toBeInTheDocument();
    });

    it('hides description by default (collapsed state)', () => {
      render(
        <ItemCard
          item={mockItem}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.getByLabelText('Expand description')).toBeInTheDocument();
      expect(screen.queryByText('A sturdy iron blade')).not.toBeInTheDocument();
    });

    it('expands description when button clicked', async () => {
      const user = userEvent.setup();
      render(
        <ItemCard
          item={mockItem}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.queryByText('A sturdy iron blade')).not.toBeInTheDocument();

      await user.click(screen.getByLabelText('Expand description'));

      expect(screen.getByText('A sturdy iron blade')).toBeInTheDocument();
    });

    it('collapses description when button clicked again', async () => {
      const user = userEvent.setup();
      render(
        <ItemCard
          item={mockItem}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      await user.click(screen.getByLabelText('Expand description'));
      expect(screen.getByText('A sturdy iron blade')).toBeInTheDocument();

      await user.click(screen.getByLabelText('Collapse description'));
      expect(screen.queryByText('A sturdy iron blade')).not.toBeInTheDocument();
    });

    it('changes aria-label when expanded/collapsed', async () => {
      const user = userEvent.setup();
      render(
        <ItemCard
          item={mockItem}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      const button = screen.getByLabelText('Expand description');
      await user.click(button);

      expect(screen.getByLabelText('Collapse description')).toBeInTheDocument();
      expect(screen.queryByLabelText('Expand description')).not.toBeInTheDocument();
    });
  });
});
