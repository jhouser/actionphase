import { useEffect, useState } from 'react';
import { Button, Input } from '../ui';
import { Modal } from '../Modal';
import type { CreateLootTableRequest, LootTable, LootTableContent } from '@/types/games';
import { AddItemModal } from '../AddItemModal';
import type { InventoryItem } from '@/types/characters';
import { useQuery } from '@tanstack/react-query';
import { apiClient } from '@/lib/api';
import { useOptionalGameContext } from '@/contexts/GameContext';


interface LootTableFormProps {
  onClose: () => void;
  onSubmit: (data: CreateLootTableRequest) => void;
  isSubmitting: boolean;
  lootTable?: LootTable;
}

export function LootTableForm({ onClose, onSubmit, isSubmitting, lootTable }: LootTableFormProps) {
  const gameContext = useOptionalGameContext();

  const { data: lootTableContents } = useQuery({
    queryKey: ['lootTableContents', lootTable?.id],
    queryFn: () => apiClient.games.getLootTableContents(gameContext!.gameId, lootTable?.id!).then(res => res.data),
    enabled: !!lootTable?.id
  });

  useEffect(() => {
    setFormData({
      name: formData?.name || '',
      items: lootTableContents || undefined,
      file: formData?.file || undefined
    });
  }, [lootTableContents]);

  const [formData, setFormData] = useState<CreateLootTableRequest>({
    name: lootTable?.name || '',
    items:  undefined,
    file: undefined
  });
  const [isAddingContent, setIsAddingContent] = useState(false);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    onSubmit(formData);
  };

  
  const addItem = (itemData: Omit<InventoryItem, 'id'>) => {
    let newContent : LootTableContent = {
      id: 0,
      name: itemData.name,
      data: JSON.stringify(itemData),
    }
    setFormData(p => ({...p, items: [...(p.items || []), newContent]}));
    setIsAddingContent(false);
  };

  return (
    <div>
      <form onSubmit={handleSubmit}>
        <div className="space-y-4">

            <div>
              <Input
                id="loot-table-name"
                label="Table Name"
                type="text"
                value={formData.name || ''}
                onChange={(e) => setFormData(prev => ({
                  ...prev,
                  name: e.target.value
                }))}
                placeholder="e.g., 'Normal Items'"
                helperText="Give this loot table a custom name"
              />
            </div>
            <div >
              {formData.items && formData.items.length > 0 
                ? (formData.items.map((item, index) => (<div className="block text-sm font-medium text-content-primary mb-2" key={index}>{index + 1} - {item.name}</div>))) 
                : (<div></div>)}
            </div>

              <div>
                <Button
                  variant="primary"
                  type="button"
                  disabled={isSubmitting}
                  onClick={() => setIsAddingContent(true)}
                >
                  Add Loot Table Content
                </Button>
            </div>
            

        </div>


        <div className="flex justify-end space-x-3 mt-6">
          <Button
            type="button"
            variant="ghost"
            onClick={onClose}
          >
            Cancel
          </Button>
          <Button
            type="submit"
            variant="primary"
            disabled={isSubmitting}
            data-faro-user-action-name="create-loot-table"
          >
            {isSubmitting 
              ? !!lootTable ? 'Updating...' : 'Creating...' 
              : !!lootTable ? 'Update Loot Table' : 'Create Loot Table'}
          </Button>
        </div>
      </form>

      {/* Add Loot Table Content Modal */}
      {isAddingContent && (
        <AddItemModal
          onAdd={addItem}
          onCancel={() => {setIsAddingContent(false)}}
        />
      )}
    </div>
  );
}
