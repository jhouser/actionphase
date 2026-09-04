import { describe, it, expect, beforeEach } from 'vitest';
import { screen, fireEvent, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { ResetPasswordPage } from './ResetPasswordPage';
import { renderWithProviders } from '../test-utils/render';
import { server } from '../mocks/server';

describe('ResetPasswordPage', () => {
  beforeEach(() => {
    server.resetHandlers();
  });

  it('shows invalid token screen when no token provided', async () => {
    renderWithProviders(<ResetPasswordPage />, {
      initialRoute: '/reset-password',
    });

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Invalid Link' })).toBeInTheDocument();
      expect(screen.getByText('Invalid password reset link')).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Request New Link' })).toBeInTheDocument();
    });

    const requestNewLinkButton = screen.getByRole('button', { name: 'Request New Link' });
    expect(requestNewLinkButton.closest('a')).toHaveAttribute('href', '/forgot-password');
  });

  it('shows invalid token screen when token validation fails', async () => {
    server.use(
      http.get('http://localhost:3000/api/v1/auth/validate-reset-token', async () => {
        return HttpResponse.json(
          { detail: 'Token expired' },
          { status: 400 }
        );
      })
    );

    renderWithProviders(<ResetPasswordPage />, {
      initialRoute: '/reset-password?token=expired-token',
    });

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Invalid Link' })).toBeInTheDocument();
      expect(screen.getByText('This password reset link is invalid or has expired')).toBeInTheDocument();
    });
  });

  it('renders reset password form with valid token', async () => {
    server.use(
      http.get('http://localhost:3000/api/v1/auth/validate-reset-token', async () => {
        return HttpResponse.json({ valid: true });
      })
    );

    renderWithProviders(<ResetPasswordPage />, {
      initialRoute: '/reset-password?token=valid-token-123',
    });

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Reset Your Password' })).toBeInTheDocument();
      expect(screen.getByLabelText('New Password')).toBeInTheDocument();
      expect(screen.getByLabelText('Confirm New Password')).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Reset Password' })).toBeInTheDocument();
    });
  });

  it('has a link back to login', async () => {
    server.use(
      http.get('http://localhost:3000/api/v1/auth/validate-reset-token', async () => {
        return HttpResponse.json({ valid: true });
      })
    );

    renderWithProviders(<ResetPasswordPage />, {
      initialRoute: '/reset-password?token=valid-token-123',
    });

    await waitFor(() => {
      const loginLink = screen.getByRole('link', { name: 'Back to Login' });
      expect(loginLink).toBeInTheDocument();
      expect(loginLink).toHaveAttribute('href', '/login');
    });
  });


  it('shows validation error for password too short', async () => {
    server.use(
      http.get('http://localhost:3000/api/v1/auth/validate-reset-token', async () => {
        return HttpResponse.json({ valid: true });
      })
    );

    renderWithProviders(<ResetPasswordPage />, {
      initialRoute: '/reset-password?token=valid-token-123',
    });

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Reset Password' })).toBeInTheDocument();
    });

    const newPasswordInput = screen.getByLabelText('New Password');
    const confirmPasswordInput = screen.getByLabelText('Confirm New Password');
    const submitButton = screen.getByRole('button', { name: 'Reset Password' });

    fireEvent.change(newPasswordInput, { target: { value: 'Short1!' } });
    fireEvent.change(confirmPasswordInput, { target: { value: 'Short1!' } });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText('Password must be at least 8 characters')).toBeInTheDocument();
    });
  });

  it('shows validation error for password missing uppercase', async () => {
    server.use(
      http.get('http://localhost:3000/api/v1/auth/validate-reset-token', async () => {
        return HttpResponse.json({ valid: true });
      })
    );

    renderWithProviders(<ResetPasswordPage />, {
      initialRoute: '/reset-password?token=valid-token-123',
    });

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Reset Password' })).toBeInTheDocument();
    });

    const newPasswordInput = screen.getByLabelText('New Password');
    const confirmPasswordInput = screen.getByLabelText('Confirm New Password');
    const submitButton = screen.getByRole('button', { name: 'Reset Password' });

    fireEvent.change(newPasswordInput, { target: { value: 'lowercase123!' } });
    fireEvent.change(confirmPasswordInput, { target: { value: 'lowercase123!' } });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText('Password must contain at least one uppercase letter')).toBeInTheDocument();
    });
  });

  it('shows validation error for password missing lowercase', async () => {
    server.use(
      http.get('http://localhost:3000/api/v1/auth/validate-reset-token', async () => {
        return HttpResponse.json({ valid: true });
      })
    );

    renderWithProviders(<ResetPasswordPage />, {
      initialRoute: '/reset-password?token=valid-token-123',
    });

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Reset Password' })).toBeInTheDocument();
    });

    const newPasswordInput = screen.getByLabelText('New Password');
    const confirmPasswordInput = screen.getByLabelText('Confirm New Password');
    const submitButton = screen.getByRole('button', { name: 'Reset Password' });

    fireEvent.change(newPasswordInput, { target: { value: 'UPPERCASE123!' } });
    fireEvent.change(confirmPasswordInput, { target: { value: 'UPPERCASE123!' } });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText('Password must contain at least one lowercase letter')).toBeInTheDocument();
    });
  });

  it('shows validation error for password missing number', async () => {
    server.use(
      http.get('http://localhost:3000/api/v1/auth/validate-reset-token', async () => {
        return HttpResponse.json({ valid: true });
      })
    );

    renderWithProviders(<ResetPasswordPage />, {
      initialRoute: '/reset-password?token=valid-token-123',
    });

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Reset Password' })).toBeInTheDocument();
    });

    const newPasswordInput = screen.getByLabelText('New Password');
    const confirmPasswordInput = screen.getByLabelText('Confirm New Password');
    const submitButton = screen.getByRole('button', { name: 'Reset Password' });

    fireEvent.change(newPasswordInput, { target: { value: 'NoNumbers!' } });
    fireEvent.change(confirmPasswordInput, { target: { value: 'NoNumbers!' } });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText('Password must contain at least one number')).toBeInTheDocument();
    });
  });

  it('shows validation error for password missing special character', async () => {
    server.use(
      http.get('http://localhost:3000/api/v1/auth/validate-reset-token', async () => {
        return HttpResponse.json({ valid: true });
      })
    );

    renderWithProviders(<ResetPasswordPage />, {
      initialRoute: '/reset-password?token=valid-token-123',
    });

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Reset Password' })).toBeInTheDocument();
    });

    const newPasswordInput = screen.getByLabelText('New Password');
    const confirmPasswordInput = screen.getByLabelText('Confirm New Password');
    const submitButton = screen.getByRole('button', { name: 'Reset Password' });

    fireEvent.change(newPasswordInput, { target: { value: 'NoSpecial123' } });
    fireEvent.change(confirmPasswordInput, { target: { value: 'NoSpecial123' } });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText('Password must contain at least one special character')).toBeInTheDocument();
    });
  });


  it('shows validation error when passwords do not match', async () => {
    server.use(
      http.get('http://localhost:3000/api/v1/auth/validate-reset-token', async () => {
        return HttpResponse.json({ valid: true });
      })
    );

    renderWithProviders(<ResetPasswordPage />, {
      initialRoute: '/reset-password?token=valid-token-123',
    });

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Reset Password' })).toBeInTheDocument();
    });

    const newPasswordInput = screen.getByLabelText('New Password');
    const confirmPasswordInput = screen.getByLabelText('Confirm New Password');
    const submitButton = screen.getByRole('button', { name: 'Reset Password' });

    fireEvent.change(newPasswordInput, { target: { value: 'ValidPassword123!' } });
    fireEvent.change(confirmPasswordInput, { target: { value: 'DifferentPassword123!' } });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText('Passwords do not match')).toBeInTheDocument();
    });
  });


  it('submits form with valid data successfully', async () => {
    server.use(
      http.get('http://localhost:3000/api/v1/auth/validate-reset-token', async () => {
        return HttpResponse.json({ valid: true });
      }),
      http.post('http://localhost:3000/api/v1/auth/reset-password', async () => {
        return HttpResponse.json({ message: 'Password reset successfully' });
      })
    );

    renderWithProviders(<ResetPasswordPage />, {
      initialRoute: '/reset-password?token=valid-token-123',
    });

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Reset Password' })).toBeInTheDocument();
    });

    const newPasswordInput = screen.getByLabelText('New Password');
    const confirmPasswordInput = screen.getByLabelText('Confirm New Password');
    const submitButton = screen.getByRole('button', { name: 'Reset Password' });

    fireEvent.change(newPasswordInput, { target: { value: 'NewValidPassword123!' } });
    fireEvent.change(confirmPasswordInput, { target: { value: 'NewValidPassword123!' } });
    fireEvent.click(submitButton);

    // Success screen should appear
    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Password Reset Successful' })).toBeInTheDocument();
      expect(screen.getByText(/Your password has been successfully reset/i)).toBeInTheDocument();
      expect(screen.getByText(/You will be redirected to the login page/i)).toBeInTheDocument();
    });
  });

  it('shows error message when API call fails', async () => {
    server.use(
      http.get('http://localhost:3000/api/v1/auth/validate-reset-token', async () => {
        return HttpResponse.json({ valid: true });
      }),
      http.post('http://localhost:3000/api/v1/auth/reset-password', async () => {
        return HttpResponse.json(
          { detail: 'Token has expired' },
          { status: 400 }
        );
      })
    );

    renderWithProviders(<ResetPasswordPage />, {
      initialRoute: '/reset-password?token=valid-token-123',
    });

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Reset Password' })).toBeInTheDocument();
    });

    const newPasswordInput = screen.getByLabelText('New Password');
    const confirmPasswordInput = screen.getByLabelText('Confirm New Password');
    const submitButton = screen.getByRole('button', { name: 'Reset Password' });

    fireEvent.change(newPasswordInput, { target: { value: 'NewValidPassword123!' } });
    fireEvent.change(confirmPasswordInput, { target: { value: 'NewValidPassword123!' } });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText('Token has expired')).toBeInTheDocument();
    });
  });

  it('shows generic error message when API error has no specific message', async () => {
    server.use(
      http.get('http://localhost:3000/api/v1/auth/validate-reset-token', async () => {
        return HttpResponse.json({ valid: true });
      }),
      http.post('http://localhost:3000/api/v1/auth/reset-password', async () => {
        return HttpResponse.json({}, { status: 500 });
      })
    );

    renderWithProviders(<ResetPasswordPage />, {
      initialRoute: '/reset-password?token=valid-token-123',
    });

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Reset Password' })).toBeInTheDocument();
    });

    const newPasswordInput = screen.getByLabelText('New Password');
    const confirmPasswordInput = screen.getByLabelText('Confirm New Password');
    const submitButton = screen.getByRole('button', { name: 'Reset Password' });

    fireEvent.change(newPasswordInput, { target: { value: 'NewValidPassword123!' } });
    fireEvent.change(confirmPasswordInput, { target: { value: 'NewValidPassword123!' } });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText('Failed to reset password. Please try again.')).toBeInTheDocument();
    });
  });

  it('disables form inputs while submitting', async () => {
    server.use(
      http.get('http://localhost:3000/api/v1/auth/validate-reset-token', async () => {
        return HttpResponse.json({ valid: true });
      }),
      http.post('http://localhost:3000/api/v1/auth/reset-password', async () => {
        // Simulate slow API
        await new Promise(resolve => setTimeout(resolve, 100));
        return HttpResponse.json({ message: 'Password reset successfully' });
      })
    );

    renderWithProviders(<ResetPasswordPage />, {
      initialRoute: '/reset-password?token=valid-token-123',
    });

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Reset Password' })).toBeInTheDocument();
    });

    const newPasswordInput = screen.getByLabelText('New Password');
    const confirmPasswordInput = screen.getByLabelText('Confirm New Password');
    const submitButton = screen.getByRole('button', { name: 'Reset Password' });

    fireEvent.change(newPasswordInput, { target: { value: 'NewValidPassword123!' } });
    fireEvent.change(confirmPasswordInput, { target: { value: 'NewValidPassword123!' } });
    fireEvent.click(submitButton);

    // Inputs should be disabled during submission
    expect(newPasswordInput).toBeDisabled();
    expect(confirmPasswordInput).toBeDisabled();

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Password Reset Successful' })).toBeInTheDocument();
    });
  });

  it('displays helper text for password requirements', async () => {
    server.use(
      http.get('http://localhost:3000/api/v1/auth/validate-reset-token', async () => {
        return HttpResponse.json({ valid: true });
      })
    );

    renderWithProviders(<ResetPasswordPage />, {
      initialRoute: '/reset-password?token=valid-token-123',
    });

    await waitFor(() => {
      expect(screen.getByText(/Must be at least 8 characters/i)).toBeInTheDocument();
    });
  });
});
