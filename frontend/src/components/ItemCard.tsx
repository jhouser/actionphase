import { useState } from 'react';
import type { InventoryItem } from '../types/characters';
import { Button, Badge } from './ui';
import { MarkdownPreview } from './MarkdownPreview';
import { ItemForm, type ItemFormData } from './character-updates/ItemForm';

interface ItemCardProps {
  item: InventoryItem;
  canEdit: boolean;
  onUpdate: (updates: Partial<InventoryItem>) => void;
  onRemove: () => void;
  /** Reports whether this card's inline editor holds uncommitted edits. */
  onDirtyChange?: (isDirty: boolean) => void;
}

export const ItemCard: React.FC<ItemCardProps> = ({ item, canEdit, onUpdate, onRemove, onDirtyChange }) => {
  const [isEditing, setIsEditing] = useState(false);
  const [isExpanded, setIsExpanded] = useState(false);

  const handleSave = (data: ItemFormData) => {
    onUpdate({
      name: data.name,
      description: data.description,
      quantity: data.quantity,
      category: data.category,
      value: data.value,
      weight: data.weight,
    });
    setIsEditing(false);
  };

  const getCategoryVariant = (category?: string): 'primary' | 'success' | 'warning' | 'danger' | 'neutral' => {
    if (!category) return 'neutral';

    switch (category.toLowerCase()) {
      case 'weapon':
        return 'danger';
      case 'armor':
        return 'primary';
      case 'consumable':
        return 'success';
      case 'tool':
        return 'warning';
      default:
        return 'neutral';
    }
  };

  if (isEditing) {
    return (
      <div className="border border-theme-default rounded-lg p-5 surface-base hover:shadow-md transition-shadow">
        <ItemForm
          initialValues={{
            name: item.name,
            description: item.description,
            quantity: item.quantity,
            category: item.category,
            value: item.value,
            weight: item.weight,
          }}
          onSubmit={handleSave}
          onCancel={() => setIsEditing(false)}
          submitLabel="Save"
          variant="inline"
          onDirtyChange={onDirtyChange}
        />
      </div>
    );
  }

  return (
    <div className="border border-theme-default rounded-lg p-5 surface-base hover:shadow-md transition-shadow">
      <div className="flex justify-between items-start mb-3">
        <div className="flex-1">
          <div className="flex items-center space-x-2 flex-wrap gap-1">
            <h4 className="text-base font-semibold text-content-primary">{item.name}</h4>
            {item.quantity > 1 && (
              <Badge variant="neutral" size="sm">
                x{item.quantity}
              </Badge>
            )}
          </div>
        </div>

        <div className="flex items-center space-x-2 ml-4">
          {item.category && (
            <Badge variant={getCategoryVariant(item.category)} size="md">
              {item.category}
            </Badge>
          )}

          {canEdit && (
            <div className="flex space-x-1">
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setIsEditing(true)}
                className="p-1 text-interactive-primary hover:text-interactive-primary-hover"
                aria-label="Edit item"
              >
                ✎
              </Button>
              <Button
                variant="ghost"
                size="sm"
                onClick={onRemove}
                className="p-1 text-semantic-danger hover:text-semantic-danger"
                aria-label="Remove item"
              >
                🗑
              </Button>
            </div>
          )}
        </div>
      </div>

      {item.description && (
        <button
          onClick={() => setIsExpanded(!isExpanded)}
          className="flex items-center gap-1 px-2 py-1 mb-3 text-sm text-content-secondary hover:text-content-primary transition-colors rounded hover:bg-surface-secondary"
          aria-label={isExpanded ? "Collapse description" : "Expand description"}
        >
          <svg
            className="w-4 h-4 transition-transform"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            {isExpanded ? (
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
            ) : (
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
            )}
          </svg>
          <span>Description</span>
        </button>
      )}

      {item.description && isExpanded && (
        <div className="mb-3 text-sm">
          <MarkdownPreview content={item.description} />
        </div>
      )}

      {/* Item Stats */}
      {(item.weight || item.value || item.condition) && (
        <div className="flex flex-wrap gap-3 text-xs text-content-secondary border-t border-theme-default pt-3">
          {item.weight !== undefined && (
            <span className="flex items-center gap-1">
              <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 6l3 1m0 0l-3 9a5.002 5.002 0 006.001 0M6 7l3 9M6 7l6-2m6 2l3-1m-3 1l-3 9a5.002 5.002 0 006.001 0M18 7l3 9m-3-9l-6-2m0-2v2m0 16V5m0 16H9m3 0h3" />
              </svg>
              Weight: {(item.weight * item.quantity).toFixed(1)}
            </span>
          )}
          {item.value !== undefined && (
            <span className="flex items-center gap-1">
              <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              Value: {item.value * item.quantity}
            </span>
          )}
          {item.condition && (
            <span className="flex items-center gap-1">
              <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              {item.condition}
            </span>
          )}
        </div>
      )}
    </div>
  );
};
