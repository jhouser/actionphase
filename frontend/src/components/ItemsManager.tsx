import React, { useState, useMemo } from 'react';
import type { InventoryItem } from '../types/characters';
import { ItemCard } from './ItemCard';
import { AddItemModal } from './AddItemModal';
import { Button } from './ui';
import { generateId } from '../utils/generateId';
import { ensureIds } from '../utils/ensureIds';
import { logger } from '@/services/LoggingService';
import { apiClient } from '@/lib/api';
import { useOptionalGameContext } from '@/contexts/GameContext';
import { useToast } from '@/contexts/ToastContext';
import { useDirtyChildren } from '@/hooks/useDirtyChildren';

interface ItemsManagerProps {
  characterId: number;
  items: InventoryItem[];
  canEdit: boolean;
  onItemsChange: (items: InventoryItem[], reloadOnly: boolean) => void;
  /**
   * Reports whether any item editor below holds edits that have not been
   * committed with Save. Ancestors use it to warn before closing the sheet.
   */
  onDirtyChange?: (isDirty: boolean) => void;
  /** The tab's name in this game; the GM may have renamed it (e.g. "Load"). */
  label: string;
}

/**
 * The Inventory tab of the character sheet.
 *
 * Extracted from the former InventoryManager, which held Items and Currency
 * behind sub-tabs. Currency was promoted to its own top-level tab (NumbersManager)
 * because nothing about a numeric track is an item, so this manages one
 * collection and needs no sub-tab bar.
 */
export const ItemsManager: React.FC<ItemsManagerProps> = ({
  characterId,
  items,
  canEdit,
  onItemsChange,
  onDirtyChange,
  label,
}) => {
  const { report: reportDirty } = useDirtyChildren(onDirtyChange);
  // Defensive: ensure every item has an ID (protects against draft-merge corruption)
  const validatedItems = useMemo(() => ensureIds(items, 'Item'), [items]);

  const [showAddItem, setShowAddItem] = useState(false);

  // Optional: this component also renders outside a GameProvider (e.g. the
  // character sheet editor). useGameContext() throws there, which took the whole
  // subtree down. Without a game we simply cannot offer loot rolls — see
  // canRollLoot below — but everything else in the inventory still works.
  const gameContext = useOptionalGameContext();
  const canRollLoot = gameContext !== null;

  const { showSuccess, showError } = useToast();

  const addItem = (itemData: Omit<InventoryItem, 'id'>) => {
    onItemsChange([...validatedItems, { id: generateId(), ...itemData }], false);
    setShowAddItem(false);
  };

  const addRandomItem = (lootTableId: number): void => {
    if (!gameContext) {
      // Defensive: the loot modes are hidden without a game context, so this is
      // unreachable through the UI.
      logger.error('Random loot roll attempted with no game context', { characterId });
      showError('Loot tables are unavailable here.');
      return;
    }
    apiClient.games.giveRandomLootTableContent(gameContext.gameId, lootTableId, characterId)
      .then((r) => {
        // The item payload is GM-authored JSON stored as free text, so it can be
        // malformed. Report that rather than throwing inside the success path.
        let rolledItem: Omit<InventoryItem, 'id'>;
        try {
          rolledItem = JSON.parse(r.data.data);
        } catch {
          logger.error('Loot item data is not valid JSON', { lootTableId, itemName: r.data.name });
          showError(`Rolled "${r.data.name}" but its item data is malformed. Check the loot table.`);
          return;
        }
        onItemsChange([...validatedItems, { id: generateId(), ...rolledItem }], true);
        setShowAddItem(false);
        showSuccess(`Added item ${r.data.name} to character sheet`);
      })
      .catch((error: unknown) => {
        // Without this the request failed silently: the modal stayed open with no
        // feedback (e.g. rolling on an empty loot table returns 400).
        const message =
          (error as { response?: { data?: { error?: string } } })?.response?.data?.error ||
          'Failed to roll for a random item. Please try again.';
        logger.error('Random loot roll failed', { lootTableId, characterId, error });
        showError(message);
      });
  };

  const removeItem = (id: string) => {
    onItemsChange(validatedItems.filter(i => i.id !== id), false);
  };

  const updateItem = (id: string, updates: Partial<InventoryItem>) => {
    onItemsChange(validatedItems.map(i => i.id === id ? { ...i, ...updates } : i), false);
  };

  const getTotalWeight = () =>
    validatedItems.reduce((total, item) => total + ((item.weight || 0) * item.quantity), 0);

  const getTotalValue = () =>
    validatedItems.reduce((total, item) => total + ((item.value || 0) * item.quantity), 0);

  // Both fields are optional and no game in play sets them, so summing them
  // unconditionally rendered a meaningless "Total Weight: 0.0 • Total Value: 0"
  // under every inventory. Show the line only once some item opts in.
  const hasWeightOrValue = validatedItems.some(
    (item) => item.weight !== undefined || item.value !== undefined
  );

  return (
    <div data-testid="items-section">
      <div className="flex justify-between items-center mb-4">
        <div>
          <h3 className="text-lg font-medium text-content-primary">{label}</h3>
          {validatedItems.length > 0 && hasWeightOrValue && (
            <div className="text-sm text-content-tertiary mt-1">
              Total Weight: {getTotalWeight().toFixed(1)} • Total Value: {getTotalValue()}
            </div>
          )}
        </div>
        {canEdit && (
          <Button variant="primary" size="sm" onClick={() => setShowAddItem(true)}>
            Add Item
          </Button>
        )}
      </div>

      {validatedItems.length === 0 ? (
        <div className="text-center py-8 text-content-secondary">
          <p>No items yet.</p>
          {canEdit && <p className="text-sm mt-1">Click "Add Item" to get started.</p>}
        </div>
      ) : (
        <div className="space-y-3">
          {validatedItems.map((item) => (
            <ItemCard
              key={item.id}
              item={item}
              canEdit={canEdit}
              onUpdate={(updates) => updateItem(item.id, updates)}
              onRemove={() => removeItem(item.id)}
              onDirtyChange={(isDirty) => reportDirty(`item:${item.id}`, isDirty)}
            />
          ))}
        </div>
      )}

      {showAddItem && (
        <AddItemModal
          onAdd={addItem}
          onAddRandom={addRandomItem}
          allowedLootModes={canRollLoot ? ['manual', 'loot_table', 'loot_table_random'] : ['manual']}
          onCancel={() => setShowAddItem(false)}
        />
      )}
    </div>
  );
};
