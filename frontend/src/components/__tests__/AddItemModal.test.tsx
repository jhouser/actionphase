import { describe, it, expect } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { AddItemModal } from '../AddItemModal';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

/** Render inside a QueryClientProvider — LootTableSelector fetches via React Query. */
const renderWithQuery = (ui: React.ReactElement) => {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>);
};


describe('AddItemModal', () => {
  describe('Display', () => {
    it('renders modal with title', async () => {
      renderWithQuery(<AddItemModal onAdd={vi.fn()} onAddRandom={vi.fn()} onCancel={vi.fn()} />);

      expect(screen.getByText('Add New Item')).toBeInTheDocument();
    });

    it('renders all form fields', () => {
      renderWithQuery(<AddItemModal onAdd={vi.fn()} onAddRandom={vi.fn()} onCancel={vi.fn()} />);

      expect(screen.getByPlaceholderText(/Iron Sword/)).toBeInTheDocument();
      expect(screen.getByPlaceholderText(/Weapon, Armor/)).toBeInTheDocument();
      expect(screen.getByPlaceholderText(/Describe this item/)).toBeInTheDocument();
    });

    it('renders action buttons', () => {
      renderWithQuery(<AddItemModal onAdd={vi.fn()} onAddRandom={vi.fn()} onCancel={vi.fn()} />);

      expect(screen.getByText('Cancel')).toBeInTheDocument();
      expect(screen.getByText('Add Item')).toBeInTheDocument();
    });

    it('shows name field as required', () => {
      renderWithQuery(<AddItemModal onAdd={vi.fn()} onAddRandom={vi.fn()} onCancel={vi.fn()} />);

      const nameField = screen.getByPlaceholderText(/Iron Sword/);
      expect(nameField).toBeRequired();
    });

    it('shows quantity field with default value of 1', () => {
      renderWithQuery(<AddItemModal onAdd={vi.fn()} onAddRandom={vi.fn()} onCancel={vi.fn()} />);

      const quantityInputs = screen.getAllByRole('spinbutton');
      const qtyInput = quantityInputs.find(input => (input as HTMLInputElement).min === '1');
      expect(qtyInput).toHaveValue(1);
    });
  });

  describe('Form Input', () => {
    it('allows entering item name', async () => {
      const user = userEvent.setup();
      renderWithQuery(<AddItemModal onAdd={vi.fn()} onAddRandom={vi.fn()} onCancel={vi.fn()} />);

      const nameInput = screen.getByPlaceholderText(/Iron Sword/);
      await user.type(nameInput, 'Iron Sword');

      expect(nameInput).toHaveValue('Iron Sword');
    });

    it('allows entering quantity', () => {
      renderWithQuery(<AddItemModal onAdd={vi.fn()} onAddRandom={vi.fn()} onCancel={vi.fn()} />);

      const quantityInputs = screen.getAllByRole('spinbutton');
      const qtyInput = quantityInputs.find(input => (input as HTMLInputElement).min === '1') as HTMLInputElement;
      fireEvent.change(qtyInput, { target: { value: '5' } });

      expect(qtyInput).toHaveValue(5);
    });

    it('allows entering category', async () => {
      const user = userEvent.setup();
      renderWithQuery(<AddItemModal onAdd={vi.fn()} onAddRandom={vi.fn()} onCancel={vi.fn()} />);

      const categoryInput = screen.getByPlaceholderText(/Weapon, Armor/);
      await user.type(categoryInput, 'Weapon');

      expect(categoryInput).toHaveValue('Weapon');
    });

    it('allows entering value', () => {
      renderWithQuery(<AddItemModal onAdd={vi.fn()} onAddRandom={vi.fn()} onCancel={vi.fn()} />);

      const valueInput = screen.getByPlaceholderText('0') as HTMLInputElement;
      fireEvent.change(valueInput, { target: { value: '100' } });

      expect(valueInput).toHaveValue(100);
    });

    it('allows entering weight', () => {
      renderWithQuery(<AddItemModal onAdd={vi.fn()} onAddRandom={vi.fn()} onCancel={vi.fn()} />);

      const weightInput = screen.getByPlaceholderText('0.0') as HTMLInputElement;
      fireEvent.change(weightInput, { target: { value: '5.5' } });

      expect(weightInput).toHaveValue(5.5);
    });

    it('allows entering description', async () => {
      const user = userEvent.setup();
      renderWithQuery(<AddItemModal onAdd={vi.fn()} onAddRandom={vi.fn()} onCancel={vi.fn()} />);

      const descInput = screen.getByPlaceholderText(/Describe this item/);
      await user.type(descInput, 'A sturdy blade');

      expect(descInput).toHaveValue('A sturdy blade');
    });
  });

  describe('Form Submission', () => {
    it('calls onAdd with complete item data', async () => {
      const onAdd = vi.fn();
      const user = userEvent.setup();
      renderWithQuery(<AddItemModal onAdd={onAdd} onCancel={vi.fn()} />);

      await user.type(screen.getByLabelText(/Item Name/), 'Iron Sword');
      fireEvent.change(screen.getByLabelText(/Quantity/), { target: { value: '3' } });
      await user.type(screen.getByLabelText(/Category/), 'Weapon');
      fireEvent.change(screen.getByLabelText(/Value/), { target: { value: '100' } });
      fireEvent.change(screen.getByLabelText(/Weight/), { target: { value: '5' } });
      await user.type(screen.getByLabelText(/Description/), 'A sturdy iron blade');

      await user.click(screen.getByText('Add Item'));

      expect(onAdd).toHaveBeenCalledWith({
        name: 'Iron Sword',
        quantity: 3,
        category: 'Weapon',
        value: 100,
        weight: 5,
        description: 'A sturdy iron blade',
      });
    });

    it('calls onAdd with only required fields', async () => {
      const onAdd = vi.fn();
      const user = userEvent.setup();
      renderWithQuery(<AddItemModal onAdd={onAdd} onCancel={vi.fn()} />);

      await user.type(screen.getByLabelText(/Item Name/), 'Simple Item');

      await user.click(screen.getByText('Add Item'));

      expect(onAdd).toHaveBeenCalledWith({
        name: 'Simple Item',
        quantity: 1,
        category: undefined,
        value: undefined,
        weight: undefined,
        description: undefined,
      });
    });

    it('trims whitespace from name', async () => {
      const onAdd = vi.fn();
      const user = userEvent.setup();
      renderWithQuery(<AddItemModal onAdd={onAdd} onCancel={vi.fn()} />);

      await user.type(screen.getByLabelText(/Item Name/), '  Iron Sword  ');
      await user.click(screen.getByText('Add Item'));

      expect(onAdd).toHaveBeenCalledWith(
        expect.objectContaining({ name: 'Iron Sword' })
      );
    });

    it('trims whitespace from category', async () => {
      const onAdd = vi.fn();
      const user = userEvent.setup();
      renderWithQuery(<AddItemModal onAdd={onAdd} onCancel={vi.fn()} />);

      await user.type(screen.getByLabelText(/Item Name/), 'Sword');
      await user.type(screen.getByLabelText(/Category/), '  Weapon  ');
      await user.click(screen.getByText('Add Item'));

      expect(onAdd).toHaveBeenCalledWith(
        expect.objectContaining({ category: 'Weapon' })
      );
    });

    it('trims whitespace from description', async () => {
      const onAdd = vi.fn();
      const user = userEvent.setup();
      renderWithQuery(<AddItemModal onAdd={onAdd} onCancel={vi.fn()} />);

      await user.type(screen.getByLabelText(/Item Name/), 'Sword');
      await user.type(screen.getByLabelText(/Description/), '  A blade  ');
      await user.click(screen.getByText('Add Item'));

      expect(onAdd).toHaveBeenCalledWith(
        expect.objectContaining({ description: 'A blade' })
      );
    });

    it('sets empty category to undefined', async () => {
      const onAdd = vi.fn();
      const user = userEvent.setup();
      renderWithQuery(<AddItemModal onAdd={onAdd} onCancel={vi.fn()} />);

      await user.type(screen.getByLabelText(/Item Name/), 'Item');
      await user.type(screen.getByLabelText(/Category/), '   ');
      await user.click(screen.getByText('Add Item'));

      expect(onAdd).toHaveBeenCalledWith(
        expect.objectContaining({ category: undefined })
      );
    });

    it('sets empty description to undefined', async () => {
      const onAdd = vi.fn();
      const user = userEvent.setup();
      renderWithQuery(<AddItemModal onAdd={onAdd} onCancel={vi.fn()} />);

      await user.type(screen.getByLabelText(/Item Name/), 'Item');
      await user.type(screen.getByLabelText(/Description/), '   ');
      await user.click(screen.getByText('Add Item'));

      expect(onAdd).toHaveBeenCalledWith(
        expect.objectContaining({ description: undefined })
      );
    });

    it('does not call onAdd when name is empty', async () => {
      const onAdd = vi.fn();
      const user = userEvent.setup();
      renderWithQuery(<AddItemModal onAdd={onAdd} onCancel={vi.fn()} />);

      await user.click(screen.getByText('Add Item'));

      expect(onAdd).not.toHaveBeenCalled();
    });

    it('does not call onAdd when name is only whitespace', async () => {
      const onAdd = vi.fn();
      const user = userEvent.setup();
      renderWithQuery(<AddItemModal onAdd={onAdd} onCancel={vi.fn()} />);

      await user.type(screen.getByLabelText(/Item Name/), '   ');
      await user.click(screen.getByText('Add Item'));

      expect(onAdd).not.toHaveBeenCalled();
    });

    // `equipped` was dropped in the Phase 5 field pass. Writing it back would
    // reintroduce a key the type no longer has and that nothing reads.
    it('does not write an equipped field', async () => {
      const onAdd = vi.fn();
      const user = userEvent.setup();
      renderWithQuery(<AddItemModal onAdd={onAdd} onCancel={vi.fn()} />);

      await user.type(screen.getByLabelText(/Item Name/), 'Sword');
      await user.click(screen.getByText('Add Item'));

      expect(onAdd).toHaveBeenCalledTimes(1);
      expect(onAdd.mock.calls[0][0]).not.toHaveProperty('equipped');
    });
  });

  describe('Cancel Functionality', () => {
    it('calls onCancel when cancel button clicked', async () => {
      const onCancel = vi.fn();
      const user = userEvent.setup();
      renderWithQuery(<AddItemModal onAdd={vi.fn()} onCancel={onCancel} />);

      await user.click(screen.getByText('Cancel'));

      expect(onCancel).toHaveBeenCalledTimes(1);
    });

    it('does not call onAdd when cancelled', async () => {
      const onAdd = vi.fn();
      const user = userEvent.setup();
      renderWithQuery(<AddItemModal onAdd={onAdd} onCancel={vi.fn()} />);

      await user.type(screen.getByLabelText(/Item Name/), 'Sword');
      await user.click(screen.getByText('Cancel'));

      expect(onAdd).not.toHaveBeenCalled();
    });
  });

  describe('Number Field Behavior', () => {
    it('defaults quantity to 1 when cleared', () => {
      renderWithQuery(<AddItemModal onAdd={vi.fn()} onAddRandom={vi.fn()} onCancel={vi.fn()} />);

      const qtyInput = screen.getByLabelText(/Quantity/) as HTMLInputElement;
      fireEvent.change(qtyInput, { target: { value: '' } });

      expect(qtyInput).toHaveValue(1);
    });

    it('allows value field to be empty', () => {
      renderWithQuery(<AddItemModal onAdd={vi.fn()} onAddRandom={vi.fn()} onCancel={vi.fn()} />);

      const valueInput = screen.getByLabelText(/Value/) as HTMLInputElement;
      expect(valueInput).toHaveValue(null);
    });

    it('allows weight field to be empty', () => {
      renderWithQuery(<AddItemModal onAdd={vi.fn()} onAddRandom={vi.fn()} onCancel={vi.fn()} />);

      const weightInput = screen.getByLabelText(/Weight/) as HTMLInputElement;
      expect(weightInput).toHaveValue(null);
    });
  });
});
