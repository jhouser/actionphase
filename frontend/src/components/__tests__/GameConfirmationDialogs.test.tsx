import { describe, it, expect, vi } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { renderWithProviders } from '../../test-utils/render';
import { DeleteGameConfirmationDialog } from '../DeleteGameConfirmationDialog';
import { CancelGameConfirmationDialog } from '../CancelGameConfirmationDialog';
import { PauseGameConfirmationDialog } from '../PauseGameConfirmationDialog';
import { CompleteGameConfirmationDialog } from '../CompleteGameConfirmationDialog';
import { EpilogueGameConfirmationDialog } from '../EpilogueGameConfirmationDialog';
import { LeaveGameConfirmationDialog } from '../LeaveGameConfirmationDialog';
import { WithdrawApplicationConfirmationDialog } from '../WithdrawApplicationConfirmationDialog';

describe('DeleteGameConfirmationDialog', () => {
  it('renders game title and delete button', () => {
    renderWithProviders(
      <DeleteGameConfirmationDialog
        isOpen
        onClose={vi.fn()}
        onConfirm={vi.fn()}
        gameTitle="Test Campaign"
      />
    );
    expect(screen.getByText('Test Campaign')).toBeInTheDocument();
    expect(screen.getByTestId('delete-game-confirm-button')).toBeInTheDocument();
  });

  it('calls onConfirm and closes when confirmed', async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn().mockResolvedValue(undefined);
    const onClose = vi.fn();
    renderWithProviders(
      <DeleteGameConfirmationDialog
        isOpen
        onClose={onClose}
        onConfirm={onConfirm}
        gameTitle="Test Campaign"
      />
    );
    await user.click(screen.getByTestId('delete-game-confirm-button'));
    await waitFor(() => {
      expect(onConfirm).toHaveBeenCalledOnce();
      expect(onClose).toHaveBeenCalled();
    });
  });

  it('calls onClose when Keep Game is clicked', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    renderWithProviders(
      <DeleteGameConfirmationDialog
        isOpen
        onClose={onClose}
        onConfirm={vi.fn()}
        gameTitle="Test Campaign"
      />
    );
    await user.click(screen.getByTestId('delete-game-cancel-button'));
    expect(onClose).toHaveBeenCalled();
  });
});

describe('CancelGameConfirmationDialog', () => {
  it('renders game title and cancel-game confirm button', () => {
    renderWithProviders(
      <CancelGameConfirmationDialog
        isOpen
        onClose={vi.fn()}
        onConfirm={vi.fn()}
        gameTitle="Adventure Game"
      />
    );
    expect(screen.getByText('Adventure Game')).toBeInTheDocument();
    expect(screen.getByTestId('cancel-game-confirm-button')).toBeInTheDocument();
  });

  it('calls onConfirm and closes when confirmed', async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn().mockResolvedValue(undefined);
    const onClose = vi.fn();
    renderWithProviders(
      <CancelGameConfirmationDialog
        isOpen
        onClose={onClose}
        onConfirm={onConfirm}
        gameTitle="Adventure Game"
      />
    );
    await user.click(screen.getByTestId('cancel-game-confirm-button'));
    await waitFor(() => {
      expect(onConfirm).toHaveBeenCalledOnce();
      expect(onClose).toHaveBeenCalled();
    });
  });

  it('calls onClose when Keep Game is clicked', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    renderWithProviders(
      <CancelGameConfirmationDialog
        isOpen
        onClose={onClose}
        onConfirm={vi.fn()}
        gameTitle="Adventure Game"
      />
    );
    await user.click(screen.getByTestId('cancel-game-keep-button'));
    expect(onClose).toHaveBeenCalled();
  });
});

describe('PauseGameConfirmationDialog', () => {
  it('renders game title and pause confirm button', () => {
    renderWithProviders(
      <PauseGameConfirmationDialog
        isOpen
        onClose={vi.fn()}
        onConfirm={vi.fn()}
        gameTitle="Epic Quest"
      />
    );
    expect(screen.getByText('Epic Quest')).toBeInTheDocument();
    expect(screen.getByTestId('pause-game-confirm-button')).toBeInTheDocument();
  });

  it('calls onConfirm and closes when confirmed', async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn().mockResolvedValue(undefined);
    const onClose = vi.fn();
    renderWithProviders(
      <PauseGameConfirmationDialog
        isOpen
        onClose={onClose}
        onConfirm={onConfirm}
        gameTitle="Epic Quest"
      />
    );
    await user.click(screen.getByTestId('pause-game-confirm-button'));
    await waitFor(() => {
      expect(onConfirm).toHaveBeenCalledOnce();
      expect(onClose).toHaveBeenCalled();
    });
  });
});

describe('CompleteGameConfirmationDialog', () => {
  it('renders game title and disabled confirm button initially', () => {
    renderWithProviders(
      <CompleteGameConfirmationDialog
        isOpen
        onClose={vi.fn()}
        onConfirm={vi.fn()}
        gameTitle="Final Chapter"
      />
    );
    expect(screen.getByText('Final Chapter')).toBeInTheDocument();
    expect(screen.getByTestId('complete-game-confirm-button')).toBeDisabled();
  });

  it('enables confirm button only after typing "completed"', async () => {
    const user = userEvent.setup();
    renderWithProviders(
      <CompleteGameConfirmationDialog
        isOpen
        onClose={vi.fn()}
        onConfirm={vi.fn()}
        gameTitle="Final Chapter"
      />
    );
    const input = screen.getByRole('textbox');
    await user.type(input, 'completed');
    expect(screen.getByTestId('complete-game-confirm-button')).not.toBeDisabled();
  });

  it('is case-insensitive for "completed" check', async () => {
    const user = userEvent.setup();
    renderWithProviders(
      <CompleteGameConfirmationDialog
        isOpen
        onClose={vi.fn()}
        onConfirm={vi.fn()}
        gameTitle="Final Chapter"
      />
    );
    await user.type(screen.getByRole('textbox'), 'COMPLETED');
    expect(screen.getByTestId('complete-game-confirm-button')).not.toBeDisabled();
  });

  it('calls onConfirm when confirmed with correct text', async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn().mockResolvedValue(undefined);
    const onClose = vi.fn();
    renderWithProviders(
      <CompleteGameConfirmationDialog
        isOpen
        onClose={onClose}
        onConfirm={onConfirm}
        gameTitle="Final Chapter"
      />
    );
    await user.type(screen.getByRole('textbox'), 'completed');
    await user.click(screen.getByTestId('complete-game-confirm-button'));
    await waitFor(() => {
      expect(onConfirm).toHaveBeenCalledOnce();
      expect(onClose).toHaveBeenCalled();
    });
  });

  it('clears input and calls onClose when Cancel is clicked', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    renderWithProviders(
      <CompleteGameConfirmationDialog
        isOpen
        onClose={onClose}
        onConfirm={vi.fn()}
        gameTitle="Final Chapter"
      />
    );
    await user.type(screen.getByRole('textbox'), 'completed');
    await user.click(screen.getByTestId('complete-game-cancel-button'));
    expect(onClose).toHaveBeenCalled();
  });
});

describe('EpilogueGameConfirmationDialog', () => {
  // The dialog mechanics (typed gate, submit failure, reset on close) are
  // covered once against ConfirmActionDialog. What matters here is the
  // epilogue-specific wiring: the right confirmation word and testids, and copy
  // that distinguishes this door from "Complete Game".
  const render = (props: Partial<React.ComponentProps<typeof EpilogueGameConfirmationDialog>> = {}) =>
    renderWithProviders(
      <EpilogueGameConfirmationDialog
        isOpen
        onClose={vi.fn()}
        onConfirm={vi.fn().mockResolvedValue(undefined)}
        gameTitle="Final Chapter"
        {...props}
      />
    );

  it('renders the game title behind a disabled confirm button', () => {
    render();
    expect(screen.getByText('Final Chapter')).toBeInTheDocument();
    expect(screen.getByTestId('epilogue-game-confirm-button')).toBeDisabled();
  });

  it('gates confirmation on typing "epilogue", not "completed"', async () => {
    // Wiring the wrong word here would let a GM confirm the wrong action from
    // muscle memory, and this transition cannot be undone.
    const user = userEvent.setup();
    render();
    const input = screen.getByRole('textbox');

    await user.type(input, 'completed');
    expect(screen.getByTestId('epilogue-game-confirm-button')).toBeDisabled();

    await user.clear(input);
    await user.type(input, 'epilogue');
    expect(screen.getByTestId('epilogue-game-confirm-button')).not.toBeDisabled();
  });

  it('calls onConfirm once the word is typed', async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn().mockResolvedValue(undefined);
    const onClose = vi.fn();
    render({ onConfirm, onClose });

    await user.type(screen.getByRole('textbox'), 'epilogue');
    await user.click(screen.getByTestId('epilogue-game-confirm-button'));

    await waitFor(() => {
      expect(onConfirm).toHaveBeenCalledOnce();
      expect(onClose).toHaveBeenCalled();
    });
  });

  it('tells the GM the game stays writable and the reveal is irreversible', () => {
    // The two facts that distinguish epilogue from completing the game. If the
    // copy stops saying them, the GM cannot tell the two options apart.
    render();
    expect(screen.getByText(/cannot be undone/i)).toBeInTheDocument();
    expect(screen.getByText(/writable/i)).toBeInTheDocument();
    expect(screen.getByText(/private messages/i)).toBeInTheDocument();
  });
});

describe('LeaveGameConfirmationDialog', () => {
  it('renders game title and leave button', () => {
    renderWithProviders(
      <LeaveGameConfirmationDialog
        isOpen
        onClose={vi.fn()}
        onConfirm={vi.fn()}
        gameTitle="Dragon Quest"
        isSubmitting={false}
      />
    );
    expect(screen.getByText('Dragon Quest')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /leave game/i })).toBeInTheDocument();
  });

  it('calls onConfirm and closes when confirmed', async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn().mockResolvedValue(undefined);
    const onClose = vi.fn();
    renderWithProviders(
      <LeaveGameConfirmationDialog
        isOpen
        onClose={onClose}
        onConfirm={onConfirm}
        gameTitle="Dragon Quest"
        isSubmitting={false}
      />
    );
    await user.click(screen.getByRole('button', { name: /leave game/i }));
    await waitFor(() => {
      expect(onConfirm).toHaveBeenCalledOnce();
      expect(onClose).toHaveBeenCalled();
    });
  });

  it('disables buttons when isSubmitting is true', () => {
    renderWithProviders(
      <LeaveGameConfirmationDialog
        isOpen
        onClose={vi.fn()}
        onConfirm={vi.fn()}
        gameTitle="Dragon Quest"
        isSubmitting
      />
    );
    expect(screen.getByRole('button', { name: /cancel/i })).toBeDisabled();
    expect(screen.getByRole('button', { name: /leaving/i })).toBeDisabled();
  });
});

describe('WithdrawApplicationConfirmationDialog', () => {
  it('renders game title and role in the description', () => {
    renderWithProviders(
      <WithdrawApplicationConfirmationDialog
        isOpen
        onClose={vi.fn()}
        onConfirm={vi.fn()}
        gameTitle="Starfall"
        isSubmitting={false}
        role="player"
      />
    );
    expect(screen.getByText('Starfall')).toBeInTheDocument();
    expect(screen.getByText(/player application/i)).toBeInTheDocument();
  });

  it('shows player-specific note for player role', () => {
    renderWithProviders(
      <WithdrawApplicationConfirmationDialog
        isOpen
        onClose={vi.fn()}
        onConfirm={vi.fn()}
        gameTitle="Starfall"
        isSubmitting={false}
        role="player"
      />
    );
    expect(screen.getByText(/can only apply again while recruitment is open/i)).toBeInTheDocument();
  });

  it('does not show player-specific note for audience role', () => {
    renderWithProviders(
      <WithdrawApplicationConfirmationDialog
        isOpen
        onClose={vi.fn()}
        onConfirm={vi.fn()}
        gameTitle="Starfall"
        isSubmitting={false}
        role="audience"
      />
    );
    expect(screen.queryByText(/can only apply again while recruitment is open/i)).not.toBeInTheDocument();
  });

  it('calls onConfirm and closes when withdraw confirmed', async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn().mockResolvedValue(undefined);
    const onClose = vi.fn();
    renderWithProviders(
      <WithdrawApplicationConfirmationDialog
        isOpen
        onClose={onClose}
        onConfirm={onConfirm}
        gameTitle="Starfall"
        isSubmitting={false}
        role="audience"
      />
    );
    await user.click(screen.getByRole('button', { name: /withdraw application/i }));
    await waitFor(() => {
      expect(onConfirm).toHaveBeenCalledOnce();
      expect(onClose).toHaveBeenCalled();
    });
  });
});
