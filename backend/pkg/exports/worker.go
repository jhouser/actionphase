package exports

import (
	"context"
	"time"

	"actionphase/pkg/observability"
)

// JobRunner is the minimal interface the worker needs, so the loop can be
// tested without a database or storage backend.
type JobRunner interface {
	RunNextJob(ctx context.Context) (claimed bool, err error)
	RequeueStalled(ctx context.Context, staleAfter time.Duration) (int64, error)
	SweepExpired(ctx context.Context) (deleted int, err error)
}

// Defaults for worker pacing.
const (
	// DefaultPollInterval is how long to wait after finding an empty queue.
	DefaultPollInterval = 15 * time.Second

	// DefaultStaleAfter bounds how long a 'running' job may sit untouched
	// before startup recovery assumes its process died. Comfortably longer
	// than any realistic assembly.
	DefaultStaleAfter = 15 * time.Minute

	// maxJobsPerTick bounds work per wakeup so a large backlog cannot starve
	// context-cancellation checks.
	maxJobsPerTick = 10

	// sweepInterval is how often expired artifacts are reclaimed. Retention is
	// measured in days, so sweeping on every job-poll tick would be pure query
	// load; hourly is far more often than the window requires.
	sweepInterval = time.Hour
)

// Worker drains the export queue in the background.
type Worker struct {
	runner   JobRunner
	logger   *observability.Logger
	interval time.Duration
}

// NewWorker creates a Worker. A non-positive interval falls back to the default.
func NewWorker(runner JobRunner, logger *observability.Logger, interval time.Duration) *Worker {
	if interval <= 0 {
		interval = DefaultPollInterval
	}
	return &Worker{runner: runner, logger: logger, interval: interval}
}

// Start runs the worker loop in a goroutine and returns a cancel func.
func (w *Worker) Start(ctx context.Context) context.CancelFunc {
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		w.logInfo(ctx, "Export worker started", "interval", w.interval)

		// Recover jobs abandoned by a previous process before taking new work,
		// so a crash mid-export doesn't leave a request stuck forever.
		observability.SafeRun(ctx, w.logger, "export-startup-requeue", func() {
			if _, err := w.runner.RequeueStalled(ctx, DefaultStaleAfter); err != nil {
				w.logErr(ctx, err, "Startup requeue of stalled exports failed")
			}
		})

		// Reclaim anything that expired while the process was down.
		w.safeSweep(ctx)

		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()
		sweepTicker := time.NewTicker(sweepInterval)
		defer sweepTicker.Stop()

		for {
			w.safeDrain(ctx)

			select {
			case <-ticker.C:
			case <-sweepTicker.C:
				w.safeSweep(ctx)
			case <-ctx.Done():
				w.logInfo(ctx, "Export worker stopped")
				return
			}
		}
	}()

	return cancel
}

// safeDrain runs one drain pass with panic recovery. Recovery is per-pass, not
// around the loop: a panic on one malformed job must not stop the queue
// permanently and silently.
func (w *Worker) safeDrain(ctx context.Context) {
	observability.SafeRun(ctx, w.logger, "export-drain-tick", func() { w.drain(ctx) })
}

// safeSweep runs one retention sweep with panic recovery.
func (w *Worker) safeSweep(ctx context.Context) {
	observability.SafeRun(ctx, w.logger, "export-sweep-tick", func() { w.sweep(ctx) })
}

// drain runs queued jobs until the queue empties, the budget is spent, or the
// context is cancelled. Consecutive jobs run without waiting for the next tick
// so a backlog clears promptly.
func (w *Worker) drain(ctx context.Context) {
	for i := 0; i < maxJobsPerTick; i++ {
		if ctx.Err() != nil {
			return
		}
		claimed, err := w.runner.RunNextJob(ctx)
		if err != nil {
			w.logErr(ctx, err, "Export job run failed")
			return
		}
		if !claimed {
			return // queue empty
		}
	}
	w.logInfo(ctx, "Export worker hit per-tick job limit", "limit", maxJobsPerTick)
}

// sweep reclaims expired artifacts. A failure is logged and the loop
// continues: retention is best-effort housekeeping, not a reason to stop
// serving exports.
func (w *Worker) sweep(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	if _, err := w.runner.SweepExpired(ctx); err != nil {
		w.logErr(ctx, err, "Expired export sweep failed")
	}
}

func (w *Worker) logInfo(ctx context.Context, msg string, args ...any) {
	if w.logger != nil {
		w.logger.Info(ctx, msg, args...)
	}
}

func (w *Worker) logErr(ctx context.Context, err error, msg string, args ...any) {
	if w.logger != nil {
		w.logger.LogError(ctx, err, msg, args...)
	}
}
