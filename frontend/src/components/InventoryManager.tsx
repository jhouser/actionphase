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
import { useGameContext } from '@/contexts/GameContext';
import { useToast } from '@/contexts/ToastContext';

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
}

export const InventoryManager: React.FC<InventoryManagerProps> = ({
  characterId,
  items,
  currency,
  canEdit,
  onItemsChange,
  onCurrencyChange
}) => {
  // Defensive: Ensure all items and currency have IDs (protects against backend data corruption)
  const validatedItems = useMemo(() => ensureIds(items, 'Item'), [items]);
  const validatedCurrency = useMemo(() => ensureIds(currency, 'Currency'), [currency]);

  const [activeTab, setActiveTab] = useState<'items' | 'currency'>('items');
  const [showAddItem, setShowAddItem] = useState(false);
  const [showAddCurrency, setShowAddCurrency] = useState(false);

  const gameContext = useGameContext();

  const { showSuccess } = useToast();

  const addItem = (itemData: Omit<InventoryItem, 'id'>) => {
    const newItem: InventoryItem = {
      id: generateId(),
      ...itemData
    };
    onItemsChange([...validatedItems, newItem], false);
    setShowAddItem(false);
  };

  const addRandomItem = (lootTableId: number): void => {
    apiClient.games.giveRandomLootTableContent(gameContext.gameId, lootTableId, characterId).then((r) => {
      onItemsChange([...validatedItems, {id: generateId(), ...JSON.parse(r.data.data)}], true);
      setShowAddItem(false);
      showSuccess(`Added item ${r.data.name} to character sheet`);
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
      {/* Tab Navigation - Hide currency tab if user cannot edit */}
      <div className="flex space-x-1 mb-6 border-b border-theme-default">
        <Button
          variant="ghost"
          onClick={() => setActiveTab('items')}
          className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors rounded-none ${
            activeTab === 'items'
              ? 'border-interactive-primary text-interactive-primary'
              : 'border-transparent text-content-secondary hover:text-content-primary'
          }`}
        >
          Items ({validatedItems.length})
        </Button>
        <Button
          variant="ghost"
          onClick={() => setActiveTab('currency')}
          className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors rounded-none ${
            activeTab === 'currency'
              ? 'border-interactive-primary text-interactive-primary'
              : 'border-transparent text-content-secondary hover:text-content-primary'
          }`}
        >
          Currency ({validatedCurrency.length})
        </Button>
      </div>

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
                />
              ))}
            </div>
          )}

          {showAddItem && (
            <AddItemModal
              onAdd={addItem}
              onAddRandom={addRandomItem}
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
