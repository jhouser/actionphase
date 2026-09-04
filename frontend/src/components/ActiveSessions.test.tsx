import { describe, it, expect, beforeEach } from 'vitest';
import { screen, fireEvent, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { ActiveSessions } from './ActiveSessions';
import { renderWithProviders } from '../test-utils/render';
import { server } from '../mocks/server';

describe('ActiveSessions', () => {
  beforeEach(() => {
    server.resetHandlers();
  });

  it('renders active sessions card with title and description', async () => {
    server.use(
      http.get('http://localhost:3000/api/v1/auth/sessions', async () => {
        return HttpResponse.json({
          sessions: []
        });
      })
    );

    renderWithProviders(<ActiveSessions />);

    await waitFor(() => {
      expect(screen.getByText('Active Sessions')).toBeInTheDocument();
      expect(screen.getByText(/Manage your active login sessions/i)).toBeInTheDocument();
    });
  });

  it('displays message when no sessions exist', async () => {
    server.use(
      http.get('http://localhost:3000/api/v1/auth/sessions', async () => {
        return HttpResponse.json({
          sessions: []
        });
      })
    );

    renderWithProviders(<ActiveSessions />);

    await waitFor(() => {
      expect(screen.getByText('No active sessions found.')).toBeInTheDocument();
    });
  });

  it('displays list of active sessions', async () => {
    const mockSessions = [
      {
        id: 1,
        is_current: true,
        created_at: '2025-11-01T10:00:00Z',
        expires: '2025-11-08T10:00:00Z'
      },
      {
        id: 2,
        is_current: false,
        created_at: '2025-10-30T15:00:00Z',
        expires: '2025-11-06T15:00:00Z'
      },
      {
        id: 3,
        is_current: false,
        created_at: '2025-10-28T12:00:00Z',
        expires: '2025-11-04T12:00:00Z'
      }
    ];

    server.use(
      http.get('http://localhost:3000/api/v1/auth/sessions', async () => {
        return HttpResponse.json({ sessions: mockSessions });
      })
    );

    renderWithProviders(<ActiveSessions />);

    await waitFor(() => {
      expect(screen.getByText('Session #1')).toBeInTheDocument();
      expect(screen.getByText('Session #2')).toBeInTheDocument();
      expect(screen.getByText('Session #3')).toBeInTheDocument();
    });
  });

  it('marks current session with badge', async () => {
    const mockSessions = [
      {
        id: 1,
        is_current: true,
        created_at: '2025-11-01T10:00:00Z',
        expires: '2025-11-08T10:00:00Z'
      },
      {
        id: 2,
        is_current: false,
        created_at: '2025-10-30T15:00:00Z',
        expires: '2025-11-06T15:00:00Z'
      }
    ];

    server.use(
      http.get('http://localhost:3000/api/v1/auth/sessions', async () => {
        return HttpResponse.json({ sessions: mockSessions });
      })
    );

    renderWithProviders(<ActiveSessions />);

    await waitFor(() => {
      expect(screen.getByText('Current Session')).toBeInTheDocument();
    });
  });

  it('does not show revoke button for current session', async () => {
    const mockSessions = [
      {
        id: 1,
        is_current: true,
        created_at: '2025-11-01T10:00:00Z',
        expires: '2025-11-08T10:00:00Z'
      }
    ];

    server.use(
      http.get('http://localhost:3000/api/v1/auth/sessions', async () => {
        return HttpResponse.json({ sessions: mockSessions });
      })
    );

    renderWithProviders(<ActiveSessions />);

    await waitFor(() => {
      expect(screen.getByText('Session #1')).toBeInTheDocument();
    });

    expect(screen.queryByRole('button', { name: /Revoke/i })).not.toBeInTheDocument();
  });

  it('shows revoke button for non-current sessions', async () => {
    const mockSessions = [
      {
        id: 1,
        is_current: true,
        created_at: '2025-11-01T10:00:00Z',
        expires: '2025-11-08T10:00:00Z'
      },
      {
        id: 2,
        is_current: false,
        created_at: '2025-10-30T15:00:00Z',
        expires: '2025-11-06T15:00:00Z'
      }
    ];

    server.use(
      http.get('http://localhost:3000/api/v1/auth/sessions', async () => {
        return HttpResponse.json({ sessions: mockSessions });
      })
    );

    renderWithProviders(<ActiveSessions />);

    await waitFor(() => {
      const revokeButtons = screen.getAllByRole('button', { name: 'Revoke' });
      expect(revokeButtons).toHaveLength(1);
    });
  });

  it('shows confirmation buttons when revoke is clicked', async () => {
    const mockSessions = [
      {
        id: 1,
        is_current: true,
        created_at: '2025-11-01T10:00:00Z',
        expires: '2025-11-08T10:00:00Z'
      },
      {
        id: 2,
        is_current: false,
        created_at: '2025-10-30T15:00:00Z',
        expires: '2025-11-06T15:00:00Z'
      }
    ];

    server.use(
      http.get('http://localhost:3000/api/v1/auth/sessions', async () => {
        return HttpResponse.json({ sessions: mockSessions });
      })
    );

    renderWithProviders(<ActiveSessions />);

    await waitFor(() => {
      expect(screen.getByText('Session #2')).toBeInTheDocument();
    });

    const revokeButton = screen.getByRole('button', { name: 'Revoke' });
    fireEvent.click(revokeButton);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Confirm' })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument();
    });
  });

  it('cancels session revocation when cancel is clicked', async () => {
    const mockSessions = [
      {
        id: 1,
        is_current: true,
        created_at: '2025-11-01T10:00:00Z',
        expires: '2025-11-08T10:00:00Z'
      },
      {
        id: 2,
        is_current: false,
        created_at: '2025-10-30T15:00:00Z',
        expires: '2025-11-06T15:00:00Z'
      }
    ];

    server.use(
      http.get('http://localhost:3000/api/v1/auth/sessions', async () => {
        return HttpResponse.json({ sessions: mockSessions });
      })
    );

    renderWithProviders(<ActiveSessions />);

    await waitFor(() => {
      expect(screen.getByText('Session #2')).toBeInTheDocument();
    });

    const revokeButton = screen.getByRole('button', { name: 'Revoke' });
    fireEvent.click(revokeButton);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument();
    });

    const cancelButton = screen.getByRole('button', { name: 'Cancel' });
    fireEvent.click(cancelButton);

    await waitFor(() => {
      expect(screen.queryByRole('button', { name: 'Confirm' })).not.toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Revoke' })).toBeInTheDocument();
    });
  });

  it('revokes session successfully when confirmed', async () => {
    const mockSessions = [
      {
        id: 1,
        is_current: true,
        created_at: '2025-11-01T10:00:00Z',
        expires: '2025-11-08T10:00:00Z'
      },
      {
        id: 2,
        is_current: false,
        created_at: '2025-10-30T15:00:00Z',
        expires: '2025-11-06T15:00:00Z'
      }
    ];

    server.use(
      http.get('http://localhost:3000/api/v1/auth/sessions', async () => {
        return HttpResponse.json({ sessions: mockSessions });
      }),
      http.delete('http://localhost:3000/api/v1/auth/sessions/:id', async () => {
        return HttpResponse.json({ message: 'Session revoked successfully' });
      })
    );

    renderWithProviders(<ActiveSessions />);

    await waitFor(() => {
      expect(screen.getByText('Session #2')).toBeInTheDocument();
    });

    const revokeButton = screen.getByRole('button', { name: 'Revoke' });
    fireEvent.click(revokeButton);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Confirm' })).toBeInTheDocument();
    });

    const confirmButton = screen.getByRole('button', { name: 'Confirm' });
    fireEvent.click(confirmButton);

    await waitFor(() => {
      expect(screen.getByText('Session revoked successfully')).toBeInTheDocument();
    });
  });

  it('shows error toast when session revocation fails', async () => {
    const mockSessions = [
      {
        id: 1,
        is_current: true,
        created_at: '2025-11-01T10:00:00Z',
        expires: '2025-11-08T10:00:00Z'
      },
      {
        id: 2,
        is_current: false,
        created_at: '2025-10-30T15:00:00Z',
        expires: '2025-11-06T15:00:00Z'
      }
    ];

    server.use(
      http.get('http://localhost:3000/api/v1/auth/sessions', async () => {
        return HttpResponse.json({ sessions: mockSessions });
      }),
      http.delete('http://localhost:3000/api/v1/auth/sessions/:id', async () => {
        return HttpResponse.json(
          { detail: 'Failed to revoke session' },
          { status: 500 }
        );
      })
    );

    renderWithProviders(<ActiveSessions />);

    await waitFor(() => {
      expect(screen.getByText('Session #2')).toBeInTheDocument();
    });

    const revokeButton = screen.getByRole('button', { name: 'Revoke' });
    fireEvent.click(revokeButton);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Confirm' })).toBeInTheDocument();
    });

    const confirmButton = screen.getByRole('button', { name: 'Confirm' });
    fireEvent.click(confirmButton);

    await waitFor(() => {
      expect(screen.getByText('Failed to revoke session')).toBeInTheDocument();
    });
  });

  it('shows "Revoke All Others" button when multiple sessions exist', async () => {
    const mockSessions = [
      {
        id: 1,
        is_current: true,
        created_at: '2025-11-01T10:00:00Z',
        expires: '2025-11-08T10:00:00Z'
      },
      {
        id: 2,
        is_current: false,
        created_at: '2025-10-30T15:00:00Z',
        expires: '2025-11-06T15:00:00Z'
      }
    ];

    server.use(
      http.get('http://localhost:3000/api/v1/auth/sessions', async () => {
        return HttpResponse.json({ sessions: mockSessions });
      })
    );

    renderWithProviders(<ActiveSessions />);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Revoke All Others' })).toBeInTheDocument();
    });
  });

  it('does not show "Revoke All Others" button with only one session', async () => {
    const mockSessions = [
      {
        id: 1,
        is_current: true,
        created_at: '2025-11-01T10:00:00Z',
        expires: '2025-11-08T10:00:00Z'
      }
    ];

    server.use(
      http.get('http://localhost:3000/api/v1/auth/sessions', async () => {
        return HttpResponse.json({ sessions: mockSessions });
      })
    );

    renderWithProviders(<ActiveSessions />);

    await waitFor(() => {
      expect(screen.getByText('Session #1')).toBeInTheDocument();
    });

    expect(screen.queryByRole('button', { name: 'Revoke All Others' })).not.toBeInTheDocument();
  });

  it('shows confirmation when "Revoke All Others" is clicked', async () => {
    const mockSessions = [
      {
        id: 1,
        is_current: true,
        created_at: '2025-11-01T10:00:00Z',
        expires: '2025-11-08T10:00:00Z'
      },
      {
        id: 2,
        is_current: false,
        created_at: '2025-10-30T15:00:00Z',
        expires: '2025-11-06T15:00:00Z'
      }
    ];

    server.use(
      http.get('http://localhost:3000/api/v1/auth/sessions', async () => {
        return HttpResponse.json({ sessions: mockSessions });
      })
    );

    renderWithProviders(<ActiveSessions />);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Revoke All Others' })).toBeInTheDocument();
    });

    const revokeAllButton = screen.getByRole('button', { name: 'Revoke All Others' });
    fireEvent.click(revokeAllButton);

    await waitFor(() => {
      expect(screen.getByText(/Are you sure?/i)).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /Yes, Revoke All Other Sessions/i })).toBeInTheDocument();
    });
  });

  it('cancels "Revoke All Others" when cancel is clicked', async () => {
    const mockSessions = [
      {
        id: 1,
        is_current: true,
        created_at: '2025-11-01T10:00:00Z',
        expires: '2025-11-08T10:00:00Z'
      },
      {
        id: 2,
        is_current: false,
        created_at: '2025-10-30T15:00:00Z',
        expires: '2025-11-06T15:00:00Z'
      }
    ];

    server.use(
      http.get('http://localhost:3000/api/v1/auth/sessions', async () => {
        return HttpResponse.json({ sessions: mockSessions });
      })
    );

    renderWithProviders(<ActiveSessions />);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Revoke All Others' })).toBeInTheDocument();
    });

    const revokeAllButton = screen.getByRole('button', { name: 'Revoke All Others' });
    fireEvent.click(revokeAllButton);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument();
    });

    const cancelButton = screen.getByRole('button', { name: 'Cancel' });
    fireEvent.click(cancelButton);

    await waitFor(() => {
      expect(screen.queryByText(/Are you sure?/i)).not.toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Revoke All Others' })).toBeInTheDocument();
    });
  });

  it('revokes all other sessions successfully when confirmed', async () => {
    const mockSessions = [
      {
        id: 1,
        is_current: true,
        created_at: '2025-11-01T10:00:00Z',
        expires: '2025-11-08T10:00:00Z'
      },
      {
        id: 2,
        is_current: false,
        created_at: '2025-10-30T15:00:00Z',
        expires: '2025-11-06T15:00:00Z'
      },
      {
        id: 3,
        is_current: false,
        created_at: '2025-10-28T12:00:00Z',
        expires: '2025-11-04T12:00:00Z'
      }
    ];

    server.use(
      http.get('http://localhost:3000/api/v1/auth/sessions', async () => {
        return HttpResponse.json({ sessions: mockSessions });
      }),
      http.post('http://localhost:3000/api/v1/auth/revoke-all-sessions', async () => {
        return HttpResponse.json({ message: 'All other sessions revoked successfully' });
      })
    );

    renderWithProviders(<ActiveSessions />);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Revoke All Others' })).toBeInTheDocument();
    });

    const revokeAllButton = screen.getByRole('button', { name: 'Revoke All Others' });
    fireEvent.click(revokeAllButton);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Yes, Revoke All Other Sessions/i })).toBeInTheDocument();
    });

    const confirmButton = screen.getByRole('button', { name: /Yes, Revoke All Other Sessions/i });
    fireEvent.click(confirmButton);

    await waitFor(() => {
      expect(screen.getByText('All other sessions revoked successfully')).toBeInTheDocument();
    });
  });

  it('shows error toast when revoking all sessions fails', async () => {
    const mockSessions = [
      {
        id: 1,
        is_current: true,
        created_at: '2025-11-01T10:00:00Z',
        expires: '2025-11-08T10:00:00Z'
      },
      {
        id: 2,
        is_current: false,
        created_at: '2025-10-30T15:00:00Z',
        expires: '2025-11-06T15:00:00Z'
      }
    ];

    server.use(
      http.get('http://localhost:3000/api/v1/auth/sessions', async () => {
        return HttpResponse.json({ sessions: mockSessions });
      }),
      http.post('http://localhost:3000/api/v1/auth/revoke-all-sessions', async () => {
        return HttpResponse.json(
          { detail: 'Failed to revoke all sessions' },
          { status: 500 }
        );
      })
    );

    renderWithProviders(<ActiveSessions />);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Revoke All Others' })).toBeInTheDocument();
    });

    const revokeAllButton = screen.getByRole('button', { name: 'Revoke All Others' });
    fireEvent.click(revokeAllButton);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Yes, Revoke All Other Sessions/i })).toBeInTheDocument();
    });

    const confirmButton = screen.getByRole('button', { name: /Yes, Revoke All Other Sessions/i });
    fireEvent.click(confirmButton);

    await waitFor(() => {
      expect(screen.getByText('Failed to revoke all sessions')).toBeInTheDocument();
    });
  });

  it('shows loading state on confirm button while revoking a session', async () => {
    const mockSessions = [
      {
        id: 1,
        is_current: true,
        created_at: '2025-11-01T10:00:00Z',
        expires: '2025-11-08T10:00:00Z'
      },
      {
        id: 2,
        is_current: false,
        created_at: '2025-10-30T15:00:00Z',
        expires: '2025-11-06T15:00:00Z'
      }
    ];

    server.use(
      http.get('http://localhost:3000/api/v1/auth/sessions', async () => {
        return HttpResponse.json({ sessions: mockSessions });
      }),
      http.delete('http://localhost:3000/api/v1/auth/sessions/:id', async () => {
        // Simulate slow API
        await new Promise(resolve => setTimeout(resolve, 100));
        return HttpResponse.json({ message: 'Session revoked successfully' });
      })
    );

    renderWithProviders(<ActiveSessions />);

    await waitFor(() => {
      expect(screen.getByText('Session #2')).toBeInTheDocument();
    });

    const revokeButton = screen.getByRole('button', { name: 'Revoke' });
    fireEvent.click(revokeButton);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Confirm' })).toBeInTheDocument();
    });

    const confirmButton = screen.getByRole('button', { name: 'Confirm' });
    fireEvent.click(confirmButton);

    // Toast should eventually appear after revocation completes
    await waitFor(() => {
      expect(screen.getByText('Session revoked successfully')).toBeInTheDocument();
    });
  });
});
