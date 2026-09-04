import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ItemsManager } from '../ItemsManager';
import { ToastProvider } from '@/contexts/ToastContext';
import { useOptionalGameContext } from '@/contexts/GameContext';
import { apiClient } from '@/lib/api';
import { lootModes } from '../character-updates/ItemForm';

// GameContext itself is not exported, so drive the optional hook directly: null
// stands in for "rendered outside a GameProvider".
vi.mock('@/contexts/GameContext', () => ({
  useOptionalGameContext: vi.fn(),
}));

vi.mock('@/services/LoggingService', () => ({
  logger: { warn: vi.fn(), error: vi.fn(), debug: vi.fn(), info: vi.fn() },
}));

vi.mock('@/lib/api', () => ({
  apiClient: {
    games: {
      giveRandomLootTableContent: vi.fn(),
    },
  },
}));

// Stand in for the real modal so these tests can drive the loot callbacks
// directly. `allowLootModes` is surfaced as text so we can assert on it.
vi.mock('../AddItemModal', () => ({
  AddItemModal: ({
    onAdd,
    onAddRandom,
    allowedLootModes,
  }: {
    onAdd: (item: { name: string; quantity: number }) => void;
    onAddRandom: (lootTableId: number) => void;
    allowedLootModes?: lootModes[];
  }) => (
    <div data-testid="add-item-modal">
      <span data-testid="loot-modes-allowed">{allowedLootModes?.join(',')}</span>
      <button data-testid="roll-loot" onClick={() => onAddRandom(7)}>
        Roll
      </button>
      <button data-testid="add-manual" onClick={() => onAdd({ name: 'Manual Item', quantity: 2 })}>
        Add
      </button>
    </div>
  ),
}));

vi.mock('../ItemCard', () => ({ ItemCard: () => null }));

const renderInventory = (onItemsChange = vi.fn()) => {
  render(
    <ToastProvider>
      <ItemsManager
        characterId={5}
        items={[]}
        canEdit={true}
        label="Inventory"
        onItemsChange={onItemsChange}
      />
    </ToastProvider>
  );
  return { onItemsChange };
};

const renderWithGame = (onItemsChange = vi.fn()) => {
  vi.mocked(useOptionalGameContext).mockReturnValue({
    gameId: 42,
  } as ReturnType<typeof useOptionalGameContext>);
  return renderInventory(onItemsChange);
};

const renderWithoutGame = () => {
  vi.mocked(useOptionalGameContext).mockReturnValue(
    null as ReturnType<typeof useOptionalGameContext>
  );
  return renderInventory();
};

// userEvent rather than a raw .click(): a bare DOM click dispatches outside
// React's act() scope, so the resulting state update warns and — worse — may be
// asserted on before it flushes.
const openAddItem = async () => {
  await userEvent.click(screen.getByTestId('add-item'));
  return screen.findByTestId('add-item-modal');
};

const clickTestId = (testId: string) => userEvent.click(screen.getByTestId(testId));

describe('ItemsManager loot rolls', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  // Regression (4b): the former InventoryManager used useGameContext(), which throws outside
  // a GameProvider, taking down the whole character-sheet editor subtree.
  it('renders outside a GameProvider instead of throwing', () => {
    expect(() => renderWithoutGame()).not.toThrow();
    expect(screen.getByTestId('add-item')).toBeInTheDocument();
  });

  it('does not offer loot modes when there is no game context', async () => {
    renderWithoutGame();
    await openAddItem();
    expect(screen.getByTestId('loot-modes-allowed')).toHaveTextContent('manual');
  });

  it('offers loot modes when a game context is present', async () => {
    renderWithGame();
    await openAddItem();
    expect(screen.getByTestId('loot-modes-allowed')).toHaveTextContent('manual,loot_table,loot_table_random');
  });

  it('adds the rolled item and reports success', async () => {
    vi.mocked(apiClient.games.giveRandomLootTableContent).mockResolvedValue({
      data: { id: 1, name: 'Gold Ring', data: '{"name":"Gold Ring","quantity":1}' },
    } as Awaited<ReturnType<typeof apiClient.games.giveRandomLootTableContent>>);

    const { onItemsChange } = renderWithGame();
    await openAddItem();
    await clickTestId('roll-loot');

    await waitFor(() => expect(onItemsChange).toHaveBeenCalledTimes(1));
    const [items, reloadOnly] = onItemsChange.mock.calls[0];
    expect(items).toHaveLength(1);
    expect(items[0]).toMatchObject({ name: 'Gold Ring', quantity: 1 });
    // reloadOnly=true: the server already persisted this, so the caller should
    // refetch rather than write the item back.
    expect(reloadOnly).toBe(true);
    expect(await screen.findByText(/added item gold ring/i)).toBeInTheDocument();
  });

  // Regression (#3): the request had no .catch(), so a failed roll — e.g. the 400
  // returned for an empty loot table — left the modal open with no feedback.
  it('surfaces the server error message when the roll fails', async () => {
    vi.mocked(apiClient.games.giveRandomLootTableContent).mockRejectedValue({
      response: { data: { detail: 'loot table is empty: add at least one item before rolling' } },
    });

    const { onItemsChange } = renderWithGame();
    await openAddItem();
    await clickTestId('roll-loot');

    expect(await screen.findByText(/loot table is empty/i)).toBeInTheDocument();
    expect(onItemsChange).not.toHaveBeenCalled();
  });

  it('reports malformed item data instead of throwing', async () => {
    vi.mocked(apiClient.games.giveRandomLootTableContent).mockResolvedValue({
      data: { id: 1, name: 'Broken Item', data: 'not json' },
    } as Awaited<ReturnType<typeof apiClient.games.giveRandomLootTableContent>>);

    const { onItemsChange } = renderWithGame();
    await openAddItem();
    await clickTestId('roll-loot');

    expect(await screen.findByText(/malformed/i)).toBeInTheDocument();
    expect(onItemsChange).not.toHaveBeenCalled();
  });

  it('falls back to a generic message when the error carries no server text', async () => {
    vi.mocked(apiClient.games.giveRandomLootTableContent).mockRejectedValue(
      new Error('Network down')
    );

    renderWithGame();
    await openAddItem();
    await clickTestId('roll-loot');

    // A network failure has no response body to quote, so the user still needs
    // something actionable rather than a blank toast.
    expect(await screen.findByText(/failed to roll/i)).toBeInTheDocument();
  });

  it('keeps the modal open when a roll fails so the GM can retry', async () => {
    vi.mocked(apiClient.games.giveRandomLootTableContent).mockRejectedValue({
      response: { data: { detail: 'loot table is empty' } },
    });

    renderWithGame();
    await openAddItem();
    await clickTestId('roll-loot');

    await screen.findByText(/loot table is empty/i);
    // Only a successful roll dismisses the modal; closing it on failure would
    // discard the GM's table choice.
    expect(screen.getByTestId('add-item-modal')).toBeInTheDocument();
  });

  it('closes the modal after a successful roll', async () => {
    vi.mocked(apiClient.games.giveRandomLootTableContent).mockResolvedValue({
      data: { id: 1, name: 'Gold Ring', data: '{"name":"Gold Ring","quantity":1}' },
    } as Awaited<ReturnType<typeof apiClient.games.giveRandomLootTableContent>>);

    renderWithGame();
    await openAddItem();
    await clickTestId('roll-loot');

    await waitFor(() =>
      expect(screen.queryByTestId('add-item-modal')).not.toBeInTheDocument()
    );
  });

  it('adds a manual item locally without reloading', async () => {
    const { onItemsChange } = renderWithGame();
    await openAddItem();
    await clickTestId('add-manual');

    await waitFor(() => expect(onItemsChange).toHaveBeenCalledTimes(1));
    const [items, reloadOnly] = onItemsChange.mock.calls[0];
    expect(items[0]).toMatchObject({ name: 'Manual Item', quantity: 2 });
    expect(items[0].id).toEqual(expect.any(String));
    // Unlike a roll, nothing was persisted server-side, so the caller keeps its
    // own state rather than refetching.
    expect(reloadOnly).toBe(false);
    expect(screen.queryByTestId('add-item-modal')).not.toBeInTheDocument();
  });

  it('hides the add control from viewers who cannot edit', () => {
    vi.mocked(useOptionalGameContext).mockReturnValue({
      gameId: 42,
    } as ReturnType<typeof useOptionalGameContext>);
    render(
      <ToastProvider>
        <ItemsManager
          characterId={5}
          items={[]}
          canEdit={false}
          label="Inventory"
          onItemsChange={vi.fn()}
        />
      </ToastProvider>
    );

    expect(screen.queryByTestId('add-item')).not.toBeInTheDocument();
  });
});
