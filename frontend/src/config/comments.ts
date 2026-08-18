/**
 * Comment Threading Configuration
 *
 * Centralized configuration for comment threading depth limits.
 *
 * These values control:
 * - How many nesting levels are visible in main view
 * - When "Continue thread" button appears
 * - When Reply buttons are shown
 *
 * **Behavior**:
 * - Comments at depths 0 through (MAX_DEPTH - 1) are visible with Reply buttons
 * - Comments at depth (MAX_DEPTH - 1) with deeper replies show "Continue thread" button
 * - Comments at depth MAX_DEPTH and beyond are NOT visible in main view
 *
 * **Example with MAX_DEPTH = 5**:
 * - Depths 0-4: Visible with Reply buttons
 * - Depth 4 with children: Shows "Continue thread"
 * - Depth 5+: Hidden (accessible via thread view modal)
 */

import { logger } from '@/services/LoggingService';

/**
 * Desktop comment threading depth limit
 * Default: 5 (shows depths 0-4, "Continue thread" at depth 4)
 */
export const COMMENT_MAX_DEPTH = parseInt(
  import.meta.env.VITE_COMMENT_MAX_DEPTH || '5',
  10
);

/**
 * Mobile comment threading depth limit
 * Lower than desktop to save screen space
 * Default: 3 (shows depths 0-2, "Continue thread" at depth 2)
 */
export const COMMENT_MAX_DEPTH_MOBILE = parseInt(
  import.meta.env.VITE_COMMENT_MAX_DEPTH_MOBILE || '3',
  10
);


// Validate configuration
if (COMMENT_MAX_DEPTH < 1 || COMMENT_MAX_DEPTH > 10) {
  logger.error(`Invalid VITE_COMMENT_MAX_DEPTH: ${COMMENT_MAX_DEPTH}. Must be 1-10.`);
}

if (COMMENT_MAX_DEPTH_MOBILE < 1 || COMMENT_MAX_DEPTH_MOBILE > 10) {
  logger.error(`Invalid VITE_COMMENT_MAX_DEPTH_MOBILE: ${COMMENT_MAX_DEPTH_MOBILE}. Must be 1-10.`);
}

/**
 * Thread view modal depth limit
 * When viewing a specific thread in modal, show many levels
 * Default: 10 (nearly unlimited for thread view)
 */
export const THREAD_VIEW_MAX_DEPTH = 10;

/**
 * Preferred number of ancestor levels to fetch alongside a deep-linked target
 * comment (e.g. when opening a notification) for enough surrounding context.
 * The actual count is capped per-viewport by parentContextForViewport so the
 * target never lands beyond the visible depth.
 */
const DEEP_LINK_PARENT_CONTEXT = 3;

/**
 * Number of ancestor levels to fetch alongside a deep-linked target comment
 * (e.g. when opening a notification), given the current viewport.
 *
 * The fetched chain is rendered nested starting at depth 0, so the target
 * comment lands at a depth equal to the number of parents included. The target
 * must land at a *visible* depth — the deepest visible depth is (maxDepth - 1) —
 * otherwise the deepest visible level shows a "Continue this thread" button that
 * hides the very comment the user clicked.
 *
 * We therefore fetch DEEP_LINK_PARENT_CONTEXT parents, but never more than the
 * viewport can render inline. Mobile's shallower max depth is the binding
 * constraint; this stays correct if COMMENT_MAX_DEPTH_MOBILE is retuned (e.g.
 * 3 → 2 parents, 4 → 3 parents) without a second code change.
 */
export function parentContextForDepth(maxDepth: number): number {
  // Deepest depth that still renders inline (depth is 0-indexed).
  const maxVisibleParents = maxDepth - 1;
  return Math.min(DEEP_LINK_PARENT_CONTEXT, maxVisibleParents);
}

export function parentContextForViewport(isMobile: boolean): number {
  return parentContextForDepth(isMobile ? COMMENT_MAX_DEPTH_MOBILE : COMMENT_MAX_DEPTH);
}

/**
 * Depth to request from the API.
 *
 * Desktop and mobile are NOT separate fetches — ThreadedComment renders both
 * subtrees from the same fetched tree and hides one with Tailwind `md:` classes
 * (there is no JS viewport check). So the fetch must cover whichever limit is
 * deeper, or the larger viewport renders "Continue thread" for replies that were
 * never fetched.
 *
 * Normally desktop >= mobile, but this does not assume that: the two values are
 * independently configurable via env vars and nothing validates their relative
 * order.
 */
export const COMMENT_FETCH_MAX_DEPTH = Math.max(
  COMMENT_MAX_DEPTH,
  COMMENT_MAX_DEPTH_MOBILE
);
