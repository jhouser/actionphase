package core

import (
	models "actionphase/pkg/db/models"
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

// IsPublicArchive reports whether a game in this state is readable by ANY
// authenticated user, participant or not.
//
// Two states qualify, for different reasons:
//   - GameStateCompleted: terminal and read-only.
//   - GameStateEpilogue: still writable, so the GM can run epilogue and
//     meta-discussion threads with the whole archive open.
//
// GameStateCancelled is deliberately NOT a public archive; cancelled games
// follow normal participant permission rules.
//
// This is the read gate. It is a separate concern from the write gate
// (ValidateGameNotCompleted, pkg/core/validation.go) — a state can be one, the
// other, both, or neither, and epilogue exists precisely because those two
// questions have different answers.
func IsPublicArchive(state string) bool {
	return state == GameStateCompleted || state == GameStateEpilogue
}

// IsUserCoGM checks if a user is a co-GM for a specific game.
// This function queries the database to check if the user has the 'co_gm' role
// for the given game.
func IsUserCoGM(ctx context.Context, db *pgxpool.Pool, gameID int32, userID int32) bool {
	queries := models.New(db)

	participant, err := queries.GetParticipantByGameAndUser(ctx, models.GetParticipantByGameAndUserParams{
		GameID: gameID,
		UserID: userID,
	})

	if err != nil {
		return false
	}

	return participant.Role == "co_gm"
}

// IsUserAudience checks if a user is an audience member for a specific game.
// This function queries the database to check if the user has the 'audience' role
// for the given game.
func IsUserAudience(ctx context.Context, db *pgxpool.Pool, gameID int32, userID int32) bool {
	queries := models.New(db)

	participant, err := queries.GetParticipantByGameAndUser(ctx, models.GetParticipantByGameAndUserParams{
		GameID: gameID,
		UserID: userID,
	})

	if err != nil {
		return false
	}

	return participant.Role == "audience"
}

// IsUserGameMaster checks if a user has Game Master permissions for a game.
// It delegates to IsUserGameMasterCtx using the request context. Admin mode
// is read from the context (set by AdminModeMiddleware from the X-Admin-Mode header).
//
// A user is considered a Game Master if:
// 1. They are the primary GM of the game (game.GmUserID == userID), OR
// 2. They are a co-GM for the game (participant role == 'co_gm'), OR
// 3. They are an admin with admin mode enabled (X-Admin-Mode: true header)
func IsUserGameMaster(r *http.Request, userID int32, isAdmin bool, game models.Game, db *pgxpool.Pool) bool {
	return IsUserGameMasterCtx(r.Context(), userID, isAdmin, game, db)
}

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	// adminModeContextKey is the context key for storing admin mode state
	adminModeContextKey contextKey = "admin_mode"
)

// WithAdminMode adds admin mode state to the context.
// This is typically called by middleware that reads the X-Admin-Mode header.
func WithAdminMode(ctx context.Context, adminMode bool) context.Context {
	return context.WithValue(ctx, adminModeContextKey, adminMode)
}

// GetAdminMode retrieves admin mode state from the context.
// Returns false if admin mode is not set in the context.
func GetAdminMode(ctx context.Context) bool {
	adminMode, ok := ctx.Value(adminModeContextKey).(bool)
	if !ok {
		return false
	}
	return adminMode
}

// IsUserGameMasterCtx is a context-based version of IsUserGameMaster.
// It uses admin mode state from the context instead of reading headers directly.
// This is useful when the admin mode state has already been extracted by middleware.
//
// Note: This function requires database access to check co-GM status.
//
// Usage Example:
//
//	// After middleware has set admin mode in context
//	user := GetAuthenticatedUser(r.Context())
//	if !IsUserGameMasterCtx(r.Context(), user.ID, user.IsAdmin, game, db) {
//		render.Render(w, r, core.ErrForbidden("only the GM can perform this action"))
//		return
//	}
func IsUserGameMasterCtx(ctx context.Context, userID int32, isAdmin bool, game models.Game, db *pgxpool.Pool) bool {
	// Check if user is the primary GM
	if game.GmUserID == userID {
		return true
	}

	// Check if user is a co-GM
	if IsUserCoGM(ctx, db, game.ID, userID) {
		return true
	}

	// Check if user is admin with admin mode enabled
	if isAdmin && GetAdminMode(ctx) {
		return true
	}

	return false
}

// CanSeeUsernamesInAnonymousGame returns true if the user is allowed to see
// author usernames in a game with is_anonymous=true.
//
// Rule: players cannot see each other's usernames in anonymous games.
// GMs, co-GMs, and audience members can always see usernames.
//
// Anonymity is a play-time protection, not a permanent one. Once a game is a
// public archive (COMPLETED or EPILOGUE — see IsPublicArchive) it is readable
// by any authenticated user (CanUserViewGame), so usernames are disclosed to
// everyone — mirroring how the archive lifts the individual-vote restriction on
// polls (checkPollViewAccess, pkg/polls/api_polls.go). Cancelled games are NOT
// public and keep the play-time rule.
//
// If the game is not anonymous, always returns true.
func CanSeeUsernamesInAnonymousGame(ctx context.Context, db *pgxpool.Pool, game models.Game, userID int32) bool {
	if !game.IsAnonymous {
		return true
	}

	// Public archive (completed or epilogue): anonymity no longer applies.
	if game.State.Valid && IsPublicArchive(game.State.String) {
		return true
	}

	// Primary GM always sees usernames
	if game.GmUserID == userID {
		return true
	}

	// Check participant role: co_gm and audience can see usernames; players cannot
	queries := models.New(db)
	participant, err := queries.GetParticipantByGameAndUser(ctx, models.GetParticipantByGameAndUserParams{
		GameID: game.ID,
		UserID: userID,
	})
	if err != nil {
		// Not a participant — redact to be safe
		return false
	}

	return participant.Role == "co_gm" || participant.Role == "audience"
}

// CanUserControlNPC checks if a user can control an NPC character.
// This includes:
// 1. The NPC is assigned to the user (via npc_assignments table)
// 2. The user is the primary GM of the game
// 3. The user is a co-GM of the game
//
// This function is used for operations like:
// - Sending messages as an NPC
// - Creating posts as an NPC in common room
// - Performing actions as an NPC
//
// Usage Example:
//
//	if !core.CanUserControlNPC(ctx, db, npcCharacterID, userID) {
//		render.Render(w, r, core.ErrForbidden("you cannot control this NPC"))
//		return
//	}
func CanUserControlNPC(ctx context.Context, db *pgxpool.Pool, characterID int32, userID int32) bool {
	queries := models.New(db)

	// Get the character to check if it's an NPC and get game_id
	char, err := queries.GetCharacter(ctx, characterID)
	if err != nil {
		return false
	}

	// If it's not an NPC (has a user_id), only that user can control it
	if char.UserID.Valid {
		return char.UserID.Int32 == userID
	}

	// It's an NPC (user_id is NULL) - check assignment, GM, or co-GM status

	// Check if NPC is assigned to this user
	assignment, err := queries.GetNPCAssignment(ctx, characterID)
	if err == nil && assignment.AssignedUserID == userID {
		return true
	}

	// Get the game to check GM status
	game, err := queries.GetGame(ctx, char.GameID)
	if err != nil {
		return false
	}

	// Check if user is primary GM
	if game.GmUserID == userID {
		return true
	}

	// Check if user is co-GM
	if IsUserCoGM(ctx, db, char.GameID, userID) {
		return true
	}

	return false
}

// ---------------------------------------------------------------- communities

// GetCommunityRole resolves a user's standing in a community: owner, moderator,
// or none. Owner outranks moderator, and the owner is deliberately not a row in
// community_moderators.
//
// This reports the user's OWN standing only. It knows nothing about site
// admins; use CanModerateCommunity / CanAdministerCommunity for the questions
// that actually gate handlers.
func GetCommunityRole(ctx context.Context, db *pgxpool.Pool, communityID, userID int32) CommunityRole {
	queries := models.New(db)

	role, err := queries.GetCommunityRole(ctx, models.GetCommunityRoleParams{
		CommunityID: communityID,
		UserID:      userID,
	})
	if err != nil {
		// A failed lookup must never read as elevated access.
		return CommunityRoleNone
	}

	switch role {
	case string(CommunityRoleOwner):
		return CommunityRoleOwner
	case string(CommunityRoleModerator):
		return CommunityRoleModerator
	default:
		return CommunityRoleNone
	}
}

// CanModerateCommunity reports whether the user may perform ordinary moderation
// in a community: managing bans, documents, webhooks, and the community profile.
//
// True for the owner, for any moderator, and for a site admin WITH ADMIN MODE
// ENABLED. The admin-mode requirement matches the GM-override convention
// (IsUserGameMasterCtx) and keeps admins from moderating by accident while
// browsing normally.
func CanModerateCommunity(ctx context.Context, db *pgxpool.Pool, communityID, userID int32, isAdmin bool) bool {
	if isAdmin && GetAdminMode(ctx) {
		return true
	}

	role := GetCommunityRole(ctx, db, communityID, userID)
	return role == CommunityRoleOwner || role == CommunityRoleModerator
}

// CanAdministerCommunity reports whether the user may change the moderator
// roster -- the one power that separates an owner from a moderator.
//
// True for the owner and for a site admin with admin mode enabled. Moderators
// are excluded on purpose: they may do everything else, but they may not
// appoint further moderators.
func CanAdministerCommunity(ctx context.Context, db *pgxpool.Pool, communityID, userID int32, isAdmin bool) bool {
	if isAdmin && GetAdminMode(ctx) {
		return true
	}

	return GetCommunityRole(ctx, db, communityID, userID) == CommunityRoleOwner
}
