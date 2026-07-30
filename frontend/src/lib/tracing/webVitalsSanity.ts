import { TransportItemType, type TransportItem, type MeasurementEvent } from '@grafana/faro-web-sdk';

/**
 * A stalled/backgrounded navigation (observed on iOS Safari: the tab is
 * suspended mid-request, then resumes) freezes the browser's navigation
 * timing clock. web-vitals reports the resulting wall-clock gap as a huge
 * `ttfb`/`fcp`/`lcp` even though the server responded in milliseconds — the
 * time isn't attributable to any real network or server phase, just a
 * suspended tab. These values poison the small-sample aggregates in
 * Grafana's Page Performance table.
 *
 * There's no client-side signal that proves "suspended" (Chromium's
 * visibility-state entries would, but this app's offending traffic is
 * Safari, which doesn't emit them). So this is a calibrated policy ceiling,
 * not a physical detector: values below it are trusted outright, values
 * above it are assumed to be stall artifacts and dropped rather than
 * reported (a fake large sample would distort percentiles; a missing
 * sample does not).
 */

// No real navigation to this app (Go backend, sub-second queries) has a
// first byte this slow. responseStart this high means the load stalled,
// not that the server was slow — drop the whole navigation's web-vitals,
// since FCP/LCP measured from a stalled load are equally poisoned.
const TTFB_STALL_CEILING_MS = 10_000;

// Covers a stall that happens *after* a fine first byte (fetch completed,
// tab backgrounded before paint) — responseStart looks fine but fcp/lcp
// don't.
const PAINT_STALL_CEILING_MS = 20_000;

function getNavigationResponseStart(): number | undefined {
  const [entry] = performance.getEntriesByType('navigation') as PerformanceNavigationTiming[];
  return entry?.responseStart;
}

/**
 * `beforeSend` predicate: true if this item should be dropped before export.
 * Only inspects web-vitals measurements; every other item type (errors,
 * traces, logs, events, and other measurement types) passes through
 * untouched.
 */
export function shouldDropWebVital(
  item: TransportItem,
  getResponseStart: () => number | undefined = getNavigationResponseStart
): boolean {
  if (item.type !== TransportItemType.MEASUREMENT) {
    return false;
  }

  const payload = item.payload as MeasurementEvent;
  if (payload.type !== 'web-vitals') {
    return false;
  }

  const responseStart = getResponseStart();
  if (typeof responseStart === 'number' && responseStart > TTFB_STALL_CEILING_MS) {
    return true;
  }

  const { fcp, lcp } = payload.values;
  if (typeof fcp === 'number' && fcp > PAINT_STALL_CEILING_MS) {
    return true;
  }
  if (typeof lcp === 'number' && lcp > PAINT_STALL_CEILING_MS) {
    return true;
  }

  return false;
}
