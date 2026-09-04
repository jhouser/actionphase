package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"actionphase/pkg/core"
	"actionphase/pkg/observability"
)

// WebhookClient posts embeds to Discord webhook URLs.
//
// Simpler than BotClient by nature: a webhook URL is itself the credential, so
// there is no bot token, no Authorization header, and no DM channel to open
// first -- just a POST to the URL with an embeds payload.
type WebhookClient struct {
	Logger     *observability.Logger
	httpClient *http.Client
}

var _ core.DiscordWebhookSender = (*WebhookClient)(nil)

// webhookHTTPTimeout bounds a SINGLE request attempt.
//
// Deliberately well under core.WebhookDispatchTimeout, which bounds the whole
// dispatch including retries: if one attempt could consume the entire budget, a
// single hanging endpoint would leave no room for the retries to happen at all.
const webhookHTTPTimeout = 5 * time.Second

func (c *WebhookClient) getHTTPClient() *http.Client {
	if c.httpClient != nil {
		return c.httpClient
	}
	return &http.Client{Timeout: webhookHTTPTimeout}
}

// RateLimitedError reports a Discord 429 and how long it asked us to wait.
//
// Surfaced as a distinct type so the dispatcher can honour Discord's own
// retry_after instead of its fixed delay. Guessing shorter than Discord asked
// earns another 429; guessing longer wastes the dispatch budget.
type RateLimitedError struct {
	RetryAfter time.Duration
}

func (e *RateLimitedError) Error() string {
	return fmt.Sprintf("discord webhook: rate limited, retry after %s", e.RetryAfter)
}

// Send posts an embed to a Discord webhook URL.
//
// Performs no retries -- that policy belongs to the dispatcher, which owns the
// overall timeout budget. Returns *RateLimitedError on a 429 so the caller can
// wait the interval Discord specified.
func (c *WebhookClient) Send(ctx context.Context, webhookURL string, embed core.DiscordEmbed) error {
	// Re-validate at dispatch time, not only at save time. Rows outlive the
	// validation code: a webhook stored before this check tightened would
	// otherwise keep being POSTed to unchecked forever, which is exactly the
	// SSRF case the save-time check exists to prevent.
	if err := core.ValidateWebhookURL(webhookURL); err != nil {
		return fmt.Errorf("discord webhook: %w", err)
	}

	body, err := json.Marshal(webhookPayloadFromEmbed(embed))
	if err != nil {
		return fmt.Errorf("discord webhook: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("discord webhook: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.getHTTPClient().Do(req)
	if err != nil {
		// The URL is a credential and error strings get logged, so never wrap an
		// error that embeds the request URL -- url.Error stringifies to include
		// it. Report the cause only.
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			return fmt.Errorf("discord webhook: request failed: %w", urlErr.Unwrap())
		}
		return fmt.Errorf("discord webhook: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return &RateLimitedError{RetryAfter: parseRetryAfter(resp)}
	}

	if resp.StatusCode >= 400 {
		// Cap the body read: an error page from something that is not Discord
		// could be arbitrarily large, and this string is written to last_error
		// and shown to a moderator.
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("discord webhook: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// parseRetryAfter reads Discord's rate-limit hint.
//
// Discord sends retry_after as seconds, in a JSON body and (usually) the
// Retry-After header. The header is preferred: reading it does not consume the
// response body, and it is present on the gateway responses that a JSON decode
// would fail on. Falls back to the fixed retry delay when absent or absurd, so
// a hostile or broken value cannot stall a dispatch goroutine.
func parseRetryAfter(resp *http.Response) time.Duration {
	const maxRetryAfter = 30 * time.Second

	if v := resp.Header.Get("Retry-After"); v != "" {
		if secs, err := time.ParseDuration(v + "s"); err == nil && secs > 0 {
			if secs > maxRetryAfter {
				return maxRetryAfter
			}
			return secs
		}
	}

	var payload struct {
		RetryAfter float64 `json:"retry_after"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 512)).Decode(&payload); err == nil {
		if payload.RetryAfter > 0 {
			d := time.Duration(payload.RetryAfter * float64(time.Second))
			if d > maxRetryAfter {
				return maxRetryAfter
			}
			return d
		}
	}

	return core.WebhookRetryDelay
}

// webhookPayload is the Discord webhook request body.
type webhookPayload struct {
	Embeds []webhookEmbed `json:"embeds"`
}

type webhookEmbed struct {
	Title       string             `json:"title"`
	URL         string             `json:"url,omitempty"`
	Description string             `json:"description,omitempty"`
	Color       int                `json:"color"`
	Footer      webhookEmbedFooter `json:"footer"`
	Timestamp   string             `json:"timestamp,omitempty"`
}

type webhookEmbedFooter struct {
	Text string `json:"text"`
}

func webhookPayloadFromEmbed(embed core.DiscordEmbed) webhookPayload {
	return webhookPayload{
		Embeds: []webhookEmbed{{
			Title:       embed.Title,
			URL:         embed.URL,
			Description: embed.Description,
			Color:       embed.Color,
			Footer:      webhookEmbedFooter{Text: embed.Footer},
			Timestamp:   embed.Timestamp,
		}},
	}
}
