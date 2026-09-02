package core

import (
	"net/url"
	"strings"
	"time"
)

// CommunityWebhook is a Discord channel a community announces its games' state
// transitions into (req 9).
//
// Delivery is BEST-EFFORT: there is no queue, no delivery table, and no
// redelivery after a restart. The three Last* fields below are the entire
// observability story, and they exist to answer the one question a moderator
// actually asks -- "my webhook stopped working, why?"
type CommunityWebhook struct {
	ID          int32 `json:"id"`
	CommunityID int32 `json:"community_id"`

	// URL is MASKED on the way out and never carries the real credential to a
	// client. Anyone holding a Discord webhook URL can post to that channel, so
	// it is treated as a password: write-only from the API's perspective.
	// Populated by MaskWebhookURL -- see the converter, which is the single
	// place a stored URL becomes this field.
	URL string `json:"url"`

	// Label is the moderator's own name for the channel ("#recruitment"), so a
	// community with several webhooks can tell masked URLs apart. Optional: one
	// webhook needs no disambiguation.
	Label *string `json:"label,omitempty"`

	IsEnabled bool `json:"is_enabled"`

	// Events are the game states this webhook fires on. Empty means it fires for
	// nothing -- a valid configuration (a moderator staging a webhook before
	// choosing its events), not an error.
	Events []string `json:"events"`

	// Delivery status. LastSuccessAt survives a later failure on purpose:
	// "worked at 09:00, broken since 14:00" is the useful diagnosis, and
	// clearing it would lose the half saying this config ever worked.
	LastSuccessAt *time.Time `json:"last_success_at,omitempty"`
	LastError     *string    `json:"last_error,omitempty"`
	LastErrorAt   *time.Time `json:"last_error_at,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Webhook label bound, matching the column width.
const WebhookLabelMaxLength = 100

// WebhookDispatchTimeout bounds the whole detached dispatch -- every retry and
// backoff sleep included. Generous for a single POST because it covers up to
// WebhookMaxAttempts of them; nothing waits on it, so the only cost of the
// ceiling is how long a dead endpoint occupies one goroutine.
const WebhookDispatchTimeout = 10 * time.Second

// Retry policy, in-goroutine and bounded by WebhookDispatchTimeout.
//
// A fixed delay rather than exponential backoff: with 3 attempts inside a 10s
// ceiling, the difference between fixed and exponential is a few seconds and
// nobody is waiting on either. Retries cover a transient blip; a genuinely
// broken URL is meant to end up in LastError, not to be retried into success.
const (
	WebhookMaxAttempts = 3
	WebhookRetryDelay  = 2 * time.Second
)

// ValidWebhookEvents is the set of game states a webhook may subscribe to.
//
// Not every game state: `setup` is excluded because a game in setup is not yet
// public -- announcing it would leak an unlisted game into a Discord channel
// before its GM has shown it to anyone.
var ValidWebhookEvents = []string{
	GameStateRecruitment,
	GameStateCharacterCreation,
	GameStateInProgress,
	GameStatePaused,
	GameStateEpilogue,
	GameStateCompleted,
	GameStateCancelled,
}

// IsValidWebhookEvent reports whether s is a game state a webhook may subscribe
// to.
func IsValidWebhookEvent(s string) bool {
	for _, e := range ValidWebhookEvents {
		if e == s {
			return true
		}
	}
	return false
}

// discordWebhookHosts are the only hosts a webhook URL may point at.
//
// This allowlist IS the SSRF control. The URL is moderator-supplied and the
// server POSTs to it from inside the network, so without this the feature is an
// open outbound request proxy: a moderator could point it at an internal
// service, a cloud metadata endpoint, or any third-party host and use the
// server as a confused deputy.
var discordWebhookHosts = []string{
	"discord.com",
	"discordapp.com",
	"ptb.discord.com",
	"canary.discord.com",
}

// ValidateWebhookURL checks that a moderator-supplied URL is a Discord webhook
// endpoint, and is the ONLY thing standing between this feature and an SSRF
// primitive.
//
// Called at save time AND again at dispatch time. The second check is not
// redundant: rows outlive the validation code, so a row written before this
// function tightened would otherwise be dispatched to unchecked forever.
//
// Rules, each one load-bearing:
//   - https only. Plain http would send the credential in cleartext, and
//     allowing it invites `http://` targets that skip TLS verification.
//   - Host must be exactly a Discord host. Compared against an allowlist after
//     stripping any port and lowercasing -- NOT a suffix match, which
//     "discord.com.evil.test" would satisfy.
//   - Path must look like /api/webhooks/{id}/{token}. This is what stops the
//     allowlisted host from being used to reach some other Discord endpoint.
//   - No userinfo. "https://user:pass@discord.com/..." parses with a Discord
//     host but lets an attacker smuggle credentials, and some clients resolve
//     such URLs differently than the host field suggests.
func ValidateWebhookURL(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ErrWebhookURLRequired
	}

	u, err := url.Parse(trimmed)
	if err != nil {
		return ErrInvalidWebhookURL
	}

	if u.Scheme != "https" {
		return ErrInvalidWebhookURL
	}

	if u.User != nil {
		return ErrInvalidWebhookURL
	}

	host := strings.ToLower(u.Hostname())
	allowed := false
	for _, h := range discordWebhookHosts {
		if host == h {
			allowed = true
			break
		}
	}
	if !allowed {
		return ErrInvalidWebhookURL
	}

	// Expect /api/webhooks/{id}/{token}, optionally under a version segment
	// (/api/v10/webhooks/...). Anything else on an allowlisted host is rejected.
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) > 0 && parts[0] == "api" {
		parts = parts[1:]
	}
	if len(parts) > 0 && strings.HasPrefix(parts[0], "v") && len(parts[0]) > 1 {
		parts = parts[1:]
	}
	if len(parts) != 3 || parts[0] != "webhooks" {
		return ErrInvalidWebhookURL
	}
	if parts[1] == "" || parts[2] == "" {
		return ErrInvalidWebhookURL
	}

	return nil
}

// webhookMaskVisibleChars is how much of the token's tail a masked URL keeps.
//
// Four is enough to tell two webhooks apart when re-checking a config, and far
// too little to reconstruct a token that is ~68 characters of entropy.
const webhookMaskVisibleChars = 4

// MaskWebhookURL renders a webhook URL safe to return to a client.
//
// The token is the credential, so the mask keeps only its last few characters:
// enough for a moderator to recognise WHICH webhook a row is, never enough to
// post to the channel. Everything before the token is structure, not secret.
//
// A URL too malformed to parse collapses to a bare mask rather than being
// echoed back -- the fallback must never be "return the input", or a row that
// predates validation would leak in full through the one path meant to redact.
func MaskWebhookURL(raw string) string {
	const fallback = "••••"

	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	u, err := url.Parse(trimmed)
	if err != nil {
		return fallback
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return fallback
	}

	token := parts[len(parts)-1]
	if token == "" {
		return fallback
	}

	visible := token
	if len(token) > webhookMaskVisibleChars {
		visible = token[len(token)-webhookMaskVisibleChars:]
	}

	// Rebuild from the parsed pieces rather than string-slicing the input, so a
	// URL carrying a query string or fragment cannot smuggle the token through.
	head := strings.Join(parts[:len(parts)-1], "/")
	return u.Scheme + "://" + u.Host + "/" + head + "/" + fallback + visible
}

// CreateCommunityWebhookRequest registers a webhook on a community.
type CreateCommunityWebhookRequest struct {
	// URL is the real Discord webhook URL. This is the ONLY direction it travels
	// in cleartext -- responses mask it.
	URL   string  `json:"url"`
	Label *string `json:"label,omitempty"`

	// IsEnabled omitted means ENABLED: a moderator who just pasted a URL and
	// chose events wants it live, and the disabled state exists for switching an
	// existing webhook off, not for staging a new one.
	IsEnabled *bool `json:"is_enabled,omitempty"`

	// Events omitted means NO events. A webhook that fires for everything by
	// default would spam a channel the first time any game moved.
	Events []string `json:"events,omitempty"`
}

// UpdateCommunityWebhookRequest is a partial update; a nil field is unchanged.
//
// URL is settable (a moderator rotating a regenerated Discord URL) but never
// clearable: the column is NOT NULL and a webhook without a URL has nothing to
// deliver to. Omitting it keeps the stored credential, which is what lets the
// config form save a label or event change WITHOUT the client ever holding the
// secret -- it only ever received a mask.
type UpdateCommunityWebhookRequest struct {
	URL       *string  `json:"url,omitempty"`
	Label     *string  `json:"label,omitempty"`
	IsEnabled *bool    `json:"is_enabled,omitempty"`
	Events    []string `json:"events,omitempty"`
}
