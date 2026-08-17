import React, { useState, useMemo } from 'react';
import type { CharacterAbility, CharacterSkill } from '../types/characters';
import { AbilityCard } from './AbilityCard';
import { SkillCard } from './SkillCard';
import { AddAbilityModal } from './AddAbilityModal';
import { AddSkillModal } from './AddSkillModal';
import { Button } from './ui';
import { generateId } from '../utils/generateId';
import { logger } from '@/services/LoggingService';
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

interface AbilitiesManagerProps {
  abilities: CharacterAbility[];
  skills: CharacterSkill[];
  canEdit: boolean;
  onAbilitiesChange: (abilities: CharacterAbility[]) => void;
  onSkillsChange: (skills: CharacterSkill[]) => void;
  /**
   * Reports whether any ability/skill editor below holds edits that have not been
   * committed with Save. Ancestors use it to warn before closing the sheet.
   */
  onDirtyChange?: (isDirty: boolean) => void;
}

export const AbilitiesManager: React.FC<AbilitiesManagerProps> = ({
  abilities,
  skills,
  canEdit,
  onAbilitiesChange,
  onSkillsChange,
  onDirtyChange
}) => {
  const { isAnyDirty, report: reportDirty } = useDirtyChildren(onDirtyChange);
  // Defensive: Ensure all abilities and skills have IDs (protects against backend data corruption)
  const validatedAbilities = useMemo(() => ensureIds(abilities, 'Ability'), [abilities]);
  const validatedSkills = useMemo(() => ensureIds(skills, 'Skill'), [skills]);

  const [activeTab, setActiveTab] = useState<'abilities' | 'skills'>('abilities');
  const [showAddAbility, setShowAddAbility] = useState(false);
  const [showAddSkill, setShowAddSkill] = useState(false);

  const addAbility = (abilityData: Omit<CharacterAbility, 'id'>) => {
    const newAbility: CharacterAbility = {
      id: generateId(),
      ...abilityData
    };
    onAbilitiesChange([...validatedAbilities, newAbility]);
    setShowAddAbility(false);
  };

  const addSkill = (skillData: Omit<CharacterSkill, 'id'>) => {
    const newSkill: CharacterSkill = {
      id: generateId(),
      ...skillData
    };
    onSkillsChange([...validatedSkills, newSkill]);
    setShowAddSkill(false);
  };

  const removeAbility = (id: string) => {
    onAbilitiesChange(validatedAbilities.filter(a => a.id !== id));
  };

  const removeSkill = (id: string) => {
    onSkillsChange(validatedSkills.filter(s => s.id !== id));
  };

  const updateAbility = (id: string, updates: Partial<CharacterAbility>) => {
    onAbilitiesChange(validatedAbilities.map(a => a.id === id ? { ...a, ...updates } : a));
  };

  const updateSkill = (id: string, updates: Partial<CharacterSkill>) => {
    onSkillsChange(validatedSkills.map(s => s.id === id ? { ...s, ...updates } : s));
  };

  return (
    <div>
      {/* Locked while an editor below holds uncommitted edits, since switching
          unmounts that editor and destroys them. */}
      <ManagerSubTabs
        tabs={[
          { id: 'abilities', label: 'Abilities', count: validatedAbilities.length },
          { id: 'skills', label: 'Skills', count: validatedSkills.length },
        ]}
        activeTab={activeTab}
        onTabChange={setActiveTab}
        locked={isAnyDirty}
      />

      {/* Abilities Tab */}
      {activeTab === 'abilities' && (
        <div>
          <div className="flex justify-between items-center mb-4">
            <h3 className="text-lg font-medium text-content-primary">Abilities</h3>
            {canEdit && (
              <Button
                variant="primary"
                size="sm"
                onClick={() => setShowAddAbility(true)}
              >
                Add Ability
              </Button>
            )}
          </div>

          {validatedAbilities.length === 0 ? (
            <div className="text-center py-8 text-content-secondary">
              <p>No abilities yet.</p>
              {canEdit && <p className="text-sm mt-1">Click "Add Ability" to get started.</p>}
            </div>
          ) : (
            <div className="space-y-3">
              {validatedAbilities.map((ability) => (
                <AbilityCard
                  key={ability.id}
                  ability={ability}
                  canEdit={canEdit}
                  onUpdate={(updates) => updateAbility(ability.id, updates)}
                  onRemove={() => removeAbility(ability.id)}
                  onDirtyChange={(isDirty) => reportDirty(`ability:${ability.id}`, isDirty)}
                />
              ))}
            </div>
          )}

          {showAddAbility && (
            <AddAbilityModal
              onAdd={addAbility}
              onCancel={() => setShowAddAbility(false)}
            />
          )}
        </div>
      )}

      {/* Skills Tab */}
      {activeTab === 'skills' && (
        <div>
          <div className="flex justify-between items-center mb-4">
            <h3 className="text-lg font-medium text-content-primary">Skills</h3>
            {canEdit && (
              <Button
                variant="primary"
                size="sm"
                onClick={() => setShowAddSkill(true)}
              >
                Add Skill
              </Button>
            )}
          </div>

          {validatedSkills.length === 0 ? (
            <div className="text-center py-8 text-content-secondary">
              <p>No skills yet.</p>
              {canEdit && <p className="text-sm mt-1">Click "Add Skill" to get started.</p>}
            </div>
          ) : (
            <div className="space-y-3">
              {validatedSkills.map((skill) => (
                <SkillCard
                  key={skill.id}
                  skill={skill}
                  canEdit={canEdit}
                  onUpdate={(updates) => updateSkill(skill.id, updates)}
                  onRemove={() => removeSkill(skill.id)}
                  onDirtyChange={(isDirty) => reportDirty(`skill:${skill.id}`, isDirty)}
                />
              ))}
            </div>
          )}

          {showAddSkill && (
            <AddSkillModal
              onAdd={addSkill}
              onCancel={() => setShowAddSkill(false)}
            />
          )}
        </div>
      )}
    </div>
  );
};
