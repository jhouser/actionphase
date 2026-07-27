import { describe, it, expect, vi } from 'vitest';
import { screen, fireEvent, waitFor } from '@testing-library/react';
import { renderWithProviders } from '../../../test-utils';
import { UtilityDrawer } from '../UtilityDrawer';
import type { UtilityContext } from '../types';
import type { Character } from '../../../types/characters';

function makeCharacter(overrides: Partial<Character> = {}): Character {
  return {
    id: 1,
    game_id: 10,
    name: 'Kael',
    status: 'approved',
    is_active: true,
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T00:00:00Z',
    ...overrides,
  };
}

function makeCtx(overrides: Partial<UtilityContext> = {}): UtilityContext {
  return {
    gameId: 10,
    currentPhase: null,
    isGM: false,
    isAudience: false,
    isGameCompleted: false,
    userRole: 'player',
    gameState: 'in_progress',
    isAnonymous: false,
    userCharacters: [makeCharacter()],
    allGameCharacters: [makeCharacter()],
    openCharacterSheet: vi.fn(),
    closeDrawer: vi.fn(),
    commentReadMode: 'manual',
    ...overrides,
  };
}

describe('UtilityDrawer', () => {
  it('lists available utilities when open', () => {
    renderWithProviders(
      <UtilityDrawer open onClose={vi.fn()} ctx={makeCtx()} />
    );

    expect(screen.getByTestId('utility-character-sheet')).toBeInTheDocument();
    expect(screen.getByTestId('utility-dice-roller')).toBeInTheDocument();
  });

  it('hides the character sheet utility when the user controls no character', () => {
    renderWithProviders(
      <UtilityDrawer open onClose={vi.fn()} ctx={makeCtx({ userCharacters: [] })} />
    );

    expect(screen.queryByTestId('utility-character-sheet')).not.toBeInTheDocument();
    // Dice roller is always available.
    expect(screen.getByTestId('utility-dice-roller')).toBeInTheDocument();
  });

  it('opens a utility panel and can navigate back to the list', async () => {
    renderWithProviders(
      <UtilityDrawer open onClose={vi.fn()} ctx={makeCtx()} />
    );

    fireEvent.click(screen.getByTestId('utility-dice-roller'));

    // Dice panel is now shown (has a Roll button); the list is gone.
    expect(await screen.findByRole('button', { name: /roll/i })).toBeInTheDocument();
    expect(screen.queryByTestId('utility-list')).not.toBeInTheDocument();

    // Back button returns to the list.
    fireEvent.click(screen.getByText('All utilities'));
    await waitFor(() =>
      expect(screen.getByTestId('utility-list')).toBeInTheDocument()
    );
  });

  it('does not render content when closed', () => {
    renderWithProviders(
      <UtilityDrawer open={false} onClose={vi.fn()} ctx={makeCtx()} />
    );
    expect(screen.queryByTestId('utility-dice-roller')).not.toBeInTheDocument();
  });

  it('opens the sheet immediately for a sole character', async () => {
    const openCharacterSheet = vi.fn();
    renderWithProviders(
      <UtilityDrawer
        open
        onClose={vi.fn()}
        ctx={makeCtx({
          userCharacters: [makeCharacter({ id: 7 })],
          openCharacterSheet,
        })}
      />
    );

    fireEvent.click(screen.getByTestId('utility-character-sheet'));

    await waitFor(() => expect(openCharacterSheet).toHaveBeenCalledWith(7));
  });

  it('reopens on the utility list after a sole-character sheet closed it externally', async () => {
    // Regression: with exactly one character, selecting the Character Sheet
    // utility opens the sheet, which closes the drawer via openCharacterSheet
    // (setting `open` to false in the parent) — NOT via the drawer's own back
    // button or onClose, so activeId was never reset. On reopen, the drawer was
    // still on the character-sheet panel; its mount effect re-fired and opened
    // the sheet again immediately, making the drawer unusable — you could never
    // reach the utility list to pick a different utility until a page refresh.
    const openCharacterSheet = vi.fn();
    const ctx = makeCtx({
      userCharacters: [makeCharacter({ id: 7 })],
      openCharacterSheet,
    });

    const { rerender } = renderWithProviders(
      <UtilityDrawer open onClose={vi.fn()} ctx={ctx} />
    );

    // Select the character sheet — the sole character's sheet opens immediately.
    fireEvent.click(screen.getByTestId('utility-character-sheet'));
    await waitFor(() => expect(openCharacterSheet).toHaveBeenCalledWith(7));

    // The parent closes the drawer in response to openCharacterSheet.
    rerender(<UtilityDrawer open={false} onClose={vi.fn()} ctx={ctx} />);

    // Reopen the drawer (e.g. after the user closes the sheet modal).
    rerender(<UtilityDrawer open onClose={vi.fn()} ctx={ctx} />);

    // It must show the utility list again, not a blank stuck panel.
    await waitFor(() =>
      expect(screen.getByTestId('utility-list')).toBeInTheDocument()
    );
    expect(screen.getByTestId('utility-character-sheet')).toBeInTheDocument();
  });

  it('shows a character picker when the user controls more than one character', async () => {
    const openCharacterSheet = vi.fn();
    renderWithProviders(
      <UtilityDrawer
        open
        onClose={vi.fn()}
        ctx={makeCtx({
          userCharacters: [
            makeCharacter({ id: 1, name: 'Kael' }),
            makeCharacter({ id: 2, name: 'Mirren' }),
          ],
          openCharacterSheet,
        })}
      />
    );

    fireEvent.click(screen.getByTestId('utility-character-sheet'));

    // Picker is shown; nothing opened yet.
    expect(await screen.findByTestId('character-sheet-picker')).toBeInTheDocument();
    expect(openCharacterSheet).not.toHaveBeenCalled();

    // Choosing a character opens that sheet.
    fireEvent.click(screen.getByTestId('character-sheet-open-2'));
    expect(openCharacterSheet).toHaveBeenCalledWith(2);
  });

  it('does not show the Screenshot Mode toggle for a non-anonymous game', () => {
    renderWithProviders(
      <UtilityDrawer open onClose={vi.fn()} ctx={makeCtx({ isAnonymous: false })} />
    );

    expect(screen.queryByTestId('screenshot-mode-toggle')).not.toBeInTheDocument();
  });

  it('shows and toggles Screenshot Mode for an anonymous game', () => {
    renderWithProviders(
      <UtilityDrawer open onClose={vi.fn()} ctx={makeCtx({ isAnonymous: true })} />
    );

    const toggle = screen.getByTestId('screenshot-mode-toggle');
    expect(toggle).toHaveAttribute('role', 'switch');
    expect(toggle).toHaveAttribute('aria-checked', 'false');

    fireEvent.click(toggle);
    expect(toggle).toHaveAttribute('aria-checked', 'true');

    fireEvent.click(toggle);
    expect(toggle).toHaveAttribute('aria-checked', 'false');
  });
});
