import React, { useState, useMemo } from 'react';
import type { InventoryItem, CurrencyEntry } from '../types/characters';
import { ItemCard } from './ItemCard';
import { CurrencyCard } from './CurrencyCard';
import { AddItemModal } from './AddItemModal';
import { AddCurrencyModal } from './AddCurrencyModal';
import { Button } from './ui';
import { generateId } from '../utils/generateId';
import { logger } from '@/services/LoggingService';
import { apiClient } from '@/lib/api';
import { useOptionalGameContext } from '@/contexts/GameContext';
import { useToast } from '@/contexts/ToastContext';
import { useDirtyChildren } from '@/hooks/useDirtyChildren';
import { ManagerSubTabs } from './ManagerSubTabs';

// Defensive helper to ensure all items have ID fields
// This protects against data corruption from draft merge bugs
const ensureIds = <T extends { id?: string }>(
  items: T[],
  itemType: string
): (T & { id: string })[] => {
  return items.map(item => {
    if (!item.id) {
      logger.warn(`${itemType} missing id field (data corruption), generating:`, item);
      return { ...item, id: generateId() };
    }
    return item as T & { id: string };
  });
};

interface InventoryManagerProps {
  characterId: number;
  items: InventoryItem[];
  currency: CurrencyEntry[];
  canEdit: boolean;
  onItemsChange: (items: InventoryItem[], reloadOnly: boolean) => void;
  onCurrencyChange: (currency: CurrencyEntry[]) => void;
  /**
   * Reports whether any item/currency editor below holds edits that have not been
   * committed with Save. Ancestors use it to warn before closing the sheet.
   */
  onDirtyChange?: (isDirty: boolean) => void;
}

export const InventoryManager: React.FC<InventoryManagerProps> = ({
  characterId,
  items,
  currency,
  canEdit,
  onItemsChange,
  onCurrencyChange,
  onDirtyChange
}) => {
  const { isAnyDirty, report: reportDirty } = useDirtyChildren(onDirtyChange);
  // Defensive: Ensure all items and currency have IDs (protects against backend data corruption)
  const validatedItems = useMemo(() => ensureIds(items, 'Item'), [items]);
  const validatedCurrency = useMemo(() => ensureIds(currency, 'Currency'), [currency]);

  const [activeTab, setActiveTab] = useState<'items' | 'currency'>('items');
  const [showAddItem, setShowAddItem] = useState(false);
  const [showAddCurrency, setShowAddCurrency] = useState(false);

  // Optional: this component also renders outside a GameProvider (e.g. the
  // character sheet editor). useGameContext() throws there, which took the whole
  // subtree down. Without a game we simply cannot offer loot rolls — see
  // canRollLoot below — but everything else in the inventory still works.
  const gameContext = useOptionalGameContext();
  const canRollLoot = gameContext !== null;

  const { showSuccess, showError } = useToast();

  const addItem = (itemData: Omit<InventoryItem, 'id'>) => {
    const newItem: InventoryItem = {
      id: generateId(),
      ...itemData
    };
    onItemsChange([...validatedItems, newItem], false);
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
  }

  const addCurrency = (currencyData: Omit<CurrencyEntry, 'id'>) => {
    const newCurrency: CurrencyEntry = {
      id: generateId(),
      ...currencyData
    };
    onCurrencyChange([...validatedCurrency, newCurrency]);
    setShowAddCurrency(false);
  };

  const removeItem = (id: string) => {
    onItemsChange(validatedItems.filter(i => i.id !== id), false);
  };

  const removeCurrency = (id: string) => {
    onCurrencyChange(validatedCurrency.filter(c => c.id !== id));
  };

  const updateItem = (id: string, updates: Partial<InventoryItem>) => {
    onItemsChange(validatedItems.map(i => i.id === id ? { ...i, ...updates } : i), false);
  };

  const updateCurrency = (id: string, updates: Partial<CurrencyEntry>) => {
    onCurrencyChange(validatedCurrency.map(c => c.id === id ? { ...c, ...updates } : c));
  };

  const getTotalWeight = () => {
    return validatedItems.reduce((total, item) => total + ((item.weight || 0) * item.quantity), 0);
  };

  const getTotalValue = () => {
    return validatedItems.reduce((total, item) => total + ((item.value || 0) * item.quantity), 0);
  };

  return (
    <div>
      {/* Locked while an editor below holds uncommitted edits, since switching
          unmounts that editor and destroys them. */}
      <ManagerSubTabs
        tabs={[
          { id: 'items', label: 'Items', count: validatedItems.length },
          { id: 'currency', label: 'Currency', count: validatedCurrency.length },
        ]}
        activeTab={activeTab}
        onTabChange={setActiveTab}
        locked={isAnyDirty}
      />

      {/* Items Tab */}
      {activeTab === 'items' && (
        <div>
          <div className="flex justify-between items-center mb-4">
            <div>
              <h3 className="text-lg font-medium text-content-primary">Items</h3>
              {validatedItems.length > 0 && (
                <div className="text-sm text-content-tertiary mt-1">
                  Total Weight: {getTotalWeight().toFixed(1)} • Total Value: {getTotalValue()}
                </div>
              )}
            </div>
            {canEdit && (
              <Button
                variant="primary"
                size="sm"
                onClick={() => setShowAddItem(true)}
              >
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
              allowedLootModes={canRollLoot ? [ 'manual', 'loot_table', 'loot_table_random' ] : [ 'manual' ]}
              onCancel={() => setShowAddItem(false)}
            />
          )}
        </div>
      )}

      {/* Currency Tab */}
      {activeTab === 'currency' && (
        <div data-testid="currency-section">
          <div className="flex justify-between items-center mb-4">
            <h3 className="text-lg font-medium text-content-primary">Currency & Resources</h3>
            {canEdit && (
              <Button
                variant="primary"
                size="sm"
                onClick={() => setShowAddCurrency(true)}
              >
                Add Currency
              </Button>
            )}
          </div>

          {validatedCurrency.length === 0 ? (
            <div className="text-center py-8 text-content-secondary">
              <p>No currency tracked yet.</p>
              {canEdit && <p className="text-sm mt-1">Click "Add Currency" to get started.</p>}
            </div>
          ) : (
            <div className="space-y-3">
              {validatedCurrency.map((curr) => (
                <CurrencyCard
                  key={curr.id}
                  currency={curr}
                  canEdit={canEdit}
                  onUpdate={(updates) => updateCurrency(curr.id, updates)}
                  onRemove={() => removeCurrency(curr.id)}
                  onDirtyChange={(isDirty) => reportDirty(`currency:${curr.id}`, isDirty)}
                />
              ))}
            </div>
          )}

          {showAddCurrency && (
            <AddCurrencyModal
              onAdd={addCurrency}
              onCancel={() => setShowAddCurrency(false)}
            />
          )}
        </div>
      )}
    </div>
  );
};
