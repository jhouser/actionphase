# ActionPhase Frontend

React 19 + TypeScript SPA built with Vite, Tailwind CSS 4, and TanStack Query,
talking to the ActionPhase Go backend.

## Quick Start

Local development is **fully containerized** — you do not need Node on the host.
From the repository root:

```bash
just up        # Start db + backend + frontend
just ps        # Confirm healthy
```

The frontend is served at **http://localhost:5173** with Vite HMR; edit a file
on the host and the container picks it up.

```bash
just test-fe run       # Run frontend tests
just test-fe watch     # Watch mode
just lint-frontend     # ESLint
just type-check        # tsc -b
```

> Avoid running `npm` on the host. `node_modules` is managed inside the
> container, and a host install can clobber platform-specific binaries. If you
> must, see `just relock-frontend`.

## Authentication

Auth uses a **single JWT** delivered as an HTTP-only cookie — there is no
separate refresh token. `GET /api/v1/auth/refresh` re-issues that same cookie;
the axios interceptor in `src/lib/api/client.ts` calls it on 401 and queues
concurrent requests so only one refresh runs at a time.

Sessions are server-side: the backend revalidates the session on every request,
so revocation is immediate regardless of token lifetime.

## Architecture

- **React Router 7** — client-side routing
- **TanStack Query 5** — server state (note: v5 object-form APIs)
- **Axios** — HTTP with the JWT/refresh interceptor
- **Tailwind CSS 4** — CSS-first `@theme` config; there is no `tailwind.config.js`
- **Vitest + Playwright** — component and E2E tests

### Project Structure

```
src/
├── components/     # UI components (see components/ui/README.md for the library)
├── contexts/       # AuthContext, GameContext, ThemeContext, …
├── hooks/          # Custom hooks
├── lib/api/        # API client, split per domain
├── pages/          # Route-level components
├── services/       # Non-HTTP service helpers
├── types/          # Shared TypeScript types
└── utils/          # Utilities
```

E2E tests live in `frontend/e2e/` — see its `README.md`.

## Styling

⚠️ **Use the UI component library** (`@/components/ui`) rather than raw HTML
elements, so dark mode works. See
[`src/components/ui/README.md`](src/components/ui/README.md) for the full API
reference and design-token list.
