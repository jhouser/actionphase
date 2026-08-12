import type { InventoryItem } from '../types/characters';
import { Modal } from './Modal';
import { ItemForm, type ItemFormData } from './character-updates/ItemForm';

interface AddItemModalProps {
  onAdd: (item: Omit<InventoryItem, 'id'>) => void;
  onAddRandom: (lootTableId: number) => void;
  /**
   * Whether the "pick from / roll on a loot table" modes are offered. Requires a
   * game context (loot tables are game-scoped) and a caller that can act on
   * onAddRandom. Off by default so contexts that only add plain items — such as
   * defining the contents of a loot table itself — do not offer them.
   */
  allowLootModes?: boolean;
  onCancel: () => void;
}

export const AddItemModal: React.FC<AddItemModalProps> = ({ onAdd, onAddRandom, allowLootModes = false, onCancel }) => {
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
        allowLootModes={allowLootModes}
      />
    </Modal>
  );
};
