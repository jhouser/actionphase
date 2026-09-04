import { useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { apiClient } from '../lib/api';
import { Alert, Button } from './ui';
import { useToast } from '../contexts/ToastContext';
import { extractApiErrorMessage } from '@/lib/errors';

export function EmailVerificationBanner() {
  const { showToast } = useToast();
  const [dismissed, setDismissed] = useState(false);

  const resendEmailMutation = useMutation({
    mutationFn: async () => {
      await apiClient.auth.resendVerificationEmail();
    },
    onSuccess: () => {
      showToast('Verification email sent! Please check your inbox.', 'success');
    },
    onError: (error: unknown) => {
      const message = extractApiErrorMessage(error) || 'Failed to send verification email';
      showToast(message, 'danger');
    },
  });

  if (dismissed) {
    return null;
  }

  return (
    <Alert
      variant="warning"
      dismissible
      onDismiss={() => setDismissed(true)}
    >
      <div className="flex items-center justify-between gap-4">
        <div>
          <strong>Email not verified</strong> - Please verify your email address to access all features.
        </div>
        <Button
          variant="secondary"
          size="sm"
          onClick={() => resendEmailMutation.mutate()}
          loading={resendEmailMutation.isPending}
        >
          Resend Email
        </Button>
      </div>
    </Alert>
  );
}
