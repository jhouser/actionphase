import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Select } from '../ui';
import { apiClient } from '@/lib/api';
import type { LootTableContent } from '@/types/games';

interface LootTableSelectorProps {
  gameId: number;
  /** Also pick a specific item, rather than only a table to roll on. */
  requireItem: boolean;
  lootTableId: number | null;
  onLootTableChange: (lootTableId: number | null) => void;
  onItemChange: (item: LootTableContent | null) => void;
}

/**
 * Loot table (and optionally item) pickers for ItemForm.
 *
 * Split out of ItemForm deliberately: both endpoints are GM-only and this is the
 * only part of the form that needs React Query. Keeping the hooks here means
 * ItemForm renders without a QueryClientProvider and issues no GM-only requests
 * for the common case of adding a plain item.
 */
export function LootTableSelector({
  gameId,
  requireItem,
  lootTableId,
  onLootTableChange,
  onItemChange,
}: LootTableSelectorProps) {
  const [selectedItemId, setSelectedItemId] = useState<number | null>(null);

  const { data: lootTables } = useQuery({
    queryKey: ['lootTables', gameId],
    queryFn: () => apiClient.games.getLootTables(gameId).then((res) => res.data),
  });

  const { data: lootTableContents } = useQuery({
    queryKey: ['lootTableContents', lootTableId],
    queryFn: () =>
      apiClient.games.getLootTableContents(gameId, lootTableId!).then((res) => res.data),
    enabled: requireItem && !!lootTableId,
  });

  return (
    <div>
      <Select
        id="item-loot-table"
        label="Loot Table"
        value={lootTableId ?? ''}
        onChange={(e) => {
          onLootTableChange(parseInt(e.target.value) || null);
          setSelectedItemId(null);
        }}
        required
      >
        <option value="">Select a loot table</option>
        {lootTables?.map((table) => (
          <option key={table.id} value={table.id}>
            {table.name}
          </option>
        ))}
      </Select>

      {requireItem && (
        <Select
          id="item-loot-table-content"
          label="Loot Table Content"
          value={(lootTableContents?.length ?? 0) > 0 ? (selectedItemId ?? '') : ''}
          onChange={(e) => {
            const id = parseInt(e.target.value) || null;
            setSelectedItemId(id);
            onItemChange(lootTableContents?.find((c) => c.id === id) ?? null);
          }}
          required
        >
          <option value="">Select content</option>
          {lootTableContents?.map((content) => (
            <option key={content.id} value={content.id}>
              {content.name}
            </option>
          ))}
        </Select>
      )}
    </div>
  );
}
