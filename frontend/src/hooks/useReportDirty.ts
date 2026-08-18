import { useEffect, useRef } from 'react';

/**
 * Reports a nested editor's unsaved-changes state to an ancestor.
 *
 * The character sheet editors (item, ability, skill, currency) hold their edits in
 * local state and only hand them to the sheet on Save. A GM who types into one of
 * those forms and then closes the sheet — via "Done", the modal backdrop, or the
 * sheet's X — loses the edit with no warning. Ancestors use this signal to confirm
 * before closing.
 *
 * Guaranteed to report `false` on unmount, so a form that is cancelled, saved, or
 * removed never leaves a stale "dirty" flag stuck on its ancestor.
 */
export function useReportDirty(
  isDirty: boolean,
  onDirtyChange?: (isDirty: boolean) => void,
): void {
  // Keep the latest callback in a ref so an inline arrow function at the call site
  // doesn't retrigger the effect (and thus the unmount cleanup) on every render.
  //
  // Synced in an effect rather than during render: a render-phase write is a side
  // effect React is free to discard or replay. Declared first so it commits before the
  // reporting effect below, which therefore always sees the current callback.
  const callbackRef = useRef(onDirtyChange);
  useEffect(() => {
    callbackRef.current = onDirtyChange;
  });

  useEffect(() => {
    callbackRef.current?.(isDirty);
  }, [isDirty]);

  useEffect(() => {
    return () => callbackRef.current?.(false);
  }, []);
}
