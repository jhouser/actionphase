import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '../lib/api';
import { Card, CardHeader, CardBody, Input, Button, Alert } from './ui';
import { useToast } from '../contexts/ToastContext';
import { useAuth } from '../contexts/AuthContext';

export function ChangeUsernameForm() {
  const { showToast } = useToast();
  const { currentUser } = useAuth();
  const queryClient = useQueryClient();
  const [newUsername, setNewUsername] = useState('');
  const [currentPassword, setCurrentPassword] = useState('');
  const [error, setError] = useState<string | null>(null);

  const changeUsernameMutation = useMutation({
    mutationFn: async (data: { new_username: string; current_password: string }) => {
      await apiClient.auth.changeUsername(data);
    },
    onSuccess: () => {
      showToast('Username changed successfully!', 'success');
      setNewUsername('');
      setCurrentPassword('');
      setError(null);
      // Invalidate current user query to refetch with new username
      queryClient.invalidateQueries({ queryKey: ['currentUser'] });
    },
    onError: (error: unknown) => {
      const message = (error as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Failed to change username';
      setError(message);
      showToast(message, 'danger');
    },
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (!newUsername.trim()) {
      setError('New username is required');
      return;
    }

    if (!currentPassword) {
      setError('Current password is required');
      return;
    }

    changeUsernameMutation.mutate({
      new_username: newUsername.trim(),
      current_password: currentPassword,
    });
  };

  return (
    <Card variant="default" padding="md" data-testid="change-username-form">
      <CardHeader>
        <h3 className="text-lg font-semibold text-content-primary">Change Username</h3>
        <p className="text-sm text-content-secondary mt-1" data-testid="current-username-display">
          Current username: <span className="font-medium">{currentUser?.username}</span>
        </p>
      </CardHeader>
      <CardBody>
        <form onSubmit={handleSubmit} className="space-y-4">
          {error && (
            <Alert variant="danger" dismissible onDismiss={() => setError(null)}>
              {error}
            </Alert>
          )}

          <Input
            id="new-username"
            label="New Username"
            type="text"
            value={newUsername}
            onChange={(e) => setNewUsername(e.target.value)}
            placeholder="Enter new username"
            disabled={changeUsernameMutation.isPending}
            data-testid="new-username-input"
          />

          <Input
            id="current-password"
            label="Current Password"
            type="password"
            value={currentPassword}
            onChange={(e) => setCurrentPassword(e.target.value)}
            placeholder="Confirm with your password"
            disabled={changeUsernameMutation.isPending}
            data-testid="username-current-password-input"
          />

          <Button
            type="submit"
            variant="primary"
            loading={changeUsernameMutation.isPending}
            data-testid="change-username-submit"
          >
            Change Username
          </Button>
        </form>
      </CardBody>
    </Card>
  );
}
