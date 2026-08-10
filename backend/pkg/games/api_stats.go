package games

import (
	"net/http"

	"actionphase/pkg/core"
	db "actionphase/pkg/db/models"

	"github.com/go-chi/render"
)

// GetGameStats returns post-game statistics for a completed game.
//
// Authorization is deliberately narrow: stats aggregate over private messages
// as well as public comments, so they are only served once the game reaches
// `completed` and becomes a public archive readable by any authenticated user
// (CanUserViewGame). A cancelled or in-progress game returns 409 even for its
// GM — mid-game these numbers would leak the shape of private activity (who is
// talking to whom, and how much) to anyone who can read them, and they would be
// a live scoreboard players could optimize against.
//
// Responses: 200 with the stats payload, 403 when the requester cannot view the
// game, 409 when the game is not completed.
func (h *Handler) GetGameStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_game_stats")()

	gameID := ctx.Value("gameID").(int32)
	game := ctx.Value("game").(*db.Game)

	// Stats exist only for finished games. Checked before the view permission
	// so an in-progress game reports the actual reason rather than a 403.
	if game.State.String != "completed" {
		h.renderError(ctx, w, r,
			core.ErrConflict("statistics are only available for completed games"),
			"Game stats requested for non-completed game",
			"game_id", gameID, "state", game.State.String)
		return
	}

	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.renderError(ctx, w, r, core.ErrForbidden("authentication required"),
			"Unauthenticated game stats request", "game_id", gameID)
		return
	}

	// Reuse the canonical read check rather than hand-rolling a participant
	// test: it already encodes public archive mode for completed games.
	canView, err := h.GameService.CanUserViewGame(ctx, gameID, authUser.ID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err),
			"Failed to check game view permission for stats",
			"error", err, "game_id", gameID, "user_id", authUser.ID)
		return
	}
	if !canView {
		h.renderError(ctx, w, r, core.ErrForbidden("you do not have access to this game"),
			"Forbidden game stats request", "game_id", gameID, "user_id", authUser.ID)
		return
	}

	stats, err := h.GameStatsService.GetGameStats(ctx, gameID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err),
			"Failed to compute game stats", "error", err, "game_id", gameID)
		return
	}

	render.JSON(w, r, stats)
}
