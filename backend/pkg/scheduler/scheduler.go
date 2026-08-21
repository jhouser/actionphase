package scheduler

import (
	"context"
	"time"

	"actionphase/pkg/observability"
)

// ActivationRunner is the minimal interface the scheduler needs.
type ActivationRunner interface {
	RunScheduledActivations(ctx context.Context) (examined int, activated int, err error)
}

// Scheduler runs periodic background tasks for the ActionPhase application.
// Currently handles: automatic phase activation based on scheduled start times.
type Scheduler struct {
	phaseService ActivationRunner
	logger       *observability.Logger
	interval     time.Duration
}

// New creates a Scheduler. interval controls how often scheduled activations are checked.
func New(phaseService ActivationRunner, logger *observability.Logger, interval time.Duration) *Scheduler {
	return &Scheduler{
		phaseService: phaseService,
		logger:       logger,
		interval:     interval,
	}
}

// Start begins the scheduler loop in a background goroutine.
// It returns immediately; call the returned cancel func to stop it.
func (s *Scheduler) Start(ctx context.Context) context.CancelFunc {
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		s.logger.Info(ctx, "Phase scheduler started", "interval", s.interval)

		// Run once immediately on startup to catch any phases that should have
		// activated while the server was down.
		s.safeRunActivations(ctx)

		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.safeRunActivations(ctx)
			case <-ctx.Done():
				s.logger.Info(ctx, "Phase scheduler stopped")
				return
			}
		}
	}()

	return cancel
}

// safeRunActivations runs one activation pass with panic recovery. Recovery is
// per-tick, not around the loop: phase activation is core game progression, and
// a silently dead scheduler would stall every scheduled phase in every game.
func (s *Scheduler) safeRunActivations(ctx context.Context) {
	observability.SafeRun(ctx, s.logger, "phase-activation-tick", func() { s.runActivations(ctx) })
}

func (s *Scheduler) runActivations(ctx context.Context) {
	s.logger.Debug(ctx, "Scheduler tick: checking for scheduled phase activations")
	examined, activated, err := s.phaseService.RunScheduledActivations(ctx)
	if err != nil {
		s.logger.LogError(ctx, err, "Scheduled phase activation run failed", "examined", examined)
		return
	}
	if activated > 0 {
		s.logger.Info(ctx, "Scheduler activated phases", "examined", examined, "activated", activated)
	} else {
		s.logger.Debug(ctx, "Scheduler tick: no phases to activate", "examined", examined)
	}
}
