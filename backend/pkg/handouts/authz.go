package handouts

import (
	"context"

	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"

	"github.com/go-chi/render"
)

// verifyUserIsGM checks if a user is the GM or Co-GM of a game
// Returns the game if verification succeeds, or an error response if it fails
func (h *Handler) verifyUserIsGM(ctx context.Context, game *models.Game, userID int32) render.Renderer {
	// Get user to check admin status
	userService := h.UserService
	user, err := userService.GetUserByID(int(userID))
	if err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to get user")
		return core.ErrUnauthorized("User not found")
	}

	// Check if user is GM, Co-GM, or admin with admin mode enabled
	if !core.IsUserGameMasterCtx(ctx, userID, user.IsAdmin, *game, h.App.Pool) {
		h.App.ObsLogger.Warn(ctx, "User is not authorized to manage handouts", "user_id", userID, "game_id", game.ID)
		return core.ErrUnauthorized("Only GM or Co-GM can manage handouts")
	}

	return nil
}
