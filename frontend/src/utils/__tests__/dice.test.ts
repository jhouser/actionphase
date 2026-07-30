import { describe, it, expect } from 'vitest';
import {
  parseDiceNotation,
  formatNotation,
  rollDice,
  formatRollMarkdown,
} from '../dice';

describe('parseDiceNotation', () => {
  it('parses a bare die like "d20" as a single die', () => {
    expect(parseDiceNotation('d20')).toEqual({ count: 1, sides: 20, modifier: 0 });
  });

  it('parses a count like "2d6"', () => {
    expect(parseDiceNotation('2d6')).toEqual({ count: 2, sides: 6, modifier: 0 });
  });

  it('parses positive and negative modifiers', () => {
    expect(parseDiceNotation('3d8+2')).toEqual({ count: 3, sides: 8, modifier: 2 });
    expect(parseDiceNotation('1d20-1')).toEqual({ count: 1, sides: 20, modifier: -1 });
  });

  it('is whitespace and case tolerant', () => {
    expect(parseDiceNotation('  2 D 6 + 3 ')).toEqual({ count: 2, sides: 6, modifier: 3 });
  });

  it('rejects invalid notation', () => {
    expect(parseDiceNotation('')).toBeNull();
    expect(parseDiceNotation('d')).toBeNull();
    expect(parseDiceNotation('abc')).toBeNull();
    expect(parseDiceNotation('20')).toBeNull();
    expect(parseDiceNotation('d1')).toBeNull(); // sides must be >= 2
  });

  it('rejects out-of-bounds dice counts and sides', () => {
    expect(parseDiceNotation('101d6')).toBeNull();
    expect(parseDiceNotation('0d6')).toBeNull();
    expect(parseDiceNotation('1d1001')).toBeNull();
  });
});

describe('formatNotation', () => {
  it('formats with and without modifiers', () => {
    expect(formatNotation({ count: 1, sides: 20, modifier: 0 })).toBe('1d20');
    expect(formatNotation({ count: 2, sides: 6, modifier: 3 })).toBe('2d6+3');
    expect(formatNotation({ count: 1, sides: 20, modifier: -1 })).toBe('1d20-1');
  });
});

describe('rollDice', () => {
  it('returns null for invalid notation', () => {
    expect(rollDice('nope')).toBeNull();
  });

  it('rolls each die within [1, sides]', () => {
    // rng of 0 -> 1, rng approaching 1 -> sides
    expect(rollDice('d20', () => 0)?.total).toBe(1);
    expect(rollDice('d20', () => 0.999)?.total).toBe(20);
  });

  it('applies the modifier to the total', () => {
    const result = rollDice('2d6+3', () => 0.5); // each d6 -> floor(0.5*6)+1 = 4
    expect(result?.dice.map((d) => d.value)).toEqual([4, 4]);
    expect(result?.modifier).toBe(3);
    expect(result?.total).toBe(11);
  });

  it('normalizes the returned notation', () => {
    expect(rollDice('d20', () => 0)?.notation).toBe('1d20');
  });
});

describe('formatRollMarkdown', () => {
  it('formats a simple single-die roll without breakdown', () => {
    const result = rollDice('d20', () => 0.65)!; // floor(0.65*20)+1 = 14
    expect(formatRollMarkdown(result)).toBe('🎲 rolled **14** (1d20)');
  });

  it('includes character attribution when provided', () => {
    const result = rollDice('d20', () => 0.65)!;
    expect(formatRollMarkdown(result, 'Kael')).toBe('🎲 @Kael rolled **14** (1d20)');
  });

  it('shows a breakdown for multiple dice or a modifier', () => {
    const result = rollDice('2d6+3', () => 0.5)!; // 4, 4, +3 = 11
    expect(formatRollMarkdown(result)).toBe('🎲 rolled **11** (2d6+3 → 4, 4 +3)');
  });

  it('shows a breakdown with a negative modifier', () => {
    const result = rollDice('1d20-2', () => 0.65)!; // 14 - 2 = 12
    expect(formatRollMarkdown(result)).toBe('🎲 rolled **12** (1d20-2 → 14 -2)');
  });
});
