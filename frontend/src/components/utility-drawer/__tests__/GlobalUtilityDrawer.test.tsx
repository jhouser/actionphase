import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, fireEvent } from '@testing-library/react';
import { renderWithProviders } from '../../../test-utils';
import { GlobalUtilityDrawer } from '../GlobalUtilityDrawer';
import { useUtilityDrawer } from '../../../contexts/UtilityDrawerContext';
import type { OpenCharacterSheetOptions } from '../types';

// The real sheet hangs under jsdom, so probe the props it is handed instead.
// The wiring is what's under test here: the drawer mounts at the app root,
// outside any GameProvider, so anything the sheet would normally read from
// GameContext has to arrive through these props or it silently defaults.
vi.mock('../../CharacterSheet', () => ({
  CharacterSheet: (props: Record<string, unknown>) => (
    <div
      data-testid="character-sheet-probe"
      data-portrait-avatars={String(props.portraitAvatars)}
      data-user-role={String(props.userRole)}
    />
  ),
}));

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
});
