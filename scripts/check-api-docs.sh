#!/bin/bash

# API Documentation Consistency Check
#
# The OpenAPI spec and the Chi router independently describe the same HTTP
# surface, with nothing connecting them at compile time:
#
#   1. registered routes    backend/pkg/http/root.go (+ RegisterRoutes helpers)
#   2. documented paths     backend/pkg/docs/openapi.yaml
#
# They drift in three distinct ways, and each fails differently:
#
#   UNDOCUMENTED  a route exists but the spec omits it. Invisible to anyone
#                 working from the docs.
#   PHANTOM       the spec describes a path with no handler behind it. Worse
#                 than missing: a caller writes against it and gets a 404. Auth
#                 middleware usually turns that into a 401 first, which hides
#                 the cause.
#   UNREACHABLE   the spec's server base is /api/v1, so a route registered at
#                 the root (e.g. /ping) is documented at a URL that 404s even
#                 though the endpoint works.
#
# All three shipped to production before this check existed.
#
# This runs as a script rather than a unit test because it parses a YAML doc
# and a Go source tree together, and because CI wants a pass/fail exit code
# rather than a judgment call. `just check-api-docs` executes it inside the
# backend container, where the repo root is bind-mounted read-only at /repo.
#
# The baseline (scripts/api-docs-baseline.txt) records drift that predates the
# check. Anything in it is reported as a known gap and does not fail the build;
# anything NEW fails. Documenting a baselined route means deleting its line —
# the check tells you when a line has gone stale.
#
# Usage: just check-api-docs
#    or: ./scripts/check-api-docs.sh   (directly, on a host with bash)

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

ROUTER="$ROOT/backend/pkg/http/root.go"
SPEC="$ROOT/backend/pkg/docs/openapi.yaml"
BASELINE="$ROOT/scripts/api-docs-baseline.txt"

for f in "$ROUTER" "$SPEC"; do
    if [ ! -f "$f" ]; then
        echo -e "${RED}✗ missing required file: $f${NC}"
        exit 1
    fi
done

# Not created if absent — /repo is mounted read-only in the container. A
# missing baseline simply means "no known gaps".

python3 - "$ROUTER" "$SPEC" "$BASELINE" <<'PYEOF'
import re, sys

router_path, spec_path, baseline_path = sys.argv[1:4]

# ---------------------------------------------------------------- real routes
#
# Chi builds the tree from nested Route()/Mount() calls, so a route literal
# like r.Get("/logs", ...) says nothing about its final path on its own. We
# reconstruct it by tracking brace depth: every Route("/x") pushes a prefix
# that stays live until its closing brace, and every Mount("/y", subRouter)
# records where a named router hangs off its parent.

src = open(router_path).read().split('\n')

mounts = {}
for ln in src:
    m = re.search(r'(\w+)\.Mount\("([^"]*)",\s*(\w+)\)', ln)
    if m:
        parent, prefix, child = m.groups()
        mounts[child] = (parent, prefix)

def full_prefix(var):
    """Walk a router var up its Mount() chain to an absolute path prefix."""
    parts, seen = [], set()
    while var in mounts and var not in seen:
        seen.add(var)
        parent, prefix = mounts[var]
        parts.insert(0, prefix)
        var = parent
    if var == 'apiV1Router':
        parts.insert(0, '/api/v1')
    return ''.join(parts)

routes = []       # (METHOD, path) under a mounted router
root_routes = []  # (METHOD, path) registered directly on the root router
stack, cur_router, depth = [], None, 0

for ln in src:
    m = re.search(r'(\w+Router)\.Route\("([^"]*)"', ln)
    if m:
        cur_router = m.group(1)
        stack.append((depth, m.group(2)))
    elif re.search(r'r\.Route\("([^"]*)"', ln):
        stack.append((depth, re.search(r'r\.Route\("([^"]*)"', ln).group(1)))

    # Matches r.Get("/x", ...) and the middleware-chained form
    # r.With(mw).Get("/x", ...) — the latter is used for rate-limited and
    # optional-auth routes, and missing it reports live endpoints as phantoms.
    rt = re.search(r'\br\.(?:With\(.*\)\.)?(Get|Post|Put|Patch|Delete)\("([^"]*)"', ln)
    if rt:
        method, p = rt.group(1).upper(), rt.group(2)
        if cur_router:
            path = (full_prefix(cur_router) + ''.join(s for _, s in stack) + p)
        else:
            # Registered on the bare root router, outside /api/v1.
            path = p
        path = re.sub(r'/+', '/', path)
        if len(path) > 1:
            path = path.rstrip('/')
        (routes if cur_router else root_routes).append((method, path))

    depth += ln.count('{') - ln.count('}')
    while stack and stack[-1][0] >= depth:
        stack.pop()

# ------------------------------------------------------------ documented spec
# An operation may override the global server base with its own `servers:`
# block (OpenAPI 3). /ping uses this because it is registered at the root
# rather than under /api/v1. Such an operation is documented at its real URL,
# so it must be compared against the root routes, not the /api/v1 ones —
# otherwise the correct spec gets reported as unreachable.
spec = open(spec_path).read().split('\n')
documented, doc_root, path, op = [], [], None, None
op_indent_re = re.compile(r'^    (get|post|put|patch|delete):\s*$')

for idx, ln in enumerate(spec):
    m = re.match(r'^  (/\S*):\s*$', ln)
    if m:
        path = m.group(1)
    m2 = op_indent_re.match(ln)
    if m2 and path:
        method = m2.group(1).upper()
        # Scan this operation's body for a `servers:` key at operation level
        # (6 spaces), stopping at the next operation or path.
        has_override = False
        for nxt in spec[idx + 1:]:
            if op_indent_re.match(nxt) or re.match(r'^  /\S*:\s*$', nxt):
                break
            if re.match(r'^      servers:\s*$', nxt):
                has_override = True
                break
        if has_override:
            doc_root.append((method, path))
        else:
            documented.append((method, '/api/v1' + path))

# Path params are named inconsistently across both sources ({id}, {gameID},
# {gameId}) and the name carries no routing meaning — normalise so we compare
# shapes, not spellings. Without this every parameterised path is a false hit.
def norm(mp):
    method, p = mp
    return (method, re.sub(r'\{[^}]*\}', '{}', p))

real_n  = {norm(r) for r in routes}
root_n  = {norm(r) for r in root_routes}
doc_n   = {norm(d) for d in documented}
# Operations documented at the root via a `servers:` override.
doc_rt  = {norm(d) for d in doc_root}

def fmt(mp):
    return f"{mp[0]:<6} {mp[1]}"

undocumented = sorted(real_n - doc_n, key=lambda x: (x[1], x[0]))
# A documented path with no route anywhere. A root-mounted endpoint is reported
# as UNREACHABLE below rather than as a phantom; one correctly documented with a
# `servers:` override is not a defect at all.
phantom = sorted(
    (doc_n - real_n - {(m, '/api/v1' + p) for m, p in root_n})
    | (doc_rt - root_n),
    key=lambda x: (x[1], x[0]))
# Documented under /api/v1 but actually served at the root: the endpoint works,
# the documented URL does not. An operation carrying its own `servers:` override
# already documents the real URL, so it is excluded.
unreachable = sorted({(m, p) for m, p in doc_n
                      if (m, p.replace('/api/v1', '', 1)) in root_n},
                     key=lambda x: (x[1], x[0]))

baseline = set()
try:
    with open(baseline_path) as fh:
        for ln in fh:
            ln = ln.split('#')[0].strip()
            if ln:
                parts = ln.split()
                if len(parts) == 2:
                    baseline.add((parts[0], parts[1]))
except FileNotFoundError:
    pass

def split_known(items):
    new = [i for i in items if i not in baseline]
    known = [i for i in items if i in baseline]
    return new, known

new_undoc, known_undoc = split_known(undocumented)
new_phantom, known_phantom = split_known(phantom)
new_unreach, known_unreach = split_known(unreachable)

RED, GREEN, YELLOW, NC = '\033[0;31m', '\033[0;32m', '\033[0;33m', '\033[0m'
fail = 0

def report(title, items, hint):
    global fail
    if not items:
        return
    fail = 1
    print(f"{RED}✗ {len(items)} {title}{NC}")
    for i in items:
        print(f"    {fmt(i)}")
    print(f"  {hint}\n")

report("route(s) not in the OpenAPI spec",
       new_undoc,
       "Add a path entry to backend/pkg/docs/openapi.yaml.")
report("documented path(s) with no handler",
       new_phantom,
       "Remove from the spec, or implement the route. Auth middleware makes\n"
       "  these return 401 rather than 404, which hides the cause.")
report("documented path(s) unreachable at the spec's base URL",
       new_unreach,
       "The spec's server base is /api/v1 but these are registered at the\n"
       "  root. Document the real URL or move the route under /api/v1.")

# Baselined entries that are now clean: the line is stale and should go.
stale = [b for b in baseline
         if b not in undocumented and b not in phantom and b not in unreachable]
if stale:
    print(f"{YELLOW}! {len(stale)} baseline entr(ies) no longer drifting{NC}")
    for s in sorted(stale, key=lambda x: (x[1], x[0])):
        print(f"    {fmt(s)}")
    print("  Fixed since baselining — delete these lines from")
    print("  scripts/api-docs-baseline.txt to lock the improvement in.\n")

known_total = len(known_undoc) + len(known_phantom) + len(known_unreach)
if fail:
    print(f"API docs are out of sync with {router_path.split('/')[-1]}.")
    if known_total:
        print(f"({known_total} pre-existing gap(s) ignored via the baseline.)")
    sys.exit(1)

if known_total:
    print(f"{GREEN}✓ no new API doc drift{NC}")
    print(f"  {len(doc_n) + len(doc_rt)} documented, "
          f"{len(real_n) + len(root_n)} registered, "
          f"{known_total} known gap(s) in the baseline")
else:
    print(f"{GREEN}✓ API docs in sync — {len(doc_n) + len(doc_rt)} paths "
          f"documented, {len(real_n) + len(root_n)} registered{NC}")
PYEOF
