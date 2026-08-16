/**
 * Delay presets offered between staged result parts, in minutes.
 *
 * The GM's mockup showed a dropdown, not a free-text box, because the useful
 * range is narrow: long enough to build tension, short enough that the player
 * is still at their desk.
 *
 * Deliberately only three options. 1 minute is shorter than the release
 * worker's own tick, so it reads as "instant" and the timing is a lie; an hour
 * or more risks outliving the phase the result belongs to, stranding a chain
 * across a boundary. The server still accepts 1..1440 (MinStagedDelayMinutes /
 * MaxStagedDelayMinutes) — this list is the editorial choice about what is
 * worth offering, not the safety bound.
 */
export const DELAY_PRESETS = [5, 15, 30] as const;

/** Default delay for a newly added follow-up part. */
export const DEFAULT_DELAY_MINUTES = 15;

/**
 * Most parts a single chain may contain, head included.
 *
 * Mirrors core.MaxStagedChainLength on the server, which rejects anything
 * longer. Duplicated here so the GM is stopped at the point of authoring rather
 * than after submitting: the create form holds unsaved text, so a server-side
 * rejection would surface as a generic failure and lose every part written.
 */
export const MAX_CHAIN_PARTS = 10;

export function formatDelayLabel(minutes: number): string {
  if (minutes === 60) return '1 hour';
  return `${minutes} minute${minutes === 1 ? '' : 's'}`;
}

export function isPresetDelay(minutes: number): boolean {
  return DELAY_PRESETS.includes(minutes as (typeof DELAY_PRESETS)[number]);
}
