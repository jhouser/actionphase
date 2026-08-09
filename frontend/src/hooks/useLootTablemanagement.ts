import { useToast } from "@/contexts/ToastContext";
import { apiClient } from "@/lib/api";
import type { CreateLootTableRequest } from "@/types/games";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

export function useLootTableManagement(gameId: number) {
    const { showError } = useToast();
    const queryClient = useQueryClient();

    // Query for all loot tables
    const { data: lootTablesData, isLoading } = useQuery({
        queryKey: ['lootTables', gameId],
        queryFn: () => apiClient.games.getLootTables(gameId).then(res => res.data),
        enabled: !!gameId,
        refetchOnMount: 'always',
        staleTime: 0
    });

    const createLootTableMutation = useMutation({
      mutationFn: (data: CreateLootTableRequest) => apiClient.games.createLootTable(gameId, data),
      onSuccess: () => {
        queryClient.invalidateQueries({ queryKey: ['lootTables', gameId] });
      }
    });

    const deleteLootTableMutation = useMutation({
      mutationFn: (lootTableId: number) => apiClient.games.deleteLootTable(gameId, lootTableId),
      onSuccess: () => {
          queryClient.invalidateQueries({ queryKey: ['lootTables', gameId] });
      }
  });

    return {
        lootTables: lootTablesData || [],
        isLoading,
        createLootTableMutation,
        deleteLootTableMutation
    }
}