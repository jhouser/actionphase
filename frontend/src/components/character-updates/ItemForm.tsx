import { useState } from 'react';
import { Button, Input, Select } from '../ui';
import { CommentEditor } from '../CommentEditor';
import { apiClient } from '@/lib/api';
import { useOptionalGameContext } from '@/contexts/GameContext';
import { useQuery } from '@tanstack/react-query';

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
}) => {
  const gameContext = useOptionalGameContext();
  const [mode, setMode] = useState<'manual' | 'loot_table' | 'loot_table_random'>('manual');
  const [name, setName] = useState(initialValues?.name || '');
  const [description, setDescription] = useState(initialValues?.description || '');
  const [quantity, setQuantity] = useState(initialValues?.quantity || 1);
  const [category, setCategory] = useState(initialValues?.category || '');
  const [value, setValue] = useState<number | ''>(initialValues?.value ?? '');
  const [weight, setWeight] = useState<number | ''>(initialValues?.weight ?? '');
  const [lootTableId, setLootTableId] = useState<number | null>(null);
  const [lootTableContentId, setLootTableContentId] = useState<number | null>(null);


  const { data: lootTables } = useQuery({
    queryKey: ['lootTables', gameContext?.gameId],
    queryFn: () => apiClient.games.getLootTables(gameContext!.gameId).then(res => res.data),
    enabled: !!gameContext?.gameId
  });

  const { data: lootTableContents } = useQuery({
    queryKey: ['lootTableContents', lootTableId],
    queryFn: () => apiClient.games.getLootTableContents(gameContext!.gameId, lootTableId!).then(res => res.data),
    enabled: !!lootTableId
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) return;

    switch (mode) {
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
      case 'loot_table':
        if (!lootTableId || !lootTableContentId || !lootTableContents) return;
        let item = lootTableContents.find(content => content.id === lootTableContentId);
        if (!item) return;
        let itemData = JSON.parse(item.data) as Omit<ItemFormData, 'lootTableId'>;
        onSubmit({
          name: item.name,
          description: itemData.description,
          quantity: 1,
          category: itemData.category,
          value: itemData.value,
          weight: itemData.weight,
        });
        break;
      case 'loot_table_random':
        if (!lootTableId) return;
        onSubmit({
          name: '',
          quantity: 1,
          lootTableId: lootTableId,
        });
        break;
    }

    if (mode === 'loot_table' && (!lootTableId || !lootTableContentId)) return;
    if (mode === 'loot_table_random' && !lootTableId) return;
  };

  let manualForm = <div>
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

  let lootTableForm = <div>
      <Select
        id="item-loot-table"
        label="Loot Table"
        value={lootTableId || ''}
        onChange={(e) => setLootTableId(parseInt(e.target.value) || null)}
        required
      >
        <option value="">Select a loot table</option>
        {lootTables?.map((table) => (
          <option key={table.id} value={table.id}>
            {table.name}
          </option>
        ))}
      </Select>
      <Select
        id="item-loot-table-content"
        label="Loot Table Content"
        value={(lootTableContents?.length ?? 0) > 0 ? (lootTableContentId || '') : ''}
        onChange={(e) => setLootTableContentId(parseInt(e.target.value) || null)}
        required
      >
        <option value="">Select content</option>
        {lootTableContents?.map((content) => (
          <option key={content.id} value={content.id}>
            {content.name}
          </option>
        ))}
      </Select>
    </div>

  let lootTableRandomForm = <div>
      <Select
        id="item-loot-table"
        label="Loot Table"
        value={lootTableId || ''}
        onChange={(e) => setLootTableId(parseInt(e.target.value) || null)}
        required
      >
        <option value="">Select a loot table</option>
        {lootTables?.map((table) => (
          <option key={table.id} value={table.id}>
            {table.name}
          </option>
        ))}
      </Select>
    </div>

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
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
      
      {mode === 'manual' && manualForm}
      {mode === 'loot_table' && lootTableForm}
      {mode === 'loot_table_random' && lootTableRandomForm}

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
