import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react';

/**
 * Per-avatar width fallback, used only before the real avatars have been
 * measured. A size="sm" avatar is 32px wide and the stack overlaps by 8px
 * (-space-x-2), so each one past the first costs 24px.
 */
const AVATAR_STEP_FALLBACK_PX = 24;
const AVATAR_FIRST_FALLBACK_PX = 32;

/** Width of the "+N" bubble, which matches one avatar. */
const OVERFLOW_BUBBLE_PX = 32;

/** Gap between the avatar stack and the name list (gap-3 + mr-1). */
const AVATAR_NAME_GAP_PX = 16;

/**
 * Slack held back from the row width. Canvas measureText and the browser's own
 * text layout disagree by a fraction of a pixel per glyph, which is enough to
 * truncate a name the model believed fit exactly.
 */
const FIT_SLACK_PX = 8;

/**
 * Fallback per-character width, used only if canvas measurement is unavailable
 * (older jsdom, a locked-down environment). Real text is measured instead
 * wherever possible: a fixed estimate is wrong in both directions — too high
 * and short names collapse that would have fit, too low and the list truncates
 * mid-word.
 */
const NAME_CHAR_FALLBACK_PX = 7;

/**
 * Measures name text against the row's real font. The canvas is created once and
 * reused; measureText is cheap, but acquiring a context is not.
 */
// Resolved once: jsdom throws from getContext rather than returning null, and
// the cost model asks for several measurements per render.
let measureCtx: CanvasRenderingContext2D | null | undefined;
function getMeasureContext(): CanvasRenderingContext2D | null {
  if (measureCtx !== undefined) return measureCtx;
  try {
    measureCtx = document.createElement('canvas').getContext('2d');
  } catch {
    measureCtx = null;
  }
  return measureCtx;
}

function measureTextWidth(text: string, font: string): number | null {
  if (typeof document === 'undefined') return null;
  const ctx = getMeasureContext();
  if (!ctx) return null;
  ctx.font = font;
  const width = ctx.measureText(text).width;
  // jsdom's stub returns 0 for every string; treat that as unavailable rather
  // than concluding every name fits.
  return width > 0 ? width : null;
}

interface UseParticipantFitOptions {
  /** Full participant name list, in display order. */
  names: string[];
}

interface UseParticipantFitResult {
  /** Attach to the row that holds the avatars and names. */
  containerRef: React.RefObject<HTMLDivElement | null>;
  /** How many avatars fit. */
  visibleAvatars: number;
  /** How many names to spell out; the rest collapse into a "+N" suffix. */
  visibleNames: number;
  /**
   * False until the first measurement lands. Callers should render the full
   * list during this pass so there are widths to measure.
   */
  measured: boolean;
}

/**
 * useParticipantFit — fit participant avatars and names into the available width.
 *
 * Priority is avatars first, names second: an avatar identifies someone at a
 * glance in the space a couple of characters would occupy, so avatars are only
 * dropped once every name has already collapsed. Both degrade to a "+N" count
 * rather than truncating, because a name cut mid-word ("Detective Marcus Kan…")
 * reads as broken while a count reads as deliberate.
 *
 * Widths come from the real rendered elements rather than breakpoints, so the
 * same card adapts to a narrow viewport, a sidebar opening, or an unusually long
 * character name — none of which a fixed `sm:` cutoff can distinguish.
 */
export function useParticipantFit({
  names,
}: UseParticipantFitOptions): UseParticipantFitResult {
  const containerRef = useRef<HTMLDivElement | null>(null);

  // Callers pass a fresh array every render (`participant_names ?? []`), so the
  // identity is not a usable dependency — recompute would re-run on every
  // render. The joined text is what the measurement actually depends on.
  const namesKey = names.join('\u0000');
  const namesRef = useRef(names);
  namesRef.current = names;

  const total = names.length;
  const [visibleAvatars, setVisibleAvatars] = useState(total);
  const [visibleNames, setVisibleNames] = useState(total);
  const [measured, setMeasured] = useState(false);

  const recompute = useCallback(() => {
    const container = containerRef.current;
    if (!container) return;

    const available = container.getBoundingClientRect().width;
    // A collapsed or hidden container measures 0 (an ancestor is display:none,
    // or layout has not settled). Measuring then would collapse everything;
    // treat it as "no information yet" and keep the previous answer.
    if (available === 0) return;

    if (total === 0) {
      setVisibleAvatars(0);
      setVisibleNames(0);
      setMeasured(true);
      return;
    }

    const names = namesRef.current;

    // Participants own the whole row — the timestamp that used to sit here moved
    // to the footer. A small slack keeps the last name from landing flush
    // against the edge, where sub-pixel rounding between the measured text and
    // the rendered text still triggers the ellipsis.
    const budget = available - FIT_SLACK_PX;

    // Match the font of the element the names actually render into.
    const nameEl = container.querySelector('[data-participant-names]');
    const font = nameEl
      ? (() => {
          const cs = window.getComputedStyle(nameEl);
          return `${cs.fontWeight} ${cs.fontSize} ${cs.fontFamily}`;
        })()
      : null;

    // Cost of showing `n` avatars, including the "+N" bubble when some are hidden.
    const avatarCost = (n: number) => {
      if (n === 0) return 0;
      const stack = AVATAR_FIRST_FALLBACK_PX + (n - 1) * AVATAR_STEP_FALLBACK_PX;
      return stack + (n < total ? OVERFLOW_BUBBLE_PX : 0);
    };

    // Cost of spelling out the first `n` names, plus a "+N" suffix when some
    // are collapsed.
    const textCost = (text: string) => {
      const measured = font ? measureTextWidth(text, font) : null;
      return measured ?? text.length * NAME_CHAR_FALLBACK_PX;
    };

    const nameCost = (n: number) => {
      if (n === 0) return 0;
      const suffix = n < total ? ` +${total - n}` : '';
      return textCost(names.slice(0, n).join(', ') + suffix);
    };

    // The "N participants" fallback is only worth using when it is actually
    // narrower than naming one person, which for a short name it is not.
    const countCost = () => textCost(`${total} people`);

    // Avatars come first: they are charged against the budget on their own, with
    // no room set aside for names, so a long name can never push a face out.
    // Names then spend whatever is left.
    //
    // The "+N" bubble is wider than the avatar it overlaps, so hiding faces does
    // not shrink the stack monotonically — showing all 6 can cost less than
    // showing 5. Scanning for the widest count that fits (rather than stepping
    // down one at a time) avoids both stalling at `total` and settling for fewer
    // faces than the row can actually hold.
    let avatars = 1;
    for (let n = total; n >= 1; n--) {
      if (avatarCost(n) <= budget) {
        avatars = n;
        break;
      }
    }

    const nameBudget = budget - avatarCost(avatars) - AVATAR_NAME_GAP_PX;
    let namesShown = total;
    while (namesShown > 1 && nameCost(namesShown) > nameBudget) {
      namesShown -= 1;
    }
    // A single name that still does not fit is better shown as a bare count —
    // but only if the count is genuinely narrower, otherwise naming the person
    // is strictly more useful.
    if (namesShown === 1 && nameCost(1) > nameBudget && countCost() < nameCost(1)) {
      namesShown = 0;
    }

    setVisibleAvatars(prev => (prev === avatars ? prev : avatars));
    setVisibleNames(prev => (prev === namesShown ? prev : namesShown));
    setMeasured(true);
    // namesKey stands in for namesRef.current, which the linter cannot see
    // through; it is the value the measurement actually depends on.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [namesKey, total]);

  // Measure synchronously after DOM mutations so the corrected layout paints in
  // the same frame, rather than flashing the full list and then collapsing.
  useLayoutEffect(() => {
    recompute();
  }, [recompute]);

  // Re-measure on container resize. ResizeObserver catches viewport changes and
  // layout-driven width changes alike; a window resize listener would miss the
  // latter.
  useEffect(() => {
    const container = containerRef.current;
    if (!container || typeof ResizeObserver === 'undefined') return;

    const observer = new ResizeObserver(() => recompute());
    observer.observe(container);
    return () => observer.disconnect();
  }, [recompute]);

  return { containerRef, visibleAvatars, visibleNames, measured };
}
