import axios from 'axios';
import { logger, setCorrelationId } from '@/services/LoggingService';

const API_BASE_URL = ''; // Use proxy in development

/**
 * Base API client with authentication and token refresh interceptors
 */
export class BaseApiClient {
  protected client: ReturnType<typeof axios.create>;
  protected refreshClient: ReturnType<typeof axios.create>;
  private refreshPromise: Promise<void> | null = null;
  private isLoggingOut: boolean = false;
  private lastLogoutTime: number = 0;

  constructor() {
    this.client = axios.create({
      baseURL: API_BASE_URL,
      headers: {
        'Content-Type': 'application/json',
      },
      withCredentials: true, // Required to send HTTP-only cookies with requests
    });

    // Separate client for refresh requests to avoid interceptor loops
    this.refreshClient = axios.create({
      baseURL: API_BASE_URL,
      headers: {
        'Content-Type': 'application/json',
      },
      withCredentials: true, // Required to send HTTP-only cookies with requests
    });

    // Add request interceptor to include auth token and admin mode header
    // Note: Authentication uses BOTH localStorage tokens AND HTTP-only cookies
    // for backwards compatibility during migration
    this.client.interceptors.request.use((config) => {
      // Add Authorization header if token exists in localStorage
      const token = localStorage.getItem('auth_token');
      if (token) {
        config.headers.Authorization = `Bearer ${token}`;
      }

      // Add admin mode header if enabled
      const adminModeEnabled = localStorage.getItem('admin_mode_enabled');
      if (adminModeEnabled === 'true') {
        config.headers['X-Admin-Mode'] = 'true';
      }

      // Trace context propagation is handled by Faro's TracingInstrumentation,
      // which patches XHR/fetch (axios uses XHR) and injects a single W3C
      // `traceparent` on each request. We intentionally do NOT inject here:
      // a second manual injection produced a malformed comma-joined `traceparent`
      // with two trace IDs, which the backend's W3C propagator rejected — so every
      // backend span became a disconnected root. Faro is the sole injector.

      // Log API request
      logger.debug('API Request', {
        method: config.method?.toUpperCase(),
        url: config.url,
        hasAuth: !!token,
        adminMode: adminModeEnabled === 'true',
      });

      return config;
    });

    // Add response interceptor to handle token refresh
    this.client.interceptors.response.use(
      (response) => {
        // Extract and store correlation ID from response headers
        const correlationId =
          response.headers['x-correlation-id'] ||
          response.headers['x-request-id'] ||
          null;

        if (correlationId) {
          setCorrelationId(correlationId);
        }

        // Log successful API response
        logger.debug('API Response', {
          method: response.config.method?.toUpperCase(),
          url: response.config.url,
          status: response.status,
          correlationId,
        });

        return response;
      },
      async (error) => {
        const originalRequest = error.config;

        // Downgrade expected non-errors to debug:
        // - /auth/me 401: expected when unauthenticated (probe endpoint)
        // - exports/latest 404: expected when a completed game has never been
        //   exported; "no export yet" is a normal state, not a failure.
        const isExpectedNonError =
          (error.response?.status === 401 && error.config?.url?.includes('/auth/me')) ||
          (error.response?.status === 404 && error.config?.url?.includes('/exports/latest'));
        const logFn = isExpectedNonError ? logger.debug : logger.error;
        logFn('API Request Failed', {
          method: error.config?.method?.toUpperCase(),
          url: error.config?.url,
          status: error.response?.status,
          error,
        });

        // Don't try to refresh token for auth endpoints (login, register, logout, me)
        // /auth/me is used to CHECK if user is authenticated, so it shouldn't trigger refresh
        // /auth/logout is intentional logout, so it shouldn't trigger refresh
        const isAuthEndpoint = originalRequest.url?.includes('/auth/login') ||
                               originalRequest.url?.includes('/auth/register') ||
                               originalRequest.url?.includes('/auth/logout') ||
                               originalRequest.url?.includes('/auth/me');

        // Don't try to refresh token if logout is in progress
        const shouldSkipRefresh = isAuthEndpoint || this.isLoggingOut;

        logger.debug('401 Response - Refresh Decision', {
          url: originalRequest.url,
          isAuthEndpoint,
          isLoggingOut: this.isLoggingOut,
          shouldSkipRefresh,
          willAttemptRefresh: !shouldSkipRefresh
        });

        if (error.response?.status === 401 && !originalRequest._retry && !shouldSkipRefresh) {
          originalRequest._retry = true;

          logger.info('Token refresh needed', {
            url: originalRequest.url,
            method: originalRequest.method,
          });

          try {
            // If a refresh is already in progress, wait for it instead of starting a new one
            if (this.refreshPromise) {
              await this.refreshPromise;
              // Refresh completed, retry the original request
              logger.debug('Retrying request after token refresh', {
                url: originalRequest.url,
              });
              return this.client(originalRequest);
            }

            // Start a new refresh operation
            this.refreshPromise = this.performTokenRefresh();

            try {
              await this.refreshPromise;
              // Refresh successful, retry the original request
              logger.info('Token refresh successful, retrying request', {
                url: originalRequest.url,
              });
              return this.client(originalRequest);
            } finally {
              // Clear the refresh promise so future requests can trigger a new refresh if needed
              this.refreshPromise = null;
            }
          } catch (refreshError) {
            logger.error('Token refresh failed', {
              error: refreshError,
              originalUrl: originalRequest.url,
            });

            // Clear any legacy localStorage tokens
            localStorage.removeItem('auth_token');

            // Don't show the modal if this is an expected logout from this tab
            const isRecentLogout = (Date.now() - this.lastLogoutTime) < 2000;
            if (!isRecentLogout) {
              window.dispatchEvent(new CustomEvent('auth:sessionExpired'));
            }
            return Promise.reject(refreshError);
          }
        }

        return Promise.reject(error);
      }
    );
  }

  /**
   * Performs the actual token refresh operation.
   * This method is called by the response interceptor and ensures
   * only one refresh happens at a time via the refreshPromise queue.
   */
  private async performTokenRefresh(): Promise<void> {
    // Authentication is now cookie-based. The refresh endpoint will read
    // the JWT from the HTTP-only cookie automatically.
    // No need to send Authorization header - the browser handles cookies.
    await this.refreshClient.get('/api/v1/auth/refresh');

    // The backend sets a new HTTP-only JWT cookie in the response.
    // We don't need to do anything with the token - the browser handles it.
  }

  // Utility methods for token management
  setAuthToken(token: string) {
    if (!token || token.trim() === '' || token === 'undefined' || token === 'null') {
      logger.error('Attempted to set invalid token');
      localStorage.removeItem('auth_token');
      return;
    }
    logger.debug('Setting auth token');
    localStorage.setItem('auth_token', token);
  }

  removeAuthToken() {
    localStorage.removeItem('auth_token');
  }

  /**
   * Mark that logout is in progress to prevent token refresh attempts
   */
  startLogout() {
    logger.info('Setting logout flag - preventing token refresh attempts');
    this.isLoggingOut = true;
    this.lastLogoutTime = Date.now();
  }

  /**
   * Clear the logout flag after logout is complete
   */
  endLogout() {
    logger.info('Clearing logout flag - token refresh attempts allowed again');
    this.isLoggingOut = false;
  }

  getAuthToken(): string | null {
    // With cookie-based authentication, we can't directly access the HTTP-only cookie.
    // Check localStorage for legacy token (for backwards compatibility during migration),
    // but always return a sentinel value indicating cookie-based auth is in use.
    // The actual authentication state should be verified by making an API call to /auth/me.
    const legacyToken = localStorage.getItem('auth_token');
    if (legacyToken) {
      // Legacy token exists, return it for backwards compatibility
      return legacyToken;
    }
    // No localStorage token, but cookies might be present
    // Return null - callers should verify auth via API call instead
    return null;
  }

  // Health check
  async ping() {
    return this.client.get<string>('/ping');
  }
}
