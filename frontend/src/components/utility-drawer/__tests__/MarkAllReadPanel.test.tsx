import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, fireEvent, waitFor } from '@testing-library/react';
import { renderWithProviders } from '../../../test-utils';
import { MarkAllReadPanel } from '../panels/MarkAllReadPanel';
import type { GameUtilityContext, UtilityContext } from '../types';

const mockMarkAllCommentsRead = vi.fn();
const mockGetGamePosts = vi.fn();

vi.mock('../../../lib/api', () => ({
  apiClient: {
    messages: {
      markAllCommentsRead: (...args: unknown[]) => mockMarkAllCommentsRead(...args),
      getGamePosts: (...args: unknown[]) => mockGetGamePosts(...args),
      getManualReadCommentIDs: vi.fn().mockResolvedValue({ data: [] }),
      getUnreadCommentIDs: vi.fn().mockResolvedValue({ data: [] }),
    },
  },
}));

const baseGame: GameUtilityContext = {
  gameId: 10,
  currentPhase: { id: 5, phase_type: 'action' } as GameUtilityContext['currentPhase'],
  isGM: false,
  isAudience: false,
  isGameCompleted: false,
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

describe('MarkAllReadPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetGamePosts.mockResolvedValue({ data: [] });
    mockMarkAllCommentsRead.mockResolvedValue({ data: null });
  });

  it('calls the bulk mark-read API with the current game and phase', async () => {
    renderWithProviders(<MarkAllReadPanel ctx={baseCtx} />);

    fireEvent.click(screen.getByRole('button', { name: /mark all comments as read/i }));

    await waitFor(() => expect(mockMarkAllCommentsRead).toHaveBeenCalledWith(10, 5));
  });

  it('closes the drawer once the mutation completes', async () => {
    const closeDrawer = vi.fn();
    renderWithProviders(<MarkAllReadPanel ctx={{ ...baseCtx, closeDrawer }} />);

    fireEvent.click(screen.getByRole('button', { name: /mark all comments as read/i }));

    await waitFor(() => expect(closeDrawer).toHaveBeenCalledTimes(1));
  });

  it('does not close the drawer when the mutation fails', async () => {
    mockMarkAllCommentsRead.mockRejectedValue(new Error('boom'));
    const closeDrawer = vi.fn();

    renderWithProviders(<MarkAllReadPanel ctx={{ ...baseCtx, closeDrawer }} />);

    fireEvent.click(screen.getByRole('button', { name: /mark all comments as read/i }));

    expect(await screen.findByTestId('mark-all-read-error')).toBeInTheDocument();
    expect(closeDrawer).not.toHaveBeenCalled();
  });

  it('disables the button when there is no current phase', () => {
    renderWithProviders(
      <MarkAllReadPanel ctx={{ ...baseCtx, game: { ...baseGame, currentPhase: null } }} />
    );

    expect(screen.getByRole('button', { name: /mark all comments as read/i })).toBeDisabled();
  });
});
