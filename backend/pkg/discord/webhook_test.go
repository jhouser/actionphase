package discord

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"actionphase/pkg/core"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The client only ever talks to a real Discord host in production, but
// ValidateWebhookURL runs inside Send, so tests cannot point it at httptest's
// 127.0.0.1 URL. clientTo rewires the transport instead: the request carries a
// valid Discord URL, and the RoundTripper redirects it to the test server.
func clientTo(t *testing.T, srv *httptest.Server) *WebhookClient {
	t.Helper()
	target, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	require.NoError(t, err)

	return &WebhookClient{
		httpClient: &http.Client{
			Transport: rewriteHost{host: target.URL.Host, base: srv.Client().Transport},
		},
	}
}

type rewriteHost struct {
	host string
	base http.RoundTripper
}

func (r rewriteHost) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Scheme = "http"
	req.URL.Host = r.host
	base := r.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}

const testWebhookURL = "https://discord.com/api/webhooks/123/testtoken"

func TestWebhookClient_Send(t *testing.T) {
	t.Run("posts the embed as Discord expects", func(t *testing.T) {
		var gotBody []byte
		var gotMethod, gotContentType string

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotContentType = r.Header.Get("Content-Type")
			gotBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		err := clientTo(t, srv).Send(context.Background(), testWebhookURL, core.DiscordEmbed{
			Title:       "Game started",
			URL:         "https://example.test/games/1",
			Description: "The Long Dark has begun",
			Color:       0x57F287,
			Footer:      "ActionPhase",
			Timestamp:   "2026-09-02T00:00:00Z",
		})
		require.NoError(t, err)

		assert.Equal(t, http.MethodPost, gotMethod)
		assert.Equal(t, "application/json", gotContentType)

		// Assert the shape Discord actually requires: embeds is an ARRAY, and
		// the footer is an object with a text field, not a bare string.
		var payload struct {
			Embeds []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
				Color       int    `json:"color"`
				Footer      struct {
					Text string `json:"text"`
				} `json:"footer"`
				Timestamp string `json:"timestamp"`
			} `json:"embeds"`
		}
		require.NoError(t, json.Unmarshal(gotBody, &payload))
		require.Len(t, payload.Embeds, 1)

		e := payload.Embeds[0]
		assert.Equal(t, "Game started", e.Title)
		assert.Equal(t, "https://example.test/games/1", e.URL)
		assert.Equal(t, "The Long Dark has begun", e.Description)
		assert.Equal(t, 0x57F287, e.Color)
		assert.Equal(t, "ActionPhase", e.Footer.Text)
		assert.Equal(t, "2026-09-02T00:00:00Z", e.Timestamp)
	})

	t.Run("rejects a non-Discord URL without making a request", func(t *testing.T) {
		// Re-validation at dispatch time. Rows outlive validation code, so a
		// webhook saved before the check tightened must still be refused here.
		var called bool
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		err := clientTo(t, srv).Send(context.Background(), "https://evil.test/api/webhooks/1/t", core.DiscordEmbed{})

		require.Error(t, err)
		assert.ErrorIs(t, err, core.ErrInvalidWebhookURL)
		assert.False(t, called, "no request may be made for a rejected URL")
	})

	t.Run("returns an error for a non-2xx response", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"Unknown Webhook","code":10015}`))
		}))
		defer srv.Close()

		err := clientTo(t, srv).Send(context.Background(), testWebhookURL, core.DiscordEmbed{})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "404")
		assert.Contains(t, err.Error(), "Unknown Webhook", "the moderator needs Discord's reason")
	})

	t.Run("caps how much of an error body it quotes", func(t *testing.T) {
		// last_error is shown to a moderator; an arbitrarily large error page
		// from something that is not Discord must not land there whole.
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(strings.Repeat("A", 100_000)))
		}))
		defer srv.Close()

		err := clientTo(t, srv).Send(context.Background(), testWebhookURL, core.DiscordEmbed{})

		require.Error(t, err)
		assert.Less(t, len(err.Error()), 1000, "error body must be truncated")
	})

	t.Run("surfaces a 429 with Discord's own retry interval", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Retry-After", "3")
			w.WriteHeader(http.StatusTooManyRequests)
		}))
		defer srv.Close()

		err := clientTo(t, srv).Send(context.Background(), testWebhookURL, core.DiscordEmbed{})

		var rl *RateLimitedError
		require.ErrorAs(t, err, &rl, "a 429 must be distinguishable from other failures")
		assert.Equal(t, 3*time.Second, rl.RetryAfter)
	})

	t.Run("reads retry_after from the body when no header is sent", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"retry_after":1.5}`))
		}))
		defer srv.Close()

		err := clientTo(t, srv).Send(context.Background(), testWebhookURL, core.DiscordEmbed{})

		var rl *RateLimitedError
		require.ErrorAs(t, err, &rl)
		assert.Equal(t, 1500*time.Millisecond, rl.RetryAfter)
	})

	t.Run("clamps an absurd retry interval", func(t *testing.T) {
		// A hostile or broken value must not park a dispatch goroutine for
		// hours; the dispatch budget is 10s regardless.
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Retry-After", "999999")
			w.WriteHeader(http.StatusTooManyRequests)
		}))
		defer srv.Close()

		err := clientTo(t, srv).Send(context.Background(), testWebhookURL, core.DiscordEmbed{})

		var rl *RateLimitedError
		require.ErrorAs(t, err, &rl)
		assert.LessOrEqual(t, rl.RetryAfter, 30*time.Second)
	})

	t.Run("never puts the webhook URL in an error string", func(t *testing.T) {
		// Error strings get logged and written to last_error. The URL is a
		// credential, so a connection failure must not leak it -- and
		// url.Error stringifies to include the URL by default.
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		srv.Close() // closed: the request will fail to connect

		err := clientTo(t, srv).Send(context.Background(), testWebhookURL, core.DiscordEmbed{})

		require.Error(t, err)
		assert.NotContains(t, err.Error(), "testtoken", "the token must never reach a log")
	})

	t.Run("honours context cancellation", func(t *testing.T) {
		// The handler blocks until released, simulating an endpoint that accepts
		// the connection and never answers. Released via the channel rather than
		// r.Context() so srv.Close() cannot deadlock waiting on this handler --
		// the client giving up does not by itself unblock it.
		release := make(chan struct{})
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-release
		}))
		defer srv.Close()
		defer close(release)

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		start := time.Now()
		err := clientTo(t, srv).Send(ctx, testWebhookURL, core.DiscordEmbed{})

		require.Error(t, err)
		assert.True(t, errors.Is(err, context.DeadlineExceeded),
			"a hanging endpoint must be bounded by the caller's context, got: %v", err)
		assert.Less(t, time.Since(start), 2*time.Second)
	})
}
