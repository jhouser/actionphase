import type { ButtonHTMLAttributes, ReactNode } from 'react';
import { forwardRef } from 'react';
import { cn } from '../../lib/theme/utils';

export type ToggleSize = 'sm' | 'md';

export interface ToggleProps
  extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'onChange' | 'type'> {
  /** Whether the toggle is on. */
  checked: boolean;
  /** Called with the next checked value when the user activates the toggle. */
  onChange: (checked: boolean) => void;
  /** Track/knob dimensions. `md` (default) matches most forms; `sm` for compact rows. */
  size?: ToggleSize;
  /** Optional primary label rendered to the left of the switch. */
  label?: ReactNode;
  /** Optional secondary description rendered under the label. */
  description?: ReactNode;
  /** Optional leading icon (rendered before the label block). */
  icon?: ReactNode;
}

/**
 * Toggle - Accessible on/off switch using semantic theme tokens.
 *
 * Fixes the recurring "invisible off-state in light mode" bug by rendering the
 * off track with a defined surface token plus a visible border, instead of the
 * legacy `surface-raised` / `bg-border-primary` utilities which are unassigned
 * in the current theme system.
 *
 * When `label`/`description`/`icon` are provided, the component renders a full
 * clickable row. Otherwise it renders just the switch (use your own layout).
 *
 * @example
 * ```tsx
 * <Toggle
 *   checked={enabled}
 *   onChange={setEnabled}
 *   label="Private Messages"
 *   description="Get a Discord DM for new private messages."
 * />
 *
 * // Bare switch
 * <Toggle checked={on} onChange={setOn} aria-label="Toggle setting" />
 * ```
 */
export const Toggle = forwardRef<HTMLButtonElement, ToggleProps>(
  ({ checked, onChange, size = 'md', label, description, icon, className, disabled, ...props }, ref) => {
    const dims =
      size === 'sm'
        ? { track: 'h-5 w-9', knob: 'h-3.5 w-3.5', on: 'translate-x-4', off: 'translate-x-0.5' }
        : { track: 'h-6 w-11', knob: 'h-4 w-4', on: 'translate-x-6', off: 'translate-x-1' };

    const switchEl = (
      <span
        className={cn(
          'relative inline-flex items-center rounded-full border transition-colors shrink-0',
          dims.track,
          checked
            ? 'bg-interactive-primary border-interactive-primary'
            : 'bg-surface-sunken border-theme-strong'
        )}
      >
        <span
          className={cn(
            'inline-block transform rounded-full bg-white shadow transition-transform',
            dims.knob,
            checked ? dims.on : dims.off
          )}
        />
      </span>
    );

    const hasLabel = label !== undefined && label !== null;
    const hasDescription = description !== undefined && description !== null;
    const hasIcon = icon !== undefined && icon !== null;
    const hasText = hasLabel || hasDescription || hasIcon;

    return (
      <button
        ref={ref}
        type="button"
        role="switch"
        aria-checked={checked}
        disabled={disabled}
        onClick={() => onChange(!checked)}
        className={cn(
          'focus:outline-none focus:ring-2 focus:ring-interactive-primary focus:ring-offset-2',
          'rounded-full disabled:opacity-50 disabled:cursor-not-allowed',
          hasText &&
            'flex w-full items-center gap-3 text-left rounded-md focus:ring-offset-0',
          className
        )}
        {...props}
      >
        {hasIcon && (
          <span className="shrink-0 text-content-secondary">{icon}</span>
        )}
        {(hasLabel || hasDescription) && (
          <span className="flex-1 min-w-0">
            {hasLabel && (
              <span className="block text-sm font-medium text-content-primary">{label}</span>
            )}
            {hasDescription && (
              <span className="block text-xs text-content-secondary mt-0.5">{description}</span>
            )}
          </span>
        )}
        {switchEl}
      </button>
    );
  }
);

Toggle.displayName = 'Toggle';
