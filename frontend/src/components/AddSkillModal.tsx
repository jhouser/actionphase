import type { CharacterSkill } from '../types/characters';
import { Modal } from './Modal';
import { SkillForm, type SkillFormData } from './character-updates/SkillForm';

interface AddSkillModalProps {
  onAdd: (skill: Omit<CharacterSkill, 'id'>) => void;
  onCancel: () => void;
}

export const AddSkillModal: React.FC<AddSkillModalProps> = ({ onAdd, onCancel }) => {
  const handleSubmit = (data: SkillFormData) => {
    onAdd({
      name: data.name,
      rank: data.rank,
      description: data.description,
      category: data.category
    });
  };

  return (
    // dismissOnBackdrop: see AddItemModal — a stray backdrop click must not discard
    // the half-typed skill held in SkillForm's local state.
    <Modal isOpen={true} onClose={onCancel} title="Add New Skill" dismissOnBackdrop={false}>
      <SkillForm
        onSubmit={handleSubmit}
        onCancel={onCancel}
        submitLabel="Add Skill"
        variant="modal"
      />
    </Modal>
  );
};
