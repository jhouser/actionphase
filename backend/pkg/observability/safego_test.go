package observability

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// syncBuffer is a concurrency-safe io.Writer for log capture. A bare
// bytes.Buffer races here: the panic is logged from a background goroutine
// while the test reads the buffer to assert on it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// captureLogger returns a Logger writing JSON to the returned buffer, so tests
// can assert on what a recovered panic actually reported.
func captureLogger() (*Logger, *syncBuffer) {
	buf := &syncBuffer{}
	l := NewLogger("test", "debug")
	l.ReplaceHandler(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return l, buf
}

func TestSafeRun_RunsFn(t *testing.T) {
	logger, buf := captureLogger()

	ran := false
	SafeRun(context.Background(), logger, "unit", func() { ran = true })

	assert.True(t, ran, "SafeRun must call fn")
	assert.Empty(t, buf.String(), "a clean run should log nothing")
}

func TestSafeRun_RecoversPanicAndReturns(t *testing.T) {
	// The whole point: a panic in background work must not escape to the
	// runtime, where it would terminate the entire process.
	logger, _ := captureLogger()

	assert.NotPanics(t, func() {
		SafeRun(context.Background(), logger, "unit", func() {
			panic("boom")
		})
	})
}

func TestSafeRun_LogsUnitNameAndStack(t *testing.T) {
	logger, buf := captureLogger()

	SafeRun(context.Background(), logger, "staged-release-tick", func() {
		panic("boom")
	})

	out := buf.String()
	require.NotEmpty(t, out, "a recovered panic must be logged, not swallowed silently")
	assert.Contains(t, out, "staged-release-tick", "log must name the unit that panicked")
	assert.Contains(t, out, "boom", "log must include the panic value")
	assert.Contains(t, out, "stack_trace", "log must include a stack trace to be actionable")
	// The stack must point at the panicking function, not just at SafeRun.
	assert.Contains(t, out, "TestSafeRun_LogsUnitNameAndStack", "stack should reach the panic site")
}

func TestSafeRun_RecoversNonStringPanic(t *testing.T) {
	// runtime errors (nil dereference) are the realistic trigger, and they
	// panic with an error value rather than a string.
	logger, buf := captureLogger()

	assert.NotPanics(t, func() {
		SafeRun(context.Background(), logger, "unit", func() {
			var m map[string]int
			m["x"] = 1 // assignment to entry in nil map
		})
	})
	assert.Contains(t, buf.String(), "nil map", "the runtime error should reach the log")
}

func TestSafeRun_NilLoggerDoesNotRepanic(t *testing.T) {
	// A nil logger inside the recover path would turn a recovered panic back
	// into a fatal one, which is exactly the failure this helper exists to
	// prevent.
	assert.NotPanics(t, func() {
		SafeRun(context.Background(), nil, "unit", func() { panic("boom") })
	})
}

func TestSafeGo_RunsFnInGoroutine(t *testing.T) {
	logger, _ := captureLogger()

	var wg sync.WaitGroup
	wg.Add(1)
	SafeGo(context.Background(), logger, "unit", func() { wg.Done() })

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SafeGo did not run fn")
	}
}

func TestSafeGo_PanicDoesNotEscape(t *testing.T) {
	// A panic in a spawned goroutine cannot be caught by the spawner, so this
	// asserts on the log instead: if recovery were missing, the test binary
	// itself would crash.
	logger, buf := captureLogger()

	SafeGo(context.Background(), logger, "notify-handout-published", func() {
		panic("boom")
	})

	assert.Eventually(t, func() bool {
		return strings.Contains(buf.String(), "notify-handout-published")
	}, 2*time.Second, 10*time.Millisecond, "SafeGo must recover and log the panic")
}
