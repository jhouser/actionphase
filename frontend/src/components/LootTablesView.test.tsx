import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { LootTablesView } from './LootTablesView';
import type { LootTable } from '@/types/games';

const mockLootTables = vi.fn<() => LootTable[]>(() => []);

vi.mock('@/hooks/useLootTablemanagement', () => ({
  useLootTableManagement: () => ({
    lootTables: mockLootTables(),
    isLoading: false,
    createLootTableMutation: { mutateAsync: vi.fn(), isPending: false },
    updateLootTableMutation: { mutateAsync: vi.fn(), isPending: false },
    updateLootTableContentsMutation: { mutateAsync: vi.fn(), isPending: false },
    deleteLootTableMutation: { mutateAsync: vi.fn(), isPending: false },
  }),
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
