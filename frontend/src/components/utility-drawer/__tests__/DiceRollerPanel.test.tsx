import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, fireEvent, waitFor } from '@testing-library/react';
import { renderWithProviders } from '../../../test-utils';
import { DiceRollerPanel } from '../panels/DiceRollerPanel';
import type { GameUtilityContext, UtilityContext } from '../types';
import { copyToClipboard } from '../../../utils/clipboard';

vi.mock('../../../utils/clipboard', () => ({
  copyToClipboard: vi.fn().mockResolvedValue(true),
}));

const baseGame: GameUtilityContext = {
  gameId: 10,
  currentPhase: null,
  isGM: false,
  isAudience: false,
  isGameWritable: true,
  userRole: 'player',
  gameState: 'in_progress',
  isAnonymous: false,
  userCharacters: [],
  allGameCharacters: [],
  commentReadMode: 'manual',
};

const baseCtx: UtilityContext = {
  game: baseGame,
  openCharacterSheet: vi.fn(),
  openHandout: vi.fn(),
    closeDrawer: vi.fn(),
};

describe('DiceRollerPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('rolls a d20 and shows a total within range', () => {
    renderWithProviders(<DiceRollerPanel ctx={baseCtx} />);

    fireEvent.click(screen.getByRole('button', { name: /^d20$/i }));

    const total = Number(screen.getByTestId('dice-total').textContent);
    expect(total).toBeGreaterThanOrEqual(1);
    expect(total).toBeLessThanOrEqual(20);
  });

  it('rejects invalid notation', () => {
    renderWithProviders(<DiceRollerPanel ctx={baseCtx} />);

    fireEvent.change(screen.getByLabelText('Dice notation'), {
      target: { value: 'nonsense' },
    });
    fireEvent.click(screen.getByRole('button', { name: /^roll$/i }));

    expect(screen.getByText(/valid dice notation/i)).toBeInTheDocument();
    expect(screen.queryByTestId('dice-total')).not.toBeInTheDocument();
  });

  it('copies the formatted markdown to the clipboard', async () => {
    renderWithProviders(<DiceRollerPanel ctx={baseCtx} />);

    fireEvent.click(screen.getByRole('button', { name: /^d20$/i }));
    fireEvent.click(screen.getByRole('button', { name: /copy roll/i }));

    await waitFor(() => expect(copyToClipboard).toHaveBeenCalledTimes(1));
    const copiedText = vi.mocked(copyToClipboard).mock.calls[0][0];
    expect(copiedText).toMatch(/🎲 rolled \*\*\d+\*\* \(1d20\)/);
    expect(await screen.findByText('Copied')).toBeInTheDocument();
  });

  it('attributes the roll to the sole controlled character', async () => {
    renderWithProviders(
      <DiceRollerPanel
        ctx={{
          ...baseCtx,
          game: {
            ...baseGame,
            userCharacters: [
              {
                id: 1,
                game_id: 10,
                name: 'Kael',
                status: 'approved',
                is_active: true,
                created_at: '2024-01-01T00:00:00Z',
                updated_at: '2024-01-01T00:00:00Z',
              },
            ],
          },
        }}
      />
    );

    fireEvent.click(screen.getByRole('button', { name: /^d20$/i }));
    fireEvent.click(screen.getByRole('button', { name: /copy roll/i }));

    await waitFor(() => expect(copyToClipboard).toHaveBeenCalled());
    expect(vi.mocked(copyToClipboard).mock.calls[0][0]).toContain('@Kael rolled');
  });
});
