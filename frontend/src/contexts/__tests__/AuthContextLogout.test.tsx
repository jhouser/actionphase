import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('../../lib/api', () => ({
  apiClient: {
    auth: { logout: vi.fn().mockResolvedValue(undefined) },
    startLogout: vi.fn(),
    endLogout: vi.fn(),
    removeAuthToken: vi.fn(),
    getAuthToken: vi.fn().mockReturnValue(null),
    setAuthToken: vi.fn(),
  },
}));
vi.mock('../../lib/simple-api', () => ({ simpleApi: { get: vi.fn(), post: vi.fn() } }));
vi.mock('@/lib/faro', () => ({ setFaroUser: vi.fn(), clearFaroUser: vi.fn() }));
vi.mock('@/components/SessionExpiredModal', () => ({ SessionExpiredModal: () => null }));

import { AuthProvider, useAuth } from '../AuthContext';
import { postCachingService } from '@/services/PostCachingService';

function LogoutButton() {
  const { logout } = useAuth();
  return <button onClick={() => logout()}>Log out</button>;
}

function renderWithAuth() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <LogoutButton />
      </AuthProvider>
    </QueryClientProvider>
  );
}

describe('AuthContext logout', () => {
  beforeEach(() => {
    localStorage.clear();
    vi.clearAllMocks();
  });

  it('clears cached post drafts so they cannot leak to the next user', async () => {
    const user = userEvent.setup();
    postCachingService.save('action-1', 'my private action');
    postCachingService.save('conversation-2', 'my private message');

    renderWithAuth();
    await user.click(screen.getByRole('button', { name: 'Log out' }));

    await waitFor(() => {
      expect(postCachingService.get('action-1')).toBeUndefined();
    });
    expect(postCachingService.get('conversation-2')).toBeUndefined();
  });

  it('leaves unrelated stored preferences intact', async () => {
    const user = userEvent.setup();
    postCachingService.save('action-1', 'my private action');
    localStorage.setItem('app-theme', 'dark');

    renderWithAuth();
    await user.click(screen.getByRole('button', { name: 'Log out' }));

    await waitFor(() => {
      expect(postCachingService.get('action-1')).toBeUndefined();
    });
    expect(localStorage.getItem('app-theme')).toBe('dark');
  });
});
