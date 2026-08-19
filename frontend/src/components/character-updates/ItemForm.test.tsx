import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ItemForm } from './ItemForm';

// Loot modes require a game context; the manual-entry tests below deliberately
// leave this null so they exercise the no-game path.
const mockGameContext = vi.hoisted(() => ({ current: null as { gameId: number } | null }));
vi.mock('@/contexts/GameContext', () => ({
  useOptionalGameContext: () => mockGameContext.current,
}));

const mockGetLootTables = vi.hoisted(() => vi.fn());
const mockGetLootTableContents = vi.hoisted(() => vi.fn());
vi.mock('@/lib/api', () => ({
  apiClient: {
    games: {
      getLootTables: mockGetLootTables,
      getLootTableContents: mockGetLootTableContents,
    },
  },
}));

const mockLoggerError = vi.hoisted(() => vi.fn());
vi.mock('@/services/LoggingService', () => ({
  logger: { error: mockLoggerError, warn: vi.fn(), info: vi.fn(), debug: vi.fn() },
}));

/** Render inside a QueryClientProvider — LootTableSelector fetches via React Query. */
const renderWithQuery = (ui: React.ReactElement) => {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>);
};

describe('ItemForm', () => {
  beforeEach(() => {
    mockGameContext.current = null;
    mockGetLootTables.mockReset();
    mockGetLootTableContents.mockReset();
    mockLoggerError.mockReset();
  });

  describe('Submit guard', () => {
    it('does not call onSubmit when item name is empty', async () => {
      const onSubmit = vi.fn();
      const user = userEvent.setup();
      renderWithQuery(
        <ItemForm
          onSubmit={onSubmit}
          onCancel={vi.fn()}
          submitLabel="Add"
        />
      );

      // Leave name empty, only fill value
      const valueInput = screen.getByLabelText(/^value$/i);
      await user.clear(valueInput);
      await user.type(valueInput, '10');

      await user.click(screen.getByRole('button', { name: /^add$/i }));

      expect(onSubmit).not.toHaveBeenCalled();
    });
  });

  describe('Decimal support', () => {
    it('accepts decimal value for item value field', async () => {
      const onSubmit = vi.fn();
      const user = userEvent.setup();
      renderWithQuery(
        <ItemForm
          onSubmit={onSubmit}
          onCancel={vi.fn()}
          submitLabel="Add"
        />
      );

      // Fill in required name
      await user.type(screen.getByLabelText(/^Name/), 'Gold Ring');

      // Enter a decimal value
      const valueInput = screen.getByLabelText(/^value$/i);
      await user.clear(valueInput);
      await user.type(valueInput, '2.5');

      await user.click(screen.getByRole('button', { name: /^add$/i }));

      expect(onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ value: 2.5 })
      );
    });

    it('accepts decimal value for item weight field', async () => {
      const onSubmit = vi.fn();
      const user = userEvent.setup();
      renderWithQuery(
        <ItemForm
          onSubmit={onSubmit}
          onCancel={vi.fn()}
          submitLabel="Add"
        />
      );

      await user.type(screen.getByLabelText(/^Name/), 'Iron Shield');

      const weightInput = screen.getByLabelText(/^weight$/i);
      await user.clear(weightInput);
      await user.type(weightInput, '3.5');

      await user.click(screen.getByRole('button', { name: /^add$/i }));

      expect(onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ weight: 3.5 })
      );
    });
  });

  describe('Description field uses markdown editor', () => {
    it('renders Write/Preview tabs for description field', () => {
      renderWithQuery(
        <ItemForm
          onSubmit={vi.fn()}
          onCancel={vi.fn()}
          submitLabel="Add"
        />
      );

      // CommentEditor renders Write/Preview tabs — plain Textarea does not
      expect(screen.getByRole('button', { name: /^write$/i })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /^preview$/i })).toBeInTheDocument();
    });

    it('pre-populates description editor with initial value', () => {
      renderWithQuery(
        <ItemForm
          onSubmit={vi.fn()}
          onCancel={vi.fn()}
          submitLabel="Add"
          initialValues={{ description: 'A sharp blade' }}
        />
      );

      expect(screen.getByDisplayValue('A sharp blade')).toBeInTheDocument();
    });
  });

  describe('Loot mode availability', () => {
    it('offers no mode picker by default, even inside a game', () => {
      mockGameContext.current = { gameId: 7 };
      renderWithQuery(
        <ItemForm onSubmit={vi.fn()} onCancel={vi.fn()} submitLabel="Add" />
      );

      // allowLootModes defaults to false: contexts that build plain items (editing
      // an item, authoring a loot table) must not be able to source from a table.
      expect(screen.queryByLabelText(/^mode$/i)).not.toBeInTheDocument();
      expect(screen.getByLabelText(/^Name/)).toBeInTheDocument();
    });

    it('offers no mode picker when allowed but outside a game', async () => {
      mockGameContext.current = null;
      renderWithQuery(
        <ItemForm onSubmit={vi.fn()} onCancel={vi.fn()} submitLabel="Add" allowedLootModes={['manual','loot_table','loot_table_random']} />
      );
      // Loot tables are game-scoped, so the opt-in alone is not enough.
      expect(screen.queryByLabelText(/^mode$/i)).not.toBeInTheDocument();
    });

    it('offers the mode picker when allowed inside a game', async () => {
      mockGameContext.current = { gameId: 7 };
      mockGetLootTables.mockResolvedValue({ data: [] });
      renderWithQuery(
        <ItemForm onSubmit={vi.fn()} onCancel={vi.fn()} submitLabel="Add" allowedLootModes={['manual','loot_table','loot_table_random']} />
      );
      await waitFor(() => expect(screen.getByLabelText(/^mode$/i)).toBeInTheDocument());
    });

    it('submits as a manual item when loot modes are unavailable', async () => {
      const onSubmit = vi.fn();
      const user = userEvent.setup();
      mockGameContext.current = null;
      renderWithQuery(
        <ItemForm onSubmit={onSubmit} onCancel={vi.fn()} submitLabel="Add" allowedLootModes={['manual','loot_table','loot_table_random']} />
      );

      await user.type(screen.getByLabelText(/^Name/), 'Rope');
      await user.click(screen.getByRole('button', { name: /^add$/i }));

      // No lootTableId key at all — AddItemModal branches on its truthiness to
      // decide between onAdd and onAddRandom.
      const payload = onSubmit.mock.calls[0][0];
      expect(payload).toMatchObject({ name: 'Rope', quantity: 1 });
      expect(payload.lootTableId).toBeUndefined();
    });
  });

  describe('Loot table mode (pick a specific item)', () => {
    beforeEach(() => {
      mockGameContext.current = { gameId: 7 };
      mockGetLootTables.mockResolvedValue({
        data: [{ id: 11, game_id: 7, name: 'Common Loot' }],
      });
    });

    it('submits the chosen item using its stored JSON payload', async () => {
      const onSubmit = vi.fn();
      const user = userEvent.setup();
      mockGetLootTableContents.mockResolvedValue({
        data: [
          {
            id: 21,
            name: 'Health Potion',
            data: JSON.stringify({ description: 'Restores 10 HP', category: 'Consumable', value: 50, weight: 0.5 }),
          },
        ],
      });

      renderWithQuery(
        <ItemForm onSubmit={onSubmit} onCancel={vi.fn()} submitLabel="Add" allowedLootModes={['manual','loot_table','loot_table_random']} />
      );
      await waitFor(() => expect(screen.getByLabelText(/^mode$/i)).toBeInTheDocument());

      await user.selectOptions(screen.getByLabelText(/^mode$/i), 'loot_table');
      await waitFor(() => expect(screen.getByRole('option', { name: 'Common Loot' })).toBeInTheDocument());
      await user.selectOptions(screen.getByLabelText(/loot table$/i), '11');
      await waitFor(() => expect(screen.getByRole('option', { name: 'Health Potion' })).toBeInTheDocument());
      await user.selectOptions(screen.getByLabelText(/loot table content/i), '21');

      await user.click(screen.getByRole('button', { name: /^add$/i }));

      // Name comes from the row; the rest is unpacked from the JSON blob.
      expect(onSubmit).toHaveBeenCalledWith({
        name: 'Health Potion',
        description: 'Restores 10 HP',
        quantity: 1,
        category: 'Consumable',
        value: 50,
        weight: 0.5,
      });
    });

    it('does not submit when no item has been chosen', async () => {
      const onSubmit = vi.fn();
      const user = userEvent.setup();
      mockGetLootTableContents.mockResolvedValue({ data: [] });

      renderWithQuery(
        <ItemForm onSubmit={onSubmit} onCancel={vi.fn()} submitLabel="Add" allowedLootModes={['manual','loot_table','loot_table_random']} />
      );
      await waitFor(() => expect(screen.getByLabelText(/^mode$/i)).toBeInTheDocument());

      await user.selectOptions(screen.getByLabelText(/^mode$/i), 'loot_table');
      await user.click(screen.getByRole('button', { name: /^add$/i }));

      expect(onSubmit).not.toHaveBeenCalled();
    });

    it('refuses to submit an item whose stored data is malformed JSON', async () => {
      const onSubmit = vi.fn();
      const user = userEvent.setup();
      mockGetLootTableContents.mockResolvedValue({
        data: [{ id: 21, name: 'Broken Item', data: 'not json{' }],
      });

      renderWithQuery(
        <ItemForm onSubmit={onSubmit} onCancel={vi.fn()} submitLabel="Add" allowedLootModes={['manual','loot_table','loot_table_random']} />
      );
      await waitFor(() => expect(screen.getByLabelText(/^mode$/i)).toBeInTheDocument());

      await user.selectOptions(screen.getByLabelText(/^mode$/i), 'loot_table');
      await waitFor(() => expect(screen.getByRole('option', { name: 'Common Loot' })).toBeInTheDocument());
      await user.selectOptions(screen.getByLabelText(/loot table$/i), '11');
      await waitFor(() => expect(screen.getByRole('option', { name: 'Broken Item' })).toBeInTheDocument());
      await user.selectOptions(screen.getByLabelText(/loot table content/i), '21');

      await user.click(screen.getByRole('button', { name: /^add$/i }));

      // The payload is GM-authored free text, so bad JSON must abort the submit
      // rather than throw and take the form down.
      expect(onSubmit).not.toHaveBeenCalled();
      expect(mockLoggerError).toHaveBeenCalled();
    });

    it('clears the chosen item when the table is changed', async () => {
      const onSubmit = vi.fn();
      const user = userEvent.setup();
      mockGetLootTables.mockResolvedValue({
        data: [
          { id: 11, game_id: 7, name: 'Common Loot' },
          { id: 12, game_id: 7, name: 'Rare Loot' },
        ],
      });
      mockGetLootTableContents.mockResolvedValue({
        data: [{ id: 21, name: 'Health Potion', data: JSON.stringify({ value: 50 }) }],
      });

      renderWithQuery(
        <ItemForm onSubmit={onSubmit} onCancel={vi.fn()} submitLabel="Add" allowedLootModes={['manual','loot_table','loot_table_random']} />
      );
      await waitFor(() => expect(screen.getByLabelText(/^mode$/i)).toBeInTheDocument());

      await user.selectOptions(screen.getByLabelText(/^mode$/i), 'loot_table');
      await waitFor(() => expect(screen.getByRole('option', { name: 'Common Loot' })).toBeInTheDocument());
      await user.selectOptions(screen.getByLabelText(/loot table$/i), '11');
      await waitFor(() => expect(screen.getByRole('option', { name: 'Health Potion' })).toBeInTheDocument());
      await user.selectOptions(screen.getByLabelText(/loot table content/i), '21');

      // Switching tables must drop the previous selection in ItemForm's own state,
      // otherwise the stale item from the old table is submitted against the newly
      // chosen one. Asserting via the item dropdown alone is not enough: the child
      // blanks its own select independently, which masks a missing reset here.
      mockGetLootTableContents.mockResolvedValue({
        data: [{ id: 31, name: 'Dragon Scale', data: JSON.stringify({ value: 900 }) }],
      });
      await user.selectOptions(screen.getByLabelText(/loot table$/i), '12');
      await waitFor(() => expect(screen.getByRole('option', { name: 'Dragon Scale' })).toBeInTheDocument());

      // The item select is blank again, so nothing is selected in the new table.
      expect((screen.getByLabelText(/loot table content/i) as HTMLSelectElement).value).toBe('');

      await user.click(screen.getByRole('button', { name: /^add$/i }));

      // Submit aborts rather than falling back to 'Health Potion' from the old table.
      expect(onSubmit).not.toHaveBeenCalled();
    });
  });

  describe('Loot table random mode', () => {
    beforeEach(() => {
      mockGameContext.current = { gameId: 7 };
      mockGetLootTables.mockResolvedValue({
        data: [{ id: 11, game_id: 7, name: 'Common Loot' }],
      });
    });

    it('submits only the table id, deferring the roll to the caller', async () => {
      const onSubmit = vi.fn();
      const user = userEvent.setup();

      renderWithQuery(
        <ItemForm onSubmit={onSubmit} onCancel={vi.fn()} submitLabel="Add" allowedLootModes={['manual','loot_table','loot_table_random']} />
      );
      await waitFor(() => expect(screen.getByLabelText(/^mode$/i)).toBeInTheDocument());

      await user.selectOptions(screen.getByLabelText(/^mode$/i), 'loot_table_random');
      await waitFor(() => expect(screen.getByRole('option', { name: 'Common Loot' })).toBeInTheDocument());
      await user.selectOptions(screen.getByLabelText(/loot table$/i), '11');

      await user.click(screen.getByRole('button', { name: /^add$/i }));

      expect(onSubmit).toHaveBeenCalledWith({ name: '', quantity: 1, lootTableId: 11 });
    });

    it('does not offer an item picker — the item is chosen server-side', async () => {
      const user = userEvent.setup();
      renderWithQuery(
        <ItemForm onSubmit={vi.fn()} onCancel={vi.fn()} submitLabel="Add" allowedLootModes={['manual','loot_table','loot_table_random']} />
      );
      await waitFor(() => expect(screen.getByLabelText(/^mode$/i)).toBeInTheDocument());

      await user.selectOptions(screen.getByLabelText(/^mode$/i), 'loot_table_random');
      await waitFor(() => expect(screen.getByLabelText(/loot table$/i)).toBeInTheDocument());

      expect(screen.queryByLabelText(/loot table content/i)).not.toBeInTheDocument();
      expect(mockGetLootTableContents).not.toHaveBeenCalled();
    });

    it('does not submit when no table has been chosen', async () => {
      const onSubmit = vi.fn();
      const user = userEvent.setup();

      renderWithQuery(
        <ItemForm onSubmit={onSubmit} onCancel={vi.fn()} submitLabel="Add" allowedLootModes={['manual','loot_table','loot_table_random']} />
      );
      await waitFor(() => expect(screen.getByLabelText(/^mode$/i)).toBeInTheDocument());

      await user.selectOptions(screen.getByLabelText(/^mode$/i), 'loot_table_random');
      await user.click(screen.getByRole('button', { name: /^add$/i }));

      expect(onSubmit).not.toHaveBeenCalled();
    });
  });

  describe('Stale mode after loot modes disappear', () => {
    it('submits as manual when the loot modes are withdrawn mid-edit', async () => {
      const onSubmit = vi.fn();
      const user = userEvent.setup();
      mockGameContext.current = { gameId: 7 };
      mockGetLootTables.mockResolvedValue({
        data: [{ id: 11, game_id: 7, name: 'Common Loot' }],
      });

      const { rerender } = renderWithQuery(
        <ItemForm onSubmit={onSubmit} onCancel={vi.fn()} submitLabel="Add" allowedLootModes={['manual','loot_table','loot_table_random']} />
      );
      await waitFor(() => expect(screen.getByLabelText(/^mode$/i)).toBeInTheDocument());

      // Pick a loot mode, then lose the opt-in. `mode` state survives the change,
      // so without the effectiveMode guard the form would submit an empty-named
      // loot payload through the manual UI.
      await user.selectOptions(screen.getByLabelText(/^mode$/i), 'loot_table_random');

      rerender(
        <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
          <ItemForm onSubmit={onSubmit} onCancel={vi.fn()} submitLabel="Add" allowedLootModes={['manual']} />
        </QueryClientProvider>
      );

      await user.click(screen.getByRole('button', { name: /^add$/i }));

      expect(onSubmit).not.toHaveBeenCalled();
    });
  });
});
