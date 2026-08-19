import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { NumberForm } from './NumberForm';

describe('NumberForm', () => {
  describe('Submit guard', () => {
    it('does not call onSubmit when name is empty', async () => {
      const onSubmit = vi.fn();
      const user = userEvent.setup();
      render(
        <NumberForm
          onSubmit={onSubmit}
          onCancel={vi.fn()}
          submitLabel="Add"
        />
      );

      // Leave the name empty, only fill the current value
      const amountInput = screen.getByLabelText(/^current$/i);
      await user.clear(amountInput);
      await user.type(amountInput, '10');

      await user.click(screen.getByRole('button', { name: /^add$/i }));

      expect(onSubmit).not.toHaveBeenCalled();
    });
  });

  describe('Decimal support', () => {
    it('accepts decimal amount', async () => {
      const onSubmit = vi.fn();
      const user = userEvent.setup();
      render(
        <NumberForm
          onSubmit={onSubmit}
          onCancel={vi.fn()}
          submitLabel="Add"
        />
      );

      await user.type(screen.getByLabelText(/^name/i), 'Gold');

      const amountInput = screen.getByLabelText(/^current$/i);
      await user.clear(amountInput);
      await user.type(amountInput, '1.5');

      await user.click(screen.getByRole('button', { name: /^add$/i }));

      expect(onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ amount: 1.5 })
      );
    });
  });

  // `max` is what makes a track possible, so `display` is meaningless without
  // it — the control stays hidden and the key is never persisted alone.
  describe('Maximum and display mode', () => {
    it('hides the display control until a maximum is entered', async () => {
      const user = userEvent.setup();
      render(<NumberForm onSubmit={vi.fn()} onCancel={vi.fn()} submitLabel="Add" />);

      expect(screen.queryByLabelText(/display as/i)).not.toBeInTheDocument();

      await user.type(screen.getByLabelText(/^maximum$/i), '9');

      expect(screen.getByLabelText(/display as/i)).toBeInTheDocument();
    });

    it('submits max and display together', async () => {
      const onSubmit = vi.fn();
      const user = userEvent.setup();
      render(<NumberForm onSubmit={onSubmit} onCancel={vi.fn()} submitLabel="Add" />);

      await user.type(screen.getByLabelText(/^name/i), 'Stress');
      await user.type(screen.getByLabelText(/^current$/i), '4');
      await user.type(screen.getByLabelText(/^maximum$/i), '9');
      await user.selectOptions(screen.getByLabelText(/display as/i), 'boxes');

      await user.click(screen.getByRole('button', { name: /^add$/i }));

      expect(onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ name: 'Stress', amount: 4, max: 9, display: 'boxes' })
      );
    });

    it('omits max and display for an unbounded entry', async () => {
      const onSubmit = vi.fn();
      const user = userEvent.setup();
      render(<NumberForm onSubmit={onSubmit} onCancel={vi.fn()} submitLabel="Add" />);

      await user.type(screen.getByLabelText(/^name/i), 'Gold');
      await user.type(screen.getByLabelText(/^current$/i), '50');

      await user.click(screen.getByRole('button', { name: /^add$/i }));

      expect(onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ max: undefined, display: undefined })
      );
    });

    // 'number' is what an absent key already means; storing it would put a
    // default into the document, which the sparse-config rule exists to prevent.
    it('does not persist the default display mode', async () => {
      const onSubmit = vi.fn();
      const user = userEvent.setup();
      render(<NumberForm onSubmit={onSubmit} onCancel={vi.fn()} submitLabel="Add" />);

      await user.type(screen.getByLabelText(/^name/i), 'Stress');
      await user.type(screen.getByLabelText(/^maximum$/i), '9');

      await user.click(screen.getByRole('button', { name: /^add$/i }));

      expect(onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ max: 9, display: undefined })
      );
    });

    // Zero boxes is not a track, so a non-positive maximum is treated as unset
    // rather than stored as a bound nothing can render against.
    it('treats a zero maximum as unset', async () => {
      const onSubmit = vi.fn();
      const user = userEvent.setup();
      render(<NumberForm onSubmit={onSubmit} onCancel={vi.fn()} submitLabel="Add" />);

      await user.type(screen.getByLabelText(/^name/i), 'Gold');
      await user.type(screen.getByLabelText(/^maximum$/i), '0');

      expect(screen.queryByLabelText(/display as/i)).not.toBeInTheDocument();

      await user.click(screen.getByRole('button', { name: /^add$/i }));

      expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({ max: undefined }));
    });
  });

  describe('Description field uses markdown editor', () => {
    it('renders Write/Preview tabs for description field', () => {
      render(
        <NumberForm
          onSubmit={vi.fn()}
          onCancel={vi.fn()}
          submitLabel="Add"
        />
      );

      // CommentEditor renders Write/Preview tabs — plain Input/Textarea does not
      expect(screen.getByRole('button', { name: /^write$/i })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /^preview$/i })).toBeInTheDocument();
    });

    it('pre-populates description editor with initial value', () => {
      render(
        <NumberForm
          onSubmit={vi.fn()}
          onCancel={vi.fn()}
          submitLabel="Add"
          initialValues={{ description: 'Earned from quests' }}
        />
      );

      expect(screen.getByDisplayValue('Earned from quests')).toBeInTheDocument();
    });
  });
});
