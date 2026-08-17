import { useCallback, useState } from 'react';

/**
 * Tracks which items in a list are expanded, keyed by id.
 *
 * Replaces the `useState<Set<number>>(new Set())` + hand-rolled toggle that was
 * duplicated across every list rendering collapsible content. Updates use the
 * functional setState form, so two toggles dispatched in the same tick can't
 * clobber each other the way a closed-over `new Set(expanded)` would.
 *
 * @example
 * const results = useExpandedSet();
 * <button onClick={() => results.toggle(result.id)}>
 *   {results.isExpanded(result.id) ? 'Show less' : 'Show full content'}
 * </button>
 */
export function useExpandedSet<T = number>(initial?: Iterable<T>) {
  const [expanded, setExpanded] = useState<Set<T>>(() => new Set(initial));

  const toggle = useCallback((id: T) => {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }, []);

  const isExpanded = useCallback((id: T) => expanded.has(id), [expanded]);

  const collapseAll = useCallback(() => {
    setExpanded((prev) => (prev.size === 0 ? prev : new Set()));
  }, []);

  return { expanded, toggle, isExpanded, collapseAll };
}
