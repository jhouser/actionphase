import type { InventoryItem } from '../types/characters';
import { Modal } from './Modal';
import { ItemForm, type ItemFormData, type lootModes } from './character-updates/ItemForm';

interface AddItemModalProps {
  onAdd: (item: Omit<InventoryItem, 'id'>) => void;
  onAddRandom: (lootTableId: number) => void;
  /**
   * Whether the "pick from / roll on a loot table" modes are offered. Requires a
   * game context (loot tables are game-scoped) and a caller that can act on
   * onAddRandom. Off by default so contexts that only add plain items — such as
   * defining the contents of a loot table itself — do not offer them.
   */
  allowedLootModes?: lootModes[];
  onCancel: () => void;
}

export const AddItemModal: React.FC<AddItemModalProps> = ({ onAdd, onAddRandom, allowedLootModes = ['manual'], onCancel }) => {
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
    });
  };

  return (
    // dismissOnBackdrop: a half-typed new item lives only in ItemForm's local state.
    // A stray backdrop click would discard it outright, so closing has to go through
    // the X or Cancel.
    <Modal isOpen={true} onClose={onCancel} title="Add New" dismissOnBackdrop={false}>
      <ItemForm
        onSubmit={handleSubmit}
        onCancel={onCancel}
        submitLabel="Add"
        variant="modal"
        allowedLootModes={allowedLootModes}
      />
    </Modal>
  );
};
