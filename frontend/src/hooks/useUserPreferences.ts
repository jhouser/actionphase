import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '../lib/api';
import type { UserPreferences, CommentReadMode, FontSize } from '../lib/api/auth';

/**
 * Hook to fetch the current user's preferences from the server.
 * Uses a long staleTime since preferences rarely change.
 */
export function useUserPreferences(options?: { enabled?: boolean }) {
  return useQuery({
    queryKey: ['userPreferences'],
    queryFn: async () => {
      const response = await apiClient.auth.getPreferences();
      return response.data.preferences as UserPreferences;
    },
    enabled: options?.enabled,
    staleTime: 10 * 60 * 1000, // 10 minutes
    // Supply defaults while loading so consumers can rely on the shape
    placeholderData: {
      theme: 'auto' as const,
      comment_read_mode: 'manual' as CommentReadMode,
      font_size: 'medium' as FontSize,
    },
  });
}

/**
 * Mutation to update user preferences.
 * Invalidates the userPreferences cache on success.
 */
export function useUpdateUserPreferences() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (preferences: UserPreferences) => {
      const response = await apiClient.auth.updatePreferences(preferences);
      return response.data.preferences as UserPreferences;
    },
    onSuccess: (updated) => {
      // Write the updated value directly into the cache for instant UI response
      queryClient.setQueryData(['userPreferences'], updated);
    },
  });
}

/**
 * Convenience hook returning the user's comment read mode.
 * Defaults to 'auto' while loading.
 */
export function useCommentReadMode(): CommentReadMode {
  const { data } = useUserPreferences();
  return data?.comment_read_mode ?? 'manual';
}

/**
 * Convenience hook returning the user's font size preference.
 * Defaults to 'medium' while loading.
 */
export function useFontSize(options?: { enabled?: boolean }): FontSize {
  const { data } = useUserPreferences(options);
  return data?.font_size ?? 'medium';
}
