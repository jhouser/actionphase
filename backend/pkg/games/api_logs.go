package games

import (
	"actionphase/pkg/core"
	db "actionphase/pkg/db/models"
	"encoding/json"
	"net/http"
)

// Logs retrieves all logs for a given game
// GET /api/v1/games/:id/logs
func (h *Handler) GetGameLogs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_logs")()

	gameID := ctx.Value("gameID").(int32)

	// Get authenticated user
	user := core.GetAuthenticatedUser(ctx)
	if user == nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "No authenticated user found")
		return
	}

	gameService := h.GameService
	// Verify user is GM of this game
	game := ctx.Value("game").(*db.Game)

	if game.State.String != core.GameStateCompleted && game.State.String != core.GameStateCancelled {
		// Check GM permissions if the game is still incomplete (considers admin mode)

		if !ctx.Value("is_gm").(bool) {
			h.renderError(ctx, w, r, core.ErrForbidden("only the GM can retrieve game logs while the game is running"), "Game logs access forbidden")
			return
		}
	}

	logs, err := gameService.GetGameLogs(ctx, int32(gameID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get game logs", "error", err, "game_id", gameID)
		return
	}

	// Convert to response format
	// Initialize as empty slice to ensure JSON encodes as [] not null
	response := make([]map[string]interface{}, 0)
	for _, log := range logs {
		logData := map[string]interface{}{
			"id":         log.ID,
			"game_id":    log.GameID,
			"type":       log.Type,
			"message":    log.Message.String,
			"created_at": log.CreatedAt.Time,
		}
		response = append(response, logData)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
