import { readFileSync } from 'fs';
import { dirname, resolve } from 'path';
import { fileURLToPath } from 'url';

/**
 * Comment threading depth limits, as the running app sees them.
 *
 * Specs must NOT hardcode depth targets ("Nested Reply Level 4"): those are
 * really `COMMENT_MAX_DEPTH - 1` written as a literal, so they silently break
 * whenever the env vars are retuned — which is the friction this helper removes.
 *
 * Why parse `frontend/.env` instead of importing `@/config/comments`:
 *   - The app module reads `import.meta.env`, which Vite populates at BUILD time.
 *     Playwright specs run in plain Node, where `import.meta.env` is undefined —
 *     importing it would silently yield the 5/3 defaults regardless of the real
 *     configured values, defeating the point.
 *   - The `@/` alias is not configured for e2e (tsconfig.app.json includes only
 *     `src`), and that module pulls in app runtime code (loglevel) besides.
 *
 * This reads the same file Vite does, so it tracks whatever the frontend was
 * actually built/served with. Values must mirror `src/config/comments.ts`.
 */

const DEFAULT_MAX_DEPTH = 5;
const DEFAULT_MAX_DEPTH_MOBILE = 3;

function readFrontendEnv(): Record<string, string> {
  // Specs run as ESM, where __dirname is undefined — derive it from import.meta.url.
  // This file lives at frontend/e2e/fixtures/, so frontend/.env is two levels up.
  const here = dirname(fileURLToPath(import.meta.url));
  const envPath = resolve(here, '../../.env');
  let raw: string;
  try {
    raw = readFileSync(envPath, 'utf-8');
  } catch {
    // No .env (fresh checkout / CI using pure defaults) — fall back to defaults.
    return {};
  }

  const vars: Record<string, string> = {};
  for (const line of raw.split('\n')) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith('#')) continue;
    const eq = trimmed.indexOf('=');
    if (eq === -1) continue;
    const key = trimmed.slice(0, eq).trim();
    // Strip inline comments and surrounding quotes, matching dotenv's behavior
    // closely enough for the numeric values we care about here.
    const value = trimmed
      .slice(eq + 1)
      .split('#')[0]
      .trim()
      .replace(/^["']|["']$/g, '');
    vars[key] = value;
  }
  return vars;
}

function parseDepth(value: string | undefined, fallback: number): number {
  const parsed = parseInt(value ?? '', 10);
  return Number.isNaN(parsed) ? fallback : parsed;
}

const env = readFrontendEnv();

export const COMMENT_MAX_DEPTH = parseDepth(
  env.VITE_COMMENT_MAX_DEPTH,
  DEFAULT_MAX_DEPTH
);

export const COMMENT_MAX_DEPTH_MOBILE = parseDepth(
  env.VITE_COMMENT_MAX_DEPTH_MOBILE,
  DEFAULT_MAX_DEPTH_MOBILE
);

/**
 * The depth limit in effect for the current viewport. Mirrors the app, which
 * renders both subtrees and hides one via Tailwind `md:` classes (breakpoint
 * 768px) rather than checking the viewport in JS.
 */
function maxDepthForViewport(isMobile: boolean): number {
  return isMobile ? COMMENT_MAX_DEPTH_MOBILE : COMMENT_MAX_DEPTH;
}

/**
 * Names of the fixture comments at the depth boundary, derived from config.
 *
 * The COMMON_ROOM_DEEP_NESTING fixture seeds a chain named "Nested Reply Level N"
 * (1-indexed) deep enough to exceed any valid limit. Comments render at depths
 * 0..(maxDepth - 1); the deepest *inline* one is therefore "Level (maxDepth - 1)",
 * and the first comment hidden behind "Continue this thread" is "Level maxDepth".
 */
export function deepNestingTargets(isMobile: boolean): {
  deepestInline: string;
  behindContinueButton: string;
} {
  const maxDepth = maxDepthForViewport(isMobile);
  return {
    deepestInline: `Nested Reply Level ${maxDepth - 1}`,
    behindContinueButton: `Nested Reply Level ${maxDepth}`,
  };
}
