package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateCommunitySlug(t *testing.T) {
	tests := []struct {
		name   string
		slug   string
		wantOK bool
		reason string // substring expected in the rejection reason
	}{
		{name: "simple lowercase", slug: "ravens", wantOK: true},
		{name: "interior hyphen", slug: "midnight-ravens", wantOK: true},
		{name: "multiple interior hyphens", slug: "the-midnight-ravens", wantOK: true},
		{name: "digits allowed", slug: "club42", wantOK: true},
		{name: "all digits allowed", slug: "1999", wantOK: true},
		{name: "minimum length", slug: "ab", wantOK: true},

		{name: "too short", slug: "a", wantOK: false, reason: "at least 2"},
		{name: "empty", slug: "", wantOK: false, reason: "at least 2"},
		{name: "too long", slug: strings.Repeat("a", 101), wantOK: false, reason: "at most 100"},
		// ValidateCommunitySlug is strict about case, but the admin handler
		// lowercases before calling it -- so "Ravens" is accepted at the API as
		// "ravens" rather than refused. See
		// TestCreateCommunity_NormalizesSlugCase for that contract.
		{name: "uppercase rejected by the validator itself", slug: "Ravens", wantOK: false, reason: "lowercase"},
		{name: "leading hyphen", slug: "-ravens", wantOK: false, reason: "lowercase"},
		{name: "trailing hyphen", slug: "ravens-", wantOK: false, reason: "lowercase"},
		{name: "consecutive hyphens", slug: "midnight--ravens", wantOK: false, reason: "lowercase"},
		{name: "underscore rejected", slug: "midnight_ravens", wantOK: false, reason: "lowercase"},
		{name: "space rejected", slug: "midnight ravens", wantOK: false, reason: "lowercase"},
		{name: "slash rejected", slug: "midnight/ravens", wantOK: false, reason: "lowercase"},
		{name: "dot rejected", slug: "midnight.ravens", wantOK: false, reason: "lowercase"},

		// Reserved slugs would collide with sibling routes under /communities/.
		{name: "reserved new", slug: "new", wantOK: false, reason: "reserved"},
		{name: "reserved admin", slug: "admin", wantOK: false, reason: "reserved"},
		{name: "reserved manage", slug: "manage", wantOK: false, reason: "reserved"},
		{name: "reserved games", slug: "games", wantOK: false, reason: "reserved"},
		{name: "reserved bans", slug: "bans", wantOK: false, reason: "reserved"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, reason := ValidateCommunitySlug(tt.slug)
			assert.Equal(t, tt.wantOK, ok, "slug %q: reason was %q", tt.slug, reason)
			if !tt.wantOK {
				assert.Contains(t, reason, tt.reason)
			} else {
				assert.Empty(t, reason)
			}
		})
	}
}

// Exactly-100 characters is the boundary the length check must accept, not
// reject. An off-by-one here would silently forbid a legal slug.
func TestValidateCommunitySlug_MaxLengthBoundary(t *testing.T) {
	ok, reason := ValidateCommunitySlug(strings.Repeat("a", CommunitySlugMaxLength))
	assert.True(t, ok, "a slug of exactly the max length must be accepted, got: %s", reason)
}

func TestSuggestCommunitySlug(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "simple", in: "Ravens", want: "ravens"},
		{name: "spaces become hyphens", in: "Midnight Ravens", want: "midnight-ravens"},
		{name: "punctuation collapsed", in: "The Midnight Ravens!", want: "the-midnight-ravens"},
		{name: "runs collapse to one hyphen", in: "A   B", want: "a-b"},
		{name: "leading punctuation trimmed", in: "  !!Ravens", want: "ravens"},
		{name: "trailing punctuation trimmed", in: "Ravens...", want: "ravens"},
		{name: "digits kept", in: "Club 42", want: "club-42"},
		{name: "ampersand collapses", in: "Cloak & Dagger", want: "cloak-dagger"},

		// A name of only punctuation yields "", which is why the suggestion is
		// documented as a convenience and not a validator.
		{name: "punctuation only yields empty", in: "!!!", want: ""},
		{name: "empty yields empty", in: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, SuggestCommunitySlug(tt.in))
		})
	}
}

// The suggestion is only a convenience, but whatever it produces from an
// ordinary name must be something ValidateCommunitySlug accepts -- otherwise
// the create form would pre-fill a value it then rejects.
func TestSuggestCommunitySlug_OutputPassesValidation(t *testing.T) {
	names := []string{
		"Midnight Ravens",
		"The Midnight Ravens!",
		"Cloak & Dagger",
		"Club 42",
		"A  Very   Long    Community Name With Spaces",
	}

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			slug := SuggestCommunitySlug(name)
			ok, reason := ValidateCommunitySlug(slug)
			assert.True(t, ok, "suggested slug %q from %q was rejected: %s", slug, name, reason)
		})
	}
}

// Truncation must not leave a trailing hyphen, which would fail validation.
func TestSuggestCommunitySlug_TruncatesWithoutTrailingHyphen(t *testing.T) {
	// Build a name whose 100th character lands on a separator.
	name := strings.Repeat("ab ", 60)

	slug := SuggestCommunitySlug(name)
	assert.LessOrEqual(t, len(slug), CommunitySlugMaxLength)
	assert.False(t, strings.HasSuffix(slug, "-"), "truncated slug %q must not end in a hyphen", slug)

	ok, reason := ValidateCommunitySlug(slug)
	assert.True(t, ok, "truncated slug %q was rejected: %s", slug, reason)
}

func TestIsReservedCommunitySlug(t *testing.T) {
	assert.True(t, IsReservedCommunitySlug("new"))
	assert.True(t, IsReservedCommunitySlug("NEW"), "reservation must be case-insensitive")
	assert.True(t, IsReservedCommunitySlug("  admin  "), "reservation must ignore surrounding space")
	assert.False(t, IsReservedCommunitySlug("ravens"))
}
