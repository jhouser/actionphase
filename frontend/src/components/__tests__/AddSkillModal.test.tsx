import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { AddSkillModal } from '../AddSkillModal';

describe('AddSkillModal', () => {
  describe('Display', () => {
    it('renders modal with title', () => {
      render(<AddSkillModal onAdd={vi.fn()} onCancel={vi.fn()} />);

      expect(screen.getByText('Add New Skill')).toBeInTheDocument();
    });

    it('renders all form fields', () => {
      render(<AddSkillModal onAdd={vi.fn()} onCancel={vi.fn()} />);

      expect(screen.getByLabelText(/Skill Name/)).toBeInTheDocument();
      expect(screen.getByLabelText(/Rank/)).toBeInTheDocument();
      expect(screen.getByLabelText(/Category/)).toBeInTheDocument();
      expect(screen.getByLabelText(/Description/)).toBeInTheDocument();
    });

    it('renders action buttons', () => {
      render(<AddSkillModal onAdd={vi.fn()} onCancel={vi.fn()} />);

      expect(screen.getByText('Cancel')).toBeInTheDocument();
      expect(screen.getByText('Add Skill')).toBeInTheDocument();
    });

    it('shows skill name field as required', () => {
      render(<AddSkillModal onAdd={vi.fn()} onCancel={vi.fn()} />);

      const nameField = screen.getByLabelText(/Skill Name/);
      expect(nameField).toBeRequired();
    });
  });

  describe('Form Input', () => {
    it('allows entering skill name', async () => {
      const user = userEvent.setup();
      render(<AddSkillModal onAdd={vi.fn()} onCancel={vi.fn()} />);

      const nameInput = screen.getByLabelText(/Skill Name/);
      await user.type(nameInput, 'Swordsmanship');

      expect(nameInput).toHaveValue('Swordsmanship');
    });

    it('allows entering rank as text', async () => {
      const user = userEvent.setup();
      render(<AddSkillModal onAdd={vi.fn()} onCancel={vi.fn()} />);

      const rankInput = screen.getByLabelText(/Rank/);
      await user.type(rankInput, 'Expert');

      expect(rankInput).toHaveValue('Expert');
    });

    it('allows entering rank as number', async () => {
      const user = userEvent.setup();
      render(<AddSkillModal onAdd={vi.fn()} onCancel={vi.fn()} />);

      const rankInput = screen.getByLabelText(/Rank/);
      await user.type(rankInput, '5');

      expect(rankInput).toHaveValue('5');
    });

    it('allows entering category', async () => {
      const user = userEvent.setup();
      render(<AddSkillModal onAdd={vi.fn()} onCancel={vi.fn()} />);

      const categoryInput = screen.getByLabelText(/Category/);
      await user.type(categoryInput, 'Combat');

      expect(categoryInput).toHaveValue('Combat');
    });

    it('allows entering description', async () => {
      const user = userEvent.setup();
      render(<AddSkillModal onAdd={vi.fn()} onCancel={vi.fn()} />);

      const descInput = screen.getByLabelText(/Description/);
      await user.type(descInput, 'Mastery of blade combat');

      expect(descInput).toHaveValue('Mastery of blade combat');
    });
  });

  describe('Form Submission', () => {
    it('calls onAdd with complete skill data', async () => {
      const onAdd = vi.fn();
      const user = userEvent.setup();
      render(<AddSkillModal onAdd={onAdd} onCancel={vi.fn()} />);

      await user.type(screen.getByLabelText(/Skill Name/), 'Swordsmanship');
      await user.type(screen.getByLabelText(/Rank/), 'Expert');
      await user.type(screen.getByLabelText(/Category/), 'Combat');
      await user.type(screen.getByLabelText(/Description/), 'Mastery of blade combat');

      await user.click(screen.getByText('Add Skill'));

      expect(onAdd).toHaveBeenCalledWith({
        name: 'Swordsmanship',
        rank: 'Expert',
        category: 'Combat',
        description: 'Mastery of blade combat'
      });
    });

    it('calls onAdd with numeric rank', async () => {
      const onAdd = vi.fn();
      const user = userEvent.setup();
      render(<AddSkillModal onAdd={onAdd} onCancel={vi.fn()} />);

      await user.type(screen.getByLabelText(/Skill Name/), 'Archery');
      await user.type(screen.getByLabelText(/Rank/), '3');

      await user.click(screen.getByText('Add Skill'));

      expect(onAdd).toHaveBeenCalledWith({
        name: 'Archery',
        rank: '3',
        category: undefined,
        description: undefined
      });
    });

    it('calls onAdd with only required fields', async () => {
      const onAdd = vi.fn();
      const user = userEvent.setup();
      render(<AddSkillModal onAdd={onAdd} onCancel={vi.fn()} />);

      await user.type(screen.getByLabelText(/Skill Name/), 'Simple Skill');

      await user.click(screen.getByText('Add Skill'));

      expect(onAdd).toHaveBeenCalledWith({
        name: 'Simple Skill',
        rank: undefined,
        category: undefined,
        description: undefined
      });
    });

    it('trims whitespace from name', async () => {
      const onAdd = vi.fn();
      const user = userEvent.setup();
      render(<AddSkillModal onAdd={onAdd} onCancel={vi.fn()} />);

      await user.type(screen.getByLabelText(/Skill Name/), '  Swordsmanship  ');
      await user.click(screen.getByText('Add Skill'));

      expect(onAdd).toHaveBeenCalledWith(
        expect.objectContaining({ name: 'Swordsmanship' })
      );
    });

    it('trims whitespace from rank', async () => {
      const onAdd = vi.fn();
      const user = userEvent.setup();
      render(<AddSkillModal onAdd={onAdd} onCancel={vi.fn()} />);

      await user.type(screen.getByLabelText(/Skill Name/), 'Skill');
      await user.type(screen.getByLabelText(/Rank/), '  Expert  ');
      await user.click(screen.getByText('Add Skill'));

      expect(onAdd).toHaveBeenCalledWith(
        expect.objectContaining({ rank: 'Expert' })
      );
    });

    it('trims whitespace from category', async () => {
      const onAdd = vi.fn();
      const user = userEvent.setup();
      render(<AddSkillModal onAdd={onAdd} onCancel={vi.fn()} />);

      await user.type(screen.getByLabelText(/Skill Name/), 'Skill');
      await user.type(screen.getByLabelText(/Category/), '  Combat  ');
      await user.click(screen.getByText('Add Skill'));

      expect(onAdd).toHaveBeenCalledWith(
        expect.objectContaining({ category: 'Combat' })
      );
    });

    it('trims whitespace from description', async () => {
      const onAdd = vi.fn();
      const user = userEvent.setup();
      render(<AddSkillModal onAdd={onAdd} onCancel={vi.fn()} />);

      await user.type(screen.getByLabelText(/Skill Name/), 'Skill');
      await user.type(screen.getByLabelText(/Description/), '  Skill description  ');
      await user.click(screen.getByText('Add Skill'));

      expect(onAdd).toHaveBeenCalledWith(
        expect.objectContaining({ description: 'Skill description' })
      );
    });

    it('sets empty rank to undefined', async () => {
      const onAdd = vi.fn();
      const user = userEvent.setup();
      render(<AddSkillModal onAdd={onAdd} onCancel={vi.fn()} />);

      await user.type(screen.getByLabelText(/Skill Name/), 'Skill');
      await user.type(screen.getByLabelText(/Rank/), '   ');
      await user.click(screen.getByText('Add Skill'));

      expect(onAdd).toHaveBeenCalledWith(
        expect.objectContaining({ rank: undefined })
      );
    });

    it('sets empty category to undefined', async () => {
      const onAdd = vi.fn();
      const user = userEvent.setup();
      render(<AddSkillModal onAdd={onAdd} onCancel={vi.fn()} />);

      await user.type(screen.getByLabelText(/Skill Name/), 'Skill');
      await user.type(screen.getByLabelText(/Category/), '   ');
      await user.click(screen.getByText('Add Skill'));

      expect(onAdd).toHaveBeenCalledWith(
        expect.objectContaining({ category: undefined })
      );
    });

    it('sets empty description to undefined', async () => {
      const onAdd = vi.fn();
      const user = userEvent.setup();
      render(<AddSkillModal onAdd={onAdd} onCancel={vi.fn()} />);

      await user.type(screen.getByLabelText(/Skill Name/), 'Skill');
      await user.type(screen.getByLabelText(/Description/), '   ');
      await user.click(screen.getByText('Add Skill'));

      expect(onAdd).toHaveBeenCalledWith(
        expect.objectContaining({ description: undefined })
      );
    });

    it('does not call onAdd when name is empty', async () => {
      const onAdd = vi.fn();
      const user = userEvent.setup();
      render(<AddSkillModal onAdd={onAdd} onCancel={vi.fn()} />);

      await user.click(screen.getByText('Add Skill'));

      expect(onAdd).not.toHaveBeenCalled();
    });

    it('does not call onAdd when name is only whitespace', async () => {
      const onAdd = vi.fn();
      const user = userEvent.setup();
      render(<AddSkillModal onAdd={onAdd} onCancel={vi.fn()} />);

      await user.type(screen.getByLabelText(/Skill Name/), '   ');
      await user.click(screen.getByText('Add Skill'));

      expect(onAdd).not.toHaveBeenCalled();
    });
  });

  describe('Cancel Functionality', () => {
    it('calls onCancel when cancel button clicked', async () => {
      const onCancel = vi.fn();
      const user = userEvent.setup();
      render(<AddSkillModal onAdd={vi.fn()} onCancel={onCancel} />);

      await user.click(screen.getByText('Cancel'));

      expect(onCancel).toHaveBeenCalledTimes(1);
    });

    it('does not call onAdd when cancelled', async () => {
      const onAdd = vi.fn();
      const user = userEvent.setup();
      render(<AddSkillModal onAdd={onAdd} onCancel={vi.fn()} />);

      await user.type(screen.getByLabelText(/Skill Name/), 'Skill');
      await user.click(screen.getByText('Cancel'));

      expect(onAdd).not.toHaveBeenCalled();
    });
  });
});
