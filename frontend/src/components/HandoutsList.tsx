import { useState, useEffect } from 'react';
import { useSearchParams } from 'react-router-dom';
import { Card, Spinner, Alert, Button } from './ui';
import { useToast } from '../contexts/ToastContext';
import { useHandouts } from '../hooks/useHandouts';
import { HandoutCard } from './HandoutCard';
import { CreateHandoutModal } from './CreateHandoutModal';
import { EditHandoutModal } from './EditHandoutModal';
import { HandoutView } from './HandoutView';
import type { Handout, CreateHandoutRequest, UpdateHandoutRequest } from '../types/handouts';
import { isGameWritable } from '@/lib/gamePermissions';

interface HandoutsListProps {
  gameId: number;
  isGM: boolean;
  gameState?: string;
}

export function HandoutsList({ gameId, isGM, gameState }: HandoutsListProps) {
  const { showError } = useToast();
  const [searchParams, setSearchParams] = useSearchParams();
  const {
    handouts,
    isLoading,
    createHandoutMutation,
    updateHandoutMutation,
    deleteHandoutMutation,
    publishHandoutMutation,
    unpublishHandoutMutation
  } = useHandouts(gameId);

  const [showCreateModal, setShowCreateModal] = useState(false);
  const [editingHandout, setEditingHandout] = useState<Handout | null>(null);
  const [viewingHandout, setViewingHandout] = useState<Handout | null>(null);

  // Drive viewed handout entirely from the URL param — handles normal clicks,
  // direct links, and browser back/forward correctly.
  useEffect(() => {
    const handoutId = searchParams.get('handout');
    if (isLoading) return;
    if (!handoutId) {
      setViewingHandout(null);
      return;
    }
    const handout = handouts.find(h => h.id === parseInt(handoutId, 10));
    setViewingHandout(handout ?? null);
  }, [searchParams, handouts, isLoading]);

  const handleCloseView = () => {
    setSearchParams(prev => {
      const next = new URLSearchParams(prev);
      next.delete('handout');
      return next;
    });
  };

  const handleCreate = async (data: CreateHandoutRequest) => {
    try {
      await createHandoutMutation.mutateAsync(data);
      setShowCreateModal(false);
    } catch (error) {
      showError(error instanceof Error ? error.message : 'Failed to create handout');
    }
  };

  const handleUpdate = async (data: UpdateHandoutRequest) => {
    if (!editingHandout) return;

    try {
      await updateHandoutMutation.mutateAsync({
        handoutId: editingHandout.id,
        data
      });
      setEditingHandout(null);
    } catch (error) {
      showError(error instanceof Error ? error.message : 'Failed to update handout');
    }
  };

  const handleDelete = async (handout: Handout) => {
    try {
      await deleteHandoutMutation.mutateAsync(handout.id);
    } catch (error) {
      showError(error instanceof Error ? error.message : 'Failed to delete handout');
    }
  };

  const handlePublish = async (handout: Handout) => {
    try {
      await publishHandoutMutation.mutateAsync(handout.id);
    } catch (error) {
      showError(error instanceof Error ? error.message : 'Failed to publish handout');
    }
  };

  const handleUnpublish = async (handout: Handout) => {
    try {
      await unpublishHandoutMutation.mutateAsync(handout.id);
    } catch (error) {
      showError(error instanceof Error ? error.message : 'Failed to unpublish handout');
    }
  };

  // Filter handouts based on user role
  const visibleHandouts = isGM
    ? handouts
    : handouts.filter(h => h.status === 'published');

  if (viewingHandout) {
    return (
      <HandoutView
        gameId={gameId}
        handout={viewingHandout}
        isGM={isGM}
        onClose={handleCloseView}
        onEdit={isGM ? () => {
          handleCloseView();
          setEditingHandout(viewingHandout);
        } : undefined}
      />
    );
  }

  if (isLoading) {
    return (
      <Card variant="elevated" padding="lg">
        <h2 className="text-2xl font-bold text-content-primary mb-6">Handouts</h2>
        <div className="flex justify-center py-8">
          <Spinner size="lg" label="Loading handouts..." />
        </div>
      </Card>
    );
  }

  return (
    <>
      <Card variant="elevated" padding="lg" data-testid="handouts-list">
        <div className="flex items-center justify-between mb-6">
          <h2 className="text-2xl font-bold text-content-primary">Handouts</h2>
          {isGM && isGameWritable(gameState) && (
            <Button
              variant="primary"
              onClick={() => setShowCreateModal(true)}
              data-testid="create-handout-button"
            >
              Create Handout
            </Button>
          )}
        </div>

        {visibleHandouts.length === 0 ? (
          <Alert variant="info">
            {isGM
              ? 'No handouts yet. Create your first handout to share rules, lore, or reference materials with your players.'
              : 'No handouts available yet. Your GM hasn\'t published any handouts.'}
          </Alert>
        ) : (
          <div className="grid gap-4 md:grid-cols-2">
            {visibleHandouts.map(handout => {
              const canEditHandout = isGM && isGameWritable(gameState);
              return (
                <HandoutCard
                  key={handout.id}
                  handout={handout}
                  isGM={isGM}
                  onEdit={canEditHandout ? setEditingHandout : undefined}
                  onDelete={canEditHandout ? handleDelete : undefined}
                  onPublish={canEditHandout ? handlePublish : undefined}
                  onUnpublish={canEditHandout ? handleUnpublish : undefined}
                />
              );
            })}
          </div>
        )}
      </Card>

      {showCreateModal && (
        <CreateHandoutModal
          onClose={() => setShowCreateModal(false)}
          onSubmit={handleCreate}
          isSubmitting={createHandoutMutation.isPending}
        />
      )}

      {editingHandout && (
        <EditHandoutModal
          handout={editingHandout}
          onClose={() => setEditingHandout(null)}
          onSubmit={handleUpdate}
          isSubmitting={updateHandoutMutation.isPending}
        />
      )}
    </>
  );
}
