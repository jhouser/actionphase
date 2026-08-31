// Package communities implements community management: the community record
// itself, its moderator roster, and (in later phases) bans, documents, and
// Discord webhooks.
//
// It is decomposed by concern, following pkg/db/services/phases and
// pkg/db/services/actions.
package communities

import (
	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"

	"github.com/jackc/pgx/v5/pgtype"
)

// textPtr maps a nullable text column to *string, sending SQL NULL to nil
// rather than to "". The distinction is load-bearing for PATCH semantics: a nil
// description means "not set", not "set to empty".
func textPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	v := t.String
	return &v
}

// int32Ptr maps a nullable integer column to *int32.
func int32Ptr(v pgtype.Int4) *int32 {
	if !v.Valid {
		return nil
	}
	out := v.Int32
	return &out
}

// communityFromDB converts the bare table row (no joins).
func communityFromDB(row models.Community) *core.Community {
	return &core.Community{
		ID:          row.ID,
		Name:        row.Name,
		Slug:        row.Slug,
		Description: textPtr(row.Description),
		BannerURL:   textPtr(row.BannerUrl),
		OwnerUserID: row.OwnerUserID,
		IsActive:    row.IsActive,
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}
}

// communityFromListRow converts a row that carries the joined owner username.
// The two list queries generate distinct row types with identical fields, so
// each gets a thin adapter onto this shared shape.
func communityFromListRow(
	id int32, name, slug string,
	description, bannerURL pgtype.Text,
	ownerUserID int32, isActive bool,
	createdAt, updatedAt pgtype.Timestamptz,
	ownerUsername string,
) *core.Community {
	return &core.Community{
		ID:            id,
		Name:          name,
		Slug:          slug,
		Description:   textPtr(description),
		BannerURL:     textPtr(bannerURL),
		OwnerUserID:   ownerUserID,
		OwnerUsername: ownerUsername,
		IsActive:      isActive,
		CreatedAt:     createdAt.Time,
		UpdatedAt:     updatedAt.Time,
	}
}

// moderatorFromListRow converts a moderator row with its joined user details.
func moderatorFromListRow(row models.ListCommunityModeratorsRow) *core.CommunityModerator {
	return &core.CommunityModerator{
		ID:                row.ID,
		CommunityID:       row.CommunityID,
		UserID:            row.UserID,
		Username:          row.Username,
		DisplayName:       textPtr(row.DisplayName),
		AvatarURL:         textPtr(row.AvatarUrl),
		GrantedByUserID:   int32Ptr(row.GrantedByUserID),
		GrantedByUsername: textPtr(row.GrantedByUsername),
		GrantedAt:         row.GrantedAt.Time,
	}
}

// moderatorFromDB converts the bare insert-returning row, which has no joined
// user details. Callers that need the username re-list.
func moderatorFromDB(row models.CommunityModerator) *core.CommunityModerator {
	return &core.CommunityModerator{
		ID:              row.ID,
		CommunityID:     row.CommunityID,
		UserID:          row.UserID,
		GrantedByUserID: int32Ptr(row.GrantedByUserID),
		GrantedAt:       row.GrantedAt.Time,
	}
}
