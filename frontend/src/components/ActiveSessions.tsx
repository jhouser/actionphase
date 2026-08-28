import { useState } from 'react';
import { useSuspenseQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '../lib/api';
import { Card, CardHeader, CardBody, Button, Alert, Badge } from './ui';
import { useToast } from '../contexts/ToastContext';

export function ActiveSessions() {
  const queryClient = useQueryClient();
  const { showToast } = useToast();
  const [revokeSessionId, setRevokeSessionId] = useState<number | null>(null);
  const [showRevokeAllConfirmation, setShowRevokeAllConfirmation] = useState(false);

  // Fetch sessions using useSuspenseQuery
  const { data } = useSuspenseQuery({
    queryKey: ['sessions'],
    queryFn: async () => {
      const response = await apiClient.auth.getSessions();
      return response.data;
    },
  });

  // Mutation for revoking a session
  const revokeMutation = useMutation({
    mutationFn: async (sessionId: number) => {
      await apiClient.auth.revokeSession(sessionId);
    },
    onSuccess: () => {
      // Invalidate and refetch sessions
      queryClient.invalidateQueries({ queryKey: ['sessions'] });
      showToast('Session revoked successfully', 'success');
      setRevokeSessionId(null);
    },
    onError: (error: unknown) => {
      const message = (error as { response?: { data?: { error?: string } } }).response?.data?.error || 'Failed to revoke session';
      showToast(message, 'danger');
      setRevokeSessionId(null);
    },
  });

  // Mutation for revoking all sessions except current
  const revokeAllMutation = useMutation({
    mutationFn: async () => {
      await apiClient.auth.revokeAllSessions();
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['sessions'] });
      showToast('All other sessions revoked successfully', 'success');
      setShowRevokeAllConfirmation(false);
    },
    onError: (error: unknown) => {
      const message = (error as { response?: { data?: { error?: string } } }).response?.data?.error || 'Failed to revoke all sessions';
      showToast(message, 'danger');
      setShowRevokeAllConfirmation(false);
    },
  });

  const handleRevokeClick = (sessionId: number) => {
    setRevokeSessionId(sessionId);
  };

  const handleConfirmRevoke = () => {
    if (revokeSessionId !== null) {
      revokeMutation.mutate(revokeSessionId);
    }
  };

  const handleCancelRevoke = () => {
    setRevokeSessionId(null);
  };

  const formatDate = (dateString: string) => {
    if (!dateString) return 'N/A';
    try {
      const date = new Date(dateString);
      return date.toLocaleString();
    } catch {
      return 'Invalid date';
    }
  };

  const sessions = data?.sessions || [];

  return (
    <Card variant="default" padding="md">
      <CardHeader>
        <div className="flex items-start justify-between">
          <div>
            <h3 className="text-lg font-semibold text-content-primary">Active Sessions</h3>
            <p className="text-sm text-content-secondary mt-1">
              Manage your active login sessions. You can revoke access from devices you no longer use.
            </p>
          </div>
          {sessions.length > 1 && !showRevokeAllConfirmation && (
            <Button
              variant="danger"
              onClick={() => setShowRevokeAllConfirmation(true)}
            >
              Revoke All Others
            </Button>
          )}
        </div>
        {showRevokeAllConfirmation && (
          <div className="mt-4 space-y-3">
            <Alert variant="warning">
              <strong>Are you sure?</strong> This will revoke all sessions except your current one. You'll need to log in again on other devices.
            </Alert>
            <div className="flex gap-2">
              <Button
                variant="danger"
                onClick={() => revokeAllMutation.mutate()}
                loading={revokeAllMutation.isPending}
              >
                Yes, Revoke All Other Sessions
              </Button>
              <Button
                variant="secondary"
                onClick={() => setShowRevokeAllConfirmation(false)}
                disabled={revokeAllMutation.isPending}
              >
                Cancel
              </Button>
            </div>
          </div>
        )}
      </CardHeader>
      <CardBody>
        {sessions.length === 0 ? (
          <Alert variant="info">
            No active sessions found.
          </Alert>
        ) : (
          <div className="space-y-3">
            {sessions.map((session) => (
              <div
                key={session.id}
                className="flex items-center justify-between p-4 border border-theme-default rounded-lg surface-raised"
              >
                <div className="flex-1">
                  <div className="flex items-center gap-2 mb-1">
                    <span className="font-medium text-content-secondary">
                      Session #{session.id}
                    </span>
                    {session.is_current && (
                      <Badge variant="success">Current Session</Badge>
                    )}
                  </div>
                  <div className="text-sm text-content-secondary space-y-1">
                    {session.created_at && (
                      <div>Created: {formatDate(session.created_at)}</div>
                    )}
                    <div>Expires: {formatDate(session.expires)}</div>
                  </div>
                </div>
                <div>
                  {!session.is_current && (
                    <>
                      {revokeSessionId === session.id ? (
                        <div className="flex gap-2">
                          <Button
                            variant="danger"
                            onClick={handleConfirmRevoke}
                            loading={revokeMutation.isPending}
                          >
                            Confirm
                          </Button>
                          <Button
                            variant="secondary"
                            onClick={handleCancelRevoke}
                            disabled={revokeMutation.isPending}
                          >
                            Cancel
                          </Button>
                        </div>
                      ) : (
                        <Button
                          variant="danger"
                          onClick={() => handleRevokeClick(session.id)}
                        >
                          Revoke
                        </Button>
                      )}
                    </>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
      </CardBody>
    </Card>
  );
}
