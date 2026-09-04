import { describe, it, expect, beforeEach } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { VerifyEmailPage } from './VerifyEmailPage';
import { renderWithProviders } from '../test-utils/render';
import { server } from '../mocks/server';

// Mock useNavigate
const mockNavigate = vi.fn();
vi.mock('react-router-dom', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router-dom')>();
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

describe('VerifyEmailPage', () => {
  beforeEach(() => {
    server.resetHandlers();
    mockNavigate.mockClear();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('shows error when no token provided', async () => {
    renderWithProviders(<VerifyEmailPage />, {
      initialRoute: '/verify-email',
    });

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Verification Failed' })).toBeInTheDocument();
      expect(screen.getByText('Invalid verification link')).toBeInTheDocument();
    });
  });

  it('shows error when token verification fails', async () => {
    server.use(
      http.post('http://localhost:3000/api/v1/auth/verify-email', async () => {
        return HttpResponse.json(
          { detail: 'Token has expired' },
          { status: 400 }
        );
      })
    );

    renderWithProviders(<VerifyEmailPage />, {
      initialRoute: '/verify-email?token=expired-token',
    });

    // Should show loading state initially
    expect(screen.getByText('Verifying your email...')).toBeInTheDocument();

    // Then show error state
    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Verification Failed' })).toBeInTheDocument();
      expect(screen.getByText('Token has expired')).toBeInTheDocument();
    });
  });

  it('shows generic error when API error has no specific message', async () => {
    server.use(
      http.post('http://localhost:3000/api/v1/auth/verify-email', async () => {
        return HttpResponse.json({}, { status: 500 });
      })
    );

    renderWithProviders(<VerifyEmailPage />, {
      initialRoute: '/verify-email?token=valid-token',
    });

    await waitFor(() => {
      expect(screen.getByText('This verification link is invalid or has expired')).toBeInTheDocument();
    });
  });

  it('shows success screen and redirects to dashboard after verification', async () => {
    server.use(
      http.post('http://localhost:3000/api/v1/auth/verify-email', async () => {
        return HttpResponse.json({ message: 'Email verified successfully' });
      })
    );

    renderWithProviders(<VerifyEmailPage />, {
      initialRoute: '/verify-email?token=valid-token-123',
    });

    // Should show loading state initially
    expect(screen.getByText('Verifying your email...')).toBeInTheDocument();

    // Then show success state
    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Email Verified!' })).toBeInTheDocument();
      expect(screen.getByText(/Your email has been successfully verified/i)).toBeInTheDocument();
      expect(screen.getByText(/You will be redirected to your dashboard/i)).toBeInTheDocument();
    });

    // Verify redirect happens after 3 second timeout
    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/dashboard');
    }, { timeout: 4000 });
  });

  it('has link to dashboard in success state', async () => {
    server.use(
      http.post('http://localhost:3000/api/v1/auth/verify-email', async () => {
        return HttpResponse.json({ message: 'Email verified successfully' });
      })
    );

    renderWithProviders(<VerifyEmailPage />, {
      initialRoute: '/verify-email?token=valid-token-123',
    });

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Email Verified!' })).toBeInTheDocument();
    });

    const dashboardButton = screen.getByRole('link', { name: 'Go to Dashboard' });
    expect(dashboardButton).toHaveAttribute('href', '/dashboard');
  });

  it('has links to dashboard and login in error state', async () => {
    renderWithProviders(<VerifyEmailPage />, {
      initialRoute: '/verify-email',
    });

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Verification Failed' })).toBeInTheDocument();
    });

    const dashboardLink = screen.getByRole('link', { name: 'Go to Dashboard' });
    expect(dashboardLink).toHaveAttribute('href', '/dashboard');

    const loginLink = screen.getByRole('link', { name: 'Back to Login' });
    expect(loginLink).toHaveAttribute('href', '/login');
  });

  it('displays help text about link expiration in error state', async () => {
    renderWithProviders(<VerifyEmailPage />, {
      initialRoute: '/verify-email',
    });

    await waitFor(() => {
      expect(screen.getByText(/Email verification links expire after 24 hours/i)).toBeInTheDocument();
    });
  });

  it('shows spinner during verification', () => {
    server.use(
      http.post('http://localhost:3000/api/v1/auth/verify-email', async () => {
        // Delay response to keep loading state visible
        await new Promise(resolve => setTimeout(resolve, 100));
        return HttpResponse.json({ message: 'Email verified successfully' });
      })
    );

    renderWithProviders(<VerifyEmailPage />, {
      initialRoute: '/verify-email?token=valid-token',
    });

    expect(screen.getByText('Verifying your email...')).toBeInTheDocument();
    // Spinner should be present (testing library doesn't have a great way to test for Spinner component)
  });
});
