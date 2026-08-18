import { describe, it, expect, vi } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useDirtyChildren } from '../useDirtyChildren';

describe('useDirtyChildren', () => {
  it('reports dirty when a child reports dirty', () => {
    const onDirtyChange = vi.fn();
    const { result } = renderHook(() => useDirtyChildren(onDirtyChange));

    act(() => result.current.report('item:1', true));

    expect(result.current.isAnyDirty).toBe(true);
    expect(onDirtyChange).toHaveBeenLastCalledWith(true);
  });

  it('reports clean once the only dirty child goes clean', () => {
    const onDirtyChange = vi.fn();
    const { result } = renderHook(() => useDirtyChildren(onDirtyChange));

    act(() => result.current.report('item:1', true));
    act(() => result.current.report('item:1', false));

    expect(result.current.isAnyDirty).toBe(false);
    expect(onDirtyChange).toHaveBeenLastCalledWith(false);
  });

  /**
   * The reason this hook tracks a key set instead of a single boolean. With one
   * shared flag, the second card's "clean" report would clear the first card's
   * still-unsaved edit and the ancestor would close without warning.
   */
  it('stays dirty while another child is still dirty', () => {
    const onDirtyChange = vi.fn();
    const { result } = renderHook(() => useDirtyChildren(onDirtyChange));

    act(() => result.current.report('item:1', true));
    act(() => result.current.report('item:2', true));
    act(() => result.current.report('item:2', false));

    expect(result.current.isAnyDirty).toBe(true);
    expect(onDirtyChange).toHaveBeenLastCalledWith(true);
  });

  it('reports clean on unmount so a removed subtree leaves no stale flag', () => {
    const onDirtyChange = vi.fn();
    const { result, unmount } = renderHook(() => useDirtyChildren(onDirtyChange));

    act(() => result.current.report('item:1', true));
    onDirtyChange.mockClear();

    unmount();

    expect(onDirtyChange).toHaveBeenCalledWith(false);
  });

  /**
   * Managers pass `onDirtyChange={(d) => reportDirty(key, d)}` — a new arrow every
   * render. Only a real change in aggregate dirtiness should reach the ancestor.
   */
  it('does not re-notify when only the callback identity changes', () => {
    const onDirtyChange = vi.fn();
    const { result, rerender } = renderHook(
      ({ cb }) => useDirtyChildren(cb),
      { initialProps: { cb: onDirtyChange } },
    );

    act(() => result.current.report('item:1', true));
    const callsAfterReport = onDirtyChange.mock.calls.length;

    // Re-render repeatedly with a fresh callback each time, as a manager does.
    for (let i = 0; i < 5; i++) {
      rerender({ cb: vi.fn() });
    }
    rerender({ cb: onDirtyChange });

    expect(onDirtyChange.mock.calls.length).toBe(callsAfterReport);
  });

  it('keeps a stable report identity across renders', () => {
    const { result, rerender } = renderHook(() => useDirtyChildren(vi.fn()));

    const first = result.current.report;
    rerender();

    expect(result.current.report).toBe(first);
  });
});
