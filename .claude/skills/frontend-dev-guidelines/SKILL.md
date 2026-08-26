---
name: frontend-dev-guidelines
description: Frontend development guidelines for the ActionPhase React/TypeScript app. Covers the real component layout (flat components/ + pages/, no features directory), TanStack Query data fetching with useQuery, React Router 7 createBrowserRouter, the @/components/ui library, Tailwind v4 semantic tokens for dark mode, and TypeScript standards. Use when creating components, pages, fetching data, styling, routing, or working with frontend code.
---

# Frontend Development Guidelines

## Purpose

Guide for React development in ActionPhase as the codebase actually is: a flat
`components/` + `pages/` layout, TanStack Query with `useQuery`, React Router 7's
data-router API, and an in-house UI component library styled with Tailwind v4
semantic tokens.

> **Rewritten 2026-08-26.** The previous version of this skill described a
> different project: it prescribed a `features/` directory, `useSuspenseQuery` as
> the primary fetching pattern, a `SuspenseLoader` component, a `routes/`
> directory, and a component template importing **MUI** (`@mui/material`). None
> of those exist here — MUI is not a dependency. Everything below is verified
> against the codebase; see `.claude/DOC_AUDIT_INVENTORY.md`.

## When to Use This Skill

- Creating components or pages
- Fetching data with TanStack Query
- Adding routes
- Styling components / dark mode
- TypeScript patterns in the frontend

---

## Quick Start

### New Component Checklist

- [ ] Put it in `frontend/src/components/` (flat, or an existing topical subdir)
- [ ] Export as a **named** `export const`/`export function` — this is the
      dominant convention (~180 files vs 6 using default exports)
- [ ] Use the UI library first: `@/components/ui` (`Button`, `Input`, `Card`, …)
- [ ] Colors via semantic tokens (`text-content-primary`, `surface-base`), never
      `dark:` variants or raw palette colors
- [ ] Co-locate the test: `ComponentName.test.tsx`
- [ ] Fetch with `useQuery` / `useMutation` from an `@/hooks` hook
- [ ] `useCallback` for handlers passed to memoized children

Typing the component with `React.FC<Props>` or as a plain function are **both**
in live use. Match the file you're editing; neither is mandated.

**[📖 resources/component-patterns.md](resources/component-patterns.md)** — ⚠️ its
`SuspenseLoader` and default-export guidance no longer applies.

### Adding a Feature

There is no `features/` directory and no per-feature scaffold. Work lands in the
existing directories:

| Layer | Location |
|---|---|
| API method | `frontend/src/lib/api/<domain>.ts` (exported via `apiClient`) |
| Hook | `frontend/src/hooks/use<Thing>.ts` |
| Component | `frontend/src/components/` |
| Page | `frontend/src/pages/` |
| Route | a route object in `frontend/src/App.tsx` |
| Types | `frontend/src/types/<domain>.ts` |

---

## Import Aliases

| Alias | Resolves To |
|---|---|
| `@/` | `frontend/src/` |

`@/` is the **only** configured alias (`frontend/vite.config.ts`). There is no
`~types/`. Relative imports (`../lib/api`) are also common in `hooks/`.

---

## Common Imports

```typescript
import { useState, useCallback, useMemo, lazy } from 'react';

// UI component library
import { Button, Input, Card, CardBody, Badge, Alert, Spinner } from '@/components/ui';

// TanStack Query
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';

// React Router 7
import { useNavigate, useParams, Navigate, Outlet } from 'react-router-dom';

// API client — note: `apiClient`, not `api`
import { apiClient } from '@/lib/api';

// Contexts
import { useAuth } from '@/contexts/AuthContext';
import { useToast } from '@/contexts/ToastContext';

import type { Game, Character } from '@/types/games';
```

---

## Data Fetching

**Primary pattern: `useQuery`.** It is used in ~54 files; `useSuspenseQuery`
appears in exactly one (`components/ActiveSessions.tsx`) and is not the house
style. Do not convert existing code to Suspense fetching.

Hooks live in `frontend/src/hooks/` and call through `apiClient`:

```typescript
export function useUnreadCommentIDs(gameId?: number) {
  return useQuery({
    queryKey: ['unreadCommentIDs', gameId],
    queryFn: async () => {
      if (!gameId) throw new Error('Game ID required');
      const response = await apiClient.messages.getUnreadCommentIDs(gameId);
      return response.data;
    },
    enabled: !!gameId,
    staleTime: 5 * 60 * 1000,
  });
}
```

Mutations invalidate the affected query keys explicitly:

```typescript
export function useRenameCharacter() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ characterId, name }: { characterId: number; name: string }) =>
      apiClient.characters.renameCharacter(characterId, { name }),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['character', variables.characterId] });
    },
  });
}
```

**[📖 resources/data-fetching.md](resources/data-fetching.md)** — ⚠️ built around
`useSuspenseQuery`; treat its Suspense guidance as inapplicable.

---

## Loading & Error States

Because fetching is `useQuery`-based, components **do** branch on `isLoading` and
`error`. Use `<Spinner>` and `<Alert>` from the UI library. Route-level code
splitting uses `lazy()` + `<Suspense fallback={<PageLoader />}>` in `App.tsx`.

User feedback goes through `useToast()` from `@/contexts/ToastContext`, or the
`<Alert>` component for inline messages. `react-toastify` is not a dependency.

**[📖 resources/loading-and-error-states.md](resources/loading-and-error-states.md)**
— ⚠️ assumes Suspense fetching and a nonexistent `SuspenseLoader`.

---

## Styling

- Use `@/components/ui` for standard elements
- Tailwind **v4** (CSS-first: `@import "tailwindcss"` + `@theme` in
  `src/index.css`). There is **no `tailwind.config.js`**.
- Colors always via semantic tokens:

```tsx
<div className="surface-base text-content-primary">   // ✅
<div className="bg-white dark:bg-gray-800">           // ❌
```

Use the `surface-*` / `text-content-*` / `border-theme-*` families. A second,
older set of names (`bg-bg-*`, `border-border-*`) is **retired**: those tokens are
declared in `@theme` but were never assigned values, so Tailwind emits **no CSS
rule at all** and elements using them render with no background or border. They
were removed from app code on 2026-08-26 (~180 usages across ~50 files, including
invisible skeleton loaders, poll result bars, and active-tab underlines).

| Use | Not (retired — renders nothing) |
|---|---|
| `surface-base` | `bg-bg-primary` |
| `surface-raised` | `bg-bg-secondary` |
| `surface-sunken` | `bg-bg-tertiary`, `bg-bg-input` |
| `border-theme-default` | `border-border-primary`, `border-border-default` |
| `border-theme-strong` | `border-border-secondary`, `border-border-input` |
| `bg-interactive-primary` | `bg-primary`, `bg-accent-primary` |
| `text-semantic-danger` | `text-danger`, `text-danger-text` |
| `text-content-primary` | `text-text-heading` |
| `text-content-secondary` | `text-text-primary`, `text-text-secondary` |
| `text-content-tertiary` | `text-text-muted`, `text-text-tertiary` |

The `text-text-*` family was also retired (2026-08-26) and its classes deleted
from `index.css`. It was the worst of the set: `text-text-primary` resolved to
`--color-content-secondary` — *not* `-primary` — so the name misrepresented the
color it produced, and `text-text-muted`/`-disabled` referenced variables no
theme assigns, silently inheriting instead of rendering. The renames above are
exact no-ops: the deleted `--color-text-*` values were byte-identical to their
`--color-content-*` counterparts in all five themes.

**Opacity modifiers do not work** on these classes: use
`bg-interactive-primary-subtle`, never `bg-interactive-primary/10`.

**Authoritative list:** `frontend/src/components/ui/README.md`. Verify any new
token against the built CSS — declaration alone does not mean it renders.

**[📖 resources/styling-guide.md](resources/styling-guide.md)** — accurate.
The fuller reference is **`frontend/src/components/ui/README.md`** (verified
token list + every exported UI component); prefer it when they disagree.

---

## Routing

React Router **7**, using the data-router API. Routes are objects passed to
`createBrowserRouter` in `App.tsx` and rendered via `<RouterProvider>` — **not**
`<Routes>`/`<Route>` JSX. Pages are `lazy()`-imported for code splitting.

```typescript
const router = createBrowserRouter([
  { path: '/', element: <RootLayout />, children: [ /* ... */ ] },
]);
```

Read `App.tsx` for the current route table — there is no separate routing guide.

---

## TypeScript

Strict mode, no `any`, `import type` for type-only imports, prop interfaces.

**[📖 resources/typescript-standards.md](resources/typescript-standards.md)** — accurate.

---

## Performance

`useMemo` for expensive computation, `useCallback` for handlers passed to
children, `React.memo` for expensive components, cleanup in `useEffect`.

**[📖 resources/performance.md](resources/performance.md)** — patterns are sound;
its examples use `React.FC` and occasionally Suspense fetching.

---

## Resource Accuracy

The surviving `resources/` deep dives predate this rewrite (Oct 2025). Four were
deleted outright as unsalvageable — `file-organization.md` (entirely
`features/`-based), `routing-guide.md` (React Router v6 `<Routes>` JSX),
`common-patterns.md` (react-hook-form, Zod, Zustand, DataGrid — none installed),
and `complete-examples.md` (`features/` + Suspense + MUI). For the rest:

| Resource | Status |
|---|---|
| `styling-guide.md` | ✅ Accurate |
| `typescript-standards.md` | ✅ Accurate |
| `performance.md` | ⚠️ Sound patterns, dated examples |
| `component-patterns.md` | ⚠️ `SuspenseLoader` (nonexistent), default-export rule |
| `data-fetching.md` | ⚠️ `useSuspenseQuery`-first, `features/` API layer |
| `loading-and-error-states.md` | ⚠️ "no early returns" rule contradicts real code |

When a resource conflicts with this file or with the codebase, the codebase wins.

---

## Core Principles

1. **Read neighboring code first** — conventions vary; match the file you're in
2. **`useQuery` is the fetching pattern**, not Suspense
3. **Named exports** for components
4. **UI library first**, Tailwind for layout, semantic tokens for all color
5. **`@/` alias** for src imports
6. **Co-locate tests** as `ComponentName.test.tsx`

---

## Component Template

```typescript
import { useCallback, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Card, CardBody, Button, Spinner, Alert } from '@/components/ui';
import { apiClient } from '@/lib/api';

interface MyComponentProps {
  id: number;
  onAction?: () => void;
}

export const MyComponent = ({ id, onAction }: MyComponentProps) => {
  const [isOpen, setIsOpen] = useState(false);

  const { data, isLoading, error } = useQuery({
    queryKey: ['thing', id],
    queryFn: async () => (await apiClient.games.getGame(id)).data,
  });

  const handleAction = useCallback(() => {
    setIsOpen(true);
    onAction?.();
  }, [onAction]);

  if (isLoading) return <Spinner size="md" />;
  if (error) return <Alert variant="danger">Failed to load.</Alert>;

  return (
    <Card variant="default" padding="md">
      <CardBody>
        <p className="text-content-primary">{data?.name}</p>
        <Button variant="primary" onClick={handleAction}>Open</Button>
      </CardBody>
    </Card>
  );
};
```

---

## Related Skills

- **backend-dev-guidelines** — the API this frontend consumes
- **testing-patterns** — component and E2E test rules
