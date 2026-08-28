import { useState, useEffect } from 'react';
import { Link, useSearchParams, useNavigate } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { apiClient } from '../lib/api';
import { Card, CardHeader, CardBody, Alert, Spinner, Button } from '../components/ui';

export function VerifyEmailPage() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const token = searchParams.get('token');

  const [isValidating, setIsValidating] = useState(true);
  const [success, setSuccess] = useState(false);
  const [error, setError] = useState('');

  // Verify email on mount
  useEffect(() => {
    const verifyEmail = async () => {
      if (!token) {
        setIsValidating(false);
        setError('Invalid verification link');
        return;
      }

      try {
        await apiClient.auth.verifyEmail(token);
        setSuccess(true);
        // Invalidate currentUser query to refetch updated user data with email_verified = true
        queryClient.invalidateQueries({ queryKey: ['currentUser'] });
        // Redirect to dashboard after 3 seconds
        setTimeout(() => {
          navigate('/dashboard');
        }, 3000);
      } catch (err: unknown) {
        setError((err as { response?: { data?: { error?: string } } })?.response?.data?.error || 'This verification link is invalid or has expired');
      } finally {
        setIsValidating(false);
      }
    };

    verifyEmail();
  }, [token, navigate, queryClient]);

  // Loading state while verifying
  if (isValidating) {
    return (
      <div className="flex items-center justify-center min-h-screen surface-base">
        <Card variant="elevated" padding="md" className="max-w-md w-full">
          <CardBody>
            <div className="flex flex-col items-center justify-center py-8">
              <Spinner size="lg" />
              <p className="mt-4 text-content-secondary">Verifying your email...</p>
            </div>
          </CardBody>
        </Card>
      </div>
    );
  }

  // Success state
  if (success) {
    return (
      <div className="flex items-center justify-center min-h-screen surface-base">
        <Card variant="elevated" padding="md" className="max-w-md w-full">
          <CardHeader>
            <h2 className="text-2xl font-bold text-content-primary">Email Verified!</h2>
          </CardHeader>
          <CardBody>
            <Alert variant="success" className="mb-4">
              Your email has been successfully verified.
            </Alert>
            <p className="text-content-secondary mb-4">
              You now have full access to all features. You will be redirected to your dashboard in a few seconds...
            </p>
            <Link to="/dashboard">
              <Button variant="primary" className="w-full">
                Go to Dashboard
              </Button>
            </Link>
          </CardBody>
        </Card>
      </div>
    );
  }

  // Error state
  return (
    <div className="flex items-center justify-center min-h-screen surface-base">
      <Card variant="elevated" padding="md" className="max-w-md w-full">
        <CardHeader>
          <h2 className="text-2xl font-bold text-content-primary">Verification Failed</h2>
        </CardHeader>
        <CardBody>
          <Alert variant="danger" className="mb-4">
            {error || 'This verification link is invalid or has expired.'}
          </Alert>
          <p className="text-content-secondary mb-4">
            Email verification links expire after 24 hours. If you need a new verification link, you can request one from your settings.
          </p>
          <div className="space-y-2">
            <Link to="/dashboard">
              <Button variant="primary" className="w-full">
                Go to Dashboard
              </Button>
            </Link>
            <Link to="/login">
              <Button variant="secondary" className="w-full">
                Back to Login
              </Button>
            </Link>
          </div>
        </CardBody>
      </Card>
    </div>
  );
}
