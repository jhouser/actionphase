package actions

import (
	"context"
	"time"

	"actionphase/pkg/observability"
)

// ReleaseRunner is the minimal interface the release worker needs, so the loop
// can be tested without a database.
type ReleaseRunner interface {
	ReleaseDueStagedParts(ctx context.Context) (examined int, released int, err error)
}

// DefaultReleaseInterval is how often the worker checks for due parts.
//
// A minute is deliberately coarse. Delays are configured in whole minutes, so
// finer polling would only shrink an error the GM cannot express anyway, and the
// player-facing countdown already accounts for the lag by showing "unlocking…"
// rather than a negative timer once it reaches zero.
const DefaultReleaseInterval = time.Minute

// StagedReleaseWorker periodically releases staged result parts whose wait has
// elapsed.
//
// There is deliberately no per-result timer. Due-ness is a SQL comparison
// against the parent's released_at, which makes the worker stateless: it can
// crash, restart, or run late without losing or double-firing a release. A part
// that came due during downtime simply releases on the next tick.
type StagedReleaseWorker struct {
	runner   ReleaseRunner
	logger   *observability.Logger
	interval time.Duration
}

// NewStagedReleaseWorker creates a worker. A non-positive interval falls back to
// the default.
func NewStagedReleaseWorker(runner ReleaseRunner, logger *observability.Logger, interval time.Duration) *StagedReleaseWorker {
	if interval <= 0 {
		interval = DefaultReleaseInterval
	}
	return &StagedReleaseWorker{runner: runner, logger: logger, interval: interval}
}

// Start runs the worker loop in a goroutine and returns a cancel func.
func (w *StagedReleaseWorker) Start(ctx context.Context) context.CancelFunc {
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		w.logger.Info(ctx, "Staged result release worker started", "interval", w.interval)

		// Release immediately on startup rather than waiting out a full tick, so
		// parts that came due while the process was down don't sit visibly late
		// for a player who is watching a countdown that already hit zero.
		w.release(ctx)

		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				w.release(ctx)
			case <-ctx.Done():
				w.logger.Info(ctx, "Staged result release worker stopped")
				return
			}
		}
	}()

	return cancel
}

func (w *StagedReleaseWorker) release(ctx context.Context) {
	examined, released, err := w.runner.ReleaseDueStagedParts(ctx)
	if err != nil {
		w.logger.LogError(ctx, err, "Staged result release run failed", "examined", examined)
		return
	}
	if released > 0 {
		w.logger.Info(ctx, "Released staged result parts", "examined", examined, "released", released)
	} else {
		w.logger.Debug(ctx, "Staged release tick: nothing due", "examined", examined)
	}
}
