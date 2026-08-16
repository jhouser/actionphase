import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { LootTableSelector } from './LootTableSelector';
import type { LootTableContent } from '@/types/games';

const mockGetLootTableContents = vi.hoisted(() => vi.fn());
vi.mock('@/lib/api', () => ({
  apiClient: {
    games: {
      getLootTableContents: mockGetLootTableContents,
    },
  },
}));

const TABLES = [
  { id: 11, game_id: 7, name: 'Common Loot' },
  { id: 12, game_id: 7, name: 'Rare Loot' },
];

const POTION: LootTableContent = {
  id: 21,
  name: 'Health Potion',
  data: JSON.stringify({ value: 50 }),
} as LootTableContent;

/**
 * Render with a fresh QueryClient per test so one test's cached loot tables
 * cannot satisfy the next test's query and mask a missing fetch.
 */
function renderSelector(props: Partial<React.ComponentProps<typeof LootTableSelector>> = {}) {
  const onLootTableChange = vi.fn();
  const onItemChange = vi.fn();
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });

  const view = render(
    <QueryClientProvider client={queryClient}>
      <LootTableSelector
        gameId={7}
        lootTables={TABLES}
        requireItem={false}
        lootTableId={null}
        onLootTableChange={onLootTableChange}
        onItemChange={onItemChange}
        {...props}
      />
    </QueryClientProvider>
  );

  return { ...view, onLootTableChange, onItemChange, queryClient };
}

const tableSelect = () => screen.getByLabelText(/^loot table$/i) as HTMLSelectElement;
const itemSelect = () => screen.getByLabelText(/loot table content/i) as HTMLSelectElement;

describe('LootTableSelector', () => {
  beforeEach(() => {
    mockGetLootTableContents.mockReset();
    mockGetLootTableContents.mockResolvedValue({ data: [POTION] });
  });

  describe('Table selection', () => {
    it('lists the tables for the given game', async () => {
      renderSelector();

      await waitFor(() => expect(screen.getByRole('option', { name: 'Common Loot' })).toBeInTheDocument());
      expect(screen.getByRole('option', { name: 'Rare Loot' })).toBeInTheDocument();
    });

    it('reports the chosen table id as a number', async () => {
      const user = userEvent.setup();
      const { onLootTableChange } = renderSelector();

      await waitFor(() => expect(screen.getByRole('option', { name: 'Common Loot' })).toBeInTheDocument());
      await user.selectOptions(tableSelect(), '11');

      // Select values are strings; the callback contract is a number.
      expect(onLootTableChange).toHaveBeenCalledWith(11);
    });

    it('reports null when the placeholder option is chosen', async () => {
      const user = userEvent.setup();
      const { onLootTableChange } = renderSelector({ lootTableId: 11 });

      await waitFor(() => expect(screen.getByRole('option', { name: 'Common Loot' })).toBeInTheDocument());
      await user.selectOptions(tableSelect(), '');

      // parseInt('') is NaN, which the `|| null` turns into an explicit null
      // rather than leaking NaN to the caller.
      expect(onLootTableChange).toHaveBeenCalledWith(null);
    });
  });

  describe('requireItem=false (random roll)', () => {
    it('offers no item picker and never fetches contents', async () => {
      renderSelector({ requireItem: false, lootTableId: 11 });

      await waitFor(() => expect(screen.getByRole('option', { name: 'Common Loot' })).toBeInTheDocument());

      // The server picks the item, so fetching the contents would be a pointless
      // GM-only request revealing the whole table.
      expect(screen.queryByLabelText(/loot table content/i)).not.toBeInTheDocument();
      expect(mockGetLootTableContents).not.toHaveBeenCalled();
    });
  });

  describe('requireItem=true (pick an item)', () => {
    it('does not fetch contents until a table is chosen', async () => {
      renderSelector({ requireItem: true, lootTableId: null });

      await waitFor(() => expect(screen.getByRole('option', { name: 'Common Loot' })).toBeInTheDocument());

      expect(mockGetLootTableContents).not.toHaveBeenCalled();
    });

    it('fetches and lists the contents of the chosen table', async () => {
      renderSelector({ requireItem: true, lootTableId: 11 });

      await waitFor(() => expect(screen.getByRole('option', { name: 'Health Potion' })).toBeInTheDocument());
      expect(mockGetLootTableContents).toHaveBeenCalledWith(7, 11);
    });

    it('reports the whole content row, not just its id', async () => {
      const user = userEvent.setup();
      const { onItemChange } = renderSelector({ requireItem: true, lootTableId: 11 });

      await waitFor(() => expect(screen.getByRole('option', { name: 'Health Potion' })).toBeInTheDocument());
      await user.selectOptions(itemSelect(), '21');

      // ItemForm needs `data` and `name` off this object to build the item, so
      // handing back a bare id would break the submit path.
      expect(onItemChange).toHaveBeenCalledWith(POTION);
    });

    it('reports null when the placeholder option is chosen', async () => {
      const user = userEvent.setup();
      const { onItemChange } = renderSelector({ requireItem: true, lootTableId: 11 });

      await waitFor(() => expect(screen.getByRole('option', { name: 'Health Potion' })).toBeInTheDocument());
      await user.selectOptions(itemSelect(), '21');
      onItemChange.mockClear();

      await user.selectOptions(itemSelect(), '');

      expect(onItemChange).toHaveBeenCalledWith(null);
    });

    it('clears the reported item when the table changes', async () => {
      const user = userEvent.setup();
      const { onLootTableChange } = renderSelector({
        requireItem: true,
        lootTableId: 11,
      });

      await waitFor(() => expect(screen.getByRole('option', { name: 'Health Potion' })).toBeInTheDocument());
      await user.selectOptions(itemSelect(), '21');
      expect(itemSelect().value).toBe('21');

      await user.selectOptions(tableSelect(), '12');

      // The previously picked item belongs to the old table. Leaving it selected
      // would submit it against the newly chosen one.
      expect(onLootTableChange).toHaveBeenCalledWith(12);
      expect(itemSelect().value).toBe('');
    });

    it('blanks the item select while the new table has no contents loaded', async () => {
      const user = userEvent.setup();
      renderSelector({ requireItem: true, lootTableId: 11 });

      await waitFor(() => expect(screen.getByRole('option', { name: 'Health Potion' })).toBeInTheDocument());
      await user.selectOptions(itemSelect(), '21');

      // An empty result must not leave the previous selection displayed — the
      // value is forced blank whenever there are no contents to back it.
      mockGetLootTableContents.mockResolvedValue({ data: [] });
      await user.selectOptions(tableSelect(), '12');

      await waitFor(() => expect(itemSelect().value).toBe(''));
    });

    it('renders an empty picker rather than failing when the table has no items', async () => {
      mockGetLootTableContents.mockResolvedValue({ data: [] });
      renderSelector({ requireItem: true, lootTableId: 11 });

      await waitFor(() => expect(screen.getByLabelText(/loot table content/i)).toBeInTheDocument());

      // Only the placeholder — an empty table is a normal GM state, not an error.
      expect(screen.queryByRole('option', { name: 'Health Potion' })).not.toBeInTheDocument();
      expect(itemSelect().value).toBe('');
    });
  });
});
