import { useQuery } from '@tanstack/react-query';
import { useMemo } from 'react';
import { apiClient } from '../lib/api';
import type { CharacterData, CharacterSkill, InventoryItem } from '../types/characters';
import { skillRank } from '../types/characters';

export interface SheetItem {
  id: string;
  name: string;
  type: 'skill' | 'item';
  description?: string;
  /** Human-readable metadata: skill rank/category, item category/quantity */
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
  // Via skillRank so mention metadata reads the same value the card shows,
  // including for rows still holding the pre-rename `level` key.
  const rank = skillRank(s);
  const meta = [s.category, rank ? `Rank ${rank}` : undefined]
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

/**
 * Turn one character's raw sheet rows into the flat item list the [[ref]]
 * tooltips resolve against.
 *
 * Rows the caller may not see are absent from the payload -- the backend has
 * already filtered them -- so anything reaching this function is showable.
 */
function toSheetItems(data: CharacterData[] | undefined): SheetItem[] {
  if (!data) return [];

  const getField = (moduleType: string, fieldName: string): string | undefined =>
    data.find((d) => d.module_type === moduleType && d.field_name === fieldName)?.field_value;

  const skills = parseJsonField<CharacterSkill>(getField('skills', 'skills'));
  const items = parseJsonField<InventoryItem>(getField('inventory', 'items'));

  return [
    ...skills.filter((s) => s.id && s.name).map(skillToSheetItem),
    ...items.filter((i) => i.id && i.name).map(itemToSheetItem),
  ];
}

export function useCharacterSheetItems(characterId: number | null): SheetItem[] {
  const { data } = useQuery({
    queryKey: ['characterData', characterId],
    queryFn: () =>
      apiClient.characters.getCharacterData(characterId!).then((r) => r.data),
    enabled: characterId !== null && characterId !== undefined,
    staleTime: 60_000,
  });

  return useMemo(() => toSheetItems(data), [data]);
}

/**
 * Sheet items for every character in a game, keyed by character id.
 *
 * One request for the whole cast, replacing a call to useCharacterSheetItems
 * per rendered character. A phase drill-down in History can show many
 * characters' action content at once, and the per-character hook meant that
 * opening a phase fired a burst of simultaneous requests.
 *
 * Visibility is entirely the backend's: each character's rows arrive already
 * reduced to what this caller may see (their own sheets in a live game; the
 * whole cast for a GM, an audience member, or a public archive). A character
 * with nothing visible simply yields an empty list, which renders as a plain
 * highlight with no tooltip.
 */
export function useGameCharacterSheetItems(gameId: number | null): Map<number, SheetItem[]> {
  const { data } = useQuery({
    queryKey: ['gameCharacterData', gameId],
    queryFn: () =>
      apiClient.characters.getGameCharacterData(gameId!).then((r) => r.data),
    enabled: gameId !== null && gameId !== undefined,
    staleTime: 60_000,
  });

  return useMemo(() => {
    const byCharacter = new Map<number, SheetItem[]>();
    if (!data) return byCharacter;

    for (const [characterId, rows] of Object.entries(data)) {
      const id = Number(characterId);
      if (Number.isNaN(id)) continue;
      byCharacter.set(id, toSheetItems(rows));
    }
    return byCharacter;
  }, [data]);
}
