package handouts

import (
	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
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

// CreateHandout creates a new handout for a game
func (h *Handler) CreateHandout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_create_handout")()

	defer h.App.ObsLogger.LogOperation(ctx, "api_create_handout")()

	// Get gameID from URL
	gameID := ctx.Value("gameID").(int32)

	// Parse request
	data := &CreateHandoutRequest{}
	if err := render.Bind(r, data); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Failed to bind create handout request", "error", err)
		return
	}

	// Get user ID from JWT token
	userService := h.UserService
	userID, errResp := core.GetUserIDFromJWT(ctx, userService)
	if errResp != nil {
		h.renderError(ctx, w, r, errResp, "Failed to authenticate user from JWT")
		return
	}

	// Check if user is GM or Co-GM of the game
	if !ctx.Value("is_gm").(bool) {
		h.renderError(ctx, w, r, core.ErrForbidden("Only the GM can create handouts"), "Request rejected in create handout")
		return
	}

	// Create handout
	handoutService := h.HandoutService
	handout, err := handoutService.CreateHandout(ctx, int32(gameID), data.Title, data.Content, data.Status, userID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to create handout", "error", err)
		return
	}

	h.App.ObsLogger.Info(ctx, "Handout created successfully", "handout_id", handout.ID, "game_id", gameID)

	// If created directly in published state, notify players
	if handout.Status == "published" {
		go func() {
			notifCtx := context.Background()
			notifService := h.NotificationService
			if err := notifService.NotifyHandoutPublished(notifCtx, handout.GameID, handout.ID, handout.Title, userID); err != nil {
				h.App.ObsLogger.Warn(notifCtx, "Failed to send handout published notifications", "error", err, "handout_id", handout.ID)
			}
		}()
	}

	response := &HandoutResponse{
		ID:        handout.ID,
		GameID:    handout.GameID,
		Title:     handout.Title,
		Content:   handout.Content,
		Status:    handout.Status,
		CreatedAt: handout.CreatedAt,
		UpdatedAt: handout.UpdatedAt,
	}

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, response)
}

// GetHandout retrieves a specific handout
func (h *Handler) GetHandout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_handout")()

	defer h.App.ObsLogger.LogOperation(ctx, "api_get_handout")()

	// Get handoutID from URL
	handoutIDStr := chi.URLParam(r, "handoutId")
	handoutID, err := strconv.ParseInt(handoutIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid handout ID")), "Invalid handout ID", "error", err)
		return
	}

	// Get user ID from JWT token
	userService := h.UserService
	userID, errResp := core.GetUserIDFromJWT(ctx, userService)
	if errResp != nil {
		h.renderError(ctx, w, r, errResp, "Failed to authenticate user from JWT")
		return
	}

	// Get handout (service will check permissions)
	handoutService := h.HandoutService
	handout, err := handoutService.GetHandout(ctx, int32(handoutID), userID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrNotFound("Handout not found"), "Failed to get handout", "error", err)
		return
	}

	response := &HandoutResponse{
		ID:        handout.ID,
		GameID:    handout.GameID,
		Title:     handout.Title,
		Content:   handout.Content,
		Status:    handout.Status,
		CreatedAt: handout.CreatedAt,
		UpdatedAt: handout.UpdatedAt,
	}

	render.JSON(w, r, response)
}

// ListHandouts lists all handouts for a game
func (h *Handler) ListHandouts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_list_handouts")()

	defer h.App.ObsLogger.LogOperation(ctx, "api_list_handouts")()

	// Get gameID from URL
	gameID := ctx.Value("gameID").(int32)

	// Get user ID from JWT token
	userService := h.UserService
	userID, errResp := core.GetUserIDFromJWT(ctx, userService)
	if errResp != nil {
		h.renderError(ctx, w, r, errResp, "Failed to authenticate user from JWT")
		return
	}

	// Check if user is GM or Co-GM (they can see draft handouts)
	isGM := ctx.Value("is_gm").(bool)

	// List handouts
	handoutService := h.HandoutService
	handouts, err := handoutService.ListHandouts(ctx, int32(gameID), userID, isGM)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to list handouts", "error", err)
		return
	}

	// Convert to response format
	response := make([]*HandoutResponse, len(handouts))
	for i, handout := range handouts {
		response[i] = &HandoutResponse{
			ID:        handout.ID,
			GameID:    handout.GameID,
			Title:     handout.Title,
			Content:   handout.Content,
			Status:    handout.Status,
			CreatedAt: handout.CreatedAt,
			UpdatedAt: handout.UpdatedAt,
		}
	}

	render.JSON(w, r, response)
}

// UpdateHandout updates a handout
func (h *Handler) UpdateHandout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_update_handout")()

	defer h.App.ObsLogger.LogOperation(ctx, "api_update_handout")()

	// Get handoutID from URL
	handoutIDStr := chi.URLParam(r, "handoutId")
	handoutID, err := strconv.ParseInt(handoutIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid handout ID")), "Invalid handout ID", "error", err)
		return
	}

	// Parse request
	data := &UpdateHandoutRequest{}
	if err := render.Bind(r, data); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Failed to bind update handout request", "error", err)
		return
	}

	// Get user ID from JWT token
	userService := h.UserService
	userID, errResp := core.GetUserIDFromJWT(ctx, userService)
	if errResp != nil {
		h.renderError(ctx, w, r, errResp, "Failed to authenticate user from JWT")
		return
	}

	// Get existing handout to check ownership
	handoutService := h.HandoutService
	existingHandout, err := handoutService.GetHandout(ctx, int32(handoutID), userID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrNotFound("Handout not found"), "Failed to get handout", "error", err)
		return
	}

	// Check if user is GM of the game
	gameService := h.GameService
	game, err := gameService.GetGame(ctx, existingHandout.GameID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrNotFound("Handout not found"), "Failed to get game", "error", err)
		return
	}

	if errResp := h.verifyUserIsGM(ctx, game, userID); errResp != nil {
		h.renderError(ctx, w, r, errResp, "Request rejected in update handout")
		return
	}

	// Update handout
	handout, err := handoutService.UpdateHandout(ctx, int32(handoutID), data.Title, data.Content, data.Status, userID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to update handout", "error", err)
		return
	}

	h.App.ObsLogger.Info(ctx, "Handout updated successfully", "handout_id", handout.ID)

	response := &HandoutResponse{
		ID:        handout.ID,
		GameID:    handout.GameID,
		Title:     handout.Title,
		Content:   handout.Content,
		Status:    handout.Status,
		CreatedAt: handout.CreatedAt,
		UpdatedAt: handout.UpdatedAt,
	}

	render.JSON(w, r, response)
}

// DeleteHandout deletes a handout
func (h *Handler) DeleteHandout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_delete_handout")()

	defer h.App.ObsLogger.LogOperation(ctx, "api_delete_handout")()

	// Get handoutID from URL
	handoutIDStr := chi.URLParam(r, "handoutId")
	handoutID, err := strconv.ParseInt(handoutIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid handout ID")), "Invalid handout ID", "error", err)
		return
	}

	// Get user ID from JWT token
	userService := h.UserService
	userID, errResp := core.GetUserIDFromJWT(ctx, userService)
	if errResp != nil {
		h.renderError(ctx, w, r, errResp, "Failed to authenticate user from JWT")
		return
	}

	// Get existing handout to check ownership
	handoutService := h.HandoutService
	existingHandout, err := handoutService.GetHandout(ctx, int32(handoutID), userID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrNotFound("Handout not found"), "Failed to get handout", "error", err)
		return
	}

	// Check if user is GM of the game
	gameService := h.GameService
	game, err := gameService.GetGame(ctx, existingHandout.GameID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrNotFound("Handout not found"), "Failed to get game", "error", err)
		return
	}

	if errResp := h.verifyUserIsGM(ctx, game, userID); errResp != nil {
		h.renderError(ctx, w, r, errResp, "Request rejected in delete handout")
		return
	}

	// Delete handout
	err = handoutService.DeleteHandout(ctx, int32(handoutID), userID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to delete handout", "error", err)
		return
	}

	h.App.ObsLogger.Info(ctx, "Handout deleted successfully", "handout_id", handoutID)

	w.WriteHeader(http.StatusNoContent)
}

// PublishHandout publishes a draft handout
func (h *Handler) PublishHandout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_publish_handout")()

	defer h.App.ObsLogger.LogOperation(ctx, "api_publish_handout")()

	// Get handoutID from URL
	handoutIDStr := chi.URLParam(r, "handoutId")
	handoutID, err := strconv.ParseInt(handoutIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid handout ID")), "Invalid handout ID", "error", err)
		return
	}

	// Get user ID from JWT token
	userService := h.UserService
	userID, errResp := core.GetUserIDFromJWT(ctx, userService)
	if errResp != nil {
		h.renderError(ctx, w, r, errResp, "Failed to authenticate user from JWT")
		return
	}

	// Get existing handout to check ownership
	handoutService := h.HandoutService
	existingHandout, err := handoutService.GetHandout(ctx, int32(handoutID), userID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrNotFound("Handout not found"), "Failed to get handout", "error", err)
		return
	}

	// Check if user is GM of the game
	gameService := h.GameService
	game, err := gameService.GetGame(ctx, existingHandout.GameID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrNotFound("Handout not found"), "Failed to get game", "error", err)
		return
	}

	if errResp := h.verifyUserIsGM(ctx, game, userID); errResp != nil {
		h.renderError(ctx, w, r, errResp, "Request rejected in publish handout")
		return
	}

	// Publish handout
	handout, err := handoutService.PublishHandout(ctx, int32(handoutID), userID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to publish handout", "error", err)
		return
	}

	h.App.ObsLogger.Info(ctx, "Handout published successfully", "handout_id", handout.ID)

	// Notify players about the published handout
	go func() {
		notifCtx := context.Background()
		notifService := h.NotificationService
		if err := notifService.NotifyHandoutPublished(notifCtx, handout.GameID, handout.ID, handout.Title, userID); err != nil {
			h.App.ObsLogger.Warn(notifCtx, "Failed to send handout published notifications", "error", err, "handout_id", handout.ID)
		}
	}()

	response := &HandoutResponse{
		ID:        handout.ID,
		GameID:    handout.GameID,
		Title:     handout.Title,
		Content:   handout.Content,
		Status:    handout.Status,
		CreatedAt: handout.CreatedAt,
		UpdatedAt: handout.UpdatedAt,
	}

	render.JSON(w, r, response)
}

// UnpublishHandout unpublishes a published handout
func (h *Handler) UnpublishHandout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_unpublish_handout")()

	defer h.App.ObsLogger.LogOperation(ctx, "api_unpublish_handout")()

	// Get handoutID from URL
	handoutIDStr := chi.URLParam(r, "handoutId")
	handoutID, err := strconv.ParseInt(handoutIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid handout ID")), "Invalid handout ID", "error", err)
		return
	}

	// Get user ID from JWT token
	userService := h.UserService
	userID, errResp := core.GetUserIDFromJWT(ctx, userService)
	if errResp != nil {
		h.renderError(ctx, w, r, errResp, "Failed to authenticate user from JWT")
		return
	}

	// Get existing handout to check ownership
	handoutService := h.HandoutService
	existingHandout, err := handoutService.GetHandout(ctx, int32(handoutID), userID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrNotFound("Handout not found"), "Failed to get handout", "error", err)
		return
	}

	// Check if user is GM of the game
	gameService := h.GameService
	game, err := gameService.GetGame(ctx, existingHandout.GameID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrNotFound("Handout not found"), "Failed to get game", "error", err)
		return
	}

	if errResp := h.verifyUserIsGM(ctx, game, userID); errResp != nil {
		h.renderError(ctx, w, r, errResp, "Request rejected in unpublish handout")
		return
	}

	// Unpublish handout
	handout, err := handoutService.UnpublishHandout(ctx, int32(handoutID), userID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to unpublish handout", "error", err)
		return
	}

	h.App.ObsLogger.Info(ctx, "Handout unpublished successfully", "handout_id", handout.ID)

	response := &HandoutResponse{
		ID:        handout.ID,
		GameID:    handout.GameID,
		Title:     handout.Title,
		Content:   handout.Content,
		Status:    handout.Status,
		CreatedAt: handout.CreatedAt,
		UpdatedAt: handout.UpdatedAt,
	}

	render.JSON(w, r, response)
}
