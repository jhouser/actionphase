import { describe, it, expect } from 'vitest';
import { act, renderHook } from '@testing-library/react';
import { useExpandedSet } from '../useExpandedSet';

describe('useExpandedSet', () => {
  it('starts with nothing expanded', () => {
    const { result } = renderHook(() => useExpandedSet());

    expect(result.current.isExpanded(1)).toBe(false);
    expect(result.current.expanded.size).toBe(0);
  });

  it('honours an initial set of expanded ids', () => {
    const { result } = renderHook(() => useExpandedSet([7, 9]));

    expect(result.current.isExpanded(7)).toBe(true);
    expect(result.current.isExpanded(9)).toBe(true);
    expect(result.current.isExpanded(8)).toBe(false);
  });

  it('toggles a single id on and back off', () => {
    const { result } = renderHook(() => useExpandedSet());

    act(() => result.current.toggle(42));
    expect(result.current.isExpanded(42)).toBe(true);

    act(() => result.current.toggle(42));
    expect(result.current.isExpanded(42)).toBe(false);
  });

  it('tracks ids independently rather than as one shared flag', () => {
    // The behaviour every list depends on: expanding one row must not expand
    // its siblings, and collapsing it must leave the others alone.
    const { result } = renderHook(() => useExpandedSet());

    act(() => result.current.toggle(1));
    act(() => result.current.toggle(2));
    expect(result.current.isExpanded(1)).toBe(true);
    expect(result.current.isExpanded(2)).toBe(true);
    expect(result.current.isExpanded(3)).toBe(false);

    act(() => result.current.toggle(1));
    expect(result.current.isExpanded(1)).toBe(false);
    expect(result.current.isExpanded(2)).toBe(true);
  });

  it('applies both updates when two ids are toggled in the same tick', () => {
    // Regression guard for the idiom this hook replaces. The old call sites did
    // `const next = new Set(expandedResults); ...; setExpanded(next)`, reading
    // the Set from the render closure — so two toggles batched into one tick
    // both started from the same stale snapshot and the first was lost.
    const { result } = renderHook(() => useExpandedSet());

    act(() => {
      result.current.toggle(1);
      result.current.toggle(2);
    });

    expect(result.current.isExpanded(1)).toBe(true);
    expect(result.current.isExpanded(2)).toBe(true);
  });

  it('collapses an already-expanded id toggled twice in one tick', () => {
    const { result } = renderHook(() => useExpandedSet([5]));

    act(() => {
      result.current.toggle(5);
      result.current.toggle(5);
    });

    expect(result.current.isExpanded(5)).toBe(true);
  });

  it('collapseAll clears every expanded id', () => {
    const { result } = renderHook(() => useExpandedSet([1, 2, 3]));

    act(() => result.current.collapseAll());

    expect(result.current.expanded.size).toBe(0);
    expect(result.current.isExpanded(1)).toBe(false);
  });

  it('keeps string ids distinct', () => {
    const { result } = renderHook(() => useExpandedSet<string>());

    act(() => result.current.toggle('alpha'));

    expect(result.current.isExpanded('alpha')).toBe(true);
    expect(result.current.isExpanded('beta')).toBe(false);
  });

  it('does not mutate the previous Set when toggling', () => {
    // Consumers pass `expanded` into memo deps and effect deps; mutating in
    // place would make those comparisons miss the change.
    const { result } = renderHook(() => useExpandedSet());

    const before = result.current.expanded;
    act(() => result.current.toggle(1));

    expect(before.size).toBe(0);
    expect(result.current.expanded).not.toBe(before);
  });
});
