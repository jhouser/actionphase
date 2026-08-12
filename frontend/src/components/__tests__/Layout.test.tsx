import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen, render, fireEvent } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { useAuth } from '../../contexts/AuthContext'
import { Layout } from '../Layout'
import { AdminModeProvider } from '../../contexts/AdminModeContext'
import { ToastProvider } from '../../contexts/ToastContext'

// Mock the useAuth hook
vi.mock('../../contexts/AuthContext', () => ({
  useAuth: vi.fn(),
}))

import { useAuth } from '../../contexts/AuthContext'

describe('Layout', () => {
  const mockLogout = vi.fn()
  let queryClient: QueryClient

  beforeEach(() => {
    vi.clearAllMocks()
    queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
      },
    })
  })

  const renderLayout = (children: React.ReactNode, initialRoute = '/dashboard') => {
    return render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={[initialRoute]}>
          <AdminModeProvider>
            <ToastProvider>
              <Layout>{children}</Layout>
            </ToastProvider>
          </AdminModeProvider>
        </MemoryRouter>
      </QueryClientProvider>
    )
  }

  describe('When user is authenticated', () => {
    beforeEach(() => {
      vi.mocked(useAuth).mockReturnValue({
        isAuthenticated: true,
        currentUser: { id: 1, username: 'testuser', email: 'test@example.com', created_at: '', updated_at: '' },
        isCheckingAuth: false,
        isLoading: false,
        login: vi.fn(),
        register: vi.fn(),
        logout: mockLogout,
        error: null,
      } as Partial<ReturnType<typeof useAuth>>)
    })

    it('should render navigation bar', () => {
      renderLayout(<div>Content</div>, '/dashboard')

      expect(screen.getByRole('navigation')).toBeInTheDocument()
      expect(screen.getByText('ActionPhase')).toBeInTheDocument()
    })

    it('should render navigation links', () => {
      renderLayout(<div>Content</div>, '/dashboard')

      expect(screen.getByRole('link', { name: 'Dashboard' })).toBeInTheDocument()
      expect(screen.getByRole('link', { name: 'Games' })).toBeInTheDocument()
    })

    it('should render logout button', async () => {
      renderLayout(<div>Content</div>, '/dashboard')

      // Hover over user menu to open dropdown
      const userButton = screen.getByRole('button', { name: /testuser/i })
      await userEvent.hover(userButton)

      expect(screen.getByRole('button', { name: 'Logout' })).toBeInTheDocument()
    })

    it('should open the user menu on click, not only on hover', async () => {
      // The trigger used to have no onClick at all, so the menu was reachable
      // only by hovering — unusable by keyboard and by touch on desktop widths.
      renderLayout(<div>Content</div>, '/dashboard')

      const userButton = screen.getByRole('button', { name: /testuser/i })
      expect(screen.queryByRole('button', { name: 'Logout' })).not.toBeInTheDocument()
      expect(userButton).toHaveAttribute('aria-expanded', 'false')

      // click() alone, without userEvent's synthetic pointer movement: the
      // wrapper still closes on mouseleave, which userEvent.click emits around
      // the click and which would immediately undo the open in jsdom.
      fireEvent.click(userButton)

      expect(screen.getByRole('button', { name: 'Logout' })).toBeInTheDocument()
      expect(userButton).toHaveAttribute('aria-expanded', 'true')
    })

    it('should highlight active dashboard link', () => {
      renderLayout(<div>Content</div>, '/dashboard')

      const dashboardLink = screen.getByRole('link', { name: 'Dashboard' })
      expect(dashboardLink).toHaveClass('bg-interactive-primary-hover', 'text-white')
    })

    it('should highlight active games link', () => {
      renderLayout(<div>Content</div>, '/games')

      const gamesLink = screen.getByRole('link', { name: 'Games' })
      expect(gamesLink).toHaveClass('bg-interactive-primary-hover', 'text-white')
    })

    it('should not highlight inactive links', () => {
      renderLayout(<div>Content</div>, '/dashboard')

      const gamesLink = screen.getByRole('link', { name: 'Games' })
      expect(gamesLink).not.toHaveClass('bg-interactive-primary-hover')
      expect(gamesLink).toHaveClass('text-white/90')
    })

    it('should render children content', () => {
      renderLayout(<div data-testid="test-content">Test Content</div>, '/dashboard')

      expect(screen.getByTestId('test-content')).toBeInTheDocument()
      expect(screen.getByText('Test Content')).toBeInTheDocument()
    })

    it('should render complex children', () => {
      renderLayout(
        <div>
          <h1>Page Title</h1>
          <p>Page content</p>
          <button>Action Button</button>
        </div>,
        '/dashboard'
      )

      expect(screen.getByRole('heading', { name: 'Page Title' })).toBeInTheDocument()
      expect(screen.getByText('Page content')).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Action Button' })).toBeInTheDocument()
    })

    it('should render footer', () => {
      renderLayout(<div>Content</div>, '/dashboard')

      expect(screen.getByText(/© 2025 ActionPhase/)).toBeInTheDocument()
      expect(screen.getByText(/collaborative role-playing platform/)).toBeInTheDocument()
    })

    it('should call logout when logout button is clicked', async () => {
      renderLayout(<div>Content</div>, '/dashboard')

      // Hover over user menu to open dropdown
      const userButton = screen.getByRole('button', { name: /testuser/i })
      await userEvent.hover(userButton)

      const logoutButton = screen.getByRole('button', { name: 'Logout' })
      fireEvent.click(logoutButton)

      expect(mockLogout).toHaveBeenCalledOnce()
    })

    it('should have correct link hrefs', () => {
      renderLayout(<div>Content</div>, '/dashboard')

      const dashboardLink = screen.getByRole('link', { name: 'Dashboard' })
      const gamesLink = screen.getByRole('link', { name: 'Games' })

      expect(dashboardLink).toHaveAttribute('href', '/dashboard')
      expect(gamesLink).toHaveAttribute('href', '/games')
    })

    it('should render brand link to dashboard', () => {
      renderLayout(<div>Content</div>, '/games')

      // There are two "ActionPhase" links - the brand and the dashboard link
      const brandLinks = screen.getAllByText('ActionPhase')
      expect(brandLinks.length).toBeGreaterThan(0)

      // The brand should be a link
      const brandLink = brandLinks[0].closest('a')
      expect(brandLink).toHaveAttribute('href', '/dashboard')
    })
  })

  describe('When user is NOT authenticated', () => {
    beforeEach(() => {
      vi.mocked(useAuth).mockReturnValue({
        isAuthenticated: false,
        currentUser: null,
        isCheckingAuth: false,
        isLoading: false,
        login: vi.fn(),
        register: vi.fn(),
        logout: mockLogout,
        error: null,
      } as Partial<ReturnType<typeof useAuth>>)
    })

    it('should NOT render navigation bar', () => {
      renderLayout(<div>Content</div>, '/login')

      expect(screen.queryByRole('navigation')).not.toBeInTheDocument()
      expect(screen.queryByText('ActionPhase')).not.toBeInTheDocument()
    })

    it('should NOT render navigation links', () => {
      renderLayout(<div>Content</div>, '/login')

      expect(screen.queryByRole('link', { name: 'Dashboard' })).not.toBeInTheDocument()
      expect(screen.queryByRole('link', { name: 'Games' })).not.toBeInTheDocument()
    })

    it('should NOT render logout button', () => {
      renderLayout(<div>Content</div>, '/login')

      expect(screen.queryByRole('button', { name: 'Logout' })).not.toBeInTheDocument()
    })

    it('should still render children content', () => {
      renderLayout(<div data-testid="test-content">Login Form</div>, '/login')

      expect(screen.getByTestId('test-content')).toBeInTheDocument()
      expect(screen.getByText('Login Form')).toBeInTheDocument()
    })

    it('should still render footer', () => {
      renderLayout(<div>Content</div>, '/login')

      expect(screen.getByText(/© 2025 ActionPhase/)).toBeInTheDocument()
    })
  })

  describe('Styling and structure', () => {
    beforeEach(() => {
      vi.mocked(useAuth).mockReturnValue({
        isAuthenticated: true,
        currentUser: { id: 1, username: 'testuser', email: 'test@example.com', created_at: '', updated_at: '' },
        isCheckingAuth: false,
        isLoading: false,
        login: vi.fn(),
        register: vi.fn(),
        logout: mockLogout,
        error: null,
      } as Partial<ReturnType<typeof useAuth>>)
    })

    it('should have proper layout structure', () => {
      const { container } = renderLayout(<div>Content</div>, '/dashboard')

      const layout = container.querySelector('.min-h-screen.surface-sunken')
      expect(layout).toBeInTheDocument()
    })

    it('should have navigation with proper styling', () => {
      const { container } = renderLayout(<div>Content</div>, '/dashboard')

      const nav = container.querySelector('.bg-interactive-primary.shadow-lg')
      expect(nav).toBeInTheDocument()
    })

    it('should have sticky navigation bar', () => {
      const { container } = renderLayout(<div>Content</div>, '/dashboard')

      const nav = container.querySelector('nav')
      expect(nav).toHaveClass('sticky', 'top-0', 'z-50')
    })

    it('should have main content wrapper', () => {
      renderLayout(<div data-testid="content">Content</div>, '/dashboard')

      const main = screen.getByTestId('content').closest('main')
      expect(main).toBeInTheDocument()
    })

    it('should have footer with border', () => {
      const { container } = renderLayout(<div>Content</div>, '/dashboard')

      const footer = container.querySelector('footer.border-t.border-theme-default')
      expect(footer).toBeInTheDocument()
    })
  })

  describe('Edge cases', () => {
    beforeEach(() => {
      vi.mocked(useAuth).mockReturnValue({
        isAuthenticated: true,
        currentUser: { id: 1, username: 'testuser', email: 'test@example.com', created_at: '', updated_at: '' },
        isCheckingAuth: false,
        isLoading: false,
        login: vi.fn(),
        register: vi.fn(),
        logout: mockLogout,
        error: null,
      } as Partial<ReturnType<typeof useAuth>>)
    })

    it('should handle empty children', () => {
      const { container } = renderLayout(<></>, '/dashboard')

      expect(container.querySelector('main')).toBeInTheDocument()
    })

    it('should handle null children', () => {
      const { container } = renderLayout(null as unknown as React.ReactNode, '/dashboard')

      expect(container.querySelector('main')).toBeInTheDocument()
    })

    it('should handle route that is not dashboard or games', () => {
      renderLayout(<div>Profile Content</div>, '/profile')

      // Both links should be inactive
      const dashboardLink = screen.getByRole('link', { name: 'Dashboard' })
      const gamesLink = screen.getByRole('link', { name: 'Games' })

      expect(dashboardLink).not.toHaveClass('bg-interactive-primary-hover')
      expect(gamesLink).not.toHaveClass('bg-interactive-primary-hover')
      expect(dashboardLink).toHaveClass('text-white/90')
      expect(gamesLink).toHaveClass('text-white/90')
    })
  })

  describe('Accessibility', () => {
    beforeEach(() => {
      vi.mocked(useAuth).mockReturnValue({
        isAuthenticated: true,
        currentUser: { id: 1, username: 'testuser', email: 'test@example.com', created_at: '', updated_at: '' },
        isCheckingAuth: false,
        isLoading: false,
        login: vi.fn(),
        register: vi.fn(),
        logout: mockLogout,
        error: null,
      } as Partial<ReturnType<typeof useAuth>>)
    })

    it('should have semantic nav element', () => {
      renderLayout(<div>Content</div>, '/dashboard')

      expect(screen.getByRole('navigation')).toBeInTheDocument()
    })

    it('should have semantic main element', () => {
      renderLayout(<div>Content</div>, '/dashboard')

      expect(screen.getByRole('main')).toBeInTheDocument()
    })

    it('should have navigation links with proper role', () => {
      renderLayout(<div>Content</div>, '/dashboard')

      const links = screen.getAllByRole('link')
      expect(links.length).toBeGreaterThan(0)
    })

    it('should have logout button with proper role', async () => {
      renderLayout(<div>Content</div>, '/dashboard')

      // Hover over user menu to open dropdown
      const userButton = screen.getByRole('button', { name: /testuser/i })
      await userEvent.hover(userButton)

      const button = screen.getByRole('button', { name: 'Logout' })
      expect(button).toBeInTheDocument()
      expect(button).toBeEnabled()
    })
  })
})
