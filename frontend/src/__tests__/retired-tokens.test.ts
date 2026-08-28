import { describe, it, expect } from 'vitest';
import { readFileSync, readdirSync, statSync } from 'fs';
import { join } from 'path';

/**
 * Guard against reintroducing retired design tokens.
 *
 * These class names are declared in the `@theme` block of `src/index.css` but
 * were never assigned values in `src/lib/theme/themes.ts`. Tailwind therefore
 * emits no CSS rule for them at all, so an element using one renders with no
 * background/border — silently, in both light and dark mode.
 *
 * This caused real, shipped bugs: invisible skeleton loaders, invisible poll
 * result bars, and missing active-tab underlines. ~180 usages were removed on
 * 2026-08-26. See frontend/src/components/ui/README.md for replacements.
 */
const RETIRED = [
  'bg-bg-primary', 'bg-bg-secondary', 'bg-bg-tertiary', 'bg-bg-page',
  'bg-bg-hover', 'bg-bg-active', 'bg-bg-input', 'bg-bg-input-disabled',
  'bg-bg-page-secondary', 'bg-bg-accent-secondary',
  'border-border-primary', 'border-border-secondary', 'border-border-default',
  'border-border-input', 'border-border-focus', 'border-border-subtle',
  'border-border-strong', 'border-border-warning',
  'bg-accent-primary', 'border-accent-primary',
  'bg-primary-light', 'text-primary-text',
  'bg-danger-light', 'text-danger-text',
  'bg-warning-light', 'text-warning-text',
  'bg-success-light', 'text-success-text',
  'placeholder-placeholder',
  // The .text-text-* family, retired 2026-08-26. These were worse than dead:
  // `text-text-primary` resolved to --color-content-secondary (NOT -primary), and
  // `text-text-muted`/`-disabled` referenced variables no theme assigns, yielding
  // an invalid color that silently inherited. Use text-content-* instead.
  'text-text-heading', 'text-text-primary', 'text-text-secondary',
  'text-text-muted', 'text-text-disabled', 'text-text-tertiary',
];

function sourceFiles(dir: string, acc: string[] = []): string[] {
  for (const entry of readdirSync(dir)) {
    if (entry === 'node_modules' || entry === '__tests__') continue;
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) sourceFiles(full, acc);
    else if (/\.(tsx|ts)$/.test(entry) && !entry.includes('.test.')) acc.push(full);
  }
  return acc;
}

describe('retired design tokens', () => {
  it('are not used anywhere in src/', () => {
    const offenders: string[] = [];
    for (const file of sourceFiles(join(process.cwd(), 'src'))) {
      const text = readFileSync(file, 'utf8');
      for (const token of RETIRED) {
        // Word-boundary match so `bg-bg-primary` doesn't match a longer name.
        if (new RegExp(`(?<![\\w-])${token}(?![\\w-])`).test(text)) {
          offenders.push(`${file.replace(process.cwd() + '/', '')}: ${token}`);
        }
      }
    }
    expect(offenders).toEqual([]);
  });
});
