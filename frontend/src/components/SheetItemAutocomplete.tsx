import React, { useEffect, useRef } from 'react';
import type { SheetItem } from '../hooks/useCharacterSheetItems';
import { Badge } from './ui';

const TYPE_BADGE_VARIANT: Record<SheetItem['type'], 'success' | 'warning'> = {
  skill: 'success',
  item: 'warning',
};

interface SheetItemAutocompleteProps {
  items: SheetItem[];
  query: string;
  position: { top: number; left: number };
  onSelect: (item: SheetItem) => void;
  selectedIndex: number;
}

/**
 * SheetItemAutocomplete - dropdown for the %% trigger in CommentEditor.
 *
 * Shows all sheet items when query is empty; filters by name when the user
 * types after %%. Follows the same pattern as CharacterAutocomplete.
 */
export function SheetItemAutocomplete({
  items,
  query,
  position,
  onSelect,
  selectedIndex,
}: SheetItemAutocompleteProps) {
  const listRef = useRef<HTMLUListElement>(null);

  const filtered = query
    ? items.filter((item) => item.name.toLowerCase().includes(query.toLowerCase()))
    : items;

  useEffect(() => {
    if (listRef.current && selectedIndex >= 0) {
      const el = listRef.current.children[selectedIndex] as HTMLElement;
      if (el && typeof el.scrollIntoView === 'function') {
        el.scrollIntoView({ block: 'nearest' });
      }
    }
  }, [selectedIndex]);

  const style: React.CSSProperties = {
    position: 'fixed',
    top: `${position.top}px`,
    left: `${position.left}px`,
    minWidth: '220px',
    maxWidth: '320px',
  };

  if (filtered.length === 0) {
    return (
      <ul
        role="listbox"
        className="z-50 surface-base border border-theme-default rounded-md shadow-lg py-2 px-3 text-sm text-content-tertiary"
        style={style}
      >
        <li role="option" aria-selected={false}>No items found</li>
      </ul>
    );
  }

  return (
    <ul
      ref={listRef}
      role="listbox"
      className="z-50 surface-base border border-theme-default rounded-md shadow-lg py-1 max-h-56 overflow-y-auto"
      style={style}
    >
      {filtered.map((item, index) => (
        <li
          key={item.id}
          role="option"
          aria-selected={index === selectedIndex}
          onClick={() => onSelect(item)}
          className={`px-3 py-2 cursor-pointer flex items-center gap-2 text-sm transition-colors ${
            index === selectedIndex
              ? 'bg-interactive-primary-subtle text-interactive-primary'
              : 'hover:surface-raised text-content-primary'
          }`}
        >
          <Badge variant={TYPE_BADGE_VARIANT[item.type]} size="sm" className="shrink-0 capitalize">
            {item.type}
          </Badge>
          <span className="font-medium truncate">{item.name}</span>
        </li>
      ))}
    </ul>
  );
}
