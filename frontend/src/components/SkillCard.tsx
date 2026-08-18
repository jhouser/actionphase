import { useState } from 'react';
import type { CharacterSkill } from '../types/characters';
import { skillRank } from '../types/characters';
import { Button, Badge } from './ui';
import { MarkdownPreview } from './MarkdownPreview';
import { SkillForm, type SkillFormData } from './character-updates/SkillForm';

interface SkillCardProps {
  skill: CharacterSkill;
  canEdit: boolean;
  onUpdate: (updates: Partial<CharacterSkill>) => void;
  onRemove: () => void;
  /** Reports whether this card's inline editor holds uncommitted edits. */
  onDirtyChange?: (isDirty: boolean) => void;
}

export const SkillCard: React.FC<SkillCardProps> = ({ skill, canEdit, onUpdate, onRemove, onDirtyChange }) => {
  const [isEditing, setIsEditing] = useState(false);
  const [isExpanded, setIsExpanded] = useState(false);
  const rank = skillRank(skill);

  const handleSave = (data: SkillFormData) => {
    onUpdate({
      name: data.name,
      rank: data.rank || undefined,
      description: data.description,
      category: data.category,
      // Clear the legacy key so an edited row stops carrying both spellings of
      // its rank. The parent merges updates onto the existing entry, so without
      // this an edited legacy row keeps a stale `level` that disagrees with the
      // `rank` now being displayed. Rows nobody edits keep `level` and read
      // through the fallback, which is why this is not a migration.
      level: undefined,
    });
    setIsEditing(false);
  };

  if (isEditing) {
    return (
      <div className="border border-theme-default rounded-lg p-5 surface-base hover:shadow-md transition-shadow">
        <SkillForm
          initialValues={{
            name: skill.name,
            rank: skillRank(skill),
            description: skill.description,
            category: skill.category,
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
          <h4 className="text-base font-semibold text-content-primary mb-1">{skill.name}</h4>
          {rank && (
            <span className="text-sm text-interactive-primary font-medium">Rank: {rank}</span>
          )}
        </div>

        {canEdit && (
          <div className="flex space-x-1 ml-4">
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
              className="p-1 text-semantic-danger hover:text-semantic-danger"
            >
              🗑
            </Button>
          </div>
        )}
      </div>

      {skill.description && (
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

      {skill.description && isExpanded && (
        <div className="mb-3 text-sm">
          <MarkdownPreview content={skill.description} />
        </div>
      )}

      {skill.category && (
        <Badge variant="primary" size="sm">
          {skill.category}
        </Badge>
      )}
    </div>
  );
};
