import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '../lib/api';
import { useToast } from '../contexts/ToastContext';
import type { CreatePhaseRequest, UpdatePhaseRequest, UpdateDeadlineRequest } from '../types/phases';
import { logger } from '@/services/LoggingService';

/**
 * Custom hook for managing game phases
 * Provides queries and mutations for phase CRUD operations
 */
export function usePhaseManagement(gameId: number) {
  const { showError } = useToast();
  const queryClient = useQueryClient();

  // Query for all game phases
  const { data: phasesData, isLoading } = useQuery({
    queryKey: ['gamePhases', gameId],
    queryFn: () => apiClient.phases.getGamePhases(gameId).then(res => res.data),
    enabled: !!gameId,
    refetchOnMount: 'always',
    staleTime: 0
  });

  // Ensure phases is always an array
  const phases = phasesData || [];

  // Mutation for creating a new phase
  const createPhaseMutation = useMutation({
    mutationFn: (data: CreatePhaseRequest) => apiClient.phases.createPhase(gameId, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['gamePhases', gameId] });
    }
  });

  // Mutation for activating a phase
  const activatePhaseMutation = useMutation({
    mutationFn: (phaseId: number) => apiClient.phases.activatePhase(phaseId),
    onSuccess: async () => {
      // Force immediate refetch instead of just invalidation
      await queryClient.refetchQueries({ queryKey: ['gamePhases', gameId] });
      // Activation is the one mutation that changes which phase is active, so
      // give the GM instant feedback on the separate ['currentPhase'] query
      // (GameDetailsPage) rather than waiting on its poll. Other mutations
      // don't change the active phase, so they rely on the poll — keeping the
      // request volume down was the point of dropping the per-mutation
      // currentPhase invalidations.
      queryClient.invalidateQueries({ queryKey: ['currentPhase', gameId] });
    },
    onError: (error) => {
      logger.error('Failed to activate phase', { error, gameId });
      showError(error instanceof Error ? error.message : 'Failed to activate phase');
    }
  });

  // Mutation for updating phase deadline
  const updateDeadlineMutation = useMutation({
    mutationFn: ({ phaseId, data }: { phaseId: number; data: UpdateDeadlineRequest }) =>
      apiClient.phases.updatePhaseDeadline(phaseId, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['gamePhases', gameId] });
    }
  });

  // Mutation for updating phase details
  const updatePhaseMutation = useMutation({
    mutationFn: ({ phaseId, data }: { phaseId: number; data: UpdatePhaseRequest }) =>
      apiClient.phases.updatePhase(phaseId, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['gamePhases', gameId] });
    }
  });

  // Mutation for deleting a phase
  const deletePhaseMutation = useMutation({
    mutationFn: (phaseId: number) => apiClient.phases.deletePhase(phaseId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['gamePhases', gameId] });
    },
    onError: (error) => {
      logger.error('Failed to delete phase', { error, gameId });
      // Error is shown in the DeletePhaseDialog component
      throw error;
    }
  });

  // getGamePhases already returns every phase including the active one, so
  // this is derived rather than fetched separately — see the removed
  // getCurrentPhase query this replaced.
  const currentPhase = phases.find(p => p.is_active);

  return {
    phases,
    currentPhase,
    isLoading,
    createPhaseMutation,
    activatePhaseMutation,
    updateDeadlineMutation,
    updatePhaseMutation,
    deletePhaseMutation,
  };
}
