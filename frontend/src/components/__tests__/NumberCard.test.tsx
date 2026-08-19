import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { NumberCard } from '../NumberCard';
import type { NumberEntry } from '../../types/characters';

const mockEntry: NumberEntry = {
  id: '1',
  name: 'Gold',
  amount: 1000,
  description: 'Standard notes',
};

describe('NumberCard', () => {
  describe('Display - View Mode', () => {
    it('displays the entry name and amount', () => {
      render(
        <NumberCard
          entry={mockEntry}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.getByText('Gold')).toBeInTheDocument();
      expect(screen.getByText('1,000')).toBeInTheDocument();
    });

    it('displays description when provided', () => {
      render(
        <NumberCard
          entry={mockEntry}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.getByText('Standard notes')).toBeInTheDocument();
    });

    it('hides description when not provided', () => {
      const entryWithoutDesc = { ...mockEntry, description: undefined };
      render(
        <NumberCard
          entry={entryWithoutDesc}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.queryByText('Standard notes')).not.toBeInTheDocument();
    });
  });

  // The `type` -> `name` rename has no migration: the key lives inside a JSON
  // blob, so old rows are resolved on read instead.
  describe('Legacy name key', () => {
    it('falls back to the legacy type key when name is unset', () => {
      const legacy = { id: '1', type: 'Gold', amount: 1000 } as NumberEntry;
      render(
        <NumberCard entry={legacy} canEdit={false} onUpdate={vi.fn()} onRemove={vi.fn()} />
      );

      expect(screen.getByText('Gold')).toBeInTheDocument();
    });

    it('prefers name when a row carries both keys', () => {
      const both = { id: '1', name: 'Coin', type: 'Gold', amount: 5 } as NumberEntry;
      render(
        <NumberCard entry={both} canEdit={false} onUpdate={vi.fn()} onRemove={vi.fn()} />
      );

      expect(screen.getByText('Coin')).toBeInTheDocument();
      expect(screen.queryByText('Gold')).not.toBeInTheDocument();
    });

    // Editing a legacy row should leave it carrying one spelling of its name,
    // not both — otherwise the fallback silently keeps choosing between them.
    it('clears the legacy key when a legacy row is saved', async () => {
      const user = userEvent.setup();
      const onUpdate = vi.fn();
      const legacy = { id: '1', type: 'Gold', amount: 1000 } as NumberEntry;
      render(
        <NumberCard entry={legacy} canEdit={true} onUpdate={onUpdate} onRemove={vi.fn()} />
      );

      await user.click(screen.getByText('✎'));
      await user.click(screen.getByRole('button', { name: 'Save' }));

      expect(onUpdate).toHaveBeenCalledWith(
        expect.objectContaining({ name: 'Gold', type: undefined })
      );
    });
  });

  // See SkillCard's equivalent. Named "entry" rather than "number" because the
  // tab is GM-renameable, so the noun the user sees is not fixed.
  describe('Accessible names', () => {
    it('labels the edit and remove buttons', () => {
      render(
        <NumberCard entry={mockEntry} canEdit={true} onUpdate={vi.fn()} onRemove={vi.fn()} />
      );

      expect(screen.getByRole('button', { name: 'Edit entry' })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Remove entry' })).toBeInTheDocument();
    });

    // The editor is NumberForm, the same one SkillCard and ItemCard use, so its
    // controls are labelled buttons rather than the ✓/✕ icons this card used to
    // hand-roll — the only editor on the sheet that did not read as Save/Cancel.
    it('presents the shared form\'s save and cancel buttons while editing', async () => {
      const user = userEvent.setup();
      render(
        <NumberCard entry={mockEntry} canEdit={true} onUpdate={vi.fn()} onRemove={vi.fn()} />
      );

      await user.click(screen.getByRole('button', { name: 'Edit entry' }));

      expect(screen.getByRole('button', { name: 'Save' })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument();
      expect(screen.queryByText('✓')).not.toBeInTheDocument();
      expect(screen.queryByText('✕')).not.toBeInTheDocument();
    });
  });

  describe('Bounded tracks', () => {
    it('shows the maximum alongside the amount', () => {
      render(
        <NumberCard
          entry={{ id: '1', name: 'Stress', amount: 4, max: 9 }}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.getByText('/ 9')).toBeInTheDocument();
    });

    it('renders boxes for a boxes entry', () => {
      render(
        <NumberCard
          entry={{ id: '1', name: 'Stress', amount: 4, max: 9, display: 'boxes' }}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.getByRole('img', { name: 'Stress: 4 of 9' })).toBeInTheDocument();
    });

    it('renders a bar for a track entry', () => {
      render(
        <NumberCard
          entry={{ id: '1', name: 'Heat', amount: 3, max: 6, display: 'track' }}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.getByRole('img', { name: 'Heat: 3 of 6' })).toBeInTheDocument();
    });

    // A bare quantity has nothing to draw a bar or boxes against.
    it('draws nothing for an unbounded entry', () => {
      render(
        <NumberCard
          entry={{ id: '1', name: 'Gold', amount: 500 }}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.queryByRole('img')).not.toBeInTheDocument();
    });

    // 'number' is an explicit opt-out of the visual track, not an absent value.
    it('draws nothing when display is number even with a maximum', () => {
      render(
        <NumberCard
          entry={{ id: '1', name: 'HP', amount: 8, max: 10, display: 'number' }}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.getByText('/ 10')).toBeInTheDocument();
      expect(screen.queryByRole('img')).not.toBeInTheDocument();
    });

    // The write path never persists 'number' — NumberForm stores undefined for it
    // (see its `display !== 'number' ? display : undefined`), so this absent-display
    // shape is what a real "Number" entry with a maximum looks like on disk. The
    // literal-'number' case above passes on a shape no code path actually writes.
    it('draws nothing for a saved Number entry with a maximum', () => {
      render(
        <NumberCard
          entry={{ id: '1', name: 'HP', amount: 8, max: 10 }}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.getByText('/ 10')).toBeInTheDocument();
      expect(screen.queryByRole('img')).not.toBeInTheDocument();
    });

    // Twenty boxes is already a wide row on a phone; past that the bar carries
    // the same information legibly.
    it('falls back to a bar when a boxes entry exceeds the box limit', () => {
      render(
        <NumberCard
          entry={{ id: '1', name: 'XP', amount: 30, max: 100, display: 'boxes' }}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      const track = screen.getByRole('img', { name: 'XP: 30 of 100' });
      // The bar is a single element; the box track renders one span per box.
      expect(track.querySelectorAll('span')).toHaveLength(0);
    });
  });

  describe('Edit Controls', () => {
    it('hides edit buttons when canEdit is false', () => {
      render(
        <NumberCard
          entry={mockEntry}
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
        <NumberCard
          entry={mockEntry}
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
      render(
        <NumberCard
          entry={mockEntry}
          canEdit={true}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      await user.click(screen.getByText('✎'));

      expect(screen.getByDisplayValue('Gold')).toBeInTheDocument();
      expect(screen.getByDisplayValue('1000')).toBeInTheDocument();
      expect(screen.getByDisplayValue('Standard notes')).toBeInTheDocument();
    });

    it('shows save and cancel buttons in edit mode', async () => {
      const user = userEvent.setup();
      render(
        <NumberCard
          entry={mockEntry}
          canEdit={true}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      await user.click(screen.getByText('✎'));

      expect(screen.getByRole('button', { name: 'Save' })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument();
    });

    it('allows editing the name', async () => {
      const user = userEvent.setup();
      render(
        <NumberCard
          entry={mockEntry}
          canEdit={true}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      await user.click(screen.getByText('✎'));
      const typeInput = screen.getByDisplayValue('Gold');
      await user.clear(typeInput);
      await user.type(typeInput, 'Silver');

      expect(screen.getByDisplayValue('Silver')).toBeInTheDocument();
    });

    it('allows editing amount', async () => {
      const user = userEvent.setup();
      render(
        <NumberCard
          entry={mockEntry}
          canEdit={true}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      await user.click(screen.getByText('✎'));
      const amountInput = screen.getByDisplayValue('1000');
      await user.clear(amountInput);
      await user.type(amountInput, '2000');

      expect(screen.getByDisplayValue('2000')).toBeInTheDocument();
    });

    it('allows editing description', async () => {
      const user = userEvent.setup();
      render(
        <NumberCard
          entry={mockEntry}
          canEdit={true}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      await user.click(screen.getByText('✎'));
      const descInput = screen.getByDisplayValue('Standard notes');
      await user.clear(descInput);
      await user.type(descInput, 'Updated notes');

      expect(screen.getByDisplayValue('Updated notes')).toBeInTheDocument();
    });
  });

  describe('Save Functionality', () => {
    it('calls onUpdate with modified values when saved', async () => {
      const onUpdate = vi.fn();
      const user = userEvent.setup();
      render(
        <NumberCard
          entry={mockEntry}
          canEdit={true}
          onUpdate={onUpdate}
          onRemove={vi.fn()}
        />
      );

      await user.click(screen.getByText('✎'));

      const nameInput = screen.getByDisplayValue('Gold');
      await user.clear(nameInput);
      await user.type(nameInput, 'Platinum');

      const amountInput = screen.getByDisplayValue('1000');
      await user.clear(amountInput);
      await user.type(amountInput, '5000');

      await user.click(screen.getByRole('button', { name: 'Save' }));

      expect(onUpdate).toHaveBeenCalledWith({
        name: 'Platinum',
        amount: 5000,
        max: undefined,
        display: undefined,
        description: 'Standard notes',
        type: undefined,
      });
    });

    it('exits edit mode after save', async () => {
      const user = userEvent.setup();
      render(
        <NumberCard
          entry={mockEntry}
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

    it('sets description to undefined when empty', async () => {
      const onUpdate = vi.fn();
      const user = userEvent.setup();
      render(
        <NumberCard
          entry={mockEntry}
          canEdit={true}
          onUpdate={onUpdate}
          onRemove={vi.fn()}
        />
      );

      await user.click(screen.getByText('✎'));

      const descInput = screen.getByDisplayValue('Standard notes');
      await user.clear(descInput);

      await user.click(screen.getByRole('button', { name: 'Save' }));

      expect(onUpdate).toHaveBeenCalledWith({
        name: 'Gold',
        amount: 1000,
        max: undefined,
        display: undefined,
        description: undefined,
        type: undefined,
      });
    });
  });

  describe('Cancel Functionality', () => {
    it('reverts changes when cancelled', async () => {
      const user = userEvent.setup();
      render(
        <NumberCard
          entry={mockEntry}
          canEdit={true}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      await user.click(screen.getByText('✎'));

      const typeInput = screen.getByDisplayValue('Gold');
      await user.clear(typeInput);
      await user.type(typeInput, 'Changed');

      await user.click(screen.getByRole('button', { name: 'Cancel' }));

      // Should show original value
      expect(screen.getByText('Gold')).toBeInTheDocument();
      expect(screen.queryByDisplayValue('Changed')).not.toBeInTheDocument();
    });

    it('does not call onUpdate when cancelled', async () => {
      const onUpdate = vi.fn();
      const user = userEvent.setup();
      render(
        <NumberCard
          entry={mockEntry}
          canEdit={true}
          onUpdate={onUpdate}
          onRemove={vi.fn()}
        />
      );

      await user.click(screen.getByText('✎'));

      const typeInput = screen.getByDisplayValue('Gold');
      await user.clear(typeInput);
      await user.type(typeInput, 'Changed');

      await user.click(screen.getByRole('button', { name: 'Cancel' }));

      expect(onUpdate).not.toHaveBeenCalled();
    });

    it('exits edit mode when cancelled', async () => {
      const user = userEvent.setup();
      render(
        <NumberCard
          entry={mockEntry}
          canEdit={true}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      await user.click(screen.getByText('✎'));
      await user.click(screen.getByRole('button', { name: 'Cancel' }));

      expect(screen.queryByRole('button', { name: 'Cancel' })).not.toBeInTheDocument();
      expect(screen.getByText('✎')).toBeInTheDocument();
    });
  });

  describe('Remove Functionality', () => {
    it('calls onRemove when delete button clicked', async () => {
      const onRemove = vi.fn();
      const user = userEvent.setup();
      render(
        <NumberCard
          entry={mockEntry}
          canEdit={true}
          onUpdate={vi.fn()}
          onRemove={onRemove}
        />
      );

      await user.click(screen.getByText('🗑'));

      expect(onRemove).toHaveBeenCalledTimes(1);
    });
  });

  describe('Decimal Amount Support', () => {
    it('accepts decimal amount input', async () => {
      const onUpdate = vi.fn();
      const user = userEvent.setup();
      render(
        <NumberCard
          entry={mockEntry}
          canEdit={true}
          onUpdate={onUpdate}
          onRemove={vi.fn()}
        />
      );

      await user.click(screen.getByText('✎'));
      const amountInput = screen.getByDisplayValue('1000');
      await user.clear(amountInput);
      await user.type(amountInput, '1.5');

      await user.click(screen.getByRole('button', { name: 'Save' }));

      expect(onUpdate).toHaveBeenCalledWith(
        expect.objectContaining({ amount: 1.5 })
      );
    });
  });

  describe('Number Formatting', () => {
    it('formats large amounts with thousands separators', () => {
      const largeEntry = { ...mockEntry, amount: 1234567 };
      render(
        <NumberCard
          entry={largeEntry}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.getByText('1,234,567')).toBeInTheDocument();
    });

    it('handles zero amount correctly', () => {
      const zeroEntry = { ...mockEntry, amount: 0 };
      render(
        <NumberCard
          entry={zeroEntry}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.getByText('0')).toBeInTheDocument();
    });
  });

  describe('Description uses markdown', () => {
    it('renders markdown bold syntax in description as HTML, not raw text', () => {
      const entryWithMarkdown = { ...mockEntry, description: '**Bold note**' };
      render(
        <NumberCard
          entry={entryWithMarkdown}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      // MarkdownPreview renders **text** as <strong>, not as literal asterisks
      expect(screen.queryByText('**Bold note**')).not.toBeInTheDocument();
      expect(screen.getByText('Bold note')).toBeInTheDocument();
    });

    it('renders Write/Preview tabs in edit mode for description field', async () => {
      const user = userEvent.setup();
      render(
        <NumberCard
          entry={mockEntry}
          canEdit={true}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      await user.click(screen.getByText('✎'));

      // CommentEditor renders Write/Preview tabs — plain Input does not
      expect(screen.getByRole('button', { name: /^write$/i })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /^preview$/i })).toBeInTheDocument();
    });
  });

  describe('Unsaved-edit reporting', () => {
    it('reports clean before anything is edited', () => {
      const onDirtyChange = vi.fn();
      render(
        <NumberCard
          entry={mockEntry}
          canEdit={true}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
          onDirtyChange={onDirtyChange}
        />
      );

      // A closed card holds no editor, so it has nothing to report — the signal
      // starts when NumberForm mounts. Same as SkillCard and ItemCard.
      expect(onDirtyChange).not.toHaveBeenCalledWith(true);
    });

    it('reports dirty once a field diverges from the saved value', async () => {
      const user = userEvent.setup();
      const onDirtyChange = vi.fn();
      render(
        <NumberCard
          entry={mockEntry}
          canEdit={true}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
          onDirtyChange={onDirtyChange}
        />
      );

      await user.click(screen.getByText('✎'));
      await user.type(screen.getByDisplayValue('Gold'), 'en');

      expect(onDirtyChange).toHaveBeenLastCalledWith(true);
    });

    it('reports clean again after cancel restores the saved values', async () => {
      const user = userEvent.setup();
      const onDirtyChange = vi.fn();
      render(
        <NumberCard
          entry={mockEntry}
          canEdit={true}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
          onDirtyChange={onDirtyChange}
        />
      );

      await user.click(screen.getByText('✎'));
      await user.type(screen.getByDisplayValue('Gold'), 'en');
      await user.click(screen.getByRole('button', { name: 'Cancel' }));

      expect(onDirtyChange).toHaveBeenLastCalledWith(false);
    });

    /**
     * The editor unmounts on save, and useReportDirty guarantees a false report on
     * unmount, so the card reports clean as soon as Save is clicked rather than
     * holding dirty until the parent's write lands.
     *
     * That in-flight window used to be guarded here, by a hand-rolled inline editor
     * that stayed mounted across it. The guard was dropped when this card moved onto
     * the shared NumberForm: SkillCard and ItemCard have always reported clean on
     * save, the window is only reachable by clicking Save and then closing the sheet
     * inside two round-trips, and the mutation has no onError, so a genuinely failed
     * write was never actually surfaced by this flag.
     */
    it('reports clean once saved, without waiting for the parent to apply it', async () => {
      const user = userEvent.setup();
      const onDirtyChange = vi.fn();
      render(
        <NumberCard
          entry={mockEntry}
          canEdit={true}
          // Parent that never writes back, standing in for a save still in flight.
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
          onDirtyChange={onDirtyChange}
        />
      );

      await user.click(screen.getByText('✎'));
      await user.type(screen.getByDisplayValue('Gold'), 'en');
      expect(onDirtyChange).toHaveBeenLastCalledWith(true);

      await user.click(screen.getByRole('button', { name: 'Save' }));

      expect(onDirtyChange).toHaveBeenLastCalledWith(false);
    });

  });
});
