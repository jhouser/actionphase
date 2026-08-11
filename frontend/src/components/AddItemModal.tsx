import type { InventoryItem } from '../types/characters';
import { Modal } from './Modal';
import { ItemForm, type ItemFormData } from './character-updates/ItemForm';

interface AddItemModalProps {
  onAdd: (item: Omit<InventoryItem, 'id'>) => void;
  onAddRandom: (lootTableId: number) => void;
  onCancel: () => void;
}

export const AddItemModal: React.FC<AddItemModalProps> = ({ onAdd, onAddRandom, onCancel }) => {
  const handleSubmit = (data: ItemFormData) => {
    if (data.lootTableId) {
      onAddRandom(data.lootTableId);
      return;
    }
    onAdd({
      name: data.name,
      description: data.description,
      quantity: data.quantity,
      category: data.category,
      value: data.value,
      weight: data.weight,
      equipped: false
    });
  };

  return (
    <Modal isOpen={true} onClose={onCancel} title="Add New Item">
      <ItemForm
        onSubmit={handleSubmit}
        onCancel={onCancel}
        submitLabel="Add Item"
        variant="modal"
      />
    </Modal>
  );
};
