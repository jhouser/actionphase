import { useState } from 'react';
import { Link } from 'react-router-dom';
import { apiClient } from '../lib/api';
import { Input, Button, Card, CardHeader, CardBody, Alert } from '../components/ui';

export function ForgotPasswordPage() {
  const [email, setEmail] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setIsLoading(true);

    try {
      await apiClient.auth.requestPasswordReset(email);
      setSuccess(true);
    } catch (err: unknown) {
      setError((err as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Failed to request password reset. Please try again.');
    } finally {
      setIsLoading(false);
    }
  };

  if (success) {
    return (
      <div className="flex items-center justify-center min-h-screen surface-base">
        <Card variant="elevated" padding="md" className="max-w-md w-full">
          <CardHeader>
            <h2 className="text-2xl font-bold text-content-primary">Check Your Email</h2>
          </CardHeader>
          <CardBody>
            <Alert variant="success" className="mb-4">
              If an account exists with this email, you will receive a password reset link shortly.
            </Alert>
            <p className="text-content-secondary mb-4">
              Please check your inbox and follow the instructions in the email to reset your password.
            </p>
            <Link to="/login">
              <Button variant="secondary" className="w-full">
                Back to Login
              </Button>
            </Link>
          </CardBody>
        </Card>
      </div>
    );
  }

  return (
    <div className="flex items-center justify-center min-h-screen surface-base">
      <Card variant="elevated" padding="md" className="max-w-md w-full">
        <CardHeader>
          <h2 className="text-2xl font-bold text-content-primary">Forgot Password</h2>
          <p className="text-content-secondary mt-2">
            Enter your email address and we'll send you a link to reset your password.
          </p>
        </CardHeader>
        <CardBody>
          <form onSubmit={handleSubmit} className="space-y-4">
            <Input
              label="Email"
              id="email"
              type="email"
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="Enter your email"
              disabled={isLoading}
            />

            {error && (
              <Alert variant="danger">
                {error}
              </Alert>
            )}

            <Button
              type="submit"
              variant="primary"
              loading={isLoading}
              className="w-full"
            >
              Send Reset Link
            </Button>

            <div className="text-center">
              <Link
                to="/login"
                className="text-sm text-interactive-primary hover:text-interactive-hover"
              >
                Back to Login
              </Link>
            </div>
          </form>
        </CardBody>
      </Card>
    </div>
  );
}
