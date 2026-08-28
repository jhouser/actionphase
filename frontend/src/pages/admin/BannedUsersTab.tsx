import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '../../lib/api';
import { useToast } from '../../contexts/ToastContext';
import { ConfirmModal } from '../../components/ConfirmModal';
import { Badge } from '../../components/ui';
import type { BannedUser } from '../../lib/api/admin';

export function BannedUsersTab() {
  const queryClient = useQueryClient();
  const { showSuccess, showError } = useToast();
  const [userToUnban, setUserToUnban] = useState<BannedUser | null>(null);

  const { data: bannedUsers, isLoading, error } = useQuery({
    queryKey: ['bannedUsers'],
    queryFn: async () => {
      const response = await apiClient.admin.listBannedUsers();
      return response.data;
    },
    staleTime: 0,
  });

  const unbanMutation = useMutation({
    mutationFn: (userId: number) => apiClient.admin.unbanUser(userId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['bannedUsers'] });
      setUserToUnban(null);
      showSuccess('User unbanned successfully');
    },
    onError: (err) => showError(`Failed to unban user: ${err}`),
  });

  return (
    <div className="bg-surface-base rounded-lg shadow">
      <div className="px-6 py-4 border-b border-theme-default">
        <h2 className="text-xl font-semibold text-content-primary">Banned Users</h2>
        <p className="text-sm text-content-tertiary mt-1">
          Manage users who have been banned from the platform
        </p>
      </div>

      {isLoading ? (
        <div className="px-6 py-8 text-center text-content-tertiary">Loading banned users...</div>
      ) : error ? (
        <div className="px-6 py-8 text-center text-red-500">
          Error loading banned users. You may not have admin permissions.
        </div>
      ) : !bannedUsers?.length ? (
        <div className="px-6 py-8 text-center text-content-tertiary">No banned users</div>
      ) : (
        <div className="divide-y divide-theme-default">
          {bannedUsers.map((user: BannedUser) => (
            <div key={user.id} className="px-6 py-4 hover:bg-surface-raised">
              <div className="flex items-center justify-between">
                <div className="flex-1">
                  <div className="flex items-center space-x-3">
                    <h3 className="text-lg font-medium text-content-primary">{user.username}</h3>
                    <Badge variant="danger" size="sm">BANNED</Badge>
                  </div>
                  <p className="text-sm text-content-secondary mt-1">{user.email}</p>
                  <div className="text-xs text-content-tertiary mt-2">
                    <p>Banned by: <span className="font-medium">{user.banned_by_username}</span></p>
                    <p>Banned at: {new Date(user.banned_at).toLocaleString()}</p>
                  </div>
                </div>
                <button
                  onClick={() => setUserToUnban(user)}
                  disabled={unbanMutation.isPending}
                  className="ml-4 px-4 py-2 bg-green-600 text-white rounded hover:bg-green-700 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  Unban User
                </button>
              </div>
            </div>
          ))}
        </div>
      )}

      <ConfirmModal
        isOpen={userToUnban !== null}
        onClose={() => setUserToUnban(null)}
        onConfirm={() => unbanMutation.mutate(userToUnban!.id)}
        title="Unban User"
        message={`Are you sure you want to unban ${userToUnban?.username}?`}
        confirmText="Unban"
        variant="primary"
        isLoading={unbanMutation.isPending}
      />
    </div>
  );
}
