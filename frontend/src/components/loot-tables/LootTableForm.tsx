import { useEffect, useState, type ChangeEvent } from 'react';
import { Button, Input } from '../ui';
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

  // A table with no name is unidentifiable in the picker, and one with no items
  // cannot be rolled on (the API rejects it), so block both here with an
  // explanation rather than letting the GM save something unusable.
  const validationError = !formData.name.trim()
    ? 'Give the loot table a name.'
    : (formData.items?.length ?? 0) === 0
      ? 'Add at least one item — an empty loot table cannot be rolled on.'
      : null;

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

  const processCSVFile = (csvText: string): LootTableContent[] => {
    const result : LootTableContent[] = Papa.parse(csvText, {
      delimiter: CSVSeparatorCharacter,
      header: true
    }).data.map(i => ({id: 0, name: (i as InventoryItem)['name'], data: JSON.stringify(i) }));
    return result;
  }

  const createCSVString = (contents: LootTableContent[]): string => {
    return Papa.unparse(contents.map(i => JSON.parse(i.data)), { delimiter: CSVSeparatorCharacter });
  }

  const importLootTable = (event: ChangeEvent<HTMLInputElement>): void => {
    if (!event.target.files) {
      return;
    }
    const reader = new FileReader();
    reader.onload = (e) => {
      if (e.target?.result) {
        const csvContents = processCSVFile(e.target.result as string)
        if (csvContents.length > 0) {
          setFormData(p => ({
            ...p,
            items: csvContents,
            itemsChanged: true
          }));
        }
      }
    }
    reader.readAsText(event.target.files[0]); 
  }


  const exportLootTable = (_: React.MouseEvent<HTMLButtonElement>): void => {
    const el = document.createElement('a');
    el.setAttribute('href', `data:application/octet-stream;charset=utf-8;base64,${btoa(createCSVString(formData.items || []))}`);
    el.setAttribute('download', `${formData.name.toLowerCase().replaceAll(' ', '_') || 'loot_table'}.csv`);
    el.style.display = 'none';

    document.body.appendChild(el);
    el.click();
    document.body.removeChild(el);
  };
  

  return (
    <div>
      <form onSubmit={handleSubmit}>
        <div className="flex justify-end space-x-3 mt-6">
          <label 
            className="inline-flex h-9 w-9 items-center justify-center rounded-md text-content-secondary hover:text-content-primary hover:bg-interactive-primary-subtle transition-colors"
            htmlFor='import-loot-table'>
            <UploadIcon  className="h-5 w-5" />
            <input disabled={isSubmitting} type="file" id="import-loot-table" accept=".csv" onChange={importLootTable} className="hidden" />
          </label>
          <button
            type="button"
            disabled={isSubmitting}
            aria-label="Export Loot Table"
            onClick={exportLootTable}
            className="inline-flex h-9 w-9 items-center justify-center rounded-md text-content-secondary hover:text-content-primary hover:bg-interactive-primary-subtle transition-colors"
          >
            <DownloadIcon  className="h-5 w-5" />
          </button>
        </div>
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
                        aria-label="Export Loot Table"
                        onClick={_ => setFormData(p => ({...p, items: formData.items?.filter(i => i !== item), itemsChanged: true }))}
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

      {/* Add Loot Table Content Modal */}
      {isAddingContent && (
        <AddItemModal
          onAdd={addItem}
          onAddRandom={_ => {}}
          onCancel={() => {setIsAddingContent(false)}}
        />
      )}
    </div>
  );
}
