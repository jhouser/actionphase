package core

import (
	models "actionphase/pkg/db/models"
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

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
// Anonymity is a play-time protection, not a permanent one. Once a game is
// COMPLETED it becomes a public archive readable by any authenticated user
// (CanUserViewGame), so usernames are disclosed to everyone — mirroring how
// completion lifts the individual-vote restriction on polls
// (checkPollViewAccess, pkg/polls/api_polls.go). Cancelled games are NOT
// public and keep the play-time rule.
//
// If the game is not anonymous, always returns true.
func CanSeeUsernamesInAnonymousGame(ctx context.Context, db *pgxpool.Pool, game models.Game, userID int32) bool {
	if !game.IsAnonymous {
		return true
	}

	// Completed games are a public archive: anonymity no longer applies.
	if game.State.Valid && game.State.String == GameStateCompleted {
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
