package games

import (
	"actionphase/pkg/core"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// LeaveGame removes a user from game participants and deactivates their characters
func (h *Handler) LeaveGame(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_leave_game")()

	gameIDStr := chi.URLParam(r, "id")
	gameID, err := strconv.ParseInt(gameIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid game ID")), "Invalid leave game request")
		return
	}

	// Get user ID from JWT token
	userService := h.UserService
	userID, errResp := core.GetUserIDFromJWT(ctx, userService)
	if errResp != nil {
		h.renderError(ctx, w, r, errResp, "Failed to authenticate user from JWT")
		return
	}

	gameService := h.GameService
	applicationService := h.GameApplicationService

	// First, try to remove user from game participants (if they are a participant)
	// Use RemovePlayer which handles both participant removal and character deactivation
	participantRemoved := false
	err = gameService.RemovePlayer(ctx, int32(gameID), userID, userID) // Self-initiated leave
	if err != nil {
		// Log but don't fail - user might not be a participant (might just have an application)
		h.App.ObsLogger.Debug(ctx, "User not found in participants (might have application instead)", "game_id", gameID, "user_id", userID)
	} else {
		participantRemoved = true
		h.App.ObsLogger.Info(ctx, "User left game (participant removed, characters deactivated)", "game_id", gameID, "user_id", userID)
	}

	// Also check for and withdraw any pending applications
	application, err := applicationService.GetGameApplicationByUserAndGame(ctx, int32(gameID), userID)
	if err != nil {
		// User has no application - that's fine if they were a participant
		if !participantRemoved {
			h.renderError(ctx, w, r, core.ErrNotFound("you are not associated with this game"), "User is neither participant nor applicant", "error", err, "game_id", gameID, "user_id", userID)
			return
		}
	} else {
		// Delete the application if it's pending (allows them to reapply if they want).
		// Approved audience applications no longer reach here: ApproveGameApplication
		// deletes the audience application when it creates the participant, so a member
		// who leaves has only a participant row (removed above), not a lingering
		// application. Any pre-existing stale 'approved' rows from before that fix are
		// self-healed on the user's next apply/withdraw.
		if application.Status.String == core.ApplicationStatusPending {
			err = applicationService.DeleteGameApplication(ctx, application.ID, userID)
			if err != nil {
				h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to delete application", "error", err, "application_id", application.ID)
				return
			}
			h.App.ObsLogger.Info(ctx, "Deleted pending application", "application_id", application.ID, "game_id", gameID, "user_id", userID)
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetGameParticipants retrieves all participants for a game
func (h *Handler) GetGameParticipants(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_game_participants")()

	gameIDStr := chi.URLParam(r, "id")
	gameID, err := strconv.ParseInt(gameIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid game ID")), "Invalid get game participants request")
		return
	}

	gameService := h.GameService
	participants, err := gameService.GetGameParticipants(ctx, int32(gameID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get game participants", "error", err, "game_id", gameID)
		return
	}

	// Determine whether is_former_player should be redacted.
	// In anonymous games only GMs, co-GMs, and audience members may know which
	// participants are former players; regular players and non-participants cannot.
	redactFormerPlayer := false
	game, err := gameService.GetGame(ctx, int32(gameID))
	if err == nil && game.IsAnonymous {
		viewerID, errResp := core.GetUserIDFromJWT(ctx, h.UserService)
		if errResp != nil || !core.CanSeeUsernamesInAnonymousGame(ctx, h.App.Pool, *game, viewerID) {
			redactFormerPlayer = true
		}
	}

	// Convert to response format
	var response []map[string]interface{}
	for _, participant := range participants {
		role := participant.Role
		isFormerPlayer := participant.IsFormerPlayer
		// In anonymous games, viewers who can't see former-player status see them as
		// regular players instead — role spoofed to "player", flag cleared.
		if redactFormerPlayer && participant.IsFormerPlayer {
			role = "player"
			isFormerPlayer = false
		}

		participantData := map[string]interface{}{
			"id":       participant.ID,
			"game_id":  participant.GameID,
			"user_id":  participant.UserID,
			"username": participant.Username,
			// Note: Email is intentionally omitted for privacy
			"role":             role,
			"status":           participant.Status,
			"joined_at":        participant.JoinedAt.Time,
			"is_former_player": isFormerPlayer,
		}

		// Include avatar_url if present
		if participant.AvatarUrl.Valid {
			participantData["avatar_url"] = participant.AvatarUrl.String
		} else {
			participantData["avatar_url"] = nil
		}

		response = append(response, participantData)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Player Management Endpoints

// RemovePlayer removes a player from the game and deactivates their characters (GM only)
func (h *Handler) RemovePlayer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_remove_player")()

	gameIDStr := chi.URLParam(r, "id")
	gameID, err := strconv.ParseInt(gameIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid game ID")), "Invalid remove player request")
		return
	}

	userIDStr := chi.URLParam(r, "userId")
	targetUserID, err := strconv.ParseInt(userIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid user ID")), "Invalid remove player request")
		return
	}

	// Get requesting user ID from JWT token
	userService := h.UserService
	requestingUserID, errResp := core.GetUserIDFromJWT(ctx, userService)
	if errResp != nil {
		h.renderError(ctx, w, r, errResp, "Failed to authenticate user from JWT")
		return
	}

	gameService := h.GameService

	// Verify requesting user is the GM
	game, err := gameService.GetGame(ctx, int32(gameID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrNotFound("game not found"), "Failed to get game", "error", err, "game_id", gameID)
		return
	}

	if game.GmUserID != requestingUserID {
		h.renderError(ctx, w, r, core.ErrForbidden("only the GM can remove players"), "Non-GM attempted to remove player", "requesting_user_id", requestingUserID, "game_id", gameID)
		return
	}

	// Prevent GM from removing themselves
	if int32(targetUserID) == game.GmUserID {
		h.renderError(ctx, w, r, core.ErrConflict("GM cannot remove themselves from the game"), "GM attempted to remove themselves", "game_id", gameID, "gm_user_id", game.GmUserID)
		return
	}

	// Remove player and deactivate their characters
	err = gameService.RemovePlayer(ctx, int32(gameID), int32(targetUserID), requestingUserID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to remove player", "error", err, "game_id", gameID, "user_id", targetUserID)
		return
	}

	h.App.ObsLogger.Info(ctx, "Player removed from game", "game_id", gameID, "removed_user_id", targetUserID, "removed_by", requestingUserID)
	w.WriteHeader(http.StatusNoContent)
}

// AddParticipantDirectly adds a user to the game without application process (GM only).
// Request body: {"user_id": N, "role": "player"|"audience"}
func (h *Handler) AddParticipantDirectly(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_add_participant_directly")()

	gameIDStr := chi.URLParam(r, "id")
	gameID, err := strconv.ParseInt(gameIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid game ID")), "Invalid add participant directly request")
		return
	}

	var req struct {
		UserID int32  `json:"user_id"`
		Role   string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid request body")), "Invalid add participant directly request")
		return
	}

	if req.UserID == 0 {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("user_id is required")), "Invalid add participant directly request")
		return
	}
	if req.Role != "player" && req.Role != "audience" {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("role must be 'player' or 'audience'")), "Invalid add participant directly request")
		return
	}

	userService := h.UserService
	requestingUserID, errResp := core.GetUserIDFromJWT(ctx, userService)
	if errResp != nil {
		h.renderError(ctx, w, r, errResp, "Failed to authenticate user from JWT")
		return
	}

	gameService := h.GameService

	game, err := gameService.GetGame(ctx, int32(gameID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrNotFound("game not found"), "Failed to get game", "error", err, "game_id", gameID)
		return
	}

	if game.GmUserID != requestingUserID {
		h.renderError(ctx, w, r, core.ErrForbidden("only the GM can add participants directly"), "Non-GM attempted to add participant directly", "requesting_user_id", requestingUserID, "game_id", gameID)
		return
	}

	_, err = userService.GetUserByID(int(req.UserID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrNotFound("user not found"), "Target user not found", "error", err, "user_id", req.UserID)
		return
	}

	participant, err := gameService.AddParticipantWithRole(ctx, int32(gameID), req.UserID, req.Role)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to add participant directly", "error", err, "game_id", gameID, "user_id", req.UserID, "role", req.Role)
		return
	}

	h.App.ObsLogger.Info(ctx, "Participant added directly to game", "game_id", gameID, "added_user_id", req.UserID, "role", req.Role, "added_by", requestingUserID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(participant)
}

// PromoteToCoGM promotes an audience member to co-GM role (GM only)
func (h *Handler) PromoteToCoGM(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_promote_to_cogm")()

	gameIDStr := chi.URLParam(r, "id")
	gameID, err := strconv.ParseInt(gameIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid game ID")), "Invalid promote to co g m request")
		return
	}

	userIDStr := chi.URLParam(r, "userId")
	targetUserID, err := strconv.ParseInt(userIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid user ID")), "Invalid promote to co g m request")
		return
	}

	// Get requesting user ID from JWT token
	userService := h.UserService
	requestingUserID, errResp := core.GetUserIDFromJWT(ctx, userService)
	if errResp != nil {
		h.renderError(ctx, w, r, errResp, "Failed to authenticate user from JWT")
		return
	}

	gameService := h.GameService

	// Call service method to promote user
	err = gameService.PromoteToCoGM(ctx, int32(gameID), int32(targetUserID), requestingUserID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to promote to co-GM", "error", err, "game_id", gameID, "user_id", targetUserID)
		// Return 403 for permission errors, 400 for validation errors
		if err.Error() == "only the primary GM can promote users to co-GM" {
			h.renderError(ctx, w, r, core.ErrForbidden(err.Error()), "Promote to co g m forbidden", "error", err.Error())
			return
		}
		h.renderError(ctx, w, r, core.ErrBadRequest(err), "Bad promote to co g m request", "error", err)
		return
	}

	h.App.ObsLogger.Info(ctx, "User promoted to co-GM", "game_id", gameID, "promoted_user_id", targetUserID, "promoted_by", requestingUserID)
	w.WriteHeader(http.StatusNoContent)
}

// DemoteFromCoGM demotes a co-GM back to audience role (GM only)
func (h *Handler) DemoteFromCoGM(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_demote_from_cogm")()

	gameIDStr := chi.URLParam(r, "id")
	gameID, err := strconv.ParseInt(gameIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid game ID")), "Invalid demote from co g m request")
		return
	}

	userIDStr := chi.URLParam(r, "userId")
	targetUserID, err := strconv.ParseInt(userIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid user ID")), "Invalid demote from co g m request")
		return
	}

	// Get requesting user ID from JWT token
	userService := h.UserService
	requestingUserID, errResp := core.GetUserIDFromJWT(ctx, userService)
	if errResp != nil {
		h.renderError(ctx, w, r, errResp, "Failed to authenticate user from JWT")
		return
	}

	gameService := h.GameService

	// Call service method to demote user
	err = gameService.DemoteFromCoGM(ctx, int32(gameID), int32(targetUserID), requestingUserID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to demote from co-GM", "error", err, "game_id", gameID, "user_id", targetUserID)
		// Return 403 for permission errors, 400 for validation errors
		if err.Error() == "only the primary GM can demote co-GMs" {
			h.renderError(ctx, w, r, core.ErrForbidden(err.Error()), "Demote from co g m forbidden", "error", err.Error())
			return
		}
		h.renderError(ctx, w, r, core.ErrBadRequest(err), "Bad demote from co g m request", "error", err)
		return
	}

	h.App.ObsLogger.Info(ctx, "Co-GM demoted to audience", "game_id", gameID, "demoted_user_id", targetUserID, "demoted_by", requestingUserID)
	w.WriteHeader(http.StatusNoContent)
}

// TransitionPlayerToAudience moves a player to audience role without deactivating their characters (primary GM only)
func (h *Handler) TransitionPlayerToAudience(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_transition_player_to_audience")()

	gameIDStr := chi.URLParam(r, "id")
	gameID, err := strconv.ParseInt(gameIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid game ID")), "Invalid transition player to audience request")
		return
	}

	userIDStr := chi.URLParam(r, "userId")
	targetUserID, err := strconv.ParseInt(userIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid user ID")), "Invalid transition player to audience request")
		return
	}

	userService := h.UserService
	requestingUserID, errResp := core.GetUserIDFromJWT(ctx, userService)
	if errResp != nil {
		h.renderError(ctx, w, r, errResp, "Failed to authenticate user from JWT")
		return
	}

	gameService := h.GameService

	err = gameService.TransitionPlayerToAudience(ctx, int32(gameID), int32(targetUserID), requestingUserID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to transition player to audience", "error", err, "game_id", gameID, "user_id", targetUserID)
		if err.Error() == "only the primary GM can transition players to audience" {
			h.renderError(ctx, w, r, core.ErrForbidden(err.Error()), "Transition player to audience forbidden", "error", err.Error())
			return
		}
		h.renderError(ctx, w, r, core.ErrBadRequest(err), "Bad transition player to audience request", "error", err)
		return
	}

	h.App.ObsLogger.Info(ctx, "Player transitioned to audience", "game_id", gameID, "user_id", targetUserID, "transitioned_by", requestingUserID)
	w.WriteHeader(http.StatusNoContent)
}
