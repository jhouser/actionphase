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
	"time"

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

// timePtr maps a nullable timestamp column to *time.Time.
func timePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

// timestampParam maps *time.Time to a nullable timestamp param, sending nil to
// SQL NULL -- which for a ban's expires_at means PERMANENT.
func timestampParam(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

// banIsActive reports whether a ban is being enforced right now.
//
// Computed at read time, never stored: nothing writes to the row when a ban
// lapses, so a stored flag would go stale the moment the clock passed it. This
// must agree with the SQL expiry test in every enforcement query --
// TestBanIsActiveAgreesWithSQL is the guard.
func banIsActive(expiresAt pgtype.Timestamptz) bool {
	if !expiresAt.Valid {
		return true // Permanent.
	}
	return expiresAt.Time.After(time.Now())
}

// banFromDB converts the bare insert/delete-returning row, which carries no
// joined user details. Callers needing the username re-list.
func banFromDB(row models.CommunityBan) *core.CommunityBan {
	return &core.CommunityBan{
		ID:             row.ID,
		CommunityID:    row.CommunityID,
		UserID:         row.UserID,
		Reason:         textPtr(row.Reason),
		BannedByUserID: int32Ptr(row.BannedByUserID),
		BannedAt:       row.BannedAt.Time,
		ExpiresAt:      timePtr(row.ExpiresAt),
		IsActive:       banIsActive(row.ExpiresAt),
	}
}

// banFromListRow converts a ban row with its joined user details.
func banFromListRow(row models.ListCommunityBansRow) *core.CommunityBan {
	return &core.CommunityBan{
		ID:               row.ID,
		CommunityID:      row.CommunityID,
		UserID:           row.UserID,
		Username:         row.Username,
		DisplayName:      textPtr(row.DisplayName),
		AvatarURL:        textPtr(row.AvatarUrl),
		Reason:           textPtr(row.Reason),
		BannedByUserID:   int32Ptr(row.BannedByUserID),
		BannedByUsername: textPtr(row.BannedByUsername),
		BannedAt:         row.BannedAt.Time,
		ExpiresAt:        timePtr(row.ExpiresAt),
		IsActive:         banIsActive(row.ExpiresAt),
	}
}

// banEventFromListRow converts an audit-log row.
//
// Both usernames are nullable: the actor may have been deleted (ON DELETE SET
// NULL, so their events survive them) and so may the target. The event still
// renders -- that is the whole point of an append-only log.
func banEventFromListRow(row models.ListCommunityBanEventsRow) *core.CommunityBanEvent {
	return &core.CommunityBanEvent{
		ID:             row.ID,
		CommunityID:    row.CommunityID,
		TargetUserID:   row.TargetUserID,
		TargetUsername: textPtr(row.TargetUsername),
		ActorUserID:    int32Ptr(row.ActorUserID),
		ActorUsername:  textPtr(row.ActorUsername),
		Action:         row.Action,
		Reason:         textPtr(row.Reason),
		ExpiresAt:      timePtr(row.ExpiresAt),
		CreatedAt:      row.CreatedAt.Time,
	}
}
