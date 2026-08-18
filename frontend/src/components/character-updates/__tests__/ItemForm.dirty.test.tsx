import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ItemForm } from '../ItemForm';

vi.mock('@/contexts/GameContext', () => ({
  useOptionalGameContext: () => ({ gameId: 3 }),
}));

vi.mock('@/lib/api', () => ({
  apiClient: {
    games: {
      getLootTables: vi.fn(() =>
        Promise.resolve({ data: [{ id: 11, name: 'Trinkets' }] }),
      ),
      getLootTableContents: vi.fn(() => Promise.resolve({ data: [] })),
    },
  },
}));

const renderForm = (props: Partial<React.ComponentProps<typeof ItemForm>> = {}) => {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <ItemForm
        onSubmit={vi.fn()}
        onCancel={vi.fn()}
        allowedLootModes={['manual', 'loot_table_random']}
        {...props}
      />
    </QueryClientProvider>,
  );
};

describe('ItemForm unsaved-edit reporting', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('reports clean on an untouched form', () => {
    const onDirtyChange = vi.fn();
    renderForm({ onDirtyChange });

    expect(onDirtyChange).toHaveBeenLastCalledWith(false);
  });

  it('reports dirty once a manual field is typed into', async () => {
    const user = userEvent.setup();
    const onDirtyChange = vi.fn();
    renderForm({ onDirtyChange });

    await user.type(await screen.findByLabelText(/item name/i), 'Rope');

    expect(onDirtyChange).toHaveBeenLastCalledWith(true);
  });

  /**
   * handleSubmit trims before submitting, so a whitespace-only change saves to an
   * identical value. Counting it dirty meant the form could never converge: the tab
   * lock stayed on with no edit the user could save or cancel to clear it.
   */
  it('ignores a whitespace-only change', async () => {
    const user = userEvent.setup();
    const onDirtyChange = vi.fn();
    renderForm({ onDirtyChange, initialValues: { name: 'Rope', quantity: 1 } });

    await user.type(await screen.findByLabelText(/item name/i), '   ');

    expect(onDirtyChange).toHaveBeenLastCalledWith(false);
  });

  /**
   * `||` treated a legitimately-zero quantity as absent and defaulted to 1, so the form
   * opened already reporting dirty with nothing typed — locking both tabs immediately.
   */
  it('opens clean for an item saved with quantity 0', async () => {
    const onDirtyChange = vi.fn();
    renderForm({ onDirtyChange, initialValues: { name: 'Rope', quantity: 0 } });

    expect(onDirtyChange).toHaveBeenLastCalledWith(false);
  });

  /**
   * A loot table picked while browsing and then abandoned by switching back to manual
   * is not pending work — handleSubmit ignores that state entirely in manual mode.
   * Reporting dirty here would warn on every close for an edit the GM never made.
   */
  it('reports clean after browsing a loot table and returning to manual', async () => {
    const user = userEvent.setup();
    const onDirtyChange = vi.fn();
    renderForm({ onDirtyChange });

    const modeSelect = await screen.findByLabelText(/^mode$/i);
    await waitFor(() =>
      expect(screen.getByRole('option', { name: /loot table \(random\)/i })).toBeInTheDocument(),
    );

    await user.selectOptions(modeSelect, 'loot_table_random');
    const tableSelect = await screen.findByLabelText(/^loot table$/i);
    await user.selectOptions(tableSelect, '11');
    expect(onDirtyChange).toHaveBeenLastCalledWith(true);

    await user.selectOptions(modeSelect, 'manual');

    expect(onDirtyChange).toHaveBeenLastCalledWith(false);
  });
});
