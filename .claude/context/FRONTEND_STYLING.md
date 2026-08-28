# Frontend Styling & UI Component Library Guide

**Last Updated:** May 2026

## Overview

ActionPhase uses a **UI Component Library** (`@/components/ui`) to ensure consistent theming and automatic dark mode support across all components. This document provides comprehensive guidelines for building React components with the UI library.

## Critical Rules

### 🚫 NEVER Do This

```tsx
// ❌ Native HTML elements with manual dark mode classes - INCONSISTENT & BREAKS THEME
<div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-4">
  <input
    type="text"
    className="border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 px-3 py-2 rounded"
    placeholder="Enter text"
  />
  <button className="bg-blue-600 hover:bg-blue-700 dark:bg-blue-500 dark:hover:bg-blue-600 text-white px-4 py-2 rounded">
    Submit
  </button>
</div>
```

**Why this breaks:**
- Manual `dark:` classes are verbose and error-prone
- Inconsistent styling across components
- Violates DRY principle (colors defined everywhere)
- Hard to maintain when theme colors change
- No centralized type safety

### ✅ ALWAYS Do This

```tsx
// ✅ UI Component Library - CONSISTENT & AUTOMATIC DARK MODE
import { Card, CardBody, Input, Button } from '@/components/ui';

<Card variant="default" padding="md">
  <CardBody>
    <Input
      label="Email"
      type="text"
      placeholder="Enter text"
      value={value}
      onChange={(e) => setValue(e.target.value)}
    />
    <Button variant="primary" onClick={handleSubmit}>
      Submit
    </Button>
  </CardBody>
</Card>
```

**Why this works:**
- Automatic dark mode via CSS variables (no `dark:` classes needed)
- Consistent design across all components
- Type-safe with TypeScript
- Centralized theme management
- Less code to write and maintain

---

## UI Component Library Reference

All components are located in `frontend/src/components/ui/` and automatically adapt to the active theme.

### Import Pattern

```tsx
import { Button, Input, Card, Badge, Alert } from '@/components/ui';
```

---

## Available Components

### 1. **Button** - Action Buttons

**Variants:** `primary`, `secondary`, `danger`, `ghost`
**Sizes:** `sm`, `md`, `lg`

```tsx
import { Button } from '@/components/ui';

// Primary action
<Button variant="primary" onClick={handleSave}>
  Save Changes
</Button>

// Secondary action
<Button variant="secondary" onClick={handleCancel}>
  Cancel
</Button>

// Destructive action
<Button variant="danger" onClick={handleDelete}>
  Delete
</Button>

// Ghost button (transparent background)
<Button variant="ghost" onClick={handleClose}>
  Close
</Button>

// With loading state
<Button variant="primary" loading={isSubmitting}>
  {isSubmitting ? 'Saving...' : 'Save'}
</Button>

// With icon
<Button variant="primary" icon={<PlusIcon />}>
  Add Item
</Button>

// Different sizes
<Button variant="primary" size="sm">Small</Button>
<Button variant="primary" size="md">Medium</Button>
<Button variant="primary" size="lg">Large</Button>

// Disabled
<Button variant="primary" disabled>
  Cannot Click
</Button>
```

---

### 2. **Input** - Text Input Fields

**Variants:** `default`, `error`
**Sizes:** `sm`, `md`, `lg`

```tsx
import { Input } from '@/components/ui';

// Basic input with label
<Input
  label="Email"
  type="email"
  placeholder="you@example.com"
  value={email}
  onChange={(e) => setEmail(e.target.value)}
/>

// Input with error
<Input
  label="Username"
  type="text"
  variant="error"
  error="Username is required"
  value={username}
  onChange={(e) => setUsername(e.target.value)}
/>

// Input with helper text
<Input
  label="Password"
  type="password"
  helperText="Must be at least 8 characters"
  value={password}
  onChange={(e) => setPassword(e.target.value)}
/>

// Different sizes
<Input inputSize="sm" placeholder="Small input" />
<Input inputSize="md" placeholder="Medium input" />
<Input inputSize="lg" placeholder="Large input" />

// Required input
<Input
  label="Email"
  required
  type="email"
/>
```

---

### 3. **Textarea** - Multi-line Text Input

```tsx
import { Textarea } from '@/components/ui';

// Basic textarea
<Textarea
  label="Description"
  placeholder="Enter description..."
  value={description}
  onChange={(e) => setDescription(e.target.value)}
  rows={5}
/>

// With error
<Textarea
  label="Bio"
  variant="error"
  error="Bio cannot be empty"
  value={bio}
  onChange={(e) => setBio(e.target.value)}
/>

// With helper text
<Textarea
  label="Notes"
  helperText="Max 500 characters"
  maxLength={500}
/>
```

---

### 4. **Card** - Container Component

**Variants:** `default`, `elevated`, `bordered`
**Padding:** `none`, `sm`, `md`, `lg`

```tsx
import { Card, CardHeader, CardBody, CardFooter } from '@/components/ui';

// Basic card
<Card variant="default" padding="md">
  <h3>Card Content</h3>
  <p>Some text here</p>
</Card>

// Elevated card (shadow)
<Card variant="elevated" padding="lg">
  <h3>Elevated Card</h3>
</Card>

// Bordered card
<Card variant="bordered" padding="md">
  <h3>Bordered Card</h3>
</Card>

// Card with sections
<Card variant="default" padding="none">
  <CardHeader>
    <h2>Dashboard</h2>
    <p>Overview of your account</p>
  </CardHeader>
  <CardBody>
    <p>Main content goes here</p>
  </CardBody>
  <CardFooter>
    <Button variant="primary">Save</Button>
    <Button variant="secondary">Cancel</Button>
  </CardFooter>
</Card>
```

---

### 5. **Badge** - Status Indicators

**Variants:** `primary`, `secondary`, `success`, `warning`, `danger`, `neutral`
**Sizes:** `sm`, `md`, `lg`

```tsx
import { Badge } from '@/components/ui';

// Status badges
<Badge variant="success">Active</Badge>
<Badge variant="warning">Pending</Badge>
<Badge variant="danger">Blocked</Badge>
<Badge variant="neutral">Draft</Badge>

// With dot indicator
<Badge variant="success" dot>Online</Badge>

// Different sizes
<Badge variant="primary" size="sm">Small</Badge>
<Badge variant="primary" size="md">Medium</Badge>
<Badge variant="primary" size="lg">Large</Badge>
```

---

### 6. **Alert** - Notification Messages

**Variants:** `info`, `success`, `warning`, `danger`

```tsx
import { Alert } from '@/components/ui';

// Basic alert
<Alert variant="info">
  Important information for you to read.
</Alert>

// Alert with title
<Alert variant="success" title="Success!">
  Your changes have been saved.
</Alert>

// Dismissible alert
<Alert
  variant="warning"
  title="Warning"
  dismissible
  onDismiss={() => setShowAlert(false)}
>
  This action cannot be undone.
</Alert>

// Error alert
<Alert variant="danger" title="Error">
  Something went wrong. Please try again.
</Alert>
```

---

### 7. **Select** - Dropdown Select

```tsx
import { Select } from '@/components/ui';

// Basic select
<Select
  label="Country"
  value={country}
  onChange={(e) => setCountry(e.target.value)}
>
  <option value="">Select country</option>
  <option value="us">United States</option>
  <option value="ca">Canada</option>
  <option value="uk">United Kingdom</option>
</Select>

// With error
<Select
  label="Role"
  variant="error"
  error="Please select a role"
  value={role}
  onChange={(e) => setRole(e.target.value)}
>
  <option value="">Choose role</option>
  <option value="admin">Admin</option>
  <option value="user">User</option>
</Select>
```

---

### 8. **Checkbox** - Checkbox Input

```tsx
import { Checkbox } from '@/components/ui';

// Basic checkbox
<Checkbox
  label="Accept terms and conditions"
  checked={accepted}
  onChange={(e) => setAccepted(e.target.checked)}
/>

// With helper text
<Checkbox
  label="Subscribe to newsletter"
  helperText="You can unsubscribe anytime"
  checked={subscribed}
  onChange={(e) => setSubscribed(e.target.checked)}
/>

// With error
<Checkbox
  label="Agree to terms"
  error="You must agree to continue"
  checked={agreed}
  onChange={(e) => setAgreed(e.target.checked)}
/>
```

---

### 9. **Radio** - Radio Button

```tsx
import { Radio } from '@/components/ui';

// Radio group
<div>
  <Radio
    name="plan"
    value="basic"
    label="Basic Plan ($10/mo)"
    checked={plan === 'basic'}
    onChange={(e) => setPlan(e.target.value)}
  />
  <Radio
    name="plan"
    value="pro"
    label="Pro Plan ($25/mo)"
    checked={plan === 'pro'}
    onChange={(e) => setPlan(e.target.value)}
  />
  <Radio
    name="plan"
    value="enterprise"
    label="Enterprise Plan ($100/mo)"
    checked={plan === 'enterprise'}
    onChange={(e) => setPlan(e.target.value)}
  />
</div>
```

---

### 10. **DateTimeInput** - Date/Time Picker

```tsx
import { DateTimeInput } from '@/components/ui';

// Date input
<DateTimeInput
  label="Start Date"
  type="date"
  value={startDate}
  onChange={(e) => setStartDate(e.target.value)}
/>

// DateTime input
<DateTimeInput
  label="Deadline"
  type="datetime-local"
  value={deadline}
  onChange={(e) => setDeadline(e.target.value)}
/>
```

---

### 11. **Spinner** - Loading Indicator

**Variants:** `primary`, `secondary`, `white`
**Sizes:** `sm`, `md`, `lg`, `xl`

```tsx
import { Spinner } from '@/components/ui';

// Basic spinner
<Spinner size="md" />

// With label
<Spinner size="lg" label="Loading..." />

// Different variants
<Spinner variant="primary" size="md" />
<Spinner variant="secondary" size="md" />
<Spinner variant="white" size="md" /> {/* For colored backgrounds */}
```

---

### 12. **Label** - Form Label

```tsx
import { Label } from '@/components/ui';

// Required label
<Label htmlFor="email" required>
  Email Address
</Label>

// Optional label
<Label htmlFor="bio" optional>
  Biography
</Label>

// Error state label
<Label htmlFor="password" error>
  Password
</Label>
```

---

### 13. **Modal** - Dialog with Overlay

```tsx
import { Modal, Button } from '@/components/ui';

<Modal
  isOpen={isOpen}
  onClose={() => setIsOpen(false)}
  title="Confirm deletion"
  size="md"                 // 'sm' | 'md' | 'lg' | 'xl' (default 'md')
  showCloseButton           // optional
  footer={
    <>
      <Button variant="secondary" onClick={close}>Cancel</Button>
      <Button variant="danger" onClick={confirm}>Delete</Button>
    </>
  }
>
  This cannot be undone.
</Modal>
```

### 14. **Drawer** - Sidebar / Bottom Sheet

Responsive by default: a right sidebar at `lg+`, a bottom sheet below `lg`.

```tsx
import { Drawer } from '@/components/ui';

<Drawer
  open={open}
  onClose={close}
  title="Filters"
  side="responsive"         // 'responsive' (default) | 'right' | 'bottom'
>
  {children}
</Drawer>
```

`zIndexClass` sets the stacking tier — it defaults to the modal tier; the utility
drawer passes `LAYERS.drawer` to sit above modals. Use the prop that suppresses
the drawer's own backdrop when opening over an already-dimmed overlay.

### 15. **HelpTooltip** - Inline Help Icon

Use instead of parenthetical clarifications in labels.

```tsx
import { HelpTooltip } from '@/components/ui';

<label className="flex items-center gap-1">
  Deadline
  <HelpTooltip text="When actions stop being accepted." align="left" />
</label>
```

`align="right"` when the icon sits near the right edge — a left-anchored panel
would overflow the container there.

### 16. **MetadataItem** - Icon + Label + Value

For information-dense areas: game cards, post headers.

```tsx
import { MetadataItem } from '@/components/ui';

<MetadataItem icon={<ClockIcon />} label="Deadline" value="2 hours" />
```

## Common Component Patterns

### Login Form

```tsx
import { Card, CardHeader, CardBody, CardFooter, Input, Button, Alert } from '@/components/ui';

function LoginForm() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  return (
    <Card variant="elevated" padding="md">
      <CardHeader>
        <h2>Sign In</h2>
        <p>Welcome back! Please sign in to continue.</p>
      </CardHeader>
      <CardBody>
        {error && (
          <Alert variant="danger" dismissible onDismiss={() => setError('')}>
            {error}
          </Alert>
        )}
        <Input
          label="Email"
          type="email"
          placeholder="you@example.com"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          required
        />
        <Input
          label="Password"
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          required
        />
      </CardBody>
      <CardFooter>
        <Button variant="primary" loading={loading} onClick={handleLogin}>
          Sign In
        </Button>
        <Button variant="ghost" onClick={() => navigate('/register')}>
          Create Account
        </Button>
      </CardFooter>
    </Card>
  );
}
```

---

### Settings Form

```tsx
import { Card, CardHeader, CardBody, Input, Textarea, Select, Checkbox, Button } from '@/components/ui';

function SettingsForm() {
  return (
    <Card variant="default" padding="md">
      <CardHeader>
        <h2>Profile Settings</h2>
      </CardHeader>
      <CardBody>
        <Input
          label="Display Name"
          type="text"
          value={displayName}
          onChange={(e) => setDisplayName(e.target.value)}
        />
        <Textarea
          label="Bio"
          rows={4}
          helperText="Tell us about yourself (max 500 characters)"
          maxLength={500}
          value={bio}
          onChange={(e) => setBio(e.target.value)}
        />
        <Select
          label="Time Zone"
          value={timezone}
          onChange={(e) => setTimezone(e.target.value)}
        >
          <option value="UTC">UTC</option>
          <option value="EST">Eastern Time</option>
          <option value="PST">Pacific Time</option>
        </Select>
        <Checkbox
          label="Email notifications"
          checked={emailNotifications}
          onChange={(e) => setEmailNotifications(e.target.checked)}
        />
      </CardBody>
      <CardFooter>
        <Button variant="primary" onClick={handleSave}>
          Save Changes
        </Button>
        <Button variant="secondary" onClick={handleReset}>
          Reset
        </Button>
      </CardFooter>
    </Card>
  );
}
```

---

### Status Dashboard

```tsx
import { Card, CardHeader, CardBody, Badge, Button } from '@/components/ui';

function StatusDashboard() {
  return (
    <Card variant="default" padding="md">
      <CardHeader>
        <div className="flex items-center justify-between">
          <h2>Game Status</h2>
          <Badge variant="success" dot>Active</Badge>
        </div>
      </CardHeader>
      <CardBody>
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <span>Players Online</span>
            <Badge variant="primary">12</Badge>
          </div>
          <div className="flex items-center justify-between">
            <span>Pending Actions</span>
            <Badge variant="warning">3</Badge>
          </div>
          <div className="flex items-center justify-between">
            <span>Completed Phases</span>
            <Badge variant="neutral">7</Badge>
          </div>
        </div>
      </CardBody>
      <CardFooter>
        <Button variant="primary">View Details</Button>
      </CardFooter>
    </Card>
  );
}
```

---

## Markdown Content

**ALWAYS use the MarkdownPreview component** for rendering markdown content. It has built-in dark mode support.

```tsx
import { MarkdownPreview } from '@/components/MarkdownPreview';

// Basic usage
<MarkdownPreview content={markdownText} />

// With character mentions
<MarkdownPreview
  content={messageContent}
  mentionedCharacters={characters}
/>
```

**DO NOT** render markdown with plain ReactMarkdown:
```tsx
// ❌ WRONG - No automatic dark mode
<ReactMarkdown className="prose dark:prose-invert">{content}</ReactMarkdown>

// ✅ CORRECT - Use MarkdownPreview
<MarkdownPreview content={content} />
```

---

## When You Can't Use UI Components

For **layout-only elements** (flexbox containers, grids, spacers), use semantic tokens:

```tsx
// Layout container
<div className="surface-page min-h-screen">
  <div className="surface-base border border-theme-default rounded-lg p-4">
    {/* UI components here */}
  </div>
</div>

// Text elements
<h1 className="text-content-primary text-2xl font-bold">Heading</h1>
<p className="text-content-secondary">Body text</p>
<span className="text-content-tertiary">Secondary text</span>
```

**Available semantic token classes**

> ⚠️ **Verified 2026-08-26 against the built CSS.** Being declared in `@theme` is
> *not* sufficient — a token also needs a value assigned in
> `frontend/src/lib/theme/themes.ts`. The `bg-bg-*` and `border-border-*`
> families are declared but **never assigned**, so they emit no CSS and render
> nothing. They were removed from app code; do not reintroduce them.
> The authoritative list is `frontend/src/components/ui/README.md`.

**Surfaces (backgrounds):**
- `surface-page` - Page background
- `surface-base` - Cards, modals, primary containers
- `surface-raised` - Hover states, active tabs, subtle contrast
- `surface-overlay` - Dropdowns, popovers
- `surface-sunken` - Input surfaces, wells, skeleton bars

**Content (text):**
- `text-content-primary` - Headings and body text
- `text-content-secondary` - Supporting text
- `text-content-tertiary` - De-emphasized text
- `text-content-disabled` - Disabled text
- `text-content-inverse` - Text on a filled background
- (The `text-text-*` family was **retired 2026-08-26** — see below.)

**Borders:**
- `border-theme-default` - Standard borders
- `border-theme-subtle` - Low-emphasis dividers
- `border-theme-strong` - High-emphasis borders

**Interactive & semantic states:**
- `bg-interactive-primary`, `bg-interactive-primary-subtle`, `bg-interactive-secondary`
- `text-interactive-primary`, `text-accent-primary`
- `bg-semantic-{danger,warning,success,info}-subtle`
- `border-semantic-{danger,warning,success,info}`, `text-semantic-danger`

**Retired — do not use.** The `text-text-*` family was removed on 2026-08-26.
It was worse than merely dead: `text-text-primary` resolved to
`--color-content-secondary` (*not* `-primary`), so the name actively lied about
the color, and `text-text-muted` / `-disabled` referenced variables no theme
assigns — producing an invalid color that silently inherited. Replacements:
`text-content-primary`→`text-content-primary`, `text-text-primary`/`-secondary`→
`text-content-secondary`, `text-text-muted`→`text-content-tertiary`.

Also retired (emit no CSS): `bg-bg-*`, `border-border-*`, `bg-primary`,
`bg-{danger,warning,success}-light`, `text-{danger,warning,success,primary}-text`,
`text-danger`, `bg-accent-primary`, `border-accent-primary`,
`placeholder-placeholder`, `ring-focus-ring`.

**Opacity modifiers don't work** on these (they are hand-written utility classes,
not `@theme` colors): use `bg-interactive-primary-subtle`, not
`bg-interactive-primary/10`.

If you need a token that isn't listed here, add it to `@theme` in
`src/index.css` rather than reaching for a hardcoded Tailwind color.

---

## Testing Dark Mode

### Manual Testing Checklist

Before committing any component:

1. **Enable Dark Mode:**
   - Navigate to `/settings`
   - Click "Dark" radio button
   - Return to your component

2. **Visual Inspection:**
   - [ ] All text is readable (proper contrast)
   - [ ] All backgrounds adapt to theme
   - [ ] Interactive elements have visible hover/focus states
   - [ ] No white boxes on dark backgrounds
   - [ ] Borders are visible but subtle

3. **Toggle Test:**
   - [ ] Switch between Light → Dark → Light
   - [ ] Verify no flashing or layout shifts
   - [ ] Verify smooth transitions

4. **Visit Theme Test Page:**
   - [ ] Navigate to `/theme-test`
   - [ ] Verify all UI components work in both themes

---

## Pre-Commit Checklist

Before submitting a PR with frontend changes:

**UI Component Usage:**
- [ ] Uses `<Button>` instead of `<button>`
- [ ] Uses `<Input>` instead of `<input>`
- [ ] Uses `<Textarea>` instead of `<textarea>`
- [ ] Uses `<Select>` instead of `<select>`
- [ ] Uses `<Card>` for containers instead of manual `<div>`
- [ ] Uses `<Badge>` for status indicators
- [ ] Uses `<Alert>` for notifications

**Testing:**
- [ ] Component tested in Light mode
- [ ] Component tested in Dark mode
- [ ] Tested on `/theme-test` page if applicable
- [ ] Focus states visible in both modes
- [ ] No layout shifts when toggling themes

**Code Quality:**
- [ ] No hardcoded colors (`bg-white`, `text-gray-900`, etc.)
- [ ] Layout containers use `surface-*`, `text-content-*`, and `border-theme-*` tokens
- [ ] Markdown uses MarkdownPreview component
- [ ] Forms include proper labels and error handling

---

## Resources

- **UI Component Library:** `frontend/src/components/ui/`
- **Component Documentation:** `frontend/src/components/ui/README.md`
- **Theme Test Page:** Navigate to `/theme-test` in browser
- **MarkdownPreview Component:** `frontend/src/components/MarkdownPreview.tsx`
- **CSS Variables:** Defined in `frontend/src/index.css`
- **Theme Provider:** `frontend/src/contexts/ThemeContext.tsx`

---

## Migration Guide

### Converting Existing Components

When updating an existing component that uses native HTML:

1. **Identify native HTML elements:**
   - `<button>` → `<Button>`
   - `<input type="text">` → `<Input>`
   - `<textarea>` → `<Textarea>`
   - `<select>` → `<Select>`
   - Container `<div>` with borders → `<Card>`

2. **Add UI library import:**
   ```tsx
   import { Button, Input, Card } from '@/components/ui';
   ```

3. **Replace elements:**
   ```tsx
   // Before
   <button className="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded">
     Save
   </button>

   // After
   <Button variant="primary" onClick={handleSave}>
     Save
   </Button>
   ```

4. **Test in both themes:**
   - Light mode (default)
   - Dark mode (verify contrast)

### Example Migration

**Before:**
```tsx
export function GameCard({ game }: GameCardProps) {
  return (
    <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-4">
      <h3 className="text-gray-900 dark:text-white font-bold">{game.name}</h3>
      <p className="text-gray-600 dark:text-gray-400 text-sm">{game.description}</p>
      <button className="mt-4 bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded">
        Join Game
      </button>
    </div>
  );
}
```

**After:**
```tsx
import { Card, CardBody, Button } from '@/components/ui';

export function GameCard({ game }: GameCardProps) {
  return (
    <Card variant="default" padding="md">
      <CardBody>
        <h3 className="text-content-primary font-bold">{game.name}</h3>
        <p className="text-content-secondary text-sm">{game.description}</p>
        <Button variant="primary" onClick={handleJoin}>
          Join Game
        </Button>
      </CardBody>
    </Card>
  );
}
```

---

## Quick Reference Cheat Sheet

```tsx
// Buttons
<Button variant="primary">Primary</Button>
<Button variant="secondary">Secondary</Button>
<Button variant="danger">Delete</Button>
<Button variant="ghost">Close</Button>

// Forms
<Input label="Email" type="email" />
<Textarea label="Description" rows={4} />
<Select label="Country"><option>US</option></Select>
<Checkbox label="Agree to terms" />
<Radio label="Option A" name="choice" />

// Containers
<Card variant="default" padding="md">
  <CardHeader><h2>Title</h2></CardHeader>
  <CardBody><p>Content</p></CardBody>
  <CardFooter><Button>Action</Button></CardFooter>
</Card>

// Status
<Badge variant="success">Active</Badge>
<Alert variant="info">Information</Alert>
<Spinner size="md" />

// Markdown
<MarkdownPreview content={markdown} />
```
