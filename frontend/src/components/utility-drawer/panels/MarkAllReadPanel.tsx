import { CheckCheck } from 'lucide-react';
import { Button, Alert } from '../../ui';
import { useMarkAllCommentsRead } from '../../../hooks/useReadTracking';
import { logger } from '@/services/LoggingService';
import type { UtilityPanelProps } from '../types';

/**
 * Marks every comment in the current phase as read for the current user, in
 * bulk. The frontend clears unread badges immediately (see
 * useMarkAllCommentsRead's optimistic update). On success the drawer closes
 * so the now-read comments are visible right away; on failure it stays open
 * so the error is visible next to the retry button.
 */
export function MarkAllReadPanel({ ctx }: UtilityPanelProps) {
  const { game, closeDrawer } = ctx;
  const currentPhase = game?.currentPhase;
  const { mutate, isPending, isError } = useMarkAllCommentsRead();

  const handleMarkAllRead = () => {
    if (!game || !currentPhase) return;
    mutate(
      { gameId: game.gameId, phaseId: currentPhase.id },
      {
        onSuccess: () => closeDrawer(),
        onError: (err) => logger.error('Failed to mark all comments read', { error: err }),
      }
    );
  };

  return (
    <div className="flex flex-col gap-4 p-4">
      <p className="text-sm text-content-secondary">
        Mark every comment in this phase as read. New comments posted after you do this will still show as unread.
        This can't be undone in bulk — you'd need to mark each comment as unread individually.
      </p>

      <Button
        type="button"
        variant="primary"
        onClick={handleMarkAllRead}
        loading={isPending}
        disabled={!currentPhase}
        data-faro-user-action-name="mark-all-comments-read"
      >
        <CheckCheck className="w-4 h-4" />
        Mark all comments as read
      </Button>

      {isError && (
        <Alert variant="danger" data-testid="mark-all-read-error">
          Something went wrong marking comments as read. Please try again.
        </Alert>
      )}
    </div>
  );
}
