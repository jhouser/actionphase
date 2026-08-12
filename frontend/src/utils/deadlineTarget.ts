import type { UnifiedDeadline } from '../types/deadlines';

/**
 * Where a deadline card should navigate when clicked.
 *
 * `params` are merged into the game page's existing search params, so they
 * describe only what this navigation changes (tab, and any sub-tab/anchor).
 */
export interface DeadlineTarget {
  params: Record<string, string>;
  /**
   * Params to strip from the current URL. A stale sub-tab (e.g. `view=polls`
   * left over from a previous poll deadline) would otherwise survive the
   * navigation and keep the user on the wrong sub-view.
   */
  clearParams?: string[];
}

interface DeadlineTargetContext {
  /** phase_type of the game's currently active phase, if known. */
  currentPhaseType?: string;
  /** Tab ids currently available on the game page. */
  availableTabIds: string[];
}

/**
 * Resolve the in-app destination for a deadline card.
 *
 * Deadlines only carry a destination when there is a concrete place to land:
 *
 * - **poll** → the Common Room's polls sub-tab, anchored to the poll itself.
 * - **phase** → the tab where that phase is acted on (Actions or Common Room).
 *   Phase deadlines are only emitted for the *active* phase, so the current
 *   phase type is what decides between them.
 * - **deadline** (GM-created, free-text) → nothing. These describe an
 *   obligation, not a location, so there is no honest place to send someone.
 *
 * Returns `null` when there is no destination, which callers use to leave the
 * card non-interactive rather than navigate somewhere arbitrary. A target is
 * also withheld when the destination tab isn't currently available (e.g. a poll
 * deadline still counting down during an action phase, when the Common Room tab
 * is absent) — navigating there would be bounced to the default tab.
 */
export function getDeadlineTarget(
  deadline: UnifiedDeadline,
  { currentPhaseType, availableTabIds }: DeadlineTargetContext
): DeadlineTarget | null {
  const hasTab = (id: string) => availableTabIds.includes(id);

  if (deadline.deadline_type === 'poll') {
    if (!deadline.poll_id || !hasTab('common-room')) {
      return null;
    }
    return {
      params: {
        tab: 'common-room',
        view: 'polls',
        poll: String(deadline.poll_id),
      },
    };
  }

  if (deadline.deadline_type === 'phase') {
    // A phase deadline points at the phase itself, so any poll sub-view must be
    // dropped — otherwise clicking it from the polls tab leaves you on polls.
    if (currentPhaseType === 'action' && hasTab('actions')) {
      return { params: { tab: 'actions' }, clearParams: ['view', 'poll'] };
    }
    if (currentPhaseType === 'common_room' && hasTab('common-room')) {
      return { params: { tab: 'common-room' }, clearParams: ['view', 'poll'] };
    }
    return null;
  }

  return null;
}
