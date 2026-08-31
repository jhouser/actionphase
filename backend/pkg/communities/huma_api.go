package communities

// Type-first (huma) handlers for the member- and moderator-facing community
// endpoints. Paths here are relative to the /api/v1/communities mount.

import (
	"context"
	"errors"
	"net/http"

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

// humaListCommunities returns the active communities.
//
// Inactive communities are omitted: they accept no new games, so listing them
// on a public surface would invite dead ends. Site admins see the full list
// through the admin endpoint instead.
func (h *Handler) humaListCommunities(ctx context.Context, _ *struct{}) (*communityListOutput, error) {
	list, err := h.CommunityService.ListActiveCommunities(ctx)
	if err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to list active communities")
		return nil, huma.Error500InternalServerError("Failed to list communities")
	}
	return &communityListOutput{Body: list}, nil
}

// humaGetCommunity returns one community's profile by slug.
func (h *Handler) humaGetCommunity(ctx context.Context, in *communitySlugInput) (*communityOutput, error) {
	community, err := h.loadCommunity(ctx, in.Slug)
	if err != nil {
		return nil, err
	}
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
