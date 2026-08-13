/**
 * Delay presets offered between staged result parts, in minutes.
 *
 * The GM's mockup showed a dropdown, not a free-text box, because the useful
 * range is narrow: long enough to build tension, short enough that the player
 * is still at their desk. "Custom" opens a number input for the rare case.
 */
export const DELAY_PRESETS = [1, 5, 15, 30, 60] as const;

export const CUSTOM_DELAY = 'custom';

/** Default delay for a newly added follow-up part. */
export const DEFAULT_DELAY_MINUTES = 15;

export function formatDelayLabel(minutes: number): string {
  if (minutes === 60) return '1 hour';
  return `${minutes} minute${minutes === 1 ? '' : 's'}`;
}

export function isPresetDelay(minutes: number): boolean {
  return DELAY_PRESETS.includes(minutes as (typeof DELAY_PRESETS)[number]);
}
