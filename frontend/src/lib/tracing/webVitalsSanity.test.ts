import { describe, it, expect } from 'vitest';
import { TransportItemType, type TransportItem, type MeasurementEvent } from '@grafana/faro-web-sdk';
import { shouldDropWebVital } from './webVitalsSanity';

function makeWebVitalItem(values: Record<string, number>): TransportItem {
  const payload: MeasurementEvent = {
    type: 'web-vitals',
    values,
    timestamp: '2026-07-10T18:21:06.170Z',
  };
  return {
    type: TransportItemType.MEASUREMENT,
    payload,
    meta: {},
  } as unknown as TransportItem;
}

describe('shouldDropWebVital', () => {
  it('drops a ttfb sample from a stalled navigation (real 19.1s iOS Safari sample)', () => {
    const item = makeWebVitalItem({
      delta: 19164,
      request_duration: 19154,
      ttfb: 19164,
      waiting_duration: 10,
    });

    expect(shouldDropWebVital(item, () => 19164)).toBe(true);
  });

  it('keeps a genuinely slow-but-real ttfb sample (real 1.3s borderline sample)', () => {
    const item = makeWebVitalItem({
      cache_duration: 106,
      connection_duration: 107,
      delta: 1312,
      dns_duration: 7,
      request_duration: 1082,
      ttfb: 1312,
      waiting_duration: 10,
    });

    expect(shouldDropWebVital(item, () => 1312)).toBe(false);
  });

  it('keeps a normal fast ttfb sample', () => {
    const item = makeWebVitalItem({ ttfb: 572 });

    expect(shouldDropWebVital(item, () => 572)).toBe(false);
  });

  it('drops an fcp/lcp sample from a post-fetch stall even when responseStart looks fine', () => {
    const item = makeWebVitalItem({ fcp: 25000 });

    expect(shouldDropWebVital(item, () => 500)).toBe(true);
  });

  it('drops an lcp sample above the paint stall ceiling', () => {
    const item = makeWebVitalItem({ lcp: 30000 });

    expect(shouldDropWebVital(item, () => 500)).toBe(true);
  });

  it('keeps a paint metric at exactly a normal magnitude', () => {
    const item = makeWebVitalItem({ fcp: 1290, lcp: 1920 });

    expect(shouldDropWebVital(item, () => 500)).toBe(false);
  });

  it('does not drop non-web-vitals measurements', () => {
    const payload: MeasurementEvent = {
      type: 'custom',
      values: { ttfb: 50000 },
      timestamp: '2026-07-10T18:21:06.170Z',
    };
    const item = {
      type: TransportItemType.MEASUREMENT,
      payload,
      meta: {},
    } as unknown as TransportItem;

    expect(shouldDropWebVital(item, () => 50000)).toBe(false);
  });

  it('does not drop non-measurement items (traces, exceptions, logs, events)', () => {
    const item = {
      type: TransportItemType.EXCEPTION,
      payload: {},
      meta: {},
    } as unknown as TransportItem;

    expect(shouldDropWebVital(item, () => 999999)).toBe(false);
  });

  it('falls back to values.fcp/lcp when navigation timing is unavailable', () => {
    const item = makeWebVitalItem({ fcp: 500 });

    expect(shouldDropWebVital(item, () => undefined)).toBe(false);
  });

  it('uses the real navigation timing API when no override is supplied (jsdom has no navigation entry)', () => {
    // Exercises the default getResponseStart param (getNavigationResponseStart),
    // not the injected mock the other cases use. jsdom doesn't populate
    // performance.getEntriesByType('navigation'), so responseStart resolves to
    // undefined here — this asserts that path degrades to the values.fcp/lcp
    // check rather than throwing.
    const item = makeWebVitalItem({ ttfb: 200 });

    expect(shouldDropWebVital(item)).toBe(false);
  });
});
