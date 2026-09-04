import { describe, it, expect, beforeEach, vi } from 'vitest'
import { screen, fireEvent, waitFor, act } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { SessionExpiredModal } from '../SessionExpiredModal'
import { renderWithProviders } from '../../test-utils/render'
import { server } from '../../mocks/server'
import { useAuth } from '../../contexts/AuthContext'

// Renders page content and exposes auth state for assertions
function AuthContextTestHarness() {
  const { isAuthenticated } = useAuth()
  return (
    <div>
      <div data-testid="page-content">Page content preserved</div>
      <div data-testid="auth-status">{isAuthenticated ? 'authenticated' : 'unauthenticated'}</div>
    </div>
  )
}

describe('SessionExpiredModal', () => {
  beforeEach(() => {
    server.resetHandlers()
    localStorage.clear()
  })

  it('renders when open', () => {
    renderWithProviders(<SessionExpiredModal isOpen={true} onSuccess={vi.fn()} />)

    expect(screen.getByRole('heading', { name: 'Session Expired' })).toBeInTheDocument()
    expect(screen.getByText(/your session has expired/i)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Login' })).toBeInTheDocument()
  })

  it('does not render when closed', () => {
    renderWithProviders(<SessionExpiredModal isOpen={false} onSuccess={vi.fn()} />)

    expect(screen.queryByRole('heading', { name: 'Session Expired' })).not.toBeInTheDocument()
  })

  it('has no close button — modal is forced', () => {
    renderWithProviders(<SessionExpiredModal isOpen={true} onSuccess={vi.fn()} />)

    expect(screen.queryByRole('button', { name: 'Close' })).not.toBeInTheDocument()
    expect(screen.queryByLabelText('Close')).not.toBeInTheDocument()
  })

  it('does not show the forgot password link', () => {
    renderWithProviders(<SessionExpiredModal isOpen={true} onSuccess={vi.fn()} />)

    expect(screen.queryByText(/forgot password/i)).not.toBeInTheDocument()
  })

  it('tells the user their work is preserved', () => {
    renderWithProviders(<SessionExpiredModal isOpen={true} onSuccess={vi.fn()} />)

    expect(screen.getByText(/your work on this page has been preserved/i)).toBeInTheDocument()
  })

  it('calls onSuccess after successful re-authentication', async () => {
    const onSuccess = vi.fn()

    server.use(
      http.post('/api/v1/auth/login', () => {
        return HttpResponse.json({
          user: { id: 1, username: 'testuser', email: 'test@example.com' },
          Token: 'mock-jwt-token',
        })
      }),
      http.get('/api/v1/auth/me', () => {
        return HttpResponse.json({ id: 1, username: 'testuser', email: 'test@example.com' })
      })
    )

    renderWithProviders(<SessionExpiredModal isOpen={true} onSuccess={onSuccess} />)

    fireEvent.change(screen.getByLabelText('Username or Email'), {
      target: { value: 'testuser' },
    })
    fireEvent.change(screen.getByLabelText('Password'), {
      target: { value: 'password123' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Login' }))

    await waitFor(() => {
      expect(onSuccess).toHaveBeenCalledOnce()
    }, { timeout: 3000 })
  })

  it('does not call onSuccess when login fails', async () => {
    const onSuccess = vi.fn()

    server.use(
      http.post('/api/v1/auth/login', () => {
        return HttpResponse.json({ detail: 'Invalid credentials' }, { status: 401 })
      })
    )

    renderWithProviders(<SessionExpiredModal isOpen={true} onSuccess={onSuccess} />)

    fireEvent.change(screen.getByLabelText('Username or Email'), {
      target: { value: 'wronguser' },
    })
    fireEvent.change(screen.getByLabelText('Password'), {
      target: { value: 'wrongpass' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Login' }))

    await waitFor(() => {
      expect(screen.getByText(/invalid credentials/i)).toBeInTheDocument()
    }, { timeout: 3000 })

    expect(onSuccess).not.toHaveBeenCalled()
  })
})

describe('SessionExpiredModal — AuthContext integration', () => {
  beforeEach(() => {
    server.resetHandlers()
    localStorage.clear()
  })

  it('modal appears when auth:sessionExpired event is dispatched', async () => {
    renderWithProviders(<AuthContextTestHarness />)

    expect(screen.queryByRole('heading', { name: 'Session Expired' })).not.toBeInTheDocument()

    act(() => {
      window.dispatchEvent(new CustomEvent('auth:sessionExpired'))
    })

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Session Expired' })).toBeInTheDocument()
    })
  })

  it('page content is still present while modal is open', async () => {
    renderWithProviders(<AuthContextTestHarness />)

    act(() => {
      window.dispatchEvent(new CustomEvent('auth:sessionExpired'))
    })

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Session Expired' })).toBeInTheDocument()
    })

    expect(screen.getByTestId('page-content')).toBeInTheDocument()
  })

  it('modal closes after successful re-authentication', async () => {
    server.use(
      http.post('/api/v1/auth/login', () => {
        return HttpResponse.json({
          user: { id: 1, username: 'testuser', email: 'test@example.com' },
          Token: 'mock-jwt-token',
        })
      }),
      http.get('/api/v1/auth/me', () => {
        return HttpResponse.json({ id: 1, username: 'testuser', email: 'test@example.com' })
      })
    )

    renderWithProviders(<AuthContextTestHarness />)

    act(() => {
      window.dispatchEvent(new CustomEvent('auth:sessionExpired'))
    })

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Session Expired' })).toBeInTheDocument()
    })

    fireEvent.change(screen.getByLabelText('Username or Email'), { target: { value: 'testuser' } })
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'password123' } })
    fireEvent.click(screen.getByRole('button', { name: 'Login' }))

    await waitFor(() => {
      expect(screen.queryByRole('heading', { name: 'Session Expired' })).not.toBeInTheDocument()
    }, { timeout: 3000 })
  })

  it('dispatching auth:sessionExpired twice only shows one modal', async () => {
    renderWithProviders(<AuthContextTestHarness />)

    act(() => {
      window.dispatchEvent(new CustomEvent('auth:sessionExpired'))
      window.dispatchEvent(new CustomEvent('auth:sessionExpired'))
    })

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Session Expired' })).toBeInTheDocument()
    })

    // Only one modal heading — not duplicated
    expect(screen.getAllByRole('heading', { name: 'Session Expired' })).toHaveLength(1)
  })

  it('user remains authenticated while modal is open — ProtectedRoute does not redirect', async () => {
    renderWithProviders(<AuthContextTestHarness />)

    // User starts authenticated (default MSW handler returns a user for /auth/me)
    await waitFor(() => {
      expect(screen.getByTestId('auth-status')).toHaveTextContent('authenticated')
    })

    act(() => {
      window.dispatchEvent(new CustomEvent('auth:sessionExpired'))
    })

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Session Expired' })).toBeInTheDocument()
    })

    // Auth state must not have changed — this is what keeps ProtectedRoute from redirecting
    expect(screen.getByTestId('auth-status')).toHaveTextContent('authenticated')
  })

  it('user is authenticated again after re-auth and modal closes', async () => {
    server.use(
      http.post('/api/v1/auth/login', () => {
        return HttpResponse.json({
          user: { id: 1, username: 'testuser', email: 'test@example.com' },
          Token: 'mock-jwt-token',
        })
      }),
      http.get('/api/v1/auth/me', () => {
        return HttpResponse.json({ id: 1, username: 'testuser', email: 'test@example.com' })
      })
    )

    renderWithProviders(<AuthContextTestHarness />)

    await waitFor(() => {
      expect(screen.getByTestId('auth-status')).toHaveTextContent('authenticated')
    })

    act(() => {
      window.dispatchEvent(new CustomEvent('auth:sessionExpired'))
    })

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Session Expired' })).toBeInTheDocument()
    })

    fireEvent.change(screen.getByLabelText('Username or Email'), { target: { value: 'testuser' } })
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'password123' } })
    fireEvent.click(screen.getByRole('button', { name: 'Login' }))

    await waitFor(() => {
      expect(screen.queryByRole('heading', { name: 'Session Expired' })).not.toBeInTheDocument()
    }, { timeout: 3000 })

    expect(screen.getByTestId('auth-status')).toHaveTextContent('authenticated')
  })
})
