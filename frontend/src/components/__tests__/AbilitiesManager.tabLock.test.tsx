import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { AbilitiesManager } from '../AbilitiesManager';
import type { CharacterAbility, CharacterSkill } from '../../types/characters';

const abilities: CharacterAbility[] = [
  { id: 'a1', name: 'Firebolt', description: 'A bolt of fire', type: 'learned', active: true },
];
const skills: CharacterSkill[] = [
  { id: 's1', name: 'Stealth', level: 3, description: 'Sneaking', category: 'physical' },
];

const renderManager = (onDirtyChange?: (d: boolean) => void) =>
  render(
    <AbilitiesManager
      abilities={abilities}
      skills={skills}
      canEdit={true}
      onAbilitiesChange={vi.fn()}
      onSkillsChange={vi.fn()}
      onDirtyChange={onDirtyChange}
    />,
  );

/**
 * Regression for the reported bug: editing an ability and then switching to the Skills
 * sub-tab unmounted the open editor, wiping the edit — while the footer went on warning
 * about unsaved changes that no longer existed. Worst of both worlds.
 *
 * Navigation is now held until the editor is saved or cancelled, which also avoids the
 * discoverability problem of preserving hidden state: on a list of ten abilities, coming
 * back to a tab gives no clue which card was mid-edit.
 */
describe('AbilitiesManager tab lock', () => {
  it('allows switching sub-tabs when no editor is open', async () => {
    const user = userEvent.setup();
    renderManager();

    await user.click(screen.getByRole('button', { name: /skills \(1\)/i }));

    expect(screen.getByText('Stealth')).toBeInTheDocument();
  });

  it('locks the sub-tabs while an ability editor holds edits', async () => {
    const user = userEvent.setup();
    renderManager();

    await user.click(screen.getByText('✎'));
    const nameInput = await screen.findByDisplayValue('Firebolt');
    await user.type(nameInput, ' II');

    const skillsTab = screen.getByRole('button', { name: /skills \(1\)/i });
    expect(skillsTab).toBeDisabled();
    expect(screen.getByTestId('editor-lock-notice')).toBeInTheDocument();
  });

  it('keeps the edit intact because the tab click cannot unmount the editor', async () => {
    const user = userEvent.setup();
    renderManager();

    await user.click(screen.getByText('✎'));
    const nameInput = await screen.findByDisplayValue('Firebolt');
    await user.type(nameInput, ' II');

    // The click is a no-op on a disabled control, so the editor survives it.
    await user.click(screen.getByRole('button', { name: /skills \(1\)/i }));

    expect(screen.getByDisplayValue('Firebolt II')).toBeInTheDocument();
  });

  it('unlocks the sub-tabs once the editor is cancelled', async () => {
    const user = userEvent.setup();
    renderManager();

    await user.click(screen.getByText('✎'));
    const nameInput = await screen.findByDisplayValue('Firebolt');
    await user.type(nameInput, ' II');

    await user.click(screen.getByRole('button', { name: /^cancel$/i }));

    expect(screen.getByRole('button', { name: /skills \(1\)/i })).toBeEnabled();
    expect(screen.queryByTestId('editor-lock-notice')).not.toBeInTheDocument();
  });
});
