package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The URL validator is an SSRF control, so these cases are grouped by the
// attack each one closes rather than by input shape. A regression here does not
// look like a bug -- it looks like the server making arbitrary outbound POSTs.
func TestValidateWebhookURL(t *testing.T) {
	const valid = "https://discord.com/api/webhooks/123456789/abcdefTOKEN"

	t.Run("accepts real Discord webhook URLs", func(t *testing.T) {
		cases := []string{
			valid,
			"https://discordapp.com/api/webhooks/123/tok",
			"https://ptb.discord.com/api/webhooks/123/tok",
			"https://canary.discord.com/api/webhooks/123/tok",
			// Versioned API path, which Discord's own UI hands out.
			"https://discord.com/api/v10/webhooks/123/tok",
			// Uppercase host: DNS is case-insensitive, so rejecting this would
			// reject a legitimate paste, not close a hole.
			"https://DISCORD.COM/api/webhooks/123/tok",
		}
		for _, c := range cases {
			assert.NoError(t, ValidateWebhookURL(c), "should accept %s", c)
		}
	})

	t.Run("rejects non-Discord hosts", func(t *testing.T) {
		cases := map[string]string{
			"plain evil host":   "https://evil.test/api/webhooks/1/t",
			"suffix lookalike":  "https://discord.com.evil.test/api/webhooks/1/t",
			"prefix lookalike":  "https://notdiscord.com/api/webhooks/1/t",
			"substring in path": "https://evil.test/discord.com/api/webhooks/1/t",
			"internal service":  "https://backend/api/webhooks/1/t",
			"cloud metadata":    "https://169.254.169.254/api/webhooks/1/t",
			"localhost":         "https://localhost/api/webhooks/1/t",
			"discord as subdom": "https://discord.com.attacker.test/api/webhooks/1/t",
		}
		for name, c := range cases {
			assert.ErrorIs(t, ValidateWebhookURL(c), ErrInvalidWebhookURL, "case: %s (%s)", name, c)
		}
	})

	t.Run("rejects non-https schemes", func(t *testing.T) {
		// http would put the credential on the wire in cleartext; file/gopher
		// are the classic SSRF scheme pivots.
		for _, c := range []string{
			"http://discord.com/api/webhooks/1/t",
			"ftp://discord.com/api/webhooks/1/t",
			"file:///etc/passwd",
			"//discord.com/api/webhooks/1/t",
		} {
			assert.ErrorIs(t, ValidateWebhookURL(c), ErrInvalidWebhookURL, "should reject %s", c)
		}
	})

	t.Run("rejects embedded userinfo", func(t *testing.T) {
		// Parses with a Discord host but smuggles credentials, and some clients
		// resolve it differently than the host field suggests.
		assert.ErrorIs(t,
			ValidateWebhookURL("https://user:pass@discord.com/api/webhooks/1/t"),
			ErrInvalidWebhookURL)
	})

	t.Run("rejects non-webhook paths on an allowed host", func(t *testing.T) {
		// The host allowlist alone is not enough: without the path check, an
		// allowlisted host becomes a way to reach other Discord endpoints.
		for _, c := range []string{
			"https://discord.com/",
			"https://discord.com/api/users/@me",
			"https://discord.com/api/webhooks/onlyone",
			"https://discord.com/api/webhooks/1/t/extra",
			"https://discord.com/api/notwebhooks/1/t",
		} {
			assert.ErrorIs(t, ValidateWebhookURL(c), ErrInvalidWebhookURL, "should reject %s", c)
		}
	})

	t.Run("distinguishes empty from invalid", func(t *testing.T) {
		// Different mistakes deserve different messages: an empty box reads
		// differently to a moderator than a rejected one.
		assert.ErrorIs(t, ValidateWebhookURL(""), ErrWebhookURLRequired)
		assert.ErrorIs(t, ValidateWebhookURL("   "), ErrWebhookURLRequired)
		assert.ErrorIs(t, ValidateWebhookURL("not a url at all"), ErrInvalidWebhookURL)
	})

	t.Run("tolerates surrounding whitespace", func(t *testing.T) {
		// Pasted URLs routinely carry a trailing newline; that is not an attack.
		assert.NoError(t, ValidateWebhookURL("  "+valid+"\n"))
	})
}

// Masking is the other half of treating the URL as a credential. The assertion
// that matters in every case is the same: the token must not survive.
func TestMaskWebhookURL(t *testing.T) {
	// Token shares no substring with the webhook id below, so the leak scan
	// cannot mistake the deliberately-preserved id for exposed token bytes.
	const token = "abcdefghijklmnopqrstuvwxyzABCDEFGH"
	const full = "https://discord.com/api/webhooks/123456789/" + token

	t.Run("hides the token but keeps the row recognisable", func(t *testing.T) {
		masked := MaskWebhookURL(full)

		require.NotContains(t, masked, token, "the token must never survive masking")
		assert.Contains(t, masked, "discord.com", "host is structure, not secret")
		assert.Contains(t, masked, "123456789", "webhook id is not the credential")
		assert.Contains(t, masked, token[len(token)-webhookMaskVisibleChars:],
			"keeps a tail so two webhooks are distinguishable")
		assert.Contains(t, masked, "••••")
	})

	t.Run("leaks no more than the visible tail", func(t *testing.T) {
		masked := MaskWebhookURL(full)

		// Every window of the token longer than the allowed tail must be absent.
		for i := 0; i+webhookMaskVisibleChars+1 <= len(token); i++ {
			window := token[i : i+webhookMaskVisibleChars+1]
			assert.NotContains(t, masked, window,
				"masked URL exposes more than %d token chars", webhookMaskVisibleChars)
		}
	})

	t.Run("does not echo a malformed URL back", func(t *testing.T) {
		// The fallback must never be "return the input". Rows can predate
		// validation, and this is the one path meant to redact them.
		for _, raw := range []string{
			"total-garbage",
			"https://discord.com",
			"://",
		} {
			masked := MaskWebhookURL(raw)
			assert.NotEqual(t, raw, masked, "must not echo %q back unmasked", raw)
		}
	})

	t.Run("drops any query string or fragment", func(t *testing.T) {
		// Rebuilt from parsed parts rather than sliced, so a token cannot ride
		// along in a query parameter.
		masked := MaskWebhookURL(full + "?wait=true#frag")

		assert.NotContains(t, masked, token)
		assert.NotContains(t, masked, "wait=true")
		assert.NotContains(t, masked, "frag")
	})

	t.Run("masks a short token without exposing it whole", func(t *testing.T) {
		// A token shorter than the visible window must not simply pass through.
		masked := MaskWebhookURL("https://discord.com/api/webhooks/1/ab")

		assert.Contains(t, masked, "••••")
		assert.True(t, strings.HasPrefix(masked, "https://discord.com/"))
	})

	t.Run("returns empty for empty input", func(t *testing.T) {
		assert.Equal(t, "", MaskWebhookURL(""))
	})
}

func TestIsValidWebhookEvent(t *testing.T) {
	t.Run("accepts notifiable game states", func(t *testing.T) {
		for _, s := range ValidWebhookEvents {
			assert.True(t, IsValidWebhookEvent(s), "should accept %s", s)
		}
	})

	t.Run("rejects setup so unlisted games are never announced", func(t *testing.T) {
		// A game in setup is not yet public; announcing it would leak it into a
		// Discord channel before its GM has shown it to anyone.
		assert.False(t, IsValidWebhookEvent(GameStateSetup))
	})

	t.Run("rejects values that are not game states", func(t *testing.T) {
		for _, s := range []string{"", "nonsense", "IN_PROGRESS", "in progress"} {
			assert.False(t, IsValidWebhookEvent(s), "should reject %q", s)
		}
	})
}

// Confirms neither security function panics on hostile input, independent of
// what the other one rejects. Masking in particular must survive rows written
// before validation existed.
func TestWebhookFunctions_NoPanicOnHostileInput(t *testing.T) {
	inputs := []string{
		"", " ", "/", "//", "///", "://", "h", "https://",
		"https://discord.com", "https://discord.com/", "https://discord.com//",
		"https://discord.com/api", "https://discord.com/api/webhooks",
		"https://discord.com/api/webhooks/", "https://discord.com/api/webhooks//",
		"https://discord.com/////", "not a url", "%%%", "https://%41",
		"https://discord.com/api/v/webhooks/1/t",
		"\x00", "https://discord.com/\x00/x",
	}
	for _, in := range inputs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("ValidateWebhookURL panicked on %q: %v", in, r)
				}
			}()
			_ = ValidateWebhookURL(in)
		}()
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("MaskWebhookURL panicked on %q: %v", in, r)
				}
			}()
			_ = MaskWebhookURL(in)
		}()
	}
}
