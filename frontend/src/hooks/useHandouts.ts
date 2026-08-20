import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '../lib/api';
import type { CreateHandoutRequest, UpdateHandoutRequest } from '../types/handouts';

/**
 * Custom hook for managing handouts
 * Provides queries and mutations for handout operations
 */
export function useHandouts(gameId: number) {
  const queryClient = useQueryClient();

  // Query for all handouts
  const { data: handoutsData, isLoading, isError } = useQuery({
    queryKey: ['handouts', gameId],
    queryFn: () => apiClient.handouts.listHandouts(gameId).then(res => res.data),
    enabled: !!gameId,
    refetchOnMount: 'always',
    staleTime: 0
  });

  // Ensure handouts is always an array
  const handouts = handoutsData || [];

  // Mutation for creating a new handout
  const createHandoutMutation = useMutation({
    mutationFn: (data: CreateHandoutRequest) => apiClient.handouts.createHandout(gameId, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['handouts', gameId] });
    }
  });

  // Mutation for updating a handout
  const updateHandoutMutation = useMutation({
    mutationFn: ({ handoutId, data }: { handoutId: number; data: UpdateHandoutRequest }) =>
      apiClient.handouts.updateHandout(gameId, handoutId, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['handouts', gameId] });
    }
  });

  // Mutation for deleting a handout
  const deleteHandoutMutation = useMutation({
    mutationFn: (handoutId: number) => apiClient.handouts.deleteHandout(gameId, handoutId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['handouts', gameId] });
    }
  });

  // Mutation for publishing a handout
  const publishHandoutMutation = useMutation({
    mutationFn: (handoutId: number) => apiClient.handouts.publishHandout(gameId, handoutId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['handouts', gameId] });
    }
  });

  // Mutation for unpublishing a handout
  const unpublishHandoutMutation = useMutation({
    mutationFn: (handoutId: number) => apiClient.handouts.unpublishHandout(gameId, handoutId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['handouts', gameId] });
    }
  });

  return {
    handouts,
    isLoading,
    isError,
    createHandoutMutation,
    updateHandoutMutation,
    deleteHandoutMutation,
    publishHandoutMutation,
    unpublishHandoutMutation,
  };
}

/**
 * Hook for fetching a single handout
 */
export function useHandout(gameId: number, handoutId: number) {
  return useQuery({
    queryKey: ['handout', gameId, handoutId],
    queryFn: () => apiClient.handouts.getHandout(gameId, handoutId).then(res => res.data),
    enabled: !!gameId && !!handoutId,
  });
}
