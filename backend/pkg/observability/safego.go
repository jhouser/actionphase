package observability

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
)

// SafeRun runs fn, converting a panic into a logged error rather than a process
// crash.
//
// Go has no per-goroutine isolation: an unrecovered panic in any goroutine
// terminates the whole process. ErrorRecoveryMiddleware protects request
// handlers, but nothing sits above background work, so a nil dereference in a
// notification path can take the API server down for everyone.
//
// Intended for one unit of background work — a single worker tick, not the loop
// around it. Wrapping an entire ticker goroutine would convert a loud crash into
// a silent permanent stall: the loop would exit, nothing would restart it, and
// no player or GM would find out. Per-tick recovery instead skips one bad row
// and lets the next tick proceed normally.
//
// http.ErrAbortHandler is deliberately not special-cased here: it is a
// net/http request-path sentinel with no meaning off the request path.
func SafeRun(ctx context.Context, logger *Logger, name string, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			// The logger is the thing we panic-log through, so a nil one would
			// turn a recovered panic back into a fatal one. Fall back to slog's
			// default rather than losing the report.
			stack := string(debug.Stack())
			if logger == nil {
				slog.Default().Error("Background work panicked",
					"error", fmt.Sprintf("panic in %s: %v", name, r),
					"unit", name,
					"stack_trace", stack)
				return
			}
			logger.LogError(ctx, fmt.Errorf("panic in %s: %v", name, r),
				"Background work panicked",
				"unit", name,
				"stack_trace", stack)
		}
	}()

	fn()
}

// SafeGo runs fn in a new goroutine under SafeRun. Use it for fire-and-forget
// work spawned from a request handler: the request has already returned to its
// client, so a panic there would crash the server while it is serving unrelated
// traffic, with no one waiting to observe the failure.
func SafeGo(ctx context.Context, logger *Logger, name string, fn func()) {
	go SafeRun(ctx, logger, name, fn)
}
