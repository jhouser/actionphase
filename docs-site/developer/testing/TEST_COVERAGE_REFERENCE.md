# Test Coverage Reference

**Single source of truth for test coverage metrics.** Other docs should link
here rather than duplicating numbers.

**Last measured**: 2026-08-26

## Current Metrics

| Metric | Value |
|---|---|
| Backend statement coverage | **58.4%** overall; **78–83%** in the service layer |
| Backend test files | 144 |
| Backend `Test*` functions | 1,131 |
| Frontend test files | 244 |
| Frontend tests | 3,857 passing, 14 skipped |
| E2E tests | 345 across 49 spec files |
| Pass rate | 100% |

The service-layer band (`pkg/db/services/actions|messages|phases`) is the number
to track against the >80% target. The overall 58.4% includes thin HTTP wiring,
generated code, and integration-shaped packages exercised through E2E instead.

## How to Regenerate

All commands run against the containerized stack — no host Go or Node needed.

```bash
# Backend coverage (prints the total)
just test-coverage

# Backend counts
find backend -name '*_test.go' | wc -l
grep -rhoE '^func Test[A-Za-z0-9_]+' backend --include='*_test.go' | wc -l

# Frontend
just test-fe run
find frontend/src -name '*.test.ts*' | wc -l

# E2E
find frontend/e2e -name '*.spec.ts' | wc -l
grep -rhoE '^\s*test(\.\w+)?\(' frontend/e2e --include='*.spec.ts' | wc -l
```

## Related Documentation

- **Detailed breakdown**: [COVERAGE_STATUS.md](./COVERAGE_STATUS.md)
- **Testing strategy**: [ADR-007](../architecture/adrs/007-testing-strategy.md)
- **Testing context**: `.claude/context/TESTING.md`
- **Implementation guide**: `.claude/reference/TESTING_GUIDE.md`
- **E2E coverage**: `frontend/e2e/STATUS.md`

---

*When updating metrics, re-run the commands above and update the "Last measured" date.*
