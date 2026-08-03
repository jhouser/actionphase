package phases

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"actionphase/pkg/core"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// CreateActionResult creates a result for a player action (GM only)
func (h *Handler) CreateActionResult(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_create_action_result")()

	gameID := ctx.Value("gameID").(int32)

	data := &CreateActionResultRequest{}
	if err := render.Bind(r, data); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Invalid create action result request", "error", err)
		return
	}

	// Get authenticated user from context (set by middleware)
	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "No authenticated user in context")
		return
	}

	gmUser := authUser

	// Check permissions - must be GM
	phaseService := h.PhaseService
	canManage, err := phaseService.CanUserManagePhases(ctx, int32(gameID), int32(gmUser.ID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to check phase management permission", "error", err)
		return
	}

	if !canManage {
		h.renderError(ctx, w, r, core.ErrForbidden("only the GM can create action results"), "Create action result forbidden")
		return
	}

	// Get active phase
	activePhase, err := phaseService.GetActivePhase(ctx, int32(gameID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get active phase", "error", err)
		return
	}

	if activePhase == nil {
		h.renderError(ctx, w, r, core.ErrBadRequest(fmt.Errorf("no active phase for this game")), "Bad create action result request")
		return
	}

	// Create action result using ActionSubmissionService
	actionService := h.ActionSubmissionService
	req := core.CreateActionResultRequest{
		GameID:             int32(gameID),
		UserID:             data.UserID,
		CharacterID:        data.CharacterID,
		ActionSubmissionID: data.ActionSubmissionID,
		PhaseID:            activePhase.ID,
		GMUserID:           int32(gmUser.ID),
		Content:            data.Content,
		IsPublished:        data.IsPublished,
	}

	result, err := actionService.CreateActionResult(ctx, req)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to create action result", "error", err)
		return
	}

	// Convert to response format
	response := &ActionResultResponse{
		ID:          result.ID,
		GameID:      result.GameID,
		UserID:      result.UserID,
		PhaseID:     result.PhaseID,
		GMUserID:    result.GmUserID,
		Content:     result.Content,
		IsPublished: result.IsPublished.Bool,
	}

	if result.SentAt.Valid {
		response.SentAt = &result.SentAt.Time
	}

	render.Status(r, http.StatusCreated)
	render.Render(w, r, response)
}

// GetUserActionResults retrieves user's action results for a game
func (h *Handler) GetUserActionResults(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_user_action_results")()

	gameID := ctx.Value("gameID").(int32)

	// Get authenticated user from context (set by middleware)
	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "No authenticated user in context")
		return
	}

	actionService := h.ActionSubmissionService
	results, err := actionService.GetUserResults(ctx, int32(gameID), int32(authUser.ID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get user action results", "error", err)
		return
	}

	// Convert to response format
	var response []ActionResultWithDetailsResponse
	for _, result := range results {
		resultResp := ActionResultWithDetailsResponse{
			ID:          result.ID,
			GameID:      result.GameID,
			UserID:      result.UserID,
			PhaseID:     result.PhaseID,
			GMUserID:    result.GmUserID,
			Content:     result.Content,
			IsPublished: result.IsPublished.Bool,
			GMUsername:  result.GmUsername,
			PhaseType:   result.PhaseType,
			PhaseNumber: result.PhaseNumber,
		}

		if result.SentAt.Valid {
			resultResp.SentAt = &result.SentAt.Time
		}

		if result.CharacterName.Valid {
			resultResp.CharacterName = result.CharacterName.String
		}

		response = append(response, resultResp)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetGameActionResults retrieves all action results for a game
// - GM: Always allowed
// - Completed games: All participants can view (public archive)
// - In-progress games: GM only
func (h *Handler) GetGameActionResults(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_game_action_results")()

	gameID := ctx.Value("gameID").(int32)

	// Get authenticated user from context (set by middleware)
	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "No authenticated user in context")
		return
	}

	// Check permissions - must be GM, audience, OR game must be completed
	phaseService := h.PhaseService
	canManage, err := phaseService.CanUserManagePhases(ctx, int32(gameID), int32(authUser.ID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to check phase management permission", "error", err)
		return
	}

	// Get game to check state and participant role
	gameService := h.GameService
	game, err := gameService.GetGame(ctx, int32(gameID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get game", "error", err)
		return
	}

	// Check if user is audience member
	isAudience := core.IsUserAudience(ctx, h.App.Pool, int32(gameID), int32(authUser.ID))

	// Allow access if: GM, audience, or game is completed (public archive)
	if !canManage && !isAudience && game.State.String != "completed" {
		h.renderError(ctx, w, r, core.ErrForbidden("only the GM, audience, or participants of completed games can view all action results"), "Get game action results forbidden")
		return
	}

	actionService := h.ActionSubmissionService
	results, err := actionService.GetGameResults(ctx, int32(gameID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get game action results", "error", err)
		return
	}

	// Convert to response format
	// GMs, audience, and completed game viewers all see published and unpublished results
	var response []ActionResultWithDetailsResponse
	for _, result := range results {
		resultResp := ActionResultWithDetailsResponse{
			ID:          result.ID,
			GameID:      result.GameID,
			UserID:      result.UserID,
			PhaseID:     result.PhaseID,
			GMUserID:    result.GmUserID,
			Content:     result.Content,
			IsPublished: result.IsPublished.Bool,
			Username:    result.Username,
			PhaseType:   result.PhaseType,
			PhaseNumber: result.PhaseNumber,
		}

		// Add character_id if available
		if result.CharacterID.Valid {
			charID := result.CharacterID.Int32
			resultResp.CharacterID = &charID
		}

		// Add action_submission_id if available
		if result.ActionSubmissionID.Valid {
			submissionID := result.ActionSubmissionID.Int32
			resultResp.ActionSubmissionID = &submissionID
		}

		// Add character_name if available
		if result.CharacterName.Valid {
			resultResp.CharacterName = result.CharacterName.String
		}

		if result.SentAt.Valid {
			resultResp.SentAt = &result.SentAt.Time
		}

		response = append(response, resultResp)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// DeleteActionResult deletes an unpublished (draft) action result (GM only)
func (h *Handler) DeleteActionResult(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_delete_action_result")()

	gameID := ctx.Value("gameID").(int32)

	resultIDStr := chi.URLParam(r, "resultId")
	resultID, err := strconv.ParseInt(resultIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid result ID")), "Invalid delete action result request")
		return
	}

	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "No authenticated user in context")
		return
	}

	phaseService := h.PhaseService
	canManage, err := phaseService.CanUserManagePhases(ctx, int32(gameID), int32(authUser.ID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to check phase management permission", "error", err)
		return
	}

	if !canManage {
		h.renderError(ctx, w, r, core.ErrForbidden("only the GM can delete action results"), "Delete action result forbidden")
		return
	}

	actionService := h.ActionSubmissionService
	if err := actionService.DeleteActionResult(ctx, int32(resultID)); err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to delete action result", "error", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// UpdateActionResult updates an unpublished action result (GM only)
func (h *Handler) UpdateActionResult(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_update_action_result")()

	gameID := ctx.Value("gameID").(int32)

	resultIDStr := chi.URLParam(r, "resultId")
	resultID, err := strconv.ParseInt(resultIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid result ID")), "Invalid update action result request")
		return
	}

	type UpdateResultRequest struct {
		Content string `json:"content" validate:"required"`
	}

	data := &UpdateResultRequest{}
	if err := json.NewDecoder(r.Body).Decode(data); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Invalid update action result request", "error", err)
		return
	}

	// Get authenticated user from context (set by middleware)
	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "No authenticated user in context")
		return
	}

	// Check permissions - must be GM
	phaseService := h.PhaseService
	canManage, err := phaseService.CanUserManagePhases(ctx, int32(gameID), int32(authUser.ID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to check phase management permission", "error", err)
		return
	}

	if !canManage {
		h.renderError(ctx, w, r, core.ErrForbidden("only the GM can update action results"), "Update action result forbidden")
		return
	}

	// Update the action result
	actionService := h.ActionSubmissionService
	result, err := actionService.UpdateActionResult(ctx, int32(resultID), data.Content)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to update action result", "error", err)
		return
	}

	// Convert to response format
	response := &ActionResultResponse{
		ID:          result.ID,
		GameID:      result.GameID,
		UserID:      result.UserID,
		PhaseID:     result.PhaseID,
		GMUserID:    result.GmUserID,
		Content:     result.Content,
		IsPublished: result.IsPublished.Bool,
	}

	if result.SentAt.Valid {
		response.SentAt = &result.SentAt.Time
	}

	render.Render(w, r, response)
}

// PublishActionResult publishes a single action result (GM only)
func (h *Handler) PublishActionResult(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_publish_action_result")()

	gameID := ctx.Value("gameID").(int32)

	resultIDStr := chi.URLParam(r, "resultId")
	resultID, err := strconv.ParseInt(resultIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid result ID")), "Invalid publish action result request")
		return
	}

	// Get authenticated user from context (set by middleware)
	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "No authenticated user in context")
		return
	}

	// Check permissions - must be GM
	phaseService := h.PhaseService
	canManage, err := phaseService.CanUserManagePhases(ctx, int32(gameID), int32(authUser.ID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to check phase management permission", "error", err)
		return
	}

	if !canManage {
		h.renderError(ctx, w, r, core.ErrForbidden("only the GM can publish action results"), "Publish action result forbidden")
		return
	}

	// Publish the action result
	actionService := h.ActionSubmissionService
	err = actionService.PublishActionResult(ctx, int32(resultID), int32(authUser.ID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to publish action result", "error", err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message":"Action result published successfully"}`))
}
