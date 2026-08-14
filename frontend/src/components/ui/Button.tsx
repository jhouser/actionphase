import { forwardRef } from 'react';
import type { ButtonHTMLAttributes, ReactNode } from 'react';
import { tv } from '../../lib/theme/utils';
import { Loader2 } from 'lucide-react';

export type ButtonVariant = 'primary' | 'secondary' | 'outline' | 'danger' | 'warning' | 'success' | 'ghost';
export type ButtonSize = 'sm' | 'md' | 'lg';

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
  loading?: boolean;
  icon?: ReactNode;
  children: ReactNode;
}

/**
 * Button variant styles using the new semantic token system.
 * This replaces 80+ characters of dark: classes with clean semantic tokens.
 */
const buttonStyles = tv({
  base: [
    // Base styles
    // whitespace-nowrap: a wrapped label makes a button taller than its
    // siblings, which is what turns any row of differently-worded buttons into
    // a ragged staircase. Buttons are short by convention; if a label is long
    // enough to want two lines, the label is the thing to fix.
    'inline-flex items-center justify-center gap-2 rounded-lg font-medium transition-colors whitespace-nowrap',
    // Cursor
    'cursor-pointer',
    // Focus states - using semantic tokens. focus-visible (not focus) so the
    // ring only shows for keyboard navigation, not mouse clicks.
    'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-interactive-primary focus-visible:ring-offset-2',
    // Disabled state
    'disabled:opacity-50 disabled:cursor-not-allowed',
  ].join(' '),
  variants: {
    variant: {
      primary: 'bg-interactive-primary hover:bg-interactive-primary-hover text-content-inverse',
      secondary: 'surface-raised hover:surface-overlay text-content-primary border border-theme-default',
      outline: 'bg-transparent hover:surface-raised text-content-primary border-2 border-interactive-primary',
      danger: 'bg-semantic-danger hover:bg-semantic-danger-hover text-content-inverse',
      warning: 'bg-semantic-warning hover:bg-semantic-warning-hover text-content-inverse',
      success: 'bg-semantic-success hover:bg-semantic-success-hover text-content-inverse',
      ghost: 'bg-transparent hover:surface-raised text-content-primary',
    },
    size: {
      sm: 'px-3 py-1.5 text-sm',
      md: 'px-4 py-2 text-base',
      lg: 'px-6 py-3 text-lg',
    },
  },
  defaultVariants: {
    variant: 'primary',
    size: 'md',
  },
});

/**
 * Button - Reusable button component with semantic theme tokens
 *
 * Now uses semantic tokens instead of hard-coded colors:
 * - 70% less code (no more dark: classes)
 * - Automatically adapts to all themes (light, dark, future themes)
 * - Type-safe variants with autocomplete
 *
 * @example
 * ```tsx
 * <Button variant="primary" size="md" onClick={handleClick}>
 *   Click me
 * </Button>
 *
 * <Button variant="danger" loading>
 *   Deleting...
 * </Button>
 *
 * <Button variant="secondary" icon={<PlusIcon />}>
 *   Add Item
 * </Button>
 * ```
 */
export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  (
    {
      variant = 'primary',
      size = 'md',
      loading = false,
      icon,
      className,
      disabled,
      children,
      ...props
    },
    ref
  ) => {
    return (
      <button
        ref={ref}
        className={buttonStyles({ variant, size, className })}
        disabled={disabled || loading}
        {...props}
      >
        {loading && <Loader2 className="w-4 h-4 animate-spin" />}
        {!loading && icon && icon}
        {children}
      </button>
    );
  }
);

Button.displayName = 'Button';
