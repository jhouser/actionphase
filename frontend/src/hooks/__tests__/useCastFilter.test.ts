import { describe, it, expect, beforeEach, vi } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useCastFilter } from '../useCastFilter';

const STORAGE_KEY = 'characterSheetCastFilter';

describe('useCastFilter', () => {
  beforeEach(() => {
    localStorage.clear();
    vi.restoreAllMocks();
  });

  it('starts on "all" so a GM sees the whole cast by default', () => {
    const { result } = renderHook(() => useCastFilter());
    expect(result.current[0]).toBe('all');
  });

  it('restores a stored preference on mount', () => {
    localStorage.setItem(STORAGE_KEY, 'mine');

    const { result } = renderHook(() => useCastFilter());

    expect(result.current[0]).toBe('mine');
  });

  it('persists a change so it survives a remount', () => {
    const { result, unmount } = renderHook(() => useCastFilter());

    act(() => result.current[1]('mine'));
    expect(result.current[0]).toBe('mine');

    // A fresh mount is what a page reload or reopened drawer looks like.
    unmount();
    const { result: remounted } = renderHook(() => useCastFilter());
    expect(remounted.current[0]).toBe('mine');
  });

  it('falls back to "all" on an unrecognized stored value', () => {
    // A hand-edited or stale entry must not wedge the picker into a filtered
    // state the GM never chose.
    localStorage.setItem(STORAGE_KEY, 'garbage');

    const { result } = renderHook(() => useCastFilter());

    expect(result.current[0]).toBe('all');
  });

  it('still works when localStorage reads throw', () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('denied');
    });

    const { result } = renderHook(() => useCastFilter());

    // Private-browsing modes can deny storage; the picker must still render.
    expect(result.current[0]).toBe('all');
  });

  it('still updates state when localStorage writes throw', () => {
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('quota');
    });

    const { result } = renderHook(() => useCastFilter());
    act(() => result.current[1]('mine'));

    // The toggle has to work for this session even if it can't be remembered.
    expect(result.current[0]).toBe('mine');
  });
});
