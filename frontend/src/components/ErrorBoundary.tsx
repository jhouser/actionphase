import React, { Component } from 'react';
import type { ErrorInfo } from 'react';
import type { AppError } from '../types/errors';
import { ErrorType, ErrorSeverity } from '../types/errors';
import { createAppError, logError, getErrorMessage, getRecoveryActions } from '../lib/errors';
import { Button } from './ui';
import { pushError } from '../lib/faro';
import { isChunkLoadError } from '../lib/chunkLoadError';

interface Props {
  children: React.ReactNode;
  fallback?: React.ComponentType<ErrorBoundaryFallbackProps>;
  onError?: (error: AppError, errorInfo: ErrorInfo) => void;
}

interface State {
  hasError: boolean;
  error: AppError | null;
  errorId: string | null;
}

export interface ErrorBoundaryFallbackProps {
  error: AppError;
  resetError: () => void;
  errorId: string;
}

/**
 * Error Boundary that catches React component errors and displays a user-friendly fallback UI.
 * Integrates with the ActionPhase error handling system for consistent error management.
 */
export class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = {
      hasError: false,
      error: null,
      errorId: null,
    };
  }

  static getDerivedStateFromError(error: Error): Partial<State> {
    const appError = createAppError(error, {
      type: ErrorType.COMPONENT_ERROR,
      severity: ErrorSeverity.HIGH,
      source: 'ErrorBoundary',
    });

    return {
      hasError: true,
      error: appError,
      errorId: generateErrorId(),
    };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo): void {
    // Chunk load failures happen when the browser has cached JS from an old deployment.
    // Auto-reload fetches the new index.html and correct chunk URLs.
    if (isChunkLoadError(error)) {
      window.location.reload();
      return;
    }

    const appError = createAppError(error, {
      type: ErrorType.COMPONENT_ERROR,
      severity: ErrorSeverity.HIGH,
      source: 'ErrorBoundary',
      metadata: {
        componentStack: errorInfo.componentStack,
        errorBoundary: this.constructor.name,
      },
    });

    // Log the error with React-specific context
    logError(appError, {
      componentStack: errorInfo.componentStack,
      errorBoundary: this.constructor.name,
    });

    // Ship error to Grafana Faro for RUM tracking
    pushError(error, {
      errorBoundary: this.constructor.name,
      errorId: this.state.errorId ?? '',
    });

    // Call custom error handler if provided
    this.props.onError?.(appError, errorInfo);

    // Update state with the processed error
    this.setState({
      error: appError,
      errorId: generateErrorId(),
    });
  }

  resetError = (): void => {
    this.setState({
      hasError: false,
      error: null,
      errorId: null,
    });
  };

  render(): React.ReactNode {
    if (this.state.hasError && this.state.error && this.state.errorId) {
      const FallbackComponent = this.props.fallback || DefaultErrorFallback;

      return (
        <FallbackComponent
          error={this.state.error}
          resetError={this.resetError}
          errorId={this.state.errorId}
        />
      );
    }

    return this.props.children;
  }
}

/**
 * Default error fallback UI component
 */
const DefaultErrorFallback: React.FC<ErrorBoundaryFallbackProps> = ({
  error,
  resetError,
  errorId
}) => {
  const message = getErrorMessage(error);
  const recoveryActions = getRecoveryActions(error);

  return (
    <div className="min-h-screen flex items-center justify-center surface-sunken py-12 px-4 sm:px-6 lg:px-8">
      <div className="max-w-md w-full space-y-8 surface-base p-6 rounded-lg shadow-md">
        <div className="text-center">
          <div className="mx-auto h-12 w-12 text-semantic-danger">
            <svg
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
              xmlns="http://www.w3.org/2000/svg"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L3.732 16.5c-.77.833.192 2.5 1.732 2.5z"
              />
            </svg>
          </div>

          <h2 className="mt-4 text-lg font-medium text-content-primary">
            Something went wrong
          </h2>

          <p className="mt-2 text-sm text-content-secondary">
            {message}
          </p>

          {recoveryActions.length > 0 && (
            <div className="mt-4 text-left">
              <p className="text-sm font-medium text-content-primary mb-2">
                You can try to:
              </p>
              <ul className="text-sm text-content-secondary space-y-1">
                {recoveryActions.map((action, index) => (
                  <li key={index} className="flex items-start">
                    <span className="text-interactive-primary mr-2">•</span>
                    {action}
                  </li>
                ))}
              </ul>
            </div>
          )}

          <div className="mt-6 flex flex-col space-y-3">
            <Button
              variant="primary"
              onClick={resetError}
              className="w-full"
            >
              Try Again
            </Button>

            <Button
              variant="outline"
              onClick={() => window.location.reload()}
              className="w-full"
            >
              Reload Page
            </Button>
          </div>

          {/* Error ID for support */}
          <div className="mt-4 pt-4 border-t border-theme-default">
            <p className="text-xs text-content-tertiary">
              Error ID: <code className="surface-raised px-1 py-0.5 rounded">{errorId}</code>
            </p>
          </div>
        </div>
      </div>
    </div>
  );
};

/**
 * Specialized Error Boundary for forms with field-specific error handling
 */
export const FormErrorBoundary: React.FC<{
  children: React.ReactNode;
  onFieldError?: (field: string, error: AppError) => void;
}> = ({ children, onFieldError }) => {
  const handleError = (error: AppError) => {
    if (error.context?.metadata?.field && onFieldError) {
      onFieldError(error.context.metadata.field as string, error);
    }
  };

  return (
    <ErrorBoundary onError={handleError}>
      {children}
    </ErrorBoundary>
  );
};

/**
 * Specialized Error Boundary for async operations with loading states
 */
export const AsyncErrorBoundary: React.FC<{
  children: React.ReactNode;
  fallback?: React.ComponentType<ErrorBoundaryFallbackProps>;
}> = ({ children, fallback }) => {
  return (
    <ErrorBoundary fallback={fallback}>
      {children}
    </ErrorBoundary>
  );
};

/**
 * Higher-order component that wraps components with error boundary
 */
// eslint-disable-next-line react-refresh/only-export-components
export function withErrorBoundary<P extends object>(
  Component: React.ComponentType<P>,
  fallback?: React.ComponentType<ErrorBoundaryFallbackProps>
) {
  const WrappedComponent = (props: P) => (
    <ErrorBoundary fallback={fallback}>
      <Component {...props} />
    </ErrorBoundary>
  );

  WrappedComponent.displayName = `withErrorBoundary(${Component.displayName || Component.name})`;
  return WrappedComponent;
}

function generateErrorId(): string {
  return `err_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
}
