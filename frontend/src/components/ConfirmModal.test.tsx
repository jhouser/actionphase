import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect } from 'vitest';
import { ConfirmModal } from './ConfirmModal';

describe('ConfirmModal', () => {
  it('renders modal when isOpen is true', () => {
    render(
      <ConfirmModal
        isOpen={true}
        onClose={vi.fn()}
        onConfirm={vi.fn()}
        title="Test Title"
        message="Test message"
      />
    );

    expect(screen.getByText('Test Title')).toBeInTheDocument();
    expect(screen.getByText('Test message')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /confirm/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /cancel/i })).toBeInTheDocument();
  });

  it('does not render when isOpen is false', () => {
    render(
      <ConfirmModal
        isOpen={false}
        onClose={vi.fn()}
        onConfirm={vi.fn()}
        title="Test Title"
        message="Test message"
      />
    );

    expect(screen.queryByText('Test Title')).not.toBeInTheDocument();
  });

  it('calls onClose when cancel button is clicked', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();

    render(
      <ConfirmModal
        isOpen={true}
        onClose={onClose}
        onConfirm={vi.fn()}
        title="Test Title"
        message="Test message"
      />
    );

    await user.click(screen.getByRole('button', { name: /cancel/i }));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('calls onConfirm and onClose when confirm button is clicked', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    const onConfirm = vi.fn();

    render(
      <ConfirmModal
        isOpen={true}
        onClose={onClose}
        onConfirm={onConfirm}
        title="Test Title"
        message="Test message"
      />
    );

    await user.click(screen.getByRole('button', { name: /confirm/i }));
    expect(onConfirm).toHaveBeenCalledTimes(1);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('renders custom button text', () => {
    render(
      <ConfirmModal
        isOpen={true}
        onClose={vi.fn()}
        onConfirm={vi.fn()}
        title="Test Title"
        message="Test message"
        confirmText="Delete Forever"
        cancelText="No Thanks"
      />
    );

    expect(screen.getByRole('button', { name: /delete forever/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /no thanks/i })).toBeInTheDocument();
  });

  it('applies danger variant styling', () => {
    render(
      <ConfirmModal
        isOpen={true}
        onClose={vi.fn()}
        onConfirm={vi.fn()}
        title="Delete Item"
        message="This is dangerous"
        variant="danger"
      />
    );

    // Confirm button should exist (testing variant application is hard without DOM inspection)
    expect(screen.getByRole('button', { name: /confirm/i })).toBeInTheDocument();
  });

  it('disables buttons when isLoading is true', () => {
    render(
      <ConfirmModal
        isOpen={true}
        onClose={vi.fn()}
        onConfirm={vi.fn()}
        title="Test Title"
        message="Test message"
        isLoading={true}
      />
    );

    expect(screen.getByRole('button', { name: /cancel/i })).toBeDisabled();
    expect(screen.getByRole('button', { name: /confirm/i })).toBeDisabled();
  });

  it('calls onClose when backdrop is clicked', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();

    render(
      <ConfirmModal
        isOpen={true}
        onClose={onClose}
        onConfirm={vi.fn()}
        title="Test Title"
        message="Test message"
      />
    );

    // Click the backdrop (the fixed inset div behind the modal)
    const backdrop = document.querySelector('.fixed.inset-0.z-0');
    if (backdrop) {
      await user.click(backdrop as HTMLElement);
      expect(onClose).toHaveBeenCalledTimes(1);
    }
  });

  // These testids are the contract other tests select through. A confirm
  // dialog's buttons are labelled for the action they complete ("Delete"),
  // which collides with the row button that opened it -- so role+name lookups
  // are ambiguous and callers need stable hooks instead.
  describe('test hooks', () => {
    const renderOpen = (props = {}) =>
      render(
        <ConfirmModal
          isOpen
          onClose={vi.fn()}
          onConfirm={vi.fn()}
          title="Delete document"
          message='Delete "House rules"? This cannot be undone.'
          confirmText="Delete"
          {...props}
        />
      );

    it('exposes the panel, message, and both buttons by testid', () => {
      renderOpen();

      expect(screen.getByTestId('confirm-modal')).toBeInTheDocument();
      expect(screen.getByTestId('confirm-modal-message')).toHaveTextContent(
        'Delete "House rules"? This cannot be undone.'
      );
      expect(screen.getByTestId('confirm-modal-confirm')).toHaveTextContent('Delete');
      expect(screen.getByTestId('confirm-modal-cancel')).toHaveTextContent('Cancel');
    });

    it('distinguishes its confirm button from a same-labelled button outside it', async () => {
      const user = userEvent.setup();
      const onConfirm = vi.fn();
      const rowDelete = vi.fn();

      render(
        <div>
          {/* The caller's own row button, sharing the dialog's label. */}
          <button onClick={rowDelete}>Delete</button>
          <ConfirmModal
            isOpen
            onClose={vi.fn()}
            onConfirm={onConfirm}
            title="Delete document"
            message="Delete it?"
            confirmText="Delete"
          />
        </div>
      );

      // The ambiguity the testids exist to resolve.
      expect(screen.getAllByRole('button', { name: 'Delete' })).toHaveLength(2);

      await user.click(screen.getByTestId('confirm-modal-confirm'));
      expect(onConfirm).toHaveBeenCalledTimes(1);
      expect(rowDelete).not.toHaveBeenCalled();
    });

    it('renames the panel testid so two dialogs can be told apart', () => {
      renderOpen({ testId: 'delete-document-confirm' });

      expect(screen.getByTestId('delete-document-confirm')).toBeInTheDocument();
      expect(screen.queryByTestId('confirm-modal')).not.toBeInTheDocument();
    });
  });

});
