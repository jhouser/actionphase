/**
 * Pure dice-rolling logic for the common-room Utility Drawer.
 *
 * Rolls are cosmetic/client-side only — there is no server verification.
 * Keeping this logic React-free makes it straightforward to unit test.
 */

interface DieResult {
  /** Number of sides on the die (e.g. 20 for a d20). */
  sides: number;
  /** The value rolled, in [1, sides]. */
  value: number;
}

export interface RollResult {
  /** Normalized notation that was rolled, e.g. "2d6+3". */
  notation: string;
  /** Individual die results, in roll order. */
  dice: DieResult[];
  /** Flat modifier applied after the dice (may be negative or zero). */
  modifier: number;
  /** Sum of all dice plus the modifier. */
  total: number;
}

/** Parsed dice notation, before any rolling happens. */
export interface DiceNotation {
  count: number;
  sides: number;
  modifier: number;
}

// Matches "d20", "2d6", "3d8+2", "1d20-1" (whitespace tolerant, case-insensitive).
const NOTATION_RE = /^\s*(\d*)\s*[dD]\s*(\d+)\s*([+-]\s*\d+)?\s*$/;

const MAX_DICE = 100;
const MAX_SIDES = 1000;

/**
 * Parse standard dice notation into its components.
 * Returns null when the string is not valid notation.
 */
export function parseDiceNotation(input: string): DiceNotation | null {
  const match = NOTATION_RE.exec(input);
  if (!match) return null;

  const count = match[1] === '' ? 1 : parseInt(match[1], 10);
  const sides = parseInt(match[2], 10);
  const modifier = match[3] ? parseInt(match[3].replace(/\s+/g, ''), 10) : 0;

  if (count < 1 || count > MAX_DICE) return null;
  if (sides < 2 || sides > MAX_SIDES) return null;

  return { count, sides, modifier };
}

/** Format parsed notation back into a canonical string, e.g. "2d6+3". */
export function formatNotation({ count, sides, modifier }: DiceNotation): string {
  const base = `${count}d${sides}`;
  if (modifier > 0) return `${base}+${modifier}`;
  if (modifier < 0) return `${base}${modifier}`;
  return base;
}

/**
 * Roll dice described by notation.
 * @param input dice notation, e.g. "d20", "2d6+3"
 * @param rng injectable RNG returning a float in [0, 1) — defaults to Math.random (tests pass a stub)
 * @returns the roll result, or null if the notation is invalid
 */
export function rollDice(
  input: string,
  rng: () => number = Math.random
): RollResult | null {
  const parsed = parseDiceNotation(input);
  if (!parsed) return null;

  const { count, sides, modifier } = parsed;
  const dice: DieResult[] = [];
  for (let i = 0; i < count; i++) {
    const value = Math.floor(rng() * sides) + 1;
    dice.push({ sides, value });
  }

  const diceSum = dice.reduce((sum, d) => sum + d.value, 0);
  return {
    notation: formatNotation(parsed),
    dice,
    modifier,
    total: diceSum + modifier,
  };
}

/**
 * Format a roll result as markdown suitable for pasting into a common-room reply.
 * @param result the roll to format
 * @param characterName optional attribution, e.g. "Kael" -> "@Kael rolled ..."
 *
 * Examples:
 *   "🎲 rolled **14** (d20)"
 *   "🎲 @Kael rolled **11** (2d6+3 → 3, 5 +3)"
 */
export function formatRollMarkdown(result: RollResult, characterName?: string): string {
  const who = characterName ? `@${characterName} ` : '';
  const showBreakdown = result.dice.length > 1 || result.modifier !== 0;

  if (!showBreakdown) {
    return `🎲 ${who}rolled **${result.total}** (${result.notation})`;
  }

  const rolls = result.dice.map((d) => d.value).join(', ');
  const mod =
    result.modifier > 0 ? ` +${result.modifier}` :
    result.modifier < 0 ? ` ${result.modifier}` : '';
  return `🎲 ${who}rolled **${result.total}** (${result.notation} → ${rolls}${mod})`;
}
