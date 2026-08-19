import { describe, it, expect } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { AddNumberModal } from '../AddNumberModal';

describe('AddNumberModal', () => {
  describe('Display', () => {
    it('renders modal with title', () => {
      render(<AddNumberModal onAdd={vi.fn()} onCancel={vi.fn()} />);

      expect(screen.getByText('Add New')).toBeInTheDocument();
    });

    // The title used to follow the game's label, which produced "Add Numbers"
    // even unrenamed: the label names the whole collection, while the modal adds
    // one entry. A generic title sidesteps needing to singularise GM wording.
    it('keeps the title generic regardless of the game label', () => {
      render(<AddNumberModal onAdd={vi.fn()} onCancel={vi.fn()} />);

      expect(screen.queryByText(/Add Numbers?$/)).not.toBeInTheDocument();
    });

    it('renders all form fields', () => {
      render(<AddNumberModal onAdd={vi.fn()} onCancel={vi.fn()} />);

      expect(screen.getByLabelText(/^Name/)).toBeInTheDocument();
      expect(screen.getByLabelText(/^Current$/)).toBeInTheDocument();
      expect(screen.getByLabelText(/^Maximum$/)).toBeInTheDocument();
      expect(screen.getByLabelText(/Description/)).toBeInTheDocument();
    });

    it('renders action buttons', () => {
      render(<AddNumberModal onAdd={vi.fn()} onCancel={vi.fn()} />);

      expect(screen.getByText('Cancel')).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /^Add$/ })).toBeInTheDocument();
    });

    it('shows the name field as required', () => {
      render(<AddNumberModal onAdd={vi.fn()} onCancel={vi.fn()} />);

      const nameField = screen.getByLabelText(/^Name/);
      expect(nameField).toBeRequired();
    });

    it('shows the current field with placeholder "0"', () => {
      render(<AddNumberModal onAdd={vi.fn()} onCancel={vi.fn()} />);

      const amountField = screen.getByLabelText(/^Current$/) as HTMLInputElement;
      expect(amountField.placeholder).toBe('0');
      expect(amountField).toHaveValue(null);
    });
  });

  describe('Form Input', () => {
    it('allows entering a name', async () => {
      const user = userEvent.setup();
      render(<AddNumberModal onAdd={vi.fn()} onCancel={vi.fn()} />);

      const nameInput = screen.getByLabelText(/^Name/);
      await user.type(nameInput, 'Gold');

      expect(nameInput).toHaveValue('Gold');
    });

    it('allows entering a current value', () => {
      render(<AddNumberModal onAdd={vi.fn()} onCancel={vi.fn()} />);

      const amountInput = screen.getByLabelText(/^Current$/) as HTMLInputElement;
      fireEvent.change(amountInput, { target: { value: '1000' } });

      expect(amountInput).toHaveValue(1000);
    });

    it('allows entering description', async () => {
      const user = userEvent.setup();
      render(<AddNumberModal onAdd={vi.fn()} onCancel={vi.fn()} />);

      const descInput = screen.getByLabelText(/Description/);
      await user.type(descInput, 'Standard currency');

      expect(descInput).toHaveValue('Standard currency');
    });
  });

  describe('Form Submission', () => {
    it('calls onAdd with complete entry data', async () => {
      const onAdd = vi.fn();
      const user = userEvent.setup();
      render(<AddNumberModal onAdd={onAdd} onCancel={vi.fn()} />);

      await user.type(screen.getByLabelText(/^Name/), 'Gold');
      fireEvent.change(screen.getByLabelText(/^Current$/), { target: { value: '5000' } });
      await user.type(screen.getByLabelText(/Description/), 'Imperial gold coins');

      await user.click(screen.getByRole('button', { name: /^Add$/ }));

      expect(onAdd).toHaveBeenCalledWith({
        name: 'Gold',
        amount: 5000,
        max: undefined,
        display: undefined,
        description: 'Imperial gold coins'
      });
    });

    it('calls onAdd with only required fields', async () => {
      const onAdd = vi.fn();
      const user = userEvent.setup();
      render(<AddNumberModal onAdd={onAdd} onCancel={vi.fn()} />);

      await user.type(screen.getByLabelText(/^Name/), 'Silver');

      await user.click(screen.getByRole('button', { name: /^Add$/ }));

      expect(onAdd).toHaveBeenCalledWith({
        name: 'Silver',
        amount: 0,
        max: undefined,
        display: undefined,
        description: undefined
      });
    });

    it('trims whitespace from the name', async () => {
      const onAdd = vi.fn();
      const user = userEvent.setup();
      render(<AddNumberModal onAdd={onAdd} onCancel={vi.fn()} />);

      await user.type(screen.getByLabelText(/^Name/), '  Gold  ');
      await user.click(screen.getByRole('button', { name: /^Add$/ }));

      expect(onAdd).toHaveBeenCalledWith(
        expect.objectContaining({ name: 'Gold' })
      );
    });

    it('trims whitespace from description', async () => {
      const onAdd = vi.fn();
      const user = userEvent.setup();
      render(<AddNumberModal onAdd={onAdd} onCancel={vi.fn()} />);

      await user.type(screen.getByLabelText(/^Name/), 'Credits');
      await user.type(screen.getByLabelText(/Description/), '  Space money  ');
      await user.click(screen.getByRole('button', { name: /^Add$/ }));

      expect(onAdd).toHaveBeenCalledWith(
        expect.objectContaining({ description: 'Space money' })
      );
    });

    it('sets empty description to undefined', async () => {
      const onAdd = vi.fn();
      const user = userEvent.setup();
      render(<AddNumberModal onAdd={onAdd} onCancel={vi.fn()} />);

      await user.type(screen.getByLabelText(/^Name/), 'Gold');
      await user.type(screen.getByLabelText(/Description/), '   ');
      await user.click(screen.getByRole('button', { name: /^Add$/ }));

      expect(onAdd).toHaveBeenCalledWith(
        expect.objectContaining({ description: undefined })
      );
    });

    it('does not call onAdd when the name is empty', async () => {
      const onAdd = vi.fn();
      const user = userEvent.setup();
      render(<AddNumberModal onAdd={onAdd} onCancel={vi.fn()} />);

      await user.click(screen.getByRole('button', { name: /^Add$/ }));

      expect(onAdd).not.toHaveBeenCalled();
    });

    it('does not call onAdd when the name is only whitespace', async () => {
      const onAdd = vi.fn();
      const user = userEvent.setup();
      render(<AddNumberModal onAdd={onAdd} onCancel={vi.fn()} />);

      await user.type(screen.getByLabelText(/^Name/), '   ');
      await user.click(screen.getByRole('button', { name: /^Add$/ }));

      expect(onAdd).not.toHaveBeenCalled();
    });
  });

  describe('Cancel Functionality', () => {
    it('calls onCancel when cancel button clicked', async () => {
      const onCancel = vi.fn();
      const user = userEvent.setup();
      render(<AddNumberModal onAdd={vi.fn()} onCancel={onCancel} />);

      await user.click(screen.getByText('Cancel'));

      expect(onCancel).toHaveBeenCalledTimes(1);
    });

    it('does not call onAdd when cancelled', async () => {
      const onAdd = vi.fn();
      const user = userEvent.setup();
      render(<AddNumberModal onAdd={onAdd} onCancel={vi.fn()} />);

      await user.type(screen.getByLabelText(/^Name/), 'Gold');
      await user.click(screen.getByText('Cancel'));

      expect(onAdd).not.toHaveBeenCalled();
    });
  });

  describe('Number Field Behavior', () => {
    it('allows the current field to be empty', () => {
      render(<AddNumberModal onAdd={vi.fn()} onCancel={vi.fn()} />);

      const amountInput = screen.getByLabelText(/^Current$/) as HTMLInputElement;
      fireEvent.change(amountInput, { target: { value: '100' } });
      expect(amountInput).toHaveValue(100);

      fireEvent.change(amountInput, { target: { value: '' } });
      expect(amountInput).toHaveValue(null);
    });
  });
});
