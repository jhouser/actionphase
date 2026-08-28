import { InformationCircleIcon } from '@heroicons/react/24/outline';

interface HelpTooltipProps {
  text: string;
  /**
   * Which edge the panel is anchored to. Defaults to 'left', which suits an icon
   * near the left of its container. Use 'right' when the icon sits near the right
   * edge — a left-anchored panel would overflow the container there.
   */
  align?: 'left' | 'right';
}

/**
 * HelpTooltip - A small info icon that reveals help text on hover.
 *
 * Replaces parenthetical clarifications in labels, allowing longer and more
 * detailed help text without cluttering the label line.
 *
 * @example
 * ```tsx
 * <label className="flex items-center gap-1">
 *   Anonymous Mode
 *   <HelpTooltip text="Hides character ownership and NPC status from players." />
 * </label>
 * ```
 *
 * @example Anchored right, for an icon near the right edge of its container:
 * ```tsx
 * <HelpTooltip text="..." align="right" />
 * ```
 */
export function HelpTooltip({ text, align = 'left' }: HelpTooltipProps) {
  // Full class strings, not interpolated fragments — Tailwind only emits classes
  // it can find literally in the source.
  const alignClasses = align === 'right' ? 'right-0' : 'left-0';
  const arrowClasses = align === 'right' ? 'right-3' : 'left-3';
  return (
    <span className="group relative inline-flex items-center">
      <InformationCircleIcon
        className="w-4 h-4 text-content-tertiary hover:text-content-primary cursor-help transition-colors"
        aria-label={text}
        role="img"
      />

      {/* Tooltip panel — anchored to the side given by `align` so it stays within
          its container's bounds */}
      <span
        role="tooltip"
        className={`
          invisible group-hover:visible
          absolute ${alignClasses} bottom-full mb-2
          w-64 p-3 rounded-lg
          bg-surface-raised border border-theme-default shadow-lg
          text-xs text-content-primary font-normal
          z-50 pointer-events-none
          whitespace-normal text-left
        `}
      >
        {text}
        {/* Arrow pointing down to icon — follows the anchored edge */}
        <span
          className={`
            absolute top-full ${arrowClasses} -mt-1
            border-8 border-transparent border-t-surface-raised
          `}
        />
      </span>
    </span>
  );
}
