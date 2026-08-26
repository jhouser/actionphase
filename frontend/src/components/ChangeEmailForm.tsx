import { useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { apiClient } from '../lib/api';
import { Card, CardHeader, CardBody, Input, Button, Alert } from './ui';
import { useToast } from '../contexts/ToastContext';
import { useAuth } from '../contexts/AuthContext';

export function ChangeEmailForm() {
  const { showToast } = useToast();
  const { currentUser } = useAuth();
  const [newEmail, setNewEmail] = useState('');
  const [currentPassword, setCurrentPassword] = useState('');
  const [error, setError] = useState<string | null>(null);

  const changeEmailMutation = useMutation({
    mutationFn: async (data: { new_email: string; current_password: string }) => {
      await apiClient.auth.requestEmailChange(data);
    },
    onSuccess: () => {
      showToast('Verification email sent to your new email address. Please check your inbox.', 'success');
      setNewEmail('');
      setCurrentPassword('');
      setError(null);
    },
    onError: (error: unknown) => {
      const message = (error as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Failed to request email change';
      setError(message);
      showToast(message, 'danger');
    },
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (!newEmail.trim()) {
      setError('New email is required');
      return;
    }

    // Basic email validation
    const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
    if (!emailRegex.test(newEmail)) {
      setError('Please enter a valid email address');
      return;
    }

    if (!currentPassword) {
      setError('Current password is required');
      return;
    }

    changeEmailMutation.mutate({
      new_email: newEmail.trim(),
      current_password: currentPassword,
    });
  };

  return (
    <Card variant="default" padding="md" data-testid="change-email-form">
      <CardHeader>
        <h3 className="text-lg font-semibold text-content-primary">Change Email</h3>
        <p className="text-sm text-content-secondary mt-1" data-testid="current-email-display">
          Current email: <span className="font-medium">{currentUser?.email}</span>
        </p>
      </CardHeader>
      <CardBody>
        <form onSubmit={handleSubmit} className="space-y-4">
          {error && (
            <Alert variant="danger" dismissible onDismiss={() => setError(null)}>
              {error}
            </Alert>
          )}

          <Alert variant="info" data-testid="email-verification-info">
            A verification email will be sent to your new email address. You must verify it to complete the change.
          </Alert>

          <Input
            id="new-email"
            label="New Email"
            type="email"
            value={newEmail}
            onChange={(e) => setNewEmail(e.target.value)}
            placeholder="Enter new email address"
            disabled={changeEmailMutation.isPending}
            data-testid="new-email-input"
          />

          <Input
            id="current-password"
            label="Current Password"
            type="password"
            value={currentPassword}
            onChange={(e) => setCurrentPassword(e.target.value)}
            placeholder="Confirm with your password"
            disabled={changeEmailMutation.isPending}
            data-testid="email-current-password-input"
          />

          <Button
            type="submit"
            variant="primary"
            loading={changeEmailMutation.isPending}
            data-testid="change-email-submit"
          >
            Send Verification Email
          </Button>
        </form>
      </CardBody>
    </Card>
  );
}
