import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, fireEvent } from '@testing-library/react';
import { renderWithProviders } from '../test-utils/render';
import { PhaseActivationDialog } from './PhaseActivationDialog';
import type { GamePhase } from '../types/phases';

const makePhase = (overrides: Partial<GamePhase> = {}): GamePhase => ({
  id: 99,
  game_id: 1,
  phase_type: 'common_room',
  phase_number: 3,
  is_active: false,
  is_published: false,
  start_time: new Date(Date.now() + 5 * 60 * 1000).toISOString(), // 5min from now
  created_at: new Date().toISOString(),
  ...overrides,
});

const makeMutation = (overrides = {}) => ({
  mutateAsync: vi.fn().mockResolvedValue(undefined),
  isPending: false,
  ...overrides,
});

describe('PhaseActivationDialog', () => {
  const mockOnActivate = vi.fn();
  const mockOnClose = vi.fn();

  beforeEach(() => {
    mockOnActivate.mockClear();
    mockOnClose.mockClear();
  });

  describe('Simple confirmation (no unpublished results)', () => {
    it('shows phase number in the heading', () => {
      renderWithProviders(
        <PhaseActivationDialog
          phaseNumber={3}
          currentPhaseId={undefined}
          unpublishedCount={0}
          isActivating={false}
          publishAllMutation={makeMutation()}
          onActivate={mockOnActivate}
          onClose={mockOnClose}
        />
      );

      expect(screen.getByText(/Activate Phase 3\?/i)).toBeInTheDocument();
    });

    it('shows Activate Phase and Cancel buttons when no unpublished results', () => {
      renderWithProviders(
        <PhaseActivationDialog
          phaseNumber={3}
          currentPhaseId={undefined}
          unpublishedCount={0}
          isActivating={false}
          publishAllMutation={makeMutation()}
          onActivate={mockOnActivate}
          onClose={mockOnClose}
        />
      );

      expect(screen.getByRole('button', { name: /Activate Phase/i })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /Cancel/i })).toBeInTheDocument();
    });

    it('calls onActivate and onClose when Activate Phase is clicked', () => {
      renderWithProviders(
        <PhaseActivationDialog
          phaseNumber={3}
          currentPhaseId={undefined}
          unpublishedCount={0}
          isActivating={false}
          publishAllMutation={makeMutation()}
          onActivate={mockOnActivate}
          onClose={mockOnClose}
        />
      );

      fireEvent.click(screen.getByRole('button', { name: /Activate Phase/i }));
      expect(mockOnActivate).toHaveBeenCalledTimes(1);
      expect(mockOnClose).toHaveBeenCalledTimes(1);
    });

    it('calls onClose when Cancel is clicked without activating', () => {
      renderWithProviders(
        <PhaseActivationDialog
          phaseNumber={3}
          currentPhaseId={undefined}
          unpublishedCount={0}
          isActivating={false}
          publishAllMutation={makeMutation()}
          onActivate={mockOnActivate}
          onClose={mockOnClose}
        />
      );

      fireEvent.click(screen.getByRole('button', { name: /Cancel/i }));
      expect(mockOnClose).toHaveBeenCalledTimes(1);
      expect(mockOnActivate).not.toHaveBeenCalled();
    });

    it('disables Activate button and shows "Activating..." when isActivating', () => {
      renderWithProviders(
        <PhaseActivationDialog
          phaseNumber={3}
          currentPhaseId={undefined}
          unpublishedCount={0}
          isActivating={true}
          publishAllMutation={makeMutation()}
          onActivate={mockOnActivate}
          onClose={mockOnClose}
        />
      );

      const button = screen.getByRole('button', { name: /Activating.../i });
      expect(button).toBeDisabled();
    });
  });

  describe('Unpublished results warning', () => {
    it('shows "Publish & Activate" and "Activate Without Publishing" when unpublished results exist', () => {
      renderWithProviders(
        <PhaseActivationDialog
          phaseNumber={3}
          currentPhaseId={1}
          unpublishedCount={3}
          isActivating={false}
          publishAllMutation={makeMutation()}
          onActivate={mockOnActivate}
          onClose={mockOnClose}
        />
      );

      expect(screen.getByRole('button', { name: /Publish & Activate Phase/i })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /Activate Without Publishing/i })).toBeInTheDocument();
    });

    it('shows the unpublished count in the warning', () => {
      renderWithProviders(
        <PhaseActivationDialog
          phaseNumber={3}
          currentPhaseId={1}
          unpublishedCount={5}
          isActivating={false}
          publishAllMutation={makeMutation()}
          onActivate={mockOnActivate}
          onClose={mockOnClose}
        />
      );

      expect(screen.getByText(/5 unpublished results/i)).toBeInTheDocument();
    });

    it('singular "result" when count is 1', () => {
      renderWithProviders(
        <PhaseActivationDialog
          phaseNumber={3}
          currentPhaseId={1}
          unpublishedCount={1}
          isActivating={false}
          publishAllMutation={makeMutation()}
          onActivate={mockOnActivate}
          onClose={mockOnClose}
        />
      );

      expect(screen.getByText(/1 unpublished result/i)).toBeInTheDocument();
      expect(screen.queryByText(/1 unpublished results/i)).not.toBeInTheDocument();
    });

    it('"Activate Without Publishing" calls onActivate and onClose', () => {
      renderWithProviders(
        <PhaseActivationDialog
          phaseNumber={3}
          currentPhaseId={1}
          unpublishedCount={3}
          isActivating={false}
          publishAllMutation={makeMutation()}
          onActivate={mockOnActivate}
          onClose={mockOnClose}
        />
      );

      fireEvent.click(screen.getByRole('button', { name: /Activate Without Publishing/i }));
      expect(mockOnActivate).toHaveBeenCalledTimes(1);
      expect(mockOnClose).toHaveBeenCalledTimes(1);
    });
  });

  describe('Near-future scheduled phase warning', () => {
    it('shows no warning when nearFutureScheduled is empty', () => {
      renderWithProviders(
        <PhaseActivationDialog
          phaseNumber={3}
          currentPhaseId={undefined}
          unpublishedCount={0}
          nearFutureScheduled={[]}
          isActivating={false}
          publishAllMutation={makeMutation()}
          onActivate={mockOnActivate}
          onClose={mockOnClose}
        />
      );

      expect(screen.queryByText(/scheduled to activate/i)).not.toBeInTheDocument();
    });

    it('shows warning when a near-future phase exists', () => {
      const scheduledPhase = makePhase({
        id: 5,
        phase_number: 4,
        start_time: new Date(Date.now() + 8 * 60 * 1000).toISOString(), // 8min from now
      });

      renderWithProviders(
        <PhaseActivationDialog
          phaseNumber={3}
          currentPhaseId={undefined}
          unpublishedCount={0}
          nearFutureScheduled={[scheduledPhase]}
          isActivating={false}
          publishAllMutation={makeMutation()}
          onActivate={mockOnActivate}
          onClose={mockOnClose}
        />
      );

      expect(screen.getByText(/scheduled phase activating soon/i)).toBeInTheDocument();
      expect(screen.getByText(/Phase 4/i)).toBeInTheDocument();
      expect(screen.getByText(/is scheduled to activate at/i)).toBeInTheDocument();
    });

    it('shows the near-future phase title when it has one', () => {
      const scheduledPhase = makePhase({
        id: 5,
        phase_number: 4,
        title: 'The Final Confrontation',
        start_time: new Date(Date.now() + 3 * 60 * 1000).toISOString(),
      });

      renderWithProviders(
        <PhaseActivationDialog
          phaseNumber={3}
          currentPhaseId={undefined}
          unpublishedCount={0}
          nearFutureScheduled={[scheduledPhase]}
          isActivating={false}
          publishAllMutation={makeMutation()}
          onActivate={mockOnActivate}
          onClose={mockOnClose}
        />
      );

      expect(screen.getByText(/"The Final Confrontation"/i)).toBeInTheDocument();
    });

    it('lists multiple near-future phases', () => {
      const scheduledPhase1 = makePhase({ id: 5, phase_number: 4, title: undefined });
      const scheduledPhase2 = makePhase({ id: 6, phase_number: 5, title: undefined });

      renderWithProviders(
        <PhaseActivationDialog
          phaseNumber={3}
          currentPhaseId={undefined}
          unpublishedCount={0}
          nearFutureScheduled={[scheduledPhase1, scheduledPhase2]}
          isActivating={false}
          publishAllMutation={makeMutation()}
          onActivate={mockOnActivate}
          onClose={mockOnClose}
        />
      );

      expect(screen.getByText(/Phase 4/i)).toBeInTheDocument();
      expect(screen.getByText(/Phase 5/i)).toBeInTheDocument();
    });

    it('shows both near-future warning and unpublished results warning when both apply', () => {
      const scheduledPhase = makePhase({ id: 5, phase_number: 4 });

      renderWithProviders(
        <PhaseActivationDialog
          phaseNumber={3}
          currentPhaseId={1}
          unpublishedCount={2}
          nearFutureScheduled={[scheduledPhase]}
          isActivating={false}
          publishAllMutation={makeMutation()}
          onActivate={mockOnActivate}
          onClose={mockOnClose}
        />
      );

      expect(screen.getByText(/scheduled phase activating soon/i)).toBeInTheDocument();
      expect(screen.getByText(/2 unpublished results/i)).toBeInTheDocument();
    });
  });

  describe('Sheet draft conflict warning', () => {
    it('warns when a character has staged sheet updates on several results', () => {
      renderWithProviders(
        <PhaseActivationDialog
          phaseNumber={3}
          currentPhaseId={1}
          unpublishedCount={2}
          isActivating={false}
          publishAllMutation={makeMutation()}
          hasSheetDraftConflict
          onActivate={mockOnActivate}
          onClose={mockOnClose}
        />
      );

      expect(screen.getByTestId('activate-sheet-conflict-warning')).toBeInTheDocument();
    });

    it('shows no warning when the staged updates do not overlap', () => {
      renderWithProviders(
        <PhaseActivationDialog
          phaseNumber={3}
          currentPhaseId={1}
          unpublishedCount={2}
          isActivating={false}
          publishAllMutation={makeMutation()}
          onActivate={mockOnActivate}
          onClose={mockOnClose}
        />
      );

      expect(screen.queryByTestId('activate-sheet-conflict-warning')).not.toBeInTheDocument();
    });
  });
});
