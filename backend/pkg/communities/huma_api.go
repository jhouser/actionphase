package communities

// Type-first (huma) handlers for the member- and moderator-facing community
// endpoints. Paths here are relative to the /api/v1/communities mount.

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"actionphase/pkg/core"
)

// ---------------------------------------------------------------- I/O types

type communitySlugInput struct {
	Slug string `path:"slug" doc:"Community URL slug"`
}

type communityOutput struct {
	Body *core.Community
}

type communityListOutput struct {
	Body []*core.Community
}

type moderatorListOutput struct {
	Body []*core.CommunityModerator
}

type moderatorOutput struct {
	Body *core.CommunityModerator
}

// updateCommunityProfileInput carries the moderator-editable slice of a
// community profile.
//
// Named for the PROFILE rather than the community because huma derives schema
// component names from this Go type: the admin package has its own
// updateCommunityInput, and two identically-named input types collapse into one
// OpenAPI component. When they did, the admin endpoint's documented body
// silently lost owner_user_id and is_active.
//
// Deliberately NARROWER than the admin PATCH: no owner_user_id and no
// is_active. Reassigning ownership and deactivating a community are site-admin
// acts, and a moderator who could set either could seize or retire a community
// they merely help run.
//
// Slug is absent because it is immutable after creation -- it appears in URLs
// communities have shared externally.
type updateCommunityProfileInput struct {
	Slug string `path:"slug" doc:"Community URL slug"`
	Body struct {
		Name *string `json:"name,omitempty" minLength:"2" maxLength:"255" doc:"Display name"`
		// Tri-state, matching core.UpdateCommunityRequest: omitted leaves the
		// blurb alone, a value sets it, and an empty string clears it. Markdown.
		Description *string `json:"description,omitempty" doc:"Profile blurb (markdown); empty string clears it"`
	}
}

type addModeratorInput struct {
	Slug string `path:"slug" doc:"Community URL slug"`
	Body struct {
		UserID int32 `json:"user_id" required:"true" minimum:"1" doc:"User to grant moderation powers"`
	}
}

type removeModeratorInput struct {
	Slug   string `path:"slug" doc:"Community URL slug"`
	UserID int32  `path:"userID" doc:"User whose moderation powers are revoked"`
}

type banListOutput struct {
	Body []*core.CommunityBan
}

type banOutput struct {
	Body *core.CommunityBan
}

// createBanInput bans a user, or edits an existing ban in place.
//
// The body mirrors core.CreateCommunityBanRequest rather than reusing it,
// because huma derives request validation from the struct tags and the core
// type is a service-layer contract with no tags on it.
type createBanInput struct {
	Slug string `path:"slug" doc:"Community URL slug"`
	Body struct {
		UserID int32   `json:"user_id" required:"true" minimum:"1" doc:"User to ban"`
		Reason *string `json:"reason,omitempty" maxLength:"1000" doc:"Why the user was banned; shown to moderators only"`

		// Omitted means PERMANENT, which is the common case. A client that
		// wants a temporary ban sends an absolute timestamp rather than a
		// duration, so the expiry does not drift with request latency.
		ExpiresAt *time.Time `json:"expires_at,omitempty" doc:"When the ban lapses; omit for a permanent ban"`
	}
}

type removeBanInput struct {
	Slug   string `path:"slug" doc:"Community URL slug"`
	UserID int32  `path:"userID" doc:"User whose ban is lifted"`
}

// banEventListInput pages the audit log. It grows without bound, so it is never
// returned whole.
type banEventListInput struct {
	Slug   string `path:"slug" doc:"Community URL slug"`
	Limit  int32  `query:"limit" doc:"Entries per page" minimum:"1" maximum:"200" default:"50"`
	Offset int32  `query:"offset" doc:"Entries to skip" minimum:"0" default:"0"`
}

type banEventListOutput struct {
	Body []*core.CommunityBanEvent
}

// ----------------------------------------------------------------- handlers

// withCallerContext stamps the CALLER's standing and ban status onto each
// community.
//
// Both are computed per request rather than stored, because the answer depends
// on who is asking -- and, for a site admin, on whether admin mode is currently
// on. A value cached at login could not track that toggle.
//
// The ban lookup is ONE query for the whole slice rather than one per
// community, so stamping a listing costs the same as stamping a single record.
//
// A failed ban lookup leaves IsBanned false rather than failing the request:
// this flag only filters a picker, and the ban check on game creation is the
// actual enforcement. Refusing to render the community list because the
// convenience lookup broke would trade a minor annoyance for an outage.
//
// Mutates in place: these are freshly built structs from the service, not
// shared cache entries.
func (h *Handler) withCallerContext(ctx context.Context, userID int32, list ...*core.Community) {
	isAdmin := h.isSiteAdmin(ctx, userID)

	bannedIDs, err := h.CommunityService.ListBannedCommunityIDs(ctx, userID)
	if err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to load caller's community bans",
			"user_id", userID)
	}
	banned := make(map[int32]bool, len(bannedIDs))
	for _, id := range bannedIDs {
		banned[id] = true
	}

	for _, c := range list {
		if c == nil {
			continue
		}

		// A ban is a fact about the user regardless of standing. Stamped
		// before the admin short-circuit below so it is never skipped.
		c.IsBanned = banned[c.ID]

		// Admin mode confers the full power set, which is the owner tier --
		// the same rule CanAdministerCommunity applies. Checked first so an
		// admin is not downgraded to their incidental standing.
		if isAdmin && core.GetAdminMode(ctx) {
			c.YourRole = core.CommunityRoleOwner
			continue
		}
		c.YourRole = core.GetCommunityRole(ctx, h.App.Pool, c.ID, userID)
	}
}

// humaListCommunities returns the active communities.
//
// Inactive communities are omitted: they accept no new games, so listing them
// on a public surface would invite dead ends. Site admins see the full list
// through the admin endpoint instead.
func (h *Handler) humaListCommunities(ctx context.Context, _ *struct{}) (*communityListOutput, error) {
	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	list, err := h.CommunityService.ListActiveCommunities(ctx)
	if err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to list active communities")
		return nil, huma.Error500InternalServerError("Failed to list communities")
	}

	h.withCallerContext(ctx, userID, list...)
	return &communityListOutput{Body: list}, nil
}

// humaGetCommunity returns one community's profile by slug.
func (h *Handler) humaGetCommunity(ctx context.Context, in *communitySlugInput) (*communityOutput, error) {
	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	community, err := h.loadCommunity(ctx, in.Slug)
	if err != nil {
		return nil, err
	}

	h.withCallerContext(ctx, userID, community)
	return &communityOutput{Body: community}, nil
}

// humaListModerators returns a community's moderator roster.
//
// Readable by anyone who can moderate the community. The roster names who holds
// power over bans and documents, which is moderation-internal rather than
// public profile information.
//
// The OWNER IS NOT IN THIS LIST -- ownership is not a moderator row. Clients
// render the owner from the community's owner_user_id alongside these entries.
func (h *Handler) humaListModerators(ctx context.Context, in *communitySlugInput) (*moderatorListOutput, error) {
	community, _, err := h.requireModerator(ctx, in.Slug)
	if err != nil {
		return nil, err
	}

	mods, err := h.CommunityService.ListModerators(ctx, community.ID)
	if err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to list community moderators",
			"community_id", community.ID)
		return nil, huma.Error500InternalServerError("Failed to list moderators")
	}
	return &moderatorListOutput{Body: mods}, nil
}

// humaAddModerator grants a user moderation powers.
//
// Owner-only (req 4): this is the one act a moderator may not perform, so it
// gates on requireOwner rather than requireModerator.
func (h *Handler) humaAddModerator(ctx context.Context, in *addModeratorInput) (*moderatorOutput, error) {
	community, actorID, err := h.requireOwner(ctx, in.Slug)
	if err != nil {
		return nil, err
	}

	// The target must exist. Without this the insert fails on a foreign key and
	// renders as a 500 rather than telling the owner what they got wrong.
	if _, err := h.UserService.GetUserByID(int(in.Body.UserID)); err != nil {
		return nil, huma.Error400BadRequest("user_id does not match an existing user")
	}

	mod, err := h.CommunityService.AddModerator(ctx, community.ID, in.Body.UserID, actorID)
	if err != nil {
		switch {
		case errors.Is(err, core.ErrOwnerCannotBeModerator):
			return nil, huma.Error400BadRequest("the community owner already has full moderation powers")
		case errors.Is(err, core.ErrAlreadyModerator):
			return nil, huma.Error400BadRequest("that user already moderates this community")
		case errors.Is(err, core.ErrCommunityNotFound):
			return nil, huma.Error404NotFound("community not found")
		}
		h.App.ObsLogger.LogError(ctx, err, "Failed to add community moderator",
			"community_id", community.ID, "user_id", in.Body.UserID)
		return nil, huma.Error500InternalServerError("Failed to add moderator")
	}

	return &moderatorOutput{Body: mod}, nil
}

// humaRemoveModerator revokes a user's moderation powers. Owner-only (req 4).
//
// Removing someone who does not moderate is a success, not a 404: the caller's
// intent -- that this user does not moderate -- already holds.
func (h *Handler) humaRemoveModerator(ctx context.Context, in *removeModeratorInput) (*struct{}, error) {
	community, _, err := h.requireOwner(ctx, in.Slug)
	if err != nil {
		return nil, err
	}

	if err := h.CommunityService.RemoveModerator(ctx, community.ID, in.UserID); err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to remove community moderator",
			"community_id", community.ID, "user_id", in.UserID)
		return nil, huma.Error500InternalServerError("Failed to remove moderator")
	}

	return nil, nil
}

// humaUpdateCommunity edits a community's name and description.
//
// Moderator-level (req 4), not owner-only: keeping the profile current is
// ordinary upkeep, the same class of work as moderating bans and documents. The
// roster stays the owner's alone.
func (h *Handler) humaUpdateCommunity(ctx context.Context, in *updateCommunityProfileInput) (*communityOutput, error) {
	community, actorID, err := h.requireModerator(ctx, in.Slug)
	if err != nil {
		return nil, err
	}

	// Trim before the blank check so a name of only spaces is rejected rather
	// than stored. minLength on the tag counts characters, not content.
	var name *string
	if in.Body.Name != nil {
		trimmed := strings.TrimSpace(*in.Body.Name)
		if trimmed == "" {
			return nil, huma.Error400BadRequest("name cannot be blank")
		}
		name = &trimmed
	}

	updated, err := h.CommunityService.UpdateCommunity(ctx, community.ID, &core.UpdateCommunityRequest{
		Name:        name,
		Description: in.Body.Description,
	})
	if err != nil {
		if errors.Is(err, core.ErrCommunityNotFound) {
			return nil, huma.Error404NotFound("community not found")
		}
		h.App.ObsLogger.LogError(ctx, err, "Failed to update community",
			"community_id", community.ID)
		return nil, huma.Error500InternalServerError("Failed to update community")
	}

	// The caller just proved they moderate, but say so explicitly rather than
	// hardcoding a tier -- an owner and a moderator both reach here.
	h.withCallerContext(ctx, actorID, updated)
	return &communityOutput{Body: updated}, nil
}

// humaListBans returns a community's banlist.
//
// Moderator-level (req 4). The list is not public: it names users and carries
// the moderator's stated reason, neither of which belongs on a profile page.
//
// EXPIRED bans are included, each carrying is_active. A moderator who set a
// two-week ban needs to see it lapse; dropping expired rows here would make a
// ban appear to vanish and invite a puzzled re-ban.
func (h *Handler) humaListBans(ctx context.Context, in *communitySlugInput) (*banListOutput, error) {
	community, _, err := h.requireModerator(ctx, in.Slug)
	if err != nil {
		return nil, err
	}

	bans, err := h.CommunityService.ListBans(ctx, community.ID)
	if err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to list community bans",
			"community_id", community.ID)
		return nil, huma.Error500InternalServerError("Failed to list bans")
	}
	return &banListOutput{Body: bans}, nil
}

// humaBanUser bans a user from a community, or edits an existing ban.
//
// Deliberately NOT idempotent-by-rejection: re-banning an already-banned user
// updates the reason and expiry in place and answers 200, because a moderator
// extending a ban is doing ordinary work and a 400 would push them into
// unban-then-reban, losing the original banned_at. The audit log distinguishes
// the two by logging "modified" rather than "banned".
func (h *Handler) humaBanUser(ctx context.Context, in *createBanInput) (*banOutput, error) {
	community, actorID, err := h.requireModerator(ctx, in.Slug)
	if err != nil {
		return nil, err
	}

	// The target must exist. Without this the insert trips a foreign key and
	// surfaces as a 500 rather than telling the moderator what they got wrong.
	if _, err := h.UserService.GetUserByID(int(in.Body.UserID)); err != nil {
		return nil, huma.Error400BadRequest("user_id does not match an existing user")
	}

	// An expiry already in the past would create a ban that is inert the moment
	// it is written -- it would appear on the list as lapsed and enforce
	// nothing. That is never what a moderator means, so it is a client error
	// rather than a silently useless write.
	if in.Body.ExpiresAt != nil && !in.Body.ExpiresAt.After(time.Now()) {
		return nil, huma.Error400BadRequest("expires_at must be in the future; omit it for a permanent ban")
	}

	ban, err := h.CommunityService.BanUser(ctx, community.ID, actorID, &core.CreateCommunityBanRequest{
		UserID:    in.Body.UserID,
		Reason:    in.Body.Reason,
		ExpiresAt: in.Body.ExpiresAt,
	})
	if err != nil {
		switch {
		case errors.Is(err, core.ErrCannotBanCommunityStaff):
			// Owner-only to fix, which is the point: a moderator cannot
			// neutralise a peer by banning them.
			return nil, huma.Error400BadRequest(
				"community staff cannot be banned; remove them from the moderator roster first")
		case errors.Is(err, core.ErrCommunityNotFound):
			return nil, huma.Error404NotFound("community not found")
		}
		h.App.ObsLogger.LogError(ctx, err, "Failed to ban user from community",
			"community_id", community.ID, "user_id", in.Body.UserID)
		return nil, huma.Error500InternalServerError("Failed to ban user")
	}

	return &banOutput{Body: ban}, nil
}

// humaUnbanUser lifts a ban.
//
// Unlike removing a moderator, an absent ban is a 404 rather than a success.
// The two differ because this endpoint is reached from a list of bans: a
// missing row means the moderator is acting on a stale view -- someone else
// lifted it already -- and silently reporting success would hide that.
func (h *Handler) humaUnbanUser(ctx context.Context, in *removeBanInput) (*struct{}, error) {
	community, actorID, err := h.requireModerator(ctx, in.Slug)
	if err != nil {
		return nil, err
	}

	if err := h.CommunityService.UnbanUser(ctx, community.ID, in.UserID, actorID); err != nil {
		if errors.Is(err, core.ErrBanNotFound) {
			return nil, huma.Error404NotFound("that user is not banned from this community")
		}
		h.App.ObsLogger.LogError(ctx, err, "Failed to lift community ban",
			"community_id", community.ID, "user_id", in.UserID)
		return nil, huma.Error500InternalServerError("Failed to lift ban")
	}

	return nil, nil
}

// humaListBanEvents returns the ban audit log, newest first.
//
// This is the record that survives a ban being lifted -- lifting DELETES the
// ban row, so for an unbanned user the log is the only evidence the ban ever
// existed. Moderator-level, and paged: the log only grows.
func (h *Handler) humaListBanEvents(ctx context.Context, in *banEventListInput) (*banEventListOutput, error) {
	community, _, err := h.requireModerator(ctx, in.Slug)
	if err != nil {
		return nil, err
	}

	// The service clamps limit/offset to its own bounds, so the zero values a
	// client can still send (huma applies defaults only to absent params)
	// resolve to the default page rather than an empty one.
	events, err := h.CommunityService.ListBanEvents(ctx, community.ID, in.Limit, in.Offset)
	if err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to list community ban events",
			"community_id", community.ID)
		return nil, huma.Error500InternalServerError("Failed to list ban events")
	}
	return &banEventListOutput{Body: events}, nil
}

// -------------------------------------------------------------- registration

// RegisterHumaCommunities registers the member- and moderator-facing community
// routes. Paths are relative to the /api/v1/communities mount.
func RegisterHumaCommunities(api huma.API, h *Handler) {
	huma.Register(api, huma.Operation{
		OperationID: "listCommunities",
		Method:      http.MethodGet,
		Path:        "/",
		Summary:     "List active communities",
		Description: "Returns every active community. Inactive communities are omitted -- " +
			"they accept no new games. Site admins see the full list via the admin endpoint.",
		Tags:     []string{"Communities"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
		},
	}, h.humaListCommunities)

	huma.Register(api, huma.Operation{
		OperationID: "getCommunity",
		Method:      http.MethodGet,
		Path:        "/{slug}",
		Summary:     "Get a community profile",
		Tags:        []string{"Communities"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"401": {Description: "Not authenticated"},
			"404": {Description: "Community not found"},
		},
	}, h.humaGetCommunity)

	huma.Register(api, huma.Operation{
		OperationID: "updateCommunity",
		Method:      http.MethodPatch,
		Path:        "/{slug}",
		Summary:     "Edit a community's name and description",
		Description: "Requires moderation rights -- keeping the profile current is ordinary " +
			"upkeep. Narrower than the admin PATCH: ownership and active status are not " +
			"editable here, and the slug is immutable. An empty description clears it.",
		Tags:     []string{"Communities"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"400": {Description: "Name is blank or out of range"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Caller does not moderate this community"},
			"404": {Description: "Community not found"},
		},
	}, h.humaUpdateCommunity)

	huma.Register(api, huma.Operation{
		OperationID: "listCommunityModerators",
		Method:      http.MethodGet,
		Path:        "/{slug}/moderators",
		Summary:     "List a community's moderators",
		Description: "Requires moderation rights. The owner is not included -- ownership is " +
			"not a moderator row; render it from the community's owner_user_id.",
		Tags:     []string{"Communities"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Caller does not moderate this community"},
			"404": {Description: "Community not found"},
		},
	}, h.humaListModerators)

	huma.Register(api, huma.Operation{
		OperationID: "addCommunityModerator",
		Method:      http.MethodPost,
		Path:        "/{slug}/moderators",
		Summary:     "Grant a user moderation powers",
		Description: "Owner-only. Managing the roster is the single power a moderator does " +
			"not have; site admins qualify only with admin mode enabled.",
		Tags:          []string{"Communities"},
		Security:      []map[string][]string{{"BearerAuth": {}}},
		DefaultStatus: http.StatusCreated,
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"400": {Description: "Unknown user, the owner, or already a moderator"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Caller is not the community owner"},
			"404": {Description: "Community not found"},
		},
	}, h.humaAddModerator)

	huma.Register(api, huma.Operation{
		OperationID: "removeCommunityModerator",
		Method:      http.MethodDelete,
		Path:        "/{slug}/moderators/{userID}",
		Summary:     "Revoke a user's moderation powers",
		Description: "Owner-only. Removing a user who does not moderate succeeds -- the " +
			"caller's intended end state already holds.",
		Tags:          []string{"Communities"},
		Security:      []map[string][]string{{"BearerAuth": {}}},
		DefaultStatus: http.StatusNoContent,
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Caller is not the community owner"},
			"404": {Description: "Community not found"},
		},
	}, h.humaRemoveModerator)

	huma.Register(api, huma.Operation{
		OperationID: "listCommunityBans",
		Method:      http.MethodGet,
		Path:        "/{slug}/bans",
		Summary:     "List a community's banned users",
		Description: "Requires moderation rights -- the list names users and carries the " +
			"moderator's reason. Expired bans are included, marked is_active=false, so a " +
			"lapsed ban is visible rather than appearing to vanish.",
		Tags:     []string{"Communities"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Caller does not moderate this community"},
			"404": {Description: "Community not found"},
		},
	}, h.humaListBans)

	huma.Register(api, huma.Operation{
		OperationID: "banUserFromCommunity",
		Method:      http.MethodPost,
		Path:        "/{slug}/bans",
		Summary:     "Ban a user from a community, or edit an existing ban",
		Description: "Requires moderation rights. Re-banning an already-banned user updates " +
			"the reason and expiry in place rather than failing, preserving the original " +
			"banned_at; the audit log records that as \"modified\". Omit expires_at for a " +
			"permanent ban. Community staff cannot be banned -- remove them from the " +
			"roster first, which is owner-only.",
		Tags:     []string{"Communities"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"400": {Description: "Unknown user, community staff, or an expiry in the past"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Caller does not moderate this community"},
			"404": {Description: "Community not found"},
		},
	}, h.humaBanUser)

	huma.Register(api, huma.Operation{
		OperationID: "unbanUserFromCommunity",
		Method:      http.MethodDelete,
		Path:        "/{slug}/bans/{userID}",
		Summary:     "Lift a user's ban",
		Description: "Requires moderation rights. Answers 404 if the user is not banned, " +
			"since reaching this from a stale banlist should surface rather than look " +
			"like success. The ban row is deleted; the audit log retains what it said.",
		Tags:          []string{"Communities"},
		Security:      []map[string][]string{{"BearerAuth": {}}},
		DefaultStatus: http.StatusNoContent,
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Caller does not moderate this community"},
			"404": {Description: "Community not found, or the user is not banned"},
		},
	}, h.humaUnbanUser)

	huma.Register(api, huma.Operation{
		OperationID: "listCommunityBanEvents",
		Method:      http.MethodGet,
		Path:        "/{slug}/ban-events",
		Summary:     "Read a community's ban audit log",
		Description: "Requires moderation rights. Newest first, paged. Lifting a ban deletes " +
			"its row, so for an unbanned user this log is the only surviving record that " +
			"the ban existed. Entries snapshot the reason and expiry as they stood.",
		Tags:     []string{"Communities"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Caller does not moderate this community"},
			"404": {Description: "Community not found"},
		},
	}, h.humaListBanEvents)
}
