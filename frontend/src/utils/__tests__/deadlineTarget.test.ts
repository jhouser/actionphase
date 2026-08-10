import { describe, it, expect } from 'vitest';
import { getDeadlineTarget } from '../deadlineTarget';
import type { UnifiedDeadline } from '../../types/deadlines';

const IN_PROGRESS_COMMON_ROOM_TABS = ['common-room', 'phases', 'messages', 'people'];
const IN_PROGRESS_ACTION_TABS = ['phases', 'actions', 'messages', 'people'];

function makeDeadline(overrides: Partial<UnifiedDeadline> = {}): UnifiedDeadline {
  return {
    deadline_type: 'deadline',
    source_id: 1,
    title: 'A deadline',
    description: '',
    deadline: '2026-09-01T12:00:00Z',
    game_id: 7,
    is_system_deadline: false,
    ...overrides,
  };
}

describe('getDeadlineTarget', () => {
  describe('poll deadlines', () => {
    it('targets the polls sub-tab anchored to the specific poll', () => {
      const target = getDeadlineTarget(
        makeDeadline({ deadline_type: 'poll', source_id: 42, poll_id: 42, phase_id: 3 }),
        { currentPhaseType: 'common_room', availableTabIds: IN_PROGRESS_COMMON_ROOM_TABS }
      );

      expect(target).toEqual({
        params: { tab: 'common-room', view: 'polls', poll: '42' },
      });
    });

    it('withholds a target when the Common Room tab is not available', () => {
      // Polls can still be open during an action phase, but the Common Room tab
      // is absent then, so navigating would bounce to the default tab.
      const target = getDeadlineTarget(
        makeDeadline({ deadline_type: 'poll', source_id: 42, poll_id: 42 }),
        { currentPhaseType: 'action', availableTabIds: IN_PROGRESS_ACTION_TABS }
      );

      expect(target).toBeNull();
    });

    it('withholds a target when poll_id is missing', () => {
      const target = getDeadlineTarget(
        makeDeadline({ deadline_type: 'poll', source_id: 42, poll_id: undefined }),
        { currentPhaseType: 'common_room', availableTabIds: IN_PROGRESS_COMMON_ROOM_TABS }
      );

      expect(target).toBeNull();
    });
  });

  describe('phase deadlines', () => {
    it('targets the Actions tab during an action phase', () => {
      const target = getDeadlineTarget(
        makeDeadline({ deadline_type: 'phase', phase_id: 9, is_system_deadline: true }),
        { currentPhaseType: 'action', availableTabIds: IN_PROGRESS_ACTION_TABS }
      );

      expect(target?.params).toEqual({ tab: 'actions' });
    });

    it('targets the Common Room tab during a common room phase', () => {
      const target = getDeadlineTarget(
        makeDeadline({ deadline_type: 'phase', phase_id: 9, is_system_deadline: true }),
        { currentPhaseType: 'common_room', availableTabIds: IN_PROGRESS_COMMON_ROOM_TABS }
      );

      expect(target?.params).toEqual({ tab: 'common-room' });
    });

    // Regression: clicking a phase deadline while on the polls sub-tab used to
    // keep view=polls, stranding the user on polls instead of the phase.
    it('clears the poll sub-view so it does not survive the navigation', () => {
      const target = getDeadlineTarget(
        makeDeadline({ deadline_type: 'phase', phase_id: 9, is_system_deadline: true }),
        { currentPhaseType: 'common_room', availableTabIds: IN_PROGRESS_COMMON_ROOM_TABS }
      );

      expect(target?.clearParams).toEqual(expect.arrayContaining(['view', 'poll']));
    });

    it('clears the poll sub-view when targeting the Actions tab too', () => {
      const target = getDeadlineTarget(
        makeDeadline({ deadline_type: 'phase', phase_id: 9, is_system_deadline: true }),
        { currentPhaseType: 'action', availableTabIds: IN_PROGRESS_ACTION_TABS }
      );

      expect(target?.clearParams).toEqual(expect.arrayContaining(['view', 'poll']));
    });

    it('withholds a target when the phase type is unknown', () => {
      const target = getDeadlineTarget(
        makeDeadline({ deadline_type: 'phase', phase_id: 9, is_system_deadline: true }),
        { currentPhaseType: undefined, availableTabIds: IN_PROGRESS_COMMON_ROOM_TABS }
      );

      expect(target).toBeNull();
    });

    it('withholds a target when the phase tab is not available to this viewer', () => {
      // Audience members during an action phase have no Actions tab.
      const target = getDeadlineTarget(
        makeDeadline({ deadline_type: 'phase', phase_id: 9, is_system_deadline: true }),
        { currentPhaseType: 'action', availableTabIds: ['audience', 'people', 'history'] }
      );

      expect(target).toBeNull();
    });
  });

  it('withholds a target for GM-created arbitrary deadlines', () => {
    const target = getDeadlineTarget(makeDeadline({ deadline_type: 'deadline' }), {
      currentPhaseType: 'common_room',
      availableTabIds: IN_PROGRESS_COMMON_ROOM_TABS,
    });

    expect(target).toBeNull();
  });
});
