package communities

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"
)

// AddModerator grants a user moderation powers over a community.
//
// Authorization is the CALLER's job: only a community owner (or a site admin in
// admin mode) may reach here, gated by core.CanAdministerCommunity. That is the
// single power separating an owner from a moderator, so a moderator calling
// this must be rejected before the service is reached.
//
// Two integrity rules are enforced here rather than at the handler, because
// they are properties of the data model and every caller needs them:
//
//  1. The owner cannot be added. Ownership already confers every moderator
//     power, and a duplicate owner row would let someone "demote" an owner by
//     deleting a moderator row while ownership itself is untouched.
//  2. Duplicates are rejected as a domain error rather than a 500.
func (s *CommunityService) AddModerator(ctx context.Context, communityID, userID, grantedBy int32) (*core.CommunityModerator, error) {
	queries := models.New(s.DB)

	community, err := queries.GetCommunityByID(ctx, communityID)
	if err != nil {
		return nil, core.ErrCommunityNotFound
	}

	if community.OwnerUserID == userID {
		return nil, core.ErrOwnerCannotBeModerator
	}

	params := models.AddCommunityModeratorParams{
		CommunityID: communityID,
		UserID:      userID,
	}
	if grantedBy != 0 {
		params.GrantedByUserID = pgtype.Int4{Int32: grantedBy, Valid: true}
	}

	row, err := queries.AddCommunityModerator(ctx, params)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return nil, core.ErrAlreadyModerator
		}
		s.Logger.LogError(ctx, err, "Failed to add community moderator",
			"community_id", communityID, "user_id", userID)
		return nil, fmt.Errorf("add community moderator: %w", err)
	}

	s.Logger.Info(ctx, "Community moderator added",
		"community_id", communityID,
		"user_id", userID,
		"granted_by", grantedBy,
	)

	return moderatorFromDB(row), nil
}

// RemoveModerator revokes a user's moderation powers.
//
// Like AddModerator, this is owner-only at the handler layer. Removing a
// non-moderator is a no-op rather than an error: the caller's intent (that this
// user does not moderate) already holds.
func (s *CommunityService) RemoveModerator(ctx context.Context, communityID, userID int32) error {
	queries := models.New(s.DB)

	if err := queries.RemoveCommunityModerator(ctx, models.RemoveCommunityModeratorParams{
		CommunityID: communityID,
		UserID:      userID,
	}); err != nil {
		s.Logger.LogError(ctx, err, "Failed to remove community moderator",
			"community_id", communityID, "user_id", userID)
		return fmt.Errorf("remove community moderator: %w", err)
	}

	s.Logger.Info(ctx, "Community moderator removed",
		"community_id", communityID,
		"user_id", userID,
	)
	return nil
}

// ListModerators returns a community's moderator roster with user details.
//
// The OWNER IS NOT INCLUDED -- ownership is not a moderator row. Surfaces that
// want to show "everyone who can moderate" render the owner from
// Community.OwnerUserID alongside this list.
func (s *CommunityService) ListModerators(ctx context.Context, communityID int32) ([]*core.CommunityModerator, error) {
	queries := models.New(s.DB)

	rows, err := queries.ListCommunityModerators(ctx, communityID)
	if err != nil {
		return nil, fmt.Errorf("list community moderators: %w", err)
	}

	out := make([]*core.CommunityModerator, 0, len(rows))
	for _, row := range rows {
		out = append(out, moderatorFromListRow(row))
	}
	return out, nil
}

// GetRole resolves a user's standing in a community in one round-trip.
// Owner outranks moderator; core.CommunityRoleNone means neither.
//
// This reports the user's own standing and knows nothing about site admins.
// Handlers gate on core.CanModerateCommunity / core.CanAdministerCommunity,
// which fold in admin mode.
func (s *CommunityService) GetRole(ctx context.Context, communityID, userID int32) (core.CommunityRole, error) {
	queries := models.New(s.DB)

	role, err := queries.GetCommunityRole(ctx, models.GetCommunityRoleParams{
		CommunityID: communityID,
		UserID:      userID,
	})
	if err != nil {
		return core.CommunityRoleNone, fmt.Errorf("get community role: %w", err)
	}

	switch role {
	case string(core.CommunityRoleOwner):
		return core.CommunityRoleOwner, nil
	case string(core.CommunityRoleModerator):
		return core.CommunityRoleModerator, nil
	default:
		return core.CommunityRoleNone, nil
	}
}
