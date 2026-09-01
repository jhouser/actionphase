package communities

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"
)

// Community bans -- the driver for the whole Communities feature.
//
// Two invariants hold across this file:
//
//  1. The BANLIST and the AUDIT LOG are written in ONE transaction. Lifting a
//     ban deletes its row, so the log is the only surviving record; a log that
//     can silently miss an entry is worse than no log, because it would be
//     trusted in exactly the disputes it exists to settle.
//
//  2. Enforcement respects expires_at. Expired rows are NOT deleted -- they stay
//     on the management list so a moderator sees a ban lapsed rather than
//     vanish -- so "a row exists" never means "banned". IsUserBanned is the only
//     correct answer to that question.

// BanUser bans a user from a community, or updates an existing ban in place.
//
// Re-banning an already-banned user is deliberately NOT an error: changing a
// reason or extending an expiry is ordinary moderation, and a unique-violation
// 400 would push moderators into unban-then-reban, which loses the original
// banned_at. The audit log distinguishes the two cases -- a fresh ban logs
// "banned", an update logs "modified" -- so the history still reads correctly.
//
// Community staff cannot be banned. A moderator who is also banned is a
// contradictory state no enforcement path knows how to read, so the roster must
// be edited first (an owner-only act, unlike banning).
func (s *CommunityService) BanUser(ctx context.Context, communityID, actorID int32, req *core.CreateCommunityBanRequest) (*core.CommunityBan, error) {
	if req == nil {
		return nil, fmt.Errorf("ban request is required")
	}

	queries := models.New(s.DB)

	community, err := queries.GetCommunityByID(ctx, communityID)
	if err != nil {
		return nil, core.ErrCommunityNotFound
	}

	// The owner is not a moderator row, so both tiers must be checked.
	if community.OwnerUserID == req.UserID {
		return nil, core.ErrCannotBanCommunityStaff
	}
	role, err := s.GetRole(ctx, communityID, req.UserID)
	if err != nil {
		return nil, err
	}
	if role != core.CommunityRoleNone {
		return nil, core.ErrCannotBanCommunityStaff
	}

	// Whether this is a new ban or an edit decides the audit action. Read it
	// BEFORE the upsert, which would otherwise make the two indistinguishable.
	// Read inside the transaction below would be cleaner still, but the upsert
	// cannot report which branch it took.
	existing, err := queries.GetCommunityBan(ctx, models.GetCommunityBanParams{
		CommunityID: communityID,
		UserID:      req.UserID,
	})
	isUpdate := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("check existing community ban: %w", err)
	}
	_ = existing

	action := core.BanEventBanned
	if isUpdate {
		action = core.BanEventModified
	}

	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin ban transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txQueries := queries.WithTx(tx)

	row, err := txQueries.CreateCommunityBan(ctx, models.CreateCommunityBanParams{
		CommunityID:    communityID,
		UserID:         req.UserID,
		Reason:         textParam(req.Reason),
		BannedByUserID: int32Param(&actorID),
		ExpiresAt:      timestampParam(req.ExpiresAt),
	})
	if err != nil {
		s.Logger.LogError(ctx, err, "Failed to ban user from community",
			"community_id", communityID, "user_id", req.UserID)
		return nil, fmt.Errorf("ban user from community: %w", err)
	}

	if _, err := txQueries.CreateCommunityBanEvent(ctx, models.CreateCommunityBanEventParams{
		CommunityID:  communityID,
		TargetUserID: req.UserID,
		ActorUserID:  int32Param(&actorID),
		Action:       action,
		Reason:       textParam(req.Reason),
		ExpiresAt:    timestampParam(req.ExpiresAt),
	}); err != nil {
		s.Logger.LogError(ctx, err, "Failed to write community ban event",
			"community_id", communityID, "user_id", req.UserID)
		return nil, fmt.Errorf("write community ban event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit ban transaction: %w", err)
	}

	s.Logger.Info(ctx, "User banned from community",
		"community_id", communityID,
		"user_id", req.UserID,
		"actor_id", actorID,
		"action", action,
		"permanent", req.ExpiresAt == nil,
	)

	return banFromDB(row), nil
}

// UnbanUser lifts a ban, recording what it said before deleting it.
//
// The DELETE returns the row precisely so its reason and expiry can be
// snapshotted into the audit event -- after the commit that row is gone and the
// log is the only record that the ban ever existed.
func (s *CommunityService) UnbanUser(ctx context.Context, communityID, userID, actorID int32) error {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin unban transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txQueries := models.New(s.DB).WithTx(tx)

	row, err := txQueries.DeleteCommunityBan(ctx, models.DeleteCommunityBanParams{
		CommunityID: communityID,
		UserID:      userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return core.ErrBanNotFound
		}
		s.Logger.LogError(ctx, err, "Failed to lift community ban",
			"community_id", communityID, "user_id", userID)
		return fmt.Errorf("lift community ban: %w", err)
	}

	if _, err := txQueries.CreateCommunityBanEvent(ctx, models.CreateCommunityBanEventParams{
		CommunityID:  communityID,
		TargetUserID: userID,
		ActorUserID:  int32Param(&actorID),
		Action:       core.BanEventUnbanned,
		// Snapshot of the ban being lifted, not of the unban: this is what the
		// ban said, which is the thing a later dispute needs to see.
		Reason:    row.Reason,
		ExpiresAt: row.ExpiresAt,
	}); err != nil {
		s.Logger.LogError(ctx, err, "Failed to write community unban event",
			"community_id", communityID, "user_id", userID)
		return fmt.Errorf("write community unban event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit unban transaction: %w", err)
	}

	s.Logger.Info(ctx, "Community ban lifted",
		"community_id", communityID,
		"user_id", userID,
		"actor_id", actorID,
	)
	return nil
}

// ListBans returns a community's banlist for the management view.
//
// EXPIRED bans are included. Each carries IsActive, so the UI can show a lapsed
// ban as lapsed instead of silently dropping it -- a moderator who set a
// two-week ban should be able to see that it has run out.
func (s *CommunityService) ListBans(ctx context.Context, communityID int32) ([]*core.CommunityBan, error) {
	rows, err := models.New(s.DB).ListCommunityBans(ctx, communityID)
	if err != nil {
		return nil, fmt.Errorf("list community bans: %w", err)
	}

	out := make([]*core.CommunityBan, 0, len(rows))
	for _, row := range rows {
		out = append(out, banFromListRow(row))
	}
	return out, nil
}

// ListBanEvents returns the audit log, newest first.
func (s *CommunityService) ListBanEvents(ctx context.Context, communityID int32, limit, offset int32) ([]*core.CommunityBanEvent, error) {
	if limit <= 0 {
		limit = core.DefaultBanEventPageSize
	}
	if limit > core.MaxBanEventPageSize {
		limit = core.MaxBanEventPageSize
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := models.New(s.DB).ListCommunityBanEvents(ctx, models.ListCommunityBanEventsParams{
		CommunityID: communityID,
		Limit:       limit,
		Offset:      offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list community ban events: %w", err)
	}

	out := make([]*core.CommunityBanEvent, 0, len(rows))
	for _, row := range rows {
		out = append(out, banEventFromListRow(row))
	}
	return out, nil
}

// IsUserBanned reports whether a user is currently barred from a community.
//
// This is THE ban primitive. Every enforcement path calls this or
// IsUserBannedFromGame -- never its own query -- so that the expiry rule and any
// future change to what "banned" means live in exactly one place.
func (s *CommunityService) IsUserBanned(ctx context.Context, communityID, userID int32) (bool, error) {
	banned, err := models.New(s.DB).IsUserBannedFromCommunity(ctx, models.IsUserBannedFromCommunityParams{
		CommunityID: communityID,
		UserID:      userID,
	})
	if err != nil {
		return false, fmt.Errorf("check community ban: %w", err)
	}
	return banned, nil
}

// IsUserBannedFromGame resolves the game's community, then checks the ban.
//
// A game with no community returns FALSE. Grandfathering is guaranteed by the
// query's inner join rather than by each caller remembering to test for NULL,
// because a caller who forgets would lock legacy players out of their own games.
func (s *CommunityService) IsUserBannedFromGame(ctx context.Context, gameID, userID int32) (bool, error) {
	banned, err := models.New(s.DB).IsUserBannedFromGameCommunity(ctx, models.IsUserBannedFromGameCommunityParams{
		GameID: gameID,
		UserID: userID,
	})
	if err != nil {
		return false, fmt.Errorf("check community ban for game: %w", err)
	}
	return banned, nil
}

// ListBannedCommunityIDs returns every community the user is currently banned
// from, for filtering the community picker on game creation in one round-trip
// rather than one per community.
func (s *CommunityService) ListBannedCommunityIDs(ctx context.Context, userID int32) ([]int32, error) {
	ids, err := models.New(s.DB).ListBannedCommunityIDsForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list banned community ids: %w", err)
	}
	return ids, nil
}

// textParam maps *string to a nullable text param, sending nil to SQL NULL.
func textParam(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

// int32Param maps *int32 to a nullable integer param.
func int32Param(v *int32) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: *v, Valid: true}
}
