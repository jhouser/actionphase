import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import {
  useBodyScrollLock,
  useOverlayLockCount,
  __resetBodyScrollLockForTests,
} from '../useBodyScrollLock';

describe('useBodyScrollLock', () => {
  beforeEach(() => {
    __resetBodyScrollLockForTests();
    document.body.style.overflow = '';
  });

  afterEach(() => {
    __resetBodyScrollLockForTests();
    document.body.style.overflow = '';
  });

  it('locks body scroll while mounted and restores on unmount', () => {
    const { unmount } = renderHook(() => useBodyScrollLock());
    expect(document.body.style.overflow).toBe('hidden');

    unmount();
    expect(document.body.style.overflow).toBe('');
  });

  it('keeps the lock until the last of several overlays releases', () => {
    const outer = renderHook(() => useBodyScrollLock());
    const inner = renderHook(() => useBodyScrollLock());
    expect(document.body.style.overflow).toBe('hidden');

    // The inner overlay closing must not unlock the page behind the outer one
    // that is still on screen — the bug the ref count exists to prevent.
    inner.unmount();
    expect(document.body.style.overflow).toBe('hidden');

    outer.unmount();
    expect(document.body.style.overflow).toBe('');
  });

  it('restores the pre-lock overflow value, not one captured mid-stack', () => {
    document.body.style.overflow = 'scroll';

    const outer = renderHook(() => useBodyScrollLock());
    const inner = renderHook(() => useBodyScrollLock());

    // Unmount outer-first so the surviving hook is the one that acquired second
    // and never saw the original value.
    outer.unmount();
    inner.unmount();

    expect(document.body.style.overflow).toBe('scroll');
  });

  it('does not lock when inactive', () => {
    renderHook(() => useBodyScrollLock(false));
    expect(document.body.style.overflow).toBe('');
  });

  it('acquires and releases as active flips', () => {
    const { rerender } = renderHook(({ active }) => useBodyScrollLock(active), {
      initialProps: { active: false },
    });
    expect(document.body.style.overflow).toBe('');

    rerender({ active: true });
    expect(document.body.style.overflow).toBe('hidden');

    rerender({ active: false });
    expect(document.body.style.overflow).toBe('');
  });

  it('reports whether this caller holds a lock', () => {
    // Callers subtract their own lock from useOverlayLockCount() to tell
    // whether anyone *else* has one. That only works if "do I hold it" tracks
    // the effect rather than the `active` argument, which leads it by a render.
    const { result, rerender } = renderHook(
      ({ active }) => useBodyScrollLock(active),
      { initialProps: { active: false } }
    );
    expect(result.current).toBe(false);

    rerender({ active: true });
    expect(result.current).toBe(true);

    rerender({ active: false });
    expect(result.current).toBe(false);
  });
});

describe('useOverlayLockCount', () => {
  beforeEach(() => {
    __resetBodyScrollLockForTests();
    document.body.style.overflow = '';
  });

  afterEach(() => {
    __resetBodyScrollLockForTests();
    document.body.style.overflow = '';
  });

  it('reports how many overlays hold a lock', () => {
    const { result } = renderHook(() => useOverlayLockCount());
    expect(result.current).toBe(0);

    const first = renderHook(() => useBodyScrollLock());
    expect(result.current).toBe(1);

    const second = renderHook(() => useBodyScrollLock());
    expect(result.current).toBe(2);

    second.unmount();
    expect(result.current).toBe(1);

    first.unmount();
    expect(result.current).toBe(0);
  });

  it('re-renders subscribers when a lock is acquired after they mount', () => {
    // The drawer reads this count on the render *before* its own lock effect
    // runs, so it only styles itself correctly if the count pushes an update.
    const { result } = renderHook(() => useOverlayLockCount());
    expect(result.current).toBe(0);

    let overlay: ReturnType<typeof renderHook> | undefined;
    act(() => {
      overlay = renderHook(() => useBodyScrollLock());
    });

    expect(result.current).toBe(1);
    overlay?.unmount();
  });
});
