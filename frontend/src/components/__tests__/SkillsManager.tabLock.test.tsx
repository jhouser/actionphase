import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { SkillsManager } from '../SkillsManager';
import { TabNavigation } from '../TabNavigation';
import { EditorLockNotice } from '../EditorLockNotice';
import { useDirtyChildren } from '@/hooks/useDirtyChildren';
import type { CharacterSkill } from '../../types/characters';

const skills: CharacterSkill[] = [
  { id: 's1', name: 'Stealth', level: 3, description: 'Sneaking', category: 'physical' },
];

/**
 * NOTE ON THE TAB CONTROL: TabNavigation renders a <select> at narrow widths and
 * a button strip at wide ones. jsdom reports no width, so these tests drive the
 * select. It carries the same `disabled` state the buttons do, so the lock under
 * test is the same one either way.
 */

/**
 * Stands in for CharacterSheet's tab strip: the real sheet needs a QueryClient, a
 * router and an API, none of which this regression is about. What it reproduces
 * faithfully is the structure that matters — a manager reporting dirty state up to
 * an ancestor that owns the tabs, and holds them while an editor is open.
 */
function SheetHarness() {
  const { isAnyDirty, report } = useDirtyChildren();
  const [activeTab, setActiveTab] = useState('skills');

  return (
    <div>
      <TabNavigation
        tabs={[
          { id: 'skills', label: 'Skills' },
          { id: 'inventory', label: 'Inventory' },
        ]}
        activeTab={activeTab}
        onTabChange={setActiveTab}
        disabled={isAnyDirty}
      />
      {isAnyDirty && <EditorLockNotice />}
      {activeTab === 'skills' ? (
        <SkillsManager
          skills={skills}
          canEdit={true}
          onSkillsChange={vi.fn()}
          onDirtyChange={(d) => report('skills', d)}
          label="Skills"
        />
      ) : (
        <div>Inventory tab</div>
      )}
    </div>
  );
}

/**
 * Regression for the reported bug, ported from the AbilitiesManager sub-tab suite
 * when Phase 4 removed sub-tabs: editing a row and then switching tabs unmounted
 * the open editor, wiping the edit — while the footer went on warning about
 * unsaved changes that no longer existed. Worst of both worlds.
 *
 * The hazard survived the restructure; only its location moved. Sub-tabs are gone,
 * so the lock that used to guard them now guards the sheet's top-level tabs.
 *
 * Navigation is held until the editor is saved or cancelled, which also avoids the
 * discoverability problem of preserving hidden state: on a list of ten skills,
 * coming back to a tab gives no clue which card was mid-edit.
 */
describe('character sheet tab lock', () => {
  it('allows switching tabs when no editor is open', async () => {
    const user = userEvent.setup();
    render(<SheetHarness />);

    await user.selectOptions(screen.getByRole('combobox'), 'inventory');

    expect(screen.getByText('Inventory tab')).toBeInTheDocument();
  });

  it('locks the tabs while a skill editor holds edits', async () => {
    const user = userEvent.setup();
    render(<SheetHarness />);

    await user.click(screen.getByText('✎'));
    await user.type(await screen.findByDisplayValue('Stealth'), ' II');

    expect(screen.getByRole('combobox')).toBeDisabled();
    expect(screen.getByTestId('editor-lock-notice')).toBeInTheDocument();
  });

  it('keeps the edit intact because the tab click cannot unmount the editor', async () => {
    const user = userEvent.setup();
    render(<SheetHarness />);

    await user.click(screen.getByText('✎'));
    await user.type(await screen.findByDisplayValue('Stealth'), ' II');

    // Selecting is a no-op on a disabled control, so the editor survives it.
    await user.selectOptions(screen.getByRole('combobox'), 'inventory').catch(() => {});

    expect(screen.getByDisplayValue('Stealth II')).toBeInTheDocument();
    // And the tab genuinely did not change underneath it.
    expect(screen.queryByText('Inventory tab')).not.toBeInTheDocument();
  });

  it('unlocks the tabs once the editor is cancelled', async () => {
    const user = userEvent.setup();
    render(<SheetHarness />);

    await user.click(screen.getByText('✎'));
    await user.type(await screen.findByDisplayValue('Stealth'), ' II');

    await user.click(screen.getByRole('button', { name: /^cancel$/i }));

    expect(screen.getByRole('combobox')).toBeEnabled();
    expect(screen.queryByTestId('editor-lock-notice')).not.toBeInTheDocument();
  });
});
