import { lazy, Suspense, useEffect } from 'react';
import { createBrowserRouter, RouterProvider, Navigate, useParams, useRouteError, Outlet } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ErrorBoundary } from './components/ErrorBoundary';
import { isChunkLoadError } from './lib/chunkLoadError';
import { ProtectedRoute } from './components/ProtectedRoute';
import { PublicArchiveRoute } from './components/PublicArchiveRoute';
import { AuthProvider, useAuth } from './contexts/AuthContext';
import { AdminModeProvider } from './contexts/AdminModeContext';
import { ScreenshotModeProvider } from './contexts/ScreenshotModeContext';
import { GameProvider } from './contexts/GameContext';
import { ThemeProvider } from './contexts/ThemeContext';
import { ToastProvider } from './contexts/ToastContext';
import { UtilityDrawerProvider } from './contexts/UtilityDrawerContext';
import { FontSizeApplier } from './components/FontSizeApplier';
import { logger } from '@/services/LoggingService';

// Lazy load Layout and all page components for better code splitting
const Layout = lazy(() => import('./components/Layout').then(m => ({ default: m.Layout })));
const GlobalUtilityDrawer = lazy(() => import('./components/utility-drawer/GlobalUtilityDrawer').then(m => ({ default: m.GlobalUtilityDrawer })));
const HomePage = lazy(() => import('./pages/HomePage').then(m => ({ default: m.HomePage })));
const LoginPage = lazy(() => import('./pages/LoginPage').then(m => ({ default: m.LoginPage })));
const ForgotPasswordPage = lazy(() => import('./pages/ForgotPasswordPage').then(m => ({ default: m.ForgotPasswordPage })));
const ResetPasswordPage = lazy(() => import('./pages/ResetPasswordPage').then(m => ({ default: m.ResetPasswordPage })));
const VerifyEmailPage = lazy(() => import('./pages/VerifyEmailPage').then(m => ({ default: m.VerifyEmailPage })));
const DashboardPage = lazy(() => import('./pages/DashboardPage').then(m => ({ default: m.DashboardPage })));
const GamesPage = lazy(() => import('./pages/GamesPage').then(m => ({ default: m.GamesPage })));
const GameDetailsPage = lazy(() => import('./pages/GameDetailsPage').then(m => ({ default: m.GameDetailsPage })));
const NotificationsPage = lazy(() => import('./pages/NotificationsPage'));
const SettingsPage = lazy(() => import('./pages/SettingsPage').then(m => ({ default: m.SettingsPage })));
const AdminPage = lazy(() => import('./pages/AdminPage').then(m => ({ default: m.AdminPage })));
const UserProfilePage = lazy(() => import('./pages/UserProfilePage').then(m => ({ default: m.UserProfilePage })));
const CharacterPage = lazy(() => import('./pages/CharacterPage').then(m => ({ default: m.CharacterPage })));
const ThemeTestPage = lazy(() => import('./pages/ThemeTestPage'));
const CommunityGuidelinesPage = lazy(() => import('./pages/CommunityGuidelinesPage').then(m => ({ default: m.CommunityGuidelinesPage })));

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      staleTime: 5 * 60 * 1000, // 5 minutes
      refetchOnWindowFocus: false, // Prevent automatic refetch on tab switch to preserve user input and scroll position
    },
  },
});

// React Router's data router routes a render/loader error to the nearest
// `errorElement` instead of walking up the React component tree, so a lazy
// route chunk failing to load here never reaches ErrorBoundary's
// componentDidCatch (and its chunk-load auto-reload) even though
// ErrorBoundary wraps the whole router. This is the router-level counterpart:
// same chunk-load detection, reload on match, otherwise re-throw so it still
// surfaces through the normal error boundary / logging path.
function RouteErrorElement() {
  const error = useRouteError();

  if (error instanceof Error && isChunkLoadError(error)) {
    window.location.reload();
    return null;
  }

  throw error;
}

// Loading fallback component
function PageLoader() {
  return (
    <div className="min-h-screen surface-page flex items-center justify-center">
      <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-interactive-primary"></div>
    </div>
  );
}

// Skeleton shown while the GameDetailsPage chunk is downloading
function GamePageSkeleton() {
  return (
    <div className="min-h-screen surface-page">
      <div className="max-w-7xl mx-auto md:px-4 py-4 md:py-8">
        <div className="surface-base shadow-md py-4 px-3 md:p-6 mb-6 md:rounded-lg animate-pulse">
          <div className="flex items-start justify-between">
            <div className="flex-1">
              <div className="h-7 surface-raised rounded w-2/3 mb-3"></div>
              <div className="flex gap-2">
                <div className="h-5 surface-raised rounded-full w-20"></div>
                <div className="h-5 surface-raised rounded-full w-16"></div>
                <div className="h-5 surface-raised rounded-full w-24"></div>
              </div>
            </div>
            <div className="h-8 surface-raised rounded w-24 ml-4"></div>
          </div>
          <div className="mt-4 space-y-2">
            <div className="h-4 surface-raised rounded w-full"></div>
            <div className="h-4 surface-raised rounded w-5/6"></div>
          </div>
        </div>
        <div className="surface-base shadow-sm md:rounded-lg mb-6 animate-pulse">
          <div className="flex gap-1 p-2 border-b border-theme-default overflow-x-auto">
            {[1,2,3,4].map(i => (
              <div key={i} className="h-8 surface-raised rounded w-20 flex-shrink-0"></div>
            ))}
          </div>
          <div className="p-4 md:p-6 space-y-4">
            <div className="h-4 surface-raised rounded w-full"></div>
            <div className="h-4 surface-raised rounded w-4/5"></div>
            <div className="h-4 surface-raised rounded w-3/5"></div>
            <div className="h-32 surface-raised rounded w-full mt-4"></div>
          </div>
        </div>
      </div>
    </div>
  );
}

// Root layout: wraps every route with the shared Layout + Suspense.
//
// UtilityDrawerProvider sits above Layout so the nav's Utilities button and the
// drawer/character-sheet modal it controls are available on every page. Game
// pages contribute their game context to it from further down the tree.
function RootLayout() {
  const { isAuthenticated } = useAuth();
  return (
    <UtilityDrawerProvider>
      <Layout>
        {isAuthenticated && <FontSizeApplier />}
        <Suspense fallback={<PageLoader />}>
          <Outlet />
        </Suspense>
      </Layout>
      {/* No fallback: the drawer renders nothing until opened, so there is
          nothing to show while its chunk loads. */}
      {isAuthenticated && (
        <Suspense fallback={null}>
          <GlobalUtilityDrawer />
        </Suspense>
      )}
    </UtilityDrawerProvider>
  );
}

// These render the public page immediately instead of blocking on the /auth/me
// check (isCheckingAuth) first. Blocking added ~2.3s to FCP on /login alone,
// since /auth/me always fires here and always 401s for the logged-out users
// who make up the vast majority of visits to these routes. An already-
// authenticated user hitting one of these URLs sees a brief flash of the
// public page before this redirects them once the check resolves — an
// acceptable tradeoff for a rare case, versus a slow first paint for everyone.
function AuthGatedLogin() {
  const { isAuthenticated } = useAuth();
  return isAuthenticated ? <Navigate to="/dashboard" replace /> : <LoginPage />;
}

function AuthGatedForgotPassword() {
  const { isAuthenticated } = useAuth();
  return isAuthenticated ? <Navigate to="/dashboard" replace /> : <ForgotPasswordPage />;
}

function AuthGatedResetPassword() {
  const { isAuthenticated } = useAuth();
  return isAuthenticated ? <Navigate to="/dashboard" replace /> : <ResetPasswordPage />;
}

function CatchAll() {
  const { isAuthenticated } = useAuth();
  return <Navigate to={isAuthenticated ? '/dashboard' : '/login'} replace />;
}

function GameDetailsPageWrapper() {
  const { gameId } = useParams<{ gameId: string }>();

  if (!gameId) {
    return <Navigate to="/games" replace />;
  }

  const gameIdNum = parseInt(gameId, 10);

  return (
    <GameProvider gameId={gameIdNum}>
      <GameDetailsPage gameId={gameIdNum} />
    </GameProvider>
  );
}

const router = createBrowserRouter([
  {
    path: '/',
    element: <Suspense fallback={<PageLoader />}><HomePage /></Suspense>,
    errorElement: <RouteErrorElement />,
  },
  {
    element: <RootLayout />,
    errorElement: <RouteErrorElement />,
    children: [
      { path: '/community-guidelines', element: <CommunityGuidelinesPage /> },
      { path: '/login', element: <AuthGatedLogin /> },
      { path: '/forgot-password', element: <AuthGatedForgotPassword /> },
      { path: '/reset-password', element: <AuthGatedResetPassword /> },
      { path: '/verify-email', element: <VerifyEmailPage /> },
      {
        path: '/dashboard',
        element: <ProtectedRoute><DashboardPage /></ProtectedRoute>,
      },
      {
        path: '/notifications',
        element: <ProtectedRoute><NotificationsPage /></ProtectedRoute>,
      },
      {
        path: '/settings',
        element: <ProtectedRoute><SettingsPage /></ProtectedRoute>,
      },
      {
        path: '/admin',
        element: <ProtectedRoute requireAdmin><AdminPage /></ProtectedRoute>,
      },
      {
        path: '/admin/:tab',
        element: <ProtectedRoute requireAdmin><AdminPage /></ProtectedRoute>,
      },
      {
        path: '/theme-test',
        element: <ProtectedRoute><ThemeTestPage /></ProtectedRoute>,
      },
      {
        path: '/games',
        element: <ProtectedRoute><GamesPage /></ProtectedRoute>,
      },
      {
        path: '/games/recruiting',
        element: <Navigate to="/games?states=recruitment" replace />,
      },
      {
        path: '/games/:gameId',
        element: (
          <Suspense fallback={<GamePageSkeleton />}>
            <PublicArchiveRoute><GameDetailsPageWrapper /></PublicArchiveRoute>
          </Suspense>
        ),
      },
      {
        path: '/users/:username',
        element: <ProtectedRoute><UserProfilePage /></ProtectedRoute>,
      },
      {
        path: '/characters/:characterId',
        element: <ProtectedRoute><CharacterPage /></ProtectedRoute>,
      },
      { path: '*', element: <CatchAll /> },
    ],
  },
]);

function App() {
  useEffect(() => {
    // Log application initialization
    logger.info('ActionPhase application initialized', {
      environment: import.meta.env.MODE,
      baseUrl: import.meta.env.VITE_API_BASE_URL || 'proxy',
      version: import.meta.env.VITE_APP_VERSION || 'development',
    });
  }, []);

  return (
    <ErrorBoundary>
      <QueryClientProvider client={queryClient}>
        <ToastProvider>
          <AuthProvider>
            <AdminModeProvider>
              <ScreenshotModeProvider>
                <ThemeProvider>
                  <RouterProvider router={router} />
                </ThemeProvider>
              </ScreenshotModeProvider>
            </AdminModeProvider>
          </AuthProvider>
        </ToastProvider>
      </QueryClientProvider>
    </ErrorBoundary>
  );
}

export default App;
