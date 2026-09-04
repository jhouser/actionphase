/**
 * Error Message Mapper
 * Converts backend error responses into user-friendly messages
 */

import { logger } from '@/services/LoggingService';
import { extractApiErrorMessage } from '@/lib/errors';

interface ErrorResponse {
  response?: {
    data?: {
      message?: string;
    };
    status?: number;
  };
  message?: string;
}

/**
 * Maps authentication-related errors to user-friendly messages
 */
export function mapAuthError(error: ErrorResponse | unknown): string {
  // Handle different error formats
  const rawError = error as ErrorResponse;
  const errorMessage =
    rawError?.response?.data?.message ||
    extractApiErrorMessage(error) ||
    rawError?.message ||
    '';

  const lowerMessage = errorMessage.toLowerCase();

  // Password validation errors
  if (lowerMessage.includes('password')) {
    // Check specific requirements first (more specific matches)
    if (lowerMessage.includes('special character')) {
      return 'Password must contain at least one special character (!@#$%^&*)';
    }
    if (lowerMessage.includes('uppercase')) {
      return 'Password must contain at least one uppercase letter (A-Z)';
    }
    if (lowerMessage.includes('lowercase')) {
      return 'Password must contain at least one lowercase letter (a-z)';
    }
    if (lowerMessage.includes('number') || lowerMessage.includes('digit')) {
      return 'Password must contain at least one number (0-9)';
    }
    // Check length requirements last (less specific)
    if (lowerMessage.includes('at least') && lowerMessage.includes('character')) {
      return 'Password must be at least 8 characters long';
    }
    if (lowerMessage.includes('at most') && lowerMessage.includes('character')) {
      return 'Password must be at most 128 characters long';
    }
    if (lowerMessage.includes('common') || lowerMessage.includes('weak') || lowerMessage.includes('easy to guess')) {
      return 'This password is too common. Please choose a more unique password.';
    }
    if (lowerMessage.includes('do not match') || lowerMessage.includes('match')) {
      return 'Passwords do not match. Please make sure both passwords are identical.';
    }
  }

  // Username validation errors
  if (lowerMessage.includes('username')) {
    if (lowerMessage.includes('taken') || lowerMessage.includes('already exists') || lowerMessage.includes('in use')) {
      return 'This username is already taken. Please choose a different username.';
    }
    if (lowerMessage.includes('invalid') || lowerMessage.includes('format')) {
      return 'Username must be 3-20 characters and contain only letters, numbers, and underscores';
    }
    if (lowerMessage.includes('too short') || lowerMessage.includes('at least')) {
      return 'Username must be at least 3 characters long';
    }
    if (lowerMessage.includes('too long') || lowerMessage.includes('at most')) {
      return 'Username must be at most 20 characters long';
    }
  }

  // Email validation errors
  if (lowerMessage.includes('email')) {
    if (lowerMessage.includes('taken') || lowerMessage.includes('already exists') || lowerMessage.includes('in use')) {
      return 'This email address is already registered. Please use a different email or try logging in.';
    }
    if (lowerMessage.includes('invalid') || lowerMessage.includes('format')) {
      return 'Please enter a valid email address';
    }
    if (lowerMessage.includes('disposable') || lowerMessage.includes('temporary')) {
      return 'Disposable email addresses are not allowed. Please use a permanent email address.';
    }
  }

  // Authentication errors
  if (lowerMessage.includes('invalid credentials') ||
      lowerMessage.includes('unauthorized') ||
      lowerMessage.includes('incorrect password') ||
      lowerMessage.includes('wrong password')) {
    return 'Invalid username or password. Please check your credentials and try again.';
  }

  if (lowerMessage.includes('account not found') || lowerMessage.includes('user not found')) {
    return 'No account found with these credentials. Please check your username or sign up.';
  }

  // CAPTCHA errors
  if (lowerMessage.includes('captcha') || lowerMessage.includes('hcaptcha')) {
    if (lowerMessage.includes('required') || lowerMessage.includes('missing')) {
      return 'Please complete the CAPTCHA verification';
    }
    if (lowerMessage.includes('invalid') || lowerMessage.includes('failed')) {
      return 'CAPTCHA verification failed. Please try again.';
    }
    if (lowerMessage.includes('expired')) {
      return 'CAPTCHA has expired. Please verify again.';
    }
  }

  // Rate limiting errors
  if (lowerMessage.includes('rate limit') ||
      lowerMessage.includes('too many') ||
      lowerMessage.includes('try again later')) {
    return 'Too many attempts. Please wait a few minutes before trying again.';
  }

  // Network errors
  if (lowerMessage.includes('network') || lowerMessage.includes('connection')) {
    return 'Network error. Please check your connection and try again.';
  }

  // HTTP status code errors
  if (rawError?.response?.status) {
    const status = rawError.response.status;
    // 400 (could not parse) and 422 (parsed, failed validation) are both
    // "your input was wrong". The specific-message branches above already
    // caught anything we can phrase well; reaching here means the message is
    // raw server text -- for 422 that is a schema error like
    // "expected length >= 8 (body.password: x)", which must not be shown.
    if (status === 400 || status === 422) {
      return 'Invalid request. Please check your information and try again.';
    }
    if (status === 401) {
      return 'Authentication failed. Please check your credentials.';
    }
    if (status === 403) {
      return 'Access denied. You do not have permission to perform this action.';
    }
    if (status === 404) {
      return 'Resource not found. The requested item does not exist.';
    }
    if (status === 409) {
      return 'Conflict. The information you provided conflicts with existing data.';
    }
    if (status === 429) {
      return 'Too many requests. Please wait a moment before trying again.';
    }
    if (status >= 500) {
      return 'Server error. Please try again later or contact support if the problem persists.';
    }
  }

  // If we have a specific error message that didn't match any patterns,
  // clean it up a bit (remove field prefixes like "password: ")
  if (errorMessage) {
    const cleaned = errorMessage.replace(/^(password|username|email|confirm_?password):\s*/i, '');

    // If the cleaned message looks generic or too short, use fallback
    const genericTerms = ['error', 'failed', 'unknown', 'something went wrong'];
    const lowerCleaned = cleaned.toLowerCase();
    const isGeneric = genericTerms.some(term => lowerCleaned.includes(term));

    // If the cleaned message is substantial and not generic, return it
    if (cleaned.length > 10 && cleaned.length < 150 && !isGeneric) {
      // Capitalize first letter
      return cleaned.charAt(0).toUpperCase() + cleaned.slice(1);
    }
  }

  // Generic fallback
  logger.error('Unmapped error', { errorMessage, rawError });
  return 'An error occurred. Please try again or contact support if the problem persists.';
}

/**
 * Password requirements for validation
 */
export interface PasswordRequirement {
  text: string;
  met: boolean;
  key: string;
}

/**
 * Validates password against requirements and returns status of each requirement
 */
export function validatePasswordRequirements(password: string): PasswordRequirement[] {
  return [
    {
      key: 'length',
      text: 'At least 8 characters',
      met: password.length >= 8,
    },
    {
      key: 'uppercase',
      text: 'One uppercase letter (A-Z)',
      met: /[A-Z]/.test(password),
    },
    {
      key: 'lowercase',
      text: 'One lowercase letter (a-z)',
      met: /[a-z]/.test(password),
    },
    {
      key: 'number',
      text: 'One number (0-9)',
      met: /\d/.test(password),
    },
    {
      key: 'special',
      text: 'One special character (!@#$%^&*)',
      met: /[!@#$%^&*()_+\-=[\]{};':"\\|,.<>/?]/.test(password),
    },
  ];
}

/**
 * Checks if all password requirements are met
 */
export function isPasswordValid(password: string): boolean {
  const requirements = validatePasswordRequirements(password);
  return requirements.every(req => req.met);
}
