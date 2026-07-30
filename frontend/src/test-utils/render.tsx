import React, { useState } from 'react'
import { render, act, RenderOptions } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createMemoryRouter, RouterProvider } from 'react-router-dom'
import { AuthProvider } from '../contexts/AuthContext'
import { AdminModeProvider } from '../contexts/AdminModeContext'
import { ScreenshotModeProvider } from '../contexts/ScreenshotModeContext'
import { ToastProvider } from '../contexts/ToastContext'
import { ConversationProvider } from '../contexts/ConversationContext'
import { GameProvider } from '../contexts/GameContext'
import { UtilityDrawerProvider } from '../contexts/UtilityDrawerContext'

interface RenderWithProvidersOptions extends Omit<RenderOptions, 'wrapper'> {
  /**
   * Query client instance to use for testing
   * If not provided, a new instance with default test config will be created
   */
  queryClient?: QueryClient

  /**
   * Initial route (path only, e.g. '/games/1')
   * Default: '/'
   */
  initialRoute?: string

  /**
   * Initial entries for the router history stack (e.g. ['/games/1?tab=foo'])
   * Takes precedence over initialRoute when provided.
   */
  initialEntries?: string[]

  /**
   * When provided, wraps children in a GameProvider with this gameId.
   * Required for components that call useGameContext().
   */
  gameId?: number
}

/**
 * Creates a new QueryClient with test-friendly defaults
 */
export function createTestQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        // Disable retries in tests to fail fast
        retry: false,
        // Disable cache to ensure fresh data in each test
        gcTime: 0,
        staleTime: 0,
      },
      mutations: {
        // Disable retries in tests
        retry: false,
      },
    },
    // Suppress error logs during tests
    logger: {
      // eslint-disable-next-line no-console
      log: console.log,
      // eslint-disable-next-line no-console
      warn: console.warn,
      error: () => {}, // Suppress error logs in tests
    },
  })
}

/**
 * Renders a component with all necessary providers for testing:
 * - QueryClientProvider (React Query)
 * - MemoryRouter (React Router)
 * - AuthProvider (Authentication context)
 * - AdminModeProvider (Admin mode context)
 * - ToastProvider (Toast notifications)
 * - ConversationProvider (Conversation context)
 *
 * @example
 * ```tsx
 * const { getByText } = renderWithProviders(<MyComponent />, {
 *   initialRoute: '/games/1',
 * })
 * ```
 */
export function renderWithProviders(
  ui: React.ReactElement,
  options: RenderWithProvidersOptions = {}
) {
  const {
    queryClient = createTestQueryClient(),
    initialRoute = '/',
    initialEntries,
    gameId,
    ...renderOptions
  } = options

  function wrapUi(element: React.ReactElement) {
    return gameId !== undefined ? (
      <GameProvider gameId={gameId}>{element}</GameProvider>
    ) : element
  }

  // Use a data router so useBlocker and other data-router hooks work in tests.
  // RouteElement uses useState so that rerender() triggers a real React re-render
  // inside RouterProvider (refs don't cause re-renders).
  let setRouteElement!: React.Dispatch<React.SetStateAction<React.ReactElement>>

  function RouteElement() {
    const [element, setElement] = useState(() => wrapUi(ui))
    setRouteElement = setElement
    return element
  }

  const router = createMemoryRouter(
    [{ path: '*', element: <RouteElement /> }],
    { initialEntries: initialEntries ?? [initialRoute] }
  )

  function Wrapper() {
    return (
      <QueryClientProvider client={queryClient}>
        <ToastProvider>
          <AuthProvider>
            <ConversationProvider>
              <AdminModeProvider>
                <ScreenshotModeProvider>
                  <UtilityDrawerProvider>
                    <RouterProvider router={router} />
                  </UtilityDrawerProvider>
                </ScreenshotModeProvider>
              </AdminModeProvider>
            </ConversationProvider>
          </AuthProvider>
        </ToastProvider>
      </QueryClientProvider>
    )
  }

  const result = render(<Wrapper />, renderOptions)

  return {
    ...result,
    // Override rerender so callers can pass a new UI element and it still has providers
    rerender: (newUi: React.ReactElement) => {
      act(() => { setRouteElement(wrapUi(newUi)) })
    },
    queryClient,
  }
}


// Re-export everything from @testing-library/react
// eslint-disable-next-line react-refresh/only-export-components
export * from '@testing-library/react'
