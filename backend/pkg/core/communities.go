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

	// IsBanned reports whether the REQUESTING user is currently barred from
	// this community. Like YourRole it describes the request, not the
	// community, and is computed per response rather than stored.
	//
	// It exists so the game-creation picker can omit communities the create
	// endpoint would refuse. That is CONVENIENCE only -- the ban check on game
	// creation is the enforcement, and it stands regardless of this flag.
	//
	// "Currently" is load-bearing: an EXPIRED ban leaves a row behind
	// deliberately, so this is computed from the same expiry rule the
	// enforcement paths use and is false once a ban lapses. Never infer a ban
	// from a row's presence.
	//
	// Browsing is deliberately NOT filtered on this: a banned user still sees
	// the community and can read its profile. A ban blocks joining, not
	// looking.
	//
	// NOT omitempty: false is the meaningful "not banned" value, and omitting
	// it would be ambiguous with "this endpoint does not populate it".
	IsBanned bool `json:"is_banned"`
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

// CommunityBan excludes one user from one community's games.
//
// Bans are per-community by design -- that separation is the entire reason
// Communities exists. A user banned from one community plays freely in the
// others, and a game with no community (a legacy game) is never covered by any
// ban.
//
// Bans are NOT retroactive: a ban blocks a user from entering games, but does
// not eject them from games they already play in. Removing an existing
// participant stays a GM decision.
type CommunityBan struct {
	ID          int32   `json:"id"`
	CommunityID int32   `json:"community_id"`
	UserID      int32   `json:"user_id"`
	Username    string  `json:"username,omitempty"`     // Populated by list queries
	DisplayName *string `json:"display_name,omitempty"` // Populated by list queries
	AvatarURL   *string `json:"avatar_url,omitempty"`   // Populated by list queries
	Reason      *string `json:"reason,omitempty"`

	// BannedByUserID is nullable because the moderator who issued the ban may
	// since have been deleted (ON DELETE SET NULL). The ban belongs to the
	// community, not to its author, so it outlives them.
	BannedByUserID   *int32  `json:"banned_by_user_id,omitempty"`
	BannedByUsername *string `json:"banned_by_username,omitempty"`

	BannedAt time.Time `json:"banned_at"`

	// ExpiresAt nil means PERMANENT. A ban whose expiry has passed still exists
	// as a row -- it stays on the management list so a moderator can see it
	// lapsed rather than watching it vanish -- so never infer "banned" from the
	// row's presence. Use IsActive, or the service's IsUserBanned.
	ExpiresAt *time.Time `json:"expires_at,omitempty"`

	// IsActive is whether this ban is being ENFORCED right now: permanent, or
	// not yet expired. Computed at read time rather than stored, since it
	// changes with the clock and nothing writes to the row when it lapses.
	IsActive bool `json:"is_active"`
}

// CommunityBanEvent is one append-only entry in a community's ban audit log.
//
// The log is separate from community_bans because lifting a ban DELETES its
// row. Three communities sharing this deployment will have disputes about who
// banned whom, and that history cannot be reconstructed after the fact.
//
// Reason and ExpiresAt are SNAPSHOTS of the ban as it stood at event time, not
// live references: by the time an "unbanned" event is read, its ban row is gone.
type CommunityBanEvent struct {
	ID             int32   `json:"id"`
	CommunityID    int32   `json:"community_id"`
	TargetUserID   int32   `json:"target_user_id"`
	TargetUsername *string `json:"target_username,omitempty"`

	// ActorUserID is nullable: a deleted moderator's events survive them
	// (ON DELETE SET NULL), because deleting the actor must not erase the record
	// of what they did.
	ActorUserID   *int32  `json:"actor_user_id,omitempty"`
	ActorUsername *string `json:"actor_username,omitempty"`

	Action    string     `json:"action"` // See ValidBanEventActions
	Reason    *string    `json:"reason,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// Ban audit-log actions. The column is a plain VARCHAR with no CHECK
// constraint, matching how phase types and character statuses are handled --
// this list is the authority.
const (
	// BanEventBanned is a new ban on a user who was not banned before.
	BanEventBanned = "banned"

	// BanEventUnbanned is a ban being lifted. Its reason/expiry are copied from
	// the row being deleted, since that row will not exist afterwards.
	BanEventUnbanned = "unbanned"

	// BanEventModified is a re-ban of an ALREADY-banned user -- changing the
	// reason or extending the expiry. Distinguished from "banned" so the log
	// reads as a history of decisions rather than implying the user was
	// unbanned and re-banned in between.
	BanEventModified = "modified"
)

// ValidBanEventActions is the canonical set of audit-log actions.
var ValidBanEventActions = []string{BanEventBanned, BanEventUnbanned, BanEventModified}

// CreateCommunityBanRequest bans a user from a community, or updates an
// existing ban in place.
//
// UserID rather than a username: the client already has ids from the member
// surfaces, and resolving a name here would make the endpoint's behaviour
// depend on a rename.
type CreateCommunityBanRequest struct {
	UserID int32   `json:"user_id"`
	Reason *string `json:"reason,omitempty"`

	// ExpiresAt nil means a PERMANENT ban. This is the common case, so it is
	// the default rather than something the client must opt into.
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// Ban audit-log pagination. The log grows without bound and is read newest-first
// in a management view, so it is paged rather than returned whole.
const (
	DefaultBanEventPageSize = 50
	MaxBanEventPageSize     = 200
)

// CommunityDocument is one of a community's rules or reference pages (req 7).
//
// Modelled on Handout -- same draft/published gate, same markdown body rendered
// through MarkdownPreview -- but with no comment thread. A community's rules
// are an announcement; games already have handouts for the discussable kind.
//
// Addressed by ID, not by a slug. Slugs were considered and rejected: a
// document's title is edited as its rules evolve, and a slug frozen at creation
// would leave a renamed document permanently at the old URL. A slug would also
// need collision handling (two "FAQ" documents) and a fallback for titles with
// no Latin characters -- costs paid at write time for a readability benefit
// only the rare hand-typed link collects.
type CommunityDocument struct {
	ID          int32  `json:"id"`
	CommunityID int32  `json:"community_id"`
	Title       string `json:"title"`
	Content     string `json:"content"` // Markdown

	// Status is "draft" or "published". Only published documents are visible to
	// anyone but a moderator, so this is the whole visibility rule -- see
	// ValidDocumentStatuses.
	Status string `json:"status"`

	// SortOrder is the display position, lowest first. Explicit because a
	// community's documents have a deliberate reading order that neither the
	// title nor the creation date expresses.
	SortOrder int32 `json:"sort_order"`

	// CreatedByUserID is nullable: the moderator who wrote a document may since
	// have been deleted (ON DELETE SET NULL). The document belongs to the
	// community, not to its author, so it outlives them.
	CreatedByUserID *int32 `json:"created_by_user_id,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// No community name or slug here, deliberately. An earlier version carried
	// them on the per-game rows so the Info tab could label its section, back
	// when GetGameWithDetails did not join communities. It does now, so the game
	// payload names its own community -- and must, since the section has to name
	// a community that has published no documents at all.
}

// Document statuses. A plain VARCHAR column with no CHECK constraint, matching
// handouts.status and the rest of this schema -- this list is the authority.
const (
	// DocumentStatusDraft is visible only to the community's moderators. Lets a
	// moderator write rules over several sittings before they bind anyone.
	DocumentStatusDraft = "draft"

	// DocumentStatusPublished is visible to everyone who can see the community.
	DocumentStatusPublished = "published"
)

// ValidDocumentStatuses is the canonical set of document statuses.
var ValidDocumentStatuses = []string{DocumentStatusDraft, DocumentStatusPublished}

// IsValidDocumentStatus reports whether s is a status the service will store.
func IsValidDocumentStatus(s string) bool {
	return s == DocumentStatusDraft || s == DocumentStatusPublished
}

// Document title bounds, matching the column width.
const (
	DocumentTitleMinLength = 1
	DocumentTitleMaxLength = 255
)

// CreateCommunityDocumentRequest creates a document in a community.
type CreateCommunityDocumentRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`

	// Status omitted means DRAFT. Creating straight to published requires
	// asking for it: the safe default is the one that shows the document to
	// nobody until its author says otherwise.
	Status    *string `json:"status,omitempty"`
	SortOrder *int32  `json:"sort_order,omitempty"`
}

// UpdateCommunityDocumentRequest is a partial update; a nil field is unchanged.
//
// Content is NOT tri-state, unlike Community.Description: the column is NOT
// NULL, so there is no "clear it" state distinct from "set it to empty". A
// blank document body is a blank page, not an absent one.
type UpdateCommunityDocumentRequest struct {
	Title     *string `json:"title,omitempty"`
	Content   *string `json:"content,omitempty"`
	Status    *string `json:"status,omitempty"`
	SortOrder *int32  `json:"sort_order,omitempty"`
}
