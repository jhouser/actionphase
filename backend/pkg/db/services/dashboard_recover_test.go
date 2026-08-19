package db

import (
	"bytes"
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"actionphase/pkg/observability"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runFanOutUnit mirrors the exact defer ordering used by each query goroutine in
// GetUserDashboard, so these tests exercise the real contract rather than a
// paraphrase of it: wg.Done() is deferred *first*, recoverFanOut second.
func runFanOutUnit(ctx context.Context, wg *sync.WaitGroup, logger *observability.Logger, name string, setErr func(error), fn func()) {
	go func() {
		defer wg.Done()
		defer recoverFanOut(ctx, logger, name, setErr)

		fn()
	}()
}

func waitTimeout(wg *sync.WaitGroup, d time.Duration) bool {
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
		return true
	case <-time.After(d):
		return false
	}
}

func TestRecoverFanOut_PanicStillCallsWaitGroupDone(t *testing.T) {
	// The load-bearing property. If recovery ran without wg.Done() still firing,
	// wg.Wait() would block forever and the request would hang rather than
	// merely failing — worse than the crash it replaced.
	var wg sync.WaitGroup
	wg.Add(1)

	var mu sync.Mutex
	var firstErr error
	setErr := func(err error) {
		mu.Lock()
		defer mu.Unlock()
		if firstErr == nil {
			firstErr = err
		}
	}

	runFanOutUnit(context.Background(), &wg, observability.NewLogger("test", "error"),
		"dashboard-games", setErr, func() {
			panic("nil dereference in query path")
		})

	require.True(t, waitTimeout(&wg, 2*time.Second),
		"wg.Wait() must not block after a panicking fan-out query")

	mu.Lock()
	defer mu.Unlock()
	assert.Error(t, firstErr, "a panicking query must report an error")
	assert.Contains(t, firstErr.Error(), "dashboard-games", "the error must name the query that failed")
}

func TestRecoverFanOut_PanicDoesNotYieldSilentPartialDashboard(t *testing.T) {
	// A recovered query leaves its result slice nil. Returning that as a 200
	// would make "no upcoming deadlines" indistinguishable from "the deadlines
	// query blew up", so the panic must surface as an error instead.
	var wg sync.WaitGroup
	wg.Add(2)

	var mu sync.Mutex
	var firstErr error
	setErr := func(err error) {
		mu.Lock()
		defer mu.Unlock()
		if firstErr == nil {
			firstErr = err
		}
	}

	ok := false
	logger := observability.NewLogger("test", "error")
	runFanOutUnit(context.Background(), &wg, logger, "dashboard-upcoming-deadlines", setErr, func() {
		panic("boom")
	})
	runFanOutUnit(context.Background(), &wg, logger, "dashboard-games", setErr, func() {
		ok = true
	})

	require.True(t, waitTimeout(&wg, 2*time.Second), "wg.Wait() must not block")

	assert.True(t, ok, "a panic in one query must not prevent the others from completing")

	mu.Lock()
	defer mu.Unlock()
	require.Error(t, firstErr)
	assert.Contains(t, firstErr.Error(), "dashboard-upcoming-deadlines")
}

func TestRecoverFanOut_NoPanicReportsNoError(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	called := false
	setErr := func(error) { called = true }

	runFanOutUnit(context.Background(), &wg, observability.NewLogger("test", "error"),
		"dashboard-games", setErr, func() {})

	require.True(t, waitTimeout(&wg, 2*time.Second))
	assert.False(t, called, "a clean query must not report an error")
}

func TestRecoverFanOut_LogsUnitAndStack(t *testing.T) {
	var buf bytes.Buffer
	logger := observability.NewLogger("test", "debug")
	logger.ReplaceHandler(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	var wg sync.WaitGroup
	wg.Add(1)

	runFanOutUnit(context.Background(), &wg, logger, "dashboard-unread-comments",
		func(error) {}, func() { panic("boom") })

	require.True(t, waitTimeout(&wg, 2*time.Second))

	out := buf.String()
	assert.Contains(t, out, "dashboard-unread-comments", "log must name the unit")
	assert.Contains(t, out, "stack_trace", "log must include a stack trace")
}

func TestRecoverFanOut_NilLoggerStillReportsError(t *testing.T) {
	// A nil logger must not turn the recovered panic back into a fatal one, and
	// the caller must still learn the query failed.
	var wg sync.WaitGroup
	wg.Add(1)

	var mu sync.Mutex
	var firstErr error
	setErr := func(err error) {
		mu.Lock()
		defer mu.Unlock()
		firstErr = err
	}

	runFanOutUnit(context.Background(), &wg, nil, "dashboard-games", setErr, func() {
		panic("boom")
	})

	require.True(t, waitTimeout(&wg, 2*time.Second))

	mu.Lock()
	defer mu.Unlock()
	assert.Error(t, firstErr, "the error must be reported even without a logger")
}
