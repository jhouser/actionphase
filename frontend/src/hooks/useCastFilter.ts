import { useCallback, useState } from 'react';
import { logger } from '@/services/LoggingService';

const STORAGE_KEY = 'characterSheetCastFilter';

/** Which slice of a game's cast the character-sheet picker is showing. */
export type CastFilter = 'all' | 'mine';

/**
 * Manages the All / Mine filter on the Utility Drawer's character-sheet picker,
 * persisted to localStorage so a GM's choice survives refreshes and return
 * visits.
 *
 * Only GMs and co-GMs ever see the control — everyone else's list is already
 * just the characters they control, so there is nothing to filter. The
 * preference is stored globally rather than per game: it expresses how the GM
 * likes to work, and in practice they run one game at a time.
 *
 * Defaults to 'all', matching the design intent that the GM's list is a cast
 * reference for the game they're running.
 *
 * Mirrors the persistence approach in useUnreadOnlyFilter.
 */
export function useCastFilter(): [CastFilter, (filter: CastFilter) => void] {
  const [filter, setFilterState] = useState<CastFilter>(() => {
    try {
      const stored = localStorage.getItem(STORAGE_KEY);
      // Anything other than the one non-default value means 'all', so a
      // corrupted or hand-edited entry degrades to the default rather than
      // wedging the picker in a filtered state the GM didn't ask for.
      return stored === 'mine' ? 'mine' : 'all';
    } catch (error) {
      logger.error('Failed to read cast filter from localStorage:', { error });
      return 'all';
    }
  });

  // Written on change rather than in an effect: an effect would also fire on
  // mount, rewriting the key on every drawer open for no reason.
  const setFilter = useCallback((next: CastFilter) => {
    setFilterState(next);
    try {
      localStorage.setItem(STORAGE_KEY, next);
    } catch (error) {
      logger.error('Failed to save cast filter to localStorage:', { error });
    }
  }, []);

  return [filter, setFilter];
}
