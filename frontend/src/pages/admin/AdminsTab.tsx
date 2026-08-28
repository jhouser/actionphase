import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '../../lib/api';
import { useAuth } from '../../contexts/AuthContext';
import { useToast } from '../../contexts/ToastContext';
import { ConfirmModal } from '../../components/ConfirmModal';
import { Badge } from '../../components/ui';
import type { AdminUser } from '../../lib/api/admin';

export function AdminsTab() {
  const queryClient = useQueryClient();
  const { currentUser } = useAuth();
  const { showSuccess, showError } = useToast();
  const currentUserId = currentUser?.id;
  const [userToRevoke, setUserToRevoke] = useState<AdminUser | null>(null);

  const { data: admins, isLoading, error } = useQuery({
    queryKey: ['admins'],
    queryFn: async () => {
      const response = await apiClient.admin.listAdmins();
      return response.data;
    },
    staleTime: 0,
  });

  const revokeAdminMutation = useMutation({
    mutationFn: (userId: number) => apiClient.admin.revokeAdminStatus(userId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admins'] });
      setUserToRevoke(null);
      showSuccess('Admin status revoked successfully');
    },
    onError: (err) => showError(`Failed to revoke admin status: ${err}`),
  });

  return (
    <div className="bg-surface-base rounded-lg shadow">
      <div className="px-6 py-4 border-b border-theme-default">
        <h2 className="text-xl font-semibold text-content-primary">Administrator Users</h2>
        <p className="text-sm text-content-tertiary mt-1">Users with administrative privileges</p>
      </div>

      {isLoading ? (
        <div className="px-6 py-8 text-center text-content-tertiary">Loading administrators...</div>
      ) : error ? (
        <div className="px-6 py-8 text-center text-red-500">
          Error loading administrators. You may not have admin permissions.
        </div>
      ) : !admins?.length ? (
        <div className="px-6 py-8 text-center text-content-tertiary">No administrators found</div>
      ) : (
        <div className="divide-y divide-theme-default">
          {admins.map((user: AdminUser) => (
            <div key={user.id} className="px-6 py-4 hover:bg-surface-raised">
              <div className="flex items-center justify-between">
                <div className="flex-1">
                  <div className="flex items-center space-x-3">
                    <h3 className="text-lg font-medium text-content-primary">{user.username}</h3>
                    <Badge variant="primary" size="sm">ADMIN</Badge>
                    {user.id === currentUserId && (
                      <Badge variant="neutral" size="sm">YOU</Badge>
                    )}
                  </div>
                  <p className="text-sm text-content-secondary mt-1">{user.email}</p>
                  <p className="text-xs text-content-tertiary mt-1">
                    Created: {new Date(user.createdAt).toLocaleDateString()}
                  </p>
                </div>
                {user.id !== currentUserId && (
                  <button
                    onClick={() => setUserToRevoke(user)}
                    disabled={revokeAdminMutation.isPending}
                    className="ml-4 px-4 py-2 bg-red-600 text-white rounded hover:bg-red-700 disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    Revoke Admin
                  </button>
                )}
              </div>
            </div>
          ))}
        </div>
      )}

      <ConfirmModal
        isOpen={userToRevoke !== null}
        onClose={() => setUserToRevoke(null)}
        onConfirm={() => revokeAdminMutation.mutate(userToRevoke!.id)}
        title="Revoke Admin Status"
        message={`Are you sure you want to revoke admin status from ${userToRevoke?.username}?`}
        confirmText="Revoke"
        variant="danger"
        isLoading={revokeAdminMutation.isPending}
      />
    </div>
  );
}
