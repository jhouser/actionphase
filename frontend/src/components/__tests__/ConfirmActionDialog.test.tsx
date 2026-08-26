import { describe, it, expect, vi } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { renderWithProviders } from '../../test-utils/render';
import { ConfirmActionDialog } from '../ConfirmActionDialog';

/**
 * Tests for the shared dialog's own behaviour. The per-action wrappers
 * (Complete/Pause/Cancel/Delete) are covered in GameConfirmationDialogs.test.tsx;
 * what matters here is the behaviour those wrappers now delegate rather than
 * each implementing — in particular the typed-confirmation gate and the
 * submit-failure path, which are the parts a misconfigured wrapper could
 * silently lose.
 */
function renderDialog(props: Partial<React.ComponentProps<typeof ConfirmActionDialog>> = {}) {
  const defaults: React.ComponentProps<typeof ConfirmActionDialog> = {
    isOpen: true,
    onClose: vi.fn(),
    onConfirm: vi.fn().mockResolvedValue(undefined),
    title: 'Do Thing',
    headline: '⚠️ Careful',
    intro: 'This will:',
    consequences: ['Break something', 'Irreversibly'],
    subjectLabel: 'You are about to act on:',
    subject: 'Test Campaign',
    tone: 'danger',
    confirmLabel: 'Do It',
    confirmPendingLabel: 'Doing...',
    confirmTestId: 'confirm-button',
    cancelLabel: 'Back Out',
    cancelTestId: 'cancel-button',
  };
  const merged = { ...defaults, ...props };
  renderWithProviders(<ConfirmActionDialog {...merged} />);
  return merged;
}

describe('ConfirmActionDialog', () => {
  it('renders the subject, headline, and every consequence', () => {
    renderDialog();
    expect(screen.getByText('Test Campaign')).toBeInTheDocument();
    expect(screen.getByText('⚠️ Careful')).toBeInTheDocument();
    expect(screen.getByText('Break something')).toBeInTheDocument();
    expect(screen.getByText('Irreversibly')).toBeInTheDocument();
  });

  it('confirms immediately when no typed confirmation is required', async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn().mockResolvedValue(undefined);
    const onClose = vi.fn();
    renderDialog({ onConfirm, onClose });

    expect(screen.getByTestId('confirm-button')).not.toBeDisabled();
    await user.click(screen.getByTestId('confirm-button'));

    await waitFor(() => {
      expect(onConfirm).toHaveBeenCalledOnce();
      expect(onClose).toHaveBeenCalled();
    });
  });

  it('renders no text input when no typed confirmation is required', () => {
    renderDialog();
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
  });

  describe('typed confirmation gate', () => {
    it('keeps confirm disabled until the exact word is typed', async () => {
      const user = userEvent.setup();
      renderDialog({ requireTypedConfirmation: 'epilogue' });

      expect(screen.getByTestId('confirm-button')).toBeDisabled();

      await user.type(screen.getByRole('textbox'), 'epi');
      expect(screen.getByTestId('confirm-button')).toBeDisabled();

      await user.type(screen.getByRole('textbox'), 'logue');
      expect(screen.getByTestId('confirm-button')).not.toBeDisabled();
    });

    it('accepts the word case-insensitively and with surrounding whitespace', async () => {
      const user = userEvent.setup();
      renderDialog({ requireTypedConfirmation: 'epilogue' });

      await user.type(screen.getByRole('textbox'), '  EPILOGUE  ');
      expect(screen.getByTestId('confirm-button')).not.toBeDisabled();
    });

    it('does not call onConfirm while the gate is unsatisfied', async () => {
      const user = userEvent.setup();
      const onConfirm = vi.fn();
      renderDialog({ requireTypedConfirmation: 'epilogue', onConfirm });

      await user.type(screen.getByRole('textbox'), 'wrong');
      await user.click(screen.getByTestId('confirm-button'));

      expect(onConfirm).not.toHaveBeenCalled();
    });

    it('clears the typed text when reopened after cancelling', async () => {
      const user = userEvent.setup();
      const onClose = vi.fn();
      renderDialog({ requireTypedConfirmation: 'epilogue', onClose });

      await user.type(screen.getByRole('textbox'), 'epilogue');
      await user.click(screen.getByTestId('cancel-button'));

      expect(onClose).toHaveBeenCalled();
      // The input is cleared on close so a reopened dialog re-arms the gate
      // rather than presenting an already-satisfied confirmation.
      expect(screen.getByRole('textbox')).toHaveValue('');
    });
  });

  describe('when onConfirm rejects', () => {
    it('stays open so the user can retry', async () => {
      const user = userEvent.setup();
      const onConfirm = vi.fn().mockRejectedValue(new Error('server said no'));
      const onClose = vi.fn();
      renderDialog({ onConfirm, onClose });

      await user.click(screen.getByTestId('confirm-button'));

      await waitFor(() => expect(onConfirm).toHaveBeenCalledOnce());
      expect(onClose).not.toHaveBeenCalled();
      expect(screen.getByTestId('confirm-button')).toBeInTheDocument();
    });

    it('re-enables the confirm button rather than leaving it stuck pending', async () => {
      const user = userEvent.setup();
      const onConfirm = vi.fn().mockRejectedValue(new Error('server said no'));
      renderDialog({ onConfirm });

      await user.click(screen.getByTestId('confirm-button'));

      await waitFor(() => {
        expect(screen.getByTestId('confirm-button')).not.toBeDisabled();
      });
    });
  });
});
