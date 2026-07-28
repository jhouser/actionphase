package characters

import (
	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// CreateCharacter creates a new character for a game
func (h *Handler) CreateCharacter(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_create_character")()

	gameIDStr := chi.URLParam(r, "gameId")
	gameID, err := strconv.ParseInt(gameIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid game ID")), "Invalid create character request")
		return
	}

	data := &CreateCharacterRequest{}
	if err := render.Bind(r, data); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Invalid create character request", "error", err)
		return
	}

	// Validate required fields
	if data.Name == "" {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("character name is required")), "Invalid create character request")
		return
	}

	// Validate character type
	validTypes := []string{"player_character", "npc"}
	isValid := false
	for _, validType := range validTypes {
		if data.CharacterType == validType {
			isValid = true
			break
		}
	}
	if !isValid {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid character type")), "Invalid create character request")
		return
	}

	// Get authenticated user
	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "No authenticated user found")
		return
	}

	// Verify user can create characters for this game
	gameService := h.GameService
	game, err := gameService.GetGame(ctx, int32(gameID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get game", "error", err, "game_id", gameID)
		return
	}

	// Check permissions based on character type
	isGM := core.IsUserGameMaster(r, authUser.ID, authUser.IsAdmin, *game, h.App.Pool)

	if data.CharacterType == "player_character" {
		// GMs can create player characters for any player
		// Regular players can only create characters for themselves
		if !isGM {
			participants, err := gameService.GetGameParticipants(ctx, int32(gameID))
			if err != nil {
				h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get game participants", "error", err)
				return
			}

			isParticipant := false
			for _, participant := range participants {
				if participant.UserID == authUser.ID && participant.Role == "player" {
					isParticipant = true
					break
				}
			}

			if !isParticipant {
				h.renderError(ctx, w, r, core.ErrForbidden("only game participants can create player characters"), "Create character forbidden")
				return
			}
		}

		// If GM is creating the character, they must specify which player
		if isGM && data.UserID == nil {
			h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("user_id is required when GM creates player characters")), "Invalid create character request")
			return
		}
	} else {
		// For NPCs, only GM can create them (considers admin mode)
		if !isGM {
			h.renderError(ctx, w, r, core.ErrForbidden("only the GM can create NPCs"), "Create character forbidden")
			return
		}
	}

	// Create character
	characterService := h.CharacterService

	var reqUserID *int32
	if data.CharacterType == "player_character" {
		if isGM {
			// GM creating player character - use provided UserID (already validated as required above)
			reqUserID = data.UserID
		} else {
			// Regular player creating their own character - use authenticated user's ID
			reqUserID = &authUser.ID
		}
	}
	// For NPCs, UserID can be nil (GM-controlled) or assigned later

	character, err := characterService.CreateCharacter(ctx, core.CreateCharacterRequest{
		GameID:        int32(gameID),
		UserID:        reqUserID,
		Name:          data.Name,
		CharacterType: data.CharacterType,
	})

	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to create character", "error", err)
		return
	}

	// Convert to response format (CreateCharacter — always include character_type)
	charType := character.CharacterType
	response := &CharacterResponse{
		ID:            character.ID,
		GameID:        character.GameID,
		Name:          character.Name,
		CharacterType: &charType,
		Status:        character.Status.String,
		CreatedAt:     character.CreatedAt.Time,
		UpdatedAt:     character.UpdatedAt.Time,
	}

	if character.UserID.Valid {
		response.UserID = &character.UserID.Int32
	}

	render.Status(r, http.StatusCreated)
	render.Render(w, r, response)
}

// GetCharacter retrieves character details
func (h *Handler) GetCharacter(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_character")()

	characterIDStr := chi.URLParam(r, "id")
	characterID, err := strconv.ParseInt(characterIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid character ID")), "Invalid get character request")
		return
	}

	characterService := h.CharacterService
	character, err := characterService.GetCharacter(ctx, int32(characterID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get character", "error", err, "character_id", characterID)
		return
	}

	// Get game to check state for filtering
	gameService := h.GameService
	game, err := gameService.GetGame(ctx, character.GameID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get game", "error", err, "game_id", character.GameID)
		return
	}

	// Check if user is GM - pending/rejected characters should be visible to GMs AND the character owner
	authUser := core.GetAuthenticatedUser(ctx)
	var isGM bool
	var isOwner bool
	var isAssignedUser bool
	var userRole string
	if authUser != nil {
		isGM = core.IsUserGameMaster(r, authUser.ID, authUser.IsAdmin, *game, h.App.Pool)
		if character.UserID.Valid {
			isOwner = character.UserID.Int32 == authUser.ID
		}
		if isGM {
			userRole = "gm"
		} else {
			participants, err := gameService.GetGameParticipants(ctx, character.GameID)
			if err == nil {
				for _, p := range participants {
					if p.UserID == authUser.ID {
						userRole = p.Role
						break
					}
				}
			}
			if userRole == "" {
				userRole = "player"
			}
		}
		// Check if user is the assigned controller of this NPC
		if character.CharacterType == "npc" {
			queries := models.New(h.App.Pool)
			if assignment, err := queries.GetNPCAssignment(ctx, character.ID); err == nil {
				isAssignedUser = assignment.AssignedUserID == authUser.ID
			}
		}
	}

	// Filter pending/rejected characters for non-GMs, non-owners, and non-assigned users in in_progress games
	// This prevents information disclosure about OTHER players' characters that haven't been approved
	// But allows players to see their own pending/rejected characters, and audience members to see their assigned NPCs
	if game.State.String == "in_progress" && !isGM && !isOwner && !isAssignedUser {
		if character.Status.String == "pending" || character.Status.String == "rejected" {
			h.renderError(ctx, w, r, core.ErrNotFound("character not found"), "Get character not found")
			return
		}
	}

	// Determine if requester can see character_type in anonymous games.
	// GMs, co-GMs, and audience always can; regular players cannot.
	canSeeCharacterType := !game.IsAnonymous ||
		userRole == "gm" || userRole == "co_gm" || userRole == "audience"

	// Convert to response format
	response := &CharacterResponse{
		ID:        character.ID,
		GameID:    character.GameID,
		Name:      character.Name,
		Status:    character.Status.String,
		CreatedAt: character.CreatedAt.Time,
		UpdatedAt: character.UpdatedAt.Time,
	}

	if canSeeCharacterType {
		charType := character.CharacterType
		response.CharacterType = &charType
	}

	if character.UserID.Valid {
		response.UserID = &character.UserID.Int32
	}

	if character.AvatarUrl.Valid {
		response.AvatarURL = &character.AvatarUrl.String
	}

	render.Render(w, r, response)
}

// GetGameCharacters retrieves all characters for a game
func (h *Handler) GetGameCharacters(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_game_characters")()

	gameIDStr := chi.URLParam(r, "gameId")
	gameID, err := strconv.ParseInt(gameIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid game ID")), "Invalid get game characters request")
		return
	}

	// Get game to check state for filtering
	gameService := h.GameService
	game, err := gameService.GetGame(ctx, int32(gameID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get game", "error", err, "game_id", gameID)
		return
	}

	// Get authenticated user to check role (GM, co-GM, audience, or player)
	authUser := core.GetAuthenticatedUser(ctx)
	var isGM bool
	var userRole string
	if authUser != nil {
		isGM = core.IsUserGameMaster(r, authUser.ID, authUser.IsAdmin, *game, h.App.Pool)
		if isGM {
			userRole = "gm"
		} else {
			// Check if user is a participant and get their role
			participants, err := gameService.GetGameParticipants(ctx, int32(gameID))
			if err != nil {
				h.App.ObsLogger.Error(ctx, "Failed to get game participants", "error", err, "game_id", gameID)
				// Don't fail the request, just assume regular player
				userRole = "player"
			} else {
				for _, p := range participants {
					if p.UserID == authUser.ID {
						userRole = p.Role
						break
					}
				}
				if userRole == "" {
					userRole = "player" // Default for authenticated users not in participants list
				}
			}
		}
	}

	characterService := h.CharacterService
	characters, err := characterService.GetCharactersByGame(ctx, int32(gameID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get game characters", "error", err, "game_id", gameID)
		return
	}

	// Filter characters based on game state and user role
	// When game is in_progress:
	// - GMs, co-GMs, and audience see ALL characters (including pending/rejected)
	// - Regular players see approved/active characters PLUS their own pending/rejected characters
	// Frontend will handle additional filtering for recipient selection (all users, including GMs)
	filteredCharacters := make([]models.GetCharactersByGameRow, 0)
	for _, char := range characters {
		// If game is in_progress and user is NOT GM/co-GM/audience, exclude OTHER players' pending/rejected characters
		// BUT always include the user's own characters regardless of status
		if !isGM && userRole != "co_gm" && userRole != "audience" {
			if char.Status.String == "pending" || char.Status.String == "rejected" {
				// Skip OTHER players' pending/rejected characters, but show user's own
				if authUser == nil || !char.UserID.Valid || char.UserID.Int32 != authUser.ID {
					continue
				}
			}
		}

		filteredCharacters = append(filteredCharacters, char)
	}

	// Helper function to determine if user can see player names in anonymous mode
	// GMs, co-GMs, and audience can see player names even in anonymous mode
	// Only regular players have player names hidden from them
	canSeePlayerNames := func(isAnonymous bool, role string) bool {
		if !isAnonymous {
			return true
		}
		return role == "gm" || role == "co_gm" || role == "audience"
	}

	// Convert to response format
	// Initialize as empty slice to ensure JSON encodes as [] not null
	response := make([]map[string]interface{}, 0)
	for _, char := range filteredCharacters {
		charData := map[string]interface{}{
			"id":             char.ID,
			"game_id":        char.GameID,
			"name":           char.Name,
			"character_type": char.CharacterType,
			"status":         char.Status,
			"created_at":     char.CreatedAt.Time,
			"updated_at":     char.UpdatedAt.Time,
		}

		// Only include player information if user is allowed to see it in anonymous mode
		if canSeePlayerNames(game.IsAnonymous, userRole) {
			if char.UserID.Valid {
				charData["user_id"] = char.UserID.Int32
			}
			if char.OwnerUsername.Valid {
				charData["username"] = char.OwnerUsername.String
			}
			if char.AssignedUserID.Valid {
				charData["assigned_user_id"] = char.AssignedUserID.Int32
			}
			if char.AssignedUsername.Valid {
				charData["assigned_username"] = char.AssignedUsername.String
			}
		}

		// Avatar is always visible regardless of anonymous mode
		if char.AvatarUrl.Valid {
			charData["avatar_url"] = char.AvatarUrl.String
		}

		response = append(response, charData)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetUserControllableCharacters retrieves all characters the current user can control in a game
func (h *Handler) GetUserControllableCharacters(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_user_controllable_characters")()

	gameIDStr := chi.URLParam(r, "gameId")
	gameID, err := strconv.ParseInt(gameIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid game ID")), "Invalid get user controllable characters request")
		return
	}

	// Get user ID from token
	userID, err := h.getUserIDFromToken(r)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized(err.Error()), "Failed to get user from token", "error", err)
		return
	}

	characterService := h.CharacterService
	characters, err := characterService.GetUserControllableCharacters(ctx, int32(gameID), userID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get user controllable characters", "error", err, "game_id", gameID, "user_id", userID)
		return
	}

	// Convert to response format
	// Initialize as empty slice to ensure JSON encodes as [] not null
	response := make([]map[string]interface{}, 0)
	for _, char := range characters {
		charData := map[string]interface{}{
			"id":             char.ID,
			"game_id":        char.GameID,
			"name":           char.Name,
			"character_type": char.CharacterType,
			"created_at":     char.CreatedAt.Time,
			"updated_at":     char.UpdatedAt.Time,
		}

		if char.UserID.Valid {
			charData["user_id"] = char.UserID.Int32
		}
		if char.Status.Valid {
			charData["status"] = char.Status.String
		}
		if char.AvatarUrl.Valid {
			charData["avatar_url"] = char.AvatarUrl.String
		}

		response = append(response, charData)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetUserControllableCharactersAcrossGames retrieves every character the current
// user can control across all their in_progress games. Unlike the per-game
// endpoint this takes no game in the path, so it can back surfaces that have no
// game in scope (e.g. the global Utility Drawer).
//
// Each entry carries its game context (title, state, flags) and the user's role
// in that game, which the client needs to resolve sheet permissions without a
// request per game.
func (h *Handler) GetUserControllableCharactersAcrossGames(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_user_controllable_characters_across_games")()

	userID, err := h.getUserIDFromToken(r)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized(err.Error()), "Failed to get user from token", "error", err)
		return
	}

	characters, err := h.CharacterService.GetUserControllableCharactersAcrossGames(ctx, userID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get cross-game controllable characters", "error", err, "user_id", userID)
		return
	}

	// Initialize as empty slice to ensure JSON encodes as [] not null
	response := make([]map[string]interface{}, 0)
	for _, char := range characters {
		charData := map[string]interface{}{
			"id":                    char.ID,
			"game_id":               char.GameID,
			"name":                  char.Name,
			"character_type":        char.CharacterType,
			"created_at":            char.CreatedAt.Time,
			"updated_at":            char.UpdatedAt.Time,
			"game_title":            char.GameTitle,
			"game_is_anonymous":     char.GameIsAnonymous,
			"game_portrait_avatars": char.GamePortraitAvatars,
			"user_role":             char.UserRole,
		}

		if char.UserID.Valid {
			charData["user_id"] = char.UserID.Int32
		}
		if char.Status.Valid {
			charData["status"] = char.Status.String
		}
		if char.AvatarUrl.Valid {
			charData["avatar_url"] = char.AvatarUrl.String
		}
		if char.GameState.Valid {
			charData["game_state"] = char.GameState.String
		}

		response = append(response, charData)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// DeleteCharacter deletes a character (GM only, character must have no activity)
func (h *Handler) DeleteCharacter(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_delete_character")()

	characterIDStr := chi.URLParam(r, "id")
	characterID, err := strconv.ParseInt(characterIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid character ID")), "Invalid delete character request")
		return
	}

	// Get authenticated user
	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "No authenticated user found")
		return
	}

	// Get character to check game ownership
	characterService := h.CharacterService
	character, err := characterService.GetCharacter(ctx, int32(characterID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrNotFound("character not found"), "Failed to get character", "error", err, "character_id", characterID)
		return
	}

	// Get game to check GM permissions
	gameService := h.GameService
	game, err := gameService.GetGame(ctx, character.GameID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get game", "error", err, "game_id", character.GameID)
		return
	}

	// Only GM can delete characters
	isGM := core.IsUserGameMaster(r, authUser.ID, authUser.IsAdmin, *game, h.App.Pool)
	if !isGM {
		h.renderError(ctx, w, r, core.ErrForbidden("only the GM can delete characters"), "Delete character forbidden")
		return
	}

	// Attempt to delete character
	err = characterService.DeleteCharacter(ctx, int32(characterID))
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to delete character", "error", err, "character_id", characterID)
		// Check if error is about activity - return 400 Bad Request
		if err.Error() == "cannot delete character with existing messages" ||
			err.Error() == "cannot delete character with existing action submissions" {
			h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Invalid delete character request", "error", err)
			return
		}
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to delete character", "error", err)
		return
	}

	// Success - return 204 No Content
	w.WriteHeader(http.StatusNoContent)
}
