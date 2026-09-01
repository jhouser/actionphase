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

func TestIsReservedCommunitySlug(t *testing.T) {
	assert.True(t, IsReservedCommunitySlug("new"))
	assert.True(t, IsReservedCommunitySlug("NEW"), "reservation must be case-insensitive")
	assert.True(t, IsReservedCommunitySlug("  admin  "), "reservation must ignore surrounding space")
	assert.False(t, IsReservedCommunitySlug("ravens"))
}
