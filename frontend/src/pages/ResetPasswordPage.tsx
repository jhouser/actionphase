import { useState, useEffect } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { apiClient } from '../lib/api';
import { Input, Button, Card, CardHeader, CardBody, Alert, Spinner } from '../components/ui';

export function ResetPasswordPage() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const token = searchParams.get('token');

  const [formData, setFormData] = useState({
    new_password: '',
    confirm_password: '',
  });
  const [isLoading, setIsLoading] = useState(false);
  const [isValidating, setIsValidating] = useState(true);
  const [isTokenValid, setIsTokenValid] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState(false);
  const [validationErrors, setValidationErrors] = useState<Record<string, string>>({});

  // Validate token on mount
  useEffect(() => {
    const validateToken = async () => {
      if (!token) {
        setIsTokenValid(false);
        setIsValidating(false);
        setError('Invalid password reset link');
        return;
      }

      try {
        await apiClient.auth.validateResetToken(token);
        setIsTokenValid(true);
      } catch (_err) {
        setIsTokenValid(false);
        setError('This password reset link is invalid or has expired');
      } finally {
        setIsValidating(false);
      }
    };

    validateToken();
  }, [token]);

  const validate = () => {
    const errors: Record<string, string> = {};

    if (!formData.new_password) {
      errors.new_password = 'New password is required';
    } else if (formData.new_password.length < 8) {
      errors.new_password = 'Password must be at least 8 characters';
    } else if (!/[A-Z]/.test(formData.new_password)) {
      errors.new_password = 'Password must contain at least one uppercase letter';
    } else if (!/[a-z]/.test(formData.new_password)) {
      errors.new_password = 'Password must contain at least one lowercase letter';
    } else if (!/[0-9]/.test(formData.new_password)) {
      errors.new_password = 'Password must contain at least one number';
    } else if (!/[!@#$%^&*(),.?":{}|<>]/.test(formData.new_password)) {
      errors.new_password = 'Password must contain at least one special character';
    }

    if (!formData.confirm_password) {
      errors.confirm_password = 'Please confirm your new password';
    } else if (formData.new_password !== formData.confirm_password) {
      errors.confirm_password = 'Passwords do not match';
    }

    setValidationErrors(errors);
    return Object.keys(errors).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');

    if (!validate()) {
      return;
    }

    if (!token) {
      setError('Invalid password reset link');
      return;
    }

    setIsLoading(true);

    try {
      await apiClient.auth.resetPassword({
        token,
        new_password: formData.new_password,
        confirm_password: formData.confirm_password,
      });
      setSuccess(true);
      setTimeout(() => {
        navigate('/login');
      }, 3000);
    } catch (err: unknown) {
      setError((err as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Failed to reset password. Please try again.');
    } finally {
      setIsLoading(false);
    }
  };

  const handleChange = (field: keyof typeof formData) => (
    e: React.ChangeEvent<HTMLInputElement>
  ) => {
    setFormData((prev) => ({ ...prev, [field]: e.target.value }));
    // Clear error for this field when user starts typing
    if (validationErrors[field]) {
      setValidationErrors((prev) => ({ ...prev, [field]: '' }));
    }
  };

  // Loading state while validating token
  if (isValidating) {
    return (
      <div className="flex items-center justify-center min-h-screen surface-base">
        <Card variant="elevated" padding="md" className="max-w-md w-full">
          <CardBody>
            <div className="flex flex-col items-center justify-center py-8">
              <Spinner size="lg" />
              <p className="mt-4 text-content-secondary">Validating reset link...</p>
            </div>
          </CardBody>
        </Card>
      </div>
    );
  }

  // Invalid token state
  if (!isTokenValid) {
    return (
      <div className="flex items-center justify-center min-h-screen surface-base">
        <Card variant="elevated" padding="md" className="max-w-md w-full">
          <CardHeader>
            <h2 className="text-2xl font-bold text-content-primary">Invalid Link</h2>
          </CardHeader>
          <CardBody>
            <Alert variant="danger" className="mb-4">
              {error || 'This password reset link is invalid or has expired.'}
            </Alert>
            <p className="text-content-secondary mb-4">
              Password reset links expire after 1 hour. Please request a new password reset link.
            </p>
            <Link to="/forgot-password">
              <Button variant="primary" className="w-full">
                Request New Link
              </Button>
            </Link>
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
            <h2 className="text-2xl font-bold text-content-primary">Password Reset Successful</h2>
          </CardHeader>
          <CardBody>
            <Alert variant="success" className="mb-4">
              Your password has been successfully reset.
            </Alert>
            <p className="text-content-secondary mb-4">
              You will be redirected to the login page in a few seconds...
            </p>
            <Link to="/login">
              <Button variant="primary" className="w-full">
                Go to Login
              </Button>
            </Link>
          </CardBody>
        </Card>
      </div>
    );
  }

  // Reset password form
  return (
    <div className="flex items-center justify-center min-h-screen surface-base">
      <Card variant="elevated" padding="md" className="max-w-md w-full">
        <CardHeader>
          <h2 className="text-2xl font-bold text-content-primary">Reset Your Password</h2>
          <p className="text-content-secondary mt-2">
            Enter your new password below.
          </p>
        </CardHeader>
        <CardBody>
          <form onSubmit={handleSubmit} className="space-y-4">
            <Input
              label="New Password"
              id="new_password"
              type="password"
              required
              value={formData.new_password}
              onChange={handleChange('new_password')}
              error={validationErrors.new_password}
              disabled={isLoading}
              helperText="Must be at least 8 characters with uppercase, lowercase, number, and special character"
            />

            <Input
              label="Confirm New Password"
              id="confirm_password"
              type="password"
              required
              value={formData.confirm_password}
              onChange={handleChange('confirm_password')}
              error={validationErrors.confirm_password}
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
              Reset Password
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
