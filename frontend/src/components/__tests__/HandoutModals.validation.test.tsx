import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { renderWithProviders } from '../../test-utils/render';
import { CreateHandoutModal } from '../CreateHandoutModal';
import { EditHandoutModal } from '../EditHandoutModal';
import type { Handout } from '../../types/handouts';

// The content field is marked required in its label, but CommentEditor is not a
// native input so the browser cannot block submit on it. The backend now
// rejects blank content at Bind with a 400, so these modals guard client-side
// to keep an unavoidable round-trip error out of the GM's way.
describe('Handout modal content validation', () => {
  const onSubmit = vi.fn();
  const onClose = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('CreateHandoutModal', () => {
    const renderModal = () =>
      renderWithProviders(
        <CreateHandoutModal onClose={onClose} onSubmit={onSubmit} isSubmitting={false} />
      );

    it('disables submit while content is empty', () => {
      renderModal();

      expect(screen.getByRole('button', { name: /create handout/i })).toBeDisabled();
    });

    it('keeps submit disabled for whitespace-only content', async () => {
      const user = userEvent.setup();
      renderModal();

      await user.type(screen.getByTestId('handout-content-input'), '   ');

      expect(screen.getByRole('button', { name: /create handout/i })).toBeDisabled();
    });

    it('enables submit once content is entered', async () => {
      const user = userEvent.setup();
      renderModal();

      await user.type(screen.getByTestId('handout-content-input'), 'Real content');

      expect(screen.getByRole('button', { name: /create handout/i })).toBeEnabled();
    });

    it('trims content before submitting', async () => {
      const user = userEvent.setup();
      renderModal();

      await user.type(screen.getByTestId('handout-title-input'), 'World Lore');
      await user.type(screen.getByTestId('handout-content-input'), '  Padded content  ');
      await user.click(screen.getByRole('button', { name: /create handout/i }));

      expect(onSubmit).toHaveBeenCalledTimes(1);
      expect(onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ title: 'World Lore', content: 'Padded content' })
      );
    });
  });

  describe('EditHandoutModal', () => {
    const handout: Handout = {
      id: 1,
      game_id: 1,
      title: 'Player Handbook',
      content: 'Existing content.',
      status: 'draft',
      created_at: '2024-01-01T00:00:00Z',
      updated_at: '2024-01-01T12:00:00Z',
    };

    const renderModal = (override: Partial<Handout> = {}) =>
      renderWithProviders(
        <EditHandoutModal
          handout={{ ...handout, ...override }}
          onClose={onClose}
          onSubmit={onSubmit}
          isSubmitting={false}
        />
      );

    it('enables submit for a handout that already has content', () => {
      renderModal();

      expect(screen.getByRole('button', { name: /save changes/i })).toBeEnabled();
    });

    it('disables submit once content is cleared', async () => {
      const user = userEvent.setup();
      renderModal();

      await user.clear(screen.getByTestId('handout-content-input'));

      expect(screen.getByRole('button', { name: /save changes/i })).toBeDisabled();
    });

    it('trims content before submitting', async () => {
      const user = userEvent.setup();
      renderModal({ content: '' });

      await user.type(screen.getByTestId('handout-content-input'), '  Edited content  ');
      await user.click(screen.getByRole('button', { name: /save changes/i }));

      expect(onSubmit).toHaveBeenCalledTimes(1);
      expect(onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ content: 'Edited content' })
      );
    });
  });
});
