import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, fireEvent } from '@testing-library/react';
import { renderWithProviders } from '../../../test-utils';
import { GlobalUtilityDrawer } from '../GlobalUtilityDrawer';
import { useUtilityDrawer } from '../../../contexts/UtilityDrawerContext';
import type { OpenCharacterSheetOptions } from '../types';

// Probe the props the sheet is handed rather than mounting it. (This used to be
// forced — the real sheet hung under jsdom until its render loop was fixed.)
// The wiring is what's under test here: the drawer mounts at the app root,
// outside any GameProvider, so anything the sheet would normally read from
// GameContext has to arrive through these props or it silently defaults.
vi.mock('../../CharacterSheet', () => ({
  CharacterSheet: (props: Record<string, unknown>) => (
    <div
      data-testid="character-sheet-probe"
      data-portrait-avatars={String(props.portraitAvatars)}
      data-user-role={String(props.userRole)}
    >
      {/* Stands in for a nested editor gaining or losing uncommitted text, which is
          the only thing the drawer learns about the sheet's interior. */}
      <button
        data-testid="go-dirty"
        onClick={() => (props.onDirtyChange as (d: boolean) => void)?.(true)}
      />
      <button
        data-testid="go-clean"
        onClick={() => (props.onDirtyChange as (d: boolean) => void)?.(false)}
      />
    </div>
  ),
}));

/** Clicks the Modal's backdrop, which carries no role or label to query by. */
function clickBackdrop() {
  const backdrop = document.querySelector('.bg-black\\/60') as HTMLElement;
  expect(backdrop).toBeInTheDocument();
  fireEvent.click(backdrop);
}

function makeOptions(
  overrides: Partial<OpenCharacterSheetOptions> = {}
): OpenCharacterSheetOptions {
  return {
    canEdit: true,
    canEditStats: false,
    isAnonymous: false,
    userRole: 'player',
    gameState: 'in_progress',
    portraitAvatars: false,
    ...overrides,
  };
}

/** Opens the sheet through the real provider, as a panel would. */
function SheetOpener({ options }: { options: OpenCharacterSheetOptions }) {
  const { utilityContext } = useUtilityDrawer();
  return (
    <button
      onClick={() => utilityContext.openCharacterSheet(42, options)}
      data-testid="open-sheet"
    />
  );
}

describe('GlobalUtilityDrawer character sheet', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders the sheet in portrait mode when the character\'s game uses portraits', async () => {
    renderWithProviders(
      <>
        <SheetOpener options={makeOptions({ portraitAvatars: true })} />
        <GlobalUtilityDrawer />
      </>
    );

    fireEvent.click(screen.getByTestId('open-sheet'));

    // Without this the sheet falls back to GameContext, which does not exist at
    // the app root, and every character renders with a circular avatar.
    expect(await screen.findByTestId('character-sheet-probe')).toHaveAttribute(
      'data-portrait-avatars',
      'true'
    );
  });

  it('renders the sheet with circular avatars when the game does not use portraits', async () => {
    renderWithProviders(
      <>
        <SheetOpener options={makeOptions({ portraitAvatars: false })} />
        <GlobalUtilityDrawer />
      </>
    );

    fireEvent.click(screen.getByTestId('open-sheet'));

    expect(await screen.findByTestId('character-sheet-probe')).toHaveAttribute(
      'data-portrait-avatars',
      'false'
    );
  });

  /**
   * Regression: protecting in-progress edits from a stray backdrop click was
   * implemented by disabling backdrop dismiss outright, so a sheet with nothing to
   * lose could only be closed via its X. Clicking away is how most people close a
   * modal — it must keep working until there is actually something to protect.
   */
  describe('backdrop dismissal', () => {
    async function openSheet() {
      renderWithProviders(
        <>
          <SheetOpener options={makeOptions()} />
          <GlobalUtilityDrawer />
        </>
      );
      fireEvent.click(screen.getByTestId('open-sheet'));
      await screen.findByTestId('character-sheet-probe');
    }

    it('closes the sheet on a backdrop click when no editor is dirty', async () => {
      await openSheet();

      clickBackdrop();

      expect(screen.queryByTestId('character-sheet-probe')).not.toBeInTheDocument();
    });

    it('keeps the sheet open on a backdrop click while an editor is dirty', async () => {
      await openSheet();

      fireEvent.click(screen.getByTestId('go-dirty'));
      clickBackdrop();

      expect(screen.getByTestId('character-sheet-probe')).toBeInTheDocument();
    });

    it('restores backdrop dismissal once the editor reports clean again', async () => {
      await openSheet();

      fireEvent.click(screen.getByTestId('go-dirty'));
      fireEvent.click(screen.getByTestId('go-clean'));
      clickBackdrop();

      expect(screen.queryByTestId('character-sheet-probe')).not.toBeInTheDocument();
    });

    it('does not carry a dirty flag from one sheet into the next', async () => {
      // The flag lives on the drawer, which outlives any one sheet. Left stuck on
      // after a dirty sheet closes, it would silently disable the backdrop for every
      // sheet opened afterwards.
      await openSheet();

      fireEvent.click(screen.getByTestId('go-dirty'));
      clickBackdrop();
      expect(screen.getByTestId('character-sheet-probe')).toBeInTheDocument();

      // Close deliberately, then reopen a fresh sheet with nothing uncommitted.
      fireEvent.click(screen.getByTestId('go-clean'));
      clickBackdrop();
      await screen.findByTestId('open-sheet');

      fireEvent.click(screen.getByTestId('open-sheet'));
      await screen.findByTestId('character-sheet-probe');

      clickBackdrop();

      expect(screen.queryByTestId('character-sheet-probe')).not.toBeInTheDocument();
    });
  });
});
