// Package cleanup holds the periodic housekeeping workers that prune rows no
// longer worth keeping: read notifications past their retention window, expired
// auth artifacts (password reset tokens, email verification records,
// registration-attempt bot data), and expired sessions.
//
// These previously lived as anonymous goroutines inline in root.go and main.go
// with no cancel func, no ctx.Done() case, and no test coverage. They are real
// worker types here for the same reasons the scheduler and export workers are:
// they can be shut down gracefully, their tick can be tested without a
// database, and a panic in one tick is recovered rather than taking the process
// down.
package cleanup

import (
	"context"
	"time"

	"actionphase/pkg/observability"
)

// DefaultInterval is how often cleanup runs. Retention windows are measured in
// days, so a daily sweep is far more often than they require.
const DefaultInterval = 24 * time.Hour

// DefaultSessionInterval is how often expired sessions are pruned. Sessions
// expire on a scale of hours rather than days, so they sweep more often than
// the daily housekeeping above.
const DefaultSessionInterval = time.Hour

// NotificationPruner is the minimal interface the notification worker needs, so
// the loop can be tested without a database.
type NotificationPruner interface {
	DeleteOldReadNotifications(ctx context.Context) error
}

// AuthPruner is the minimal interface the auth cleanup worker needs. Each
// method prunes one table; a failure in one must not skip the others, since
// they are independent housekeeping tasks that happen to share a schedule.
type AuthPruner interface {
	CleanupExpiredTokens(ctx context.Context) error
	CleanupExpiredVerificationTokens(ctx context.Context) error
	CleanupOldRegistrationAttempts(ctx context.Context) error
}

// NotificationWorker prunes read notifications past the retention window.
type NotificationWorker struct {
	pruner   NotificationPruner
	logger   *observability.Logger
	interval time.Duration
}

// NewNotificationWorker creates a worker. A non-positive interval falls back to
// the default.
func NewNotificationWorker(pruner NotificationPruner, logger *observability.Logger, interval time.Duration) *NotificationWorker {
	if interval <= 0 {
		interval = DefaultInterval
	}
	return &NotificationWorker{pruner: pruner, logger: logger, interval: interval}
}

// Start runs the worker loop in a goroutine and returns a cancel func.
//
// Unlike the scheduler and export workers there is no catch-up run on startup:
// nothing is time-sensitive about pruning, and running it on every boot would
// put a delete sweep on the startup path of a process that may restart often.
func (w *NotificationWorker) Start(ctx context.Context) context.CancelFunc {
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		logInfo(ctx, w.logger, "Notification cleanup worker started", "interval", w.interval)

		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				w.safeRun(ctx)
			case <-ctx.Done():
				logInfo(ctx, w.logger, "Notification cleanup worker stopped")
				return
			}
		}
	}()

	return cancel
}

// safeRun runs one prune tick with panic recovery, per-tick rather than around
// the loop so one bad tick does not silently end all future cleanup.
func (w *NotificationWorker) safeRun(ctx context.Context) {
	observability.SafeRun(ctx, w.logger, "notification-cleanup-tick", func() { w.run(ctx) })
}

func (w *NotificationWorker) run(ctx context.Context) {
	if err := w.pruner.DeleteOldReadNotifications(ctx); err != nil {
		logErr(ctx, w.logger, err, "Background notification cleanup failed")
		return
	}
	logDebug(ctx, w.logger, "Notification cleanup tick complete")
}

// AuthWorker prunes expired auth artifacts.
type AuthWorker struct {
	pruner   AuthPruner
	logger   *observability.Logger
	interval time.Duration
}

// NewAuthWorker creates a worker. A non-positive interval falls back to the
// default.
func NewAuthWorker(pruner AuthPruner, logger *observability.Logger, interval time.Duration) *AuthWorker {
	if interval <= 0 {
		interval = DefaultInterval
	}
	return &AuthWorker{pruner: pruner, logger: logger, interval: interval}
}

// Start runs the worker loop in a goroutine and returns a cancel func.
func (w *AuthWorker) Start(ctx context.Context) context.CancelFunc {
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		logInfo(ctx, w.logger, "Auth cleanup worker started", "interval", w.interval)

		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				w.safeRun(ctx)
			case <-ctx.Done():
				logInfo(ctx, w.logger, "Auth cleanup worker stopped")
				return
			}
		}
	}()

	return cancel
}

// safeRun runs one prune tick with panic recovery.
func (w *AuthWorker) safeRun(ctx context.Context) {
	observability.SafeRun(ctx, w.logger, "auth-cleanup-tick", func() { w.run(ctx) })
}

func (w *AuthWorker) run(ctx context.Context) {
	if err := w.pruner.CleanupExpiredTokens(ctx); err != nil {
		logErr(ctx, w.logger, err, "Background password token cleanup failed")
	}
	if err := w.pruner.CleanupExpiredVerificationTokens(ctx); err != nil {
		logErr(ctx, w.logger, err, "Background verification token cleanup failed")
	}
	if err := w.pruner.CleanupOldRegistrationAttempts(ctx); err != nil {
		logErr(ctx, w.logger, err, "Background registration attempt cleanup failed")
	}
}

func logInfo(ctx context.Context, l *observability.Logger, msg string, args ...any) {
	if l != nil {
		l.Info(ctx, msg, args...)
	}
}

func logDebug(ctx context.Context, l *observability.Logger, msg string, args ...any) {
	if l != nil {
		l.Debug(ctx, msg, args...)
	}
}

func logErr(ctx context.Context, l *observability.Logger, err error, msg string, args ...any) {
	if l != nil {
		l.LogError(ctx, err, msg, args...)
	}
}

// SessionPruner is the minimal interface the session worker needs.
type SessionPruner interface {
	CleanupExpiredSessions(ctx context.Context) error
}

// SessionWorker deletes expired sessions so they do not accumulate.
type SessionWorker struct {
	pruner   SessionPruner
	logger   *observability.Logger
	interval time.Duration
}

// NewSessionWorker creates a worker. A non-positive interval falls back to
// DefaultSessionInterval.
func NewSessionWorker(pruner SessionPruner, logger *observability.Logger, interval time.Duration) *SessionWorker {
	if interval <= 0 {
		interval = DefaultSessionInterval
	}
	return &SessionWorker{pruner: pruner, logger: logger, interval: interval}
}

// Start runs the worker loop in a goroutine and returns a cancel func.
//
// Unlike the notification and auth workers this does run once on startup,
// preserving the behaviour of the inline loop it replaces: expired sessions
// accumulated while the process was down are rows that should already be gone,
// and clearing them is a single bounded delete.
func (w *SessionWorker) Start(ctx context.Context) context.CancelFunc {
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		logInfo(ctx, w.logger, "Session cleanup worker started", "interval", w.interval)

		w.safeRun(ctx, "Startup expired session cleanup failed")

		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				w.safeRun(ctx, "Periodic expired session cleanup failed")
			case <-ctx.Done():
				logInfo(ctx, w.logger, "Session cleanup worker stopped")
				return
			}
		}
	}()

	return cancel
}

// safeRun runs one prune tick with panic recovery. The startup run is wrapped
// too: a panic there would otherwise stop the ticker from ever being created.
func (w *SessionWorker) safeRun(ctx context.Context, failureMsg string) {
	observability.SafeRun(ctx, w.logger, "session-cleanup-tick", func() {
		if err := w.pruner.CleanupExpiredSessions(ctx); err != nil {
			logErr(ctx, w.logger, err, failureMsg)
		}
	})
}
