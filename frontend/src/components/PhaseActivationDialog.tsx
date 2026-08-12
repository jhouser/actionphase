import type { UseMutationResult } from '@tanstack/react-query';
import { Button, Alert } from './ui';
import type { GamePhase } from '../types/phases';
import { utcToLocalDateTime } from '../utils/timezone';

interface PhaseActivationDialogProps {
  phaseNumber: number;
  currentPhaseId: number | undefined;
  unpublishedCount: number;
  nearFutureScheduled?: GamePhase[];
  isActivating: boolean;
  publishAllMutation: UseMutationResult<unknown, Error, void, unknown>;
  /**
   * True when some character has staged sheet updates on more than one of the
   * unpublished results. Publishing them together overwrites one set with another.
   */
  hasSheetDraftConflict?: boolean;
  onActivate: () => void;
  onClose: () => void;
}

export function PhaseActivationDialog({
  phaseNumber,
  currentPhaseId,
  unpublishedCount,
  nearFutureScheduled = [],
  isActivating,
  publishAllMutation,
  hasSheetDraftConflict = false,
  onActivate,
  onClose
}: PhaseActivationDialogProps) {
  return (
    <div
      className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center p-4 z-50"
      onClick={(e) => e.stopPropagation()}
    >
      <div className="surface-base rounded-lg max-w-md w-full p-6">
        <h3 className="text-lg font-semibold text-content-primary mb-2">
          Activate Phase {phaseNumber}?
        </h3>

        {nearFutureScheduled.length > 0 && (
          <div className="mb-4 p-3 bg-semantic-warning-subtle border border-semantic-warning rounded">
            <div className="flex items-start gap-2">
              <svg className="w-5 h-5 text-semantic-warning mt-0.5 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
              <div>
                <p className="text-sm font-medium text-content-primary">Scheduled phase activating soon</p>
                {nearFutureScheduled.map(p => (
                  <p key={p.id} className="text-sm text-content-secondary mt-1">
                    Phase {p.phase_number}{p.title ? ` "${p.title}"` : ''} is scheduled to activate at{' '}
                    {p.start_time ? utcToLocalDateTime(p.start_time) : ''}.
                    It will override this activation unless you remove its scheduled time.
                  </p>
                ))}
              </div>
            </div>
          </div>
        )}

        {currentPhaseId && unpublishedCount > 0 ? (
          <>
            <div className="mb-4 p-3 bg-semantic-warning-subtle border border-semantic-warning rounded">
              <div className="flex items-start">
                <svg
                  className="w-5 h-5 text-semantic-warning mr-2 mt-0.5 flex-shrink-0"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                  />
                </svg>
                <div className="flex-1">
                  <p className="text-sm font-medium text-content-primary">
                    You have {unpublishedCount} unpublished {unpublishedCount === 1 ? 'result' : 'results'}
                  </p>
                  <p className="text-sm text-semantic-warning mt-1">
                    Do you want to publish {unpublishedCount === 1 ? 'it' : 'them'} before activating the next phase?
                  </p>
                </div>
              </div>
            </div>
            {/* Publishing in bulk applies every conflicting result at once, making the
                clobber certain rather than possible. */}
            {hasSheetDraftConflict && (
              <Alert variant="danger" className="mb-4" data-testid="activate-sheet-conflict-warning">
                A character has staged sheet updates on more than one of these results.
                Each result stores a complete snapshot of the character sheet, so
                publishing them together will overwrite one set of changes with another.
                Consider activating without publishing, then consolidating the sheet
                updates into a single result.
              </Alert>
            )}
            <div className="flex flex-col space-y-2">
              <Button
                variant="success"
                onClick={async () => {
                  await publishAllMutation.mutateAsync();
                  onActivate();
                  onClose();
                }}
                disabled={isActivating || publishAllMutation.isPending}
                className="w-full"
                data-faro-user-action-name="activate-phase"
              >
                {publishAllMutation.isPending ? 'Publishing...' : isActivating ? 'Activating...' : 'Publish & Activate Phase'}
              </Button>
              <Button
                variant="primary"
                onClick={() => {
                  onActivate();
                  onClose();
                }}
                disabled={isActivating || publishAllMutation.isPending}
                className="w-full"
                data-faro-user-action-name="activate-phase"
              >
                {isActivating ? 'Activating...' : 'Activate Without Publishing'}
              </Button>
              <Button
                variant="ghost"
                onClick={onClose}
                disabled={isActivating || publishAllMutation.isPending}
                className="w-full"
              >
                Cancel
              </Button>
            </div>
          </>
        ) : (
          <>
            <p className="text-sm text-content-secondary mb-6">
              This will deactivate the current phase and make Phase {phaseNumber} active. Continue?
            </p>
            <div className="flex justify-end space-x-3">
              <Button
                variant="ghost"
                onClick={onClose}
                disabled={isActivating}
              >
                Cancel
              </Button>
              <Button
                variant="primary"
                onClick={() => {
                  onActivate();
                  onClose();
                }}
                disabled={isActivating}
              >
                {isActivating ? 'Activating...' : 'Activate Phase'}
              </Button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
