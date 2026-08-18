import { useState } from 'react';
import type { CharacterAbility } from '../types/characters';
import { Button, Badge } from './ui';
import { MarkdownPreview } from './MarkdownPreview';
import { AbilityForm, type AbilityFormData } from './character-updates/AbilityForm';

interface AbilityCardProps {
  ability: CharacterAbility;
  canEdit: boolean;
  onUpdate: (updates: Partial<CharacterAbility>) => void;
  onRemove: () => void;
  /** Reports whether this card's inline editor holds uncommitted edits. */
  onDirtyChange?: (isDirty: boolean) => void;
}

export const AbilityCard: React.FC<AbilityCardProps> = ({ ability, canEdit, onUpdate, onRemove, onDirtyChange }) => {
  const [isEditing, setIsEditing] = useState(false);
  const [isExpanded, setIsExpanded] = useState(false);

  const handleSave = (data: AbilityFormData) => {
    onUpdate({
      name: data.name,
      description: data.description,
      type: data.type,
    });
    setIsEditing(false);
  };

  const getTypeVariant = (type: CharacterAbility['type']): 'primary' | 'success' | 'warning' | 'neutral' => {
    switch (type) {
      case 'gm_assigned':
        return 'warning';
      case 'learned':
        return 'primary';
      case 'innate':
        return 'success';
      default:
        return 'neutral';
    }
  };

  if (isEditing) {
    return (
      <div className="border border-theme-default rounded-lg p-5 surface-base hover:shadow-md transition-shadow">
        <AbilityForm
          initialValues={{
            name: ability.name,
            description: ability.description,
            type: ability.type,
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
          <h4 className="text-base font-semibold text-content-primary mb-1">{ability.name}</h4>
        </div>

        <div className="flex items-center space-x-2 ml-4">
          <Badge variant={getTypeVariant(ability.type)} size="md">
            {ability.type.replace('_', ' ')}
          </Badge>

          {canEdit && ability.type !== 'gm_assigned' && (
            <div className="flex space-x-1">
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setIsEditing(true)}
                className="p-1 text-interactive-primary hover:text-interactive-primary-hover"
              >
                ✎
              </Button>
              <Button
                variant="ghost"
                size="sm"
                onClick={onRemove}
                aria-label="Remove ability"
                className="p-1 text-semantic-danger hover:text-semantic-danger"
              >
                🗑
              </Button>
            </div>
          )}
        </div>
      </div>

      {ability.description && (
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

      {ability.description && isExpanded && (
        <div className="mb-3 text-sm">
          <MarkdownPreview content={ability.description} />
        </div>
      )}

      <div className="flex items-center gap-3 text-xs text-content-tertiary">
        {ability.source && (
          <span className="flex items-center gap-1">
            <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            Source: {ability.source}
          </span>
        )}
        {!ability.active && (
          <Badge variant="neutral" size="sm">Inactive</Badge>
        )}
      </div>
    </div>
  );
};
