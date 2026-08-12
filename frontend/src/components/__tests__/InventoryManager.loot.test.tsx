import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { InventoryManager } from '../InventoryManager';
import { ToastProvider } from '@/contexts/ToastContext';
import { useOptionalGameContext } from '@/contexts/GameContext';
import { apiClient } from '@/lib/api';

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
    onAddRandom,
    allowLootModes,
  }: {
    onAddRandom: (lootTableId: number) => void;
    allowLootModes?: boolean;
  }) => (
    <div data-testid="add-item-modal">
      <span data-testid="loot-modes-allowed">{String(!!allowLootModes)}</span>
      <button data-testid="roll-loot" onClick={() => onAddRandom(7)}>
        Roll
      </button>
    </div>
  ),
}));

vi.mock('../AddCurrencyModal', () => ({ AddCurrencyModal: () => null }));
vi.mock('../ItemCard', () => ({ ItemCard: () => null }));
vi.mock('../CurrencyCard', () => ({ CurrencyCard: () => null }));

const renderInventory = (onItemsChange = vi.fn()) => {
  render(
    <ToastProvider>
      <InventoryManager
        characterId={5}
        items={[]}
        currency={[]}
        canEdit={true}
        onItemsChange={onItemsChange}
        onCurrencyChange={vi.fn()}
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

const openAddItem = async () => {
  screen.getByRole('button', { name: /^add item$/i }).click();
  return screen.findByTestId('add-item-modal');
};

describe('InventoryManager loot rolls', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  // Regression (4b): InventoryManager used useGameContext(), which throws outside
  // a GameProvider, taking down the whole character-sheet editor subtree.
  it('renders outside a GameProvider instead of throwing', () => {
    expect(() => renderWithoutGame()).not.toThrow();
    expect(screen.getByRole('button', { name: /^add item$/i })).toBeInTheDocument();
  });

  it('does not offer loot modes when there is no game context', async () => {
    renderWithoutGame();
    await openAddItem();
    expect(screen.getByTestId('loot-modes-allowed')).toHaveTextContent('false');
  });

  it('offers loot modes when a game context is present', async () => {
    renderWithGame();
    await openAddItem();
    expect(screen.getByTestId('loot-modes-allowed')).toHaveTextContent('true');
  });

  it('adds the rolled item and reports success', async () => {
    vi.mocked(apiClient.games.giveRandomLootTableContent).mockResolvedValue({
      data: { id: 1, name: 'Gold Ring', data: '{"name":"Gold Ring","quantity":1}' },
    } as Awaited<ReturnType<typeof apiClient.games.giveRandomLootTableContent>>);

    const { onItemsChange } = renderWithGame();
    await openAddItem();
    screen.getByTestId('roll-loot').click();

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
      response: { data: { error: 'loot table is empty: add at least one item before rolling' } },
    });

    const { onItemsChange } = renderWithGame();
    await openAddItem();
    screen.getByTestId('roll-loot').click();

    expect(await screen.findByText(/loot table is empty/i)).toBeInTheDocument();
    expect(onItemsChange).not.toHaveBeenCalled();
  });

  it('reports malformed item data instead of throwing', async () => {
    vi.mocked(apiClient.games.giveRandomLootTableContent).mockResolvedValue({
      data: { id: 1, name: 'Broken Item', data: 'not json' },
    } as Awaited<ReturnType<typeof apiClient.games.giveRandomLootTableContent>>);

    const { onItemsChange } = renderWithGame();
    await openAddItem();
    screen.getByTestId('roll-loot').click();

    expect(await screen.findByText(/malformed/i)).toBeInTheDocument();
    expect(onItemsChange).not.toHaveBeenCalled();
  });
});
