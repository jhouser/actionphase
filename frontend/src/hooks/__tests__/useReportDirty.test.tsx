import { describe, it, expect, vi } from 'vitest';
import { renderHook } from '@testing-library/react';
import { useReportDirty } from '../useReportDirty';

describe('useReportDirty', () => {
  it('reports the initial dirty state', () => {
    const onDirtyChange = vi.fn();
    renderHook(() => useReportDirty(false, onDirtyChange));

    expect(onDirtyChange).toHaveBeenCalledWith(false);
  });

  it('reports true once the form diverges from its initial values', () => {
    const onDirtyChange = vi.fn();
    const { rerender } = renderHook(
      ({ dirty }) => useReportDirty(dirty, onDirtyChange),
      { initialProps: { dirty: false } },
    );

    rerender({ dirty: true });

    expect(onDirtyChange).toHaveBeenLastCalledWith(true);
  });

  /**
   * Cancel and Save both unmount the form. Without this the ancestor would keep a
   * dirty flag for an editor that no longer exists and refuse to close.
   */
  it('reports clean on unmount even while dirty', () => {
    const onDirtyChange = vi.fn();
    const { unmount } = renderHook(() => useReportDirty(true, onDirtyChange));

    onDirtyChange.mockClear();
    unmount();

    expect(onDirtyChange).toHaveBeenCalledWith(false);
  });

  it('does not notify again when the dirty value is unchanged', () => {
    const onDirtyChange = vi.fn();
    const { rerender } = renderHook(() => useReportDirty(true, onDirtyChange));

    const callsAfterMount = onDirtyChange.mock.calls.length;
    rerender();
    rerender();

    expect(onDirtyChange.mock.calls.length).toBe(callsAfterMount);
  });

  // Callers pass a fresh arrow every render; only a real dirty-value change should
  // reach the ancestor. (Note: renderHook cannot reproduce a render feedback loop —
  // this asserts the notification contract, not loop-freedom.)
  it('does not re-notify when only the callback identity changes', () => {
    const onDirtyChange = vi.fn();
    const { rerender } = renderHook(
      ({ cb }) => useReportDirty(true, cb),
      { initialProps: { cb: onDirtyChange } },
    );

    const callsAfterMount = onDirtyChange.mock.calls.length;
    for (let i = 0; i < 5; i++) {
      rerender({ cb: vi.fn() });
    }
    rerender({ cb: onDirtyChange });

    expect(onDirtyChange.mock.calls.length).toBe(callsAfterMount);
  });

  it('tolerates an absent callback', () => {
    expect(() => {
      const { unmount } = renderHook(() => useReportDirty(true, undefined));
      unmount();
    }).not.toThrow();
  });
});
