import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useUnreadOnlyFilter } from '../useUnreadOnlyFilter';

const STORAGE_KEY = 'newCommentsUnreadOnly';

describe('useUnreadOnlyFilter', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('defaults to off when nothing is stored', () => {
    const { result } = renderHook(() => useUnreadOnlyFilter(1));

    expect(result.current[0]).toBe(false);
  });

  it('persists the enabled filter to localStorage', () => {
    const { result } = renderHook(() => useUnreadOnlyFilter(1));

    act(() => result.current[1](true));

    expect(result.current[0]).toBe(true);
    expect(JSON.parse(localStorage.getItem(STORAGE_KEY)!)).toEqual({ 1: true });
  });

  it('restores the stored value on remount (survives a refresh)', () => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ 7: true }));

    const { result } = renderHook(() => useUnreadOnlyFilter(7));

    expect(result.current[0]).toBe(true);
  });

  it('keeps preferences separate per game', () => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ 1: true }));

    const gameOne = renderHook(() => useUnreadOnlyFilter(1));
    const gameTwo = renderHook(() => useUnreadOnlyFilter(2));

    expect(gameOne.result.current[0]).toBe(true);
    expect(gameTwo.result.current[0]).toBe(false);

    // Enabling game 2 must not disturb game 1's stored preference
    act(() => gameTwo.result.current[1](true));
    expect(JSON.parse(localStorage.getItem(STORAGE_KEY)!)).toEqual({ 1: true, 2: true });
  });

  it('re-reads the stored value when switching games', () => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ 1: true, 2: false }));

    const { result, rerender } = renderHook(({ gameId }) => useUnreadOnlyFilter(gameId), {
      initialProps: { gameId: 1 },
    });
    expect(result.current[0]).toBe(true);

    rerender({ gameId: 2 });

    expect(result.current[0]).toBe(false);
  });

  it('falls back to off when stored data is corrupt', () => {
    localStorage.setItem(STORAGE_KEY, 'not json');

    const { result } = renderHook(() => useUnreadOnlyFilter(1));

    expect(result.current[0]).toBe(false);
  });

  it('still works when localStorage writes throw (e.g. quota exceeded)', () => {
    const setItem = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('QuotaExceededError');
    });

    const { result } = renderHook(() => useUnreadOnlyFilter(1));
    act(() => result.current[1](true));

    // In-memory state still updates so the UI stays responsive
    expect(result.current[0]).toBe(true);
    setItem.mockRestore();
  });
});
