package admin

// Site-admin community endpoints: creating a community and assigning its owner,
// listing every community, and editing one (including owner reassignment and
// deactivation).
//
// These are admin-only by construction -- the whole /api/v1/admin group sits
// behind RequireAdmin. Moderator-facing community endpoints live in
// pkg/communities and gate on core.CanModerateCommunity instead.

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"actionphase/pkg/core"

	"github.com/danielgtaylor/huma/v2"
)

// ---------------------------------------------------------------- I/O types

type communityOutput struct {
	Body *core.Community
}

type listCommunitiesOutput struct {
	Body []*core.Community
}

type createCommunityInput struct {
	Body struct {
		Name        string  `json:"name" required:"true" minLength:"2" maxLength:"255" doc:"Display name"`
		Slug        string  `json:"slug" required:"true" minLength:"2" maxLength:"100" doc:"URL identifier; lowercase letters, digits, and single interior hyphens. Immutable after creation."`
		Description *string `json:"description,omitempty" doc:"Markdown blurb shown on the community page"`
		OwnerUserID int32   `json:"owner_user_id" required:"true" minimum:"1" doc:"User who will own this community"`
	}
}

type updateCommunityInput struct {
	ID   int32 `path:"id" doc:"Community ID"`
	Body struct {
		// Every field is optional: this is a PATCH, and an omitted field is
		// left unchanged. Slug is absent on purpose -- it is immutable.
		Name        *string `json:"name,omitempty" minLength:"2" maxLength:"255" doc:"Display name"`
		Description *string `json:"description,omitempty" doc:"Markdown blurb"`
		OwnerUserID *int32  `json:"owner_user_id,omitempty" minimum:"1" doc:"Reassign ownership to this user"`
		IsActive    *bool   `json:"is_active,omitempty" doc:"Deactivate to stop the community accepting new games"`
	}
}

// ----------------------------------------------------------------- handlers

// HumaCreateCommunity creates a community and assigns its owner (requirement 1).
func (h *Handler) HumaCreateCommunity(ctx context.Context, in *createCommunityInput) (*communityOutput, error) {
	slug := strings.ToLower(strings.TrimSpace(in.Body.Slug))

	// Length bounds came from the schema tags; this is the semantic check they
	// cannot express (charset, hyphen placement, reserved words).
	if ok, reason := core.ValidateCommunitySlug(slug); !ok {
		return nil, huma.Error400BadRequest(reason)
	}

	name := strings.TrimSpace(in.Body.Name)
	if name == "" {
		return nil, huma.Error400BadRequest("name cannot be blank")
	}

	// The owner must exist. Without this the insert fails on a foreign key and
	// renders as a 500 rather than telling the admin what they got wrong.
	if _, err := h.UserService.GetUserByID(int(in.Body.OwnerUserID)); err != nil {
		return nil, huma.Error400BadRequest("owner_user_id does not match an existing user")
	}

	community, err := h.CommunityService.CreateCommunity(ctx, &core.CreateCommunityRequest{
		Name:        name,
		Slug:        slug,
		Description: in.Body.Description,
		OwnerUserID: in.Body.OwnerUserID,
	})
	if err != nil {
		if errors.Is(err, core.ErrCommunitySlugTaken) {
			return nil, huma.Error400BadRequest("that slug is already taken")
		}
		return nil, huma.Error500InternalServerError("Failed to create community")
	}

	return &communityOutput{Body: community}, nil
}

// HumaListCommunities returns every community, active or not.
func (h *Handler) HumaListCommunities(ctx context.Context, _ *struct{}) (*listCommunitiesOutput, error) {
	communities, err := h.CommunityService.ListCommunities(ctx)
	if err != nil {
		return nil, huma.Error500InternalServerError("Failed to list communities")
	}
	return &listCommunitiesOutput{Body: communities}, nil
}

// HumaUpdateCommunity applies a partial update: profile fields, owner
// reassignment, or deactivation.
func (h *Handler) HumaUpdateCommunity(ctx context.Context, in *updateCommunityInput) (*communityOutput, error) {
	if in.Body.Name != nil && strings.TrimSpace(*in.Body.Name) == "" {
		return nil, huma.Error400BadRequest("name cannot be blank")
	}

	// Reassigning to a user who does not exist would fail on the foreign key
	// and surface as a 500.
	if in.Body.OwnerUserID != nil {
		if _, err := h.UserService.GetUserByID(int(*in.Body.OwnerUserID)); err != nil {
			return nil, huma.Error400BadRequest("owner_user_id does not match an existing user")
		}
	}

	community, err := h.CommunityService.UpdateCommunity(ctx, in.ID, &core.UpdateCommunityRequest{
		Name:        in.Body.Name,
		Description: in.Body.Description,
		OwnerUserID: in.Body.OwnerUserID,
		IsActive:    in.Body.IsActive,
	})
	if err != nil {
		if errors.Is(err, core.ErrCommunityNotFound) {
			return nil, huma.Error404NotFound("community not found")
		}
		return nil, huma.Error500InternalServerError("Failed to update community")
	}

	return &communityOutput{Body: community}, nil
}

// RegisterHumaAdminCommunities registers the site-admin community routes.
// Called from RegisterHumaAdmin so the whole admin surface stays in one place.
func RegisterHumaAdminCommunities(api huma.API, h *Handler, op func(id, method, path, summary string, status int) huma.Operation) {
	huma.Register(api, op("adminListCommunities", http.MethodGet, "/communities",
		"List all communities", http.StatusOK), h.HumaListCommunities)
	huma.Register(api, op("adminCreateCommunity", http.MethodPost, "/communities",
		"Create a community and assign its owner", http.StatusCreated), h.HumaCreateCommunity)
	huma.Register(api, op("adminUpdateCommunity", http.MethodPatch, "/communities/{id}",
		"Update a community, reassign its owner, or deactivate it", http.StatusOK), h.HumaUpdateCommunity)
}
