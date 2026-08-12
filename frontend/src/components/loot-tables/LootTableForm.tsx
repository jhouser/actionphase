import { useEffect, useState, type ChangeEvent } from 'react';
import { Alert, Button, HelpTooltip, Input } from '../ui';
import type { LootTable, LootTableContent } from '@/types/games';
import { AddItemModal } from '../AddItemModal';
import type { InventoryItem } from '@/types/characters';
import { useQuery } from '@tanstack/react-query';
import { apiClient } from '@/lib/api';
import { useOptionalGameContext } from '@/contexts/GameContext';
import { DownloadIcon, TrashIcon, UploadIcon } from 'lucide-react';
import Papa from 'papaparse'

export interface EditLootTable {
  id?: number;
  name: string;
  items?: LootTableContent[];
  itemsChanged: boolean;
}

const CSVSeparatorCharacter = ';';

/**
 * Item fields the CSV deliberately does not round-trip.
 *
 * `equipped` is written as a hardcoded `false` by AddItemModal and rendered as a
 * badge by ItemCard, but nothing can set it — there is no control for it anywhere
 * in the inventory UI. Exposing it through CSV would make the importer the only
 * way to equip an item, and it round-trips wrongly besides: CSV values parse as
 * strings, so an exported `false` returns as the truthy string "false" and the
 * badge lights up. Drop it in both directions until the field has real UI.
 */
const CSV_EXCLUDED_FIELDS = new Set(['equipped']);

const stripExcludedFields = (row: Record<string, unknown>): Record<string, unknown> =>
  Object.fromEntries(Object.entries(row).filter(([key]) => !CSV_EXCLUDED_FIELDS.has(key)));

/**
 * Import/export help. Leads with the delimiter because it is the one rule that is
 * impossible to guess and fails silently in most spreadsheet exports, which
 * default to commas. Export-then-edit is offered first as the reliable path: it
 * hands the GM a correctly shaped file instead of asking them to build one.
 */
const CSV_FORMAT_HELP =
  `Semicolon-separated (${CSVSeparatorCharacter}), not commas. The first row must be ` +
  `column headers and must include "name"; each row after it is one item. ` +
  `Optional columns: description, quantity, category, value, weight. ` +
  `Descriptions support Markdown; wrap any value containing "${CSVSeparatorCharacter}", ` +
  `a line break, or a double quote in double quotes. ` +
  `Importing replaces all current items. Easiest route: add one item, Export, ` +
  `then edit that file and re-import it.`;

interface LootTableFormProps {
  onClose: () => void;
  onSubmit: (data: EditLootTable) => void;
  isSubmitting: boolean;
  lootTable?: LootTable;
}

export function LootTableForm({ onClose, onSubmit, isSubmitting, lootTable }: LootTableFormProps) {
  const gameContext = useOptionalGameContext();

  const { data: lootTableContents } = useQuery({
    queryKey: ['lootTableContents', lootTable?.id],
    queryFn: () => apiClient.games.getLootTableContents(gameContext!.gameId, lootTable?.id ?? 0).then(res => res.data),
    enabled: !!lootTable?.id
  });

  useEffect(() => {
    setFormData(p => ({
      ...p,
      items: lootTableContents || undefined,
      itemsChanged: false
    }));
  }, [lootTableContents]);

  const [formData, setFormData] = useState<EditLootTable>({
    id: lootTable?.id,
    name: lootTable?.name || '',
    items:  undefined,
    itemsChanged: false
  });
  const [isAddingContent, setIsAddingContent] = useState(false);
  const [importError, setImportError] = useState<string | null>(null);

  // A table with no name is unidentifiable in the picker, so that stays blocked.
  //
  // Empty tables are allowed on purpose: GMs build a table before they have
  // decided its contents, and importing a CSV into a saved table is a normal
  // flow. Rolling on an empty table is already handled in depth — the API
  // returns 400 and InventoryManager surfaces that as an error toast — so
  // blocking creation here only got in the way of authoring.
  const validationError = !formData.name.trim() ? 'Give the loot table a name.' : null;

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (validationError) return;
    onSubmit(formData);
  };

  
  const addItem = (itemData: Omit<InventoryItem, 'id'>) => {
    const newContent : LootTableContent = {
      id: 0,
      name: itemData.name,
      data: JSON.stringify(itemData),
    }
    setFormData(p => ({...p, items: [...(p.items || []), newContent], itemsChanged: true}));
    setIsAddingContent(false);
  };

  /**
   * Parse an uploaded CSV into loot table contents.
   *
   * Returns either the parsed items or a human-readable error. Both failure modes
   * here are silent without this: papaparse does not error on a wrong delimiter
   * (the whole line becomes one column) or a missing `name` header, so the import
   * would replace the table with rows whose name is undefined.
   */
  const processCSVFile = (csvText: string): { items: LootTableContent[] } | { error: string } => {
    const parsed = Papa.parse<Record<string, string>>(csvText, {
      delimiter: CSVSeparatorCharacter,
      header: true,
      // A trailing newline is normal in any editor-saved file, and without this it
      // parses as a final row of empty strings — a phantom nameless item that the
      // server then rejects, failing the whole import.
      skipEmptyLines: true,
    });

    if (!parsed.meta.fields?.includes('name')) {
      return {
        error: `The CSV needs a "name" column. Columns must be separated by "${CSVSeparatorCharacter}", not commas.`,
      };
    }

    // A row with more fields than headers means an unquoted value contained the
    // delimiter — overwhelmingly a description like "Sharp; very sharp". Papaparse
    // keeps the first part and stashes the rest in __parsed_extra, so accepting
    // this row would silently truncate the GM's text. Name the row and the fix.
    const raggedRow = parsed.errors.find((e) => e.code === 'TooManyFields');
    if (raggedRow) {
      const rowLabel = typeof raggedRow.row === 'number' ? `Row ${raggedRow.row + 1}` : 'A row';
      return {
        error:
          `${rowLabel} has more values than there are columns. A value containing "${CSVSeparatorCharacter}" ` +
          `must be wrapped in double quotes — for example: "Sharp${CSVSeparatorCharacter} very sharp".`,
      };
    }

    // Guard rows that are blank or name-less rather than sending them to a server
    // that will reject the batch and name a row number the GM cannot see.
    const items = parsed.data
      .filter((row) => Object.values(row).some((v) => v?.trim()))
      // Strip on the way in too: a hand-authored CSV could otherwise set a field
      // the UI has no control for.
      .map((row) => ({ id: 0, name: (row['name'] ?? '').trim(), data: JSON.stringify(stripExcludedFields(row)) }));

    const nameless = items.findIndex((i) => !i.name);
    if (nameless !== -1) {
      return { error: `Row ${nameless + 1} has no name. Every item needs a value in the "name" column.` };
    }
    if (items.length === 0) {
      return { error: 'That file has no item rows.' };
    }
    return { items };
  }

  const createCSVString = (contents: LootTableContent[]): string => {
    // Items are GM-authored JSON and can be malformed or have differing keys, so
    // parse defensively and union the columns. Passing ragged objects straight to
    // unparse emits a trailing all-empty row, which reimporting then reads back as
    // a junk item — the export/import round trip has to be lossless.
    const rows = contents.flatMap((i) => {
      try {
        const parsed = JSON.parse(i.data);
        return parsed && typeof parsed === 'object'
          ? [stripExcludedFields(parsed as Record<string, unknown>)]
          : [{ name: i.name }];
      } catch {
        return [{ name: i.name }];
      }
    });
    const columns = Array.from(new Set(rows.flatMap((r) => Object.keys(r))));
    if (columns.length === 0) return '';
    return Papa.unparse(rows, { delimiter: CSVSeparatorCharacter, columns });
  }

  const importLootTable = (event: ChangeEvent<HTMLInputElement>): void => {
    if (!event.target.files?.length) {
      return;
    }
    const reader = new FileReader();
    reader.onload = (e) => {
      if (!e.target?.result) {
        setImportError('That file could not be read.');
        return;
      }
      const result = processCSVFile(e.target.result as string);
      if ('error' in result) {
        setImportError(result.error);
        return;
      }
      setImportError(null);
      setFormData(p => ({
        ...p,
        items: result.items,
        itemsChanged: true
      }));
    }
    reader.onerror = () => setImportError('That file could not be read.');
    reader.readAsText(event.target.files[0]);
    // Reset so picking the same file again after fixing it re-triggers onChange.
    event.target.value = '';
  }


  const exportLootTable = (_: React.MouseEvent<HTMLButtonElement>): void => {
    // btoa throws on any character outside Latin-1, which item names and
    // descriptions routinely contain (accents, em dashes, curly quotes). A Blob
    // URL carries UTF-8 directly and needs no base64 step.
    const url = URL.createObjectURL(
      new Blob([createCSVString(formData.items || [])], { type: 'text/csv;charset=utf-8;' })
    );
    const el = document.createElement('a');
    el.setAttribute('href', url);
    el.setAttribute('download', `${formData.name.toLowerCase().replaceAll(' ', '_') || 'loot_table'}.csv`);
    el.style.display = 'none';

    document.body.appendChild(el);
    el.click();
    document.body.removeChild(el);
    URL.revokeObjectURL(url);
  };

  const hasItems = (formData.items?.length ?? 0) > 0;


  return (
    <div>
      <form onSubmit={handleSubmit}>
        <div className="flex flex-wrap items-center justify-end gap-x-3 gap-y-2 mt-6">
          {/* Sits directly beside Import/Export rather than across the row from
              them, so the label and tooltip read as describing those two buttons. */}
          <span className="flex items-center gap-1 text-sm text-content-secondary">
            Bulk edit with CSV
            {/* Right-anchored: the icon now sits near the modal's right edge, where
                the default left anchoring overflows it. */}
            <HelpTooltip text={CSV_FORMAT_HELP} align="right" />
          </span>

          {/* Labelled, not icon-only: a bare up-arrow gives no hint that this
              screen supports CSV at all, which is how GMs missed the feature. */}
          <label
            className={`inline-flex h-9 items-center gap-2 px-3 rounded-md text-sm text-content-secondary transition-colors ${
              isSubmitting
                ? 'opacity-50 cursor-not-allowed'
                : 'cursor-pointer hover:text-content-primary hover:bg-interactive-primary-subtle'
            }`}
            htmlFor='import-loot-table'>
            <UploadIcon className="h-5 w-5" />
            Import
            <input disabled={isSubmitting} type="file" id="import-loot-table" accept=".csv,text/csv" onChange={importLootTable} className="hidden" />
          </label>
          <button
            type="button"
            // Exporting an empty table produces a file with no rows, which then
            // fails to reimport — nothing useful to hand the GM.
            disabled={isSubmitting || !hasItems}
            title={hasItems ? undefined : 'Add at least one item to export'}
            aria-label="Export loot table as CSV"
            onClick={exportLootTable}
            className="inline-flex h-9 items-center gap-2 px-3 rounded-md text-sm text-content-secondary hover:text-content-primary hover:bg-interactive-primary-subtle transition-colors disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:bg-transparent"
          >
            <DownloadIcon className="h-5 w-5" />
            Export
          </button>
        </div>

        {importError && (
          <Alert variant="danger" className="mt-3" title="Import failed">
            {importError}
          </Alert>
        )}
        <div className="space-y-4">

            <div>
              <Input
                id="loot-table-name"
                label="Table Name"
                type="text"
                value={formData.name || ''}
                onChange={(e) => setFormData(prev => ({
                  ...prev,
                  name: e.target.value
                }))}
                placeholder="e.g., 'Normal Items'"
                helperText="Give this loot table a custom name"
              />
            </div>
            <div >
              {formData.items && formData.items.length > 0 
                ? (formData.items.map((item, index) => (
                  <div className="md:flex items-center" key={index}>
                    <div className="mb-1">
                      <button
                        type="button"
                        disabled={isSubmitting}
                        aria-label={`Remove ${item.name}`}
                        // Filter positionally off the updater's `p`, not the `formData`
                        // captured at render. The previous version closed over the render
                        // snapshot, so two removals before the next render both filtered
                        // the same stale array and the second undid the first.
                        onClick={_ => setFormData(p => ({...p, items: p.items?.filter((_unused, i) => i !== index), itemsChanged: true }))}
                        className="inline-flex h-9 w-9 items-center justify-center rounded-md text-content-secondary hover:text-content-primary hover:bg-interactive-primary-subtle transition-colors"
                      >
                        <TrashIcon  className="h-5 w-5" />
                      </button>
                    </div>
                    <div className="block text-sm font-medium text-content-primary mb-2">{index + 1} - {item.name}</div>
                  </div>))) 
                : (<div></div>)}
            </div>

              <div>
                <Button
                  variant="primary"
                  type="button"
                  disabled={isSubmitting}
                  onClick={() => setIsAddingContent(true)}
                >
                  Add Loot Table Content
                </Button>
            </div>
            

        </div>


        {validationError && (
          <p className="mt-4 text-sm text-content-secondary" role="status">
            {validationError}
          </p>
        )}

        <div className="flex justify-end space-x-3 mt-6">
          <Button
            type="button"
            variant="ghost"
            onClick={onClose}
          >
            Cancel
          </Button>
          <Button
            type="submit"
            variant="primary"
            disabled={isSubmitting || validationError !== null}
            data-faro-user-action-name="create-loot-table"
          >
            {isSubmitting 
              ? lootTable ? 'Updating...' : 'Creating...' 
              : lootTable ? 'Update Loot Table' : 'Create Loot Table'}
          </Button>
        </div>
      </form>

      {/*
        Add Loot Table Content Modal. allowLootModes is intentionally left off:
        this modal defines the contents of a loot table, so sourcing an item
        *from* a loot table makes no sense here, and onAddRandom is unreachable.
      */}
      {isAddingContent && (
        <AddItemModal
          onAdd={addItem}
          onAddRandom={() => {}}
          onCancel={() => {setIsAddingContent(false)}}
        />
      )}
    </div>
  );
}
