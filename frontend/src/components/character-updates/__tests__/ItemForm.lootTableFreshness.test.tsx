import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ItemForm } from '../ItemForm';

vi.mock('@/contexts/GameContext', () => ({
  useOptionalGameContext: () => ({ gameId: 3 }),
}));

const getLootTables = vi.fn();
vi.mock('@/lib/api', () => ({
  apiClient: {
    games: {
      getLootTables: (...args: unknown[]) => getLootTables(...args),
      getLootTableContents: vi.fn(() => Promise.resolve({ data: [] })),
    },
  },
}));

/**
 * Regression cover for a loot table created in the game view not showing up on
 * the character sheet until a manual page reload.
 *
 * Two independent causes, both needed to reproduce it:
 *   1. This query inherited the app-wide 5-minute staleTime, so a list fetched
 *      before the table existed was replayed from cache on mount. The mutation
 *      that creates a table cannot help via invalidation — this form is
 *      unmounted at the time, so there is no active observer to refetch.
 *   2. Mode availability was state narrowed by an effect that could only clear a
 *      mode, so the empty cached list latched the loot modes off and no later
 *      data could turn them back on.
 */
describe('ItemForm loot table freshness', () => {
  beforeEach(() => vi.clearAllMocks());

  const renderWithCache = (cached: unknown[] | undefined) => {
    // Mirrors the real client's defaults (see App.tsx) — the default staleTime
    // is the whole point of the first bug, so a test client without it would
    // pass regardless of the fix.
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false, staleTime: 5 * 60 * 1000 } },
    });
    if (cached !== undefined) {
      client.setQueryData(['lootTables', 3, true], cached);
    }
    return render(
      <QueryClientProvider client={client}>
        <ItemForm
          onSubmit={vi.fn()}
          onCancel={vi.fn()}
          allowedLootModes={['manual', 'loot_table_random']}
        />
      </QueryClientProvider>,
    );
  };

  it('offers a loot table created after an empty list was cached', async () => {
    // The GM opened an item form before creating any table...
    // ...then created one, which this form's cache entry knows nothing about.
    getLootTables.mockResolvedValue({ data: [{ id: 11, name: 'Trinkets' }] });

    renderWithCache([]);

    expect(
      await screen.findByRole('option', { name: /loot table \(random\)/i }),
    ).toBeInTheDocument();
    // Served from cache instead of refetched is the failure mode, so assert the
    // network was actually consulted.
    expect(getLootTables).toHaveBeenCalledWith(3, true);
  });

  it('hides the loot modes when the game genuinely has no loot tables', async () => {
    getLootTables.mockResolvedValue({ data: [] });

    renderWithCache(undefined);

    await waitFor(() => expect(getLootTables).toHaveBeenCalled());
    // Only manual remains, so the Mode selector has nothing to choose between
    // and is not rendered at all.
    await waitFor(() =>
      expect(screen.queryByLabelText(/^mode$/i)).not.toBeInTheDocument(),
    );
    expect(screen.getByLabelText(/name/i)).toBeInTheDocument();
  });
});
