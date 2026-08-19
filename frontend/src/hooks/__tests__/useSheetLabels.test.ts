import { describe, it, expect } from 'vitest';
import { renderHook } from '@testing-library/react';
import { useSheetLabels, resolveSheetLabels, DEFAULT_SHEET_LABELS } from '../useSheetLabels';

describe('resolveSheetLabels', () => {
  it('returns every default when the game has no config at all', () => {
    // The common case by a wide margin: the backend omits the key entirely for
    // any game whose GM never renamed a tab.
    expect(resolveSheetLabels(undefined)).toEqual({
      skills: 'Skills',
      inventory: 'Inventory',
      numbers: 'Numbers',
    });
  });

  it('applies defaults per key, not all-or-nothing', () => {
    // The config is sparse: overriding one tab must not blank the other two.
    expect(resolveSheetLabels({ labels: { numbers: 'Stress' } })).toEqual({
      skills: 'Skills',
      inventory: 'Inventory',
      numbers: 'Stress',
    });
  });

  it('overrides every label when the GM renamed all three', () => {
    expect(
      resolveSheetLabels({
        labels: { skills: 'Playbook', inventory: 'Load', numbers: 'Stress' },
      })
    ).toEqual({ skills: 'Playbook', inventory: 'Load', numbers: 'Stress' });
  });

  it('falls back to the default for a whitespace-only label', () => {
    // The backend strips these on the way in rather than storing them, so this
    // should not fire for anything written through the API — it is a floor for
    // hand-edited rows. A tab whose name renders blank cannot be pointed at at
    // all, which is strictly worse than showing the default.
    expect(resolveSheetLabels({ labels: { skills: '   ' } }).skills).toBe('Skills');
  });

  it('trims surrounding whitespace off a real override', () => {
    expect(resolveSheetLabels({ labels: { skills: '  Playbook  ' } }).skills).toBe('Playbook');
  });

  it('treats an empty labels object as no overrides', () => {
    expect(resolveSheetLabels({ labels: {} })).toEqual({
      skills: 'Skills',
      inventory: 'Inventory',
      numbers: 'Numbers',
    });
  });

  it('exposes the defaults it applies', () => {
    // Pins that DEFAULT_SHEET_LABELS is genuinely the source the resolver reads,
    // rather than a second copy that happens to agree today.
    expect(resolveSheetLabels(undefined)).toEqual({ ...DEFAULT_SHEET_LABELS });
  });

  it('keeps each key identical to its own default label', () => {
    // The refactor's invariant: storage key == React symbol == default label.
    // Breaking it is what would force this hook to grow a translation table.
    for (const [key, label] of Object.entries(DEFAULT_SHEET_LABELS)) {
      expect(label.toLowerCase()).toBe(key);
    }
  });
});

describe('useSheetLabels', () => {
  it('reads the config off a game-shaped source', () => {
    const { result } = renderHook(() =>
      useSheetLabels({ character_sheet: { labels: { inventory: 'Load' } } })
    );
    expect(result.current.inventory).toBe('Load');
    expect(result.current.skills).toBe('Skills');
  });

  it('yields defaults for a game that has not loaded yet', () => {
    // Callers pass gameContext?.game, which is undefined on first render and
    // null while loading. Neither may throw.
    expect(renderHook(() => useSheetLabels(undefined)).result.current.skills).toBe('Skills');
    expect(renderHook(() => useSheetLabels(null)).result.current.skills).toBe('Skills');
  });

  it('keeps a stable object identity across re-renders with the same config', () => {
    // The result feeds tab construction in Phase 4; a new object every render
    // would defeat memoization downstream.
    const config = { labels: { skills: 'Playbook' } };
    const { result, rerender } = renderHook(() => useSheetLabels({ character_sheet: config }));
    const first = result.current;
    rerender();
    expect(result.current).toBe(first);
  });
});
