import { AxiosError } from 'axios';
import type {
  ApiError,
  ApiErrorDetail,
  AppError,
  ErrorContext
} from '../types/errors';
import {
  ErrorType,
  ErrorCategory,
  ErrorSeverity,
  ERROR_MESSAGES,
  STATUS_CODE_TO_ERROR_TYPE
} from '../types/errors';
import { logger } from '@/services/LoggingService';

/**
 * Pulls a displayable message out of an API error response body.
 *
 * Understands both shapes the API can return (see `ApiError`): the legacy
 * `error` field and RFC 7807's `detail` / `errors[]`.
 *
 * Deliberately never returns `status`. In the legacy shape that field is a
 * human-readable string ("Forbidden."), but under RFC 7807 it is the numeric
 * status code — so falling back to it renders a bare `403` as the user's error
 * message, silently and without throwing. Callers that want a fallback should
 * supply their own via `??`.
 *
 * Returns undefined when the body carries no usable message, which lets callers
 * distinguish "server said nothing" from "server said something" — the
 * distinction the 401/403 fallbacks in handleAxiosError depend on.
 */
export function extractApiErrorMessage(error: unknown): string | undefined {
  const body = extractApiErrorBody(error);
  if (!body) return undefined;

  // Legacy shape first: while both formats are live, an `error` field is the
  // more specific signal, since RFC 7807 responses never carry one.
  if (typeof body.error === 'string' && body.error.trim() !== '') {
    return body.error;
  }

  if (typeof body.detail === 'string' && body.detail.trim() !== '') {
    return body.detail;
  }

  // Field-level validation failures. `detail` for these is generic
  // ("validation failed"), so the individual messages are what a user can act
  // on; join them rather than surfacing only the first.
  const details = Array.isArray(body.errors) ? body.errors : [];
  const messages = details
    .map((detail: ApiErrorDetail) => detail?.message)
    .filter((message): message is string => typeof message === 'string' && message.trim() !== '');
  if (messages.length > 0) {
    return messages.join('. ');
  }

  // `title` is a static summary of the error class rather than of this
  // occurrence, so it is a last resort — but it is still prose, unlike `status`.
  if (typeof body.title === 'string' && body.title.trim() !== '') {
    return body.title;
  }

  return undefined;
}

/**
 * Normalizes the various things callers hold to the error response body:
 * an axios error, an already-unwrapped `response`, or the body itself.
 */
function extractApiErrorBody(error: unknown): ApiError | undefined {
  if (!error || typeof error !== 'object') return undefined;

  const candidate = error as {
    response?: { data?: unknown };
    data?: unknown;
  };

  const body =
    (candidate.response?.data as unknown) ??
    (candidate.data as unknown) ??
    error;

  if (!body || typeof body !== 'object') return undefined;
  return body as ApiError;
}

/**
 * Creates a standardized AppError from various error sources
 */
export function createAppError(
  error: unknown,
  context?: Partial<ErrorContext>
): AppError {
  const baseContext: ErrorContext = {
    type: ErrorType.UNKNOWN_ERROR,
    category: ErrorCategory.NON_RECOVERABLE,
    severity: ErrorSeverity.MEDIUM,
    userMessage: ERROR_MESSAGES.UNKNOWN_ERROR,
    timestamp: new Date(),
    ...context,
  };

  if (isAxiosError(error)) {
    return handleAxiosError(error, baseContext);
  }

  if (error instanceof Error) {
    return handleGenericError(error, baseContext);
  }

  // Unknown error type
  const appError = new Error(baseContext.userMessage) as AppError;
  appError.type = baseContext.type;
  appError.context = baseContext;
  return appError;
}

/**
 * Handles Axios HTTP errors with structured backend responses
 */
function handleAxiosError(error: AxiosError, baseContext: ErrorContext): AppError {
  const statusCode = error.response?.status;
  const apiError = error.response?.data as ApiError;

  const errorType = statusCode && statusCode in STATUS_CODE_TO_ERROR_TYPE
    ? STATUS_CODE_TO_ERROR_TYPE[statusCode as keyof typeof STATUS_CODE_TO_ERROR_TYPE]
    : ErrorType.NETWORK_ERROR;

  let userMessage: string = ERROR_MESSAGES.UNKNOWN_ERROR;
  let category: ErrorCategory = ErrorCategory.NON_RECOVERABLE;
  let severity: ErrorSeverity = ErrorSeverity.MEDIUM;

  // Extract user-friendly message from API response.
  // Note this never falls back to `apiError.status`: under RFC 7807 that is the
  // numeric status code, and displaying it shows the user a bare "422".
  const apiMessage = extractApiErrorMessage(error);
  if (apiMessage) {
    userMessage = apiMessage;
  }

  // Categorize by status code
  switch (statusCode) {
    case 400:
    case 422:
      category = ErrorCategory.RECOVERABLE;
      severity = ErrorSeverity.LOW;
      break;
    case 401:
      // Use API error message if available, otherwise use default session expired message
      if (!apiMessage) {
        userMessage = ERROR_MESSAGES.SESSION_EXPIRED;
      }
      category = ErrorCategory.RECOVERABLE;
      severity = ErrorSeverity.HIGH;
      break;
    case 403:
      // Use API error message if available, otherwise use default unauthorized message
      if (!apiMessage) {
        userMessage = ERROR_MESSAGES.UNAUTHORIZED;
      }
      category = ErrorCategory.NON_RECOVERABLE;
      severity = ErrorSeverity.MEDIUM;
      break;
    case 404:
      category = ErrorCategory.NON_RECOVERABLE;
      severity = ErrorSeverity.LOW;
      break;
    case 500:
    case 502:
    case 503:
    case 504:
      userMessage = ERROR_MESSAGES.SERVER_ERROR;
      category = ErrorCategory.RECOVERABLE;
      severity = ErrorSeverity.HIGH;
      break;
    default:
      if (!statusCode) {
        userMessage = ERROR_MESSAGES.NETWORK_UNAVAILABLE;
        category = ErrorCategory.RECOVERABLE;
        severity = ErrorSeverity.HIGH;
      }
      break;
  }

  const context: ErrorContext = {
    ...baseContext,
    type: errorType,
    category,
    severity,
    userMessage,
    technicalMessage: error.message,
  };

  const appError = new Error(userMessage) as AppError;
  appError.type = errorType;
  appError.statusCode = statusCode;
  appError.apiError = apiError;
  appError.context = context;

  return appError;
}

/**
 * Handles generic JavaScript errors
 */
function handleGenericError(error: Error, baseContext: ErrorContext): AppError {
  const context: ErrorContext = {
    ...baseContext,
    type: ErrorType.COMPONENT_ERROR,
    category: ErrorCategory.NON_RECOVERABLE,
    severity: ErrorSeverity.MEDIUM,
    userMessage: ERROR_MESSAGES.UNKNOWN_ERROR,
    technicalMessage: error.message,
  };

  const appError = error as AppError;
  appError.type = context.type;
  appError.context = context;

  return appError;
}

/**
 * Type guard to check if an error is an AxiosError
 */
function isAxiosError(error: unknown): error is AxiosError {
  return error !== null &&
    typeof error === 'object' &&
    'isAxiosError' in error &&
    (error as AxiosError).isAxiosError === true;
}

/**
 * Extracts a user-friendly error message from an AppError
 */
export function getErrorMessage(error: AppError): string {
  return error.context?.userMessage || error.message || ERROR_MESSAGES.UNKNOWN_ERROR;
}

/**
 * Determines if an error is recoverable (user can retry)
 */
export function isRecoverable(error: AppError): boolean {
  return error.context?.category === ErrorCategory.RECOVERABLE;
}

/**
 * Determines if an error should be displayed to the user
 */
export function shouldDisplayError(error: AppError): boolean {
  return error.context?.category !== ErrorCategory.SILENT;
}

/**
 * Gets recovery actions for an error
 */
export function getRecoveryActions(error: AppError): string[] {
  const actions: string[] = error.context?.recoveryActions || [];

  // Add default recovery actions based on error type
  switch (error.type) {
    case ErrorType.AUTHENTICATION_ERROR:
      actions.push('Log in again');
      break;
    case ErrorType.NETWORK_ERROR:
      actions.push('Check your internet connection', 'Try again later');
      break;
    case ErrorType.VALIDATION_ERROR:
      actions.push('Check your input and try again');
      break;
    case ErrorType.API_ERROR:
      if (error.statusCode && error.statusCode >= 500) {
        actions.push('Try again in a few minutes');
      }
      break;
  }

  return actions;
}

/**
 * Logs an error with appropriate level and context
 * Uses LoggingService for structured logging with correlation ID tracking
 */
export function logError(error: AppError, additionalContext?: Record<string, unknown>): void {
  const logData = {
    type: error.type,
    message: error.message,
    statusCode: error.statusCode,
    context: error.context,
    stack: error.stack,
    ...additionalContext,
  };

  const severity = error.context?.severity || ErrorSeverity.MEDIUM;

  switch (severity) {
    case ErrorSeverity.CRITICAL:
    case ErrorSeverity.HIGH:
      logger.error('Application error', logData);
      break;
    case ErrorSeverity.MEDIUM:
      logger.warn('Application warning', logData);
      break;
    case ErrorSeverity.LOW:
      logger.info('Application info', logData);
      break;
    default:
      logger.debug('Application debug', logData);
  }
}

