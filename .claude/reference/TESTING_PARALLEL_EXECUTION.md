# Safe Parallel Test Execution

> ⚠️ **HISTORICAL ARTIFACT.** Records the **October 2025** work that first made
> parallel test execution safe. Accurate as history — the named tests still
> exist — but it is not the current model: tests now achieve isolation by having
> each **package clone its own database** from `actionphase_test_template`.
> For current practice see `.claude/context/TESTING.md`.

This document outlines the improvements made to enable safe parallel test execution in the ActionPhase backend.

## Overview

Parallel test execution dramatically improves CI/CD performance by running tests concurrently. However, it requires careful attention to test isolation and state management to avoid race conditions and flaky tests.

## Changes Made

### 1. Fixed Test State Dependencies ✅

**Problem**: The original `TestAuthFlow_CompleteWorkflow` test had shared state variables (`accessToken`, `refreshToken`) that created dependencies between subtests.

**Solution**: Refactored into independent test functions:
- `TestAuthFlow_Registration`
- `TestAuthFlow_Login`
- `TestAuthFlow_ProtectedEndpointAccess`
- `TestAuthFlow_TokenRefresh`

Each test now creates its own complete setup including user registration when needed.

**File**: `pkg/auth/auth_integration_test.go`

### 2. Repository Interfaces for Mocking ✅

**Problem**: Integration tests were slow because they required real database connections.

**Solution**: Created repository interfaces and mock implementations:
- `UserRepository`, `GameRepository`, `SessionRepository`, `GameParticipantRepository` interfaces
- Function-based mocks (`MockUserRepository`, etc.) for flexible behavior
- Simple mocks (`SimpleMockUserRepository`, etc.) for common scenarios
- Compile-time interface verification

**Files**:
- `pkg/core/repositories.go` - Interface definitions
- `pkg/core/repository_mocks.go` - Function-based mocks
- `pkg/core/repository_mocks_simple.go` - Simple mocks with default behavior
- `pkg/core/repository_mocks_example_test.go` - Usage examples

### 3. Test Data Factories ✅

**Problem**: Creating test data was repetitive and error-prone.

**Solution**: Implemented builder pattern factories:
- `TestDataFactory` with fluent builder interfaces
- `UserBuilder`, `GameBuilder`, `SessionBuilder`, `GameParticipantBuilder`
- Convenience methods for common scenarios
- Batch creation methods for performance testing

**Files**:
- `pkg/core/test_factories.go` - Factory implementations
- `pkg/core/test_factories_example_test.go` - Usage examples

### 4. Parallel Execution Enablement ✅

**Problem**: Tests ran sequentially, leading to slow CI/CD pipelines.

**Solution**: Added `t.Parallel()` calls throughout test suites with proper isolation:
- Each test/subtest gets its own database connection
- Independent test data creation per test
- Proper cleanup scoped to individual tests
- Loop variable capture for parallel subtests

## How to Write Parallel-Safe Tests

### Basic Pattern

```go
func TestMyFeature(t *testing.T) {
    t.Parallel() // Enable parallel execution

    // Create isolated test setup
    testDB := NewTestDatabase(t)
    defer testDB.Close()
    defer testDB.CleanupTables(t)

    // Test logic here...
}
```

### Subtest Pattern

```go
func TestMyFeature_MultipleScenarios(t *testing.T) {
    t.Parallel() // Enable parallel execution for main test

    testCases := []struct{
        name string
        // ... test case fields
    }{
        // ... test cases
    }

    for _, tc := range testCases {
        tc := tc // Capture loop variable!
        t.Run(tc.name, func(t *testing.T) {
            t.Parallel() // Enable parallel execution for subtest

            // Each subtest gets independent setup
            testDB := NewTestDatabase(t)
            defer testDB.Close()
            defer testDB.CleanupTables(t)

            // Test logic using tc...
        })
    }
}
```

### Using Test Data Factories

```go
func TestWithFactories(t *testing.T) {
    t.Parallel()

    testDB := NewTestDatabase(t)
    defer testDB.Close()
    defer testDB.CleanupTables(t)

    factory := NewTestDataFactory(testDB, t)

    // Create test data using fluent builders
    user := factory.NewUser().
        WithUsername("testuser").
        WithEmail("test@example.com").
        AsAdmin().
        Create()

    game := factory.NewGame().
        WithTitle("Test Game").
        WithGM(user.ID).
        WithMaxPlayers(4).
        Create()

    // Test logic...
}
```

### Using Repository Mocks

```go
func TestWithMocks(t *testing.T) {
    t.Parallel() // Fast tests with no database needed

    // Simple mock with default behavior
    mockRepo := CreateMockDatabaseRepo()

    // Or function-based mock for custom behavior
    mockUserRepo := &MockUserRepository{
        GetUserFn: func(ctx context.Context, id int32) (db.User, error) {
            if id == 999 {
                return db.User{}, errors.New("user not found")
            }
            return db.User{ID: id, Username: "testuser"}, nil
        },
    }

    // Test your service layer with mocks...
}
```

## Performance Benefits

### Before Improvements
- Tests ran sequentially
- All tests used real database connections
- Shared test state caused dependencies
- Typical test suite: ~30-60 seconds

### After Improvements
- Tests run in parallel with `go test -parallel N`
- Unit tests use fast in-memory mocks
- Integration tests are properly isolated
- Expected improvement: 3-5x faster test execution

## Running Tests in Parallel

```bash
# Run with default parallelism (GOMAXPROCS)
go test ./...

# Run with specific parallelism level
go test -parallel 4 ./...

# Run only fast unit tests (no database required)
go test -run "Mock|Factory" ./pkg/core

# Run integration tests (requires database)
go test -run "Integration|API" ./...
```

## Best Practices Checklist

- ✅ Each test calls `t.Parallel()`
- ✅ No shared global variables between tests
- ✅ Each test creates its own database connection
- ✅ Loop variables captured in subtests (`tc := tc`)
- ✅ Proper cleanup scoped to individual tests
- ✅ Unique test data (usernames, emails, etc.)
- ✅ Use factories for complex object creation
- ✅ Use mocks for unit testing service layers
- ✅ Database tests use transactions or cleanup

## Common Pitfalls to Avoid

1. **Shared State**: Don't use global variables or shared test data
2. **Database Conflicts**: Each test needs unique data or proper cleanup
3. **Loop Variables**: Always capture loop variables in subtests
4. **Resource Leaks**: Always defer Close() and cleanup calls
5. **Non-deterministic Tests**: Don't rely on execution order

## Migration Guide

When updating existing tests:

1. Add `t.Parallel()` to test functions
2. Move shared setup into individual tests
3. Replace global test data with factories
4. Add proper cleanup with defer statements
5. Capture loop variables in subtests
6. Convert slow integration tests to fast unit tests with mocks where appropriate

## Future Improvements

- [ ] Add more comprehensive factory methods
- [ ] Implement test database pooling for faster setup
- [ ] Add parallel benchmark tests
- [ ] Create test data seeding utilities
- [ ] Add integration with testcontainers for isolated database testing
