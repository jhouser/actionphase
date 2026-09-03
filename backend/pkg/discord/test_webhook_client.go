package discord

import (
	"context"
	"fmt"
	"sync"
	"time"

	"actionphase/pkg/core"
	"actionphase/pkg/observability"
)

// SentWebhook records one webhook POST dispatched via MockWebhookClient.
type SentWebhook struct {
	URL   string
	Embed core.DiscordEmbed
}

// MockWebhookClient implements core.DiscordWebhookSender for tests.
//
// Named test_*.go rather than mock_*.go on purpose: it is imported by tests in
// two other packages, so it cannot be a _test.go file, and unlike MockClient next
// door it has NO production instantiation to make it reachable -- webhooks
// deliberately ship without a mock fallback, since a mock that reports success
// for an undelivered webhook is worse than no feature (see main.go). The
// deadcode analyzer therefore sees it as unreachable, and the `test_` prefix is
// what the dead-code recipe excludes.
//
// Beyond recording sends it can simulate the two failure modes the dispatcher
// must survive: a hard failure (ShouldFail) and a slow endpoint (Delay). The
// latter exists because "a hanging webhook must not delay the transition" is a
// property no amount of happy-path testing demonstrates.
type MockWebhookClient struct {
	mu   sync.Mutex
	sent []SentWebhook

	// ShouldFail makes every Send return an error.
	ShouldFail bool

	// FailTimes makes the first N sends fail, then succeed. Drives the retry
	// tests: a webhook that fails twice and succeeds on the third attempt is
	// the case that distinguishes real retrying from a single attempt.
	FailTimes int
	attempts  int

	// Delay blocks each Send, simulating a slow or hanging Discord endpoint.
	Delay time.Duration

	Logger *observability.Logger
}

var _ core.DiscordWebhookSender = (*MockWebhookClient)(nil)

// Send records the webhook POST, honouring the configured failure and delay
// behaviour.
func (m *MockWebhookClient) Send(ctx context.Context, webhookURL string, embed core.DiscordEmbed) error {
	m.mu.Lock()
	m.attempts++
	attempt := m.attempts
	failTimes := m.FailTimes
	delay := m.Delay
	m.mu.Unlock()

	if delay > 0 {
		// Honour cancellation while delaying: a test asserting that the dispatch
		// timeout actually bounds a hanging endpoint needs this to return early
		// rather than sleep past the deadline.
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if m.ShouldFail {
		return fmt.Errorf("discord webhook mock: forced failure")
	}
	if attempt <= failTimes {
		return fmt.Errorf("discord webhook mock: forced failure (attempt %d)", attempt)
	}

	m.mu.Lock()
	m.sent = append(m.sent, SentWebhook{URL: webhookURL, Embed: embed})
	m.mu.Unlock()

	return nil
}

// Sent returns a copy of all recorded webhook sends. Safe for concurrent use --
// which matters here, since dispatch runs on a detached goroutine and a test
// reading this races the dispatcher by construction.
func (m *MockWebhookClient) Sent() []SentWebhook {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]SentWebhook, len(m.sent))
	copy(out, m.sent)
	return out
}

// Reset clears recorded sends, attempts, and failure behaviour.
//
// Needed because a router built once per test captures a single sender, so
// state from an earlier test would otherwise be read as this one's.
func (m *MockWebhookClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = nil
	m.attempts = 0
	m.ShouldFail = false
	m.FailTimes = 0
	m.Delay = 0
}

// Attempts returns how many times Send was called, successes and failures
// alike. Distinguishes "retried three times" from "sent once" -- the sent list
// alone cannot, since failed attempts record nothing.
func (m *MockWebhookClient) Attempts() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.attempts
}
