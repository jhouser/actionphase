import { useState } from 'react';
import { Button, Input, Select } from '../ui';
import { CommentEditor } from '../CommentEditor';
import { useOptionalGameContext } from '@/contexts/GameContext';
import { logger } from '@/services/LoggingService';
import type { LootTableContent } from '@/types/games';
import { LootTableSelector } from './LootTableSelector';

export interface ItemFormData {
  name: string;
  description?: string;
  quantity: number;
  category?: string;
  value?: number;
  weight?: number;
  lootTableId?: number | null;
}

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
  allowLootModes?: boolean;
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
  allowLootModes = false,
}) => {
  const gameContext = useOptionalGameContext();
  // Loot tables are game-scoped, so both a caller opt-in and a game are required.
  const lootModesEnabled = allowLootModes && !!gameContext?.gameId;
  const [mode, setMode] = useState<'manual' | 'loot_table' | 'loot_table_random'>('manual');
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
    // Loot modes hidden => always submit as manual, regardless of any stale mode.
    const effectiveMode = lootModesEnabled ? mode : 'manual';
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

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      {lootModesEnabled && (
        <Select
          id="add-item-mode"
          label="Mode"
          value={mode}
          onChange={(e) => setMode(e.target.value as 'manual' | 'loot_table' | 'loot_table_random')}
          required
        >
          <option value="manual">Manual</option>
          <option value="loot_table">Loot Table</option>
          <option value="loot_table_random">Loot Table (Random)</option>
        </Select>
      )}

      {(!lootModesEnabled || mode === 'manual') && manualForm}
      {lootModesEnabled && mode !== 'manual' && (
        <LootTableSelector
          gameId={gameContext!.gameId}
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
