import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi } from 'vitest';
import { Modal } from './Modal';

/**
 * Modal is the shared overlay behind ~39 callers, so the things asserted here
 * are the ones a caller relies on without restating: the testids tests select
 * through, and whether a backdrop click can discard what the panel holds.
 */
describe('Modal', () => {
  const renderOpen = (props = {}) =>
    render(
      <Modal isOpen onClose={vi.fn()} title="Edit document" {...props}>
        <p>body content</p>
      </Modal>
    );

  it('renders nothing when closed', () => {
    render(
      <Modal isOpen={false} onClose={vi.fn()} title="Edit document">
        <p>body content</p>
      </Modal>
    );

    expect(screen.queryByTestId('modal-panel')).not.toBeInTheDocument();
    expect(screen.queryByText('body content')).not.toBeInTheDocument();
  });

  describe('test hooks', () => {
    it('exposes the panel, backdrop, and close button by testid', () => {
      renderOpen();

      expect(screen.getByTestId('modal-panel')).toBeInTheDocument();
      expect(screen.getByTestId('modal-backdrop')).toBeInTheDocument();
      expect(screen.getByTestId('modal-close')).toBeInTheDocument();
    });

    // The default covers the common case, so no caller has to opt in to be
    // selectable; the override exists for a screen showing two at once.
    it('renames the panel testid when a caller asks', () => {
      renderOpen({ testId: 'document-editor' });

      expect(screen.getByTestId('document-editor')).toBeInTheDocument();
      expect(screen.queryByTestId('modal-panel')).not.toBeInTheDocument();
    });

    // The close control is an icon with no text, so without this it is
    // reachable only by testid -- and unreadable to a screen reader.
    it('gives the close button an accessible name', () => {
      renderOpen();

      expect(screen.getByRole('button', { name: 'Close' })).toBe(
        screen.getByTestId('modal-close')
      );
    });
  });

  describe('dismissal', () => {
    it('closes on a backdrop click by default', async () => {
      const user = userEvent.setup();
      const onClose = vi.fn();
      renderOpen({ onClose });

      await user.click(screen.getByTestId('modal-backdrop'));

      expect(onClose).toHaveBeenCalledTimes(1);
    });

    // The case that matters for editor modals: a backdrop click is a slip, not
    // a decision, and the panel may hold a draft living only in component
    // state. Losing it to a stray click is unrecoverable.
    it('ignores a backdrop click when dismissOnBackdrop is false', async () => {
      const user = userEvent.setup();
      const onClose = vi.fn();
      renderOpen({ onClose, dismissOnBackdrop: false });

      await user.click(screen.getByTestId('modal-backdrop'));

      expect(onClose).not.toHaveBeenCalled();
    });

    // ...but closing must stay POSSIBLE, or the modal is a trap. The X is the
    // deliberate exit that survives dismissOnBackdrop={false}.
    it('still closes via the X when the backdrop is disabled', async () => {
      const user = userEvent.setup();
      const onClose = vi.fn();
      renderOpen({ onClose, dismissOnBackdrop: false });

      await user.click(screen.getByTestId('modal-close'));

      expect(onClose).toHaveBeenCalledTimes(1);
    });
  });
});
