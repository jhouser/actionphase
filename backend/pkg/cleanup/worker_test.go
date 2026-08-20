package cleanup

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
)

func testLogger() *observability.Logger {
	return observability.NewLogger("test", "error")
}

// syncBuffer is a concurrency-safe io.Writer for log capture. A bare
// bytes.Buffer races here: the worker logs from its own goroutine while the
// test reads the buffer to assert on it.
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

func captureLogger() (*observability.Logger, *syncBuffer) {
	buf := &syncBuffer{}
	l := observability.NewLogger("test", "debug")
	l.ReplaceHandler(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return l, buf
}

// fakeNotificationPruner records calls so the loop can be exercised without a
// database.
type fakeNotificationPruner struct {
	callCount  atomic.Int32
	returnErr  error
	panicUntil int32
}

func (f *fakeNotificationPruner) DeleteOldReadNotifications(_ context.Context) error {
	n := f.callCount.Add(1)
	if n <= f.panicUntil {
		panic("nil dereference in cleanup path")
	}
	return f.returnErr
}

func TestNotificationWorker_FiresOnInterval(t *testing.T) {
	pruner := &fakeNotificationPruner{}
	w := NewNotificationWorker(pruner, testLogger(), 25*time.Millisecond)

	cancel := w.Start(context.Background())
	defer cancel()

	assert.Eventually(t, func() bool {
		return pruner.callCount.Load() >= 2
	}, 2*time.Second, 10*time.Millisecond, "worker should prune on each tick")
}

func TestNotificationWorker_NoStartupRun(t *testing.T) {
	// Pruning is not time-sensitive, so it must not run on the startup path of a
	// process that may restart often.
	pruner := &fakeNotificationPruner{}
	w := NewNotificationWorker(pruner, testLogger(), time.Hour)

	cancel := w.Start(context.Background())
	defer cancel()

	time.Sleep(100 * time.Millisecond)
	assert.Zero(t, pruner.callCount.Load(), "cleanup should wait for the first tick")
}

func TestNotificationWorker_ContinuesAfterError(t *testing.T) {
	pruner := &fakeNotificationPruner{returnErr: errors.New("database unavailable")}
	w := NewNotificationWorker(pruner, testLogger(), 25*time.Millisecond)

	cancel := w.Start(context.Background())
	defer cancel()

	assert.Eventually(t, func() bool {
		return pruner.callCount.Load() >= 2
	}, 2*time.Second, 10*time.Millisecond, "an error must not stop the loop")
}

func TestNotificationWorker_PanicDoesNotKillTheLoop(t *testing.T) {
	// Per-tick recovery, not whole-goroutine: a later tick must still fire.
	pruner := &fakeNotificationPruner{panicUntil: 1}
	w := NewNotificationWorker(pruner, testLogger(), 25*time.Millisecond)

	cancel := w.Start(context.Background())
	defer cancel()

	assert.Eventually(t, func() bool {
		return pruner.callCount.Load() >= 3
	}, 2*time.Second, 10*time.Millisecond, "the loop must keep ticking after a panicking tick")
}

func TestNotificationWorker_PanicIsLogged(t *testing.T) {
	logger, buf := captureLogger()
	pruner := &fakeNotificationPruner{panicUntil: 1}
	w := NewNotificationWorker(pruner, logger, 25*time.Millisecond)

	cancel := w.Start(context.Background())
	defer cancel()

	assert.Eventually(t, func() bool {
		return strings.Contains(buf.String(), "notification-cleanup-tick")
	}, 2*time.Second, 10*time.Millisecond, "the recovered panic must name its unit")
}

func TestNotificationWorker_StopsOnCancel(t *testing.T) {
	// The inline goroutines this replaced had no ctx.Done() case and could not
	// be shut down at all.
	pruner := &fakeNotificationPruner{}
	w := NewNotificationWorker(pruner, testLogger(), 25*time.Millisecond)

	cancel := w.Start(context.Background())
	assert.Eventually(t, func() bool {
		return pruner.callCount.Load() >= 1
	}, 2*time.Second, 10*time.Millisecond)

	cancel()
	time.Sleep(100 * time.Millisecond)
	settled := pruner.callCount.Load()
	time.Sleep(150 * time.Millisecond)

	assert.Equal(t, settled, pruner.callCount.Load(), "no ticks should run after cancel")
}

func TestNewNotificationWorker_DefaultsInterval(t *testing.T) {
	w := NewNotificationWorker(&fakeNotificationPruner{}, testLogger(), 0)
	assert.Equal(t, DefaultInterval, w.interval, "a non-positive interval should fall back to the default")
}

// fakeAuthPruner records which of the three independent prunes ran.
type fakeAuthPruner struct {
	tokens        atomic.Int32
	verifications atomic.Int32
	attempts      atomic.Int32

	tokensErr    error
	tokensPanics bool
}

func (f *fakeAuthPruner) CleanupExpiredTokens(_ context.Context) error {
	f.tokens.Add(1)
	if f.tokensPanics {
		panic("nil dereference in token cleanup")
	}
	return f.tokensErr
}

func (f *fakeAuthPruner) CleanupExpiredVerificationTokens(_ context.Context) error {
	f.verifications.Add(1)
	return nil
}

func (f *fakeAuthPruner) CleanupOldRegistrationAttempts(_ context.Context) error {
	f.attempts.Add(1)
	return nil
}

func TestAuthWorker_RunsAllThreePrunes(t *testing.T) {
	pruner := &fakeAuthPruner{}
	w := NewAuthWorker(pruner, testLogger(), 25*time.Millisecond)

	cancel := w.Start(context.Background())
	defer cancel()

	assert.Eventually(t, func() bool {
		return pruner.tokens.Load() >= 1 && pruner.verifications.Load() >= 1 && pruner.attempts.Load() >= 1
	}, 2*time.Second, 10*time.Millisecond, "every tick should prune all three tables")
}

func TestAuthWorker_ErrorInOnePruneDoesNotSkipTheRest(t *testing.T) {
	// The three prunes are independent housekeeping tasks that merely share a
	// schedule, so a failure in the first must not starve the other two.
	pruner := &fakeAuthPruner{tokensErr: errors.New("database unavailable")}
	w := NewAuthWorker(pruner, testLogger(), 25*time.Millisecond)

	cancel := w.Start(context.Background())
	defer cancel()

	assert.Eventually(t, func() bool {
		return pruner.verifications.Load() >= 1 && pruner.attempts.Load() >= 1
	}, 2*time.Second, 10*time.Millisecond, "a failing prune must not skip the others")
}

func TestAuthWorker_PanicDoesNotKillTheLoop(t *testing.T) {
	pruner := &fakeAuthPruner{tokensPanics: true}
	w := NewAuthWorker(pruner, testLogger(), 25*time.Millisecond)

	cancel := w.Start(context.Background())
	defer cancel()

	assert.Eventually(t, func() bool {
		return pruner.tokens.Load() >= 3
	}, 2*time.Second, 10*time.Millisecond, "the loop must keep ticking after a panicking tick")
}

func TestAuthWorker_StopsOnCancel(t *testing.T) {
	pruner := &fakeAuthPruner{}
	w := NewAuthWorker(pruner, testLogger(), 25*time.Millisecond)

	cancel := w.Start(context.Background())
	assert.Eventually(t, func() bool {
		return pruner.tokens.Load() >= 1
	}, 2*time.Second, 10*time.Millisecond)

	cancel()
	time.Sleep(100 * time.Millisecond)
	settled := pruner.tokens.Load()
	time.Sleep(150 * time.Millisecond)

	assert.Equal(t, settled, pruner.tokens.Load(), "no ticks should run after cancel")
}

func TestNewAuthWorker_DefaultsInterval(t *testing.T) {
	w := NewAuthWorker(&fakeAuthPruner{}, testLogger(), -1)
	assert.Equal(t, DefaultInterval, w.interval, "a non-positive interval should fall back to the default")
}

// fakeSessionPruner records calls so the loop can be exercised without a
// database.
type fakeSessionPruner struct {
	callCount  atomic.Int32
	returnErr  error
	panicUntil int32
}

func (f *fakeSessionPruner) CleanupExpiredSessions(_ context.Context) error {
	n := f.callCount.Add(1)
	if n <= f.panicUntil {
		panic("nil dereference in session cleanup")
	}
	return f.returnErr
}

func TestSessionWorker_RunsOnStartup(t *testing.T) {
	// Unlike the other two cleanup workers this keeps the startup run the
	// inline loop it replaced had: expired sessions are rows that should
	// already be gone.
	pruner := &fakeSessionPruner{}
	w := NewSessionWorker(pruner, testLogger(), time.Hour)

	cancel := w.Start(context.Background())
	defer cancel()

	assert.Eventually(t, func() bool {
		return pruner.callCount.Load() >= 1
	}, 2*time.Second, 10*time.Millisecond, "startup run should not wait for the first tick")
}

func TestSessionWorker_FiresOnInterval(t *testing.T) {
	pruner := &fakeSessionPruner{}
	w := NewSessionWorker(pruner, testLogger(), 25*time.Millisecond)

	cancel := w.Start(context.Background())
	defer cancel()

	assert.Eventually(t, func() bool {
		return pruner.callCount.Load() >= 3
	}, 2*time.Second, 10*time.Millisecond, "worker should prune on each tick")
}

func TestSessionWorker_ContinuesAfterError(t *testing.T) {
	pruner := &fakeSessionPruner{returnErr: errors.New("database unavailable")}
	w := NewSessionWorker(pruner, testLogger(), 25*time.Millisecond)

	cancel := w.Start(context.Background())
	defer cancel()

	assert.Eventually(t, func() bool {
		return pruner.callCount.Load() >= 3
	}, 2*time.Second, 10*time.Millisecond, "an error must not stop the loop")
}

func TestSessionWorker_StartupPanicDoesNotPreventTicking(t *testing.T) {
	// The startup run is wrapped too: an unrecovered panic there would stop the
	// ticker from ever being created, which is exactly the silent permanent
	// stall per-tick recovery exists to avoid.
	pruner := &fakeSessionPruner{panicUntil: 1}
	w := NewSessionWorker(pruner, testLogger(), 25*time.Millisecond)

	cancel := w.Start(context.Background())
	defer cancel()

	assert.Eventually(t, func() bool {
		return pruner.callCount.Load() >= 3
	}, 2*time.Second, 10*time.Millisecond, "a panicking startup run must not stop the loop")
}

func TestSessionWorker_PanicIsLogged(t *testing.T) {
	logger, buf := captureLogger()
	pruner := &fakeSessionPruner{panicUntil: 1}
	w := NewSessionWorker(pruner, logger, 25*time.Millisecond)

	cancel := w.Start(context.Background())
	defer cancel()

	assert.Eventually(t, func() bool {
		return strings.Contains(buf.String(), "session-cleanup-tick")
	}, 2*time.Second, 10*time.Millisecond, "the recovered panic must name its unit")
}

func TestSessionWorker_StopsOnCancel(t *testing.T) {
	pruner := &fakeSessionPruner{}
	w := NewSessionWorker(pruner, testLogger(), 25*time.Millisecond)

	cancel := w.Start(context.Background())
	assert.Eventually(t, func() bool {
		return pruner.callCount.Load() >= 1
	}, 2*time.Second, 10*time.Millisecond)

	cancel()
	time.Sleep(100 * time.Millisecond)
	settled := pruner.callCount.Load()
	time.Sleep(150 * time.Millisecond)

	assert.Equal(t, settled, pruner.callCount.Load(), "no ticks should run after cancel")
}

func TestNewSessionWorker_DefaultsInterval(t *testing.T) {
	w := NewSessionWorker(&fakeSessionPruner{}, testLogger(), 0)
	assert.Equal(t, DefaultSessionInterval, w.interval, "a non-positive interval should fall back to the default")
}
