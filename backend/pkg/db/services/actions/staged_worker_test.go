package actions

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"actionphase/pkg/observability"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeReleaseRunner records calls to ReleaseDueStagedParts so the loop can be
// exercised without a database.
type fakeReleaseRunner struct {
	callCount atomic.Int32
	released  int
	returnErr error
}

func (f *fakeReleaseRunner) ReleaseDueStagedParts(_ context.Context) (int, int, error) {
	f.callCount.Add(1)
	if f.returnErr != nil {
		return 0, 0, f.returnErr
	}
	return f.released, f.released, nil
}

func newWorkerTestLogger() *observability.Logger {
	return observability.NewLogger("test", "error")
}

func TestStagedReleaseWorker_ReleasesOnStartup(t *testing.T) {
	// Parts that came due while the process was down must not wait out a full
	// tick, so the first release happens before the ticker ever fires.
	runner := &fakeReleaseRunner{released: 1}
	w := NewStagedReleaseWorker(runner, newWorkerTestLogger(), time.Hour)

	cancel := w.Start(context.Background())
	defer cancel()

	assert.Eventually(t, func() bool {
		return runner.callCount.Load() >= 1
	}, 2*time.Second, 10*time.Millisecond, "worker should release on startup, not wait for the first tick")
}

func TestStagedReleaseWorker_FiresOnInterval(t *testing.T) {
	runner := &fakeReleaseRunner{}
	w := NewStagedReleaseWorker(runner, newWorkerTestLogger(), 50*time.Millisecond)

	cancel := w.Start(context.Background())
	defer cancel()

	// Startup call plus at least two ticks.
	assert.Eventually(t, func() bool {
		return runner.callCount.Load() >= 3
	}, 2*time.Second, 10*time.Millisecond, "worker should keep firing on the ticker interval")
}

func TestStagedReleaseWorker_ContinuesAfterError(t *testing.T) {
	// A failing run must not kill the loop. A chain whose release errors once
	// is still due on the next tick, so giving up would strand it forever.
	runner := &fakeReleaseRunner{returnErr: errors.New("database unavailable")}
	w := NewStagedReleaseWorker(runner, newWorkerTestLogger(), 50*time.Millisecond)

	cancel := w.Start(context.Background())
	defer cancel()

	assert.Eventually(t, func() bool {
		return runner.callCount.Load() >= 3
	}, 2*time.Second, 10*time.Millisecond, "worker should keep ticking after a failed run")
}

func TestStagedReleaseWorker_StopsOnCancel(t *testing.T) {
	runner := &fakeReleaseRunner{}
	w := NewStagedReleaseWorker(runner, newWorkerTestLogger(), 20*time.Millisecond)

	cancel := w.Start(context.Background())

	assert.Eventually(t, func() bool {
		return runner.callCount.Load() >= 1
	}, 2*time.Second, 10*time.Millisecond)

	cancel()

	// Allow any in-flight tick to finish, then confirm the loop is quiet.
	time.Sleep(50 * time.Millisecond)
	countAfterCancel := runner.callCount.Load()
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, countAfterCancel, runner.callCount.Load(), "no releases should run after cancel")
}

func TestStagedReleaseWorker_DefaultsNonPositiveInterval(t *testing.T) {
	runner := &fakeReleaseRunner{}

	assert.Equal(t, DefaultReleaseInterval, NewStagedReleaseWorker(runner, newWorkerTestLogger(), 0).interval)
	assert.Equal(t, DefaultReleaseInterval, NewStagedReleaseWorker(runner, newWorkerTestLogger(), -time.Second).interval)
}

// TestStagedReleaseWorker_ReleasesAgainstRealDatabase drives the real service
// through the real loop. The fake-runner tests above prove the loop ticks; only
// this one proves that ticking actually reveals a part, which is what the
// wiring in main.go depends on.
func TestStagedReleaseWorker_ReleasesAgainstRealDatabase(t *testing.T) {
	env := setupStagedTest(t)

	req := env.chainRequest(0, 15)
	req.IsPublished = true
	chain, err := env.actionService.CreateStagedResultChain(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, chain, 2)

	// The part is not due yet, so a running worker must leave it alone.
	w := NewStagedReleaseWorker(env.actionService, newWorkerTestLogger(), 20*time.Millisecond)
	cancel := w.Start(context.Background())
	defer cancel()

	time.Sleep(100 * time.Millisecond)
	require.False(t, env.getResult(t, chain[1].ID).ReleasedAt.Valid,
		"worker released a part whose delay had not elapsed")

	// Make the delay elapse. The next tick should pick it up with no restart,
	// no re-registration, and no in-memory timer.
	env.backdateRelease(t, chain[0].ID, 20)

	assert.Eventually(t, func() bool {
		return env.getResult(t, chain[1].ID).ReleasedAt.Valid
	}, 3*time.Second, 20*time.Millisecond, "worker should release the part once its delay has elapsed")
}

// syncLogBuffer is a concurrency-safe io.Writer for log capture. A bare
// bytes.Buffer races here: the worker logs from its own goroutine while the
// test reads the buffer to assert on it.
type syncLogBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncLogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// panickingReleaseRunner panics on its first N calls, then succeeds. It exists
// to prove recovery is per-tick: a whole-goroutine recover would survive the
// first panic but never call ReleaseDueStagedParts again.
type panickingReleaseRunner struct {
	callCount  atomic.Int32
	panicUntil int32
}

func (f *panickingReleaseRunner) ReleaseDueStagedParts(_ context.Context) (int, int, error) {
	n := f.callCount.Add(1)
	if n <= f.panicUntil {
		panic("nil dereference in release path")
	}
	return 0, 0, nil
}

func TestStagedReleaseWorker_PanicDoesNotKillTheLoop(t *testing.T) {
	// A panic in one tick must be recovered *per tick*. Asserting only that the
	// process survived would also pass for a recover wrapped around the whole
	// goroutine — which is strictly worse than crashing, because the loop would
	// exit silently and every pending chain in every game would stall forever
	// with no crash loop to notice. So assert that later ticks still fire.
	runner := &panickingReleaseRunner{panicUntil: 1}
	w := NewStagedReleaseWorker(runner, newWorkerTestLogger(), 25*time.Millisecond)

	cancel := w.Start(context.Background())
	defer cancel()

	assert.Eventually(t, func() bool {
		return runner.callCount.Load() >= 3
	}, 2*time.Second, 10*time.Millisecond,
		"the loop must keep ticking after a panicking tick, not exit silently")
}

func TestStagedReleaseWorker_PanicOnEveryTickStillTicks(t *testing.T) {
	// The bad case from the plan: one poison row panics on every single tick.
	// The worker must keep retrying rather than dying, so that fixing the data
	// resolves it without a restart.
	runner := &panickingReleaseRunner{panicUntil: 1 << 30}
	w := NewStagedReleaseWorker(runner, newWorkerTestLogger(), 25*time.Millisecond)

	cancel := w.Start(context.Background())
	defer cancel()

	assert.Eventually(t, func() bool {
		return runner.callCount.Load() >= 3
	}, 2*time.Second, 10*time.Millisecond,
		"a persistently panicking runner must not stop the loop")
}

func TestStagedReleaseWorker_PanicOnStartupRunStillStartsTicker(t *testing.T) {
	// The startup catch-up run is outside the ticker loop; a panic there must
	// not prevent the ticker from ever being created.
	runner := &panickingReleaseRunner{panicUntil: 1}
	w := NewStagedReleaseWorker(runner, newWorkerTestLogger(), 25*time.Millisecond)

	cancel := w.Start(context.Background())
	defer cancel()

	assert.Eventually(t, func() bool {
		return runner.callCount.Load() >= 2
	}, 2*time.Second, 10*time.Millisecond,
		"a panic in the startup release must still leave the ticker running")
}

func TestStagedReleaseWorker_PanicIsLogged(t *testing.T) {
	// Recovery must be loud: a silently swallowed panic is an invisible bug.
	buf := &syncLogBuffer{}
	logger := observability.NewLogger("test", "debug")
	logger.ReplaceHandler(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	runner := &panickingReleaseRunner{panicUntil: 1}
	w := NewStagedReleaseWorker(runner, logger, time.Hour)

	cancel := w.Start(context.Background())
	defer cancel()

	assert.Eventually(t, func() bool {
		return strings.Contains(buf.String(), "staged-release-tick")
	}, 2*time.Second, 10*time.Millisecond, "the recovered panic must be logged with its unit name")
	assert.Contains(t, buf.String(), "nil dereference in release path")
}
