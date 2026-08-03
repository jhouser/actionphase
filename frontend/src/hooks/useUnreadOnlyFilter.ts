import { useState, useEffect } from 'react';
import { logger } from '@/services/LoggingService';

const STORAGE_KEY = 'newCommentsUnreadOnly';

/**
 * Manages the "Unread only" filter on the New Comments tab with localStorage
 * persistence, so the choice survives refreshes and return visits.
 *
 * State is keyed per game: a player who wants the filter on in a busy game
 * shouldn't have it silently applied to every other game they're in.
 *
 * Mirrors the persistence approach in usePostCollapseState.
 *
 * @param gameId - The game whose filter preference to track
 * @returns [unreadOnly, setUnreadOnly] - Tuple matching the useState API
 */
export function useUnreadOnlyFilter(
  gameId: number
): [boolean, (unreadOnly: boolean) => void] {
  const [unreadOnly, setUnreadOnly] = useState<boolean>(() => {
    try {
      const stored = localStorage.getItem(STORAGE_KEY);
      if (stored) {
        const parsed = JSON.parse(stored) as Record<string, boolean>;
        return parsed[gameId] ?? false;
      }
    } catch (error) {
      logger.error('Failed to read unread-only filter from localStorage:', { error });
    }
    return false;
  });

  // Re-read when switching games so the hook reflects the new game's preference
  // rather than carrying over the previous game's state.
  useEffect(() => {
    try {
      const stored = localStorage.getItem(STORAGE_KEY);
      const parsed = stored ? (JSON.parse(stored) as Record<string, boolean>) : {};
      setUnreadOnly(parsed[gameId] ?? false);
    } catch (error) {
      logger.error('Failed to read unread-only filter from localStorage:', { error });
      setUnreadOnly(false);
    }
  }, [gameId]);

  useEffect(() => {
    try {
      const stored = localStorage.getItem(STORAGE_KEY);
      const current: Record<string, boolean> = stored
        ? (JSON.parse(stored) as Record<string, boolean>)
        : {};
      current[gameId] = unreadOnly;
      localStorage.setItem(STORAGE_KEY, JSON.stringify(current));
    } catch (error) {
      logger.error('Failed to save unread-only filter to localStorage:', { error });
    }
  }, [gameId, unreadOnly]);

  return [unreadOnly, setUnreadOnly];
}
