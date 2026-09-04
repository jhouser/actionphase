import { describe, it, expect } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { UserProfilePage } from './UserProfilePage';
import { server } from '../mocks/server';
import { http, HttpResponse } from 'msw';

// Helper to render component with all required providers
const renderWithProviders = (username: string = 'testuser') => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        gcTime: 0,
      },
    },
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[`/users/${username}`]}>
        <Routes>
          <Route path="/users/:username" element={<UserProfilePage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  );
};

describe('UserProfilePage', () => {
  it('renders loading state initially', () => {
    renderWithProviders();
    // Check for loading spinner by role instead of text (to avoid duplicate text issue)
    expect(screen.getByRole('status')).toBeInTheDocument();
  });

  it('renders user profile when data loads successfully', async () => {
    renderWithProviders();

    // Wait for loading to complete
    await waitFor(() => {
      expect(screen.queryByRole('status')).not.toBeInTheDocument();
    });

    // Check user info is displayed
    expect(screen.getByText('Test User')).toBeInTheDocument();
    expect(screen.getByText('@testuser')).toBeInTheDocument();
    expect(screen.getByText('This is a test bio')).toBeInTheDocument();
  });

  it('renders game history when data loads successfully', async () => {
    renderWithProviders();

    await waitFor(() => {
      expect(screen.queryByRole('status')).not.toBeInTheDocument();
    });

    // Check games are displayed
    expect(screen.getByText('Test Game 1')).toBeInTheDocument();
    expect(screen.getByText('Test Game 2')).toBeInTheDocument();
  });

  it('renders error state when API call fails', async () => {
    // Mock API error
    server.use(
      http.get('/api/v1/users/username/:username/profile', () => {
        return HttpResponse.json(
          { detail: 'User not found' },
          { status: 404 }
        );
      })
    );

    renderWithProviders('nonexistent');

    await waitFor(() => {
      expect(screen.getByText('Failed to load profile')).toBeInTheDocument();
    });

    // Check retry button is present
    expect(screen.getByRole('button', { name: /try again/i })).toBeInTheDocument();
  });

  it('displays error message for network errors', async () => {
    // Mock network error
    server.use(
      http.get('/api/v1/users/username/:username/profile', () => {
        return HttpResponse.error();
      })
    );

    renderWithProviders();

    await waitFor(() => {
      expect(screen.getByText('Failed to load profile')).toBeInTheDocument();
    });

    // Should show network error message
    expect(screen.getByText('Network Error')).toBeInTheDocument();
  });

  it('shows invalid username message when username is empty', () => {
    // Render with empty route param
    const queryClient = new QueryClient();

    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={['/users/']}>
          <Routes>
            <Route path="/users/:username?" element={<UserProfilePage />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>
    );

    expect(screen.getByText('Invalid username')).toBeInTheDocument();
    expect(screen.getByText('Please provide a valid username in the URL.')).toBeInTheDocument();
  });
});
