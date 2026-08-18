import { useState, useMemo } from 'react';
import type { SheetItem } from '../hooks/useCharacterSheetItems';
import { Badge, Input } from './ui';

interface SheetPanelProps {
  items: SheetItem[];
  onInsert: (item: SheetItem) => void;
  characterName?: string;
}

const TYPE_LABELS: Record<SheetItem['type'], string> = {
  skill: 'Skills',
  item: 'Inventory',
};

const TYPE_ORDER: SheetItem['type'][] = ['skill', 'item'];

const TYPE_BADGE_VARIANT: Record<SheetItem['type'], 'success' | 'warning'> = {
  skill: 'success',
  item: 'warning',
};

export function SheetPanel({ items, onInsert, characterName }: SheetPanelProps) {
  const [filter, setFilter] = useState('');

  const filtered = useMemo(() => {
    const q = filter.toLowerCase().trim();
    return q ? items.filter((i) => i.name.toLowerCase().includes(q)) : items;
  }, [items, filter]);

  const grouped = useMemo(() => {
    const map = new Map<SheetItem['type'], SheetItem[]>();
    for (const type of TYPE_ORDER) map.set(type, []);
    for (const item of filtered) {
      map.get(item.type)?.push(item);
    }
    return map;
  }, [filtered]);

  const hasAny = filtered.length > 0;

  return (
    <div className="flex flex-col h-full">
      {characterName && (
        <p className="px-4 pt-3 pb-1 text-xs text-content-secondary truncate">{characterName}</p>
      )}
      <div className="px-3 py-2 shrink-0">
        <Input
          placeholder="Filter…"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          aria-label="Filter sheet items"
        />
      </div>

      <div className="flex-1 overflow-y-auto px-2 pb-3">
        {!hasAny && (
          <p className="text-sm text-content-secondary text-center py-6">
            {filter ? 'No items match your filter.' : 'No items on this character\'s sheet yet.'}
          </p>
        )}

        {TYPE_ORDER.map((type) => {
          const group = grouped.get(type) ?? [];
          if (group.length === 0) return null;
          return (
            <section key={type} className="mb-4">
              <h3 className="px-2 py-1 text-xs font-semibold uppercase tracking-wide text-content-secondary">
                {TYPE_LABELS[type]}
              </h3>
              <ul className="space-y-1">
                {group.map((item) => (
                  <li key={item.id}>
                    <button
                      type="button"
                      onClick={() => onInsert(item)}
                      className="w-full text-left px-2 py-2 rounded-md hover:surface-raised transition-colors group"
                    >
                      <div className="flex items-start gap-2">
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center gap-1.5 flex-wrap">
                            <span className="text-sm font-medium text-content-primary group-hover:text-interactive-primary truncate">
                              {item.name}
                            </span>
                            {item.metadata && (
                              <Badge variant={TYPE_BADGE_VARIANT[type]} size="sm">
                                {item.metadata}
                              </Badge>
                            )}
                          </div>
                        </div>
                      </div>
                    </button>
                  </li>
                ))}
              </ul>
            </section>
          );
        })}
      </div>
    </div>
  );
}
