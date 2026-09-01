package communities

// Type-first (huma) handlers for the member- and moderator-facing community
// endpoints. Paths here are relative to the /api/v1/communities mount.

import (
	"context"
	"errors"
	"net/http"
	"strings"

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

// ----------------------------------------------------------------- handlers

// withYourRole stamps the CALLER's standing onto each community.
//
// Computed per request rather than stored, because the answer depends on who is
// asking -- and, for a site admin, on whether admin mode is currently on. A
// value cached at login could not track that toggle.
//
// Mutates in place: these are freshly built structs from the service, not
// shared cache entries.
func (h *Handler) withYourRole(ctx context.Context, userID int32, list ...*core.Community) {
	isAdmin := h.isSiteAdmin(ctx, userID)

	for _, c := range list {
		if c == nil {
			continue
		}
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

	h.withYourRole(ctx, userID, list...)
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

	h.withYourRole(ctx, userID, community)
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
	h.withYourRole(ctx, actorID, updated)
	return &communityOutput{Body: updated}, nil
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
			"401": {Description: "Not authenticated"},
			"403": {Description: "Caller is not the community owner"},
			"404": {Description: "Community not found"},
		},
	}, h.humaRemoveModerator)
}
