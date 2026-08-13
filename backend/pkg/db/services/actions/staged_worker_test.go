package actions

import (
	"context"
	"errors"
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
