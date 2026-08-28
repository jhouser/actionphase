# Testing Guide

This guide explains how to run the different types of tests in the ActionPhase backend.

**Last Verified**: August 2026

> Tests run **inside the backend container**. There is no host PostgreSQL to
> install and no separate test-database setup step: `just test`,
> `just test-integration`, `just test-coverage`, and `just test-race` each
> prepare the migrated test template automatically. Each test **package** then
> clones its own database from that template, so packages run in parallel.

## Quick Start

### Fast Unit Tests (No Database Required)
```bash
# Run only mock-based tests - fastest option
just test-mocks
```

### Full Test Suite (Requires Database)
```bash
just up      # stack must be running
just test    # prepares the test template, then runs everything
```

## Test Types

### 1. Mock Tests 🚀 **FASTEST**
- **Runtime**: < 1 second
- **Requirements**: None
- **Purpose**: Unit testing service logic with mocked dependencies

```bash
just test-mocks
```

These tests use in-memory mocks and don't require any external dependencies. Perfect for:
- TDD/rapid development
- CI/CD environments
- Local development without database setup

### 2. Integration Tests 🐢 **COMPREHENSIVE**
- **Runtime**: Several seconds
- **Requirements**: PostgreSQL database
- **Purpose**: Testing full request/response flows

```bash
just test-integration
```

These tests use a real database and test the full stack including:
- HTTP endpoints
- Database operations
- Authentication flows
- Data persistence

### 3. All Tests 🔄 **COMPLETE**
- **Runtime**: Varies based on database setup
- **Requirements**: PostgreSQL database (optional - skips DB tests if unavailable)
- **Purpose**: Complete test coverage

```bash
just test              # All tests (sequential)
```

## Database Setup

### Option 1: Local PostgreSQL
```bash
just up     # starts db + backend + frontend
```

The test database and its migrated template are created for you by the test
recipes. To rebuild them after a bad migration or dirty state:

```bash
just reset-test-db
```

### Skipping Database Tests
```bash
just test-mocks     # preferred: mock-only suite, no DB
# or set the env var directly when invoking go test in-container:
export SKIP_DB_TESTS=true
just test
```

## Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `SKIP_DB_TESTS` | `false` | Skip all tests requiring database |
| `TEST_DATABASE_URL` | `postgres://postgres:example@localhost:5432/actionphase_test?sslmode=disable` | Database connection string |

## Test Commands Reference

| Command | Description | Speed | Database Required |
|---------|-------------|-------|-------------------|
| `just test-mocks` | Mock-based unit tests only | ⚡ Fastest | ❌ No |
| `just test-integration` | Integration tests only | 🐢 Slow | ✅ Yes |
| `just test` | All tests (sequential) | 🐢 Slow | ⚠️ Optional |
| `just test-coverage` | Tests with coverage report | 🐢 Slow | ⚠️ Optional |
| `just test-race` | Tests with race condition detection | 🐢 Slow | ⚠️ Optional |
| `just test-clean` | Clean test cache and artifacts | ⚡ Fast | ❌ No |
| `just test-run <pattern>` | Run specific test by pattern | ⚡ Fast | ⚠️ Optional |

## Continuous Integration

For CI/CD environments, we recommend:

### Fast CI (PR checks)
```bash
just test-mocks  # Run in ~1 second
```

### Full CI (pre-merge)
```bash
just ci-test   # lint + test + race
```

## Troubleshooting

### Tests Failing with Database Errors
```
ERROR: database "actionphase_test" does not exist
```

**Solutions:**
1. Rebuild the test DB + template: `just reset-test-db`
2. Confirm the stack is up: `just ps`
3. Use mock tests only: `just test-mocks`

### Tests Hanging or Running Slowly
```bash
# Use mock tests for faster feedback
just test-mocks

# Or run specific tests
just test-run TestNamePattern
```

### Connection Refused Errors
```bash
just ps           # is the db container up?
just db up        # start just the database
just dev-logs db  # inspect its logs
```

## Writing Tests

### Mock-Based Unit Tests
```go
func TestMyService_WithMocks(t *testing.T) {
    t.Parallel() // Always enable parallel execution

    // Use in-memory mocks
    mockRepo := CreateMockDatabaseRepo()

    // Test your service logic
    // ...
}
```

### Integration Tests
```go
func TestMyService_Integration(t *testing.T) {
    t.Parallel() // Always enable parallel execution

    // Use real database (automatically skipped if unavailable)
    testDB := NewTestDatabase(t)
    defer testDB.Close()
    defer testDB.CleanupTables(t)

    // Use test data factories
    factory := NewTestDataFactory(testDB, t)
    user := factory.NewUser().WithUsername("testuser").Create()

    // Test with real database
    // ...
}
```

## Best Practices

1. **Always add `t.Parallel()`** to enable concurrent test execution
2. **Use mocks for unit tests** - faster and more reliable
3. **Use real database for integration tests** - catches real issues
4. **Clean up after tests** - use `defer testDB.CleanupTables(t)`
5. **Make tests independent** - don't rely on execution order
6. **Use factories for test data** - reduces boilerplate and ensures consistency
