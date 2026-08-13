import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react';
import type { Tab } from '../components/TabNavigation';

/**
 * Width reserved for the "More" button when it is going to be shown.
 *
 * Measured rather than assumed wherever possible — this constant is only the
 * first-paint fallback, used before the button exists in the DOM. It is
 * deliberately generous: overestimating hides one extra tab for a frame,
 * whereas underestimating lets the bar overflow and wrap, which is visible.
 */
const MORE_BUTTON_FALLBACK_PX = 92;

interface UseTabOverflowOptions {
  tabs: Tab[];
  activeTab: string;
  /** Skip all measurement (e.g. the caller renders a mobile <select> instead). */
  enabled?: boolean;
}

interface UseTabOverflowResult {
  /** Attach to the scrolling nav that holds the tabs. */
  containerRef: React.RefObject<HTMLDivElement | null>;
  /** Attach to the "More" button so its real width feeds the next measurement. */
  moreButtonRef: React.RefObject<HTMLButtonElement | null>;
  /** Call with each tab's element so its natural width can be recorded. */
  registerTab: (tabId: string, el: HTMLElement | null) => void;
  /** IDs that do not fit and belong in the More dropdown. */
  overflowIds: Set<string>;
  /**
   * False until the first measurement lands. The caller renders every tab
   * during this pass so widths exist to measure; it should keep the bar
   * invisible meanwhile to avoid a flash of all-tabs-then-collapse.
   */
  measured: boolean;
}

/**
 * useTabOverflow — decide which tabs fit the available width.
 *
 * Tabs are dropped last-to-first, so a tab's importance is expressed by its
 * position in the list: earlier survives longer. The active tab is always kept
 * visible, displacing a later-but-still-fitting tab if necessary, so the user
 * can always see which tab they are on without opening the dropdown.
 *
 * Widths are measured from the real rendered tabs rather than estimated from
 * label length, because padding, icons and badges all contribute and vary per
 * tab. That means the first paint must render everything; `measured` reports
 * when the real answer is ready.
 */
export function useTabOverflow({
  tabs,
  activeTab,
  enabled = true,
}: UseTabOverflowOptions): UseTabOverflowResult {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const moreButtonRef = useRef<HTMLButtonElement | null>(null);

  // Natural (unhidden) width of every tab, keyed by id. Persisted across
  // renders so a tab moved into the dropdown keeps the width it had in the bar
  // — once hidden it can no longer be measured.
  const widthsRef = useRef<Map<string, number>>(new Map());

  const [overflowIds, setOverflowIds] = useState<Set<string>>(() => new Set());
  const [measured, setMeasured] = useState(false);

  const registerTab = useCallback((tabId: string, el: HTMLElement | null) => {
    if (!el) return;
    const width = el.getBoundingClientRect().width;
    // A hidden tab measures 0; keep the last real width instead of clobbering it.
    if (width > 0) {
      widthsRef.current.set(tabId, width);
    }
  }, []);

  const recompute = useCallback(() => {
    const container = containerRef.current;
    if (!container) return;

    const available = container.getBoundingClientRect().width;
    // A collapsed/hidden container measures 0 (e.g. below the md breakpoint, or
    // while an ancestor is display:none). Measuring then would overflow every
    // tab; treat it as "no information yet" and keep the previous answer.
    if (available === 0) return;

    const widths = widthsRef.current;
    // Wait until every tab has contributed a width, otherwise the totals are
    // wrong and the bar visibly re-collapses a frame later.
    if (tabs.some(t => !widths.has(t.id))) return;

    const total = tabs.reduce((sum, t) => sum + (widths.get(t.id) ?? 0), 0);

    if (total <= available) {
      setOverflowIds(prev => (prev.size === 0 ? prev : new Set()));
      setMeasured(true);
      return;
    }

    // Everything does not fit, so a More button is certain — reserve its width.
    const moreWidth =
      moreButtonRef.current?.getBoundingClientRect().width || MORE_BUTTON_FALLBACK_PX;
    const budget = available - moreWidth;

    // The active tab is pinned, so it is charged against the budget up front and
    // then skipped in the fill loop below.
    const activeWidth = tabs.some(t => t.id === activeTab)
      ? (widths.get(activeTab) ?? 0)
      : 0;

    let used = activeWidth;
    const nextOverflow = new Set<string>();

    for (const tab of tabs) {
      if (tab.id === activeTab) continue; // already paid for
      const width = widths.get(tab.id) ?? 0;
      if (used + width <= budget) {
        used += width;
      } else {
        // Last-to-first: once one tab fails to fit, every later tab overflows
        // too. Continuing the loop (rather than breaking) would let a narrow
        // later tab leapfrog a wider earlier one and scramble the tab order.
        nextOverflow.add(tab.id);
      }
    }

    setOverflowIds(prev => {
      if (prev.size === nextOverflow.size && [...prev].every(id => nextOverflow.has(id))) {
        return prev; // identical — avoid a pointless re-render
      }
      return nextOverflow;
    });
    setMeasured(true);
  }, [tabs, activeTab]);

  // Measure synchronously after DOM mutations so the corrected layout paints in
  // the same frame the tabs do, rather than flashing the full list first.
  useLayoutEffect(() => {
    if (!enabled) {
      setOverflowIds(prev => (prev.size === 0 ? prev : new Set()));
      setMeasured(false);
      return;
    }
    recompute();
  }, [enabled, recompute]);

  // Re-measure on container resize (viewport changes, sidebar toggles, the
  // sticky bar condensing). ResizeObserver covers all of them; a window resize
  // listener would miss layout-driven changes at a constant viewport width.
  useEffect(() => {
    if (!enabled) return;
    const container = containerRef.current;
    if (!container || typeof ResizeObserver === 'undefined') return;

    const observer = new ResizeObserver(() => recompute());
    observer.observe(container);
    return () => observer.disconnect();
  }, [enabled, recompute]);

  // Tabs are removed from the width cache when they leave the list, so a tab id
  // reused later with a different label cannot inherit a stale width.
  useEffect(() => {
    const live = new Set(tabs.map(t => t.id));
    for (const id of widthsRef.current.keys()) {
      if (!live.has(id)) widthsRef.current.delete(id);
    }
  }, [tabs]);

  return { containerRef, moreButtonRef, registerTab, overflowIds, measured };
}
