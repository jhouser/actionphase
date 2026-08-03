package phases

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"actionphase/pkg/core"
	gamesvc "actionphase/pkg/db/services"
	actionsvc "actionphase/pkg/db/services/actions"
	phasesvc "actionphase/pkg/db/services/phases"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// validateGMAccessAndResult checks GM permissions and validates the action result belongs to the game.
// Returns gameID, resultID, actionService, and error. This helper reduces ~40 lines of duplication per handler.
func (h *Handler) validateGMAccessAndResult(w http.ResponseWriter, r *http.Request) (int32, int32, *actionsvc.ActionSubmissionService, error) {
	// Parse gameID from URL params
	gameID := r.Context().Value("gameID").(int32)

	// Parse resultID from URL params
	resultIDStr := chi.URLParam(r, "resultId")
	resultID, err := strconv.ParseInt(resultIDStr, 10, 32)
	if err != nil {
		h.renderError(r.Context(), w, r, core.ErrInvalidRequest(fmt.Errorf("invalid result ID")), "Invalid validate g m access and result request")
		return 0, 0, nil, err
	}

	// Get authenticated user from context
	authUser := core.GetAuthenticatedUser(r.Context())
	if authUser == nil {
		h.renderError(r.Context(), w, r, core.ErrUnauthorized("authentication required"), "No authenticated user in context")
		return 0, 0, nil, fmt.Errorf("no authenticated user")
	}

	// Check GM permissions
	phaseService := &phasesvc.PhaseService{DB: h.App.Pool, Logger: h.App.ObsLogger}
	canManage, err := phaseService.CanUserManagePhases(r.Context(), int32(gameID), int32(authUser.ID))
	if err != nil {
		h.renderError(r.Context(), w, r, core.ErrInternalError(err), "Failed to check phase management permission", "error", err)
		return 0, 0, nil, err
	}

	if !canManage {
		h.renderError(r.Context(), w, r, core.ErrForbidden("only the GM can manage draft character updates"), "Validate g m access and result forbidden")
		return 0, 0, nil, fmt.Errorf("insufficient permissions")
	}

	// Verify the action result exists and belongs to this game
	actionService := &actionsvc.ActionSubmissionService{DB: h.App.Pool, Logger: h.App.ObsLogger, NotificationService: gamesvc.NewNotificationService(h.App.Pool, h.App.ObsLogger)}
	result, err := actionService.GetActionResult(r.Context(), int32(resultID))
	if err != nil {
		h.renderError(r.Context(), w, r, core.ErrNotFound("action result not found"), "Failed to get action result", "error", err)
		return 0, 0, nil, err
	}

	if result.GameID != int32(gameID) {
		h.renderError(r.Context(), w, r, core.ErrBadRequest(fmt.Errorf("action result does not belong to this game")), "Bad validate g m access and result request")
		return 0, 0, nil, fmt.Errorf("game mismatch")
	}

	return int32(gameID), int32(resultID), actionService, nil
}

// CreateDraftCharacterUpdate creates a draft character sheet update for an action result (GM only)
func (h *Handler) CreateDraftCharacterUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_create_draft_character_update")()

	// Validate GM access and get validated parameters
	gameID, resultID, actionService, err := h.validateGMAccessAndResult(w, r)
	if err != nil {
		return // Error response already sent by helper
	}

	// Parse request body
	data := &CreateDraftCharacterUpdateRequest{}
	if err := render.Bind(r, data); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Invalid create draft character update request", "error", err)
		return
	}

	// Get result to access user_id for validation
	result, err := actionService.GetActionResult(ctx, resultID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get action result", "error", err)
		return
	}

	// Use the character_id from the request and validate it belongs to the correct user/game
	// SECURITY: We must validate that the character belongs to the correct user and game
	characterID := data.CharacterID
	var validatedCharacterID int32
	query := `SELECT id FROM characters WHERE id = $1 AND user_id = $2 AND game_id = $3`
	err = h.App.Pool.QueryRow(ctx, query, characterID, result.UserID, gameID).Scan(&validatedCharacterID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrBadRequest(fmt.Errorf("character not found or does not belong to this user/game")), "Character validation failed", "error", err, "character_id", characterID, "user_id", result.UserID, "game_id", gameID)
		return
	}

	// Create the draft character update
	req := core.CreateDraftCharacterUpdateRequest{
		ActionResultID: int32(resultID),
		CharacterID:    characterID,
		ModuleType:     data.ModuleType,
		FieldName:      data.FieldName,
		FieldValue:     data.FieldValue,
		FieldType:      data.FieldType,
		Operation:      data.Operation,
	}

	draft, err := actionService.CreateDraftCharacterUpdate(ctx, req)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to create draft character update", "error", err)
		return
	}

	// Convert to response format
	response := &DraftCharacterUpdateResponse{
		ID:             draft.ID,
		ActionResultID: draft.ActionResultID,
		CharacterID:    draft.CharacterID,
		ModuleType:     draft.ModuleType,
		FieldName:      draft.FieldName,
		FieldValue:     draft.FieldValue.String,
		FieldType:      draft.FieldType,
		Operation:      draft.Operation,
		CreatedAt:      draft.CreatedAt.Time,
		UpdatedAt:      draft.UpdatedAt.Time,
	}

	render.Status(r, http.StatusCreated)
	render.Render(w, r, response)
}

// GetDraftCharacterUpdates retrieves all draft updates for an action result (GM only)
func (h *Handler) GetDraftCharacterUpdates(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_draft_character_updates")()

	// Validate GM access and get validated parameters
	_, resultID, actionService, err := h.validateGMAccessAndResult(w, r)
	if err != nil {
		return // Error response already sent by helper
	}

	// Get all draft updates for the action result
	drafts, err := actionService.GetDraftCharacterUpdates(ctx, resultID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get draft character updates", "error", err)
		return
	}

	// Convert to response format
	var response []DraftCharacterUpdateResponse
	for _, draft := range drafts {
		draftResp := DraftCharacterUpdateResponse{
			ID:             draft.ID,
			ActionResultID: draft.ActionResultID,
			CharacterID:    draft.CharacterID,
			ModuleType:     draft.ModuleType,
			FieldName:      draft.FieldName,
			FieldValue:     draft.FieldValue.String,
			FieldType:      draft.FieldType,
			Operation:      draft.Operation,
			CreatedAt:      draft.CreatedAt.Time,
			UpdatedAt:      draft.UpdatedAt.Time,
		}
		response = append(response, draftResp)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// UpdateDraftCharacterUpdate updates the field value of an existing draft (GM only)
func (h *Handler) UpdateDraftCharacterUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_update_draft_character_update")()

	// Validate GM access and get validated parameters
	_, _, actionService, err := h.validateGMAccessAndResult(w, r)
	if err != nil {
		return // Error response already sent by helper
	}

	draftIDStr := chi.URLParam(r, "draftId")
	draftID, err := strconv.ParseInt(draftIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid draft ID")), "Invalid update draft character update request")
		return
	}

	type UpdateDraftRequest struct {
		FieldValue string `json:"field_value" validate:"required"`
	}

	data := &UpdateDraftRequest{}
	if err := json.NewDecoder(r.Body).Decode(data); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Invalid update draft character update request", "error", err)
		return
	}

	// Update the draft
	draft, err := actionService.UpdateDraftCharacterUpdate(ctx, int32(draftID), data.FieldValue)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to update draft character update", "error", err)
		return
	}

	// Convert to response format
	response := &DraftCharacterUpdateResponse{
		ID:             draft.ID,
		ActionResultID: draft.ActionResultID,
		CharacterID:    draft.CharacterID,
		ModuleType:     draft.ModuleType,
		FieldName:      draft.FieldName,
		FieldValue:     draft.FieldValue.String,
		FieldType:      draft.FieldType,
		Operation:      draft.Operation,
		CreatedAt:      draft.CreatedAt.Time,
		UpdatedAt:      draft.UpdatedAt.Time,
	}

	render.Render(w, r, response)
}

// DeleteDraftCharacterUpdate deletes a draft character update (GM only)
func (h *Handler) DeleteDraftCharacterUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_delete_draft_character_update")()

	// Validate GM access and get validated parameters
	_, _, actionService, err := h.validateGMAccessAndResult(w, r)
	if err != nil {
		return // Error response already sent by helper
	}

	draftIDStr := chi.URLParam(r, "draftId")
	draftID, err := strconv.ParseInt(draftIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid draft ID")), "Invalid delete draft character update request")
		return
	}

	// Delete the draft
	err = actionService.DeleteDraftCharacterUpdate(ctx, int32(draftID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to delete draft character update", "error", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetDraftUpdateCount retrieves the count of draft updates for an action result (GM only)
func (h *Handler) GetDraftUpdateCount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_draft_update_count")()

	// Validate GM access and get validated parameters
	_, resultID, actionService, err := h.validateGMAccessAndResult(w, r)
	if err != nil {
		return // Error response already sent by helper
	}

	// Get the count
	count, err := actionService.GetDraftUpdateCount(ctx, resultID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get draft update count", "error", err)
		return
	}

	type CountResponse struct {
		Count int64 `json:"count"`
	}

	response := &CountResponse{Count: count}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
