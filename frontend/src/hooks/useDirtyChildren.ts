import { useCallback, useEffect, useRef, useState } from 'react';

/**
 * Aggregates unsaved-changes signals from a variable set of child editors.
 *
 * A manager renders one card per ability/item/etc., any of which can be mid-edit.
 * Tracking a single boolean would let one card's "clean" report erase another's
 * "dirty", so each child is tracked under its own key and the result is "any child
 * is dirty".
 *
 * Returns a stable `report` callback: passing it inline to a child is safe, and it
 * is what {@link useReportDirty} calls on unmount to drop a removed child's entry.
 */
export function useDirtyChildren(onDirtyChange?: (isDirty: boolean) => void) {
  const dirtyKeys = useRef(new Set<string>());
  const [isAnyDirty, setIsAnyDirty] = useState(false);

  const report = useCallback((key: string, isDirty: boolean) => {
    if (isDirty) {
      dirtyKeys.current.add(key);
    } else {
      dirtyKeys.current.delete(key);
    }
    // Derived from the set on every report, never from the incoming flag alone. When one
    // keyed child replaces another React mounts the newcomer before cleaning up the
    // departed one, so reports arrive out of order; recomputing from the set means a
    // late arrival still yields the correct answer for whoever is actually mounted.
    setIsAnyDirty(dirtyKeys.current.size > 0);
  }, []);

  // Synced in an effect rather than during render — see useReportDirty. Declared before
  // the reporting effect so it commits first and that effect sees the current callback.
  const callbackRef = useRef(onDirtyChange);
  useEffect(() => {
    callbackRef.current = onDirtyChange;
  });

  useEffect(() => {
    callbackRef.current?.(isAnyDirty);
  }, [isAnyDirty]);

  useEffect(() => {
    return () => callbackRef.current?.(false);
  }, []);

  return { isAnyDirty, report };
}
