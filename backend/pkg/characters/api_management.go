package characters

import (
	"actionphase/pkg/core"
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// ApproveCharacter approves or rejects a character (GM only)
func (h *Handler) ApproveCharacter(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_approve_character")()

	characterIDStr := chi.URLParam(r, "id")
	characterID, err := strconv.ParseInt(characterIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid character ID")), "Invalid approve character request")
		return
	}

	data := &ApproveCharacterRequest{}
	if err := render.Bind(r, data); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Invalid approve character request", "error", err)
		return
	}

	// Validate status
	if data.Status != "approved" {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("status must be 'approved'")), "Invalid approve character request")
		return
	}

	// Get authenticated user
	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "No authenticated user found")
		return
	}

	// Verify user is GM of this game
	characterService := h.CharacterService
	character, err := characterService.GetCharacter(ctx, int32(characterID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get character", "error", err)
		return
	}

	gameService := h.GameService
	game, err := gameService.GetGame(ctx, character.GameID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get game", "error", err)
		return
	}

	// Check GM permissions (considers admin mode)
	if !core.IsUserGameMaster(r, authUser.ID, authUser.IsAdmin, *game, h.App.Pool) {
		h.renderError(ctx, w, r, core.ErrForbidden("only the GM can approve characters"), "Approve character forbidden")
		return
	}

	// Update character status
	updatedCharacter, err := characterService.ApproveCharacter(ctx, int32(characterID))

	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to update character status", "error", err)
		return
	}

	// Notify the character owner (if there is one)
	if updatedCharacter.UserID.Valid {
		notifSvc := h.NotificationService
		go func() {
			notifCtx := context.Background()
			if err := notifSvc.NotifyCharacterApproved(notifCtx, updatedCharacter.UserID.Int32, updatedCharacter.GameID, updatedCharacter.ID, updatedCharacter.Name); err != nil {
				h.App.ObsLogger.Warn(notifCtx, "Failed to send character approved notification", "error", err, "character_id", updatedCharacter.ID)
			}
		}()
	}

	// Convert to response format
	charType0 := updatedCharacter.CharacterType
	response := &CharacterResponse{
		ID:            updatedCharacter.ID,
		GameID:        updatedCharacter.GameID,
		Name:          updatedCharacter.Name,
		CharacterType: &charType0,
		Status:        updatedCharacter.Status.String,
		CreatedAt:     updatedCharacter.CreatedAt.Time,
		UpdatedAt:     updatedCharacter.UpdatedAt.Time,
	}

	if updatedCharacter.UserID.Valid {
		response.UserID = &updatedCharacter.UserID.Int32
	}

	render.Render(w, r, response)
}

// AssignNPC assigns an NPC to a user (GM only)
func (h *Handler) AssignNPC(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_assign_npc")()

	characterIDStr := chi.URLParam(r, "id")
	characterID, err := strconv.ParseInt(characterIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid character ID")), "Invalid assign NPC request")
		return
	}

	data := &AssignNPCRequest{}
	if err := render.Bind(r, data); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Invalid assign NPC request", "error", err)
		return
	}

	// Get user ID from token
	// Get authenticated user
	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "No authenticated user found")
		return
	}

	// Verify user is GM
	characterService := h.CharacterService
	character, err := characterService.GetCharacter(ctx, int32(characterID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get character", "error", err)
		return
	}

	gameService := h.GameService
	game, err := gameService.GetGame(ctx, character.GameID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get game", "error", err)
		return
	}

	// Check GM permissions (considers admin mode)
	if !core.IsUserGameMaster(r, authUser.ID, authUser.IsAdmin, *game, h.App.Pool) {
		h.renderError(ctx, w, r, core.ErrForbidden("only the GM can assign NPCs"), "Assign NPC forbidden")
		return
	}

	// Verify the assigned user is an audience member or the GM (for taking back control)
	// GM can assign to themselves without being in the audience
	if data.AssignedUserID != authUser.ID {
		participants, err := gameService.GetGameParticipants(ctx, character.GameID)
		if err != nil {
			h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get game participants", "error", err)
			return
		}

		// Check if assigned user is an audience member
		isAudience := false
		for _, participant := range participants {
			if participant.UserID == data.AssignedUserID && participant.Role == "audience" {
				isAudience = true
				break
			}
		}

		if !isAudience {
			h.renderError(ctx, w, r, core.ErrBadRequest(fmt.Errorf("NPCs can only be assigned to audience members")), "Bad assign NPC request")
			return
		}
	}

	// Assign NPC
	err = characterService.AssignNPCToUser(ctx, int32(characterID), data.AssignedUserID, authUser.ID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to assign NPC", "error", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ReassignCharacter reassigns an inactive character to a new owner (GM only)
func (h *Handler) ReassignCharacter(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_reassign_character")()

	characterIDStr := chi.URLParam(r, "id")
	characterID, err := strconv.ParseInt(characterIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid character ID")), "Invalid reassign character request")
		return
	}

	data := &ReassignCharacterRequest{}
	if err := render.Bind(r, data); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Invalid reassign character request", "error", err)
		return
	}

	// Get authenticated user
	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "No authenticated user found")
		return
	}

	// Get character and verify it exists
	characterService := h.CharacterService
	character, err := characterService.GetCharacter(ctx, int32(characterID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrNotFound("character not found"), "Failed to get character", "error", err)
		return
	}

	// Verify user is GM of this game
	gameService := h.GameService
	game, err := gameService.GetGame(ctx, character.GameID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get game", "error", err)
		return
	}

	// Check GM permissions (considers admin mode)
	if !core.IsUserGameMaster(r, authUser.ID, authUser.IsAdmin, *game, h.App.Pool) {
		h.renderError(ctx, w, r, core.ErrForbidden("only the GM can reassign characters"), "Reassign character forbidden")
		return
	}

	// Verify character is inactive
	if character.IsActive {
		h.renderError(ctx, w, r, core.ErrConflict("can only reassign inactive characters"), "Reassign character conflict")
		return
	}

	// Reassign character
	updatedCharacter, err := characterService.ReassignCharacter(ctx, int32(characterID), data.NewOwnerUserID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to reassign character", "error", err)
		return
	}

	h.App.ObsLogger.Info(ctx, "Character reassigned", "character_id", characterID, "new_owner", data.NewOwnerUserID, "reassigned_by", authUser.ID)

	// Convert to response format
	charType1 := updatedCharacter.CharacterType
	response := &CharacterResponse{
		ID:            updatedCharacter.ID,
		GameID:        updatedCharacter.GameID,
		Name:          updatedCharacter.Name,
		CharacterType: &charType1,
		Status:        updatedCharacter.Status.String,
		CreatedAt:     updatedCharacter.CreatedAt.Time,
		UpdatedAt:     updatedCharacter.UpdatedAt.Time,
	}

	if updatedCharacter.UserID.Valid {
		response.UserID = &updatedCharacter.UserID.Int32
	}

	render.Render(w, r, response)
}

// ListInactiveCharacters lists all inactive characters for a game (GM only)
func (h *Handler) ListInactiveCharacters(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_list_inactive_characters")()

	gameID := ctx.Value("gameID").(int32)
	// Check GM permissions (considers admin mode)
	if !ctx.Value("is_gm").(bool) {
		h.renderError(ctx, w, r, core.ErrForbidden("only the GM can view inactive characters"), "List inactive characters forbidden")
		return
	}

	// Get inactive characters
	characterService := h.CharacterService
	characters, err := characterService.ListInactiveCharacters(ctx, int32(gameID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to list inactive characters", "error", err)
		return
	}

	// Convert to response format
	response := make([]map[string]interface{}, 0)
	for _, char := range characters {
		charData := map[string]interface{}{
			"id":                      char.ID,
			"game_id":                 char.GameID,
			"name":                    char.Name,
			"character_type":          char.CharacterType,
			"status":                  char.Status.String,
			"is_active":               char.IsActive,
			"created_at":              char.CreatedAt.Time,
			"updated_at":              char.UpdatedAt.Time,
			"current_owner_username":  char.CurrentOwnerUsername,
			"original_owner_username": char.OriginalOwnerUsername,
		}

		if char.UserID.Valid {
			charData["user_id"] = char.UserID.Int32
		}
		if char.OriginalOwnerUserID.Valid {
			charData["original_owner_user_id"] = char.OriginalOwnerUserID.Int32
		}

		response = append(response, charData)
	}

	w.Header().Set("Content-Type", "application/json")
	render.JSON(w, r, response)
}

// RenameCharacter renames a character (GM or character owner)
func (h *Handler) RenameCharacter(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_rename_character")()

	characterIDStr := chi.URLParam(r, "id")
	characterID, err := strconv.ParseInt(characterIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid character ID")), "Invalid rename character request")
		return
	}

	data := &RenameCharacterRequest{}
	if err := render.Bind(r, data); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Invalid rename character request", "error", err)
		return
	}

	// Get authenticated user
	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "No authenticated user found")
		return
	}

	// Verify user can edit this character (owner or GM)
	characterService := h.CharacterService
	canEdit, err := characterService.CanUserEditCharacter(ctx, int32(characterID), authUser.ID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to check character edit permission", "error", err)
		return
	}

	if !canEdit {
		h.renderError(ctx, w, r, core.ErrForbidden("you do not have permission to rename this character"), "Character rename permission denied", "character_id", characterID, "user_id", authUser.ID)
		return
	}

	// Rename the character
	updatedCharacter, err := characterService.RenameCharacter(ctx, int32(characterID), data.Name)
	if err != nil {
		// Check if it's a duplicate name error
		if err.Error() == fmt.Sprintf("a character named '%s' already exists in this game", data.Name) {
			h.renderError(ctx, w, r, core.ErrConflict(err.Error()), "Rename character conflict", "error", err.Error())
			return
		}
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to rename character", "error", err)
		return
	}

	h.App.ObsLogger.Info(ctx, "Character renamed successfully",
		"character_id", characterID,
		"new_name", data.Name,
		"renamed_by", authUser.ID)

	// Convert to response format
	charType2 := updatedCharacter.CharacterType
	response := &CharacterResponse{
		ID:            updatedCharacter.ID,
		GameID:        updatedCharacter.GameID,
		Name:          updatedCharacter.Name,
		CharacterType: &charType2,
		Status:        updatedCharacter.Status.String,
		CreatedAt:     updatedCharacter.CreatedAt.Time,
		UpdatedAt:     updatedCharacter.UpdatedAt.Time,
	}

	if updatedCharacter.UserID.Valid {
		response.UserID = &updatedCharacter.UserID.Int32
	}

	render.Render(w, r, response)
}
