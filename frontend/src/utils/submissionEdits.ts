/**
 * Edit detection for action submissions.
 *
 * Unlike messages, action_submissions has no is_edited column. The backend
 * stamps submitted_at once on insert and preserves it across edits, while
 * updated_at moves to NOW() on every write (see SubmitAction in
 * backend/pkg/db/queries/phases.sql). So the two columns being equal means
 * "never edited since first submitted".
 *
 * This mirrors the isFirstSubmission check the submit handler uses to decide
 * whether to notify the GM (backend/pkg/phases/api_actions.go), deliberately:
 * if the badge and that notification ever disagreed, a GM would be told an
 * action was new while the UI called it edited, or vice versa. Both sides key
 * off exact equality, which holds because a single INSERT ... ON CONFLICT
 * takes both timestamps from the same transaction-stable NOW().
 */
export function wasSubmissionEdited(
  submittedAt: string | null | undefined,
  updatedAt: string | null | undefined
): boolean {
  if (!submittedAt || !updatedAt) return false;

  const submitted = new Date(submittedAt).getTime();
  const updated = new Date(updatedAt).getTime();
  if (isNaN(submitted) || isNaN(updated)) return false;

  // Strict inequality, matching the backend's isFirstSubmission check exactly.
  // No tolerance: an unedited row's two columns come from the same
  // transaction-stable clock and are identical, so any difference at all is a
  // real edit -- including a resubmit that lands within a second of the first.
  return updated !== submitted;
}
