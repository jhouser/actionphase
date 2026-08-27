# ActionPhase `docs/` — Operations & Deployment

This directory holds **deployment, operations, and long-form feature docs** that
are not part of the published documentation site.

> **Looking for architecture, ADRs, API reference, or testing docs?**
> Those live in **`docs-site/developer/`** and are published via VitePress
> (`just docs-dev` to preview). They are *not* in this directory — an older
> version of this index linked to `docs/architecture/` and `docs/adrs/`, which
> have not existed for some time.

## Contents

### Deployment
- **[Production Env Checklist](deployment/PRODUCTION_ENV_CHECKLIST.md)** — what
  must be set before going live, and what is (and isn't) auto-configured
- **[SSL Bootstrap Guide](deployment/SSL_BOOTSTRAP_GUIDE.md)** — certbot/nginx
  first-run and auto-renewal
- **[Route53 + SSL Setup](deployment/ROUTE53_SSL_SETUP.md)** — DNS configuration

### Operations
- **[Logging Quick Reference](operations/LOGGING_QUICK_REFERENCE.md)** — day-to-day
  log commands
- **[Logging Strategy](operations/LOGGING_STRATEGY.md)** — decision record for the
  logging approach (Option 2 shipped)

### Development
- **[API Docs Quick Start](development/API_DOCS_QUICK_START.md)**
- **[API Docs Automation](development/API_DOCS_AUTOMATION.md)** — ⚠️ note the
  broken-validator warning at the top
- **[API Docs in Production](development/API_DOCS_PRODUCTION.md)**

### Features
- **[State Management](features/STATE_MANAGEMENT.md)** — comprehensive frontend
  state guide (AuthContext, GameContext, hooks, React Query v5)

### Getting Started
- **[Developer Onboarding](getting-started/DEVELOPER_ONBOARDING.md)** — a pointer;
  the maintained guide is in `docs-site/`

## Where Everything Else Lives

| Topic | Location |
|---|---|
| Architecture, ADRs, API reference, testing | `docs-site/developer/` |
| End-user guide | `docs-site/guide/` |
| AI context & skills | `.claude/` |
| E2E testing | `frontend/e2e/README.md` |
| UI component library | `frontend/src/components/ui/README.md` |

## Quick Start

```bash
just dev-setup   # first time
just up          # start the stack
just dev-help    # full command cheatsheet
```
