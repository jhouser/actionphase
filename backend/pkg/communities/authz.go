package communities

// Authorization for the community endpoints.
//
// Every write here resolves the community from its SLUG first, then checks the
// caller's standing against that community's id. Slugs are the public
// identifier -- they appear in URLs communities share externally -- so the
// handlers never take a community id from the client.

import (
	"context"

	"github.com/danielgtaylor/huma/v2"

	"actionphase/pkg/core"
)

// authUser resolves the caller's user id from the JWT.
func (h *Handler) authUser(ctx context.Context) (int32, error) {
	userID, errResp := core.GetUserIDFromJWT(ctx, h.UserService)
	if errResp != nil {
		h.App.ObsLogger.Warn(ctx, "Failed to authenticate user from JWT")
		return 0, huma.Error401Unauthorized("Not authenticated")
	}
	return userID, nil
}

// isSiteAdmin reports whether the caller holds the site-admin flag.
//
// This is only half of an admin check: core.CanModerateCommunity and
// core.CanAdministerCommunity additionally require ADMIN MODE to be on, so an
// admin browsing normally is treated as any other user.
func (h *Handler) isSiteAdmin(ctx context.Context, userID int32) bool {
	user, err := h.UserService.GetUserByID(int(userID))
	if err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to load user for admin check", "user_id", userID)
		return false
	}
	return user.IsAdmin
}

// loadCommunity resolves a community by slug, answering 404 for an unknown one.
//
// This applies NO visibility rule of its own: an inactive community resolves
// here like any other, and callers who need a permission check apply it after.
// Inactivity is filtered at the LISTING (ListActiveCommunities) rather than on
// the profile, so a link to a community that has since gone inactive still
// resolves instead of breaking.
func (h *Handler) loadCommunity(ctx context.Context, slug string) (*core.Community, error) {
	community, err := h.CommunityService.GetCommunityBySlug(ctx, slug)
	if err != nil {
		return nil, huma.Error404NotFound("community not found")
	}
	return community, nil
}

// requireModerator resolves a community by slug and confirms the caller may
// perform ordinary moderation in it (req 4): owner, moderator, or site admin
// with admin mode on.
func (h *Handler) requireModerator(ctx context.Context, slug string) (*core.Community, int32, error) {
	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, 0, err
	}

	community, err := h.loadCommunity(ctx, slug)
	if err != nil {
		return nil, 0, err
	}

	if !core.CanModerateCommunity(ctx, h.App.Pool, community.ID, userID, h.isSiteAdmin(ctx, userID)) {
		h.App.ObsLogger.Warn(ctx, "User cannot moderate community",
			"user_id", userID, "community_id", community.ID)
		return nil, 0, huma.Error403Forbidden("you do not moderate this community")
	}

	return community, userID, nil
}

// requireOwner resolves a community by slug and confirms the caller may change
// its MODERATOR ROSTER -- the single power separating an owner from a moderator
// (req 4). Owner or site admin with admin mode on; a plain moderator is
// rejected here even though they pass requireModerator.
func (h *Handler) requireOwner(ctx context.Context, slug string) (*core.Community, int32, error) {
	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, 0, err
	}

	community, err := h.loadCommunity(ctx, slug)
	if err != nil {
		return nil, 0, err
	}

	if !core.CanAdministerCommunity(ctx, h.App.Pool, community.ID, userID, h.isSiteAdmin(ctx, userID)) {
		h.App.ObsLogger.Warn(ctx, "User cannot administer community roster",
			"user_id", userID, "community_id", community.ID)
		return nil, 0, huma.Error403Forbidden("only the community owner can manage moderators")
	}

	return community, userID, nil
}
