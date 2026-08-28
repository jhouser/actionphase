package messages

import (
	"context"

	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"

	"github.com/go-chi/render"
)

// Draft posts are addressed by phase, so every operation resolves the phase to
// its game before it can ask whether the caller runs that game.

func getGameIDForPhase(ctx context.Context, app *core.App, phaseID int32) (int32, error) {
	queries := models.New(app.Pool)
	phase, err := queries.GetPhase(ctx, phaseID)
	if err != nil {
		return 0, err
	}
	return phase.GameID, nil
}

func requireGMOrCoGM(ctx context.Context, app *core.App, gameID, userID int32) render.Renderer {
	queries := models.New(app.Pool)
	game, err := queries.GetGame(ctx, gameID)
	if err != nil {
		return core.ErrInternalError(err)
	}
	if game.GmUserID != userID && !core.IsUserCoGM(ctx, app.Pool, gameID, userID) {
		return core.ErrForbidden("only the Game Master or co-GM can manage draft posts")
	}
	return nil
}

func requireGMForPhase(ctx context.Context, app *core.App, phaseID, userID int32) render.Renderer {
	gameID, err := getGameIDForPhase(ctx, app, phaseID)
	if err != nil {
		return core.ErrNotFound("phase not found")
	}
	return requireGMOrCoGM(ctx, app, gameID, userID)
}
