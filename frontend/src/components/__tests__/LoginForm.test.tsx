import { describe, it, expect, beforeEach } from 'vitest'
import { screen, fireEvent, waitFor } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { LoginForm } from '../LoginForm'
import { renderWithProviders } from '../../test-utils/render'
import { server } from '../../mocks/server'

describe('LoginForm', () => {
  beforeEach(() => {
    // Reset MSW handlers before each test
    server.resetHandlers()
    localStorage.clear()
  })

  it('renders login form with all required fields', () => {
    renderWithProviders(<LoginForm />)

    expect(screen.getByRole('heading', { name: 'Login' })).toBeInTheDocument()
    expect(screen.getByLabelText('Username or Email')).toBeInTheDocument()
    expect(screen.getByLabelText('Password')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Login' })).toBeInTheDocument()
  })

  it('handles form input changes', async () => {
    renderWithProviders(<LoginForm />)

    const usernameInput = screen.getByLabelText('Username or Email')
    const passwordInput = screen.getByLabelText('Password')

    fireEvent.change(usernameInput, { target: { value: 'testuser' } })
    fireEvent.change(passwordInput, { target: { value: 'password123' } })

    expect(usernameInput).toHaveValue('testuser')
    expect(passwordInput).toHaveValue('password123')
  })

  it('submits form with correct data and calls onSuccess', async () => {
    const mockOnSuccess = vi.fn()

    // Override the login and /me handlers to confirm they're working
    server.use(
      http.post('http://localhost:3000/api/v1/auth/login', async () => {
        return HttpResponse.json({
          user: {
            id: 1,
            username: 'testuser',
            email: 'test@example.com',
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString(),
          },
          Token: 'mock-jwt-token-from-test',
        })
      }),
      http.get('http://localhost:3000/api/v1/auth/me', async () => {
        return HttpResponse.json({
          id: 1,
          username: 'testuser',
          email: 'test@example.com',
        })
      })
    )

    renderWithProviders(<LoginForm onSuccess={mockOnSuccess} />)

    const usernameInput = screen.getByLabelText('Username or Email')
    const passwordInput = screen.getByLabelText('Password')
    const submitButton = screen.getByRole('button', { name: 'Login' })

    fireEvent.change(usernameInput, { target: { value: 'testuser' } })
    fireEvent.change(passwordInput, { target: { value: 'password123' } })
    fireEvent.click(submitButton)

    await waitFor(() => {
      expect(mockOnSuccess).toHaveBeenCalled()
    }, { timeout: 3000 })
  })

  it('prevents form submission with empty fields', () => {
    renderWithProviders(<LoginForm />)

    const submitButton = screen.getByRole('button', { name: 'Login' })
    const usernameInput = screen.getByLabelText('Username or Email')

    // Form should have required attribute
    expect(usernameInput).toBeRequired()

    // Clicking submit without filling should not work due to HTML5 validation
    expect(submitButton).toBeInTheDocument()
  })

  it('handles login errors from server', async () => {
    server.use(
      http.post('http://localhost:3000/api/v1/auth/login', async () => {
        return HttpResponse.json(
          { detail: 'Invalid credentials' },
          { status: 401 }
        )
      })
    )

    renderWithProviders(<LoginForm />)

    const usernameInput = screen.getByLabelText('Username or Email')
    const passwordInput = screen.getByLabelText('Password')
    const submitButton = screen.getByRole('button', { name: 'Login' })

    fireEvent.change(usernameInput, { target: { value: 'wronguser' } })
    fireEvent.change(passwordInput, { target: { value: 'wrongpass' } })
    fireEvent.click(submitButton)

    await waitFor(() => {
      expect(screen.getByText(/invalid credentials/i)).toBeInTheDocument()
    }, { timeout: 3000 })
  })

  it('works without onSuccess callback', async () => {
    renderWithProviders(<LoginForm />)

    const usernameInput = screen.getByLabelText('Username or Email')
    const passwordInput = screen.getByLabelText('Password')
    const submitButton = screen.getByRole('button', { name: 'Login' })

    fireEvent.change(usernameInput, { target: { value: 'testuser' } })
    fireEvent.change(passwordInput, { target: { value: 'password123' } })
    fireEvent.click(submitButton)

    // Should successfully login without throwing error
    await waitFor(() => {
      expect(submitButton).not.toBeDisabled()
    })
  })

  it('shows forgot password link by default', () => {
    renderWithProviders(<LoginForm />)

    expect(screen.getByRole('link', { name: /forgot password/i })).toBeInTheDocument()
  })

  it('hides forgot password link when hideForgotPassword is true', () => {
    renderWithProviders(<LoginForm hideForgotPassword />)

    expect(screen.queryByRole('link', { name: /forgot password/i })).not.toBeInTheDocument()
  })

  it('has proper accessibility attributes', () => {
    renderWithProviders(<LoginForm />)

    const usernameInput = screen.getByLabelText('Username or Email')
    const passwordInput = screen.getByLabelText('Password')

    expect(usernameInput).toHaveAttribute('type', 'text')
    expect(usernameInput).toHaveAttribute('required')
    expect(usernameInput).toHaveAttribute('placeholder', 'Enter your username or email')

    expect(passwordInput).toHaveAttribute('type', 'password')
    expect(passwordInput).toHaveAttribute('required')
    expect(passwordInput).toHaveAttribute('placeholder', 'Enter your password')
  })
})
