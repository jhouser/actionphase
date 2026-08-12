import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { PhaseCard } from './PhaseCard';
import type { GamePhase } from '../types/phases';

// Mock the hook that makes API calls — we don't need it for these tests
vi.mock('../hooks/usePhaseActivation', () => ({
  usePhaseActivation: () => ({
    unpublishedCount: 0,
    publishAllMutation: { mutate: vi.fn(), isPending: false },
  }),
}));

// Mock DraftPostSection to avoid needing QueryClientProvider in these tests
vi.mock('./DraftPostSection', () => ({
  DraftPostSection: () => null,
}));

// Also API-backed; PhaseActivationDialog.test.tsx covers the conflict warning itself.
vi.mock('../hooks/useConflictingSheetDrafts', () => ({
  usePhaseSheetDraftConflicts: () => ({ affectedCharacterCount: 0, hasConflict: false }),
}));

const basePhase: GamePhase = {
  id: 1,
  game_id: 100,
  phase_type: 'common_room',
  phase_number: 1,
  title: 'Test Phase',
  is_active: false,
  is_published: false,
  start_time: new Date().toISOString(),
  created_at: new Date().toISOString(),
};

const defaultProps = {
  phase: basePhase,
  gameId: 100,
  isActive: false,
  isSelected: false,
  isEditingDeadline: false,
  onSelect: vi.fn(),
  onActivate: vi.fn(),
  onEdit: vi.fn(),
  onDelete: vi.fn().mockResolvedValue(undefined),
  onEditDeadline: vi.fn(),
  onUpdateDeadline: vi.fn(),
  onCancelEditDeadline: vi.fn(),
  isActivating: false,
  isUpdatingDeadline: false,
};

describe('PhaseCard scheduled indicator', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('shows "Activates in" when phase has a future start_time and is inactive', () => {
    const futureTime = new Date(Date.now() + 2 * 60 * 60 * 1000).toISOString(); // 2h from now
    render(
      <PhaseCard
        {...defaultProps}
        phase={{ ...basePhase, start_time: futureTime }}
      />
    );
    expect(screen.getAllByText(/Activates in/i).length).toBeGreaterThan(0);
  });

  it('does not show "Activates in" when start_time is in the past', () => {
    const pastTime = new Date(Date.now() - 60 * 1000).toISOString(); // 1min ago
    render(
      <PhaseCard
        {...defaultProps}
        phase={{ ...basePhase, start_time: pastTime }}
      />
    );
    expect(screen.queryByText(/Activates in/i)).not.toBeInTheDocument();
  });

  it('does not show "Activates in" when phase is already active', () => {
    const futureTime = new Date(Date.now() + 2 * 60 * 60 * 1000).toISOString();
    render(
      <PhaseCard
        {...defaultProps}
        isActive={true}
        phase={{ ...basePhase, start_time: futureTime, is_active: true }}
      />
    );
    expect(screen.queryByText(/Activates in/i)).not.toBeInTheDocument();
  });

  it('does not show "Activates in" when phase has no start_time', () => {
    render(
      <PhaseCard
        {...defaultProps}
        phase={{ ...basePhase, start_time: undefined }}
      />
    );
    expect(screen.queryByText(/Activates in/i)).not.toBeInTheDocument();
  });

  it('shows "Currently Active" indicator for active phases', () => {
    render(
      <PhaseCard
        {...defaultProps}
        isActive={true}
        phase={{ ...basePhase, is_active: true }}
      />
    );
    expect(screen.getByText('Currently Active')).toBeInTheDocument();
  });

  it('does not show "Currently Active" indicator for inactive phases', () => {
    render(
      <PhaseCard
        {...defaultProps}
        isActive={false}
        phase={{ ...basePhase, is_active: false }}
      />
    );
    expect(screen.queryByText('Currently Active')).not.toBeInTheDocument();
  });
});
