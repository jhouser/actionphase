package games

import (
	"actionphase/pkg/core"
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

func (h *Handler) GameMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			gameIDStr := chi.URLParam(r, "gameID")
			gameID, err := strconv.ParseInt(gameIDStr, 10, 32)
			if err != nil {
				render.Render(w, r, core.ErrInvalidRequest(fmt.Errorf("invalid game ID")))
				return
			}

			// Get authenticated user
			user := core.GetAuthenticatedUser(ctx)

			gameService := h.GameService

			// Verify user is GM of this game. Distinguish a missing game (404)
			// from a transient database failure (500) so clients and alerting
			// are not misled by a blanket 404.
			game, err := gameService.GetGame(ctx, int32(gameID))
			if err != nil {
				h.renderError(ctx, w, r, core.HandleDBErrorWithID(err, "game", gameID), "Failed to get game", "error", err, "game_id", gameID)
				return
			}

			ctx = context.WithValue(ctx, "game", game)
			ctx = context.WithValue(ctx, "gameID", game.ID)

			// Check GM permissions (considers admin mode)
			ctx = context.WithValue(ctx, "is_gm", user != nil && core.IsUserGameMaster(r, user.ID, user.IsAdmin, *game, h.App.Pool))

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
