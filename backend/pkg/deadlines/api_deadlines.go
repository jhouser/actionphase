package deadlines

import (
	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/jackc/pgx/v5/pgtype"
)

// CreateDeadline creates a new deadline for a game
func (h *Handler) CreateDeadline(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_create_deadline")()

	defer h.App.ObsLogger.LogOperation(ctx, "api_create_deadline")()

	// Get gameID from URL
	gameID := ctx.Value("gameID").(int32)

	// Parse request
	data := &CreateDeadlineRequest{}
	if err := render.Bind(r, data); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Failed to bind create deadline request", "error", err)
		return
	}

	// Get user ID from JWT token
	userID, errResp := core.GetUserIDFromJWT(ctx, h.UserService)
	if errResp != nil {
		h.renderError(ctx, w, r, errResp, "Failed to authenticate user from JWT")
		return
	}

	// Check if user is GM of the game
	_, errResp = h.verifyUserIsGM(ctx, int32(gameID), userID)
	if errResp != nil {
		h.renderError(ctx, w, r, errResp, "Request rejected in create deadline")
		return
	}

	// Create deadline
	req := core.CreateDeadlineRequest{
		GameID:      int32(gameID),
		Title:       data.Title,
		Description: data.Description,
		Deadline:    data.Deadline,
		CreatedBy:   userID,
	}
	deadline, err := h.DeadlineService.CreateDeadline(ctx, req)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to create deadline", "error", err)
		return
	}

	h.App.ObsLogger.Info(ctx, "Deadline created successfully", "deadline_id", deadline.ID, "game_id", gameID)

	response := toDeadlineResponse(deadline)

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, response)
}

// GetGameDeadlines retrieves all deadlines for a game
func (h *Handler) GetGameDeadlines(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_game_deadlines")()

	defer h.App.ObsLogger.LogOperation(ctx, "api_get_game_deadlines")()

	// Get gameID from URL
	gameID := ctx.Value("gameID").(int32)

	// Get includeExpired query parameter (defaults to false)
	includeExpired := r.URL.Query().Get("includeExpired") == "true"

	// Verify the game exists
	_, err := h.GameService.GetGame(ctx, int32(gameID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrNotFound("Game not found"), "Failed to get game", "error", err)
		return
	}

	// Get all deadlines (unified view of arbitrary, phase, and poll deadlines)
	deadlines, err := h.DeadlineService.GetAllGameDeadlines(ctx, int32(gameID), includeExpired)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get all game deadlines", "error", err)
		return
	}

	// Convert to response format
	response := make([]*UnifiedDeadlineResponse, len(deadlines))
	for i := range deadlines {
		response[i] = toUnifiedDeadlineResponse(&deadlines[i])
	}

	render.JSON(w, r, response)
}

// UpdateDeadline updates a deadline
func (h *Handler) UpdateDeadline(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_update_deadline")()

	defer h.App.ObsLogger.LogOperation(ctx, "api_update_deadline")()

	// Get deadlineID from URL
	deadlineIDStr := chi.URLParam(r, "deadlineId")
	deadlineID, err := strconv.ParseInt(deadlineIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid deadline ID")), "Invalid deadline ID", "error", err)
		return
	}

	// Parse request
	data := &UpdateDeadlineRequest{}
	if err := render.Bind(r, data); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Failed to bind update deadline request", "error", err)
		return
	}

	// Get user ID from JWT token
	userID, errResp := core.GetUserIDFromJWT(ctx, h.UserService)
	if errResp != nil {
		h.renderError(ctx, w, r, errResp, "Failed to authenticate user from JWT")
		return
	}

	// Get existing deadline to check ownership
	existingDeadline, err := h.DeadlineService.GetDeadline(ctx, int32(deadlineID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrNotFound("Deadline not found"), "Failed to get deadline", "error", err)
		return
	}

	// Check if user is GM of the game
	_, errResp = h.verifyUserIsGM(ctx, existingDeadline.GameID, userID)
	if errResp != nil {
		h.renderError(ctx, w, r, errResp, "Request rejected in update deadline")
		return
	}

	// Update deadline
	updateReq := core.UpdateDeadlineRequest{
		Title:       data.Title,
		Description: data.Description,
		Deadline:    data.Deadline,
	}
	deadline, err := h.DeadlineService.UpdateDeadline(ctx, int32(deadlineID), updateReq)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to update deadline", "error", err)
		return
	}

	h.App.ObsLogger.Info(ctx, "Deadline updated successfully", "deadline_id", deadline.ID)

	response := toDeadlineResponse(deadline)

	render.JSON(w, r, response)
}

// DeleteDeadline deletes a deadline
func (h *Handler) DeleteDeadline(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_delete_deadline")()

	defer h.App.ObsLogger.LogOperation(ctx, "api_delete_deadline")()

	// Get deadlineID from URL
	deadlineIDStr := chi.URLParam(r, "deadlineId")
	deadlineID, err := strconv.ParseInt(deadlineIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid deadline ID")), "Invalid deadline ID", "error", err)
		return
	}

	// Get user ID from JWT token
	userID, errResp := core.GetUserIDFromJWT(ctx, h.UserService)
	if errResp != nil {
		h.renderError(ctx, w, r, errResp, "Failed to authenticate user from JWT")
		return
	}

	// Get existing deadline to check ownership
	existingDeadline, err := h.DeadlineService.GetDeadline(ctx, int32(deadlineID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrNotFound("Deadline not found"), "Failed to get deadline", "error", err)
		return
	}

	// Check if user is GM of the game
	_, errResp = h.verifyUserIsGM(ctx, existingDeadline.GameID, userID)
	if errResp != nil {
		h.renderError(ctx, w, r, errResp, "Request rejected in delete deadline")
		return
	}

	// Delete deadline
	err = h.DeadlineService.DeleteDeadline(ctx, int32(deadlineID), userID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to delete deadline", "error", err)
		return
	}

	h.App.ObsLogger.Info(ctx, "Deadline deleted successfully", "deadline_id", deadlineID)

	w.WriteHeader(http.StatusNoContent)
}

// GetUpcomingDeadlines retrieves upcoming deadlines across all user's games
func (h *Handler) GetUpcomingDeadlines(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_upcoming_deadlines")()

	defer h.App.ObsLogger.LogOperation(ctx, "api_get_upcoming_deadlines")()

	// Get user ID from JWT token
	userID, errResp := core.GetUserIDFromJWT(ctx, h.UserService)
	if errResp != nil {
		h.renderError(ctx, w, r, errResp, "Failed to authenticate user from JWT")
		return
	}

	// Get limit query parameter (defaults to 10)
	limitStr := r.URL.Query().Get("limit")
	limit := int64(10) // default
	if limitStr != "" {
		parsedLimit, err := strconv.ParseInt(limitStr, 10, 32)
		if err == nil && parsedLimit > 0 && parsedLimit <= 100 {
			limit = parsedLimit
		}
	}

	// Get upcoming deadlines
	deadlines, err := h.DeadlineService.GetUpcomingDeadlines(ctx, userID, int32(limit))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get upcoming deadlines", "error", err)
		return
	}

	// Convert to response format
	response := make([]*DeadlineWithGameResponse, len(deadlines))
	for i, deadline := range deadlines {
		response[i] = toDeadlineWithGameResponse(&deadline)
	}

	render.JSON(w, r, response)
}

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
