import { describe, it, expect } from 'vitest';
import { wasSubmissionEdited } from '../submissionEdits';

describe('wasSubmissionEdited', () => {
  it('returns false when the submission has never been edited', () => {
    // A fresh insert stamps both columns from the same NOW().
    const ts = '2026-01-10T12:00:00Z';
    expect(wasSubmissionEdited(ts, ts)).toBe(false);
  });

  it('returns true when updated_at is later than submitted_at', () => {
    expect(
      wasSubmissionEdited('2026-01-10T12:00:00Z', '2026-01-10T13:30:00Z')
    ).toBe(true);
  });

  it('treats a sub-second difference as a real edit', () => {
    // No tolerance window: the backend's first-submission check is strict
    // equality, so a resubmit landing 400ms after the first must agree with it.
    // An unedited row cannot drift, since both columns share one NOW().
    expect(
      wasSubmissionEdited('2026-01-10T12:00:00.000Z', '2026-01-10T12:00:00.400Z')
    ).toBe(true);
  });

  it('treats a one-second difference as a real edit', () => {
    expect(
      wasSubmissionEdited('2026-01-10T12:00:00Z', '2026-01-10T12:00:01Z')
    ).toBe(true);
  });

  it('returns false when either timestamp is missing or unparseable', () => {
    expect(wasSubmissionEdited(null, '2026-01-10T12:00:00Z')).toBe(false);
    expect(wasSubmissionEdited('2026-01-10T12:00:00Z', undefined)).toBe(false);
    expect(wasSubmissionEdited('not-a-date', '2026-01-10T12:00:00Z')).toBe(false);
  });
});
