import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { SkillsManager } from '../SkillsManager';
import type { CharacterSkill } from '../../types/characters';
import { logger } from '@/services/LoggingService';

vi.mock('@/services/LoggingService', () => ({
  logger: { warn: vi.fn(), error: vi.fn(), debug: vi.fn(), info: vi.fn() }
}));

vi.mock('../AddSkillModal', () => ({
  AddSkillModal: () => <div data-testid="add-skill-modal">Add Skill Modal</div>
}));

vi.mock('../SkillCard', () => ({
  SkillCard: ({ skill, onRemove }: { skill: CharacterSkill; onRemove: () => void }) => (
    <div data-testid={`skill-card-${skill.id}`}>
      {skill.name}
      <button onClick={onRemove} data-testid={`remove-skill-${skill.id}`}>Remove</button>
    </div>
  )
}));

const renderSkills = (props: Partial<React.ComponentProps<typeof SkillsManager>> = {}) =>
  render(
    <SkillsManager
      skills={[]}
      canEdit={true}
      onSkillsChange={vi.fn()}
      label="Skills"
      {...props}
    />
  );

describe('SkillsManager', () => {
  // Same defensive-id regression the item and number managers carry: rows are
  // removed by id, so a row without one takes its neighbours with it.
  it('generates IDs defensively and logs a warning', () => {
    vi.mocked(logger.warn).mockClear();

    renderSkills({
      skills: [
        { name: 'Stealth' } as CharacterSkill, // Missing id!
        { name: 'Lockpicking' } as CharacterSkill, // Missing id!
      ],
    });

    expect(logger.warn).toHaveBeenCalledWith(
      expect.stringContaining('Skill missing id field'),
      expect.any(Object)
    );
    expect(logger.warn).toHaveBeenCalledTimes(2);
  });

  it('removes only the targeted skill after defensive ID generation', () => {
    const onSkillsChange = vi.fn();
    renderSkills({
      skills: [
        { name: 'Stealth' } as CharacterSkill,
        { name: 'Lockpicking' } as CharacterSkill,
        { name: 'Persuasion' } as CharacterSkill,
      ],
      onSkillsChange,
    });

    const card = screen.getByText('Lockpicking').closest('[data-testid^="skill-card-"]');
    fireEvent.click(card!.querySelector('button')!);

    const updated = onSkillsChange.mock.calls[0][0];
    expect(updated).toHaveLength(2);
    const names = updated.map((s: CharacterSkill) => s.name);
    expect(names).toEqual(expect.arrayContaining(['Stealth', 'Persuasion']));
    expect(names).not.toContain('Lockpicking');
  });

  it('does not warn when the data already has IDs', () => {
    vi.mocked(logger.warn).mockClear();
    renderSkills({ skills: [{ id: 's1', name: 'Stealth' }] });
    expect(logger.warn).not.toHaveBeenCalled();
  });

  describe('game-specific labels', () => {
    it('names the heading and controls after the game label', () => {
      // A Blades game calls this tab "Playbook"; nothing here may say "Skills".
      renderSkills({ label: 'Playbook' });
      expect(screen.getByRole('heading', { name: 'Playbook' })).toBeInTheDocument();
      expect(screen.getByText(/no playbook yet/i)).toBeInTheDocument();
      // The button's visible text is generic — "Add Playbook" reads wrong for a
      // control that adds one entry, and no rule can singularise GM wording.
      // The label reaches assistive tech through the accessible name instead.
      expect(screen.getByTestId('add-skill')).toHaveTextContent('Add New');
      expect(screen.getByRole('button', { name: 'Add to Playbook' })).toBeInTheDocument();
    });

    it('hides the add control from viewers who cannot edit', () => {
      renderSkills({ canEdit: false });
      expect(screen.queryByTestId('add-skill')).not.toBeInTheDocument();
    });
  });
});
