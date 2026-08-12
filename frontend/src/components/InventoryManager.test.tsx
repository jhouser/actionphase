import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { InventoryManager } from './InventoryManager';
import type { InventoryItem, CurrencyEntry } from '../types/characters';

// Loot rolls need a game; leaving this null exercises the standalone
// character-sheet path where the modes must be hidden.
const mockGameContext = vi.hoisted(() => ({ current: null as { gameId: number } | null }));
vi.mock('@/contexts/GameContext', () => ({
  useOptionalGameContext: () => mockGameContext.current,
}));

const mockGiveRandomLoot = vi.hoisted(() => vi.fn());
vi.mock('@/lib/api', () => ({
  apiClient: { games: { giveRandomLootTableContent: mockGiveRandomLoot } },
}));

const mockShowSuccess = vi.hoisted(() => vi.fn());
const mockShowError = vi.hoisted(() => vi.fn());
vi.mock('@/contexts/ToastContext', () => ({
  useToast: () => ({ showSuccess: mockShowSuccess, showError: mockShowError }),
}));

const mockLoggerError = vi.hoisted(() => vi.fn());
const mockLoggerWarn = vi.hoisted(() => vi.fn());
vi.mock('@/services/LoggingService', () => ({
  logger: { error: mockLoggerError, warn: mockLoggerWarn, info: vi.fn(), debug: vi.fn() },
}));

/**
 * AddItemModal is replaced with a harness that exposes its two callbacks as
 * buttons. The real modal's behaviour is covered by ItemForm/LootTableSelector
 * tests; what matters here is which callback InventoryManager wires up and what
 * it does with the result.
 */
vi.mock('./AddItemModal', () => ({
  AddItemModal: ({
    onAdd,
    onAddRandom,
    allowLootModes,
  }: {
    onAdd: (item: Omit<InventoryItem, 'id'>) => void;
    onAddRandom: (lootTableId: number) => void;
    allowLootModes?: boolean;
  }) => (
    <div data-testid="add-item-modal" data-allow-loot-modes={String(!!allowLootModes)}>
      <button onClick={() => onAdd({ name: 'Manual Item', quantity: 2 })}>harness-add</button>
      <button onClick={() => onAddRandom(11)}>harness-roll</button>
    </div>
  ),
}));

const baseItems: InventoryItem[] = [
  { id: 'item-1', name: 'Torch', quantity: 3, weight: 1, value: 2 },
];
const baseCurrency: CurrencyEntry[] = [];

function renderManager(overrides: Partial<React.ComponentProps<typeof InventoryManager>> = {}) {
  const onItemsChange = vi.fn();
  const onCurrencyChange = vi.fn();
  const view = render(
    <InventoryManager
      characterId={42}
      items={baseItems}
      currency={baseCurrency}
      canEdit
      onItemsChange={onItemsChange}
      onCurrencyChange={onCurrencyChange}
      {...overrides}
    />
  );
  return { ...view, onItemsChange, onCurrencyChange };
}

const openAddItem = async (user: ReturnType<typeof userEvent.setup>) => {
  await user.click(screen.getByRole('button', { name: /^add item$/i }));
  return screen.getByTestId('add-item-modal');
};

describe('InventoryManager', () => {
  beforeEach(() => {
    mockGameContext.current = { gameId: 7 };
    mockGiveRandomLoot.mockReset();
    mockShowSuccess.mockReset();
    mockShowError.mockReset();
    mockLoggerError.mockReset();
    mockLoggerWarn.mockReset();
  });

  describe('Loot mode availability', () => {
    it('offers loot modes when rendered inside a game', async () => {
      const user = userEvent.setup();
      renderManager();

      const modal = await openAddItem(user);

      expect(modal).toHaveAttribute('data-allow-loot-modes', 'true');
    });

    it('withholds loot modes outside a game rather than crashing', async () => {
      const user = userEvent.setup();
      mockGameContext.current = null;
      renderManager();

      // useGameContext() throws outside a provider, which previously took the
      // whole character sheet down; the optional variant must degrade instead.
      const modal = await openAddItem(user);

      expect(modal).toHaveAttribute('data-allow-loot-modes', 'false');
      expect(screen.getByText('Torch')).toBeInTheDocument();
    });
  });

  describe('Adding a manual item', () => {
    it('appends the item with a generated id and closes the modal', async () => {
      const user = userEvent.setup();
      const { onItemsChange } = renderManager();

      await openAddItem(user);
      await user.click(screen.getByRole('button', { name: 'harness-add' }));

      expect(onItemsChange).toHaveBeenCalledTimes(1);
      const [items, reloadOnly] = onItemsChange.mock.calls[0];
      expect(items).toHaveLength(2);
      expect(items[1]).toMatchObject({ name: 'Manual Item', quantity: 2 });
      expect(items[1].id).toEqual(expect.any(String));
      // A local edit, so the caller keeps its own state rather than refetching.
      expect(reloadOnly).toBe(false);
      expect(screen.queryByTestId('add-item-modal')).not.toBeInTheDocument();
    });
  });

  describe('Rolling for a random item', () => {
    it('rolls against the current game and adds the returned item', async () => {
      const user = userEvent.setup();
      mockGiveRandomLoot.mockResolvedValue({
        data: { name: 'Health Potion', data: JSON.stringify({ quantity: 1, value: 50 }) },
      });
      const { onItemsChange } = renderManager();

      await openAddItem(user);
      await user.click(screen.getByRole('button', { name: 'harness-roll' }));

      await waitFor(() => expect(onItemsChange).toHaveBeenCalled());
      expect(mockGiveRandomLoot).toHaveBeenCalledWith(7, 11, 42);

      const [items, reloadOnly] = onItemsChange.mock.calls[0];
      expect(items).toHaveLength(2);
      expect(items[1]).toMatchObject({ value: 50 });
      // The server already persisted this item, so the caller must reload rather
      // than push its local copy back and risk clobbering it.
      expect(reloadOnly).toBe(true);
      expect(mockShowSuccess).toHaveBeenCalledWith(expect.stringContaining('Health Potion'));
      // A successful roll dismisses the modal; only failures keep it open.
      await waitFor(() => expect(screen.queryByTestId('add-item-modal')).not.toBeInTheDocument());
    });

    it('reports malformed item data instead of adding a broken item', async () => {
      const user = userEvent.setup();
      mockGiveRandomLoot.mockResolvedValue({
        data: { name: 'Broken Relic', data: 'not json{' },
      });
      const { onItemsChange } = renderManager();

      await openAddItem(user);
      await user.click(screen.getByRole('button', { name: 'harness-roll' }));

      // GM-authored free text, so bad JSON must surface as a toast rather than
      // throwing inside the promise chain.
      await waitFor(() => expect(mockShowError).toHaveBeenCalled());
      expect(mockShowError.mock.calls[0][0]).toContain('Broken Relic');
      expect(onItemsChange).not.toHaveBeenCalled();
      expect(mockShowSuccess).not.toHaveBeenCalled();
      expect(mockLoggerError).toHaveBeenCalled();
    });

    it('surfaces the server error message when the roll fails', async () => {
      const user = userEvent.setup();
      mockGiveRandomLoot.mockRejectedValue({
        response: { data: { error: 'Loot table is empty' } },
      });
      const { onItemsChange } = renderManager();

      await openAddItem(user);
      await user.click(screen.getByRole('button', { name: 'harness-roll' }));

      // Rolling an empty table returns 400; without this the modal sat open with
      // no feedback at all.
      await waitFor(() => expect(mockShowError).toHaveBeenCalledWith('Loot table is empty'));
      expect(onItemsChange).not.toHaveBeenCalled();
    });

    it('falls back to a generic message when the error has no server text', async () => {
      const user = userEvent.setup();
      mockGiveRandomLoot.mockRejectedValue(new Error('Network down'));
      renderManager();

      await openAddItem(user);
      await user.click(screen.getByRole('button', { name: 'harness-roll' }));

      await waitFor(() => expect(mockShowError).toHaveBeenCalled());
      expect(mockShowError.mock.calls[0][0]).toMatch(/failed to roll/i);
    });

    it('keeps the modal open when the roll fails so the GM can retry', async () => {
      const user = userEvent.setup();
      mockGiveRandomLoot.mockRejectedValue({ response: { data: { error: 'nope' } } });
      renderManager();

      await openAddItem(user);
      await user.click(screen.getByRole('button', { name: 'harness-roll' }));

      await waitFor(() => expect(mockShowError).toHaveBeenCalled());
      expect(screen.getByTestId('add-item-modal')).toBeInTheDocument();
    });
  });

  describe('Corrupted data defence', () => {
    it('generates ids for items that arrive without one', async () => {
      const user = userEvent.setup();
      const { onItemsChange } = renderManager({
        items: [{ name: 'Legacy Item', quantity: 1 } as InventoryItem],
      });

      expect(mockLoggerWarn).toHaveBeenCalled();
      expect(screen.getByText('Legacy Item')).toBeInTheDocument();

      // Draft-merge bugs have produced id-less rows. The backfill is only
      // observable downstream: the id must survive into the list handed back to
      // the caller, otherwise React keys collide and edits target the wrong row.
      await openAddItem(user);
      await user.click(screen.getByRole('button', { name: 'harness-add' }));

      const [items] = onItemsChange.mock.calls[0];
      expect(items[0]).toMatchObject({ name: 'Legacy Item' });
      expect(items[0].id).toEqual(expect.any(String));
      expect(items[0].id).not.toBe('');
    });
  });

  describe('Read-only mode', () => {
    it('hides the add control when the viewer cannot edit', () => {
      renderManager({ canEdit: false });

      expect(screen.queryByRole('button', { name: /^add item$/i })).not.toBeInTheDocument();
      expect(screen.getByText('Torch')).toBeInTheDocument();
    });
  });
});
