import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MessageCharacterButton } from '../MessageCharacterButton';
import type { Character } from '../../types/characters';

const mockNavigate = vi.fn();
vi.mock('react-router-dom', () => ({
  useNavigate: () => mockNavigate,
}));

vi.mock('../../hooks/useCanMessageCharacter', () => ({
  useCanMessageCharacter: () => ({ canMessage: true, gameId: 7 }),
}));

const character = { id: 42, name: 'Vex' } as Character;

describe('MessageCharacterButton', () => {
  beforeEach(() => {
    mockNavigate.mockClear();
  });

  it('navigates to the new-conversation form for the character', async () => {
    const user = userEvent.setup();
    render(<MessageCharacterButton character={character} />);

    await user.click(screen.getByRole('button'));

    expect(mockNavigate).toHaveBeenCalledWith(
      '/games/7?tab=messages&newConversationWith=42',
    );
  });

  it('still navigates when onNavigate returns nothing', async () => {
    const user = userEvent.setup();
    const onNavigate = vi.fn();
    render(<MessageCharacterButton character={character} onNavigate={onNavigate} />);

    await user.click(screen.getByRole('button'));

    expect(onNavigate).toHaveBeenCalledTimes(1);
    expect(mockNavigate).toHaveBeenCalledTimes(1);
  });

  /**
   * The character sheet vetoes here to hold position while it asks about unsaved
   * edits. Navigating anyway would unmount the sheet and destroy them — the whole
   * point of asking.
   */
  it('cancels navigation when onNavigate returns false', async () => {
    const user = userEvent.setup();
    const onNavigate = vi.fn(() => false);
    render(<MessageCharacterButton character={character} onNavigate={onNavigate} />);

    await user.click(screen.getByRole('button'));

    expect(onNavigate).toHaveBeenCalledTimes(1);
    expect(mockNavigate).not.toHaveBeenCalled();
  });
});
