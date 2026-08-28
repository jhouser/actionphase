# ADR-007: Testing Strategy

## Status
Accepted

## Context
ActionPhase requires a comprehensive testing strategy that ensures:
- Code quality and reliability across frontend and backend
- Confidence in deployments and refactoring
- Prevention of regressions in complex game logic
- Performance validation under load
- Integration reliability between components
- Developer productivity with fast feedback loops
- Maintainable test suite that scales with codebase

The strategy must balance thorough coverage with development velocity and maintenance overhead.

## Decision
We implemented a **Multi-Layer Testing Strategy** with different types of tests for different purposes:

**Backend Testing (Go)**:
- Unit Tests: Fast, isolated tests with interface mocks
- Integration Tests: Database and service integration validation
- API Tests: End-to-end HTTP API validation
- Performance Tests: Load testing and benchmarking

**Frontend Testing (React/TypeScript)**:
- Unit Tests: Component and hook testing with React Testing Library
- Integration Tests: Multi-component interaction testing
- E2E Tests: Full user journey testing with Playwright
- Visual Tests: Component visual regression testing

**Continuous Integration**:
- Automated test execution on all commits
- Parallel test execution for faster feedback
- Code coverage reporting and enforcement
- Performance regression detection

## Alternatives Considered

### 1. TDD-First Approach
**Approach**: Test-Driven Development with tests written before implementation.

**Pros**:
- Forces clear interface design upfront
- Excellent test coverage by design
- Reduces over-engineering and scope creep
- Strong confidence in code correctness

**Cons**:
- Slower initial development velocity
- Requires significant developer discipline
- Can lead to brittle tests tied to implementation
- Learning curve for teams new to TDD

### 2. Minimal Testing Strategy
**Approach**: Focus only on critical path testing with minimal test infrastructure.

**Pros**:
- Faster development with less test maintenance
- Lower barrier to entry for contributors
- Reduced CI/CD complexity and time
- Focus on shipping features quickly

**Cons**:
- High risk of regressions and production bugs
- Difficult to refactor code safely
- Manual testing becomes bottleneck
- Technical debt accumulates quickly

### 3. Property-Based Testing Focus
**Approach**: Generate test cases automatically based on property definitions.

**Pros**:
- Discovers edge cases automatically
- Excellent for testing complex business logic
- Reduces test maintenance overhead
- High confidence in algorithm correctness

**Cons**:
- Complex setup and learning curve
- Difficult to debug when tests fail
- Not suitable for all types of testing
- Limited tooling ecosystem in Go and TypeScript

### 4. Manual Testing Primary
**Approach**: Rely primarily on manual QA testing with minimal automated testing.

**Pros**:
- Flexible testing scenarios and exploration
- Good for user experience validation
- Lower upfront tooling investment
- Suitable for rapidly changing requirements

**Cons**:
- Doesn't scale with team or feature growth
- Slow feedback loop for developers
- High risk of human error and missed issues
- Expensive long-term maintenance

## Consequences

### Positive Consequences

**Code Quality**:
- Interface-driven design improves architecture
- Comprehensive test coverage prevents regressions
- Fast unit tests enable rapid development cycles
- Integration tests catch system-level issues early

**Developer Productivity**:
- Fast feedback loop with comprehensive test suite
- Confidence to refactor and optimize code
- Clear documentation of expected behavior
- Automated testing reduces manual QA overhead

**Production Reliability**:
- Reduced bug rate in production deployments
- Performance tests prevent performance regressions
- API tests ensure contract compliance
- End-to-end tests validate complete user journeys

**Team Collaboration**:
- Tests serve as executable documentation
- Interface mocks enable parallel development
- Clear testing standards improve code reviews
- Shared understanding of system behavior

### Negative Consequences

**Development Overhead**:
- Initial setup complexity for testing infrastructure
- Test maintenance effort alongside feature development
- Learning curve for testing best practices
- Increased CI/CD execution time and complexity

**Test Maintenance Burden**:
- Brittle tests require frequent updates
- Mock maintenance when interfaces change
- Test data management complexity
- False positive test failures impact productivity

**Tooling Complexity**:
- Multiple testing frameworks and tools to manage
- Environment differences between test and production
- Database testing requires careful transaction management
- Test isolation challenges with shared resources

### Risk Mitigation Strategies

**Test Reliability**:
- Implement proper test isolation with database transactions
- Use deterministic test data and avoid randomness
- Implement retry logic for flaky integration tests
- Monitor test suite performance and reliability metrics

**Maintenance Efficiency**:
- Prefer testing behavior over implementation details
- Use test helpers and utilities to reduce duplication
- Implement test data builders for complex objects
- Regular refactoring of test code alongside production code

**Performance Management**:
- Parallel test execution to minimize CI time
- Selective test execution based on changed files
- Performance budgets for test suite execution
- Regular performance profiling of slow tests

## Implementation Details

### Backend Testing Architecture

#### Unit Testing with Mocks
```go
// Service interface for mockability
type GameServiceInterface interface {
    CreateGame(ctx context.Context, game *core.Game) (*core.Game, error)
    GetGame(ctx context.Context, id int) (*core.Game, error)
}

// Mock implementation for testing
type MockGameService struct {
    CreateGameFunc func(ctx context.Context, game *core.Game) (*core.Game, error)
    GetGameFunc    func(ctx context.Context, id int) (*core.Game, error)
}

func (m *MockGameService) CreateGame(ctx context.Context, game *core.Game) (*core.Game, error) {
    if m.CreateGameFunc != nil {
        return m.CreateGameFunc(ctx, game)
    }
    return nil, errors.New("not implemented")
}

// Unit test example
func TestGameHandler_CreateGame(t *testing.T) {
    mockService := &MockGameService{
        CreateGameFunc: func(ctx context.Context, game *core.Game) (*core.Game, error) {
            return &core.Game{ID: 1, Title: game.Title}, nil
        },
    }

    handler := &games.Handler{GameService: mockService}

    reqBody := `{"title": "Test Game", "max_players": 4}`
    req := httptest.NewRequest("POST", "/api/v1/games", strings.NewReader(reqBody))
    w := httptest.NewRecorder()

    handler.CreateGame(w, req)

    assert.Equal(t, http.StatusCreated, w.Code)

    var response map[string]interface{}
    json.Unmarshal(w.Body.Bytes(), &response)
    assert.Equal(t, "Test Game", response["data"].(map[string]interface{})["title"])
}
```

#### Integration Testing with Database
```go
func TestGameService_Integration(t *testing.T) {
    // Skip if database tests disabled
    if os.Getenv("SKIP_DB_TESTS") == "true" {
        t.Skip("Skipping database integration test")
    }

    // Setup test database
    pool := testutil.SetupTestDB(t)
    defer testutil.CleanupTestDB(t, pool)

    service := &db.GameService{DB: pool}

    // Test within transaction for isolation
    testutil.WithTransaction(t, pool, func(tx pgx.Tx) {
        game := &core.Game{
            Title:      "Integration Test Game",
            MaxPlayers: 4,
            GMUserID:   1,
        }

        createdGame, err := service.CreateGame(context.Background(), game)
        require.NoError(t, err)
        assert.NotZero(t, createdGame.ID)
        assert.Equal(t, "Integration Test Game", createdGame.Title)

        // Verify game can be retrieved
        retrievedGame, err := service.GetGame(context.Background(), createdGame.ID)
        require.NoError(t, err)
        assert.Equal(t, createdGame.ID, retrievedGame.ID)
        assert.Equal(t, "Integration Test Game", retrievedGame.Title)
    })
}
```

#### API Testing
```go
func TestGameAPI_EndToEnd(t *testing.T) {
    // Setup test server with real dependencies
    app := testutil.SetupTestApp(t)
    server := httptest.NewServer(app.Handler())
    defer server.Close()

    // Create test user and get auth token
    token := testutil.CreateTestUser(t, app, "testuser", "test@example.com")

    // Test game creation
    gameData := map[string]interface{}{
        "title":       "E2E Test Game",
        "description": "End-to-end test game",
        "max_players": 4,
    }

    jsonBody, _ := json.Marshal(gameData)
    req, _ := http.NewRequest("POST", server.URL+"/api/v1/games", bytes.NewBuffer(jsonBody))
    req.Header.Set("Authorization", "Bearer "+token)
    req.Header.Set("Content-Type", "application/json")

    client := &http.Client{}
    resp, err := client.Do(req)
    require.NoError(t, err)
    defer resp.Body.Close()

    assert.Equal(t, http.StatusCreated, resp.StatusCode)

    var response map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&response)

    gameID := int(response["data"].(map[string]interface{})["id"].(float64))
    assert.NotZero(t, gameID)

    // Test game retrieval
    getReq, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/games/%d", server.URL, gameID), nil)
    getReq.Header.Set("Authorization", "Bearer "+token)

    getResp, err := client.Do(getReq)
    require.NoError(t, err)
    defer getResp.Body.Close()

    assert.Equal(t, http.StatusOK, getResp.StatusCode)
}
```

### Frontend Testing Architecture

#### Component Unit Testing
```typescript
// Component test with React Testing Library
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { GameForm } from '../GameForm';
import { vi } from 'vitest';

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });

  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      {children}
    </QueryClientProvider>
  );
};

describe('GameForm', () => {
  it('submits form with valid data', async () => {
    const mockOnSubmit = vi.fn();

    render(
      <GameForm onSubmit={mockOnSubmit} />,
      { wrapper: createWrapper() }
    );

    // Fill out form
    fireEvent.change(screen.getByLabelText(/title/i), {
      target: { value: 'Test Game' },
    });

    fireEvent.change(screen.getByLabelText(/max players/i), {
      target: { value: '4' },
    });

    // Submit form
    fireEvent.click(screen.getByRole('button', { name: /create game/i }));

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith({
        title: 'Test Game',
        maxPlayers: 4,
        description: '',
        gameConfig: {},
      });
    });
  });

  it('displays validation errors for invalid input', async () => {
    render(<GameForm onSubmit={vi.fn()} />, { wrapper: createWrapper() });

    // Submit without required fields
    fireEvent.click(screen.getByRole('button', { name: /create game/i }));

    await waitFor(() => {
      expect(screen.getByText(/title is required/i)).toBeInTheDocument();
    });
  });
});
```

#### Custom Hook Testing
```typescript
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { useGameManagement } from '../hooks/useGameManagement';
import { vi } from 'vitest';
import * as api from '../lib/api';

// Mock API module
vi.mock('../lib/api');

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });

  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      {children}
    </QueryClientProvider>
  );
};

describe('useGameManagement', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('loads game data on mount', async () => {
    const mockGame = { id: 1, title: 'Test Game' };
    vi.mocked(api.games.get).mockResolvedValue(mockGame);

    const { result } = renderHook(
      () => useGameManagement('1'),
      { wrapper: createWrapper() }
    );

    await waitFor(() => {
      expect(result.current.game).toEqual(mockGame);
      expect(result.current.isLoading).toBe(false);
    });

    expect(api.games.get).toHaveBeenCalledWith('1');
  });

  it('handles update game mutation', async () => {
    const mockGame = { id: 1, title: 'Updated Game' };
    vi.mocked(api.games.get).mockResolvedValue(mockGame);
    vi.mocked(api.games.update).mockResolvedValue(mockGame);

    const { result } = renderHook(
      () => useGameManagement('1'),
      { wrapper: createWrapper() }
    );

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    result.current.updateGame({ title: 'Updated Game' });

    await waitFor(() => {
      expect(api.games.update).toHaveBeenCalledWith('1', { title: 'Updated Game' });
    });
  });
});
```

#### Integration Testing
```typescript
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { BrowserRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { AuthProvider } from '../contexts/AuthContext';
import { GamesList } from '../components/GamesList';
import { server } from '../mocks/server';
import { http, HttpResponse } from 'msw';

const AllProviders = ({ children }: { children: React.ReactNode }) => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });

  return (
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <BrowserRouter>
          {children}
        </BrowserRouter>
      </AuthProvider>
    </QueryClientProvider>
  );
};

describe('GamesList Integration', () => {
  beforeEach(() => {
    // Setup default API responses
    server.use(
      http.get('/api/v1/games', () => {
        return HttpResponse.json({
          data: [
            { id: 1, title: 'Game 1', status: 'active' },
            { id: 2, title: 'Game 2', status: 'setup' },
          ],
        });
      })
    );
  });

  it('displays games and handles actions', async () => {
    render(<GamesList />, { wrapper: AllProviders });

    // Wait for games to load
    await waitFor(() => {
      expect(screen.getByText('Game 1')).toBeInTheDocument();
      expect(screen.getByText('Game 2')).toBeInTheDocument();
    });

    // Test filtering
    fireEvent.change(screen.getByPlaceholderText(/search games/i), {
      target: { value: 'Game 1' },
    });

    await waitFor(() => {
      expect(screen.getByText('Game 1')).toBeInTheDocument();
      expect(screen.queryByText('Game 2')).not.toBeInTheDocument();
    });
  });
});
```

### Performance Testing

#### Go Benchmarks
```go
func BenchmarkGameService_CreateGame(b *testing.B) {
    pool := benchutil.SetupBenchDB(b)
    defer benchutil.CleanupBenchDB(b, pool)

    service := &db.GameService{DB: pool}

    game := &core.Game{
        Title:      "Benchmark Game",
        MaxPlayers: 4,
        GMUserID:   1,
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        game.Title = fmt.Sprintf("Benchmark Game %d", i)
        _, err := service.CreateGame(context.Background(), game)
        if err != nil {
            b.Fatal(err)
        }
    }
}

func BenchmarkJSONSerialization(b *testing.B) {
    game := &core.Game{
        ID:         1,
        Title:      "Benchmark Game",
        MaxPlayers: 4,
        GameConfig: map[string]interface{}{
            "rules": map[string]interface{}{
                "dice_system": "d20",
                "turn_time":   3600,
            },
        },
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := json.Marshal(game)
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

#### Load Testing Setup
```go
func TestGameAPI_LoadTest(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping load test in short mode")
    }

    app := testutil.SetupTestApp(t)
    server := httptest.NewServer(app.Handler())
    defer server.Close()

    // Create multiple users
    tokens := make([]string, 10)
    for i := 0; i < 10; i++ {
        tokens[i] = testutil.CreateTestUser(t, app,
            fmt.Sprintf("user%d", i),
            fmt.Sprintf("user%d@example.com", i))
    }

    // Concurrent game creation test
    var wg sync.WaitGroup
    errors := make(chan error, 100)

    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(i int) {
            defer wg.Done()

            token := tokens[i%len(tokens)]
            gameData := map[string]interface{}{
                "title":       fmt.Sprintf("Load Test Game %d", i),
                "max_players": 4,
            }

            jsonBody, _ := json.Marshal(gameData)
            req, _ := http.NewRequest("POST", server.URL+"/api/v1/games",
                bytes.NewBuffer(jsonBody))
            req.Header.Set("Authorization", "Bearer "+token)
            req.Header.Set("Content-Type", "application/json")

            client := &http.Client{Timeout: 10 * time.Second}
            resp, err := client.Do(req)
            if err != nil {
                errors <- err
                return
            }
            defer resp.Body.Close()

            if resp.StatusCode != http.StatusCreated {
                errors <- fmt.Errorf("unexpected status: %d", resp.StatusCode)
            }
        }(i)
    }

    wg.Wait()
    close(errors)

    // Check for errors
    var errorCount int
    for err := range errors {
        if err != nil {
            t.Logf("Load test error: %v", err)
            errorCount++
        }
    }

    // Allow up to 5% error rate
    if errorCount > 5 {
        t.Fatalf("Too many errors in load test: %d/100", errorCount)
    }
}
```

### Test Data Management

#### Test Builders
```go
type GameBuilder struct {
    game *core.Game
}

func NewGameBuilder() *GameBuilder {
    return &GameBuilder{
        game: &core.Game{
            Title:      "Test Game",
            MaxPlayers: 4,
            GMUserID:   1,
            State:      "setup",
            GameConfig: make(map[string]interface{}),
        },
    }
}

func (b *GameBuilder) WithTitle(title string) *GameBuilder {
    b.game.Title = title
    return b
}

func (b *GameBuilder) WithMaxPlayers(max int) *GameBuilder {
    b.game.MaxPlayers = max
    return b
}

func (b *GameBuilder) WithGM(userID int) *GameBuilder {
    b.game.GMUserID = userID
    return b
}

func (b *GameBuilder) Build() *core.Game {
    return b.game
}

// Usage in tests
func TestGameValidation(t *testing.T) {
    validGame := NewGameBuilder().
        WithTitle("Valid Game").
        WithMaxPlayers(6).
        Build()

    assert.NoError(t, validateGame(validGame))

    invalidGame := NewGameBuilder().
        WithTitle("").
        WithMaxPlayers(0).
        Build()

    assert.Error(t, validateGame(invalidGame))
}
```

## Test Organization

### Directory Structure

> Corrected 2026-08-26 — `backend/pkg/testutil/`, `backend/tests/`,
> `frontend/src/__tests__/integration/`, and `frontend/src/__tests__/e2e/` were
> listed here but do not exist. Actual layout:

```
backend/
├── pkg/
│   ├── games/
│   │   ├── api.go
│   │   └── api_test.go              # Co-located tests
│   ├── db/
│   │   ├── services/                # Service implementations + *_test.go
│   │   └── test_fixtures/           # SQL fixtures (apply_all.sh)
│   └── core/
│       ├── test_utils.go            # NewTestDatabase, per-package DB cloning
│       ├── test_factories.go        # Test data factories
│       ├── test_builders_test.go    # Builder helpers
│       ├── mocks.go                  # Interface mocks (files, not a package)
│       └── repository_mocks.go
└── main.go

frontend/
├── src/
│   ├── components/
│   │   ├── GameForm.tsx
│   │   └── GameForm.test.tsx        # Co-located component tests
│   ├── hooks/
│   │   ├── useGameListing.ts
│   │   └── useGameListing.test.ts   # Co-located hook tests
│   └── __tests__/                   # Cross-cutting suites only
│       ├── App.test.tsx
│       └── retired-tokens.test.ts   # Guard: no retired design tokens
├── e2e/                             # Playwright specs, grouped by domain
│   ├── auth/  characters/  admin/  common-room/  edge-cases/
│   └── config/
├── vitest.config.ts
└── playwright.config.ts
```

Backend tests are **co-located** with the code they test; there is no separate
`tests/` tree. Each test package clones its own database from a migrated
template (`core.NewTestDatabase`), so packages run in parallel safely.

### Test Commands
```bash
# Backend tests
just test-mocks           # Fast unit tests only
just test-integration     # Database integration tests
just test                 # All tests
just test-coverage        # Coverage report
just test-race            # Race condition detection

# Frontend tests
just test-fe run          # Unit and integration tests
just test-fe watch        # Watch mode
just e2e                  # End-to-end tests (desktop + mobile)
just e2e-desktop          # Desktop only
just e2e-mobile           # Mobile only

# Combined
just ci-test             # Full CI test suite (lint + test + race)

# Pre-push / end-of-session checks
just verify-quick        # Fast non-mutating checks, no builds
just verify              # All checks + backend & frontend production builds
```

## Continuous Integration

### Test Pipeline
```yaml
name: Test Pipeline

on: [push, pull_request]

jobs:
  backend-tests:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:15
        env:
          POSTGRES_PASSWORD: test
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5

    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Run unit tests
        run: just test-mocks

      - name: Run integration tests
        env:
          DATABASE_URL: postgres://postgres:test@localhost/test?sslmode=disable
        run: just test-integration

      - name: Run benchmarks
        run: just test-race

      - name: Generate coverage
        run: just test-coverage

  frontend-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-node@v3
        with:
          node-version: '18'

      - name: Install dependencies
        run: npm install

      - name: Run unit tests
        run: just test-fe run

      - name: Run E2E tests
        run: just e2e
```

## Quality Gates

### Coverage Thresholds
- Backend unit tests: >85% line coverage
- Backend integration tests: >70% line coverage
- Frontend component tests: >80% line coverage
- Critical business logic: 100% line coverage

### Performance Benchmarks
- API response time: <100ms p95
- Database query time: <50ms p95
- Frontend render time: <16ms (60fps)
- Test suite execution: <5 minutes

### Code Quality
- Zero compiler warnings
- All linter rules pass
- No security vulnerabilities in dependencies
- Documentation coverage for public APIs

## Future Enhancements

### Advanced Testing
- **Contract Testing**: Pact for API contract verification
- **Chaos Testing**: Fault injection for resilience testing
- **A/B Testing**: Statistical significance testing framework
- **Accessibility Testing**: Automated a11y testing in CI

### Tooling Improvements
- **Test Analytics**: Historical test performance tracking
- **Flaky Test Detection**: Automated identification and remediation
- **Test Impact Analysis**: Run only tests affected by changes
- **Parallel Test Distribution**: Distributed test execution

## References
- [Go Testing Best Practices](https://golang.org/doc/tutorial/add-a-test)
- [React Testing Library Documentation](https://testing-library.com/docs/react-testing-library/intro/)
- [Vitest Testing Framework](https://vitest.dev/)
- [Playwright E2E Testing](https://playwright.dev/)
