package exports

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRunner counts calls and serves scripted outcomes.
type fakeRunner struct {
	mu sync.Mutex

	runCalls     atomic.Int32
	requeueCalls atomic.Int32
	sweepCalls   atomic.Int32

	// queued is how many jobs remain to be "claimed".
	queued int
	// runErr is returned on every RunNextJob call when set.
	runErr error
	// requeueErr is returned by RequeueStalled when set.
	requeueErr error
	// sweepErr is returned by SweepExpired when set.
	sweepErr error
	// staleAfterSeen records the argument passed to RequeueStalled.
	staleAfterSeen time.Duration
}

func (f *fakeRunner) RunNextJob(context.Context) (bool, error) {
	f.runCalls.Add(1)
	if f.runErr != nil {
		return false, f.runErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.queued <= 0 {
		return false, nil
	}
	f.queued--
	return true, nil
}

func (f *fakeRunner) RequeueStalled(_ context.Context, staleAfter time.Duration) (int64, error) {
	f.requeueCalls.Add(1)
	f.mu.Lock()
	f.staleAfterSeen = staleAfter
	f.mu.Unlock()
	if f.requeueErr != nil {
		return 0, f.requeueErr
	}
	return 0, nil
}

func (f *fakeRunner) SweepExpired(context.Context) (int, error) {
	f.sweepCalls.Add(1)
	if f.sweepErr != nil {
		return 0, f.sweepErr
	}
	return 0, nil
}

func TestWorker_RequeuesStalledOnStartup(t *testing.T) {
	r := &fakeRunner{}
	w := NewWorker(r, nil, 10*time.Millisecond)

	cancel := w.Start(context.Background())
	defer cancel()

	assert.Eventually(t, func() bool { return r.requeueCalls.Load() >= 1 },
		2*time.Second, 5*time.Millisecond,
		"worker must recover abandoned jobs before taking new work")

	r.mu.Lock()
	seen := r.staleAfterSeen
	r.mu.Unlock()
	assert.Equal(t, DefaultStaleAfter, seen)
}

func TestWorker_DrainsQueuedJobs(t *testing.T) {
	r := &fakeRunner{queued: 3}
	w := NewWorker(r, nil, time.Hour) // long tick: all work must happen in one drain

	cancel := w.Start(context.Background())
	defer cancel()

	// 3 claims + 1 empty-queue probe.
	assert.Eventually(t, func() bool { return r.runCalls.Load() >= 4 },
		2*time.Second, 5*time.Millisecond,
		"a backlog must drain within a single tick, not one job per tick")
}

func TestWorker_StopsAtPerTickLimit(t *testing.T) {
	r := &fakeRunner{queued: 1000}
	w := NewWorker(r, nil, time.Hour)

	cancel := w.Start(context.Background())
	defer cancel()

	assert.Eventually(t, func() bool { return r.runCalls.Load() >= int32(maxJobsPerTick) },
		2*time.Second, 5*time.Millisecond)

	// Give the loop a chance to overrun the budget if the cap were broken.
	time.Sleep(100 * time.Millisecond)
	assert.LessOrEqual(t, r.runCalls.Load(), int32(maxJobsPerTick),
		"a large backlog must not monopolize the worker within one tick")
}

func TestWorker_StopsOnRunError(t *testing.T) {
	r := &fakeRunner{queued: 5, runErr: errors.New("db down")}
	w := NewWorker(r, nil, time.Hour)

	cancel := w.Start(context.Background())
	defer cancel()

	assert.Eventually(t, func() bool { return r.runCalls.Load() >= 1 },
		2*time.Second, 5*time.Millisecond)

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(1), r.runCalls.Load(),
		"an infrastructure error must end the drain rather than spin")
}

func TestWorker_SurvivesRequeueError(t *testing.T) {
	r := &fakeRunner{queued: 1, requeueErr: errors.New("requeue failed")}
	w := NewWorker(r, nil, 10*time.Millisecond)

	cancel := w.Start(context.Background())
	defer cancel()

	// A failed startup recovery must not prevent normal processing.
	assert.Eventually(t, func() bool { return r.runCalls.Load() >= 1 },
		2*time.Second, 5*time.Millisecond)
}

func TestWorker_StopsOnCancel(t *testing.T) {
	r := &fakeRunner{}
	w := NewWorker(r, nil, 10*time.Millisecond)

	cancel := w.Start(context.Background())
	assert.Eventually(t, func() bool { return r.runCalls.Load() >= 1 },
		2*time.Second, 5*time.Millisecond)

	cancel()
	settled := r.runCalls.Load()
	time.Sleep(120 * time.Millisecond)

	assert.LessOrEqual(t, r.runCalls.Load(), settled+1,
		"worker must stop promptly after cancel")
}

func TestWorker_ParentCancelStops(t *testing.T) {
	r := &fakeRunner{}
	ctx, cancelParent := context.WithCancel(context.Background())
	w := NewWorker(r, nil, 10*time.Millisecond)

	stop := w.Start(ctx)
	defer stop()

	assert.Eventually(t, func() bool { return r.runCalls.Load() >= 1 },
		2*time.Second, 5*time.Millisecond)

	cancelParent()
	settled := r.runCalls.Load()
	time.Sleep(120 * time.Millisecond)
	assert.LessOrEqual(t, r.runCalls.Load(), settled+1)
}

// Artifacts that expired while the process was down must be reclaimed at
// startup rather than waiting a full sweep interval.
func TestWorker_SweepsOnStartup(t *testing.T) {
	r := &fakeRunner{}
	w := NewWorker(r, nil, 10*time.Millisecond)

	cancel := w.Start(context.Background())
	defer cancel()

	assert.Eventually(t, func() bool { return r.sweepCalls.Load() >= 1 },
		2*time.Second, 5*time.Millisecond,
		"worker must sweep expired artifacts on startup")
}

// Retention is measured in days, so the sweep must not run on every job tick.
func TestWorker_DoesNotSweepEveryPollTick(t *testing.T) {
	r := &fakeRunner{}
	w := NewWorker(r, nil, 10*time.Millisecond)

	cancel := w.Start(context.Background())
	defer cancel()

	// Let many poll ticks elapse.
	assert.Eventually(t, func() bool { return r.runCalls.Load() >= 5 },
		2*time.Second, 5*time.Millisecond)

	assert.Equal(t, int32(1), r.sweepCalls.Load(),
		"sweep runs on its own hourly schedule, not once per poll tick")
}

func TestWorker_SurvivesSweepError(t *testing.T) {
	r := &fakeRunner{queued: 1, sweepErr: errors.New("storage unavailable")}
	w := NewWorker(r, nil, 10*time.Millisecond)

	cancel := w.Start(context.Background())
	defer cancel()

	// Housekeeping failure must not stop exports from being served.
	assert.Eventually(t, func() bool { return r.runCalls.Load() >= 1 },
		2*time.Second, 5*time.Millisecond)
}

func TestNewWorker_DefaultsInterval(t *testing.T) {
	assert.Equal(t, DefaultPollInterval, NewWorker(&fakeRunner{}, nil, 0).interval)
	assert.Equal(t, DefaultPollInterval, NewWorker(&fakeRunner{}, nil, -time.Second).interval)
	assert.Equal(t, time.Minute, NewWorker(&fakeRunner{}, nil, time.Minute).interval)
}

func TestExportStoragePath(t *testing.T) {
	assert.Equal(t, "exports/game-164/export-9.zip", ExportStoragePath(164, 9))

	// Distinct export ids must not collide, so an already-issued download URL
	// keeps resolving to the bytes it described.
	assert.NotEqual(t, ExportStoragePath(164, 9), ExportStoragePath(164, 10))
	assert.NotEqual(t, ExportStoragePath(164, 9), ExportStoragePath(165, 9))
}

func TestTruncateError(t *testing.T) {
	assert.Equal(t, "short", truncateError("short"))

	long := truncateError(string(make([]byte, 5000)))
	assert.LessOrEqual(t, len(long), 1000+len("… (truncated)"))
	assert.Contains(t, long, "truncated")
}

func TestDurationToInterval(t *testing.T) {
	iv := durationToInterval(15 * time.Minute)
	require.True(t, iv.Valid)
	assert.Equal(t, int64(15*60*1_000_000), iv.Microseconds)
}
