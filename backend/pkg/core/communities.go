package core

import (
	"regexp"
	"strings"
	"time"
)

// Community is a tenant-like grouping that owns games, a moderator roster, a
// banlist, and its own documentation.
//
// Membership is deliberately OPEN: there is no roster and no allowlist. Anyone
// not banned may join or create games in a community. The banlist is the whole
// access-control mechanism -- negative space only.
type Community struct {
	ID            int32     `json:"id"`
	Name          string    `json:"name"`
	Slug          string    `json:"slug"`        // URL identifier; immutable after creation
	Description   *string   `json:"description"` // Markdown blurb shown on the community page
	BannerURL     *string   `json:"banner_url"`  // Optional header image
	OwnerUserID   int32     `json:"owner_user_id"`
	OwnerUsername string    `json:"owner_username,omitempty"` // Populated by list queries
	IsActive      bool      `json:"is_active"`                // Inactive communities accept no new games
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	GameCount     *int64    `json:"game_count,omitempty"` // Populated only where the caller asked for it

	// YourRole is the REQUESTING user's standing in this community: "",
	// "moderator", or "owner". It is a property of the request, not of the
	// community, so it is computed per response and never stored.
	//
	// It exists because the community record names only the owner. Without it a
	// client cannot distinguish a moderator from an ordinary visitor, and the
	// only alternative signal -- whether the moderator-gated roster endpoint
	// 403s -- means firing a request expected to fail for most viewers.
	//
	// NOT omitempty: "" is the meaningful "no standing" value, and omitting it
	// would make an absent field ambiguous between "no role" and "this endpoint
	// does not populate it".
	//
	// This reflects ADMIN MODE, which is per-request: a site admin browsing
	// normally sees "" here and gets "owner" only with admin mode enabled. That
	// is precisely why the role rides on the response rather than on a cached
	// login payload.
	YourRole CommunityRole `json:"your_role"`
}

// CommunityModerator is a user granted moderation powers over a community.
//
// The community OWNER is never a row here -- ownership lives in
// Community.OwnerUserID. See CommunityRole for why that split exists.
type CommunityModerator struct {
	ID                int32     `json:"id"`
	CommunityID       int32     `json:"community_id"`
	UserID            int32     `json:"user_id"`
	Username          string    `json:"username"`
	DisplayName       *string   `json:"display_name,omitempty"`
	AvatarURL         *string   `json:"avatar_url,omitempty"`
	GrantedByUserID   *int32    `json:"granted_by_user_id,omitempty"`
	GrantedByUsername *string   `json:"granted_by_username,omitempty"`
	GrantedAt         time.Time `json:"granted_at"`
}

// CommunityRole is a user's standing within one community.
//
// Owner and moderator are two tiers rather than one enum with an implicit
// ordering, because the requirement that separates them is narrow and precise:
// moderators may do everything a community owner can EXCEPT manage the
// moderator roster. Keeping owner out of community_moderators makes that a
// clean check instead of a rank comparison.
type CommunityRole string

const (
	// CommunityRoleNone means the user is neither owner nor moderator.
	CommunityRoleNone CommunityRole = ""

	// CommunityRoleModerator may manage bans, documents, webhooks, and the
	// community profile -- but NOT the moderator roster.
	CommunityRoleModerator CommunityRole = "moderator"

	// CommunityRoleOwner may do everything a moderator can, plus manage the
	// moderator roster.
	CommunityRoleOwner CommunityRole = "owner"
)

// CreateCommunityRequest is the admin-only payload for creating a community.
// Only site admins may create communities and assign their owner.
//
// BannerURL is absent: a banner is uploaded after the community exists, through
// the dedicated banner endpoint, not typed in at creation time.
type CreateCommunityRequest struct {
	Name        string  `json:"name"`
	Slug        string  `json:"slug"`
	Description *string `json:"description,omitempty"`
	OwnerUserID int32   `json:"owner_user_id"`
}

// UpdateCommunityRequest is a partial update. A nil field is left unchanged.
//
// Slug is absent on purpose: it is immutable after creation because it appears
// in URLs communities will have shared externally.
type UpdateCommunityRequest struct {
	Name *string `json:"name,omitempty"`

	// Description is tri-state on purpose. Omitted (nil) leaves the blurb
	// alone; a non-nil value sets it; and an EMPTY STRING clears it back to
	// NULL. Without the empty-string case a description would be
	// write-once-then-permanent, since nil alone cannot distinguish "unchanged"
	// from "remove it". This matches UpdateGame, which also treats a blank
	// description as no description.
	Description *string `json:"description,omitempty"`

	OwnerUserID *int32 `json:"owner_user_id,omitempty"`
	IsActive    *bool  `json:"is_active,omitempty"`

	// BannerURL is absent on purpose. Banners are uploaded objects whose file
	// and column must stay in sync, so they are written only through a
	// dedicated upload/delete endpoint -- never through a general PATCH.
}

// Slug constraints. Lowercase alphanumeric with single interior hyphens, which
// keeps community URLs unambiguous and safe to embed in a path segment.
const (
	CommunitySlugMinLength = 2
	CommunitySlugMaxLength = 100
	CommunityNameMinLength = 2
	CommunityNameMaxLength = 255
)

// communitySlugPattern permits lowercase letters, digits, and interior hyphens.
// It rejects leading/trailing hyphens and consecutive hyphens.
var communitySlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// ValidateCommunitySlug reports whether s is a usable community slug.
// Returns a human-readable reason when it is not.
//
// This is the semantic check that a struct tag cannot express; length bounds
// are enforced upstream by the request schema.
func ValidateCommunitySlug(s string) (bool, string) {
	if len(s) < CommunitySlugMinLength {
		return false, "slug must be at least 2 characters"
	}
	if len(s) > CommunitySlugMaxLength {
		return false, "slug must be at most 100 characters"
	}
	if !communitySlugPattern.MatchString(s) {
		return false, "slug must be lowercase letters, digits, and single interior hyphens (e.g. midnight-ravens)"
	}
	if IsReservedCommunitySlug(s) {
		return false, "that slug is reserved"
	}
	return true, ""
}

// reservedCommunitySlugs are slugs that would collide with sibling routes under
// /communities/... or read as a site-level page rather than a community.
var reservedCommunitySlugs = map[string]bool{
	"new": true, "create": true, "edit": true, "admin": true, "manage": true,
	"settings": true, "api": true, "docs": true, "doc": true, "documents": true,
	"guidelines": true, "site-guidelines": true, "community-guidelines": true,
	"games": true, "moderators": true, "bans": true, "webhooks": true,
}

// IsReservedCommunitySlug reports whether s is reserved for site use.
func IsReservedCommunitySlug(s string) bool {
	return reservedCommunitySlugs[strings.ToLower(strings.TrimSpace(s))]
}

// SuggestCommunitySlug derives a candidate slug from a community name. It is a
// convenience for the create form, not a validator -- the result must still
// pass ValidateCommunitySlug, since a name of only punctuation yields "".
func SuggestCommunitySlug(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))

	var b strings.Builder
	lastHyphen := false
	for _, r := range lower {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastHyphen = false
		default:
			// Collapse any run of non-alphanumerics into one hyphen.
			if !lastHyphen && b.Len() > 0 {
				b.WriteByte('-')
				lastHyphen = true
			}
		}
	}

	out := strings.Trim(b.String(), "-")
	if len(out) > CommunitySlugMaxLength {
		out = strings.Trim(out[:CommunitySlugMaxLength], "-")
	}
	return out
}
