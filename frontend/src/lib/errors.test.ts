import { describe, it, expect } from 'vitest';
import { AxiosError } from 'axios';
import { createAppError, extractApiErrorMessage } from './errors';
import { ERROR_MESSAGES } from '../types/errors';

/**
 * The API emits one error shape: RFC 7807 `{type, title, status, detail}`.
 *
 * The failure this guards against is silent: RFC 7807's `status` is a number,
 * so a fallback chain that reaches it renders a bare "422" to the user without
 * throwing. Nothing here can be asserted by status code alone.
 */

function axios422(data: unknown): AxiosError {
  const err = new AxiosError('Request failed with status code 422');
  err.response = { status: 422, data } as AxiosError['response'];
  return err;
}

describe('extractApiErrorMessage', () => {
  it('ignores a stray legacy error field', () => {
    // No endpoint emits `error` any more. If one regresses, `detail` is still
    // absent, so the caller must fall back rather than surface a half-migrated
    // body -- and must never reach for `status`.
    expect(
      extractApiErrorMessage(axios422({ status: 422, error: 'title is required' }))
    ).toBeUndefined();
  });

  it('reads the RFC 7807 detail field', () => {
    expect(
      extractApiErrorMessage(
        axios422({ type: 'about:blank', title: 'Unprocessable Entity', status: 422, detail: 'title is required' })
      )
    ).toBe('title is required');
  });

  it('joins field-level messages from the RFC 7807 errors array', () => {
    expect(
      extractApiErrorMessage(
        axios422({
          title: 'Unprocessable Entity',
          status: 422,
          detail: 'validation failed',
          errors: [
            { message: 'expected length >= 8', location: 'body.password' },
            { message: 'expected a valid email', location: 'body.email' },
          ],
        })
      )
      // `detail` wins when present -- it is specific to the occurrence. The
      // array is the fallback for bodies that carry only field errors.
    ).toBe('validation failed');
  });

  it('falls back to the errors array when detail is absent', () => {
    expect(
      extractApiErrorMessage(
        axios422({
          title: 'Unprocessable Entity',
          status: 422,
          errors: [
            { message: 'expected length >= 8', location: 'body.password' },
            { message: 'expected a valid email', location: 'body.email' },
          ],
        })
      )
    ).toBe('expected length >= 8. expected a valid email');
  });

  it('never returns the numeric status as a message', () => {
    // The core trap: a 7807 body with no usable prose must yield undefined,
    // not the number 422, so callers apply their own fallback.
    expect(extractApiErrorMessage(axios422({ status: 422 }))).toBeUndefined();
  });

  it('ignores blank and whitespace-only messages', () => {
    expect(extractApiErrorMessage(axios422({ detail: '   ' }))).toBeUndefined();
    expect(extractApiErrorMessage(axios422({ detail: '' }))).toBeUndefined();
    expect(extractApiErrorMessage(axios422({ title: '  ' }))).toBeUndefined();
  });

  it('returns undefined for non-API errors', () => {
    expect(extractApiErrorMessage(new Error('boom'))).toBeUndefined();
    expect(extractApiErrorMessage(null)).toBeUndefined();
    expect(extractApiErrorMessage(undefined)).toBeUndefined();
    expect(extractApiErrorMessage('a string')).toBeUndefined();
  });

  it('accepts a bare response body as well as an axios error', () => {
    expect(extractApiErrorMessage({ detail: 'already taken' })).toBe('already taken');
  });
});

describe('createAppError fallbacks', () => {
  function axiosWithStatus(status: number, data: unknown): AxiosError {
    const err = new AxiosError(`Request failed with status code ${status}`);
    err.response = { status, data } as AxiosError['response'];
    return err;
  }

  it('shows the session-expired message for a 401 carrying no detail', () => {
    // Guards the regression where a truthy numeric `status` suppressed this
    // fallback, leaving the user with a bare "401".
    const appError = createAppError(axiosWithStatus(401, { status: 401, title: '' }));
    expect(appError.context?.userMessage).toBe(ERROR_MESSAGES.SESSION_EXPIRED);
  });

  it('shows the unauthorized message for a 403 carrying no detail', () => {
    const appError = createAppError(axiosWithStatus(403, { status: 403, title: '' }));
    expect(appError.context?.userMessage).toBe(ERROR_MESSAGES.UNAUTHORIZED);
  });

  it('prefers the server detail over the generic fallback on a 403', () => {
    const appError = createAppError(
      axiosWithStatus(403, { status: 403, detail: 'you are banned from this community' })
    );
    expect(appError.context?.userMessage).toBe('you are banned from this community');
  });

  it('falls back to the default 403 message when the body carries no detail', () => {
    const appError = createAppError(axiosWithStatus(403, { status: 403 }));
    expect(appError.context?.userMessage).toBe(ERROR_MESSAGES.UNAUTHORIZED);
  });
});

/**
 * Payloads captured verbatim from the running API after the backend adopted
 * RFC 7807, one per emitter. The API has five independent error emitters -- the
 * jwt middleware, chi/render for other middleware, huma for handlers, the
 * tollbooth rate limiter, and the panic recovery middleware --
 * and they can drift apart without any of them failing on its own. Pinning a
 * real body from each is what catches that.
 */
describe('extractApiErrorMessage against live API payloads', () => {
  it('reads a 401 from the jwt middleware', () => {
    expect(
      extractApiErrorMessage({
        response: {
          data: {
            title: 'Unauthorized',
            status: 401,
            detail: 'no token found',
            instance: 'urn:actionphase:correlation:corr_7bcd73cd936d3896',
          },
        },
      })
    ).toBe('no token found');
  });

  it('reads a 403 from the admin middleware (chi/render)', () => {
    expect(
      extractApiErrorMessage({
        response: {
          data: { title: 'Forbidden', status: 403, detail: 'admin privileges required' },
        },
      })
    ).toBe('admin privileges required');
  });

  it('reads a 429 from the rate limiter', () => {
    // tollbooth takes a fixed string, so this body alone carries no
    // `instance` -- the one emitter that cannot.
    expect(
      extractApiErrorMessage({
        response: {
          data: {
            title: 'Too Many Requests',
            status: 429,
            detail: 'Rate limit exceeded. Please try again later.',
          },
        },
      })
    ).toBe('Rate limit exceeded. Please try again later.');
  });

  it('reads a 500 from the panic recovery middleware', () => {
    // Shape asserted server-side by
    // TestErrorRecoveryMiddleware_EmitsProblemJSON; there is deliberately no
    // route that panics on demand to capture this from a live server.
    expect(
      extractApiErrorMessage({
        response: {
          data: {
            title: 'Internal Server Error',
            status: 500,
            detail: 'Internal server error',
            instance: 'urn:actionphase:correlation:corr_1f2a9c4b7e0d5a63',
          },
        },
      })
    ).toBe('Internal server error');
  });

  it('reads a 404 from a huma handler', () => {
    expect(
      extractApiErrorMessage({
        response: {
          data: { title: 'Not Found', status: 404, detail: 'game with ID 999999 not found' },
        },
      })
    ).toBe('game with ID 999999 not found');
  });

  it('reads a 422 carrying field-level errors', () => {
    expect(
      extractApiErrorMessage({
        response: {
          data: {
            title: 'Unprocessable Entity',
            status: 422,
            detail: 'validation failed',
            errors: [
              { message: 'expected length >= 3', location: 'body.title', value: '' },
              { message: 'title must not be blank', location: 'body.title' },
            ],
          },
        },
      })
    ).toBe('validation failed');
  });

  it('never renders the numeric status as the message', () => {
    // The trap this whole migration exists to defuse: `status` is a number in
    // RFC 7807, so a fallback chain reaching it shows the user a bare "422".
    for (const status of [401, 403, 404, 422, 500]) {
      expect(extractApiErrorMessage({ response: { data: { status } } })).toBeUndefined();
    }
  });
});
