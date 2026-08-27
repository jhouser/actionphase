package deadlines

import (
	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"
	"context"
	"time"

	"github.com/go-chi/render"
	"github.com/jackc/pgx/v5/pgtype"
)

// Helper function to convert database model to API response
func toDeadlineResponse(d *models.GameDeadline) *DeadlineResponse {
	resp := &DeadlineResponse{
		ID:        d.ID,
		GameID:    d.GameID,
		Title:     d.Title,
		CreatedAt: pgTimestampToTimePtr(d.CreatedAt),
		UpdatedAt: pgTimestampToTimePtr(d.UpdatedAt),
	}

	// Handle optional description
	if d.Description.Valid {
		resp.Description = &d.Description.String
	}

	// Handle optional deadline timestamp
	if d.Deadline.Valid {
		deadlineTime := d.Deadline.Time
		resp.Deadline = &deadlineTime
	}

	return resp
}

// Helper function to convert DeadlineWithGame to API response
func toDeadlineWithGameResponse(d *core.DeadlineWithGame) *DeadlineWithGameResponse {
	resp := &DeadlineWithGameResponse{
		ID:        d.ID,
		GameID:    d.GameID,
		GameTitle: d.GameTitle,
		Title:     d.Title,
		CreatedAt: pgTimestampToTimePtr(d.CreatedAt),
		UpdatedAt: pgTimestampToTimePtr(d.UpdatedAt),
	}

	// Handle optional description
	if d.Description.Valid {
		resp.Description = &d.Description.String
	}

	// Handle optional deadline timestamp
	if d.Deadline.Valid {
		deadlineTime := d.Deadline.Time
		resp.Deadline = &deadlineTime
	}

	return resp
}

// Helper function to verify user is GM of a game
// Returns the game if verification succeeds, or an error response if it fails
// Uses the unified permission check for GM, Co-GM, and admin mode support
func (h *Handler) verifyUserIsGM(ctx context.Context, gameID int32, userID int32) (*models.Game, render.Renderer) {
	game, err := h.GameService.GetGame(ctx, gameID)
	if err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to get game")
		return nil, core.ErrNotFound("Game not found")
	}

	// Get user to check admin status
	user, err := h.UserService.GetUserByID(int(userID))
	if err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to get user")
		return nil, core.ErrUnauthorized("User not found")
	}

	// Check if user is GM, Co-GM, or admin with admin mode enabled
	if !core.IsUserGameMasterCtx(ctx, userID, user.IsAdmin, *game, h.App.Pool) {
		h.App.ObsLogger.Warn(ctx, "User is not authorized to manage deadlines", "user_id", userID, "game_id", gameID)
		return nil, core.ErrUnauthorized("Only GM or Co-GM can manage deadlines")
	}

	return game, nil
}

// Helper function to convert pgtype.Timestamptz to *time.Time
func pgTimestampToTimePtr(t pgtype.Timestamptz) *time.Time {
	if t.Valid {
		return &t.Time
	}
	return nil
}

// Helper function to convert core.UnifiedDeadline to UnifiedDeadlineResponse
func toUnifiedDeadlineResponse(d *core.UnifiedDeadline) *UnifiedDeadlineResponse {
	resp := &UnifiedDeadlineResponse{
		DeadlineType:     d.DeadlineType,
		SourceID:         d.SourceID,
		Title:            d.Title,
		Description:      d.Description,
		GameID:           d.GameID,
		PhaseID:          d.PhaseID,
		PollID:           d.PollID,
		IsSystemDeadline: d.IsSystemDeadline,
	}

	// Handle optional deadline timestamp
	if !d.Deadline.IsZero() {
		deadline := d.Deadline
		resp.Deadline = &deadline
	}

	return resp
}
