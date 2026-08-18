import { useQuery } from '@tanstack/react-query';
import { useMemo } from 'react';
import { apiClient } from '../lib/api';
import type { CharacterSkill, InventoryItem } from '../types/characters';

export interface SheetItem {
  id: string;
  name: string;
  type: 'skill' | 'item';
  description?: string;
  /** Human-readable metadata: skill level/category, item category/quantity */
  metadata?: string;
}

function parseJsonField<T>(value: string | undefined): T[] {
  if (!value) return [];
  try {
    const parsed = JSON.parse(value);
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

function skillToSheetItem(s: CharacterSkill): SheetItem {
  const meta = [s.category, s.level !== null && s.level !== undefined ? `Level ${s.level}` : undefined]
    .filter(Boolean)
    .join(' · ');
  return {
    id: s.id,
    name: s.name,
    type: 'skill',
    description: s.description,
    metadata: meta || undefined,
  };
}

function itemToSheetItem(i: InventoryItem): SheetItem {
  const meta = [i.category, i.quantity > 1 ? `×${i.quantity}` : undefined]
    .filter(Boolean)
    .join(' · ');
  return {
    id: i.id,
    name: i.name,
    type: 'item',
    description: i.description,
    metadata: meta || undefined,
  };
}

export function useCharacterSheetItems(characterId: number | null): SheetItem[] {
  const { data } = useQuery({
    queryKey: ['characterData', characterId],
    queryFn: () =>
      apiClient.characters.getCharacterData(characterId!).then((r) => r.data),
    enabled: characterId !== null && characterId !== undefined,
    staleTime: 60_000,
  });

  return useMemo(() => {
    if (!data) return [];

    const getField = (moduleType: string, fieldName: string): string | undefined =>
      data.find((d) => d.module_type === moduleType && d.field_name === fieldName)?.field_value;

    const skills = parseJsonField<CharacterSkill>(getField('skills', 'skills'));
    const items = parseJsonField<InventoryItem>(getField('inventory', 'items'));

    return [
      ...skills.filter((s) => s.id && s.name).map(skillToSheetItem),
      ...items.filter((i) => i.id && i.name).map(itemToSheetItem),
    ];
  }, [data]);
}
