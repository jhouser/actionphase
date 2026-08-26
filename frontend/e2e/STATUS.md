# E2E Testing Status

**Last verified**: 2026-08-26 (counts generated from the tree, not hand-maintained)

## Current Coverage

**345 E2E tests across 49 spec files**, plus **26 page objects**.

| Area | Spec files | Tests |
|---|---|---|
| `admin/` | 1 | 6 |
| `auth/` | 2 | 18 |
| `characters/` | 5 | 23 |
| `common-room/` | 2 | 11 |
| `edge-cases/` | 2 | 16 |
| `gameplay/` | 13 | 82 |
| `games/` | 8 | 80 |
| `messaging/` | 10 | 67 |
| `notifications/` | 2 | 10 |
| `security/` | 1 | 15 |
| `settings/` | 2 | 11 |
| `smoke/` | 1 | 6 |
| **Total** | **49** | **345** |

Regenerate these numbers with:

```bash
find frontend/e2e -name '*.spec.ts' | wc -l
grep -rhoE '^\s*test(\.\w+)?\(' frontend/e2e --include='*.spec.ts' | wc -l
```

## Projects

Two Playwright projects run the same specs against different viewports:

| Project | Recipe | Device |
|---|---|---|
| `chromium` | `just e2e-desktop` | Desktop Chrome |
| `mobile-chrome` | `just e2e-mobile` | Pixel 5 |

`just e2e` runs both **sequentially** — they share fixture data, so running them
concurrently causes interference.

## Infrastructure

- **Page objects** — 26 in `e2e/pages/`; see `e2e/pages/README.md`. POM-first is
  a hard rule: no inline selectors in specs.
- **Fixtures** — per-worker E2E fixtures loaded by `just load-e2e`
  (`apply_e2e_worker.sh`, workers 0–5, game ID offset `worker * 10000`).
  Resolve game IDs by title via `getFixtureGameId()`, never hardcode them.
- **Auth helpers** — `loginAs(page, 'GM' | 'PLAYER_1' | ...)` in
  `e2e/fixtures/auth-helpers.ts`.
- **Containerized** — tests run in a one-shot `playwright` compose service
  (`--profile e2e`) that carries its own browsers.

## Known Gaps

- **Test tags are barely applied.** `e2e/fixtures/test-tags.ts` declares 14
  tags, but only **2 of 49** spec files use them (via the `tagTest()` helper):
  `smoke/health-check.spec.ts` and `auth/registration.spec.ts`. `--grep "@smoke"`
  selects 6 tests; `@game`, `@character`, and `@message` select none.
- **`tags.REGRESSION` is undefined.** `auth/registration.spec.ts:53` references
  it, but `test-tags.ts` never defines it. `tagTest()` filters the resulting
  `undefined`, so the test name silently renders with a double space
  (`@auth  registration errors…`) instead of a second tag. Either add
  `REGRESSION` to `test-tags.ts` or drop it from that call.
- **Mobile project instability.** Adding the Pixel 5 project surfaced a large
  batch of mobile-specific failures; treat `just e2e-mobile` as less stable than
  `just e2e-desktop`.
- **No visual regression tests.** The `e2e/visual/` suite was removed in
  `a439acc0` (2025-11-01). The `test:e2e:visual` npm scripts in
  `frontend/package.json` still point at the deleted directory and will fail.

## Related Documentation

- `e2e/README.md` — complete E2E guide
- `e2e/pages/README.md` — page object reference
- `docs-site/developer/testing/E2E_QUICK_START.md` — quick reference
- `docs-site/developer/testing/E2E_FIXTURES.md` — fixture reference
- `.claude/context/TESTING.md` — testing philosophy
