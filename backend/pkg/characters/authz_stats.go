package characters

import (
	"context"

	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"

	"github.com/jackc/pgx/v5/pgtype"
)

// gameLevelPrivateStatsAccess reports whether the requester may see private
// message counts for *any* character in the game on grounds that don't depend
// on which character is being viewed: they're a GM/co-GM, an audience member,
// or the game is completed. The remaining, character-specific grant — the
// requester owning the character — is ORed in at the call site (see
// canSeeCharacterPrivateStats). Computing these game-level flags once avoids
// re-running the IsUserGameMaster / IsUserAudience DB lookups per character in
// the batch endpoint.
//
// A nil authUser (unauthenticated) never has game-level access.
func (h *Handler) gameLevelPrivateStatsAccess(ctx context.Context, authUser *core.AuthenticatedUser, game models.Game) bool {
	if authUser == nil {
		return false
	}
	isGM := core.IsUserGameMasterCtx(ctx, authUser.ID, authUser.IsAdmin, game, h.App.Pool)
	isAudience := core.IsUserAudience(ctx, h.App.Pool, game.ID, authUser.ID)
	// Completed AND epilogue: both disclose the archive to every viewer.
	isArchive := core.IsPublicArchive(game.State.String)
	return isGM || isAudience || isArchive
}

// canSeeCharacterPrivateStats combines the game-level grant with per-character
// ownership: the character's owner always sees their own private count.
func canSeeCharacterPrivateStats(gameLevelAccess bool, authUser *core.AuthenticatedUser, ownerUserID pgtype.Int4) bool {
	isOwner := authUser != nil && ownerUserID.Valid && ownerUserID.Int32 == authUser.ID
	return gameLevelAccess || isOwner
}
