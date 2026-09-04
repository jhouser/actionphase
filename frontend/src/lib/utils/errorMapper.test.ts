import { describe, it, expect } from 'vitest';
import { mapAuthError, validatePasswordRequirements, isPasswordValid } from './errorMapper';

describe('mapAuthError', () => {
  describe('Password validation errors', () => {
    it('maps "password must be at least 8 characters" error', () => {
      const error = {
        response: {
          data: {
            message: 'password: must be at least 8 characters long',
          },
        },
      };
      expect(mapAuthError(error)).toBe('Password must be at least 8 characters long');
    });

    it('maps "password must contain uppercase" error', () => {
      const error = {
        response: {
          data: {
            message: 'password: must contain at least one uppercase letter',
          },
        },
      };
      expect(mapAuthError(error)).toBe('Password must contain at least one uppercase letter (A-Z)');
    });

    it('maps "password must contain lowercase" error', () => {
      const error = {
        response: {
          data: {
            message: 'password: must contain at least one lowercase letter',
          },
        },
      };
      expect(mapAuthError(error)).toBe('Password must contain at least one lowercase letter (a-z)');
    });

    it('maps "password must contain number" error', () => {
      const error = {
        response: {
          data: {
            message: 'password: must contain at least one number',
          },
        },
      };
      expect(mapAuthError(error)).toBe('Password must contain at least one number (0-9)');
    });

    it('maps "password must contain special character" error', () => {
      const error = {
        response: {
          data: {
            message: 'password: must contain at least one special character',
          },
        },
      };
      expect(mapAuthError(error)).toBe('Password must contain at least one special character (!@#$%^&*)');
    });

    it('maps "password is too common" error', () => {
      const error = {
        response: {
          data: {
            message: 'password: password is too common and easy to guess',
          },
        },
      };
      expect(mapAuthError(error)).toBe('This password is too common. Please choose a more unique password.');
    });

    it('maps "passwords do not match" error', () => {
      const error = {
        response: {
          data: {
            message: 'confirmPassword: passwords do not match',
          },
        },
      };
      expect(mapAuthError(error)).toBe('Passwords do not match. Please make sure both passwords are identical.');
    });
  });

  describe('Username validation errors', () => {
    it('maps "username already taken" error', () => {
      const error = {
        response: {
          data: {
            message: 'username: already taken',
          },
        },
      };
      expect(mapAuthError(error)).toBe('This username is already taken. Please choose a different username.');
    });

    it('maps "username invalid format" error', () => {
      const error = {
        response: {
          data: {
            message: 'username: invalid format',
          },
        },
      };
      expect(mapAuthError(error)).toBe('Username must be 3-20 characters and contain only letters, numbers, and underscores');
    });

    it('maps "username too short" error', () => {
      const error = {
        response: {
          data: {
            message: 'username: too short, must be at least 3 characters',
          },
        },
      };
      expect(mapAuthError(error)).toBe('Username must be at least 3 characters long');
    });
  });

  describe('Email validation errors', () => {
    it('maps "email already exists" error', () => {
      const error = {
        response: {
          data: {
            message: 'email: already exists',
          },
        },
      };
      expect(mapAuthError(error)).toBe('This email address is already registered. Please use a different email or try logging in.');
    });

    it('maps "email invalid format" error', () => {
      const error = {
        response: {
          data: {
            message: 'email: invalid format',
          },
        },
      };
      expect(mapAuthError(error)).toBe('Please enter a valid email address');
    });

    it('maps "disposable email" error', () => {
      const error = {
        response: {
          data: {
            message: 'email: disposable email addresses are not allowed',
          },
        },
      };
      expect(mapAuthError(error)).toBe('Disposable email addresses are not allowed. Please use a permanent email address.');
    });
  });

  describe('Authentication errors', () => {
    it('maps "invalid credentials" error', () => {
      const error = {
        response: {
          data: {
            message: 'invalid credentials',
          },
        },
      };
      expect(mapAuthError(error)).toBe('Invalid username or password. Please check your credentials and try again.');
    });

    it('maps "unauthorized" error', () => {
      const error = {
        response: {
          data: {
            message: 'unauthorized',
          },
        },
      };
      expect(mapAuthError(error)).toBe('Invalid username or password. Please check your credentials and try again.');
    });

    it('maps "account not found" error', () => {
      const error = {
        response: {
          data: {
            message: 'account not found',
          },
        },
      };
      expect(mapAuthError(error)).toBe('No account found with these credentials. Please check your username or sign up.');
    });
  });

  describe('CAPTCHA errors', () => {
    it('maps "captcha required" error', () => {
      const error = {
        response: {
          data: {
            message: 'hcaptcha token is required',
          },
        },
      };
      expect(mapAuthError(error)).toBe('Please complete the CAPTCHA verification');
    });

    it('maps "captcha failed" error', () => {
      const error = {
        response: {
          data: {
            message: 'captcha verification failed',
          },
        },
      };
      expect(mapAuthError(error)).toBe('CAPTCHA verification failed. Please try again.');
    });
  });

  describe('Rate limiting errors', () => {
    it('maps "rate limit exceeded" error', () => {
      const error = {
        response: {
          data: {
            message: 'rate limit exceeded',
          },
        },
      };
      expect(mapAuthError(error)).toBe('Too many attempts. Please wait a few minutes before trying again.');
    });

    it('maps "too many requests" error', () => {
      const error = {
        response: {
          data: {
            message: 'too many registration attempts',
          },
        },
      };
      expect(mapAuthError(error)).toBe('Too many attempts. Please wait a few minutes before trying again.');
    });
  });

  describe('HTTP status code errors', () => {
    it('maps 400 Bad Request', () => {
      const error = {
        response: {
          status: 400,
          data: {},
        },
      };
      expect(mapAuthError(error)).toBe('Invalid request. Please check your information and try again.');
    });

    it('maps 422 Unprocessable Entity the same as 400', () => {
      const error = {
        response: {
          status: 422,
          data: {},
        },
      };
      expect(mapAuthError(error)).toBe('Invalid request. Please check your information and try again.');
    });

    // Regression: the backend returns 422 (not 400) for schema violations since
    // huma's 400/422 split was adopted. Without a 422 branch these fell through
    // to the raw-message fallback and showed the user huma's internals, e.g.
    // "Expected length >= 8 (body.password: x)".
    it.each([
      ['a missing required property', 'expected required property username to be present (body: map[email:a@b.com])'],
      ['a minLength violation', 'expected length >= 8 (body.password: x)'],
    ])('does not leak huma schema text for %s', (_label, serverMessage) => {
      const error = {
        response: {
          status: 422,
          data: { error: serverMessage },
        },
      };
      expect(mapAuthError(error)).toBe('Invalid request. Please check your information and try again.');
    });

    it('maps 401 Unauthorized', () => {
      const error = {
        response: {
          status: 401,
          data: {},
        },
      };
      expect(mapAuthError(error)).toBe('Authentication failed. Please check your credentials.');
    });

    it('maps 403 Forbidden', () => {
      const error = {
        response: {
          status: 403,
          data: {},
        },
      };
      expect(mapAuthError(error)).toBe('Access denied. You do not have permission to perform this action.');
    });

    it('maps 404 Not Found', () => {
      const error = {
        response: {
          status: 404,
          data: {},
        },
      };
      expect(mapAuthError(error)).toBe('Resource not found. The requested item does not exist.');
    });

    it('maps 409 Conflict', () => {
      const error = {
        response: {
          status: 409,
          data: {},
        },
      };
      expect(mapAuthError(error)).toBe('Conflict. The information you provided conflicts with existing data.');
    });

    it('maps 429 Too Many Requests', () => {
      const error = {
        response: {
          status: 429,
          data: {},
        },
      };
      expect(mapAuthError(error)).toBe('Too many requests. Please wait a moment before trying again.');
    });

    it('maps 500 Internal Server Error', () => {
      const error = {
        response: {
          status: 500,
          data: {},
        },
      };
      expect(mapAuthError(error)).toBe('Server error. Please try again later or contact support if the problem persists.');
    });
  });

  describe('Fallback handling', () => {
    it('provides generic message for unknown errors', () => {
      const error = {
        message: 'Unknown error',
      };
      expect(mapAuthError(error)).toBe('An error occurred. Please try again or contact support if the problem persists.');
    });

    it('cleans up error messages by removing field prefixes', () => {
      const error = {
        response: {
          data: {
            message: 'password: some custom validation message',
          },
        },
      };
      // Should remove "password:" prefix and capitalize
      expect(mapAuthError(error)).toBe('Some custom validation message');
    });
  });
});

describe('validatePasswordRequirements', () => {
  it('validates all requirements correctly for valid password', () => {
    const requirements = validatePasswordRequirements('Test1234!');

    expect(requirements).toHaveLength(5);
    expect(requirements.every(req => req.met)).toBe(true);
  });

  it('identifies missing uppercase letter', () => {
    const requirements = validatePasswordRequirements('test1234!');
    const uppercaseReq = requirements.find(req => req.key === 'uppercase');

    expect(uppercaseReq?.met).toBe(false);
  });

  it('identifies missing lowercase letter', () => {
    const requirements = validatePasswordRequirements('TEST1234!');
    const lowercaseReq = requirements.find(req => req.key === 'lowercase');

    expect(lowercaseReq?.met).toBe(false);
  });

  it('identifies missing number', () => {
    const requirements = validatePasswordRequirements('TestPass!');
    const numberReq = requirements.find(req => req.key === 'number');

    expect(numberReq?.met).toBe(false);
  });

  it('identifies missing special character', () => {
    const requirements = validatePasswordRequirements('Test1234');
    const specialReq = requirements.find(req => req.key === 'special');

    expect(specialReq?.met).toBe(false);
  });

  it('identifies insufficient length', () => {
    const requirements = validatePasswordRequirements('Test1!');
    const lengthReq = requirements.find(req => req.key === 'length');

    expect(lengthReq?.met).toBe(false);
  });

  it('validates empty password as all requirements not met', () => {
    const requirements = validatePasswordRequirements('');

    expect(requirements.every(req => !req.met)).toBe(true);
  });
});

describe('isPasswordValid', () => {
  it('returns true for valid password', () => {
    expect(isPasswordValid('Test1234!')).toBe(true);
  });

  it('returns false for password missing requirements', () => {
    expect(isPasswordValid('test')).toBe(false);
    expect(isPasswordValid('test1234')).toBe(false); // No uppercase or special
    expect(isPasswordValid('TEST1234')).toBe(false); // No lowercase or special
    expect(isPasswordValid('TestTest')).toBe(false); // No number or special
  });

  it('returns false for empty password', () => {
    expect(isPasswordValid('')).toBe(false);
  });
});
