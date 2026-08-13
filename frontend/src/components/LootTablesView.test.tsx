import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { LootTablesView } from './LootTablesView';
import type { LootTable } from '@/types/games';

const mockLootTables = vi.fn<() => LootTable[]>(() => []);
const mockIsLoading = vi.fn<() => boolean>(() => false);

// Hoisted so tests can assert which mutation the view routed a submit to.
// Recreating these inside the factory would hand every render a fresh spy,
// leaving nothing to assert against.
const createMutate = vi.hoisted(() => vi.fn());
const updateMutate = vi.hoisted(() => vi.fn());
const updateContentsMutate = vi.hoisted(() => vi.fn());
const deleteMutate = vi.hoisted(() => vi.fn());

vi.mock('@/hooks/useLootTablemanagement', () => ({
  useLootTableManagement: () => ({
    lootTables: mockLootTables(),
    isLoading: mockIsLoading(),
    createLootTableMutation: { mutateAsync: createMutate, isPending: false },
    updateLootTableMutation: { mutateAsync: updateMutate, isPending: false },
    updateLootTableContentsMutation: { mutateAsync: updateContentsMutate, isPending: false },
    deleteLootTableMutation: { mutateAsync: deleteMutate, isPending: false },
  }),
}));

// The real form is exercised by LootTableForm.test.tsx. Here it stands in as a
// harness so these tests can drive onSubmit with an exact payload and assert on
// how LootTablesView routes it.
vi.mock('./loot-tables/LootTableForm', () => ({
  LootTableForm: ({
    onSubmit,
    onClose,
    lootTable,
  }: {
    onSubmit: (data: {
      id?: number;
      name: string;
      items?: { id: number; name: string; data: string }[];
      itemsChanged?: boolean;
    }) => void;
    onClose: () => void;
    lootTable?: LootTable;
  }) => (
    <div data-testid="loot-table-form">
      <span data-testid="form-mode">{lootTable ? `edit:${lootTable.id}` : 'create'}</span>
      <button
        data-testid="submit-create"
        onClick={() => onSubmit({ name: 'Brand New Table', items: [] })}
      >
        create
      </button>
      <button
        data-testid="submit-rename"
        onClick={() => onSubmit({ id: lootTable?.id, name: 'Renamed Table' })}
      >
        rename
      </button>
      <button
        data-testid="submit-items-only"
        onClick={() =>
          onSubmit({
            id: lootTable?.id,
            name: lootTable?.name ?? '',
            items: [{ id: 0, name: 'Potion', data: '{}' }],
            itemsChanged: true,
          })
        }
      >
        save items
      </button>
      <button data-testid="form-close" onClick={onClose}>
        close
      </button>
    </div>
  ),
}));

const renderView = () =>
  render(
    <MemoryRouter>
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <LootTablesView gameId={1} />
      </QueryClientProvider>
    </MemoryRouter>
  );

const table = (overrides: Partial<LootTable> = {}): LootTable => ({
  id: 1,
  game_id: 1,
  name: 'Normal Items',
  created_at: '2026-08-01T10:00:00Z',
  updated_at: '2026-08-01T10:00:00Z',
  ...overrides,
});

describe('LootTablesView cards', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockLootTables.mockReturnValue([]);
  });

  // The cards previously used surface-base — the same background as the container
  // behind them — so only a shadow separated them and the list read as one block.
  // surface-raised + border is the site-wide pattern (see GameStatsView).
  it('renders each card on a raised surface with a border so rows are distinct', () => {
    mockLootTables.mockReturnValue([table(), table({ id: 2, name: 'Rare Items' })]);
    renderView();

    const card = screen.getByRole('heading', { name: 'Normal Items' }).closest('div')!.parentElement!;
    expect(card.className).toContain('surface-raised');
    expect(card.className).toContain('border-theme-default');
    expect(card.className).not.toContain('surface-base');
  });

  it('shows the created date on each card', () => {
    mockLootTables.mockReturnValue([table()]);
    renderView();

    expect(screen.getByText(/Created Aug 1, 2026/)).toBeInTheDocument();
  });

  // updated_at is backfilled from created_at and defaults to NOW() on insert, so
  // an untouched table would otherwise display the same date twice.
  it('omits the updated date when the table has never been edited', () => {
    mockLootTables.mockReturnValue([table()]);
    renderView();

    expect(screen.getByText(/Created Aug 1, 2026/)).toBeInTheDocument();
    expect(screen.queryByText(/Updated/)).not.toBeInTheDocument();
  });

  it('shows the updated date once the table has been edited', () => {
    mockLootTables.mockReturnValue([
      table({ updated_at: '2026-08-05T12:00:00Z' }),
    ]);
    renderView();

    expect(screen.getByText(/Created Aug 1, 2026/)).toBeInTheDocument();
    expect(screen.getByText(/Updated Aug 5, 2026/)).toBeInTheDocument();
  });

  // The insert sets both columns from the same NOW(), but they are not always
  // byte-identical, so a string compare would report spurious edits.
  it('treats a sub-second difference between the timestamps as never edited', () => {
    mockLootTables.mockReturnValue([
      table({ created_at: '2026-08-01T10:00:00.000000Z', updated_at: '2026-08-01T10:00:00.123456Z' }),
    ]);
    renderView();

    expect(screen.queryByText(/Updated/)).not.toBeInTheDocument();
  });

  // Every row otherwise announces a bare "Edit"/"Delete", which is ambiguous.
  it('names the table in each row action label', () => {
    mockLootTables.mockReturnValue([table({ name: 'Rare Items' })]);
    renderView();

    expect(screen.getByRole('button', { name: 'Edit Rare Items' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Delete Rare Items' })).toBeInTheDocument();
  });

  it('renders one card per loot table', () => {
    mockLootTables.mockReturnValue([
      table(),
      table({ id: 2, name: 'Rare Items' }),
      table({ id: 3, name: 'Boss Drops' }),
    ]);
    renderView();

    const headings = screen.getAllByRole('heading', { level: 3 });
    expect(headings.map((h) => h.textContent)).toEqual([
      'Normal Items',
      'Rare Items',
      'Boss Drops',
    ]);
  });

  it('shows the empty state when there are no loot tables', () => {
    renderView();

    expect(screen.getByText(/no loot tables created yet/i)).toBeInTheDocument();
    expect(screen.queryByRole('heading', { level: 3 })).not.toBeInTheDocument();
  });
});

describe('LootTablesView timestamp formatting', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('keeps the created and updated dates on the same line as one summary', () => {
    mockLootTables.mockReturnValue([table({ updated_at: '2026-08-05T12:00:00Z' })]);
    renderView();

    const summary = screen.getByText(/Created Aug 1, 2026/);
    expect(within(summary).getByText(/Updated Aug 5, 2026/)).toBeInTheDocument();
  });
});


describe('LootTablesView loading state', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockIsLoading.mockReturnValue(true);
  });

  it('shows a skeleton instead of an empty state while tables load', () => {
    mockLootTables.mockReturnValue([]);
    renderView();

    // Without this the GM briefly sees "No loot tables created yet" on every
    // visit, which reads as data loss rather than loading.
    expect(screen.queryByText(/no loot tables created yet/i)).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /new loot table/i })).not.toBeInTheDocument();
  });
});

describe('LootTablesView deletion', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockIsLoading.mockReturnValue(false);
    mockLootTables.mockReturnValue([table(), table({ id: 2, name: 'Rare Items' })]);
  });

  it('asks for confirmation before deleting rather than deleting on first click', async () => {
    const user = userEvent.setup();
    renderView();

    await user.click(screen.getByRole('button', { name: 'Delete Normal Items' }));

    expect(screen.getByText(/delete loot table\?/i)).toBeInTheDocument();
    // Deleting a table destroys all of its contents, so the first click must
    // only open the confirmation.
    expect(deleteMutate).not.toHaveBeenCalled();
  });

  it('deletes the chosen table once confirmed', async () => {
    const user = userEvent.setup();
    renderView();

    await user.click(screen.getByRole('button', { name: 'Delete Rare Items' }));
    await user.click(screen.getByRole('button', { name: /^delete loot table$/i }));

    // Must delete the row that was clicked, not the first in the list.
    await waitFor(() => expect(deleteMutate).toHaveBeenCalledWith(2));
  });

  it('cancelling the dialog leaves the table alone', async () => {
    const user = userEvent.setup();
    renderView();

    await user.click(screen.getByRole('button', { name: 'Delete Normal Items' }));
    await user.click(screen.getByRole('button', { name: /^cancel$/i }));

    expect(screen.queryByText(/delete loot table\?/i)).not.toBeInTheDocument();
    expect(deleteMutate).not.toHaveBeenCalled();
  });

  it('does not delete a stale table if the dialog is confirmed after cancelling', async () => {
    const user = userEvent.setup();
    renderView();

    await user.click(screen.getByRole('button', { name: 'Delete Normal Items' }));
    await user.click(screen.getByRole('button', { name: /^cancel$/i }));
    await user.click(screen.getByRole('button', { name: 'Delete Rare Items' }));
    await user.click(screen.getByRole('button', { name: /^delete loot table$/i }));

    // The pending target is cleared on cancel, so the second confirmation must
    // act on the second table only.
    await waitFor(() => expect(deleteMutate).toHaveBeenCalledWith(2));
    expect(deleteMutate).toHaveBeenCalledTimes(1);
  });
});

describe('LootTablesView create and edit routing', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockIsLoading.mockReturnValue(false);
    mockLootTables.mockReturnValue([table()]);
  });

  it('opens a blank form from the New Loot Table button', async () => {
    const user = userEvent.setup();
    renderView();

    await user.click(screen.getByRole('button', { name: /new loot table/i }));

    expect(screen.getByTestId('form-mode')).toHaveTextContent('create');
  });

  it('opens the clicked table in the form when editing', async () => {
    const user = userEvent.setup();
    mockLootTables.mockReturnValue([table(), table({ id: 2, name: 'Rare Items' })]);
    renderView();

    await user.click(screen.getByRole('button', { name: 'Edit Rare Items' }));

    expect(screen.getByTestId('form-mode')).toHaveTextContent('edit:2');
  });

  it('returns to the list without saving when the form is closed', async () => {
    const user = userEvent.setup();
    renderView();

    await user.click(screen.getByRole('button', { name: /new loot table/i }));
    await user.click(screen.getByTestId('form-close'));

    expect(screen.getByRole('heading', { name: /loot table management/i })).toBeInTheDocument();
    expect(createMutate).not.toHaveBeenCalled();
  });

  it('creates a table when the submitted data has no id', async () => {
    const user = userEvent.setup();
    renderView();

    await user.click(screen.getByRole('button', { name: /new loot table/i }));
    await user.click(screen.getByTestId('submit-create'));

    await waitFor(() =>
      expect(createMutate).toHaveBeenCalledWith({ name: 'Brand New Table', items: [] })
    );
    expect(updateMutate).not.toHaveBeenCalled();
  });

  it('returns to the list after a successful create', async () => {
    const user = userEvent.setup();
    renderView();

    await user.click(screen.getByRole('button', { name: /new loot table/i }));
    await user.click(screen.getByTestId('submit-create'));

    await waitFor(() =>
      expect(screen.getByRole('heading', { name: /loot table management/i })).toBeInTheDocument()
    );
  });

  it('renames an existing table without rewriting its contents', async () => {
    const user = userEvent.setup();
    renderView();

    await user.click(screen.getByRole('button', { name: 'Edit Normal Items' }));
    await user.click(screen.getByTestId('submit-rename'));

    await waitFor(() => expect(updateMutate).toHaveBeenCalledWith({ id: 1, name: 'Renamed Table' }));
    // itemsChanged was false, so the contents rewrite — which deletes every
    // existing row — must not run.
    expect(updateContentsMutate).not.toHaveBeenCalled();
    expect(createMutate).not.toHaveBeenCalled();
  });

  it('rewrites contents without renaming when only the items changed', async () => {
    const user = userEvent.setup();
    renderView();

    await user.click(screen.getByRole('button', { name: 'Edit Normal Items' }));
    await user.click(screen.getByTestId('submit-items-only'));

    await waitFor(() =>
      expect(updateContentsMutate).toHaveBeenCalledWith({
        id: 1,
        items: [{ id: 0, name: 'Potion', data: '{}' }],
      })
    );
    // The name was unchanged, so no pointless rename request.
    expect(updateMutate).not.toHaveBeenCalled();
  });
});
