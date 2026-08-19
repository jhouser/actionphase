import { useCallback } from 'react';
import type { FormEvent } from 'react';
import type { GameFormTabId } from '../components/gameFormTabs';
import { findFirstInvalidTab } from '../components/gameFormTabs';

/**
 * Reveals the tab holding an invalid `required` control before the browser
 * reports the failure.
 *
 * Inactive tab panels are hidden with `display: none` rather than unmounted, so
 * their fields still validate. But Chromium refuses to focus a control it cannot
 * show: it logs "An invalid form control ... is not focusable" and the submit
 * does nothing at all — no message, no indication anything went wrong. Measured
 * in Chromium; a hidden control behaves exactly like a detached one.
 *
 * The `invalid` event fires on each failing control *before* the submit is
 * cancelled, so switching tabs here makes the field visible in time for the
 * browser's own bubble to land on it. `invalid` does not bubble, hence the
 * capture-phase listener React gives us via `onInvalid` on the form.
 */
export function useRevealInvalidTab(setActiveTab: (tab: GameFormTabId) => void) {
  return useCallback(
    (e: FormEvent<HTMLFormElement>) => {
      const form = e.currentTarget;
      const tab = findFirstInvalidTab(form);
      if (tab) setActiveTab(tab);
    },
    [setActiveTab]
  );
}
