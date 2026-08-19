import type { NumberEntry } from '../types/characters';
import { Modal } from './Modal';
import { NumberForm, type NumberFormData } from './character-updates/NumberForm';

interface AddNumberModalProps {
  onAdd: (entry: Omit<NumberEntry, 'id'>) => void;
  onCancel: () => void;
}

export const AddNumberModal: React.FC<AddNumberModalProps> = ({ onAdd, onCancel }) => {
  const handleSubmit = (data: NumberFormData) => {
    onAdd({
      name: data.name,
      amount: data.amount,
      max: data.max,
      display: data.display,
      description: data.description,
    });
  };

  return (
    // dismissOnBackdrop: see AddItemModal — a stray backdrop click must not discard
    // the half-typed entry held in NumberForm's local state.
    <Modal isOpen={true} onClose={onCancel} title="Add New" dismissOnBackdrop={false}>
      <NumberForm
        onSubmit={handleSubmit}
        onCancel={onCancel}
        submitLabel="Add"
        variant="modal"
      />
    </Modal>
  );
};
