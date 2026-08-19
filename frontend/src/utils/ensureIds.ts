import { generateId } from './generateId';
import { logger } from '@/services/LoggingService';

/**
 * Guarantees every row carries an `id`, generating one where it is missing.
 *
 * Defensive against data corruption from draft-merge bugs: the sheet's lists are
 * keyed and updated by id, so a row without one cannot be edited or removed.
 *
 * Shared by the sheet's three managers, which each had a private copy of this
 * when they were two — identical in all of them, so the split lifted it out
 * rather than making a third.
 */
export const ensureIds = <T extends { id?: string }>(
  items: T[],
  itemType: string
): (T & { id: string })[] => {
  return items.map(item => {
    if (!item.id) {
      logger.warn(`${itemType} missing id field (data corruption), generating:`, item);
      return { ...item, id: generateId() };
    }
    return item as T & { id: string };
  });
};
