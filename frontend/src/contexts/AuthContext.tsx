import React, { createContext, useContext, useEffect, useState, useCallback } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '../lib/api';
import { simpleApi } from '../lib/simple-api';
import type { LoginRequest, RegisterRequest, User, AuthResponse } from '../types/auth';
import type { AxiosResponse } from 'axios';
import { logger } from '@/services/LoggingService';
import { SessionExpiredModal } from '@/components/SessionExpiredModal';
import { setFaroUser, clearFaroUser } from '@/lib/faro';

interface AuthContextValue {
  // User data
  currentUser: User | null;
  isAuthenticated: boolean;

  // Loading states
  isLoading: boolean;
  isCheckingAuth: boolean;

  // Auth methods
  login: (data: LoginRequest) => Promise<void>;
  register: (data: RegisterRequest) => Promise<AxiosResponse<AuthResponse>>;
  logout: () => void;
  clearError: () => void;

  // Error state
  error: Error | null;
}

const AuthContext = createContext<AuthContextValue | undefined>(undefined);

interface AuthProviderProps {
  children: React.ReactNode;
}

export function AuthProvider({ children }: AuthProviderProps) {
  const queryClient = useQueryClient();
  const [authError, setAuthError] = useState<Error | null>(null);
  const [showSessionExpiredModal, setShowSessionExpiredModal] = useState(false);


  // Check if user is authenticated by fetching current user data
  // This works for both localStorage tokens AND HTTP-only cookies
  // Note: This query runs on all pages (including public ones) which causes 401 errors
  // in the console for unauthenticated users. This is expected and normal behavior.
  const {
    data: currentUser,
    isLoading: isCheckingAuth,
    error: userError,
    isError: hasAuthError,
  } = useQuery({
    queryKey: ['currentUser'],
    queryFn: async () => {
      logger.debug('Checking authentication via /auth/me');
      const response = await apiClient.auth.getCurrentUser();
      if (!response.data || 'user' in response.data && response.data.user === null) {
        logger.debug('Not authenticated');
        return null;
      }
      const user = response.data as import('../types/auth').User;
      logger.debug('Authentication successful', { userId: user.id, username: user.username });
      return user;
    },
    retry: false,
    staleTime: 5 * 60 * 1000, // Cache for 5 minutes
    // refetchOnWindowFocus: false is the global default - prevents cascading re-renders
  });

  // Derive authentication state from currentUser query
  const isAuthenticated = !hasAuthError && currentUser !== null && currentUser !== undefined;

  // Attach the user's id to Faro so traces carry business context; clear it on
  // logout. Driven off currentUser so it covers login, register, and session
  // restore via /auth/me in one place. Only the id is sent — no PII in Grafana.
  useEffect(() => {
    if (currentUser?.id !== undefined && currentUser.id !== null) {
      setFaroUser(currentUser.id);
    } else {
      clearFaroUser();
    }
  }, [currentUser?.id]);

  // Log user error if it occurs
  useEffect(() => {
    if (userError) {
      const status = (userError as { response?: { status?: number } }).response?.status;
      if (status === 401) {
        logger.debug('Not authenticated (no session)');
      } else {
        logger.error('Failed to load user data', { error: userError });
      }
      setAuthError(userError as Error);
    }
  }, [userError]);

  // Login mutation
  const loginMutation = useMutation({
    mutationFn: async (data: LoginRequest) => {
      logger.debug('Attempting login', { username: data.username });
      const response = await apiClient.auth.login(data);
      return response;
    },
    onSuccess: (response) => {
      const token = response.data.Token || response.data.token;
      logger.info('Login successful', { hasToken: !!token });

      if (token) {
        apiClient.setAuthToken(token);
        // Prefetch dashboard in parallel with the auth re-check so data is already
        // in cache when DashboardPage mounts, eliminating the sequential waterfall.
        queryClient.prefetchQuery({
          queryKey: ['dashboard'],
          queryFn: () => simpleApi.getDashboard().then(r => r.data),
        });
        queryClient.invalidateQueries({ queryKey: ['currentUser'] });
        setAuthError(null);
      }
    },
    onError: (error: Error) => {
      logger.error('Login failed', { error });
      setAuthError(error);
    },
  });

  // Register mutation
  const registerMutation = useMutation({
    mutationFn: async (data: RegisterRequest) => {
      logger.debug('Attempting registration', { username: data.username, email: data.email });
      const response = await apiClient.auth.register(data);
      return response;
    },
    onSuccess: (response) => {
      const token = response.data.Token || response.data.token;
      logger.info('Registration successful', { hasToken: !!token });

      if (token) {
        apiClient.setAuthToken(token);
        queryClient.prefetchQuery({
          queryKey: ['dashboard'],
          queryFn: () => simpleApi.getDashboard().then(r => r.data),
        });
        queryClient.invalidateQueries({ queryKey: ['currentUser'] });
        setAuthError(null);
      }
    },
    onError: (error: Error) => {
      logger.error('Registration failed', { error });
      setAuthError(error);
    },
  });

  // Logout function
  const logout = async () => {
    logger.info('User logging out');

    // Mark logout as in progress to prevent token refresh attempts
    apiClient.startLogout();

    try {
      // Call backend to invalidate session/clear cookie
      await apiClient.auth.logout();
      logger.debug('Backend logout successful');
    } catch (error) {
      // Log error but continue with frontend logout
      logger.error('Backend logout failed', { error });
    } finally {
      // Always clear frontend state regardless of backend response
      apiClient.removeAuthToken();
      queryClient.setQueryData(['currentUser'], null);
      queryClient.clear();
      setAuthError(null);

      // Clear logout flag after cleanup is complete
      apiClient.endLogout();
    }
  };

  // Listen for session expiry events from API client (token refresh failure)
  useEffect(() => {
    const handleSessionExpired = () => {
      logger.info('Handling auth:sessionExpired event - showing re-auth modal');
      setShowSessionExpiredModal(true);
    };

    window.addEventListener('auth:sessionExpired', handleSessionExpired);

    return () => {
      window.removeEventListener('auth:sessionExpired', handleSessionExpired);
    };
  }, []);

  // Combined loading state
  const isLoading = loginMutation.isPending || registerMutation.isPending;

  // Function to clear auth errors - memoized to prevent infinite loops
  const clearError = useCallback(() => {
    setAuthError(null);
    loginMutation.reset();
    registerMutation.reset();
  }, [loginMutation, registerMutation]);

  const value: AuthContextValue = {
    currentUser: currentUser || null,
    isAuthenticated: isAuthenticated || false,
    isLoading,
    isCheckingAuth: isCheckingAuth,
    login: async (data: LoginRequest) => {
      await loginMutation.mutateAsync(data);
    },
    register: async (data: RegisterRequest) => {
      const response = await registerMutation.mutateAsync(data);
      return response;
    },
    logout,
    clearError,
    error: authError || loginMutation.error || registerMutation.error,
  };

  logger.debug('AuthContext state updated', {
    isAuthenticated: value.isAuthenticated,
    hasUser: !!value.currentUser,
    userId: value.currentUser?.id,
    isLoading: value.isLoading,
    isCheckingAuth: value.isCheckingAuth,
  });

  const handleSessionExpiredSuccess = () => {
    logger.info('Re-authentication successful, closing session expired modal');
    setShowSessionExpiredModal(false);
    // Invalidate all queries so components refresh with the new session
    queryClient.invalidateQueries();
  };

  return (
    <AuthContext.Provider value={value}>
      {children}
      <SessionExpiredModal
        isOpen={showSessionExpiredModal}
        onSuccess={handleSessionExpiredSuccess}
      />
    </AuthContext.Provider>
  );
}

// eslint-disable-next-line react-refresh/only-export-components
export function useAuth() {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}
