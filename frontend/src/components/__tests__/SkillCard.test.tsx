import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { SkillCard } from '../SkillCard';
import type { CharacterSkill } from '../../types/characters';

const mockSkill: CharacterSkill = {
  id: '1',
  name: 'Swordsmanship',
  rank: 'Expert',
  description: 'Mastery of blade combat',
  category: 'Combat',
};

describe('SkillCard', () => {
  describe('Display - Basic Info', () => {
    it('displays skill name', () => {
      render(
        <SkillCard
          skill={mockSkill}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.getByText('Swordsmanship')).toBeInTheDocument();
    });

    it('displays rank when provided', () => {
      render(
        <SkillCard
          skill={mockSkill}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.getByText('Rank: Expert')).toBeInTheDocument();
    });

    it('hides rank when not provided', () => {
      const skillWithoutRank = { ...mockSkill, rank: undefined };
      render(
        <SkillCard
          skill={skillWithoutRank}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.queryByText(/Rank:/)).not.toBeInTheDocument();
    });

    // The legacy `level` key is resolved on read rather than migrated, so an
    // unmigrated row must still render — including a numeric one, which is what
    // the old `number | string` union allowed on disk.
    it('falls back to the legacy numeric level when rank is unset', () => {
      const skillWithNumericLevel = { ...mockSkill, rank: undefined, level: 5 };
      render(
        <SkillCard
          skill={skillWithNumericLevel}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.getByText('Rank: 5')).toBeInTheDocument();
    });

    it('shows description button when description provided', () => {
      render(
        <SkillCard
          skill={mockSkill}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.getByText('Description')).toBeInTheDocument();
    });

    it('hides description button when not provided', () => {
      const skillWithoutDesc = { ...mockSkill, description: undefined };
      render(
        <SkillCard
          skill={skillWithoutDesc}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.queryByText('Description')).not.toBeInTheDocument();
      expect(screen.queryByText('Mastery of blade combat')).not.toBeInTheDocument();
    });
  });

  describe('Category Display', () => {
    it('displays category badge when provided', () => {
      render(
        <SkillCard
          skill={mockSkill}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.getByText('Combat')).toBeInTheDocument();
    });

    it('hides category badge when not provided', () => {
      const skillWithoutCategory = { ...mockSkill, category: undefined };
      render(
        <SkillCard
          skill={skillWithoutCategory}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.queryByText('Combat')).not.toBeInTheDocument();
    });
  });

  // The parent merges updates onto the existing entry, so a save that only sets
  // `rank` leaves the legacy `level` in place — a row carrying two spellings of
  // the same value, disagreeing, with the display silently preferring one.
  // The action buttons render as bare emoji. Without an explicit label their
  // accessible name is the glyph itself — a screen reader announces "🗑" — which
  // is what happened when AbilityCard (which did label them) was retired and
  // skills inherited the surface without inheriting the labels.
  describe('Accessible names', () => {
    it('labels the edit and remove buttons', () => {
      render(
        <SkillCard skill={mockSkill} canEdit={true} onUpdate={vi.fn()} onRemove={vi.fn()} />
      );

      expect(screen.getByRole('button', { name: 'Edit skill' })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Remove skill' })).toBeInTheDocument();
    });
  });

  describe('Legacy level key', () => {
    it('clears the legacy key when a legacy row is saved', async () => {
      const user = userEvent.setup();
      const onUpdate = vi.fn();
      const legacy = { ...mockSkill, rank: undefined, level: 'Expert' };
      render(
        <SkillCard skill={legacy} canEdit={true} onUpdate={onUpdate} onRemove={vi.fn()} />
      );

      await user.click(screen.getByText('✎'));
      const rankInput = screen.getByDisplayValue('Expert');
      await user.clear(rankInput);
      await user.type(rankInput, 'Master');
      await user.click(screen.getByRole('button', { name: /save/i }));

      expect(onUpdate).toHaveBeenCalledWith(
        expect.objectContaining({ rank: 'Master', level: undefined })
      );
    });
  });

  describe('Edit Controls', () => {
    it('hides edit buttons when canEdit is false', () => {
      render(
        <SkillCard
          skill={mockSkill}
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
        <SkillCard
          skill={mockSkill}
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
        <SkillCard
          skill={mockSkill}
          canEdit={true}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      await user.click(screen.getByText('✎'));

      expect(screen.getByDisplayValue('Swordsmanship')).toBeInTheDocument();
      expect(screen.getByDisplayValue('Expert')).toBeInTheDocument();
      expect(screen.getByDisplayValue('Mastery of blade combat')).toBeInTheDocument();
      expect(screen.getByDisplayValue('Combat')).toBeInTheDocument();
    });

    it('shows Save and Cancel buttons in edit mode', async () => {
      const user = userEvent.setup();
      render(
        <SkillCard
          skill={mockSkill}
          canEdit={true}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      await user.click(screen.getByText('✎'));

      expect(screen.getByRole('button', { name: 'Save' })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument();
    });

    it('allows editing skill name', async () => {
      const user = userEvent.setup();
      render(
        <SkillCard
          skill={mockSkill}
          canEdit={true}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      await user.click(screen.getByText('✎'));
      const nameInput = screen.getByDisplayValue('Swordsmanship');
      await user.clear(nameInput);
      await user.type(nameInput, 'Archery');

      expect(screen.getByDisplayValue('Archery')).toBeInTheDocument();
    });

    it('allows editing rank', async () => {
      const user = userEvent.setup();
      render(
        <SkillCard
          skill={mockSkill}
          canEdit={true}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      await user.click(screen.getByText('✎'));
      const rankInput = screen.getByDisplayValue('Expert');
      await user.clear(rankInput);
      await user.type(rankInput, 'Master');

      expect(screen.getByDisplayValue('Master')).toBeInTheDocument();
    });

    it('allows editing description', async () => {
      const user = userEvent.setup();
      render(
        <SkillCard
          skill={mockSkill}
          canEdit={true}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      await user.click(screen.getByText('✎'));
      const descInput = screen.getByDisplayValue('Mastery of blade combat');
      await user.clear(descInput);
      await user.type(descInput, 'Advanced weapon techniques');

      expect(screen.getByDisplayValue('Advanced weapon techniques')).toBeInTheDocument();
    });

    it('allows editing category', async () => {
      const user = userEvent.setup();
      render(
        <SkillCard
          skill={mockSkill}
          canEdit={true}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      await user.click(screen.getByText('✎'));
      const categoryInput = screen.getByDisplayValue('Combat');
      await user.clear(categoryInput);
      await user.type(categoryInput, 'Social');

      expect(screen.getByDisplayValue('Social')).toBeInTheDocument();
    });
  });

  describe('Save Functionality', () => {
    it('calls onUpdate with all field values when saved', async () => {
      const onUpdate = vi.fn();
      const user = userEvent.setup();
      render(
        <SkillCard
          skill={mockSkill}
          canEdit={true}
          onUpdate={onUpdate}
          onRemove={vi.fn()}
        />
      );

      await user.click(screen.getByText('✎'));

      const nameInput = screen.getByDisplayValue('Swordsmanship');
      await user.clear(nameInput);
      await user.type(nameInput, 'Archery');

      const rankInput = screen.getByDisplayValue('Expert');
      await user.clear(rankInput);
      await user.type(rankInput, 'Novice');

      const categoryInput = screen.getByDisplayValue('Combat');
      await user.clear(categoryInput);
      await user.type(categoryInput, 'Ranged');

      await user.click(screen.getByRole('button', { name: 'Save' }));

      expect(onUpdate).toHaveBeenCalledWith(
        expect.objectContaining({
          name: 'Archery',
          rank: 'Novice',
          category: 'Ranged',
        })
      );
    });

    it('exits edit mode after save', async () => {
      const user = userEvent.setup();
      render(
        <SkillCard
          skill={mockSkill}
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
      render(
        <SkillCard
          skill={mockSkill}
          canEdit={true}
          onUpdate={onUpdate}
          onRemove={vi.fn()}
        />
      );

      await user.click(screen.getByText('✎'));

      const nameInput = screen.getByDisplayValue('Swordsmanship');
      await user.clear(nameInput);
      await user.type(nameInput, 'Changed Name');

      await user.click(screen.getByRole('button', { name: 'Cancel' }));

      expect(onUpdate).not.toHaveBeenCalled();
      expect(screen.getByText('Swordsmanship')).toBeInTheDocument();
      expect(screen.queryByRole('button', { name: 'Cancel' })).not.toBeInTheDocument();
    });
  });

  describe('Remove Functionality', () => {
    it('calls onRemove when delete button clicked', async () => {
      const onRemove = vi.fn();
      const user = userEvent.setup();
      render(
        <SkillCard
          skill={mockSkill}
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
        <SkillCard
          skill={mockSkill}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.getByLabelText('Expand description')).toBeInTheDocument();
      expect(screen.getByText('Description')).toBeInTheDocument();
    });

    it('hides description button when no description', () => {
      const skillWithoutDesc = { ...mockSkill, description: undefined };
      render(
        <SkillCard
          skill={skillWithoutDesc}
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
        <SkillCard
          skill={mockSkill}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.getByLabelText('Expand description')).toBeInTheDocument();
      expect(screen.queryByText('Mastery of blade combat')).not.toBeInTheDocument();
    });

    it('expands description when button clicked', async () => {
      const user = userEvent.setup();
      render(
        <SkillCard
          skill={mockSkill}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      expect(screen.queryByText('Mastery of blade combat')).not.toBeInTheDocument();

      await user.click(screen.getByLabelText('Expand description'));

      expect(screen.getByText('Mastery of blade combat')).toBeInTheDocument();
    });

    it('collapses description when button clicked again', async () => {
      const user = userEvent.setup();
      render(
        <SkillCard
          skill={mockSkill}
          canEdit={false}
          onUpdate={vi.fn()}
          onRemove={vi.fn()}
        />
      );

      await user.click(screen.getByLabelText('Expand description'));
      expect(screen.getByText('Mastery of blade combat')).toBeInTheDocument();

      await user.click(screen.getByLabelText('Collapse description'));
      expect(screen.queryByText('Mastery of blade combat')).not.toBeInTheDocument();
    });

    it('changes aria-label when expanded/collapsed', async () => {
      const user = userEvent.setup();
      render(
        <SkillCard
          skill={mockSkill}
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
