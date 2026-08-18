import React, { useState, useMemo } from 'react';
import type { CharacterSkill } from '../types/characters';
import { SkillCard } from './SkillCard';
import { AddSkillModal } from './AddSkillModal';
import { Button } from './ui';
import { generateId } from '../utils/generateId';
import { ensureIds } from '../utils/ensureIds';
import { useDirtyChildren } from '@/hooks/useDirtyChildren';

interface SkillsManagerProps {
  skills: CharacterSkill[];
  canEdit: boolean;
  onSkillsChange: (skills: CharacterSkill[]) => void;
  /**
   * Reports whether any skill editor below holds edits that have not been
   * committed with Save. Ancestors use it to warn before closing the sheet.
   */
  onDirtyChange?: (isDirty: boolean) => void;
  /**
   * The tab's name in this game, for the heading and empty-state copy. The GM
   * may have renamed it (a Blades game calls this "Playbook"), so nothing here
   * hardcodes "Skills".
   */
  label: string;
}

/**
 * The Skills tab of the character sheet.
 *
 * Extracted from the former AbilitiesManager, which held Skills and Abilities
 * behind sub-tabs. Abilities were retired in the same change (they duplicated
 * Skills, which is strictly more featured), so this manages one collection and
 * needs no sub-tab bar of its own.
 */
export const SkillsManager: React.FC<SkillsManagerProps> = ({
  skills,
  canEdit,
  onSkillsChange,
  onDirtyChange,
  label,
}) => {
  const { report: reportDirty } = useDirtyChildren(onDirtyChange);
  // Defensive: ensure every skill has an ID (protects against draft-merge corruption)
  const validatedSkills = useMemo(() => ensureIds(skills, 'Skill'), [skills]);

  const [showAddSkill, setShowAddSkill] = useState(false);

  const addSkill = (skillData: Omit<CharacterSkill, 'id'>) => {
    onSkillsChange([...validatedSkills, { id: generateId(), ...skillData }]);
    setShowAddSkill(false);
  };

  const removeSkill = (id: string) => {
    onSkillsChange(validatedSkills.filter(s => s.id !== id));
  };

  const updateSkill = (id: string, updates: Partial<CharacterSkill>) => {
    onSkillsChange(validatedSkills.map(s => s.id === id ? { ...s, ...updates } : s));
  };

  return (
    <div data-testid="skills-section">
      <div className="flex justify-between items-center mb-4">
        <h3 className="text-lg font-medium text-content-primary">{label}</h3>
        {canEdit && (
          <Button variant="primary" size="sm" onClick={() => setShowAddSkill(true)}>
            Add {label}
          </Button>
        )}
      </div>

      {validatedSkills.length === 0 ? (
        <div className="text-center py-8 text-content-secondary">
          <p>No {label.toLowerCase()} yet.</p>
          {canEdit && <p className="text-sm mt-1">Click "Add {label}" to get started.</p>}
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
        <AddSkillModal onAdd={addSkill} onCancel={() => setShowAddSkill(false)} />
      )}
    </div>
  );
};
