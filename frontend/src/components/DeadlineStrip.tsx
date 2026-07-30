import { useState, useMemo } from 'react';
import { PlusIcon } from '@heroicons/react/24/outline';
import { Button, Modal } from './ui';
import { DeadlineCard } from './DeadlineCard';
import { CreateDeadlineModal } from './CreateDeadlineModal';
import { EditDeadlineModal } from './EditDeadlineModal';
import type { UnifiedDeadline } from '../types/deadlines';

export interface DeadlineStripProps {
  deadlines: UnifiedDeadline[];
  isLoading?: boolean;
  isGM?: boolean;
  gameState?: string; // Hide deadlines for completed/cancelled games
  onCreateDeadline: (data: { title: string; description: string; deadline: string }) => Promise<void>;
  onUpdateDeadline: (deadlineId: number, data: { title: string; description: string; deadline: string }) => Promise<void>;
  onDeleteDeadline: (deadlineId: number) => Promise<void>;
  onExtendDeadline: (deadlineId: number, hours: number) => Promise<void>;
}

/**
 * DeadlineStrip - Compact horizontal deadline display strip
 *
 * Features:
 * - Horizontal card layout (always visible, not collapsible)
 * - Shows max 3 deadline cards inline
 * - "View All" expansion for additional deadlines
 * - Color-coded urgency indicators
 * - Full deadline management (create, edit, delete, extend) for GMs
 *
 * @example
 * ```tsx
 * <DeadlineStrip
 *   deadlines={deadlines}
 *   isLoading={isLoading}
 *   isGM={isGM}
 *   onCreateDeadline={handleCreate}
 *   onUpdateDeadline={handleUpdate}
 *   onDeleteDeadline={handleDelete}
 *   onExtendDeadline={handleExtend}
 * />
 * ```
 */
export function DeadlineStrip({
  deadlines,
  isLoading = false,
  isGM = false,
  gameState,
  onCreateDeadline,
  onUpdateDeadline,
  onDeleteDeadline,
  onExtendDeadline,
}: DeadlineStripProps) {
  const [showAll, setShowAll] = useState(false);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [showEditModal, setShowEditModal] = useState(false);
  const [selectedDeadline, setSelectedDeadline] = useState<UnifiedDeadline | null>(null);
  const [deadlineToDelete, setDeadlineToDelete] = useState<UnifiedDeadline | null>(null);
  const [extendingId, setExtendingId] = useState<number | null>(null);
  const [extendHours, setExtendHours] = useState<number>(24);

  // Separate and sort deadlines
  const { activeDeadlines, expiredDeadlines } = useMemo(() => {
    const now = new Date();
    const active: UnifiedDeadline[] = [];
    const expired: UnifiedDeadline[] = [];

    deadlines.forEach((deadline) => {
      if (!deadline.deadline) {
        active.push(deadline);
      } else {
        const deadlineDate = new Date(deadline.deadline);
        if (deadlineDate > now) {
          active.push(deadline);
        } else {
          expired.push(deadline);
        }
      }
    });

    // Sort active by soonest first
    active.sort((a, b) => {
      if (!a.deadline || !b.deadline) return 0;
      return new Date(a.deadline).getTime() - new Date(b.deadline).getTime();
    });

    // Sort expired by most recent first
    expired.sort((a, b) => {
      if (!a.deadline || !b.deadline) return 0;
      return new Date(b.deadline).getTime() - new Date(a.deadline).getTime();
    });

    return { activeDeadlines: active, expiredDeadlines: expired };
  }, [deadlines]);

  // Show max 3 active deadlines by default
  const visibleActiveDeadlines = showAll ? activeDeadlines : activeDeadlines.slice(0, 3);
  const hasMore = activeDeadlines.length > 3 || expiredDeadlines.length > 0;

  const handleEdit = (deadline: UnifiedDeadline) => {
    setSelectedDeadline(deadline);
    setShowEditModal(true);
  };

  const handleUpdate = async (deadlineId: number, data: { title: string; description: string; deadline: string }) => {
    await onUpdateDeadline(deadlineId, data);
    setShowEditModal(false);
    setSelectedDeadline(null);
  };

  const handleDeleteClick = (deadline: UnifiedDeadline) => {
    setDeadlineToDelete(deadline);
  };

  const handleConfirmDelete = async () => {
    if (deadlineToDelete) {
      await onDeleteDeadline(deadlineToDelete.source_id);
      setDeadlineToDelete(null);
    }
  };

  const handleCancelDelete = () => {
    setDeadlineToDelete(null);
  };

  const handleExtendClick = (deadlineId: number) => {
    setExtendingId(deadlineId);
  };

  const handleConfirmExtend = async () => {
    if (extendingId !== null) {
      await onExtendDeadline(extendingId, extendHours);
      setExtendingId(null);
      setExtendHours(24);
    }
  };

  const handleCancelExtend = () => {
    setExtendingId(null);
    setExtendHours(24);
  };

  const handleCreate = async (data: { title: string; description: string; deadline: string }) => {
    await onCreateDeadline(data);
    setShowCreateModal(false);
  };

  // Only show deadlines during character creation and in-progress states
  if (gameState !== 'character_creation' && gameState !== 'in_progress') {
    return null;
  }

  // Hide during loading
  if (isLoading) {
    return null;
  }

  // Don't show strip if no deadlines and user is not GM
  if (deadlines.length === 0 && !isGM) {
    return null;
  }

  return (
    <>
      <div className="border-t border-border-primary pt-4 pb-4">
        {/* Header */}
        <div className="flex items-center justify-between gap-2 mb-3">
          <div className="flex items-center gap-2 min-w-0">
            <span className="text-base font-semibold text-content-primary whitespace-nowrap">
              ⚠️ Active Deadlines
            </span>
            {/* Count only shown once it's actually informative (more than fit inline as cards) */}
            {activeDeadlines.length > 3 && (
              <span className="text-sm text-content-secondary whitespace-nowrap">
                ({activeDeadlines.length} active{expiredDeadlines.length > 0 ? `, ${expiredDeadlines.length} expired` : ''})
              </span>
            )}
          </div>

          {isGM && (
            <Button
              variant="primary"
              size="sm"
              onClick={() => setShowCreateModal(true)}
              icon={<PlusIcon className="h-4 w-4" />}
              className="flex-shrink-0 px-2 md:px-3"
              aria-label="Add Deadline"
            >
              <span className="hidden md:inline">Add Deadline</span>
            </Button>
          )}
        </div>

        {/* Deadline Cards */}
        {deadlines.length === 0 ? (
          <div className="text-sm text-content-secondary py-2 text-center">
            {isGM ? 'No deadlines yet. Click "Add Deadline" to create one.' : 'No deadlines for this game yet.'}
          </div>
        ) : (
          <div className="space-y-4">
            {/* Active Deadlines - Horizontal Grid */}
            {visibleActiveDeadlines.length > 0 && (
              <div className="flex flex-wrap gap-3">
                {visibleActiveDeadlines.map((deadline) => (
                  <DeadlineCard
                    key={`${deadline.deadline_type}-${deadline.source_id}`}
                    deadline={deadline}
                    isGM={isGM}
                    onEdit={() => handleEdit(deadline)}
                    onExtend={() => handleExtendClick(deadline.source_id)}
                    onDelete={() => handleDeleteClick(deadline)}
                  />
                ))}
              </div>
            )}

            {/* Expired Deadlines (shown when expanded) */}
            {showAll && expiredDeadlines.length > 0 && (
              <div>
                <h3 className="text-sm font-semibold text-content-secondary mb-3">
                  Expired ({expiredDeadlines.length})
                </h3>
                <div className="flex flex-wrap gap-3">
                  {expiredDeadlines.map((deadline) => (
                    <DeadlineCard
                      key={`${deadline.deadline_type}-${deadline.source_id}`}
                      deadline={deadline}
                      isGM={isGM}
                      onDelete={() => handleDeleteClick(deadline)}
                    />
                  ))}
                </div>
              </div>
            )}

            {/* View All / Show Less Toggle */}
            {hasMore && (
              <div className="text-center">
                <button
                  onClick={() => setShowAll(!showAll)}
                  className="text-sm text-interactive-primary hover:text-interactive-primary-hover font-medium transition-colors"
                >
                  {showAll ? '← Show Less' : `View All (${activeDeadlines.length + expiredDeadlines.length}) →`}
                </button>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Create Deadline Modal */}
      <CreateDeadlineModal
        isOpen={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        onSubmit={handleCreate}
        isLoading={false}
      />

      {/* Edit Deadline Modal */}
      <EditDeadlineModal
        isOpen={showEditModal}
        onClose={() => {
          setShowEditModal(false);
          setSelectedDeadline(null);
        }}
        onSubmit={handleUpdate}
        deadline={selectedDeadline}
        isLoading={false}
      />

      {/* Delete Confirmation Modal */}
      {deadlineToDelete && (
        <Modal
          isOpen={true}
          onClose={handleCancelDelete}
          title="Delete Deadline"
          size="sm"
          footer={
            <>
              <Button variant="secondary" onClick={handleCancelDelete}>
                Cancel
              </Button>
              <Button variant="danger" onClick={handleConfirmDelete}>
                Delete
              </Button>
            </>
          }
        >
          <p className="text-content-primary">
            Are you sure you want to delete "{deadlineToDelete.title}"?
          </p>
          <p className="mt-2 text-sm text-content-secondary">
            This action cannot be undone.
          </p>
        </Modal>
      )}

      {/* Extend Deadline Modal */}
      {extendingId !== null && (
        <Modal
          isOpen={true}
          onClose={handleCancelExtend}
          title="Extend Deadline"
          size="sm"
          footer={
            <>
              <Button variant="secondary" onClick={handleCancelExtend}>
                Cancel
              </Button>
              <Button variant="primary" onClick={handleConfirmExtend}>
                Extend Deadline
              </Button>
            </>
          }
        >
          <div className="space-y-4">
            <p className="text-content-primary">
              How many hours would you like to extend this deadline?
            </p>

            <div className="flex flex-wrap gap-2">
              <Button
                variant={extendHours === 24 ? 'primary' : 'secondary'}
                size="sm"
                onClick={() => setExtendHours(24)}
              >
                24 hours
              </Button>
              <Button
                variant={extendHours === 48 ? 'primary' : 'secondary'}
                size="sm"
                onClick={() => setExtendHours(48)}
              >
                48 hours
              </Button>
              <Button
                variant={extendHours === 72 ? 'primary' : 'secondary'}
                size="sm"
                onClick={() => setExtendHours(72)}
              >
                72 hours
              </Button>
              <Button
                variant={extendHours === 168 ? 'primary' : 'secondary'}
                size="sm"
                onClick={() => setExtendHours(168)}
              >
                1 week
              </Button>
            </div>

            <div className="flex items-center gap-2">
              <label htmlFor="custom-hours" className="text-sm text-content-secondary">
                Custom hours:
              </label>
              <input
                id="custom-hours"
                type="number"
                min="1"
                max="720"
                value={extendHours}
                onChange={(e) => setExtendHours(Math.max(1, Math.min(720, parseInt(e.target.value) || 24)))}
                className="w-24 px-3 py-2 border border-border-primary rounded bg-surface-secondary text-content-primary focus:outline-none focus:ring-2 focus:ring-interactive-primary"
              />
            </div>

            {(() => {
              const currentDeadline = deadlines.find(d => d.source_id === extendingId);
              if (currentDeadline?.deadline) {
                const currentDate = new Date(currentDeadline.deadline);
                const newDate = new Date(currentDate.getTime() + extendHours * 60 * 60 * 1000);
                return (
                  <div className="mt-4 p-3 bg-surface-tertiary border border-border-primary rounded">
                    <p className="text-xs text-content-tertiary mb-1">Current deadline:</p>
                    <p className="text-sm text-content-secondary mb-2">
                      {currentDate.toLocaleString()}
                    </p>
                    <p className="text-xs text-content-tertiary mb-1">New deadline:</p>
                    <p className="text-sm text-content-primary font-medium">
                      {newDate.toLocaleString()}
                    </p>
                  </div>
                );
              }
              return null;
            })()}
          </div>
        </Modal>
      )}
    </>
  );
}
