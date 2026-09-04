import { describe, it, expect, beforeEach } from 'vitest';
import { screen, fireEvent, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { ForgotPasswordPage } from './ForgotPasswordPage';
import { renderWithProviders } from '../test-utils/render';
import { server } from '../mocks/server';

describe('ForgotPasswordPage', () => {
  beforeEach(() => {
    server.resetHandlers();
  });

  it('renders forgot password form', () => {
    renderWithProviders(<ForgotPasswordPage />);

    expect(screen.getByRole('heading', { name: 'Forgot Password' })).toBeInTheDocument();
    expect(screen.getByText(/Enter your email address/i)).toBeInTheDocument();
    expect(screen.getByPlaceholderText('Enter your email')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Send Reset Link' })).toBeInTheDocument();
  });

  it('has a link back to login', () => {
    renderWithProviders(<ForgotPasswordPage />);

    const loginLink = screen.getByRole('link', { name: 'Back to Login' });
    expect(loginLink).toBeInTheDocument();
    expect(loginLink).toHaveAttribute('href', '/login');
  });

  it('has required attribute on email input', () => {
    renderWithProviders(<ForgotPasswordPage />);

    const emailInput = screen.getByPlaceholderText('Enter your email');
    expect(emailInput).toHaveAttribute('required');
    expect(emailInput).toHaveAttribute('type', 'email');
  });

  it('submits form with valid email successfully', async () => {
    server.use(
      http.post('http://localhost:3000/api/v1/auth/request-password-reset', async () => {
        return HttpResponse.json({ message: 'Password reset email sent' });
      })
    );

    renderWithProviders(<ForgotPasswordPage />);

    const emailInput = screen.getByPlaceholderText('Enter your email');
    const submitButton = screen.getByRole('button', { name: 'Send Reset Link' });

    fireEvent.change(emailInput, { target: { value: 'user@example.com' } });
    fireEvent.click(submitButton);

    // Success screen should appear
    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Check Your Email' })).toBeInTheDocument();
      expect(screen.getByText(/If an account exists with this email/i)).toBeInTheDocument();
      expect(screen.getByText(/check your inbox and follow the instructions/i)).toBeInTheDocument();
    });
  });

  it('shows error message when API call fails', async () => {
    server.use(
      http.post('http://localhost:3000/api/v1/auth/request-password-reset', async () => {
        return HttpResponse.json(
          { detail: 'Email not found' },
          { status: 404 }
        );
      })
    );

    renderWithProviders(<ForgotPasswordPage />);

    const emailInput = screen.getByPlaceholderText('Enter your email');
    const submitButton = screen.getByRole('button', { name: 'Send Reset Link' });

    fireEvent.change(emailInput, { target: { value: 'notfound@example.com' } });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText('Email not found')).toBeInTheDocument();
    });
  });

  it('shows generic error message when API error has no specific message', async () => {
    server.use(
      http.post('http://localhost:3000/api/v1/auth/request-password-reset', async () => {
        return HttpResponse.json({}, { status: 500 });
      })
    );

    renderWithProviders(<ForgotPasswordPage />);

    const emailInput = screen.getByPlaceholderText('Enter your email');
    const submitButton = screen.getByRole('button', { name: 'Send Reset Link' });

    fireEvent.change(emailInput, { target: { value: 'user@example.com' } });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText('Failed to request password reset. Please try again.')).toBeInTheDocument();
    });
  });

  it('disables form input while submitting', async () => {
    server.use(
      http.post('http://localhost:3000/api/v1/auth/request-password-reset', async () => {
        // Simulate slow API
        await new Promise(resolve => setTimeout(resolve, 200));
        return HttpResponse.json({ message: 'Password reset email sent' });
      })
    );

    renderWithProviders(<ForgotPasswordPage />);

    const emailInput = screen.getByPlaceholderText('Enter your email');
    const submitButton = screen.getByRole('button', { name: 'Send Reset Link' });

    fireEvent.change(emailInput, { target: { value: 'user@example.com' } });
    fireEvent.click(submitButton);

    // Input should be disabled during submission
    await waitFor(() => {
      expect(emailInput).toBeDisabled();
    });

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Check Your Email' })).toBeInTheDocument();
    });
  });

  it('trims whitespace from email', async () => {
    let requestBody: unknown = null;

    server.use(
      http.post('http://localhost:3000/api/v1/auth/request-password-reset', async ({ request }) => {
        requestBody = await request.json();
        return HttpResponse.json({ message: 'Password reset email sent' });
      })
    );

    renderWithProviders(<ForgotPasswordPage />);

    const emailInput = screen.getByPlaceholderText('Enter your email');
    const submitButton = screen.getByRole('button', { name: 'Send Reset Link' });

    fireEvent.change(emailInput, { target: { value: '  user@example.com  ' } });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(requestBody?.email).toBe('user@example.com');
    });
  });
});
