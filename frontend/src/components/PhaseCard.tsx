import { useState } from 'react';
import { SimpleCountdown } from './CountdownTimer';
import { PhaseActivationDialog } from './PhaseActivationDialog';
import { DeletePhaseDialog } from './DeletePhaseDialog';
import { DraftPostSection } from './DraftPostSection';
import { usePhaseActivation } from '../hooks/usePhaseActivation';
import { usePhaseSheetDraftConflicts } from '../hooks/useConflictingSheetDrafts';
import { MarkdownPreview } from './MarkdownPreview';
import { Button, DateTimeInput } from './ui';
import {
  PHASE_TYPE_DESCRIPTIONS,
  getActionPhaseLabel,
  getActionPhaseColor
} from '../types/phases';
import type { GamePhase } from '../types/phases';

function isScheduledFuture(phase: GamePhase): boolean {
  if (!phase.start_time || phase.is_active) return false;
  return new Date(phase.start_time).getTime() > Date.now();
}

// Phases with a scheduled start_time within this window may override a manual activation
const NEAR_FUTURE_WINDOW_MS = 10 * 60 * 1000; // 10 minutes

interface PhaseCardProps {
  phase: GamePhase;
  gameId: number;
  allPhases?: GamePhase[];
  currentPhaseId?: number;
  isActive: boolean;
  isSelected: boolean;
  isEditingDeadline: boolean;
  onSelect: () => void;
  onActivate: () => void;
  onEdit: () => void;
  onDelete: () => Promise<void>;
  onEditDeadline: () => void;
  onUpdateDeadline: (deadline: string) => void;
  onCancelEditDeadline: () => void;
  isActivating: boolean;
  isUpdatingDeadline: boolean;
}

export function PhaseCard({
  phase,
  gameId,
  allPhases = [],
  currentPhaseId,
  isActive,
  isSelected,
  isEditingDeadline,
  onSelect,
  onActivate,
  onEdit,
  onDelete,
  onEditDeadline: _onEditDeadline,
  onUpdateDeadline,
  onCancelEditDeadline,
  isActivating,
  isUpdatingDeadline,
}: PhaseCardProps) {
  const [deadlineInput, setDeadlineInput] = useState('');
  const [showActivateConfirm, setShowActivateConfirm] = useState(false);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);

  // Use the activation hook for unpublished results logic
  const { unpublishedCount, publishAllMutation } = usePhaseActivation(
    gameId,
    currentPhaseId,
    showActivateConfirm
  );

  // Detects characters whose sheet updates are split across several unpublished
  // results, which bulk publishing would clobber. Gated on the dialog being open like
  // usePhaseActivation above: every phase card receives the same currentPhaseId, so an
  // ungated call repeats identical work once per phase in the game.
  const { hasConflict: hasSheetDraftConflict } = usePhaseSheetDraftConflicts(
    gameId,
    currentPhaseId,
    showActivateConfirm
  );

  // Find any other inactive phases with start_time in the near future — activating
  // this phase manually won't prevent them from firing on the next scheduler tick.
  const nearFutureScheduled = allPhases.filter(p => {
    if (p.id === phase.id || p.is_active) return false;
    if (!p.start_time) return false;
    const ms = new Date(p.start_time).getTime() - Date.now();
    return ms > 0 && ms <= NEAR_FUTURE_WINDOW_MS;
  });

  const phaseColorClass = getActionPhaseColor(phase);
  const phaseLabel = getActionPhaseLabel(phase);

  const scheduled = isScheduledFuture(phase);

  const borderClass = isActive
    ? 'border-interactive-primary bg-interactive-primary-subtle'
    : scheduled
    ? 'border-semantic-warning bg-semantic-warning-subtle'
    : isSelected
    ? 'border-theme-strong surface-raised'
    : 'border-theme-default hover:border-theme-strong';

  const handleDeadlineSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (deadlineInput) {
      onUpdateDeadline(deadlineInput);
    }
  };

  return (
    <div
      className={`border rounded-lg p-3 md:p-4 transition-colors cursor-pointer ${borderClass}`}
      onClick={onSelect}
      data-testid="phase-card"
    >
      {/* Mobile: Vertical Stack Layout */}
      <div className="md:hidden space-y-3">
        {/* Header: Badge + Edit/Delete buttons */}
        <div className="flex items-center justify-between">
          <span className={`px-2.5 py-1 text-xs rounded-full font-medium border whitespace-nowrap ${phaseColorClass}`}>
            Phase {phase.phase_number}
          </span>
          <div className="flex items-center gap-1">
            <button
              onClick={(e) => {
                e.stopPropagation();
                onEdit();
              }}
              className="p-1.5 text-content-tertiary hover:text-content-primary transition-colors"
              title="Edit phase details"
            >
              <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
              </svg>
            </button>
            {!isActive && (
              <button
                onClick={(e) => {
                  e.stopPropagation();
                  setShowDeleteConfirm(true);
                }}
                className="p-1.5 text-semantic-danger hover:text-semantic-danger-hover transition-colors"
                title="Delete phase"
              >
                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                </svg>
              </button>
            )}
          </div>
        </div>

        {/* Title + Type Badge */}
        <div>
          <h4 className="font-semibold text-base text-content-primary mb-1">
            {phase.title || phaseLabel}
          </h4>
          {phase.title && (
            <span className={`inline-block px-2.5 py-1 text-xs rounded font-medium whitespace-nowrap ${phaseColorClass}`}>
              {phaseLabel}
            </span>
          )}
        </div>

        {/* Description */}
        <div className="text-sm text-content-secondary leading-relaxed">
          {phase.description
            ? <MarkdownPreview content={phase.description} />
            : PHASE_TYPE_DESCRIPTIONS[phase.phase_type]
          }
        </div>

        {/* Countdown + Activate button */}
        <div className="flex items-center justify-between gap-3 pt-2">
          <div className="flex items-center gap-2">
            {phase.deadline && !isEditingDeadline && (
              <SimpleCountdown
                deadline={phase.deadline}
                className="text-content-secondary text-sm"
              />
            )}
            {scheduled && phase.start_time && (
              <span className="text-sm text-content-primary flex items-center gap-1">
                <svg className="w-3.5 h-3.5 shrink-0 text-semantic-warning" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                Activates in <SimpleCountdown deadline={phase.start_time} className="text-content-primary" />
              </span>
            )}
          </div>
          {!isActive && (
            <Button
              variant="primary"
              size="sm"
              onClick={(e) => {
                e.stopPropagation();
                setShowActivateConfirm(true);
              }}
              disabled={isActivating}
              className="ml-auto"
            >
              {isActivating ? 'Activating...' : 'Activate'}
            </Button>
          )}
        </div>
      </div>

      {/* Desktop: Horizontal Layout (Original) */}
      <div className="hidden md:flex items-center justify-between">
        <div className="flex items-center space-x-3">
          <span className={`px-2 py-1 text-xs rounded-full font-medium border whitespace-nowrap ${phaseColorClass}`}>
            Phase {phase.phase_number}
          </span>
          <div>
            <div className="flex items-center gap-2">
              <h4 className="font-medium text-content-primary">{phase.title || phaseLabel}</h4>
              {phase.title && (
                <span className={`px-2 py-0.5 text-xs rounded font-medium whitespace-nowrap ${phaseColorClass}`}>
                  {phaseLabel}
                </span>
              )}
            </div>
            <div className="text-sm text-content-secondary">
              {phase.description
                ? <MarkdownPreview content={phase.description} />
                : PHASE_TYPE_DESCRIPTIONS[phase.phase_type]
              }
            </div>
          </div>
        </div>

        <div className="flex items-center space-x-2">
          {phase.deadline && !isEditingDeadline && (
            <SimpleCountdown
              deadline={phase.deadline}
              className="text-content-secondary"
            />
          )}

          {scheduled && phase.start_time && (
            <span className="text-sm text-content-primary flex items-center gap-1">
              <svg className="w-3.5 h-3.5 shrink-0 text-semantic-warning" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              Activates in <SimpleCountdown deadline={phase.start_time} className="text-content-primary" />
            </span>
          )}

          {!isActive && (
            <Button
              variant="primary"
              size="sm"
              onClick={(e) => {
                e.stopPropagation();
                setShowActivateConfirm(true);
              }}
              disabled={isActivating}
            >
              {isActivating ? 'Activating...' : 'Activate'}
            </Button>
          )}

          <button
            onClick={(e) => {
              e.stopPropagation();
              onEdit();
            }}
            className="p-1 text-content-tertiary hover:text-content-primary transition-colors"
            title="Edit phase details"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
            </svg>
          </button>

          {!isActive && (
            <button
              onClick={(e) => {
                e.stopPropagation();
                setShowDeleteConfirm(true);
              }}
              className="p-1 text-semantic-danger hover:text-semantic-danger-hover transition-colors"
              title="Delete phase"
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
              </svg>
            </button>
          )}
        </div>
      </div>

      {/* Edit Deadline Form */}
      {isEditingDeadline && (
        <form onSubmit={handleDeadlineSubmit} className="mt-4 pt-4 border-t border-theme-default" onClick={(e) => e.stopPropagation()}>
          <div className="flex items-end space-x-2">
            <div className="flex-1">
              <DateTimeInput
                label="Set Deadline"
                value={deadlineInput}
                onChange={(e) => setDeadlineInput(e.target.value)}
                required
              />
            </div>
            <Button
              type="submit"
              variant="primary"
              size="sm"
              disabled={isUpdatingDeadline}
            >
              {isUpdatingDeadline ? 'Saving...' : 'Save'}
            </Button>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={onCancelEditDeadline}
            >
              Cancel
            </Button>
          </div>
        </form>
      )}

      {/* Active Phase Indicator */}
      {isActive && (
        <div className="mt-3 flex items-center text-sm text-interactive-primary">
          <div className="w-2 h-2 bg-interactive-primary rounded-full mr-2"></div>
          Currently Active
        </div>
      )}

      {/* Draft Post Section — shown for pending common_room phases only */}
      {!isActive && phase.phase_type === 'common_room' && (
        <DraftPostSection phaseId={phase.id} onCreateDraft={() => {}} />
      )}

      {/* Phase Activation Confirmation Dialog */}
      {showActivateConfirm && (
        <PhaseActivationDialog
          phaseNumber={phase.phase_number}
          currentPhaseId={currentPhaseId}
          unpublishedCount={unpublishedCount}
          nearFutureScheduled={nearFutureScheduled}
          isActivating={isActivating}
          publishAllMutation={publishAllMutation}
          hasSheetDraftConflict={hasSheetDraftConflict}
          onActivate={onActivate}
          onClose={() => setShowActivateConfirm(false)}
        />
      )}

      {/* Phase Deletion Confirmation Dialog */}
      {showDeleteConfirm && (
        <DeletePhaseDialog
          isOpen={showDeleteConfirm}
          onClose={() => setShowDeleteConfirm(false)}
          onConfirm={onDelete}
          phase={phase}
        />
      )}
    </div>
  );
}
