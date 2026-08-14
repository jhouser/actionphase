import { useEffect, useState } from 'react';
import { Button, Input, Select } from '../ui';
import { CommentEditor } from '../CommentEditor';
import { useOptionalGameContext } from '@/contexts/GameContext';
import { logger } from '@/services/LoggingService';
import type { LootTableContent } from '@/types/games';
import { LootTableSelector } from './LootTableSelector';
import { useQuery } from '@tanstack/react-query';
import { apiClient } from '@/lib/api';

export interface ItemFormData {
  name: string;
  description?: string;
  quantity: number;
  category?: string;
  value?: number;
  weight?: number;
  lootTableId?: number | null;
}

export type lootModes = 'manual' | 'loot_table' | 'loot_table_random';

interface ItemFormProps {
  onSubmit: (data: ItemFormData) => void;
  onCancel: () => void;
  initialValues?: Partial<ItemFormData>;
  submitLabel?: string;
  variant?: 'modal' | 'inline';
  submitButtonTestId?: string;
  /**
   * Whether to offer the loot-table modes in addition to manual entry. Off by
   * default: callers that only build plain items — editing an existing item, or
   * defining the contents of a loot table — should not be able to source an item
   * from a loot table.
   */
  allowedLootModes?: lootModes[];
}


/**
 * Shared form component for adding/editing inventory items.
 * Used in both AddItemModal and InventoryTab to ensure consistency.
 */
export const ItemForm: React.FC<ItemFormProps> = ({
  onSubmit,
  onCancel,
  initialValues,
  submitLabel = 'Add Item',
  variant = 'modal',
  submitButtonTestId,
  allowedLootModes = ['manual'],
}) => {
  const gameContext = useOptionalGameContext();

  // Loot tables are game-scoped, so both a caller opt-in and a game are required.
  // Loot Mode manual is always enabled, we disable the others if there is no game context or loot tables
  const [lootModesEnabled, setLootModesEnabled] = useState<Record<lootModes, boolean>>({
    manual: true,
    loot_table: !!gameContext?.gameId && allowedLootModes.includes('loot_table') ,
    loot_table_random: !!gameContext?.gameId && allowedLootModes.includes('loot_table_random'),
  });
  
  const { data: lootTables, isLoading: isLootTablesLoading, isFetched: isLootTablesFetched, isError: isLootTablesError } = useQuery({
    queryKey: ['lootTables', gameContext?.gameId, true],
    queryFn: () => apiClient.games.getLootTables(gameContext!.gameId, true).then((res) => res.data),
    enabled: !!gameContext?.gameId && (lootModesEnabled.loot_table || lootModesEnabled.loot_table_random)
  });

  useEffect(() => {
    if (isLootTablesFetched || isLootTablesError) {
      setLootModesEnabled({
        manual: lootModesEnabled.manual,
        loot_table: lootModesEnabled.loot_table && (lootTables?.length ?? 0) > 0,
        loot_table_random: lootModesEnabled.loot_table_random && (lootTables?.length ?? 0) > 0,
      });
    }
  }, [ lootTables ]);

  const [mode, setMode] = useState<lootModes>('manual');
  const [name, setName] = useState(initialValues?.name || '');
  const [description, setDescription] = useState(initialValues?.description || '');
  const [quantity, setQuantity] = useState(initialValues?.quantity || 1);
  const [category, setCategory] = useState(initialValues?.category || '');
  const [value, setValue] = useState<number | ''>(initialValues?.value ?? '');
  const [weight, setWeight] = useState<number | ''>(initialValues?.weight ?? '');
  const [lootTableId, setLootTableId] = useState<number | null>(null);
  const [selectedLootItem, setSelectedLootItem] = useState<LootTableContent | null>(null);


  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    // Disabled loot mode => always submit as manual, regardless of any stale mode.
    const effectiveMode = lootModesEnabled[mode] ? mode : 'manual';
    if (effectiveMode === 'manual' && !name.trim()) return;

    switch (effectiveMode) {
      case 'manual':
        onSubmit({
          name: name.trim(),
          description: description.trim() || undefined,
          quantity,
          category: category.trim() || undefined,
          value: value || undefined,
          weight: weight || undefined,
        });
        break;
      case 'loot_table': {
        if (!selectedLootItem) return;
        let itemData: Omit<ItemFormData, 'lootTableId'>;
        try {
          itemData = JSON.parse(selectedLootItem.data);
        } catch {
          // GM-authored free text, so it can be malformed; don't throw mid-submit.
          logger.error('Loot table item data is not valid JSON', { itemName: selectedLootItem.name });
          return;
        }
        onSubmit({
          name: selectedLootItem.name,
          description: itemData.description,
          quantity: 1,
          category: itemData.category,
          value: itemData.value,
          weight: itemData.weight,
        });
        break;
      }
      case 'loot_table_random':
        if (!lootTableId) return;
        onSubmit({
          name: '',
          quantity: 1,
          lootTableId: lootTableId,
        });
        break;
    }
  };

  const manualForm = <div>
    <Input
        id="item-name"
        label="Item Name *"
        type="text"
        value={name}
        onChange={(e) => setName(e.target.value)}
        placeholder="e.g., Iron Sword, Health Potion"
        required
      />

      <div className="grid grid-cols-2 gap-3">
        <Input
          id="item-quantity"
          label="Quantity"
          type="number"
          value={quantity}
          onChange={(e) => setQuantity(parseInt(e.target.value) || 1)}
          min={1}
          required
        />
        <Input
          id="item-category"
          label="Category"
          type="text"
          value={category}
          onChange={(e) => setCategory(e.target.value)}
          placeholder="Weapon, Armor, etc."
        />
      </div>

      <div className="grid grid-cols-2 gap-3">
        <Input
          id="item-value"
          label="Value"
          type="number"
          value={value}
          onChange={(e) => setValue(parseFloat(e.target.value) || '')}
          min={0}
          step="any"
          placeholder="0"
        />
        <Input
          id="item-weight"
          label="Weight"
          type="number"
          value={weight}
          onChange={(e) => setWeight(parseFloat(e.target.value) || '')}
          min={0}
          step="any"
          placeholder="0.0"
        />
      </div>

      <div>
        <label htmlFor="item-description" className="block text-sm font-medium text-content-primary mb-2">
          Description <span className="text-xs text-content-tertiary font-normal">(Markdown supported)</span>
        </label>
        <CommentEditor
          id="item-description"
          value={description}
          onChange={setDescription}
          placeholder="Describe this item..."
          rows={2}
          showPreviewByDefault={false}
        />
      </div>
  </div>

  if (isLootTablesLoading) {
    return (
      <div className={`surface-base rounded-lg border border-theme-default p-6`}>
        <div className="animate-pulse">
          <div className="h-6 surface-sunken rounded mb-4 w-1/3"></div>
          <div className="space-y-3">
            {[...Array(3)].map((_, i) => (
              <div key={i} className="h-16 surface-sunken rounded"></div>
            ))}
          </div>
        </div>
      </div>);
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      {(lootModesEnabled.loot_table || lootModesEnabled.loot_table_random) && (
        <Select
          id="add-item-mode"
          label="Mode"
          value={mode}
          onChange={(e) => setMode(e.target.value as lootModes)}
          required
        >
          <option value="manual">Manual</option>
          {lootModesEnabled.loot_table && (<option value="loot_table">Loot Table</option>)}
          {lootModesEnabled.loot_table_random && (<option value="loot_table_random">Loot Table (Random)</option>)}
        </Select>
      )}

      {(!lootModesEnabled[mode] || mode === 'manual') && manualForm}
      {lootModesEnabled[mode] && mode !== 'manual' && (
        <LootTableSelector
          gameId={gameContext!.gameId}
          lootTables={lootTables!}
          requireItem={mode === 'loot_table'}
          lootTableId={lootTableId}
          onLootTableChange={(id) => {
            setLootTableId(id);
            setSelectedLootItem(null);
          }}
          onItemChange={setSelectedLootItem}
        />
      )}

      <div className={`flex justify-end gap-3 ${variant === 'modal' ? 'pt-4' : 'pt-2'}`}>
        <Button
          type="button"
          variant="secondary"
          onClick={onCancel}
        >
          Cancel
        </Button>
        <Button
          type="submit"
          variant="primary"
          data-testid={submitButtonTestId}
        >
          {submitLabel}
        </Button>
      </div>
    </form>
  );
};
