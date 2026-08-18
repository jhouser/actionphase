import { useMemo } from 'react';
import type { CharacterSheetConfig, SheetLabels } from '../types/characters';

/**
 * Default label for each renameable character sheet tab.
 *
 * **This is the only place in the codebase that knows these strings.** The
 * backend deliberately stores only genuine GM overrides and never fills
 * defaults in, so an absent config means "use these" — which only works if
 * exactly one place has them. Adding a second copy (a placeholder typed inline,
 * a fallback in a component) reintroduces the drift this centralisation exists
 * to prevent.
 *
 * Per the refactor plan's invariant, each key is identical to its own default
 * label and to its storage `module_type`. That is what keeps this a flat lookup
 * rather than a translation table.
 */
export type { SheetLabels };

export const DEFAULT_SHEET_LABELS = {
  skills: 'Skills',
  inventory: 'Inventory',
  numbers: 'Numbers',
} as const;

/** Source of labels: a game, a cross-game character payload, or nothing. */
interface SheetLabelSource {
  character_sheet?: CharacterSheetConfig;
}

/**
 * Resolves a game's character sheet tab labels, applying defaults for anything
 * the GM has not overridden.
 *
 * Accepts undefined so callers can pass a game that has not loaded yet, or no
 * game at all (the utility drawer renders sheets outside a GameProvider) —
 * both cases correctly yield all defaults.
 *
 * A stored label is only honoured if it has non-whitespace content. The backend
 * strips whitespace-only labels on the way in rather than storing them, so this
 * should not fire for anything written through the API; it is a floor for
 * hand-edited rows and for anything that predates that validation. Falling back
 * to the default there is far better than rendering a tab with a blank name
 * nobody can point at.
 */
export function useSheetLabels(source?: SheetLabelSource | null): SheetLabels {
  const config = source?.character_sheet;

  return useMemo(() => resolveSheetLabels(config), [config]);
}

/**
 * Non-hook form, for the places that need labels outside a render — and for
 * tests that want to check resolution without mounting a component.
 */
export function resolveSheetLabels(config?: CharacterSheetConfig | null): SheetLabels {
  const labels = config?.labels;

  return {
    skills: pick(labels?.skills, DEFAULT_SHEET_LABELS.skills),
    inventory: pick(labels?.inventory, DEFAULT_SHEET_LABELS.inventory),
    numbers: pick(labels?.numbers, DEFAULT_SHEET_LABELS.numbers),
  };
}

function pick(override: string | undefined, fallback: string): string {
  const trimmed = override?.trim();
  return trimmed ? trimmed : fallback;
}
