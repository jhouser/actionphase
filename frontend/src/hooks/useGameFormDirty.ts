import { useMemo } from 'react';
import type { GameFormData } from '../components/GameFormFields';

/**
 * Fields compared to decide whether the game form has unsaved edits.
 *
 * Derived from the form data's own keys rather than listed by hand, so a field
 * added to GameFormData is covered automatically. A hand-written list would go
 * stale silently — the guard would keep passing while quietly ignoring the new
 * field.
 */
function normalize(value: GameFormData[keyof GameFormData]): string | number | boolean {
  // Trimmed, matching what buildApiPayload sends. An untrimmed comparison would
  // report dirty for a change Save discards, soft-locking the guard on an edit
  // that cannot be committed away.
  if (typeof value === 'string') return value.trim();
  return value ?? '';
}

/**
 * True when the form differs from the state it was opened in, or when a banner
 * file has been chosen but not yet uploaded.
 *
 * The form's fields all live in `formData`, so this is a plain comparison
 * against the initial snapshot — no per-child dirty reporting is needed here,
 * unlike the character sheet where editors hold state the parent cannot see.
 */
export function useGameFormDirty(
  formData: GameFormData,
  initialData: GameFormData,
  pendingBannerFile: File | null
): boolean {
  return useMemo(() => {
    if (pendingBannerFile !== null) return true;

    const keys = new Set([
      ...Object.keys(formData),
      ...Object.keys(initialData),
    ]) as Set<keyof GameFormData>;

    for (const key of keys) {
      if (normalize(formData[key]) !== normalize(initialData[key])) return true;
    }
    return false;
  }, [formData, initialData, pendingBannerFile]);
}
