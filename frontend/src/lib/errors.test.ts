import { describe, it, expect } from 'vitest';
import { AxiosError } from 'axios';
import { createAppError, extractApiErrorMessage } from './errors';
import { ERROR_MESSAGES } from '../types/errors';

/**
 * These cover the transition described in .claude/planning/rfc7807-error-format.md:
 * the API emits the legacy `{status, error}` body today and RFC 7807
 * `{type, title, status, detail}` after the backend migrates. Both shapes must
 * produce a sensible user-facing message throughout.
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
  it('reads the legacy error field', () => {
    expect(
      extractApiErrorMessage(axios422({ status: 'Invalid request.', error: 'title is required' }))
    ).toBe('title is required');
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

  it('does not return the legacy status string either', () => {
    expect(extractApiErrorMessage(axios422({ status: 'Invalid request.' }))).toBeUndefined();
  });

  it('ignores blank and whitespace-only messages', () => {
    expect(extractApiErrorMessage(axios422({ error: '   ' }))).toBeUndefined();
    expect(extractApiErrorMessage(axios422({ detail: '' }))).toBeUndefined();
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

  it('surfaces the legacy error field unchanged', () => {
    const appError = createAppError(
      axiosWithStatus(403, { status: 'Forbidden.', error: 'admin privileges required' })
    );
    expect(appError.context?.userMessage).toBe('admin privileges required');
  });
});
