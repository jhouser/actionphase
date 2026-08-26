# UI Component Library

Reusable React components with built-in dark mode support using CSS variables.

## Overview

This component library provides consistent, accessible, and themeable components that automatically adapt to light and dark modes without requiring manual `dark:` class prefixes.

**Key Features:**
- ✅ Automatic dark mode switching via CSS variables
- ✅ TypeScript-first with full type safety
- ✅ Consistent design language across all components
- ✅ Accessible by default
- ✅ Composable and flexible APIs
- ✅ 29-58% fewer characters compared to manual dark mode classes

## Available Components

### Button

A versatile button component with multiple variants, sizes, and states.

**Import:**
```tsx
import { Button } from '@/components/ui';
```

**Props:**
- `variant`: `'primary' | 'secondary' | 'danger' | 'ghost'` (default: `'primary'`)
- `size`: `'sm' | 'md' | 'lg'` (default: `'md'`)
- `loading`: `boolean` - Shows spinner when true
- `icon`: `ReactNode` - Icon to display before text
- All standard button HTML attributes

**Examples:**
```tsx
// Basic usage
<Button variant="primary" onClick={handleClick}>
  Click me
</Button>

// With icon
<Button variant="secondary" icon={<PlusIcon />}>
  Add Item
</Button>

// Loading state
<Button variant="primary" loading={isSubmitting}>
  Submit
</Button>

// Different sizes
<Button variant="danger" size="sm">Delete</Button>
<Button variant="primary" size="lg">Get Started</Button>
```

---

### Input

Form input component with labels, validation, and helper text.

**Import:**
```tsx
import { Input } from '@/components/ui';
```

**Props:**
- `label`: `string` - Optional label text
- `inputSize`: `'sm' | 'md' | 'lg'` (default: `'md'`)
- `variant`: `'default' | 'error'` (default: `'default'`)
- `error`: `string` - Error message to display
- `helperText`: `string` - Helper text below input
- All standard input HTML attributes

**Examples:**
```tsx
// Basic input with label
<Input
  label="Email"
  type="email"
  placeholder="Enter your email"
  value={email}
  onChange={(e) => setEmail(e.target.value)}
/>

// Input with error
<Input
  label="Username"
  type="text"
  variant="error"
  error="Username is required"
/>

// Input with helper text
<Input
  label="Password"
  type="password"
  helperText="Must be at least 8 characters"
/>

// Different sizes
<Input inputSize="sm" placeholder="Small input" />
<Input inputSize="lg" placeholder="Large input" />
```

---

### Card

Container component with multiple variants and optional sections.

**Import:**
```tsx
import { Card, CardHeader, CardBody, CardFooter } from '@/components/ui';
```

**Card Props:**
- `variant`: `'default' | 'elevated' | 'bordered'` (default: `'default'`)
- `padding`: `'none' | 'sm' | 'md' | 'lg'` (default: `'md'`)
- All standard div HTML attributes

**Examples:**
```tsx
// Basic card
<Card variant="elevated" padding="md">
  <h3>Card Title</h3>
  <p>Card content</p>
</Card>

// Card with sections
<Card variant="default" padding="none">
  <CardHeader>
    <h3>Dashboard</h3>
    <p>Overview of your account</p>
  </CardHeader>
  <CardBody>
    <p>Main content goes here</p>
  </CardBody>
  <CardFooter>
    <Button variant="primary">Save</Button>
  </CardFooter>
</Card>

// Bordered card
<Card variant="bordered" padding="lg">
  <div>Important content</div>
</Card>
```

---

### Badge

Status indicator component for labels and tags.

**Import:**
```tsx
import { Badge } from '@/components/ui';
```

**Props:**
- `variant`: `'primary' | 'secondary' | 'success' | 'warning' | 'danger' | 'neutral'` (default: `'neutral'`)
- `size`: `'sm' | 'md' | 'lg'` (default: `'md'`)
- `dot`: `boolean` - Shows dot indicator before text
- All standard span HTML attributes

**Examples:**
```tsx
<Badge variant="success">Active</Badge>
<Badge variant="warning" size="sm">Pending</Badge>
<Badge variant="danger" dot>Urgent</Badge>
```

---

### Alert

Notification and message component with icons.

**Import:**
```tsx
import { Alert } from '@/components/ui';
```

**Props:**
- `variant`: `'info' | 'success' | 'warning' | 'danger'` (default: `'info'`)
- `title`: `string` - Optional title text
- `dismissible`: `boolean` - Shows dismiss button
- `onDismiss`: `() => void` - Callback when dismissed
- All standard div HTML attributes

**Examples:**
```tsx
<Alert variant="success" title="Success!">
  Your changes have been saved.
</Alert>

<Alert variant="danger" dismissible onDismiss={handleClose}>
  An error occurred.
</Alert>
```

---

### Spinner

Loading indicator component.

**Import:**
```tsx
import { Spinner } from '@/components/ui';
```

**Props:**
- `size`: `'sm' | 'md' | 'lg' | 'xl'` (default: `'md'`)
- `variant`: `'primary' | 'secondary' | 'white'` (default: `'primary'`)
- `label`: `string` - Optional label text
- All standard div HTML attributes (except children)

**Examples:**
```tsx
<Spinner size="lg" />
<Spinner variant="white" label="Loading..." />
```

---

### Label

Form label component with required/optional indicators.

**Import:**
```tsx
import { Label } from '@/components/ui';
```

**Props:**
- `required`: `boolean` - Shows * indicator
- `optional`: `boolean` - Shows (optional) text
- `error`: `boolean` - Error styling
- All standard label HTML attributes

**Examples:**
```tsx
<Label htmlFor="email" required>Email</Label>
<Label htmlFor="bio" optional>Biography</Label>
```

---

### Checkbox

Checkbox input with label and helper text.

**Import:**
```tsx
import { Checkbox } from '@/components/ui';
```

**Props:**
- `label`: `string` - Label text
- `error`: `string` - Error message
- `helperText`: `string` - Helper text
- All standard input HTML attributes (except type)

**Examples:**
```tsx
<Checkbox
  label="Accept terms"
  checked={accepted}
  onChange={(e) => setAccepted(e.target.checked)}
/>

<Checkbox
  label="Subscribe"
  helperText="You can unsubscribe anytime"
/>
```

---

### Radio

Radio button with label and helper text.

**Import:**
```tsx
import { Radio } from '@/components/ui';
```

**Props:**
- `label`: `string` - Label text
- `error`: `string` - Error message
- `helperText`: `string` - Helper text
- All standard input HTML attributes (except type)

**Examples:**
```tsx
<Radio
  name="plan"
  value="basic"
  label="Basic Plan"
  checked={plan === 'basic'}
  onChange={(e) => setPlan(e.target.value)}
/>
```

---

### Toggle

Accessible on/off switch (`role="switch"`). Use this instead of hand-rolled
toggle markup — the off-state uses defined theme tokens (`bg-surface-sunken` +
`border-theme-strong`) so it stays visible in light mode. (The legacy
`bg-bg-secondary` / `border-border-primary` utilities are unassigned in the
current theme system and render invisible — that was the original toggle bug.
See "Retired tokens" below; the same class of bug was found and fixed across
~50 more files on 2026-08-26.)

**Import:**
```tsx
import { Toggle } from '@/components/ui';
```

**Props:**
- `checked`: `boolean` - Whether the toggle is on (required)
- `onChange`: `(checked: boolean) => void` - Called with the next value (required)
- `size`: `'sm' | 'md'` - Track size (default `md`)
- `label`: `ReactNode` - Optional label rendered left of the switch
- `description`: `ReactNode` - Optional secondary description under the label
- `icon`: `ReactNode` - Optional leading icon
- All standard button HTML attributes (except `type` and `onChange`), e.g.
  `data-testid`, `aria-label`, `disabled`

**Examples:**
```tsx
// Full row with label + description (renders one clickable row)
<Toggle
  checked={enabled}
  onChange={setEnabled}
  label="Private Messages"
  description="Get a Discord DM for new private messages."
/>

// Compact switch with an icon
<Toggle
  checked={on}
  onChange={setOn}
  size="sm"
  icon={<EyeOff className="w-5 h-5" />}
  label="Screenshot Mode"
  description="Hide all usernames in screenshots."
/>

// Bare switch (bring your own layout) — always pass an aria-label
<Toggle checked={on} onChange={setOn} aria-label="Toggle setting" />
```

---

## Design Token Reference

All components use semantic tokens that switch automatically with the theme.
**Never write `dark:` variants or raw palette colors** (`bg-white`, `text-gray-600`).

> **Verified 2026-08-26** against the built CSS. Every token below emits a real
> rule. Tokens *not* listed here may look plausible but produce **no CSS at all**
> — see "Retired tokens" at the end of this section.

### Surfaces (backgrounds)
- `surface-page` - Page background
- `surface-base` - Cards, modals, primary containers
- `surface-raised` - Hover states, active tabs, subtle contrast
- `surface-overlay` - Dropdowns, popovers
- `surface-sunken` - Input backgrounds, wells, skeleton bars

### Content (text)
- `text-content-primary` - Headings and body text
- `text-content-secondary` - Supporting text
- `text-content-tertiary` - De-emphasized text
- `text-content-disabled` - Disabled text
- `text-content-inverse` - Text on a filled/colored background
  (The `text-text-*` family was retired 2026-08-26 — see below.)

### Borders
- `border-theme-default` - Standard borders
- `border-theme-subtle` - Light dividers
- `border-theme-strong` - Emphasized borders

### Interactive
- `bg-interactive-primary` - Primary buttons, active indicators
- `bg-interactive-primary-subtle` - Selected-row / tinted primary background
- `bg-interactive-secondary` - Secondary buttons, inactive bars
- `text-interactive-primary` - Links, active tab labels
- `text-accent-primary` / `hover:text-accent-primary`

### Semantic states (alerts, badges)
- `bg-semantic-danger-subtle` / `border-semantic-danger` / `text-semantic-danger`
- `bg-semantic-warning-subtle` / `border-semantic-warning`
- `bg-semantic-success-subtle` / `border-semantic-success`
- `bg-semantic-info-subtle` / `border-semantic-info`

### ⚠️ Retired tokens — these render nothing

These names are registered in `@theme` but were **never assigned values**, so
Tailwind emits no rule for them. An element using one gets no background/border
at all. They were removed from app code on 2026-08-26.

| Retired (dead) | Use instead |
|---|---|
| `bg-bg-primary` | `surface-base` |
| `bg-bg-secondary` | `surface-raised` |
| `bg-bg-tertiary`, `bg-bg-input` | `surface-sunken` |
| `bg-bg-page`, `bg-bg-hover`, `bg-bg-active` | `surface-page` / `surface-raised` |
| `border-border-primary`, `border-border-default` | `border-theme-default` |
| `border-border-secondary`, `border-border-input` | `border-theme-strong` |
| `bg-primary`, `bg-primary-light`, `text-primary-text` | `bg-interactive-primary` / `-subtle` |
| `bg-danger-light`, `text-danger-text`, `text-danger` | `bg-semantic-danger-subtle` / `text-semantic-danger` |
| `bg-warning-light`, `text-warning-text` | `bg-semantic-warning-subtle` |
| `bg-success-light`, `text-success-text` | `bg-semantic-success-subtle` |
| `bg-accent-primary`, `border-accent-primary` | `bg-interactive-primary` / `border-interactive-primary` |
| `placeholder-placeholder`, `ring-focus-ring` | no replacement needed |
| `text-text-heading` | `text-content-primary` |
| `text-text-primary`, `text-text-secondary` | `text-content-secondary` |
| `text-text-muted` | `text-content-tertiary` |
| `text-text-disabled` | `text-content-disabled` |

The `text-text-*` family deserves special mention: it was not merely unassigned.
`text-text-primary` resolved to `--color-content-secondary` — *not* `-primary` —
so the class name actively misrepresented the color it produced, and
`text-text-muted`/`-disabled` pointed at variables no theme assigns, yielding an
invalid color that silently inherited from the parent. All five classes were
deleted from `index.css`.

**Note on opacity modifiers:** these are hand-written utility classes, not
`@theme` colors, so `bg-interactive-primary/10` does **not** work. Use the
`-subtle` variant instead.

### Focus States
- `focus:ring-focus-ring` - Focus ring color
- `ring-offset-focus-ring-offset` - Focus ring offset

### Placeholders
- `placeholder-placeholder` - Placeholder text color

---

## Usage Guidelines

### When to Use Components

✅ **Do:**
- Use these components for new features
- Migrate existing components during refactors
- Combine components for complex UIs
- Extend components with additional props

❌ **Don't:**
- Mix old `dark:` classes with new components (causes inconsistency)
- Override core color styles (use variants instead)
- Create duplicate components (extend existing ones)

### Composition Patterns

**Form with Card:**
```tsx
<Card variant="elevated" padding="md">
  <CardHeader>
    <h2>Login</h2>
  </CardHeader>
  <CardBody>
    <Input label="Email" type="email" />
    <Input label="Password" type="password" />
  </CardBody>
  <CardFooter>
    <Button variant="primary" loading={isLoading}>
      Sign In
    </Button>
  </CardFooter>
</Card>
```

**Action List:**
```tsx
<Card variant="default">
  <CardHeader>
    <h3>Actions</h3>
  </CardHeader>
  <CardBody>
    <div className="space-y-2">
      <Button variant="primary" icon={<PlusIcon />}>
        Add
      </Button>
      <Button variant="secondary" icon={<EditIcon />}>
        Edit
      </Button>
      <Button variant="danger" icon={<TrashIcon />}>
        Delete
      </Button>
    </div>
  </CardBody>
</Card>
```

---

## Testing

Visit `/theme-test` to see all components in action with both light and dark themes.

**Test Checklist:**
- [ ] All variants render correctly
- [ ] Components work in light mode
- [ ] Components work in dark mode
- [ ] Interactive states (hover, focus, disabled) work
- [ ] Loading states display correctly
- [ ] Form validation displays properly

---

## Adding New Components

When adding new components to this library:

1. **Create the component file** in `src/components/ui/`
2. **Use CSS variables** for all colors (no `dark:` classes)
3. **Export types** for all props
4. **Add to barrel export** in `src/components/ui/index.ts`
5. **Document** in this README
6. **Add to test page** at `src/pages/ThemeTestPage.tsx`

**Template:**
```tsx
import { HTMLAttributes } from 'react';
import { clsx } from 'clsx';

export interface MyComponentProps extends HTMLAttributes<HTMLDivElement> {
  variant?: 'default' | 'custom';
  // ... other props
}

export function MyComponent({ variant = 'default', className, ...props }: MyComponentProps) {
  return (
    <div
      className={clsx(
        'base-styles',
        variant === 'default' && 'surface-base text-content-primary',
        variant === 'custom' && 'surface-raised text-content-secondary',
        className
      )}
      {...props}
    />
  );
}
```

---

## Migration Guide

**Quick Migration:**
1. Replace `bg-white dark:bg-gray-800` → `surface-base`
2. Replace `text-gray-900 dark:text-white` → `text-content-primary`
3. Replace `border-gray-200 dark:border-gray-700` → `border-theme-default`
4. Use components instead of custom markup where possible

Verify any token you introduce against the reference above — several
plausible-looking names are retired and emit no CSS.

---

## Performance

**Bundle Size:**
- Components: ~3KB gzipped (minimal overhead)
- CSS Variables: ~1KB (one-time addition to global CSS)
- Total: Same or smaller than manual dark mode classes

**Runtime:**
- CSS variable resolution is native browser behavior (fast)
- No JavaScript needed for theme switching
- Better caching due to fewer unique class combinations

---

## Browser Support

✅ All modern browsers:
- Chrome/Edge (latest 2 versions)
- Firefox (latest 2 versions)
- Safari (latest 2 versions)

CSS custom properties are well-supported in all target browsers.

---

## Contributing

When contributing components:
1. Follow existing patterns
2. Use TypeScript strict mode
3. Include prop documentation
4. Add examples to README
5. Test in both light and dark modes

---

## Future Components (Planned)

**Tier 2:**
- Label
- Checkbox
- Radio
- Badge
- Alert
- Spinner

**Tier 3:**
- Tabs
- Dropdown/Select
- DatePicker
- Tooltip
- Modal (refactor existing)
- Toast/Notification

See `.claude/planning/DARK_MODE_REFACTOR_PLAN.md` for complete roadmap.
