import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, fireEvent, act } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { LootTableForm } from './LootTableForm';

// The form only needs the game id from context; the real provider pulls in the
// whole game fetch chain.
vi.mock('@/contexts/GameContext', () => ({
  useOptionalGameContext: () => ({ gameId: 1 }),
}));

vi.mock('@/lib/api', () => ({
  apiClient: {
    games: {
      getLootTableContents: vi.fn().mockResolvedValue({ data: [] }),
    },
  },
}));

const renderForm = (props: Partial<React.ComponentProps<typeof LootTableForm>> = {}) => {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const onSubmit = vi.fn();
  render(
    <QueryClientProvider client={queryClient}>
      <LootTableForm
        onClose={vi.fn()}
        onSubmit={onSubmit}
        isSubmitting={false}
        {...props}
      />
    </QueryClientProvider>
  );
  return { onSubmit };
};

const submitButton = () => screen.getByRole('button', { name: /create loot table/i });

describe('LootTableForm validation', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('blocks submission when the table has no name', () => {
    const { onSubmit } = renderForm();

    expect(screen.getByText(/give the loot table a name/i)).toBeInTheDocument();
    expect(submitButton()).toBeDisabled();

    fireEvent.click(submitButton());
    expect(onSubmit).not.toHaveBeenCalled();
  });

  // Empty tables are allowed by design: GMs create a table before deciding its
  // contents, and importing a CSV into a saved table is a normal flow. Rolling
  // on an empty table is defended separately (API 400 -> error toast), so the
  // form must not block authoring one.
  it('allows submission when the table is named but has no items', () => {
    const { onSubmit } = renderForm();

    fireEvent.change(screen.getByLabelText(/table name/i), {
      target: { value: 'Treasure Chest' },
    });

    expect(
      screen.queryByText(/an empty loot table cannot be rolled on/i)
    ).not.toBeInTheDocument();
    expect(submitButton()).toBeEnabled();

    fireEvent.click(submitButton());
    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({ name: 'Treasure Chest' })
    );
  });

  it('enables submission once the table has a name and an item', async () => {
    const { onSubmit } = renderForm();

    fireEvent.change(screen.getByLabelText(/table name/i), {
      target: { value: 'Treasure Chest' },
    });

    fireEvent.click(screen.getByRole('button', { name: /add loot table content/i }));

    fireEvent.change(await screen.findByLabelText(/^Name/), {
      target: { value: 'Gold Coins' },
    });
    fireEvent.click(screen.getByRole('button', { name: /^add$/i }));

    expect(submitButton()).toBeEnabled();

    fireEvent.click(submitButton());
    expect(onSubmit).toHaveBeenCalledTimes(1);
    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({
        name: 'Treasure Chest',
        items: expect.arrayContaining([
          expect.objectContaining({ name: 'Gold Coins' }),
        ]),
      })
    );
  });
});

describe('LootTableForm CSV import', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  // jsdom's FileReader does not read real File contents, so drive onload directly
  // with the text the browser would have produced.
  const importCsv = async (csv: string) => {
    const input = document.getElementById('import-loot-table') as HTMLInputElement;
    const file = new File([csv], 'loot.csv', { type: 'text/csv' });
    let onload: ((e: ProgressEvent<FileReader>) => void) | null = null;
    vi.spyOn(FileReader.prototype, 'readAsText').mockImplementation(function (this: FileReader) {
      onload = this.onload as never;
    });
    fireEvent.change(input, { target: { files: [file] } });
    await act(async () => {
      onload?.({ target: { result: csv } } as ProgressEvent<FileReader>);
    });
  };

  const nameTable = () =>
    fireEvent.change(screen.getByLabelText(/table name/i), { target: { value: 'Imported' } });

  it('explains the CSV format, naming the comma delimiter', () => {
    renderForm();
    // The delimiter is the one rule that cannot be guessed and fails silently,
    // so it must be stated rather than implied.
    const help = screen.getByRole('tooltip');
    expect(help).toHaveTextContent(/Comma-separated/i);
    expect(help).toHaveTextContent(/must include "name"/i);
    expect(help).toHaveTextContent(/replaces all current items/i);
  });

  // Regression: a trailing newline is present in essentially every editor-saved
  // file, and parsed as a final row of empty strings. That produced a phantom
  // nameless item, which the backend validator then rejected — failing the whole
  // import over a row the GM cannot see.
  it('ignores the trailing newline instead of importing a phantom empty item', async () => {
    const { onSubmit } = renderForm();
    nameTable();

    await importCsv('name,quantity\nIron Sword,1\nHealth Potion,3\n');

    expect(screen.queryByText(/import failed/i)).not.toBeInTheDocument();
    fireEvent.click(submitButton());

    const submitted = onSubmit.mock.calls[0][0];
    expect(submitted.items.map((i: { name: string }) => i.name)).toEqual([
      'Iron Sword',
      'Health Potion',
    ]);
  });

  // Regression: papaparse does not error on a wrong delimiter — the entire line
  // becomes a single column — so a CSV with another separator silently replaced the table with
  // items whose name was undefined.
  it('rejects an incorrectly-delimited file rather than importing nameless items', async () => {
    const { onSubmit } = renderForm();
    nameTable();

    await importCsv('name;quantity\nIron Sword;1\n');

    expect(screen.getByText(/needs a "name" column/i)).toBeInTheDocument();
    // The existing item list must be left alone on a failed import. Assert on
    // the list itself rather than the submit button — an empty table is now a
    // legitimate thing to save, so the button stays enabled either way. Each
    // item renders a "Remove <name>" control, so none means nothing imported.
    expect(screen.queryByRole('button', { name: /^remove /i })).not.toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it('rejects a file whose rows have a blank name', async () => {
    renderForm();
    nameTable();

    await importCsv('name,quantity\nIron Sword,1\n,5\n');

    expect(screen.getByText(/row 2 has no name/i)).toBeInTheDocument();
  });

  // `equipped` is written as a hardcoded false by AddItemModal and has no control
  // anywhere in the inventory UI. It must not round-trip through CSV: values parse
  // as strings, so an exported `false` would come back as the truthy string
  // "false" and light up ItemCard's equipped badge.
  it('drops the equipped field from imported items', async () => {
    const { onSubmit } = renderForm();
    nameTable();

    await importCsv('name,quantity,equipped\nIron Sword,1,false\n');

    fireEvent.click(submitButton());
    const submitted = onSubmit.mock.calls[0][0];
    const data = JSON.parse(submitted.items[0].data);
    expect(data).not.toHaveProperty('equipped');
    expect(data.name).toBe('Iron Sword');
  });

  // Regression: an unquoted value containing the delimiter (a description like
  // "Sharp, very sharp") splits into an extra field. Papaparse keeps the first
  // part and stashes the rest in __parsed_extra, so the row imported with the
  // description silently truncated to "Sharp".
  it('rejects a row whose unquoted value contains the delimiter', async () => {
    const { onSubmit } = renderForm();
    nameTable();

    await importCsv('name,description\nSword,Sharp, very sharp\n');

    expect(screen.getByText(/more values than there are columns/i)).toBeInTheDocument();
    // The truncated text must never reach the table.
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it('keeps a quoted description containing the delimiter intact', async () => {
    const { onSubmit } = renderForm();
    nameTable();

    await importCsv('name,description\nSword,"Sharp, very sharp"\n');

    expect(screen.queryByText(/import failed/i)).not.toBeInTheDocument();
    fireEvent.click(submitButton());

    const data = JSON.parse(onSubmit.mock.calls[0][0].items[0].data);
    // Quotes are CSV syntax, not content — they must not survive into the value.
    expect(data.description).toBe('Sharp, very sharp');
  });

  it('preserves Markdown in descriptions without adding quotes', async () => {
    const { onSubmit } = renderForm();
    nameTable();

    await importCsv('name,description\nSword,**Cursed** blade with _drain_\n');

    fireEvent.click(submitButton());
    const data = JSON.parse(onSubmit.mock.calls[0][0].items[0].data);
    expect(data.description).toBe('**Cursed** blade with _drain_');
  });

  it('clears a previous import error after a good file', async () => {
    renderForm();
    nameTable();

    await importCsv('name;quantity\nIron Sword;1\n');
    expect(screen.getByText(/needs a "name" column/i)).toBeInTheDocument();

    await importCsv('name,quantity\nIron Sword,1\n');
    expect(screen.queryByText(/import failed/i)).not.toBeInTheDocument();
  });
});

describe('LootTableForm CSV export', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  // Captures the CSV text handed to the Blob, since export builds a Blob URL.
  const captureExport = async (item: { name: string; description?: string }) => {
    vi.stubGlobal('URL', { ...URL, createObjectURL: vi.fn(() => 'blob:mock'), revokeObjectURL: vi.fn() });
    let captured = '';
    vi.stubGlobal('Blob', vi.fn(function (parts: string[]) {
      captured = parts.join('');
      return {} as Blob;
    }));
    // Exporting clicks a detached anchor; stub it so jsdom does not navigate.
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});

    renderForm();
    fireEvent.change(screen.getByLabelText(/table name/i), { target: { value: 'Chest' } });
    fireEvent.click(screen.getByRole('button', { name: /add loot table content/i }));
    fireEvent.change(await screen.findByLabelText(/^Name/), { target: { value: item.name } });
    if (item.description !== undefined) {
      // Target the textarea by id: /description/i also matches CommentEditor's
      // preview toggle, so getByLabelText is ambiguous here.
      const descriptionField = document.getElementById('item-description')!;
      fireEvent.change(descriptionField, { target: { value: item.description } });
    }
    fireEvent.click(screen.getByRole('button', { name: /^add$/i }));
    fireEvent.click(screen.getByRole('button', { name: /export loot table as csv/i }));

    return () => captured;
  };

  // AddItemModal stamps `equipped: false` onto every item it creates, so without
  // filtering it surfaces as a column in the exported CSV — a field the GM has no
  // way to set and should not be editing by hand.
  it('omits the equipped column when exporting items added through the form', async () => {
    const captured = await captureExport({ name: 'Iron Sword' });

    expect(captured()).not.toMatch(/equipped/i);
    expect(captured()).toMatch(/Iron Sword/);

    vi.unstubAllGlobals();
  });

  // Papaparse already quotes exactly when quoting is required, so forcing quotes
  // on every field would only add noise to a file GMs hand-edit. These two cases
  // are the evidence for leaving the default alone.
  it('quotes a description containing the delimiter but leaves plain text bare', async () => {
    const captured = await captureExport({ name: 'Sword', description: 'Sharp, very sharp' });

    expect(captured()).toContain('"Sharp, very sharp"');

    vi.unstubAllGlobals();
  });

  it('does not quote a plain Markdown description', async () => {
    const captured = await captureExport({ name: 'Sword', description: '**Cursed** blade' });

    expect(captured()).toContain('**Cursed** blade');
    expect(captured()).not.toContain('"**Cursed** blade"');

    vi.unstubAllGlobals();
  });
});

describe('LootTableForm item removal', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  const addItem = async (name: string) => {
    fireEvent.click(screen.getByRole('button', { name: /add loot table content/i }));
    fireEvent.change(await screen.findByLabelText(/^Name/), { target: { value: name } });
    fireEvent.click(screen.getByRole('button', { name: /^add$/i }));
  };

  // Regression: removal read `formData.items` from the render closure instead of
  // the updater's `p`, so two removals dispatched before a re-render both filtered
  // the *original* list — the second undid the first. Identity vs index is not the
  // issue (each item is a distinct object); the stale closure is.
  it('applies both removals when two are dispatched in one batch', async () => {
    const { onSubmit } = renderForm();

    fireEvent.change(screen.getByLabelText(/table name/i), {
      target: { value: 'Batch' },
    });

    await addItem('Potion');
    await addItem('Elixir');
    await addItem('Tonic');

    const removeButtons = screen.getAllByRole('button', { name: /^remove /i });
    // Both clicks inside one act() batch: with a stale closure the second
    // overwrites the first and only one item ends up removed.
    await act(async () => {
      fireEvent.click(removeButtons[2]);
      fireEvent.click(removeButtons[1]);
    });

    fireEvent.click(submitButton());
    expect(onSubmit).toHaveBeenCalledTimes(1);
    const submitted = onSubmit.mock.calls[0][0];
    expect(submitted.items.map((i: { name: string }) => i.name)).toEqual(['Potion']);
  });
});
