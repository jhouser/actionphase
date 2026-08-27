# GitHub Actions CI/CD

This directory contains GitHub Actions workflows for automated pre-merge checks.

## Workflows

Three workflows live here: `ci.yml`, `e2e.yml`, and `nightly.yml`.

### CI Workflow (`ci.yml`)

Runs on pushes and pull requests to `master`, `develop`, and `go_rewrite`.

**Jobs:**

1. **backend-lint** — `gofmt` check and `go vet`. Go 1.25.0.
2. **backend-test** — PostgreSQL **17** service container; creates the test
   database, runs migrations, then mock tests, integration tests, and the race
   detector. Go 1.25.0.
3. **frontend-lint** — ESLint. Node **24**.
4. **frontend-test** — Vitest unit tests. Node 24.
5. **build-check** — runs after the others; builds backend (`go build`) and
   frontend (`npm run build`).
6. **upload-sourcemaps** — uploads frontend source maps to Grafana Faro.
   Gated on `github.ref == 'refs/heads/master' && github.event_name == 'push'`,
   so it does not run on pull requests.

> **On the type check:** `frontend-lint` runs `npx tsc -b --force`, **not**
> `tsc --noEmit`. `frontend/tsconfig.json` is a solution-style file
> (`"files": []` plus project references), so a bare `--noEmit` type-checks
> **zero files** and exits 0 even with type errors present. `-b` walks the
> referenced projects, matching `just type-check` and `npm run build`.
> Fixed 2026-08-27 — CI had been passing vacuously.

### E2E Workflow (`e2e.yml`)

**Manual trigger only** (`workflow_dispatch`) — E2E tests are expected to run
locally before merging. Accepts a `test_suite` input (`all`, `smoke`,
`critical`, `auth`, `game`, `character`, `message`).

> The non-`all` choices filter by Playwright tag, and **almost no specs carry
> tags** — only `smoke/health-check.spec.ts` and `auth/registration.spec.ts` use
> the `tagTest()` helper. `game`, `character`, and `message` currently select
> zero tests. See `frontend/e2e/STATUS.md`.

### Nightly Workflow (`nightly.yml`)

Scheduled at **04:00 UTC daily**, plus `workflow_dispatch`. Runs the
`race-detector` job.

## Caching

The workflow uses caching to speed up builds:
- **Go modules**: Cached via `setup-go` action (backend/go.sum)
- **npm packages**: Cached via `setup-node` action (frontend/package-lock.json)

## What's NOT Included

Handled locally, not in CI:
- Docker image builds
- Deployments
- Coverage reports

**Note**: E2E tests are **manual-only** (`workflow_dispatch`) — they do *not*
run automatically on push to `master`. `e2e.yml` does upload the Playwright
report as an artifact when it runs.

## Local Testing

Reproduce CI locally against the containerized stack:

```bash
just verify            # lint + type-check + builds (the full pre-push gate)
just verify-quick      # non-mutating checks only (~8-11s)

just fmt-check         # gofmt
just vet               # go vet
just test              # backend tests
just test-fe run       # frontend tests
just lint-frontend     # ESLint
just type-check        # tsc -b  (note: NOT --noEmit)
```

## Workflow Triggers

| Workflow | Trigger |
|---|---|
| `ci.yml` | push + PR to `master`, `develop`, `go_rewrite` |
| `e2e.yml` | manual only (`workflow_dispatch`) |
| `nightly.yml` | cron `0 4 * * *` (04:00 UTC) + manual |

## Job Dependencies

```
backend-lint ──┐
backend-test ──┤
frontend-lint ─┼──> build-check ──> upload-sourcemaps
frontend-test ─┘                    (master push only)
```

All lint and test jobs run in parallel. `build-check` runs only if they all
succeed; `upload-sourcemaps` runs only on a push to `master`.

## Troubleshooting

### Workflow fails on format check
Run `just fmt` locally to format Go code.

### Workflow fails on vet
Run `just vet` locally to see vet issues.

### Workflow fails on ESLint
Run `just lint-frontend` locally to see linting errors.

### Workflow fails on TypeScript check
Run `just type-check` locally (it uses `tsc -b`). Do **not** use
`tsc --noEmit` against the root tsconfig — it checks zero files and always passes.

### Workflow fails on tests
Run `just test` (backend) or `just test-fe run` (frontend) locally.

### Workflow fails on migration
Ensure all migration files in `backend/pkg/db/migrations/` are valid and run in order.
