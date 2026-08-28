# Developer Onboarding

**This guide has moved.**

👉 **[docs-site/developer/getting-started/onboarding.md](../../docs-site/developer/getting-started/onboarding.md)**

That copy is maintained and verified against the current containerized dev
stack. The version that used to live here described a host-based setup (host Go
and Node prerequisites) and referenced 11 `just` recipes that no longer exist —
`just dev`, `just db_up`, `just make_migration`, `just test-frontend`, and
others. Following it would fail at the first step.

## The 30-second version

```bash
just dev-setup   # first time: create .env, build images, start the stack
just up          # subsequently
just ps          # confirm healthy
```

Frontend `http://localhost:5173` · backend `http://localhost:3000` ·
Postgres `localhost:5432`.

Run `just dev-help` for the full cheatsheet.
